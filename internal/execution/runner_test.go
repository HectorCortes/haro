package execution

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestParseArgv(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "simple",
			input: "echo hello world",
			want:  []string{"echo", "hello", "world"},
		},
		{
			name:  "quoted double",
			input: `echo "hello world"`,
			want:  []string{"echo", "hello world"},
		},
		{
			name:  "quoted single",
			input: "echo 'hello world'",
			want:  []string{"echo", "hello world"},
		},
		{
			name:  "no shell expansion semicolon",
			input: "echo foo; rm -rf /",
			want:  []string{"echo", "foo;", "rm", "-rf", "/"},
		},
		{
			name:  "no shell expansion pipe",
			input: "echo hi | cat",
			want:  []string{"echo", "hi", "|", "cat"},
		},
		{
			name:  "no shell expansion dollar",
			input: "echo $HOME $(whoami)",
			want:  []string{"echo", "$HOME", "$(whoami)"},
		},
		{
			name:  "escaped space",
			input: `echo hello\ world`,
			want:  []string{"echo", "hello world"},
		},
		{
			name:  "mixed quotes",
			input: `cmd "a b" 'c d' e`,
			want:  []string{"cmd", "a b", "c d", "e"},
		},
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:    "unclosed quote",
			input:   `echo "unclosed`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseArgv(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseArgv(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseArgv(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ParseArgv(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
	// triangulation: ensure no shell expansion — metachars remain literal
	got, _ := ParseArgv("run ;|$()")
	if len(got) != 2 || got[1] != ";|$()" {
		t.Fatalf("metachars should remain literal, got %v", got)
	}
}

func TestCommandRunner_Fake(t *testing.T) {
	fake := &FakeRunner{
		Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			if cwd != "/tmp" {
				t.Fatalf("cwd = %q, want /tmp", cwd)
			}
			if len(argv) != 2 || argv[0] != "echo" || argv[1] != "hi" {
				t.Fatalf("argv = %v, want [echo hi]", argv)
			}
			if env["FOO"] != "bar" {
				t.Fatalf("env FOO = %q, want bar", env["FOO"])
			}
			return 0, "hi\n", "", nil
		},
	}
	ctx := context.Background()
	code, out, errOut, err := fake.Run(ctx, "/tmp", []string{"echo", "hi"}, map[string]string{"FOO": "bar"})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if code != 0 || out != "hi\n" || errOut != "" {
		t.Fatalf("Run result = %d %q %q, want 0 hi", code, out, errOut)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("calls len = %d, want 1", len(fake.Calls))
	}
}

func TestCommandRunner_Real_NoShell(t *testing.T) {
	// Real runner should NOT use shell; ";" should be passed as argument, not executed.
	r := NewRunner()
	ctx := context.Background()
	// Use echo with semicolon arg — should output literal ";"
	argv, _ := ParseArgv(`echo "a; b"`)
	code, out, _, err := r.Run(ctx, t.TempDir(), argv, nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "a; b") {
		t.Fatalf("output = %q, want contains 'a; b' (no shell)", out)
	}
}

func TestCommandRunner_Timeout(t *testing.T) {
	r := NewRunner()
	// Runner has 300s ceiling; we test that context cancellation is respected and that runner enforces timeout.
	// Use a command that sleeps; cancel quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	argv := []string{"sleep", "10"}
	start := time.Now()
	_, _, _, err := r.Run(ctx, t.TempDir(), argv, nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected timeout/cancel error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

func TestCommandRunner_ExitCode(t *testing.T) {
	r := NewRunner()
	ctx := context.Background()
	argv := []string{"sh", "-c", "exit 42"}
	code, _, _, err := r.Run(ctx, t.TempDir(), argv, nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if code != 42 {
		t.Fatalf("exit code = %d, want 42", code)
	}
}
