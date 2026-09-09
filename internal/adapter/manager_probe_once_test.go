package adapter

import (
	"context"
	"errors"
	"testing"
)

// TestManager_ProbeCachedOnce covers the F-01 single-probe lifecycle: the
// manager probes each registered adapter exactly once per manager lifetime;
// later Probe calls (and ProbeResults readers) return the cached snapshot
// without touching the adapters again.
func TestManager_ProbeCachedOnce(t *testing.T) {
	ctx := context.Background()
	ok := &fakeAdapter{probeResult: ProbeResult{Available: true, Capabilities: Capabilities{ProtocolVersion: 1}}}
	broken := &fakeAdapter{probeErr: errors.New("probe exploded")}
	mgr := NewManager(map[string]Adapter{"good": ok, "broken": broken})

	first, err := mgr.Probe(ctx)
	if err != nil {
		t.Fatalf("first probe: %v", err)
	}
	if !first["good"].Available || first["broken"].Available {
		t.Fatalf("first probe results wrong: %+v", first)
	}
	// A second Probe call must return the cached snapshot and must not
	// re-invoke the adapters (single probe per manager lifetime).
	second, err := mgr.Probe(ctx)
	if err != nil {
		t.Fatalf("second probe: %v", err)
	}
	if !second["good"].Available || second["broken"].Available {
		t.Fatalf("cached probe results wrong: %+v", second)
	}
	if ok.probeCount != 1 || broken.probeCount != 1 {
		t.Fatalf("adapters probed %d/%d times, want exactly 1 each", ok.probeCount, broken.probeCount)
	}
	// ProbeResults exposes the captured availability state without probing.
	cached := mgr.ProbeResults()
	if !cached["good"].Available || cached["broken"].Available {
		t.Fatalf("ProbeResults snapshot wrong: %+v", cached)
	}
	if ok.probeCount != 1 {
		t.Fatalf("ProbeResults must not re-probe, count = %d", ok.probeCount)
	}
}
