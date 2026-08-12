package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/service"
)

// registerTaskRoutes wires /v1/tasks and /v1/projects onto the mux, matching
// the shape the other collections already use.
func (h *Handler) registerTaskRoutes(taskSvc *service.TaskService, projectSvc *service.ProjectService) {
	authWrap := h.withAuth

	h.mux.HandleFunc("/v1/tasks", authWrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.listTasks(w, r, taskSvc)
		case http.MethodPost:
			h.createTask(w, r, taskSvc)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
		}
	}))

	h.mux.HandleFunc("/v1/tasks/", authWrap(func(w http.ResponseWriter, r *http.Request) {
		taskID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/tasks/"), "/")
		if taskID == "" {
			writeJSON(w, http.StatusNotFound, errorBody("NOT_FOUND"))
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.getTask(w, r, taskSvc, taskID)
		case http.MethodPatch:
			h.updateTask(w, r, taskSvc, taskID)
		case http.MethodDelete:
			h.deleteTask(w, r, taskSvc, taskID)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
		}
	}))

	h.mux.HandleFunc("/v1/projects", authWrap(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.listProjects(w, r, projectSvc)
		case http.MethodPost:
			h.createProject(w, r, projectSvc)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
		}
	}))

	h.mux.HandleFunc("/v1/projects/", authWrap(func(w http.ResponseWriter, r *http.Request) {
		projectID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/projects/"), "/")
		if projectID == "" {
			writeJSON(w, http.StatusNotFound, errorBody("NOT_FOUND"))
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.getProject(w, r, projectSvc, projectID)
		case http.MethodPatch:
			h.updateProject(w, r, projectSvc, projectID)
		case http.MethodDelete:
			h.deleteProject(w, r, projectSvc, projectID)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
		}
	}))
}

func errorBody(code string) map[string]any {
	return map[string]any{"error": map[string]any{"code": code}}
}

// writeRepoError maps a repository error onto the status the API promises.
func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case repository.IsValidationError(err):
		writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
	case err == repository.ErrForbidden:
		writeJSON(w, http.StatusForbidden, errorBody("FORBIDDEN"))
	case err == repository.ErrConflict:
		writeJSON(w, http.StatusConflict, errorBody("CONFLICT"))
	case err == repository.ErrDuplicate:
		writeJSON(w, http.StatusConflict, errorBody("CONFLICT"))
	default:
		writeJSON(w, http.StatusNotFound, errorBody("NOT_FOUND"))
	}
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request, svc *service.TaskService) {
	items, err := svc.List(h.resolveUserID(r))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeTasks(items)})
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request, svc *service.TaskService) {
	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		DueAt       string `json:"dueAt"`
		Status      string `json:"status"`
		ProjectID   string `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
		return
	}
	item, err := svc.Create(h.resolveUserID(r), service.CreateTaskInput{
		Title: input.Title, Description: input.Description,
		DueAt: input.DueAt, Status: input.Status, ProjectID: input.ProjectID,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": serializeTask(item)})
}

func (h *Handler) getTask(w http.ResponseWriter, r *http.Request, svc *service.TaskService, taskID string) {
	item, err := svc.Get(h.resolveUserID(r), taskID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeTask(item)})
}

func (h *Handler) updateTask(w http.ResponseWriter, r *http.Request, svc *service.TaskService, taskID string) {
	var input struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		DueAt       *string `json:"dueAt"`
		Status      *string `json:"status"`
		// Raw, so an absent projectId (leave it filed) stays distinct from an
		// explicit null (unfile it).
		ProjectID json.RawMessage `json:"projectId"`
		Version   int             `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
		return
	}
	var projectID **string
	if len(input.ProjectID) > 0 {
		var parsed *string
		if err := json.Unmarshal(input.ProjectID, &parsed); err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
			return
		}
		projectID = &parsed
	}
	item, err := svc.Update(h.resolveUserID(r), taskID, service.UpdateTaskInput{
		Title: input.Title, Description: input.Description, DueAt: input.DueAt,
		Status: input.Status, ProjectID: projectID, Version: input.Version,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeTask(item)})
}

func (h *Handler) deleteTask(w http.ResponseWriter, r *http.Request, svc *service.TaskService, taskID string) {
	if err := svc.Delete(h.resolveUserID(r), taskID); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request, svc *service.ProjectService) {
	items, err := svc.List(h.resolveUserID(r))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, serializeProject(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request, svc *service.ProjectService) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
		return
	}
	item, err := svc.Create(h.resolveUserID(r), service.CreateProjectInput{Name: input.Name, Description: input.Description})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": serializeProject(item)})
}

func (h *Handler) getProject(w http.ResponseWriter, r *http.Request, svc *service.ProjectService, projectID string) {
	item, err := svc.Get(h.resolveUserID(r), projectID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeProject(item)})
}

func (h *Handler) updateProject(w http.ResponseWriter, r *http.Request, svc *service.ProjectService, projectID string) {
	var input struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Version     int     `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
		return
	}
	item, err := svc.Update(h.resolveUserID(r), projectID, service.UpdateProjectInput{
		Name: input.Name, Description: input.Description, Version: input.Version,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": serializeProject(item)})
}

func (h *Handler) deleteProject(w http.ResponseWriter, r *http.Request, svc *service.ProjectService, projectID string) {
	if err := svc.Delete(h.resolveUserID(r), projectID); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func serializeTask(item repository.Task) map[string]any {
	out := map[string]any{
		"id": item.ID, "title": item.Title, "description": item.Description,
		"status": item.Status, "version": item.Version,
	}
	// Both are sent even when unset, so a client can read "no deadline" and
	// "not filed under a project" rather than guessing from a missing key.
	out["dueAt"] = item.DueAt
	if item.ProjectID == "" {
		out["projectId"] = nil
	} else {
		out["projectId"] = item.ProjectID
	}
	return out
}

func serializeTasks(items []repository.Task) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, serializeTask(item))
	}
	return out
}

func serializeProject(item repository.Project) map[string]any {
	return map[string]any{
		"id": item.ID, "name": item.Name, "description": item.Description,
		"version": item.Version, "createdAt": item.CreatedAt,
	}
}
