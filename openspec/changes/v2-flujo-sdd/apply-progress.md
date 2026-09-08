# Apply Progress: v2-flujo-sdd (D09)

**Status**: all 20 tasks complete (1.1 → 5.5). Mode: Strict TDD. Artifact store: hybrid (OpenSpec + Engram).

## Completed Tasks

### Phase 1 — Contract edits (deltas-acceptance.md)
- [x] 1.1 L45 + L492: id → `v2-flujo-sdd`, content → `Development flow`, status stays `pending`. No other row touched.
- [x] 1.2 L456: `# Spec: v2-flujo-sdd — Development flow (previously v2-flujo-gentle-ai (D09))`.
- [x] 1.3 §Usage: heading `## Usage — SDD flow`; L51 neutralized; L54 → `go test ./...` (`go test ./... -race` in CI), E2E vs UNIT/INT layers, gates: build, vet, golangci-lint, govulncheck.
- [x] 1.4 L5 + L25 → design's Pure Go/no-cgo stack lines (verbatim from design).
- [x] 1.5 L7 → `in the SDD flow (see §Usage).`; L19 → `(it matches the SDD change name).`
- [x] 1.6 L463 → `verification of the SDD flow`; L466 → `### F-03 — Delivery through the SDD flow`; L467 → `SDD flow gates (review receipts, delivery gates)`.
- [x] 1.7 Guard verified: `git diff -U0` shows exactly 13 changed lines (5,7,19,25,45,49,51,54,456,463,466,467,492). L26–28 and L147–148 verbatim; all other specs untouched.

### Phase 2 — AGENTS.md
- [x] 2.1 L7 only: `including openspec/SDD artifacts`. 0 `gentle-ai` hits remain.

