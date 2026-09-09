// Package factory builds the CLI-direct adapter manager for one CLI
// invocation. OpenCode and Claude are registered now that their sessions are
// real; ACP remains unregistered until it implements a real session
// contract. The subpackage exists to confine provider wiring without forcing
// the parent adapter package to import provider adapters (import cycle) and
// to keep provider literals confined to internal/adapter.
package factory

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/adapter/claude"
	"github.com/HectorCortes/haro/internal/adapter/opencode"
	"github.com/HectorCortes/haro/internal/project"
)

// TestBinaryEnv is the hermetic test seam for OpenCode: when set, it takes
// precedence over the configured binary so CI never needs a real opencode
// install.
const TestBinaryEnv = "HARO_TEST_OPENCODE_BINARY"

// ClaudeTestBinaryEnv is the hermetic test seam for Claude: when set, it
// takes precedence over the configured binary so CI never needs a real
// claude install.
const ClaudeTestBinaryEnv = "HARO_TEST_CLAUDE_BINARY"

// ResolveBinary applies the OpenCode binary precedence:
// HARO_TEST_OPENCODE_BINARY, then the configured binary, then the default
// "opencode".
func ResolveBinary(configured string) string {
	if env := os.Getenv(TestBinaryEnv); env != "" {
		return env
	}
	if configured != "" {
		return configured
	}
	return opencode.DefaultBinary
}

// ResolveClaudeBinary applies the per-harness Claude binary precedence:
// HARO_TEST_CLAUDE_BINARY, then the configured binary, then the default
// "claude". The OpenCode seam never affects this resolution.
func ResolveClaudeBinary(configured string) string {
	if env := os.Getenv(ClaudeTestBinaryEnv); env != "" {
		return env
	}
	if configured != "" {
		return configured
	}
	return claude.DefaultBinary
}

// buildAdapter maps one enabled harness record to its adapter. An enabled
// key named exactly "claude" or "claudecode" gets the Claude adapter; every
// other enabled key keeps the OpenCode adapter so existing arbitrary
// OpenCode names are preserved. No ACP adapter is built.
func buildAdapter(name string, hc project.HarnessConfig) adapter.Adapter {
	timeout := time.Duration(hc.TimeoutSeconds) * time.Second
	switch name {
	case "claude", "claudecode":
		return claude.NewAdapter(ResolveClaudeBinary(hc.Binary), hc.Env, timeout)
	default:
		return opencode.NewAdapter(ResolveBinary(hc.Binary), hc.Env, timeout)
	}
}

// NewManager builds the adapter manager from the project harness
// configuration: enabled harnesses are registered, probed, and each usable
// one is initialized exactly once before any session is created.
// Unavailable candidates remain registered but un-initialized; the engine
// intersects with probe results and falls through per candidate.
func NewManager(ctx context.Context, cfg *project.Config) (*adapter.Manager, error) {
	adapters := make(map[string]adapter.Adapter)
	for name, hc := range cfg.Harnesses {
		if !hc.IsEnabled() {
			continue
		}
		adapters[name] = buildAdapter(name, hc)
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
