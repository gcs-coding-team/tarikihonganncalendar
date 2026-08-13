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

	// Asking for a reset always answers 204, registered or not — the response
	// must not reveal whether an address has an account here.
	h.mux.HandleFunc("/v1/auth/password-reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
			return
		}
		var in struct {
			Email string `json:"email"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if err := svc.RequestPasswordReset(in.Email, h.delivery); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorBody("INTERNAL_ERROR"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	h.mux.HandleFunc("/v1/auth/password-reset/confirm", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
			return
		}
		var in struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
			return
		}
		switch err := svc.ResetPassword(in.Token, in.Password); {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, service.ErrInvalidResetToken):
			writeJSON(w, http.StatusBadRequest, errorBody("INVALID_TOKEN"))
		case repository.IsValidationError(err):
			writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
		default:
			writeJSON(w, http.StatusInternalServerError, errorBody("INTERNAL_ERROR"))
		}
	})

	h.mux.HandleFunc("/v1/auth/me", h.withAuth(func(w http.ResponseWriter, r *http.Request) {
		userID := h.resolveUserID(r)
		switch r.Method {
		case http.MethodGet:
			user, err := h.repo.GetUserByID(userID)
			if err != nil {
				writeJSON(w, http.StatusNotFound, errorBody("NOT_FOUND"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": serializeUser(user)})

		case http.MethodPatch:
			var in struct {
				DisplayName string `json:"displayName"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
				return
			}
			user, err := svc.UpdateDisplayName(userID, in.DisplayName)
			if err != nil {
				writeRepoError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": serializeUser(user)})

		case http.MethodDelete:
			var in struct {
				CurrentPassword string `json:"currentPassword"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if err := svc.DeleteAccount(userID, in.CurrentPassword); err != nil {
				if errors.Is(err, service.ErrInvalidCredentials) {
					writeJSON(w, http.StatusUnauthorized, errorBody("UNAUTHORIZED"))
					return
				}
				writeRepoError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
		}
	}))

	h.mux.HandleFunc("/v1/auth/change-password", h.withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
			return
		}
		var in struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
			return
		}
		session, err := svc.ChangePassword(h.resolveUserID(r), in.CurrentPassword, in.NewPassword)
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"token": session.Token}})
		case errors.Is(err, service.ErrInvalidCredentials):
			writeJSON(w, http.StatusUnauthorized, errorBody("UNAUTHORIZED"))
		case repository.IsValidationError(err):
			writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
		default:
			writeJSON(w, http.StatusInternalServerError, errorBody("INTERNAL_ERROR"))
		}
	}))

	h.mux.HandleFunc("/v1/auth/change-email", h.withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorBody("METHOD_NOT_ALLOWED"))
			return
		}
		var in struct {
			CurrentPassword string `json:"currentPassword"`
			NewEmail        string `json:"newEmail"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
			return
		}
		user, err := svc.ChangeEmail(h.resolveUserID(r), in.CurrentPassword, in.NewEmail)
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, map[string]any{"data": serializeUser(user)})
		case errors.Is(err, service.ErrInvalidCredentials):
			writeJSON(w, http.StatusUnauthorized, errorBody("UNAUTHORIZED"))
		case errors.Is(err, service.ErrEmailTaken):
			writeJSON(w, http.StatusConflict, errorBody("CONFLICT"))
		case repository.IsValidationError(err):
			writeJSON(w, http.StatusBadRequest, errorBody("VALIDATION_ERROR"))
		default:
			writeJSON(w, http.StatusInternalServerError, errorBody("INTERNAL_ERROR"))
		}
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
	// No password hash, obviously. The email is fine here — this only ever
	// serializes the caller's own account (register/login/me), never someone
	// else's; a colony's member list is built separately and only carries a
	// display name.
	return map[string]any{"id": u.ID, "displayName": u.DisplayName, "email": u.Email}
}

func bearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}
