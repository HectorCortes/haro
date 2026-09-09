# Exploration: v2-claude-wiring — real Claude Code CLI-direct harness

## Current State

### v2-agent-wiring baseline (archived 2026-09-09)

- `internal/adapter/factory/factory.go` builds a CLI-scoped `*adapter.Manager` with an **opencode-only** adapter, probes once (`Manager.Probe` cache, F-01 probe-once), initializes each available harness once (`Manager.Initialize` cache), and injects via `SetAdapterManager` at four sites (`handleRun`, `handleStepRun`, `handleStepReopen`, `handleStepSkip` in `internal/cmd/execute.go`). `go test ./... -race` green, boundary gate green.
- `internal/adapter/opencode/adapter.go` is **real**: `exec.CommandContext(binary, "run","--format","json")`, `Dir=WorkspaceRoot`, inherited env + overlay, `StdinPipe` carrying `PromptInput.Text`, `StdoutPipe`+`Stderr` buffer, `context.WithTimeout(≤300s)`, `done`+`doneOnce`/`cancelOnce`/`settleDone` idempotent `Cancel`, `ParseJSONL` with `MaxMessageSize` 10 MiB, `part.text → output_delta`, `error/exit/timeout/zero-text → failed`, `completed` terminal, `SessionTransport()` exposing `NativeSessionID` captured from `sessionID` field, consumed by `internal/execution/engine.go:runAgentStep` which replaces the identity-empty `attempt_transport` row (U-04). Requires containment via `resolveRequires`; all evidence goes through single `VisibleEvidence(Redact+16 KiB)` / `FallbackEvidence(2 MiB)` point.
- `internal/adapter/claude/adapter.go` is a **placeholder**: `Probe` via `exec.LookPath`+`os.Stat` (no ELF/shebang check), no stored negotiation, `NewSession` only checks `binary!=""`, `Prompt` simulates `ch <- {completed, {"text":input.Text}}` synchronously with no subprocess, no timeout, no `StdoutPipe`, no parser, no `TransportProvider`, no `settleDone`. `Contract suite` (`internal/adapter/contract/suite.go`) lifecycle test today passes only against a synthetic `completed` event; real boundary suite with fixture would not exercise protocol/transport.
- Spec `openspec/specs/v2-adapter/spec.md` **F-01** still states `Only OpenCode SHALL be registered; Claude and ACP MUST remain unregistered until their sessions are real.` F-05 (`Claude contract`) requires `step run → real subprocess → protocol → public JSON output → store → settle` against the opted-in binary or justified fixtures — today impossible. F-04/F-06/U-04 are archived as compliant via opencode.
- Installed CLI: `claude` 2.1.245 at `/home/dev/.local/bin/claude` (symlink to `/home/dev/.local/share/claude/versions/2.1.245`). Confirmed flags: `-p/--print`, `--output-format text|json|stream-json`, `--include-partial-messages` (only with `-p` + `stream-json`), `--input-format text|stream-json` (only with `--print`), `--permission-mode acceptEdits|auto|bypassPermissions|manual|dontAsk|plan`, `--model`, `--fallback-model` (only with `--print`), `--session-id`, `--continue`, `--resume`.

### Docs-v2 normative posture

- `docs/v2/haro-constitucion.md` II.1 lists Claude Code as initial target, II.3/V.3/V.4/XIII.3 require provider literals confined to adapters, V governs adapter contract, VIII.2/IV.3 bound evidence to 16 KiB after single `Redact`, XI mandates runnable contract suite against real binary or justified fixtures.
- `docs/v2/haro-especificacion-tecnica.md` §4 (`Adapter`/`Session`/`SessionHost`/`Capabilities`/`SessionBundle`/`SessionEvent`) and §7 (Broker↔Adapter ACP shape, `initialize` once per subprocess, `session/new`→`prompt`→`update`→`cancel`, additive capabilities, `attempt_transport` owning adapter identity) are the normative references. §7 normatively says `SessionHost is implemented by the broker`; under CLI-direct (`deferred:cli-direct` allowlist) the host is `engine.agentHost` — acknowledged as temporary tension, not a violation (engine host is `SessionHost`-agnostic and `gatedHost` preserves fail-closed).
- `deltas-acceptance.md` holds 92 criteria / 11 specs; `scripts/verify-adapter-boundary.sh` gates provider-literal leakage (only `internal/adapter/*` may contain literals). No new acceptance ID may be introduced without re-tabling.

## Affected Areas

