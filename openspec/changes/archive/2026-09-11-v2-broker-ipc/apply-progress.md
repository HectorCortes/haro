# Apply Progress: v2 Broker IPC

Date: 2026-09-13 · Mode: Strict TDD (bounded focused Go tests) · Delivery: `single-pr` with maintainer-approved `size:exception` against the default 400-line threshold and a 20,000-line review budget · Direct commits to `main`, not pushed.

## Status

38/39 tasks complete. U1 through U6 and U7 lease/fencing/lifecycle/notification/publish slices are applied; U7.11 is complete and U7.12 remains pending because the corrective race gate failed in `internal/broker/TestStartStepAndExecuteAttemptAreSeparated` with a lease-still-active assertion. The dispatcher supplied `applyState: ready` for this continuation.

## Completed Work Units

### U1 — Socket identity + daemon/launcher lifecycle

Commit: `ddc162d` · `feat(broker): socket identity, transport seam, daemon and launcher lifecycle`

#### TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.1 | `internal/ipc/socket_test.go` | Unit | N/A (inherited new file) | ✅ Existing canonical tests were temporarily run against reverted derivation and failed | ✅ `go test ./internal/ipc -run 'SocketPath'` passed | ✅ Symlink, trailing slash, distinct root, long root, worktree, and platform-shape cases | ✅ Canonicalization and bounded endpoint naming retained |
| 1.2 | `internal/ipc/socket_test.go` | Unit | N/A (new implementation) | ✅ Socket derivation tests failed during inherited-code RED reconstruction | ✅ `go test ./internal/ipc -run 'SocketPath'` passed | ✅ Multiple roots and long paths force distinct code paths | ✅ Fixed-width hash and UDS bound constants extracted |
| 1.3 | `internal/broker/launcher_test.go` | Integration | N/A (inherited new file) | ✅ Launcher tests failed against temporarily disabled `Ensure` | ✅ `go test ./internal/broker -run 'Ensure' -count=20` passed | ✅ Absent launch, herd of 8, zombie endpoint, and held flock cases | ✅ Startup marker closes the post-flock detached-daemon race |
| 1.4 | `internal/broker/{launcher,daemon}_test.go` | Integration | ✅ Existing repository suite passed before U1 edits | ✅ Inherited launcher tests exposed herd/daemon lifecycle failures | ✅ `go test ./internal/broker -run 'SocketPath|Ensure|Daemon' -count=20` passed | ✅ Repeated launch, daemon hand-off, zombie recovery, and liveness-only paths | ✅ `errors.Is` preserves wrapped flock ownership detection |
| 1.5 | `internal/broker/launcher_test.go` | Integration | ✅ Existing command tests passed before U1 edits | ✅ Broker helper invocation failed with disabled launcher/derivation | ✅ Broker helper process reached the live health endpoint under `Ensure` tests | ✅ Detached helper outlives the launching call and idempotent reuse is covered | ✅ JSON command error path remains unchanged |
| 1.6 | `internal/ipc/{transport_unix,transport_windows}.go` | Unit/build | N/A (new platform seam) | ✅ Windows build failed before timeout signature correction | ✅ `GOOS=windows go build ./...` passed | ✅ Unix tests plus Windows cross-build exercise platform selection | ✅ Endpoint derivation moved behind `Transport.Endpoint` |

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test ./internal/ipc ./internal/broker -run 'SocketPath|Ensure|Daemon' -count=20` — exit 0; 20 repeated U1 runs passed in each package |
| Runtime harness command/scenario and exact result | `go test ./internal/broker -run 'TestEnsure(LaunchesDaemonWhenAbsent|HerdExactlyOneDaemon|RemovesZombieSocket|LockHeldDialsOnly)$'` — exit 0; detached/helper broker health, herd, zombie, and flock scenarios passed |
| Rollback boundary | Revert U1 commit: remove `internal/broker/{daemon,launcher,flock_*,detach_*}.go` and tests, `internal/ipc/{socket,transport_*}.go` and tests, broker CLI entrypoint, and go-winio dependency; leave unrelated execution/store changes untouched |

#### Gates

- `go test ./...` — exit 0.
- `go vet ./...` — exit 0.
- `go build ./...` — exit 0 (included in the suite's successful build path).
- `GOOS=windows go build ./...` — exit 0.

### U2 — RPC server + strict validation

Commit: `465f9b2` · `feat(ipc): add JSON-RPC server validation and error handling`

#### TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 2.1 | `internal/broker/server_test.go` | Integration | ✅ U1 package tests passed before U2 edits | ✅ New server API tests failed to compile before implementation | ✅ `go test ./internal/broker -run 'TestServer'` passed | ✅ Malformed, oversized, valid-following, invalid envelope, unknown method, and 12 concurrent connections | ✅ Per-frame recovery and panic containment centralized |
| 2.2 | `internal/broker/server.go`, `internal/ipc/jsonrpc/codec.go` | Integration | ✅ `go test ./internal/broker ./internal/ipc/jsonrpc` passed before U2 edits | ✅ Server tests failed with undefined server/dispatcher and missing error codes | ✅ `go test ./internal/broker ./internal/ipc/jsonrpc -run 'TestServer|TestCodec'` passed | ✅ Same connection recovers after two failure classes; independent connections match IDs | ✅ Oversized frames are drained before the next NDJSON frame; strict trailing JSON rejected |
| 2.3 | `internal/broker/validation_test.go` | Unit | ✅ U1 and codec safety net passed | ✅ Tests were run against a temporarily disabled `DecodeParams` and failed | ✅ `go test ./internal/broker -run 'TestDecodeParams|TestValidateEnum'` passed | ✅ Unknown, missing, enum, trailing, non-object, and zero-effect cases | ✅ Decode into a temporary value prevents partial destination mutation |
| 2.4 | `internal/broker/handlers.go`, `internal/broker/validation.go` | Unit | ✅ Existing broker tests passed before handler edits | ✅ Strict validation tests failed before dispatcher and helper APIs existed | ✅ `go test ./internal/broker -run 'TestServer|TestDecodeParams|TestValidateEnum'` passed | ✅ Registered, unknown, panic, and invalid boundary paths exercise the error table | ✅ Handler panic is converted to `-32603` without terminating the connection |

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test ./internal/broker ./internal/ipc/jsonrpc -run 'TestServer|TestCodec|TestDecodeParams|TestValidateEnum'` — exit 0; all selected server, codec, validation, and enum tests passed |
| Runtime harness command/scenario and exact result | `go test ./internal/broker -run 'TestServerRecoversMalformedAndOversizedFrames|TestServerMatchesConcurrentConnectionResponses'` — exit 0; net.Pipe runtime exercised recoverable frames and 12 concurrent connections |
| Rollback boundary | Revert U2 commit: remove `internal/broker/{server,handlers,validation}.go` and tests, restore daemon connection dispatch, and restore codec framing behavior; retain U1 endpoint/lifecycle files |

#### Gates

- `go test ./...` — exit 0.
- `go vet ./...` — exit 0.
- `go build ./...` — exit 0.
- `GOOS=windows go build ./...` — exit 0.

### U4 — Async step.run + step.events projection

