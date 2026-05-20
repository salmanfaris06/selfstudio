package api

import (
	"net/http"

	"selfstudio/agent/internal/auth"
)

func RequireAuth(manager *auth.Manager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if manager == nil {
			writeAPIError(w, http.StatusInternalServerError, "AUTH_UNAVAILABLE", "Layanan auth belum siap.", "Restart aplikasi lalu coba lagi.")
			return
		}

		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil || !manager.Authenticated(cookie.Value) {
			writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi operator tidak aktif.", "Login ulang dengan PIN/password operator.")
			return
		}

		next.ServeHTTP(w, r)
	})
}
