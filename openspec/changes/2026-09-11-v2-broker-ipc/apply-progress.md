# Apply Progress: v2 Broker IPC

Date: 2026-09-12 · Mode: Strict TDD (`go test ./...`) · Delivery: `single-pr` with maintainer-approved `size:exception` (200000-line review budget) · Direct commits to `main`, not pushed.

## Status

6/41 tasks complete. U1 is complete; U2 is the next work unit.

## Completed Work Units

### U1 — Socket identity + daemon/launcher lifecycle

Commit: pending at artifact write; intended commit message: `feat(broker): socket identity, transport seam (go-winio), daemon and launcher lifecycle`

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

## Remaining Tasks

- [ ] 2.1 through 2.4 — RPC server and strict validation
- [ ] 3.1 through 3.3 — execution start/status and client wiring
- [ ] 4.1 through 4.5 — asynchronous step run and event projection
- [ ] 5.1 through 5.5 — approval CAS and cancellation
- [ ] 6.1 through 6.4 — reopen cascade and feedback
- [ ] 7.1 through 7.12 — fencing, notifications, shutdown, E2E, and final gates
