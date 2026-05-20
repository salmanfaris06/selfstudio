package api

import "net/http"

func RequireTrustedOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = r.Header.Get("Referer")
		}
		if origin != "" {
			if _, ok := allowedWebOrigins[origin]; !ok {
				writeAPIError(w, http.StatusForbidden, "UNTRUSTED_ORIGIN", "Origin request tidak dipercaya.", "Gunakan dashboard lokal Selfstudio resmi.")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
