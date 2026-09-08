# Proposal: v2-flujo-sdd (development flow)

## Intent

Deliver D09 (`F-01`–`F-03`, `U-01`, 4/4): rename `v2-flujo-gentle-ai` to `v2-flujo-sdd`, make project components tool-neutral, replace stale TypeScript/Node flow guidance with real Go gates, and add executable criterion-to-test auditing. The historical alias is **previously `v2-flujo-gentle-ai` (D09)**.

## Scope

### In Scope
- Update only flow-related text in `deltas-acceptance.md`: purpose/conventions, D09 IDs/title/tracking, and §Usage using the single term **SDD flow** and real Go commands. Preserve criterion intent; mark D09 complete only during archive.
- Remove `gentle-ai/` from `AGENTS.md:7`, leaving “openspec/SDD artifacts”.
- Add `scripts/verify-traceability.sh`, following existing fail-closed gates: strictly require D09's four named passing tests; report the other 88 IDs informationally with explicit CLI-direct deferrals (`v2-broker`, `v2-ipc`) and archived-pending allowances (`v2-no-regresion`, `v2-adapter`, `v2-path-claims`).
- Add `internal/cmd/flow_test.go` with `TestFlowEachSpecIsSDDChange`, `TestFlowVerificationViaVerify`, `TestFlowDeliveryThroughGates`, and `TestFlowCriterionTraceability`. This location follows the existing command/process gate harness. Tests inspect repository properties; the script invokes the named tests, avoiding recursive test→script→test execution.

### Out of Scope
- Historical `openspec/` content (31 hits remain verbatim, preserving evidence hashes); `docs/v2/` (0 hits); product code; `.docs/initiatives/`; D01/D02 delivery; PTY; dependencies; and `docs/reference/SPECS.md`.

## Capabilities

### New Capabilities
- `v2-flujo-sdd`: neutral SDD lifecycle, Go verification layers, delivery gates, and executable traceability.

### Modified Capabilities
- None.

## Approach

Adopt Approach 1: bounded rename/stale cleanup plus a two-level executable auditor. Approach 2 fails literal U-01; 3 leaves stale guidance and U-01 open; 4 violates archive immutability and explicit scope.

## Affected Areas

| Area | Impact |
|---|---|
| `deltas-acceptance.md`, `AGENTS.md` | Neutral, Go-current contract |
| `scripts/verify-traceability.sh` | New 92-ID/D09 audit |
| `internal/cmd/flow_test.go` | New four-test D09 harness |

## Risks

| Risk | Mitigation |
|---|---|
| Rename breaks searches | Preserve the D09 alias. |
| Strict 92-ID gate fails known gaps | Enforce 4 D09 IDs; classify 88 informationally. |
| Stale cleanup overreaches | Limit edits to the flow surface. |
| Wording or archive drift | Use “SDD flow”; prohibit archive edits. |

## Rollback Plan

Revert the D09 commits: documentation returns to prior wording, while additive script/test files can be removed independently. No data or dependency rollback is required.

## Dependencies

- Existing Bash, Go toolchain, `go test ./...`, Deltas, Constitution I.1/I.3/I.5, and the v1 oracle.

## Traceability and Success Criteria

| Criterion | Deliverable/proof |
|---|---|
| F-01 | SDD-cycle audit and named test |
| F-02 | Go-layer verification audit and named test |
| F-03 | gate/no-initiatives audit and named test |
| U-01 | 92-ID script, strict D09 mapping, named test |

- [ ] All 4/4 D09 tests and the auditor pass; scoped provider hits are removed; archived hashes remain unchanged; archive updates D09 checkboxes/tracking to complete.
