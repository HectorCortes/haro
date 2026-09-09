# Archive Report: v2-agent-wiring (V2 Agent Wiring)

**Change**: v2-agent-wiring
**Archived**: 2026-09-09
**Archived to**: `openspec/changes/archive/2026-09-09-v2-agent-wiring/` (hybrid — OpenSpec files + Engram)
**Mode**: hybrid
**Spec synced to**: `openspec/specs/v2-adapter/spec.md` (4 MODIFIED requirement blocks)

## Goal

Wire real adapter harnesses for agent steps through the CLI-direct adapter contract. Persist real session identity and sanitized output (≤16 KiB), replacing simulation. Add strict harness configuration with `yaml.v3` `KnownFields(true)`, ordered intersection fallback, and fail-closed behavior for unconfigured/unavailable harnesses. Four acceptance criteria (`v2-adapter/F-01`, `F-04`, `F-06`, `U-04`) map to 9 named Go test scenarios; the change adds no acceptance ID and preserves the 92/11 traceability invariant.

## Final State (authoritative at close — outranks any intermediate snapshot)

```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:d7e77f6f615ae1694733d37bfbb1f1c9154914efb0f6fa1ad26c3b33fcb9f7ae
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 4/4
scenarios: 9/9
test_command: go test ./... -race -count=1
test_exit_code: 0
test_output_hash: sha256:8d106e74f3a0f4043c7a33d34cc82e41bfd4196083fe49b553fd1ad01428491c
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

- **Verify command (round 3, PASS WITH WARNINGS)**: `go test ./... -race -count=1` exit 0 (output hash `sha256:8d106e74…491c`); `go build ./...` exit 0 (empty output); `go vet ./...` exit 0; `scripts/verify-adapter-boundary.sh` exit 0. `TestFlowCriterionTraceability` PASS (92/11 invariant). `deltas-acceptance.md` UNCHANGED (no acceptance ID added, per v2-evidencia precedent).
- **Completeness at close**: 4/4 requirements (`F-01`, `F-04`, `F-06`, `U-04`), 9/9 scenarios.
- **Tasks**: 15/15 complete (plus remediation tasks). The archived `tasks.md` was read in full in this phase: 15 `[x]`, 0 `[ ]`. Task Completion Gate: PASS.
- **Review receipt gate**: `reviewGate` is structurally ABSENT — no review was ever started for this candidate and the receipt-driven kill switch is off. Archive proceeds under ordinary repository policy.
- **Delivery**: `single-pr` with maintainer pre-approved `size:exception` (review budget 200000). Delivered as a direct push to `main`, no PR. Implementation commits are already in `main`; the orchestrator pushes the archive commit after this phase (this phase does NOT push).

## Commits (all in main, NOT pushed — orchestrator pushes after archive)

| Commit | Message | Scope |
|---|---|---|
| `f407c5a` | feat(project): strict optional harness configuration | U1: config.go + tests |
| `3758693` | feat(adapter): real OpenCode CLI-direct sessions and opencode-only factory | U2: opencode adapter + factory + manager |
| `c15b252` | feat(execution): ordered harness intersection and fail-closed agent steps | U3: engine.go + fallback |
| `4d4d4af` | test(execution): prove real agent evidence and transport identity persist | U4: evidence + transport |
| `1d0453e` | feat(cmd): wire one adapter manager per agent-capable CLI invocation | U5: cmd/execute.go |
| `c777510` | fix(execution): probe harnesses once per CLI invocation | Remediation round 1: probe-once |
| `89f1c0a` | fix(adapter): settle session completion on every Prompt failure path | Remediation round 1: cancel deadlock |
| `d57911e` | docs(sdd): record v2-agent-wiring apply progress with TDD evidence | Documentation |
| `4af648a` | docs(sdd): record v2-agent-wiring remediation round 1 evidence | Documentation |
| `c014ec4` | test(cmd): cover step reopen agent wiring end to end | Remediation round 2: reopen coverage |
| `3abd3c3` | docs(sdd): record v2-agent-wiring remediation round 2 evidence | Documentation |

## Criteria Covered (criterion → named Go test, per the change's spec)

| ID | Named Go test | Result at close |
|----|---------------|-----------------|
| `v2-adapter/F-01` | `TestAgentManagerCLIInjection` (+ `StepReopen`) | PASS |
| `v2-adapter/F-01` | `TestOpenCodeRealSessionLifecycle` | PASS |
| `v2-adapter/F-04` | `TestHarnessConfigKnownFields` | PASS |
| `v2-adapter/F-06` | `TestAgentHarnessIntersectionFallback` | PASS |
| `v2-adapter/F-06` | `TestAgentStepFailsWithoutConfiguredHarness` | PASS |
| `v2-adapter/U-04` | `TestAgentStepPersistsRealEvidenceAndTransport` | PASS |
| `v2-adapter/F-01` | `TestReopenDoesNotReprobeHarness` | PASS |
| `v2-adapter/F-01` | `TestManager_ProbeCachedOnce` | PASS |
| `v2-adapter/F-01` | `TestCancelReturnsAfterLaunchFailure` | PASS |

No acceptance ID is added by this change. The `v2-no-regresion/F-04` checkbox remains `[ ]` pending in `deltas-acceptance.md` — this change enables but does not claim it. The 92-criteria/11-spec invariant is preserved (no row added, `TestFlowCriterionTraceability` PASS).

## Spec Sync (Step 2 — completed before the archive move)

| Domain | Action | Details |
|--------|--------|---------|
| `v2-adapter` | Updated | Main spec existed. Merged MODIFIED blocks `F-01`, `F-04`, `F-06`, `U-04` by replacing the matching requirement blocks; `(Previously: …)` notes dropped as delta bookkeeping; all other requirements (`F-02`, `F-03`, `F-05`, `U-01`, `U-02`, `U-03`) preserved untouched. |

Faithful-merge verification (MANDATORY readback): all 4 merged requirement blocks were diffed against their delta MODIFIED sources with `(Previously: …)` bookkeeping removed — all 4 comparisons byte-identical. No REMOVED or RENAMED sections exist in the delta, so no destructive-merge warning was required (`openspec/config.yaml` rule `archive: warn before merging destructive deltas` not triggered). The delta's `## Non-Requirements` section is not a requirement block and was not merged into the main spec (per precedent).

