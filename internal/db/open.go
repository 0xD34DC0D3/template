// Package db opens SQLite as one writer plus a read pool. The sqlc-generated
// query code (db.go, models.go, queries.sql.go) lives in the same package so
// callers write db.New(d.Reader) / db.New(d.Writer) with no extra layer.
//
// SQLite allows a single writer at a time. Funnelling every write through one
// connection serialises them in Go instead of in SQLITE_BUSY retries, while
// WAL lets the read pool run concurrently with that writer.
package db

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// DB holds two handles on the same file. Use Writer for anything that
// modifies data (and for migrations), Reader for plain queries.
type DB struct {
	Writer *sql.DB
	Reader *sql.DB
}

// Pragmas are in the DSN, not Exec'd, so every pooled connection gets them.
const pragmas = "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"

// Open opens both handles. The writer is opened first: it creates the file and
// switches it to WAL, which the read-only pool could not do.
func Open(path string) (*DB, error) {
	// _txlock=immediate makes BEGIN take the write lock up front. A deferred
	// transaction that reads and then writes can fail with SQLITE_BUSY on
	// upgrade, and busy_timeout does not help there.
	w, err := open(fmt.Sprintf("file:%s?%s&_txlock=immediate", path, pragmas))
	if err != nil {
		return nil, err
	}
	w.SetMaxOpenConns(1)

	// mode=ro makes a stray write through the read pool fail loudly instead of
	// contending with the writer.
	r, err := open(fmt.Sprintf("file:%s?%s&mode=ro", path, pragmas))
	if err != nil {
		_ = w.Close()
		return nil, err
	}
	return &DB{Writer: w, Reader: r}, nil
}

func open(dsn string) (*sql.DB, error) {
	h, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := h.Ping(); err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return h, nil
}

func (d *DB) Close() error {
	return errors.Join(d.Reader.Close(), d.Writer.Close())
}
