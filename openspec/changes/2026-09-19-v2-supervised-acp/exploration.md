## Exploration: Complete `mode: supervised` with ACP

### Current State
Haro now has a working per-project Go broker over JSON-RPC/Unix sockets, asynchronous `step.run` for command attempts, cursor-persisted attempt events, notification fanout, interaction persistence with idempotent CAS, lease/fencing repositories, and adapter capability negotiation. The broker already has a `BrokerSessionHost` that persists a permission interaction, publishes `interaction_required`, waits for `step.approve`, and releases the decision through `SessionRegistry`; however, this host is not connected to a managed supervised session.

The execution path is still explicitly fail-closed for supervised and terminal modes. `Engine.StartStep` rejects both modes before claims, lease, attempt, or generation creation; `Engine.runAgentStep` retains the same guard. The broker's async executor currently supports command attempts, not agent attempts. Headless agents remain CLI-direct through `adapter.Manager`, with OpenCode and Claude subprocess adapters and a fail-closed negotiated permission wrapper, but their sessions are blocking and do not provide a persistent supervised lifecycle.

The ACP package is only a translation helper and unit fixture suite (`ToACP`, `FromACP`, and method-shaped parameter builders). There is no ACP JSON-RPC client/session implementation, generic `acp-generic` adapter, subprocess lifecycle for an ACP-speaking harness, reverse notification dispatch, or capability-backed admission proof. There is also no bundle/manifest/admission implementation in the Go repository: `SessionBundle` currently carries instructions, workspace root, and required artifact paths only. The normative documents describe bundle, admission, supervisor, output, recovery, and timeout behavior as implemented/required, but the current Go implementation has not reached that state.

### Affected Areas
- `internal/execution/async.go` — currently rejects supervised/terminal in `StartStep` and only executes command attempts; it must become the lifecycle boundary for non-blocking supervised starts, leases, fencing, cancellation, and finalization.
- `internal/execution/engine.go` — current headless agent flow performs candidate intersection, session creation, blocking event collection, and a fail-closed-but-currently-automatic first permission decision; supervised mode must not reuse this blocking path or bypass policy.
- `internal/broker/steps.go` — already exposes `step.run`, `step.events`, `step.approve`, and `step.cancel`, but `step.run` starts only the existing async command worker and has no readiness handshake, supervised result, or supervisor endpoint/generation routing.
- `internal/broker/runtime.go` — contains the reusable interaction wait/approval host and session registry, but lacks a managed-session owner, ACP event loop, decision timeout, reconciliation, or supervisor lifecycle.
- `internal/broker/daemon.go` and `internal/broker/event_sink.go` — provide the per-project owner, hub, and persist-before-fanout seam; supervised workers need daemon shutdown, lease renewal, and event publication integrated here.
- `internal/adapter/adapter.go`, `internal/adapter/manager.go`, `internal/adapter/capabilities.go` — define the current adapter contract and additive capability negotiation; the contract needs a bundle/admission capability and a safe supervised-session surface without provider literals in core.
- `internal/adapter/acp/translate.go` and `internal/adapter/acp/acp_test.go` — translation-only ACP coverage exists; a real ACP transport/client, typed boundary validation, notification handling, and generic adapter are missing.
- `internal/adapter/opencode/adapter.go` and `internal/adapter/claude/adapter.go` — are CLI-direct headless adapters. They explicitly report no supervised/terminal capabilities and should remain unchanged unless a harness-specific managed adapter is added.
- `internal/store/store.go`, `internal/store/{interactions,leases}.go`, `internal/execution/fenced_store.go`, and migrations/fakes — provide useful repositories and fencing, but do not yet model supervisor/session state, resolving interactions, admission receipts, output retention, or orphaned attempts.
- `internal/ipc/{client.go,jsonrpc}` and `internal/broker/server.go` — are suitable for CLI↔broker UDS IPC; broker↔ACP transport requires a separate adapter-owned JSON-RPC client/stdio boundary, not reuse of provider details in broker code.
- `deltas-acceptance.md` — F-08 and F-09 are the direct acceptance targets; F-11 and adapter U-03 constrain permission detection and ACP fixtures, while broker F-06/F-07 and IPC F-03–F-05/U-01–U-04 are dependencies.
- `docs/v2/haro-constitucion.md` — requires fail-closed behavior, provider-neutral core, reverse permission flow, monotonic persisted cursors, lease fencing, and no optional capability invocation without negotiation.
- `docs/v2/haro-especificacion-tecnica.md` — defines the Go adapter contract, attempt transport separation, CLI↔broker methods, ACP-shaped broker↔adapter methods, and persistence DDL.
- `docs/reference/SPECS.md` §9b — is the behavioral authority for readiness, immutable bundle admission, supervisor ownership, interactions, event/output separation, recovery, and independent inactivity/decision timers.
- `openspec/changes/archive/2026-09-10-v2-supervised-guard/` and `openspec/changes/2026-09-11-v2-broker-ipc/` — explicitly document the current guard and the broker/IPC slice's deliberate exclusion of supervised runtime and ACP dispatch.

