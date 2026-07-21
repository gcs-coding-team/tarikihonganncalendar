package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

func TestAuthAndUploadEndpoints(t *testing.T) {
	repo := repository.NewMemoryRepository()
	mux := NewHandler(repo)

	sessionReq := []byte(`{"userId":"user-1","name":"Alice"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/sessions", bytes.NewReader(sessionReq))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected auth create 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	jobReq := []byte(`{"contentType":"image/png","filename":"sample.png"}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/uploads/jobs", bytes.NewReader(jobReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected upload job create 201, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestEventsAndColoniesEndpoints(t *testing.T) {
	repo := repository.NewMemoryRepository()
	mux := NewHandler(repo)

	reqBody := []byte(`{"title":"学校行事","description":"体育館集合","startAt":"2026-07-25T09:00:00Z","endAt":"2026-07-25T12:00:00Z","allDay":false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	req.Header.Set("X-User-ID", "user-1")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("expected 1 event, got %d", len(listResp.Data))
	}

	colonyReq := []byte(`{"name":"3年1組","description":"クラス共有"}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/colonies", bytes.NewReader(colonyReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected colony create 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	var colonyResp struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&colonyResp); err != nil {
		t.Fatalf("decode colony response: %v", err)
	}
	colonyID, ok := colonyResp.Data["id"].(string)
	if !ok || colonyID == "" {
		t.Fatalf("expected colony id in response: %#v", colonyResp.Data)
	}

	sharedReq := []byte(`{"sourceType":"TASK","sourceId":"task-1"}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/colonies/"+colonyID+"/shared-items", bytes.NewReader(sharedReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected shared item create 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/colonies/"+colonyID+"/shared-items", bytes.NewReader(sharedReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected duplicate shared item 409, got %d body=%s", rr.Code, rr.Body.String())
	}
}
