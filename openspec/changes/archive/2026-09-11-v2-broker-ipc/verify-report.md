```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:8ae89d4fce6fecca706fd6fcbb7a6b785deebd3c2db6399cf635c898d78d0075
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 21/21
scenarios: 21/21
test_command: "systemd-run --user --scope -u haro-reverify-focused-20260920-01 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=300s ./internal/broker ./internal/ipc ./internal/store -run 'Test(DispatcherValidatesProductionResultSchemas|ServerRejectsInvalidOutgoingResultWithoutSendingPayload|DispatcherRejectsMalformedOutgoingResults|DecodeParams|ValidateEnum|SocketPath(CanonicalEquivalents|DistinctRoots|ExtendsHashForForeignOccupant|UnixLength|LongRootSafe|WorktreeCoherence|WindowsShape)|SubscribedServerReceivesPersistedStatusAndInteractionNotifications|OpenConfiguresSQLiteConcurrencyPragmas|OpenLimitsSQLiteConnectionsForBrokerSerialization|SQLiteConcurrentLeasesAndReadThenWriteTransactionsSerialize)$'"
test_exit_code: 0
test_output_hash: sha256:00236e9746310a8a7f8e4d1628c50e0f52109b9438bac92ecfc26eb973e6c5dc
build_command: "systemd-run --user --scope -u haro-reverify-build-20260920-01 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go build -p=1 ./..."
build_exit_code: 0
build_output_hash: sha256:e2895f90931f0c43dacc6f69e24fde6219a29b25d678ae98148d231c0a17390d
```

## Verification Report

**Change**: `2026-09-11-v2-broker-ipc`  
**Version**: v2  
**Mode**: Strict TDD  
**Verification**: Independent re-verification after remediation; no normative files were modified.

### Completeness

| Metric | Value |
|---|---:|
| Requirements retrieved | 21 |
| Scenarios retrieved | 21 |
| Tasks total | 39 |
| Tasks complete | 39 |
| Tasks incomplete | 0 |
| Proposal/specs/design/tasks/apply-progress | Complete |

The native status reports 39/39 tasks complete and the active attempt is the authorized `u7-reverify` continuation. The authoritative delta count is 9 `v2-broker` requirements plus 12 `v2-ipc` delta requirements. The active IPC spec now records inherited base `v2-ipc/U-03` separately; it is not added to the 21 retrieved delta requirements.

### Build and test evidence

All fresh runtime commands used unique user-scope cgroup units, `MemoryMax=2560M`, `MemorySwapMax=0`, `GOMAXPROCS=1`, `GOMEMLIMIT=384MiB`, `-p=1`, and bounded `timeout` values.

| Scope | Unit | Exit | Output hash | Result |
|---|---|---:|---|---|
| Focused remediation selectors | `haro-reverify-focused-20260920-01` | 0 | `sha256:00236e9746310a8a7f8e4d1628c50e0f52109b9438bac92ecfc26eb973e6c5dc` | Broker, IPC, and store packages passed. |
| C-01 result validation | `haro-reverify-c01-20260920-02` | 0 | `sha256:d4d8ab1fe9103bf1dd988a05bd356032786ee62b2a559de47f7ce5de4e52a87a` | Valid production shapes round-trip; malformed results return `-32602` with field paths and emit no result payload. |
| W-02 socket collision | `haro-reverify-w02-20260920-01` | 0 | `sha256:f18ed70f65ca80e1d2a3e4f627c98a50090d953003b2e8dceb539f26d7d5f54c` | Foreign non-0600 occupancy extends the hash and remains within the UDS bound. |
| W-03 notification E2E | `haro-reverify-w03-20260920-01` | 0 | `sha256:3ae2f59dd25c81547c6af01a25c82eac4038cb31c6ba4fb2c7b49ad1e7df88fc` | One subscribed `ServeConn`/`Hub` client received persisted status and interaction notifications in cursor order. |
| SQLite hardening | `haro-reverify-store-20260920-01` | 0 | `sha256:e37b7de033fcddc38950c3124085ac305e25a48bfd38bbea33fbcfe6e0a037be` | Busy-timeout, one-connection serialization, and concurrent lease/read-then-write coverage passed. |
| Linux build | `haro-reverify-build-20260920-01` | 0 | `sha256:e2895f90931f0c43dacc6f69e24fde6219a29b25d678ae98148d231c0a17390d` | Passed. |
| Vet | `haro-reverify-vet-20260920-01` | 0 | `sha256:ed6f6d111a6179c66e6d01d856eeb0a329f86a5a1aad8ace9f898b7c0d590d17` | Passed. |
| Project lint | `haro-reverify-lint-20260920-01` | 1 | `sha256:076538537e65ca24549384bae3b06c5f3c20f5df79ba693484ffbc7b812a608a` | 21 findings: 16 errcheck, 1 govet, 3 staticcheck, 1 unused; the same project-wide pre-existing set remains. |

