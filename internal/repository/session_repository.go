package repository

import (
	"context"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
)

type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	FindByTokenHash(ctx context.Context, tokenHash []byte) (*domain.Session, error)
	DeleteByUserID(ctx context.Context, userID string) error
	DeleteByID(ctx context.Context, id string) error
}
