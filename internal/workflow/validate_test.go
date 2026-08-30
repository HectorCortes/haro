package workflow

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Run("valid single", func(t *testing.T) {
		wf, err := Parse(strings.NewReader("version: 2\nname: x\nsteps:\n  - id: a\n    type: command\n    run: echo hi\n"))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := Validate(wf); err != nil {
			t.Fatalf("Validate valid = %v, want nil", err)
		}
	})

	t.Run("valid dag", func(t *testing.T) {
		yaml := "version: 2\nname: x\nsteps:\n  - id: a\n    type: command\n    run: echo a\n  - id: b\n    type: command\n    run: echo b\n    depends_on: [a]\n  - id: c\n    type: command\n    run: echo c\n    depends_on: [a]\n"
		wf, _ := Parse(strings.NewReader(yaml))
		if err := Validate(wf); err != nil {
			t.Fatalf("Validate dag = %v, want nil", err)
		}
	})

	t.Run("duplicate id", func(t *testing.T) {
		yaml := "version: 2\nname: x\nsteps:\n  - id: a\n    type: command\n    run: echo a\n  - id: a\n    type: command\n    run: echo dup\n"
		wf, _ := Parse(strings.NewReader(yaml))
		err := Validate(wf)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			t.Fatalf("expected duplicate error, got %v", err)
		}
	})

	t.Run("missing dependency", func(t *testing.T) {
		yaml := "version: 2\nname: x\nsteps:\n  - id: a\n    type: command\n    run: echo a\n    depends_on: [missing]\n"
		wf, _ := Parse(strings.NewReader(yaml))
		err := Validate(wf)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "missing") {
			if err != nil && strings.Contains(err.Error(), "unknown") {
				// also acceptable: unknown dependency
			} else {
				t.Fatalf("expected missing dependency error, got %v", err)
			}
		}
	})

	t.Run("cycle", func(t *testing.T) {
		yaml := "version: 2\nname: x\nsteps:\n  - id: a\n    type: command\n    run: echo a\n    depends_on: [b]\n  - id: b\n    type: command\n    run: echo b\n    depends_on: [a]\n"
		wf, _ := Parse(strings.NewReader(yaml))
		err := Validate(wf)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "cycle") {
			t.Fatalf("expected cycle error, got %v", err)
		}
	})

	t.Run("cycle three", func(t *testing.T) {
		yaml := "version: 2\nname: x\nsteps:\n  - id: a\n    type: command\n    run: echo a\n    depends_on: [c]\n  - id: b\n    type: command\n    run: echo b\n    depends_on: [a]\n  - id: c\n    type: command\n    run: echo c\n    depends_on: [b]\n"
		wf, _ := Parse(strings.NewReader(yaml))
		err := Validate(wf)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "cycle") {
			t.Fatalf("expected cycle error (3), got %v", err)
		}
	})

	t.Run("no entry point", func(t *testing.T) {
		// All steps depend on something — no entry. But also each dependency exists.
		// With 2 steps each depending on the other, that's cycle already. Need case where deps exist but none is entry yet not cycle?
		// Actually any DAG must have at least one entry. If every node has a dependency, and graph is acyclic, it must still have entry.
		// For test, use single step that depends on itself (cycle covers no-entry). Alternatively create 2 steps where each depends on the other but still cycle.
		// Spec says "no entry point" as distinct from cycle. Could be workflow where all steps have depends_on.
		// For DAG with a->b->c->a would be cycle. For no-entry without cycle impossible unless empty? But task requires separate check.
		// We'll test self-dependency as both missing? Actually self depends: cycle.
		// Let's test 2 steps where a depends on b and b depends on a is cycle. We already test.
		// For explicit no-entry non-cycle, consider workflow where steps are: a depends on b, b depends on a is still cycle.
		// So to test no-entry without cycle we need at least 1 step that depends on existing but graph still has entry? Actually if every step has depends_on, the graph must have cycle if it's finite, because you can follow dependencies backwards infinitely.
		// Simpler: test single step with depends_on that points to itself is cycle, but spec wants "no entry point" maybe means workflow with zero steps (already checked) or all steps have dependencies that are self-consistent?
		// We'll implement Validate to flag "no entry point" when no step has empty depends_on.
		// Triangulate with a valid workflow that has entry vs one where every step has a dependency but acyclic with external missing would already be missing.
		// For this test we use: a->b, b->a is cycle; to isolate no-entry we need a graph that is not cycle but has no entry — impossible DAG property.
		// So we test that a workflow where every step depends on another valid step but still forms cycle should report cycle (already). We'll test a special case: 1 step that depends_on [a] (self-cycle) should be cycle.
		yaml := "version: 2\nname: x\nsteps:\n  - id: a\n    type: command\n    run: echo a\n    depends_on: [a]\n"
		wf, _ := Parse(strings.NewReader(yaml))
		err := Validate(wf)
		if err == nil {
			t.Fatalf("expected error for self-dependency/no-entry, got nil")
		}
		// acceptable errors: cycle or no entry
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "cycle") && !strings.Contains(msg, "entry") {
			t.Fatalf("expected cycle or no-entry error, got %v", err)
		}
	})
}
