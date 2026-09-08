# Archive Report: v2-evidencia (Inline Attempt Evidence and Reopen Recovery)

**Change**: v2-evidencia
**Archived**: 2026-09-08
**Archived to**: `openspec/changes/archive/2026-09-08-v2-evidencia/` (specs/ subdirectory layout preserved, per orchestrator mandate)
**Mode**: hybrid (OpenSpec files + Engram)
**Spec synced to**: four `openspec/specs/<domain>/spec.md` targets (details in Spec Sync section)

## Goal

Implement inline, bounded attempt evidence: new attempts store sanitized, redacted evidence in nullable `attempt_events.payload` (≤16 KiB) instead of files, legacy `payload_ref` rows remain readable, execution-identifying headers compose into the budget, feedback reconstruction is database-first with legacy fallback, and `requires` recovery uses only the producer's latest valid current generation after reopen. Eight existing acceptance criteria (`v2-no-regresion/F-05`, `F-07`, `F-12`; `v2-store/F-02`, `U-01`, `U-02`; `v2-ipc/U-03`; `v2-flujo-sdd/U-01`) map to named Go tests; the change adds no acceptance ID.

## Final State (authoritative at close — outranks any intermediate snapshot)

```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:29382d51c163d298f42eb9dc79ded851d3a082e7dcc8d139740a4ea1289ee3dd
verdict: pass
blockers: 0
critical_findings: 0
requirements: 11/11
scenarios: 19/19
```

- **Verify command**: `go test ./... -race -count=1` exit 0 (output hash `sha256:26ffa4d2…3c375896`); `go build ./...` exit 0 (empty output). Engram obs 2513; file `verify-report.md` in the archived folder.
- **Completeness at close**: 11/11 requirements (4 v2-evidencia, 1 v2-ipc, 3 v2-no-regresion, 3 v2-store), 19/19 scenarios (7/2/5/5). The `v2-flujo-sdd/U-01` traceability criterion is executed as `TestEvidenceCriterionTraceability` (8 mapping subtests, PASS) but excluded from the 11/19 totals because no `v2-flujo-sdd` spec file was supplied to this change.
- **Tasks**: 17/17 complete. The archived `tasks.md` was read in full in this phase: 17 `[x]`, 0 `[ ]`. Task Completion Gate: PASS — no stale unchecked implementation boxes in the persisted tasks artifact (the OpenSpec filesystem artifact is the target of record for hybrid mode).
- **Gates green at close**: `go build ./...` exit 0; `go test ./internal/cmd/ -run 'TestFlowEachSpecIsSDDChange' -count=1` → ok (0.004s). The traceability assertion `| \`v2-flujo-sdd\` | Development flow | 4 | 3 | **complete** |` (flow_test.go:86) passes against the live contract.
- **Review receipt gate**: `reviewGate` is structurally ABSENT — no review folder and no `state.yaml` were ever created for this candidate and the receipt-driven kill switch is off for this candidate, so zero review code ran. Archive proceeds under ordinary repository policy (mirrors the `v2-flujo-sdd` archive, which likewise had no review folder).
- **Delivery**: `single-pr` with maintainer pre-approved `size:exception` (review budget 200000). Delivered as a direct push to `main`, no PR, per user delivery preference. Implementation commits are already in `main`; the orchestrator pushes this archive commit after this phase (the archive phase itself does NOT push).

## Commits (implementation range `98d431c~1..HEAD`, all already in main)

| Commit | Message | Scope |
|---|---|---|
| `98d431c` | feat(store): add inline attempt event payload | schema, migration, parity, PriorOutputDelta |
| `f54a0c8` | feat(execution): persist bounded evidence inline | formatters, inline persistence, DB-first feedback, doctrine |
| `9580341` | docs(sdd): mark v2-evidencia U1 and U2 tasks complete | tasks.md |
| `9492d36` | fix(execution): recover requires after producer rerun | latest-generation predicate, command+agent |
| `496af18` | docs(sdd): mark v2-evidencia U3 tasks complete | tasks.md |
| `73e067a` | docs(sdd): record v2-evidencia apply progress with TDD evidence | apply-progress.md |
| `cb42323` | test(cmd): align v2-flujo-sdd tracking assertion with archived complete row | flow_test.go:86 (one-line, test-only) |

