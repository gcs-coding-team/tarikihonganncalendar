package domain

import "time"

type UploadStatus string

const (
	UploadStatusPending    UploadStatus = "PENDING"
	UploadStatusCompleted  UploadStatus = "COMPLETED"
	UploadStatusFailed     UploadStatus = "FAILED"
)

type Print struct {
	ID               string
	UserID           string
	ObjectKey        string
	OriginalFileName string
	ContentType      string
	SizeBytes        int64
	UploadStatus     UploadStatus
	CreatedAt        time.Time
}