Exact commands for the table above:

```text
systemd-run --user --scope -u haro-reverify-focused-20260920-01 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=300s ./internal/broker ./internal/ipc ./internal/store -run 'Test(DispatcherValidatesProductionResultSchemas|ServerRejectsInvalidOutgoingResultWithoutSendingPayload|DispatcherRejectsMalformedOutgoingResults|DecodeParams|ValidateEnum|SocketPath(CanonicalEquivalents|DistinctRoots|ExtendsHashForForeignOccupant|UnixLength|LongRootSafe|WorktreeCoherence|WindowsShape)|SubscribedServerReceivesPersistedStatusAndInteractionNotifications|OpenConfiguresSQLiteConcurrencyPragmas|OpenLimitsSQLiteConnectionsForBrokerSerialization|SQLiteConcurrentLeasesAndReadThenWriteTransactionsSerialize)$'
systemd-run --user --scope -u haro-reverify-c01-20260920-02 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=300s ./internal/broker -run 'Test(DispatcherValidatesProductionResultSchemas|ServerRejectsInvalidOutgoingResultWithoutSendingPayload|DispatcherRejectsMalformedOutgoingResults|DecodeParams|ValidateEnum)'
systemd-run --user --scope -u haro-reverify-w02-20260920-01 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=300s ./internal/ipc -run 'TestSocketPath(CanonicalEquivalents|DistinctRoots|ExtendsHashForForeignOccupant|UnixLength|LongRootSafe|WorktreeCoherence|WindowsShape)'
systemd-run --user --scope -u haro-reverify-w03-20260920-01 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=300s ./internal/broker -run '^TestSubscribedServerReceivesPersistedStatusAndInteractionNotifications$'
systemd-run --user --scope -u haro-reverify-store-20260920-01 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go test -p=1 -count=1 -timeout=300s ./internal/store -run 'Test(OpenConfiguresSQLiteConcurrencyPragmas|OpenLimitsSQLiteConnectionsForBrokerSerialization|SQLiteConcurrentLeasesAndReadThenWriteTransactionsSerialize)$'
systemd-run --user --scope -u haro-reverify-build-20260920-01 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go build -p=1 ./...
systemd-run --user --scope -u haro-reverify-vet-20260920-01 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB go vet -p=1 ./...
systemd-run --user --scope -u haro-reverify-lint-20260920-01 -p MemoryMax=2560M -p MemorySwapMax=0 -- timeout 300s env GOMAXPROCS=1 GOMEMLIMIT=384MiB golangci-lint run ./...
```

The apply evidence records `haro-lock-5` full non-race tests exit 0 and two consecutive clean full race suites (`haro-lock-6`, `haro-lock-7`) exit 0 after removing SQLite shared-cache mode. Those full suites were not repeated blindly in this re-verification; the fresh selectors above directly cover each remediation. Coverage analysis is skipped because `openspec/config.yaml` declares coverage unavailable and no threshold.

### Spec compliance matrix