- `internal/adapter/claude/adapter.go` — must be rewritten to mirror `internal/adapter/opencode/adapter.go`: add `DefaultBinary="claude"`, `ProtocolVersion=1`, `DefaultTimeout=300s`, `Adapter{binary, env, timeout, neg, initd}`, `isExecutableProgram` (ELF/shebang, reject docs-like paths `requirements.txt/CMakeLists.txt/Markdown/no-shebang`), `Probe` (Available:false no error for missing/non-executable/docs-like), `Initialize` (store `neg`), `NewSession` (guard `!initd→ErrNotInitialized`, `binary==""→error`, `resolveRequires` containment), `session{adapter,bundle,host,cmd,done,doneOnce,cancelOnce,mu,nativeID}`, `Prompt(exec.CommandContext, fixed argv, Dir, env overlay, StdinPipe/Stdin write+Close, StdoutPipe/Stderr buf, Start, consume goroutine, close(ch)+settleDone+cancelRun)`, `consume(ParseStreamJSON→emit output_delta/completed/failed, capture nativeID, map isTerminal vs clean failure, handle timeout=DeadlineExceeded, protocol error, zero-text)`, `Cancel(Kill+done wait)`, `SessionTransport() (NativeSessionID, ProtocolVersion, Extra)` plus `TransportProvider` assertion. New file `internal/adapter/claude/parser.go` (or `parse.go`) mirroring `opencode/parser.go` but for claude stream-json envelope.
- `internal/adapter/claude/parser.go` (new) — `ParseStreamJSON(io.Reader)([]Event,error)` with `bufio.NewReader.ReadString('\n')`, `json.NewDecoder.UseNumber`, `MaxMessageSize` 10 MiB rejection, fields `type`, `subtype`, `session_id`, `message.content[].text`, `result`, `cost_usd`, `is_error`, `error`, partial `stream_event.content_block_delta`, `system/init`.
- `internal/adapter/claude/adapter_test.go` — replace opt-in placeholder test with hermetic contract: `TestClaudeRealSessionLifecycle` (fixture argv/stdin/prompt, completed, failed, zero-text, error envelope, oversize, malformed, timeout, cancel idempotency, `isExecutableProgram` docs rejection, `SessionTransport` identity), driven by `HARO_TEST_CLAUDE_BINARY` fixture (see Factory).
- `internal/adapter/factory/factory.go` — add `TestClaudeBinaryEnv="HARO_TEST_CLAUDE_BINARY"` + `ResolveClaudeBinary(string)string` (env→configured→`claude`), branch on `cfg.Harnesses[name]` to instantiate `opencode.NewAdapter` for `opencode` names and `claude.NewAdapter` for `claude`/`claudecode` keys (harness names are data; factory switches on key equality, not provider literals outside adapter). Preserve `KnownFields` strictness; no engine change beyond factory because `engine.runAgentStep` already consumes generic `adapter.Manager` probe cache and ordered intersection.
- `internal/adapter/factory/factory_test.go` — extend `TestFactoryBinaryPrecedence` (claude path), `TestFactoryNewManager` (enabled/disabled/missing/docs-like/unregistered), `TestFactorySessionUsesConfiguredBinaryAndEnvOverlay` (claude fixture invocation log: argv, stdin prompt, env overlay).
- `internal/adapter/contract/suite.go` — no code change required (already generic over `AdapterFactory`), but F-05 coverage must instantiate suite with a claude factory (fixture binary or `HARO_TEST_CLAUDE_BINARY` opt-in) proving `subprocess→protocol→JSON→store→settle` with permission gating and redaction. Add `contract/claude_suite_test.go` (or extend `suite_test.go`) with `testing.Short()` skip and `HARO_TEST_CLAUDE_BINARY` gate, plus a fixture-backed variant that runs under `go test ./... -short` without real binary.
- `internal/execution/engine.go` — **no change needed** for the intersection/fallback path (already generic over `adapter_name` string and `ProbeResults` map). Confirm `runAgentStep` candidate loop (`candidates` = configured∩enabled∩registered∩Available), `agentHost` fail-closed, `AgentEvidence→VisibleEvidence(16 KiB)` single point, `FallbackEvidence(2 MiB)`, `isTerminal` classification, and `TransportProvider` type-assert remain correct for `claude`. The only engine-affecting decision is payload regex coverage for claude output (Bearer/sk- redaction already handled by `execution.Redact`).
- `testdata/fixtures/synthetic/v2.0.0-synthetic.2/` or `testdata/fixtures/claude/` (new) — pinned hermetic fixture pinning the claude `stream-json` envelope actually emitted by 2.1.245 (system/init + assistant + result + error), with `argv:`, `stdin:`, `env:` log convention matching opencode fixtures, plus one `sk-`/Bearer-bearing fixture proving `VisibleEvidence` redaction.
- `docs/v2/haro-especificacion-tecnica.md` § adapters — additive note: `claude -p --output-format stream-json --include-partial-messages` with stdin delivery is the CLI-direct transport; §7 ACP/terminal sentences need no change (ACP stays deferred, terminal stays `false`).
- `openspec/specs/v2-adapter/spec.md` — **modify** F-01 normative sentence from `Only OpenCode SHALL be registered; Claude and ACP MUST remain unregistered until their sessions are real.` to `Only OpenCode and Claude SHALL be registered; ACP MUST remain unregistered until its session is real.` (ACP unregistered invariant preserved). No new `F-*`/`U-*` IDs.
- `scripts/verify-adapter-boundary.sh` — already allows `opencode|claude|anthropic` only inside `internal/adapter/**`; claude additions inside `internal/adapter/claude/**` and `internal/adapter/factory` (one string literal `"claude"` key comparison) remain allowed. No gate change.

