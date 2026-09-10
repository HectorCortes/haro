# Archive Report: v2-supervised-guard (V2 Supervised Guard)

**Change**: 2026-09-10-v2-supervised-guard
**Archived**: 2026-09-10
**Archived to**: `openspec/changes/archive/2026-09-10-v2-supervised-guard/` (hybrid — OpenSpec files + Engram)
**Mode**: hybrid
**Spec synced to**: `openspec/specs/v2-no-regresion/spec.md` (1 MODIFIED requirement block: `F-02`, plus 1 ADDED scenario)

## Goal

Eliminate the silent runtime downgrade where an agent step declared with `mode: supervised` executes through the headless path. Until broker, IPC, and supervisor support exists, runtime execution fails closed with the distinct terminal reason `supervised mode not supported` — before harness intersection, fallback, or adapter invocation — while `supervised` remains a valid declared mode at validation time. The change modifies one requirement block (`v2-no-regresion/F-02`), adds one scenario ("Unsupported declared execution modes"), and adds no acceptance ID, preserving the 92/11 traceability invariant.

## Final State (authoritative at close — outranks any intermediate snapshot)

```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:759379c9536746a6442f051e02fa4fd5db0ab594149db819460f0a6b08ec19ff
verdict: pass
blockers: 0
critical_findings: 0
requirements: 1/1
scenarios: 2/2
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:759379c9536746a6442f051e02fa4fd5db0ab594149db819460f0a6b08ec19ff
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

- **Verify final verdict (PASS)**: fresh `-count=1` evidence — `go test -count=1 ./...` exit 0 (15 test-bearing packages green, 2 packages report no test files); `go test -count=1 ./... -race` exit 0 (15 green, 0 FAIL); focused `TestSupervisedAgentStepFailsClosed` and `TestTerminalAgentStepRegressionStoreSeeded` PASS; `go vet ./...` exit 0; `go build ./...` exit 0; `golangci-lint run ./internal/execution` exit 0 (`0 issues.`). Evidence revision hash `sha256:759379c9536746a6442f051e02fa4fd5db0ab594149db819460f0a6b08ec19ff` recorded in `verify-report.md`.
- **Corrective re-run rationale (final state)**: the first verify run failed solely because `apply-progress.md` (OpenSpec) and Engram obs #2577 lacked the canonical per-task Strict TDD Cycle Evidence table. Both artifacts were corrected to contain the canonical table (tasks 1.1, 1.2, 2.1, 3.1, 3.2 rows with Safety net/RED/GREEN/TRIANGULATE/REFACTOR columns), and the corrective verify re-run PASSED. This is the state at close; the earlier TDD-notation issue is resolved.
- **Completeness at close**: 1/1 requirement (`v2-no-regresion/F-02`), 2/2 scenarios (Discovery; Unsupported declared execution modes).
- **Tasks**: 8/8 complete. The archived filesystem `tasks.md` (target of record for hybrid mode) was read in full in this phase: 8 `[x]`, 0 `[ ]`. Task Completion Gate: PASS. Engram obs #2576 is the tasks-phase snapshot (planning-time unchecked boxes) — intermediate history only, superseded by the filesystem artifact at close.
- **Verify-report attestation**: full `verify-report.md` was read (corrective re-run, frontmatter `verdict: pass` — the schema signal). 0 blockers, 0 CRITICAL, 1/1 requirements, 2/2 scenarios. No CRITICAL findings exist at close.
- **Review receipt gate**: `reviewGate` is structurally ABSENT — no review was ever started for this candidate and receipt-driven development is not enabled. Archive proceeds under ordinary repository policy.
- **Delivery**: `single-pr` with maintainer pre-approved `size:exception` (200000-line review budget), direct push to `main`, no PR. Implementation commits are in `main` (local, NOT pushed); the orchestrator pushes implementation + archive commits after this phase (this phase does NOT push).

## Commits (all in main, NOT pushed — orchestrator pushes after archive)

| Commit | Message | Scope |
|---|---|---|
| `95f7f1b` | test(execution): supervised mode fail-closed tests | U1: `internal/execution/engine_test.go` +135 |
| `fa7a2b6` | feat(execution): reject supervised mode terminally at runtime | U2: `internal/execution/engine.go` 4+/3- |

Tracking range `fd15ca7..HEAD` changes only `internal/execution/engine.go` (7 +- total) and `internal/execution/engine_test.go` (+135): 2 files, 139 insertions, 3 deletions. No docs/v2, no `deltas-acceptance.md`, no validation/fallback/adapter/store/CLI changes in the implementation.

## Criteria Covered (criterion → scenario → named Go test, per the change's spec)

| ID | Scenario | Named Go test | Result at close |
|----|----------|---------------|-----------------|
| `v2-no-regresion/F-02` | Discovery | `internal/cmd/e2e_test.go > TestE2EWorkflowsDiscovery`; `internal/cmd/execute_test.go > TestCLI_Routing` | COMPLIANT (pre-existing coverage) |
| `v2-no-regresion/F-02` | Unsupported declared execution modes | `internal/execution/engine_test.go > TestSupervisedAgentStepFailsClosed` | PASS — distinct reason `supervised mode not supported`; step/execution `failed`; 0 attempts; 0 adapter sessions despite a usable injected fake adapter; no fallback/headless downgrade |
| `v2-no-regresion/F-02` | Unsupported declared execution modes (terminal case) | `internal/execution/engine_test.go > TestTerminalAgentStepRegressionStoreSeeded` | PASS — unchanged `terminal mode not supported`; step/execution `failed`; 0 attempts; 0 sessions (store-seeded regression pin) |

`deltas-acceptance.md` is UNCHANGED: the `v2-no-regresion/F-02` row (line 94) remains `[ ]` unchecked per the parent-spec convention (this delta extends the spec; it does not claim the acceptance row), and this change adds NO new acceptance IDs — the 92-criteria/11-spec invariant is preserved.

## Spec Sync (Step 2 — completed before the archive move)

| Domain | Action | Details |
|--------|--------|---------|
| `v2-no-regresion` | Updated | Main spec existed. Merged the MODIFIED `F-02` requirement block by replacing the matching requirement text with the delta's updated text; ADDED the new scenario "Unsupported declared execution modes" after the preserved "Discovery" scenario; `(Previously: …)` note dropped as delta bookkeeping; all other requirements (`F-01`, `F-03`, `F-05`, `F-07`, `F-12`, `F-13`, `F-14`, `U-01`, `U-03`) and the "Non-Requirements and Ownership" section preserved untouched. |

Faithful-merge verification: the merged F-02 block was compared against the delta's MODIFIED source with `(Previously: …)` bookkeeping removed — byte-identical match (scripted extraction + `diff`). No REMOVED or RENAMED sections exist in the delta, so no destructive-merge warning was required (`openspec/config.yaml` rule `archive: Warn before merging destructive deltas` not triggered).

**Verbatim spec sync verification output:**
```
=== SPEC SYNC VERIFICATION: MODIFIED BLOCK MATCH (after bookkeeping removal) ===
=== F-02: MATCH ===
```

Post-merge structural check: git diff of the main spec shows exactly the F-02 text replacement + the added scenario block; nothing else changed (verified by full `git diff` review). Main spec is now 126 lines, 10 requirement headings (`F-01`..`F-03`, `F-05`, `F-07`, `F-12`..`F-14`, `U-01`, `U-03`) — unchanged count.

## Archive Move (Step 3 — MANDATORY mechanical copy)

- Change folder `openspec/changes/2026-09-10-v2-supervised-guard/` was untracked (verified with `git ls-files` empty); `git mv` correctly declined ("source directory is empty"), and the mechanical `mv` fallback moved the whole folder to `openspec/changes/archive/2026-09-10-v2-supervised-guard/` in one transaction. Git will track the moved files with the archive commit.
- Snapshot taken before the move: `cp -R` of the change folder into `$(mktemp -d …)`, removed by EXIT trap after readback.

**Verbatim mandatory `diff -r` readback output (Step 3):**
```
=== MANDATORY diff -r readback (snapshot vs archived) ===
diff exit 0 — byte-identical
```

Empty diff, exit 0 — the archived tree is byte-identical to the pre-move snapshot (`archive-report.md` excluded: additive, it did not exist in the source snapshot).

## Archive Contents (verified present, byte-identical to snapshot)

- `proposal.md` ✅
- `specs/v2-no-regresion/spec.md` ✅
- `design.md` ✅
- `tasks.md` ✅ (8/8 `[x]`, 0 unchecked)
- `apply-progress.md` ✅ (corrected canonical TDD Cycle Evidence table)
- `verify-report.md` ✅ (corrective re-run, verdict pass)
- `archive-report.md` ✅ (this file, additive)

Active `openspec/changes/` no longer contains this change.

## Engram Observations (read in this phase for traceability)

| Observation ID | Artifact / Topic | Note |
|---|---|---|
| 2572 | `sdd/v2-supervised-guard/proposal` | read |
| 2573 | `sdd/v2-supervised-guard/spec` | read (delta) |
| 2574 | `sdd/v2-supervised-guard/design` | read |
| 2576 | `sdd/v2-supervised-guard/tasks` | read; snapshot from tasks phase (planning-time unchecked boxes) — filesystem `tasks.md` is the target of record for hybrid mode and shows 8/8 `[x]` |
| 2577 | `sdd/v2-supervised-guard/apply-progress` | read; corrected content matches the OpenSpec apply-progress artifact (canonical TDD Cycle Evidence table) |
| 2579 | `sdd/v2-supervised-guard/apply-validation` | read via search; apply conformance validation PASS |
| 2580 | `sdd/v2-supervised-guard/verify-report` | read; corrective re-run, final verify state |

The archive report is persisted to Engram as topic `sdd/v2-supervised-guard/archive-report` (type `architecture`).

## Non-Blocking Notes

1. **Terminal regression is green-from-start by design**: `CreateExecution` rejects `mode: terminal` during `ValidateFile`, so the terminal runtime regression must seed the execution directly through the store (recorded in design.md and the corrected apply-progress). This is a documented deviation from a literal RED expectation, not a missing test or defect; fresh runtime evidence confirms the preserved behavior. Verify-report WARNING 1; no production impact.
2. **TDD evidence correction history**: the first verify run failed solely on the missing canonical TDD Cycle Evidence table in `apply-progress.md`/Engram #2577; corrected artifacts contain it and the corrective re-run passed. No code changed between the two runs.
3. **Supervised mode remains declaratively valid**: validation code (`internal/workflow/validate.go`) is unchanged; enforcement is runtime-only in `internal/execution/engine.go` (single parameterized branch `mode == "terminal" || mode == "supervised"` at lines 789-792), so future broker/IPC/supervisor support can ship without a spec-mode revert.
4. **Rollback boundary**: revert `95f7f1b` + `fa7a2b6` as one unit (guard + its tests + F-02 delta); reverting only the guard would restore the unacceptable silent headless downgrade.

## SDD Cycle Complete

The change was fully planned, specified, designed, implemented (Strict TDD RED→GREEN, 2 work-unit commits), verified (verdict PASS, 0 blockers, 0 CRITICAL, 1/1 requirements, 2/2 scenarios), and archived. The main spec for `v2-no-regresion` (F-02) now reflects the fail-closed runtime rejection of declared-but-unimplemented execution modes. Ready for the next change.