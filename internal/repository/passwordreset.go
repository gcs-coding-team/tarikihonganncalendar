package repository

import (
	"time"
)

// A password reset is a one-shot token with an expiry. Like a session token it
// is stored hashed, so the database never holds anything that can be redeemed.
//
// The window is deliberately short. A reset link is the one thing that can take
// an account over without knowing the password, so it should not sit valid in a
// mailbox for a week.
const PasswordResetTTL = 30 * time.Minute

type PasswordReset struct {
	ID        string
	UserID    string
	Token     string // only set when issued; never read back from storage
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type PasswordResetRepository interface {
	CreatePasswordReset(reset PasswordReset) (PasswordReset, error)
	// ConsumePasswordReset returns the reset for a token and marks it spent, in
	// one step. Splitting the check from the spend leaves room for the same
	// token to be redeemed twice.
	ConsumePasswordReset(token string) (PasswordReset, error)
	UpdateUserPassword(userID, passwordHash string) error
	// DeleteSessionsForUser signs out everywhere. Someone resetting a password
	// may be doing it because another person is using their account.
	DeleteSessionsForUser(userID string) error
}

func (r *MemoryRepository) CreatePasswordReset(reset PasswordReset) (PasswordReset, error) {
	r.userMu.Lock()
	defer r.userMu.Unlock()
	if reset.ID == "" {
		reset.ID = newID()
	}
	reset.CreatedAt = time.Now().UTC()
	if reset.ExpiresAt.IsZero() {
		reset.ExpiresAt = reset.CreatedAt.Add(PasswordResetTTL)
	}
	r.resets[string(HashToken(reset.Token))] = reset
	return reset, nil
}

func (r *MemoryRepository) ConsumePasswordReset(token string) (PasswordReset, error) {
	r.userMu.Lock()
	defer r.userMu.Unlock()
	key := string(HashToken(token))
	reset, ok := r.resets[key]
	if !ok || reset.UsedAt != nil || time.Now().After(reset.ExpiresAt) {
		return PasswordReset{}, ErrNotFound
	}
	now := time.Now().UTC()
	reset.UsedAt = &now
	r.resets[key] = reset
	return reset, nil
}

func (r *MemoryRepository) UpdateUserPassword(userID, passwordHash string) error {
	r.userMu.Lock()
	defer r.userMu.Unlock()
	user, ok := r.users[userID]
	if !ok {
		return ErrNotFound
	}
	user.PasswordHash = passwordHash
	user.UpdatedAt = time.Now().UTC()
	r.users[userID] = user
	return nil
}

func (r *MemoryRepository) DeleteSessionsForUser(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for token, session := range r.sessions {
		if session.UserID == userID {
			delete(r.sessions, token)
		}
	}
	return nil
}