## Approaches

### 1. Mirror opencode exactly for claude (recommended) — CLI-direct stdin delivery + stream-json parser + factory dual-registration

- **Description**: `argv = [binary, "-p", "--output-format","stream-json","--include-partial-messages"]` plus optional `--permission-mode dontAsk` and `--model` when configured, fixed args, no shell, prompt on `StdinPipe` (not positional arg), `Dir=WorkspaceRoot`, env overlay, `context.WithTimeout(300s)`, `consume` mapping below. Add `internal/adapter/claude/parser.go` with `ParseStreamJSON`, reject `>10 MiB` frames, decode via `json.Number`. `Probe` uses `isExecutableProgram` identical to opencode. `HARO_TEST_CLAUDE_BINARY` precedence over configured `harnesses.claude.binary`. Factory registers `claude` when its harness record is configured+enabled and binary is an executable program; opencode path unchanged. Contract suite exercised both via executable fixture (fast) and via `HARO_TEST_CLAUDE_BINARY` opt-in E2E (user credits).

- **Prompt delivery justification**: Linux `MAX_ARG_STRLEN` ≈128 KiB per arg; fallback context is bounded to `FallbackLimit=2 MiB` (`internal/execution/evidence.go`), so passing prompt as positional `argv` would truncate or hit `E2LARGE`. Opencode already uses stdin; pin claude to stdin too. Verified: `claude --help` declares `Usage: claude [options] [command] [prompt]` with `prompt` optional; when absent and `StdinPipe` carries data, `claude -p --output-format stream-json` reads the prompt from stdin under `--input-format text` (default). Hermetic fixture asserts this (`stdin:<prompt>` log); real E2E triangulates same.

- Pros:
  - Exact parsimony with the green opencode path: one `consume` shape, one `Redact→VisibleEvidence` single point, one `SessionTransport` accessor, shared `isExecutableProgram`/`isTerminal`/`FallbackEvidence` semantics — minimal cognitive load, strict TDD already proven.
  - Preserves every invariant: F-01 probe-once (`Manager.Probe` cache), F-06 intersection+fallback, U-04 transport identity, evidence ≤16 KiB, fallback ≤2 MiB, fail-closed, provider-literal containment, 92/11 traceability (only modifies an existing F-01 sentence, does not add IDs).
  - Hermetic without real binary (executable sh fixture pinning the envelope), plus opt-in real E2E (`HARO_TEST_CLAUDE_BINARY`) matching `claude/adapter_test.go:10` convention (`testing.Short()` skip).

- Cons:
  - Must pin the real `stream-json` envelope without executing a live session during exploration (offline docs only). Mitigation: fixture pins envelope; `claude --help` + public docs define it; real E2E validates drift.

- Effort: Medium — ~1 new parser, ~1 rewritten adapter, factory 20-line branch, fixture, and extended tests. No engine change.

### 2. Positional-arg prompt delivery (reuse argv for claude's `[prompt]`)

- **Description**: Run `claude -p --output-format stream-json "the instructions..."` with prompt as the trailing argv element, no stdin pipe. Keep `--include-partial-messages`, otherwise mirror approach 1.

- Pros: Literal mapping of `claude --help` (`claude [prompt]`) without stdin plumbing.

