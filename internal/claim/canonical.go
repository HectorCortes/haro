package claim

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Canonicalize returns a clean, relative, symlink-free logical path anchored to repoRoot.
// It rejects absolute paths, lexical ".." escapes, and symlink escapes outside repoRoot.
func Canonicalize(repoRoot, raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(raw) {
		return "", fmt.Errorf("absolute path not allowed: %q", raw)
	}
	clean := filepath.Clean(raw)
	if clean == "." {
		clean = ""
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes root via ..: %q", raw)
	}
	// For empty (root) return empty or "."
	if clean == "" {
		return "", nil
	}
	// Use slash-normalized for logical identity, but evaluate with OS filepath.
	joined := filepath.Join(repoRoot, filepath.FromSlash(clean))

	// Resolve repo root.
	evalRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		evalRoot = filepath.Clean(repoRoot)
	} else {
		evalRoot = filepath.Clean(evalRoot)
	}

	resolved, err := resolveWithSymlinks(joined, evalRoot)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	resolved = filepath.Clean(resolved)

	if resolved != evalRoot && !strings.HasPrefix(resolved, evalRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes root via symlink: %q -> %q not inside %q", raw, resolved, evalRoot)
	}
	// Return canonical logical path as slash-normalized clean.
	return filepath.ToSlash(clean), nil
}

// IsPrefix reports whether parent is a prefix of child with boundary awareness:
// parent == child or child has prefix parent+"/".
func IsPrefix(parent, child string) bool {
	p := filepath.ToSlash(filepath.Clean(parent))
	c := filepath.ToSlash(filepath.Clean(child))
	if p == "." {
		p = ""
	}
	if c == "." {
		c = ""
	}
	if p == "" {
		return c == "" || true // root prefixes everything (not used in claims)
	}
	if p == c {
		return true
	}
	return strings.HasPrefix(c, p+"/")
}

// Overlaps reports whether a and b overlap (one prefixes the other).
func Overlaps(a, b string) bool {
	return IsPrefix(a, b) || IsPrefix(b, a)
}

func resolveWithSymlinks(joined, evalRoot string) (string, error) {
	if _, err := os.Lstat(joined); err == nil {
		ev, err := filepath.EvalSymlinks(joined)
		if err != nil {
			return "", err
		}
		return ev, nil
	}
	var suffix []string
	cur := joined
	for {
		if _, err := os.Lstat(cur); err == nil {
			ev, err := filepath.EvalSymlinks(cur)
			if err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				ev = filepath.Join(ev, suffix[i])
			}
			return filepath.Clean(ev), nil
		}
		if cur == evalRoot || cur == filepath.Dir(cur) {
			break
		}
		parent := filepath.Dir(cur)
		base := filepath.Base(cur)
		suffix = append(suffix, base)
		cur = parent
		if cur == "." {
			break
		}
	}
	if ev, err := filepath.EvalSymlinks(evalRoot); err == nil {
		_ = ev
		return filepath.Clean(joined), nil
	}
	return filepath.Clean(joined), nil
}
