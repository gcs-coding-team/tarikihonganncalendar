package repository

import (
	"context"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
)

type PrintRepository interface {
	Create(ctx context.Context, print *domain.Print) error
	FindByID(ctx context.Context, id string) (*domain.Print, error)
	FindByUserID(ctx context.Context, userID string) ([]*domain.Print, error)
	UpdateStatus(ctx context.Context, id string, status domain.UploadStatus) error
}
