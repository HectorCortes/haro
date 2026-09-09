# Apply Progress: v2-agent-wiring

Change: v2-agent-wiring | Mode: Strict TDD | Delivery: single-pr with maintainer pre-approved `size:exception` (direct push to main, no PR). Commit unit strategy: one conventional commit per work unit (U1-U5), commits on `main`, not pushed (orchestrator pushes after verify).

## Completed Tasks (15/15)

- [x] U1: 1.1, 1.2 — strict harness configuration
- [x] U2: 2.1, 2.2, 2.3, 2.4, 2.5 — real OpenCode session, factory, per-candidate probe
- [x] U3: 3.1, 3.2 — ordered intersection fallback, simulation deleted
- [x] U4: 4.1, 4.2 — real evidence + transport identity
- [x] U5: 5.1, 5.2, 5.3, 5.4 — CLI wiring + final gate

## Work Unit Commits

| Unit | Commit | Message | Focused test (exact result) |
|---|---|---|---|
| U1 | `f407c5a` | feat(project): strict optional harness configuration | `go test ./internal/project/` → ok (10 RUN incl. subtests) |
| U2 | `3758693` | feat(adapter): real OpenCode CLI-direct sessions and opencode-only factory | `go test ./internal/adapter/...` → ok (13 RUN opencode, 3 factory, 14 adapter) |
| U3 | `c15b252` | feat(execution): ordered harness intersection and fail-closed agent steps | `go test ./internal/execution/` → ok (101 RUN) |
| U4 | `4d4d4af` | test(execution): prove real agent evidence and transport identity persist | `go test ./internal/execution/ ./internal/store/...` → ok |
| U5 | `1d0453e` | feat(cmd): wire one adapter manager per agent-capable CLI invocation | `go test ./internal/cmd/ -run TestAgentManagerCLIInjection` → ok (cmd pkg 53 RUN) |

## Work Unit Evidence

| Unit | Focused command + result | Runtime harness + result | Rollback boundary |
|---|---|---|---|
| U1 | `go test ./internal/project/` → ok | N/A: pure parsing, no runtime boundary | revert `internal/project/config.go` + `config_test.go` |
| U2 | `go test ./internal/adapter/...` → ok | `HARO_TEST_OPENCODE_BINARY` shell fixture (t.TempDir, shebang script): lifecycle, timeout, cancel, oversized-frame all pass | revert `internal/adapter/opencode/adapter.go`, `factory/`, `manager.go` probe change, parser fields |
| U3 | `go test ./internal/execution/` → ok | Fake manager adapter (hermetic; no subprocess): intersection, exhausted, fail-closed all pass | revert `internal/execution/engine.go`, `fallback.go`, updated tests; does not remove unrelated work |
| U4 | `go test ./internal/execution/ ./internal/store/...` → ok | Fake adapter with TransportProvider + temp SQLite: identity, ≤16 KiB redacted payload, transport-neutral attempts all pass | revert `internal/execution/agent_evidence_test.go` + fake seam; no production change in unit |
| U5 | `go test ./internal/cmd/` → ok | CLI E2E through `Execute` with fixture: exactly one subprocess session, `run --format json`, native identity persisted; skip site exercised | revert `internal/cmd/execute.go` wiring + test; engine fails closed without manager |

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | `internal/project/config_test.go` | Unit | ok (existing) | Written (compile RED: `cfg.Harnesses undefined`) | Passed | 6 cases (absent/keys/unknown field/unknown top/range/disabled) | Clean |
| 1.2 | `internal/project/config_test.go` | Unit | ok | Same cycle as 1.1 | Passed | defaults asserted in arbitrary-keys case | Clean |
| 2.1 | `internal/adapter/opencode/adapter_test.go` | Integration (subprocess fixture) | ok | Written (compile RED: `Adapter undefined`) | Passed | 7 cases (lifecycle/probe rejections/zero-text/error/timeout/oversized/cancel) | Clean |
| 2.2 | `internal/adapter/opencode/parser_test.go` | Unit | ok (3 tests) | Written (envelope assertions) | Passed | envelope + legacy-frame cases | Clean |
| 2.3 | `internal/adapter/opencode/adapter_test.go` | Integration | ok | Written (probe rejection table) | Passed | 6 rejection cases + ELF/shebang acceptance | Clean |
| 2.4 | `internal/adapter/factory/factory_test.go` | Integration | N/A (new pkg) | Written (compile RED: `ResolveBinary/NewManager undefined`) | Passed | precedence, registration/probe/init, session env overlay | Clean |
| 2.5 | `internal/adapter/manager_probe_test.go` | Unit | ok | Written (compile RED: `Registered undefined`; behavioral abort-all) | Passed | per-candidate + Registered accessor | Clean |
| 3.1 | `internal/execution/agent_fallback_test.go` | Integration (fake manager) | ok (execution baseline) | Written (behavioral RED: req="", attempts=3, fail-closed violated, timeout not terminal) | Passed | intersection/exhausted/fail-closed 3-subtest matrix | Clean |
| 3.2 | `internal/execution/agent_fallback_test.go` | Integration | ok | Written (no-manager simulation success observed in RED) | Passed | 3 fail-closed subtests + isTerminal classification | Simulation branch deleted |
| 4.1 | `internal/execution/agent_evidence_test.go` | Integration | ok | Written after engine consumption landed in U3 cycle (see deviation D1) | Passed (approval) | identity + no-identity cases | Clean |
| 4.2 | `internal/execution/agent_evidence_test.go` | Integration | ok | Accessor itself was RED-first via U2 lifecycle test (compile RED on `TransportProvider`) | Passed | optional-row triangulation | Clean |
| 5.1 | `internal/cmd/agent_wiring_test.go` | E2E (CLI) | ok (cmd baseline) | Written (behavioral RED: "not registered") | Passed | run/step-run/skip sites + identity | Clean |
| 5.2 | `internal/cmd/agent_wiring_test.go` | E2E | ok | Same cycle | Passed | fixture invoked exactly once | Clean |
| 5.3 | `scripts/verify-adapter-boundary.sh` | Gate | ok | N/A (gate) | Passed (exit 0) | comment reworded to keep gate output clean | Clean |
| 5.4 | full suite | Gate | ok | N/A | Passed | 4 commands (below) | Clean |

