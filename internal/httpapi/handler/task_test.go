package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/middleware"
	"github.com/zatunohito/tarikihonganncalendar/internal/repository"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/task"
)

func withChiURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

type mockTaskRepo struct {
	tasks map[string]*domain.Task
}

func (m *mockTaskRepo) Create(_ context.Context, t *domain.Task) error {
	m.tasks[t.ID] = t
	return nil
}

func (m *mockTaskRepo) FindByID(_ context.Context, id string) (*domain.Task, error) {
	return m.tasks[id], nil
}

func (m *mockTaskRepo) FindByUserID(_ context.Context, userID string, filter repository.ListTasksFilter) ([]*domain.Task, string, error) {
	var result []*domain.Task
	for _, t := range m.tasks {
		if t.UserID == userID {
			result = append(result, t)
		}
	}
	return result, "", nil
}

func (m *mockTaskRepo) Update(_ context.Context, t *domain.Task) error {
	m.tasks[t.ID] = t
	return nil
}

func (m *mockTaskRepo) Delete(_ context.Context, id string) error {
	delete(m.tasks, id)
	return nil
}

func newTestTaskHandler() *TaskHandler {
	return NewTaskHandler(task.NewService(&mockTaskRepo{tasks: map[string]*domain.Task{}}))
}

func authenticatedRequest(method, path string, body []byte) *http.Request {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(r.Context(), middleware.CtxKeyUserID, "user-1")
	ctx = context.WithValue(ctx, middleware.CtxKeyRequestID, "req-1")
	return r.WithContext(ctx)
}

func TestTaskHandler_Create_Success(t *testing.T) {
	h := newTestTaskHandler()

	body := map[string]any{
		"title":       "Test Task",
		"description": "Test description",
		"dueAt":       "2026-07-30T10:00:00Z",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	h.Create(w, authenticatedRequest("POST", "/v1/tasks", b))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Status  string `json:"status"`
			Version int    `json:"version"`
		} `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Data.Title != "Test Task" {
		t.Fatal("title mismatch")
	}
	if resp.Data.Status != "OPEN" {
		t.Fatal("expected OPEN status")
	}
	if resp.Data.Version != 1 {
		t.Fatal("expected version 1")
	}
}

func TestTaskHandler_Create_InvalidBody(t *testing.T) {
	h := newTestTaskHandler()

	w := httptest.NewRecorder()
	h.Create(w, authenticatedRequest("POST", "/v1/tasks", []byte(`{invalid`)))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTaskHandler_Get_Success(t *testing.T) {
	h := newTestTaskHandler()

	created := createTestTask(t, h)
	w := httptest.NewRecorder()
	h.Get(w, withChiURLParam(authenticatedRequest("GET", "/v1/tasks/"+created.Data.ID, nil), "taskId", created.Data.ID))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_Get_NotFound(t *testing.T) {
	h := newTestTaskHandler()

	w := httptest.NewRecorder()
	h.Get(w, authenticatedRequest("GET", "/v1/tasks/non-existent", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTaskHandler_List(t *testing.T) {
	h := newTestTaskHandler()
	createTestTask(t, h)
	createTestTask(t, h)

	w := httptest.NewRecorder()
	h.List(w, authenticatedRequest("GET", "/v1/tasks", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data       []any  `json:"data"`
		NextCursor string `json:"nextCursor"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(resp.Data))
	}
}

func TestTaskHandler_Update_Success(t *testing.T) {
	h := newTestTaskHandler()
	created := createTestTask(t, h)

	body := map[string]any{
		"title":       "Updated",
		"description": "Updated desc",
		"status":      "DONE",
		"version":     1,
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := withChiURLParam(authenticatedRequest("PATCH", "/v1/tasks/"+created.Data.ID, b), "taskId", created.Data.ID)
	h.Update(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_Update_VersionConflict(t *testing.T) {
	h := newTestTaskHandler()
	created := createTestTask(t, h)

	body := map[string]any{
		"title":   "Conflict",
		"status":  "DONE",
		"version": 999,
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := withChiURLParam(authenticatedRequest("PATCH", "/v1/tasks/"+created.Data.ID, b), "taskId", created.Data.ID)
	h.Update(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_Delete_Success(t *testing.T) {
	h := newTestTaskHandler()
	created := createTestTask(t, h)

	w := httptest.NewRecorder()
	r := withChiURLParam(authenticatedRequest("DELETE", "/v1/tasks/"+created.Data.ID, nil), "taskId", created.Data.ID)
	h.Delete(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestTaskHandler_Delete_NotFound(t *testing.T) {
	h := newTestTaskHandler()

	w := httptest.NewRecorder()
	r := withChiURLParam(authenticatedRequest("DELETE", "/v1/tasks/non-existent", nil), "taskId", "non-existent")
	h.Delete(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

type taskResp struct {
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

func createTestTask(t *testing.T, h *TaskHandler) taskResp {
	t.Helper()
	body := map[string]string{"title": "Test Task", "description": ""}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	h.Create(w, authenticatedRequest("POST", "/v1/tasks", b))

	var resp taskResp
	json.NewDecoder(w.Body).Decode(&resp)
	return resp
}
