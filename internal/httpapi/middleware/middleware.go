package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func LimitBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func drainAndClose(r io.ReadCloser) {
	io.Copy(io.Discard, r)
	r.Close()
}

type ctxKey string

const (
	CtxKeyRequestID  ctxKey = "request_id"
	CtxKeyUserID     ctxKey = "user_id"
)

func GetRequestID(ctx context.Context) string {
	v, _ := ctx.Value(CtxKeyRequestID).(string)
	return v
}

func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(CtxKeyUserID).(string)
	return v
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		ctx := context.WithValue(r.Context(), CtxKeyRequestID, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"requestId", GetRequestID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.status,
			"latencyMs", time.Since(start).Milliseconds(),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"requestId", GetRequestID(r.Context()),
					"panic", rec,
				)
				http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"internal server error"}}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS answers cross-origin requests from allowedOrigin. The preflight is
// answered here and never reaches what sits behind it: a browser sends
// OPTIONS without credentials, so letting it through to the auth check
// would fail every cross-origin request with a 401 before the real one runs.
//
// An empty allowedOrigin means same-origin only — no headers go out, and a
// browser refuses the cross-origin call on its own.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				// Callers are identified by X-User-ID or Authorization, so a
				// browser has to be told it may send them.
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Cookie, X-User-ID, Authorization")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
