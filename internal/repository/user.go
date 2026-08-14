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
	// Confidence is "high" or "medium" when the model volunteers one, empty
	// otherwise. It is a hint, not a guarantee — the model is not asked for it
	// and nothing here is rejected for lacking it.
	Confidence string `json:"confidence,omitempty"`
}

type UserRepository interface {
	CreateUser(user User) (User, error)
	GetUserByEmail(email string) (User, error)
	GetUserByID(id string) (User, error)
	UpdateUserDisplayName(userID, name string) (User, error)
	// UpdateUserEmail returns ErrDuplicate if another account already uses it.
	UpdateUserEmail(userID, email string) (User, error)
	// DeleteUser removes the account and everything it owns: events, tasks,
	// projects, timetable entries, prints, sessions, any colony it owns (and
	// that colony's membership and shared items), and its membership in
	// colonies owned by someone else. There is no undo.
	DeleteUser(userID string) error
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

func (r *MemoryRepository) UpdateUserDisplayName(userID, name string) (User, error) {
	r.userMu.Lock()
	defer r.userMu.Unlock()
	user, ok := r.users[userID]
	if !ok {
		return User{}, ErrNotFound
	}
	user.DisplayName = name
	user.UpdatedAt = time.Now().UTC()
	r.users[userID] = user
	return user, nil
}

func (r *MemoryRepository) UpdateUserEmail(userID, email string) (User, error) {
	r.userMu.Lock()
	defer r.userMu.Unlock()
	user, ok := r.users[userID]
	if !ok {
		return User{}, ErrNotFound
	}
	for id, u := range r.users {
		if id != userID && u.Email == email {
			return User{}, ErrDuplicate
		}
	}
	user.Email = email
	user.UpdatedAt = time.Now().UTC()
	r.users[userID] = user
	return user, nil
}

// DeleteUser locks r.mu, then r.taskMu, then r.userMu — in that order and
// never nested the other way round — since it is the only place that needs
// more than one of the three at once.
func (r *MemoryRepository) DeleteUser(userID string) error {
	r.mu.Lock()
	for id, e := range r.events {
		if e.UserID == userID {
			delete(r.events, id)
		}
	}
	for id, t := range r.timetable {
		if t.UserID == userID {
			delete(r.timetable, id)
		}
	}
	for id, c := range r.colonies {
		if c.OwnerUserID == userID {
			delete(r.colonies, id)
			delete(r.colonyMembers, id)
			r.deleteSharedItemsForColonyLocked(id)
		}
	}
	for _, members := range r.colonyMembers {
		delete(members, userID)
	}
	delete(r.colonyIndex, userID)
	for token, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, token)
		}
	}
	var jobIDs []string
	for id, j := range r.analysisJobs {
		if j.UserID == userID {
			jobIDs = append(jobIDs, id)
			delete(r.analysisJobs, id)
		}
	}
	r.mu.Unlock()

	r.taskMu.Lock()
	for id, t := range r.tasks {
		if t.UserID == userID {
			delete(r.tasks, id)
		}
	}
	for id, p := range r.projects {
		if p.UserID == userID {
			delete(r.projects, id)
		}
	}
	r.taskMu.Unlock()

	r.userMu.Lock()
	for _, jobID := range jobIDs {
		delete(r.candidates, jobID)
	}
	for id, p := range r.prints {
		if p.UserID == userID {
			delete(r.prints, id)
		}
	}
	for id, reset := range r.resets {
		if reset.UserID == userID {
			delete(r.resets, id)
		}
	}
	delete(r.users, userID)
	r.userMu.Unlock()
	return nil
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
