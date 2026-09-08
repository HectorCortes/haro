package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// D09 (v2-flujo-sdd) flow property tests. These tests verify the repository's
// documented development flow: each spec is an SDD change (F-01), verification
// uses the real Go boundary (F-02), delivery goes through review and delivery
// gates with no initiatives pipeline (F-03), and criterion-to-test
// traceability is executable and fail-closed (U-01).

func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return abs
}

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// section returns the text from the line starting with startMarker up to (not
// including) the next standalone "---" separator in the same document.
func section(t *testing.T, doc, startMarker string) string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, startMarker) {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("section start marker %q not found", startMarker)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// TestFlowEachSpecIsSDDChange proves v2-flujo-sdd/F-01: the live contract
// names the spec v2-flujo-sdd with the historical alias, the seven-phase SDD
// cycle is documented, and every completed spec keeps its full archived
// artifact set.
func TestFlowEachSpecIsSDDChange(t *testing.T) {
	root := repoRoot(t)
	deltas := readRepoFile(t, root, "deltas-acceptance.md")

	const liveSpec = "# Spec: v2-flujo-sdd — Development flow (previously v2-flujo-gentle-ai (D09))"
	if !strings.Contains(deltas, liveSpec) {
		t.Fatalf("live D09 spec heading with historical alias not found; want %q", liveSpec)
	}
	// Only the historical alias may mention the retired tool name.
	if n := strings.Count(deltas, "gentle-ai"); n != 1 {
		t.Fatalf("gentle-ai mentions in live contract = %d, want exactly 1 (the historical alias)", n)
	}

	const cycle = "proposal → spec → design → tasks → apply → verify → archive"
	usage := section(t, deltas, "## Usage — SDD flow")
	if !strings.Contains(usage, cycle) {
		t.Fatalf("§Usage does not document the seven-phase SDD cycle %q", cycle)
	}
	d09 := section(t, deltas, "# Spec: v2-flujo-sdd")
	if !strings.Contains(d09, cycle) {
		t.Fatalf("D09 F-01 criterion does not document the seven-phase SDD cycle %q", cycle)
	}
	const trackingRow = "| `v2-flujo-sdd` | Development flow | 4 | 3 | **complete** |"
	if !strings.Contains(deltas, trackingRow) {
		t.Fatalf("tracking table row %q not found", trackingRow)
	}

	// Required archived artifact sets: each completed spec must keep the full
	// SDD cycle artifacts in openspec/changes/archive/ (flat or specs/ layout).
	type archivedSpec struct {
		name     string
		specPath string
	}
	for _, s := range []archivedSpec{
		{name: "v2-reconciliacion", specPath: "specs/v2-reconciliacion/spec.md"},
		{name: "v2-composicion", specPath: "spec.md"},
		{name: "v2-reporte", specPath: "spec.md"},
		{name: "v2-store", specPath: "spec.md"},
		{name: "v2-distribucion", specPath: "spec.md"},
	} {
		t.Run(s.name, func(t *testing.T) {
			glob := filepath.Join(root, "openspec", "changes", "archive", "*-"+s.name)
			dirs, err := filepath.Glob(glob)
			if err != nil {
				t.Fatalf("glob archive dir: %v", err)
			}
			if len(dirs) != 1 {
				t.Fatalf("archive dirs matching %s = %d, want 1", s.name, len(dirs))
			}
			for _, art := range []string{
				"proposal.md",
				s.specPath,
				"design.md",
				"tasks.md",
				"verify-report.md",
				"archive-report.md",
			} {
				path := filepath.Join(dirs[0], art)
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("archived artifact missing for %s: %s: %v", s.name, art, err)
				}
			}
		})
	}
}

