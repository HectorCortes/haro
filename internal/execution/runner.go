package execution

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CommandRunner runs a command with argv, cwd and env, returning exit code, stdout, stderr.
type CommandRunner interface {
	Run(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error)
}

// ParseArgv tokenizes run into argv using quoted-argv lexer.
// It handles single/double quotes and backslash escapes, but performs NO shell expansion.
func ParseArgv(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var (
		args     []string
		cur      []rune
		inSingle bool
		inDouble bool
		escaped  bool
	)
	flush := func() {
		if len(cur) > 0 {
			args = append(args, string(cur))
			cur = nil
		}
	}
	for _, r := range s {
		if escaped {
			cur = append(cur, r)
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			escaped = true
			continue
		}
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
			continue
		case r == '"' && !inSingle:
			inDouble = !inDouble
			continue
		case !inSingle && !inDouble && (r == ' ' || r == '\t' || r == '\n'):
			flush()
			continue
		default:
			cur = append(cur, r)
		}
	}
	if escaped {
		// trailing backslash -> treat as literal
		cur = append(cur, '\\')
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unclosed quote in %q", s)
	}
	flush()
	return args, nil
}

// RealRunner is the production CommandRunner using exec.CommandContext.
type RealRunner struct{}

// NewRunner creates a RealRunner.
func NewRunner() *RealRunner { return &RealRunner{} }

// Run executes argv[0] with argv[1:]. It never invokes a shell.
// It enforces a 300s ceiling in addition to ctx cancellation.
func (r *RealRunner) Run(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
	if len(argv) == 0 {
		return 0, "", "", fmt.Errorf("empty argv")
	}
	// Enforce 300s ceiling
	const ceiling = 300 * time.Second
	runCtx := ctx
	var cancel context.CancelFunc
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) > ceiling {
			runCtx, cancel = context.WithTimeout(ctx, ceiling)
			defer cancel()
		}
	} else {
		runCtx, cancel = context.WithTimeout(ctx, ceiling)
		defer cancel()
	}

	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	// Env: inherit + overrides
	if env != nil {
		base := os.Environ()
		// Convert to map for override
		envMap := make(map[string]string, len(base)+len(env))
		for _, kv := range base {
			if idx := strings.Index(kv, "="); idx != -1 {
				envMap[kv[:idx]] = kv[idx+1:]
			}
		}
		for k, v := range env {
			envMap[k] = v
		}
		var envList []string
		for k, v := range envMap {
			envList = append(envList, k+"="+v)
		}
		cmd.Env = envList
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if runCtx.Err() == context.DeadlineExceeded {
		return 0, stdout.String(), stderr.String(), runCtx.Err()
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), stdout.String(), stderr.String(), nil
		}
		// Start failure or other
		if ctx.Err() != nil {
			return 0, stdout.String(), stderr.String(), ctx.Err()
		}
		return 0, stdout.String(), stderr.String(), err
	}
	return 0, stdout.String(), stderr.String(), nil
}

// RunCall records a FakeRunner invocation.
type RunCall struct {
	Cwd  string
	Argv []string
	Env  map[string]string
}

// FakeRunner is an injectable fake for unit tests.
type FakeRunner struct {
	Calls   []RunCall
	Handler func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error)
}

// Run records the call and delegates to Handler if set.
func (f *FakeRunner) Run(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
	cpArgv := append([]string(nil), argv...)
	cpEnv := make(map[string]string, len(env))
	for k, v := range env {
		cpEnv[k] = v
	}
	f.Calls = append(f.Calls, RunCall{Cwd: cwd, Argv: cpArgv, Env: cpEnv})
	if f.Handler != nil {
		return f.Handler(ctx, cwd, argv, env)
	}
	return 0, "", "", nil
}
