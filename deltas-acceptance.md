# Haro — v2 delta acceptance criteria

> Purpose: **verifiable** acceptance criteria for the deltas between Shardeo v1 (Specs 1–9 delivered on `main`) and the project's v2 normative proposal. This document is **self-contained**: it does not depend on external documents or their sections; each criterion fully states the expected behavior.
>
> **Stack decision (locked): Pure Go, no cgo.** SQLite uses modernc.org/sqlite; YAML uses gopkg.in/yaml.v3; ACP remains the adapter protocol standard (see Conventions).
>
> Each criterion is testable at the functional level (real command or integration) or at the unit level. A criterion is **met** only when its described verification passes reproducibly in the SDD flow (see §Usage).

---

## Conventions

- **ID hierarchy — two levels**:

  ```
  id-spec > id-local
  ```

  - `id-spec` identifies the **spec** (a complete delta; it matches the SDD change name). Example: `v2-broker`.
  - `id-local` identifies the criterion **within** that spec: `F-<n>` for **functional** criteria (E2E or integration) and `U-<n>` for **unit tests**. Numbering restarts in each spec.
  - Full reference: `v2-broker/F-01` (spec `v2-broker`, functional criterion 1).

- **Verification types**: `[E2E]` real end-to-end command · `[INT]` integration with controlled fixtures/subprocesses · `[UNIT]` unit test · `[REV]` document review · `[PROC]` process criterion.
- **Priority**: `P0` blocking (without it the delta does not exist) · `P1` important · `P2` desirable.
- **Decided stack — Pure Go, no cgo**: tests: `go test ./...` (`go test ./... -race` in CI); gates: `go build ./...`, `go vet ./...`, golangci-lint, govulncheck.
- **ACP as the Broker↔Adapter protocol standard (explicit requirement)**: the protocol between broker and adapters/harnesses is based on **Agent Client Protocol** (open standard driven by Zed) — JSON-RPC 2.0 over stdio, capability negotiation in `initialize`, and the methods `initialize`, `session/new`, `session/prompt`, `session/update`, `session/cancel` and `session/request_permission` (`terminal/*` deferred). Adapters translate each harness's native protocol to this contract; the generic `acp-generic` adapter implements it directly when the harness already speaks ACP (see `v2-adapter/U-03`).
- **Runtime validation — current repo practice**: schema validation (workflows, config, internal payloads) keeps using **zod** (lowercase, the library), already established in v1 (`src/schema/*`); no JSON Schema or ad-hoc validators are introduced. This is repo practice, independent of the ACP standard.
- **Maintainability**: the resulting code must preserve the v2 proposal rules adapted to TS: core without provider literals, fail-closed, evidence budgets, idempotent migrations, and an ACP-based adapter protocol.
- **Status**: each criterion carries a tracking checkbox; they start as `pending`.

### Specs (deltas)

| id-spec | Delta | Content |
|---|---|---|
| `v2-reconciliacion` | D00 | Reconciliation of the normative documentation |
| `v2-no-regresion` | R00 | Preservation of Specs 1–9 |
| `v2-broker` | D01 | Persistent per-project broker |
| `v2-ipc` | D02 | JSON-RPC IPC CLI↔Broker and event model |
| `v2-adapter` | D03 | v2 adapter contract and multi-harness |
| `v2-path-claims` | D04 | path_claims and workspace isolation |
| `v2-composicion` | D05 | Workflow composition |
| `v2-reporte` | D06 | Change reporting |
| `v2-store` | D07 | Persistence behind a repository interface |
| `v2-distribucion` | D08 | Distribution |
| `v2-flujo-sdd` | D09 | Development flow |

---

## Usage — SDD flow

The development flow **no longer** uses the initiatives pipeline (`.docs/initiatives/`, `tools/scripts/initiative/`). Each spec in this file is implemented as an **SDD change** in the SDD flow with the proposal → spec → design → tasks → apply → verify → archive cycle. The change name is the `id-spec`.

- The criteria in this file are the **source of the acceptance criteria** for each change spec.
- `sdd-verify` (or the equivalent verification) runs the verifications described here with `go test ./...` (`go test ./... -race` in CI); the `[E2E]` ones are public boundary, the `[UNIT]`/`[INT]` ones are package/module tests. Delivery gates: `go build ./...`, `go vet ./...`, golangci-lint, govulncheck.
- A delta is complete only when all its P0 and P1 criteria pass and the change is archived.
- No new initiatives are created; existing ones (e.g. 008) are migrated or explicitly closed.

---

# Spec: v2-reconciliacion — Reconciliation of the normative documentation

The v2 normative documentation must be explicitly updated when contradicted by reality. Before implementing any other spec, it must be reconciled with the current state of the repo. Criteria on that document, verifiable by diff review.

