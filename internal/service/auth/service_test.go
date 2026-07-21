package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
)

type mockUserRepo struct {
	users  map[string]*domain.User
	emails map[string]*domain.User
}

func (m *mockUserRepo) Create(_ context.Context, user *domain.User) error {
	m.users[user.ID] = user
	m.emails[user.Email] = user
	return nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	return m.emails[email], nil
}

func (m *mockUserRepo) FindByID(_ context.Context, id string) (*domain.User, error) {
	return m.users[id], nil
}

type mockSessionRepo struct {
	sessions map[string]*domain.Session
	tokens   map[string]*domain.Session
}

func (m *mockSessionRepo) Create(_ context.Context, s *domain.Session) error {
	m.sessions[s.ID] = s
	m.tokens[string(s.TokenHash)] = s
	return nil
}

func (m *mockSessionRepo) FindByTokenHash(_ context.Context, h []byte) (*domain.Session, error) {
	return m.tokens[string(h)], nil
}

func (m *mockSessionRepo) DeleteByUserID(_ context.Context, _ string) error {
	return nil
}

func (m *mockSessionRepo) DeleteByID(_ context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func newMockAuth() *Service {
	return NewService(
		&mockUserRepo{users: map[string]*domain.User{}, emails: map[string]*domain.User{}},
		&mockSessionRepo{sessions: map[string]*domain.Session{}, tokens: map[string]*domain.Session{}},
	)
}

func TestRegister_Success(t *testing.T) {
	svc := newMockAuth()
	result, err := svc.Register(context.Background(), RegisterInput{
		Email:       "Test@Example.com",
		Password:    "password123",
		DisplayName: "Test User",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.User.ID == "" {
		t.Fatal("expected non-empty user ID")
	}
	if result.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if result.Session == nil {
		t.Fatal("expected session")
	}
}

func TestRegister_EmailNormalized(t *testing.T) {
	svc := newMockAuth()
	_, err := svc.Register(context.Background(), RegisterInput{
		Email:       "Test@Example.com",
		Password:    "password123",
		DisplayName: "T",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Register(context.Background(), RegisterInput{
		Email:       "test@example.com",
		Password:    "password123",
		DisplayName: "T",
	})
	if err != ErrEmailAlreadyRegistered {
		t.Fatal("expected ErrEmailAlreadyRegistered for duplicate normalized email")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc := newMockAuth()
	_, err := svc.Register(context.Background(), RegisterInput{
		Email:       "dup@example.com",
		Password:    "password123",
		DisplayName: "U1",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Register(context.Background(), RegisterInput{
		Email:       "dup@example.com",
		Password:    "password456",
		DisplayName: "U2",
	})
	if err != ErrEmailAlreadyRegistered {
		t.Fatal("expected ErrEmailAlreadyRegistered")
	}
}

func TestLogin_Success(t *testing.T) {
	svc := newMockAuth()
	_, err := svc.Register(context.Background(), RegisterInput{
		Email:       "login@example.com",
		Password:    "mypassword",
		DisplayName: "Login User",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := svc.Login(context.Background(), LoginInput{
		Email:    "login@example.com",
		Password: "mypassword",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Token == "" {
		t.Fatal("expected session token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc := newMockAuth()
	svc.Register(context.Background(), RegisterInput{
		Email:       "wrongpw@example.com",
		Password:    "correctpw",
		DisplayName: "U",
	})

	_, err := svc.Login(context.Background(), LoginInput{
		Email:    "wrongpw@example.com",
		Password: "wrongpw",
	})
	if err != ErrInvalidCredentials {
		t.Fatal("expected ErrInvalidCredentials for wrong password")
	}
}

func TestLogin_NonExistentEmail(t *testing.T) {
	svc := newMockAuth()
	_, err := svc.Login(context.Background(), LoginInput{
		Email:    "noone@example.com",
		Password: "password",
	})
	if err != ErrInvalidCredentials {
		t.Fatal("expected ErrInvalidCredentials for non-existent email")
	}
}

func TestLogout_Success(t *testing.T) {
	svc := newMockAuth()
	result, err := svc.Register(context.Background(), RegisterInput{
		Email:       "logout@example.com",
		Password:    "password",
		DisplayName: "U",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = svc.Logout(context.Background(), result.Token)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLogout_InvalidToken(t *testing.T) {
	svc := newMockAuth()
	err := svc.Logout(context.Background(), "invalid-token")
	if err != ErrSessionNotFound {
		t.Fatal("expected ErrSessionNotFound")
	}
}

func TestGetUser(t *testing.T) {
	svc := newMockAuth()
	result, err := svc.Register(context.Background(), RegisterInput{
		Email:       "getuser@example.com",
		Password:    "password",
		DisplayName: "Get User",
	})
	if err != nil {
		t.Fatal(err)
	}

	user, err := svc.GetUser(context.Background(), result.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("expected user")
	}
	if user.Email != "getuser@example.com" {
		t.Fatalf("expected getuser@example.com, got %s", user.Email)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	svc := newMockAuth()
	user, err := svc.GetUser(context.Background(), "non-existent-id")
	if err != nil {
		t.Fatal(err)
	}
	if user != nil {
		t.Fatal("expected nil user")
	}
}

func TestPasswordVerification(t *testing.T) {
	svc := newMockAuth()
	result, err := svc.Register(context.Background(), RegisterInput{
		Email:       "pwtest@example.com",
		Password:    "complex-password-123!@#",
		DisplayName: "U",
	})
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.SplitN(result.User.PasswordHash, ":", 2)
	if len(parts) != 2 {
		t.Fatal("expected salt:hash format")
	}
	if len(parts[0]) != 32 {
		t.Fatalf("expected 32 hex chars for salt, got %d", len(parts[0]))
	}
	if len(parts[1]) != 64 {
		t.Fatalf("expected 64 hex chars for hash, got %d", len(parts[1]))
	}
}

func TestPasswordVerifyCorrect(t *testing.T) {
	svc := newMockAuth()
	result, _ := svc.Register(context.Background(), RegisterInput{
		Email: "verify@example.com", Password: "correct123", DisplayName: "U",
	})

	if !verifyPassword(result.User.PasswordHash, "correct123") {
		t.Fatal("expected password verification to succeed")
	}
}

func TestPasswordVerifyWrong(t *testing.T) {
	svc := newMockAuth()
	result, _ := svc.Register(context.Background(), RegisterInput{
		Email: "verify2@example.com", Password: "correct123", DisplayName: "U",
	})

	if verifyPassword(result.User.PasswordHash, "wrong") {
		t.Fatal("expected password verification to fail")
	}
}

func TestPasswordVerifyInvalidHash(t *testing.T) {
	if verifyPassword("invalid", "pwd") {
		t.Fatal("expected false for invalid hash format")
	}
	if verifyPassword("invalid:hash:extra", "pwd") {
		t.Fatal("expected false for extra parts")
	}
	if verifyPassword("nothex:nothex", "pwd") {
		t.Fatal("expected false for non-hex values")
	}
}
