// Package factory builds the CLI-direct adapter manager for one CLI
// invocation. Only OpenCode is registered; Claude and ACP remain
// unregistered until their sessions are real. The subpackage exists to
// confine provider wiring without forcing the parent adapter package to
// import provider adapters (import cycle) and to keep provider literals
// confined to internal/adapter.
package factory

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/adapter/opencode"
	"github.com/HectorCortes/haro/internal/project"
)

// TestBinaryEnv is the hermetic test seam: when set, it takes precedence
// over the configured binary so CI never needs a real opencode install.
const TestBinaryEnv = "HARO_TEST_OPENCODE_BINARY"

// ResolveBinary applies the binary precedence: HARO_TEST_OPENCODE_BINARY,
// then the configured binary, then the default "opencode".
func ResolveBinary(configured string) string {
	if env := os.Getenv(TestBinaryEnv); env != "" {
		return env
	}
	if configured != "" {
		return configured
	}
	return opencode.DefaultBinary
}

// NewManager builds an OpenCode-only adapter manager from the project
// harness configuration: enabled harnesses are registered, probed, and each
// usable one is initialized exactly once before any session is created.
// Unavailable candidates remain registered but un-initialized; the engine
// intersects with probe results and falls through per candidate.
func NewManager(ctx context.Context, cfg *project.Config) (*adapter.Manager, error) {
	adapters := make(map[string]adapter.Adapter)
	for name, hc := range cfg.Harnesses {
		if !hc.IsEnabled() {
			continue
		}
		adapters[name] = opencode.NewAdapter(
			ResolveBinary(hc.Binary),
			hc.Env,
			time.Duration(hc.TimeoutSeconds)*time.Second,
		)
	}
	mgr := adapter.NewManager(adapters)
	probes, err := mgr.Probe(ctx)
	if err != nil {
		return nil, fmt.Errorf("probe harnesses: %w", err)
	}
	core := adapter.Capabilities{ProtocolVersion: opencode.ProtocolVersion}
	for name, pr := range probes {
		if !pr.Available {
			continue
		}
		if _, err := mgr.Initialize(ctx, name, core); err != nil {
			return nil, fmt.Errorf("initialize %s: %w", name, err)
		}
	}
	return mgr, nil
}
