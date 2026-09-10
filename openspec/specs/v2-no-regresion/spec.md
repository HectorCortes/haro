# v2-no-regresion Specification

## Purpose

Go preservation.

## Constraints

Core MUST use pure Go/no cgo, stdlib `flag`, no provider literals, Store interfaces, and evidence limits. `deltas-acceptance.md` and `docs/` MUST remain unchanged.

## Requirements

### Requirement: Idempotent initialization [v2-no-regresion/F-01] — P0 [E2E]

`haro init` MUST create `.haro/config.yaml`, `workflows/`, `skills/`, `artifacts/`, and `docs/`; reruns MUST preserve bytes, report initialization, and exit 0.

#### Scenario: Init
- GIVEN empty project
- WHEN init runs twice
- THEN bytes remain unchanged

### Requirement: Workflow discovery and validation [v2-no-regresion/F-02] — P0 [E2E]

`workflows list/describe` MUST return structured JSON, flag malformed/invalid YAML, and report concrete errors: duplicates, missing dependencies, cycles, or no entry point. Workflow validation MUST continue to accept `supervised` as a valid declared mode. At runtime, `step run` MUST reject an agent step declared with `mode: supervised` before harness intersection, fallback, or adapter invocation; the step MUST end `failed` with the distinct reason `supervised mode not supported`, no attempt MUST be created, no adapter MUST be invoked, no fallback MUST run, and execution MUST NOT silently downgrade to headless. Agent steps declared with `mode: terminal` MUST retain their existing terminal rejection behavior.

#### Scenario: Discovery
- GIVEN mixed workflow YAML
- WHEN both run
- THEN details and errors appear

#### Scenario: Unsupported declared execution modes
- GIVEN a valid workflow declaring an agent step with `mode: supervised` and the existing `mode: terminal` rejection case
- WHEN `step run` executes each step
- THEN the supervised step ends `failed` with reason `supervised mode not supported`
- AND no attempt is created, no adapter is invoked, no fallback runs, and no headless downgrade occurs
- AND the terminal step retains its existing terminal rejection behavior

### Requirement: Complete command cycle [v2-no-regresion/F-03] — P0 [E2E]

`run`, `steps next`, `step run`, and `status` MUST persist state. Only `depends_on` orders; `requires` precedes execution and `produces` follows exit.

#### Scenario: Outcomes
- GIVEN four command cases
- WHEN commands run
- THEN exit 0 with outputs completes; nonzero fails with stdout/stderr
- AND exit 0 with missing outputs fails listing them
- AND unmet dependencies fail explicitly, unexecuted

### Requirement: Feedback reconstruction [v2-no-regresion/F-05] — P0 [INT]

`step run --feedback` MUST deliver delimited feedback plus bounded prior response and record a distinct reconstruction attempt preserving history. Prior evidence MUST come from DB `payload`, with fallback to a legacy `payload_ref` file only when payload is absent.

#### Scenario: Feedback
- GIVEN a prior attempt with inline payload or only a legacy evidence reference
- WHEN feedback reconstructs
- THEN bounded context and delimited feedback are recorded with preserved history

### Requirement: Generational reopen and skip [v2-no-regresion/F-07] — P0 [INT]

`reopen --cascade` MUST invalidate current generations and reset descendants while retaining files/history; an invalid producer's latest current generation MUST NOT satisfy `requires` or completion. Older invalid generations MUST NOT block once the producer has a newer valid current generation. Skip MUST require a reason, pending state, and no attempts. `step_transition_events` MUST audit both operations.

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

#### Scenario: Budgets
- GIVEN oversized headers, output, fallback context, snapshots, diagnostics, and credentials
- WHEN each projection is processed and visible evidence is persisted inline
- THEN every limit and redaction rule applies after complete visible composition

### Requirement: Path containment [v2-no-regresion/F-13] — P0 [INT]

Workflow, instruction, skill, artifact, and bundle paths MUST reject absolutes, `..`, and symlink escapes; internal symlinks MAY stay inside their root.

#### Scenario: Containment
- GIVEN malicious and internal paths
- WHEN containment runs
- THEN only contained targets pass

### Requirement: CLI output contract [v2-no-regresion/F-14] — P0 [E2E]

With `--json`, commands MUST return structured JSON; errors MUST return `{error, code}` and exit 1. EPIPE MUST exit 0. Extra arguments MUST respect `--`.

#### Scenario: CLI
- GIVEN CLI boundary cases
- WHEN commands run
- THEN output, exits, and parsing conform

### Requirement: Go quality gate [v2-no-regresion/U-01] — P0 [UNIT]

The change MUST pass `go test ./... -race`, `go vet ./...`, `golangci-lint run`, and `govulncheck ./...`.

#### Scenario: Gates
- GIVEN completed changes
- WHEN gates run
- THEN each exits 0

### Requirement: State-machine transitions [v2-no-regresion/U-03] — P0 [UNIT]

The Store machine MUST idempotently allow `pending→running→completed|failed`, `(completed|failed)→pending`, and `pending→skipped`, rejecting others without duplicate effects.

#### Scenario: Transitions
- GIVEN allowed, repeated, forbidden transitions
- WHEN each runs twice
- THEN allowed effects occur once; forbidden states remain

## Non-Requirements and Ownership

- U-02 (neutral OpenCode fixtures as-is): deferred to `v2-adapter` — no test in this change references them (their only consumers are permission-detection/supervised/agent-diagnostics tests, all adapter-owned); the original fixture source is not available in this repository, so byte-identical import is deferred with its owning surface. Recorded decision (orchestrator, 2026-08-28).
- F-04/F-11: `v2-adapter`; agent F-05/F-12: `v2-adapter`/`v2-broker`; F-06: `v2-broker`/`v2-store`; F-08: `v2-adapter`/`v2-store`; F-09: `v2-broker`/`v2-ipc`/`v2-adapter`; F-10: deferred PTY.
- Broker JSON-RPC, full DDL, claims, composition, reporting, and distribution belong to `v2-broker`/`v2-ipc`, `v2-store`, `v2-path-claims`, `v2-composicion`, `v2-reporte`, and `v2-distribucion`.