### F-01 — The terminal/PTY surface is no longer listed as deferred work [REV] · P0 · [x]
**Criterion**: The v2 normative documentation recognizes that the `terminal`/PTY surface **is already implemented** (real PTY, human attach, presence) and treats it as an existing surface to preserve, not as deferred work.
**Verification**: Document review: the `terminal`/PTY surface no longer appears in the deferred-work list.

### F-02 — The immutable bundle with manifest is no longer listed as deferred work [REV] · P0 · [x]
**Criterion**: The v2 normative documentation recognizes that the immutable bundle with SHA-256 manifest and admission test **is already implemented** and refers it to no-regression (`v2-no-regresion`), not to deferred work.
**Verification**: Document review: the bundle with SHA-256 manifest no longer appears in the deferred-work list.

### F-03 — The Broker↔Adapter transport is not dictated by the core [REV] · P0 · [x]
**Criterion**: The v2 normative documentation does not dictate the Broker↔Adapter transport: the adapter chooses it according to what its harness natively supports (stdio-RPC, local HTTP + SSE, JSONL), declares it in capabilities and negotiates it in `initialize`. Local HTTP + SSE is **not a "fallback"** but a valid native path when the harness exposes it (real case: OpenCode supervised uses a managed HTTP/SSE server).
**Verification**: Document review: HTTP+SSE is no longer qualified as a "never general" fallback mechanism.

### F-04 — Known debt of the JSONL parser registered [REV] · P1 · [x]
**Criterion**: The v2 normative documentation records the current confined exception: the OpenCode JSONL parser lives in `src/utils/agent.ts` (headless path) and moving it to the adapter is known debt that the `v2-adapter` spec must close.
**Verification**: Document review: there is a known-debt note with the exact reference.

### U-01 — Auditable diff of the normative update [REV] · P1 · [x]
**Criterion**: There is a diff (or changelog) of the update showing exactly which rules changed and why — never a silent edit.
**Verification**: Review of the update diff: only the changes described in F-01..F-04 of this spec, each with its justification.

---

# Spec: v2-no-regresion — Preservation of Specs 1–9

Without a rewrite, these criteria safeguard that the v2 specs do not break tested behavior: the existing suite (~550 cases) must stay green on every change. `[E2E]`/`[INT]` verifications against the system; `[UNIT]` against the current suite.

### F-01 — Idempotent init (Spec 1) [E2E] · P0 · [ ]
**Criterion**: `shardeo init` creates a complete `.shardeo/` in a clean project; run twice it modifies nothing existing and does not fail.
**Verification**: init in an empty temporary directory → expected structure; init again → same state, informative output, exit 0.

### F-02 — Workflow discovery and validation (Spec 2) [E2E] · P0 · [ ]
**Criterion**: `workflows list` and `workflows describe` return structured JSON output; a workflow with invalid YAML appears flagged as invalid in `list` and `describe` reports the concrete validation error.
**Verification**: valid and invalid workflow fixtures in `.shardeo/workflows/` → expected outputs.

### F-03 — Complete command cycle (Spec 3) [E2E] · P0 · [ ]
**Criterion**: `run` + `steps next` + `step run` + `status`: exit 0 with `produces` → automatic `completed`; exit ≠ 0 → `failed` with stdout/stderr; exit 0 without `produces` → `failed` listing the missing artifacts; unsatisfied `depends_on` → explicit error.
**Verification**: test command workflow covering the four cases.

### F-04 — Headless agent cycle with fallback (Spec 4) [E2E] · P0 · [ ]
**Criterion**: headless agent `step run` follows the probe → lease → invoke → finalize cycle; on a clean error it falls back to the next candidate **without semantic classification**; on a terminal error (`permission_required`, `process_start_failed`, etc.) there is no fallback and the step ends `failed`.
**Verification**: controlled test adapters producing success, clean error and terminal error.

### F-05 — Re-execution with feedback (Spec 6) [INT] · P0 · [ ]
**Criterion**: `step run --feedback` delivers the delimited feedback together with the previous partial response; the attempt is recorded as a reconstruction with its history.
**Verification**: fixture with two attempts: the second one receives the full textual feedback and the bounded previous context.

### F-06 — Resume and reconciliation (Spec 7) [INT] · P0 · [ ]
**Criterion**: `resume` reconciles expired attempts, detects missing artifacts (`reconstruction_required`), delivers the previous response as context to rebuild and returns continuation guidance.
**Verification**: execution stopped midway → `resume` → expected states and guidance.

