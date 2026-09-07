package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// TestClaimAcquireForeignKey verifies F-03 dedicated claim connection:
// Acquire referencing an absent project through its dedicated Conn
// must return ErrForeignKeyViolation, leave no orphan row (rollback),
// and enforce foreign_keys on that connection.
func TestClaimAcquireForeignKey(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "claim-fk.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	missingProject := "proj-does-not-exist-fk"

	// Verify store-level pragma is enabled (DSN + Open).
	var fk int
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("pragma foreign_keys query: %v", err)
	}
	if fk != 1 {
		t.Fatalf("expected PRAGMA foreign_keys=1 on store conn, got %d", fk)
	}

	claim := PathClaim{
		ProjectID:        missingProject,
		LogicalPath:      "src/fk.ts",
		Mode:             "shared",
		OwnerExecutionID: "exec-fk",
		OwnerStepID:      "step-fk",
	}

	ok, conflict, err := s.PathClaims().Acquire(ctx, claim)
	if !errors.Is(err, ErrForeignKeyViolation) {
		t.Fatalf("expected ErrForeignKeyViolation, got ok=%v conflict=%v err=%v", ok, conflict, err)
	}
	if ok {
		t.Fatalf("expected ok=false on FK violation, got true")
	}
	if conflict != nil {
		t.Fatalf("expected conflict=nil on FK violation, got %+v", conflict)
	}

	// Rollback: no orphan row must remain.
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM path_claims WHERE project_id = ?", missingProject).Scan(&count); err != nil {
		t.Fatalf("count path_claims: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 orphan rows after FK rollback, got %d", count)
	}

	// Dedicated connection also had foreign_keys=1 — verified transitively by
	// the FK enforcement above, and explicitly by re-checking the store pragma
	// remains 1 after the rollback (dedicated Conn closed, store Conn unaffected).
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("pragma foreign_keys after acquire: %v", err)
	}
	if fk != 1 {
		t.Fatalf("expected PRAGMA foreign_keys=1 after failed Acquire, got %d", fk)
	}

	// Store remains usable after rollback: create a real project and acquire succeeds.
	if err := s.Projects().Create(ctx, missingProject, "/tmp/proj-fk"); err != nil {
		t.Fatalf("create project after FK failure: %v", err)
	}
	ok, conflict, err = s.PathClaims().Acquire(ctx, claim)
	if err != nil {
		t.Fatalf("acquire after project created: %v", err)
	}
	if !ok || conflict != nil {
		t.Fatalf("expected acquire ok after project exists, got ok=%v conflict=%v err=%v", ok, conflict, err)
	}
}