`cb42323` is a test-only one-line correction (`pending` → `**complete**`) for a PRE-EXISTING flow-test failure against the already-archived v2-flujo-sdd row. It is not evidence or reopen work; it is included in this change's range and documented here for audit trail completeness. The pre-`cb42323` failure is preserved as a historical note in the archived `apply-progress.md` U3.4 section; the full race gate is green with the correction present (per Final-State Authority, the archive supersedes that stale observation).

## Criteria Covered (criterion → named Go test, per the change's acceptance table)

| ID | Named Go test | Result at close |
|----|---------------|-----------------|
| `v2-no-regresion/F-05` | `TestFeedbackReconstructionPrefersPayloadAndFallsBackToLegacyReference` | PASS |
| `v2-no-regresion/F-07` | `TestReopenRequiresCurrentGenerationRecovery` | PASS (GREEN only after U3.1, fail-closed) |
| `v2-no-regresion/F-12` | `TestAttemptEvidenceInlineBoundedAndRedacted` | PASS |
| `v2-store/F-02` | `TestAttemptEventsPayloadSchema` | PASS |
| `v2-store/U-01` | `TestAttemptEventPayloadBackendParity` | PASS |
| `v2-store/U-02` | `TestMigratePayloadIdempotent` | PASS |
| `v2-ipc/U-03` | `TestAttemptEventPayloadDoctrine` | PASS |
| `v2-flujo-sdd/U-01` | `TestEvidenceCriterionTraceability` | PASS; all 8 mapping subtests |

No acceptance ID is added by this change. The live contract tracking table is therefore UNCHANGED: no `v2-evidencia` row exists (the contract has no row for this capability), and none was added — adding one would break the repository's 92-criterion / 11-spec audit invariant (`scripts/verify-traceability.sh` → `TOTAL=92 STRICT=4 INFO=88`) enforced by `TestFlowCriterionTraceability`, and the repo convention is to record capability rows only when the contract defines acceptance IDs for them. The `v2-no-regresion` F-05/F-07/F-12 checkboxes remain `[ ]` pending in the contract (that spec's own row stays pending); `v2-store` criteria were already `[x]`.

## Spec Sync (Step 2 — completed before the archive move)

| Domain | Action | Details |
|--------|--------|---------|
| `v2-evidencia` | Created | NEW domain with no main spec; the delta IS a full spec (`# v2-evidencia Specification`). Copied MECHANICALLY via shell `cp` → `openspec/specs/v2-evidencia/spec.md`; `diff -r` readback exit 0, byte-identical. |
| `v2-ipc` | Created | No main spec existed; the delta is a MODIFIED-only delta (`# Delta for v2-ipc`, U-03). Composed per repo convention (v2-composicion precedent) into `openspec/specs/v2-ipc/spec.md`: title + Purpose authored, `## MODIFIED Requirements` → `## Requirements`, `(Previously: …)` paragraph dropped as delta bookkeeping, requirement + scenarios + Non-Requirements carried verbatim. |
| `v2-store` | Updated | Main spec existed. Merged MODIFIED blocks F-02, U-01, U-02 by replacing the matching requirement; `(Previously: …)` notes dropped as delta bookkeeping; all other requirements (F-01, F-03, U-03, Purpose, Constraints, Non-Requirements) preserved untouched. |
| `v2-no-regresion` | Updated | Main spec existed. Merged MODIFIED blocks F-05, F-07, F-12 (F-07 gains the new "Recovery" scenario); `(Previously: …)` notes dropped; all other requirements preserved untouched. |

