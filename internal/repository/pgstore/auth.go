package pgstore

import (
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

// Sessions here are the token-bearing rows the handler resolves callers with.
// Tokens are stored hashed: a leaked database should not hand out live sessions.

func (s *Store) CreateSession(session repository.Session) (repository.Session, error) {
	if session.ID == "" {
		session.ID = repository.NewID()
	}
	now := time.Now().UTC()
	if _, err := s.pool.Exec(ctxb(),
		`INSERT INTO sessions (id, user_id, token_hash, expires_at, last_used_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$5)
		 ON CONFLICT (token_hash) DO UPDATE SET last_used_at = EXCLUDED.last_used_at`,
		session.ID, session.UserID, repository.HashToken(session.Token),
		now.Add(repository.SessionTTL), now); err != nil {
		return repository.Session{}, mapErr(err)
	}
	session.CreatedAt, session.UpdatedAt = now, now
	return session, nil
}

func (s *Store) GetSessionByToken(token string) (repository.Session, error) {
	var out repository.Session
	var expires time.Time
	err := s.pool.QueryRow(ctxb(),
		`SELECT s.id, s.user_id, s.expires_at, s.created_at, COALESCE(u.display_name, '')
		 FROM sessions s LEFT JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1`, repository.HashToken(token)).
		Scan(&out.ID, &out.UserID, &expires, &out.CreatedAt, &out.Name)
	if err != nil {
		return repository.Session{}, mapErr(err)
	}
	if time.Now().After(expires) {
		_ = s.DeleteSession(token)
		return repository.Session{}, repository.ErrNotFound
	}
	out.Token = token
	out.UpdatedAt = out.CreatedAt
	return out, nil
}

func (s *Store) DeleteSession(token string) error {
	tag, err := s.pool.Exec(ctxb(),
		`DELETE FROM sessions WHERE token_hash = $1`, repository.HashToken(token))
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

/* ---------- users ---------- */

const userCols = `id, email, password_hash, display_name, created_at, updated_at`

func scanUser(row pgx.Row) (repository.User, error) {
	var u repository.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)
	return u, mapErr(err)
}

func (s *Store) CreateUser(user repository.User) (repository.User, error) {
	if user.ID == "" {
		user.ID = repository.NewID()
	}
	now := time.Now().UTC()
	return scanUser(s.pool.QueryRow(ctxb(), `INSERT INTO users (`+userCols+`)
		VALUES ($1,$2,$3,$4,$5,$5) RETURNING `+userCols,
		user.ID, user.Email, user.PasswordHash, user.DisplayName, now))
}

func (s *Store) GetUserByEmail(email string) (repository.User, error) {
	return scanUser(s.pool.QueryRow(ctxb(),
		`SELECT `+userCols+` FROM users WHERE email = $1`, email))
}

func (s *Store) GetUserByID(id string) (repository.User, error) {
	return scanUser(s.pool.QueryRow(ctxb(),
		`SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

/* ---------- analysis jobs ---------- */

const jobCols = `id, user_id, content_type, filename, status, result_summary, created_at, updated_at`

func scanJob(row pgx.Row) (repository.AnalysisJob, error) {
	var j repository.AnalysisJob
	err := row.Scan(&j.ID, &j.UserID, &j.ContentType, &j.Filename, &j.Status,
		&j.ResultSummary, &j.CreatedAt, &j.UpdatedAt)
	return j, mapErr(err)
}

func (s *Store) CreateAnalysisJob(job repository.AnalysisJob) (repository.AnalysisJob, error) {
	if job.ID == "" {
		job.ID = repository.NewID()
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	now := time.Now().UTC()
	return scanJob(s.pool.QueryRow(ctxb(), `INSERT INTO analysis_jobs (`+jobCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$7) RETURNING `+jobCols,
		job.ID, job.UserID, job.ContentType, job.Filename, job.Status, job.ResultSummary, now))
}

func (s *Store) GetAnalysisJob(jobID string) (repository.AnalysisJob, error) {
	return scanJob(s.pool.QueryRow(ctxb(),
		`SELECT `+jobCols+` FROM analysis_jobs WHERE id = $1`, jobID))
}

func (s *Store) ListAnalysisJobs(userID string) ([]repository.AnalysisJob, error) {
	rows, err := s.pool.Query(ctxb(), `SELECT `+jobCols+` FROM analysis_jobs
		WHERE user_id=$1 ORDER BY created_at DESC, id`, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.AnalysisJob, 0)
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, j)
	}
	return items, mapErr(rows.Err())
}

func (s *Store) UpdateAnalysisJob(job repository.AnalysisJob) (repository.AnalysisJob, error) {
	return scanJob(s.pool.QueryRow(ctxb(), `UPDATE analysis_jobs
		SET status=$1, result_summary=$2, updated_at=now()
		WHERE id=$3 RETURNING `+jobCols,
		job.Status, job.ResultSummary, job.ID))
}

/* ---------- analysis results ---------- */

func (s *Store) SaveCandidates(jobID string, cands []repository.Candidate) error {
	tx, err := s.pool.Begin(ctxb())
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback(ctxb())
	if _, err := tx.Exec(ctxb(), `DELETE FROM analysis_results WHERE job_id = $1`, jobID); err != nil {
		return mapErr(err)
	}
	for _, c := range cands {
		if _, err := tx.Exec(ctxb(),
			`INSERT INTO analysis_results (id, job_id, candidate_type, title, date, time, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			repository.NewID(), jobID, c.Type, c.Title, c.Date, c.Time, time.Now().UTC()); err != nil {
			return mapErr(err)
		}
	}
	return mapErr(tx.Commit(ctxb()))
}

func (s *Store) ListCandidates(jobID string) ([]repository.Candidate, error) {
	rows, err := s.pool.Query(ctxb(),
		`SELECT candidate_type, title, date, time FROM analysis_results
		 WHERE job_id=$1 ORDER BY created_at, id`, jobID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.Candidate, 0)
	for rows.Next() {
		var c repository.Candidate
		if err := rows.Scan(&c.Type, &c.Title, &c.Date, &c.Time); err != nil {
			return nil, mapErr(err)
		}
		items = append(items, c)
	}
	return items, mapErr(rows.Err())
}
