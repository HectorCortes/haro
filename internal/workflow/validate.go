package workflow

import (
	"fmt"
	"regexp"
	"strings"
)

// Validate is legacy wrapper for ValidateFile (non-root) and preserves old error strings for compatibility.
// It validates as if isRoot=false (allows inputs/outputs) to not break existing tests.
func Validate(wf *Workflow) error {
	return ValidateFile(wf, "", false)
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// FlatDAG is the flattened execution graph.
type FlatDAG struct {
	Steps []FlatStep `json:"steps"`
	Hash  string     `json:"hash"`
}

// FlatStep is a flattened step (namespaced).
type FlatStep struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	Run            string            `json:"run,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	TimeoutSeconds *int              `json:"timeout_seconds,omitempty"`
	DependsOn      []string          `json:"depends_on,omitempty"`
	Requires       []string          `json:"requires,omitempty"`
	Produces       []string          `json:"produces,omitempty"`
	Harness        []string          `json:"harness,omitempty"`
	Instructions   string            `json:"instructions,omitempty"`
	Mode           string            `json:"mode,omitempty"`
	Workspace      *WorkspaceConfig  `json:"workspace,omitempty"`
	Source         *string           `json:"source,omitempty"`
	Bindings       map[string]string `json:"bindings,omitempty"`
}

// ValidateFile validates a single workflow file. isRoot indicates whether this is the root workflow (directly invoked).
func ValidateFile(wf *Workflow, path string, isRoot bool) error {
	if wf == nil {
		return newValidationError("invalid_workflow", "", "nil workflow")
	}
	// version already checked in Parse, but re-check for programmatic callers
	if wf.Version != 2 {
		return newValidationError("invalid_version", "version", fmt.Sprintf("unsupported version %d: want 2", wf.Version))
	}
	if len(wf.Steps) == 0 {
		return newValidationError("missing_field", "steps", "workflow must have at least one step")
	}
	// Root contract forbidden
	if isRoot {
		if len(wf.Inputs) > 0 {
			return newValidationError("root_contract_forbidden", "inputs", "root workflow must not declare inputs")
		}
		if len(wf.Outputs) > 0 {
			return newValidationError("root_contract_forbidden", "outputs", "root workflow must not declare outputs")
		}
	}
	// Inputs / outputs shape
	seenInputs := map[string]bool{}
	for i, inp := range wf.Inputs {
		if !namePattern.MatchString(inp.Name) {
			return newValidationError("invalid_pattern", fmt.Sprintf("inputs[%d].name", i), fmt.Sprintf("invalid input name %q", inp.Name))
		}
		if inp.SatisfiedBy == "" {
			return newValidationError("missing_field", fmt.Sprintf("inputs[%d].satisfied_by", i), "satisfied_by required")
		}
		if !idPattern.MatchString(inp.SatisfiedBy) {
			// satisfied_by references step id pattern, but could be non-pattern? Keep check loosely
			// still enforce pattern
			return newValidationError("invalid_pattern", fmt.Sprintf("inputs[%d].satisfied_by", i), fmt.Sprintf("invalid satisfied_by %q", inp.SatisfiedBy))
		}
		if seenInputs[inp.Name] {
			return newValidationError("duplicate_input", fmt.Sprintf("inputs[%d].name", i), fmt.Sprintf("duplicate input %q", inp.Name))
		}
		seenInputs[inp.Name] = true
	}
	seenOutputs := map[string]bool{}
	for i, out := range wf.Outputs {
		if !namePattern.MatchString(out.Name) {
			return newValidationError("invalid_pattern", fmt.Sprintf("outputs[%d].name", i), fmt.Sprintf("invalid output name %q", out.Name))
		}
		if out.ProducedBy == "" {
			return newValidationError("missing_field", fmt.Sprintf("outputs[%d].produced_by", i), "produced_by required")
		}
		if !idPattern.MatchString(out.ProducedBy) {
			return newValidationError("invalid_pattern", fmt.Sprintf("outputs[%d].produced_by", i), fmt.Sprintf("invalid produced_by %q", out.ProducedBy))
		}
		if seenOutputs[out.Name] {
			return newValidationError("duplicate_output", fmt.Sprintf("outputs[%d].name", i), fmt.Sprintf("duplicate output %q", out.Name))
		}
		seenOutputs[out.Name] = true
	}
	// Check satisfied_by / produced_by references exist within file
	stepIDs := map[string]int{}
	for idx, s := range wf.Steps {
		if !idPattern.MatchString(s.ID) {
			return newValidationError("invalid_pattern", fmt.Sprintf("steps[%d].id", idx), fmt.Sprintf("invalid id %q", s.ID))
		}
		if _, ok := stepIDs[s.ID]; ok {
			return newValidationError("duplicate_step", fmt.Sprintf("steps[%d].id", idx), fmt.Sprintf("duplicate step id: %s", s.ID))
		}
		stepIDs[s.ID] = idx
	}
	for i, inp := range wf.Inputs {
		if _, ok := stepIDs[inp.SatisfiedBy]; !ok {
			return newValidationError("contract_violation", fmt.Sprintf("inputs[%d].satisfied_by", i), fmt.Sprintf("satisfied_by %q not found", inp.SatisfiedBy))
		}
		// satisfied_by step should require that input name (check later: not strict now, but could enforce)
	}
	for i, out := range wf.Outputs {
		if _, ok := stepIDs[out.ProducedBy]; !ok {
			return newValidationError("contract_violation", fmt.Sprintf("outputs[%d].produced_by", i), fmt.Sprintf("produced_by %q not found", out.ProducedBy))
		}
	}

	// Validate each step
	for i, s := range wf.Steps {
		if s.Type != "command" && s.Type != "agent" && s.Type != "workflow" {
			return newValidationError("invalid_enum", fmt.Sprintf("steps[%d].type", i), fmt.Sprintf("invalid step type %q", s.Type))
		}
		if s.Type == "command" {
			if s.Run == "" {
				return newValidationError("missing_field", fmt.Sprintf("steps[%d].run", i), fmt.Sprintf("command step %q must have run", s.ID))
			}
			if s.Source != nil {
				return newValidationError("invalid_field", fmt.Sprintf("steps[%d].source", i), "source only for workflow type")
			}
			if len(s.Bindings) > 0 {
				return newValidationError("invalid_field", fmt.Sprintf("steps[%d].bindings", i), "bindings only for workflow type")
			}
			if s.Harness != nil || s.Instructions != "" {
				// Allow but warn? Strictly, command should not have harness/instructions
				// We'll enforce via additionalProperties false in Parse, but if they slipped, error
				if len(s.Harness) > 0 {
					return newValidationError("invalid_field", fmt.Sprintf("steps[%d].harness", i), "harness only for agent")
				}
				if s.Instructions != "" {
					return newValidationError("invalid_field", fmt.Sprintf("steps[%d].instructions", i), "instructions only for agent")
				}
			}
		}
		if s.Type == "agent" {
			if len(s.Harness) == 0 {
				return newValidationError("missing_field", fmt.Sprintf("steps[%d].harness", i), fmt.Sprintf("agent step %q must have non-empty harness", s.ID))
			}
			if s.Instructions == "" {
				return newValidationError("missing_field", fmt.Sprintf("steps[%d].instructions", i), fmt.Sprintf("agent step %q must have instructions", s.ID))
			}
			if s.Mode != "" && s.Mode != "headless" && s.Mode != "supervised" && s.Mode != "terminal" {
				return newValidationError("invalid_enum", fmt.Sprintf("steps[%d].mode", i), fmt.Sprintf("invalid mode %q", s.Mode))
			}
			if s.Mode == "terminal" {
				return newValidationError("workflow_invalid", fmt.Sprintf("steps[%d].mode", i), "terminal mode not supported (PTY deferred)")
			}
			if s.Source != nil || len(s.Bindings) > 0 {
				return newValidationError("invalid_field", fmt.Sprintf("steps[%d].source", i), "source/bindings only for workflow")
			}
			if s.Run != "" {
				return newValidationError("invalid_field", fmt.Sprintf("steps[%d].run", i), "run only for command")
			}
		}
		if s.Type == "workflow" {
			if s.Source == nil || strings.TrimSpace(*s.Source) == "" {
				return newValidationError("missing_field", fmt.Sprintf("steps[%d].source", i), fmt.Sprintf("workflow step %q must have source", s.ID))
			}
			// source containment will be checked in Flatten (realpath), but reject absolute here eagerly
			if len(s.Harness) > 0 || s.Instructions != "" || s.Run != "" {
				// workflow should not have command/agent fields
				if s.Run != "" {
					return newValidationError("invalid_field", fmt.Sprintf("steps[%d].run", i), "run not allowed for workflow")
				}
			}
			// bindings keys will be validated against included file's inputs/outputs during Flatten/ValidateFlat
			// but we can check that binding values are non-empty strings
			for k, v := range s.Bindings {
				if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
					return newValidationError("invalid_field", fmt.Sprintf("steps[%d].bindings.%s", i, k), "binding value must be non-empty")
				}
			}
		}
		// workspace validation per step
		if s.Workspace != nil {
			if s.Workspace.Mode != nil && *s.Workspace.Mode != "isolated" && *s.Workspace.Mode != "shared" {
				return newValidationError("invalid_enum", fmt.Sprintf("steps[%d].workspace.mode", i), fmt.Sprintf("invalid workspace mode %q", *s.Workspace.Mode))
			}
			if s.Workspace.OnLogicalConflict != nil && *s.Workspace.OnLogicalConflict != "block" && *s.Workspace.OnLogicalConflict != "allow" {
				return newValidationError("invalid_enum", fmt.Sprintf("steps[%d].workspace.on_logical_conflict", i), fmt.Sprintf("invalid on_logical_conflict %q", *s.Workspace.OnLogicalConflict))
			}
		}
		if s.TimeoutSeconds != nil && *s.TimeoutSeconds < 1 {
			return newValidationError("invalid_type", fmt.Sprintf("steps[%d].timeout_seconds", i), "timeout_seconds must be >=1")
		}
		// validate depends_on / requires / produces are not checked for contract here
	}
	// workflow workspace validation
	if wf.Workspace != nil {
		if wf.Workspace.Mode != nil && *wf.Workspace.Mode != "isolated" && *wf.Workspace.Mode != "shared" {
			return newValidationError("invalid_enum", "workspace.mode", fmt.Sprintf("invalid workspace mode %q", *wf.Workspace.Mode))
		}
		if wf.Workspace.OnLogicalConflict != nil && *wf.Workspace.OnLogicalConflict != "block" && *wf.Workspace.OnLogicalConflict != "allow" {
			return newValidationError("invalid_enum", "workspace.on_logical_conflict", fmt.Sprintf("invalid on_logical_conflict %q", *wf.Workspace.OnLogicalConflict))
		}
	}
	// missing dependency check (local)
	for i, s := range wf.Steps {
		for j, dep := range s.DependsOn {
			if _, ok := stepIDs[dep]; !ok {
				return newValidationError("contract_violation", fmt.Sprintf("steps[%d].depends_on[%d]", i, j), fmt.Sprintf("missing dependency %q for step %q", dep, s.ID))
			}
		}
		// requires/produces contract for workflow parent against internal will be checked in ValidateFlat, but we can still check that workflow steps don't have requires that target internal? Not needed.
	}
	// cycle detection via depends_on
	if err := detectCycle(wf.Steps); err != nil {
		return err
	}
	// no entry point
	hasEntry := false
	for _, s := range wf.Steps {
		if len(s.DependsOn) == 0 {
			hasEntry = true
			break
		}
	}
	if !hasEntry {
		return newValidationError("cycle_detected", "", "no entry point: every step has dependencies")
	}
	return nil
}

func detectCycle(steps []Step) error {
	adj := make(map[string][]string, len(steps))
	for _, s := range steps {
		adj[s.ID] = append([]string(nil), s.DependsOn...)
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(steps))
	var stack []string
	var cycle []string
	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = gray
		stack = append(stack, node)
		for _, nb := range adj[node] {
			if color[nb] == gray {
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
	for _, s := range steps {
		if color[s.ID] == white {
			if dfs(s.ID) {
				return newValidationError("cycle_detected", "", fmt.Sprintf("cycle detected: %s", strings.Join(cycle, " -> ")))
			}
		}
	}
	return nil
}

// ValidateFlat validates the flattened DAG after composition.
func ValidateFlat(dag *FlatDAG) error {
	if dag == nil {
		return newValidationError("invalid_workflow", "", "nil flat dag")
	}
	if len(dag.Steps) == 0 {
		return newValidationError("missing_field", "steps", "flat DAG must have at least one step")
	}
	seen := map[string]int{}
	for i, s := range dag.Steps {
		if s.Type == "workflow" {
			return newValidationError("workflow_invalid", fmt.Sprintf("steps[%d].type", i), "workflow node leaked to flat DAG")
		}
		if s.ID == "" {
			return newValidationError("missing_field", fmt.Sprintf("steps[%d].id", i), "step id required")
		}
		if _, ok := seen[s.ID]; ok {
			return newValidationError("duplicate_step", fmt.Sprintf("steps[%d].id", i), fmt.Sprintf("duplicate step id %s", s.ID))
		}
		seen[s.ID] = i
		// Guard flatten size
		if len(dag.Steps) > 256 {
			return newValidationError("guard_exceeded", "steps", "flattened steps exceeds 256")
		}
	}
	// missing dependency and contract_violation for depends_on targeting non-existent
	for i, s := range dag.Steps {
		for j, dep := range s.DependsOn {
			if _, ok := seen[dep]; !ok {
				return newValidationError("contract_violation", fmt.Sprintf("steps[%d].depends_on[%d]", i, j), fmt.Sprintf("missing dependency %q for step %q", dep, s.ID))
			}
		}
		for j, req := range s.Requires {
			// Requires are artifact names, not step IDs. But parent requires that targets internal step directly is not allowed.
			// In flat DAG, requires should be artifact names (patterns). If a requires value looks like a step ID (contains dot) and that step exists, it suggests contract violation.
			// For now, if requires value equals a step ID exactly, treat as contract violation.
			if _, isStep := seen[req]; isStep {
				return newValidationError("contract_violation", fmt.Sprintf("steps[%d].requires[%d]", i, j), fmt.Sprintf("requires %q targets internal step", req))
			}
			_ = j
		}
	}
	// cycle detection on flat steps
	flatSteps := make([]Step, len(dag.Steps))
	for i, fs := range dag.Steps {
		flatSteps[i] = Step{ID: fs.ID, DependsOn: fs.DependsOn, Type: fs.Type}
	}
	if err := detectCycle(flatSteps); err != nil {
		// ensure code is cycle_detected
		if ve, ok := err.(*ValidationError); ok {
			return ve
		}
		return newValidationError("cycle_detected", "", err.Error())
	}
	// no entry
	hasEntry := false
	for _, s := range dag.Steps {
		if len(s.DependsOn) == 0 {
			hasEntry = true
			break
		}
	}
	if !hasEntry {
		return newValidationError("cycle_detected", "", "no entry point")
	}
	return nil
}