Commits: `61b7eba`, `d2b650b` · `feat(broker): add async step execution and event projection`; `fix(execution): retain leases across async attempts`

#### TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 4.1 | `internal/broker/steps_test.go` | Integration | ✅ Focused broker baseline passed | ✅ Missing `handleStepRun`/mode guard compile failure | ✅ `TestStepRunRejectsUnsupportedModeBeforeAttempt` passed; both guarded modes leave zero runtime rows | ✅ Supervised and terminal cases | ✅ Centralized `stepRPCError` mode mapping |
| 4.2 | `internal/broker/steps_test.go`, `internal/execution/async.go` | Integration | ✅ Execution/store baseline passed | ✅ Missing `StartStep` and `ExecuteAttempt` compile failure | ✅ `TestStartStepAndExecuteAttemptAreSeparated` passed; early result, one attempt, lease acquire/release, and completion | ✅ Slow runner and cancellation-aware release path | ✅ Attempt claim/lease tracking extracted from handler |
| 4.3 | `internal/broker/steps_test.go` | Integration | ✅ Store event tests passed | ✅ Missing current-attempt/page repository APIs compile failure | ✅ `TestStepEventsProjectsCurrentAttemptWithStablePages` passed; 128-event page, retry, and sequential cursor | ✅ 129 events force bounded second page; transition events remain excluded | ✅ Repository owns current-attempt ordering and cursor paging |
| 4.4 | `internal/broker/steps.go`, `internal/store/{repositories,fake}.go` | Unit/integration | ✅ Store parity baseline passed | ✅ Missing `ListAttemptEvents` compile failure | ✅ Focused broker/store tests passed; inline payloads are sanitized and bounded to 16 KiB, legacy refs remain refs | ✅ SQLite and fake repository paths | ✅ Shared store interface keeps backend parity |
| 4.5 | `internal/cmd/step_broker_test.go` | Integration | ✅ Command package baseline passed | ✅ Missing broker step routing compile failure | ✅ `TestCLIStepRunAndEventsUseBroker` passed; JSON early result and event cursor | ✅ Run and events methods both asserted | ✅ Existing direct `--feedback` path remains selected |

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `timeout 120s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=90s ./internal/broker ./internal/execution ./internal/store ./internal/cmd -run 'Test(StepRunRejectsUnsupportedModeBeforeAttempt|StartStepAndExecuteAttemptAreSeparated|StepEventsProjectsCurrentAttemptWithStablePages|CLI.*Step|CommandCycle|Feedback|AttemptEventPayloadBackendParity)'` — exit 0; broker, execution, store, and command selectors passed |
| Runtime harness command/scenario and exact result | The broker handler test ran a slow fake command through `step.run`, observed the immediate `{attempt_id,next_cursor:0}` response, then released the runner and observed persisted completion; exit 0 |
| Rollback boundary | Revert commits `d2b650b` and `61b7eba`: remove async step/event handlers, async engine/store APIs, pending-feedback migration, and CLI broker step routing; retain U1–U3 and unrelated launcher cleanup |

#### Gates and Resource Cleanup

- Focused bounded tests passed; no full suite, race run, build, or verify phase was started.
- After every broker/runtime selector, `pgrep -af 'HARO_TEST_BROKER|haro broker'` showed no broker process and `/tmp/haro-*.sock` glob returned no files.

### U5 — step.approve CAS + step.cancel

Implementation is currently uncommitted in this apply slice; it is bounded by the U5 rollback boundary below.

#### TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 5.1 | `internal/broker/interactions_test.go` | Integration | ✅ U4 broker/store selectors passed | ✅ New handler/host symbols failed to compile before implementation | ✅ CAS table covers unknown decision, different key, foreign execution, pending preservation, identical retry, and changed decision | ✅ Bounded focused broker/store selector passed; store state remains unchanged on rejected approvals |
| 5.2 | `internal/broker/runtime.go`, `internal/broker/interactions_test.go` | Integration | ✅ Existing interaction repository CAS behavior retained | ✅ `BrokerSessionHost` and registry were initially undefined | ✅ Permission request persists a bounded `interaction_required` payload and approval unblocks the waiter | ✅ Payload redaction/bounding and transaction-scoped interaction/event creation extracted into runtime host |
| 5.3 | `internal/broker/steps.go`, `internal/broker/interactions_test.go` | Integration | ✅ Strict `DecodeParams` boundary retained | ✅ `handleStepApprove` was initially undefined | ✅ Current-attempt ownership, interaction ID key, available-decision gate, CAS resolution, and idempotent repeat pass | ✅ `-32002` conflict mapping centralized |
| 5.4 | `internal/execution/async.go`, `internal/broker/interactions_test.go` | Integration | ✅ U4 async attempt lifecycle passed | ✅ Cancel handler/session behavior was initially undefined | ✅ Adapter session receives `Cancel`, attempt persists `cancelled`, lease expires, and later execution is rejected | ✅ Cancellation context and stale-write checks use cancel-independent cleanup contexts |
| 5.5 | `internal/broker/steps.go`, `internal/broker/interactions_test.go` | Integration | ✅ U4 handler registration and engine split passed | ✅ `step.cancel` was initially undefined | ✅ First cancel returns `{}`, second cancel is idempotent, registry and engine cleanup both run | ✅ SessionRegistry owns both headless cancel functions and adapter sessions |

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `timeout 120s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=90s ./internal/broker ./internal/store -run 'Test.*(Approve|Cancel|Lease)|TestStep|TestBrokerSession'` — exit 0; selected broker and store tests passed |
| Runtime harness command/scenario and exact result | `timeout 120s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=90s ./internal/broker -run 'Test(BrokerSessionHostPersistsPermissionAndWaitsForApproval|StepCancelCancelsSessionAndPersistsCancelledAttempt)'` — exit 0; persisted interaction approval unblocked the host, and cancellation notified the fake session, persisted `cancelled`, and expired the lease |
| Rollback boundary | Revert the U5 slice: remove `internal/broker/runtime.go` and `internal/broker/interactions_test.go`; revert U5 additions in `internal/broker/{daemon,steps}.go` and `internal/execution/async.go`; retain U1–U4 and unrelated launcher cleanup |

#### Gates and Resource Cleanup

- Focused U5/U4 regression selectors passed; no full suite, race run, build, or verify phase was started.
- After the focused selectors, `pgrep -af 'HARO_TEST_BROKER|haro broker'` showed no broker process and `/tmp/haro-*.sock` returned no files.

### U6 — step.reopen cascade + feedback

Implementation is currently uncommitted in this apply slice.

#### TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 6.1 | `internal/broker/reopen_test.go` | Integration | ✅ Existing execution cascade tests passed | ✅ Broker reopen handler was initially undefined | ✅ Three-step dependency cascade returns stable sorted IDs, invalidates generations, and preserves transition history | ✅ Handler derives the result from generation invalidation audit |
| 6.2 | `internal/broker/steps.go`, `internal/broker/reopen_test.go` | Integration | ✅ `Engine.ReopenStep` existing cascade behavior retained | ✅ `handleStepReopen` was initially undefined | ✅ RPC accepts reopen and returns `{invalidated}` | ✅ Optional `cascade` defaults true at the broker boundary |
| 6.3 | `internal/execution/state.go`, `internal/broker/reopen_test.go` | Integration | ✅ U4 pending-feedback consumption path passed | ✅ Feedback persistence was a no-op before this slice | ✅ Reopen persists bounded redacted feedback; next attempt emits complete delimited feedback and clears it | ✅ Feedback uses the shared 2 MiB fallback bound and propagates repository errors |
| 6.4 | `internal/cmd/step_broker_test.go` | Integration | ✅ Existing broker CLI routing test passed | ✅ CLI rejected `--json`/did not call `step.reopen` broker method | ✅ CLI broker route returns `{invalidated}` JSON and forwards feedback | ✅ Direct runner override and explicit direct `step run --feedback` behavior remain unchanged |

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `timeout 120s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=90s ./internal/execution ./internal/broker -run 'Reopen'` — exit 0; execution and broker reopen selectors passed |
| Runtime harness command/scenario and exact result | `TestStepReopenReturnsStableCascadeAndPersistsFeedback` and `TestStepReopenFeedbackIsDeliveredToNextAttempt` — exit 0; persisted cascade and next-attempt feedback delivery passed |
| Rollback boundary | Revert U6 additions in `internal/{broker/steps,reopen_test}.go`, `internal/execution/state.go`, and `internal/cmd/{execute,step_broker_test}.go`; retain U1–U5 |

### U7 — completed lease, notification, and shutdown slices

#### TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 7.1 | `internal/store/leases_test.go` | Unit/parity | ✅ Existing lease monotonic tests passed | ✅ Foreign acquire initially succeeded | ✅ SQLite and fake reject an unexpired foreign holder with `ErrLeaseConflict`; same-holder reacquire increments token | ✅ Expiry comparison is centralized in both repository implementations |
| 7.2 | `internal/store/{leases,fake}.go` | Unit/parity | ✅ Existing release/renew fencing tests passed | ✅ Lease acquire ignored holder/expiry ownership | ✅ Bounded lease selectors passed with existing 60-second TTL behavior | ✅ Release still expires rows and token monotonicity remains intact |
| 7.7 | `internal/broker/notify_test.go` | Integration | ✅ Existing JSON-RPC notification codec tests passed | ✅ `Hub` was initially undefined | ✅ Two cursor-bearing notifications arrive in order without response IDs | ✅ Hub snapshots subscribers and removes failed writers |
| 7.8 | `internal/broker/{notify,server,daemon}.go` | Integration | ✅ Server response writer mutex retained | ✅ Server had no notification hub seam | ✅ Hub subscriptions share the connection writer mutex and daemon connections receive broadcasts | ✅ Variadic server seam preserves existing callers |
| 7.9 | `internal/broker/daemon_shutdown_test.go` | Integration/runtime | ✅ Existing daemon lifecycle selectors passed | ✅ Signal/shutdown runtime harness exposed orphan socket before this slice | ✅ Context shutdown closes listener, removes socket, and permits immediate replacement | ✅ SessionRegistry cancellation occurs before connection/store teardown |
| 7.10 | `main.go`, `internal/broker/daemon.go` | Runtime | ✅ Bounded built binary launched successfully | ✅ Main used `context.Background()` and ignored TERM | ✅ Built binary TERM harness exited 0 and removed its socket | ✅ `signal.NotifyContext` owns SIGINT/SIGTERM while daemon shutdown remains ordered |

#### U7 Partial Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `timeout 120s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=90s ./internal/broker ./internal/execution ./internal/store ./internal/cmd -run 'Test(Step|Reopen|Hub|DaemonShutdown|Lease|CommandCycle|Feedback|CLI.*Step|AttemptEventPayloadBackendParity)'` — exit 0; all selected broker, execution, store, and command tests passed |
| Runtime harness command/scenario and exact result | Bounded `/tmp/haro-u7-test broker --project <temp>` harness sent SIGTERM; process exited 0 and `/tmp/haro-98ee86449111ac1a.sock` was removed |
| Rollback boundary | Revert U7 partial files `main.go`, `internal/store/{leases,fake}.go`, `internal/broker/{notify,daemon,server,runtime,steps}.go`, and their new tests; retain U1–U6 |

### U7 — fencing wrapper and stale-write validation

Implementation is currently uncommitted in this apply slice.

#### TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 7.3 | `internal/execution/fenced_store_test.go` | Integration/unit | ✅ U5/U6 async and lease selectors passed | ✅ Stale-write and expired-lease tests fail when the guard is disabled | ✅ `go test ./internal/execution -run 'TestFencedStore'` passed; persisted state remains unchanged and errors carry reopen/rerun guidance | ✅ Fake-store token replacement and transaction callback paths | ✅ Guarded repository views centralize lease validation |
| 7.4 | `internal/execution/{fenced_store,async}.go`, `internal/execution/engine.go` | Integration | ✅ Existing async completion/cancellation selectors passed | ✅ Async write routing was initially absent from the final fenced-store cycle | ✅ Focused broker/execution selectors passed; completion writes and resync use the fenced view before lease release | ✅ Lease-change test plus broker cancel/reopen regressions | ✅ Read paths remain available while all attempt-state writes are guarded |

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `timeout 120s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=90s ./internal/execution -run 'TestFencedStore'` — exit 0; 2 fencing tests passed |
| Runtime harness command/scenario and exact result | `timeout 120s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=90s ./internal/broker ./internal/execution -run 'Test(Step|Reopen|Interaction|FencedStore)'` — exit 0; async broker paths and stale-write rejection passed |
| Rollback boundary | Revert `internal/execution/fenced_store.go`, `internal/execution/fenced_store_test.go`, and the fenced-store routing changes in `internal/execution/{async,engine}.go`; retain U1–U6 and U7 lease/notification/shutdown files |

### U7 — commit-before-fanout event sink

Implementation is currently uncommitted in this apply slice.

#### TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 7.5 | `internal/broker/publish_test.go` | Integration | ✅ Existing store transaction behavior passed | ✅ `TestEventSinkRollsBackBeforeCommit` failed under the temporary no-rollback mutation | ✅ `go test ./internal/broker -run 'TestEventSink'` passed; pre-commit rows are absent and post-commit rows are pollable | ✅ Before/after commit failpoints and subscriber observation | ✅ EventSink keeps commit and fanout sequencing in one serialized path |
| 7.6 | `internal/broker/{event_sink,runtime,steps}.go` | Integration | ✅ Existing notification and interaction selectors passed | ✅ EventSink tests failed before the rollback/fanout implementation was restored | ✅ Focused broker selectors passed; Runtime exposes fenced sinks and status/interaction fanout occurs after persistence | ✅ Hub subscribers observe only committed events | ✅ Failpoint hooks are isolated from production behavior |

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `timeout 120s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=90s ./internal/broker -run 'TestEventSink'` — exit 0; all 3 publish-order/failpoint tests passed |
| Runtime harness command/scenario and exact result | EventSink tests exercised pre-commit rollback, post-commit polling recovery, and subscriber visibility after commit; exit 0 |
| Rollback boundary | Revert `internal/broker/event_sink.go`, `internal/broker/publish_test.go`, and EventSink wiring in `internal/broker/{runtime,steps,daemon}.go`; retain U1–U6 and U7 lease/fencing/notification/shutdown files |

