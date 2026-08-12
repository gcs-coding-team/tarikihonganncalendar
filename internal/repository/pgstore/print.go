package pgstore

import (
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

const printCols = `id, user_id, object_key, content_type, filename, status, created_at`

func scanPrint(row pgx.Row) (repository.Print, error) {
	var p repository.Print
	var status string
	err := row.Scan(&p.ID, &p.UserID, &p.ObjectKey, &p.ContentType, &p.Filename, &status, &p.CreatedAt)
	if err != nil {
		return repository.Print{}, mapErr(err)
	}
	// status doubles as the owning job id; the column predates the field.
	p.JobID = status
	return p, nil
}

func (s *Store) CreatePrint(print repository.Print) (repository.Print, error) {
	if print.ID == "" {
		print.ID = repository.NewID()
	}
	return scanPrint(s.pool.QueryRow(ctxb(), `INSERT INTO prints (`+printCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+printCols,
		print.ID, print.UserID, print.ObjectKey, print.ContentType, print.Filename,
		print.JobID, time.Now().UTC()))
}

func (s *Store) GetPrint(userID, printID string) (repository.Print, error) {
	p, err := scanPrint(s.pool.QueryRow(ctxb(),
		`SELECT `+printCols+` FROM prints WHERE id = $1`, printID))
	if err != nil {
		return repository.Print{}, err
	}
	if p.UserID != userID {
		return repository.Print{}, repository.ErrForbidden
	}
	return p, nil
}

func (s *Store) ListPrints(userID string) ([]repository.Print, error) {
	rows, err := s.pool.Query(ctxb(), `SELECT `+printCols+` FROM prints
		WHERE user_id=$1 ORDER BY created_at DESC, id`, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.Print, 0)
	for rows.Next() {
		p, err := scanPrint(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, mapErr(rows.Err())
}