Faithful-merge verification (MANDATORY readback): every merged requirement block was diffed against its delta MODIFIED source with only the `(Previously: …)` bookkeeping removed — all 7 comparisons byte-identical (6 requirement blocks + v2-ipc requirement/Non-Requirements sections; trailing block-separator blanks normalized). No REMOVED or RENAMED sections exist in any delta, so no destructive-merge warning was required (`openspec/config.yaml` rule `archive: warn before merging destructive deltas` not triggered).

## Archive Move (Step 3 — MANDATORY mechanical copy)

- Untracked artifacts (`design.md`, `exploration.md`, `proposal.md`, `specs/`, `verify-report.md`) were staged with `git add`; then the whole change folder was renamed with `git mv` to `openspec/changes/archive/2026-09-08-v2-evidencia/` (fallback `mv` not needed).
- Snapshot taken before the move: `cp -R` of the change folder into `$(mktemp -d …)`, removed by EXIT trap after readback.
- **Verbatim mandatory `diff -r` readback output (Step 3):**
  ```
  === MANDATORY diff -r readback (snapshot vs archived) ===
  diff exit 0 — byte-identical
  ```
  Empty diff, exit 0 — the archived tree is byte-identical to the pre-move snapshot (archive-report.md excluded: additive, it did not exist in the source snapshot).
- **Verbatim mandatory `diff -r` readback output (Step 2, v2-evidencia new main spec):**
  ```
  === diff -r (source vs temp) ===
  diff exit: 0
  === verify final ===
  FINAL IDENTICAL
  ```

## Archive Contents (verified present, byte-identical to snapshot)

- `proposal.md` ✅
- `specs/v2-evidencia/spec.md`, `specs/v2-ipc/spec.md`, `specs/v2-no-regresion/spec.md`, `specs/v2-store/spec.md` ✅
- `design.md` ✅
- `tasks.md` ✅ (17/17 `[x]`, 0 unchecked)
- `apply-progress.md` ✅ (tracked, renamed)
- `verify-report.md` ✅
- `exploration.md` ✅ (this change's exploration artifact; the pre-existing `testdata/compose/symlink-escape/.haro/workflows/lib/link.yaml` symlink fixture from commit `7d15107` was NOT touched — unrelated to this change)
- `archive-report.md` ✅ (this file, additive)

## Engram Observations (read in this phase for traceability)

| Observation ID | Artifact | Note |
|---|---|---|
| 2499 | `sdd/v2-evidencia/exploration` | read |
| 2501 | `sdd/v2-evidencia/proposal` | read |
| 2502 | `sdd/v2-evidencia/spec` | read |
| 2505 | `sdd/v2-evidencia/design` | read |
| 2508 | `sdd/v2-evidencia/tasks` | STALE planning snapshot with unchecked boxes — NOT authoritative; the filesystem `tasks.md` (17/17 `[x]`) governs per the Task Completion Gate. Recorded, not blocking (same situation as the v2-flujo-sdd archive, obs 2483). |
| 2513 | `sdd/v2-evidencia/verify-report` | read; final verify state |

No Engram observation exists for `apply-progress` (filesystem-only artifact for this change). The archive report is persisted to Engram as topic `sdd/v2-evidencia/archive-report` (type `architecture`).

## Non-Blocking Informational Notes

1. Agent/CLI adapter wiring remains a documented follow-up outside this change's scope; the synthetic agent evidence used by the evidence tests is unchanged.
2. The verify-time ledger settle attempt (request-id `settle-verify-1`, evidence hash `sha256:1e5ba85e…`) is informational; no review gate exists for this candidate (see Final State), so no receipt governs this archive.
3. `apply-progress.md` U3.4 documents the pre-`cb42323` `TestFlowEachSpecIsSDDChange` failure. Superseded at close by the green gate + the test-only correction commit; preserved as history in the archive.

## SDD Cycle Complete

The change was fully planned, specified, designed, implemented (Strict TDD RED→GREEN), verified (verdict pass, 0 blockers, 0 CRITICAL), and archived. Main specs for all four touched domains reflect the new behavior. Ready for the next change.