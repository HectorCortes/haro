# Delta for v2-ipc

## MODIFIED Requirements

### Requirement: Event payloads without raw output [v2-ipc/U-03] — P1 [UNIT]

`attempt_events.payload` MAY contain a sanitized, bounded evidence delta and MUST NEVER contain raw or full harness output. `attempt_events.payload_ref` MUST remain an external or legacy reference and MUST NOT be repurposed for inline evidence. Technical Specification §2 MUST declare nullable `payload TEXT`, and §2.1 MUST preserve this narrow distinction under Constitution VIII.2.

(Previously: only `payload_ref` was defined, and all sanitized evidence was required outside the table.)

#### Scenario: Inline doctrine
- GIVEN an event produced by a new attempt
- WHEN its persisted fields and normative §2/§2.1 text are inspected
- THEN bounded sanitized deltas MAY appear in `payload` but raw or full output never does
- AND `payload_ref` remains null or an external/legacy reference

#### Scenario: Legacy reference doctrine
- GIVEN an event created before inline payload support
- WHEN its evidence is read
- THEN its external `payload_ref` remains valid without changing its historical meaning

## Non-Requirements

JSON-RPC or broker implementation is not introduced by this doctrine amendment.
