package domain

import "time"

type Session struct {
	ID         string
	UserID     string
	TokenHash  []byte
	ExpiresAt  time.Time
	LastUsedAt time.Time
	CreatedAt  time.Time
}
