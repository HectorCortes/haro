# Delta for v2-ipc

> Dependency: these replacements apply after `2026-09-11-v2-broker-ipc` establishes F-03–F-06, U-01, U-02, and U-04.

## MODIFIED Requirements

### Requirement: Non-blocking step.run [v2-ipc/F-03] — P0 [E2E]

`step.run` MUST return `{attempt_id,next_cursor}` without waiting for completion. For supervised mode, it MUST wait only for readiness proving frozen admission and a started fenced session; failure before readiness MUST return a structured error without provider work or headless downgrade. Terminal mode MUST remain rejected.

(Previously: supervised and terminal were both rejected without attempts.)

#### Scenario: Ready supervised return
- GIVEN slow headless, ready supervised, unready supervised, and terminal requests
- WHEN `step.run` processes each
- THEN headless and ready supervised return early while unready supervised and terminal fail closed

### Requirement: Idempotent step.approve [v2-ipc/F-05] — P0 [E2E]

`step.approve` MUST accept only a pending interaction's `available_decisions`, atomically move it through `resolving`, and return the stored result for the same idempotency key without duplicate provider effects. Conflicting, foreign, unknown, resolution-failed, or unsafe post-crash requests MUST fail closed.

(Previously: resolution used a pending-to-resolved CAS without an explicit provider-application state.)

#### Scenario: Approval lifecycle
- GIVEN pending, duplicate, conflicting, foreign, and interrupted resolutions
- WHEN each approval is submitted
- THEN only one authorized provider effect occurs and uncertain application orphans safely

### Requirement: step.cancel [v2-ipc/F-06] — P1 [E2E]

`step.cancel` MUST route to the active supervised endpoint generation, enter `cancelling`, send `session/cancel`, persist the confirmed outcome, invalidate the lease, and reject later stale writes. Unconfirmed shutdown MUST produce `orphaned`, not a fabricated terminal result.

(Previously: cancellation persisted `cancelled` and invalidated the lease without managed-generation and uncertain-shutdown semantics.)

#### Scenario: Active cancellation
- GIVEN active confirmed, unconfirmed, and stale-generation sessions
- WHEN `step.cancel` runs
- THEN confirmation cancels, uncertainty orphans, and stale routing has no effects

### Requirement: Monotonic persisted cursor [v2-ipc/U-01] — P0 [UNIT]

Cursors MUST be monotonic per attempt. Every bounded sanitized control event MUST commit before any response or notification exposes it; operational output MUST remain outside control payloads.

(Previously: commit-before-publication applied to bounded events without the managed output separation.)

#### Scenario: Publication crash
- GIVEN crashes before and after event persistence plus high-volume output
- WHEN consumption resumes
- THEN only committed events appear and no event contains full output

### Requirement: Interaction CAS boundary [v2-ipc/U-02] — P0 [UNIT]

Resolution MUST atomically enforce attempt ownership, `pending → resolving → resolved|resolution_failed`, and idempotency. Identical completed retries MUST return stored results; another key, decision, attempt, or stale endpoint generation MUST fail without provider re-execution.

(Previously: CAS covered pending/resolved, duplicate, conflicting, and foreign cases.)

#### Scenario: CAS table
- GIVEN every lifecycle state plus duplicate, conflicting, foreign, and stale-generation requests
- WHEN each resolves through RPC
- THEN only the valid transition applies once and unsafe cases preserve state

### Requirement: JSON-RPC boundary validation [v2-ipc/U-04] — P1 [UNIT]

Incoming and outgoing CLI↔Broker and ACP stdio payloads MUST be schema-validated. Unknown methods or fields, missing data, malformed framing, invalid enums, unsupported capabilities, and identity/generation mismatch MUST return stable structured fail-closed errors with no state effects.

(Previously: validation covered broker JSON-RPC payloads only.)

#### Scenario: Invalid payloads
- GIVEN malformed, unknown, unsupported, and stale-identity payloads at both boundaries
- WHEN validation runs
- THEN each identifies the failure and persisted state remains unchanged
