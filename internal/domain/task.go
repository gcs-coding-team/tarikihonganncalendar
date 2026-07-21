package domain

import "time"

type TaskStatus string

const (
	TaskStatusOpen TaskStatus = "OPEN"
	TaskStatusDone TaskStatus = "DONE"
)

type Task struct {
	ID            string
	UserID        string
	Title         string
	Description   string
	DueAt         *time.Time
	Status        TaskStatus
	SourcePrintID *string
	Version       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
