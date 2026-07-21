package task

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
	"github.com/zatunohito/tarikihonganncalendar/internal/repository"
)

var (
	ErrNotFound = errors.New("task not found")
	ErrConflict = errors.New("version conflict")
)

type Service struct {
	tasks repository.TaskRepository
}

func NewService(tasks repository.TaskRepository) *Service {
	return &Service{tasks: tasks}
}

type CreateInput struct {
	UserID      string
	Title       string
	Description string
	DueAt       *time.Time
}

type UpdateInput struct {
	ID          string
	UserID      string
	Title       string
	Description string
	DueAt       *time.Time
	Status      domain.TaskStatus
	Version     int
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*domain.Task, error) {
	now := time.Now()
	task := &domain.Task{
		ID:          uuid.Must(uuid.NewV7()).String(),
		UserID:      input.UserID,
		Title:       input.Title,
		Description: input.Description,
		DueAt:       input.DueAt,
		Status:      domain.TaskStatusOpen,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Service) GetByID(ctx context.Context, taskID, userID string) (*domain.Task, error) {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil || task.UserID != userID {
		return nil, ErrNotFound
	}
	return task, nil
}

func (s *Service) List(ctx context.Context, userID string, filter repository.ListTasksFilter) ([]*domain.Task, string, error) {
	return s.tasks.FindByUserID(ctx, userID, filter)
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (*domain.Task, error) {
	task, err := s.tasks.FindByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	if task == nil || task.UserID != input.UserID {
		return nil, ErrNotFound
	}
	if task.Version != input.Version {
		return nil, ErrConflict
	}

	task.Title = input.Title
	task.Description = input.Description
	task.DueAt = input.DueAt
	task.Status = input.Status

	if err := s.tasks.Update(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Service) Delete(ctx context.Context, taskID, userID string) error {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil || task.UserID != userID {
		return ErrNotFound
	}
	return s.tasks.Delete(ctx, taskID)
}
