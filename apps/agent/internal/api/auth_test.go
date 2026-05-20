package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"selfstudio/agent/internal/auth"
)

func TestAuthLoginSessionAndLogout(t *testing.T) {
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	handler := NewAuthHandler(manager)

	loginBody := bytes.NewBufferString(`{"pin":"123456"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", loginBody)
	loginRec := httptest.NewRecorder()
	handler.Login(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginRec.Code, http.StatusOK)
	}

	cookie := findCookie(loginRec.Result().Cookies(), auth.SessionCookieName)
	if cookie == nil {
		t.Fatal("session cookie was not set")
	}
	if !cookie.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want Lax", cookie.SameSite)
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	sessionReq.AddCookie(cookie)
	sessionRec := httptest.NewRecorder()
	handler.Session(sessionRec, sessionReq)

	var sessionResponse DataResponse[AuthSessionData]
	if err := json.NewDecoder(sessionRec.Body).Decode(&sessionResponse); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if !sessionResponse.Data.Authenticated {
		t.Fatal("expected authenticated session")
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(cookie)
	logoutRec := httptest.NewRecorder()
	handler.Logout(logoutRec, logoutReq)

	if manager.Authenticated(cookie.Value) {
		t.Fatal("session should be cleared after logout")
	}
}

func TestAuthLoginRejectsInvalidPINWithoutLeakingSecret(t *testing.T) {
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	handler := NewAuthHandler(manager)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"pin":"000000"}`))
	rec := httptest.NewRecorder()
	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("123456")) {
		t.Fatal("response leaked configured PIN")
	}

	var response ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error.Code != "INVALID_PIN" {
		t.Fatalf("code = %q, want INVALID_PIN", response.Error.Code)
	}
}

func TestSessionWithoutCookieIsUnauthenticated(t *testing.T) {
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	handler := NewAuthHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	rec := httptest.NewRecorder()
	handler.Session(rec, req)

	var response DataResponse[AuthSessionData]
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Authenticated {
		t.Fatal("expected unauthenticated session")
	}
}

func TestRequireAuthRejectsMissingSession(t *testing.T) {
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	protected := RequireAuth(manager, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("protected handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/protected", nil)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuthAllowsValidSession(t *testing.T) {
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	called := false
	protected := RequireAuth(manager, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("protected handler was not called")
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
