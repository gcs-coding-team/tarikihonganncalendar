package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/middleware"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/response"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/print"
)

type PrintHandler struct {
	svc *print.Service
}

func NewPrintHandler(svc *print.Service) *PrintHandler {
	return &PrintHandler{svc: svc}
}

func (h *PrintHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	printID := chi.URLParam(r, "printId")

	result, err := h.svc.GetByID(r.Context(), printID, userID)
	if err != nil {
		if errors.Is(err, print.ErrNotFound) {
			response.NotFound(w, middleware.GetRequestID(r.Context()), "print not found")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to get print")
		return
	}

	response.OK(w, middleware.GetRequestID(r.Context()), toPrintResponse(result))
}

func (h *PrintHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	prints, err := h.svc.ListByUserID(r.Context(), userID)
	if err != nil {
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to list prints")
		return
	}

	data := make([]any, 0, len(prints))
	for _, p := range prints {
		data = append(data, toPrintResponse(p))
	}

	response.ListResponse(w, middleware.GetRequestID(r.Context()), data, "")
}

func (h *PrintHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	printID := chi.URLParam(r, "printId")

	if err := h.svc.Delete(r.Context(), printID, userID); err != nil {
		if errors.Is(err, print.ErrNotFound) {
			response.NotFound(w, middleware.GetRequestID(r.Context()), "print not found")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to delete print")
		return
	}

	response.NoContent(w)
}

func toPrintResponse(p *domain.Print) map[string]any {
	return map[string]any{
		"id":               p.ID,
		"originalFileName": p.OriginalFileName,
		"contentType":      p.ContentType,
		"sizeBytes":        p.SizeBytes,
		"uploadStatus":     p.UploadStatus,
		"createdAt":        p.CreatedAt,
	}
}