| Requirement | Scenario | Runtime evidence | Result |
|---|---|---|---|
| `v2-broker/F-01` | Shared project broker | Recorded `TestLinuxBrokerProcessE2EMatrix/F-01 shared broker serves two CLIs` | COMPLIANT |
| `v2-broker/F-02` | Relaunch after death | Recorded `TestLinuxBrokerProcessE2EMatrix/F-02 relaunches after broker death` | COMPLIANT |
| `v2-broker/F-03` | Parallel executions | Recorded 20/20 non-race and 5/5 race F-03 shakeouts; full suites passed after SQLite hardening | COMPLIANT |
| `v2-broker/F-04` | Isolated projects | Recorded `TestLinuxBrokerProcessE2EMatrix/F-04 independent projects survive peer shutdown` | COMPLIANT |
| `v2-broker/F-05` | Launch herd | Recorded `TestLinuxBrokerProcessE2EMatrix/F-05 launch herd starts one process` | COMPLIANT |
| `v2-broker/F-06` | Stale writer after death | Recorded `TestLinuxBrokerProcessE2EMatrix/F-06 kill nine fences stale writer` | COMPLIANT |
| `v2-broker/F-07` | Immediate restart | Recorded `TestDaemonShutdownRemovesSocketAndAllowsReplacement` and Linux process matrix | COMPLIANT |
| `v2-broker/U-01` | Canonical path table | Recorded socket path table; fresh W-02 selector passed | COMPLIANT |
| `v2-broker/U-02` | Recoverable framing failures | Recorded server malformed/oversized/concurrent frame tests | COMPLIANT |
| `v2-ipc/F-01` | Workflow validation | Recorded execution-start path, invalid-workflow, and zero-row tests | COMPLIANT |
| `v2-ipc/F-02` | In-progress execution | Recorded persisted aggregate/status tests and Linux matrix | COMPLIANT |
| `v2-ipc/F-03` | Async and guarded modes | Recorded async/mode-guard tests and Linux matrix | COMPLIANT |
| `v2-ipc/F-04` | Pagination retry | Recorded stable current-attempt pagination tests and Linux matrix | COMPLIANT |
| `v2-ipc/F-05` | Approval gate | Recorded CAS/available-decision/permission-wait tests | COMPLIANT |
| `v2-ipc/F-06` | Active cancellation | Recorded cancellation, persistence, and lease invalidation tests | COMPLIANT |
| `v2-ipc/F-07` | Cascade | Recorded stable reopen cascade and execution coverage | COMPLIANT |
| `v2-ipc/F-08` | Feedback | Recorded complete next-attempt feedback delivery test | COMPLIANT |
| `v2-ipc/F-09` | Subscription | Fresh `TestSubscribedServerReceivesPersistedStatusAndInteractionNotifications` plus prior Hub/EventSink tests | COMPLIANT |
| `v2-ipc/U-01` | Publication crash | Recorded pre/post-commit EventSink failpoint tests | COMPLIANT |
| `v2-ipc/U-02` | CAS table | Recorded approval CAS table and interaction repository tests | COMPLIANT |
| `v2-ipc/U-04` | Invalid payloads | Fresh outgoing-result, inbound `DecodeParams`, enum, and no-payload tests | COMPLIANT |

**Compliance summary**: 21/21 requirements and 21/21 scenarios compliant.

### C-01 outgoing validation assessment

`v2-ipc/U-04` requires incoming and outgoing validation, `-32602`, stable messages and field paths, and fail-closed behavior without effects. `DecodeParams` remains the pre-handler gate, while `Dispatcher.Dispatch` invokes `validateResult` at `internal/broker/handlers.go:57-60`, before `Server.ServeConn` reaches `EncodeResponse` at `internal/broker/server.go:95-102`. The validator covers every production registration: `health`, `execution.start`, `execution.status`, `step.run`, `step.events`, `step.approve`, `step.cancel`, and `step.reopen`.

