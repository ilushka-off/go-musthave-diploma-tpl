package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ilushka-off/go-musthave-diploma-tpl/internal/auth"
)

type userIDKey struct {
}

func tokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(auth.CookieName)
	if err == nil {
		return cookie.Value
	}
	headerValue := r.Header.Get("Authorization")
	if value, ok := strings.CutPrefix(headerValue, "Bearer "); ok {
		return value
	}
	return ""
}

func Auth(tokens *auth.TokenManager) func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := tokenFromRequest(r)
			if token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID, err := tokens.Parse(token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(userIDKey{}).(int)
	return userID, ok
}