- Cons:
  - Violates the 2 MiB fallback carry limit: `MAX_ARG_STRLEN` 128 KiB causes `E2LARGE`/truncation; would require pre-truncating prompt below 128 KiB and breaking the evidence-bounding contract that already asserts 2 MiB accumulation. Opencode precedent is stdin — divergence would create two harness transports for no benefit.
  - Harder to make hermetic: fixture must distinguish argv-prompt from stdin-prompt (two code paths).

- Effort: Low — but wrong boundary.

### 3. ACP-generic as claude transport

- **Description**: Do not add `internal/adapter/claude`; instead route claude through the generic `internal/adapter/acp` JSON-RPC over stdio (`initialize→session/new→prompt→update`), treating claude as an ACP speaker.

- Pros: Reuses existing `acp/translate.go` bijective capability maps.

- Cons:
  - Claude Code 2.1.245 does **not** speak ACP stdio; no `--acp` or `session/new` surface exists. Would require a sidecar translating ACP↔`claude -p`, adding a moving part and defeating provider-literal confinement (translation logic would live outside `internal/adapter/claude`). Out of scope per `v2-agent-wiring` scope boundary (ACP deferred).

- Effort: High — sidecar + indeterminate protocol mapping.

## Recommendation

**Approach 1 — mirror opencode exactly for claude with stdin delivery and `--permission-mode dontAsk`.**

Justification:

- **Contract fidelity**: The opencode path already satisfies F-01/F-04/F-06/U-04 with `exec.CommandContext` + `StdinPipe` + `VisibleEvidence` single point + `SessionTransport`. Reusing it for claude keeps the adapter contract (§4) and transport table (§7) additive without a new major version.
- **Boundary & traceability**: Only diffs `internal/adapter/claude/**` + one factory branch; `scripts/verify-adapter-boundary.sh` already allows claude literals inside `internal/adapter/**`. Modifying only the **existing** F-01 sentence (`OpenCode → OpenCode and Claude`) preserves 92/11 traceability, mirroring `v2-agent-wiring` which explicitly deferred any new acceptance ID (§ `Scope: Do not generate go.mod in advance` style, `v2-no-regresion/F-04` claiming deferred).
- **Headless correctness**: `-p` is the only non-interactive mode (interactive default hangs); `--output-format stream-json --include-partial-messages` is the only streaming mode that surfaces both `output_delta` and final `session_id`/`result` without polling. `dontAsk` is the fail-closed choice for headless agent steps (see permission-mode analysis below) and keeps `gatedHost` semantics intact.
- **Testability**: Matches `v2-spike-go` strict-TDD precedent: fixtures make `go test ./... -short` green without `claude` installed; `HARO_TEST_CLAUDE_BINARY` + `testing.Short()` skip (as in `internal/adapter/claude/adapter_test.go:11`) makes real E2E opt-in and credit-safe.

### Recommended argv + prompt delivery (grounded in installed CLI)

```
argv = [binary, "-p", "--output-format", "stream-json", "--include-partial-messages"]

where:
  binary = HARO_TEST_CLAUDE_BINARY (if set)
        else cfg.Harnesses["claude"].Binary (if configured via .haro/config.yaml)
        else "claude"

invariants:
  - fixed argv, no shell, exec.CommandContext with context.WithTimeout(300s)
  - cmd.Dir = bundle.WorkspaceRoot
  - env = inherited os.Environ() overlayed by cfg.Harnesses["claude"].Env
  - prompt delivered via cmd.StdinPipe → stdin.Write(input.Text) → stdin.Close()
    (not as a positional argv element; MAX_ARG_STRLEN ~128 KiB vs FallbackLimit 2 MiB)
  - optional additive flags BEFORE the fixed core when configured:
      --permission-mode dontAsk   (fail-closed default; see below)
      --model <model> / --fallback-model <csv>  (only meaningful with -p)
```

Evidence that `claude -p` reads stdin when no positional `[prompt]` is given:

