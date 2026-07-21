package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
	"github.com/zatunohito/tarikihonganncalendar/internal/repository"
)

type TaskRepository struct {
	pool *pgxpool.Pool
}

func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{pool: pool}
}

func (r *TaskRepository) Create(ctx context.Context, task *domain.Task) error {
	query := `INSERT INTO tasks (id, user_id, title, description, due_at, status, source_print_id, version, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := r.pool.Exec(ctx, query,
		task.ID, task.UserID, task.Title, task.Description, task.DueAt, task.Status, task.SourcePrintID, task.Version, task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (r *TaskRepository) FindByID(ctx context.Context, id string) (*domain.Task, error) {
	query := `SELECT id, user_id, title, description, due_at, status, source_print_id, version, created_at, updated_at FROM tasks WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)

	t := &domain.Task{}
	err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.DueAt, &t.Status, &t.SourcePrintID, &t.Version, &t.CreatedAt, &t.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *TaskRepository) FindByUserID(ctx context.Context, userID string, filter repository.ListTasksFilter) ([]*domain.Task, string, error) {
	args := []any{userID}
	query := `SELECT id, user_id, title, description, due_at, status, source_print_id, version, created_at, updated_at FROM tasks WHERE user_id = $1`

	if filter.UpdatedAfter != nil {
		args = append(args, *filter.UpdatedAfter)
		query += ` AND updated_at > $` + string(rune(len(args)+'0'))
	}

	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		query += ` AND (updated_at, id) < (SELECT updated_at, id FROM tasks WHERE id = $` + string(rune(len(args)+'0')) + `)`
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	args = append(args, limit+1)
	query += ` ORDER BY updated_at DESC, id DESC LIMIT $` + string(rune(len(args)+'0'))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var tasks []*domain.Task
	for rows.Next() {
		t := &domain.Task{}
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.DueAt, &t.Status, &t.SourcePrintID, &t.Version, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, "", err
		}
		tasks = append(tasks, t)
	}

	var nextCursor string
	if len(tasks) > limit {
		nextCursor = tasks[limit-1].ID
		tasks = tasks[:limit]
	}

	return tasks, nextCursor, nil
}

func (r *TaskRepository) Update(ctx context.Context, task *domain.Task) error {
	query := `UPDATE tasks SET title = $1, description = $2, due_at = $3, status = $4, version = $5, updated_at = $6 WHERE id = $7 AND version = $8`
	tag, err := r.pool.Exec(ctx, query,
		task.Title, task.Description, task.DueAt, task.Status, task.Version+1, time.Now(), task.ID, task.Version,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	task.Version++
	task.UpdatedAt = time.Now()
	return nil
}

func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	return err
}
