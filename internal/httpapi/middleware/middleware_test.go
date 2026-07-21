package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRequestID(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxKeyRequestID, "abc-123")
	if id := GetRequestID(ctx); id != "abc-123" {
		t.Fatalf("expected abc-123, got %s", id)
	}
}

func TestGetRequestID_Empty(t *testing.T) {
	if id := GetRequestID(context.Background()); id != "" {
		t.Fatal("expected empty")
	}
}

func TestGetUserID(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxKeyUserID, "user-1")
	if id := GetUserID(ctx); id != "user-1" {
		t.Fatalf("expected user-1, got %s", id)
	}
}

func TestGetUserID_Empty(t *testing.T) {
	if id := GetUserID(context.Background()); id != "" {
		t.Fatal("expected empty")
	}
}

func TestRequestID(t *testing.T) {
	w := httptest.NewRecorder()
	var gotID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if gotID == "" {
		t.Fatal("expected non-empty request ID")
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header")
	}
	if gotID != w.Header().Get("X-Request-ID") {
		t.Fatal("context and header should match")
	}
}

func TestRecoverer(t *testing.T) {
	w := httptest.NewRecorder()
	handler := Recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	handler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCORS(t *testing.T) {
	origin := "https://app.example.com"
	mw := CORS(origin)

	t.Run("sets headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

		if w.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Fatal("origin mismatch")
		}
		if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Fatal("expected credentials true")
		}
	})

	t.Run("handles preflight", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("OPTIONS", "/", nil)
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next should not be called for OPTIONS")
		})).ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", w.Code)
		}
	})

	t.Run("empty origin", func(t *testing.T) {
		mwEmpty := CORS("")
		w := httptest.NewRecorder()
		mwEmpty(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("expected no origin header")
		}
	})
}

func TestResponseWriter(t *testing.T) {
	rw := &responseWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
	rw.WriteHeader(http.StatusNotFound)
	if rw.status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rw.status)
	}
}
