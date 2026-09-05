package project

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents .haro/config.yaml.
type Config struct {
	Version       int      `yaml:"version"`
	ExternalPaths []string `yaml:"external_paths"`
}

// LoadConfig loads .haro/config.yaml from root, returning defaults if missing.
// Empty external_paths defaults to empty slice.
func LoadConfig(root string) (*Config, error) {
	path := filepath.Join(root, ".haro", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Version: 2, ExternalPaths: nil}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Version == 0 {
		cfg.Version = 2
	}
	if cfg.ExternalPaths == nil {
		cfg.ExternalPaths = nil
	}
	return &cfg, nil
}
