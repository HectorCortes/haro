# Exploration: v2-agent-wiring — real adapter wiring for agent steps (CLI-direct)

## Current State

### Execution path — `internal/execution/engine.go:runAgentStep` (L691-960)

`Engine.RunStep` dispatches on `step.Type == "agent"` to `runAgentStep`. The method mirrors the `command` path for preconditions:

- Re-flattens and verifies DAG hash (`verifyDAGHash`), fetches `ExecutionStep`, guards `workflow` leakage, handles `pending` vs `feedback`-driven reconstruction, checks `depends_on` and `requires` (including `requiresStale`/`latestInvalidGeneration` gating), acquires path claims, transitions `pending -> running`.

Then it diverges:

1. Loads workflow YAML to obtain `harness []string`, `instructions string`, `mode string` by re-opening `exec.WorkflowSource` via `workflow.Parse` or `workflow.Discover` fallback (L751-784). If `len(harness)==0` → `failAttempt(no-attempt)` and error. If `mode=="terminal"` → immediate fail (PTY deferred per spec §7 / `haro-constitucion.md` XII.1, `validate.go` rejects it at parse time as `workflow_invalid`).

2. Iterates `harnessCandidates` in order (fallback loop L796-958). For **each** candidate it atomically creates a new `Generation`+`Attempt`+`Transport` via `store.WithTx`:
   - `native = harness + "-session"` (synthetic, L817), `protocolVersion=1`, `extra="{}"` (L818-825). This is placeholder; real harness will overwrite `native_session_id` via transport row or adapter return.
   - Errors in `WithTx` are classified via `isTerminal(err)` → terminal reason + `completeAttempt("failed")` + `transitionStep("running","failed")`.

3. If `e.adapterMgr != nil` it enters the **real path** (L862-907):
   - `bundle = SessionBundle{Instructions: instructions+"\n"+accumulated, WorkspaceRoot: wtRoot, Requires: map[string]string{}}` — note `Requires` is empty today (no logical→physical resolution).
   - `host = &agentHost{store, attemptID}` (L871) — `RequestPermission` records a `permission_requested` event (payloadRef=description) and auto-approves with `Options[0]`.
   - `sess,err := mgr.NewSession(ctx,harness,bundle,host)` — requires prior `Initialize`; on `ErrNotInitialized` this becomes `runErr`.
   - `input.Text = instructions` (or `accumulated+"\n"+instructions` when fallback context exists) — L878-882.
   - `ch,err := sess.Prompt(ctx,input)` then consumes `ch` until `completed` or `failed` (`output_delta` payloads concatenated). `Cancel` deferred. If Prompt succeeded and output empty → `output = "harness X completed"`.

4. Else (`adapterMgr==nil`) it takes the **simulated path** (L908-915) for tests without a manager: `idx < len-1` → `runErr = clean failure for X` with output `output from X Bearer secret123`, last idx → success `success from X`. This is the evidence of the critical finding (`engram #2498`): production today follows this branch because CLI never sets `adapterMgr`.

5. Evidence + persistence per candidate (L918-958):
   - `visible := AgentEvidence(harness, idx+1, total, mode, instructions, output)` → composed as `harness: <name> (i/n)\nmode: <mode>\ninstructions:\n<bounded 4KiB>\n--- output ---\n<output>` then `Redact` (Bearer/Basic/token patterns + `ghp_*`) and bounded to 16 KiB (`VisibleEvidence`). One `CreateAttemptEvent` with `Payload=&visible`, `eventType="output_delta"` per attempt.
   - On `runErr==nil`: `sha256(visible)` → `completeAttempt(completed, digest)`, `transitionStep→completed`, `resyncExecution`, return nil (short-circuits loop; remaining candidates untried).
   - On `isTerminal(runErr)`: `VisibleEvidence("terminal: "+err)` + `FallbackEvidence(join(evidences))` → `completeAttempt(failed, reason, accumulated)` + `transitionStep→failed`, return err (no further fallback).
   - On clean failure: `accumulated = FallbackEvidence(join(evidences))` (2 MiB cap), `completeAttempt(failed, VisibleEvidence(err), accumulated)` for this attempt, then `continue` unless exhausted → `transitionStep→failed`.

