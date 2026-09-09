package execution

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/HectorCortes/haro/internal/adapter"
)

// fakeHarnessAdapter is an injectable fake for engine fallback tests. It
// implements the adapter contract without any subprocess: Probe availability
// is configurable, NewSession records the bundle, and Prompt emits a
// configured outcome (completed or a clean failed event).
type fakeHarnessAdapter struct {
	mu         sync.Mutex
	available  bool
	outcome    string // "completed" or "failed"
	output     string
	noIdentity bool // session captures no transport identity
	probeCount int  // number of Probe invocations on this adapter
	sessions   int
	bundles    []adapter.SessionBundle
	prompts    []string
	nativeIDs  []string
}

func newFakeHarnessAdapter(available bool, outcome, output string) *fakeHarnessAdapter {
	return &fakeHarnessAdapter{available: available, outcome: outcome, output: output}
}

func (f *fakeHarnessAdapter) Probe(_ context.Context) (adapter.ProbeResult, error) {
	f.mu.Lock()
	f.probeCount++
	f.mu.Unlock()
	return adapter.ProbeResult{
		Available:    f.available,
		Version:      "fake-1.0",
		Capabilities: adapter.Capabilities{ProtocolVersion: 1},
	}, nil
}

// ProbeCount reports how many times Probe ran on this adapter. F-01 allows
// exactly one probe per harness per CLI invocation.
func (f *fakeHarnessAdapter) ProbeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.probeCount
}

func (f *fakeHarnessAdapter) Initialize(_ context.Context, core adapter.Capabilities) (adapter.Capabilities, error) {
	return adapter.Negotiate(core, adapter.Capabilities{ProtocolVersion: 1})
}

func (f *fakeHarnessAdapter) NewSession(_ context.Context, bundle adapter.SessionBundle, _ adapter.SessionHost) (adapter.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions++
	// Mirror the real adapter contract: required artifact names are mapped
	// to absolute .haro/artifacts paths relative to the workspace root.
	if len(bundle.Requires) > 0 {
		artifactsRoot := filepath.Join(bundle.WorkspaceRoot, ".haro", "artifacts")
		resolved := make(map[string]string, len(bundle.Requires))
		for name, rel := range bundle.Requires {
			resolved[name] = filepath.Join(artifactsRoot, rel)
		}
		bundle.Requires = resolved
	}
	f.bundles = append(f.bundles, bundle)
	id := "fake-sess-" + strings.Repeat("x", f.sessions)
	f.nativeIDs = append(f.nativeIDs, id)
	return &fakeHarnessSession{adapter: f, nativeID: id, noIdentity: f.noIdentity}, nil
}

func (f *fakeHarnessAdapter) NewSessionCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sessions
}

func (f *fakeHarnessAdapter) LastBundle() adapter.SessionBundle {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.bundles) == 0 {
		return adapter.SessionBundle{}
	}
	return f.bundles[len(f.bundles)-1]
}

func (f *fakeHarnessAdapter) LastPrompt() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.prompts) == 0 {
		return ""
	}
	return f.prompts[len(f.prompts)-1]
}

type fakeHarnessSession struct {
	adapter    *fakeHarnessAdapter
	nativeID   string
	noIdentity bool
}

func (s *fakeHarnessSession) Prompt(ctx context.Context, input adapter.PromptInput) (<-chan adapter.SessionEvent, error) {
	s.adapter.mu.Lock()
	s.adapter.prompts = append(s.adapter.prompts, input.Text)
	s.adapter.mu.Unlock()
	ch := make(chan adapter.SessionEvent, 2)
	go func() {
		defer close(ch)
		s.adapter.mu.Lock()
		outcome := s.adapter.outcome
		output := s.adapter.output
		s.adapter.mu.Unlock()
		if output != "" {
			ch <- adapter.SessionEvent{Cursor: 1, Type: "output_delta", Payload: []byte(output)}
		}
		switch outcome {
		case "failed":
			ch <- adapter.SessionEvent{Cursor: 2, Type: "failed", Payload: []byte("boom from fake")}
		default:
			ch <- adapter.SessionEvent{Cursor: 2, Type: "completed", Payload: []byte("")}
		}
	}()
	return ch, nil
}

func (s *fakeHarnessSession) Cancel(context.Context) error { return nil }

func (s *fakeHarnessSession) LoadPrevious(context.Context, string) error {
	return adapter.ErrUnsupportedCapability
}

func (s *fakeHarnessSession) Terminal(context.Context) (adapter.TerminalHandle, error) {
	return adapter.TerminalHandle{}, adapter.ErrUnsupportedCapability
}

// SessionTransport exposes the fake real identity so the engine can persist
// actual transport fields for the attempt.
func (s *fakeHarnessSession) SessionTransport() (adapter.SessionTransport, bool) {
	if s.noIdentity {
		return adapter.SessionTransport{}, false
	}
	return adapter.SessionTransport{
		NativeSessionID: s.nativeID,
		ProtocolVersion: 1,
		Extra:           map[string]any{"fake": true},
	}, true
}