### F-07 — Reopen/skip with generations (Spec 8) [INT] · P0 · [ ]
**Criterion**: `step reopen --cascade` invalidates the current generation (the files remain but stop satisfying `requires`/`complete`) and restarts descendants preserving history; `step skip --reason` only applies to pending steps without attempts; everything lands in `step_transition_events`.
**Verification**: scenario of 3 chained steps; reopening the first one → cascade and audit verified.

### F-08 — Immutable bundle and admission (Spec 9a) [INT] · P0 · [ ]
**Criterion**: each attempt materializes a bundle with manifest (semantic order, roles, bytes + SHA-256); the admission test is mandatory before harness work; context drift → closed failure.
**Verification**: fixture where context changes between freezing and admission → attempt fails with `adapter_contract_error`/drift.

### F-09 — Complete supervised (Spec 9b) [E2E] · P0 · [ ]
**Criterion**: supervisor with lease + fencing, UDS IPC, events with cursor persisted before visible, idempotent CAS interaction, `step approve` only with decisions from `available_decisions`, fail-closed permission policy (unknown → rejection).
**Verification**: simulated harness requesting permission → `step events` → `step approve` → unique resolution; re-sending the same resolution does not duplicate effects.

### F-10 — Terminal and human attach (Spec 9c) [E2E] · P0 · [ ]
**Criterion**: `step run --mode terminal` creates an attachable PTY and returns `awaiting_human` + attach command; detach does not kill the child; unlimited reattach within `human_presence_seconds`; the orchestrator never types keys nor interprets the screen.
**Verification**: real PTY with test harness; attach/detach/reattach and shutdown.

### F-11 — Adapter-native permission detection [INT] · P1 · [ ]
**Criterion**: permission-request detection works with the versioned fixtures (OpenCode 1.17.18) and the live bridge (1.18.x); ambiguous or unknown evidence → closed failure, never guessing.
**Verification**: existing fixture suite ported without behavioral changes.

### F-12 — Evidence budgets and sanitization [INT] · P0 · [ ]
**Criterion**: visible projections/evidence ≤ 16 KiB; `DiagnosticRaw` ≤ 1 MiB (raw-byte authority, with prefix+suffix+SHA-256 if exceeded); snapshots ≤ 1 MiB; fallback context ≤ 2 MiB; credential redaction by patterns in all projected output.
**Verification**: fixtures with giant outputs and with credentials (Bearer, Basic, tokens) → bounded and redacted projections.

### F-13 — Path containment [INT] · P0 · [ ]
**Criterion**: rejection of absolute paths, `..` and symlink escapes in workflows, instructions, skills, artifacts and bundles; internal symlinks allowed only if their real target stays inside.
**Verification**: table of malicious paths against the `validateContainedPath` equivalent.

### F-14 — CLI output contract [E2E] · P0 · [ ]
**Criterion**: all command output is structured JSON; errors `{error, code}` with exit 1; EPIPE-safe output (exit 0 if the consumer closes the pipe); extra positional arguments rejected respecting `--`.
**Verification**: script consuming with closed pipe and passing extra arguments.

### U-01 — Current suite green [UNIT] · P0 · [ ]
**Criterion**: the existing suite (~550 cases, 74 suites) passes fully in `npm test` and `npm run typecheck` stays clean; both stay green after each v2 spec.
**Verification**: `npm test` and `npm run typecheck` green.

### U-02 — Reused neutral fixtures [UNIT] · P0 · [ ]
**Criterion**: the OpenCode v1.17.18 JSONL fixtures and the v1.18.16 simulated server are reused as-is (language-neutral format).
**Verification**: the same `test/fixtures/` files referenced by the new stack's tests.

### U-03 — State machine with current coverage [UNIT] · P0 · [ ]
**Criterion**: atomic transitions (claim, finalize, reopen, skip), resolution idempotency and lease fencing keep their unit coverage (today in `src/db/queries.ts`) and are extended with the v2 changes.
**Verification**: transition table covered case by case, green.

---

# Spec: v2-broker — Persistent per-project broker

### F-01 — Single broker per project [E2E] · P0 · [ ]
**Criterion**: there is one broker per project identified by the project's canonical path; CLI↔Broker over Unix socket (or named pipe on Windows).
**Verification**: `execution.start` from the CLI against a live broker responds over the socket; two invocations from the same repo use the same broker.

### F-02 — Lazy startup [E2E] · P0 · [ ]
**Criterion**: if the socket does not respond, the CLI starts the broker and retries; the broker outlives the CLI invocation that started it.
**Verification**: kill the broker → first CLI invocation relaunches it and completes its operation; the broker process stays alive after the CLI exits.

### F-03 — Concurrent sessions [E2E] · P0 · [ ]
**Criterion**: one broker holds multiple active sessions: two parallel executions (same or different workflow, same project) progress without state interference.
**Verification**: launch two simultaneous executions; both complete and each persists its own state without stepping on the other.

