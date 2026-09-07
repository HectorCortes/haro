package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HectorCortes/haro/internal/claim"
	"github.com/HectorCortes/haro/internal/store"
)

// Report is execution-scoped, side-effect-free result.
type Report struct {
	ExecutionID  string  `json:"execution_id"`
	BaseCommit   *string `json:"base_commit"`
	ChangedFiles []string `json:"changed_files"`
}

// ReportError is a stable error with code and optional field for CLI mapping.
type ReportError struct {
	Code    string
	Field   string
	Message string
}

func (e *ReportError) Error() string { return e.Message }

// ChangedFiles returns sorted, deduped, canonical repo-relative changed paths for a terminal execution.
func (e *Engine) ChangedFiles(ctx context.Context, executionID string) ([]string, error) {
	rpt, err := e.Report(ctx, executionID)
	if err != nil {
		return nil, err
	}
	return rpt.ChangedFiles, nil
}

// Report returns the full report for a terminal execution.
func (e *Engine) Report(ctx context.Context, executionID string) (*Report, error) {
	execRec, err := e.store.Executions().Get(ctx, executionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, &ReportError{Code: "not_found", Message: fmt.Sprintf("execution %q not found", executionID)}
		}
		return nil, &ReportError{Code: "not_found", Message: fmt.Sprintf("execution %q not found: %v", executionID, err)}
	}
	if execRec == nil {
		return nil, &ReportError{Code: "not_found", Message: fmt.Sprintf("execution %q not found", executionID)}
	}
	// Terminal gate
	if execRec.Status != "completed" && execRec.Status != "failed" {
		return nil, &ReportError{Code: "not_completed", Message: fmt.Sprintf("execution %q not terminal (status=%s)", executionID, execRec.Status)}
	}
	if execRec.BaseCommit == nil {
		return nil, &ReportError{Code: "not_available", Field: "base_commit", Message: fmt.Sprintf("execution %q has no base_commit (legacy row)", executionID)}
	}
	base := *execRec.BaseCommit
	if strings.TrimSpace(base) == "" {
		return nil, &ReportError{Code: "not_available", Field: "base_commit", Message: fmt.Sprintf("execution %q has empty base_commit", executionID)}
	}
	// Resolve repo root via EvalSymlinks/clean pattern (engine-owned canonical)
	var repoRoot string
	if eval, err := filepath.EvalSymlinks(e.root); err == nil {
		repoRoot = filepath.Clean(eval)
	} else {
		repoRoot = filepath.Clean(e.root)
	}
	// Determine workspace selection
	var cwd string
	var useFallback bool
	if execRec.WorkspaceMode == "shared" {
		cwd = repoRoot
		useFallback = false
	} else {
		// isolated: check workspace existence
		ws := execRec.WorkspaceRoot
		if ws == "" {
			ws = repoRoot
		}
		// Clean check existence
		if info, err := os.Stat(ws); err == nil && info.IsDir() {
			// Existing workspace: check if git worktree still present by presence of .git file? But just stat existence is enough.
			// Ensure cwd is inside repoRoot? WorkspaceRoot is .haro/worktrees/<id>
			cwd = ws
			// Even if workspace exists but is same as repoRoot (edge), treat as repoRoot
			useFallback = false
		} else {
			// Removed -> fallback to repo root, no ls-files
			cwd = repoRoot
			useFallback = true
		}
	}
	// Prepare git diff argv
	// Fixed argv, never git -C, cwd = selected dir
	var diffArgs []string
	if useFallback {
		diffArgs = []string{"diff", "--name-only", "--diff-filter=ADMR", "--find-renames", "-z", base, "HEAD", "--"}
	} else {
		diffArgs = []string{"diff", "--name-only", "--diff-filter=ADMR", "--find-renames", "-z", base, "--"}
	}
	diffOut, err := runGit(ctx, cwd, diffArgs)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	tracked := parseNUL(diffOut)
	var untracked []string
	if !useFallback {
		// Scan untracked only while workspace exists
		if lsOut, err := runGit(ctx, cwd, []string{"ls-files", "--others", "--exclude-standard", "-z"}); err == nil {
			untracked = parseNUL(lsOut)
		}
	}
	all := append(tracked, untracked...)
	// Normalization via claim-style checks, .haro filter, dedupe, sort
	normalized, err := normalizePaths(cwd, repoRoot, execRec.WorkspaceMode, all)
	if err != nil {
		return nil, err
	}
	return &Report{
		ExecutionID:  executionID,
		BaseCommit:   execRec.BaseCommit,
		ChangedFiles: normalized,
	}, nil
}

func runGit(ctx context.Context, dir string, args []string) ([]byte, error) {
	// Fixed argv, no shell, dir = engine-owned canonical
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w output: %s", strings.Join(args, " "), err, string(out))
	}
	return out, nil
}

func parseNUL(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	parts := bytes.Split(b, []byte{0})
	var out []string
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		out = append(out, string(p))
	}
	return out
}

func normalizePaths(cwd, repoRoot, workspaceMode string, paths []string) ([]string, error) {
	// Determine anchor for symlink checks: for isolated existing, cwd is the worktree; else repoRoot
	anchor := repoRoot
	if workspaceMode != "shared" {
		// If cwd != repoRoot and cwd exists, use cwd as anchor for symlink evaluation (file exists in worktree)
		if cwd != repoRoot {
			if info, err := os.Stat(cwd); err == nil && info.IsDir() {
				anchor = cwd
			}
		}
	}
	seen := make(map[string]bool)
	var filtered []string
	for _, raw := range paths {
		if raw == "" {
			continue
		}
		// Use claim canonicalize for clean/absolute/../symlink checks anchored to anchor
		canonical, err := claim.Canonicalize(anchor, raw)
		if err != nil {
			// For deleted paths, canonicalize may still succeed via parent; if fails due to symlink escape, skip (reject)
			// Also if raw contains absolute or escape, we skip
			// We also attempt simple clean check for .haro filtering without filesystem: if canonical fails due to not found anchor, fallback to simple clean filtering
			// Try simple slash/clean filtering for deleted entries that may not exist on disk
			cleaned := filepath.ToSlash(filepath.Clean(raw))
			if filepath.IsAbs(raw) || strings.HasPrefix(cleaned, "/") {
				continue
			}
			if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
				continue
			}
			// Still skip if symlink check would require filesystem but file missing -> use cleaned path as fallback if not escape
			// Apply .haro filter on cleaned
			if cleaned == ".haro" || strings.HasPrefix(cleaned, ".haro/") {
				continue
			}
			if seen[cleaned] {
				continue
			}
			seen[cleaned] = true
			// For this fallback, we don't have symlink assurance, but allow
			filtered = append(filtered, cleaned)
			continue
		}
		// canonical is slash-normalized, relative
		if canonical == "" {
			continue
		}
		// Filter .haro internals
		if canonical == ".haro" || strings.HasPrefix(canonical, ".haro/") {
			continue
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		filtered = append(filtered, canonical)
	}
	sort.Strings(filtered)
	return filtered, nil
}
