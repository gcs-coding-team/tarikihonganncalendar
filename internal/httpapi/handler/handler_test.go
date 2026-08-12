package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

func newRequest(method, path, body string) *http.Request {
	if body == "" {
		return httptest.NewRequest(method, path, nil)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func serve(mux http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestUploadJobNeedsAnAccount(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())

	rr := call(t, mux, http.MethodPost, "/v1/uploads/jobs", "",
		`{"contentType":"image/png","filename":"sample.png"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous upload: got %d want 401", rr.Code)
	}

	rr = call(t, mux, http.MethodPost, "/v1/uploads/jobs", "alice",
		`{"contentType":"image/png","filename":"sample.png"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload job create: got %d body=%s", rr.Code, rr.Body.String())
	}
}

// The pre-password shortcuts have to stay shut unless someone opts in. A
// trusted X-User-ID makes every password on every account decorative.
func TestDevAuthShortcutsAreOffByDefault(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())

	req := newRequest(http.MethodGet, "/v1/events", "")
	req.Header.Set("X-User-ID", "somebody-else")
	if rr := serve(mux, req); rr.Code != http.StatusUnauthorized {
		t.Fatalf("X-User-ID was trusted: got %d body=%s", rr.Code, rr.Body.String())
	}

	// Passwordless session creation should not even be routed.
	if rr := call(t, mux, http.MethodPost, "/v1/auth/sessions", "",
		`{"userId":"user-1","name":"Alice"}`); rr.Code != http.StatusNotFound {
		t.Fatalf("session endpoint reachable: got %d body=%s", rr.Code, rr.Body.String())
	}

	// With DevAuth on both work again — that is what the flag is for.
	dev := NewHandler(repository.NewMemoryRepository(), Options{DevAuth: true})
	req = newRequest(http.MethodGet, "/v1/events", "")
	req.Header.Set("X-User-ID", "somebody-else")
	if rr := serve(dev, req); rr.Code != http.StatusOK {
		t.Fatalf("DevAuth did not re-open X-User-ID: got %d", rr.Code)
	}
	if rr := serve(dev, newRequest(http.MethodPost, "/v1/auth/sessions",
		`{"userId":"user-1","name":"Alice"}`)); rr.Code != http.StatusCreated {
		t.Fatalf("DevAuth did not re-open sessions: got %d body=%s", rr.Code, rr.Body.String())
	}
}

// A session token must not be derivable from the account it belongs to.
func TestDevSessionTokenIsNotGuessable(t *testing.T) {
	dev := NewHandler(repository.NewMemoryRepository(), Options{DevAuth: true})
	rr := serve(dev, newRequest(http.MethodPost, "/v1/auth/sessions",
		`{"userId":"user-1","name":"Alice"}`))
	var out struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Data.Token == "sess-user-1" {
		t.Fatal("token is still derived from the user id")
	}
	if len(out.Data.Token) < 32 {
		t.Fatalf("token is too short to be unguessable: %q", out.Data.Token)
	}
}

func TestEventsAndColoniesEndpoints(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())

	rr := call(t, mux, http.MethodPost, "/v1/events", "user1",
		`{"title":"学校行事","description":"体育館集合","startAt":"2026-07-25T09:00:00Z","endAt":"2026-07-25T12:00:00Z","allDay":false}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create event: got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = call(t, mux, http.MethodGet, "/v1/events", "user1", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list events: got %d body=%s", rr.Code, rr.Body.String())
	}
	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("expected 1 event, got %d", len(listResp.Data))
	}

	// Another account must not see it.
	rr = call(t, mux, http.MethodGet, "/v1/events", "user2", "")
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode other list: %v", err)
	}
	if len(listResp.Data) != 0 {
		t.Fatalf("events leaked to another account: %s", rr.Body.String())
	}

	colony := newColony(t, mux, "user1", "3年1組")

	rr = call(t, mux, http.MethodPost, "/v1/colonies/"+colony.Data.ID+"/shared-items", "user1",
		`{"sourceType":"TASK","sourceId":"task-1"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("share: got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = call(t, mux, http.MethodPost, "/v1/colonies/"+colony.Data.ID+"/shared-items", "user1",
		`{"sourceType":"TASK","sourceId":"task-1"}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("sharing the same task twice: got %d want 409", rr.Code)
	}
}

func decodeInto(t *testing.T, raw []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
}
