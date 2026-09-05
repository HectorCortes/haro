package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAcquire(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "claims.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func(){ _ = s.Close() }()

	// Verify DDL exists and has NOT NULL, FK, partial index
	rows, err := s.QueryForTest(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='path_claims'")
	if err != nil {
		t.Fatalf("query master: %v", err)
	}
	var ddl string
	if rows.Next() {
		_ = rows.Scan(&ddl)
	}
	_ = rows.Close()
	if ddl == "" {
		t.Fatalf("path_claims table missing ddl")
	}
	// check NOT NULL and FK in ddl
	if !contains(ddl, "project_id") || !contains(ddl, "NOT NULL") {
		t.Fatalf("ddl missing NOT NULL/project_id: %s", ddl)
	}
	if !contains(ddl, "REFERENCES projects") {
		t.Fatalf("ddl missing FK REFERENCES projects: %s", ddl)
	}
	rows2, err := s.QueryForTest(ctx, "SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_path_claims_active'")
	if err != nil {
		t.Fatalf("query index: %v", err)
	}
	var idxSQL string
	if rows2.Next() {
		_ = rows2.Scan(&idxSQL)
	}
	_ = rows2.Close()
	if idxSQL == "" {
		t.Fatalf("idx_path_claims_active missing")
	}
	if !contains(idxSQL, "WHERE released_at IS NULL") {
		t.Fatalf("partial index missing WHERE released_at IS NULL: %s", idxSQL)
	}

	// Need project
	projectID := "proj1"
	_ = s.Projects().Create(ctx, projectID, "/tmp/proj1")
	// Also create via direct insert if FK requires project row - ensure exists
	// Test atomic acquire concurrency: incompatible owners share same logical path
	// Use file-backed DB with shared cache - Open already uses file: scheme

	// Clean any existing claims
	_, _ = s.exec(ctx, "DELETE FROM path_claims")

	claimA := PathClaim{
		ProjectID:        projectID,
		LogicalPath:      "src",
		Mode:             "shared",
		OwnerExecutionID: "execA",
		OwnerStepID:      "step1",
	}
	claimB := PathClaim{
		ProjectID:        projectID,
		LogicalPath:      "src/foo.ts",
		Mode:             "shared",
		OwnerExecutionID: "execB",
		OwnerStepID:      "step2",
	}

	// Single acquire should succeed
	ok, conflict, err := s.PathClaims().Acquire(ctx, claimA)
	if err != nil {
		t.Fatalf("acquire A: %v", err)
	}
	if !ok || conflict != nil {
		t.Fatalf("expected A acquire true, got ok=%v conflict=%v", ok, conflict)
	}

	// Overlapping child should block (shared/shared)
	ok, conflict, err = s.PathClaims().Acquire(ctx, claimB)
	if err != nil {
		t.Fatalf("acquire B: %v", err)
	}
	if ok {
		t.Fatalf("expected B blocked, got ok")
	}
	if conflict == nil || conflict.OwnerExecutionID != "execA" {
		t.Fatalf("expected conflict owner execA, got %v", conflict)
	}

	// Release with wrong owner should fail
	err = s.PathClaims().Release(ctx, projectID, "src", "execB", "step1")
	if err == nil {
		t.Fatalf("expected wrong-owner release error")
	}
	// Correct release should succeed
	if err := s.PathClaims().Release(ctx, projectID, "src", "execA", "step1"); err != nil {
		t.Fatalf("release correct owner: %v", err)
	}
	// After release, B should succeed
	ok, conflict, err = s.PathClaims().Acquire(ctx, claimB)
	if err != nil {
		t.Fatalf("acquire B after release: %v", err)
	}
	if !ok {
		t.Fatalf("expected B after release ok, conflict %v", conflict)
	}

	// ListActive should show B
	active, err := s.PathClaims().ListActive(ctx, projectID)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 || active[0].LogicalPath != "src/foo.ts" {
		t.Fatalf("list active unexpected: %+v", active)
	}

	// Isolated/isolated should allow coexistence: clean
	_, _ = s.exec(ctx, "DELETE FROM path_claims")
	claimIso1 := PathClaim{ProjectID: projectID, LogicalPath: "src", Mode: "isolated:exec1", OwnerExecutionID: "exec1", OwnerStepID: "s1"}
	claimIso2 := PathClaim{ProjectID: projectID, LogicalPath: "src", Mode: "isolated:exec2", OwnerExecutionID: "exec2", OwnerStepID: "s1"}
	ok, _, err = s.PathClaims().Acquire(ctx, claimIso1)
	if err != nil || !ok {
		t.Fatalf("iso1 acquire failed: %v ok %v", err, ok)
	}
	ok, _, err = s.PathClaims().Acquire(ctx, claimIso2)
	if err != nil {
		t.Fatalf("iso2 acquire err: %v", err)
	}
	if !ok {
		t.Fatalf("isolated/isolated should coexist, but blocked")
	}
}

func TestAcquireConcurrency(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "claims.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func(){ _ = s.Close() }()
	projectID := "proj-conc"
	_ = s.Projects().Create(ctx, projectID, "/tmp/proj-conc")
	_, _ = s.exec(ctx, "DELETE FROM path_claims WHERE project_id=?", projectID)

	const workers = 8
	var wg sync.WaitGroup
	results := make([]bool, workers)
	conflicts := make([]*PathClaim, workers)
	errs := make([]error, workers)
	barrier := make(chan struct{})
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			<-barrier
			// each worker uses its own store connection to same file to test serialization
			ss, err := Open(ctx, dbPath)
			if err != nil {
				errs[idx] = err
				return
			}
			defer func(){ _ = ss.Close() }()
			claim := PathClaim{
				ProjectID:        projectID,
				LogicalPath:      "src/a",
				Mode:             "shared",
				OwnerExecutionID: filepath.Base(suffix(idx)),
				OwnerStepID:      "step1",
			}
			// use unique owner per worker
			claim.OwnerExecutionID = "exec" + string(rune('A'+idx))
			ok, conf, err := ss.PathClaims().Acquire(ctx, claim)
			results[idx] = ok
			conflicts[idx] = conf
			errs[idx] = err
		}(i)
	}
	close(barrier)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d err: %v", i, err)
		}
	}
	winners := 0
	var winnerOwner string
	for i, ok := range results {
		if ok {
			winners++
			winnerOwner = "exec" + string(rune('A'+i))
		} else {
			if conflicts[i] == nil {
				t.Fatalf("loser %d missing conflict", i)
			}
		}
	}
	if winners != 1 {
		t.Fatalf("expected exactly 1 winner, got %d results %v", winners, results)
	}
	// all losers should report same winner
	for i, ok := range results {
		if !ok {
			if conflicts[i].OwnerExecutionID != winnerOwner {
				t.Fatalf("loser %d conflict owner %q != winner %q", i, conflicts[i].OwnerExecutionID, winnerOwner)
			}
		}
	}
	// verify ListActive still 1
	active, err := s.PathClaims().ListActive(ctx, projectID)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active after concurrency, got %d", len(active))
	}
	_ = time.Now()
}

func suffix(i int) string { return string(rune('0' + i)) }

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
