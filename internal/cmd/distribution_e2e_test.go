package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildHaroBinary builds the real root binary (main.go) with CGO_ENABLED=0
// into a temporary directory and returns its path. The build exercises the
// same command the release workflow uses, minus the strip flags.
func buildHaroBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping binary build E2E in -short mode")
	}
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "haro")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "../.."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// runHaro runs the built binary with the given args in the given working
// directory and returns its exit code, stdout and stderr combined.
func runHaro(t *testing.T, bin string, dir string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	// GOPROXY=off is the network-denial control required by D08 F-03: init
	// must succeed without any module proxy or network access.
	cmd.Env = append(os.Environ(), "GOPROXY=off")
	out, err := cmd.CombinedOutput()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v\n%s", args, err, out)
	}
	return code, string(out)
}

// TestDistributionOfflineInitE2E proves D08 F-03: an already-built haro
// binary initializes an empty project offline (GOPROXY=off) with no prior
// dependencies, creating the complete .haro/ structure.
func TestDistributionOfflineInitE2E(t *testing.T) {
	bin := buildHaroBinary(t)
	project := t.TempDir()

	code, out := runHaro(t, bin, project, "init")
	if code != 0 {
		t.Fatalf("init exit = %d, want 0\n%s", code, out)
	}

	haroRoot := filepath.Join(project, ".haro")
	for _, p := range []string{
		filepath.Join(haroRoot, "config.yaml"),
		filepath.Join(haroRoot, "workflows"),
		filepath.Join(haroRoot, "skills"),
		filepath.Join(haroRoot, "artifacts"),
		filepath.Join(haroRoot, "docs"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("init did not create %s: %v", p, err)
		}
	}
}

// TestDistributionOfflineInitIdempotentE2E proves D08 F-03 idempotence: a
// second offline init preserves custom config bytes and user content.
func TestDistributionOfflineInitIdempotentE2E(t *testing.T) {
	bin := buildHaroBinary(t)
	project := t.TempDir()

	if code, out := runHaro(t, bin, project, "init"); code != 0 {
		t.Fatalf("first init exit = %d\n%s", code, out)
	}

	cfgPath := filepath.Join(project, ".haro", "config.yaml")
	custom := "# custom project config\nversion: 2\nexternal_paths:\n  docs: custom-docs\n"
	if err := os.WriteFile(cfgPath, []byte(custom), 0o600); err != nil {
		t.Fatalf("write custom config: %v", err)
	}
	userDoc := filepath.Join(project, ".haro", "docs", "user.md")
	if err := os.WriteFile(userDoc, []byte("# user content\n"), 0o600); err != nil {
		t.Fatalf("write user doc: %v", err)
	}

	if code, out := runHaro(t, bin, project, "init"); code != 0 {
		t.Fatalf("second init exit = %d\n%s", code, out)
	}

	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config after rerun: %v", err)
	}
	if string(got) != custom {
		t.Fatalf("config bytes replaced:\n got %q\nwant %q", got, custom)
	}
	if _, err := os.Stat(userDoc); err != nil {
		t.Fatalf("user content removed by rerun: %v", err)
	}
}
