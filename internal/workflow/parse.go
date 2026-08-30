package workflow

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Workflow is the minimal subset parsed for the spike.
type Workflow struct {
	Version int    `yaml:"version"`
	Name    string `yaml:"name"`
	Steps   []Step `yaml:"steps"`
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
}

// Parse decodes a Workflow from r and validates required fields.
// It errors unless version is 2 and at least one step exists.
func Parse(r io.Reader) (*Workflow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read workflow: %w", err)
	}
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	if wf.Version != 2 {
		return nil, fmt.Errorf("unsupported version %d: want 2", wf.Version)
	}
	if len(wf.Steps) == 0 {
		return nil, fmt.Errorf("workflow must have at least one step")
	}
	return &wf, nil
}
