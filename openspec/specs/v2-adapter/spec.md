# v2-adapter Specification

## Purpose

Define adapter negotiation, fallback, verification, and transport boundaries.

## Constraints

Implementation MUST be pure Go/no cgo with dependency-free stdlib JSON, confine provider literals to adapters, reject payloads above 10 MiB, let adapters choose transport, and retain broker-rehomable stable interfaces. Evidence limits MUST hold; accumulation MUST be sanitized and at most 2 MiB. PTY is deferred; `Terminal` MUST be false. Versioned synthetic neutral fixtures MUST document unavailable OpenCode 1.17.18 originals without byte-identity claims. Claude tests MUST use `HARO_TEST_CLAUDE_BINARY` and skip under CI `testing.Short()`. This change solely owns the additive, idempotent `attempt_transport` migration; only that table overlaps `v2-store`. Broker/UDS, CLI↔Broker RPC, `leases`, `interactions`, `path_claims`, composition, claims, reporting, and distribution remain out of scope.

## Requirements

### Requirement: Prior initialization [v2-adapter/F-01]
Each subprocess MUST receive exactly one bilateral protocol/capability `initialize` before any `session/*` call.

#### Scenario: Call order
- GIVEN a recorded subprocess
- WHEN a session runs
- THEN `initialize` is first and occurs once

### Requirement: Optional-method gate [v2-adapter/F-02]
The core MUST NOT invoke unannounced optional methods and MUST return `unsupported_capability` when their use is requested.

#### Scenario: Missing capabilities
- GIVEN unannounced `Terminal` and `LoadSession`
- WHEN either feature is requested
- THEN no method is emitted and `unsupported_capability` is returned

### Requirement: Permission negotiation [v2-adapter/F-03]
A harness MUST request permission only when core permission support was negotiated; otherwise it MUST apply its default policy or fail closed without bypass.

#### Scenario: Fail-closed permission
- GIVEN permission was not negotiated
- WHEN the harness requires permission
- THEN no request is sent and default policy resolves it or execution fails closed

### Requirement: Adapter boundary [v2-adapter/F-04]
The OpenCode JSONL parser and provider details MUST remain in its adapter; `scripts/verify-adapter-boundary.sh` MUST enforce this by grep/import graph.

#### Scenario: Boundary gate
- GIVEN sources and an oversized message
- WHEN boundary and parser checks run
- THEN provider leakage fails the gate and the oversized message is rejected

### Requirement: Claude contract [v2-adapter/F-05]
The Claude Code adapter MUST pass the public-boundary suite using the opted-in binary or justified fixtures.

#### Scenario: Public cycle
- GIVEN a binary or versioned fixtures
- WHEN `step run` crosses subprocess, protocol, JSON, store, and settlement
- THEN the contract suite passes

### Requirement: Ordered fallback [v2-adapter/F-06]
Candidates MUST run in declared order; clean failure MUST advance without semantic classification, carrying sanitized accumulated context no larger than 2 MiB.

#### Scenario: E2E fallback
- GIVEN two candidates whose first fails cleanly
- WHEN `step run` executes
- THEN the second receives bounded sanitized context and runs

#### Scenario: Candidates exhausted
- GIVEN all candidates fail cleanly
- WHEN fallback exhausts the list
- THEN the step fails with sanitized evidence from every candidate

### Requirement: Capability drift table [v2-adapter/U-01]
Optional calls MUST be table-gated; capabilities MUST be additive; major versions MUST change only for mandatory-method incompatibility.

#### Scenario: Capability combinations
- GIVEN empty, partial, complete, and future-additive rows
- WHEN negotiation round-trips each row
- THEN options and additions survive without a major bump

### Requirement: Runnable contract suite [v2-adapter/U-02]
A first-class, versioned, runnable boundary contract suite MUST exist.

#### Scenario: Repository execution
- GIVEN an adapter factory
- WHEN the suite runs
- THEN lifecycle, gating, permission, and boundary assertions run

### Requirement: Generic ACP adapter [v2-adapter/U-03]
The generic ACP adapter MUST translate initialize, new, prompt, update, cancel, and request_permission bijectively with protocol fixtures.

#### Scenario: ACP round trip
- GIVEN a complete ACP fixture cycle
- WHEN capabilities and messages translate out and back
- THEN equivalent capabilities and messages return

### Requirement: Transport-neutral attempts [v2-adapter/U-04]
`attempts` MUST contain no transport fields; native session ID, protocol version, and extra data MUST live in per-adapter `attempt_transport`.

#### Scenario: Optional transport row
- GIVEN the idempotent additive migration runs repeatedly
- WHEN attempts are inserted with and without transport details
- THEN both succeed and native fields remain absent from `attempts`
