package repository

import "time"

// A Print is the image a job read. Keeping it is what lets someone go back and
// ask "which handout was that?" — without it the app throws away the only
// evidence for the dates it put in your calendar.
type Print struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	JobID       string    `json:"jobId"`
	ObjectKey   string    `json:"-"`
	ContentType string    `json:"contentType"`
	Filename    string    `json:"filename"`
	CreatedAt   time.Time `json:"createdAt"`
}

type PrintRepository interface {
	CreatePrint(print Print) (Print, error)
	GetPrint(userID, printID string) (Print, error)
	ListPrints(userID string) ([]Print, error)
}

func (r *MemoryRepository) CreatePrint(print Print) (Print, error) {
	r.userMu.Lock()
	defer r.userMu.Unlock()
	if print.ID == "" {
		print.ID = newID()
	}
	print.CreatedAt = time.Now().UTC()
	r.prints[print.ID] = print
	return print, nil
}

func (r *MemoryRepository) GetPrint(userID, printID string) (Print, error) {
	r.userMu.RLock()
	defer r.userMu.RUnlock()
	print, ok := r.prints[printID]
	if !ok {
		return Print{}, ErrNotFound
	}
	if print.UserID != userID {
		return Print{}, ErrForbidden
	}
	return print, nil
}

func (r *MemoryRepository) ListPrints(userID string) ([]Print, error) {
	r.userMu.RLock()
	defer r.userMu.RUnlock()
	items := make([]Print, 0)
	for _, print := range r.prints {
		if print.UserID == userID {
			items = append(items, print)
		}
	}
	return items, nil
}
