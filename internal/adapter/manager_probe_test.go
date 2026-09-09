package adapter

import (
	"context"
	"errors"
	"testing"
)

// TestManager_ProbePerCandidateFallback covers task 2.5: a probe failure for
// one candidate must degrade to Available:false for that candidate only and
// never abort probing of the rest (clean fallback, never abort-all).
func TestManager_ProbePerCandidateFallback(t *testing.T) {
	ctx := context.Background()
	ok := &fakeAdapter{probeResult: ProbeResult{Available: true, Version: "1.0", Capabilities: Capabilities{ProtocolVersion: 1}}}
	broken := &fakeAdapter{probeErr: errors.New("probe exploded")}
	mgr := NewManager(map[string]Adapter{"good": ok, "broken": broken})
	results, err := mgr.Probe(ctx)
	if err != nil {
		t.Fatalf("Probe must not abort on a single candidate failure: %v", err)
	}
	if !results["good"].Available {
		t.Fatalf("good candidate must stay available")
	}
	if results["broken"].Available {
		t.Fatalf("broken candidate must degrade to Available:false")
	}
	// Registered accessor used by the engine intersection.
	if !mgr.Registered("good") || !mgr.Registered("broken") {
		t.Fatalf("Registered must report registered adapters")
	}
	if mgr.Registered("unknown") {
		t.Fatalf("unregistered name must be reported as unknown")
	}
}