### F-04 — Independent brokers per project [E2E] · P1 · [ ]
**Criterion**: different projects have completely independent brokers, with no coordination between them.
**Verification**: simultaneous operations in two different repos share no socket or state; stopping one does not affect the other.

### F-05 — No broker duplication [E2E] · P1 · [ ]
**Criterion**: a second CLI invocation with a live socket finds the existing broker and does not launch another one.
**Verification**: broker process count before/after N invocations → 1.

### F-06 — Broker death and fencing [E2E] · P0 · [ ]
**Criterion**: if the broker dies with active sessions, leases expire and no write after expiration is valid even if the original process is still alive; `resume` detects the state and guides recovery.
**Verification**: kill the broker mid-attempt → attempts expired; write retry with old fencing token → rejected.

### F-07 — Clean shutdown [E2E] · P1 · [ ]
**Criterion**: the broker stops cleanly on termination signal: releases leases and sockets; leaves no zombie sockets blocking the next startup.
**Verification**: TERM signal → process exits, socket removed, immediate subsequent startup works.

### U-01 — Socket path derivation [UNIT] · P1 · [ ]
**Criterion**: the socket path is derived from the project's canonical path (stable hash): same repo → same socket; different repos → different sockets; the path does not exceed system length limits.
**Verification**: table of canonical paths (incl. symlinks, trailing slashes) → stable and unique derivation.

### U-02 — Robust framing [UNIT] · P1 · [ ]
**Criterion**: socket framing (JSON-RPC 2.0) rejects malformed and oversized messages without breaking the connection, and supports concurrent connections.
**Verification**: framing tests with invalid messages, > limit, and N simultaneous clients.

---

# Spec: v2-ipc — JSON-RPC IPC CLI↔Broker and event model

### F-01 — execution.start [E2E] · P0 · [ ]
**Criterion**: `execution.start` validates the workflow (schema + graph + cycles), creates the execution and returns `execution_id`.
**Verification**: valid workflow → id; workflow with cycle/nonexistent step → structured error without a created execution.

### F-02 — execution.status [E2E] · P1 · [ ]
**Criterion**: `execution.status` returns the aggregated execution state and the step summary.
**Verification**: in-progress execution → per-step states consistent with the transitions that occurred.

### F-03 — Non-blocking step.run [E2E] · P0 · [ ]
**Criterion**: `step.run` for agent steps returns `{attempt_id, cursor}` without blocking; progress is consumed through `step.events`.
**Verification**: slow agent step → `step.run` returns immediately; subsequent events appear via `step.events`.

### F-04 — step.events with stable pagination [E2E] · P0 · [ ]
**Criterion**: `step.events {since_cursor}` returns the subsequent events and `next_cursor`; retrying the same query returns exactly the same events (no duplicates) and advancing the cursor never loses events.
**Verification**: known event sequence (fixture) consumed with retries and cursor jumps.

### F-05 — Idempotent step.approve [E2E] · P0 · [ ]
**Criterion**: `step.approve` resolves an interaction and is idempotent: re-sending the same resolution produces no duplicate effects.
**Verification**: approving the same interaction twice with the same key → a single observable effect in the harness and in the store.

### F-06 — step.cancel [E2E] · P1 · [ ]
**Criterion**: `step.cancel` cancels the in-flight attempt and the harness receives the cancellation; the attempt ends `cancelled`/`failed` per the defined semantics, with no subsequent valid writes.
**Verification**: active attempt → cancel → harness notified, state persisted.

### F-07 — step.reopen with invalidation [E2E] · P0 · [ ]
**Criterion**: `step.reopen` invalidates the current generation and returns the `{invalidated: []}` list of steps affected by the generation cascade.
**Verification**: chain of 3 steps → reopen the first → `invalidated` lists the actual descendants.

### F-08 — step.reopen with optional feedback [E2E] · P1 · [ ]
**Criterion**: `step.reopen` accepts optional feedback for re-execution (preserving the v1 `--feedback` behavior).
**Verification**: reopen with feedback → the step's next attempt receives the full feedback.

### F-09 — CLI notifications [E2E] · P1 · [ ]
**Criterion**: the CLI receives `step.status_changed` and `step.interaction_required` from the broker while consuming events.
**Verification**: test subscription receives the notifications in the expected order with their cursor.

### U-01 — Monotonic cursor and prior persistence [UNIT] · P0 · [ ]
**Criterion**: the cursor is monotonic per session/attempt; the event is persisted **before** becoming visible (a consumer never sees a gap or a non-persisted event).
**Verification**: crash test between persistence and publication → no visible non-persisted events.

