package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestLeaseMonotonicFencing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "leases.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	execID := "exec-1"
	stepID := "step-1"
	holderA := "holder-A"
	holderB := "holder-B"

	// First acquire should give token 1
	tok1, err := s.Leases().Acquire(ctx, execID, stepID, holderA)
	if err != nil {
		t.Fatalf("Acquire 1: %v", err)
	}
	if tok1 != 1 {
		t.Fatalf("tok1 = %d want 1", tok1)
	}
	// Release with correct token
	if err := s.Leases().Release(ctx, execID, stepID, holderA, tok1); err != nil {
		t.Fatalf("Release 1: %v", err)
	}
	// Second acquire should give token 2 (retained MAX+1)
	tok2, err := s.Leases().Acquire(ctx, execID, stepID, holderB)
	if err != nil {
		t.Fatalf("Acquire 2: %v", err)
	}
	if tok2 != 2 {
		t.Fatalf("tok2 = %d want 2", tok2)
	}
	if err := s.Leases().Release(ctx, execID, stepID, holderB, tok2); err != nil {
		t.Fatalf("Release 2: %v", err)
	}
	// Third acquire should give token 3
	tok3, err := s.Leases().Acquire(ctx, execID, stepID, holderA)
	if err != nil {
		t.Fatalf("Acquire 3: %v", err)
	}
	if tok3 != 3 {
		t.Fatalf("tok3 = %d want 3", tok3)
	}
	// Stale renew with old token should fail closed with ErrNotFound
	if err := s.Leases().Renew(ctx, execID, stepID, holderA, tok1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale Renew tok1 expected ErrNotFound, got %v", err)
	}
	if err := s.Leases().Renew(ctx, execID, stepID, holderA, tok2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale Renew tok2 expected ErrNotFound, got %v", err)
	}
	// Correct renew should succeed
	if err := s.Leases().Renew(ctx, execID, stepID, holderA, tok3); err != nil {
		t.Fatalf("Renew tok3: %v", err)
	}
	// Stale release should fail
	if err := s.Leases().Release(ctx, execID, stepID, holderA, tok1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale Release tok1 expected ErrNotFound, got %v", err)
	}
	// Wrong holder
	if err := s.Leases().Release(ctx, execID, stepID, holderB, tok3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong holder Release expected ErrNotFound, got %v", err)
	}
	// Correct release should succeed
	if err := s.Leases().Release(ctx, execID, stepID, holderA, tok3); err != nil {
		t.Fatalf("Release tok3: %v", err)
	}
}

func TestLeaseRenewAndReleaseFailClosed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "leases2.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	// Renew non-existent lease should fail
	if err := s.Leases().Renew(ctx, "no-exec", "no-step", "h", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Renew non-existent expected ErrNotFound got %v", err)
	}
	if err := s.Leases().Release(ctx, "no-exec", "no-step", "h", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Release non-existent expected ErrNotFound got %v", err)
	}
}
