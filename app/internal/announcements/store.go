package announcements

import (
	"database/sql"

	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	s := &SQLiteStore{db: db}
	if err := s.ensureSchema(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) ensureSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS announcements (
			id INTEGER PRIMARY KEY,
			scope TEXT NOT NULL,
			user_id INTEGER,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			link TEXT,
			priority INTEGER NOT NULL DEFAULT 0,
			is_active INTEGER NOT NULL DEFAULT 1,
			starts_at INTEGER,
			ends_at INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS announcements_scope_idx ON announcements (scope, is_active, priority, created_at);
		CREATE INDEX IF NOT EXISTS announcements_user_idx ON announcements (user_id, is_active, priority, created_at);
		CREATE TABLE IF NOT EXISTS announcement_dismissals (
			user_id INTEGER NOT NULL,
			announcement_id INTEGER NOT NULL,
			dismissed_at INTEGER NOT NULL,
			PRIMARY KEY (user_id, announcement_id)
		);
	`)
	return err
}

func (s *SQLiteStore) Create(input UpsertInput) (int64, error) {
	id := snowflake.Next()
	now := utils.NowNano()
	_, err := s.db.Exec(`insert into announcements (id, scope, user_id, title, body, link, priority, is_active, starts_at, ends_at, created_at, updated_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		input.Scope,
		nullableUserID(input.UserID),
		input.Title,
		input.Body,
		nullableString(input.Link),
		input.Priority,
		boolToInt(input.IsActive),
		nullableUint64(input.StartsAt),
		nullableUint64(input.EndsAt),
		now,
		now,
	)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *SQLiteStore) Update(id int64, input UpsertInput) error {
	_, err := s.db.Exec(`update announcements set scope = ?, user_id = ?, title = ?, body = ?, link = ?, priority = ?, is_active = ?, starts_at = ?, ends_at = ?, updated_at = ? where id = ?`,
		input.Scope,
		nullableUserID(input.UserID),
		input.Title,
		input.Body,
		nullableString(input.Link),
		input.Priority,
		boolToInt(input.IsActive),
		nullableUint64(input.StartsAt),
		nullableUint64(input.EndsAt),
		utils.NowNano(),
		id,
	)
	return err
}

func (s *SQLiteStore) List(scope string, userID *types.UserID, active *bool) ([]Announcement, error) {
	query := `select id, scope, user_id, title, body, link, priority, is_active, starts_at, ends_at, created_at, updated_at from announcements where 1=1`
	args := make([]interface{}, 0, 3)
	if scope != "" {
		query += ` and scope = ?`
		args = append(args, scope)
	}
	if userID != nil {
		query += ` and user_id = ?`
		args = append(args, int64(*userID))
	}
	if active != nil {
		query += ` and is_active = ?`
		args = append(args, boolToInt(*active))
	}
	query += ` order by priority desc, created_at desc`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanAnnouncements(rows)
}

func (s *SQLiteStore) ListActiveForUser(userID types.UserID, now uint64) ([]Announcement, error) {
	rows, err := s.db.Query(`
		select a.id, a.scope, a.user_id, a.title, a.body, a.link, a.priority, a.is_active, a.starts_at, a.ends_at, a.created_at, a.updated_at
		from announcements a
		where a.is_active = 1
		  and (a.scope = ? or (a.scope = ? and a.user_id = ?))
		  and (a.starts_at is null or a.starts_at <= ?)
		  and (a.ends_at is null or a.ends_at >= ?)
		order by case when a.scope = ? then 0 else 1 end, a.priority desc, a.created_at desc
	`, ScopeGlobal, ScopeUser, userID, now, now, ScopeUser)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanAnnouncements(rows)
}

func (s *SQLiteStore) Delete(id int64) error {
	if _, err := s.db.Exec(`delete from announcement_dismissals where announcement_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`delete from announcements where id = ?`, id)
	return err
}

func scanAnnouncements(rows *sql.Rows) ([]Announcement, error) {
	items := make([]Announcement, 0)
	for rows.Next() {
		var rec Announcement
		var uid sql.NullInt64
		var link sql.NullString
		var startsAt sql.NullInt64
		var endsAt sql.NullInt64
		var active int
		if err := rows.Scan(&rec.ID, &rec.Scope, &uid, &rec.Title, &rec.Body, &link, &rec.Priority, &active, &startsAt, &endsAt, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		if uid.Valid {
			v := types.UserID(uid.Int64)
			rec.UserID = &v
		}
		if link.Valid {
			v := link.String
			rec.Link = &v
		}
		if startsAt.Valid {
			v := uint64(startsAt.Int64)
			rec.StartsAt = &v
		}
		if endsAt.Valid {
			v := uint64(endsAt.Int64)
			rec.EndsAt = &v
		}
		rec.IsActive = active == 1
		items = append(items, rec)
	}
	return items, rows.Err()
}

func nullableString(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func nullableUserID(value *types.UserID) interface{} {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func nullableUint64(value *uint64) interface{} {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