### U-02 — Interaction CAS [UNIT] · P0 · [ ]
**Criterion**: interaction resolution is atomic: same `idempotency_key` → same result without re-execution; a different key on the same interaction → rejected; identity crossing (wrong attempt) → rejected.
**Verification**: case table (pending/resolved/duplicate/foreign).

### U-03 — payload_ref without raw output [UNIT] · P1 · [ ]
**Criterion**: `attempt_events.payload_ref` references sanitized evidence managed outside the table; it never contains the full raw output.
**Verification**: store inspection after an attempt → events only contain references/deltas, not complete blobs.

### U-04 — JSON-RPC payloads validated at the boundary [UNIT] · P1 · [ ]
**Criterion**: every incoming and outgoing payload of the JSON-RPC protocols (CLI↔Broker and Broker↔Adapter) is validated at the boundary against its schema (zod, current repo practice); unknown or malformed payload → structured fail-closed error, without state effects.
**Verification**: table of malformed/unknown payloads → rejection with stable code and message with the field path.

---

# Spec: v2-adapter — v2 adapter contract and multi-harness

### F-01 — Single prior initialize [E2E] · P0 · [ ]
**Criterion**: `initialize` runs exactly once per subprocess, before any `session/*`; negotiates `protocolVersion` and capabilities in both directions.
**Verification**: test harness records the call order → initialize exactly once and first.

### F-02 — Never invoke what was not announced [E2E] · P0 · [ ]
**Criterion**: the broker never invokes an optional method the adapter did not declare in `Initialize`.
**Verification**: adapter without `Terminal`/`LoadSession` → the broker does not emit those methods; attempted use from the CLI → `unsupported_capability` error.

### F-03 — request_permission only if negotiated [E2E] · P0 · [ ]
**Criterion**: the harness only calls `session/request_permission` if the broker announced `Permission: true`; otherwise it resolves with its default policy or fails (fail-closed).
**Verification**: broker without negotiated permission + harness that needs it → the harness fails closed, never skips the policy.

### F-04 — Closing the JSONL parser debt [INT] · P0 · [ ]
**Criterion**: the OpenCode JSONL stream parser lives in the OpenCode adapter, not in the core; the core contains no provider literal.
**Verification**: verification script (grep/import graph) that fails if `opencode`, provider endpoints, flags or event names appear outside `adapters/`.

### F-05 — Claude Code adapter with contract tests [E2E] · P1 · [ ]
**Criterion**: a Claude Code adapter exists that passes the public-boundary contract test suite against the real binary, or against recorded fixtures only if the real binary is not viable in CI (justified case).
**Verification**: the suite `step run → real subprocess → protocol → public JSON output → store → settle` runs against `claude` (or its recorded fixtures) and passes.

### F-06 — Fallback between harnesses without semantic classification [E2E] · P0 · [ ]
**Criterion**: a step with `harness: [opencode, claudecode]` (or the reverse) falls back to the next candidate on clean error, without classifying the cause; when candidates are exhausted the step fails with sanitized evidence from each candidate.
**Verification**: first candidate fails cleanly → the second receives the bounded accumulated context (≤ 2 MiB) and runs; both fail → step `failed` with evidence from both.

### U-01 — Capability negotiation by table [UNIT] · P0 · [ ]
**Criterion**: each optional method is only invoked if announced; new capabilities are additive without incrementing the protocol major version; the major version only changes on incompatible changes to the mandatory methods.
**Verification**: table tests with capability combinations (empty, partial, complete, future additive).

### U-02 — First-class contract test suite [UNIT] · P1 · [ ]
**Criterion**: the public-boundary contract test suite exists as a first-class repository artifact from the start.
**Verification**: the repository contains the versioned, runnable suite, not an intent document.

### U-03 — Generic ACP adapter (Zed's standard) [INT] · P1 · [ ]
**Criterion**: the "acp-generic" adapter translates the internal contract to **ACP (Agent Client Protocol, open standard driven by Zed)** and back (initialize, session/new, session/prompt, session/update, session/cancel, session/request_permission) with protocol fixtures.
**Verification**: simulated ACP harness (fixtures) → the adapter completes the full cycle and the capability translations are bijective.

### U-04 — Transport-agnostic attempt [UNIT] · P0 · [ ]
**Criterion**: `attempts` contains no transport fields; the details (native_session_id, protocol_version, extra) live in the per-adapter extension table.
**Verification**: schema test: inserting an attempt without transport and with transport → both valid; native fields never appear in `attempts`.

---

# Spec: v2-path-claims — path_claims and workspace isolation

### F-01 — Worktree per execution by default [E2E] · P0 · [ ]
**Criterion**: the system default is that each workflow execution runs in its own `git worktree`; the effective `workspace_root` is that worktree.
**Verification**: `git worktree list` shows a new worktree per execution; the step's work happens inside it.

