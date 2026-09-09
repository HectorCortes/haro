# Delta for v2-adapter

## Purpose

Require configured CLI-direct adapter sessions without simulation.

## MODIFIED Requirements

### Requirement: Prior initialization [v2-adapter/F-01]

Each CLI invocation MUST construct one manager from project configuration, probe and initialize each usable harness once before `session/*`, and inject it into `run`, `step run`, `step reopen`, and `step skip` engines. Sessions MUST use `exec.CommandContext`, fixed arguments, no shell, configured environment, and a timeout defaulting to at most 300 seconds; cancellation MUST be idempotent. Only OpenCode SHALL be registered; Claude and ACP MUST remain unregistered until their sessions are real.

(Previously: only per-subprocess initialization order was specified.)

#### Scenario: CLI lifecycle (`TestAgentManagerCLIInjection`)
- GIVEN enabled configuration and any agent-capable run site
- WHEN the CLI creates its execution engine
- THEN one manager is injected and each usable harness is initialized once before its first session

#### Scenario: Real OpenCode session (`TestOpenCodeRealSessionLifecycle`)
- GIVEN an OpenCode fixture and simulated Claude and ACP sessions
- WHEN a session runs, times out, or is cancelled repeatedly
- THEN the subprocess is bounded and only OpenCode is registered

### Requirement: Adapter boundary [v2-adapter/F-04]

`.haro/config.yaml` MUST accept optional `harnesses: map[string]{binary, env, timeout_seconds, enabled}`. Parsing MUST use `yaml.v3` `KnownFields(true)` and reject unknown record fields. Harness names are data keys; provider literals MUST remain confined to adapters. `scripts/verify-adapter-boundary.sh` MUST enforce confinement and oversized-message rejection.

(Previously: strict generic harness configuration was unspecified.)

#### Scenario: Boundary gate
- GIVEN sources and an oversized message
- WHEN boundary and parser checks run
- THEN provider leakage fails the gate and the oversized message is rejected

#### Scenario: Strict optional configuration (`TestHarnessConfigKnownFields`)
- GIVEN absent harnesses, arbitrary harness keys, or an unknown record field
- WHEN project configuration is loaded
- THEN absence and arbitrary keys pass, while the unknown field fails

### Requirement: Ordered fallback [v2-adapter/F-06]

The engine MUST intersect a step's ordered harness list with configured, enabled, successfully probed harnesses without reordering. Unknown, disabled, unavailable, or cleanly failing candidates MUST fall through, carrying sanitized context of at most 2 MiB. Exhaustion MUST fail closed with clear evidence. Production MUST NOT synthesize success, identity, or output; tests MUST inject fakes.

(Previously: fallback did not define configuration intersection or prohibit simulation.)

#### Scenario: E2E fallback (`TestAgentHarnessIntersectionFallback`)
- GIVEN unavailable candidates before a usable adapter
- WHEN `step run` executes
- THEN they fall through and the usable adapter receives bounded sanitized context

#### Scenario: Candidates exhausted
- GIVEN every intersected candidate fails cleanly
- WHEN fallback exhausts the list
- THEN the step fails with sanitized evidence from every attempted candidate

#### Scenario: No configured harness (`TestAgentStepFailsWithoutConfiguredHarness`)
- GIVEN no configuration or no usable candidate
- WHEN an agent step runs without an injected fake
- THEN the step fails closed with clear evidence and no simulated result

### Requirement: Transport-neutral attempts [v2-adapter/U-04]

`attempts` MUST contain no transport fields. Real attempts MUST persist actual `native_session_id`, `adapter_name`, protocol version, and metadata in `attempt_transport`. `SessionBundle.Requires` MUST contain resolved required-artifact paths. Harness output MUST be redacted once, bounded to 16 KiB, and stored in `attempt_events.payload`; legacy `payload_ref` MUST remain unchanged.

(Previously: real identity, resolved requirements, and inline evidence were unspecified.)

#### Scenario: Optional transport row
- GIVEN the idempotent additive migration runs repeatedly
- WHEN attempts are inserted with and without transport details
- THEN both succeed and native fields remain absent from `attempts`

#### Scenario: Real identity and evidence (`TestAgentStepPersistsRealEvidenceAndTransport`)
- GIVEN required artifacts, protocol metadata, credentials, and oversized output
- WHEN the attempt settles
- THEN resolved requirements reach the session and actual transport identity plus redacted payload of at most 16 KiB are persisted

## Non-Requirements

`v2-broker` (D01), `v2-ipc` (D02), hosting, concurrency, PTY/terminal, claiming `v2-no-regresion/F-04`, and new acceptance IDs are out of scope. The 92-criteria/11-spec invariant MUST remain unchanged.
