# Design: Supervised Managed Sessions through ACP

## Technical Approach

Add `SupervisedCoordinator` beside the existing async command path. It validates discovery, freezes and admits an immutable bundle, starts one fenced `acp-generic` session, and returns from `step.run` only after `ready`. ACP stays adapter-owned; OpenCode/Claude remain CLI-direct headless; terminal/PTY stays rejected.

`2026-09-11-v2-broker-ipc` MUST pass unfinished task 7.12, archive, and reconcile F-03–F-06/U-01/U-02/U-04 before implementation tasks proceed.

## Architecture Decisions

| Decision | Choice and rationale |
|---|---|
| Ownership | One in-process supervisor per attempt, identified by daemon holder, lease/fencing token, and endpoint generation. A sidecar cannot preserve broker-owned fencing. |
| ACP | Direct executable plus ordered argv and explicit environment over JSON-RPC 2.0 stdio; no shell, interpolation, PTY, or OpenCode HTTP/SSE. This keeps provider details outside core. |
| Admission | Contained copies and byte digests, not source paths, prove immutable input. |
| Recovery | Reconnect only with negotiated idempotent resume and matching attempt/session/transport/admission identities; otherwise orphan. Never auto-restart or replay authorization. |

## Data Flow

```text
workflows list/describe → parse + graph diagnostics
step.run → freeze/hash/admit → exec → initialize → session/new → fenced ready commit
ACP update/permission → sanitize → EventSink transaction → response/fanout
approve/cancel → active generation → provider effect → durable outcome or orphan
```

## Interfaces and Contracts

Discovery returns structured `workflows:[{name,path,valid,errors,steps}]`; `describe` returns the same record. `workflow.Discover` accumulates per-file malformed/unknown YAML errors, and graph validation reports duplicate workflow/step IDs, missing dependencies, cycles, and no zero-dependency entry point instead of hiding valid siblings. `supervised` is schema-valid, but runtime readiness requires all F-08/F-09/F-11 proofs; any missing, drifted, malformed, unknown, or unsupported prerequisite fails before provider work without fallback.

One CLI composition root creates one `adapter.Manager`, probes each configured harness once, initializes each usable adapter once, and injects that same manager into every agent-capable engine (`run`, `step run/reopen/skip`, and broker engine); report-only paths remain unwired. Existing OpenCode/Claude registrations and headless behavior are unchanged. Each ACP subprocess separately sends exactly one wire `initialize`, validates negotiated capabilities, then permits `session/new`, `session/prompt`, `session/update`, `session/cancel`, and negotiated `session/request_permission`; unknown, malformed, oversized, unsupported, or out-of-order traffic terminates fail-closed.

Strict `.haro/config.yaml` `harnesses.acp-generic` fields are `executable`, `argv`, `env`, `enabled`, and `timeout_seconds`; executable is non-empty/direct, argv preserves YAML order, env is an explicit key/value allowlist, and timeout is 1–300 seconds. Reject unknown fields, command strings, NULs, interpolation, and ambient secret inheritance. `supervised` fields are `retention_days` (default 30), `retention_bytes` (default 268435456), `inactivity_timeout_seconds`, and `decision_timeout_seconds` (both default 300); require positive bounded integers and sanitize before use.

Bundle manifest order is instructions, skills in declaration order, required artifacts in `requires` order, then prior and fallback context. Before exec, persist a receipt matching `bundle_id`, canonical manifest SHA-256, every ordered mandatory-entry SHA-256/category, and no omitted/excess entries; symlink escape or drift rejects admission.

Interactions expose only persisted `available_decisions` and atomically transition `pending→resolving→resolved|resolution_failed`. The same `(interaction,idempotency_key,decision)` returns its stored result; any different/foreign/unknown/stale tuple has no provider effect. `step.cancel` routes only to the active endpoint generation, enters `cancelling`, sends `session/cancel`, commits confirmed status, invalidates the lease, and rejects stale writes; uncertain shutdown becomes `orphaned`. Attempt-event cursors are monotonic per attempt, allocated with the event transaction; bounded sanitized control events commit before any response/notification, while retained output remains external and path-free.

## File Changes

| Files | Action |
|---|---|
| `internal/workflow/*`, `internal/cmd/execute.go`, matching tests | Structured discovery diagnostics, supervised validation, single-manager composition. |
| `internal/project/config.go`, `internal/adapter/{factory,acp}/*` | Strict launch config, live ACP client/fixture, unchanged headless adapters. |
| `internal/execution/{async,supervised,bundle}.go`, `internal/broker/{steps,supervisor,output,recovery}.go` | Readiness, routing, timers, guided recovery. |
| `internal/store/{store,migrations,managed_sessions,bundles,interactions,output,fake}.go` | Receipts, lifecycle, output, SQLite/fake parity. |

## Testing Strategy

RED unit/integration tables cover discovery errors; manager call counts/injection; config rejection; exact ACP ordering/methods; bundle order/digests; CAS, generation, cursor, and crash windows; independent timers and retention. E2E proves readiness, cancellation uncertainty, broker restart/guided recovery, unchanged headless behavior, terminal rejection, and persist-before-visible events.

## Threat Matrix

| Boundary | Applicability and RED behavior |
|---|---|
| ACP process integration | Applicable: exact executable/argv/env succeeds; shell strings, disguised/non-executable files, inherited secrets, bad framing, or uncertain death fail without effects. |
| Documentation-like paths | Applicable to executable validation: reject `requirements.txt`, `CMakeLists.txt`, Markdown/MDX, and `README.sh` lacking valid executable semantics. |
| Git repository selection | N/A: no VCS routing. |
| Commit state | N/A: no commits. |
| Push state | N/A: no pushes. |
| PR commands | N/A: no PR automation. |

## Migration / Rollout

Add schema and dormant ACP capability first. Keep the unconditional supervised guard until dependency reconciliation and final readiness/E2E proof; replace it only with the fail-closed admission route. Rollback unregisters ACP and restores rejection while retaining evidence. No destructive data migration.

## Open Questions

None.
