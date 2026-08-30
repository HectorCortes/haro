package adapter

import (
	"context"
	"errors"
)

// ErrUnsupportedCapability is returned when an optional method is invoked without negotiation.
var ErrUnsupportedCapability = errors.New("unsupported_capability")

// ErrNotInitialized is returned when NewSession is called before Initialize.
var ErrNotInitialized = errors.New("not initialized: initialize must be called first")

// ProbeResult describes adapter availability.
type ProbeResult struct {
	Available    bool
	Version      string
	Capabilities Capabilities
}

// SessionBundle is the immutable input context of an attempt.
type SessionBundle struct {
	Instructions  string
	WorkspaceRoot string
	Requires      map[string]string
}

// PromptInput is the user prompt.
type PromptInput struct {
	Text string
}

// SessionEvent is an event from the harness.
type SessionEvent struct {
	Cursor  int64
	Type    string // "output_delta" | "permission_requested" | "completed" | "failed"
	Payload []byte // JSON
}

// PermissionRequest is a permission query from harness to host.
type PermissionRequest struct {
	Kind        string
	Description string
	Options     []string
}

// PermissionDecision is the host's answer.
type PermissionDecision struct {
	Option string
}

// TerminalHandle is a stub for PTY deferred.
type TerminalHandle struct{}

// SessionHost is implemented by the core/broker and passed to adapter in NewSession.
type SessionHost interface {
	RequestPermission(ctx context.Context, req PermissionRequest) (PermissionDecision, error)
}

// Session is the handle of an in-flight attempt.
type Session interface {
	Prompt(ctx context.Context, input PromptInput) (<-chan SessionEvent, error)
	Cancel(ctx context.Context) error
	LoadPrevious(ctx context.Context, nativeSessionID string) error
	Terminal(ctx context.Context) (TerminalHandle, error)
}

// Adapter is implemented once per harness.
type Adapter interface {
	Probe(ctx context.Context) (ProbeResult, error)
	Initialize(ctx context.Context, core Capabilities) (Capabilities, error)
	NewSession(ctx context.Context, bundle SessionBundle, host SessionHost) (Session, error)
}
