package adapter

import (
	"context"
	"fmt"
	"sync"
)

// Manager owns CLI-direct adapter lifecycle.
// It ensures one bilateral initialize per adapter before any session.
type Manager struct {
	mu           sync.Mutex
	adapters     map[string]Adapter
	initialized  map[string]Capabilities
	negotiated   map[string]Capabilities
	initCalled   map[string]bool
}

// NewManager creates a manager with registered adapters.
func NewManager(adapters map[string]Adapter) *Manager {
	return &Manager{
		adapters:    adapters,
		initialized: make(map[string]Capabilities),
		negotiated:  make(map[string]Capabilities),
		initCalled:  make(map[string]bool),
	}
}

// Probe probes all registered adapters.
func (m *Manager) Probe(ctx context.Context) (map[string]ProbeResult, error) {
	results := make(map[string]ProbeResult)
	for name, a := range m.adapters {
		pr, err := a.Probe(ctx)
		if err != nil {
			return nil, fmt.Errorf("probe %s: %w", name, err)
		}
		results[name] = pr
	}
	return results, nil
}

// Initialize performs bilateral negotiation for the named adapter.
// It is called exactly once per adapter; subsequent calls return cached result.
func (m *Manager) Initialize(ctx context.Context, name string, core Capabilities) (Capabilities, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.initCalled[name] {
		return m.negotiated[name], nil
	}
	adapter, ok := m.adapters[name]
	if !ok {
		return Capabilities{}, fmt.Errorf("adapter %q not found", name)
	}
	neg, err := adapter.Initialize(ctx, core)
	if err != nil {
		return Capabilities{}, err
	}
	// Also run local negotiation check for additive preservation (optional)
	// We trust adapter's returned caps as negotiated; but validate additive
	m.initialized[name] = core
	m.negotiated[name] = neg
	m.initCalled[name] = true
	return neg, nil
}

// NewSession creates a session for the named adapter, requiring prior Initialize.
func (m *Manager) NewSession(ctx context.Context, name string, bundle SessionBundle, host SessionHost) (Session, error) {
	m.mu.Lock()
	initialized := m.initCalled[name]
	negotiated := m.negotiated[name]
	m.mu.Unlock()
	if !initialized {
		return nil, ErrNotInitialized
	}
	adapter, ok := m.adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter %q not found", name)
	}
	// Wrap host to enforce fail-closed permission gating based on negotiated caps
	wrappedHost := &gatedHost{inner: host, caps: negotiated}
	return adapter.NewSession(ctx, bundle, wrappedHost)
}

// gatedHost enforces fail-closed permission.
type gatedHost struct {
	inner SessionHost
	caps  Capabilities
}

func (g *gatedHost) RequestPermission(ctx context.Context, req PermissionRequest) (PermissionDecision, error) {
	if !g.caps.Permission {
		return PermissionDecision{}, ErrUnsupportedCapability
	}
	return g.inner.RequestPermission(ctx, req)
}

// Negotiated returns cached negotiated caps for name.
func (m *Manager) Negotiated(name string) (Capabilities, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.negotiated[name]
	return c, ok
}