#### U7 Remaining

- Full Linux E2E and final gates: 7.11–7.12.

#### U7 E2E Progress (7.11 partial)

- Added `internal/broker/broker_e2e_test.go` (Linux-only) covering a built
  binary, two concurrent IPC clients, `execution.start/status`, TERM cleanup,
  and immediate broker restart with persisted status lookup.
- Focused runtime command: `timeout 180s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=150s ./internal/broker -run 'TestLinuxBrokerProcessSharedAndRestart'` — exit 0 in 0.844s on the latest run.
- Process/socket cleanup check after the run found no broker process and no
  `/tmp/haro-*.sock` files.
- The final correction extended the same file with the Linux process matrix:
  F-01 two CLI executions on one broker; F-02 lazy relaunch after SIGKILL;
  F-03 concurrent execution isolation; F-04 independent project shutdown;
  F-05 a four-caller launch herd; F-06 SIGKILL plus a stale fenced write and
  reopen/rerun recovery; and U-02 malformed-frame recovery, concurrent
  requests, and zero-attempt mode guards.

### U7.11 — Linux broker process E2E matrix

#### TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 7.11 | `internal/broker/broker_e2e_test.go` | Linux process E2E | Existing U7 lease, fencing, notification, publish, and shutdown selectors | ✅ The new matrix was authored before execution; the first bounded run was red at compile time with `undefined: bufio` before the missing import was corrected | ✅ `timeout 180s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=150s ./internal/broker -run 'TestLinuxBrokerProcess'` passed | ✅ Built-binary UDS scenarios covered shared CLIs, lazy relaunch, isolated concurrent executions, independent projects, herd single-launch, SIGKILL stale-write fencing, malformed-frame recovery, concurrent requests, and zero-attempt mode guards | ✅ Added bounded process cleanup, endpoint refusal/removal checks, and direct stale-store verification without changing production code |

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `timeout 180s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=150s ./internal/broker -run 'TestLinuxBrokerProcess'` — exit 0; `internal/broker` passed in 4.273s, including the prior shared/restart test and all matrix subtests |
| Runtime harness command/scenario and exact result | The passing selector built a temporary `haro-linux-e2e` binary with bounded `go build -p=1`, started real `haro broker --project` processes over Linux UDS, exercised all F-01–F-06/U-02 matrix scenarios, and verified TERM removal plus SIGKILL refusal/restart behavior — exit 0 |
| Rollback boundary | Remove the `TestLinuxBrokerProcessE2EMatrix` function and its helpers added after `mustDialE2E` in `internal/broker/broker_e2e_test.go`; retain the pre-existing `TestLinuxBrokerProcessSharedAndRestart` slice and all unrelated U7 changes |

#### Process and socket cleanup evidence

- `pgrep -af '[H]ARO_TEST_BROKER|[h]aro broker' || true` — exit 0; no matching broker process output.
- `setopt null_glob; for path in /tmp/haro-*.sock; do stat -c '%A %a %U %G %n' "$path"; done` — exit 0; no socket output.

### U7.12 — Final gates

U7.12 remains pending. The bounded serial gate subset passed, but the explicit host guardrail prohibited the required `go test ./...` and `go test ./... -race` commands; therefore the complete final-gate task and the 92-criterion claim are not asserted.

| Gate | Bounded command and exact result |
|---|---|
| Type check | `timeout 180s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go vet -p=1 ./...` — exit 0 |
| Linux build | `timeout 180s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go build -p=1 ./...` — exit 0 |
| Windows cross-build | `timeout 180s env GOMAXPROCS=2 GOMEMLIMIT=512MiB GOOS=windows GOARCH=amd64 go build -p=1 ./...` — exit 0 |
| Full test suite | Not run: user safety instruction explicitly forbids `go test ./...` |
| Race suite | Not run: user safety instruction explicitly forbids `-race` |

#### U7.12 blocker

The remaining checkbox can only be marked after an authorized bounded runtime continuation permits the full test and race gates. `sdd-verify` was not started.

## Remaining Tasks

- [x] 1.1 through 1.6 — socket identity, transport seam, daemon, launcher, and broker entrypoint
- [x] 2.1 through 2.4 — RPC server and strict validation
- [x] 3.1 through 3.3 — execution start/status and client wiring
- [x] 4.1 through 4.5 — asynchronous step run and event projection
- [x] 5.1 through 5.5 — approval CAS and cancellation
- [x] 6.1 through 6.4 — reopen cascade and feedback
- [x] 7.3 through 7.4 — fencing wrapper and stale-write validation
- [x] 7.5 through 7.6 — commit-before-fanout event sink
- [x] 7.11 — Linux E2E matrix (shared/restart, lazy relaunch, isolation, independent projects, herd, fencing recovery, malformed/concurrent frames, and mode guards)
- [ ] 7.12 — final gates

### U3 — Execution start/status + CLI client wiring

Commit: `dd9b980` · `feat(broker): wire execution start and status through RPC`

#### TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 3.1 | `internal/broker/exec_test.go` | Integration | ✅ Focused broker package baseline passed at resume | ✅ Inherited RED evidence; task was complete in the dispatcher state | ✅ `TestExecutionStartResolvesCanonicalWorkflowPath` and `TestExecutionStartRejectsUnknownPathAndInvalidWorkflowWithoutRows` passed | ✅ Valid path, symlink-equivalent path, unknown path, and cycle/schema rejection | ✅ Canonical path helper and stable RPC error mapping retained |
| 3.2 | `internal/broker/exec_test.go` | Integration | ✅ Focused broker package baseline passed at resume | ✅ Inherited RED evidence; task was complete in the dispatcher state | ✅ `TestExecutionStatusReturnsPersistedAggregateAndSteps` passed | ✅ Running aggregate, pending step, and unknown execution/no mutation cases | ✅ Status projection uses the persisted execution and step repositories |
| 3.3 | `internal/ipc/client_test.go`, `internal/cmd/execute_test.go` | Integration | ✅ IPC and command package baseline passed at resume | ✅ Inherited RED evidence; task was complete in the dispatcher state | ✅ Client multiplexing/remote-error tests and `TestCLIUsesBrokerForRunAndStatus` passed | ✅ Matched responses, notifications, remote errors, JSON output, and human output | ✅ Client response mux and CLI routing remain behind injectable seams |

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `timeout 90s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=60s ./internal/broker ./internal/ipc ./internal/cmd -run 'TestExecution|TestClient|TestCLIUsesBroker'` — exit 0; broker 0.057s, IPC 0.007s, command 0.006s |
| Runtime harness command/scenario and exact result | Bounded built-binary harness started `haro broker --project <temp git repo>`, then ran `haro run demo --json` and `haro status <execution_id> --json` through the Unix socket — functional exit 0; execution ID, aggregate `running`, and step `pending` matched. TERM cleanup exposed the pending U7 signal-shutdown gap and left one safe 0600 orphan socket; no broker process remained, and the socket was removed after verification |
| Rollback boundary | Revert `dd9b980`: remove `internal/broker/exec.go` and its tests, `internal/ipc/client.go` and its tests, and the U3 broker routing additions in `internal/broker/daemon.go` and `internal/cmd/{execute,execute_test}.go`; leave U1/U2 and unrelated launcher cleanup changes untouched |

