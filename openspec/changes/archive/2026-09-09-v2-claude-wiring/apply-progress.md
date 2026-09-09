# Apply Progress: v2 Claude Wiring

Date: 2026-09-09 · Mode: Strict TDD (`go test ./...`) · Delivery: `single-pr` with maintainer pre-approved `size:exception` (200000-line budget; ~1,900-line forecast) · Direct commits to `main`, not pushed (orchestrator pushes after verify).

## Status

20/20 tasks complete (tasks.md checkbox count: 20 checked, 0 open). Ready for verify.

## Commits (one per unit, local on `main`, NOT pushed)

| Unit | Commit | Message |
|---|---|---|
| U1 parser | `4126e29` | `feat(adapter): add claude stream-json envelope parser` |
| U2 session | `16af1b8` | `feat(adapter): replace claude simulation with real stream-json session` |
| U3 factory | `ccfb64b` | `feat(adapter): dual-register claude in the factory` |
| U4 redaction+contract | `6e74e48` | `feat(execution): extend redaction to sk-ant tokens and add claude contract fixtures` |
| U5 integration+gate | `865a83b` | `test(engine): cover claude attempts end to end and settle the gate` |

## TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.1 | `internal/adapter/claude/parser_test.go` | Unit | ✅ baseline `go test ./...` all green | ✅ build fail: undefined `ParseStreamJSON` | ✅ `ok internal/adapter/claude` | ✅ 9 subtests (init/assistant/delta/result success+error/malformed/oversized/raw/blank) | ✅ clean |
| 1.2 | `internal/adapter/claude/parser.go` | Unit | same | (paired with 1.1) | ✅ | ✅ oversized just-under-limit case | ✅ fixed `event` JSON tag found via failing delta test |
| 1.3 | `parser_test.go` (`TestClaudeParseStreamJSON_FirstSessionIDWins`) | Unit | ✅ | ✅ | ✅ | ✅ no-session-id → empty (transport stays uncaptured) | ➖ none needed |
| 2.1/2.2 | `internal/adapter/claude/adapter_test.go` | Integration (subprocess) | ✅ baseline green; old test suite rewritten for new signature | ✅ compile fail: old `NewAdapter(string)` | ✅ `ok internal/adapter/claude` 5.28s | ✅ 13 lifecycle subtests | ✅ drain-on-parser-error refactor from deadlock finding |
| 2.3 | `adapter_test.go` (transport identity subtests) | Integration | ✅ | ✅ | ✅ | ✅ first-wins identity + stream-json extra | ➖ |
| 2.4 | `adapter_test.go` (probe subtests) | Unit | ✅ | ✅ | ✅ | ✅ reject 6 doc-like paths; accept README.sh/ELF/PATH-name; missing → unavailable | ✅ |
| 3.1 | `internal/adapter/factory/factory_test.go` (`TestFactoryDualRegistration`) | Integration | ✅ | ✅ (behavioral: `claude` key must run print-mode argv) | ✅ | ✅ claude/claudecode/oc/acp keys | ✅ |
| 3.2 | `factory_test.go` (`TestFactoryClaudeBinaryPrecedence`) | Unit | ✅ | ✅ undefined `ResolveClaudeBinary` | ✅ | ✅ seam isolation from OpenCode seam | ➖ |
| 3.3 | `factory_test.go` (`TestFactoryClaudeModelFlags`) | Integration | ✅ | ✅ | ✅ | ✅ overlay→flags; process env ignored (N1) | ➖ |
| 3.4 | no-regression run | — | ✅ | ➖ | ✅ `TestProbeCalledOncePerHarness`, `TestManager_ProbeCachedOnce`, `TestReopenDoesNotReprobeHarness`, `TestAgentManagerCLIInjection`, `TestOpenCodeRealSessionLifecycle` all pass | ➖ |
| 4.1/4.2 | `internal/execution/evidence_test.go` (`TestRedactAnthropicTokens`) | Unit | ✅ | ✅ bare `sk-ant-` + generic `sk-` unredacted | ✅ single-point extension in `Redact` | ✅ substring `sk-` (task-/risk-/ask-) untouched; Bearer unchanged | ✅ comment reword (boundary literal) in U5 commit |
| 4.3 | `internal/execution/agent_claude_evidence_test.go` (`TestClaudeEvidenceBoundedAndRedacted`) | Integration (engine+store) | ✅ | ✅ | ✅ | ✅ ≤16 KiB, redacted once, `adapter_name="claude"`, transport Extra | ✅ |
| 4.4 | `testdata/fixtures/synthetic/v2.0.0-synthetic.2/` | Fixture | N/A (new) | ✅ (new test 4.5 pins it) | ✅ | ➖ single pinned envelope | ➖ |
| 4.5 | `internal/adapter/contract/claude_suite_test.go` (`TestClaudeContractSuiteFixture`) | Integration | N/A (new) | ✅ test-first (no prod change needed; adapter behavior landed in U1–U3) | ✅ RunSuite via pinned fixture + argv/stdin assertions | ➖ single fixture | ➖ |
| 5.1 | `internal/execution/engine_test.go` (`TestClaudeAgentAttemptEndToEnd`, `TestClaudeAgentEmptyIntersectionFailsClosed`, `TestClaudeAgentCleanFailureFallsThrough`) | Integration | ✅ | ✅ test-first | ✅ | ✅ end-to-end, empty intersection fail-closed, clean-failure fall-through | ✅ |
| 5.2 | opt-in real-binary variants | Opt-in | N/A | ✅ | ✅ (skip-gated: `TestClaudeAdapter_OptIn`, `TestClaudeContractSuiteReal` skip in short/unset env) | ➖ | ➖ |
| 5.3 | delta sentence check | — | — | ➖ | ✅ `v2-adapter/F-01` delta sentence matches implementation (OpenCode AND Claude registered; ACP unregistered); main-spec update deferred to archive | ➖ | ➖ |
| 5.4 | final gate | — | — | ➖ | ✅ see below | ➖ | ➖ |

