package project

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultConfig = `# haro project config
version: 2
`

// Init creates .haro/{config.yaml,workflows/,skills/,artifacts/,docs/} non-destructively.
// It is idempotent: rerunning preserves bytes, reports initialization, and returns nil.
func Init(root string) error {
	haroRoot := filepath.Join(root, ".haro")
	if err := os.MkdirAll(haroRoot, 0o755); err != nil {
		return fmt.Errorf("create .haro: %w", err)
	}
	// Subdirectories
	for _, sub := range []string{"workflows", "skills", "artifacts", "docs"} {
		p := filepath.Join(haroRoot, sub)
		if err := os.MkdirAll(p, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", sub, err)
		}
	}
	// config.yaml — create only if not exists
	cfgPath := filepath.Join(haroRoot, "config.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		// exists — preserve
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config.yaml: %w", err)
	}
	if err := os.WriteFile(cfgPath, []byte(defaultConfig), 0o600); err != nil {
		return fmt.Errorf("write config.yaml: %w", err)
	}
	return nil
}
