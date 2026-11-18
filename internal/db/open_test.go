package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestPragmas(t *testing.T) {
	d := openTest(t)
	handles := map[string]*sql.DB{"writer": d.Writer, "reader": d.Reader}
	want := []struct{ pragma, val string }{
		{"foreign_keys", "1"},
		{"journal_mode", "wal"},
		{"busy_timeout", "5000"},
	}
	for name, h := range handles {
		for _, tc := range want {
			t.Run(name+"/"+tc.pragma, func(t *testing.T) {
				var got string
				if err := h.QueryRow("PRAGMA " + tc.pragma).Scan(&got); err != nil {
					t.Fatal(err)
				}
				if got != tc.val {
					t.Fatalf("PRAGMA %s = %q, want %q", tc.pragma, got, tc.val)
				}
			})
		}
	}
}

func TestReaderCannotWrite(t *testing.T) {
	d := openTest(t)
	if _, err := d.Writer.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Reader.Exec(`INSERT INTO t DEFAULT VALUES`); err == nil {
		t.Fatal("write through reader succeeded, want error")
	}
	if _, err := d.Writer.Exec(`INSERT INTO t DEFAULT VALUES`); err != nil {
		t.Fatalf("writer insert: %v", err)
	}
	var n int
	if err := d.Reader.QueryRow(`SELECT count(*) FROM t`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("reader sees n=%d err=%v, want 1", n, err)
	}
}

func TestWriterSingleConn(t *testing.T) {
	d := openTest(t)
	if got := d.Writer.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("writer MaxOpenConnections = %d, want 1", got)
	}
}
