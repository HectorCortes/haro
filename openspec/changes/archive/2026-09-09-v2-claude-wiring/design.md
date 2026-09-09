# Design: v2 Claude Wiring

## Technical Approach

Mirror the OpenCode CLI-direct lifecycle. The factory builds both providers, the manager probes/initializes once, and unchanged `runAgentStep` intersects usable candidates, persists transport/evidence, and falls back. Print mode enables F-05; ACP remains future authority and unregistered.

`config → factory → Manager probe/init cache → runAgentStep → Claude session → AgentEvidence/VisibleEvidence → store`

## Architecture Decisions

| Decision | Alternatives | Rationale |
|---|---|---|
| Use `claude -p` stream JSON now; evolve to ACP later | ACP sidecar now | Claude 2.1.245 has no ACP surface; print mode satisfies the real-session boundary without changing core. |
| Send prompts only through stdin | Positional prompt | The 2 MiB fallback exceeds Linux's approximately 128 KiB per-argument limit. |
| Always add `--permission-mode dontAsk` | manual/auto/bypass | Headless permission needs deny without TTY wait; denial is a clean fallback. |
| Register Claude only for an enabled key exactly named `claude`; preserve OpenCode for other enabled keys | Always register; aliases | Configuration controls availability without regressing existing arbitrary OpenCode names. |
| Translate non-empty `CLAUDE_MODEL` and `CLAUDE_FALLBACK_MODEL` harness-env values to flags | Extend `HarnessConfig`; leave unset | Supports models without a schema change. |
| Fixture suite plus opt-in real binary | CI real binary only | Hermetic short tests avoid auth/cost; opt-in validates real CLI drift. |
| No engine change; enable but do not claim F-05 | Provider branch in engine; claim acceptance now | Existing generic intersection, transport, fallback, and evidence paths already apply; the 92/11 acceptance invariant remains unchanged. |

## Data Flow

```text
Success: engine → claude(stdin) → init/session_id → text deltas → completed → redacted store
Denied:  engine → claude(dontAsk) → failed(clean) → next candidate
Closed:  engine → intersection(empty/exhausted) → step failed; no synthetic success
```

## Interfaces / Contracts

`NewAdapter(binary string, env map[string]string, timeout time.Duration)` stores protocol 1 negotiation (`Permission:true`; other optional capabilities false). `Prompt` executes `[binary,"-p","--output-format","stream-json","--include-partial-messages","--permission-mode","dontAsk"]`, then optional model flags, using `exec.CommandContext`, timeout ≤300s, `Dir=WorkspaceRoot`, inherited environment plus overlay, and `StdinPipe`; no shell or prompt argv. The session mirrors `doneOnce`, `cancelOnce`, `settleDone`, mutex-protected `cmd/nativeID`, idempotent `Cancel`, and `SessionTransport{NativeSessionID, ProtocolVersion, Extra:{"transport":"stream-json"}}`.

`ParseStreamJSON(io.Reader) ([]Event,error)` uses newline frames bounded by `jsonrpc.MaxMessageSize` (10 MiB) and `UseNumber`. Event fields are `type`, `subtype`, `session_id`, `message.content[].{type,text}`, `event.type`, `event.delta.{type,text}`, `result`, `is_error`, and `error`.

| Claude 2.1.245 envelope | Mapping |
|---|---|
| `system/init` | Capture first `session_id` |
| `assistant` text content | `output_delta` |
| `stream_event` / `content_block_delta` / `text_delta` | `output_delta` |
| `result`, `subtype:success`, `is_error:false` | Emit `result` as fallback text only if no text was seen, then `completed` |
| error result/top-level error | `failed` (clean fallback) |
| malformed/oversized | protocol `failed`; timeout is terminal |
| EOF without usable text | clean `failed`, never `completed` |

The fixture pins these assumed 2.1.245 names. Nested `stream_event.event.delta` is not live-confirmed; opt-in testing fails loudly on drift. `execution.Redact` adds `sk-ant-`/`sk-`; `AgentEvidence → VisibleEvidence` remains the single redaction and 16 KiB bound.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/adapter/claude/parser.go` | Create | Bounded envelope parser. |
| `internal/adapter/claude/{adapter,parser}_test.go` | Modify/Create | Parser, lifecycle, stdin, permission, settlement tests. |
| `internal/adapter/claude/adapter.go` | Modify | Replace simulation with real session. |
| `internal/adapter/factory/{factory.go,factory_test.go}` | Modify | Dual registration and binary precedence. |
| `internal/adapter/contract/claude_suite_test.go` | Create | Fixture and opt-in boundary variants. |
| `internal/execution/{evidence.go,evidence_test.go,engine_test.go}` | Modify | Token redaction and persisted Claude evidence/transport assertions. |
| `testdata/fixtures/synthetic/v2.0.0-synthetic.2/*` | Create | Versioned executable/envelope provenance. |

`internal/execution/engine.go`, `docs/v2/*`, and the boundary script remain unchanged.

## Testing Strategy

Unit tests pin all nine scenarios, frame limits, ELF/shebang classification, model flags, and exact-once settlement. `TestClaudeContractSuiteFixture` runs under `go test -short`, combining `contract.RunSuite` with factory/engine/store coverage through subprocess→protocol→JSON→store→settlement, permission denial, redaction, bounds, and identity. A real variant requires non-short mode plus `HARO_TEST_CLAUDE_BINARY`; authenticated/cost-bearing E2E runs only at cycle end.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | Applicable | Executable regular ELF/shebang files probe available; other files are unavailable and never launched. | Reject requirements.txt, CMakeLists.txt, Markdown/MDX without shebang; accept executable README.sh with shebang |
| Git repository selection | N/A: subprocess cwd is the supplied workspace, not Git selection | — | — |
| Commit state | N/A: no VCS operation | — | — |
| Push state | N/A: no VCS operation | — | — |
| PR commands | N/A: no PR automation | — | — |

## Migration / Rollout

No migration. Disable `claude` to roll back. Risks: envelope/flag/version drift (fixture, stderr, opt-in test), prompt size (1 MiB stdin test), permission hang (dontAsk/timeout), and redaction gaps (Bearer/sk-ant-/sk- tests).

## Open Questions

None blocking; live confirmation of the nested partial-event envelope is an explicit opt-in verification gate.
