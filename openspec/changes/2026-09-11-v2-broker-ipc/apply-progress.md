# Apply Progress: v2 Broker IPC

Date: 2026-09-13 · Mode: Strict TDD (bounded focused Go tests) · Delivery: `single-pr` with maintainer-approved `size:exception` against the default 400-line threshold and a 20,000-line review budget · Direct commits to `main`, not pushed.

## Status

38/39 tasks complete. U1 through U6 and U7 lease/fencing/lifecycle/notification/publish slices are applied; U7.11 is complete and U7.12 remains pending because the host guardrail prohibits the full-suite and race commands. The dispatcher supplied `applyState: ready` for this continuation.

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
