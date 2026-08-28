package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
