# Exploration: v2-adapter (v2 adapter contract and multi-harness)

## Current State

**Project is pure-Go CLI-direct (no broker) after v2-no-regresion.** The orchestrator is `internal/execution.Engine` talking directly to `store.Store` (7-table SQLite via `modernc.org/sqlite`) and `CommandRunner` for `command` steps. `v2-broker`/`v2-ipc` are explicitly later changes; `deltas-acceptance.md` carry-forward notes defer F-06, F-08, F-09, agent work, and PTY.

**What exists today relevant to v2-adapter:**

- `internal/adapter/capabilities.go` — spike `Capabilities` struct (`ProtocolVersion`, `Permission`, `Terminal`, `LoadSession`, `Extra map[string]any` with JSON tags `protocolVersion` etc.). Roundtrip test only. No `Adapter` interface, no `Session`, no `Probe`/`Initialize`/`NewSession` logic.
- `internal/ipc/health.go` — spike JSON-RPC health probe (`health` method over Unix socket via `encoding/json` + `net.Conn`). Demonstrates stdlib framing with `json.Decoder`/`Encoder` per connection, with unit tests for roundtrip/decode error/unsupported method. Not a full JSON-RPC 2.0 implementation (no `id` string support, no notifications, no error object, no size limiting, no concurrent handling).
- `internal/store/migrations.go` — 7-table DDL: `projects`, `executions`, `execution_steps`, `generations`, `attempts`, `attempt_events`, `step_transition_events`. `attempts` is already transport-agnostic (no provider columns) — compliant with U-04 shape — but `attempt_transport`, `leases`, `interactions`, `path_claims` are missing (owned by `v2-store` per spec F-02 which expects 11 tables). `attempts` DDL: `id, execution_id, step_id, generation_id, status CHECK(running|completed|failed|cancelled), started_at TEXT, ended_at, termination_reason, result_digest`, FK to `execution_steps`.
- `internal/store/store.go` — `Store` facade with `Projects/Executions/Steps/Attempts/Generations/Events` + `WithTx`. No `AttemptTransport`, `Leases`, `PathClaims`, `Interactions` repos yet.
- `internal/execution/engine.go` — `Engine` creates executions, validates workflows, runs `command` steps via `CommandRunner` with evidence budgets (`VisibleLimit 16 KiB`, `SnapshotLimit 1 MiB`, `FallbackLimit 2 MiB`), redaction, feedback delimited with bounded prior context, generation/attempt lifecycle, and `resyncExecution`. Agent steps currently rejected (`unsupported_step_type`). `state.go` handles `ReopenStep`/`SkipStep` and idempotent transitions.
- `internal/execution/evidence.go` — `Redact`, `VisibleEvidence`, `SnapshotBytes`, `FallbackEvidence` already implement the budgets required by F-12/U-02 side; reusable for adapter fallback evidence.
- `internal/workflow/parse.go`, `validate.go` — parses `version: 2` YAML with `steps: [{id,type,run,env,depends_on,requires,produces}]`. No `harness`/`instructions`/`mode`/`source`/`bindings`/`inputs`/`outputs`/`workspace` fields yet; validation does DAG cycles/entry-point but not per-type schema (command vs agent vs workflow). `path.go` handles containment for `requires`/`produces`.

**Constitution/Tech-spec context:**
- Constitution II.3 / XIII.3: core never knows provider literals; V.1-V.4 define mandatory/optional adapter methods and capability rule; VII.4 says adapter chooses transport (stdio-RPC, HTTP+SSE, JSONL) and negotiates in `initialize`.
- Tech-spec §4 defines `adapter.Capabilities`, `Adapter` (`Probe`, `Initialize`, `NewSession`), `Session` (`Prompt`, `Cancel`, optional `LoadPrevious`, `Terminal`), `SessionHost.RequestPermission`, `SessionBundle`, `PromptInput`, `SessionEvent`. §2 defines `attempt_transport` extension table. §6/§7 define JSON-RPC CLI↔Broker and Broker↔Adapter (ACP-based) method tables.

**No prior v2-adapter artifacts exist** — `openspec/changes/v2-adapter/` was empty before this exploration; no `adapters/` directory on disk; `grep -R opencode|claude internal --include=*.go` is empty per archive-report.

---

## Affected Areas