## Work Unit Evidence

| Unit | Focused test command + result | Runtime harness + result | Rollback boundary |
|---|---|---|---|
| U1 | `go test ./internal/adapter/claude` → ok (11 parser subtests) | N/A pure parser | revert `parser.go`+`parser_test.go` (commit `4126e29`) |
| U2 | `go test ./internal/adapter/claude` → ok 5.28s | executable fixtures in `t.TempDir()`, real subprocess | revert `adapter.go`+`adapter_test.go` (commit `16af1b8`) |
| U3 | `go test ./internal/adapter/factory` → ok | fixture sessions through the manager | revert `factory.go`+`factory_test.go` (commit `ccfb64b`); OpenCode behavior untouched |
| U4 | `go test ./internal/execution -run 'TestClaude\|TestRedact'` + `go test ./internal/adapter/contract` → ok | `RunSuite` via pinned synthetic.2 fixture | revert `evidence.go`, `agent_claude_evidence_test.go`, `claude_suite_test.go`, fixtures .2 (commit `6e74e48`) |
| U5 | `go test ./... -race` → all ok; `go build ./...` ok; `go vet ./...` ok; `scripts/verify-adapter-boundary.sh` pass | full suite incl. engine end-to-end | revert `engine_test.go` additions (commit `865a83b`) |

## Final Gate (task 5.4) — all four commands

- `go test ./... -race` → **all packages ok** (claude 8.81s, opencode 8.88s, execution 32.2s)
- `go build ./...` → **ok**
- `go vet ./...` → **clean**
- `scripts/verify-adapter-boundary.sh` → **all adapter boundary checks passed** (claude literals confined to `internal/adapter/claude` + `internal/adapter/factory`)

## Key discoveries / gotchas

1. **Pipe deadlock on early parser return (fixed)**: when the parser fails on an early malformed frame, it returns before draining stdout; a child blocked writing to a full pipe never exits and `cmd.Wait()` deadlocks on the stderr copier. Fix in `consume`: drain stdout with `io.Copy(io.Discard, …)` on parser error before `Wait`. Test fixture also reshaped so the oversized frame is one giant newline-terminated line (mirrors the opencode fixture shape).
2. **N2 pinned expectations recorded**: `dontAsk` deny-vs-auto-allow semantics and the nested `stream_event.event.delta` shape are pinned envelope expectations in the synthetic.2 fixture; they are NOT live-confirmed against a real binary. The opt-in real variants (`TestClaudeAdapter_OptIn`, `TestClaudeContractSuiteReal`, non-short + `HARO_TEST_CLAUDE_BINARY`) fail loudly on drift; run at cycle end only.
3. **N1 honored**: `CLAUDE_MODEL`/`CLAUDE_FALLBACK_MODEL` are read only from the harness config `Env` overlay; `t.Setenv` process-env case proves process env is ignored and no flags are added when the overlay omits them.
4. **Boundary literal**: the word "Anthropic" in a non-test Go comment trips `scripts/verify-adapter-boundary.sh` (hard-fail pattern); reworded to "sk-ant-style". Keep provider literals out of `internal/execution` comments.
5. **Contract package stays adapter-focused (N3)**: `claude_suite_test.go` covers RunSuite + fixture argv/stdin/identity; store/engine/settlement assertions live in `internal/execution` (`agent_claude_evidence_test.go`, `engine_test.go`).
6. **Fixture provenance (N4)**: envelope JSON is checked in under `testdata/fixtures/synthetic/v2.0.0-synthetic.2/`; executables are always generated in `t.TempDir()`; short mode never touches a real claude binary.

## Deviations

- `FirstSessionID` was extracted as a pure parser-package function (task 1.3 lives in `parser_test.go`) instead of inline logic in the adapter `consume`; the adapter consumes it, preserving N5 first-wins semantics with a directly testable seam.
- `claude` package `Probe` resolves bare binary names through PATH before the executable check (opencode only stats paths) — required so the default `claude` name remains usable while staying fail-closed.
- No extra commits were created for SDD docs; tasks.md checkbox marks are on disk (change dir untracked, managed by the orchestrator), and `openspec/changes/v2-claude-wiring/apply-progress.md` is this file.
- Workload: pre-approved `size:exception`; actual diff ≈ 2,059 added lines across 5 commits (parser 315, session 973 net, factory 270, redaction/contract/fixtures 303, engine 198).

## Non-requirement invariants verified

- `v2-adapter/F-05` checkbox remains `[ ]` in `deltas-acceptance.md`; file untouched (`git diff` empty).
- `docs/v2/*` untouched.
- No simulation restored; no ACP/PTY/broker/IPC/concurrency work; engine `runAgentStep` unchanged.

## Next

`verify` (sdd-verify) — all 20 tasks complete, gates green.
