# Tasks: Inline Attempt Evidence and Reopen Recovery (v2-evidencia)

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~1,400 |
| 400-line budget risk | High |
| Chained PRs recommended | No — pre-approved size:exception (200000) |
| Suggested split | Single delivery; U1 -> U2 -> U3 |
| Delivery strategy | single-pr; direct push to main, no PR |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

### Work Units

| Unit | Goal | Focused test | Runtime harness | Rollback |
|---|---|---|---|---|
| U1 | schema, migration, parity | `go test ./internal/store/...` | N/A — unit/contract | revert store files; column harmless |
| U2 | inline evidence, feedback, doctrine | `go test ./internal/execution/... ./internal/cmd/...` | temp workflow run | revert engine writes; legacy path returns |
| U3 | reopen recovery | `go test ./internal/execution/ -run Reopen` | N/A — state tests | revert predicate |

## Phase U1: Store

- [x] U1.1 RED `internal/store/migrations_payload_test.go`: `TestAttemptEventsPayloadSchema` (v2-store/F-02) — fresh DDL exposes nullable `payload TEXT`.
- [x] U1.2 GREEN `internal/store/migrations.go`: `payload TEXT` in fresh DDL; `ensurePayloadColumn` mirroring `ensureDagHashColumn` (PRAGMA probe, ADD COLUMN, `isDuplicateColumnError` guard), called by `migrate`.
- [x] U1.3 RED/GREEN `TestMigratePayloadIdempotent` (`migrations_payload_test.go`, v2-store/U-02): pre-payload DB with records; repeated/concurrent opens succeed, records unchanged.
- [x] U1.4 RED `internal/store/parity_test.go`: `TestAttemptEventPayloadBackendParity` (v2-store/U-01) — null/non-null `Payload` round-trips identically on SQLite and fake.
- [x] U1.5 GREEN `internal/store/store.go` (`AttemptEvent.Payload *string`), `repositories.go` (`CreateAttemptEvent` inserts both columns), `fake.go` (deep-copies both pointers), `contract/suite.go` (nullable-payload case).
- [x] U1.6 RED/GREEN `EventsRepository.PriorOutputDelta(ctx, attemptID)` in `repositories.go`+`fake.go`: prior same-execution/step latest `output_delta`; non-nil `Payload` wins even empty; fake parity in suite.

## Phase U2: Execution

- [x] U2.1 RED `internal/execution/evidence_inline_test.go`: `TestAttemptEvidenceInlineBoundedAndRedacted` (F-12) — silent command yields non-empty `$ <argv>` payload; oversized instructions + credential output stay redacted <=16 KiB, harness/index/total/mode present.
- [x] U2.2 GREEN `internal/execution/evidence.go`: `AgentInstructionsLimit = 4*1024`, `BoundAgentInstructions`, `CommandEvidence(argv, stdout, stderr)`, `AgentEvidence(...)`; each calls `VisibleEvidence` once after composition.
- [x] U2.3 GREEN `internal/execution/engine.go`: `RunStep` (~L620) and `runAgentStep` (~L899) persist `AttemptEvent{output_delta, Payload: &visible, PayloadRef: nil}`; remove `evidence/` writes; snapshots untouched.
- [x] U2.4 RED `internal/execution/feedback_test.go`: `TestFeedbackReconstructionPrefersPayloadAndFallsBackToLegacyReference` (F-05) — inline preferred, legacy `payload_ref` fallback; combined <=2 MiB.
- [x] U2.5 GREEN `engine.go` feedback (~L558): `PriorOutputDelta` DB-first; legacy file read only when `Payload == nil`; `FallbackEvidence` kept.
- [x] U2.6 RED `internal/cmd/evidence_contract_test.go`: `TestAttemptEventPayloadDoctrine` (v2-ipc/U-03) — §2/§2.1 declare nullable payload, never raw output; `TestEvidenceCriterionTraceability` (v2-flujo-sdd/U-01) — all 8 tests exist.
- [x] U2.7 GREEN `docs/v2/haro-especificacion-tecnica.md`: §2 DDL comment `payload TEXT -- sanitized bounded evidence delta; never raw/full output`; §2.1 note replaced per design.

## Phase U3: Recovery + gate

- [ ] U3.1 RED `internal/execution/state_test.go`: `TestReopenRequiresCurrentGenerationRecovery` (F-07) — reopen cascade -> producer rerun (valid gen N+1) -> downstream succeeds; negative tests stay green.
- [ ] U3.2 GREEN `engine.go`: shared latest-generation predicate (explicit max `Number`) in command (~L423-454) and agent (~L700-729) `requires`.
- [ ] U3.3 Update `feedback_test.go` and tests asserting evidence files: assert `payload` content, null `payload_ref`, no `evidence/` dir; snapshots retained.
- [ ] U3.4 `go build ./...`, `go vet ./...`, `go test ./... -race`, `golangci-lint run`; all 8 named tests green.