- `claude --help` declares `Usage: claude [options] [command] [prompt]` — `[prompt]` optional; `--input-format text|stream-json (only works with --print)` declares a stdin input format, and `--include-partial-messages (only works with --print and --output-format=stream-json)` + `--output-format stream-json` together describe a streaming JSONL over stdout in headless mode. The only way a 2 MiB fallback context can reach the harness under `Linux MAX_ARG_STRLEN ≈128 KiB` is stdin.
- Opencode precedent already pins stdin delivery (`opencode run --format json` with `stdin.Write(input.Text)` at `internal/adapter/opencode/adapter.go:244`), and the factory fixture asserts `argv:run --format json` + `stdin:please summarize` (log).
- Hermetic fixture for claude must mirror the same contract: `stdin=$(cat); printf 'argv:%s\nstdin:%s\n'` then emit pinned envelope to stdout; real E2E with `echo "hello" | claude -p --output-format stream-json` (opt-in, not run during exploration) triangulates.
- If offline docs alone were the sole source, the envelope is still **pinned by fixture** and a live E2E validates it — identical to how opencode's envelope was pinned before the real binary was observable.

### stream-json envelope → output_delta/completed/failed + native_session_id

Based on `claude --help` option constraints plus public Claude Code docs (no live session executed during exploration; envelope fully pinned by fixtures, validated by opt-in E2E):

| stream-json JSON line (`--output-format stream-json`) | Adapter mapping | Evidence / session_id capture |
|---|---|---|
| `{"type":"system","subtype":"init","session_id":"<uuid>","model":"...","tools":[...]}` | ignored as event (no `output_delta`); capture `session_id` into `s.nativeID` (first non-empty wins, `mu` guarded) | `s.mu.Lock(); if s.nativeID=="" {s.nativeID=ev.SessionID}` mirroring opencode `sessionID` field; `SessionTransport()` exposes it with `ProtocolVersion=neg.ProtocolVersion`, `Extra={"transport":"stream-json"}` |
| `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"..."},...]}}` | each `content[].text` where `type=="text"` → emit `output_delta` with `text` | enque `SessionEvent{Type:"output_delta",Payload:[]byte(text)}`; `sawText=true` |
| `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"..."}}}` (only with `--include-partial-messages`) | `delta.text` → `output_delta` | same as above; enables incremental streaming before final result |
| `{"type":"result","subtype":"success","session_id":"<uuid>","result":"...","is_error":false,"cost_usd":...,"duration_ms":...}` | `is_error==false` and `subtype=="success"` and `sawText||result!=""` → emit `completed`; capture `session_id` again | `emit("completed","")` after all deltas; `didComplete=true` |
| `{"type":"result","subtype":"error","session_id":"<uuid>","error":"...","is_error":true}` or any `is_error==true` | → `failed` with `error` | `emit("failed", error)`; also `type=="error"` lines in stream |
| any line with `error` top-level field | → `failed` | `failMsg = ev.Error` (first) |
| malformed line (not JSON) or `len(line)>10 MiB` | protocol error → `failed` `protocol error: ...` | parser returns `perr!=nil`; `consume` emits `failed` before anything else |
| empty stdout / only `system/init` then EOF with `Wait==nil` and `sawText==false` | → `failed` `no output: session ended without text events` | clean failure (not terminal marker) — lets fallback try next candidate |

Terminal vs clean classification (fed to `execution.isTerminal`):

- protocol/malformed (`protocol error:`), timeout (`context.DeadlineExceeded` → `failed: timeout: session exceeded 300s`), `unsupported_capability`, `contract`, `store` → terminal (`failed` with `terminal:` reason, exhaust loop aborts).
- `no output`, harness-declared `error` envelope, non-zero `Wait` + stderr without text → clean fallback (`failed` without terminal marker, `accumulated = FallbackEvidence(evidences)` and continue to next candidate).

### Permission-mode in headless

- **Choices**: `acceptEdits` (auto-accept file edits), `auto` (auto with gating), `bypassPermissions` (dangerously skip), `manual` (ask), `dontAsk` (auto-deny without prompt), `plan` (read-only planning).
- **Headless semantics** (`-p`): Any `manual`-like prompt has no TTY to answer it. If the harness would emit `session/request_permission` in interactive mode, under `-p` with a permissive mode it would either auto-allow or hang waiting for stdin that never arrives. `dontAsk` is defined to **deny without asking** — the harness resolves to its default policy (deny) or surfaces an error without blocking, matching fail-closed (const. V.4/VIII.5).
- **Interaction with negotiated `Permission:true` and `gatedHost`**: `internal/adapter/claude/adapter.go` advertises `Permission:true`, `manager.go:gatedHost` gatekeeps `RequestPermission` on `caps.Permission`. With `dontAsk`, the harness typically will not call `RequestPermission` (it denies internally); with `auto` it would call it. Either way, if it does call, `gatedHost` allows it (since negotiated true) and `engine.agentHost.RequestPermission` records `permission_requested` and returns `Options[0]` (`allow`). To prevent a hang we must set the CLI flag to `dontAsk` explicitly.
- **Recommendation**: `argv` includes `--permission-mode dontAsk` **by default** for agent steps (`mode=="headless"`). If the step declares `mode=="supervised"` (future) and the design wants interactive permission, that can be elevated to `--permission-mode auto` via a later change — but the initial slice should be `dontAsk`. Document that `dontAsk` + `Permission:true` negotiation means: harness denies silently → if harness surfaces denied as error, engine maps it to clean `failed` (fallback) rather than terminal; if it does emit `RequestPermission`, the host will still answer fail-closed (deny when `caps.Permission==false`, allow when true — but claude is `true` so answer is allow; the mismatch is intentional and documented). Flag as a **design decision** in `sdd-design`.