### F-02 — 3-level isolation configuration [E2E] · P0 · [ ]
**Criterion**: `workspace.mode` resolves by inheritance `system → workflow → step`, with per-step override; `workspace.shared` in a workflow disables isolation for all its steps unless overridden.
**Verification**: three fixtures (workflow shared, step shared with isolated workflow, step isolated with shared workflow) → correct workspace_root in each case.

### F-03 — Claims by logical identity and prefix [E2E] · P1 · [ ]
**Criterion**: claims are made on the path's logical identity (relative, canonicalized, symlink-free) and claiming a directory blocks its children by prefix comparison.
**Verification**: claiming `src/` → a second claim of `src/foo.ts` reports a conflict; `src/foobar/` does not collide with `src/foo/` (prefix boundary).

### F-04 — shared vs shared blocks [E2E] · P0 · [ ]
**Criterion**: two `shared` steps with the same logical path: the second does not start until the first claim is released; release happens when the step finishes (success or failure), not when the execution ends.
**Verification**: slow step A claiming `src/` → step B with `src/` stays blocked; A finishes (success) → B starts; failure case → B starts all the same.

### F-05 — isolated vs isolated allows [E2E] · P1 · [ ]
**Criterion**: two `isolated` steps with the same logical path run without blocking; divergences are only discovered at integration, outside Haro's scope.
**Verification**: two concurrent isolated steps with `src/` → both start; no conflict is recorded.

### F-06 — isolated vs shared governed by on_logical_conflict [E2E] · P0 · [ ]
**Criterion**: `isolated` vs `shared` on the same path: `on_logical_conflict: block` (default) prevents startup; `allow` (explicit workflow opt-in) permits it.
**Verification**: block fixture → second step blocked; allow fixture → starts.

### F-07 — External resources always shared [E2E] · P1 · [ ]
**Criterion**: paths declared as external to the repo in the project configuration are always treated as `shared`.
**Verification**: external path (e.g. cache) claimed by two steps → shared blocking even if both are isolated.

### U-01 — Logical path canonicalization [UNIT] · P0 · [ ]
**Criterion**: canonicalization (resolved, symlink-free, relative to the repo) handles symlinks, `..`, absolutes and prefix boundaries without false positives or escapes.
**Verification**: table of edge cases including `src/foo` vs `src/foobar`, symlink inside/outside the repo.

### U-02 — Atomic Acquire [UNIT] · P0 · [ ]
**Criterion**: `Acquire` is atomic (INSERT ... ON CONFLICT): two incompatible concurrent acquisitions → one wins, the other receives `Acquired=false` with the current owner.
**Verification**: concurrency test (workers / `Promise.all` over the same database) with the same path and incompatible modes.

### U-03 — Complete behavior matrix [UNIT] · P1 · [ ]
**Criterion**: the four rows of the behavior matrix (isolated/isolated, shared/shared, isolated/shared block, isolated/shared allow) have table tests; `Release` with the wrong owner does not release someone else's claim.
**Verification**: table tests parameterized by mode and policy; release with a foreign owner/step → rejected.

---

# Spec: v2-composicion — Workflow composition

### F-01 — Flat DAG under a single execution_id [E2E] · P0 · [x]
**Criterion**: a `type: workflow` step references another YAML; composition happens at planning time, the result is a single flat DAG under a single `execution_id`, with no child executions.
**Verification**: workflow with a workflow node → `steps next` exposes the flattened internal steps; `status` shows no child execution.

### F-02 — Internal step namespacing [E2E] · P0 · [x]
**Criterion**: internal steps are identified `<node>.<internal_step>`; including the same workflow twice under different nodes does not collide.
**Verification**: workflow with two nodes pointing at the same file → ids `a.x`, `a.y`, `b.x`, `b.y` coexist and run independently.

### F-03 — Explicit inputs/outputs contract [E2E] · P0 · [x]
**Criterion**: `inputs`/`outputs` are only valid in the included file; the parent wires (`depends_on`, `requires`) only against that contract, never against internal steps.
**Verification**: parent referencing an internal step of the included file (not declared as input/output) → clear validation error.

### F-04 — Workflow node bindings [INT] · P1 · [x]
**Criterion**: `bindings` connects the parent's `requires`/`produces` with the included file's `inputs`/`outputs` and artifact resolution respects those mappings.
**Verification**: fixture with bindings → the parent's artifacts feed the included file's correct inputs and vice versa.

### F-05 — Static cycle detection [E2E] · P0 · [x]
**Criterion**: a workflow cannot include itself, directly or transitively; detection is static, over the file-reference graph, before flattening.
**Verification**: fixture with direct and transitive self-inclusion → `run` fails with a cycle error, without a created execution.

