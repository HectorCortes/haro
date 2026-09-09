# Design: V2 Agent Wiring

## Technical Approach

After `store.Open`, load configuration before each agent-capable engine. An adapter-local factory builds, probes, and initializes one CLI-scoped manager; `SetAdapterManager` injects it into `run`, `step run`, `step reopen`, and `step skip`. The fifth site, `report`, remains unwired. This covers F-01/F-04/F-06/U-04 while deferring broker hosting.

## Architecture Decisions

| ADR | Choice | Alternative / rationale |
|---|---|---|
| Runtime | CLI-direct manager lifecycle | Broker/IPC is normative but deferred; avoid hosting/concurrency. |
| Factory | `internal/adapter/factory/factory.go` | Parent `adapter` cannot import `opencode` without an import cycle; subpackage confines provider literals. |
| Registration | Register only configured OpenCode | Claude `Prompt` simulates and ACP only translates; unknown/unregistered names remain unavailable. |
| Simulation | Delete engine simulation | Test gating could leak synthetic success; the demo break preserves fail-closed behavior. |
| Parsing | `yaml.Decoder.KnownFields(true)` over `Config` | Current parsing is non-strict; existing fields remain compatible and map keys arbitrary. |
| OpenCode transport | `opencode run --format json`, prompt on stdin | Current CLI supports non-interactive JSONL and emits `sessionID`; fixed argv avoids shell and prompt leakage. |

## Data Flow

`config.yaml → LoadConfig → factory probe/initialize → SetAdapterManager → runAgentStep → NewSession/Prompt → AgentEvidence → attempt_events + attempt_transport`

```text
success: Step → skip unavailable A → session B → JSONL → redact/bound → completed
failure: Step → session A clean-fails → ≤2 MiB context → session B fails → evidence(all) → failed
```

## Interfaces / Contracts

```go
type HarnessConfig struct {
    Binary string `yaml:"binary"`; Env map[string]string `yaml:"env"`
    TimeoutSeconds int `yaml:"timeout_seconds"`; Enabled *bool `yaml:"enabled"`
}
type Config struct { Version int; ExternalPaths []string; Harnesses map[string]HarnessConfig }
type SessionTransport struct { NativeSessionID string; ProtocolVersion int; Extra map[string]any }
```

Absent `harnesses` is valid. Nil `Enabled` defaults true, zero timeout to 300 seconds, and values outside 1–300 fail. Binary precedence is `HARO_TEST_OPENCODE_BINARY`, configured `binary`, then `opencode`; configured env overlays inherited env.

`opencode.Adapter` probes an executable regular file, negotiates protocol 1, and returns a session carrying `SessionBundle`, with validated requirements mapped to absolute `.haro/artifacts/...` paths. `Prompt` uses `context.WithTimeout` and `exec.CommandContext(binary,"run","--format","json")`, `Dir=WorkspaceRoot`, prompt stdin, and no shell. It incrementally validates ≤10 MiB JSONL frames through the existing parser, maps `part.text` to `output_delta`, `error` to `failed`, successful EOF to `completed`, and captures the common real `sessionID`. A session transport accessor lets `Transport.Put` replace the identity-empty row with identity, negotiated version, and JSON metadata. `Cancel` uses `sync.Once` and waits for exit.

The engine preserves workflow order across configured, enabled, registered, `Probe.Available` candidates. Unknown/disabled/`ErrUnavailable` (including `Available:false` or launch `ENOENT`) produce sanitized skip diagnostics, never terminal. Harness-declared failure is clean fallback; malformed protocol, timeout, contract, store, and permission errors are terminal. Each usable candidate owns a generation/attempt; exhaustion persists all sanitized attempt evidence and fails. Context is redacted before `FallbackEvidence` bounds it to 2 MiB. Raw output reaches only `AgentEvidence`, then ≤16 KiB `attempt_events.payload`; `payload_ref` and transport-neutral `attempts` stay unchanged. Existing idempotent transport DDL and `ensurePayloadColumn` need no schema change.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/project/config.go`, `config_test.go` | Modify | Harness schema, strict decode/default validation. |
| `internal/adapter/factory/factory.go` | Create | OpenCode-only CLI runtime factory. |
| `internal/adapter/opencode/adapter.go`, `parser.go` | Create/modify | Real process session, JSONL translation/identity. |
| `internal/adapter/adapter.go`, `manager.go` | Modify | Transport accessor and availability state. |
| `internal/cmd/execute.go` | Modify | Four-site config/factory injection. |
| `internal/execution/engine.go` | Modify | Intersection, requires, identity, no simulation. |
| Adapter/cmd/execution/store tests | Modify/create | Contract and scenario coverage. |

## Testing Strategy

Hermetic tests use a fake manager adapter and executable fixture selected by `HARO_TEST_OPENCODE_BINARY`; CI needs no OpenCode. Add `TestAgentManagerCLIInjection`, `TestOpenCodeRealSessionLifecycle`, `TestHarnessConfigKnownFields`, `TestAgentHarnessIntersectionFallback`, `TestAgentStepFailsWithoutConfiguredHarness`, and `TestAgentStepPersistsRealEvidenceAndTransport`; retain exhausted-candidate, repeated-migration/optional-row, boundary, and oversized-frame tests. Run `go test ./...` and the boundary script.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | Applicable | Probe accepts only executable regular files; reject non-executable disguised binaries; executable configured paths run only fixed argv. | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` cases. |
| Git repository selection | N/A: cwd is engine-owned workspace, no Git command added | — | — |
| Commit state | N/A: no commit operation | — | — |
| Push state | N/A: no push operation | — | — |
| PR commands | N/A: no PR automation | — | — |

## Migration / Rollout and Risks

No data migration required. Missing binaries fall through; deadlines/cancellation prevent leaks; boundary CI contains literals; one evidence path mitigates redaction/size gaps. Roll back by disabling harnesses or reverting wiring, never restoring simulation. Do not add broker/IPC/PTY/concurrency, edit `deltas-acceptance.md` (92/11), or claim `v2-no-regresion/F-04`.

## Open Questions

None; the fake binary pins the documented current OpenCode JSON envelope so future CLI drift fails contract tests.
