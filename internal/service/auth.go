package service

import (
	"fmt"
	"strings"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

type AuthService struct {
	repo repository.SessionRepository
}

func NewAuthService(repo repository.SessionRepository) *AuthService {
	return &AuthService{repo: repo}
}

func (s *AuthService) CreateSession(userID, name string) (repository.Session, error) {
	if userID == "" {
		return repository.Session{}, repository.ErrForbidden
	}
	return s.repo.CreateSession(repository.Session{UserID: userID, Name: name, Token: fmt.Sprintf("sess-%s", userID)})
}

func (s *AuthService) ResolveUserID(headerUserID, authorization string) string {
	if headerUserID != "" {
		return headerUserID
	}
	if authorization == "" {
		return ""
	}
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	token := parts[1]
	if token == "" {
		return ""
	}
	if session, err := s.repo.GetSessionByToken(token); err == nil {
		return session.UserID
	}
	return ""
}

func (s *AuthService) Logout(token string) error {
	return s.repo.DeleteSession(token)
}
