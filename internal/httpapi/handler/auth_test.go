package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/middleware"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/auth"
)

type mockUserRepoForHandler struct {
	users  map[string]*domain.User
	emails map[string]*domain.User
}

func (m *mockUserRepoForHandler) Create(_ context.Context, u *domain.User) error {
	if m.users == nil {
		m.users = make(map[string]*domain.User)
		m.emails = make(map[string]*domain.User)
	}
	m.users[u.ID] = u
	m.emails[u.Email] = u
	return nil
}
func (m *mockUserRepoForHandler) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	return m.emails[email], nil
}
func (m *mockUserRepoForHandler) FindByID(_ context.Context, id string) (*domain.User, error) {
	return m.users[id], nil
}

type mockSessionRepoForHandler struct {
	sessions map[string]*domain.Session
	tokens   map[string]*domain.Session
}

func (m *mockSessionRepoForHandler) Create(_ context.Context, s *domain.Session) error {
	if m.sessions == nil {
		m.sessions = make(map[string]*domain.Session)
		m.tokens = make(map[string]*domain.Session)
	}
	m.sessions[s.ID] = s
	m.tokens[string(s.TokenHash)] = s
	return nil
}
func (m *mockSessionRepoForHandler) FindByTokenHash(_ context.Context, h []byte) (*domain.Session, error) {
	return m.tokens[string(h)], nil
}
func (m *mockSessionRepoForHandler) DeleteByUserID(_ context.Context, _ string) error { return nil }
func (m *mockSessionRepoForHandler) DeleteByID(_ context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func newTestAuthHandler() *AuthHandler {
	return NewAuthHandler(auth.NewService(
		&mockUserRepoForHandler{users: map[string]*domain.User{}, emails: map[string]*domain.User{}},
		&mockSessionRepoForHandler{sessions: map[string]*domain.Session{}, tokens: map[string]*domain.Session{}},
	))
}

func TestAuthHandler_Register_Success(t *testing.T) {
	h := newTestAuthHandler()

	body := map[string]string{
		"email":       "new@example.com",
		"password":    "password123",
		"displayName": "New User",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")

	h.Register(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "session" {
			found = true
			if !c.HttpOnly {
				t.Fatal("expected HttpOnly cookie")
			}
			if c.MaxAge != 720*3600 {
				t.Fatal("expected 720h max-age")
			}
		}
	}
	if !found {
		t.Fatal("expected session cookie")
	}
}

func TestAuthHandler_Register_InvalidBody(t *testing.T) {
	h := newTestAuthHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewReader([]byte(`{invalid`)))
	r.Header.Set("Content-Type", "application/json")

	h.Register(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	h := newTestAuthHandler()

	regBody := map[string]string{
		"email":       "login@example.com",
		"password":    "password123",
		"displayName": "Login User",
	}
	b, _ := json.Marshal(regBody)
	h.Register(httptest.NewRecorder(), httptest.NewRequest("POST", "/", bytes.NewReader(b)))

	loginBody := map[string]string{
		"email":    "login@example.com",
		"password": "password123",
	}
	b, _ = json.Marshal(loginBody)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")

	h.Login(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthHandler_Logout_Success(t *testing.T) {
	h := newTestAuthHandler()

	regBody := map[string]string{
		"email":       "logout@example.com",
		"password":    "password123",
		"displayName": "Logout User",
	}
	b, _ := json.Marshal(regBody)
	wReg := httptest.NewRecorder()
	h.Register(wReg, httptest.NewRequest("POST", "/", bytes.NewReader(b)))
	cookies := wReg.Result().Cookies()
	var token string
	for _, c := range cookies {
		if c.Name == "session" {
			token = c.Value
			break
		}
	}
	if token == "" {
		t.Fatal("expected session cookie from register")
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/auth/logout", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: token})

	h.Logout(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	respCookies := w.Result().Cookies()
	for _, c := range respCookies {
		if c.Name == "session" && c.MaxAge != -1 {
			t.Fatal("expected session cookie to be cleared")
		}
	}
}

func TestAuthHandler_Logout_NoCookie(t *testing.T) {
	h := newTestAuthHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/auth/logout", nil)

	h.Logout(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthHandler_Me_RequiresAuth(t *testing.T) {
	h := newTestAuthHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/auth/me", nil)

	// No session cookie - Me handler itself checks userID
	h.Me(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthHandler_Me_Success(t *testing.T) {
	h := newTestAuthHandler()

	regBody := map[string]string{
		"email":       "me@example.com",
		"password":    "pass",
		"displayName": "Me User",
	}
	b, _ := json.Marshal(regBody)
	wReg := httptest.NewRecorder()
	h.Register(wReg, httptest.NewRequest("POST", "/", bytes.NewReader(b)))

	var regResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.NewDecoder(wReg.Body).Decode(&regResp)
	if regResp.Data.ID == "" {
		t.Fatal("expected user ID from register")
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/auth/me", nil)
	ctx := context.WithValue(r.Context(), middleware.CtxKeyUserID, regResp.Data.ID)
	r = r.WithContext(ctx)

	h.Me(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			ID          string `json:"id"`
			Email       string `json:"email"`
			DisplayName string `json:"displayName"`
		} `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Data.ID != regResp.Data.ID {
		t.Fatal("user ID mismatch")
	}
	if resp.Data.Email != "me@example.com" {
		t.Fatal("email mismatch")
	}
	if resp.Data.DisplayName != "Me User" {
		t.Fatal("display name mismatch")
	}
}
