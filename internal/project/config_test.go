package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHarnessConfigKnownFields covers the F-04 strict-optional configuration
// scenario: absent harnesses and arbitrary harness keys must pass, while an
// unknown record field and an unknown top-level key must fail. Defaults and
// the 1-300 timeout range are validated here too.
func TestHarnessConfigKnownFields(t *testing.T) {
	writeConfig := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".haro"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".haro", "config.yaml"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	t.Run("absent harnesses passes", func(t *testing.T) {
		dir := writeConfig(t, "version: 2\nexternal_paths: []\n")
		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("load absent harnesses: %v", err)
		}
		if len(cfg.Harnesses) != 0 {
			t.Fatalf("expected no harnesses, got %v", cfg.Harnesses)
		}
	})

	t.Run("arbitrary harness keys pass", func(t *testing.T) {
		dir := writeConfig(t, `version: 2
harnesses:
  opencode:
    binary: /usr/local/bin/oc
    env:
      OC_API_KEY: from-env
    timeout_seconds: 45
  weird-name-123:
    binary: oc2
`)
		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("load arbitrary keys: %v", err)
		}
		if len(cfg.Harnesses) != 2 {
			t.Fatalf("expected 2 harnesses, got %d", len(cfg.Harnesses))
		}
		oc := cfg.Harnesses["opencode"]
		if oc.Binary != "/usr/local/bin/oc" {
			t.Fatalf("binary = %q", oc.Binary)
		}
		if oc.Env["OC_API_KEY"] != "from-env" {
			t.Fatalf("env overlay lost: %v", oc.Env)
		}
		if oc.TimeoutSeconds != 45 {
			t.Fatalf("timeout = %d, want 45", oc.TimeoutSeconds)
		}
		// Nil enabled defaults to true; zero timeout defaults to 300.
		weird := cfg.Harnesses["weird-name-123"]
		if weird.Enabled == nil || !*weird.Enabled {
			t.Fatalf("nil enabled must default to true, got %v", weird.Enabled)
		}
		if weird.TimeoutSeconds != 300 {
			t.Fatalf("zero timeout must default to 300, got %d", weird.TimeoutSeconds)
		}
	})

	t.Run("unknown record field rejected", func(t *testing.T) {
		dir := writeConfig(t, `version: 2
harnesses:
  opencode:
    binary: oc
    bogus_field: 1
`)
		_, err := LoadConfig(dir)
		if err == nil {
			t.Fatalf("unknown record field must fail")
		}
		if !strings.Contains(err.Error(), "bogus_field") {
			t.Fatalf("error should name the unknown field, got %q", err.Error())
		}
	})

	t.Run("unknown top-level key rejected", func(t *testing.T) {
		dir := writeConfig(t, "version: 2\nbogus_top: 1\n")
		if _, err := LoadConfig(dir); err == nil {
			t.Fatalf("unknown top-level key must fail")
		}
	})

	t.Run("timeout range enforced", func(t *testing.T) {
		for _, tc := range []struct {
			seconds int
			wantErr bool
		}{
			{1, false},
			{300, false},
			{301, true},
			{0, false}, // defaults to 300
			{-5, true},
		} {
			dir := writeConfig(t, fmt.Sprintf("version: 2\nharnesses:\n  oc:\n    binary: oc\n    timeout_seconds: %d\n", tc.seconds))
			cfg, err := LoadConfig(dir)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("timeout %d must fail", tc.seconds)
				}
				continue
			}
			if err != nil {
				t.Fatalf("timeout %d must pass: %v", tc.seconds, err)
			}
			if got := cfg.Harnesses["oc"].TimeoutSeconds; got < 1 || got > 300 {
				t.Fatalf("timeout %d normalized to %d, out of range", tc.seconds, got)
			}
		}
	})

	t.Run("disabled explicit false preserved", func(t *testing.T) {
		dir := writeConfig(t, `version: 2
harnesses:
  oc:
    binary: oc
    enabled: false
`)
		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("load disabled: %v", err)
		}
		if cfg.Harnesses["oc"].Enabled == nil || *cfg.Harnesses["oc"].Enabled {
			t.Fatalf("explicit false must be preserved")
		}
	})
}

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