### Phase 3 — Auditor RED→GREEN (scripts/verify-traceability.sh)
- [x] 3.1 RED: `TestFlowCriterionTraceability` written first; all 6 subtests failed with script absent (exit 127, "No such file or directory"). Real-root `--check-only` asserts exit 0 + `TOTAL=92 STRICT=4 INFO=88`; fixtures: non-git/relative/git roots (exit 0 + full enumeration), duplicate ID → exit 1 naming `v2-distribucion/F-01`, removed mapping → exit 1 naming `v2-flujo-sdd/F-02` + `TestFlowVerificationViaVerify`.
- [x] 3.2 GREEN: script (+x, `set -euo pipefail`), script-derived default root + optional fixture root; `git -C` tracked-file discovery with `find` fallback (non-git fixtures); awk parse of `# Spec: v2-*` + `### (F|U)-[0-9]+`; rejects duplicates, malformed headings, unattached criteria, count≠92, specs≠11; strict: D09's 4 mappings verified by `grep '^func <Test>(t *testing.T) {'` in `internal/cmd/flow_test.go`; informational: 59-ID allowlist (`v2-broker` 7F+2U and `v2-ipc` 9F+4U = `deferred:cli-direct`; `v2-no-regresion` 14F+3U, `v2-adapter` 6F+4U, `v2-path-claims` 7F+3U = `archived-pending`; out-of-range → fail), 29 completed-spec IDs require archived evidence; unknown ID fails; prints every ID/status + `TOTAL=92 STRICT=4 INFO=88`; default runs the 4 tests; `--check-only` skips; fail-closed exit 0/1.
- [x] 3.3 MAJOR validator fix: archived evidence accepts BOTH `<archive>/<dir>/spec.md` flat (composicion, distribucion, reporte, store) AND `<archive>/<dir>/specs/<name>/spec.md` (reconciliacion, spike-go, no-regresion, adapter, path-claims); `verify-report.md` at change-dir level. RED fixture satisfied: non-allowlisted complete `v2-reconciliacion` resolves via directory layout (verified in real-root run output: `info:archived v2-reconciliacion/F-01`).

### Phase 4 — Property tests (internal/cmd/flow_test.go)
- [x] 4.1 `TestFlowEachSpecIsSDDChange` (F-01): live name + alias line + 7-phase cycle in §Usage and F-01 criterion + tracking row + full archived artifact sets for the 5 completed specs (flat and dir layouts). RED proven against pre-edit docs baseline.
- [x] 4.2 `TestFlowVerificationViaVerify` (F-02): race/local runners, E2E-vs-module wording, CI gates in §Usage/AGENTS.md/ci.yml; header + conventions stack lines Go-correct; rejects `node --test`/`npm test`/`tsc --noEmit` on every flow surface.
- [x] 4.3 `TestFlowDeliveryThroughGates` (F-03; validator MINOR): `.docs/initiatives/` + `tools/scripts/initiative/` absent on disk; contract names `SDD flow gates (review receipts, delivery gates)`; AGENTS.md requires PR work units + green CI; ci.yml has `pull_request` review gate, push-to-main delivery gate, and jobs build-and-test/lint/vulncheck; hybrid clause: docs-only archive receipts (verify-report) accepted post-verify, feature code via reviewed PR.
- [x] 4.4 Focused run green (`go test ./internal/cmd -run '^TestFlow' -count=1` → ok); script default mode executes all four tests (`== running D09 flow tests == ok`); real-root `--check-only` green.

### Phase 5 — Final gates
- [x] 5.1 `go build ./...` OK; `go vet ./...` OK; `go test ./... -race -count=1` → 14 packages ok; `golangci-lint run` → 0 issues.
- [x] 5.2 `bash scripts/verify-traceability.sh` (default) → exit 0.
- [x] 5.3 No regression: `verify-store-boundary.sh` exit 0; `verify-distribution.sh` exit 0.
- [x] 5.4 Grep decision: alias stays literal (F-01 mandates it); exactly 1 `gentle-ai` hit in deltas-acceptance.md (the alias line 456), 0 in AGENTS.md, 0 in docs/v2/, scripts/, .github/.
- [x] 5.5 `git status --porcelain`: only pre-existing untracked tooling dirs (`.atl/`, `.engram/`, `testdata/compose/symlink-escape/...`) plus this change's artifacts (committed as `docs(sdd)`). Tracked changes are exactly the 4 intended files.

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 3.1/3.2 | `internal/cmd/flow_test.go` (TestFlowCriterionTraceability) | Integration (bash auditor) | ✅ internal/cmd ok 1.802s | ✅ exit 127 ×6 | ✅ 6/6 subtests pass | ✅ 6 scenarios (4 roots + 2 fail-closed) | ➖ None needed |
| 4.1 | `internal/cmd/flow_test.go` | Unit (docs property) | ✅ same | ✅ fails vs pre-edit docs | ✅ pass | ✅ alias count + 5 archived sets | ➖ None needed |
| 4.2 | `internal/cmd/flow_test.go` | Unit (docs property) | ✅ same | ✅ fails vs pre-edit docs | ✅ pass | ✅ 3 surfaces × stale runners | ➖ None needed |
| 4.3 | `internal/cmd/flow_test.go` | Unit (fs + docs property) | ✅ same | ✅ fails vs pre-edit docs | ✅ pass | ✅ fs absence + 3 doc surfaces | ➖ None needed |

**Test summary**: 8 test functions/subtest groups written (4 named tests, 10 leaf subtests in U-01, 5 archived-spec subtests in F-01). All passing. Layers: Unit (3), Integration (1). Approval tests: none — no refactoring tasks. Pure functions: `traceabilityFixture`, `assertAuditOK`, `section`, `d09Section` helpers.

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/cmd -run '^TestFlow' -count=1` → `ok github.com/HectorCortes/haro/internal/cmd 0.375s` |
| Runtime harness command/scenario and exact result | `bash scripts/verify-traceability.sh` → exit 0, `TOTAL=92 STRICT=4 INFO=88`, runs the 4 flow tests (`ok ... 0.282s`); `--check-only` real root exit 0; fixtures (non-git/relative/git) exit 0; duplicate-ID and removed-mapping exit 1 with named ID/criterion/test |
| Rollback boundary | Unit 1: revert commits b3cd9f0 (deltas) + 254c375 (AGENTS). Unit 2: revert commit 4e78511 (script + flow_test.go). No product code, deps, `openspec/` history, or `docs/v2/` affected |

## Deviations from Design

1. **Sequencing**: the three property tests (4.1–4.3) were written together with `TestFlowCriterionTraceability` (3.1) before the script implementation, because the auditor's strict mapping check requires all four test functions to exist for the real-root `--check-only` GREEN. Their individual RED was proven against the pre-edit docs baseline instead of pre-edit working tree.
2. **Toolchain discovery**: go1.27.0 rejects mixed positional+spread calls (`f(a, b, args...)`) in any variadic function; the test harness builds argument slices explicitly (`append([]string{script}, args...)`).
3. **Test assertion fix**: initial enumeration assertion used non-existent `v2-composicion/F-11` (spec has F-01..F-07 + U-01..U-04); corrected to `v2-composicion/U-04`.
4. None of these deviate from the spec's requirements or the design's file contract.

## Delivery / Workload

- Mode: single-pr, `size:exception` pre-approved (budget 200000); direct push to main, conventional commits, no PRs opened, no push executed.
- Work units: (1) docs(contract) b3cd9f0, (2) docs(agents) 254c375, (3) feat(cmd) 4e78511 (script + tests together), (4) docs(sdd) — this change's artifacts.

## Next Step

`sdd-verify` for v2-flujo-sdd.