- `internal/adapter/*` — Core contract: add `adapter.go` (Adapter/Session/SessionHost interfaces), `negotiation.go` (capability validation, bijective translation), `manager.go` (lifecycle owner for CLI-direct), `jsonrpc/` codec (stdlib framing), `acp/` generic adapter, `opencode/` and `claude/` subpackages. Keep `capabilities.go`.
- `internal/ipc/*` — Extend or add `internal/ipc/jsonrpc/` (or `internal/adapter/jsonrpc`) for full JSON-RPC 2.0 codec; `health.go` pattern is reused but hardened. CLI↔Broker socket handling stays in `v2-broker`/`v2-ipc`; no change needed now except codec reuse.
- `internal/store/migrations.go` + `repositories.go` + `store.go` — Add `attempt_transport` table and `TransportRepository`; expose via `Store.Transport()`; wire WAL/FK pragmas unchanged. Touches `v2-store` boundary — migration must be idempotent and not couple to leases/path_claims.
- `internal/execution/engine.go` + `runner.go` + `evidence.go` + new `fallback.go` — Add agent-step path: harness candidate iteration, `AgentSpec` wiring, fallback without semantic classification, bounded accumulated context (≤2 MiB), sanitized evidence aggregation. `CommandRunner` stays for `command` steps; agent path delegates to `adapter.Manager`.
- `internal/workflow/parse.go` + `validate.go` — Extend YAML schema for `agent` steps (`harness: []string`, `instructions`, `mode`) and `workflow` steps (`source`, `bindings`, `inputs`/`outputs`). Validation adds per-type conditionals (zod-equivalent logic in Go).
- `internal/cmd/*` + `main.go` — Minimal `step run` wiring to pass through `harness` fallback; no new CLI commands.
- `testdata/` + `internal/adapter/contract/` + `internal/adapter/acp/testdata/` — New first-class contract test suite (U-02) and protocol fixtures (U-03); `testdata/fixtures/{acp,opencode,claude}/` for synthetic neutral fixtures.
- `scripts/verify-adapter-boundary.sh` — New verification script for F-04 containment (grep + import graph).
- `go.mod` — No new `cgo`; may add `github.com/google/uuid` already present via indirect; no new deps required (stdlib JSON-RPC).

---

## Approaches

### 1. CLI-direct AdapterManager (recommended slice) — Adapter lifecycle owned by Engine in-process

**Description:** `Engine` owns a `Manager` that `Probe`s available adapters, calls `Initialize` once per adapter subprocess, then `NewSession` per attempt. For CLI-direct, an adapter subprocess lifetime = one attempt (spawned via `exec.CommandContext` with stdio pipes). No broker daemon, no socket. `SessionHost.RequestPermission` is implemented by `Engine` (fail-closed if capability not negotiated). Fallback loop lives in `Engine` (iterates `harness` candidates), carrying forward bounded prior evidence (≤2 MiB) to the next candidate via `SessionBundle` + `PromptInput`. Codec is newline-delimited JSON over stdio using `encoding/json` + `bufio.Scanner` with max size.

- Pros:
  - Fits current architecture without premature broker; respects `v2-no-regresion` decision that minimal Store replaces §6 Broker/JSON-RPC for this slice (archive-report line 21).
  - Testable with `FakeRunner`/`FakeAdapter` pattern already used (`execution.FakeRunner`) — table-driven, `t.TempDir`, skippable integration.
  - U-04 `attempt_transport` insert happens transactionally alongside `attempts` in Engine (no cross-process coordination).
  - Capability negotiation logic (F-01/F-02/F-03/U-01) is unit-testable without sockets.
- Cons:
  - Adapter subprocess management duplicates some logic later needed in broker (`v2-broker` will need to re-home lifecycle ownership — requires clear handoff).
  - Stdio subprocess handling must handle cancellation (`session/cancel`) and reverse call (`session/request_permission`) multiplexing.
- Effort: Medium (new interfaces + manager + codec + fallback + store extension, but no daemon/UDS work)

### 2. Introduce Broker stub now, own lifecycle in broker package (called inline)

**Description:** Create `internal/broker/` now that manages adapters, but invoke it synchronously inside the CLI process (no socket). Broker owns `Initialize`/`Session` and exposes a Go method API that `Engine` calls.

- Pros: Closer to final architecture; less re-home churn.
- Cons: Premature — couples `v2-adapter` to `v2-broker`/`v2-ipc` which are separate SDD changes with their own specs (F-01..F-07, U-01/U-02). Risks building half a broker without full event model (`attempt_events`, `step_transition_events` cursor guarantees, notifications) and fencing. Higher reviewer load (>400 lines + cross-cutting).
- Effort: High