### F-06 — Cascade that ignores the boundary [E2E] · P0 · [x]
**Criterion**: the invalidation cascade operates on the flattened generation graph: reopening a step invalidates only the internal steps whose `produces` actually feed what was reopened, with fine granularity, regardless of the declared `outputs` contract.
**Verification**: node with two internal steps where only one feeds what was reopened → `step.reopen` invalidates only that internal step, not the other or those of other nodes.

### F-07 — Parallelization through the same DAG [E2E] · P1 · [x]
**Criterion**: `workflow` nodes without `depends_on` between them are parallelizable through the same mechanism as any step.
**Verification**: two workflow nodes without dependencies → both appear in `steps next` as available.

### U-01 — Pure and reproducible flattening [UNIT] · P0 · [x]
**Criterion**: flattening is a pure function: same YAML files → same flat DAG; includes namespacing, contract and stable topological order.
**Verification**: structural equality tests (DAG hash) on nested composition fixtures, run twice.

### U-02 — Static cycles by table [UNIT] · P0 · [x]
**Criterion**: cycle detection covers direct, transitive and self-inclusion with different node name vs file.
**Verification**: table of file-reference fixtures → expected result in each case.

### U-03 — Contract validation by table [UNIT] · P1 · [x]
**Criterion**: wiring against an undeclared internal step, referencing a nonexistent input/output or declaring `inputs`/`outputs` in a root file → specific validation errors.
**Verification**: table of invalid YAMLs → expected error code and message.

### U-04 — v2 schema validated with zod (not JSON Schema) [UNIT] · P1 · [x]
**Criterion**: the v2 workflow schema (version, steps, workspace, inputs/outputs, bindings and per-step-type conditionals) is defined with zod (current repo practice); the YAML is validated against the equivalent zod representation, not against JSON Schema; errors name the exact field.
**Verification**: table of invalid YAMLs → error messages with field path and stable code.

---

# Spec: v2-reporte — Change reporting

### F-01 — Report at top-level execution completion [E2E] · P0 · [x]
**Criterion**: when the top-level execution finishes, the system computes and shows which files changed relative to the starting point; `execution.report` returns `changed_files`.
**Verification**: execution that creates/modifies/deletes files → the report lists exactly those changes.

### F-02 — Computed once, only at top level [E2E] · P1 · [x]
**Criterion**: the report is computed once, at the top-level execution level — never per internally nested workflow.
**Verification**: execution with nested workflow nodes → a single report at the end; no intermediate sub-reports.

### F-03 — Accuracy against real git [INT] · P1 · [x]
**Criterion**: the report is accurate against the workspace's real state: new, modified, deleted files (and renamed if applicable), against the starting point.
**Verification**: fixture with the four change types → the report matches `git status`/`git diff --name-status` of the workspace.

### U-01 — Diff computation by table [UNIT] · P1 · [x]
**Criterion**: the `changed_files` computation compares the starting point (base/commit) with the final state and covers new, modified, deleted and renamed.
**Verification**: table tests with test repos (real git in memory or temporary directory).

---

# Spec: v2-store — Persistence behind a repository interface

### F-01 — No direct access outside the store [INT] · P0 · [x]
**Criterion**: no layer outside the store accesses SQLite directly; all access goes through the repository interfaces.
**Verification**: dependency verification script (import graph / packages) that fails if a domain or CLI module imports the persistence driver.

### F-02 — Complete v2 DDL [INT] · P0 · [x]
**Criterion**: the schema implements the reference tables: `projects`, `executions`, `execution_steps`, `generations`, `attempts`, `attempt_transport`, `leases`, `step_transition_events`, `attempt_events`, `interactions`, `path_claims` — with their constraints and enum CHECKs.
**Verification**: inspection of the schema created by the system (without manual migrations) against the reference DDL.

### F-03 — WAL and timestamps [INT] · P1 · [x]
**Criterion**: `PRAGMA journal_mode = WAL`, `PRAGMA foreign_keys = ON`; all time columns are ISO 8601 UTC.
**Verification**: pragma query and sampling of time values in created records.

### U-01 — Interchangeable backend [UNIT] · P0 · [x]
**Criterion**: the domain suite (state machine, leases, claims, events) runs against an alternative backend (in-memory/fake) implementing the same interfaces, without changes to the domain code (door open to future concurrency).
**Verification**: the full suite passes against the fake backend and against SQLite with the same tests.

### U-02 — Idempotent schema [UNIT] · P1 · [x]
**Criterion**: schema creation/migration is idempotent and transactional (re-running does not break, mid-way failure leaves no partial state).
**Verification**: create twice + simulate mid-migration failure → consistent state.

### U-03 — Enum CHECKs [UNIT] · P1 · [x]
**Criterion**: the enums (`status` of executions/steps/attempts, `type` of steps, workspace modes) reject invalid values at the database level.
**Verification**: INSERT with invalid value → constraint error.

