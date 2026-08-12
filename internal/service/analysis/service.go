package analysis

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/domain"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository/dbrepo"
)

var (
	ErrJobNotFound    = errors.New("analysis job not found")
	ErrJobNotOwned    = errors.New("analysis job not owned by user")
	ErrInvalidStatus  = errors.New("invalid status for operation")
)

type Service struct {
	jobs    dbrepo.AnalysisJobRepository
	results dbrepo.AnalysisResultRepository
	prints  dbrepo.PrintRepository
}

func NewService(
	jobs dbrepo.AnalysisJobRepository,
	results dbrepo.AnalysisResultRepository,
	prints dbrepo.PrintRepository,
) *Service {
	return &Service{jobs: jobs, results: results, prints: prints}
}

type StartInput struct {
	UserID         string
	PrintID        string
	IdempotencyKey string
}

func (s *Service) Start(ctx context.Context, input StartInput) (*domain.AnalysisJob, error) {
	print, err := s.prints.FindByID(ctx, input.PrintID)
	if err != nil {
		return nil, err
	}
	if print == nil || print.UserID != input.UserID {
		return nil, ErrJobNotFound
	}

	existing, err := s.jobs.FindByPrintID(ctx, input.PrintID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	now := time.Now()
	job := &domain.AnalysisJob{
		ID:             uuid.Must(uuid.NewV7()).String(),
		UserID:         input.UserID,
		PrintID:        input.PrintID,
		IdempotencyKey: input.IdempotencyKey,
		Status:         domain.AnalysisStatusQueued,
		AttemptCount:   0,
		AvailableAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

func (s *Service) GetByID(ctx context.Context, jobID, userID string) (*domain.AnalysisJob, *domain.AnalysisResult, error) {
	job, err := s.jobs.FindByID(ctx, jobID)
	if err != nil {
		return nil, nil, err
	}
	if job == nil || job.UserID != userID {
		return nil, nil, ErrJobNotFound
	}

	result, _ := s.results.FindByJobID(ctx, jobID)
	return job, result, nil
}

func (s *Service) Retry(ctx context.Context, jobID, userID string) error {
	job, err := s.jobs.FindByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job == nil || job.UserID != userID {
		return ErrJobNotFound
	}
	if job.Status != domain.AnalysisStatusFailed {
		return ErrInvalidStatus
	}
	return s.jobs.UpdateStatus(ctx, jobID, domain.AnalysisStatusQueued, 0)
}