**Verbatim spec sync verification output:**
```
=== Prior initialization: MATCH ===
=== Adapter boundary: MATCH ===
=== Ordered fallback: MATCH ===
=== Transport-neutral attempts: MATCH ===

=== SPEC SYNC VERIFICATION: ALL MODIFIED BLOCKS MATCH (after bookkeeping removal) ===
```

## Archive Move (Step 3 — MANDATORY mechanical copy)

- Untracked artifacts (`verify-report.md`) staged with `git add`; then the whole change folder was renamed with `git mv` to `openspec/changes/archive/2026-09-09-v2-agent-wiring/`.
- Snapshot taken before the move: `cp -R` of the change folder into `$(mktemp -d …)`, removed by EXIT trap after readback.

**Verbatim mandatory `diff -r` readback output (Step 3):**
```
=== MANDATORY diff -r readback (snapshot vs archived) ===
diff exit 0 — byte-identical
```
Empty diff, exit 0 — the archived tree is byte-identical to the pre-move snapshot (`archive-report.md` excluded: additive, it did not exist in the source snapshot).

## Archive Contents (verified present, byte-identical to snapshot)

- `proposal.md` ✅
- `specs/v2-adapter/spec.md` ✅
- `design.md` ✅
- `tasks.md` ✅ (15/15 `[x]`, 0 unchecked)
- `apply-progress.md` ✅ (tracked, includes remediation rounds 1 and 2)
- `verify-report.md` ✅
- `exploration.md` ✅
- `archive-report.md` ✅ (this file, additive)

## Engram Observations (read in this phase for traceability)

| Observation ID | Artifact | Note |
|---|---|---|
| 2530 | `sdd/v2-agent-wiring/explore` | read |
| 2531 | `sdd/v2-agent-wiring/proposal` | read |
| 2533 | `sdd/v2-agent-wiring/spec` | read |
| 2535 | `sdd/v2-agent-wiring/design` | read |
| 2537 | `sdd/v2-agent-wiring/tasks` | read; 15/15 complete |
| 2539 | `v2-agent-wiring adapter contract decisions and gotchas` | read (decision record) |
| 2538 | `Record v2-agent-wiring remediation round 2` | read |
| 2541 | `Verify v2-agent-wiring round 3 final` | read; final verify state |

The archive report is persisted to Engram as topic `sdd/v2-agent-wiring/archive-report` (type `architecture`).

## Non-Blocking Informational Notes

1. `v2-no-regresion/F-04` is enabled by this change (real E2E harness cycle) but NOT claimed — no acceptance ID added to `deltas-acceptance.md`, per v2-evidencia precedent. The checkbox remains pending until the contract is updated atomically.
2. Only OpenCode is registered; Claude and ACP remain unregistered. The Claude adapter retains a simulated prompt implementation that is unreachable through the OpenCode-only factory, explicitly allowed by the design. Registration of Claude/ACP requires real-contract adapters (out of scope).
3. Verify warnings are documentation-only: strict TDD artifact notation labels (`Written`/`Passed` vs `✅ Written`/`✅ Passed`) and new-file safety-net entries use `ok` instead of strict `N/A (new)`. No production defect.
4. `apply-progress.md` is the merged authoritative record of both implementation and remediation work (filesystem artifact is target of record for hybrid mode).

## SDD Cycle Complete

The change was fully planned, specified, designed, implemented (Strict TDD RED→GREEN, 2 remediation rounds), verified (verdict pass_with_warnings, 0 blockers, 0 CRITICAL, 4/4 requirements, 9/9 scenarios), and archived. The main spec for `v2-adapter` reflects the new behavior. Ready for the next change.