6. Evidence storage aligns with `v2-evidencia` / `migrations.go:ensurePayloadColumn`: `attempt_events.payload` holds the sanitized bounded delta inline (≤16 KiB после `VisibleEvidence`), `payload_ref` remains readable as legacy (`PriorOutputDelta` prefers `payload` when non-nil). Transport headers are separate: `attempt_transport` row per attempt (adapter_name, native_session_id, protocol_version, extra) inserted in same Tx.

The simulated branch exercises fallback semantics and redaction (`Bearer ***`) but uses synthetic session ids `opencode-session`/`claude-session` and synthetic output — NOT real harness invocation.

### CLI wiring point — `internal/cmd/execute.go`

- `Execute` L37 routes `run` → `handleRun` L231 and `step run` → `handleStepRun` L444.
- `handleRun` L260: `eng := execution.NewEngine(s, getRunner(), cwd)` then `eng.CreateExecution(ctx,name)` — no adapter wiring.
- `handleStepRun` L475: `eng := execution.NewEngine(s, getRunner(), cwd)` then `eng.RunStep(ctx,execID,stepID,feedback)` — no adapter wiring. Identical pattern in `handleStepReopen`/`handleStepSkip` (engine created without manager). `SetAdapterManager` is defined L51-53 but has zero production callers (grep confirmed).
- Workflow discovery→run→evidence end-to-end today: `workflow.Discover`/`Flatten`/`ValidateFile`→ `CreateExecution` (writes `executions`+`execution_steps`+worktree+dag_hash/base_commit)→ `steps next` (filters by deps+claims)→ `step run`→ `RunStep`→ generation/attempt/transport/event rows + `attempts.result_digest` (sha256 of visible). Evidence read path for `feedback` uses `Events().PriorOutputDelta` with `payload` precedence (L586-593).

The smallest real wiring change is injecting a `*adapter.Manager` into the engines created in `handleRun`/`handleStep*` (and covering `steps next` only if claims logic needed harness awareness — it does not). Construction must happen after store open and before any engine method, using the same `cwd` that is the project root / `projectID`.

### Adapter layer surface — `internal/adapter/*`

- **`manager.go`**: `NewManager(map[string]Adapter)`, `Probe(ctx) map[string]ProbeResult`, `Initialize(ctx,name,Capabilities) (Capabilities,error)` (exactly-once per adapter, cached, mutex-guarded), `NewSession(ctx,name,SessionBundle,SessionHost) (Session,error)` (fails with `ErrNotInitialized` if not initialized, wraps host in `gatedHost` enforcing `caps.Permission` fail-closed), `Negotiated(name) (Capabilities,bool)`.

- **`adapter.go`**: `Adapter { Probe, Initialize, NewSession }`, `Session { Prompt(ctx,PromptInput)<-chan SessionEvent, Cancel, LoadPrevious, Terminal }`, `SessionBundle{Instructions, WorkspaceRoot, Requires map[string]string}`, `PromptInput{Text}`, `SessionEvent{Cursor, Type, Payload}`, `SessionHost{RequestPermission}`, `ProbeResult{Available,Version,Capabilities}`, `Capabilities{ProtocolVersion,Permission,Terminal,LoadSession,Extra}`. Errors: `ErrNotInitialized`, `ErrUnsupportedCapability`.

- **Per-adapter runtime needs**:
  - `claude/adapter.go` L13-70: `NewAdapter(binary string)` (empty→`"claude"`, env override `HARO_TEST_CLAUDE_BINARY`), `Probe` via `exec.LookPath`+`os.Stat`, reports `Permission:true,Terminal:false,LoadSession:false`, `Initialize` via `adapter.Negotiate` requiring `ProtocolVersion==1`, `NewSession` only checks `binary!=""` and returns `claudeSession` whose `Prompt` currently simulates `{"text":input.Text}` → `completed` (real subprocess TODO). Needs binary on PATH or absolute path, no extra flags today.
  - `opencode/parser.go`: JSONL parser present (`ParseJSONL` with 10 MiB frame limit via `jsonrpc.MaxMessageSize`), but no `internal/adapter/opencode/adapter.go` file exists — the harness implementation is not yet fully materialized as an `Adapter`; only parser unit.
  - `acp/translate.go`: bijective `ToACP`/`FromACP` + `TranslateInitialize`/`TranslateSessionNew`/`TranslatePrompt`/`TranslateUpdate`/`TranslateCancel`/`TranslateRequestPermission` for ACP generic harness. Implies `acp` adapter would speak ACP over stdio; harness binary selection similar to claude.

