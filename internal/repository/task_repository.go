package repository

import (
	"context"
	"time"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
)

type ListTasksFilter struct {
	Cursor       string
	Limit        int
	UpdatedAfter *time.Time
}

type TaskRepository interface {
	Create(ctx context.Context, task *domain.Task) error
	FindByID(ctx context.Context, id string) (*domain.Task, error)
	FindByUserID(ctx context.Context, userID string, filter ListTasksFilter) ([]*domain.Task, string, error)
	Update(ctx context.Context, task *domain.Task) error
	Delete(ctx context.Context, id string) error
}
