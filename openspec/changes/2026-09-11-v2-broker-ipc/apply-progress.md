# Apply Progress: v2 Broker IPC

Date: 2026-09-13 · Mode: Strict TDD (bounded focused Go tests) · Delivery: `single-pr` with maintainer-approved `size:exception` against the default 400-line threshold and a 20,000-line review budget · Direct commits to `main`, not pushed.

## Status

13/39 tasks complete. U1, U2, and U3 are complete; U4 is the next work unit. The dispatcher supplied `applyState: ready` for this continuation.

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

## Remaining Tasks

- [x] 1.1 through 1.6 — socket identity, transport seam, daemon, launcher, and broker entrypoint
- [x] 2.1 through 2.4 — RPC server and strict validation
- [x] 3.1 through 3.3 — execution start/status and client wiring
- [ ] 4.1 through 4.5 — asynchronous step run and event projection
- [ ] 5.1 through 5.5 — approval CAS and cancellation
- [ ] 6.1 through 6.4 — reopen cascade and feedback
- [ ] 7.1 through 7.12 — fencing, notifications, shutdown, E2E, and final gates

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
| Focused test command and exact result | `timeout 90s env GOMAXPROCS=2 GOMEMLIMIT=512MiB go test -p=1 -count=1 -timeout=60s ./internal/broker ./internal/ipc ./internal/cmd -run 'TestExecution\|TestClient\|TestCLIUsesBroker'` — exit 0; broker 0.057s, IPC 0.007s, command 0.006s |
| Runtime harness command/scenario and exact result | Bounded built-binary harness started `haro broker --project <temp git repo>`, then ran `haro run demo --json` and `haro status <execution_id> --json` through the Unix socket — functional exit 0; execution ID, aggregate `running`, and step `pending` matched. TERM cleanup exposed the pending U7 signal-shutdown gap and left one safe 0600 orphan socket; no broker process remained, and the socket was removed after verification |
| Rollback boundary | Revert `dd9b980`: remove `internal/broker/exec.go` and its tests, `internal/ipc/client.go` and its tests, and the U3 broker routing additions in `internal/broker/daemon.go` and `internal/cmd/{execute,execute_test}.go`; leave U1/U2 and unrelated launcher cleanup changes untouched |

#### Gates

- `git diff --check` — exit 0 before commit.
- No full-suite, race, or broad broker-loop command was run in this continuation by resource-safety instruction.

#### Continuation Boundary

U3 was the only implementation unit committed in this continuation. U4–U7 remain pending because the runtime cleanup check exposed the not-yet-implemented U7 signal/shutdown behavior; no further broker tests or implementation were started after that finding.