### 3. Adapter-per-process with no Manager (ad-hoc spawning in Engine)

**Description:** `Engine` directly spawns harness binaries without an `Adapter` abstraction — ad-hoc JSON parsing per harness inline.

- Pros: Minimal abstraction.
- Cons: Violates constitution II.3/V.4 containment (provider literals leak into core), fails F-04 verification, no reusable contract suite (U-02), no bijective capability translation, no fallback policy. Non-viable.
- Effort: Low initial, high debt

**Decision matrix:**

| Approach | Pros | Cons | Complexity |
|----------|------|------|------------|
| 1 — CLI-direct Manager | Fits current slice, fail-closed, testable, minimal deps | Lifecycle re-home later | Medium |
| 2 — Broker stub | Future-proof | Premature coupling, scope bleed into v2-broker/v2-ipc | High |
| 3 — Ad-hoc no abstraction | Simple | Leaks provider literals, fails F-04/U-02 | Low (debt) |

---

## Recommendation

**Adopt Approach 1 (CLI-direct AdapterManager) as the v2-adapter slice.**

**Rationale:**
- The repo's authoritative deferral (spec `v2-no-regresion` Non-Requirements, archive-report) states broker JSON-RPC, full DDL, claims, composition, reporting belong to later changes. The current CLI-direct architecture is intentional, not a gap.
- Constitution V.3.2/V.3.3/U-01 requires capability-gated invocation and additive capabilities without major bump — this logic belongs to a `negotiation.go` unit-testable module, independent of broker sockets.
- F-06 fallback without semantic classification is an Engine concern (ordered candidates, clean vs terminal error distinction) — locating it in `Engine` with `adapter.Manager` as factory keeps evidence budgets co-located with `evidence.go`.

**Concrete recommendations per exploration question:**

1. **What to keep/replace in spike packages:**
   - Keep `Capabilities` struct as-is; add helpers `Negotiate`, `Validate`, `Supports(method)`.
   - Keep `health.go` pattern for stdio framing but add `internal/ipc/jsonrpc` (or `internal/adapter/jsonrpc`) implementing full JSON-RPC 2.0: `Request{jsonrpc:"2.0", id, method, params}`, `Response{result, error:{code,message,data}}`, `Notification`, size limit (e.g. 10 MiB), handling of `id` as string|int|null, and streaming `Decoder` with `UseNumber`. Hardening: reject malformed/oversized without breaking connection facility (the framing layer returns structured error, connection stays usable for next message — relevant for future broker socket).
   - No modification to `health.go` behavior; new codec is additive.

2. **Lifecycle slicing (who owns initialize/session):**
   - Owner is `Engine` via `adapter.Manager` (lifecycle per CLI invocation). `Manager.Probe(ctx)` returns `ProbeResult`. `Manager.Initialize(ctx, coreCaps)` negotiates once per adapter binary path (cached per process). `Manager.NewSession(ctx, bundle, host)` creates session handle; `Session.Prompt` returns `<-chan SessionEvent`; `Session.Cancel` terminates harness. Revocation on `v2-broker`: Manager moves into broker process; Engine becomes broker client via `v2-ipc` — interface stays stable.

3. **JSON-RPC 2.0 + ACP mapping (U-03):**
   - Stdlib-only: `encoding/json` + `bufio` + `net`/`os/exec` pipes. Framing: newline-delimited JSON (NDJSON) over stdio — one JSON value per line, `Decoder.Buffered()` safe; alternative Content-Length framing deferred unless harness requires it. Enforce max via `io.LimitedReader`.
   - Methods (from tech-spec §7): `initialize`, `session/new`, `session/prompt`, `session/update` (harness→core notification), `session/cancel`, `session/request_permission` (reverse), optional `session/load`, `terminal/*` deferred.
   - Bijective capability translation: `toACP(Capabilities) -> map[string]any` and `fromACP(map) -> Capabilities` preserving `_`-prefixed keys verbatim; tested via roundtrip table (empty/partial/complete/future additive).
   - Fixture location: `internal/adapter/acp/testdata/*.json` (protocol sequences) and `testdata/fixtures/acp/*.json` (public). Suite: `internal/adapter/acp/acp_test.go` with `testing.Short()` for fixture-only fast path.

