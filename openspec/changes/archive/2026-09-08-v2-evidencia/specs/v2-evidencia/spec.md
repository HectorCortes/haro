# v2-evidencia Specification

## Purpose

Define inline, bounded attempt evidence and recoverable generation staleness while preserving legacy evidence reads and snapshot behavior.

## Requirements

### Requirement: Inline attempt evidence

New command and agent attempts MUST store sanitized evidence in nullable `attempt_events.payload`; their `output_delta` events MUST NOT use `payload_ref`. The complete evidence MUST be redacted and then limited to 16 KiB. New runs MUST NOT create evidence files or an `evidence/` directory. Snapshot files and their 1 MiB budget MUST remain unchanged. Legacy rows with only `payload_ref` MUST remain readable.

#### Scenario: Inline-only new run
- GIVEN a new attempt with credential-bearing output
- WHEN its evidence is persisted
- THEN `payload` is redacted and at most 16 KiB and `payload_ref` is null
- AND no evidence file or directory is created, while snapshots remain supported

#### Scenario: Legacy evidence
- GIVEN an old event whose `payload` is null and `payload_ref` identifies an existing evidence file
- WHEN prior evidence is requested
- THEN the sanitized legacy evidence remains readable

### Requirement: Execution-identifying headers

Command evidence MUST compose `$ <argv>` and visible output before redaction and the shared 16 KiB limit. Agent evidence MUST compose `harness: <name> (<index>/<total>)`, mode, instructions limited to 4 KiB, and session output before redaction and that same limit.

#### Scenario: Silent command
- GIVEN a command that emits no visible output
- WHEN evidence is recorded
- THEN `payload` contains its `$ <argv>` header and is non-empty

#### Scenario: Oversized agent evidence
- GIVEN oversized instructions and session output containing credentials
- WHEN an agent candidate records evidence
- THEN harness, one-based attempt index, total, and mode identify the attempt
- AND the persisted composition is redacted and at most 16 KiB

### Requirement: Database-first feedback reconstruction

Feedback reconstruction MUST use the prior attempt's DB `payload`; it MUST fall back to its legacy `payload_ref` only when inline payload is absent. Combined prior evidence and delimited feedback MUST remain sanitized and at most 2 MiB.

#### Scenario: Preferred and fallback sources
- GIVEN prior attempts represented once inline and once by a legacy reference
- WHEN each is reconstructed with feedback
- THEN the inline value is preferred and the legacy file is the fallback
- AND each bounded reconstruction preserves feedback and history

### Requirement: Current-generation staleness

A `requires` check MUST consider only each producer step's latest current generation. An invalid current generation MUST block downstream work; an older invalid generation MUST NOT block after a newer valid producer generation exists.

#### Scenario: Reopen blocks stale consumer
- GIVEN a completed producer and downstream consumer
- WHEN the producer is reopened and has not rerun
- THEN the downstream step is blocked by its invalid current generation

#### Scenario: Producer rerun recovers consumer
- GIVEN the producer was reopened and its old generation remains invalid
- WHEN the producer reruns successfully and creates a valid current generation
- THEN the downstream step reruns successfully

## Acceptance Traceability

| Existing criterion | Named Go test |
|---|---|
| `v2-no-regresion/F-05` | `TestFeedbackReconstructionPrefersPayloadAndFallsBackToLegacyReference` |
| `v2-no-regresion/F-07` | `TestReopenRequiresCurrentGenerationRecovery` |
| `v2-no-regresion/F-12` | `TestAttemptEvidenceInlineBoundedAndRedacted` |
| `v2-store/F-02` | `TestAttemptEventsPayloadSchema` |
| `v2-store/U-01` | `TestAttemptEventPayloadBackendParity` |
| `v2-store/U-02` | `TestMigratePayloadIdempotent` |
| `v2-ipc/U-03` | `TestAttemptEventPayloadDoctrine` |
| `v2-flujo-sdd/U-01` | `TestEvidenceCriterionTraceability` |

Each listed test MUST run under `go test ./...`; no acceptance ID is added.

## Non-Requirements

Adapter-manager wiring, snapshot pruning, legacy cleanup, broker behavior, and IPC implementation are outside this change.
