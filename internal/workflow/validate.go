package workflow

import (
	"fmt"
	"strings"
)

// Validate checks workflow DAG correctness: duplicate IDs, missing dependencies,
// cycles, and no entry point. It assumes Parse already validated version and
// non-empty steps, but re-checks empty.
func Validate(wf *Workflow) error {
	if wf == nil {
		return fmt.Errorf("nil workflow")
	}
	if len(wf.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}
	// duplicate check
	seen := make(map[string]int, len(wf.Steps))
	for _, s := range wf.Steps {
		if s.ID == "" {
			return fmt.Errorf("step with empty id")
		}
		if _, ok := seen[s.ID]; ok {
			return fmt.Errorf("duplicate step id: %s", s.ID)
		}
		seen[s.ID] = 1
		if s.Type != "command" && s.Type != "agent" && s.Type != "workflow" {
			return fmt.Errorf("invalid step type %q for step %q", s.Type, s.ID)
		}
		if s.Type == "agent" {
			if len(s.Harness) == 0 {
				return fmt.Errorf("agent step %q must have non-empty harness", s.ID)
			}
			if s.Instructions == "" {
				return fmt.Errorf("agent step %q must have instructions", s.ID)
			}
			if s.Mode != "" && s.Mode != "headless" && s.Mode != "supervised" && s.Mode != "terminal" {
				return fmt.Errorf("invalid mode %q for agent step %q", s.Mode, s.ID)
			}
			if s.Mode == "terminal" {
				return fmt.Errorf("terminal mode not supported (PTY deferred) for step %q", s.ID)
			}
		}
		if s.Type == "command" {
			if s.Run == "" {
				return fmt.Errorf("command step %q must have run", s.ID)
			}
		}
	}
	// missing dependency check
	for _, s := range wf.Steps {
		for _, dep := range s.DependsOn {
			if _, ok := seen[dep]; !ok {
				return fmt.Errorf("missing dependency %q for step %q", dep, s.ID)
			}
		}
	}
	// cycle detection via DFS
	// Build adjacency: edge from step -> dependencies (or dependencies -> step for topological?)
	// For cycle detection we need to follow depends_on edges: step depends on dep, so edge step->dep.
	// Cycle exists if traversing dependencies returns to start.
	adj := make(map[string][]string, len(wf.Steps))
	for _, s := range wf.Steps {
		adj[s.ID] = append([]string(nil), s.DependsOn...)
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(wf.Steps))
	var stack []string
	var cycle []string
	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = gray
		stack = append(stack, node)
		for _, nb := range adj[node] {
			if color[nb] == gray {
				// found cycle: extract from nb to end
				idx := -1
				for i, v := range stack {
					if v == nb {
						idx = i
						break
					}
				}
				if idx >= 0 {
					cycle = append([]string(nil), stack[idx:]...)
					cycle = append(cycle, nb)
				} else {
					cycle = []string{nb, node, nb}
				}
				return true
			}
			if color[nb] == white {
				if dfs(nb) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = black
		return false
	}
	for _, s := range wf.Steps {
		if color[s.ID] == white {
			if dfs(s.ID) {
				return fmt.Errorf("cycle detected: %s", strings.Join(cycle, " -> "))
			}
		}
	}
	// no entry point: at least one step must have no dependencies
	hasEntry := false
	for _, s := range wf.Steps {
		if len(s.DependsOn) == 0 {
			hasEntry = true
			break
		}
	}
	if !hasEntry {
		return fmt.Errorf("no entry point: every step has dependencies")
	}
	return nil
}
