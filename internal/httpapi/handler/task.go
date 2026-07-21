package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/middleware"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/response"
	"github.com/zatunohito/tarikihonganncalendar/internal/repository"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/task"
)

type TaskHandler struct {
	svc *task.Service
}

func NewTaskHandler(svc *task.Service) *TaskHandler {
	return &TaskHandler{svc: svc}
}

type createTaskRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueAt       *time.Time `json:"dueAt"`
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, middleware.GetRequestID(r.Context()), "invalid request body", nil)
		return
	}

	result, err := h.svc.Create(r.Context(), task.CreateInput{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		DueAt:       req.DueAt,
	})
	if err != nil {
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to create task")
		return
	}

	response.Created(w, middleware.GetRequestID(r.Context()), toTaskResponse(result))
}

func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	taskID := chi.URLParam(r, "taskId")

	result, err := h.svc.GetByID(r.Context(), taskID, userID)
	if err != nil {
		if errors.Is(err, task.ErrNotFound) {
			response.NotFound(w, middleware.GetRequestID(r.Context()), "task not found")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to get task")
		return
	}

	response.OK(w, middleware.GetRequestID(r.Context()), toTaskResponse(result))
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if r.URL.Query().Get("limit") != "" {
		if l, err := parseInt(r.URL.Query().Get("limit")); err == nil && l > 0 {
			limit = l
		}
	}

	var updatedAfter *time.Time
	if u := r.URL.Query().Get("updatedAfter"); u != "" {
		if t, err := time.Parse(time.RFC3339, u); err == nil {
			updatedAfter = &t
		}
	}

	tasks, nextCursor, err := h.svc.List(r.Context(), userID, repository.ListTasksFilter{
		Cursor:       cursor,
		Limit:        limit,
		UpdatedAfter: updatedAfter,
	})
	if err != nil {
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to list tasks")
		return
	}

	data := make([]any, 0, len(tasks))
	for _, t := range tasks {
		data = append(data, toTaskResponse(t))
	}

	response.ListResponse(w, middleware.GetRequestID(r.Context()), data, nextCursor)
}

type updateTaskRequest struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	DueAt       *time.Time        `json:"dueAt"`
	Status      domain.TaskStatus `json:"status"`
	Version     int               `json:"version"`
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	taskID := chi.URLParam(r, "taskId")

	var req updateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, middleware.GetRequestID(r.Context()), "invalid request body", nil)
		return
	}

	result, err := h.svc.Update(r.Context(), task.UpdateInput{
		ID:          taskID,
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		DueAt:       req.DueAt,
		Status:      req.Status,
		Version:     req.Version,
	})
	if err != nil {
		if errors.Is(err, task.ErrNotFound) {
			response.NotFound(w, middleware.GetRequestID(r.Context()), "task not found")
			return
		}
		if errors.Is(err, task.ErrConflict) {
			response.Conflict(w, middleware.GetRequestID(r.Context()), "version conflict")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to update task")
		return
	}

	response.OK(w, middleware.GetRequestID(r.Context()), toTaskResponse(result))
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	taskID := chi.URLParam(r, "taskId")

	if err := h.svc.Delete(r.Context(), taskID, userID); err != nil {
		if errors.Is(err, task.ErrNotFound) {
			response.NotFound(w, middleware.GetRequestID(r.Context()), "task not found")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to delete task")
		return
	}

	response.NoContent(w)
}

func toTaskResponse(t *domain.Task) map[string]any {
	return map[string]any{
		"id":          t.ID,
		"title":       t.Title,
		"description": t.Description,
		"dueAt":       t.DueAt,
		"status":      t.Status,
		"version":     t.Version,
		"createdAt":   t.CreatedAt,
		"updatedAt":   t.UpdatedAt,
	}
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
