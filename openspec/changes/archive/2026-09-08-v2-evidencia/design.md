# Design: Inline Attempt Evidence and Reopen Recovery

## Technical Approach

Move only sanitized visible deltas from per-attempt files into `attempt_events.payload`, preserving `payload_ref` for legacy/external values. Both command and agent paths compose identifying context before the existing redact-then-16-KiB boundary. Feedback reads through the repository, and both `requires` gates evaluate only the latest producer generation. Snapshots, their 1-MiB limit, and 2-MiB reconstruction remain unchanged.

## Architecture Decisions

| Decision | Alternative | Rationale |
|---|---|---|
| Add nullable `payload`; retain `payload_ref` | Repurpose `payload_ref` | Additive compatibility preserves historical rows and doctrine. |
| Add `EventsRepository.PriorOutputDelta(ctx context.Context, attemptID string) (*AttemptEvent, error)` | Production `QueryForTest`/inline SQL | Keeps SQLite behind repositories and gives fake/SQLite parity. SQLite joins the current attempt to prior attempts of the same execution/step and selects the latest `output_delta` by attempt start, event cursor/id. |
| Select maximum generation number in engine code | Trust slice position | `ListByStep` currently guarantees `ORDER BY number` (`repositories.go:290`), but an explicit max remains backend/order robust. Apply it to command and agent checks for parity. |

## Data Flow

```text
ParseArgv/agent session -> compose header + output -> VisibleEvidence
    -> AttemptEvent{output_delta, Payload: &visible, PayloadRef: nil} -> SQLite/fake
feedback -> PriorOutputDelta -> payload if non-nil -> otherwise read payload_ref file
         -> prior + delimited feedback -> FallbackEvidence -> reconstruction event

reopen -> invalidate current producer generation -> downstream blocks
producer rerun -> append valid generation N+1 -> max(Number) is valid -> downstream runs
```

## Interfaces and Composition

`internal/execution/evidence.go` adds `AgentInstructionsLimit = 4 * 1024`, `func BoundAgentInstructions(string) string`, `func CommandEvidence(argv []string, stdout, stderr string) string`, and `func AgentEvidence(harness string, index, total int, mode, instructions, output string) string`. Command composition is `"$ "+strings.Join(argv, " ")+"\n"+stdout+stderr`. Agent composition is exactly `harness: <name> (<one-based>/<total>)\nmode: <mode>\ninstructions:\n<first 4 KiB>\n--- output ---\n<output>`. Each formatter calls `VisibleEvidence` once after full composition.

`AttemptEvent` gains `Payload *string`. `CreateAttemptEvent` inserts both columns. `PriorOutputDelta` returns both pointers; non-nil `Payload` wins even when empty, and only nil permits the legacy file read. Other event types (`feedback`, `reconstruction_context`, `permission_requested`) retain `payload_ref`.

## File Changes

| File | Change |
|---|---|
| `internal/store/migrations.go` | Add `payload TEXT` to fresh DDL and `ensurePayloadColumn`, called by `migrate`; mirror `ensureDagHashColumn`: `PRAGMA table_info(attempt_events)`, `ALTER TABLE ... ADD COLUMN payload TEXT`, and `isDuplicateColumnError` race guard. |
| `internal/store/store.go`, `repositories.go`, `fake.go` | Extend event model/write/copy logic and implement `PriorOutputDelta`; preserve cursors and deep-copy both pointers. |
| `internal/store/contract/suite.go`, `parity_test.go`, `migrations_payload_test.go` | Cover null/non-null retrieval, backend parity, schema, retained records, repeated/concurrent migration. |
| `internal/execution/evidence.go`, `engine.go` | Add formatters; compose at `RunStep` near current line 620 and `runAgentStep` near 899; remove `evidence/` creation/writes. Replace feedback file-first logic near 558 with repository-first fallback. Share latest-generation predicate across command and agent gates. |
| `internal/execution/evidence_inline_test.go`, `feedback_test.go`, `state_test.go` | Add evidence/reconstruction/recovery tests; replace evidence-file expectations with payload/no-directory assertions while retaining snapshot assertions. |
| `internal/cmd/evidence_contract_test.go` | Add doctrine and criterion traceability tests. |
| `docs/v2/haro-especificacion-tecnica.md` | In §2 DDL, add `payload TEXT -- sanitized bounded evidence delta; never raw/full output`. In §2.1 replace the payload-reference note with: payload may contain only a sanitized bounded delta; `payload_ref` remains an external/legacy sanitized blob/file reference and is never inline evidence. |

## Testing and Work Units

Strict RED→GREEN with `go test ./...` after each unit:

1. **U1** RED `TestAttemptEventsPayloadSchema`, `TestAttemptEventPayloadBackendParity`, `TestMigratePayloadIdempotent`; GREEN schema/store/fake/contract; commit `feat(store): add inline attempt event payload`.
2. **U2** RED `TestAttemptEvidenceInlineBoundedAndRedacted`, `TestFeedbackReconstructionPrefersPayloadAndFallsBackToLegacyReference`, `TestAttemptEventPayloadDoctrine`, `TestEvidenceCriterionTraceability`; GREEN formatters, DB-first feedback, no files, doctrine; commit `feat(execution): persist bounded evidence inline`.
3. **U3** RED `TestReopenRequiresCurrentGenerationRecovery` while existing immediate-post-reopen negative tests stay green; GREEN shared latest-only predicate for command and agent; commit `fix(execution): recover requires after producer rerun`.

## Threat Matrix

| Boundary | Applicability | Response / RED tests |
|---|---|---|
| Documentation-like paths | N/A: no executable classification changes | None |
| Git repository selection | N/A: no Git invocation changes | None |
| Commit state | N/A: no commit automation | None |
| Push state | N/A: no push automation | None |
| PR commands | N/A: no PR automation | None |

## Migration, Risks, Rollback

Migration is additive, idempotent, and nullable; old data remains readable. Risks are doctrine/history drift (explicit amendment plus legacy fallback), redaction/budget bypass (compose-first boundary tests), backend/migration drift (shared contract plus race/idempotency tests), and synthetic agent output (documented out of scope; adapter-manager wiring remains follow-up). Rollback restores file writes/reads and old doctrine while safely retaining the nullable column. No blocking open questions.
