# Proposal: v2 Broker IPC

## Intent

Introduce the persistent per-project broker and CLI↔broker JSON-RPC boundary required by Constitution IV.1, VI–VIII, `SPECS.md`, and the v2 technical specification. Deliver all 9 `v2-broker` and 13 `v2-ipc` criteria (22 total) while preserving the 92 criteria/11-spec contract. The broker outlives CLIs and provides a fail-closed interaction skeleton without enabling supervised execution.

## Scope

### In Scope
- Same-binary `haro broker`; canonical `EvalSymlinks`+`Clean`, worktree-coherent SHA-256 socket keys, UDS-length safety, and pure-Go Windows named-pipe parity.
- Flock-guarded lazy launch with double-check/dial retry, no PID files, zombie-socket recovery, independent projects, clean signal shutdown, and broker-death lease/fencing checks before every engine write.
- Reused 10 MiB NDJSON codec, concurrent connections, malformed-frame recovery, strict per-method decoding (`DisallowUnknownFields`, required fields, enums), and side-effect-free `-32602` failures.
- `execution.start/status`; asynchronous `step.run → {attempt_id,next_cursor}`; stable `step.events`; persisted-before-visible bounded events; fanout `step.status_changed` and `step.interaction_required` notifications.
- `step.approve` CAS with attempt identity and `available_decisions` gate; `step.cancel` session cancellation, `cancelled` persistence, and fencing invalidation; `step.reopen → {invalidated}` with cascade and optional feedback.

### Out of Scope
- Supervised runtime, bundles/admission (`F-08`), ACP server dispatch, terminal/PTY, and resume/reconciliation. `step.run` MUST reject `supervised` and `terminal` with the engine guard's terminal reason.
- No new acceptance IDs or normative contract edits.

## Capabilities

### New Capabilities
- `v2-broker`: per-project daemon lifecycle, isolation, framing, concurrency, fencing, and shutdown.

### Modified Capabilities
- `v2-ipc`: expand the existing payload doctrine with the complete CLI RPC, event, interaction, and notification contract.

## Approach

Use seven revertible units: (1) daemon/socket lifecycle; (2) RPC server/validation; (3) execution start/status; (4) async run/events; (5) approve/cancel; (6) reopen; (7) notifications, fencing, and shutdown. `step.events` uses the current attempt's cursor space; transitions remain available through status and cursor-bearing notifications. Design MUST confirm this projection and worktree identity algorithm. Broker runtime keeps adapter/session ownership behind an ACP-compatible boundary without ACP dispatch.

## Compliance and Impact

Primary areas are `internal/{broker,ipc,cmd,execution,store}`, tests, and delta specs. Preserve provider neutrality, bounded evidence, CAS, and persist-then-publish ordering.

## Risks

UDS length; launch herd duplication; incomplete fencing; cursor ambiguity; approval gate bypass; mode-guard bypass; worktree key mismatch; zombie sockets; Windows drift; persist/publication crash window. Each receives platform tables, race/E2E coverage, centralized guards, or commit-before-fanout ordering.

## Rollback Plan

Revert units in reverse order. The explicit `haro broker` entrypoint serves `run`, `status`, and `step run/events/approve/cancel/reopen`; `report`, `step skip`, and unrelated commands retain direct-engine behavior. Reverting routing restores each command's direct path without migration.

## Delivery and Success

- `single-pr`, maintainer-approved `size:exception` (200000 lines), direct `main`; strict TDD with `go test ./...`.
- [ ] All 22 mapped criteria pass reproducibly, including race, death, pagination, CAS, and malformed-boundary cases.
- [ ] Existing direct paths and the 92/11 traceability count remain green.
