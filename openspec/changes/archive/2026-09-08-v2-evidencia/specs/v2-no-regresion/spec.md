# Delta for v2-no-regresion

## MODIFIED Requirements

### Requirement: Feedback reconstruction [v2-no-regresion/F-05] — P0 [INT]

`step run --feedback` MUST deliver delimited feedback plus bounded prior response and record a distinct reconstruction attempt preserving history. Prior evidence MUST come from DB `payload`, with fallback to a legacy `payload_ref` file only when payload is absent.

(Previously: the requirement did not define database-first evidence retrieval or legacy fallback.)

#### Scenario: Feedback
- GIVEN a prior attempt with inline payload or only a legacy evidence reference
- WHEN feedback reconstructs
- THEN bounded context and delimited feedback are recorded with preserved history

### Requirement: Generational reopen and skip [v2-no-regresion/F-07] — P0 [INT]

`reopen --cascade` MUST invalidate current generations and reset descendants while retaining files/history; an invalid producer's latest current generation MUST NOT satisfy `requires` or completion. Older invalid generations MUST NOT block once the producer has a newer valid current generation. Skip MUST require a reason, pending state, and no attempts. `step_transition_events` MUST audit both operations.

(Previously: the requirement covered stale blocking but not successful downstream recovery after producer rerun.)

#### Scenario: Cascade
- GIVEN three completed steps
- WHEN the first reopens cascading before rerunning
- THEN generations invalidate, descendants pend, files remain, and downstream execution is blocked

#### Scenario: Recovery
- GIVEN a reopened producer with invalid history
- WHEN it reruns successfully before its downstream step reruns
- THEN only its valid latest generation satisfies `requires` and downstream execution succeeds

#### Scenario: Skip
- GIVEN varied step histories
- WHEN skip has a reason
- THEN only virgin pending work skips, audited

### Requirement: Evidence budgets and redaction [v2-no-regresion/F-12] — P0 [INT]

Visible evidence MUST be redacted and ≤16 KiB after its command or agent header is composed with output; headers and output share that limit. Snapshots MUST be ≤1 MiB, fallback context ≤2 MiB, and `DiagnosticRaw` ≤1 MiB or prefix+suffix+size+SHA-256. Bearer, Basic, and token credentials MUST be redacted from every projection.

(Previously: the visible budget did not explicitly include execution headers or inline payload persistence.)

#### Scenario: Budgets
- GIVEN oversized headers, output, fallback context, snapshots, diagnostics, and credentials
- WHEN each projection is processed and visible evidence is persisted inline
- THEN every limit and redaction rule applies after complete visible composition
