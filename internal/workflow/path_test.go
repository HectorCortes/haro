package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContainment(t *testing.T) {
	root := t.TempDir()
	artifacts := filepath.Join(root, ".haro", "artifacts")
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		t.Fatalf("mkdir artifacts: %v", err)
	}

	// valid relative inside
	if err := ValidateContainedPath(root, "a/b/c.txt"); err != nil {
		t.Fatalf("valid relative should pass, got %v", err)
	}
	if err := ValidateContainedPath(root, "output.txt"); err != nil {
		t.Fatalf("valid single file should pass, got %v", err)
	}
	if err := ValidateContainedPath(artifacts, "snap.txt"); err != nil {
		t.Fatalf("valid artifacts file should pass, got %v", err)
	}

	// absolute rejected
	if err := ValidateContainedPath(root, "/etc/passwd"); err == nil {
		t.Fatalf("absolute path should be rejected")
	}
	if err := ValidateContainedPath(root, "/tmp/out.txt"); err == nil {
		t.Fatalf("absolute path should be rejected")
	}

	// .. rejected
	if err := ValidateContainedPath(root, "../escape.txt"); err == nil {
		t.Fatalf("../ should be rejected")
	}
	if err := ValidateContainedPath(root, "a/../../escape.txt"); err == nil {
		t.Fatalf("traversal should be rejected")
	}
	if err := ValidateContainedPath(root, "a/../b/../../escape"); err == nil {
		t.Fatalf("complex traversal should be rejected")
	}

	// symlink escape: create outside file and symlink inside pointing outside
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	// symlink inside artifacts pointing to outsideFile
	linkPath := filepath.Join(artifacts, "evil")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := ValidateContainedPath(artifacts, "evil"); err == nil {
		t.Fatalf("symlink escape should be rejected")
	}
	// symlink to outside dir
	linkDir := filepath.Join(artifacts, "evilDir")
	if err := os.Symlink(outside, linkDir); err != nil {
		t.Fatalf("symlink dir: %v", err)
	}
	if err := ValidateContainedPath(artifacts, "evilDir/file.txt"); err == nil {
		t.Fatalf("symlink dir escape should be rejected")
	}

	// internal symlink staying inside should pass
	insideTarget := filepath.Join(artifacts, "real.txt")
	if err := os.WriteFile(insideTarget, []byte("inside"), 0o600); err != nil {
		t.Fatalf("write inside: %v", err)
	}
	internalLink := filepath.Join(artifacts, "good")
	if err := os.Symlink(insideTarget, internalLink); err != nil {
		t.Fatalf("internal symlink: %v", err)
	}
	if err := ValidateContainedPath(artifacts, "good"); err != nil {
		t.Fatalf("internal symlink should pass, got %v", err)
	}

	// triangulation: internal symlink to subdir inside
	subDir := filepath.Join(artifacts, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	subFile := filepath.Join(subDir, "inner.txt")
	if err := os.WriteFile(subFile, []byte("inner"), 0o600); err != nil {
		t.Fatalf("write sub: %v", err)
	}
	linkSub := filepath.Join(artifacts, "linkSub")
	if err := os.Symlink(subDir, linkSub); err != nil {
		t.Fatalf("symlink sub: %v", err)
	}
	if err := ValidateContainedPath(artifacts, "linkSub/inner.txt"); err != nil {
		t.Fatalf("internal dir symlink should pass, got %v", err)
	}

	// non-existent path that would be inside should pass (containment checks logical path, not existence, but symlink check only if exists?)
	// For non-existent, EvalSymlinks will not resolve missing file; we should check cleaned path still inside.
	if err := ValidateContainedPath(root, "newdir/newfile.txt"); err != nil {
		t.Fatalf("non-existent inside should pass, got %v", err)
	}
}
