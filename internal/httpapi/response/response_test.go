package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	OK(w, "req-123", map[string]string{"key": "val"})

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Data      map[string]string `json:"data"`
		RequestID string            `json:"requestId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.RequestID != "req-123" {
		t.Fatalf("expected req-123, got %s", body.RequestID)
	}
	if body.Data["key"] != "val" {
		t.Fatalf("expected val, got %s", body.Data["key"])
	}
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	Created(w, "req-456", map[string]int{"id": 42})

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body struct {
		Data      map[string]int `json:"data"`
		RequestID string         `json:"requestId"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Data["id"] != 42 {
		t.Fatal("expected id 42")
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	NoContent(w)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestListResponse(t *testing.T) {
	w := httptest.NewRecorder()
	ListResponse(w, "req-789", []string{"a", "b"}, "cursor-next")

	var body struct {
		Data       []string `json:"data"`
		NextCursor string   `json:"nextCursor"`
		RequestID  string   `json:"requestId"`
	}
	json.NewDecoder(w.Body).Decode(&body)

	if body.RequestID != "req-789" {
		t.Fatal("requestId mismatch")
	}
	if len(body.Data) != 2 {
		t.Fatal("expected 2 items")
	}
	if body.NextCursor != "cursor-next" {
		t.Fatal("cursor mismatch")
	}
}

func TestListResponse_NoCursor(t *testing.T) {
	w := httptest.NewRecorder()
	ListResponse(w, "req-000", []int{1}, "")

	var body struct {
		NextCursor string `json:"nextCursor"`
	}
	json.NewDecoder(w.Body).Decode(&body)
	if body.NextCursor != "" {
		t.Fatal("expected empty cursor")
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	Error(w, "req-err", http.StatusBadRequest, "VALIDATION_ERROR", "bad input", map[string]string{"title": "required"})

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var body struct {
		Error struct {
			Code    string            `json:"code"`
			Message string            `json:"message"`
			Fields  map[string]string `json:"fields"`
		} `json:"error"`
		RequestID string `json:"requestId"`
	}
	json.NewDecoder(resp.Body).Decode(&body)

	if body.Error.Code != "VALIDATION_ERROR" {
		t.Fatal("code mismatch")
	}
	if body.Error.Fields["title"] != "required" {
		t.Fatal("fields mismatch")
	}
}

func TestError_WithoutFields(t *testing.T) {
	w := httptest.NewRecorder()
	Error(w, "req-500", http.StatusInternalServerError, "INTERNAL_ERROR", "oops", nil)

	var body struct {
		Error struct {
			Code   string `json:"code"`
			Fields any    `json:"fields,omitempty"`
		} `json:"error"`
	}
	json.NewDecoder(w.Body).Decode(&body)
	if body.Error.Code != "INTERNAL_ERROR" {
		t.Fatal("code mismatch")
	}
	if body.Error.Fields != nil {
		t.Fatal("expected no fields")
	}
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	BadRequest(w, "r1", "bad", map[string]string{"f": "err"})
	if w.Code != 400 {
		t.Fatal("expected 400")
	}
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	Unauthorized(w, "r2", "unauthorized")
	if w.Code != 401 {
		t.Fatal("expected 401")
	}
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	Forbidden(w, "r3", "forbidden")
	if w.Code != 403 {
		t.Fatal("expected 403")
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	NotFound(w, "r4", "not found")
	if w.Code != 404 {
		t.Fatal("expected 404")
	}
}

func TestConflict(t *testing.T) {
	w := httptest.NewRecorder()
	Conflict(w, "r5", "conflict")
	if w.Code != 409 {
		t.Fatal("expected 409")
	}
}

func TestPayloadTooLarge(t *testing.T) {
	w := httptest.NewRecorder()
	PayloadTooLarge(w, "r6", "too large")
	if w.Code != 413 {
		t.Fatal("expected 413")
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	InternalError(w, "r7", "internal")
	if w.Code != 500 {
		t.Fatal("expected 500")
	}
}

func TestServiceUnavailable(t *testing.T) {
	w := httptest.NewRecorder()
	ServiceUnavailable(w, "r8", "unavailable")
	if w.Code != 503 {
		t.Fatal("expected 503")
	}
}

func TestContentType(t *testing.T) {
	w := httptest.NewRecorder()
	OK(w, "r", nil)
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatal("expected application/json")
	}
}
