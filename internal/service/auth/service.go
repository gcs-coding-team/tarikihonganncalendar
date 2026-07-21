package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/google/uuid"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
	"github.com/zatunohito/tarikihonganncalendar/internal/repository"
)

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidCredentials     = errors.New("invalid email or password")
	ErrSessionNotFound        = errors.New("session not found")
)

type Service struct {
	users    repository.UserRepository
	sessions repository.SessionRepository
}

func NewService(users repository.UserRepository, sessions repository.SessionRepository) *Service {
	return &Service{users: users, sessions: sessions}
}

type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
}

type AuthResult struct {
	User        *domain.User
	Session     *domain.Session
	Token       string
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	existing, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailAlreadyRegistered
	}

	userID := uuid.Must(uuid.NewV7()).String()
	now := time.Now()

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	hash := argon2.IDKey([]byte(input.Password), salt, 1, 64*1024, 4, 32)
	passwordHash := hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash)

	user := &domain.User{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
		DisplayName:  strings.TrimSpace(input.DisplayName),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	return s.createSession(ctx, userID, now)
}

func (s *Service) createSession(ctx context.Context, userID string, now time.Time) (*AuthResult, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	tokenHash := sha256.Sum256([]byte(token))

	session := &domain.Session{
		ID:         uuid.Must(uuid.NewV7()).String(),
		UserID:     userID,
		TokenHash:  tokenHash[:],
		ExpiresAt:  now.Add(720 * time.Hour),
		LastUsedAt: now,
		CreatedAt:  now,
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, err
	}

	return &AuthResult{
		User:    &domain.User{ID: userID},
		Session: session,
		Token:   token,
	}, nil
}
