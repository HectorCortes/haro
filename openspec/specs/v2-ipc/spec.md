# v2-ipc Specification

## Purpose

Amend the attempt-event payload doctrine: `attempt_events.payload` MAY contain a sanitized, bounded evidence delta and MUST NEVER contain raw or full harness output, while `attempt_events.payload_ref` remains an external or legacy reference and is never repurposed for inline evidence.

## Requirements

### Requirement: Event payloads without raw output [v2-ipc/U-03] — P1 [UNIT]

`attempt_events.payload` MAY contain a sanitized, bounded evidence delta and MUST NEVER contain raw or full harness output. `attempt_events.payload_ref` MUST remain an external or legacy reference and MUST NOT be repurposed for inline evidence. Technical Specification §2 MUST declare nullable `payload TEXT`, and §2.1 MUST preserve this narrow distinction under Constitution VIII.2.

#### Scenario: Inline doctrine
- GIVEN an event produced by a new attempt
- WHEN its persisted fields and normative §2/§2.1 text are inspected
- THEN bounded sanitized deltas MAY appear in `payload` but raw or full output never does
- AND `payload_ref` remains null or an external/legacy reference

#### Scenario: Legacy reference doctrine
- GIVEN an event created before inline payload support
- WHEN its evidence is read
- THEN its external `payload_ref` remains valid without changing its historical meaning

### Requirement: execution.start [v2-ipc/F-01] — P0 [E2E]

`execution.start` MUST validate schema, references, and cycles before atomically creating an execution and returning `{execution_id}`.

#### Scenario: Workflow validation
- GIVEN valid and invalid workflows
- WHEN each starts
- THEN only valid input returns an ID; invalid input creates nothing

### Requirement: execution.status [v2-ipc/F-02] — P1 [E2E]

`execution.status` MUST return aggregate and step status consistent with persisted transitions.

#### Scenario: In-progress execution
- GIVEN progressed and pending steps
- WHEN status runs
- THEN aggregate and step states match transitions

### Requirement: Non-blocking step.run [v2-ipc/F-03] — P0 [E2E]

`step.run` MUST return `{attempt_id,next_cursor}` immediately and expose progress through `step.events`. It MUST reject `supervised` and `terminal` with `<mode> mode not supported`, creating no attempt or RPC bypass.

#### Scenario: Async and guarded modes
- GIVEN slow headless, supervised, and terminal requests
- WHEN `step.run` processes each
- THEN headless returns early; guarded modes return the engine reason without attempts

### Requirement: Stable step.events pagination [v2-ipc/F-04] — P0 [E2E]

`step.events {since_cursor}` MUST return `{events,next_cursor}` from the current attempt with stable order, no duplication or loss, and persisted sanitized payloads bounded to 16 KiB.

#### Scenario: Pagination retry
- GIVEN a persisted event sequence
- WHEN a page is retried then advanced
- THEN retries match and subsequent events appear once

### Requirement: Idempotent step.approve [v2-ipc/F-05] — P0 [E2E]

`step.approve` MUST accept only `available_decisions`, resolve by CAS, and return the same result for the same `idempotency_key` without duplicate effects. Unknown decisions MUST fail closed.

#### Scenario: Approval gate
- GIVEN a pending interaction
- WHEN an allowed decision repeats or an unknown decision arrives
- THEN the repeat has one effect; the unknown leaves state unchanged

### Requirement: step.cancel [v2-ipc/F-06] — P1 [E2E]

`step.cancel` MUST cancel the session, persist `cancelled`, invalidate its lease, and reject later stale writes.

#### Scenario: Active cancellation
- GIVEN an active session
- WHEN `step.cancel` runs
- THEN the harness is notified, `cancelled` persists, and later writes fail

### Requirement: step.reopen invalidation [v2-ipc/F-07] — P0 [E2E]

`step.reopen` MUST invalidate the generation cascade and return `{invalidated}` with affected step IDs.

#### Scenario: Cascade
- GIVEN a completed three-step chain
- WHEN its first step reopens
- THEN `{invalidated}` lists exactly the affected descendants

### Requirement: step.reopen feedback [v2-ipc/F-08] — P1 [E2E]

`step.reopen` MUST accept optional feedback, preserve history, and deliver it completely to the next attempt.

#### Scenario: Feedback
- GIVEN a completed step
- WHEN feedback accompanies reopen and rerun
- THEN the next attempt receives it completely and history remains

### Requirement: CLI notifications [v2-ipc/F-09] — P1 [E2E]

While events are consumed, the broker MUST emit cursor-bearing `step.status_changed` and `step.interaction_required` in persisted order.

#### Scenario: Subscription
- GIVEN a subscribed CLI
- WHEN status and interaction events publish
- THEN both notifications arrive in cursor order

### Requirement: Monotonic persisted cursor [v2-ipc/U-01] — P0 [UNIT]

Cursors MUST be monotonic per session or attempt. Every bounded event MUST commit before responses or notifications expose it.

#### Scenario: Publication crash
- GIVEN crashes before and after persistence
- WHEN consumption resumes
- THEN committed events remain discoverable and uncommitted events were never visible

### Requirement: Interaction CAS boundary [v2-ipc/U-02] — P0 [UNIT]

Resolution MUST atomically enforce pending state, attempt ownership, and idempotency: identical keys match; another key or attempt MUST fail.

#### Scenario: CAS table
- GIVEN pending, resolved, duplicate, conflicting, and foreign cases
- WHEN each resolves through RPC
- THEN only pending and identical duplicate cases succeed without re-execution

### Requirement: JSON-RPC boundary validation [v2-ipc/U-04] — P1 [UNIT]

Incoming and outgoing broker JSON-RPC payloads MUST be schema-validated. Unknown, missing, malformed, or invalid-enum fields MUST fail closed with `-32602`, stable message and field path, and no effects.

#### Scenario: Invalid payloads
- GIVEN malformed or unknown method and response fields
- WHEN validation runs
- THEN `-32602` identifies the field and state remains unchanged

## Non-Requirements

JSON-RPC or broker implementation is not introduced by this doctrine amendment.
