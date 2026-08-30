package contract

import (
	"context"
	"testing"

	"github.com/HectorCortes/haro/internal/adapter"
)

type fakeAdapterForContract struct {
	caps adapter.Capabilities
}

func (f *fakeAdapterForContract) Probe(_ context.Context) (adapter.ProbeResult, error) {
	return adapter.ProbeResult{Available: true, Version: "fake", Capabilities: adapter.Capabilities{ProtocolVersion: 1}}, nil
}
func (f *fakeAdapterForContract) Initialize(_ context.Context, core adapter.Capabilities) (adapter.Capabilities, error) {
	return adapter.Negotiate(core, f.caps)
}
func (f *fakeAdapterForContract) NewSession(_ context.Context, bundle adapter.SessionBundle, host adapter.SessionHost) (adapter.Session, error) {
	return &fakeSessionForContract{host: host, caps: f.caps}, nil
}

type fakeSessionForContract struct {
	host adapter.SessionHost
	caps adapter.Capabilities
}

func (s *fakeSessionForContract) Prompt(_ context.Context, input adapter.PromptInput) (<-chan adapter.SessionEvent, error) {
	ch := make(chan adapter.SessionEvent, 1)
	ch <- adapter.SessionEvent{Cursor: 1, Type: "completed", Payload: []byte(input.Text)}
	close(ch)
	return ch, nil
}
func (s *fakeSessionForContract) Cancel(_ context.Context) error { return nil }
func (s *fakeSessionForContract) LoadPrevious(_ context.Context, id string) error {
	if !s.caps.LoadSession {
		return adapter.ErrUnsupportedCapability
	}
	return nil
}
func (s *fakeSessionForContract) Terminal(_ context.Context) (adapter.TerminalHandle, error) {
	if !s.caps.Terminal {
		return adapter.TerminalHandle{}, adapter.ErrUnsupportedCapability
	}
	return adapter.TerminalHandle{}, nil
}

func TestContractSuite(t *testing.T) {
	factory := func() adapter.Adapter {
		return &fakeAdapterForContract{caps: adapter.Capabilities{ProtocolVersion: 1, Permission: true, Terminal: false, LoadSession: false}}
	}
	RunSuite(t, factory)
}

func TestContractSuite_Gating(t *testing.T) {
	factory := func() adapter.Adapter {
		return &fakeAdapterForContract{caps: adapter.Capabilities{ProtocolVersion: 1, Permission: false, Terminal: false, LoadSession: false}}
	}
	RunSuite(t, factory)
}
