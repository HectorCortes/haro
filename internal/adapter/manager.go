package adapter

import (
	"context"
	"fmt"
	"sync"
)

// Manager owns CLI-direct adapter lifecycle.
// It ensures one bilateral initialize per adapter before any session, and a
// single probe of each registered adapter per manager lifetime.
type Manager struct {
	mu           sync.Mutex
	probeMu      sync.Mutex
	adapters     map[string]Adapter
	initialized  map[string]Capabilities
	negotiated   map[string]Capabilities
	initCalled   map[string]bool
	probeResults map[string]ProbeResult
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

// Probe probes all registered adapters exactly once per manager lifetime
// and caches the availability snapshot. Subsequent Probe calls and
// ProbeResults readers return the cached snapshot without touching the
// adapters again (F-01: one probe per harness per CLI invocation). A
// per-candidate probe failure degrades that candidate to Available:false
// (clean fallback) and never aborts probing of the remaining candidates.
func (m *Manager) Probe(ctx context.Context) (map[string]ProbeResult, error) {
	m.probeMu.Lock()
	defer m.probeMu.Unlock()
	if m.probeResults != nil {
		return m.probeResults, nil
	}
	results := make(map[string]ProbeResult)
	for name, a := range m.adapters {
		pr, err := a.Probe(ctx)
		if err != nil {
			results[name] = ProbeResult{Available: false}
			continue
		}
		results[name] = pr
	}
	m.mu.Lock()
	m.probeResults = results
	m.mu.Unlock()
	return results, nil
}

// ProbeResults returns the availability snapshot captured by the single
// probe of the manager lifetime without probing again. It returns nil when
// no probe has run yet; consumers must treat missing names as unavailable.
func (m *Manager) ProbeResults() map[string]ProbeResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.probeResults
}

// Registered reports whether the named adapter is registered with the
// manager. Unregistered harness names remain unavailable for sessions.
func (m *Manager) Registered(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.adapters[name]
	return ok
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
