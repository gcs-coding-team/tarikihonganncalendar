// Package pgstore backs repository.Repository with Postgres.
//
// It is a drop-in alternative to repository.MemoryRepository: the handler talks
// to the same interface either way, so the only thing that decides which one is
// running is whether DATABASE_URL is set. That is what lets the app keep its
// data across a restart without the rest of the code knowing.
package pgstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Migrate applies the schema. Every statement is IF NOT EXISTS, so it is safe to
// run on every boot and there is no separate migration step to forget.
func (s *Store) Migrate(ctx context.Context, schema string) error {
	_, err := s.pool.Exec(ctx, schema)
	return err
}

func ctxb() context.Context { return context.Background() }

// mapErr turns the driver's errors into the ones the rest of the app checks for.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.ErrNotFound
	}
	if strings.Contains(err.Error(), "SQLSTATE 23505") { // unique_violation
		return repository.ErrDuplicate
	}
	if strings.Contains(err.Error(), "SQLSTATE 23503") { // foreign_key_violation
		return repository.ValidationError("referenced row does not exist")
	}
	return err
}

/* ---------- events ---------- */

func scanEvent(row pgx.Row) (repository.Event, error) {
	var e repository.Event
	var freq, until *string
	err := row.Scan(&e.ID, &e.UserID, &e.Title, &e.Description, &e.StartAt, &e.EndAt,
		&e.AllDay, &freq, &until, &e.ExDates, &e.Version, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return repository.Event{}, mapErr(err)
	}
	if freq != nil {
		e.Repeat = &repository.Repeat{Freq: *freq}
		if until != nil {
			e.Repeat.Until = *until
		}
	}
	return e, nil
}

const eventCols = `id, user_id, title, description, start_at, end_at, all_day,
	repeat_freq, repeat_until, exdates, version, created_at, updated_at`

func repeatCols(r *repository.Repeat) (*string, *string) {
	if r == nil {
		return nil, nil
	}
	freq := r.Freq
	if r.Until == "" {
		return &freq, nil
	}
	until := r.Until
	return &freq, &until
}