## Test Summary

- Total test invocations written this change: ~35 (new test functions + subtests); suite-wide RUN counts: project 10, opencode 13, factory 3, adapter 14, execution 101, cmd 53, store 51 — all passing.
- Layers used: Unit, Integration (subprocess fixture + fake manager), E2E (CLI).
- Approval tests: U4 pair (see deviation D1).
- Pure functions created: `ResolveBinary` (factory), `isExecutableProgram`, `resolveRequires` (opencode), `NormalizeHarnesses`/`IsEnabled` (project).
- Skipped integration scope: none; no real opencode binary required (hermetic fixture only).

## Final Gate (task 5.4)

| Command | Result |
|---|---|
| `go test ./... -race` | ok, all 15 packages, exit 0 |
| `go build ./...` | OK |
| `go vet ./...` | OK |
| `scripts/verify-adapter-boundary.sh` | All adapter boundary checks passed (exit 0) |

## Files Changed

Created: `internal/adapter/opencode/adapter.go`, `internal/adapter/opencode/adapter_test.go`, `internal/adapter/factory/factory.go`, `internal/adapter/factory/factory_test.go`, `internal/adapter/manager_probe_test.go`, `internal/execution/agent_fallback_test.go`, `internal/execution/fake_adapter_test.go`, `internal/execution/agent_evidence_test.go`, `internal/cmd/agent_wiring_test.go`.
Modified: `internal/project/config.go`, `internal/project/config_test.go`, `internal/adapter/opencode/parser.go`, `internal/adapter/opencode/parser_test.go`, `internal/adapter/adapter.go`, `internal/adapter/manager.go`, `internal/execution/engine.go`, `internal/execution/fallback.go`, `internal/execution/evidence_inline_test.go`, `internal/execution/state_test.go`, `internal/cmd/execute.go`.

## Deviations from Design/Tasks

- **D1 (TDD ordering, task 4.2)**: the engine-side consumption of the transport accessor (identity-empty row replaced by real identity) was implemented during the U3 GREEN cycle (commit `c15b252`) because the 3.1 RED cycle required the loop to create attempts without synthesized identity. Task 4.1/4.2 tests were then written first for that behavior and passed as approval tests. The accessor type itself (`adapter.TransportProvider`) was RED-first via the U2 lifecycle test. Justification recorded; no production change happened without a written test in the U2/U3 cycles.
- **D2 (probe rejection rule refinement)**: "executable regular file" is implemented as executable regular file whose content is directly executable (ELF magic or `#!` interpreter line). This is what makes the threat-matrix cases (executable Markdown/MDX, shebang-less `README.sh`) rejectable in a provider-neutral way; `requirements.txt`/`CMakeLists.txt` are rejected by the regular-executable check.
- **D3 (isTerminal extension)**: added `timeout` and `contract` markers to `isTerminal` per design classification (timeout/contract errors terminal). Task list did not call this out explicitly; design.md did.
- No other deviations. No simulation restored; `deltas-acceptance.md` untouched (92/11 preserved); v2-broker/v2-ipc/PTY/concurrency not implemented; docs/v2 untouched.

