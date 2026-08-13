package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

func TestUpdateDisplayName(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())

	rr := call(t, mux, http.MethodPatch, "/v1/auth/me", "renamer", `{"displayName":"新しい名前"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch me: got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = call(t, mux, http.MethodGet, "/v1/auth/me", "renamer", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get me: got %d body=%s", rr.Code, rr.Body.String())
	}
	body := decodeUser(t, rr)
	if body.Data.DisplayName != "新しい名前" {
		t.Fatalf("display name not updated: got %q", body.Data.DisplayName)
	}

	// blank name is rejected rather than silently accepted
	rr = call(t, mux, http.MethodPatch, "/v1/auth/me", "renamer", `{"displayName":"  "}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("blank name: got %d want 400", rr.Code)
	}
}

func TestChangeEmail(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	tokenFor(t, mux, "mover")
	tokenFor(t, mux, "other")

	// wrong current password is rejected
	rr := call(t, mux, http.MethodPost, "/v1/auth/change-email", "mover",
		`{"currentPassword":"wrong","newEmail":"new@test.local"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: got %d body=%s", rr.Code, rr.Body.String())
	}

	// an address already in use is rejected, not silently swapped in
	rr = call(t, mux, http.MethodPost, "/v1/auth/change-email", "mover",
		`{"currentPassword":"testpassword","newEmail":"other@test.local"}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("taken email: got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = call(t, mux, http.MethodPost, "/v1/auth/change-email", "mover",
		`{"currentPassword":"testpassword","newEmail":"moved@test.local"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("change email: got %d body=%s", rr.Code, rr.Body.String())
	}
	body := decodeUser(t, rr)
	if body.Data.Email != "moved@test.local" {
		t.Fatalf("email not updated: got %q", body.Data.Email)
	}

	// the account can now log in with the new address
	rr = call(t, mux, http.MethodPost, "/v1/auth/login", "",
		`{"email":"moved@test.local","password":"testpassword"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("login with new email: got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestChangePassword(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	oldToken := tokenFor(t, mux, "changer")

	rr := call(t, mux, http.MethodPost, "/v1/auth/change-password", "changer",
		`{"currentPassword":"wrong","newPassword":"brandnewpass"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong current password: got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = call(t, mux, http.MethodPost, "/v1/auth/change-password", "changer",
		`{"currentPassword":"testpassword","newPassword":"short"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("too-short new password: got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = call(t, mux, http.MethodPost, "/v1/auth/change-password", "changer",
		`{"currentPassword":"testpassword","newPassword":"brandnewpass"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("change password: got %d body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode change-password response: %v", err)
	}
	newToken := out.Data.Token
	if newToken == "" || newToken == oldToken {
		t.Fatalf("expected a fresh, different token, got %q (old %q)", newToken, oldToken)
	}

	// the old token from before the change is dead
	req := newRequest(http.MethodGet, "/v1/auth/me", "")
	req.Header.Set("Authorization", "Bearer "+oldToken)
	if rr := serve(mux, req); rr.Code != http.StatusUnauthorized {
		t.Fatalf("old token still works: got %d", rr.Code)
	}

	// the new one does, and the old password no longer signs in
	req = newRequest(http.MethodGet, "/v1/auth/me", "")
	req.Header.Set("Authorization", "Bearer "+newToken)
	if rr := serve(mux, req); rr.Code != http.StatusOK {
		t.Fatalf("new token rejected: got %d", rr.Code)
	}
	rr = call(t, mux, http.MethodPost, "/v1/auth/login", "",
		`{"email":"changer@test.local","password":"testpassword"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("old password still works: got %d", rr.Code)
	}
}

func TestDeleteAccount(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	token := tokenFor(t, mux, "leaver")

	// something owned by the account, to confirm it actually goes away
	if rr := call(t, mux, http.MethodPost, "/v1/events", "leaver",
		`{"title":"予定","startAt":"2026-09-01T10:00:00+09:00","endAt":"2026-09-01T11:00:00+09:00"}`); rr.Code != http.StatusCreated {
		t.Fatalf("seed event: got %d body=%s", rr.Code, rr.Body.String())
	}

	rr := call(t, mux, http.MethodDelete, "/v1/auth/me", "leaver", `{"currentPassword":"wrong"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = call(t, mux, http.MethodDelete, "/v1/auth/me", "leaver", `{"currentPassword":"testpassword"}`)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete account: got %d body=%s", rr.Code, rr.Body.String())
	}

	// the session that just deleted the account is itself gone
	req := newRequest(http.MethodGet, "/v1/auth/me", "")
	req.Header.Set("Authorization", "Bearer "+token)
	if rr := serve(mux, req); rr.Code != http.StatusUnauthorized {
		t.Fatalf("session survived account deletion: got %d", rr.Code)
	}

	// the email is free again
	rr = call(t, mux, http.MethodPost, "/v1/auth/register", "",
		`{"email":"leaver@test.local","password":"testpassword"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("re-register freed email: got %d body=%s", rr.Code, rr.Body.String())
	}
}

type userBody struct {
	Data struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
	} `json:"data"`
}

func decodeUser(t *testing.T, rr *httptest.ResponseRecorder) userBody {
	t.Helper()
	var out userBody
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode user response: %v\nbody=%s", err, rr.Body.String())
	}
	return out
}
