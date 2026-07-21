package response

import (
	"net/http"
)

const (
	CodeValidationError     = "VALIDATION_ERROR"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeConflict            = "CONFLICT"
	CodePayloadTooLarge     = "PAYLOAD_TOO_LARGE"
	CodeUnsupportedMedia    = "UNSUPPORTED_MEDIA_TYPE"
	CodeInternalError       = "INTERNAL_ERROR"
	CodeAIProcessingFailed  = "AI_PROCESSING_FAILED"
	CodeServiceUnavailable  = "SERVICE_UNAVAILABLE"
)

func BadRequest(w http.ResponseWriter, requestID string, message string, fields map[string]string) {
	Error(w, requestID, http.StatusBadRequest, CodeValidationError, message, fields)
}

func Unauthorized(w http.ResponseWriter, requestID string, message string) {
	Error(w, requestID, http.StatusUnauthorized, CodeUnauthorized, message, nil)
}

func Forbidden(w http.ResponseWriter, requestID string, message string) {
	Error(w, requestID, http.StatusForbidden, CodeForbidden, message, nil)
}

func NotFound(w http.ResponseWriter, requestID string, message string) {
	Error(w, requestID, http.StatusNotFound, CodeNotFound, message, nil)
}

func Conflict(w http.ResponseWriter, requestID string, message string) {
	Error(w, requestID, http.StatusConflict, CodeConflict, message, nil)
}

func PayloadTooLarge(w http.ResponseWriter, requestID string, message string) {
	Error(w, requestID, http.StatusRequestEntityTooLarge, CodePayloadTooLarge, message, nil)
}

func InternalError(w http.ResponseWriter, requestID string, message string) {
	Error(w, requestID, http.StatusInternalServerError, CodeInternalError, message, nil)
}

func ServiceUnavailable(w http.ResponseWriter, requestID string, message string) {
	Error(w, requestID, http.StatusServiceUnavailable, CodeServiceUnavailable, message, nil)
}
