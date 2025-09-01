package dedupe

import (
	"context"
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// SQLite persists dedupe state in an on-disk database.
type SQLite struct { db *sql.DB }

// Record contains the stored metadata.
type Record struct {
	SHA256    string
	S3Key     string
	Size      int64
	MIME      string
	FirstSeen time.Time
	LastSeen  time.Time
	Count     int64
}

// OpenSQLite opens/initializes the database at path.
func OpenSQLite(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil { return nil, err }
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil { return nil, err }
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS seen (
  sha256 TEXT PRIMARY KEY,
  s3_key TEXT,
  size INTEGER,
  mime TEXT,
  first_seen TEXT,
  last_seen TEXT,
  count INTEGER DEFAULT 1
);`); err != nil { return nil, err }
	return &SQLite{db: db}, nil
}

// Close closes the DB.
func (s *SQLite) Close() error { return s.db.Close() }

// Seen checks whether sha exists.
func (s *SQLite) Seen(ctx context.Context, sha string) (bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT 1 FROM seen WHERE sha256=?`, sha)
	var one int
	switch err := row.Scan(&one); err {
	case nil:
		return true, nil
	case sql.ErrNoRows:
		return false, nil
	default:
		return false, err
	}
}

// Mark inserts/updates a record.
func (s *SQLite) Mark(ctx context.Context, rec Record) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `INSERT INTO seen(sha256, s3_key, size, mime, first_seen, last_seen, count)
VALUES(?,?,?,?,?, ?, 1)
ON CONFLICT(sha256) DO UPDATE SET last_seen=excluded.last_seen, count=count+1;`,
		rec.SHA256, rec.S3Key, rec.Size, rec.MIME, now, now)
	return err
}
