package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
)

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	query := `INSERT INTO sessions (id, user_id, token_hash, expires_at, last_used_at, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, query,
		session.ID, session.UserID, session.TokenHash, session.ExpiresAt, session.LastUsedAt, session.CreatedAt,
	)
	return err
}

func (r *SessionRepository) FindByTokenHash(ctx context.Context, tokenHash []byte) (*domain.Session, error) {
	query := `SELECT id, user_id, token_hash, expires_at, last_used_at, created_at FROM sessions WHERE token_hash = $1`
	row := r.pool.QueryRow(ctx, query, tokenHash)

	s := &domain.Session{}
	err := row.Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.LastUsedAt, &s.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM sessions WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

func (r *SessionRepository) DeleteByID(ctx context.Context, id string) error {
	query := `DELETE FROM sessions WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}
