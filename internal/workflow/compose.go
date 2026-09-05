package workflow

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ReadFile is injected file reader for pure flattening.
type ReadFile func(string) ([]byte, error)

// Flatten recursively expands workflow nodes into a flat DAG.
func Flatten(rootPath string, readFile ReadFile) (*FlatDAG, error) {
	if readFile == nil {
		readFile = os.ReadFile
	}
	cleanRoot := filepath.Clean(rootPath)
	root := findRoot(cleanRoot)
	base := filepath.Join(root, ".haro", "workflows")
	stack := map[string]bool{}
	expansions := 0
	totalSteps := 0
	steps, err := flattenRec(cleanRoot, "", stack, 0, &expansions, &totalSteps, root, base, readFile)
	if err != nil {
		return nil, err
	}
	if len(steps) > 256 {
		return nil, newValidationError("guard_exceeded", "steps", "flattened steps exceeds 256")
	}
	// Stable topological sort (Kahn) to ensure deterministic order.
	sorted, err := topoSort(steps)
	if err != nil {
		return nil, err
	}
	hash, err := ComputeHash(&FlatDAG{Steps: sorted})
	if err != nil {
		return nil, err
	}
	return &FlatDAG{Steps: sorted, Hash: hash}, nil
}

func flattenRec(currentPath, prefix string, stack map[string]bool, depth int, expansions *int, totalSteps *int, root, base string, readFile ReadFile) ([]FlatStep, error) {
	if depth > 16 {
		return nil, newValidationError("guard_exceeded", "depth", "composition depth exceeds 16")
	}
	if *expansions > 256 {
		return nil, newValidationError("guard_exceeded", "expansions", "expansions exceeds 256")
	}
	// canonical for cycle check
	canonical := canonicalPath(currentPath)
	if stack[canonical] {
		return nil, newValidationError("cycle_detected", "", fmt.Sprintf("cycle detected: %s", canonical))
	}
	stack[canonical] = true
	defer delete(stack, canonical)

	data, err := readFile(currentPath)
	if err != nil {
		return nil, newValidationError("invalid_field", "", fmt.Sprintf("read %s: %v", currentPath, err))
	}
	// Parse with strict
	wf, err := Parse(bytes.NewReader(data))
	if err != nil {
		// Parse already returns ValidationError with code/field
		return nil, err
	}
	isRoot := depth == 0
	if err := ValidateFile(wf, currentPath, isRoot); err != nil {
		return nil, err
	}

	// Track workflow nodes for dependency rewriting at this level
	type wfInfo struct {
		nodeID      string
		producerIDs []string
		allIDs      []string
	}
	wfMap := map[string]wfInfo{}
	var result []FlatStep

	for idx, step := range wf.Steps {
		if step.Type == "workflow" {
			*expansions++
			if *expansions > 256 {
				return nil, newValidationError("guard_exceeded", "expansions", "expansions exceeds 256")
			}
			// source containment
			src := ""
			if step.Source != nil {
				src = *step.Source
			}
			if filepath.IsAbs(src) {
				return nil, newValidationError("invalid_field", fmt.Sprintf("steps[%d].source", idx), fmt.Sprintf("absolute path not allowed: %q", src))
			}
			// Resolve relative to current file's directory
			joined := filepath.Join(filepath.Dir(currentPath), src)
			cleaned := filepath.Clean(joined)
			// Check containment logically before symlink
			if isEscape(cleaned, base) {
				return nil, newValidationError("invalid_field", fmt.Sprintf("steps[%d].source", idx), fmt.Sprintf("path escapes workflows: %q", src))
			}
			canonicalJoined := canonicalPath(cleaned)
			// containment with symlink resolved
			if isEscape(canonicalJoined, canonicalPath(base)) {
				return nil, newValidationError("invalid_field", fmt.Sprintf("steps[%d].source", idx), fmt.Sprintf("path escapes workflows via symlink: %q -> %q", src, canonicalJoined))
			}
			// cycle check will happen in recursion via stack

			// Load included workflow to check bindings keys before recursion
			// Need to read included file to validate bindings against its inputs/outputs
			incData, err := readFile(cleaned)
			if err != nil {
				return nil, newValidationError("invalid_field", fmt.Sprintf("steps[%d].source", idx), fmt.Sprintf("source not found %q: %v", src, err))
			}
			incWf, err := Parse(bytes.NewReader(incData))
			if err != nil {
				return nil, err
			}
			// Validate bindings keys
			// Build sets of input/output names
			inputSet := map[string]bool{}
			outputSet := map[string]bool{}
			for _, inp := range incWf.Inputs {
				inputSet[inp.Name] = true
			}
			for _, out := range incWf.Outputs {
				outputSet[out.Name] = true
			}
			for k := range step.Bindings {
				if !inputSet[k] && !outputSet[k] {
					return nil, newValidationError("unknown_input", fmt.Sprintf("steps[%d].bindings.%s", idx, k), fmt.Sprintf("unknown binding %q", k))
				}
			}
			// Check missing_binding for inputs/outputs that are required
			for _, inp := range incWf.Inputs {
				// does consumer require this input?
				consumerID := inp.SatisfiedBy
				found := false
				for _, s := range incWf.Steps {
					if s.ID == consumerID {
						for _, req := range s.Requires {
							if req == inp.Name {
								found = true
								break
							}
						}
					}
				}
				if found {
					if _, ok := step.Bindings[inp.Name]; !ok {
						return nil, newValidationError("missing_binding", fmt.Sprintf("steps[%d].bindings.%s", idx, inp.Name), fmt.Sprintf("missing binding for input %q", inp.Name))
					}
				}
			}
			for _, out := range incWf.Outputs {
				producerID := out.ProducedBy
				found := false
				for _, s := range incWf.Steps {
					if s.ID == producerID {
						for _, prod := range s.Produces {
							if prod == out.Name {
								found = true
								break
							}
						}
					}
				}
				if found {
					if _, ok := step.Bindings[out.Name]; !ok {
						return nil, newValidationError("missing_binding", fmt.Sprintf("steps[%d].bindings.%s", idx, out.Name), fmt.Sprintf("missing binding for output %q", out.Name))
					}
				}
			}
			// Also check that included workflow's inputs/outputs satisfied_by/produced_by references are valid (already in ValidateFile)

			newPrefix := prefix + step.ID + "."
			subSteps, err := flattenRec(cleaned, newPrefix, stack, depth+1, expansions, totalSteps, root, base, readFile)
			if err != nil {
				return nil, err
			}
			// Binding rewrite for subSteps
			for i := range subSteps {
				// Requires
				newReq := make([]string, 0, len(subSteps[i].Requires))
				for _, req := range subSteps[i].Requires {
					if bound, ok := step.Bindings[req]; ok {
						newReq = append(newReq, bound)
					} else {
						newReq = append(newReq, req)
					}
				}
				subSteps[i].Requires = newReq
				// Produces
				newProd := make([]string, 0, len(subSteps[i].Produces))
				for _, prod := range subSteps[i].Produces {
					if bound, ok := step.Bindings[prod]; ok {
						newProd = append(newProd, bound)
					} else {
						newProd = append(newProd, prod)
					}
				}
				subSteps[i].Produces = newProd
			}
			// depends_on via inputs: input consumers inherit parent node's depends_on
			// Build expanded parent deps (handle workflow deps that are workflow nodes)
			parentDepsExpanded := []string{}
			for _, dep := range step.DependsOn {
				depID := prefix + dep
				if info, ok := wfMap[depID]; ok {
					// dep is workflow node at this level, expand to its producers
					parentDepsExpanded = append(parentDepsExpanded, info.producerIDs...)
				} else {
					parentDepsExpanded = append(parentDepsExpanded, depID)
				}
			}
			if len(parentDepsExpanded) > 0 {
				// Find consumers
				for _, inp := range incWf.Inputs {
					consumerFQ := newPrefix + inp.SatisfiedBy
					for i := range subSteps {
						if subSteps[i].ID == consumerFQ {
							// merge deps
							merged := append([]string{}, subSteps[i].DependsOn...)
							for _, pd := range parentDepsExpanded {
								if !contains(merged, pd) {
									merged = append(merged, pd)
								}
							}
							subSteps[i].DependsOn = merged
						}
					}
				}
			}
			// Compute producerIDs for this workflow node
			producerIDs := []string{}
			if len(incWf.Outputs) > 0 {
				for _, out := range incWf.Outputs {
					producerIDs = append(producerIDs, newPrefix+out.ProducedBy)
				}
			} else {
				// no outputs: all subSteps are producers (or at least leaf nodes)
				for _, s := range subSteps {
					producerIDs = append(producerIDs, s.ID)
				}
			}
			allIDs := make([]string, len(subSteps))
			for i, s := range subSteps {
				allIDs[i] = s.ID
			}
			workflowNodeID := prefix + step.ID
			wfMap[workflowNodeID] = wfInfo{nodeID: workflowNodeID, producerIDs: producerIDs, allIDs: allIDs}
			// Validate flattened steps count guard incrementally
			*totalSteps += len(subSteps)
			if *totalSteps > 256 {
				return nil, newValidationError("guard_exceeded", "steps", "flattened steps exceeds 256")
			}
			result = append(result, subSteps...)
		} else {
			// command/agent
			flat := FlatStep{
				ID:             prefix + step.ID,
				Type:           step.Type,
				Run:            step.Run,
				Env:            step.Env,
				TimeoutSeconds: step.TimeoutSeconds,
				Requires:       append([]string{}, step.Requires...),
				Produces:       append([]string{}, step.Produces...),
				Harness:        append([]string{}, step.Harness...),
				Instructions:   step.Instructions,
				Mode:           step.Mode,
				Workspace:      step.Workspace,
			}
			// DependsOn with prefix and workflow expansion
			newDeps := []string{}
			for _, dep := range step.DependsOn {
				depID := prefix + dep
				if info, ok := wfMap[depID]; ok {
					newDeps = append(newDeps, info.producerIDs...)
				} else {
					newDeps = append(newDeps, depID)
				}
			}
			flat.DependsOn = newDeps
			*totalSteps++
			if *totalSteps > 256 {
				return nil, newValidationError("guard_exceeded", "steps", "flattened steps exceeds 256")
			}
			result = append(result, flat)
		}
	}
	return result, nil
}

