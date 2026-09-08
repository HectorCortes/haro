# Apply Progress: v2-evidencia (Inline Attempt Evidence and Reopen Recovery)

**Mode**: Strict TDD (RED→GREEN→REFACTOR per task, `go test ./...`)
**Delivery**: single-pr, maintainer-approved `size:exception` (200000) — direct push to main, no PR
**Status**: 17/17 tasks complete — ready for verify

## Commits

| Unit | Commit | Scope |
|---|---|---|
| U1 | `98d431c` feat(store): add inline attempt event payload | schema, migration, parity, PriorOutputDelta |
| U2 | `feat(execution)` (see git log) + `docs(sdd)` tasks | formatters, inline persistence, DB-first feedback, doctrine |
| U3 | `9492d36` fix(execution): recover requires after producer rerun | latest-generation predicate, command+agent |
| — | `496af18` docs(sdd): mark v2-evidencia U3 tasks complete | tasks.md |

## Work Unit Evidence

| Unit | Focused test command and exact result | Runtime harness | Rollback boundary |
|---|---|---|---|
| U1 | `go test ./internal/store/... -count=1 -race` → ok (store 4.46s), incl. `TestStoreInterchangeability/{fake,sqlite}/EventsPayload` PASS | N/A — unit/contract (no runtime boundary: store layer only) | revert `internal/store/{migrations,store,repositories,fake,contract/suite}.go` + the two test files; column is harmless if retained |
| U2 | `go test ./internal/execution/ ./internal/cmd/ -count=1` → ok / doctrine PASS (traceability F-07 subtest GREEN only after U3.1, fail-closed) | temp workflow run via `FakeRunner` inside `TestAttemptEvidenceInlineBoundedAndRedacted` (engine→store→payload assertions; real command boundary not required — runner is injectable by design) | revert `internal/execution/{evidence,engine}.go`, `internal/cmd/evidence_contract_test.go`, docs §2/§2.1; legacy payload_ref path returns |
| U3 | `go test ./internal/execution/ -run TestReopenRequiresCurrentGenerationRecovery -count=1 -v` → PASS (both subtests); then full gate below | N/A — state tests (reopen/recovery exercised through Engine against SQLite store) | revert `latestInvalidGeneration`/`requiresStale` + two call sites; old any-invalid behavior returns |

## Final Gate (U3.4)

