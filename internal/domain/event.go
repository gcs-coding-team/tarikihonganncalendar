package domain

import "time"

type Event struct {
	ID            string
	UserID        string
	Title         string
	Description   string
	StartAt       time.Time
	EndAt         *time.Time
	AllDay        bool
	SourcePrintID *string
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
