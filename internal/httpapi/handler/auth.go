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
	svc       *auth.Service
	secure    bool
}

func NewAuthHandler(svc *auth.Service, secure bool) *AuthHandler {
	return &AuthHandler{svc: svc, secure: secure}
}

func (h *AuthHandler) sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     "session",
		Value:    value,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   maxAge,
	}
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
		if errors.Is(err, auth.ErrValidation) {
			response.BadRequest(w, middleware.GetRequestID(r.Context()), "invalid input", nil)
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "registration failed")
		return
	}

	http.SetCookie(w, h.sessionCookie(result.Token, 720*3600))

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

	http.SetCookie(w, h.sessionCookie(result.Token, 720*3600))

	response.OK(w, middleware.GetRequestID(r.Context()), map[string]any{
		"id":           result.User.ID,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		response.Unauthorized(w, middleware.GetRequestID(r.Context()), "not authenticated")
		return
	}

	if err := h.svc.Logout(r.Context(), cookie.Value); err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			response.Unauthorized(w, middleware.GetRequestID(r.Context()), "session not found")
			return
		}
		response.InternalError(w, middleware.GetRequestID(r.Context()), "logout failed")
		return
	}

	http.SetCookie(w, h.sessionCookie("", -1))

	response.NoContent(w)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		response.Unauthorized(w, middleware.GetRequestID(r.Context()), "not authenticated")
		return
	}

	user, err := h.svc.GetUser(r.Context(), userID)
	if err != nil {
		response.InternalError(w, middleware.GetRequestID(r.Context()), "failed to get user")
		return
	}
	if user == nil {
		response.NotFound(w, middleware.GetRequestID(r.Context()), "user not found")
		return
	}

	response.OK(w, middleware.GetRequestID(r.Context()), map[string]any{
		"id":          user.ID,
		"email":       user.Email,
		"displayName": user.DisplayName,
	})
}
