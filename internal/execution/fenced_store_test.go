package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/store"
)

func TestFencedStoreRejectsWritesAfterLeaseChanges(t *testing.T) {
	ctx := context.Background()
	s := store.NewFakeStore()
	if err := s.Projects().Create(ctx, "project", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := s.Executions().Create(ctx, &store.Execution{ID: "execution", ProjectID: "project", Status: "running", WorkspaceMode: "shared"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Steps().Create(ctx, &store.ExecutionStep{ExecutionID: "execution", StepID: "step", Type: "command", Status: "running", WorkspaceMode: "shared"}); err != nil {
		t.Fatal(err)
	}

	token, err := s.Leases().Acquire(ctx, "execution", "step", "holder-a")
	if err != nil {
		t.Fatal(err)
	}
	fenced := NewFencedStore(s, "execution", "step", "holder-a", token)
	if err := fenced.Steps().UpdateStatus(ctx, "execution", "step", "completed"); err != nil {
		t.Fatalf("current holder write: %v", err)
	}

	if err := s.Leases().Release(ctx, "execution", "step", "holder-a", token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Leases().Acquire(ctx, "execution", "step", "holder-b"); err != nil {
		t.Fatal(err)
	}

	err = fenced.Steps().UpdateStatus(ctx, "execution", "step", "failed")
	if !errors.Is(err, ErrStaleLease) {
		t.Fatalf("stale write error = %v, want ErrStaleLease", err)
	}
	step, err := s.Steps().Get(ctx, "execution", "step")
	if err != nil {
		t.Fatal(err)
	}
	if step.Status != "completed" {
		t.Fatalf("stale write changed status to %q", step.Status)
	}

	err = fenced.WithTx(ctx, func(tx store.Store) error {
		return tx.Steps().UpdateStatus(ctx, "execution", "step", "failed")
	})
	if !errors.Is(err, ErrStaleLease) {
		t.Fatalf("stale transaction error = %v, want ErrStaleLease", err)
	}
}

func TestFencedStoreRejectsExpiredLease(t *testing.T) {
	ctx := context.Background()
	s := store.NewFakeStore()
	if _, err := s.Leases().Acquire(ctx, "execution", "step", "holder"); err != nil {
		t.Fatal(err)
	}
	lease, err := s.Leases().Get(ctx, "execution", "step")
	if err != nil {
		t.Fatal(err)
	}
	lease.ExpiresAt = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	// FakeStore returns copies, so force an expired lease through its
	// transaction surface without exposing implementation details.
	if err := s.WithTx(ctx, func(tx store.Store) error {
		// A release makes the persisted lease expired and keeps the token.
		return tx.Leases().Release(ctx, "execution", "step", "holder", lease.FencingToken)
	}); err != nil {
		t.Fatal(err)
	}

	fenced := NewFencedStore(s, "execution", "step", "holder", lease.FencingToken)
	err = fenced.WithTx(ctx, func(store.Store) error { return nil })
	if !errors.Is(err, ErrStaleLease) {
		t.Fatalf("expired lease error = %v, want ErrStaleLease", err)
	}
}
