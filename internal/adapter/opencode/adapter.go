// Package opencode implements the real CLI-direct session adapter for the
// OpenCode harness. It is the only registered provider adapter in v2; all
// provider literals for OpenCode are confined to this package.
package opencode

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/HectorCortes/haro/internal/adapter"
)

// DefaultBinary is used when neither the test seam nor configuration
// provides a binary path.
const DefaultBinary = "opencode"

// ProtocolVersion is the adapter protocol version negotiated with the core.
const ProtocolVersion = 1

// DefaultTimeout bounds a session when configuration omits timeout_seconds.
const DefaultTimeout = 300 * time.Second

// Adapter is the OpenCode CLI-direct adapter.
type Adapter struct {
	binary  string
	env     map[string]string
	timeout time.Duration
	neg     adapter.Capabilities
	initd   bool
}

// NewAdapter creates an OpenCode adapter with the given binary path, env
// overlay, and per-session timeout. A zero timeout defaults to 300s.
func NewAdapter(binary string, env map[string]string, timeout time.Duration) *Adapter {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Adapter{binary: binary, env: env, timeout: timeout}
}

// isExecutableProgram reports whether path is an executable regular file the
// kernel can exec directly: an ELF binary or a script with an interpreter
// line. Documentation-like files (requirements.txt, CMakeLists.txt, Markdown,
// extensionless text) and non-executable scripts are rejected so a disguised
// path can never be launched with the fixed argv.
func isExecutableProgram(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 4)
	n, err := f.Read(head)
	if err != nil && n == 0 {
		return false
	}
	if bytes.HasPrefix(head[:n], []byte("\x7fELF")) {
		return true
	}
	return bytes.HasPrefix(head[:n], []byte("#!"))
}

// Probe checks availability of the binary. A missing, non-executable, or
// documentation-like path yields Available:false with a nil error so the
// manager falls through per candidate instead of aborting everything.
func (a *Adapter) Probe(_ context.Context) (adapter.ProbeResult, error) {
	binary := a.binary
	if !isExecutableProgram(binary) {
		return adapter.ProbeResult{Available: false}, nil
	}
	return adapter.ProbeResult{
		Available: true,
		Version:   "opencode",
		Capabilities: adapter.Capabilities{
			ProtocolVersion: ProtocolVersion,
			Permission:      false,
			Terminal:        false,
			LoadSession:     false,
		},
	}, nil
}

// Initialize negotiates capabilities bilaterally. OpenCode supports neither
// permission gating, terminal, nor session loading in this slice.
func (a *Adapter) Initialize(_ context.Context, core adapter.Capabilities) (adapter.Capabilities, error) {
	neg, err := adapter.Negotiate(core, adapter.Capabilities{
		ProtocolVersion: ProtocolVersion,
		Permission:      false,
		Terminal:        false,
		LoadSession:     false,
	})
	if err != nil {
		return adapter.Capabilities{}, err
	}
	a.neg = neg
	a.initd = true
	return neg, nil
}

// NewSession creates a session handle for the given bundle. Required
// artifacts are validated for containment and mapped to absolute
// .haro/artifacts/... paths before any subprocess runs.
func (a *Adapter) NewSession(_ context.Context, bundle adapter.SessionBundle, host adapter.SessionHost) (adapter.Session, error) {
	if !a.initd {
		return nil, adapter.ErrNotInitialized
	}
	if a.binary == "" {
		return nil, fmt.Errorf("binary not set")
	}
	requires, err := resolveRequires(bundle.WorkspaceRoot, bundle.Requires)
	if err != nil {
		return nil, fmt.Errorf("contract: %w", err)
	}
	resolved := bundle
	resolved.Requires = requires
	return &session{
		adapter: a,
		bundle:  resolved,
		host:    host,
		done:    make(chan struct{}),
	}, nil
}

// resolveRequires maps required artifact names to absolute paths under
// <workspaceRoot>/.haro/artifacts, rejecting absolute paths, escapes, and
// missing files.
func resolveRequires(workspaceRoot string, requires map[string]string) (map[string]string, error) {
	if len(requires) == 0 {
		return map[string]string{}, nil
	}
	artifactsRoot := filepath.Join(workspaceRoot, ".haro", "artifacts")
	out := make(map[string]string, len(requires))
	for name, rel := range requires {
		if strings.TrimSpace(rel) == "" {
			return nil, fmt.Errorf("requirement %q: empty path", name)
		}
		if filepath.IsAbs(rel) {
			return nil, fmt.Errorf("requirement %q: absolute path", name)
		}
		clean := filepath.ToSlash(filepath.Clean(rel))
		if clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("requirement %q: escapes artifacts root", name)
		}
		abs := filepath.Join(artifactsRoot, filepath.FromSlash(clean))
		if _, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("requirement %q: %w", name, err)
		}
		out[name] = abs
	}
	return out, nil
}

// session is a real OpenCode subprocess session.
type session struct {
	adapter    *Adapter
	bundle     adapter.SessionBundle
	host       adapter.SessionHost
	cmd        *exec.Cmd
	done       chan struct{}
	doneOnce   sync.Once
	cancelOnce sync.Once
	mu         sync.Mutex
	nativeID   string
}

// settleDone closes the completion signal exactly once, on every path:
// successful runs, launch failures, and pipe errors. Cancel waits on this
// signal, so leaving it open on an early-return path would deadlock a
// non-cancellable Cancel.
func (s *session) settleDone() {
	s.doneOnce.Do(func() { close(s.done) })
}