The result schemas match current production shapes: non-empty IDs, status enums, non-negative cursors, strict nested fields, mutually exclusive bounded `payload`/`payload_ref`, booleans, and the empty cancel object. `TestDispatcherValidatesProductionResultSchemas` proves valid round-trips. `TestServerRejectsInvalidOutgoingResultWithoutSendingPayload` proves an unknown outgoing field becomes `-32602` with `result.unexpected`, emits no result payload, and leaves the connection usable for a subsequent valid result. `TestDispatcherRejectsMalformedOutgoingResults` covers invalid enum, negative cursor, missing field, and nested unknown field. Incoming invalid payloads remain side-effect free because they are rejected before handler invocation.

### Correctness

| Area | Status | Evidence |
|---|---|---|
| JSON-RPC incoming and outgoing boundary | Implemented | Dispatcher result schemas and fresh C-01 selectors pass; all production methods are enumerated. |
| Per-project lifecycle, fencing, and shutdown | Implemented | Prior Linux process matrix, fencing tests, and final gates remain recorded as passing. |
| Endpoint identity and collision handling | Implemented | `SocketPath` extends bounded SHA-256 prefixes for foreign/non-0600 occupants; fresh W-02 selectors pass. |
| Notifications and persisted ordering | Implemented | Fresh subscribed `ServeConn`/`Hub`/SQLite test verifies status and interaction fields and cursor order. |
| Async execution and SQLite concurrency | Implemented | Full non-race/race evidence after shared-cache removal plus fresh store selectors pass. |

### Design coherence

| Decision | Result | Notes |
|---|---|---|
| Canonical endpoint with bounded collision extension | Yes | Current `socket.go` preserves canonical hashing, safe 0600 stale handling, and bounded prefix extension. |
| Strict RPC result validation before encoding | Yes | Dispatcher is the common final gate for all production response methods. |
| Persist-before-fanout notifications | Yes | EventSink commits before Hub broadcast; the new subscribed test observes persisted status and interaction events. |
| One WAL store with bounded SQLite concurrency | Yes | `cache=shared` is removed; `_busy_timeout`, `_txlock=immediate`, `SetMaxOpenConns(1)`, and serialized open-time migration remain. |
| Async, fencing, CAS, pagination, and shutdown decisions | Yes | No regression in the recorded final gates or the focused remediation selectors. |

### Strict TDD compliance

| Check | Result | Details |
|---|---|---|
| TDD evidence reported | PASS | `apply-progress.md` contains task-cycle tables and remediation/hardening evidence. |
| Test-bearing tasks have files | PASS | 39/39 completed tasks have recorded test or gate evidence; remediation files exist. |
| RED confirmed | PASS WITH DOCUMENTATION NOTE | RED evidence is present for the implementation and remediation cycles; some rows use descriptive `✅` text rather than the literal `✅ Written` label. |
| GREEN confirmed | PASS | Fresh focused selectors, build, vet, and recorded full non-race/race gates pass. |
| Triangulation adequate | PASS | C-01 has valid round-trip, no-payload, malformed enum/missing/nested cases; W-02 and W-03 each have behavioral regression coverage. |
| Safety net | PASS | Existing lifecycle, store, and broker suites are recorded as safety nets; focused remediation tests pass now. |

**TDD Compliance**: 6/6 checks pass on runtime substance; the RED-label wording note is non-blocking.

### Test layer distribution

| Layer | Representative changed/remediation files | Tools |
|---|---|---|
| Unit/repository | `internal/broker/result_validation_test.go`, `internal/ipc/socket_test.go`, `internal/store/store_test.go` | Go `testing`, SQLite repository tests |
| Integration | `internal/broker/notify_test.go`, validation/server/interaction tests, command and execution regression tests | Go `testing`, `net.Pipe`, SQLite, fake runners |
| E2E | `internal/broker/broker_e2e_test.go` | Go `testing`, built Haro subprocesses, Linux UDS |

### Changed file coverage

Coverage analysis skipped — no coverage tool is configured (`openspec/config.yaml:31`). This is informational and not a failure.

### Assertion quality

