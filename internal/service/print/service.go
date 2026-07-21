package print

import (
	"context"
	"errors"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
	"github.com/zatunohito/tarikihonganncalendar/internal/repository"
	"github.com/zatunohito/tarikihonganncalendar/internal/storage"
)

var ErrNotFound = errors.New("print not found")

type Service struct {
	prints  repository.PrintRepository
	storage storage.Client
}

func NewService(prints repository.PrintRepository, s storage.Client) *Service {
	return &Service{prints: prints, storage: s}
}

func (s *Service) GetByID(ctx context.Context, printID, userID string) (*domain.Print, error) {
	print, err := s.prints.FindByID(ctx, printID)
	if err != nil {
		return nil, err
	}
	if print == nil || print.UserID != userID {
		return nil, ErrNotFound
	}
	return print, nil
}

func (s *Service) ListByUserID(ctx context.Context, userID string) ([]*domain.Print, error) {
	return s.prints.FindByUserID(ctx, userID)
}

func (s *Service) Delete(ctx context.Context, printID, userID string) error {
	print, err := s.prints.FindByID(ctx, printID)
	if err != nil {
		return err
	}
	if print == nil || print.UserID != userID {
		return ErrNotFound
	}

	if err := s.storage.DeleteObject(ctx, print.ObjectKey); err != nil {
		return err
	}

	if err := s.prints.UpdateStatus(ctx, printID, domain.UploadStatusFailed); err != nil {
		return err
	}

	return nil
}

func (s *Service) MarkCompleted(ctx context.Context, printID, userID string) error {
	print, err := s.prints.FindByID(ctx, printID)
	if err != nil {
		return err
	}
	if print == nil || print.UserID != userID {
		return ErrNotFound
	}
	return s.prints.UpdateStatus(ctx, printID, domain.UploadStatusCompleted)
}