// TestFlowVerificationViaVerify proves v2-flujo-sdd/F-02: the flow guidance
// and verification evidence identify the real Go race suite and the
// public-boundary/module distinction, name all CI gates, and contain no
// stale Node/npm runner claim on any flow surface.
func TestFlowVerificationViaVerify(t *testing.T) {
	root := repoRoot(t)
	deltas := readRepoFile(t, root, "deltas-acceptance.md")

	usage := section(t, deltas, "## Usage — SDD flow")
	for _, want := range []string{
		"`go test ./...`",
		"`go test ./... -race`",
		"in CI",
		"public boundary",
		"package/module tests",
		"`go build ./...`",
		"`go vet ./...`",
		"golangci-lint",
		"govulncheck",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("§Usage is missing %q", want)
		}
	}

	// Header stack decision and Conventions stack line must be Go-correct.
	header := section(t, deltas, "# Haro — v2 delta acceptance criteria")
	conventions := section(t, deltas, "## Conventions")
	if !strings.Contains(header, "**Stack decision (locked): Pure Go, no cgo.**") {
		t.Fatalf("header stack decision is not the locked Pure Go/no-cgo decision")
	}
	if !strings.Contains(conventions, "**Decided stack — Pure Go, no cgo**") {
		t.Fatalf("Conventions stack line is not the Pure Go/no-cgo stack")
	}
	staleSurfaces := map[string]string{
		"header":      header,
		"conventions": conventions,
		"usage":       usage,
		"D09 spec":    d09Section(t, deltas),
	}
	for surface, text := range staleSurfaces {
		for _, stale := range []string{"node --test", "npm test", "tsc --noEmit"} {
			if strings.Contains(text, stale) {
				t.Fatalf("%s surface still claims stale runner %q", surface, stale)
			}
		}
	}
	if strings.Contains(header, "TypeScript") {
		t.Fatalf("header stack decision still claims TypeScript")
	}

	// D09 F-02 criterion must name the SDD flow, not a retired tool.
	d09 := d09Section(t, deltas)
	if !strings.Contains(d09, "equivalent verification of the SDD flow") {
		t.Fatalf("D09 F-02 criterion does not name the SDD flow verification")
	}
	if strings.Contains(d09, "gentle-ai flow") {
		t.Fatalf("D09 F-02 criterion still names the retired tool flow")
	}

	agents := readRepoFile(t, root, "AGENTS.md")
	for _, want := range []string{
		"`go build ./...`",
		"`go vet ./...`",
		"`go test ./... -race`",
		"golangci-lint",
		"govulncheck",
	} {
		if !strings.Contains(agents, want) {
			t.Fatalf("AGENTS.md gate line is missing %q", want)
		}
	}

	ci := readRepoFile(t, root, ".github/workflows/ci.yml")
	for _, want := range []string{
		"go build ./...",
		"go vet ./...",
		"go test ./... -race",
		"golangci-lint",
		"govulncheck",
	} {
		if !strings.Contains(ci, want) {
			t.Fatalf("CI workflow is missing gate %q", want)
		}
	}
	for _, stale := range []string{"node --test", "npm test"} {
		if strings.Contains(ci, stale) {
			t.Fatalf("CI workflow still claims stale runner %q", stale)
		}
	}
}

// d09Section isolates the D09 spec section without failing inside another
// test's t.
func d09Section(t *testing.T, deltas string) string {
	t.Helper()
	return section(t, deltas, "# Spec: v2-flujo-sdd")
}

