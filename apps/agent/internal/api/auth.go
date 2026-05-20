package api

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
)

const maxLoginBodyBytes = 1024

type AuthHandler struct {
	manager       *auth.Manager
	activityStore *activity.Store
}

type LoginRequest struct {
	PIN string `json:"pin"`
}

type AuthSessionData struct {
	Authenticated bool `json:"authenticated"`
}

func NewAuthHandler(manager *auth.Manager) AuthHandler {
	return AuthHandler{manager: manager}
}

func NewAuthHandlerWithActivity(manager *auth.Manager, activityStore *activity.Store) AuthHandler {
	return AuthHandler{manager: manager, activityStore: activityStore}
}

func (h AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeAPIError(w, http.StatusInternalServerError, "AUTH_UNAVAILABLE", "Layanan auth belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}

	request, ok := decodeLoginRequest(w, r)
	if !ok {
		return
	}

	if request.PIN == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "PIN/password wajib diisi.", "Masukkan PIN/password lalu coba lagi.")
		return
	}

	token, expiresAt, err := h.manager.LoginForKey(clientKey(r), request.PIN)
	if errors.Is(err, auth.ErrRateLimited) {
		h.recordActivity("login.failure", activity.ResultFailure, "Login ditolak karena terlalu banyak percobaan gagal.")
		writeAPIError(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "Terlalu banyak percobaan PIN/password gagal.", "Tunggu sebentar lalu coba lagi atau hubungi admin teknis.")
		return
	}
	if errors.Is(err, auth.ErrInvalidPIN) {
		h.recordActivity("login.failure", activity.ResultFailure, "Login gagal karena PIN/password tidak valid.")
		writeAPIError(w, http.StatusUnauthorized, "INVALID_PIN", "PIN/password tidak valid.", "Coba lagi atau hubungi admin teknis.")
		return
	}
	if err != nil {
		h.recordActivity("login.failure", activity.ResultFailure, "Sesi operator gagal dibuat.")
		writeAPIError(w, http.StatusInternalServerError, "AUTH_SESSION_FAILED", "Sesi operator gagal dibuat.", "Coba login ulang. Jika tetap gagal, restart aplikasi.")
		return
	}

	sessionID := activitySessionID(token)
	h.recordActivityWithRefs("login.success", activity.ResultSuccess, "Operator login berhasil.", nil, &sessionID)
	setSessionCookie(w, token, expiresAt, r.TLS != nil)
	writeData(w, http.StatusOK, AuthSessionData{Authenticated: true})
}

func (h AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeAPIError(w, http.StatusInternalServerError, "AUTH_UNAVAILABLE", "Layanan auth belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}

	result := activity.ResultFailure
	message := "Logout diminta tanpa sesi operator aktif."
	var sessionID *string
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil {
		removed := h.manager.Logout(cookie.Value)
		if removed {
			result = activity.ResultSuccess
			message = "Operator logout berhasil."
			id := activitySessionID(cookie.Value)
			sessionID = &id
		}
	}

	h.recordActivityWithRefs("logout.success", result, message, nil, sessionID)
	clearSessionCookie(w, r.TLS != nil)
	writeData(w, http.StatusOK, AuthSessionData{Authenticated: false})
}

func (h AuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeAPIError(w, http.StatusInternalServerError, "AUTH_UNAVAILABLE", "Layanan auth belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}

	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		writeData(w, http.StatusOK, AuthSessionData{Authenticated: false})
		return
	}

	writeData(w, http.StatusOK, AuthSessionData{Authenticated: h.manager.Authenticated(cookie.Value)})
}

func (h AuthHandler) recordActivity(actionType string, result activity.Result, message string) {
	h.recordActivityWithRefs(actionType, result, message, nil, nil)
}

func (h AuthHandler) recordActivityWithRefs(actionType string, result activity.Result, message string, stationID *string, sessionID *string) {
	if h.activityStore == nil {
		return
	}
	h.activityStore.RecordWithRefs(actionType, result, message, stationID, sessionID)
}

func decodeLoginRequest(w http.ResponseWriter, r *http.Request) (LoginRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request LoginRequest
	if err := decoder.Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Format request login tidak valid.", "Masukkan PIN/password lalu coba lagi.")
		return LoginRequest{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Format request login tidak valid.", "Kirim satu payload JSON login yang valid.")
		return LoginRequest{}, false
	}

	return request, true
}

func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		return "local"
	}

	return host
}

func activitySessionID(token string) string {
	if len(token) <= 12 {
		return "sess_" + token
	}
	return "sess_" + token[:12]
}

func setSessionCookie(w http.ResponseWriter, token string, expires time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}
