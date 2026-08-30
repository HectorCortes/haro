# Design: v2 adapter contract and multi-harness

## Technical Approach

`internal/execution.Engine` will own a CLI-direct `adapter.Manager`: command steps retain `CommandRunner`; agent steps resolve ordered harness names through registered adapters. Each adapter subprocess follows `new → initialized → session` and receives one bilateral `initialize` negotiating `protocolVersion` and capabilities before any `session/*` call (F-01). Stable §4 `Adapter`, `Session`, and `SessionHost` interfaces isolate lifecycle ownership so a future broker can re-home the manager without changing adapters. `internal/adapter/` keeps `capabilities.go` and adds `opencode/`, `claude/`, and `acp/`; provider literals remain there.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| CLI-direct manager | Later broker re-home, smallest current coupling | Chosen; Engine owns lifecycle and fallback |
| Broker stub | Less re-home, premature UDS/lease scope | Rejected |
| Ad-hoc Engine subprocesses | Simple, leaks provider details | Rejected |
| Stdlib NDJSON JSON-RPC 2.0 | Harness-neutral framing needs strict bounds | Chosen; bounded line reader, `UseNumber`, 10 MiB limit, request/response/error/notification and string/number/null IDs; extend `internal/ipc/health.go` patterns |
| Additive transport extension | Narrow overlap with `v2-store` | `v2-adapter` is sole migration owner |

Negotiation gates `LoadPrevious` and `Terminal` by the negotiated table; absent support returns `unsupported_capability` without emitting a method (F-02/U-01). Unknown additive capabilities round-trip without a major bump. `SessionHost.RequestPermission` is exposed only when core permission support was negotiated; otherwise adapter default policy resolves the need or execution fails closed (F-03). `Terminal` remains false.

## Data Flow

```text
YAML agent step → Engine → ordered AdapterManager candidate
                         → initialize → session/new → prompt/update
                         ↔ fail-closed SessionHost permission
                         → attempt + attempt_transport transaction → Store
clean failure → Redact → VisibleEvidence → FallbackBytes (≤2 MiB) → next candidate
```

Fallback interprets no provider semantics: only a completed clean failure advances; process-start, protocol, permission, or capability violations fail immediately. Exhaustion records sanitized evidence from every candidate. OpenCode parses JSONL inside its adapter; ACP bijectively maps `initialize`, `session/new`, `prompt`, `update`, `cancel`, and `request_permission`.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/adapter/{adapter.go,manager.go,negotiation.go}` | Create | §4 contracts, lifecycle, gating |
| `internal/adapter/{opencode,claude,acp}/` | Create | Provider parsing/transports and ACP translation |
| `internal/ipc/jsonrpc/` | Create | Hardened reusable NDJSON codec |
| `internal/execution/{engine.go,fallback.go,evidence.go}` | Modify/Create | Agent execution and sanitized fallback |
| `internal/workflow/{parse.go,validate.go}` | Modify | `harness []string`, `instructions`, `mode` agent fields |
| `internal/store/{migrations.go,store.go}` | Modify | Transport DDL/repository/facade |
| `internal/adapter/contract/`, `testdata/fixtures/synthetic/v2.0.0-synthetic.1/README.md` | Create | Runnable suite, neutral fixtures, provenance/no byte-identity claim |
| `scripts/verify-adapter-boundary.sh` | Create | Grep/import-graph containment gate |

## Interfaces / Contracts

Interfaces match technical specification §4: `Adapter.Probe/Initialize/NewSession`; `Session.Prompt/Cancel` plus gated `LoadPrevious/Terminal`; reverse `SessionHost.RequestPermission`; and `SessionBundle`, `PromptInput`, `SessionEvent`. `TransportRepository.Put/Get` is exposed as `Store.Transport()`.

```sql
CREATE TABLE IF NOT EXISTS attempt_transport (
  attempt_id TEXT PRIMARY KEY REFERENCES attempts(id), adapter_name TEXT NOT NULL,
  native_session_id TEXT, protocol_version INTEGER,
  extra TEXT NOT NULL DEFAULT '{}'
);
```

`attempts` remains transport-neutral; Engine inserts the attempt and optional transport row in one `WithTx`.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | Negotiation/drift/gates, ACP round-trip, codec IDs/errors/10 MiB, migration | Table-driven RED tests, `t.TempDir`; GREEN minimal implementation; TRIANGULATE empty/partial/complete/future and boundary rows |
| Integration | Lifecycle, permission, parser, fixtures | Versioned contract suite against `FakeAdapter`/`FixtureAdapter`; synthetic provenance |
| E2E | Ordered fallback and Claude public cycle | `FakeRunner`; real Claude only via `HARO_TEST_CLAUDE_BINARY`, skip under `testing.Short()` |

CI runs `go test ./... -race`; short mode excludes external binaries, not fixture coverage.

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: no executable-file classifier | Direct argv/process adapter boundary | N/A |
| Git repository selection | N/A: no VCS routing | No repository selector | N/A |
| Commit state | N/A: no commit automation | None | N/A |
| Push state | N/A: no push automation | None | N/A |
| PR commands | N/A: no PR automation | None | N/A |

## Migration / Rollout

The additive migration is idempotent and retained on rollback. `v2-store` must not recreate or diverge this table. Broker/UDS, CLI↔Broker RPC, leases, interactions, path claims, PTY, composition, claims, reporting, and distribution are out of scope.

## Risks

| Risk | Mitigation |
|---|---|
| Double migration ownership | Sole owner plus repeat-migration schema test |
| Credential leakage | Redact before every accumulation |
| JSON-RPC DoS | Drain/reject frames over 10 MiB with structured error |
| Capability/provider drift | Drift tables and boundary script |
| Fixture trust | Versioned provenance and opt-in real binary |
| Lifecycle re-home churn | Keep §4 interfaces independent of Engine/broker |

## Open Questions

None.
