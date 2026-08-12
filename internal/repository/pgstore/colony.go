package pgstore

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

const colonyCols = `id, name, description, owner_user_id, invite_code, created_at, updated_at`

func scanColony(row pgx.Row) (repository.Colony, error) {
	var c repository.Colony
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.OwnerUserID, &c.InviteCode,
		&c.CreatedAt, &c.UpdatedAt)
	return c, mapErr(err)
}

func (s *Store) CreateColony(colony repository.Colony) (repository.Colony, error) {
	if colony.ID == "" {
		colony.ID = repository.NewID()
	}
	tx, err := s.pool.Begin(ctxb())
	if err != nil {
		return repository.Colony{}, mapErr(err)
	}
	defer tx.Rollback(ctxb())

	if colony.InviteCode == "" {
		var n int
		if err := tx.QueryRow(ctxb(), `SELECT count(*) FROM colonies`).Scan(&n); err != nil {
			return repository.Colony{}, mapErr(err)
		}
		colony.InviteCode = fmt.Sprintf("%08d", n+1)
	}
	now := time.Now().UTC()
	c, err := scanColony(tx.QueryRow(ctxb(), `INSERT INTO colonies (`+colonyCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$6) RETURNING `+colonyCols,
		colony.ID, colony.Name, colony.Description, colony.OwnerUserID, colony.InviteCode, now))
	if err != nil {
		return repository.Colony{}, err
	}
	// The creator is enrolled straight away, so membership is the single test
	// for "may see this colony" everywhere else.
	if _, err := tx.Exec(ctxb(),
		`INSERT INTO colony_members (colony_id, user_id, role, joined_at) VALUES ($1,$2,'OWNER',$3)`,
		c.ID, c.OwnerUserID, now); err != nil {
		return repository.Colony{}, mapErr(err)
	}
	return c, mapErr(tx.Commit(ctxb()))
}

// ListColonies returns what the user belongs to, creations included, since
// creating one enrolls them.
func (s *Store) ListColonies(userID string) ([]repository.Colony, error) {
	rows, err := s.pool.Query(ctxb(), `SELECT `+prefixed("c", colonyCols)+`
		FROM colonies c JOIN colony_members m ON m.colony_id = c.id
		WHERE m.user_id = $1 ORDER BY c.created_at, c.id`, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.Colony, 0)
	for rows.Next() {
		c, err := scanColony(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, mapErr(rows.Err())
}

func prefixed(alias, cols string) string {
	out := ""
	for i, c := range splitCols(cols) {
		if i > 0 {
			out += ", "
		}
		out += alias + "." + c
	}
	return out
}

func splitCols(cols string) []string {
	var out []string
	cur := ""
	for _, r := range cols {
		switch r {
		case ',':
			out = append(out, trimSpace(cur))
			cur = ""
		case '\n', '\t':
		default:
			cur += string(r)
		}
	}
	if trimSpace(cur) != "" {
		out = append(out, trimSpace(cur))
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}

func (s *Store) GetColony(userID, colonyID string) (repository.Colony, error) {
	c, err := scanColony(s.pool.QueryRow(ctxb(),
		`SELECT `+colonyCols+` FROM colonies WHERE id = $1`, colonyID))
	if err != nil {
		return repository.Colony{}, err
	}
	var n int
	if err := s.pool.QueryRow(ctxb(),
		`SELECT count(*) FROM colony_members WHERE colony_id=$1 AND user_id=$2`,
		colonyID, userID).Scan(&n); err != nil {
		return repository.Colony{}, mapErr(err)
	}
	if n == 0 {
		return repository.Colony{}, repository.ErrForbidden
	}
	return c, nil
}

func (s *Store) UpdateColony(colony repository.Colony) (repository.Colony, error) {
	return scanColony(s.pool.QueryRow(ctxb(), `UPDATE colonies
		SET name=$1, description=$2, updated_at=now()
		WHERE id=$3 RETURNING `+colonyCols,
		colony.Name, colony.Description, colony.ID))
}

func (s *Store) DeleteColony(userID, colonyID string) error {
	tag, err := s.pool.Exec(ctxb(),
		`DELETE FROM colonies WHERE id=$1 AND owner_user_id=$2`, colonyID, userID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrForbidden
	}
	return nil
}

func (s *Store) FindColonyByInviteCode(inviteCode string) (repository.Colony, error) {
	return scanColony(s.pool.QueryRow(ctxb(),
		`SELECT `+colonyCols+` FROM colonies WHERE invite_code = $1`, inviteCode))
}

func (s *Store) JoinColony(userID, colonyID, inviteCode string) (repository.Colony, error) {
	c, err := scanColony(s.pool.QueryRow(ctxb(),
		`SELECT `+colonyCols+` FROM colonies WHERE id = $1`, colonyID))
	if err != nil {
		return repository.Colony{}, err
	}
	if c.InviteCode != inviteCode {
		return repository.Colony{}, repository.ErrForbidden
	}
	if _, err := s.pool.Exec(ctxb(),
		`INSERT INTO colony_members (colony_id, user_id, role, joined_at) VALUES ($1,$2,'MEMBER',$3)
		 ON CONFLICT (colony_id, user_id) DO NOTHING`,
		colonyID, userID, time.Now().UTC()); err != nil {
		return repository.Colony{}, mapErr(err)
	}
	return c, nil
}

func (s *Store) LeaveColony(userID, colonyID string) error {
	_, err := s.pool.Exec(ctxb(),
		`DELETE FROM colony_members WHERE colony_id=$1 AND user_id=$2`, colonyID, userID)
	return mapErr(err)
}

func (s *Store) ListColonyMembers(colonyID string) ([]repository.ColonyMember, error) {
	// LEFT JOIN, so a member without a user record still appears; their id
	// stands in for the name rather than the row vanishing.
	rows, err := s.pool.Query(ctxb(),
		`SELECT m.colony_id, m.user_id, COALESCE(NULLIF(u.display_name, ''), m.user_id), m.role, m.joined_at
		 FROM colony_members m LEFT JOIN users u ON u.id = m.user_id
		 WHERE m.colony_id=$1 ORDER BY m.joined_at, m.user_id`, colonyID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.ColonyMember, 0)
	for rows.Next() {
		var m repository.ColonyMember
		if err := rows.Scan(&m.ColonyID, &m.UserID, &m.DisplayName, &m.Role, &m.JoinedAt); err != nil {
			return nil, mapErr(err)
		}
		items = append(items, m)
	}
	return items, mapErr(rows.Err())
}

const sharedCols = `id, colony_id, source_type, source_id, created_by, title_snapshot, created_at`

func scanShared(row pgx.Row) (repository.SharedItem, error) {
	var i repository.SharedItem
	err := row.Scan(&i.ID, &i.ColonyID, &i.SourceType, &i.SourceID, &i.CreatedBy,
		&i.TitleSnapshot, &i.CreatedAt)
	return i, mapErr(err)
}

func (s *Store) CreateSharedItem(item repository.SharedItem) (repository.SharedItem, error) {
	if item.ID == "" {
		item.ID = repository.NewID()
	}
	// The unique constraint on (colony_id, source_type, source_id) is what makes
	// sharing the same thing twice a conflict rather than a duplicate row.
	i, err := scanShared(s.pool.QueryRow(ctxb(), `INSERT INTO shared_items (`+sharedCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+sharedCols,
		item.ID, item.ColonyID, item.SourceType, item.SourceID, item.CreatedBy,
		item.TitleSnapshot, time.Now().UTC()))
	if errors.Is(err, repository.ErrDuplicate) {
		return repository.SharedItem{}, repository.ErrDuplicate
	}
	return i, err
}

func (s *Store) ListSharedItems(colonyID string) ([]repository.SharedItem, error) {
	rows, err := s.pool.Query(ctxb(), `SELECT `+sharedCols+` FROM shared_items
		WHERE colony_id=$1 ORDER BY created_at DESC, id`, colonyID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	items := make([]repository.SharedItem, 0)
	for rows.Next() {
		i, err := scanShared(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, mapErr(rows.Err())
}

func (s *Store) DeleteSharedItem(userID, colonyID, sharedItemID string) error {
	tag, err := s.pool.Exec(ctxb(),
		`DELETE FROM shared_items WHERE id=$1 AND colony_id=$2`, sharedItemID, colonyID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}
