# Tasks: V2 Agent Wiring

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~1800–2400 (code/tests ~900 each, docs ~60) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | Work-unit commits U1–U5 |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High

Delivery: direct push to main; `size:exception` pre-approved.

### Suggested Work Units

| Unit | Goal | Focused test | Runtime harness | Rollback |
|---|---|---|---|---|
| U1 | Strict harness config | `go test ./internal/project/` | N/A: pure parsing | config.go+tests |
| U2 | OpenCode session | `go test ./internal/adapter/...` | `HARO_TEST_OPENCODE_BINARY` fixture | factory+opencode |
| U3 | Fallback, no simulation | `go test ./internal/execution/` | Fake manager adapter | engine.go |
| U4 | Evidence+transport | `go test ./internal/store/...` | Fake adapter+temp SQLite | store paths |
| U5 | CLI wiring | `go test ./...` | Suite+boundary script | cmd/execute.go |

## U1: Harness configuration (F-04)

- [x] 1.1 RED `TestHarnessConfigKnownFields`: absent `harnesses` and arbitrary keys pass; unknown record field AND unknown top-level key rejected via `yaml.Decoder.KnownFields(true)` (`internal/project/config.go`); `version`/`external_paths` compatible.
- [x] 1.2 GREEN add `HarnessConfig{Binary, Env, TimeoutSeconds, Enabled *bool}` + `Config.Harnesses`; defaults `Enabled=true`, timeout 300; 1–300 range enforced.

## U2: OpenCode real session (F-01)

- [x] 2.1 RED `TestOpenCodeRealSessionLifecycle`: fixture via `HARO_TEST_OPENCODE_BINARY`; assert argv `run --format json`, no shell, stdin prompt, JSONL ≤10 MiB frames, timeout ≤300s, idempotent `Cancel` (`sync.Once`, waits exit).
- [x] 2.2 Extend `internal/adapter/opencode/parser.go` `Event` with `sessionID`/`type`; fixture pins envelope (`part.text`→`output_delta`, `error`→`failed`).
- [x] 2.3 GREEN `internal/adapter/opencode/adapter.go`: Probe accepts only executable regular files (reject `requirements.txt`, `CMakeLists.txt`, executable MD/MDX, `README.sh`); negotiate; `NewSession`→`SessionBundle`; EOF→`completed` only with ≥1 text event (zero-text `step_start` fails cleanly).
- [x] 2.4 Create `internal/adapter/factory/factory.go`: OpenCode-only factory; binary precedence `HARO_TEST_OPENCODE_BINARY` → configured → `opencode`; env overlays inherited.
- [x] 2.5 `internal/adapter/manager.go`: probe failure/ENOENT → per-candidate `Available:false`/`ErrUnavailable` fallback, never abort-all.

## U3: Engine ordered fallback (F-06)

- [x] 3.1 RED `TestAgentHarnessIntersectionFallback` + exhausted candidates: ordered intersection (enabled, registered, probed), no reordering; ≤2 MiB redacted context; exhaustion persists all evidence, fails closed.
- [x] 3.2 RED `TestAgentStepFailsWithoutConfiguredHarness`; GREEN delete simulation branch in `internal/execution/engine.go` `runAgentStep` — no synthesized success/identity/output; missing config fails closed (intentional demo break).

## U4: Evidence + transport identity (U-04)

- [x] 4.1 RED `TestAgentStepPersistsRealEvidenceAndTransport`: resolved `SessionBundle.Requires` → absolute `.haro/artifacts/...` paths; redact-once ≤16 KiB `attempt_events.payload`; `payload_ref` and transport-neutral `attempts` unchanged.
- [x] 4.2 GREEN transport accessor in `internal/adapter/adapter.go`; `Transport.Put` replaces identity-empty row (`native_session_id`, `adapter_name`, protocol version, metadata); optional-row/repeated-migration tests green.

## U5: CLI wiring + integration

- [x] 5.1 RED `TestAgentManagerCLIInjection`: one manager per invocation at `handleRun`, `handleStepRun`, `handleStepReopen`, `handleStepSkip` in `internal/cmd/execute.go`; init once before first session; `report` unwired.
- [x] 5.2 GREEN four-site `SetAdapterManager` via factory; retain oversized-frame/boundary tests.
- [x] 5.3 Keep `scripts/verify-adapter-boundary.sh` green: provider literals confined to adapter/factory; harness names as data; oversized-message rejection.
- [x] 5.4 Gate: `go test ./... -race`, `go build ./...`, `go vet ./...`; boundary script; work-unit commits; no simulation restored.
