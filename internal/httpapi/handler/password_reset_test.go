package handler

import (
	"net/http"
	"testing"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

// Catches the token instead of mailing it, so the flow can be run end to end.
type captureDelivery struct{ token string }

func (d *captureDelivery) SendPasswordReset(email, token string) error {
	d.token = token
	return nil
}

func (d *captureDelivery) SendTaskReminder(email, title, dueAt string) error {
	return nil
}

func signIn(t *testing.T, mux *Handler, email, password string) int {
	t.Helper()
	return serve(mux, newRequest(http.MethodPost, "/v1/auth/login",
		`{"email":"`+email+`","password":"`+password+`"}`)).Code
}

func TestPasswordResetFlow(t *testing.T) {
	catch := &captureDelivery{}
	mux := NewHandler(repository.NewMemoryRepository(), Options{Delivery: catch})

	const email = "zav@test.local"
	if rr := serve(mux, newRequest(http.MethodPost, "/v1/auth/register",
		`{"email":"`+email+`","password":"oldpassword"}`)); rr.Code != http.StatusOK {
		t.Fatalf("register: got %d body=%s", rr.Code, rr.Body.String())
	}
	// A live session, to prove the reset ends it.
	before := serve(mux, newRequest(http.MethodPost, "/v1/auth/login",
		`{"email":"`+email+`","password":"oldpassword"}`))
	var loginOut struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	decodeInto(t, before.Body.Bytes(), &loginOut)

	if rr := serve(mux, newRequest(http.MethodPost, "/v1/auth/password-reset",
		`{"email":"`+email+`"}`)); rr.Code != http.StatusNoContent {
		t.Fatalf("request reset: got %d", rr.Code)
	}
	if catch.token == "" {
		t.Fatal("no reset token was issued")
	}

	// Too short a password is refused, and the token survives to be used again.
	if rr := serve(mux, newRequest(http.MethodPost, "/v1/auth/password-reset/confirm",
		`{"token":"`+catch.token+`","password":"short"}`)); rr.Code != http.StatusBadRequest {
		t.Fatalf("short password: got %d", rr.Code)
	}

	if rr := serve(mux, newRequest(http.MethodPost, "/v1/auth/password-reset/confirm",
		`{"token":"`+catch.token+`","password":"newpassword"}`)); rr.Code != http.StatusNoContent {
		t.Fatalf("confirm: got %d body=%s", rr.Code, rr.Body.String())
	}

	if code := signIn(t, mux, email, "oldpassword"); code != http.StatusUnauthorized {
		t.Fatalf("old password still works: got %d", code)
	}
	if code := signIn(t, mux, email, "newpassword"); code != http.StatusOK {
		t.Fatalf("new password rejected: got %d", code)
	}

	// The session that existed before the reset must be gone: someone resetting
	// a password may be locking another person out.
	req := newRequest(http.MethodGet, "/v1/auth/me", "")
	req.Header.Set("Authorization", "Bearer "+loginOut.Data.Token)
	if rr := serve(mux, req); rr.Code != http.StatusUnauthorized {
		t.Fatalf("old session survived the reset: got %d", rr.Code)
	}
}

// One link, one use.
func TestResetTokenCannotBeReused(t *testing.T) {
	catch := &captureDelivery{}
	mux := NewHandler(repository.NewMemoryRepository(), Options{Delivery: catch})
	serve(mux, newRequest(http.MethodPost, "/v1/auth/register",
		`{"email":"a@test.local","password":"oldpassword"}`))
	serve(mux, newRequest(http.MethodPost, "/v1/auth/password-reset", `{"email":"a@test.local"}`))

	first := serve(mux, newRequest(http.MethodPost, "/v1/auth/password-reset/confirm",
		`{"token":"`+catch.token+`","password":"newpassword"}`))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first use: got %d", first.Code)
	}
	second := serve(mux, newRequest(http.MethodPost, "/v1/auth/password-reset/confirm",
		`{"token":"`+catch.token+`","password":"anotherpassword"}`))
	if second.Code != http.StatusBadRequest {
		t.Fatalf("token was reusable: got %d", second.Code)
	}
	if code := signIn(t, mux, "a@test.local", "anotherpassword"); code != http.StatusUnauthorized {
		t.Fatal("the second reset took effect anyway")
	}
}

// An unknown address must look exactly like a known one, or the endpoint
// becomes a way to find out who has an account.
func TestResetDoesNotRevealWhoHasAnAccount(t *testing.T) {
	catch := &captureDelivery{}
	mux := NewHandler(repository.NewMemoryRepository(), Options{Delivery: catch})

	rr := serve(mux, newRequest(http.MethodPost, "/v1/auth/password-reset",
		`{"email":"nobody@test.local"}`))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("unknown address: got %d, want the same 204 as a known one", rr.Code)
	}
	if catch.token != "" {
		t.Fatal("a token was issued for an address with no account")
	}
}

func TestGarbageResetTokenIsRefused(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	if rr := serve(mux, newRequest(http.MethodPost, "/v1/auth/password-reset/confirm",
		`{"token":"made-up","password":"newpassword"}`)); rr.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400", rr.Code)
	}
}