// Prompt launches `binary run --format json` with the prompt on stdin, no
// shell, the configured environment overlay, and a bounded timeout. The
// subprocess is started synchronously so launch failures surface as a clean
// error before any event is streamed.
func (s *session) Prompt(ctx context.Context, input adapter.PromptInput) (<-chan adapter.SessionEvent, error) {
	// Every early-return error path below must close the completion signal
	// so Cancel never waits forever. Once the consumer goroutine takes
	// ownership (started=true) it settles the signal at run completion
	// instead; the deferred settle then becomes a no-op.
	started := false
	defer func() {
		if !started {
			s.settleDone()
		}
	}()
	runCtx, cancelRun := context.WithTimeout(ctx, s.adapter.timeout)
	argv := []string{s.adapter.binary, "run", "--format", "json"}
	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	if s.bundle.WorkspaceRoot != "" {
		cmd.Dir = s.bundle.WorkspaceRoot
	}
	// Inherited environment with the configured overlay applied last.
	if len(s.adapter.env) > 0 {
		envMap := make(map[string]string)
		for _, kv := range os.Environ() {
			if idx := strings.IndexByte(kv, '='); idx != -1 {
				envMap[kv[:idx]] = kv[idx+1:]
			}
		}
		for k, v := range s.adapter.env {
			envMap[k] = v
		}
		envList := make([]string, 0, len(envMap))
		for k, v := range envMap {
			envList = append(envList, k+"="+v)
		}
		cmd.Env = envList
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancelRun()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancelRun()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancelRun()
		return nil, fmt.Errorf("launch %s: %w", s.adapter.binary, err)
	}
	s.cmd = cmd
	if _, err := stdin.Write([]byte(input.Text)); err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		cancelRun()
		return nil, fmt.Errorf("write prompt: %w", err)
	}
	if err := stdin.Close(); err != nil {
		cancelRun()
		return nil, fmt.Errorf("close prompt: %w", err)
	}
	ch := make(chan adapter.SessionEvent, 8)
	started = true
	go func() {
		defer close(ch)
		defer s.settleDone()
		defer cancelRun()
		s.consume(runCtx, stdout, &stderr, ch)
	}()
	return ch, nil
}

// consume parses the JSONL stream through the existing parser, translates
// the pinned envelope to session events, and captures the common real
// sessionID for the transport accessor. All output_delta events are emitted
// before the single final terminal event so a consumer that breaks on the
// final event still drains the channel without deadlock.
func (s *session) consume(runCtx context.Context, stdout io.ReadCloser, stderr *bytes.Buffer, ch chan<- adapter.SessionEvent) {
	events, perr := ParseJSONL(stdout)
	waitErr := s.cmd.Wait()
	cursor := int64(0)
	emit := func(evType string, payload string) {
		cursor++
		ch <- adapter.SessionEvent{Cursor: cursor, Type: evType, Payload: []byte(payload)}
	}
	sawText := false
	var failMsg string
	for _, ev := range events {
		if ev.SessionID != "" {
			s.mu.Lock()
			if s.nativeID == "" {
				s.nativeID = ev.SessionID
			}
			s.mu.Unlock()
		}
		if ev.Error != "" || ev.Type == "error" {
			if failMsg == "" {
				failMsg = ev.Error
			}
			continue
		}
		if text := ev.Text(); text != "" {
			sawText = true
			emit("output_delta", text)
		}
	}
	switch {
	case perr != nil:
		// Malformed or oversized frame: malformed protocol is terminal.
		emit("failed", fmt.Sprintf("protocol error: %v", perr))
	case runCtx.Err() == context.DeadlineExceeded:
		// Timeout is terminal.
		emit("failed", fmt.Sprintf("timeout: session exceeded %v", s.adapter.timeout))
	case failMsg != "":
		// Harness-declared error envelope: clean failure.
		emit("failed", failMsg)
	case waitErr != nil && !sawText:
		msg := waitErr.Error()
		if stderr.Len() > 0 {
			msg += ": " + stderr.String()
		}
		emit("failed", msg)
	case !sawText:
		// Zero-text EOF fails cleanly (not terminal).
		emit("failed", "no output: session ended without text events")
	default:
		emit("completed", "")
	}
}

// Cancel kills the subprocess exactly once and waits for the run goroutine
// to drain. Calling Cancel on an already-settled or never-started session is
// a no-op.
func (s *session) Cancel(ctx context.Context) error {
	s.cancelOnce.Do(func() {
		if s.cmd != nil && s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	})
	if s.done == nil {
		return nil
	}
	select {
	case <-s.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// LoadPrevious is unsupported: OpenCode sessions here cannot resume a prior
// native session.
func (s *session) LoadPrevious(_ context.Context, _ string) error {
	return adapter.ErrUnsupportedCapability
}

// Terminal is unsupported: embedded terminal/PTY is deferred.
func (s *session) Terminal(_ context.Context) (adapter.TerminalHandle, error) {
	return adapter.TerminalHandle{}, adapter.ErrUnsupportedCapability
}

// SessionTransport exposes the real transport identity captured during the
// run. It reports false until the JSONL stream carried a sessionID.
func (s *session) SessionTransport() (adapter.SessionTransport, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nativeID == "" {
		return adapter.SessionTransport{}, false
	}
	return adapter.SessionTransport{
		NativeSessionID: s.nativeID,
		ProtocolVersion: s.adapter.neg.ProtocolVersion,
		Extra:           map[string]any{"transport": "jsonl"},
	}, true
}
