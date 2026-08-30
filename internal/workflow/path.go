package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateContainedPath ensures rel is contained under root.
// It rejects absolute paths and traversal via "..", and after resolving
// symlinks with filepath.EvalSymlinks it ensures the resolved target remains
// inside the resolved root. Internal symlinks that stay inside are allowed.
func ValidateContainedPath(root, rel string) error {
	if filepath.IsAbs(rel) {
		return fmt.Errorf("absolute path not allowed: %q", rel)
	}
	cleanRel := filepath.Clean(rel)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes root via ..: %q", rel)
	}
	// Also reject if cleaned rel tries to escape after join (e.g., "a/../../b" already caught).
	// For "." (empty) we treat as root itself — allowed.
	if cleanRel == "." {
		cleanRel = ""
	}
	joined := filepath.Join(root, cleanRel)

	// Resolve root for comparison.
	evalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		// If root does not exist, fall back to clean root; but containment
		// will be checked logically.
		evalRoot = filepath.Clean(root)
	} else {
		evalRoot = filepath.Clean(evalRoot)
	}

	// Try to resolve joined via EvalSymlinks, handling non-existent suffixes.
	resolved, err := resolveWithSymlinks(joined, evalRoot)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	resolved = filepath.Clean(resolved)

	// Check containment: resolved must be evalRoot or inside it.
	if resolved != evalRoot && !strings.HasPrefix(resolved, evalRoot+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes root via symlink: %q -> %q not inside %q", rel, resolved, evalRoot)
	}
	return nil
}

// resolveWithSymlinks resolves joined using EvalSymlinks, handling non-existent tail.
// It finds the longest existing prefix, evaluates it, and appends the remaining components.
func resolveWithSymlinks(joined, evalRoot string) (string, error) {
	// Fast path: if joined exists (file, dir, or symlink), EvalSymlinks succeeds.
	if _, err := os.Lstat(joined); err == nil {
		// Exists — EvalSymlinks will resolve symlink if any.
		if ev, err := filepath.EvalSymlinks(joined); err == nil {
			return ev, nil
		} else {
			return "", err
		}
	}
	// For non-existent, walk up to find existing ancestor.
	// Collect non-existing suffix components.
	var suffix []string
	cur := joined
	for {
		if _, err := os.Lstat(cur); err == nil {
			// Found existing ancestor.
			ev, err := filepath.EvalSymlinks(cur)
			if err != nil {
				return "", err
			}
			// Re-append suffix in correct order.
			for i := len(suffix) - 1; i >= 0; i-- {
				ev = filepath.Join(ev, suffix[i])
			}
			return filepath.Clean(ev), nil
		}
		// If we've reached evalRoot or filesystem root, break.
		if cur == evalRoot || cur == filepath.Dir(cur) {
			break
		}
		parent := filepath.Dir(cur)
		base := filepath.Base(cur)
		suffix = append(suffix, base)
		cur = parent
		// Avoid infinite loop if cur becomes ".".
		if cur == "." {
			break
		}
	}
	// No existing ancestor found besides maybe evalRoot itself.
	// Try evalRoot directly.
	if ev, err := filepath.EvalSymlinks(evalRoot); err == nil {
		// Append original suffix relative to evalRoot? We need to compute remainder from evalRoot to joined.
		// Simpler: clean joined logical path and check prefix without symlink resolution.
		// Since no ancestor existed beyond root, the path is logically inside if it passed earlier checks.
		// Return logical cleaned joined for prefix check.
		_ = ev
		return filepath.Clean(joined), nil
	}
	return filepath.Clean(joined), nil
}