#### Gates

- `git diff --check` — exit 0 before commit.
- No full-suite, race, or broad broker-loop command was run in this continuation by resource-safety instruction.

#### Continuation Boundary

U3 was the only implementation unit committed in this continuation before the current apply slice. U5 is implemented but uncommitted; U6 and U7 remain pending. The U7 signal/shutdown behavior and final runtime cleanup gates are still deferred.

### U7.12 — Corrective gate-blocker apply (2026-09-20)

This continuation used the authorized native attempt `acquire-u7-12-fix-20260920-01` and opaque token supplied by the orchestrator. It remained a single-PR `size:exception` work unit. The two confirmed blockers were fixed minimally: production launcher spawning of Go test binaries is refused, and headless agent steps remain CLI-direct when no runner override or feedback is present. The three unprotected command tests now install the established runner override before their first relevant `Execute` call.

#### Corrective TDD Cycle Evidence

| Behavior | Test file | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Test-binary broker spawn guard | `internal/broker/launcher_test.go` | Integration/unit | Pre-fix execution intentionally skipped because it would recursively spawn the test binary | Written before the first guarded test run; pre-fix execution was prohibited by the mandatory recursion safety rule | `haro-fix-1` focused gate passed; helper-process launcher tests also passed | Existing helper-process spawn and `Ensure` scenarios remained green | Guard is limited to `.test` and `.test.exe` executable basenames |
| Headless agent CLI-direct routing | `internal/cmd/agent_wiring_test.go` | Integration/runtime | Pre-fix execution intentionally skipped because unprotected agent tests could recurse through the broker | Regression harness written before the first guarded test run; pre-fix execution was prohibited by the mandatory recursion safety rule | `haro-fix-2` direct-engine regression passed | Broker command routing and existing agent-manager/reopen tests passed in `haro-fix-1` | Store lookup is isolated in a small fail-closed helper |
| Broker-free test setup | `internal/cmd/{agent_wiring,report,logical_conflict}_test.go` | Integration | Pre-fix execution intentionally skipped for the same recursion guard | Override setup was added before the first guarded test run | `haro-fix-1` passed all selected command tests | Existing assertions and handler fake remained unchanged | Only test-scoped runner setup was added |

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `systemd-run --user --scope -u haro-fix-1 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=180s ./internal/broker ./internal/cmd -run 'TestProductionSpawn|TestEnsure|TestAgentManager|TestReportCLI|TestLogicalConflict|TestCLIStep|TestCLIUsesBroker'` — exit 0; broker passed in 0.368s and cmd passed in 0.267s |
| Runtime harness command/scenario and exact result | `systemd-run --user --scope -u haro-fix-2 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=180s ./internal/cmd -run '^TestAgentStepRunUsesDirectEngineWithoutRunnerOverride$'` — exit 0 in 0.038s; direct `execution.Engine.CreateExecution` setup completed the fixture-backed agent step and the broker spy was not called |
| Rollback boundary | Revert only the corrective hunks in `internal/broker/launcher.go`, `internal/broker/launcher_test.go`, `internal/cmd/execute.go`, `internal/cmd/agent_wiring_test.go`, `internal/cmd/report_test.go`, and `internal/cmd/logical_conflict_test.go`; leave all prior U1–U7 implementation and unrelated test changes intact |

#### Final Gate Evidence

| Gate | Exact bounded command | Unit | Result |
|---|---|---|---|
| Focused blocker suite | `systemd-run --user --scope -u haro-fix-1 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=180s ./internal/broker ./internal/cmd -run 'TestProductionSpawn|TestEnsure|TestAgentManager|TestReportCLI|TestLogicalConflict|TestCLIStep|TestCLIUsesBroker'` | `haro-fix-1` | exit 0 |
| Focused direct-agent regression | `systemd-run --user --scope -u haro-fix-2 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=180s ./internal/cmd -run '^TestAgentStepRunUsesDirectEngineWithoutRunnerOverride$'` | `haro-fix-2` | exit 0 |
| Vet | `systemd-run --user --scope -u haro-fix-3 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go vet -p=1 ./...` | `haro-fix-3` | exit 0 |
| Linux build | `systemd-run --user --scope -u haro-fix-4 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go build -p=1 ./...` | `haro-fix-4` | exit 0 |
| Windows cross-build | `systemd-run --user --scope -u haro-fix-5 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB GOOS=windows GOARCH=amd64 go build -p=1 ./...` | `haro-fix-5` | exit 0 |
| Full suite | `systemd-run --user --scope -u haro-fix-6 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=300s ./...` | `haro-fix-6` | exit 0; all packages passed |
| Race suite | `systemd-run --user --scope -u haro-fix-7 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=300s -race ./...` | `haro-fix-7` | exit 1; `internal/broker/TestStartStepAndExecuteAttemptAreSeparated` failed because the lease was still active after the attempt (`steps_test.go:123`); this was a test failure, not an OOM kill, so the documented 3072M retry was not used |

#### Process and Socket Cleanup Evidence

- `bash -lc 'printf "%s\\n" "process cleanup:"; pgrep -af "go test|go build|go vet|haro|\\\\.test" | while IFS= read -r line; do case "$line" in *"pgrep -af"*) ;; *) printf "%s\\n" "$line" ;; esac; done; printf "%s\\n" "socket cleanup:"; ls /tmp/haro-*.sock 2>/dev/null || true'` — no `go test`, `go build`, `go vet`, Haro, or test-binary process remained; the only matching process was the long-lived CodeGraph server; no socket paths were listed.

#### U7.12 Status

The corrective defects and all non-race final gates passed. U7.12 remains unchecked because the required race gate failed in `internal/broker` with the precise lease-cleanup assertion above. No race retry was authorized by the resource rules because the failure was not an OOM kill or wrapper failure. `sdd-verify` remains blocked.

### U7.12 — Race-gate follow-up (2026-09-20)

The confirmed timing flake was corrected only in `internal/broker/steps_test.go`. Production lease ordering and all prior corrective code remain unchanged. The test now waits up to five seconds for both terminal attempt completion and lease expiry, while preserving immediate failures for lease reads and expiry parsing.

#### Follow-up Gate Evidence

| Gate | Exact bounded command | Unit | Result |
|---|---|---|---|
| Focused race shake-out | `systemd-run --user --scope -u haro-fix-8 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 ./internal/broker -run 'TestStartStepAndExecuteAttemptAreSeparated' -count=20 -race -timeout=120s` | `haro-fix-8` | exit 0; 20 race iterations passed in 4.293s |
| Full non-race suite | `systemd-run --user --scope -u haro-fix-9 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=300s ./...` | `haro-fix-9` | exit 1; `internal/broker/TestLinuxBrokerProcessE2EMatrix/F-03_parallel_executions_isolate_state` failed at `broker_e2e_test.go:323` because `step.run` returned attempt `""` and `rpc error -32603: internal error`; other displayed packages passed |
| Full race suite | Not run; stopped after the full non-race gate failed | `haro-fix-10` | Not run |
| Vet | Not run; stopped after the full non-race gate failed | `haro-fix-11` | Not run |
| Linux build | Not run; stopped after the full non-race gate failed | `haro-fix-12` | Not run |
| Windows cross-build | Not run; stopped after the full non-race gate failed | `haro-fix-13` | Not run |

