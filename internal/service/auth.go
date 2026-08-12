package service

import (
	"strings"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

// AuthService turns a request's headers into the user making it.
//
// There used to be two ways straight past it, both left over from before
// passwords existed. An X-User-ID header was taken at face value, and POST
// /v1/auth/sessions minted a session for any user id with no password at all,
// under a token of the form "sess-<userID>" that anyone could guess. Either one
// made the password on an account decorative.
//
// Both now sit behind DevAuth, which is off unless ALLOW_DEV_AUTH is set.
type AuthService struct {
	repo    repository.SessionRepository
	DevAuth bool
}

func (s *AuthService) Repo() repository.SessionRepository {
	return s.repo
}

func NewAuthService(repo repository.SessionRepository) *AuthService {
	return &AuthService{repo: repo}
}

func (s *AuthService) CreateSession(userID, name string) (repository.Session, error) {
	if !s.DevAuth || userID == "" {
		return repository.Session{}, repository.ErrForbidden
	}
	// Still a development shortcut, but no longer a guessable token: whoever
	// leaves DevAuth on should not also be handing out sessions by name.
	return s.repo.CreateSession(repository.Session{UserID: userID, Name: name, Token: newToken()})
}

// ResolveUserID identifies the caller. A bearer token is the only way in unless
// DevAuth is on.
func (s *AuthService) ResolveUserID(headerUserID, authorization string) string {
	if s.DevAuth && headerUserID != "" {
		return headerUserID
	}
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	token := strings.TrimSpace(parts[1])
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