func findRoot(path string) string {
	dir := filepath.Dir(path)
	for {
		if filepath.Base(dir) == ".haro" {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// fallback
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(path))))
}

func canonicalPath(p string) string {
	clean := filepath.Clean(p)
	if ev, err := filepath.EvalSymlinks(clean); err == nil {
		return filepath.Clean(ev)
	}
	// Fallback: try to resolve longest existing prefix
	// mimic path.go logic
	return clean
}

func isEscape(resolved, base string) bool {
	resolved = filepath.Clean(resolved)
	base = filepath.Clean(base)
	if resolved == base {
		return false
	}
	if strings.HasPrefix(resolved, base+string(os.PathSeparator)) {
		return false
	}
	return true
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func topoSort(steps []FlatStep) ([]FlatStep, error) {
	// Build map and indegree
	idToIdx := map[string]int{}
	for i, s := range steps {
		idToIdx[s.ID] = i
	}
	indeg := map[string]int{}
	adj := map[string][]string{}
	for _, s := range steps {
		indeg[s.ID] = len(s.DependsOn)
		for _, dep := range s.DependsOn {
			adj[dep] = append(adj[dep], s.ID)
		}
	}
	// Kahn with stable sort: at each level, pick nodes in sorted order by ID/stable
	var queue []string
	for _, s := range steps {
		if indeg[s.ID] == 0 {
			queue = append(queue, s.ID)
		}
	}
	sort.Strings(queue)
	var out []FlatStep
	visited := 0
	for len(queue) > 0 {
		// pop first
		cur := queue[0]
		queue = queue[1:]
		idx := idToIdx[cur]
		out = append(out, steps[idx])
		visited++
		for _, nxt := range adj[cur] {
			indeg[nxt]--
			if indeg[nxt] == 0 {
				queue = append(queue, nxt)
			}
		}
		sort.Strings(queue)
	}
	if visited != len(steps) {
		// cycle in flat deps (should have been caught earlier)
		return nil, newValidationError("cycle_detected", "", "cycle in flat depends_on")
	}
	return out, nil
}
