package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitIdempotent(t *testing.T) {
	root := t.TempDir()

	// First init
	if err := Init(root); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	// Verify structure
	for _, p := range []string{".haro/config.yaml", ".haro/workflows", ".haro/skills", ".haro/artifacts", ".haro/docs"} {
		full := filepath.Join(root, p)
		if _, err := os.Stat(full); err != nil {
			t.Fatalf("expected %s to exist: %v", p, err)
		}
	}
	// Read bytes
	cfgPath := filepath.Join(root, ".haro", "config.yaml")
	b1, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if len(b1) == 0 {
		t.Fatalf("config.yaml empty")
	}

	// Second init — should be idempotent, bytes unchanged, exit 0
	if err := Init(root); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	b2, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config second: %v", err)
	}
	if string(b1) != string(b2) {
		t.Fatalf("config bytes changed on rerun: first %q, second %q", string(b1), string(b2))
	}

	// Triangulation: custom config should be preserved
	custom := []byte("custom: preserved\n")
	if err := os.WriteFile(cfgPath, custom, 0o600); err != nil {
		t.Fatalf("write custom: %v", err)
	}
	if err := Init(root); err != nil {
		t.Fatalf("third Init with custom: %v", err)
	}
	b3, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read custom: %v", err)
	}
	if string(b3) != string(custom) {
		t.Fatalf("custom config overwritten: got %q, want %q", string(b3), string(custom))
	}

	// Verify other dirs still exist and were not removed
	for _, p := range []string{".haro/workflows", ".haro/skills", ".haro/artifacts", ".haro/docs"} {
		full := filepath.Join(root, p)
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			t.Fatalf("expected dir %s after third init: %v", p, err)
		}
	}
	// Create a file inside workflows and ensure init does not delete it
	extra := filepath.Join(root, ".haro", "workflows", "keep.txt")
	if err := os.WriteFile(extra, []byte("keep"), 0o600); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	if err := Init(root); err != nil {
		t.Fatalf("fourth Init: %v", err)
	}
	if _, err := os.Stat(extra); err != nil {
		t.Fatalf("init deleted existing file: %v", err)
	}
}

func TestInit_NestedRoot(t *testing.T) {
	root := t.TempDir()
	// root with existing .haro partially missing config.yaml
	if err := os.MkdirAll(filepath.Join(root, ".haro", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := Init(root); err != nil {
		t.Fatalf("Init with partial: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".haro", "config.yaml")); err != nil {
		t.Fatalf("config.yaml should be created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".haro", "artifacts")); err != nil {
		t.Fatalf("artifacts should be created: %v", err)
	}
}
