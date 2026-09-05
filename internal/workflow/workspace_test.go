package workflow

import (
	"strings"
	"testing"
)

func TestWorkspaceParse(t *testing.T) {
	// valid isolated
	yaml := "version: 2\nname: demo\nworkspace:\n  mode: isolated\n  on_logical_conflict: block\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"
	wf, err := Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("parse workspace isolated: %v", err)
	}
	if wf.Workspace == nil || wf.Workspace.Mode == nil || *wf.Workspace.Mode != "isolated" {
		t.Fatalf("expected workspace mode isolated, got %v", wf.Workspace)
	}
	if wf.Workspace.OnLogicalConflict == nil || *wf.Workspace.OnLogicalConflict != "block" {
		t.Fatalf("expected on_logical_conflict block, got %v", wf.Workspace)
	}

	// step workspace override
	yaml2 := "version: 2\nname: demo\nworkspace:\n  mode: shared\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n    workspace:\n      mode: isolated\n"
	wf2, err := Parse(strings.NewReader(yaml2))
	if err != nil {
		t.Fatalf("parse step workspace: %v", err)
	}
	if wf2.Workspace == nil || *wf2.Workspace.Mode != "shared" {
		t.Fatalf("workflow mode shared")
	}
	if wf2.Steps[0].Workspace == nil || *wf2.Steps[0].Workspace.Mode != "isolated" {
		t.Fatalf("step workspace isolated override")
	}

	// defaults when absent: should be nil, resolver will default to isolated/block
	yaml3 := "version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"
	wf3, err := Parse(strings.NewReader(yaml3))
	if err != nil {
		t.Fatalf("parse minimal: %v", err)
	}
	if wf3.Workspace != nil {
		t.Fatalf("expected nil workspace for minimal, got %v", wf3.Workspace)
	}

	// invalid mode should be caught by Validate
	invalidMode := "version: 2\nname: demo\nworkspace:\n  mode: bogus\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"
	wf4, _ := Parse(strings.NewReader(invalidMode))
	if err := Validate(wf4); err == nil {
		t.Fatalf("expected validate error for bogus mode")
	}

	invalidConflict := "version: 2\nname: demo\nworkspace:\n  mode: isolated\n  on_logical_conflict: bogus\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"
	wf5, _ := Parse(strings.NewReader(invalidConflict))
	if err := Validate(wf5); err == nil {
		t.Fatalf("expected validate error for bogus on_logical_conflict")
	}

	// inheritance resolver
	yaml6 := "version: 2\nname: demo\nworkspace:\n  mode: shared\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n  - id: s2\n    type: command\n    run: echo hi\n    workspace:\n      mode: isolated\n"
	wf6, _ := Parse(strings.NewReader(yaml6))
	if err := Validate(wf6); err != nil {
		t.Fatalf("validate wf6: %v", err)
	}
	// system default isolated, workflow shared, step overrides
	// step s1 should inherit shared from workflow
	m1, c1 := ResolveWorkspace(wf6, "s1")
	if m1 != "shared" || c1 != "block" {
		t.Fatalf("resolve s1 expected shared/block got %s/%s", m1, c1)
	}
	// s2 overrides to isolated
	m2, c2 := ResolveWorkspace(wf6, "s2")
	if m2 != "isolated" || c2 != "block" {
		t.Fatalf("resolve s2 expected isolated/block got %s/%s", m2, c2)
	}

	// workflow isolated, step shared
	yaml7 := "version: 2\nname: demo\nworkspace:\n  mode: isolated\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n    workspace:\n      mode: shared\n      on_logical_conflict: allow\n"
	wf7, _ := Parse(strings.NewReader(yaml7))
	_ = Validate(wf7)
	m3, c3 := ResolveWorkspace(wf7, "s1")
	if m3 != "shared" || c3 != "allow" {
		t.Fatalf("resolve s1 isolated->shared allow got %s/%s", m3, c3)
	}
}

func TestWorkspaceDefaults(t *testing.T) {
	yaml := "version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"
	wf, _ := Parse(strings.NewReader(yaml))
	_ = Validate(wf)
	m, c := ResolveWorkspace(wf, "s1")
	if m != "isolated" || c != "block" {
		t.Fatalf("default isolated/block got %s/%s", m, c)
	}
}