- **Config mapping gap**: `.haro/config.yaml` today (`project/config.go`) holds only `version int` + `external_paths []string` with strict yaml.v3 parsing (no harness fields). Workflow `Step.Harness []string`, `Instructions`, `Mode` already parsed via `workflow/parse.go:51-53` with `KnownFields(true)` and validated in `validate.go:170-216`. No harness binary / env mapping exists yet; `internal/project` has no harness config type. Negotiation expects `Capabilities{ProtocolVersion:1, Permission:true}` as core proposal (seen in `manager_test.go`).

### Evidence pipeline

- `execution/evidence.go`: `VisibleLimit=16 KiB`, `FallbackLimit=2 MiB`, `SnapshotLimit=1 MiB`, `AgentInstructionsLimit=4 KiB`. `Redact` replaces `Bearer <...>`→`Bearer ***`, `Basic <...>`→`Basic ***`, `token|secret|api_token|... := <val>`→`***`, `ghp_*`→`***`. `AgentEvidence` composes harness header + 4 KiB-bounded instructions + output, then `VisibleEvidence` (single redact+bound). `CommandEvidence` does the same for command steps. `FallbackEvidence` bounds `join(evidences)` to 2 MiB.
- `store/migrations.go:ensurePayloadColumn` added `attempt_events.payload TEXT` nullable; DDL `spec` §2 declares `payload` as sanitized bounded delta, `payload_ref` as legacy reference. `engine.go` L638-644 writes `payload=&visible` for command, L922-927 for agent. `attempt_transport` holds `adapter_name, native_session_id, protocol_version, extra` (`extra="{}"` today).
- Feedback reconstruction: `PriorOutputDelta` is selected from prior attempt's inline payload first (L586-593), falling back to file read, then `FallbackEvidence(prior+delimited)` with header `---FEEDBACK---`.

## Affected Areas

- `internal/execution/engine.go` — must call real adapter per harness; fix `RunFallback`/`execute` embedding of real output, correct `Requires` map population, ensure per-candidate `Generate+Attempt+Transport` semantics remain, propagate real `native_session_id` from harness.
- `internal/cmd/execute.go` — add engine→adapter wiring (construct `Manager` from project config + workflow harness allowlist, call `Initialize` per harness before first `NewSession`, inject via `SetAdapterManager`). Touchpoints: `handleRun` L260, `handleStepRun` L475, `handleStepReopen` L556, `handleStepSkip` L596 (at minimum `handleStepRun`; others for completeness). Also consider `NewEngine` signature vs setter.
- `internal/project/config.go` — extend `.haro/config.yaml` shape (harnesses map + per-harness binary/env/timeout) or keep minimal harness-agnostic pointer; impacts `LoadConfig`.
- `internal-workflow/parse.go` + `validate.go` — no core change needed for agentic wiring, but config shape must remain provider-literal-free in core (constitution II.3 / XIII.3); harness names are data, not switch logic.
- `internal/adapter/*` — ensure each adapter honours `Probe`+`Initialize`+`NewSession` contract, exposes binary resolution, and returns real `Session` that speaks harness subprocess (claude/opencode/acp). Missing `internal/adapter/opencode/adapter.go` and generic harness wiring is the implementation gap.
- `internal/store/{transport.go,migrations.go,fake.go}` — transport row already present; need to ensure `native_session_id` propagation from real session (today synthetic).
- `scripts/verify-adapter-boundary.sh` + `scripts/verify-traceability.sh` + `deltas-acceptance.md` — traceability invariant 92/11 must not be violated; harness name literals already allowlisted in non-test boundary check via description (fallback/workflow exclusion).
- `internal/execution/evidence.go` + `fallback.go` — already correct; wiring must not duplicate redaction/bounding.

