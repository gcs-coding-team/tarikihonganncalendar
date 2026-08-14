package pgstore

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

const taskCols = `id, user_id, title, description, due_at, status, project_id, version, created_at, updated_at`

func scanTask(row pgx.Row) (repository.Task, error) {
	var t repository.Task
	var projectID *string
	err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.DueAt, &t.Status,
		&projectID, &t.Version, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return repository.Task{}, mapErr(err)
	}
	if projectID != nil {
		t.ProjectID = *projectID
	}
	return t, nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *Store) CreateTask(task repository.Task) (repository.Task, error) {
	if task.ID == "" {
		task.ID = repository.NewID()
	}
	if task.Status == "" {
		task.Status = repository.TaskStatusOpen
	}
	now := time.Now().UTC()
	t, err := scanTask(s.pool.QueryRow(ctxb(), `INSERT INTO tasks (`+taskCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,$8) RETURNING `+taskCols,
		task.ID, task.UserID, task.Title, task.Description, task.DueAt, task.Status,
		nullable(task.ProjectID), now))
	if repository.IsValidationError(err) {
		return repository.Task{}, repository.ValidationError("projectId does not exist")
	}
	return t, err
}

func (s *Store) ListTasks(userID string) ([]repository.Task, error) {
	// Soonest deadline first, undated trailing behind — the empty string sorts
	// before real dates, so it is pushed to the end explicitly.
	rows, err := s.pool.Query(ctxb(), `SELECT `+taskCols+` FROM tasks
		WHERE user_id=$1 ORDER BY (due_at = '') , due_at, id`, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, mapErr(rows.Err())
}

func (s *Store) GetTask(userID, taskID string) (repository.Task, error) {
	t, err := scanTask(s.pool.QueryRow(ctxb(),
		`SELECT `+taskCols+` FROM tasks WHERE id=$1`, taskID))
	if err != nil {
		return repository.Task{}, err
	}
	if t.UserID != userID {
		return repository.Task{}, repository.ErrForbidden
	}
	return t, nil
}

func (s *Store) UpdateTask(task repository.Task) (repository.Task, error) {
	row := s.pool.QueryRow(ctxb(), `UPDATE tasks SET
		title=$1, description=$2, due_at=$3, status=$4, project_id=$5,
		version = version + 1, updated_at = now()
		WHERE id=$6 AND user_id=$7 AND version=$8 RETURNING `+taskCols,
		task.Title, task.Description, task.DueAt, task.Status, nullable(task.ProjectID),
		task.ID, task.UserID, task.Version)
	t, err := scanTask(row)
	if repository.IsValidationError(err) {
		return repository.Task{}, repository.ValidationError("projectId does not exist")
	}
	if errors.Is(err, repository.ErrNotFound) {
		return repository.Task{}, s.existsOrConflict("tasks", task.ID)
	}
	return t, err
}

func (s *Store) DeleteTask(userID, taskID string) error {
	tag, err := s.pool.Exec(ctxb(), `DELETE FROM tasks WHERE id=$1 AND user_id=$2`, taskID, userID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// ListTasksDueUnreminded is not scoped to one user: the reminder job sweeps
// every account at once, so it queries across the table directly rather than
// going through taskCols/scanTask, which assume a single-user context.
func (s *Store) ListTasksDueUnreminded(date string) ([]repository.Task, error) {
	rows, err := s.pool.Query(ctxb(), `SELECT `+taskCols+` FROM tasks
		WHERE status=$1 AND due_at=$2 AND reminded_at <> $2 ORDER BY id`,
		repository.TaskStatusOpen, date)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, mapErr(rows.Err())
}

func (s *Store) MarkTaskReminded(taskID, date string) error {
	tag, err := s.pool.Exec(ctxb(), `UPDATE tasks SET reminded_at=$1 WHERE id=$2`, date, taskID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

const projectCols = `id, user_id, name, description, version, created_at, updated_at`

func scanProject(row pgx.Row) (repository.Project, error) {
	var p repository.Project
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Version, &p.CreatedAt, &p.UpdatedAt)
	return p, mapErr(err)
}

func (s *Store) CreateProject(project repository.Project) (repository.Project, error) {
	if project.ID == "" {
		project.ID = repository.NewID()
	}
	now := time.Now().UTC()
	return scanProject(s.pool.QueryRow(ctxb(), `INSERT INTO projects (`+projectCols+`)
		VALUES ($1,$2,$3,$4,1,$5,$5) RETURNING `+projectCols,
		project.ID, project.UserID, project.Name, project.Description, now))
}

func (s *Store) ListProjects(userID string) ([]repository.Project, error) {
	rows, err := s.pool.Query(ctxb(), `SELECT `+projectCols+` FROM projects
		WHERE user_id=$1 ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.Project, 0)
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, mapErr(rows.Err())
}

func (s *Store) GetProject(userID, projectID string) (repository.Project, error) {
	p, err := scanProject(s.pool.QueryRow(ctxb(),
		`SELECT `+projectCols+` FROM projects WHERE id=$1`, projectID))
	if err != nil {
		return repository.Project{}, err
	}
	if p.UserID != userID {
		return repository.Project{}, repository.ErrForbidden
	}
	return p, nil
}

func (s *Store) UpdateProject(project repository.Project) (repository.Project, error) {
	row := s.pool.QueryRow(ctxb(), `UPDATE projects SET
		name=$1, description=$2, version = version + 1, updated_at = now()
		WHERE id=$3 AND user_id=$4 AND version=$5 RETURNING `+projectCols,
		project.Name, project.Description, project.ID, project.UserID, project.Version)
	p, err := scanProject(row)
	if errors.Is(err, repository.ErrNotFound) {
		return repository.Project{}, s.existsOrConflict("projects", project.ID)
	}
	return p, err
}

// DeleteProject leaves the tasks behind; the ON DELETE SET NULL on tasks.project_id
// unfiles them. Dropping a grouping should not destroy the work filed under it.
func (s *Store) DeleteProject(userID, projectID string) error {
	tag, err := s.pool.Exec(ctxb(),
		`DELETE FROM projects WHERE id=$1 AND user_id=$2`, projectID, userID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}