// TestFlowDeliveryThroughGates proves v2-flujo-sdd/F-03 (validator MINOR):
// the initiatives pipeline stays absent, the contract/AGENTS/CI describe the
// review and delivery gates, and the hybrid delivery clause holds — feature
// code via reviewed PRs, documentation-only archive pushes after verification.
func TestFlowDeliveryThroughGates(t *testing.T) {
	root := repoRoot(t)

	// No initiatives pipeline path may exist.
	for _, p := range []string{".docs/initiatives", "tools/scripts/initiative"} {
		if _, err := os.Stat(filepath.Join(root, p)); !os.IsNotExist(err) {
			t.Fatalf("initiatives path %q must remain absent", p)
		}
	}

	deltas := readRepoFile(t, root, "deltas-acceptance.md")
	d09 := d09Section(t, deltas)
	for _, want := range []string{
		"SDD flow gates (review receipts, delivery gates)",
		"not through the initiatives pipeline",
		"no new initiatives are created",
		"absence of new `.docs/initiatives/`",
	} {
		if !strings.Contains(d09, want) {
			t.Fatalf("D09 F-03 criterion is missing %q", want)
		}
	}

	// Delivery guidance: docs-only archive records are pushed after
	// verification; the archived verify-reports are the receipts.
	usage := section(t, deltas, "## Usage — SDD flow")
	if !strings.Contains(usage, "**no longer** uses the initiatives pipeline") {
		t.Fatalf("§Usage does not retire the initiatives pipeline")
	}
	archiveReport := filepath.Join(root, "openspec", "changes", "archive", "2026-09-07-v2-distribucion", "verify-report.md")
	if _, err := os.Stat(archiveReport); err != nil {
		t.Fatalf("archived verify-report (docs-only delivery receipt) missing: %v", err)
	}

	// Feature code rides reviewed PRs: AGENTS.md requires PR work units and
	// the CI workflow enforces gates on both pull requests and main pushes.
	agents := readRepoFile(t, root, "AGENTS.md")
	for _, want := range []string{
		"small PRs as work units",
		"Conventional commits",
		"green CI",
	} {
		if !strings.Contains(agents, want) {
			t.Fatalf("AGENTS.md delivery guidance is missing %q", want)
		}
	}
	ci := readRepoFile(t, root, ".github/workflows/ci.yml")
	if !strings.Contains(ci, "pull_request:") {
		t.Fatalf("CI workflow has no pull_request review gate")
	}
	if !strings.Contains(ci, "branches: [main]") {
		t.Fatalf("CI workflow has no push-to-main delivery gate")
	}
	for _, job := range []string{"build-and-test:", "lint:", "vulncheck:"} {
		if !strings.Contains(ci, job) {
			t.Fatalf("CI workflow is missing required gate job %q", strings.TrimSuffix(job, ":"))
		}
	}
}

// --- U-01 harness -----------------------------------------------------------

// runTraceabilityScript runs scripts/verify-traceability.sh with the given
// arguments and command working directory, returning its exit code and
// combined output.
func runTraceabilityScript(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	script := filepath.Join(repoRoot(t), "scripts", "verify-traceability.sh")
	// Note: this toolchain rejects mixed positional+spread calls, so the
	// argument list is built explicitly.
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		return 127, string(out)
	}
	return code, string(out)
}

