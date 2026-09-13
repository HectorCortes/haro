# Tasks: v2 Broker IPC

## Review Workload Forecast

Estimated lines: ~4,200 (prod ~2,100 / tests ~1,900 / wiring+docs ~200). Delivery: `single-pr`, maintainer-approved `size:exception` against the default 400-line threshold with a 20,000-line review budget; direct push to `main`, no PR, no chaining unless the forecast crosses 20,000 lines.
Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception (no chaining unless the forecast crosses 20,000 lines)
400-line budget risk: High

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|
| U1 | Socket identity + daemon/launcher lifecycle | `go test ./internal/ipc ./internal/broker -run 'SocketPath|Ensure|Daemon'` | `haro broker --project` on t.TempDir repo; herd of 8 CLIs | internal/ipc/socket*,transport*, internal/broker/{daemon,launcher}+tests |
| U2 | RPC server + strict validation | `go test ./internal/broker -run 'Server|Validation'` | two conns: malformed frame then valid | internal/broker/{server,handlers,validation}+tests |
| U3 | execution.start/status + CLI client (run/status) | `go test ./internal/broker ./internal/cmd -run 'Exec'` | `haro run` / `haro status` via socket | handlers/exec.go, ipc/client.go wiring |
| U4 | Async step.run + step.events projection | `go test ./internal/execution ./internal/broker -run 'StartStep|Events'` | slow command fixture returns <200ms | engine split, handlers/events.go |
| U5 | step.approve CAS + step.cancel | `go test ./internal/broker ./internal/store -run 'Approve|Cancel|Lease'` | pending interaction seeded via store | handlers/interactions.go, runtime.go |
| U6 | step.reopen cascade + feedback delivery | `go test ./internal/execution ./internal/broker -run 'Reopen'` | 3-step chain reopen→rerun | state.go feedback persistence, handlers/reopen |
| U7 | Fencing + notifications + shutdown + E2E | `go test ./... -race` (final gate) | kill -9 mid-attempt; kill -TERM restart | leases/FencedStore/notify/shutdown, e2e |

Strict TDD: RED task before GREEN per task (`go test ./...` locally mandatory; final `-race`, `go vet ./...`, `go build ./...`; CI adds golangci-lint + govulncheck). Linux-only socket E2E documented in `broker_e2e_test.go` header; Windows gets cross-build (`GOOS=windows go build ./...`) + path-table units only.

## Validator-Correction Resolutions (binding for apply)

1. **F-08 feedback mechanism**: `step.reopen{feedback?}` persists bounded sanitized feedback; next `step.run` on that attempt delivers it via the existing `---FEEDBACK---` composition (`RunStep` semantics unchanged; `step.run` gains NO feedback param). Storage: additive nullable `pending_feedback` column on `execution_steps` (same additive-ALTER precedent as `ensurePayloadColumn`; no new tables). Direct CLI `step run --feedback` still wins when explicit.
2. **execution.start path→name**: canonicalize `workflow_path` (`EvalSymlinks+Clean`, relative resolved against canonical project root), exact canonical-path equality against `workflow.Discover(e.root)` entries; match → pass `d.Workflow.Name` to `CreateExecution`; mismatch → `-32602`, stable message `workflow_not_found`, field `params.workflow_path`, zero rows created. (`-32002` stays reserved for interaction conflict per design code table; path resolution is reference validation per v2-ipc/F-01.)
3. **Windows transport dependency**: `github.com/Microsoft/go-winio` (pure-Go named pipes) added to `go.mod` behind the `Transport` seam — explicit task 1.6. **Lease rejection is a designed behavior change**: `Leases.Acquire` today always grants token+1 (`internal/store/leases.go:55-58`, `fake.go:883-899`); task 7.1 changes it to reject another unexpired holder (`ErrLeaseConflict`), with SQLite+fake parity tests.

## U1: Socket identity + daemon/launcher lifecycle [v2-broker/F-01,F-02,F-05*,U-01]

