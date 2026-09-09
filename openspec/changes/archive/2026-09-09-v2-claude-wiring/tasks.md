# Tasks: v2 Claude Wiring

## Review Workload Forecast

Estimated lines: ~1,900 (prod/test/fix 500/1150/230)
Delivery: single-pr, pre-approved size:exception (200000-line budget); 5 commits to main.
Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|
| U1 | Parser | claude pkg (`-run` stream test) | N/A pure parser | parser.go+test |
| U2 | Session | claude pkg | t.TempDir fixture | adapter.go+test |
| U3 | Factory | adapter pkgs | t.TempDir fixture | factory.go+test |
| U4 | Redaction+contract | execution+contract pkgs | RunSuite via fixture | evidence.go, claude_suite_test.go, fixtures .2 |
| U5 | Gate | full suite | race+vet+script | engine_test.go |

Strict TDD: RED test first per task (`go test ./...`). Hermetic: pinned JSON in `testdata/fixtures/synthetic/v2.0.0-synthetic.2/` (synthetic.1 convention); executables in `t.TempDir()`; no real claude in `-short`.

Binding: N1 `CLAUDE_MODEL`/`CLAUDE_FALLBACK_MODEL` from config `Env` overlay only (not process env) → `--model`/`--fallback-model`. N2 `dontAsk` (deny-fallback vs auto-allow) and nested `stream_event.event.delta` = pinned envelope expectations, untested vs real binary; opt-in fails loudly on drift. N3 store/engine assertions in `internal/execution` (contract stays adapter-focused). N5 `session_id` first-wins.

## U1: Parser (internal/adapter/claude/parser.go)

- [x] 1.1 RED `parser_test.go` `TestClaudeParseStreamJSON`: init/assistant/stream_event deltas/result success+error/malformed/oversized>10MiB/`UseNumber`.
- [x] 1.2 GREEN `parser.go` `ParseStreamJSON`: newline frames bounded by `jsonrpc.MaxMessageSize`; fields type/subtype/session_id/message.content[].{type,text}/event.type/event.delta.{type,text}/result/is_error/error.
- [x] 1.3 RED first-wins `session_id`: two inits → first retained (N5).

## U2: Real session (internal/adapter/claude/adapter.go)

- [x] 2.1 RED `adapter_test.go`: `TestClaudeRealSessionLifecycle`, `TestClaudePromptViaStdin`, `TestClaudePermissionDontAsk`, `TestClaudeZeroTextFailsCleanly`.
- [x] 2.2 GREEN rewrite `adapter.go`: `NewAdapter(binary, env, timeout)`; `Prompt` runs `claude -p --output-format stream-json --include-partial-messages --permission-mode dontAsk` (+N1), `exec.CommandContext`, `Dir=WorkspaceRoot`, env overlay, timeout ≤300s, stdin-only; `doneOnce`/`cancelOnce`/`settleDone`, mutex `cmd`/`nativeID`, idempotent `Cancel`; precedence `HARO_TEST_CLAUDE_BINARY`→configured→`claude`; replaces simulation.
- [x] 2.3 GREEN transport: `SessionTransport{NativeSessionID, ProtocolVersion, Extra:{"transport":"stream-json"}}` via `TransportProvider`.

- [x] 2.4 RED probe classification: ELF/shebang executables available; reject `requirements.txt`/`CMakeLists.txt`/MD/MDX sans shebang; accept `README.sh`.

## U3: Factory (internal/adapter/factory/factory.go)

- [x] 3.1 RED `factory_test.go`: key exactly `claude`→`claude.NewAdapter`; other enabled keys stay `opencode.NewAdapter`; disabled unregistered; ACP never registered.
- [x] 3.2 GREEN `factory.go`: build both providers; per-harness binary precedence.
- [x] 3.3 RED model flags: claude `Env` models → flags; process env ignored (N1).
- [x] 3.4 No-regression: `TestProbeCalledOncePerHarness`, `TestManager_ProbeCachedOnce`, `TestReopenDoesNotReprobeHarness`, `TestAgentManagerCLIInjection`, `TestOpenCodeRealSessionLifecycle` green.

## U4: Redaction, contract, fixtures

- [x] 4.1 RED `evidence_test.go`: `Redact` covers `sk-ant-`/`sk-`; Bearer unchanged; `VisibleEvidence` ≤16 KiB once.
- [x] 4.2 GREEN `evidence.go`: extend single redaction point with `sk-ant-`/`sk-`.
- [x] 4.3 RED `TestClaudeEvidenceBoundedAndRedacted`: oversized Bearer+`sk-ant-` → redacted once, ≤16 KiB, `adapter_name="claude"`, protocol version, transport Extra.
- [x] 4.4 Create `testdata/fixtures/synthetic/v2.0.0-synthetic.2/` (`README.md`+`claude-fixture.jsonl` pinned 2.1.245; N2/N4).
- [x] 4.5 RED+GREEN `TestClaudeContractSuiteFixture` (`claude_suite_test.go`): `RunSuite` + factory/store/settlement across subprocess→protocol→JSON→store→settlement in `-short`.

## U5: Integration + gate

- [x] 5.1 RED `engine_test.go`: claude attempt end-to-end, evidence+transport persisted; empty intersection fails, no synthetic success.
- [x] 5.2 Opt-in real-binary variant (non-short + `HARO_TEST_CLAUDE_BINARY`); cycle end only.
- [x] 5.3 Confirm delta F-01 sentence (OpenCode AND Claude registered; ACP unregistered); main-spec update at archive.
- [x] 5.4 Gate: `go test ./... -race`, `go build ./...`, `go vet ./...`, `scripts/verify-adapter-boundary.sh`; F-05 unclaimed; no `deltas-acceptance.md`/`docs/v2` changes.
