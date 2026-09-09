# Delta for v2-adapter

## Purpose

Register Claude Code as a real CLI-direct harness.

## MODIFIED Requirements

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

## Non-Requirements

ACP JSON-RPC evolution, PTY/terminal, broker/IPC (D01/D02), and concurrency are out of scope. This change MUST NOT claim the `v2-adapter/F-05` checkbox, add an acceptance ID, modify `deltas-acceptance.md`, change the 92-criteria/11-spec invariant, or make normative `docs/v2` changes.
