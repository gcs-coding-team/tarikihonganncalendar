package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

// AccountService is the real sign-in: an email, a password, and a session token.
//
// It replaces the earlier arrangement where naming yourself was enough to become
// anyone. That mattered once colonies let people share work with each other —
// until then the only person you could impersonate was yourself.

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already registered")
)

type AccountService struct {
	repo repository.Repository
}

func NewAccountService(repo repository.Repository) *AccountService {
	return &AccountService{repo: repo}
}

type Credentials struct {
	Email       string
	Password    string
	DisplayName string
}

func (s *AccountService) Register(in Credentials) (repository.User, repository.Session, error) {
	email := normalizeEmail(in.Email)
	if email == "" || !strings.Contains(email, "@") {
		return repository.User{}, repository.Session{}, repository.ValidationError("email is required")
	}
	// Eight is not much, but a floor that people actually clear beats a rule
	// they work around with "Passw0rd!".
	if len(in.Password) < 8 {
		return repository.User{}, repository.Session{}, repository.ValidationError("password must be at least 8 characters")
	}
	if _, err := s.repo.GetUserByEmail(email); err == nil {
		return repository.User{}, repository.Session{}, ErrEmailTaken
	}
	name := strings.TrimSpace(in.DisplayName)
	if name == "" {
		name = email[:strings.Index(email, "@")]
	}
	user, err := s.repo.CreateUser(repository.User{
		Email:        email,
		PasswordHash: hashPassword(in.Password),
		DisplayName:  name,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return repository.User{}, repository.Session{}, ErrEmailTaken
		}
		return repository.User{}, repository.Session{}, err
	}
	session, err := s.issue(user)
	return user, session, err
}

func (s *AccountService) Login(in Credentials) (repository.User, repository.Session, error) {
	user, err := s.repo.GetUserByEmail(normalizeEmail(in.Email))
	if err != nil {
		// The same answer for an unknown address as for a wrong password, so the
		// endpoint cannot be used to find out who has an account here.
		return repository.User{}, repository.Session{}, ErrInvalidCredentials
	}
	if !verifyPassword(user.PasswordHash, in.Password) {
		return repository.User{}, repository.Session{}, ErrInvalidCredentials
	}
	session, err := s.issue(user)
	return user, session, err
}

func (s *AccountService) Logout(token string) error {
	return s.repo.DeleteSession(token)
}

func (s *AccountService) issue(user repository.User) (repository.Session, error) {
	return s.repo.CreateSession(repository.Session{
		UserID: user.ID,
		Token:  newToken(),
		Name:   user.DisplayName,
	})
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// hashPassword uses argon2id with a per-password salt, stored as salt:hash.
func hashPassword(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash)
}

func verifyPassword(stored, password string) bool {
	salt, want, ok := strings.Cut(stored, ":")
	if !ok {
		return false
	}
	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		return false
	}
	wantBytes, err := hex.DecodeString(want)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), saltBytes, 1, 64*1024, 4, 32)
	// Constant time, so a wrong password cannot be narrowed down by how long
	// the comparison took.
	return subtle.ConstantTimeCompare(got, wantBytes) == 1
}
