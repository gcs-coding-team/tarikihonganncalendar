package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/middleware"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/response"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/auth"
)

type AuthHandler struct {
	svc *auth.Service
}

func NewAuthHandler(svc *auth.Service) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, middleware.GetRequestID(r.Context()), "invalid request body", nil)
		return
	}

	result, err := h.svc.Register(r.Context(), auth.RegisterInput{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		if errors.Is(err, auth.ErrEmailAlreadyRegistered) {
			response.Conflict(w, middleware.GetRequestID(r.Context()), "email already registered")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "registration failed")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    result.Token,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   720 * 3600,
	})

	response.Created(w, middleware.GetRequestID(r.Context()), map[string]any{
		"id":           result.User.ID,
		"email":        req.Email,
		"displayName":  req.DisplayName,
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, middleware.GetRequestID(r.Context()), "invalid request body", nil)
		return
	}

	result, err := h.svc.Login(r.Context(), auth.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			response.Unauthorized(w, middleware.GetRequestID(r.Context()), "invalid email or password")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "login failed")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    result.Token,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   720 * 3600,
	})

	response.OK(w, middleware.GetRequestID(r.Context()), map[string]any{
		"id":           result.User.ID,
	})
}