Reviewed remediation tests call production code before asserting behavior. No tautologies, ghost loops, empty-only defects, smoke-only checks, implementation-detail assertions, or mock-heavy assertion defects were found. The C-01 tests assert actual wire/error behavior; W-02 asserts endpoint selection; W-03 asserts notification fields, order, and persisted events; store tests assert real SQLite concurrency behavior.

### Previous finding closure table

| Previous finding | Status | Current evidence |
|---|---|---|
| C-01 critical — missing outgoing result validation | CLOSED | `validateResult` is wired before response encoding; fresh C-01 selectors pass valid round-trips, stable field paths, `-32602`, no invalid result bytes, and connection recovery. |
| W-01 — IPC traceability count inconsistency | CLOSED | Active `v2-ipc/spec.md` now explicitly identifies 12 delta requirements plus inherited base `v2-ipc/U-03`; authoritative retrieved count remains 21. |
| W-02 — missing endpoint collision extension | CLOSED | `SocketPath` checks occupancy/0600 metadata and extends the bounded hash; `TestSocketPathExtendsHashForForeignOccupant` passes. |
| W-03 — split notification coverage | CLOSED | One subscribed `ServeConn`/`Hub`/SQLite test receives persisted status and interaction notifications with persisted execution/step IDs and cursor order. |
| W-04 — project-wide lint not green | OPEN | Fresh lint exits 1 with the same 21 project-wide findings; no remediation introduced a new lint class. |
| S-01 — terminal approval retry regression suggestion | OPEN | No new regression test was added; the existing `handleStepApprove` terminal-attempt behavior remains outside this remediation scope. |

### New findings

None. The fresh lint failure is the previously reported W-04 set, and the current diff shows its `validation.go` `govet` finding predates the added result-validator block.

### Issues found

**CRITICAL**: None.

**WARNING**:

1. **W-04 remains open** — `golangci-lint run ./...` exits 1 with 21 project-wide findings (16 errcheck, 1 govet, 3 staticcheck, 1 unused), matching the prior report. This is non-blocking for the change-specific runtime verdict but prevents a clean repository-wide lint gate.

**SUGGESTION**:

1. **S-01 remains open** — add a regression for retrying an identical approval after its attempt becomes terminal.

### Cleanup evidence

After all fresh bounded commands, `pgrep -af '[g]o test|[g]o build|[g]o vet|[g]olangci-lint|[g]ovulncheck|[h]aro broker|[.]test'` produced no process output. A null-glob check for `/tmp/haro-*.sock` reported `(none)`. No OOM event or residual Haro socket was observed.

### Verdict

**PASS WITH WARNINGS** — all 21 retrieved requirements and scenarios are compliant, C-01 and W-01/W-02/W-03 are closed, and fresh focused/build/vet evidence passes. W-04 remains a non-blocking project-wide lint warning and S-01 remains an optional regression suggestion. Native status still routes through `resolve-review` because the bounded review transaction is missing; after that gate, archive is the next phase.

**status**: `success`  
**executive_summary**: Independent re-verification confirms the outgoing JSON-RPC validation, traceability note, collision extension, notification E2E, and SQLite hardening are real and passing. No critical or new findings were found; the prior lint warning remains open.  
**artifacts**: `openspec/changes/2026-09-11-v2-broker-ipc/verify-report.md`; Engram topic `sdd/2026-09-11-v2-broker-ipc/verify-report`  
**next_recommended**: `resolve-review`, then `archive`  
**risks**: Project-wide lint remains non-green; the native review transaction is missing.  
**findings_summary**: 0 critical, 1 warning, 1 suggestion; previous findings closed 4, open 2.  
**skill_resolution**: `paths-injected` — sdd-verify contract, shared SDD references, go-testing, and strict-tdd-verify loaded.

## Key Learnings

1. Dispatcher-level result validation closes the response boundary only when every production method has an explicit wire schema.
2. Persisted notification tests must drive one subscribed connection through both status and interaction publication paths.
3. Removing SQLite shared-cache mode eliminated the observed table-lock failure while preserving bounded concurrent store behavior.
