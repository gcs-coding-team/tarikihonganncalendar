package response

import (
	"encoding/json"
	"net/http"
)

type Success struct {
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
}

type List struct {
	Data       any    `json:"data"`
	NextCursor string `json:"nextCursor"`
	RequestID  string `json:"requestId"`
}

type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type ErrorResponse struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"requestId"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func OK(w http.ResponseWriter, requestID string, data any) {
	writeJSON(w, http.StatusOK, Success{Data: data, RequestID: requestID})
}

func Created(w http.ResponseWriter, requestID string, data any) {
	writeJSON(w, http.StatusCreated, Success{Data: data, RequestID: requestID})
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func ListResponse(w http.ResponseWriter, requestID string, data any, nextCursor string) {
	var nc *string
	if nextCursor != "" {
		nc = &nextCursor
	}
	writeJSON(w, http.StatusOK, List{Data: data, NextCursor: *nc, RequestID: requestID})
}

func Error(w http.ResponseWriter, requestID string, httpStatus int, code, message string, fields map[string]string) {
	writeJSON(w, httpStatus, ErrorResponse{
		Error:     ErrorBody{Code: code, Message: message, Fields: fields},
		RequestID: requestID,
	})
}
