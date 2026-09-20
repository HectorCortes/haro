# v2-supervised-acp Specification

## Purpose

Broker-owned ACP supervision contract.

## Requirements

### Requirement: ACP managed-session boundary [v2-adapter/U-03, v2-ipc/U-04]

`acp-generic` MUST launch an executable with ordered argv and allowed environment, without shell or PTY. Over JSON-RPC 2.0 stdio it MUST call `initialize` once before `session/*` and support `session/new`, `session/prompt`, `session/update`, `session/cancel`, and negotiated `session/request_permission`. Invalid or unsupported traffic MUST fail closed.

#### Scenario: Negotiated ACP cycle
- GIVEN compliant and invalid ACP fixtures
- WHEN a supervised session initializes and runs
- THEN valid traffic completes in order and invalid traffic has no effects

### Requirement: Immutable bundle admission [v2-no-regresion/F-08]

Before provider work, the system MUST freeze contained copies in semantic order for instructions, skills, required artifacts, and prior/fallback context. Admission MUST match `bundle_id`, manifest SHA-256, and mandatory-entry SHA-256 values; drift, escape, omission, or excess MUST fail closed. Bundle retention MUST default to 30 days and 256 MiB, remain configurable, and protect orphans.

#### Scenario: Exact admission and drift
- GIVEN frozen entries and a changed or escaping entry
- WHEN admission is verified
- THEN exact bytes are admitted and changed or escaping bytes prevent provider work

### Requirement: Fenced supervised lifecycle [v2-no-regresion/F-09, v2-broker/F-06,F-07]

The broker MUST persist attempt/session/transport identity, admission, state, endpoint generation, lease, fencing token, and heartbeat. States MUST include `starting`, `ready`, `running`, `awaiting_interaction`, `cancelling`, `completed`, `failed`, `cancelled`, and `orphaned`; terminal states MUST NOT transition automatically. `step.run` MUST return only after readiness proves admission and a started session.

#### Scenario: Ready owner and stale generation
- GIVEN a starting attempt and an expired prior generation
- WHEN readiness succeeds and both generations write
- THEN `step.run` returns identity and cursor while only the fenced owner persists

### Requirement: Permission resolution lifecycle [v2-no-regresion/F-09,F-11; v2-ipc/F-05,U-02]

Permission interactions MUST persist `pending → resolving → resolved|resolution_failed`, bind to one attempt, and expose only announced decisions. Identical retries MUST return the stored result without reapplication; conflicting, foreign, unknown, or ambiguous decisions MUST fail closed. A crash in `resolving` MUST orphan unless idempotent reconciliation is proven.

#### Scenario: Resolve exactly once
- GIVEN announced, duplicate, conflicting, and foreign decisions
- WHEN approvals are submitted
- THEN one provider effect resolves and all unsafe requests leave authority unchanged

### Requirement: Persisted events and retained output [v2-ipc/F-04,U-01,U-03]

Control events MUST commit before visibility with monotonic cursors and only bounded sanitized summaries and control data. Sanitized output MUST use contained retention defaulting to 30 days and 256 MiB with configurable limits. `step output --tail` MUST return a bounded path-free view without cursor movement; `--full` MUST be rejected outside terminal mode.

#### Scenario: Crash-safe event and output access
- GIVEN output containing credentials and a crash around publication
- WHEN events and output are read
- THEN only committed events appear and retained output is sanitized, bounded, and path-free

### Requirement: Independent supervised timers [v2-no-regresion/F-09]

Inactivity and decision timers MUST use independent counters and configurable 300-second defaults. Output or recognized progress MUST reset inactivity; heartbeat MUST NOT. Pending interaction MUST pause inactivity and start decision time; expiry MUST persist its distinct event, request safe cancellation, and orphan when shutdown is unconfirmed.

#### Scenario: Independent expiry
- GIVEN activity, heartbeat, and a pending interaction under a controllable clock
- WHEN each timer crosses its own deadline
- THEN only its defined reset/pause rules and terminal reason apply

### Requirement: Broker restart and guided orphan recovery [v2-broker/F-06,F-07]

Broker death or restart MUST reject stale writes and MUST NOT duplicate supervisors, restart orphans, or repeat authorization. Recovery MUST reconnect only after negotiated idempotent resume and validated identities; otherwise it MUST persist `orphaned`, retain evidence, and provide guided status and next actions requiring human choice. Supervised enablement MUST NOT alter headless behavior or enable terminal/PTY.

#### Scenario: Unsafe and safe recovery
- GIVEN orphaned attempts with and without provable resume
- WHEN guided recovery runs
- THEN only the proven session may reconnect and the other remains evidenced and orphaned