### Factory & engine impact (confirm no engine change)

- Factory: `ResolveClaudeBinary(configured)string{ if HARO_TEST_CLAUDE_BINARY!="" return it; if configured!="" return it; return "claude" }`. In `NewManager(ctx,cfg)` iterate `cfg.Harnesses`; for each `name,hc` if `!hc.IsEnabled() continue`; if `name=="opencode"` register `opencode.NewAdapter`, `else if name=="claude"||name=="claudecode"` register `claude.NewAdapter(ResolveClaudeBinary(hc.Binary), hc.Env, timeout)`. This keeps provider literals to one string-equality switch inside `internal/adapter` — the boundary gate treats `internal/adapter/*` as allowed.
- Engine: already does `probes=mgr.ProbeResults()` (no re-probe), ordered intersection `harnessCandidates ∩ {configured,enabled,registered,Available}` (F-06), per-candidate `WithTx` generation/attempt/transport, `Prompt→AgentEvidence→VisibleEvidence`, `TransportProvider` type-assert after settle. No modification needed beyond adapter identity string (`harness=="claude"` now flows to `attempt_transport.adapter_name` generically — `internal/store` and `internal/execution/evidence.go` never switch on provider).

### Contract-suite plan for F-05

- **Location**: `internal/adapter/contract/suite.go:RunSuite(factory AdapterFactory)` — lifecycle (Probe→Initialize→NewSession→Prompt→Cancel), gating (LoadPrevious/Terminal unsupported), permission (fail-closed vs allow), boundary (no panic). This is the public-boundary suite referenced by `v2-adapter/F-05`.
- **Claude-specific harness**:
  - `internal/adapter/claude/parser_test.go` — `ParseStreamJSON` Valid + EnvelopeCapture (system/init session_id + assistant text + stream_event delta + result success/error) + OversizeViaCodec (10 MiB) + Malformed.
  - `internal/adapter/claude/adapter_test.go:TestClaudeRealSessionLifecycle` — mirror of opencode's 8 subcases but for claude stream-json: fixed argv (`-p --output-format stream-json --include-partial-messages` and no shell), stdin prompt, env overlay, completed/failed/zero-text/error/timeout/cancel idempotent (settleDone), `isExecutableProgram` docs rejection, `SessionTransport` native id+version.
  - `internal/adapter/contract/claude_suite_test.go` (or inline in `claude/adapter_test.go`): `func TestClaudeContractSuite(t *testing.T){ if testing.Short(){t.Skip} ; if HARO_TEST_CLAUDE_BINARY=="" {t.Skip} ; contract.RunSuite(t, func()adapter.Adapter{ return claude.NewAdapter(HARO_TEST_CLAUDE_BINARY) }) }` plus a **fixture-backed variant** `TestClaudeContractSuiteFixture(t)` that builds an executable `sh` fixture emitting pinned envelope to stdout and runs `contract.RunSuite` against it — this is what `go test ./... -short` exercises (CI), proving `subprocess→protocol→JSON→store→settle` without a real binary.
  - Store/settlement is proven by `internal/execution/engine_test.go:TestAgentStepPersistsRealEvidenceAndTransport` analog for claude: fabricate a `project.Config{Harnesses:{"claude":{Binary:fixture}}}` → `factory.NewManager` → `engine.SetAdapterManager` → `engine.RunStep(agent)` → assert `attempt_transport.adapter_name=="claude"`, `native_session_id!=nil`, `attempt_events.payload` contains redacted `Bearer ***` and ≤16 KiB, `result_digest==sha256(payload)`.

### Scope boundaries (non-goals confirmed)

