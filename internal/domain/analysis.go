package domain

import "time"

type AnalysisStatus string

const (
	AnalysisStatusQueued         AnalysisStatus = "QUEUED"
	AnalysisStatusProcessing     AnalysisStatus = "PROCESSING"
	AnalysisStatusReviewRequired AnalysisStatus = "REVIEW_REQUIRED"
	AnalysisStatusRetrying       AnalysisStatus = "RETRYING"
	AnalysisStatusFailed         AnalysisStatus = "FAILED"
	AnalysisStatusCompleted      AnalysisStatus = "COMPLETED"
)

type AnalysisJob struct {
	ID              string
	UserID          string
	PrintID         string
	IdempotencyKey  string
	Status          AnalysisStatus
	AttemptCount    int
	AvailableAt     time.Time
	ErrorCode       *string
	ErrorMessage    *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AnalysisResult struct {
	ID            string
	JobID         string
	DocumentTitle string
	ResultJSON    []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
