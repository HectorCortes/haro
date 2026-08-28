package store

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteRoundtrip(t *testing.T) {
	// Open shared in-memory database via modernc.org/sqlite.
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Use a single connection: temporary tables cannot cross pooled connections.
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	defer conn.Close()

	// Enable foreign keys and assert it is 1.
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("PRAGMA foreign_keys=ON: %v", err)
	}
	var fk int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys query: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, want 1", fk)
	}

	// Execute journal_mode WAL without asserting its value.
	// In-memory databases return "memory", not "wal".
	var journalMode string
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode=WAL: %v", err)
	}
	// Only assert no error, never that journalMode == "wal".

	// Create temp table, insert and select.
	if _, err := conn.ExecContext(ctx, "CREATE TEMP TABLE t (id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatalf("CREATE TEMP TABLE: %v", err)
	}
	const want = "hello"
	if _, err := conn.ExecContext(ctx, "INSERT INTO t(v) VALUES (?)", want); err != nil {
		t.Fatalf("INSERT: %v", err)
	}
	var got string
	if err := conn.QueryRowContext(ctx, "SELECT v FROM t WHERE id=1").Scan(&got); err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	if got != want {
		t.Fatalf("selected = %q, want %q", got, want)
	}
}
