package worktree

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Manager handles git worktree lifecycle.
type Manager interface {
	Create(ctx context.Context, executionID, repoRoot string) (string, error)
	Remove(ctx context.Context, worktreePath string) error
	Prune(ctx context.Context, repoRoot string) error
	List(ctx context.Context, repoRoot string) ([]string, error)
}

// realManager is production implementation using exec.CommandContext with fixed argv.
type realManager struct{}

// NewManager creates a real git worktree manager.
func NewManager() Manager { return &realManager{} }

func (m *realManager) Create(ctx context.Context, executionID, repoRoot string) (string, error) {
	if executionID == "" {
		return "", fmt.Errorf("empty executionID")
	}
	// Resolve repoRoot via EvalSymlinks for threat-matrix anchored root.
	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		resolvedRoot = filepath.Clean(repoRoot)
	} else {
		resolvedRoot = filepath.Clean(resolvedRoot)
	}
	// Destination is .haro/worktrees/<executionID> under resolved root.
	dest := filepath.Join(resolvedRoot, ".haro", "worktrees", executionID)
	// Idempotent: if already exists, return.
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("mkdir worktrees: %w", err)
	}
	// Fixed argv, no shell, cwd = resolvedRoot, no git -C
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "--detach", dest, "HEAD")
	cmd.Dir = resolvedRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If error indicates already exists, treat as idempotent
		if _, sErr := os.Stat(dest); sErr == nil {
			return dest, nil
		}
		return "", fmt.Errorf("git worktree add: %w output: %s", err, string(out))
	}
	return dest, nil
}

func (m *realManager) Remove(ctx context.Context, worktreePath string) error {
	if worktreePath == "" {
		return fmt.Errorf("empty worktreePath")
	}
	// Determine repo root from worktreePath? For Prune we need repoRoot, but Remove can use worktree's parent repo via git.
	// Try to find git dir by walking up? Simpler: use execution via parent dir's repoRoot if available.
	// Use worktreePath's repoRoot by looking for .git up the tree from parent.
	// For fixed argv we need Dir = resolved git root. We can infer from worktreePath: strip /.haro/worktrees/<id>
	repoRoot := inferRepoRoot(worktreePath)
	if repoRoot != "" {
		if eval, err := filepath.EvalSymlinks(repoRoot); err == nil {
			repoRoot = filepath.Clean(eval)
		}
	}
	// If worktreePath does not exist, idempotent success.
	if _, err := os.Stat(worktreePath); err != nil && os.IsNotExist(err) {
		// Still try git prune to clean stale entry, but not error.
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreePath)
	if repoRoot != "" {
		cmd.Dir = repoRoot
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If already removed, idempotent
		if _, sErr := os.Stat(worktreePath); sErr != nil && os.IsNotExist(sErr) {
			return nil
		}
		return fmt.Errorf("git worktree remove: %w output: %s", err, string(out))
	}
	return nil
}

func (m *realManager) Prune(ctx context.Context, repoRoot string) error {
	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		resolvedRoot = filepath.Clean(repoRoot)
	} else {
		resolvedRoot = filepath.Clean(resolvedRoot)
	}
	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = resolvedRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree prune: %w output: %s", err, string(out))
	}
	return nil
}

func (m *realManager) List(ctx context.Context, repoRoot string) ([]string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		resolvedRoot = filepath.Clean(repoRoot)
	} else {
		resolvedRoot = filepath.Clean(resolvedRoot)
	}
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = resolvedRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w output: %s", err, string(out))
	}
	// Parse porcelain: lines "worktree <path>"
	var list []string
	for _, line := range splitLines(string(out)) {
		if len(line) > 9 && line[:9] == "worktree " {
			list = append(list, line[9:])
		}
	}
	return list, nil
}

func inferRepoRoot(worktreePath string) string {
	// worktreePath is <repo>/.haro/worktrees/<id> -> repo is two dirs up from .haro
	// Find ".haro" segment.
	clean := filepath.Clean(worktreePath)
	parts := splitPath(clean)
	for i, p := range parts {
		if p == ".haro" && i > 0 {
			return filepath.Join(parts[:i]...)
		}
	}
	// fallback: parent dir
	return filepath.Dir(filepath.Dir(clean))
}

func splitPath(p string) []string {
	var out []string
	for p != "" && p != "/" && p != "." {
		dir := filepath.Dir(p)
		base := filepath.Base(p)
		out = append([]string{base}, out...)
		if dir == p {
			break
		}
		p = dir
		if p == "/" {
			out = append([]string{"/"}, out...)
			break
		}
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// FakeManager is in-memory fake for unit tests.
type FakeManager struct {
	Calls []FakeCall
	worktrees map[string]bool
}

type FakeCall struct {
	Op   string
	ID   string
	Root string
	Path string
}

func (f *FakeManager) ensure() {
	if f.worktrees == nil {
		f.worktrees = make(map[string]bool)
	}
}

func (f *FakeManager) Create(ctx context.Context, executionID, repoRoot string) (string, error) {
	f.ensure()
	f.Calls = append(f.Calls, FakeCall{Op: "create", ID: executionID, Root: repoRoot})
	dest := filepath.Join(repoRoot, ".haro", "worktrees", executionID)
	f.worktrees[dest] = true
	return dest, nil
}

func (f *FakeManager) Remove(ctx context.Context, worktreePath string) error {
	f.ensure()
	f.Calls = append(f.Calls, FakeCall{Op: "remove", Path: worktreePath})
	delete(f.worktrees, worktreePath)
	return nil
}

func (f *FakeManager) Prune(ctx context.Context, repoRoot string) error {
	f.ensure()
	f.Calls = append(f.Calls, FakeCall{Op: "prune", Root: repoRoot})
	return nil
}

func (f *FakeManager) List(ctx context.Context, repoRoot string) ([]string, error) {
	f.ensure()
	f.Calls = append(f.Calls, FakeCall{Op: "list", Root: repoRoot})
	var out []string
	for k := range f.worktrees {
		out = append(out, k)
	}
	return out, nil
}
