package handler

import (
	"encoding/json"
	"net/http"

	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/middleware"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/response"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/upload"
)

type UploadHandler struct {
	svc *upload.Service
}

func NewUploadHandler(svc *upload.Service) *UploadHandler {
	return &UploadHandler{svc: svc}
}

type createUploadRequest struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

func (h *UploadHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req createUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, middleware.GetRequestID(r.Context()), "invalid request body", nil)
		return
	}

	result, err := h.svc.CreateUpload(r.Context(), upload.CreateUploadInput{
		UserID:      userID,
		FileName:    req.FileName,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
	})
	if err != nil {
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to create upload")
		return
	}

	response.Created(w, middleware.GetRequestID(r.Context()), map[string]any{
		"printId":   result.Print.ID,
		"uploadUrl": result.UploadURL,
		"expiresAt": result.ExpiresAt,
	})
}