## Issues Found

- Pre-existing tests `state_test.go` (agent requires subtest) and `evidence_inline_test.go` (agent inline evidence subtest) depended on the removed simulation; updated to inject fake harness adapters (per spec "tests MUST inject fakes"). They are intentional demo-break casualties, not regressions.
- Pre-existing gofmt non-compliance exists across many repo files (untouched; not introduced by this change).

## Next Steps

- `next_recommended`: `sdd-verify`

---

## Remediation Round 1 (post-VERIFY, CRITICAL findings)

Both CRITICAL verify findings fixed via strict TDD (RED confirmed, then GREEN), two focused commits on `main`, not pushed.

### Finding 1 — Probe-once violation (v2-adapter/F-01)

- **RED test**: `TestProbeCalledOncePerHarness` (`internal/execution/agent_fallback_test.go`) — failed with `good probed 2 times, want exactly 1` (factory probe in `setupFakeManager` + engine re-probe at `runAgentStep`). Supporting RED: `TestManager_ProbeCachedOnce` (`internal/adapter/manager_probe_once_test.go`) — adapters re-probed on a second `Manager.Probe` call and `ProbeResults` was undefined (compile RED).
- **Fix**: `internal/adapter/manager.go` — `Manager` now caches the availability snapshot of a single serialized probe per manager lifetime (`probeMu` + `probeResults`); subsequent `Probe` calls return the cached snapshot without touching adapters, and a new `ProbeResults()` accessor exposes the captured state without probing. `internal/execution/engine.go` `runAgentStep` consumes `adapterMgr.ProbeResults()` instead of calling `Probe` during execution. Per-candidate clean-fallback semantics preserved (probe failure → that candidate `Available:false`, never terminal, never abort-all); missing snapshot (never probed) → all unavailable → fail closed.
- **Commit**: `c777510` fix(execution): probe harnesses once per CLI invocation

### Finding 2 — Cancellation deadlock on Prompt early returns

- **RED test**: `TestCancelReturnsAfterLaunchFailure` (`internal/adapter/opencode/adapter_test.go`) — the `launch failure settles Cancel promptly` subtest deadlocked (2s guard hit: `Cancel deadlocked after launch failure (non-cancellable context)`), because Prompt's early returns (stdin/stdout pipe errors, `cmd.Start` failure, prompt write/close errors) left `s.done` open while Cancel waits on it.
- **Fix**: `internal/adapter/opencode/adapter.go` — session gains `doneOnce sync.Once` and `settleDone()` closing `done` exactly once. The consumer goroutine owns the settle on the success path; a deferred settle in `Prompt` (guarded by a `started` flag that flips when the goroutine takes ownership) covers every early-return error path without closing early on success — preserving the wait-for-exit semantics of `Cancel` for successful sessions. Covered cases: launch failure Cancel prompt return, idempotent second Cancel, settled-successful-session Cancel.
- **Commit**: `89f1c0a` fix(adapter): settle session completion on every Prompt failure path

### Remediation gates

| Command | Result |
|---|---|
| `go test ./internal/execution/ ./internal/adapter/opencode/ ./internal/cmd/ -count=1` | ok, exit 0 |
| `go test ./... -race -count=1` | ok, 15/15 packages, exit 0 |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `scripts/verify-adapter-boundary.sh` | exit 0 |

### TDD Cycle Evidence (remediation)

| Finding | RED test | Layer | RED observed | GREEN | REFACTOR |
|---|---|---|---|---|---|
| 1 | `TestProbeCalledOncePerHarness` + `TestManager_ProbeCachedOnce` | Integration (fake manager) / Unit | `probed 2 times, want exactly 1`; re-probe on 2nd call + compile RED `ProbeResults undefined` | Passed | Clean (probeMu serialization) |
| 2 | `TestCancelReturnsAfterLaunchFailure` | Integration (subprocess fixture) | `Cancel deadlocked after launch failure (non-cancellable context)` (2s guard) | Passed | `started` flag prevents early close on success path |

### Next Steps (updated)

- `next_recommended`: `sdd-verify` (re-run)
