package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func ComputeHash(dag *FlatDAG) (string, error) {
	if dag == nil {
		return "", nil
	}
	// Canonical representation: sorted steps by ID, sorted maps, etc.
	// Build deterministic JSON
	type hashStep struct {
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
	}
	steps := make([]hashStep, len(dag.Steps))
	for i, s := range dag.Steps {
		// Sort slices
		deps := append([]string{}, s.DependsOn...)
		sort.Strings(deps)
		req := append([]string{}, s.Requires...)
		sort.Strings(req)
		prod := append([]string{}, s.Produces...)
		sort.Strings(prod)
		har := append([]string{}, s.Harness...)
		sort.Strings(har)
		// Sorted env map: json marshaling sorts keys anyway, but ensure deterministic
		steps[i] = hashStep{
			ID: s.ID, Type: s.Type, Run: s.Run, Env: s.Env, TimeoutSeconds: s.TimeoutSeconds,
			DependsOn: deps, Requires: req, Produces: prod, Harness: har, Instructions: s.Instructions, Mode: s.Mode, Workspace: s.Workspace,
		}
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].ID < steps[j].ID })
	data, err := json.Marshal(steps)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}
