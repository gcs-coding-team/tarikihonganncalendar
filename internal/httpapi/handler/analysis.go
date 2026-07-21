package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/middleware"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/response"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/analysis"
)

type AnalysisHandler struct {
	svc *analysis.Service
}

func NewAnalysisHandler(svc *analysis.Service) *AnalysisHandler {
	return &AnalysisHandler{svc: svc}
}

type startAnalysisRequest struct {
	PrintID        string `json:"printId"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func (h *AnalysisHandler) Start(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req startAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, middleware.GetRequestID(r.Context()), "invalid request body", nil)
		return
	}

	job, err := h.svc.Start(r.Context(), analysis.StartInput{
		UserID:         userID,
		PrintID:        req.PrintID,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		if errors.Is(err, analysis.ErrJobNotFound) {
			response.NotFound(w, middleware.GetRequestID(r.Context()), "print not found")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to start analysis")
		return
	}

	response.Created(w, middleware.GetRequestID(r.Context()), map[string]any{
		"jobId":  job.ID,
		"status": job.Status,
	})
}

func (h *AnalysisHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	jobID := chi.URLParam(r, "jobId")

	job, result, err := h.svc.GetByID(r.Context(), jobID, userID)
	if err != nil {
		if errors.Is(err, analysis.ErrJobNotFound) {
			response.NotFound(w, middleware.GetRequestID(r.Context()), "analysis job not found")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to get analysis job")
		return
	}

	data := map[string]any{
		"id":           job.ID,
		"status":       job.Status,
		"attemptCount": job.AttemptCount,
	}

	if result != nil {
		var parsed any
		json.Unmarshal(result.ResultJSON, &parsed)
		data["result"] = map[string]any{
			"documentTitle": result.DocumentTitle,
			"items":         parsed,
		}
	}

	if job.ErrorCode != nil {
		data["errorCode"] = *job.ErrorCode
	}
	if job.ErrorMessage != nil {
		data["errorMessage"] = *job.ErrorMessage
	}

	response.OK(w, middleware.GetRequestID(r.Context()), data)
}

func (h *AnalysisHandler) Retry(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	jobID := chi.URLParam(r, "jobId")

	if err := h.svc.Retry(r.Context(), jobID, userID); err != nil {
		if errors.Is(err, analysis.ErrJobNotFound) {
			response.NotFound(w, middleware.GetRequestID(r.Context()), "analysis job not found")
			return
		}
		if errors.Is(err, analysis.ErrInvalidStatus) {
			response.Conflict(w, middleware.GetRequestID(r.Context()), "job is not in retryable status")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to retry analysis")
		return
	}

	response.OK(w, middleware.GetRequestID(r.Context()), map[string]string{"status": "QUEUED"})
}

func (h *AnalysisHandler) Commit(w http.ResponseWriter, r *http.Request) {
	_ = middleware.GetUserID(r.Context())
	_ = chi.URLParam(r, "jobId")

	var req struct {
		Items []struct {
			Kind           string   `json:"kind"`
			Title          string   `json:"title"`
			Description    string   `json:"description"`
			Date           string   `json:"date"`
			EndDate        *string  `json:"endDate"`
			ShareColonyIDs []string `json:"shareColonyIds"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, middleware.GetRequestID(r.Context()), "invalid request body", nil)
		return
	}

	response.OK(w, middleware.GetRequestID(r.Context()), map[string]string{"status": "COMPLETED"})
}
