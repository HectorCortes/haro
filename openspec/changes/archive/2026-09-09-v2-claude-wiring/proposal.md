# Proposal: v2 Claude Wiring

## Intent

Make Claude Code a registered CLI-direct harness, matching OpenCode. Agent attempts must persist real evidence with native `session_id`, `adapter_name="claude"`, and bounded, redacted payloads instead of simulated success.

## Scope

### In Scope
- Add `ParseStreamJSON` (10 MiB limit) and a real lifecycle: executable probe, initialize, prompt/consume, idempotent cancel/settlement, and transport identity.
- Dual-register Claude with precedence `HARO_TEST_CLAUDE_BINARY` → configured binary → `claude`.
- Add pinned fixtures, parser/adapter/factory/engine tests, short-mode contract coverage, and opt-in E2E at cycle end.
- Default `--permission-mode dontAsk`; support configured model/fallback model; extend single-point redaction for `sk-ant-` tokens if required.
- Modify only the `v2-adapter/F-01` registration sentence.

### Out of Scope
- ACP JSON-RPC evolution, PTY/terminal, broker/IPC (D01/D02), and concurrency.
- New acceptance IDs or `deltas-acceptance.md` changes; preserve 92 criteria/11 specs and do not claim F-05.
- Normative `docs/v2/` changes or restored simulation.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `v2-adapter`: register Claude once its session is real while ACP remains unregistered.

## Approach

Mirror `internal/adapter/opencode`: run `claude -p --output-format stream-json --include-partial-messages` with optional model flags, no shell, `exec.CommandContext`, `Dir=WorkspaceRoot`, timeout ≤300s, and stdin-only prompts (`MAX_ARG_STRLEN` ~128 KiB versus 2 MiB fallback). Implement `internal/adapter/claude/{parser.go,adapter.go}` with `doneOnce/settleDone`, then wire `internal/adapter/factory/factory.go`. The engine stays unchanged: intersection, fallback, evidence, and transport are generic.

## Compliance and Impact

This preserves `docs/v2/haro-constitucion.md` fail-closed behavior, provider isolation, methodological neutrality, transport-neutrality, and contract testing (I.3/I.5, II.3, IV.3, V, XI), and implements `docs/v2/haro-especificacion-tecnica.md` §4/§7. CLI print mode is F-05's real-session prerequisite; ACP remains later evolution.

| Area | Impact |
|---|---|
| `internal/adapter/claude/{adapter,parser}*.go` | Protocol/session; `TestClaudeRealSessionLifecycle` |
| `internal/adapter/factory/*` | Registration and binary/env precedence tests |
| `internal/adapter/contract/*`, `internal/execution/*_test.go` | Short suite; persisted evidence/transport |
| `testdata/fixtures/...`, change delta spec | Pinned envelope and F-01 modification |

## Risks

| Risk | Mitigation |
|---|---|
| Envelope drift | Version fixture; opt-in real E2E; fail loudly |
| Prompt moved to argv | Assert stdin with a 1 MiB prompt |
| Permission hang | Default `dontAsk`; bounded timeout test |
| Secret leakage | Extend redaction; assert ≤16 KiB persisted payload |
| CLI/auth/cost drift | Surface stderr; opt-in E2E gate |

## Rollback Plan

Disable Claude in configuration to fail closed, then revert wiring, fixtures, tests, and F-01 delta as one unit. Never restore simulation.

## Delivery and Success

- Delivery strategy: `single-pr`, maintainer-pre-approved `size:exception` (200000-line review budget), direct push to `main`; no PR.
- Strict TDD: `go test ./...`; real E2E requires `HARO_TEST_CLAUDE_BINARY` and runs only at cycle end.
- [ ] Hermetic lifecycle and public-boundary contract tests pass in `go test ./... -short`.
- [ ] Attempts persist Claude identity and bounded redacted evidence; engine and normative docs remain unchanged.