- [x] 1.1 RED `internal/ipc/socket_test.go`: canonical table — symlink/trailing-slash equivalents share endpoint; distinct roots differ; UDS ≤108 bytes incl NUL; long canonical path safe (bounded hash); worktree coherence table (execution cwd under `.haro/worktrees/<id>` → endpoint keyed by passed canonical project root, never the worktree).
- [x] 1.2 GREEN `internal/ipc/socket.go`: `EvalSymlinks(Clean(root))` → sha256 short hex → `os.TempDir()/haro-<hash>.sock`; Windows branch `\\.\pipe\haro-<hash>`; collision extends hash prefix only after verifying `0600` metadata.
- [x] 1.3 RED `internal/broker/launcher_test.go`: absent broker → launch + retry + outlives CLI (F-02); N=8 concurrent `Ensure` → exactly one daemon (F-05); zombie socket (file exists, Dial refused) → removed, launch proceeds; flock EWOULDBLOCK → re-Dial liveness only.
- [x] 1.4 GREEN `internal/broker/launcher.go`: `Ensure` = Dial → flock lockfile → double-check Dial → remove only refused socket → detached `os.Executable() broker --project <canonical>` (`Setsid`) → backoff Dial; only idempotent calls retry; EOF detects daemon death. `daemon.go`: `Daemon.Run` holds flock, single WAL store+engine, concurrent accept, states starting/running/draining/stopped.
- [x] 1.5 GREEN `main.go` + `internal/cmd`: `broker --project <root>` subcommand; JSON output contract preserved.
- [x] 1.6 GREEN transport seam: `internal/ipc` `Transport{Listen,Dial,Endpoint}` interface; `transport_unix.go` (build tag `!windows`); `transport_windows.go` via go-winio; add `github.com/Microsoft/go-winio` to `go.mod` (correction 3); `GOOS=windows go build ./...` green.

## U2: RPC server + strict validation [v2-broker/U-02, v2-ipc/U-04]

- [x] 2.1 RED `internal/broker/server_test.go`: malformed frame → `-32700`, connection stays healthy; oversized (>10 MiB) → `-32600`/`-32700` then next valid frame matched; N concurrent connections each get matched responses; no panic on any input.
- [x] 2.2 GREEN `internal/broker/server.go`: per-conn goroutine, reused `jsonrpc/codec.go` NDJSON codec, per-conn writer mutex interleaving replies and notifications, envelope checks (`jsonrpc=="2.0"`, id) → `-32600`, unknown method → `-32601`; health demux without store retained.
- [x] 2.3 RED `internal/broker/validation_test.go`: per-method strict decode (`DisallowUnknownFields`, required fields, enums): unknown field / missing field / invalid enum / trailing data / non-object params → `-32602` with stable message + field path, no `WithTx` entered (zero effects).
- [x] 2.4 GREEN `internal/broker/handlers.go`: `Dispatcher` map, typed params structs per method, fail-closed decode helper; error-code table enforced: `-32700` parse, `-32600` envelope, `-32601` method, `-32602` boundary/field, `-32603` internal, `-32001` stale/reopen-and-rerun, `-32002` interaction conflict, `-32003` unsupported mode.

## U3: execution.start/status + CLI client wiring [v2-ipc/F-01,F-02]

- [x] 3.1 RED `internal/broker/exec_test.go`: start path→name resolution per correction 2 — valid path → `{execution_id}`; unknown/mismatched canonical path → `-32602 workflow_not_found`, nothing persisted; invalid workflow (cycle/schema) → error, zero executions; symlinked path resolves equal.
- [x] 3.2 GREEN `internal/broker/handlers` exec.go: `execution.start{workflow_path,workspace_override?}` → canonicalize → Discover match → `Engine.CreateExecution(name)`; `execution.status{execution_id}` → `Executions.Get` + `Steps.List` → `{status,steps}` consistent with persisted transitions.
- [x] 3.3 GREEN `internal/ipc/client.go`: mux responses/notifications, `Call` via launcher `Ensure` with dial/launch/retry; route `run` and `status` through broker in `internal/cmd/execute.go`; `report` and `step skip` stay direct-engine; error mapping preserves existing JSON codes.