### Approaches
1. **Broker-native supervised runtime with an ACP generic adapter** — add a managed-attempt supervisor behind the existing broker and execution seams; materialize and verify bundles before readiness; acquire and renew fenced leases; use an adapter-owned ACP JSON-RPC client for `initialize`, `session/new`, `session/prompt`, `session/update`, `session/cancel`, and `session/request_permission`; persist normalized events before publication; route approvals through CAS and the supervisor; retain OpenCode/Claude CLI-direct headless behavior.
   - Pros: matches the locked ACP decision and normative layering; keeps provider literals in adapters; reuses existing broker IPC, event sink, store repositories, capability negotiation, and session-host concepts; supports later ACP-speaking harnesses without changing core policy.
   - Cons: large cross-cutting change; requires explicit schema/API decisions for supervisor state, bundle receipts, output storage, recovery, and timeout/orphan semantics; requires end-to-end harness fixtures and likely multiple implementation slices despite the current 20,000-line single-PR budget.
   - Effort: High

2. **Extend current CLI-direct adapters with an in-process supervised mode** — make OpenCode/Claude sessions asynchronous and keep supervision in `adapter.Manager`, adding broker callbacks around the existing subprocess streams without a generic ACP transport.
   - Pros: could reuse existing process parsers and reduce initial transport code.
   - Cons: violates the explicit ACP product decision and the provider-neutral broker↔adapter contract; does not provide a generic ACP adapter; would couple supervision to JSONL/CLI behavior, make structured permission resolution inconsistent, and risk silently changing headless semantics.
   - Effort: High

3. **Add an ACP sidecar/bridge outside the core and keep the current engine guard** — implement a separate external process that speaks ACP while Haro remains fail-closed.
   - Pros: isolates protocol experimentation and limits immediate core changes.
   - Cons: does not deliver supervised product behavior or acceptance F-09; cannot provide Haro-owned leases, persisted cursors, CAS approvals, readiness, or recovery; leaves the user-visible mode unavailable.
   - Effort: Medium for a spike, unsuitable for the requested product scope

### Recommendation
Proceed with Approach 1. Treat this as a new supervised-runtime change that first defines the managed-attempt state machine and bundle/admission receipts, then implements a supervisor abstraction, then adds the ACP generic adapter and fixtures, and finally removes the supervised guard only after readiness, event, approval, timeout, fencing, cancellation, and crash-recovery paths are proven. Preserve terminal/PTY as out of scope: the existing repository does not contain a Go PTY implementation, and supervised does not need terminal methods. Keep OpenCode and Claude CLI-direct headless adapters unchanged; an ACP-speaking harness should be registered separately as `acp-generic` only when its real session contract exists.

The implementation should use the existing `Store`/`FencedStore` and `EventSink` as foundations but should not assume that the current interaction table is sufficient for the normative `resolving`/`resolution_failed` lifecycle. The design must decide whether to extend the schema or constrain the first supervised slice to the currently supported pending/resolved CAS while explicitly specifying crash behavior. Similarly, `SessionBundle` must evolve from path hints to an immutable bundle identity and admission receipt without exposing provider-specific transport syntax to core code.

### Risks
- Removing the guard before a readiness handshake and fenced supervisor exists would permit a supervised attempt to run headless or lose ownership; the guard must remain until the complete path is wired.
- The current `agentHost` automatically selects the first permission option and is unsafe for supervised use; it must never be reused as the supervised policy path.
- Existing lease validation protects repository writes but does not itself manage heartbeat, supervisor process identity, stale endpoint generations, or broker-death reconciliation.
- Persist-before-visible ordering is present in `EventSink`, but high-volume output is not a managed on-disk stream and must remain separate from bounded control-event payloads.
- ACP translation currently has weak/fixture-only boundary validation and no live notification/client loop; malformed, unknown, or drifted ACP messages must fail closed without state effects.
- The normative documents claim bundle/admission and terminal surfaces as implemented, while the Go tree lacks them; proposal/design work must reconcile the source-of-truth language rather than silently claiming existing behavior.
- Interaction resolution is not a distributed transaction with provider application; crash handling during resolution needs an explicit safe reconciliation/orphan policy.
- Timeout counters must remain independent: process inactivity, interaction decision, and terminal human-presence behavior must not be conflated. Terminal/PTY remains outside this change.
- The current acceptance document contains legacy references to the earlier TypeScript stack; the new change should trace the Go criteria and avoid broad unrelated document rewrites unless proposal work identifies a required normative correction.

### Ready for Proposal
Yes. The product decision is sufficiently clear: implement `mode: supervised` through ACP, not opencode HTTP/SSE, while keeping terminal/PTY out of scope. The proposal should resolve the remaining product choices before design: the exact supported ACP harness/launch contract, bundle entry sources and admission proof shape, supervisor persistence/state model, output retention interface, timeout defaults/configuration, broker restart/orphan semantics, and whether the existing interaction schema is extended for `resolving`/`resolution_failed`. It should map work to F-08, F-09, F-11, U-03, and the broker/IPC dependency criteria.
