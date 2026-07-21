package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
)

type PrintRepository struct {
	pool *pgxpool.Pool
}

func NewPrintRepository(pool *pgxpool.Pool) *PrintRepository {
	return &PrintRepository{pool: pool}
}

func (r *PrintRepository) Create(ctx context.Context, print *domain.Print) error {
	query := `INSERT INTO prints (id, user_id, object_key, original_file_name, content_type, size_bytes, upload_status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.pool.Exec(ctx, query,
		print.ID, print.UserID, print.ObjectKey, print.OriginalFileName, print.ContentType, print.SizeBytes, print.UploadStatus, print.CreatedAt,
	)
	return err
}

func (r *PrintRepository) FindByID(ctx context.Context, id string) (*domain.Print, error) {
	query := `SELECT id, user_id, object_key, original_file_name, content_type, size_bytes, upload_status, created_at FROM prints WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)

	p := &domain.Print{}
	err := row.Scan(&p.ID, &p.UserID, &p.ObjectKey, &p.OriginalFileName, &p.ContentType, &p.SizeBytes, &p.UploadStatus, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *PrintRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.Print, error) {
	query := `SELECT id, user_id, object_key, original_file_name, content_type, size_bytes, upload_status, created_at FROM prints WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prints []*domain.Print
	for rows.Next() {
		p := &domain.Print{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.ObjectKey, &p.OriginalFileName, &p.ContentType, &p.SizeBytes, &p.UploadStatus, &p.CreatedAt); err != nil {
			return nil, err
		}
		prints = append(prints, p)
	}
	return prints, nil
}

func (r *PrintRepository) UpdateStatus(ctx context.Context, id string, status domain.UploadStatus) error {
	_, err := r.pool.Exec(ctx, `UPDATE prints SET upload_status = $1 WHERE id = $2`, status, id)
	return err
}