## U4: Async step.run + step.events projection [v2-ipc/F-03,F-04,U-01,U-03]

- [ ] 4.1 RED mode guard: `step.run{mode:"supervised"|"terminal"}` → `-32003` `<mode> mode not supported` BEFORE lease/attempt creation — zero attempts, generations, leases, transitions (design: pre-attempt centralized guard).
- [ ] 4.2 GREEN engine split (`internal/execution/engine.go`): `StartStep` (DAG verify, guards, validation, lease acquire, atomic pending→running + generation + attempt in one tx) vs `ExecuteAttempt` (continuation under caller ctx); broker `step.run` replies `{attempt_id,next_cursor:0}` immediately then goroutine runs `ExecuteAttempt`; `SessionRegistry[attempt_id]` retains cancelable adapter/session handle.
- [ ] 4.3 RED `events_projection_test.go`: current attempt = `attempts WHERE execution_id=? AND step_id=? ORDER BY started_at DESC,id DESC LIMIT 1`; page = `attempt_events WHERE attempt_id=? AND cursor>=? ORDER BY cursor LIMIT 128`; `next_cursor=last+1` or unchanged when empty; retry same `since_cursor` → identical page; no gaps/duplicates across sequential pages; transition events never merged (available via status/notifications only).
- [ ] 4.4 GREEN `step.events` handler + `Events.ListAttemptEvents` repo method; outgoing payloads sanitized inline ≤16 KiB, `payload_ref` external/legacy only (U-03 boundary revalidation).
- [ ] 4.5 GREEN CLI: `step run`/`step events` routed via broker; client polls `step.events` from returned cursor; `step run` remains JSON `ok` on early return.

## U5: step.approve CAS + step.cancel [v2-ipc/F-05,F-06,U-02]

- [ ] 5.1 RED `internal/broker/interactions_test.go`: CAS table — pending+identical key → resolved once; repeat identical → same result, no re-execution; different key → rejected; foreign attempt/execution → rejected fail-closed; unknown decision (not in `available_decisions`) → state unchanged; idempotency key = interaction ID.
- [ ] 5.2 GREEN `BrokerSessionHost.RequestPermission`: atomically create pending interaction + `interaction_required` event with `available_decisions` persisted in the bounded payload (description/options ≤16 KiB), publish, then wait on registry channel; supervised guard untouched.
- [ ] 5.3 GREEN `step.approve{execution_id,step_id,interaction_id,decision,idempotency_key}`: verify current attempt ownership + persisted `available_decisions` + repeated decision before `Resolve` CAS → `{resolved}`; foreign/changed → `-32002` fail closed.
- [ ] 5.4 RED cancel: active attempt → session `Cancel` (harness notified), `attempts.status=cancelled` persisted, lease released/expired, subsequent stale writes rejected with `-32001`; second `step.cancel` idempotent.
- [ ] 5.5 GREEN `step.cancel{execution_id,step_id}` → `{}`: resolve running attempt, cancel via `SessionRegistry`, persist cancelled + transition, invalidate lease.

## U6: step.reopen cascade + feedback [v2-ipc/F-07,F-08]

- [ ] 6.1 RED reopen cascade: completed 3-step chain, reopen root → `{invalidated}` lists exactly affected descendants (depends_on closure + produces→requires feeder drill-down); generations invalidated; transition history preserved.
- [ ] 6.2 GREEN `step.reopen{execution_id,step_id,feedback?}` handler → `Engine.ReopenStep` → `{invalidated}` derived from generations audit (`InvalidateByStep` + `ListByStep` where `invalidated_by_step` matches).
- [ ] 6.3 RED+GREEN feedback delivery per correction 1: additive `pending_feedback` ALTER (`ensurePayloadColumn` pattern); reopen with feedback → bounded sanitized persist → next `step.run` (no param) consumes+clears and delivers complete `---FEEDBACK---` context; history intact; explicit `--feedback` (direct path) still wins.
- [ ] 6.4 GREEN CLI: `step reopen` and `step run` fully broker-routed with `{invalidated}` JSON passthrough.

