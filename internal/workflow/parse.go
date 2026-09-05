package workflow

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkspaceConfig holds workspace isolation settings.
type WorkspaceConfig struct {
	Mode              *string `yaml:"mode"`
	OnLogicalConflict *string `yaml:"on_logical_conflict"`
}

// Input declares a required input for an included workflow.
type Input struct {
	Name        string `yaml:"name"`
	SatisfiedBy string `yaml:"satisfied_by"`
}

// Output declares a produced output for an included workflow.
type Output struct {
	Name       string `yaml:"name"`
	ProducedBy string `yaml:"produced_by"`
}

// Workflow is the minimal subset parsed for the spike.
type Workflow struct {
	Version   int              `yaml:"version"`
	Name      string           `yaml:"name"`
	Workspace *WorkspaceConfig `yaml:"workspace"`
	Inputs    []Input          `yaml:"inputs"`
	Outputs   []Output         `yaml:"outputs"`
	Steps     []Step           `yaml:"steps"`
}

// Step is a single workflow step.
type Step struct {
	ID             string            `yaml:"id"`
	Type           string            `yaml:"type"`
	Run            string            `yaml:"run"`
	Env            map[string]string `yaml:"env"`
	TimeoutSeconds *int              `yaml:"timeout_seconds"`
	DependsOn      []string          `yaml:"depends_on"`
	Requires       []string          `yaml:"requires"`
	Produces       []string          `yaml:"produces"`
	Harness        []string          `yaml:"harness"`
	Instructions   string            `yaml:"instructions"`
	Mode           string            `yaml:"mode"`
	Workspace      *WorkspaceConfig  `yaml:"workspace"`
	Source         *string           `yaml:"source"`
	Bindings       map[string]string `yaml:"bindings"`
}

// ResolveWorkspace resolves system→workflow→step inheritance for the given stepID.
// Defaults are isolated/block.
func ResolveWorkspace(wf *Workflow, stepID string) (mode string, onConflict string) {
	mode = "isolated"
	onConflict = "block"
	if wf.Workspace != nil {
		if wf.Workspace.Mode != nil {
			mode = *wf.Workspace.Mode
		}
		if wf.Workspace.OnLogicalConflict != nil {
			onConflict = *wf.Workspace.OnLogicalConflict
		}
	}
	for _, s := range wf.Steps {
		if s.ID == stepID && s.Workspace != nil {
			if s.Workspace.Mode != nil {
				mode = *s.Workspace.Mode
			}
			if s.Workspace.OnLogicalConflict != nil {
				onConflict = *s.Workspace.OnLogicalConflict
			}
			break
		}
	}
	return mode, onConflict
}

// Parse decodes a Workflow from r and validates required fields.
// It errors unless version is 2 and at least one step exists.
func Parse(r io.Reader) (*Workflow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read workflow: %w", err)
	}
	// Strict decoding: fail on unknown fields.
	var wf Workflow
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&wf); err != nil {
		// Convert strict field errors into ValidationError with field path.
		msg := err.Error()
		if strings.Contains(msg, "not found in type") {
			// Extract field name: "field <name> not found"
			re := regexp.MustCompile(`field ([^\s]+) not found`)
			m := re.FindStringSubmatch(msg)
			field := "unknown"
			if len(m) == 2 {
				field = m[1]
			}
			return nil, newValidationError("unknown_field", field, msg)
		}
		if strings.Contains(msg, "line") {
			return nil, newValidationError("invalid_yaml", "", msg)
		}
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	if wf.Version != 2 {
		return nil, newValidationError("invalid_version", "version", fmt.Sprintf("unsupported version %d: want 2", wf.Version))
	}
	if len(wf.Steps) == 0 {
		return nil, newValidationError("missing_field", "steps", "workflow must have at least one step")
	}
	return &wf, nil
}