4. **Provider-literal containment (F-04):**
   - Boundary is `internal/adapter/` (with subpackages `opencode/`, `claude/`, `acp/`). Top-level `adapters/` is not used in Go layout; `deltas-acceptance.md` reference to `adapters/` is conceptual (TS layout) — Go adaptation is `internal/adapter/*` (already present). Document this mapping in proposal/design.
   - Verification: `scripts/verify-adapter-boundary.sh` — fails if `grep -R -i "opencode|claude|anthropic|api_key|provider"` matches outside `internal/adapter` + deny list check on `go list -json ./...` imports.

5. **Harness fallback (F-06):**
   - Lives in `Engine.RunAgentStep` (new) + `adapter.Manager`. Policy: iterate `harness: [opencode, claudecode]` in order; on clean error (exit≠0, no terminal signal) try next; on terminal (`permission_required`, `process_start_failed`, capability violation) fail immediately with no fallback. Bounded accumulated context: `FallbackBytes` concatenation of prior candidates' sanitized visible evidence (each ≤16 KiB) trampolined into next `SessionBundle.Instructions` / `PromptInput.Text` with overhead, capped at 2 MiB. Each attempt persists `attempt_transport` row (adapter name, version). When exhausted: step `failed` with `terminationReason` containing sanitized evidence refs from all candidates (joined, redacted).

6. **Claude adapter + contract suite (F-05/U-02):**
   - Real binary: `HARO_TEST_CLAUDE_BINARY=claude` (or `CLAUDE_CODE_BIN`) when present; otherwise fixtures. CI: fixtures only with `testing.Short()` or env skip — justified because `claude` binary is not hermetic/pinned in CI (docs policy + network-free init requirement `v2-distribucion/F-03`).
   - Suite is `internal/adapter/contract/suite.go` — generic harness-agnostic checks: F-01 single initialize, F-02 optional not invoked, F-03 request_permission fail-closed, fallback smoke. Adapters implement `contract.AdapterFactory` interface.
   - Fixture provenance: original neutral OpenCode 1.17.18 JSONL not available in repo (deferred via `v2-no-regresion` archive-report U-02 note). Propose **synthetic neutral fixtures** in `testdata/fixtures/synthetic/` with README provenance: shapes derived from tech-spec §7 + ACP spec, JSON lines with `cursor`, `delta`, `payload_ref` analogues, versioned `v2.0.0-synthetic.1`, not claiming byte-identity. Justification is explicit and auditable.

7. **Attempts schema (U-04):**
   - Current `attempts` DDL is correct — no transport fields. Add migration idempotently: `CREATE TABLE IF NOT EXISTS attempt_transport (attempt_id TEXT PRIMARY KEY REFERENCES attempts(id), adapter_name TEXT NOT NULL, native_session_id TEXT, protocol_version INTEGER, extra TEXT NOT NULL DEFAULT '{}')`. Add `TransportRepository` with `Put/Get` and wire `Store.Transport()`. `attempts` insertion and `attempt_transport` insertion happen in same `WithTx` (Engine). Verification: schema test `INSERT attempt without transport` succeeds; `INSERT attempt_transport` requires valid attempt id; native fields query on `attempts` returns no column.

8. **Dependencies/risks (scope boundaries vs other specs):**
   - Blocks on nothing for core contract slice; `v2-store` is co-owned for `attempt_transport` but scoped narrowly (just one table, no leases/interactions/path_claims).
   - `v2-broker`/`v2-ipc` details (socket path derivation, concurrent sessions, cursor persistence before visibility U-01, interaction CAS U-02, payload_ref U-03, JSON-RPC validation U-04) are out of scope and must not be built here.
   - PTY (`terminal/*`) stays deferred per archive-report and constitution XII.1 — adapter declares `Terminal: false` in this slice.
   - Broker IPC payload validation (U-04 zod-equivalent) reuses workflow `validate.go` pattern but broker↔adapter validation belongs to adapter layer.

---

## Boundary Decisions (v2-adapter vs siblings)

| Decision | Owner | This change |
|----------|-------|-------------|
| `attempt_transport` table + repo | `v2-store` F-02 expects it, but `v2-adapter` U-04 requires it | **Include narrowly** (single table, no other v2-store tables) |
| `leases` / `interactions` / `path_claims` | `v2-store` + `v2-broker` | Exclude — no code, no migration |
| Broker daemon, UDS socket, lazy startup | `v2-broker` | Exclude — note future home |
| CLI↔Broker `execution.start`, `step.events` etc. | `v2-ipc` | Exclude — health probe is only IPC evidence |
| PTY (`terminal/*`) | Deferred (SPEC 9c preserved) | Exclude — `Terminal` capability false |
| OpenCode JSONL debt (headless parser) | F-04 | **Include** — new `internal/adapter/opencode/parser.go` with NDJSON streaming |
| Claude real-binary suite | F-05 | Include as skippable suite + fixtures |

