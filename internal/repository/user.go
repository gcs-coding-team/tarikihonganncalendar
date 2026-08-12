package repository

import (
	"crypto/sha256"
	"sync"
	"time"
)

// Users, and the candidates an analysis produced, live here so both backings —
// the in-memory one and repository/pgstore — implement the same interface.

const SessionTTL = 30 * 24 * time.Hour

// HashToken is what actually gets stored for a session. Keeping the raw token
// out of storage means a leaked database does not hand out live sessions.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"displayName"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Candidate is one line an analysis pulled out of a print, before a human
// confirms it into a real task or event.
type Candidate struct {
	Type  string `json:"type"` // "task" or "event"
	Title string `json:"title"`
	Date  string `json:"date"` // YYYY-MM-DD
	Time  string `json:"time"` // HH:MM, events only
}

type UserRepository interface {
	CreateUser(user User) (User, error)
	GetUserByEmail(email string) (User, error)
	GetUserByID(id string) (User, error)
}

type CandidateRepository interface {
	SaveCandidates(jobID string, cands []Candidate) error
	ListCandidates(jobID string) ([]Candidate, error)
}

type userStore struct {
	userMu     sync.RWMutex
	users      map[string]User
	candidates map[string][]Candidate
	resets     map[string]PasswordReset
	prints     map[string]Print
}

func newUserStore() userStore {
	return userStore{
		users:      make(map[string]User),
		candidates: make(map[string][]Candidate),
		resets:     make(map[string]PasswordReset),
		prints:     make(map[string]Print),
	}
}

func (r *MemoryRepository) CreateUser(user User) (User, error) {
	r.userMu.Lock()
	defer r.userMu.Unlock()
	for _, u := range r.users {
		if u.Email == user.Email {
			return User{}, ErrDuplicate
		}
	}
	if user.ID == "" {
		user.ID = newID()
	}
	user.CreatedAt = time.Now().UTC()
	user.UpdatedAt = user.CreatedAt
	r.users[user.ID] = user
	return user, nil
}

func (r *MemoryRepository) GetUserByEmail(email string) (User, error) {
	r.userMu.RLock()
	defer r.userMu.RUnlock()
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (r *MemoryRepository) GetUserByID(id string) (User, error) {
	r.userMu.RLock()
	defer r.userMu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (r *MemoryRepository) SaveCandidates(jobID string, cands []Candidate) error {
	r.userMu.Lock()
	defer r.userMu.Unlock()
	r.candidates[jobID] = cands
	return nil
}

func (r *MemoryRepository) ListCandidates(jobID string) ([]Candidate, error) {
	r.userMu.RLock()
	defer r.userMu.RUnlock()
	out := r.candidates[jobID]
	if out == nil {
		out = []Candidate{}
	}
	return out, nil
}
