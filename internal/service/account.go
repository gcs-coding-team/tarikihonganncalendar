package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log"
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
	ErrInvalidResetToken  = errors.New("reset link is invalid or has expired")
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

// UpdateDisplayName is the one field on an account nobody needs a password to
// prove they own — it is just a label.
func (s *AccountService) UpdateDisplayName(userID, name string) (repository.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return repository.User{}, repository.ValidationError("displayName is required")
	}
	return s.repo.UpdateUserDisplayName(userID, name)
}

// ChangeEmail and ChangePassword both re-check the current password even
// though the caller already holds a valid session: a session can be left open
// on a shared computer, and the one thing that should still stop someone
// there from taking the account over is the password.

func (s *AccountService) ChangeEmail(userID, currentPassword, newEmail string) (repository.User, error) {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return repository.User{}, err
	}
	if !verifyPassword(user.PasswordHash, currentPassword) {
		return repository.User{}, ErrInvalidCredentials
	}
	email := normalizeEmail(newEmail)
	if email == "" || !strings.Contains(email, "@") {
		return repository.User{}, repository.ValidationError("email is required")
	}
	updated, err := s.repo.UpdateUserEmail(userID, email)
	if errors.Is(err, repository.ErrDuplicate) {
		return repository.User{}, ErrEmailTaken
	}
	return updated, err
}

// ChangePassword drops every other session so a stolen one stops working the
// moment the real owner changes the password, then hands back a fresh token
// for the device that just proved it is them — otherwise the person who just
// changed their own password would find themselves logged out by it.
func (s *AccountService) ChangePassword(userID, currentPassword, newPassword string) (repository.Session, error) {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return repository.Session{}, err
	}
	if !verifyPassword(user.PasswordHash, currentPassword) {
		return repository.Session{}, ErrInvalidCredentials
	}
	if len(newPassword) < 8 {
		return repository.Session{}, repository.ValidationError("password must be at least 8 characters")
	}
	if err := s.repo.UpdateUserPassword(userID, hashPassword(newPassword)); err != nil {
		return repository.Session{}, err
	}
	if err := s.repo.DeleteSessionsForUser(userID); err != nil {
		return repository.Session{}, err
	}
	return s.issue(user)
}

// DeleteAccount takes the password once more, for the same reason as above,
// and then there is nothing left to reconsider — everything it owns goes with it.
func (s *AccountService) DeleteAccount(userID, currentPassword string) error {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return err
	}
	if !verifyPassword(user.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}
	return s.repo.DeleteUser(userID)
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

// Delivery hands a reset link to its owner. There is no mailer wired up here,
// so the default writes the token to the log: enough to run the flow end to end
// in development, and obvious enough in production logs that nobody mistakes it
// for a finished feature.
type Delivery interface {
	SendPasswordReset(email, token string) error
	SendTaskReminder(email, title, dueAt string) error
}

type LogDelivery struct{}

func (LogDelivery) SendPasswordReset(email, token string) error {
	log.Printf("password reset for %s: token=%s (no mailer configured)", email, token)
	return nil
}

func (LogDelivery) SendTaskReminder(email, title, dueAt string) error {
	log.Printf("task reminder for %s: %q due %s (no mailer configured)", email, title, dueAt)
	return nil
}

// RequestPasswordReset always reports success, whether or not the address is
// registered. Saying "no such account" here would turn the endpoint into a way
// to find out who has one.
func (s *AccountService) RequestPasswordReset(email string, deliver Delivery) error {
	user, err := s.repo.GetUserByEmail(normalizeEmail(email))
	if err != nil {
		return nil
	}
	reset, err := s.repo.CreatePasswordReset(repository.PasswordReset{
		UserID: user.ID,
		Token:  newToken(),
	})
	if err != nil {
		return err
	}
	if deliver == nil {
		deliver = LogDelivery{}
	}
	return deliver.SendPasswordReset(user.Email, reset.Token)
}

// ResetPassword spends the token and sets the new password. Every existing
// session is dropped: someone resetting a password may be doing it precisely
// because somebody else is signed in as them.
func (s *AccountService) ResetPassword(token, password string) error {
	if len(password) < 8 {
		return repository.ValidationError("password must be at least 8 characters")
	}
	reset, err := s.repo.ConsumePasswordReset(token)
	if err != nil {
		return ErrInvalidResetToken
	}
	if err := s.repo.UpdateUserPassword(reset.UserID, hashPassword(password)); err != nil {
		return err
	}
	return s.repo.DeleteSessionsForUser(reset.UserID)
}
