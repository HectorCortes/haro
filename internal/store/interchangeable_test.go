package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/store/contract"
)

func TestStoreInterchangeability(t *testing.T) {
	t.Run("fake", func(t *testing.T) {
		factory := func(t *testing.T) store.Store {
			t.Helper()
			return store.NewFakeStore()
		}
		contract.Run(t, factory)
	})
	t.Run("sqlite", func(t *testing.T) {
		factory := func(t *testing.T) store.Store {
			t.Helper()
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "store.db")
			s, err := store.Open(context.Background(), dbPath)
			if err != nil {
				t.Fatalf("Open sqlite: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })
			return s
		}
		contract.Run(t, factory)
	})
}
