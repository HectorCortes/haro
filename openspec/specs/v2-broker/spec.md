# v2-broker Specification

## Purpose

Define the persistent, isolated, and fenced per-project broker lifecycle.

## Requirements

### Requirement: Single broker per project [v2-broker/F-01] — P0 [E2E]

Each canonical project path MUST identify one broker reached through a Unix-domain socket or a Windows named pipe.

#### Scenario: Shared project broker
- GIVEN two CLI invocations from the same repository
- WHEN each calls `execution.start`
- THEN both use the same live broker endpoint and receive responses

### Requirement: Lazy startup [v2-broker/F-02] — P0 [E2E]

When no broker responds, the CLI MUST start one, retry the operation, and leave the broker alive after the starting CLI exits.

#### Scenario: Relaunch after death
- GIVEN the project broker is absent or dead
- WHEN the first CLI operation runs and exits
- THEN it succeeds after relaunch and the broker remains responsive

### Requirement: Concurrent sessions [v2-broker/F-03] — P0 [E2E]

One broker MUST progress multiple active executions concurrently without cross-session state interference.

#### Scenario: Parallel executions
- GIVEN two workflows execute simultaneously in one project
- WHEN both progress to completion
- THEN each persists only its own attempts, events, and state

### Requirement: Independent project brokers [v2-broker/F-04] — P1 [E2E]

Different canonical projects MUST use independent endpoints, brokers, and state without cross-project coordination.

#### Scenario: Isolated projects
- GIVEN live brokers in two repositories
- WHEN one broker stops during simultaneous operations
- THEN its endpoint and state differ and the other operation continues

### Requirement: No broker duplication [v2-broker/F-05] — P1 [E2E]

CLI startup MUST reuse a responsive broker and MUST prevent concurrent launchers from creating duplicates.

#### Scenario: Launch herd
- GIVEN no broker and N concurrent CLI invocations
- WHEN all attempt lazy startup
- THEN exactly one broker serves the project

### Requirement: Broker death and fencing [v2-broker/F-06] — P0 [E2E]

The broker MUST acquire and check the current lease `fencing_token` before every engine write. After broker death or lease expiry, stale-token writes MUST fail even if their originating process survives, and clients MUST receive recovery guidance.

#### Scenario: Stale writer after death
- GIVEN an active attempt whose broker is killed
- WHEN its lease expires and the old token attempts a write
- THEN the write is rejected and persisted state remains unchanged

### Requirement: Clean shutdown [v2-broker/F-07] — P1 [E2E]

On a termination signal, the broker MUST release owned leases and its endpoint and MUST NOT leave a zombie socket.

#### Scenario: Immediate restart
- GIVEN a running broker with an owned lease
- WHEN it receives TERM and a new operation starts
- THEN it exits, cleans resources, and the replacement starts immediately

### Requirement: Socket path derivation [v2-broker/U-01] — P1 [UNIT]

Endpoint identity MUST derive from a stable hash of the symlink-resolved, cleaned project path so worktrees resolve coherently. Distinct projects MUST differ, and a Linux UDS path MUST be at most 108 bytes including NUL.

#### Scenario: Canonical path table
- GIVEN equivalent symlink/trailing-slash paths, worktrees, distinct roots, and long paths
- WHEN endpoint paths are derived
- THEN equivalents share a safe endpoint and distinct roots do not

### Requirement: Robust framing [v2-broker/U-02] — P1 [UNIT]

JSON-RPC 2.0 framing MUST reject malformed or oversized frames without panicking or discarding a healthy connection and MUST support concurrent connections.

#### Scenario: Recoverable framing failures
- GIVEN malformed, oversized, valid-following, and N concurrent frames
- WHEN the broker processes them
- THEN invalid frames fail and valid frames still receive matched responses