#### Follow-up Cleanup Evidence

- `bash -lc 'printf "%s\\n" "process cleanup:"; pgrep -af "go test|go build|go vet|haro|\\.test" | while IFS= read -r line; do case "$line" in *"pgrep -af"*) ;; *) printf "%s\\n" "$line" ;; esac; done; printf "%s\\n" "socket cleanup:"; ls /tmp/haro-*.sock 2>/dev/null || true'` — no Go/Haro/test-binary processes or Haro sockets remained; only the long-lived CodeGraph server matched the broad process pattern.

#### Follow-up Status

U7.12 remains unchecked. The focused race shake-out passed, but the required full non-race suite failed on the unrelated Linux broker E2E parallel-execution scenario. Per the completion rules, the remaining gates were not launched and no pass is asserted.

### U7.12 — F-03 SQLite concurrency follow-up (2026-09-20)

The modernc.org/sqlite v1.57.0 driver documentation was checked before editing: `_busy_timeout` accepts an integer, and `_txlock` accepts `deferred`, `immediate`, or `exclusive`. `store.Open` now uses `_busy_timeout=5000&_txlock=immediate` while retaining shared cache and `foreign_keys(1)`. Store coverage verifies both pragmas and exercises two independent handles with concurrent lease writes and read-then-write transactions. The F-03 diagnostic assertion now formats RPC errors with `%+v`.

#### F-03 Follow-up Gate Evidence

| Gate | Exact bounded command | Unit | Result |
|---|---|---|---|
| Focused store tests | `systemd-run --user --scope -u haro-fix-14 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 ./internal/store -run 'TestOpenConfiguresSQLiteConcurrencyPragmas|TestSQLiteConcurrentLeasesAndReadThenWriteTransactionsSerialize' -count=5 -timeout=120s` | `haro-fix-14` | exit 0; both new tests passed across 5 repetitions in 0.170s |
| F-03 shake-out | `systemd-run --user --scope -u haro-fix-15 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 ./internal/broker -run 'TestLinuxBrokerProcessE2EMatrix/F-03' -count=10 -timeout=200s` | `haro-fix-15` | exit 1; 3 of 10 repetitions failed at `broker_e2e_test.go:323` with `attempt ""` and `rpc error -32603: internal error`; 7 repetitions passed |
| Full non-race suite | Not run; stopped after F-03 shake-out failure | `haro-fix-16` | Not run |
| Full race suite | Not run; stopped after F-03 shake-out failure | `haro-fix-17` | Not run |
| Vet | Not run; stopped after F-03 shake-out failure | `haro-fix-18` | Not run |
| Linux build | Not run; stopped after F-03 shake-out failure | `haro-fix-19` | Not run |
| Windows cross-build | Not run; stopped after F-03 shake-out failure | `haro-fix-20` | Not run |

#### F-03 Follow-up Cleanup Evidence

- `bash -lc 'printf "%s\\n" "process cleanup:"; pgrep -af "go test|go build|go vet|haro|\\.test" | while IFS= read -r line; do case "$line" in *"pgrep -af"*) ;; *) printf "%s\\n" "$line" ;; esac; done; printf "%s\\n" "socket cleanup:"; ls /tmp/haro-*.sock 2>/dev/null || true'` — no Go/Haro/test-binary processes or Haro sockets remained; only the long-lived CodeGraph server matched the broad process pattern.

#### F-03 Follow-up Status

U7.12 remains unchecked. The new store pragma/concurrency tests pass, but the F-03 shake-out remains intermittently failing despite `_busy_timeout=5000` and `_txlock=immediate`. Per the completion rules, later gates were not launched and no pass is asserted.

### U7.12 — F-03 diagnostic observability follow-up (2026-09-20)

Diagnostics were added without changing the suspected root cause: broker handler panic recovery now logs method, panic value, and stack to stderr; F-03 reports `ipc.RemoteError.Data`; and all four direct Linux broker launches route stderr to the test process. No root-cause fix was attempted.

#### Diagnostic Shake-out Evidence

| Run | Exact bounded command | Unit | Result |
|---|---|---|---|
| Non-race | `systemd-run --user --scope -u haro-diag-1 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 ./internal/broker -run 'TestLinuxBrokerProcessE2EMatrix/F-03' -count=20 -timeout=240s > /tmp/opencode/f03-diag-nonrace.log 2>&1` | `haro-diag-1` | exit 1; 4/20 repetitions failed, 16 passed |
| Race | `systemd-run --user --scope -u haro-diag-2 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 ./internal/broker -run 'TestLinuxBrokerProcessE2EMatrix/F-03' -count=5 -race -timeout=180s > /tmp/opencode/f03-diag-race.log 2>&1` | `haro-diag-2` | exit 1; 3/5 repetitions failed, 2 passed |

#### Raw Diagnostic Excerpts

Non-race `/tmp/opencode/f03-diag-nonrace.log` first failure and repeated detail:

```text
broker_e2e_test.go:327: parallel step.run for 1fb7dcb2-15ea-404c-81b9-0a89dc11037c = attempt "", err=rpc error -32603: internal error data=map[detail:begin tx: database table is locked (262)]
broker_e2e_test.go:327: parallel step.run for d655d8dd-b7e9-4d2c-9116-0804feec8530 = attempt "", err=rpc error -32603: internal error data=map[detail:begin tx: database table is locked (262)]
broker_e2e_test.go:327: parallel step.run for 0fa27dcf-8968-4123-a8e1-e903f6742ebe = attempt "", err=rpc error -32603: internal error data=map[detail:begin tx: database table is locked (262)]
broker_e2e_test.go:327: parallel step.run for 7aa9e658-6885-4e97-8b37-dbb880f8f19f = attempt "", err=rpc error -32603: internal error data=map[detail:begin tx: database table is locked (262)]
```

Race `/tmp/opencode/f03-diag-race.log` first failure and repeated detail:

```text
broker_e2e_test.go:327: parallel step.run for 9894aa4b-e5e3-4de7-9e5f-df7cd821256f = attempt "", err=rpc error -32603: internal error data=map[detail:begin tx: database table is locked (262)]
broker_e2e_test.go:327: parallel step.run for 476700a4-fc02-4721-8039-1d05d4bd176e = attempt "", err=rpc error -32603: internal error data=map[detail:begin tx: database table is locked (262)]
broker_e2e_test.go:327: parallel step.run for 3c1f31bf-2765-4df3-811f-17a2f314aa43 = attempt "", err=rpc error -32603: internal error data=map[detail:begin tx: database table is locked (262)]
```

- `recovered panic` excerpts: none in either log.
- `DATA RACE` report: none in either log.
- Finding only, no fix attempted: the opaque `-32603` is backed by `begin tx: database table is locked (262)`, not a recovered handler panic.

