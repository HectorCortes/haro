package project

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultHarnessTimeoutSeconds is applied when a harness record omits
// timeout_seconds. The valid range is 1..300 seconds.
const DefaultHarnessTimeoutSeconds = 300

// HarnessConfig is one configured harness record from .haro/config.yaml.
// Record keys are arbitrary data names; provider literals must stay confined
// to adapters. Unknown record fields are rejected by strict decoding.
type HarnessConfig struct {
	Binary         string            `yaml:"binary"`
	Env            map[string]string `yaml:"env"`
	TimeoutSeconds int               `yaml:"timeout_seconds"`
	// Enabled is a pointer so an omitted field (default true) can be
	// distinguished from an explicit `enabled: false`.
	Enabled *bool `yaml:"enabled"`
}

// Config represents .haro/config.yaml.
type Config struct {
	Version       int                      `yaml:"version"`
	ExternalPaths []string                 `yaml:"external_paths"`
	Harnesses     map[string]HarnessConfig `yaml:"harnesses,omitempty"`
}

// IsEnabled reports whether the harness record is enabled. A nil Enabled
// field defaults to true.
func (h HarnessConfig) IsEnabled() bool {
	return h.Enabled == nil || *h.Enabled
}

// LoadConfig loads .haro/config.yaml from root, returning defaults if missing.
// Empty external_paths defaults to empty slice. Decoding is strict:
// yaml.Decoder.KnownFields(true) rejects both unknown record fields inside
// harnesses and unknown top-level keys, while keeping version/external_paths
// compatible and harness names as arbitrary data keys.
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
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	if cfg.Version == 0 {
		cfg.Version = 2
	}
	if cfg.ExternalPaths == nil {
		cfg.ExternalPaths = nil
	}
	if err := cfg.NormalizeHarnesses(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// NormalizeHarnesses applies harness defaults and validates ranges:
// nil Enabled defaults to true, zero timeout_seconds defaults to 300, and
// values outside the 1..300 range fail.
func (c *Config) NormalizeHarnesses() error {
	for name, hc := range c.Harnesses {
		if hc.Enabled == nil {
			enabled := true
			hc.Enabled = &enabled
		}
		if hc.TimeoutSeconds == 0 {
			hc.TimeoutSeconds = DefaultHarnessTimeoutSeconds
		}
		if hc.TimeoutSeconds < 1 || hc.TimeoutSeconds > DefaultHarnessTimeoutSeconds {
			return fmt.Errorf("harness %q: timeout_seconds %d outside 1..%d", name, hc.TimeoutSeconds, DefaultHarnessTimeoutSeconds)
		}
		c.Harnesses[name] = hc
	}
	return nil
}
