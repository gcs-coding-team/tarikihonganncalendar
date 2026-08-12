package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/service"
)

// registerAccountRoutes wires sign-up and sign-in. These are the only endpoints
// reachable without a session, for the obvious reason.
func (h *Handler) registerAccountRoutes(svc *service.AccountService) {
	h.mux.HandleFunc("/v1/auth/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
			return
		}
		user, session, err := svc.Register(readCredentials(r))
		h.answerAuth(w, user, session, err)
	})

	h.mux.HandleFunc("/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
			return
		}
		user, session, err := svc.Login(readCredentials(r))
		h.answerAuth(w, user, session, err)
	})

	h.mux.HandleFunc("/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
			return
		}
		token := bearer(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, errorBody("UNAUTHORIZED"))
			return
		}
		_ = svc.Logout(token)
		w.WriteHeader(http.StatusNoContent)
	})

	h.mux.HandleFunc("/v1/auth/me", h.withAuth(func(w http.ResponseWriter, r *http.Request) {
		user, err := h.repo.GetUserByID(h.resolveUserID(r))
		if err != nil {
			writeJSON(w, http.StatusNotFound, errorBody("NOT_FOUND"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": serializeUser(user)})
	}))
}

func readCredentials(r *http.Request) service.Credentials {
	var in struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	return service.Credentials{Email: in.Email, Password: in.Password, DisplayName: in.DisplayName}
}

func (h *Handler) answerAuth(w http.ResponseWriter, user repository.User, session repository.Session, err error) {
	switch {
	case err == nil:
	case errors.Is(err, service.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, errorBody("UNAUTHORIZED"))
		return
	case errors.Is(err, service.ErrEmailTaken):
		writeJSON(w, http.StatusConflict, errorBody("CONFLICT"))
		return
	case repository.IsValidationError(err):
		writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
		return
	default:
		writeJSON(w, http.StatusInternalServerError, errorBody("INTERNAL_ERROR"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"token": session.Token,
		"user":  serializeUser(user),
	}})
}

func serializeUser(u repository.User) map[string]any {
	// No password hash here, obviously, and no email either: the display name
	// and id are all any screen needs.
	return map[string]any{"id": u.ID, "displayName": u.DisplayName}
}

func bearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}