#### Diagnostic Cleanup Evidence

- `bash -lc 'printf "%s\\n" "process cleanup:"; pgrep -af "go test|go build|go vet|haro|\\.test" | while IFS= read -r line; do case "$line" in *"pgrep -af"*) ;; *) printf "%s\\n" "$line" ;; esac; done; printf "%s\\n" "socket cleanup:"; ls /tmp/haro-*.sock 2>/dev/null || true'` — no Go/Haro/test-binary processes or Haro sockets remained; only the long-lived CodeGraph server matched the broad process pattern.

#### Diagnostic Status

U7.12 remains unchecked. Observability now exposes the underlying SQLite lock error, but no root-cause fix was attempted in this step.

### U7.12 — F-03 SQLite lock fix and final gates (2026-09-20)

The diagnostic detail identified the intermittent `-32603` as a shared-cache SQLite `SQLITE_LOCKED` error at transaction begin. The minimal production fix limits each broker-owned `*sql.DB` to one open connection and serializes schema/WAL migration setup across concurrent store handles. This preserves concurrent command execution while preventing one broker's RPC handlers from opening competing SQLite connections. No provider or acceptance documents were changed.

#### Corrective TDD Cycle Evidence

| Behavior | Test file | RED | GREEN | REFACTOR |
|---|---|---|---|---|
| Broker store connection serialization | `internal/store/store_test.go` | `haro-fix-21`: `TestOpenLimitsSQLiteConnectionsForBrokerSerialization` failed with `MaxOpenConnections = 0, want 1` | `haro-fix-22`: SQLite pragma, connection-limit, and concurrent lease/read-then-write tests passed 5 repetitions | `gofmt` and `git diff --check` passed; production comment documents why external command execution remains concurrent |
| Concurrent migration setup | Existing `internal/store/{claim,migrations_payload}_test.go` coverage | `haro-fix-30`: full race suite exposed `migrate: database table is locked (262)` during concurrent opens | `haro-fix-31`: both migration/concurrency selectors passed 5 race repetitions | `sqliteOpenMu` scopes only Open-time schema/WAL setup and does not serialize normal store operations |

The one-connection configuration exposed two existing test-only misuse patterns: an open event `Rows` was held while issuing another query, and a transaction callback used `db.ExecContext` instead of the transaction. These were corrected in `internal/execution/agent_evidence_test.go` and `internal/store/transport_test.go` respectively; no production behavior was changed for either case.

#### F-03 Corrective Evidence

| Run | Exact bounded command | Unit | Result |
|---|---|---|---|
| Non-race | `systemd-run --user --scope -u haro-fix-23 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 ./internal/broker -run 'TestLinuxBrokerProcessE2EMatrix/F-03' -count=20 -timeout=240s` | `haro-fix-23` | exit 0; 20/20 repetitions passed in 23.797s |
| Race | `systemd-run --user --scope -u haro-fix-24 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 ./internal/broker -run 'TestLinuxBrokerProcessE2EMatrix/F-03' -count=5 -race -timeout=180s` | `haro-fix-24` | exit 0; 5/5 repetitions passed in 7.533s |

#### Final Gate Evidence

| Gate | Exact bounded command | Unit | Result |
|---|---|---|---|
| Full non-race suite | `systemd-run --user --scope -u haro-fix-38 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 360s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=300s ./...` | `haro-fix-38` | exit 0; all packages passed on the final code state |
| Full race suite | `systemd-run --user --scope -u haro-fix-32 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 420s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=360s -race ./...` | `haro-fix-32` | exit 0; all packages passed with no race report |
| Vet | `systemd-run --user --scope -u haro-fix-33 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go vet -p=1 ./...` | `haro-fix-33` | exit 0 |
| Linux build | `systemd-run --user --scope -u haro-fix-34 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go build -p=1 ./...` | `haro-fix-34` | exit 0 |
| Windows cross-build | `systemd-run --user --scope -u haro-fix-35 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 240s env GOMAXPROCS=1 GOMEMLIMIT=384MiB GOOS=windows GOARCH=amd64 go build -p=1 ./...` | `haro-fix-35` | exit 0 |
| Vulnerability scan | `systemd-run --user --scope -u haro-fix-37 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 360s env GOMAXPROCS=1 GOMEMLIMIT=384MiB govulncheck ./...` | `haro-fix-37` | exit 0; no vulnerabilities found |

`golangci-lint run` was also attempted as `haro-fix-36` and exited 1 on 21 existing findings (16 errcheck, 1 govet, 3 staticcheck, 1 unused) across pre-existing files; these findings are outside U7.12's listed gate commands and were not broadened into this fix.

#### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `haro-fix-22` store selectors, `haro-fix-23` F-03 non-race 20/20, and `haro-fix-24` F-03 race 5/5 all exited 0 |
| Runtime harness command/scenario and exact result | `haro-fix-23`/`haro-fix-24` ran the real built-binary Linux broker process matrix's F-03 path through UDS with two parallel executions; all corrective repetitions passed |
| Rollback boundary | Revert the new connection/migration setup in `internal/store/store.go`, remove `TestOpenLimitsSQLiteConnectionsForBrokerSerialization`, revert the test resource-lifetime corrections in `internal/execution/agent_evidence_test.go` and `internal/store/transport_test.go`, and retain prior broker/U7 work |

#### Final Cleanup

- `pgrep -af "go test|go build|go vet|golangci-lint|govulncheck|haro|\\.test"` — no Go/Haro/test/lint processes remained; only the long-lived CodeGraph server matched.
- `ls /tmp/haro-*.sock` — no Haro sockets remained.

#### U7.12 Status

All 39 implementation tasks are complete and the required final gates pass. U7.12 is marked `[x]` in `tasks.md`; the change is ready for `sdd-verify`.

### Remediation — broker IPC verification findings (2026-09-20)

Native remediation attempt: `acquire-u7-remediate-20260920-01`.
The immutable verification report was not modified, `tasks.md` was not
modified, and the verifier was not rerun. The existing 39/39 task history is
preserved.

#### Remediation TDD Cycle Evidence

| Finding | Test-first evidence | GREEN | REFACTOR |
|---|---|---|---|
| C-01 / `v2-ipc/U-04` outgoing results | `haro-rem-focused-20260920-01` failed because an invalid `execution.start` result was encoded as a JSON-RPC result | Dispatcher validation and malformed-result table passed in `haro-rem-c01-test-20260920-05` | `gofmt`, `git diff --check`, full non-race/race suites, vet, and builds passed |
| W-02 endpoint collision | Initial collision test failed because the occupied endpoint was reused; the test was corrected to model a non-0600 foreign occupant while preserving the existing safe-0600 zombie contract | `TestSocketPathExtendsHashForForeignOccupant` and full IPC package passed | Prefix remains bounded, long-TMPDIR fallback remains intact, and existing zombie recovery passed |
| W-03 notification E2E | `TestSubscribedServerReceivesPersistedStatusAndInteractionNotifications` was added before the notification field refinement | Net-pipe `ServeConn` + `Hub` + SQLite persistence test passed in `haro-rem-w02-w03-20260920-01` and full broker suite | Interaction notification now includes persisted attempt `execution_id` and `step_id`; cursor order and commit-before-visible path remain covered |

