package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/service"
)

// Uploads are capped so a single request cannot exhaust memory. The frontend
// already tells people 10MB.
const maxUploadBytes = 10 << 20

// registerAnalysisRoutes wires reading a photographed handout.
//
//	POST /v1/uploads/jobs              start a job
//	GET  /v1/uploads/jobs              list them
//	GET  /v1/uploads/jobs/{id}         one job's state
//	POST /v1/uploads/jobs/{id}/analyse send the image, get candidates
//	GET  /v1/uploads/jobs/{id}/candidates
func (h *Handler) registerAnalysisRoutes(svc *service.AnalysisService) {
	h.mux.HandleFunc("/v1/uploads/jobs", h.withAuth(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var in struct {
				ContentType string `json:"contentType"`
				Filename    string `json:"filename"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			job, err := svc.Create(h.resolveUserID(r), in.ContentType, in.Filename)
			if err != nil {
				writeRepoError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"data": serializeJob(job)})
		case http.MethodGet:
			jobs, err := svc.List(h.resolveUserID(r))
			if err != nil {
				writeRepoError(w, err)
				return
			}
			out := make([]map[string]any, 0, len(jobs))
			for _, j := range jobs {
				out = append(out, serializeJob(j))
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": out})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
		}
	}))

	h.mux.HandleFunc("/v1/uploads/jobs/", h.withAuth(func(w http.ResponseWriter, r *http.Request) {
		rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/uploads/jobs/"), "/")
		parts := strings.Split(rest, "/")
		userID := h.resolveUserID(r)
		jobID := parts[0]
		if jobID == "" {
			writeJSON(w, http.StatusNotFound, errorBody("NOT_FOUND"))
			return
		}

		switch {
		case len(parts) == 1 && r.Method == http.MethodGet:
			job, err := svc.Get(userID, jobID)
			if err != nil {
				writeRepoError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": serializeJob(job)})

		case len(parts) == 2 && parts[1] == "analyse" && r.Method == http.MethodPost:
			image, err := io.ReadAll(io.LimitReader(r.Body, maxUploadBytes+1))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
				return
			}
			if len(image) > maxUploadBytes {
				writeJSON(w, http.StatusRequestEntityTooLarge, errorBody("VALIDATION_ERROR"))
				return
			}
			job, err := svc.Run(r.Context(), userID, jobID, image)
			if err != nil {
				writeRepoError(w, err)
				return
			}
			cands, err := svc.Candidates(userID, jobID)
			if err != nil {
				writeRepoError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
				"job":        serializeJob(job),
				"candidates": cands,
			}})

		case len(parts) == 2 && parts[1] == "candidates" && r.Method == http.MethodGet:
			cands, err := svc.Candidates(userID, jobID)
			if err != nil {
				writeRepoError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": cands})

		default:
			writeJSON(w, http.StatusNotFound, errorBody("NOT_FOUND"))
		}
	}))
}

func serializeJob(j repository.AnalysisJob) map[string]any {
	return map[string]any{
		"id":            j.ID,
		"userId":        j.UserID,
		"contentType":   j.ContentType,
		"filename":      j.Filename,
		"status":        j.Status,
		"resultSummary": j.ResultSummary,
		"createdAt":     j.CreatedAt,
		"updatedAt":     j.UpdatedAt,
	}
}