## Approaches

### 1. A — CLI-direct wiring (recommended) — Wire `adapter.Manager` into the CLI runtime, harness config in `.haro/config.yaml` (+ workflow fallback ordering)

- **Description**: Keep the repo posture (CLI-direct, `deferred:cli-direct` allowlist for `v2-broker`/`v2-ipc`). In `internal/cmd/execute.go` build a `map[string]adapter.Adapter` from config (`opencode`→`opencode.NewAdapter(binary)`, `claude`→`claude.NewAdapter(binary)`, `acp-generic` if present) and call `adapter.NewManager(m)`. Call `Initialize` once per harness (core caps `ProtocolVersion:1, Permission:true`) before the first attempt that needs it, then `SetAdapterManager(mgr)` on the engine just after `NewEngine`. Engine's existing `if e.adapterMgr!=nil` branch does the rest — simulated branch becomes fallback-only when no manager or harness unavailable. Config lives in `.haro/config.yaml` under a generic key (e.g. `harnesses:`) with per-harness `binary?: string`, `env?: map[string]string`, `timeout_seconds?: int`, and optional `enabled: bool`. Workflow `steps[].harness` retains fallback order; engine resolves it against config. No broker process, no JSON-RPC IPC, no Unix socket.

- Pros:
  - Matches constitution VII (CLI↔Broker is *future* IPC; III.2 `no network except local bridge to harness`), technical spec §6 (`deferred:cli-direct` exists precisely for this), and archived `2026-08-30-v2-adapter` design which explicitly deferred broker to `v2-broker`.
  - Minimal touch surface: ~1 new config struct + 1 factory function + 2-3 call sites in `internal/cmd/execute.go`; engine already has real branch.
  - Preserves fail-closed (`gatedHost`), bounded evidence, generation/attempt atomicity, and path-claims without new concurrency model.
  - Does not touch traceability: `v2-evidencia` pattern (add no acceptance ID) is deliberately followed so `TestFlowCriterionTraceability` (92 criteria / 11 specs, 4 strict D09 tests) stays green.
  - Unblocks `v2-no-regresion/F-04` with a real E2E harness cycle under Strict TDD without bootstrapping a daemon.

- Cons:
  - No concurrent broker semantics (VII.2 `multiple concurrent active sessions` requires broker); CLI-direct serialises per `step run`. Acceptable because F-04 does not require concurrency.
  - Each CLI invocation initializes adapters fresh (cold start) — no persistent sessions.

- Effort: Low (config type + factory + wiring + tests), after adapters actually spawn subprocesses.

### 2. B — Broker now (implement v2-broker + v2-ipc eagerly)

- **Description**: Implement `v2-broker` (persistent per-project daemon, lazy startup, Unix socket) + `v2-ipc` (§6 JSON-RPC `execution.start`, `step.run`, `step.events`, `step.approve`, `step.cancel`, `step.reopen` + notifications `step.status_changed`, `step.interaction_required`), then run agent steps through the broker where `adapter.Manager` lives. This is the normative VII architecture.

- Pros:
  - Unlocks VII.2 concurrent sessions, lease semantics (VI), and true `step.approve` idempotency path.
  - Aligns with long-term IPC spec (§6) rather than CLI-direct stepping stone.

- Cons:
  - Very large surface: daemon lifecycle (pidfile/socket, lazy startup, `SO_REUSEADDR` race, cleanup), JSON-RPC codec, Store concurrency across CLI + broker (WAL ok but needs fencing), signal handling, `LeasesRepository` integration now required, plus all of Approach A anyway (adapter still needed behind broker). Risk of breaking `Strict TDD` budget and the 400-line guard.
  - Breaks the repo's stated posture: `verify-traceability.sh` explicitly allowlists `v2-broker`/`v2-ipc` as `deferred:cli-direct` — starting them now without SDD `propose→spec→design→tasks` would violate the planned deferral and force reclassification of 22 allowlisted criteria prematurely.
  - F-04 does not need a broker; adding one now couples an unrelated P0 (fallback) to two deferred specs (D01+D02).

