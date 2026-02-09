package kyc

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// Repository provides KYC persistence operations.
type Repository struct {
	db *sql.DB
}

// RequestRecord stores a KYC request row.
type RequestRecord struct {
	ID           int64
	UserID       types.UserID
	DocType      string
	Country      string
	Status       string
	RejectReason *string
	CreatedAt    uint64
	UpdatedAt    uint64
}

// FileRecord stores a KYC file row.
type FileRecord struct {
	ID          int64
	KYCID       int64
	Kind        string
	Filename    string
	ContentType string
	Size        int64
	Path        string
	CreatedAt   uint64
}

// NewRepository creates a KYC repository and ensures schema.
func NewRepository(db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("kyc repository requires db")
	}
	if err := ensureSchema(db); err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

// CreateRequest inserts a KYC request and files in one transaction.
func (r *Repository) CreateRequest(req RequestRecord, files []FileRecord) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		"insert into kyc_requests (id, user_id, doc_type, country, status, reject_reason, created_at, updated_at) values (?, ?, ?, ?, ?, ?, ?, ?)",
		req.ID,
		uint64(req.UserID),
		req.DocType,
		req.Country,
		req.Status,
		req.RejectReason,
		req.CreatedAt,
		req.UpdatedAt,
	); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, file := range files {
		if _, err := tx.Exec(
			"insert into kyc_files (id, kyc_id, kind, filename, content_type, size, path, created_at) values (?, ?, ?, ?, ?, ?, ?, ?)",
			file.ID,
			file.KYCID,
			file.Kind,
			file.Filename,
			file.ContentType,
			file.Size,
			file.Path,
			file.CreatedAt,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// GetRequestByUser returns the latest KYC request for a user.
func (r *Repository) GetRequestByUser(userID types.UserID) (*RequestRecord, []FileRecord, error) {
	row := r.db.QueryRow(
		"select id, user_id, doc_type, country, status, reject_reason, created_at, updated_at from kyc_requests where user_id = ? order by created_at desc limit 1",
		uint64(userID),
	)
	var rec RequestRecord
	if err := row.Scan(&rec.ID, &rec.UserID, &rec.DocType, &rec.Country, &rec.Status, &rec.RejectReason, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	files, err := r.ListFiles(rec.ID)
	if err != nil {
		return nil, nil, err
	}
	return &rec, files, nil
}

// GetRequest returns a KYC request by id.
func (r *Repository) GetRequest(id int64) (*RequestRecord, []FileRecord, error) {
	row := r.db.QueryRow(
		"select id, user_id, doc_type, country, status, reject_reason, created_at, updated_at from kyc_requests where id = ?",
		id,
	)
	var rec RequestRecord
	if err := row.Scan(&rec.ID, &rec.UserID, &rec.DocType, &rec.Country, &rec.Status, &rec.RejectReason, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	files, err := r.ListFiles(rec.ID)
	if err != nil {
		return nil, nil, err
	}
	return &rec, files, nil
}

// ListRequests returns KYC requests with optional status and search filter.
func (r *Repository) ListRequests(status string, limit int, offset int, search string) ([]RequestRecord, error) {
	query := "select id, user_id, doc_type, country, status, reject_reason, created_at, updated_at from kyc_requests"
	args := make([]any, 0)
	clauses := make([]string, 0)
	if status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, status)
	}
	if search != "" {
		clauses = append(clauses, "(lower(doc_type) like ? or lower(country) like ?)")
		pattern := "%" + strings.ToLower(search) + "%"
		args = append(args, pattern, pattern)
	}
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by updated_at desc"
	if limit > 0 {
		query += " limit ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " offset ?"
		args = append(args, offset)
	}
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	res := make([]RequestRecord, 0)
	for rows.Next() {
		var rec RequestRecord
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.DocType, &rec.Country, &rec.Status, &rec.RejectReason, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, rec)
	}
	return res, nil
}

// UpdateStatus updates a KYC request status and reason.
func (r *Repository) UpdateStatus(id int64, status string, reason *string, updatedAt uint64) error {
	res, err := r.db.Exec(
		"update kyc_requests set status = ?, reject_reason = ?, updated_at = ? where id = ?",
		status,
		reason,
		updatedAt,
		id,
	)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("kyc request not found")
	}
	return nil
}

// CountPending returns the total number of pending requests.
func (r *Repository) CountPending() (int, error) {
	row := r.db.QueryRow("select count(1) from kyc_requests where status = ?", "PENDING")
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ListFiles returns files for a KYC request.
func (r *Repository) ListFiles(kycID int64) ([]FileRecord, error) {
	rows, err := r.db.Query(
		"select id, kyc_id, kind, filename, content_type, size, path, created_at from kyc_files where kyc_id = ?",
		kycID,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	res := make([]FileRecord, 0)
	for rows.Next() {
		var file FileRecord
		if err := rows.Scan(&file.ID, &file.KYCID, &file.Kind, &file.Filename, &file.ContentType, &file.Size, &file.Path, &file.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, file)
	}
	return res, nil
}

// ensureSchema creates KYC tables and indexes.
func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`
    create table if not exists kyc_requests (
      id integer primary key,
      user_id integer not null,
      doc_type text not null,
      country text not null,
      status text not null,
      reject_reason text,
      created_at integer not null,
      updated_at integer not null
    );

    create table if not exists kyc_files (
      id integer primary key,
      kyc_id integer not null,
      kind text not null,
      filename text not null,
      content_type text not null,
      size integer not null,
      path text not null,
      created_at integer not null
    );

    create index if not exists kyc_user_idx on kyc_requests (user_id, created_at);
    create index if not exists kyc_status_idx on kyc_requests (status, updated_at);
    create index if not exists kyc_files_kyc_idx on kyc_files (kyc_id);
  `)
	return err
}