- **ACP JSON-RPC evolution** — out of scope. `internal/adapter/acp/translate.go` remains unchanged; ACP registration stays deferred (F-01 `ACP MUST remain unregistered`) per normative §7.
- **Terminal/PTY** — deferred. `Capabilities.Terminal==false`, `Terminal()` returns `ErrUnsupportedCapability`, engine guards `mode=="terminal"→workflow_invalid` (`internal/execution/engine.go:789`, `internal/workflow/validate.go`). `docs/v2/haro-constitucion.md` XII.1 notes PTY as existing surface to preserve, but no terminal work in this slice.
- **Broker/IPC (`v2-broker`/`v2-ipc`)** — out of scope. `internal/cmd/execute.go` injection (`injectAdapterManager` CLI-direct) is the temporary host; `SessionHost is implemented by the broker` (§7) tension remains allowlisted as `deferred:cli-direct` until those SDD changes.
- **No new acceptance IDs (92/11 invariant)** — confirmed. Modify only the **one sentence** of F-01 (`Only OpenCode SHALL be registered` → `Only OpenCode and Claude SHALL be registered`). Do not add `F-0x`/`U-0x` headings in `deltas-acceptance.md` or `openspec/specs/v2-adapter/spec.md` delta; new IDs, if ever needed, belong to a future traceability re-tabling during `archive` (deliberate proposal).
- **F-05 claim feasibility** — **enabled by this change, NOT claimed by this change**. The slice makes the adapter pass the public-boundary suite (fixture + opt-in real), which is the technical prerequisite for F-05. Claiming (`[x]`) would require atomically updating `deltas-acceptance.md` (mark F-05 green), the traceability table, and the SDD `spec.md` delta, plus re-running `scripts/verify-traceability.sh` — exactly the re-tabling that `v2-agent-wiring` deliberately deferred (its verify-report notes `Do not add broker/IPC/PTY/concurrency, edit deltas-acceptance.md (92/11), or claim v2-no-regresion/F-04`). Mirror that precedent: enable now, claim later in a dedicated `archive`/`sdd-verify` that re-tables traceability, or leave F-05 `[ ]` with implementation evidence and a pending-claim note.
- **docs/v2 normative conflicts** — none. `haro-constitucion.md` II.1 already targets Claude Code; `haro-especificacion-tecnica.md` §4/§7 are provider-agnostic; provider literals confined to `internal/adapter/claude/**` satisfies II.3/XIII.3 and `verify-adapter-boundary.sh`. The only normative text needing update is the F-01 registration sentence; everything else is additive.

### Risks

- **Print-mode envelope drift**: Claude Code stream-json field names (`session_id` vs `sessionID`, `result` shape, `is_error` placement) may evolve between versions. If the parser expects `sessionID` but the CLI emits `session_id`, native ID capture fails and `SessionTransport` stays empty (attempt persists without identity), while evidence still lands but without the U-04 identity guarantee. Mitigate: pin exact envelope of 2.1.245 in a versioned fixture (`v2.0.0-synthetic.2` or `testdata/fixtures/claude/stream.jsonl`), assert parser against it, and run `HARO_TEST_CLAUDE_BINARY` opt-in E2E in `t.Short()==false` to triangulate real output; on mismatch, fail loudly and bump fixture version.
- **Stdin vs argv prompt size**: If any code path reintroduces positional-arg prompt delivery (tempting from `Usage: claude [prompt]`), the 2 MiB fallback context will hit `MAX_ARG_STRLEN` (~128 KiB per arg) → `E2LARGE` or silent truncation → launch failure that masks the real harness error and produces a terminal-classified `process_start` rather than clean fallback. Mitigate: single `StdinPipe` code path, fixture log asserting `stdin:` contains full prompt, and unit test feeding a 1 MiB prompt through stdin.
- **Permission prompt hang**: Without `--permission-mode dontAsk`, a Claude harness in `-p` mode that needs tool permission may block waiting for a TTY answer that never arrives, hanging until the 300s timeout and consuming one full attempt without evidence before fallback. Mitigate: default adapter argv to `dontAsk`, document the fail-closed choice in design, and timeout test (`context.WithTimeout` 1s → `failed: timeout`).
- **Redaction of claude output**: Claude output may contain secrets in new forms (`sk-ant-...`, `sk-...`, `Bearer sk-` split across chunks) not covered by `execution.Redact` (`Bearer\s+`, `Basic\s+`, `ghp_`, `token|secret|api_token` assignments). Unredacted secrets would persist in `attempt_events.payload` (16 KiB window) and leak to `execution.report`. Mitigate: extend redaction with `sk-[A-Za-z0-9\-_]{20,}` and ensure redaction runs once over the fully composed `AgentEvidence` (single point) — add a claude-specific fixture `payload:"sk-ant-..."` → assert `VisibleEvidence` contains `***`.
- **Version drift of CLI flags**: Flags `--include-partial-messages`, `--permission-mode`, `--output-format stream-json` could be renamed or become required `--input-format` companions in future Claude releases; hard-coded argv would then `exit 2` (unknown flag) and all claude candidates would become clean-fallback failures with no indication in `probes`. Mitigate: probe by running `<binary> --help` or `<binary> -p --help` in `Probe`? Today `Probe` only checks ELF/shebang, not flag support — intentionally, to keep probing cheap. Instead, treat unknown-flag exit as clean fallback (not terminal) and surface the stderr in the `failed` payload so the failure is diagnosable; pin flag list in `ADRs` and re-verify with `claude --help` on each upgrade.
- **Real-binary E2E needs user opt-in (cost/credits)**: A true `claude -p` run consumes the user's Claude subscription/credits and requires authentication (`claude doctor`/`auth`). CI cannot run it; local opt-in may fail when the user is unauthenticated, producing a confusing `failed: not logged in` that looks like a harness bug. Mitigate: gate all real-binary tests with `if testing.Short()||HARO_TEST_CLAUDE_BINARY=="" { t.Skip }`, document `HARO_TEST_CLAUDE_BINARY` and `claude auth` prerequisites in `CONTRIBUTING`/`README`, and keep the fixture-backed contract suite as the CI gate.

