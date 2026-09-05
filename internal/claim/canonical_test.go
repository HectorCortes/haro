package claim

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonical(t *testing.T) {
	root := t.TempDir()
	// repo root creation: emulate git repo (needs .git dir for realism but not required)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "foo.ts"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"clean relative", "src/foo.ts", "src/foo.ts", false},
		{"dot traversal inside", "src/../src/foo.ts", "src/foo.ts", false},
		{"absolute rejected", "/etc/passwd", "", true},
		{"dotdot escape", "../etc/passwd", "", true},
		{"dotdot via segment", "a/../../b", "", true},
		{"prefix sibling not conflict", "src/foobar/file.ts", "src/foobar/file.ts", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Canonicalize(root, tt.raw)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q, got %q", tt.raw, got)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.raw, err)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("Canonicalize(%q)=%q want %q", tt.raw, got, tt.want)
			}
		})
	}

	t.Run("prefix boundary", func(t *testing.T) {
		// src/ should be prefix of src/foo.ts; boundary ensures src does not match srcfoobar
		if !IsPrefix("src", "src/foo.ts") {
			t.Fatalf("expected src prefix src/foo.ts")
		}
		if !IsPrefix("src", "src/foobar/file.ts") {
			t.Fatalf("expected src prefix src/foobar/file.ts (subdirectory)")
		}
		if IsPrefix("src", "srcfoobar") {
			t.Fatalf("src should not prefix srcfoobar")
		}
		if !Overlaps("src/foo.ts", "src") {
			t.Fatalf("Overlaps should detect child/parent")
		}
		if Overlaps("src", "srcfoobar") {
			t.Fatalf("src should not overlap srcfoobar")
		}
	})

	t.Run("symlink internal stays", func(t *testing.T) {
		// internal symlink pointing inside stays allowed
		target := filepath.Join(root, "src")
		link := filepath.Join(root, "link_inside")
		if err := os.Symlink(target, link); err != nil {
			t.Skip("symlink not supported")
		}
		// canonicalize via symlink path: link/foo.ts should resolve to src/foo.ts and stay inside
		got, err := Canonicalize(root, "link_inside/foo.ts")
		if err != nil {
			t.Fatalf("internal symlink should not escape: %v", err)
		}
		// canonical should return clean logical path (maybe link_inside/foo.ts cleaned? but resolved stays inside)
		_ = got
	})

	t.Run("symlink external escape rejected", func(t *testing.T) {
		ext := t.TempDir()
		link := filepath.Join(root, "link_outside")
		if err := os.Symlink(ext, link); err != nil {
			t.Skip("symlink not supported")
		}
		if _, err := Canonicalize(root, "link_outside/file.ts"); err == nil {
			t.Fatalf("expected escape error for symlink pointing outside")
		}
	})

	t.Run("anchored root symlink escape src/link->/etc", func(t *testing.T) {
		srcDir := filepath.Join(root, "src")
		_ = os.MkdirAll(srcDir, 0o755)
		link := filepath.Join(srcDir, "link")
		// symlink inside src pointing to /etc
		if err := os.Symlink("/etc", link); err != nil {
			t.Skip("symlink not supported")
		}
		// canonicalize via src/link should be rejected as escape
		if _, err := Canonicalize(root, "src/link/passwd"); err == nil {
			t.Fatalf("expected escape for src/link->/etc passwd")
		}
		// also test root-anchored EvalSymlinks: even if root itself is symlink
		// create alternative root symlink and canonicalize via it
		altRoot := filepath.Join(t.TempDir(), "altroot")
		if err := os.Symlink(root, altRoot); err != nil {
			t.Skip("symlink not supported")
		}
		if _, err := Canonicalize(altRoot, "src/foo.ts"); err != nil {
			t.Fatalf("altroot via symlink should still resolve inside: %v", err)
		}
		if _, err := Canonicalize(altRoot, "src/link/passwd"); err == nil {
			t.Fatalf("altroot anchored should still reject escape via src/link->/etc")
		}
	})
}

func TestCanonicalRequiresBoundaryPredicate(t *testing.T) {
	if !IsPrefix("src", "src") {
		t.Fatalf("same path should be prefix")
	}
	if !IsPrefix("src", "src/foo.ts") {
		t.Fatalf("src should prefix src/foo.ts")
	}
	if IsPrefix("src", "srcfoobar") {
		t.Fatalf("src should not prefix srcfoobar - boundary check")
	}
	if IsPrefix("src/foo", "src/foobar") {
		t.Fatalf("src/foo should not prefix src/foobar")
	}
}

func TestOverlaps(t *testing.T) {
	if !Overlaps("src", "src/foo.ts") {
		t.Fatalf("src overlaps src/foo.ts")
	}
	if !Overlaps("src/foo.ts", "src") {
		t.Fatalf("symmetric overlaps")
	}
	if Overlaps("src/foo", "src/foobar") {
		t.Fatalf("should not overlap sibling")
	}
	if Overlaps("a", "b") {
		t.Fatalf("distinct should not overlap")
	}
}
