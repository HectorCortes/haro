package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Discovered is a workflow found on disk.
type Discovered struct {
	Name     string
	Path     string
	Workflow *Workflow
}

// Discover scans root/.haro/workflows/*/workflow.yaml deterministically.
// It returns discovered workflows sorted by directory name. Missing or empty
// workflows directory yields an empty slice without error. Invalid YAML returns
// an error.
func Discover(root string) ([]Discovered, error) {
	base := filepath.Join(root, ".haro", "workflows")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workflows dir: %w", err)
	}
	var out []Discovered
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		wfPath := filepath.Join(base, name, "workflow.yaml")
		info, err := os.Stat(wfPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", wfPath, err)
		}
		if info.IsDir() {
			continue
		}
		f, err := os.Open(wfPath)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", wfPath, err)
		}
		wf, err := Parse(f)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", wfPath, err)
		}
		out = append(out, Discovered{
			Name:     name,
			Path:     wfPath,
			Workflow: wf,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