- Effort: High (broker + IPC + race-hardening + contract tests).

### 3. C — Hybrid (CLI-direct for F-04, broker behind a flag)

- **Description**: Implement A for F-04, plus scaffold broker/IPC behind `--broker` / env flag or `config.harnesses.broker: true` without making it the default path. Shared `internal/adapter` + `internal/store` remain single-source.

- Pros:
  - Keeps F-04 unblocked while starting broker incrementally.

- Cons:
  - Dual runtime paths to test and maintain; feature-flag branching in `Engine` and `handleStepRun` doubles verification cost. Traceability either claims broker criteria or leaves them deferred — flag makes classification ambiguous and risks `TestFlowCriterionTraceability` drift. Strict TDD guard expects one focused E2E fixture shape; two modes require two.

- Effort: Medium-High, with flag-debt that must later be removed.

## Recommendation

**Approach A — CLI-direct wiring** — is the only posture-consistent choice.

- The constitution and technical spec normatively describe CLI↔Broker JSON-RPC (VII / §6) but the repository **deliberately defers** it (`verify-traceability.sh: ALLOWED_STATUS[v2-broker]=deferred:cli-direct`, same for `v2-ipc`, 22 criteria). Archived design docs (`2026-08-30-v2-adapter/design.md` and `exploration.md`) explicitly state `No broker daemon, no socket` for CLI-direct and `Owner is Engine via adapter.Manager (lifecycle per CLI invocation)`. `v2-evidencia` demonstrated that evidence realness can be shipped without adding an acceptance ID — preserving the 92/11 invariant that adding IDs to `deltas-acceptance.md` would break. F-04 (“Headless agent cycle with fallback” [E2E] P0) needs only `probe→initialize→NewSession→Prompt→finalize` + harness fallback without semantic classification — all of which already exists behind the `if e.adapterMgr!=nil` guard. Wiring the manager at the two engine construction sites satisfies it with the smallest blast radius and the strongest alignment to fail-closed, bounded evidence, and provider-literal containment (only `internal/adapter/*` imports harness literals).

Tension noted: §7 says `SessionHost is implemented by the broker`; under CLI-direct the host is `Engine` (`agentHost` L962). This is an intended temporary divergence allowed by the `deferred:cli-direct` allowlist; the `SessionHost` interface is host-agnostic and `gatedHost` preserves fail-closed semantics, so no spec violation arises.

## Risks

- **Binary availability & `Probe` semantics**: `claude/Probe` uses `exec.LookPath`+`Stat`; `Probe` returning `Available:false` is not an error but must suppress that harness candidate (clean failure, not terminal). Misclassifying unavailable as terminal breaks fallback. Mitigate by having factory return adapters for all configured harnesses and letting engine treat `NewSession(ErrNotInitialized)` or `Probe==false` as clean → next candidate.

- **Provider-literal containment gate** (`scripts/verify-adapter-boundary.sh`): only `internal/adapter/*` may contain literals `opencode`/`claude`/`anthropic`/`api_key`. Harness names in `workflow/parse.go` and `execution/fallback.go` are allowlisted as data, but any new config key containing provider names in core (`internal/cmd`, `internal/project`, `internal/store`) would trip the grep/import gate. Keep config generic (`harnesses: map[string]HarnessConfig`).

- **Session cleanup & timeouts**: `Session.Prompt` returns a channel; engine calls `Cancel` after collection. Real harnesses must be `exec.CommandContext`-spawned with fixed argv, no shell, 300s ceiling (matching `RealRunner`), and context-cancellable. Leaked subprocesses break leases and path claims. Add `Cancel` idempotency + `context.WithTimeout` per attempt; record `ended_at` even on cancellation.

