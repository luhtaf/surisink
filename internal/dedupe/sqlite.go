package dedupe

import (
	"context"
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type SQLite struct {
	db *sql.DB
}

type Record struct {
	SHA256    string
	S3Key     string
	Size      int64
	MIME      string
	FirstSeen time.Time
	LastSeen  time.Time
	Count     int64
}

func OpenSQLite(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS seen (
  sha256 TEXT PRIMARY KEY,
  s3_key TEXT,
  size INTEGER,
  mime TEXT,
  first_seen TEXT,
  last_seen TEXT,
  count INTEGER DEFAULT 1
);`); err != nil {
		return nil, err
	}
	return &SQLite{db: db}, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

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

func (s *SQLite) Mark(ctx context.Context, rec Record) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `INSERT INTO seen(sha256, s3_key, size, mime, first_seen, last_seen, count)
VALUES(?,?,?,?,?, ?, 1)
ON CONFLICT(sha256) DO UPDATE SET last_seen=excluded.last_seen, count=count+1;`,
		rec.SHA256, rec.S3Key, rec.Size, rec.MIME, now, now)
	return err
}

func (s *SQLite) GC(ctx context.Context, olderThanDays int) (int64, error) {
	if olderThanDays <= 0 {
		return 0, nil
	}
	threshold := time.Now().AddDate(0, 0, -olderThanDays).UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `DELETE FROM seen WHERE last_seen < ?`, threshold)
	if err != nil {
		return 0, err
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}
