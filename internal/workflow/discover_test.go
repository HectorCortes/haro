package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	// create .haro/workflows/{alpha,beta,gamma}/workflow.yaml
	names := []string{"gamma", "alpha", "beta"}
	for _, n := range names {
		dir := filepath.Join(root, ".haro", "workflows", n)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", n, err)
		}
		yaml := "version: 2\nname: " + n + "\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"
		if err := os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(yaml), 0o600); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}
	// add a directory without workflow.yaml — should be ignored
	if err := os.MkdirAll(filepath.Join(root, ".haro", "workflows", "empty"), 0o755); err != nil {
		t.Fatalf("mkdir empty: %v", err)
	}
	// add file at root's .haro/workflows should be ignored
	if err := os.WriteFile(filepath.Join(root, ".haro", "workflows", "not-a-dir.yaml"), []byte("version: 2\nname: x\nsteps: []\n"), 0o600); err != nil {
		t.Fatalf("write stray file: %v", err)
	}

	results, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("Discover len = %d, want 3", len(results))
	}
	// must be sorted deterministically by directory name
	wantOrder := []string{"alpha", "beta", "gamma"}
	for i, want := range wantOrder {
		if results[i].Name != want {
			t.Fatalf("results[%d].Name = %q, want %q", i, results[i].Name, want)
		}
		if results[i].Workflow == nil || results[i].Workflow.Name != want {
			t.Fatalf("results[%d] workflow name = %v, want %q", i, results[i].Workflow, want)
		}
		if filepath.Base(filepath.Dir(results[i].Path)) != want {
			t.Fatalf("results[%d].Path = %q, want dir %q", i, results[i].Path, want)
		}
	}

	// triangulation: empty workflows directory → empty slice, no error
	root2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root2, ".haro", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	results2, err := Discover(root2)
	if err != nil {
		t.Fatalf("Discover empty: %v", err)
	}
	if len(results2) != 0 {
		t.Fatalf("Discover empty len = %d, want 0", len(results2))
	}

	// missing .haro/workflows → empty slice, no error (init may not have run)
	root3 := t.TempDir()
	results3, err := Discover(root3)
	if err != nil {
		t.Fatalf("Discover missing: %v", err)
	}
	if len(results3) != 0 {
		t.Fatalf("Discover missing len = %d, want 0", len(results3))
	}
}

func TestDiscover_InvalidYAML(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".haro", "workflows", "bad")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte("::: not yaml :::\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Discover(root)
	if err == nil {
		t.Fatalf("expected error for invalid YAML, got nil")
	}
}
