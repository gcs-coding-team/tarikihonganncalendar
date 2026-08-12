package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

type taskBody struct {
	Data struct {
		ID        string  `json:"id"`
		Title     string  `json:"title"`
		Status    string  `json:"status"`
		DueAt     string  `json:"dueAt"`
		ProjectID *string `json:"projectId"`
		Version   int     `json:"version"`
	} `json:"data"`
}

func decodeTask(t *testing.T, raw []byte) taskBody {
	t.Helper()
	var out taskBody
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode task: %v (%s)", err, raw)
	}
	return out
}

func newProject(t *testing.T, mux *Handler, userID, name string) string {
	t.Helper()
	rr := call(t, mux, http.MethodPost, "/v1/projects", userID, `{"name":"`+name+`"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create project: got %d body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	return out.Data.ID
}

func TestTaskLifecycle(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	projectID := newProject(t, mux, "u1", "文化祭")

	rr := call(t, mux, http.MethodPost, "/v1/tasks", "u1",
		`{"title":"看板デザイン案","dueAt":"2026-08-20","projectId":"`+projectID+`"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: got %d body=%s", rr.Code, rr.Body.String())
	}
	task := decodeTask(t, rr.Body.Bytes())
	if task.Data.Status != repository.TaskStatusOpen {
		t.Fatalf("new task should be OPEN, got %q", task.Data.Status)
	}
	if task.Data.ProjectID == nil || *task.Data.ProjectID != projectID {
		t.Fatalf("projectId not kept: %s", rr.Body.String())
	}

	rr = call(t, mux, http.MethodPatch, "/v1/tasks/"+task.Data.ID, "u1", `{"status":"DONE","version":1}`)
	if task = decodeTask(t, rr.Body.Bytes()); task.Data.Status != repository.TaskStatusDone {
		t.Fatalf("status not updated: %s", rr.Body.String())
	}

	// A stale version must not silently overwrite.
	if rr := call(t, mux, http.MethodPatch, "/v1/tasks/"+task.Data.ID, "u1",
		`{"title":"x","version":1}`); rr.Code != http.StatusConflict {
		t.Fatalf("stale version: got %d want 409", rr.Code)
	}

	// Sending projectId: null unfiles the task.
	rr = call(t, mux, http.MethodPatch, "/v1/tasks/"+task.Data.ID, "u1",
		`{"projectId":null,"version":`+itoa(task.Data.Version)+`}`)
	if task = decodeTask(t, rr.Body.Bytes()); task.Data.ProjectID != nil {
		t.Fatalf("projectId null did not unfile: %s", rr.Body.String())
	}

	if rr := call(t, mux, http.MethodDelete, "/v1/tasks/"+task.Data.ID, "u1", ""); rr.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d", rr.Code)
	}
	if rr := call(t, mux, http.MethodGet, "/v1/tasks/"+task.Data.ID, "u1", ""); rr.Code != http.StatusNotFound {
		t.Fatalf("deleted task still readable: got %d", rr.Code)
	}
}

func TestTaskRejectsBadInput(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	for _, body := range []string{
		`{"title":""}`,
		`{"title":"x","status":"MAYBE"}`,
		`{"title":"x","dueAt":"8/20"}`,
		`{"title":"x","projectId":"does-not-exist"}`,
	} {
		if rr := call(t, mux, http.MethodPost, "/v1/tasks", "u1", body); rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d body=%s", body, rr.Code, rr.Body.String())
		}
	}
}

func TestTasksAreScopedToTheirOwner(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	rr := call(t, mux, http.MethodPost, "/v1/tasks", "u1", `{"title":"私のタスク"}`)
	task := decodeTask(t, rr.Body.Bytes())

	if rr := call(t, mux, http.MethodGet, "/v1/tasks/"+task.Data.ID, "u2", ""); rr.Code != http.StatusForbidden {
		t.Fatalf("another user could read it: got %d", rr.Code)
	}
	rr = call(t, mux, http.MethodGet, "/v1/tasks", "u2", "")
	var list struct {
		Data []taskBody `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Data) != 0 {
		t.Fatalf("another user saw tasks: %s", rr.Body.String())
	}
}

// Removing a project must not take the work filed under it.
func TestDeletingProjectUnfilesItsTasks(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	projectID := newProject(t, mux, "u1", "文化祭")

	rr := call(t, mux, http.MethodPost, "/v1/tasks", "u1",
		`{"title":"看板デザイン案","projectId":"`+projectID+`"}`)
	task := decodeTask(t, rr.Body.Bytes())

	if rr := call(t, mux, http.MethodDelete, "/v1/projects/"+projectID, "u1", ""); rr.Code != http.StatusNoContent {
		t.Fatalf("delete project: got %d", rr.Code)
	}

	rr = call(t, mux, http.MethodGet, "/v1/tasks/"+task.Data.ID, "u1", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("task went away with its project: got %d", rr.Code)
	}
	if got := decodeTask(t, rr.Body.Bytes()); got.Data.ProjectID != nil {
		t.Fatalf("task still points at the deleted project: %s", rr.Body.String())
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