func (s *Store) CreateEvent(event repository.Event) (repository.Event, error) {
	if event.ID == "" {
		event.ID = repository.NewID()
	}
	now := time.Now().UTC()
	freq, until := repeatCols(event.Repeat)
	if event.ExDates == nil {
		event.ExDates = []string{}
	}
	row := s.pool.QueryRow(ctxb(), `INSERT INTO events (`+eventCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,1,$11,$11) RETURNING `+eventCols,
		event.ID, event.UserID, event.Title, event.Description, event.StartAt, event.EndAt,
		event.AllDay, freq, until, event.ExDates, now)
	return scanEvent(row)
}

func (s *Store) ListEvents(userID, cursor string, limit int) ([]repository.Event, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := s.pool.Query(ctxb(), `SELECT `+eventCols+` FROM events
		WHERE user_id = $1 ORDER BY start_at, id LIMIT $2`, userID, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.Event, 0)
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, mapErr(rows.Err())
}

func (s *Store) GetEvent(userID, eventID string) (repository.Event, error) {
	return scanEvent(s.pool.QueryRow(ctxb(),
		`SELECT `+eventCols+` FROM events WHERE id = $1 AND user_id = $2`, eventID, userID))
}

func (s *Store) UpdateEvent(event repository.Event) (repository.Event, error) {
	freq, until := repeatCols(event.Repeat)
	if event.ExDates == nil {
		event.ExDates = []string{}
	}
	// The version in the WHERE clause is the whole optimistic lock: no row comes
	// back when someone else has written since this copy was read.
	row := s.pool.QueryRow(ctxb(), `UPDATE events SET
		title=$1, description=$2, start_at=$3, end_at=$4, all_day=$5,
		repeat_freq=$6, repeat_until=$7, exdates=$8,
		version = version + 1, updated_at = now()
		WHERE id=$9 AND user_id=$10 AND version=$11 RETURNING `+eventCols,
		event.Title, event.Description, event.StartAt, event.EndAt, event.AllDay,
		freq, until, event.ExDates, event.ID, event.UserID, event.Version)
	e, err := scanEvent(row)
	if errors.Is(err, repository.ErrNotFound) {
		return repository.Event{}, s.existsOrConflict("events", event.ID)
	}
	return e, err
}

func (s *Store) DeleteEvent(userID, eventID string) error {
	tag, err := s.pool.Exec(ctxb(), `DELETE FROM events WHERE id=$1 AND user_id=$2`, eventID, userID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// existsOrConflict distinguishes "gone" from "someone got there first" after an
// update matched nothing.
func (s *Store) existsOrConflict(table, id string) error {
	var n int
	if err := s.pool.QueryRow(ctxb(),
		`SELECT count(*) FROM `+table+` WHERE id = $1`, id).Scan(&n); err != nil {
		return mapErr(err)
	}
	if n == 0 {
		return repository.ErrNotFound
	}
	return repository.ErrConflict
}

/* ---------- timetable ---------- */

const ttCols = `id, user_id, day_of_week, period, subject, room, teacher, version, created_at, updated_at`

func scanEntry(row pgx.Row) (repository.TimetableEntry, error) {
	var e repository.TimetableEntry
	err := row.Scan(&e.ID, &e.UserID, &e.DayOfWeek, &e.Period, &e.Subject,
		&e.Room, &e.Teacher, &e.Version, &e.CreatedAt, &e.UpdatedAt)
	return e, mapErr(err)
}

func (s *Store) CreateTimetableEntry(entry repository.TimetableEntry) (repository.TimetableEntry, error) {
	if entry.ID == "" {
		entry.ID = repository.NewID()
	}
	now := time.Now().UTC()
	return scanEntry(s.pool.QueryRow(ctxb(), `INSERT INTO timetable_entries (`+ttCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,$8) RETURNING `+ttCols,
		entry.ID, entry.UserID, entry.DayOfWeek, entry.Period, entry.Subject,
		entry.Room, entry.Teacher, now))
}

func (s *Store) ListTimetableEntries(userID string) ([]repository.TimetableEntry, error) {
	rows, err := s.pool.Query(ctxb(), `SELECT `+ttCols+` FROM timetable_entries
		WHERE user_id=$1 ORDER BY day_of_week, period`, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.TimetableEntry, 0)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, mapErr(rows.Err())
}

func (s *Store) GetTimetableEntry(userID, entryID string) (repository.TimetableEntry, error) {
	return scanEntry(s.pool.QueryRow(ctxb(),
		`SELECT `+ttCols+` FROM timetable_entries WHERE id=$1 AND user_id=$2`, entryID, userID))
}

func (s *Store) UpdateTimetableEntry(entry repository.TimetableEntry) (repository.TimetableEntry, error) {
	row := s.pool.QueryRow(ctxb(), `UPDATE timetable_entries SET
		day_of_week=$1, period=$2, subject=$3, room=$4, teacher=$5,
		version = version + 1, updated_at = now()
		WHERE id=$6 AND user_id=$7 AND version=$8 RETURNING `+ttCols,
		entry.DayOfWeek, entry.Period, entry.Subject, entry.Room, entry.Teacher,
		entry.ID, entry.UserID, entry.Version)
	e, err := scanEntry(row)
	if errors.Is(err, repository.ErrNotFound) {
		return repository.TimetableEntry{}, s.existsOrConflict("timetable_entries", entry.ID)
	}
	return e, err
}

func (s *Store) DeleteTimetableEntry(userID, entryID string) error {
	tag, err := s.pool.Exec(ctxb(),
		`DELETE FROM timetable_entries WHERE id=$1 AND user_id=$2`, entryID, userID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// Fail the build if this store ever drifts from the interface the handler uses,
// rather than at the first request that hits a missing method.
var _ repository.Repository = (*Store)(nil)
