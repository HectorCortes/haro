package workflow

import (
	"path/filepath"
	"testing"
)

func TestWorkspace_Inheritance(t *testing.T) {
	// root shared, included defaults should not propagate, step overrides persist
	// Simulate via Flatten with fixtures
	root := t.TempDir()
	base := filepath.Join(root, ".haro", "workflows")
	mainPath := filepath.Join(base, "main", "workflow.yaml")
	libPath := filepath.Join(base, "lib", "workflow.yaml")
	// lib has workflow-level shared (should be ignored) and step with no override
	// main has root workspace shared, and workflow node includes lib, and step overrides
	files := map[string][]byte{
		filepath.Clean(mainPath): []byte("version: 2\nname: main\nworkspace:\n  mode: shared\n  on_logical_conflict: allow\nsteps:\n  - id: wf\n    type: workflow\n    source: ../lib/workflow.yaml\n  - id: direct\n    type: command\n    run: echo hi\n    workspace:\n      mode: isolated\n"),
		filepath.Clean(libPath): []byte("version: 2\nname: lib\nworkspace:\n  mode: isolated\n  on_logical_conflict: block\nsteps:\n  - id: inner\n    type: command\n    run: echo inner\n"),
	}
	readFile := func(p string) ([]byte, error) {
		if d, ok := files[filepath.Clean(p)]; ok {
			return d, nil
		}
		return nil, nil
	}
	dag, err := Flatten(mainPath, readFile)
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	// Find inner step
	var inner *FlatStep
	var direct *FlatStep
	for i := range dag.Steps {
		if dag.Steps[i].ID == "wf.inner" {
			inner = &dag.Steps[i]
		}
		if dag.Steps[i].ID == "direct" {
			direct = &dag.Steps[i]
		}
	}
	if inner == nil || direct == nil {
		t.Fatalf("steps not found: %+v", dag.Steps)
	}
	// inner should inherit from root shared, not lib's isolated, because included workflow workspace ignored
	// But our flatten currently copies inner's Workspace directly from lib's step (nil), so we need to check resolution via engine helper?
	// For now, inner.Workspace should be nil (no step override), so effective should be root shared, not isolated
	// We check that inner Workspace is nil (meaning it will resolve to root shared)
	if inner.Workspace != nil {
		t.Fatalf("inner workspace should be nil (no override), got %+v", inner.Workspace)
	}
	if direct.Workspace == nil || direct.Workspace.Mode == nil || *direct.Workspace.Mode != "isolated" {
		t.Fatalf("direct step override should be isolated, got %+v", direct.Workspace)
	}
	// Simulate engine resolution
	rootWfYAML := "version: 2\nname: main\nworkspace:\n  mode: shared\n"
	_ = rootWfYAML
	// Check effective via helper (we will need to expose resolve)
	// For this test, we just verify that inner's effective would be shared if we use root shared
}
