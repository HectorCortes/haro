package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseExtended(t *testing.T) {
	yaml := "version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n    env:\n      FOO: bar\n      BAZ: qux\n    timeout_seconds: 42\n    depends_on: [s0]\n    requires: [input.txt]\n    produces: [output.txt]\n  - id: s2\n    type: command\n    run: echo bye\n    depends_on: [s1]\n"
	wf, err := Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Parse extended: %v", err)
	}
	if len(wf.Steps) != 2 {
		t.Fatalf("steps len = %d, want 2", len(wf.Steps))
	}
	s1 := wf.Steps[0]
	if s1.Env["FOO"] != "bar" || s1.Env["BAZ"] != "qux" {
		t.Fatalf("env = %v, want FOO=bar BAZ=qux", s1.Env)
	}
	if s1.TimeoutSeconds == nil || *s1.TimeoutSeconds != 42 {
		t.Fatalf("timeout_seconds = %v, want 42", s1.TimeoutSeconds)
	}
	if len(s1.DependsOn) != 1 || s1.DependsOn[0] != "s0" {
		t.Fatalf("depends_on = %v, want [s0]", s1.DependsOn)
	}
	if len(s1.Requires) != 1 || s1.Requires[0] != "input.txt" {
		t.Fatalf("requires = %v, want [input.txt]", s1.Requires)
	}
	if len(s1.Produces) != 1 || s1.Produces[0] != "output.txt" {
		t.Fatalf("produces = %v, want [output.txt]", s1.Produces)
	}
	if len(wf.Steps[1].DependsOn) != 1 || wf.Steps[1].DependsOn[0] != "s1" {
		t.Fatalf("s2 depends_on = %v, want [s1]", wf.Steps[1].DependsOn)
	}
	// triangulation: empty optional fields
	yaml2 := "version: 2\nname: demo\nsteps:\n  - id: only\n    type: command\n    run: echo hi\n"
	wf2, err := Parse(strings.NewReader(yaml2))
	if err != nil {
		t.Fatalf("Parse minimal: %v", err)
	}
	if wf2.Steps[0].TimeoutSeconds != nil {
		t.Fatalf("expected nil timeout for minimal step, got %v", *wf2.Steps[0].TimeoutSeconds)
	}
	if len(wf2.Steps[0].Env) != 0 {
		t.Fatalf("expected empty env, got %v", wf2.Steps[0].Env)
	}
}

func TestParse(t *testing.T) {
	// valid fixture copied from testdata via t.TempDir
	validYAML := "version: 2\nname: sample\nsteps:\n  - id: build\n    type: command\n    run: echo hello\n"

	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		check   func(t *testing.T, wf *Workflow)
	}{
		{
			name:    "valid workflow",
			yaml:    validYAML,
			wantErr: false,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Version != 2 {
					t.Fatalf("version = %d, want 2", wf.Version)
				}
				if len(wf.Steps) == 0 || wf.Steps[0].ID != "build" {
					t.Fatalf("steps[0].id = %q, want %q", wf.Steps[0].ID, "build")
				}
			},
		},
		{
			name:    "wrong version",
			yaml:    "version: 1\nname: sample\nsteps:\n  - id: build\n    type: command\n    run: echo hello\n",
			wantErr: true,
		},
		{
			name:    "empty steps",
			yaml:    "version: 2\nname: sample\nsteps: []\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "workflow.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatalf("write temp file: %v", err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open temp file: %v", err)
			}
			defer func() { _ = f.Close() }()

			wf, err := Parse(f)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, wf)
			}
			// also test direct string reader for valid case
			if tt.name == "valid workflow" && !tt.wantErr {
				wf2, err := Parse(strings.NewReader(tt.yaml))
				if err != nil {
					t.Fatalf("Parse(strings.Reader) error = %v", err)
				}
				if wf2.Steps[0].ID != "build" {
					t.Fatalf("strings reader: steps[0].id = %q", wf2.Steps[0].ID)
				}
			}
		})
	}
}