---

# Spec: v2-distribucion — Distribution

### F-01 — End-to-end installation [E2E] · P0 · [x]
**Criterion**: the distribution mechanism is the current one — global npm package (`pnpm add -g shardeo`), bin `dist/index.js`, `files: ["dist"]`, `prepublishOnly` with build — and installation works in a clean project without additional manual steps.
**Verification**: installation in a clean environment → `shardeo` resolvable and executable; `npm run build` produces a complete `dist/`.

### F-02 — No post-install scripts [E2E] · P1 · [x]
**Criterion**: installation does not run post-install scripts that execute code (known attack vector; pnpm disables them by default).
**Verification**: review of the distribution package/artifact → no executable install hooks.

### F-03 — init without additional dependencies [E2E] · P1 · [x]
**Criterion**: `shardeo init` works in a project without any previously installed dependency (single-checkout local tool, no network infrastructure).
**Verification**: empty project → complete init; no network call required (verifiable with network disabled).

---

# Spec: v2-flujo-sdd — Development flow (previously v2-flujo-gentle-ai (D09))

### F-01 — Each spec as an SDD change [PROC] · P0 · [ ]
**Criterion**: each spec in this file (v2-reconciliacion, v2-no-regresion, v2-broker…v2-distribucion) is developed as an SDD change with proposal → spec → design → tasks → apply → verify → archive; the criteria in this file are the source of each spec's acceptance criteria.
**Verification**: one change per spec with criterion → spec → tests traceability.

### F-02 — Verification via sdd-verify [PROC] · P0 · [ ]
**Criterion**: `sdd-verify` (or the equivalent verification of the SDD flow) runs the described verifications: the `[E2E]` ones as public boundary, the `[UNIT]`/`[INT]` ones as module tests.
**Verification**: each change's verification report lists the covered criteria with their evidence.

### F-03 — Delivery through the SDD flow [PROC] · P0 · [ ]
**Criterion**: each delta's delivery goes through the SDD flow gates (review receipts, delivery gates), not through the initiatives pipeline; no new initiatives are created.
**Verification**: delivery history with receipts; absence of new `.docs/initiatives/`.

### U-01 — Criterion→test traceability [UNIT] · P1 · [ ]
**Criterion**: each criterion in this file has at least one named test verifying it (explicit mapping, e.g. table in the change's spec).
**Verification**: audit script that walks the criterion IDs and confirms their corresponding test exists and passes.

---

## Tracking summary

> **Single tracking source**: this document. Each criterion carries a checkbox (`- [ ]` pending / `- [x]` met) in its header. The **Status** column of the table reflects the SDD phase of each spec's change — `pending → proposal → spec → design → tasks → apply → verify → archive → complete` — and is updated at the close of each phase. Fine detail (artifacts, evidence, decisions) lives in the change's artifacts (openspec/ + Engram).

| id-spec | Content | Criteria | P0 | Status |
|---|---|---|---|---|
| `v2-reconciliacion` | Reconciliation of the normative documentation | 5 | 3 | **complete** |
| `v2-no-regresion` | Preservation of Specs 1–9 | 17 | 14 | pending |
| `v2-broker` | Persistent per-project broker | 9 | 4 | pending |
| `v2-ipc` | JSON-RPC IPC CLI↔Broker and events | 13 | 6 | pending |
| `v2-adapter` | v2 adapter contract and multi-harness | 10 | 6 | pending |
| `v2-path-claims` | path_claims and workspace isolation | 10 | 5 | pending |
| `v2-composicion` | Workflow composition | 11 | 6 | **complete** |
| `v2-reporte` | Change reporting | 4 | 1 | **complete** |
| `v2-store` | Persistence behind a repository interface | 6 | 3 | **complete** |
| `v2-distribucion` | Distribution | 3 | 1 | **complete** |
| `v2-flujo-sdd` | Development flow | 4 | 3 | pending |
| **Total** | | **92** | **52** | |

Last updated: 2026-09-07 — `v2-distribucion` **complete** (3/3 criteria, verify PASS WITH WARNINGS, archived in `openspec/changes/archive/2026-09-07-v2-distribucion/`).

## Out of scope (explicitly not covered)

- `shardeo doctor`, `shardeo setup` and CLI registration in `config.yaml` (AGENTS.md's "Future: v2" roadmap): they are not part of the v2 normative proposal; they are managed as separate specs if decided.
- Shared multi-user concurrency: deferred; `v2-store/U-01` only guarantees that the interface does not block it.
- Interaction bundle CAS (unimplemented part): deferred; `v2-no-regresion/F-08` covers only what already exists (Spec 9a).