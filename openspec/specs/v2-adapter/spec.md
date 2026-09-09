# v2-adapter Specification

## Purpose

Define adapter negotiation, fallback, verification, and transport boundaries.

## Constraints

Implementation MUST be pure Go/no cgo with dependency-free stdlib JSON, confine provider literals to adapters, reject payloads above 10 MiB, let adapters choose transport, and retain broker-rehomable stable interfaces. Evidence limits MUST hold; accumulation MUST be sanitized and at most 2 MiB. PTY is deferred; `Terminal` MUST be false. Versioned synthetic neutral fixtures MUST document unavailable OpenCode 1.17.18 originals without byte-identity claims. Claude tests MUST use `HARO_TEST_CLAUDE_BINARY` and skip under CI `testing.Short()`. This change solely owns the additive, idempotent `attempt_transport` migration; only that table overlaps `v2-store`. Broker/UDS, CLI↔Broker RPC, `leases`, `interactions`, `path_claims`, composition, claims, reporting, and distribution remain out of scope.

## Requirements

### Requirement: Prior initialization [v2-adapter/F-01]

Each CLI invocation MUST construct one manager from project configuration, probe and initialize each usable harness once before `session/*`, and inject it into `run`, `step run`, `step reopen`, and `step skip` engines. Sessions MUST use `exec.CommandContext`, fixed arguments, no shell, configured environment, and a timeout defaulting to at most 300 seconds; cancellation MUST be idempotent. OpenCode and Claude SHALL be registered now that their sessions are real; ACP MUST remain unregistered until it implements a real session contract.

(Previously: only OpenCode was registered, while Claude and ACP remained unregistered.)

#### Scenario: CLI lifecycle (`TestAgentManagerCLIInjection`)
- GIVEN enabled configuration and any agent-capable run site
- WHEN the CLI creates its execution engine
- THEN one manager is injected and each usable harness is initialized once before its first session

#### Scenario: Real OpenCode and Claude sessions (`TestOpenCodeRealSessionLifecycle`, `TestClaudeRealSessionLifecycle`)
- GIVEN OpenCode and Claude fixtures and a simulated ACP session
- WHEN a session runs, times out, or is cancelled repeatedly
- THEN each subprocess is bounded, OpenCode and Claude are registered, and ACP is not registered


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

`.haro/config.yaml` MUST accept optional `harnesses: map[string]{binary, env, timeout_seconds, enabled}`. Parsing MUST use `yaml.v3` `KnownFields(true)` and reject unknown record fields. Harness names are data keys; provider literals MUST remain confined to adapters. `scripts/verify-adapter-boundary.sh` MUST enforce confinement and oversized-message rejection.


#### Scenario: Boundary gate
- GIVEN sources and an oversized message
- WHEN boundary and parser checks run
- THEN provider leakage fails the gate and the oversized message is rejected

#### Scenario: Strict optional configuration (`TestHarnessConfigKnownFields`)
- GIVEN absent harnesses, arbitrary harness keys, or an unknown record field
- WHEN project configuration is loaded
- THEN absence and arbitrary keys pass, while the unknown field fails

### Requirement: Claude contract [v2-adapter/F-05]

The Claude Code adapter MUST run a real CLI-direct `claude -p --output-format stream-json --include-partial-messages` session through `exec.CommandContext` without a shell, with `Dir=WorkspaceRoot`, inherited environment plus configured overlay, and binary precedence `HARO_TEST_CLAUDE_BINARY` → configured binary → `claude`. The prompt MUST use stdin and MUST NOT appear in positional argv. Sessions MUST default to `--permission-mode dontAsk`, time out within 300 seconds, and cancel and settle idempotently. The adapter MUST parse stream-JSON frames no larger than 10 MiB, capture native `session_id`, and map `system/init`, assistant text, `stream_event` deltas, and `result` to `output_delta`, `completed`, or `failed`; zero-text EOF MUST fail cleanly. Evidence MUST pass one redaction point covering Bearer and `sk-ant-` secrets, persist at most 16 KiB inline, and expose `adapter_name="claude"`, protocol version, and extra metadata. The adapter SHALL pass the public-boundary suite with a justified fixture and MAY also use an opted-in real binary.

(Previously: Claude only had to pass the public-boundary suite using an opted-in binary or justified fixtures.)

#### Scenario: Real session lifecycle (`TestClaudeRealSessionLifecycle`)
- GIVEN an initialized Claude adapter and executable fixture
- WHEN prompt, timeout, repeated cancellation, and settlement paths run
- THEN one bounded real subprocess produces exactly one terminal outcome

#### Scenario: Envelope mapping (`TestClaudeParseStreamJSON`)
- GIVEN init, assistant, delta, result, malformed, and oversized envelopes
- WHEN the stream is parsed
- THEN text and terminal events map correctly, native session identity is captured, and invalid frames fail

#### Scenario: Prompt via stdin (`TestClaudePromptViaStdin`)
- GIVEN an oversized positional-argument prompt
- WHEN Claude is invoked
- THEN the complete prompt is received through stdin and is absent from positional argv

#### Scenario: Fail-closed permission (`TestClaudePermissionDontAsk`)
- GIVEN a headless session requiring permission
- WHEN the session starts without an explicit permission mode
- THEN `dontAsk` is applied and execution does not wait for a TTY

#### Scenario: Zero-text clean failure (`TestClaudeZeroTextFailsCleanly`)
- GIVEN a stream that ends without assistant text or result text
- WHEN EOF is reached
- THEN the session fails cleanly and does not report completion

#### Scenario: Bounded redacted evidence (`TestClaudeEvidenceBoundedAndRedacted`)
- GIVEN oversized output containing Bearer and `sk-ant-` credentials
- WHEN attempt evidence and transport are persisted
- THEN secrets are redacted once, inline payload is at most 16 KiB, and Claude transport identity is retained

#### Scenario: Public boundary fixture (`TestClaudeContractSuiteFixture`)
- GIVEN a justified executable fixture
- WHEN the public-boundary suite crosses subprocess, protocol, JSON, store, and settlement
- THEN the contract passes without an installed or authenticated real binary


### Requirement: Ordered fallback [v2-adapter/F-06]

The engine MUST intersect a step's ordered harness list with configured, enabled, successfully probed harnesses without reordering. Unknown, disabled, unavailable, or cleanly failing candidates MUST fall through, carrying sanitized context of at most 2 MiB. Exhaustion MUST fail closed with clear evidence. Production MUST NOT synthesize success, identity, or output; tests MUST inject fakes.


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

`attempts` MUST contain no transport fields. Real attempts MUST persist actual `native_session_id`, `adapter_name`, protocol version, and metadata in `attempt_transport`. `SessionBundle.Requires` MUST contain resolved required-artifact paths. Harness output MUST be redacted once, bounded to 16 KiB, and stored in `attempt_events.payload`; legacy `payload_ref` MUST remain unchanged.


#### Scenario: Optional transport row
- GIVEN the idempotent additive migration runs repeatedly
- WHEN attempts are inserted with and without transport details
- THEN both succeed and native fields remain absent from `attempts`

#### Scenario: Real identity and evidence (`TestAgentStepPersistsRealEvidenceAndTransport`)
- GIVEN required artifacts, protocol metadata, credentials, and oversized output
- WHEN the attempt settles
- THEN resolved requirements reach the session and actual transport identity plus redacted payload of at most 16 KiB are persisted
