package claude

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/HectorCortes/haro/internal/adapter"
)

// Adapter is the Claude Code adapter.
type Adapter struct {
	binary string
}

// NewAdapter creates a Claude adapter with the given binary path.
func NewAdapter(binary string) *Adapter {
	if binary == "" {
		binary = "claude"
	}
	return &Adapter{binary: binary}
}

// Probe checks availability of the binary.
func (a *Adapter) Probe(ctx context.Context) (adapter.ProbeResult, error) {
	// Check env override
	if env := os.Getenv("HARO_TEST_CLAUDE_BINARY"); env != "" {
		a.binary = env
	}
	// If binary contains slash, check file exists; otherwise look in PATH
	if _, err := exec.LookPath(a.binary); err != nil {
		// Also try stat if LookPath fails but path is absolute
		if _, statErr := os.Stat(a.binary); statErr != nil {
			return adapter.ProbeResult{Available: false}, nil
		}
	}
	return adapter.ProbeResult{
		Available: true,
		Version:   "claude",
		Capabilities: adapter.Capabilities{
			ProtocolVersion: 1,
			Permission:      true,
			Terminal:        false,
			LoadSession:     false,
		},
	}, nil
}

// Initialize negotiates capabilities bilateraly.
func (a *Adapter) Initialize(ctx context.Context, core adapter.Capabilities) (adapter.Capabilities, error) {
	neg, err := adapter.Negotiate(core, adapter.Capabilities{
		ProtocolVersion: 1,
		Permission:      true,
		Terminal:        false,
		LoadSession:     false,
	})
	if err != nil {
		return adapter.Capabilities{}, err
	}
	return neg, nil
}

// NewSession creates a session handle.
func (a *Adapter) NewSession(ctx context.Context, bundle adapter.SessionBundle, host adapter.SessionHost) (adapter.Session, error) {
	if a.binary == "" {
		return nil, fmt.Errorf("binary not set")
	}
	return &claudeSession{bundle: bundle, host: host, binary: a.binary}, nil
}

type claudeSession struct {
	bundle adapter.SessionBundle
	host   adapter.SessionHost
	binary string
}

func (s *claudeSession) Prompt(ctx context.Context, input adapter.PromptInput) (<-chan adapter.SessionEvent, error) {
	ch := make(chan adapter.SessionEvent, 1)
	go func() {
		defer close(ch)
		// In real implementation, would spawn binary via exec.CommandContext with JSON-RPC.
		// For this slice, simulate immediate completion with fallback context.
		payload := []byte(fmt.Sprintf(`{"text":%q}`, input.Text))
		ch <- adapter.SessionEvent{Cursor: 1, Type: "completed", Payload: payload}
	}()
	return ch, nil
}

func (s *claudeSession) Cancel(ctx context.Context) error { return nil }

func (s *claudeSession) LoadPrevious(ctx context.Context, nativeSessionID string) error {
	return adapter.ErrUnsupportedCapability
}

func (s *claudeSession) Terminal(ctx context.Context) (adapter.TerminalHandle, error) {
	return adapter.TerminalHandle{}, adapter.ErrUnsupportedCapability
}