#### Remediation Changes

- C-01: `Dispatcher.Dispatch` now validates the encoded wire shape for every
  registered production method: `health`, `execution.start/status`, and all
  five `step.*` methods. Unknown, missing, malformed, invalid-enum, negative
  cursor, oversized inline payload, and mixed payload/payload_ref fields fail
  with `-32602` and stable `result...` field paths before response encoding.
- W-02: `SocketPath` checks endpoint occupancy and private metadata, extends
  the SHA-256 prefix in bounded increments for foreign/non-0600 occupants,
  preserves safe 0600 stale endpoint recovery, and retains the 108-byte UDS
  bound plus short-prefix long-TMPDIR fallback.
- W-03: `notify_test.go` now drives persisted status and interaction events
  through one subscribed `ServeConn`/`Hub` client and verifies cursor order,
  fields, and persisted interaction evidence. The interaction notification
  includes `execution_id` and `step_id` from the bound attempt.
- W-01: the active IPC delta now contains a non-normative traceability note
  stating that the delta adds 12 IPC requirements and inherits the existing
  base `openspec/specs/v2-ipc/spec.md` requirement `v2-ipc/U-03`, yielding the
  13-criterion IPC trace without changing normative acceptance text.

#### Work Unit Evidence

| Evidence | Exact command / scenario | Result |
|---|---|---|
| Focused tests | `systemd-run --user --scope --quiet --unit=haro-rem-focused-20260920-06 -p MemoryMax=2560M -p MemorySwapMax=0 env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 ./internal/broker ./internal/ipc -count=1` | exit 0; broker and IPC packages passed |
| C-01 runtime harness | `go test -p=1 ./internal/broker -run 'TestServerRejectsInvalidOutgoingResultWithoutSendingPayload|TestDispatcherRejectsMalformedOutgoingResults' -count=1` under bounded cgroup | exit 0; invalid results became `-32602` errors and valid results round-tripped unchanged |
| W-03 runtime harness | `go test -p=1 ./internal/broker -run '^TestSubscribedServerReceivesPersistedStatusAndInteractionNotifications$' -count=1` under bounded cgroup | exit 0; persisted status and interaction notifications arrived through one subscribed server connection in cursor order |
| Full non-race | `systemd-run --user --scope --quiet --unit=haro-rem-full-20260920-01 -p MemoryMax=2560M -p MemorySwapMax=0 env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 ./...` | exit 0; all packages passed |
| Full race | First run hit an intermittent existing `internal/store/TestAcquireConcurrency` SQLite lock; rerun `haro-rem-race-20260920-04` exited 0 | final race suite passed with no race report |
| Static/build/security gates | Bounded `go vet ./...`, Linux `go build -p=1 ./...`, Windows `GOOS=windows GOARCH=amd64 go build -p=1 ./...`, and `govulncheck ./...` | all exit 0; no vulnerabilities found |

#### Rollback Boundary

Revert only the remediation changes in `internal/broker/handlers.go`,
`internal/broker/validation.go`, `internal/broker/result_validation_test.go`,
`internal/broker/runtime.go`, `internal/broker/notify_test.go`,
`internal/ipc/socket.go`, `internal/ipc/socket_test.go`, and the
non-normative traceability note in the active IPC delta spec. Preserve all
prior U1–U7.12 implementation, tests, task checkboxes, and verification
report history.

#### Remediation Status

C-01, W-01, W-02, and W-03 remediation is implemented and locally verified.
The change is ready for the orchestrator to run the next verification phase;
this apply unit did not launch `sdd-verify`.

Post-hardening focused rerun: `haro-rem-c01-test-20260920-06` exited 0 after
rejecting nullable optional result strings as malformed values.

### Hardening — remove SQLite shared-cache locking (2026-09-20)

Same native remediation attempt: `acquire-u7-remediate-20260920-01`.
No reset, acquire, commit, push, task-artifact edit, verification-report edit,
or verifier run was performed. The only production change in this hardening
step is the SQLite DSN in `internal/store/store.go`:

`file:%s?_busy_timeout=5000&_txlock=immediate&_pragma=foreign_keys(1)`

The DSN comment now documents WAL, normal file-level locking,
`busy_timeout`, and immediate transactions, and explains that deprecated
shared-cache mode was removed because its table locks return `SQLITE_LOCKED`
without busy-timeout retry. `SetMaxOpenConns(1)` and `sqliteOpenMu` remain.

#### Hardening Evidence

All commands used `MemoryMax=2560M`, `MemorySwapMax=0`, `GOMAXPROCS=1`,
`GOMEMLIMIT=384MiB`, `-p=1`, and wrote logs to `/tmp/opencode/`.

| Unit | Exact command | Exact result |
|---|---|---|
| `haro-lock-1` | `systemd-run --user --scope -u haro-lock-1 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 600s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=50 -race -run 'TestAcquireConcurrency' ./internal/store` | exit 0; `internal/store` passed in 18.480s |
| `haro-lock-2` | `systemd-run --user --scope -u haro-lock-2 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 600s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=5 -race ./internal/store` | exit 0; full store race package passed in 30.611s |
| `haro-lock-3` | `systemd-run --user --scope -u haro-lock-3 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 600s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=10 -run 'TestLinuxBrokerProcessE2EMatrix/F-03' ./internal/broker` | exit 0; F-03 non-race shakeout passed in 12.261s |
| `haro-lock-4` | `systemd-run --user --scope -u haro-lock-4 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 600s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=5 -race -run 'TestLinuxBrokerProcessE2EMatrix/F-03' ./internal/broker` | exit 0; F-03 race shakeout passed in 5.846s |
| `haro-lock-5` | `systemd-run --user --scope -u haro-lock-5 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 600s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 ./...` | exit 0; all packages passed |
| `haro-lock-6` | `systemd-run --user --scope -u haro-lock-6 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 900s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -race ./...` | exit 0; first clean full race pass, all packages passed |
| `haro-lock-7` | `systemd-run --user --scope -u haro-lock-7 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 900s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -race ./...` | exit 0; second consecutive clean full race pass, all packages passed |
| `haro-lock-8` | `systemd-run --user --scope -u haro-lock-8 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 600s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go vet -p=1 ./...` | exit 0 |
| `haro-lock-9` | `systemd-run --user --scope -u haro-lock-9 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 600s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go build -p=1 ./...` | exit 0 |
| `haro-lock-10` | `systemd-run --user --scope -u haro-lock-10 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 600s env GOMAXPROCS=1 GOMEMLIMIT=384MiB GOOS=windows GOARCH=amd64 go build -p=1 ./...` | exit 0 |

#### Hardening Rollback Boundary

Revert only the DSN and explanatory comment changes in
`internal/store/store.go`. Keep `SetMaxOpenConns(1)`, `sqliteOpenMu`, all
prior broker remediation, task history, and verification artifacts intact.

#### Hardening Status

The shared-cache SQLite locking defect class is addressed. All required
hardening evidence units passed, including two consecutive clean full race
runs. The change remains ready for orchestrator verification; this unit did
not run `sdd-verify`.