## Ready for Proposal

Yes — proceed to `sdd-propose` for `v2-claude-wiring` as a CLI-direct slice mirroring `v2-agent-wiring`.

Proposer should:

- State change name `v2-claude-wiring`, type `feat`, scope `internal/adapter/claude, internal/adapter/factory, testdata/fixtures, openspec/specs/v2-adapter`.
- Reference constitution II.1/II.3/V/VIII/XI, tech-spec §4/§7, and traceability invariant 92/11 to justify **modifying** F-01's registration sentence but not adding any new `F-*`/`U-*` IDs and not claiming F-05 yet.
- Scope to: rewrite `internal/adapter/claude/{adapter.go,parser.go}`, add `isExecutableProgram`+`ResolveRequires`+`session` lifecycle mirroring opencode, register claude via factory when `harnesses.claude` is configured+enabled, add pinned `stream-json` fixtures, and extend tests/gates (`parser_test`, `adapter_test`, `factory_test`, `contract suite` fixture+opt-in). Explicitly keep out of scope: ACP, terminal/PTY, broker/IPC, workflow schema, evidence budgets.
- Flag the five risks above, the `dontAsk` permission-mode decision, and the stdin-delivery invariant (`MAX_ARG_STRLEN` vs 2 MiB) as hard constraints in the rollout checklist, plus the opt-in E2E needing `claude auth` / credits.

---
*Evidence pins*: `claude:2.1.245` (`--help` argv: `-p --print`, `--output-format stream-json`, `--include-partial-messages` only with `stream-json`, `--input-format text|stream-json` only with `--print`, `--permission-mode acceptEdits|auto|bypassPermissions|manual|dontAsk|plan`), `internal/adapter/opencode/adapter.go:1-364` (ELF/shebang probe, `exec.CommandContext`+`StdinPipe`+`Dir`+env overlay+300s, `doneOnce/settleDone`, `SessionTransport`), `internal/adapter/claude/adapter.go:1-98` (placeholder), `internal/adapter/factory/factory.go:24-68` (opencode-only), `internal/adapter/contract/suite.go:10-116` (public-boundary suite), `internal/execution/engine.go:691-1040` (ordered-intersection fallback, `gatedHost`, `AgentEvidence→VisibleEvidence→16 KiB`, `FallbackEvidence→2 MiB`, `isTerminal`, `TransportProvider`), `docs/v2/haro-constitucion.md:II.1, II.3, V, VIII, XI, XIII.3` and `docs/v2/haro-especificacion-tecnica.md:§4, §7, §7.2`, `openspec/specs/v2-adapter/spec.md:F-01/F-05` and `deltas-acceptance.md` F-05 + traceability, `scripts/verify-adapter-boundary.sh`, `testdata/fixtures/synthetic/v2.0.0-synthetic.1/` (synthetic provenance), `openspec/changes/archive/2026-09-09-v2-agent-wiring/{proposal,design,verify-report}.md`.