## U7: Fencing + notifications + shutdown + E2E [v2-broker/F-06,F-07; v2-ipc/F-09,U-01]

- [ ] 7.1 RED `internal/store/leases_test.go` + fake parity: `Acquire` by another unexpired holder → `ErrLeaseConflict` (correction 3 designed change); after expiry or release → token+1; same-holder reacquire allowed; both SQLite and fake (`fake.go`).
- [ ] 7.2 GREEN `leases.go`+`fake.go`: reject unexpired foreign holder; TTL 60s, renewal 20s (daemon heartbeat renews owned leases).
- [ ] 7.3 RED `internal/broker/fencing_test.go`: `FencedStore` validates (holder,token,expires_at) inside EVERY mutating engine `WithTx`; stale-token write → rejected, persisted state unchanged, error carries recovery guidance (reopen-and-rerun); kill broker mid-attempt → lease expiry → old token rejected.
- [ ] 7.4 GREEN `internal/execution` `FencedStore` wrapper routing all mutating engine writes; reopen acquires affected leases in sorted order; cancel expires its lease.
- [ ] 7.5 RED `internal/broker/publish_test.go` (commit failpoints): crash before commit → event never visible, no phantom cursor; crash after commit before fanout → consumer retry discovers committed event; INSERT→commit→fanout ordering proven.
- [ ] 7.6 GREEN `internal/broker/runtime.go`: `Runtime.EventSink` serializes event transactions — validate `(holder,token,expires_at)` in `WithTx`, INSERT state/event, commit, then fanout retaining sequence ownership.
- [ ] 7.7 RED `internal/broker/notify_test.go`: subscribed conn receives `step.status_changed{execution_id,step_id,from,to,cursor}` and `step.interaction_required{...,interaction_id,kind,description,options,cursor}` in persisted/cursor order.
- [ ] 7.8 GREEN `internal/broker/notify.go`: `Hub` subscribes connections through `step.events`; writer mutex frames replies + cursor-bearing notifications in commit order.
- [ ] 7.9 RED shutdown test: INT/TERM → listener closed, work rejected (draining), sessions cancelled, replies drained, leases released, socket removed (no zombie), store closed, flock released; immediate replacement starts.
- [ ] 7.10 GREEN daemon shutdown orchestration (`internal/broker/daemon.go` signal handling per design).
- [ ] 7.11 GREEN `internal/broker/broker_e2e_test.go` (Linux sockets): F-01 shared broker two CLIs; F-02 relaunch-after-death; F-03 two parallel executions isolate state; F-04 two projects independent, one broker stops, other continues; F-05 herd; F-06 kill -9 mid-attempt + stale write; F-07 TERM→restart; U-02 concurrent/malformed frames; zero-attempt mode guards.
- [ ] 7.12 Gate: `go test ./...`, `go test ./... -race`, `go vet ./...`, `go build ./...`, `GOOS=windows go build ./...`; 92 existing criteria green; no edits to `deltas-acceptance.md`/`docs/v2`.

## Commit Plan (work-unit-commits — 7 conventional commits to main, revertible in reverse)

1. `feat(broker): socket identity, transport seam (go-winio), daemon and launcher lifecycle` — U1
2. `feat(ipc): JSON-RPC server loop with strict per-method validation and error-code table` — U2
3. `feat(broker): execution.start path resolution and status over RPC; broker client for run/status` — U3
4. `feat(broker): async step.run engine split and step.events attempt projection with mode guards` — U4
5. `feat(broker): step.approve CAS gate and step.cancel with lease invalidation` — U5
6. `feat(broker): step.reopen cascade with persisted feedback delivery` — U6
7. `feat(broker): fencing store, notification fanout, clean shutdown, and Linux E2E` — U7

Each commit: tests included, `go test ./...` green at commit boundary, rollback = revert that commit without touching unrelated work (units 1–2 independent; 3–7 layer on prior units, reverted last-first).
