package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
)

type AnalysisJobRepository struct {
	pool *pgxpool.Pool
}

func NewAnalysisJobRepository(pool *pgxpool.Pool) *AnalysisJobRepository {
	return &AnalysisJobRepository{pool: pool}
}

func (r *AnalysisJobRepository) Create(ctx context.Context, job *domain.AnalysisJob) error {
	query := `INSERT INTO analysis_jobs (id, user_id, print_id, idempotency_key, status, attempt_count, available_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.pool.Exec(ctx, query,
		job.ID, job.UserID, job.PrintID, job.IdempotencyKey, job.Status, job.AttemptCount, job.AvailableAt, job.CreatedAt, job.UpdatedAt,
	)
	return err
}

func (r *AnalysisJobRepository) FindByID(ctx context.Context, id string) (*domain.AnalysisJob, error) {
	query := `SELECT id, user_id, print_id, idempotency_key, status, attempt_count, available_at, error_code, error_message, created_at, updated_at FROM analysis_jobs WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)

	j := &domain.AnalysisJob{}
	err := row.Scan(&j.ID, &j.UserID, &j.PrintID, &j.IdempotencyKey, &j.Status, &j.AttemptCount, &j.AvailableAt, &j.ErrorCode, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (r *AnalysisJobRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.AnalysisJob, error) {
	query := `SELECT id, user_id, print_id, idempotency_key, status, attempt_count, available_at, error_code, error_message, created_at, updated_at FROM analysis_jobs WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*domain.AnalysisJob
	for rows.Next() {
		j := &domain.AnalysisJob{}
		if err := rows.Scan(&j.ID, &j.UserID, &j.PrintID, &j.IdempotencyKey, &j.Status, &j.AttemptCount, &j.AvailableAt, &j.ErrorCode, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (r *AnalysisJobRepository) FindByPrintID(ctx context.Context, printID string) (*domain.AnalysisJob, error) {
	query := `SELECT id, user_id, print_id, idempotency_key, status, attempt_count, available_at, error_code, error_message, created_at, updated_at FROM analysis_jobs WHERE print_id = $1 ORDER BY created_at DESC LIMIT 1`
	row := r.pool.QueryRow(ctx, query, printID)

	j := &domain.AnalysisJob{}
	err := row.Scan(&j.ID, &j.UserID, &j.PrintID, &j.IdempotencyKey, &j.Status, &j.AttemptCount, &j.AvailableAt, &j.ErrorCode, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (r *AnalysisJobRepository) UpdateStatus(ctx context.Context, id string, status domain.AnalysisStatus, attemptCount int) error {
	_, err := r.pool.Exec(ctx, `UPDATE analysis_jobs SET status = $1, attempt_count = $2, updated_at = NOW() WHERE id = $3`, status, attemptCount, id)
	return err
}

func (r *AnalysisJobRepository) UpdateError(ctx context.Context, id string, status domain.AnalysisStatus, errorCode, errorMessage string) error {
	_, err := r.pool.Exec(ctx, `UPDATE analysis_jobs SET status = $1, error_code = $2, error_message = $3, updated_at = NOW() WHERE id = $4`, status, errorCode, errorMessage, id)
	return err
}

func (r *AnalysisJobRepository) FindPendingJobs(ctx context.Context, limit int) ([]*domain.AnalysisJob, error) {
	query := `SELECT id, user_id, print_id, idempotency_key, status, attempt_count, available_at, error_code, error_message, created_at, updated_at FROM analysis_jobs WHERE status IN ('QUEUED', 'RETRYING') AND available_at <= NOW() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*domain.AnalysisJob
	for rows.Next() {
		j := &domain.AnalysisJob{}
		if err := rows.Scan(&j.ID, &j.UserID, &j.PrintID, &j.IdempotencyKey, &j.Status, &j.AttemptCount, &j.AvailableAt, &j.ErrorCode, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

type AnalysisResultRepository struct {
	pool *pgxpool.Pool
}

func NewAnalysisResultRepository(pool *pgxpool.Pool) *AnalysisResultRepository {
	return &AnalysisResultRepository{pool: pool}
}

func (r *AnalysisResultRepository) Create(ctx context.Context, result *domain.AnalysisResult) error {
	query := `INSERT INTO analysis_results (id, job_id, document_title, result_json, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, query,
		result.ID, result.JobID, result.DocumentTitle, result.ResultJSON, result.CreatedAt, result.UpdatedAt,
	)
	return err
}

func (r *AnalysisResultRepository) FindByJobID(ctx context.Context, jobID string) (*domain.AnalysisResult, error) {
	query := `SELECT id, job_id, document_title, result_json, created_at, updated_at FROM analysis_results WHERE job_id = $1`
	row := r.pool.QueryRow(ctx, query, jobID)

	res := &domain.AnalysisResult{}
	err := row.Scan(&res.ID, &res.JobID, &res.DocumentTitle, &res.ResultJSON, &res.CreatedAt, &res.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *AnalysisResultRepository) Update(ctx context.Context, result *domain.AnalysisResult) error {
	_, err := r.pool.Exec(ctx, `UPDATE analysis_results SET document_title = $1, result_json = $2, updated_at = NOW() WHERE id = $3`, result.DocumentTitle, result.ResultJSON, result.ID)
	return err
}
