package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigExternalPaths(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("load default: %v", err)
	}
	if cfg.ExternalPaths == nil {
		// allow nil but should be empty slice behavior
	} else if len(cfg.ExternalPaths) != 0 {
		t.Fatalf("expected empty external_paths default, got %v", cfg.ExternalPaths)
	}

	// write config with external_paths
	content := "version: 2\nexternal_paths:\n  - cache/\n  - \".tmp/cache\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".haro", "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg2, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("load with external: %v", err)
	}
	if len(cfg2.ExternalPaths) != 2 {
		t.Fatalf("expected 2 external_paths, got %v", cfg2.ExternalPaths)
	}
	if cfg2.ExternalPaths[0] != "cache/" || cfg2.ExternalPaths[1] != ".tmp/cache" {
		t.Fatalf("unexpected external_paths %v", cfg2.ExternalPaths)
	}

	// empty default preserves
	dir2 := t.TempDir()
	if err := Init(dir2); err != nil {
		t.Fatalf("init2: %v", err)
	}
	cfg3, _ := LoadConfig(dir2)
	if len(cfg3.ExternalPaths) != 0 {
		t.Fatalf("second init should be empty, got %v", cfg3.ExternalPaths)
	}

	// LoadConfig should not fail if file missing (return default)
	dir3 := t.TempDir()
	cfg4, err := LoadConfig(dir3)
	if err != nil {
		t.Fatalf("load missing should return default: %v", err)
	}
	if len(cfg4.ExternalPaths) != 0 {
		t.Fatalf("missing file should give empty, got %v", cfg4.ExternalPaths)
	}
}