---

## Deferred Items (explicitly NOT in v2-adapter)

- F-10 PTY/terminal human attach (no `internal/pty` work)
- Multi-user concurrency, interaction bundle CAS (deferred per deltas footer)
- Full 11-table DDL (`leases`, `interactions`, `path_claims` beyond `attempt_transport`)
- Broker death/fencing, cursor-prior-persistence (v2-broker/v2-ipc U-01)
- Workflow composition (`v2-composicion`), path claims (`v2-path-claims`), reporting (`v2-reporte`), distribution (`v2-distribucion`)

---

## Risks

- **CRITICAL — Double migration ownership:** Both `v2-adapter` and `v2-store` want `attempt_transport`. Must decide single owner (recommend adapter owns it narrowly) or coordinate via sequenced migrations with no duplication — otherwise schema drift. Mitigate by documenting ownership and making migration `CREATE TABLE IF NOT EXISTS` + idempotent test.
- **Provider literal leak regression:** Any string literal `"opencode"` outside `internal/adapter` fails F-04. Risk is accidental `workflow` parsing that enumerates harness names. Mitigate with CI script in `golangci-lint` or `go vet` custom analyzer; add `grep` gate to CI workflow.
- **Log/evidence bypass:** Fallback evidence concatenation could unintentionally carry credentials if `Redact` not applied per-candidate before accumulation. Must apply `VisibleEvidence` (which calls `Redact`) before `FallbackBytes` accumulation.
- **JSON-RPC DoS:** Stdio decoder without size limit allows unbounded allocation. Must enforce limit (e.g. 10 MiB) and reject oversize with `code: -32600` without killing adapter process.
- **Fixture trust:** Synthetic fixtures not byte-identical to unavailable originals could mask real harness behavior. Mitigate by documenting provenance, keeping fixtures minimal and neutral, and making real-binary path opt-in with `HARO_TEST_*_BINARY` env (skipped in `-short`).
- **Capability additive drift:** Adding `Extra` keys must not require major version bump; need table test covering future additive unknown keys preserved roundtrip.
- **Strict TDD compliance:** Adapter contract has no failing test yet; going forward must use RED→GREEN→TRIANGULATE with `t.TempDir` and `FakeAdapter` — helpers per `go-testing` skill (table-driven, skippable integration).

---

## Ready for Proposal

Yes — ready for `sdd-propose` for `v2-adapter`.

**Proposal must include:**
- Intent: close JSONL parser debt, expose transport-agnostic adapter contract with capability negotiation, bijective ACP translation, harness fallback, and Claude+ACP adapters behind containment.
- Non-goals: broker daemon/UDS, full 11-table DDL, PTY, composition/claims/reporting.
- Scope slice table vs `v2-broker`/`v2-ipc`/`v2-store`.
- Risk and mitigation: migration ownership, provider-literal script, evidence redaction in fallback.
- Rollback: migrations are `CREATE IF NOT EXISTS`; adapter code isolated under `internal/adapter` — no core contamination.

---

## References (sources of truth not modified)

- `deltas-acceptance.md` lines 256–297 (v2-adapter F-01..F-06, U-01..U-04)
- `openspec/specs/v2-no-regresion/spec.md` lines 110–113 (deferral notes) + `openspec/changes/archive/2026-08-30-v2-no-regresion/archive-report.md` (U-02 provenance)
- `docs/v2/haro-especificacion-tecnica.md` §§2,4,7 (DDL, Go interfaces, ACP protocol)
- `docs/v2/haro-constitucion.md` §§II.3, V.1-V.4, VII.4, XII.4 (core without literals, capability rule, transport choice, JSONL debt)
- Code inspected: `internal/adapter/capabilities.go`, `internal/ipc/health.go`, `internal/store/migrations.go`, `internal/store/store.go`, `internal/execution/engine.go`, `internal/execution/evidence.go`, `internal/workflow/parse.go`, `internal/execution/state.go`, `internal/cmd/execute.go`, `go.mod`, `openspec/config.yaml`
