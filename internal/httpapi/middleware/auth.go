package middleware

import (
	"context"
	"crypto/sha256"
	"net/http"

	"github.com/zatunohito/tarikihonganncalendar/internal/repository"
)

func RequireAuth(sessions repository.SessionRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"not authenticated"}}`, http.StatusUnauthorized)
				return
			}

			tokenHash := sha256.Sum256([]byte(cookie.Value))
			session, err := sessions.FindByTokenHash(r.Context(), tokenHash[:])
			if err != nil {
				http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"internal error"}}`, http.StatusInternalServerError)
				return
			}
			if session == nil {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid session"}}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), CtxKeyUserID, session.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
