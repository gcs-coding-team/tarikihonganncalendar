package repository

import (
	"context"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
)

type AnalysisJobRepository interface {
	Create(ctx context.Context, job *domain.AnalysisJob) error
	FindByID(ctx context.Context, id string) (*domain.AnalysisJob, error)
	FindByUserID(ctx context.Context, userID string) ([]*domain.AnalysisJob, error)
	FindByPrintID(ctx context.Context, printID string) (*domain.AnalysisJob, error)
	UpdateStatus(ctx context.Context, id string, status domain.AnalysisStatus, attemptCount int) error
	UpdateError(ctx context.Context, id string, status domain.AnalysisStatus, errorCode, errorMessage string) error
	FindPendingJobs(ctx context.Context, limit int) ([]*domain.AnalysisJob, error)
}

type AnalysisResultRepository interface {
	Create(ctx context.Context, result *domain.AnalysisResult) error
	FindByJobID(ctx context.Context, jobID string) (*domain.AnalysisResult, error)
	Update(ctx context.Context, result *domain.AnalysisResult) error
}
