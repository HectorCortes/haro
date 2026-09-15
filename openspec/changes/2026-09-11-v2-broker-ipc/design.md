# Design: v2 Broker IPC

## Technical Approach

Add a same-binary per-project broker and route `run`, `status`, and broker-backed `step` operations through strict NDJSON JSON-RPC. Keep `report`, `step skip`, supervised, terminal/PTY, ACP, and resume unchanged. Units: endpoint; server; start/status; run/events; approve/cancel; reopen; notifications/fencing/shutdown.

## Architecture Decisions

| Decision | Choice and rationale |
|---|---|
| Endpoint | `EvalSymlinks(Clean(root))`; SHA-256 short hex; collisions extend the prefix after checking `0600` metadata. Unix uses `os.TempDir()/haro-<hash>.sock` (NUL included ≤108 bytes); Windows uses `\\.\pipe\haro-<hash>`. |
| Execution | Split `Engine.StartStep` (guard, validation, lease, atomic transition/generation/attempt) from `ExecuteAttempt`; return `{attempt_id,next_cursor:0}`, then run under broker context. This avoids duplicating engine policy. `supervised`/`terminal` return `<mode> mode not supported` before lease/attempt creation. |
| Events | `step.events` serves only the latest attempt (`attempts WHERE execution_id=? AND step_id=? ORDER BY started_at DESC,id DESC LIMIT 1`), then `attempt_events WHERE attempt_id=? AND cursor>=? ORDER BY cursor LIMIT 128`. `next_cursor=last+1`, or unchanged when empty. Transition events never merge; status and notifications expose them. This gives deterministic retry, no gaps/duplicates, and bounded pages. Payload is sanitized inline ≤16 KiB; `payload_ref` is external/legacy only. |
| Windows | `ipc.Transport{Listen,Dial,Endpoint}` has build-tag Unix/Windows implementations (pure-Go named pipes). Linux tests both path formats; socket E2E is Linux-only. |

## Components and Flow

`internal/broker/{daemon,launcher,runtime,server,handlers,notify}.go` defines `Daemon.Run`, `Ensure`, `Runtime`, `Dispatcher`, handlers, `Hub`, and `SessionRegistry`. `internal/ipc/client.go` multiplexes responses/notifications; `socket.go` owns identity; platform files own transport/flock/detach (`Setsid`). `cmd.Execute` adds `broker --project`; JSON output remains.

`Ensure`: Dial → flock → Dial → remove only a refused socket → detached `os.Executable() broker --project <canonical>` → backoff Dial. Daemon holds flock, opens one WAL store/engine, accepts concurrently, and tracks `starting/running/draining/stopped`. INT/TERM closes listener, rejects work, cancels sessions, drains replies, releases leases, removes socket, closes store, unlocks. EOF detects death; only idempotent calls retry.

## RPC Contracts

Mappings/shapes: `execution.start{workflow_path,workspace_override?}→Engine.CreateExecution→{execution_id}`; `execution.status{execution_id}→Get+Steps.List→{status,steps}`; `step.run{execution_id,step_id,mode?}→StartStep+goroutine→{attempt_id,next_cursor}`; `step.events{execution_id,step_id,since_cursor}→{events,next_cursor}`; `step.approve{execution_id,step_id,interaction_id,decision,idempotency_key}→{resolved}`; `step.cancel{execution_id,step_id}→{}`; `step.reopen{execution_id,step_id,feedback?}→{invalidated}`. Cancelable adapter/session handles live in `SessionRegistry[attempt_id]`. Notifications are `step.status_changed{execution_id,step_id,from,to,cursor}` and `step.interaction_required{execution_id,step_id,interaction_id,kind,description,options,cursor}`. Dedicated structs use `DisallowUnknownFields`, reject trailing/missing/invalid data before effects, and validate results. Codes: `-32700` parse, `-32600` envelope, `-32601` method, `-32602` boundary/field, `-32603` internal; `-32001` stale/reopen-and-rerun, `-32002` interaction conflict, `-32003` unsupported mode.

`Runtime.EventSink` serializes event transactions: validate `(holder,token,expires_at)` in `WithTx`, INSERT state/event, commit, then fanout while retaining sequence ownership. Pre-commit crashes expose nothing; post-commit crashes recover by polling. `Hub` subscribes connections through `step.events`; a writer mutex frames replies and cursor-bearing notifications in commit order.

`BrokerSessionHost.RequestPermission` atomically creates a pending interaction and `interaction_required` event with bounded description/options, publishes, then waits on a registry channel; supervised remains guarded. Idempotency key equals interaction ID. Approve verifies current attempt, persisted `available_decisions`, and repeated decision before `Resolve`; foreign/changed decisions fail closed.

## Fencing

Lease scope is `(execution_id,step_id)`, holder daemon UUID, TTL 60s, renewal 20s. `Acquire` rejects another unexpired holder and increments after expiry/release. `execution.FencedStore` checks holder/token/expiry inside every mutating engine transaction; reopen acquires affected leases sorted. Cancel expires its lease; stale goroutines stop without writes.

## File Changes

| Action | Files |
|---|---|
| Create | `internal/broker/*.go`, `internal/ipc/{client,socket,transport_unix,transport_windows}.go` and matching tests |
| Modify | `main.go`, `internal/cmd/{execute,output}.go`, `internal/ipc/jsonrpc/codec.go`, `internal/ipc/health.go`, `internal/execution/{engine,state}.go`, `internal/store/{store,repositories,leases,interactions,fake}.go`, contract tests, `go.mod` |
| Delete | None |

## Testing Strategy

Strict RED/GREEN uses `go test ./...`; final `-race`. Unit: `socket_test.go` (symlink/long/collision/worktree/Windows), `validation_test.go`, `events_projection_test.go`, `fencing_test.go`, `interactions_test.go`, `publish_test.go` (commit failpoints). `broker_e2e_test.go` uses Linux sockets for lazy/herd, isolation/concurrency, zombie/TERM/death, CAS/cancel, pagination/notifications, and zero-attempt guards. Windows gets cross-build and path units only.

## Threat Matrix

| Boundary | Applicability | Response / RED tests |
|---|---|---|
| Documentation-like paths | N/A: no executable classification | None |
| Git repository selection | N/A: no VCS command routing; project root is explicit | None |
| Commit state | N/A: no commit behavior | None |
| Push state | N/A: no push behavior | None |
| PR commands | N/A: no PR automation | None |

## Risks and Rollout

| Risk | Mitigation |
|---|---|
| UDS length/collision; worktree mismatch | bounded hash, metadata collision extension; launcher and daemon retain the passed project root when execution cwd is a worktree; coherence table |
| Launch herd/zombie | flock, double-check, liveness-only removal, N-client test |
| Incomplete fencing/death | transactional `FencedStore`, fake/SQLite parity, kill/expiry test |
| Cursor ambiguity/publication crash | attempt-only projection, serialized commit-then-fanout, failpoints |
| Approval or mode bypass | centralized identity/options/CAS and pre-attempt mode guards |
| Windows drift | interface/build tags/cross-build plus Linux path tables |

No schema migration is required. Roll back the seven units in reverse; direct unaffected commands remain available.

## Open Questions

None.