// traceabilityFixture builds a minimal repository-like root containing the
// live contract, the flow test file, and a symlinked openspec tree so the
// archived evidence resolves. withGit initializes a git work tree and tracks
// the discovered files.
func traceabilityFixture(t *testing.T, withGit bool) string {
	t.Helper()
	root := repoRoot(t)
	fix := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(fix, "deltas-acceptance.md"),
		[]byte(readRepoFile(t, root, "deltas-acceptance.md")), 0o644); err != nil {
		t.Fatalf("write fixture deltas: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(fix, "internal", "cmd"), 0o755); err != nil {
		t.Fatalf("mkdir fixture internal/cmd: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(fix, "internal", "cmd", "flow_test.go"),
		[]byte(readRepoFile(t, root, filepath.Join("internal", "cmd", "flow_test.go"))), 0o644); err != nil {
		t.Fatalf("write fixture flow test: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "openspec"), filepath.Join(fix, "openspec")); err != nil {
		t.Fatalf("symlink fixture openspec: %v", err)
	}
	if withGit {
		run := func(args ...string) {
			t.Helper()
			cmd := exec.Command("git", append([]string{"-C", fix}, args...)...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		run("init", "-q")
		run("add", "deltas-acceptance.md", "internal/cmd/flow_test.go")
	}
	return fix
}

// assertAuditOK asserts a successful auditor run enumerating the full catalog.
func assertAuditOK(t *testing.T, out string) {
	t.Helper()
	for _, want := range []string{"TOTAL=92", "STRICT=4", "INFO=88"} {
		if !strings.Contains(out, want) {
			t.Fatalf("auditor output missing %q\n%s", want, out)
		}
	}
	for _, id := range []string{
		"v2-flujo-sdd/F-01",
		"v2-flujo-sdd/U-01",
		"v2-reconciliacion/F-01",
		"v2-composicion/U-04",
		"v2-reporte/F-01",
		"v2-store/U-03",
		"v2-distribucion/F-03",
		"v2-broker/F-01",
		"v2-ipc/U-04",
		"v2-no-regresion/F-14",
		"v2-adapter/U-01",
		"v2-path-claims/U-03",
	} {
		if !strings.Contains(out, id) {
			t.Fatalf("auditor output does not enumerate %q\n%s", id, out)
		}
	}
}

// TestFlowCriterionTraceability proves v2-flujo-sdd/U-01: the audit script
// enumerates all 92 criteria with 4 strict D09 mappings, resolves archived
// evidence in both flat and specs/ layouts (v2-reconciliacion is the
// non-allowlisted directory-layout fixture), and fails closed naming the
// affected criterion and test.
func TestFlowCriterionTraceability(t *testing.T) {
	t.Run("real root check-only", func(t *testing.T) {
		root := repoRoot(t)
		code, out := runTraceabilityScript(t, root, "--check-only")
		if code != 0 {
			t.Fatalf("real-root --check-only exit = %d, want 0\n%s", code, out)
		}
		assertAuditOK(t, out)
	})

	t.Run("non-git fixture root", func(t *testing.T) {
		fix := traceabilityFixture(t, false)
		if code, out := runTraceabilityScript(t, fix, "--check-only", fix); code != 0 {
			t.Fatalf("non-git fixture exit = %d, want 0\n%s", code, out)
		} else {
			assertAuditOK(t, out)
		}
	})

	t.Run("relative fixture root", func(t *testing.T) {
		fix := traceabilityFixture(t, false)
		parent := filepath.Dir(fix)
		code, out := runTraceabilityScript(t, parent, "--check-only", filepath.Base(fix))
		if code != 0 {
			t.Fatalf("relative fixture exit = %d, want 0\n%s", code, out)
		}
		assertAuditOK(t, out)
	})

	t.Run("git fixture root", func(t *testing.T) {
		fix := traceabilityFixture(t, true)
		code, out := runTraceabilityScript(t, fix, "--check-only", fix)
		if code != 0 {
			t.Fatalf("git fixture exit = %d, want 0\n%s", code, out)
		}
		assertAuditOK(t, out)
	})

	t.Run("duplicate id fails closed", func(t *testing.T) {
		fix := traceabilityFixture(t, false)
		deltas := readRepoFile(t, repoRoot(t), "deltas-acceptance.md")
		// Duplicate a criterion heading inside the v2-distribucion section.
		anchor := "# Spec: v2-distribucion — Distribution\n"
		i := strings.Index(deltas, anchor)
		if i < 0 {
			t.Fatalf("distribucion anchor not found")
		}
		poisoned := deltas[:i+len(anchor)] + "\n### F-01 — duplicate [E2E] · P0 · [x]\n" + deltas[i+len(anchor):]
		if err := os.WriteFile(filepath.Join(fix, "deltas-acceptance.md"), []byte(poisoned), 0o644); err != nil {
			t.Fatalf("write poisoned deltas: %v", err)
		}
		code, out := runTraceabilityScript(t, fix, "--check-only", fix)
		if code == 0 {
			t.Fatalf("duplicate ID exit = 0, want nonzero\n%s", out)
		}
		if !strings.Contains(out, "v2-distribucion/F-01") {
			t.Fatalf("duplicate ID failure does not name the ID\n%s", out)
		}
	})

	t.Run("removed mapping fails closed", func(t *testing.T) {
		fix := traceabilityFixture(t, false)
		// Fake flow test file that only names three of the four required tests.
		fake := "package cmd\n\n" +
			"func TestFlowEachSpecIsSDDChange(t *testing.T) {}\n" +
			"func TestFlowDeliveryThroughGates(t *testing.T) {}\n" +
			"func TestFlowCriterionTraceability(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(fix, "internal", "cmd", "flow_test.go"), []byte(fake), 0o644); err != nil {
			t.Fatalf("write fake flow test: %v", err)
		}
		code, out := runTraceabilityScript(t, fix, "--check-only", fix)
		if code == 0 {
			t.Fatalf("removed mapping exit = 0, want nonzero\n%s", out)
		}
		if !strings.Contains(out, "v2-flujo-sdd/F-02") || !strings.Contains(out, "TestFlowVerificationViaVerify") {
			t.Fatalf("removed mapping failure does not name criterion and test\n%s", out)
		}
	})
}