- **Redaction & boundedness gaps**: new adapter output may introduce credential formats not covered by `Redact` (e.g. `sk-...`). Evidence must remain `Redact→VisibleEvidence(16 KiB)` exactly once before `CreateAttemptEvent`. Never log raw `Payload` outside `Payload`/`attempt_transport.extra`.

- **Fail-closed when unconfigured**: if `.haro/config.yaml` has no `harnesses` entry and workflow lists `harness: [opencode,claude]`, the engine must fail closed with `no harness candidates` or `exhausted candidates` → `failed` + `logical_conflict`-style structured error, not silent success via simulated branch. The simulated branch must be gated to test-only (`HARO_TEST_*` or `adapterMgr==nil && isTestBuild`) or removed once wiring lands.

- **Strict TDD & harness fixtures**: `go test ./...` must stay green without real binaries. Preserve `contract/suite.go` + synthetic `testdata/fixtures` + `HARO_TEST_*_BINARY` env (as in `claude/adapter.go:28`) and make real-binary E2E `t.Skip` when unavailable. `v2-adapter` precedent: synthetic fixtures are neutral and never provider literals outside adapter.

- **Traceability invariant 92/11**: `TestFlowCriterionTraceability` in `internal/cmd/flow_test.go` counts 92 criteria across 11 specs and enforces `TOTAL==92 && STRICT==4`. Adding any heading `### F-...`/`U-...` or `Spec: v2-...` to `deltas-acceptance.md` increments the count and fails. `v2-evidencia` avoided this by shipping without an acceptance ID; `v2-agent-wiring` should do the same. Claiming `F-04` is informational (`archived-pending` → `strict` only when the Go test exists and traceability table is updated atomically) — do not update the deltas file until the SDD `archive` phase re-tables 92→93 with spec count bump, which requires a deliberate proposal.

- **Worktree & path-claim interaction**: `effectiveWorktreeRoot` resolves per-execution worktree; adapters must run with `WorkspaceRoot` as cwd. Claims are acquired before `pending→running` and released on defer; harness file writes outside `requires`/`produces` bypass claim checks — document and keep acyclic.

## Ready for Proposal

Yes — proceed to `sdd-propose` for `v2-agent-wiring` as a CLI-direct slice (Approach A). Proposer should:

- State change name `v2-agent-wiring`, type `feat`, scope `internal/cmd, internal/project, internal/adapter, internal/execution`.
- Reference constitution II.3/V.3/V.4/VII, tech-spec §4/§5/§7, and traceability allowlist `deferred:cli-direct` to justify deferring `v2-broker`/`v2-ipc`.
- Scope proposal to: add `HarnessConfig` to `.haro/config.yaml`, a factory `NewManagerFromConfig(root) (*adapter.Manager,error)` (or `internal/adapter/factory.go`), wire in `handleStepRun` (and optionally `handleRun` path for `run` that auto-runs first step), populate `SessionBundle.Requires` from `requires` artifacts, propagate real `native_session_id` into `attempt_transport`, and cover with `contract/suite`-backed tests plus one E2E fallback test (`F-04`) that asserts `output_delta.payload` contains redacted real harness output and `attempt_transport.adapter_name` reflects the winning candidate, with last-wins semantics.
- Explicitly keep out of scope: `v2-broker`, `v2-ipc` JSON-RPC, PTY/terminal, broker-hosted `SessionHost` move (later).
- Flag the 92/11 invariant and adapter-boundary gate as hard constraints in the proposal's rollout checklist.

---
*Evidence pins*: `engine.go:691-960` (simulated branch L908-915, real branch L862-907, `AgentEvidence` L919/79-87, transport L817-825), `cmd/execute.go:260,475` (wiring points), `adapter/{manager.go,adapter.go,claude/adapter.go}` (surface), `project/config.go:11-14` (no harness), `workflow/parse.go:51-53` + `validate.go:170-216` (harness schema), `store/migrations.go:88-93,148-154,230-264` (transport+payload), `execution/evidence.go:9-87` (budgets+redaction), `execution/fallback.go:45-73` (terminal classification), `verify-traceability.sh:ALLOWED_STATUS[deferred:cli-direct]`, `verify-adapter-boundary.sh` (containment).
