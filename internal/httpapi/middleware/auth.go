package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/response"
	"github.com/zatunohito/tarikihonganncalendar/internal/repository"
)

func RequireAuth(sessions repository.SessionRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := GetRequestID(r.Context())

			cookie, err := r.Cookie("session")
			if err != nil {
				response.Unauthorized(w, reqID, "not authenticated")
				return
			}

			tokenHash := sha256.Sum256([]byte(cookie.Value))
			session, err := sessions.FindByTokenHash(r.Context(), tokenHash[:])
			if err != nil {
				response.InternalError(w, reqID, "internal error")
				return
			}
			if session == nil {
				response.Unauthorized(w, reqID, "invalid session")
				return
			}

			if subtle.ConstantTimeCompare(tokenHash[:], session.TokenHash) != 1 {
				response.Unauthorized(w, reqID, "invalid session")
				return
			}

			if time.Now().After(session.ExpiresAt) {
				response.Unauthorized(w, reqID, "session expired")
				return
			}

			ctx := context.WithValue(r.Context(), CtxKeyUserID, session.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