- `go build ./...` → OK
- `go vet ./...` → OK
- `go test ./... -race -count=1` → all packages ok EXCEPT one pre-existing failure: `TestFlowEachSpecIsSDDChange` (internal/cmd) — stale assertion `flow_test.go:86` expects tracking row `pending` while commit `df0d8b9` (v2-flujo-sdd archive) set it to `**complete**`. Pre-existing on HEAD before any v2-evidencia work; NOT fixed (out of scope, reported).
- `golangci-lint run` → 0 issues
- All 8 named traceability tests green: `TestAttemptEventsPayloadSchema`, `TestMigratePayloadIdempotent`, `TestAttemptEventPayloadBackendParity`, `TestAttemptEvidenceInlineBoundedAndRedacted`, `TestFeedbackReconstructionPrefersPayloadAndFallsBackToLegacyReference`, `TestReopenRequiresCurrentGenerationRecovery`, `TestAttemptEventPayloadDoctrine`, `TestEvidenceCriterionTraceability` (8/8 PASS).

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| U1.1 | `internal/store/migrations_payload_test.go` | Unit | 34/34 store pkgs ok | ✅ no payload column | ✅ passed | ✅ DDL text + PRAGMA type/nullability | ➖ None needed |
| U1.2 | same | Unit | same | ➖ (RED from U1.1) | ✅ passed | ➖ covered by U1.1 | ✅ helper mirrors ensureDagHashColumn |
| U1.3 | same | Unit | same | ✅ payload column count 0 | ✅ passed (-race) | ✅ repeated + concurrent opens + records intact | ➖ None needed |
| U1.4 | `internal/store/parity_test.go` | Unit+Integration | store ok | ✅ compile-RED (undefined PriorOutputDelta/Payload) | ✅ passed both backends | ✅ null/non-null/empty/not-found × fake+sqlite | ✅ restructured scenarios to correct prior semantics |
| U1.5 | contract/suite.go `EventsPayload` | Contract | interchange suite ok | ✅ via U1.4 RED | ✅ fake+sqlite PASS | ✅ legacy/inline/empty/cross-step | ➖ None needed |
| U1.6 | parity_test + suite | Unit | same | ✅ same RED | ✅ passed | ✅ ordering + self-exclusion | ➖ None needed |
| U2.1 | `internal/execution/evidence_inline_test.go` | Unit+Integration | execution pkg ok | ✅ compile-RED (undefined formatters) + engine file asserts | ✅ passed | ✅ 4 subtests (silent cmd, composer, agent redact, agent e2e) | ✅ fixed test ordering/assertion bugs |
| U2.2 | same | Unit | same | ➖ (RED from U2.1) | ✅ passed | ✅ index (1/2)/(2/2), oversized, redaction | ➖ None needed |
| U2.3 | same | Integration | same | ✅ evidence-dir + payload_ref asserts | ✅ passed | ✅ agent fallback candidates | ➖ None needed |
| U2.4 | `internal/execution/feedback_test.go` | Integration | feedback_test ok | ✅ 3 scenarios: no reconstruction_context | ✅ passed | ✅ inline/legacy-fallback/2MiB-bound | ➖ None needed |
| U2.5 | same | Integration | same | ➖ | ✅ passed | ✅ covered by U2.4 scenarios | ✅ removed SQLite-only QueryForTest path |
| U2.6 | `internal/cmd/evidence_contract_test.go` | Contract | cmd pkg (1 pre-existing fail reported) | ✅ §2 text missing; F-07 test missing | ✅ doctrine PASS; traceability PASS except F-07→U3.1 (by design) | ✅ 8 criterion subtests | ➖ None needed |
| U2.7 | same | Contract | same | ➖ | ✅ doctrine PASS | ➖ literal text assertions | ✅ whitespace-normalized assertions |
| U3.1 | `internal/execution/state_test.go` | Integration | state_test ok | ✅ `requires invalidated (generation 1)` after valid rerun | ✅ passed | ✅ negative path + agent parity subtest | ➖ None needed |
| U3.2 | same | Integration | same | ➖ | ✅ passed both gates | ✅ command + agent + recovery | ✅ extracted shared predicate helpers |
| U3.3 | `internal/execution/feedback_test.go` | Integration | same | ➖ approval-style: new asserts describe new reality | ✅ passed | ✅ payload content, null ref, no evidence dir, snapshots | ✅ replaced stale comments |
| U3.4 | full gate | Gate | — | — | ✅ build/vet/race/lint (1 pre-existing unrelated red reported) | — | — |

**Test Summary**
- Total tests written: 13 test functions (3 new files, 3 existing files extended) + 8 criterion subtests
- Total tests passing: all except pre-existing `TestFlowEachSpecIsSDDChange` (stale, out of scope)
- Layers used: Unit (store formatters/migrations), Integration (engine paths against SQLite), Contract (dual-backend suite)
- Approval tests: U3.3 (new-reality asserts), none others needed
- Pure functions created: `BoundAgentInstructions`, `CommandEvidence`, `AgentEvidence`, `latestInvalidGeneration` (+`requiresStale` method)

## Deviations from Design

1. `PriorOutputDelta` "prior" = any other attempt of the same execution/step (self-exclusion), not "strictly earlier by started_at" — the design's join description is honored literally; strict `<` would break with same-second RFC3339 timestamps.
2. Doctrine test asserts the DDL comment with whitespace/`, --` normalization to keep the spec doc's column alignment.
3. `TestEvidenceCriterionTraceability` GREEN completes at U3.1 (F-07 test) — the task listed it under U2.6; traceability is intentionally fail-closed until then.

## Issues Found

- **Pre-existing failure (not fixed, out of scope)**: `TestFlowEachSpecIsSDDChange` in `internal/cmd/flow_test.go:86` expects tracking row `| v2-flujo-sdd | Development flow | 4 | 3 | pending |` but `deltas-acceptance.md:492` says `**complete**` since commit `df0d8b9` (v2-flujo-sdd archive). Suggested fix (needs orchestrator approval): update the test constant to `**complete**` or make it status-agnostic.
