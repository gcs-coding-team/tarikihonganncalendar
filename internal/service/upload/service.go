package upload

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
	"github.com/zatunohito/tarikihonganncalendar/internal/repository"
	"github.com/zatunohito/tarikihonganncalendar/internal/storage"
)

type Service struct {
	prints  repository.PrintRepository
	storage storage.Client
	cfg     Config
}

type Config struct {
	PresignedURLTTL time.Duration
	MaxUploadBytes  int64
}

func NewService(prints repository.PrintRepository, s storage.Client, cfg Config) *Service {
	return &Service{prints: prints, storage: s, cfg: cfg}
}

type CreateUploadInput struct {
	UserID      string
	FileName    string
	ContentType string
	SizeBytes   int64
}

type CreateUploadResult struct {
	Print     *domain.Print
	UploadURL string
	ExpiresAt time.Time
}

func (s *Service) CreateUpload(ctx context.Context, input CreateUploadInput) (*CreateUploadResult, error) {
	if input.SizeBytes > s.cfg.MaxUploadBytes {
		return nil, fmt.Errorf("file too large: max %d bytes", s.cfg.MaxUploadBytes)
	}

	printID := uuid.Must(uuid.NewV7()).String()
	sanitized := sanitizeFileName(input.FileName)
	objectKey := fmt.Sprintf("private/%s/prints/%s/%s", input.UserID, printID, sanitized)

	print := &domain.Print{
		ID:               printID,
		UserID:           input.UserID,
		ObjectKey:        objectKey,
		OriginalFileName: input.FileName,
		ContentType:      input.ContentType,
		SizeBytes:        input.SizeBytes,
		UploadStatus:     domain.UploadStatusPending,
		CreatedAt:        time.Now(),
	}

	if err := s.prints.Create(ctx, print); err != nil {
		return nil, err
	}

	uploadURL, expiresAt, err := s.storage.PresignedPutURL(ctx, objectKey, s.cfg.PresignedURLTTL)
	if err != nil {
		return nil, err
	}

	return &CreateUploadResult{
		Print:     print,
		UploadURL: uploadURL,
		ExpiresAt: expiresAt,
	}, nil
}

func sanitizeFileName(name string) string {
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}
