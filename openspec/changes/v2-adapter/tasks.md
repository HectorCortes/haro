# Tasks: v2 adapter (multi-harness)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1900–2400 |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | Single PR (size:exception) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | PR | Focused test | Runtime harness | Rollback boundary |
|------|------|----|--------------|-----------------|-------------------|
| 1 | Store transport | 1 | `go test ./internal/store -run TestTransport` | `go test ./internal/store -race` | `migrations.go,transport.go` |
| 2 | JSON-RPC codec 10 MiB | 1 | `go test ./internal/ipc/jsonrpc` | `go test ./internal/ipc/jsonrpc -race` | `ipc/jsonrpc/` |
| 3 | Adapter + negotiation + Manager | 1 | `go test ./internal/adapter -run TestNegotiate` | `go test ./internal/adapter -race` | `adapter.go,negotiation.go` |
| 4 | Workflow + fallback ≤2MiB | 1 | `go test ./internal/execution -run TestFallback` | `go test ./... -race` | `workflow/,fallback.go` |
| 5 | adapters+suite+fixtures+gate+docs | 1 | `go test ./internal/adapter/contract && bash scripts/verify-adapter-boundary.sh` | `HARO_TEST_CLAUDE_BINARY=claude go test ./internal/adapter/claude -short` | `adapter/*,fixtures/,scripts/` |

## Phase 1: Foundation

- [x] 1.1 `store/transport_test.go` RED DDL, neutral `attempts`, `WithTx`; GREEN `migrations.go`/`transport.go`; TRIANGULATE dup. `go test ./internal/store -race` `U-04`
- [x] 1.2 `ipc/jsonrpc/codec_test.go` RED 10 MiB, IDs, req/resp/err/notif; GREEN `codec.go`; TRIANGULATE oversize. `go test ./internal/ipc/jsonrpc -race` `F-04`
- [x] 1.3 `adapter/adapter_test.go` RED §4 `Probe/Initialize/NewSession`, `Prompt/Cancel`, fail-closed `RequestPermission` (F-01/F-03); GREEN `adapter.go`. `go test ./internal/adapter -race`
- [x] 1.4 `workflow/parse_test.go` RED `harness[]` `instructions` `mode` `Terminal` false (F-06); GREEN `parse.go`. `go test ./internal/workflow -race`

## Phase 2: Core

- [x] 2.1 `adapter/negotiation_test.go` RED drift empty/partial/complete/future, `unsupported_capability` (F-02/U-01); GREEN `negotiation.go`. `go test ./internal/adapter -run TestNegotiate -race`
- [x] 2.2 `adapter/manager_test.go` RED single prior `initialize` (F-01); GREEN `manager.go`. `go test ./internal/adapter -run TestManager -race`
- [x] 2.3 `adapter/permission_test.go` RED fail-closed when not negotiated (F-03); GREEN gating. `go test ./internal/adapter -run TestPermission -race`
- [x] 2.4 `adapter/acp/acp_test.go` RED `initialize/new/prompt/update/cancel/request_permission` (U-03); GREEN `acp/translate.go`. `go test ./internal/adapter/acp -race`
- [x] 2.5 `adapter/opencode/parser_test.go` RED JSONL in adapter, oversize via codec (F-04); GREEN `opencode/parser.go`. `go test ./internal/adapter/opencode -race`
- [x] 2.6 `adapter/claude/adapter_test.go` RED `HARO_TEST_CLAUDE_BINARY` opt-in `Short()` skip (F-05); GREEN `claude/adapter.go`. `go test ./internal/adapter/claude -short`

## Phase 3: Integration

- [x] 3.1 `execution/fallback_test.go` RED ordered clean→next terminal→fail (F-06); GREEN `fallback.go`+`engine.go`. `go test ./internal/execution -run TestFallback -race`
- [x] 3.2 `execution/evidence_test.go` RED `Redact→Visible→Fallback≤2MiB`, exhaustion (F-06); GREEN `fallback.go`. `go test ./internal/execution -run TestEvidence -race`
- [x] 3.3 `execution/engine_transport_test.go` RED attempt+transport `WithTx` (U-04); GREEN wire `Store.Transport()`. `go test ./internal/execution -run TestAgentTransport -race`

## Phase 4: Verification

- [x] 4.1 `adapter/contract/suite_test.go` RED lifecycle/gating/boundary vs `FakeAdapter` (U-02); GREEN `contract/suite.go`. `go test ./internal/adapter/contract -race`
- [x] 4.2 `testdata/fixtures/synthetic/v2.0.0-synthetic.1/README.md` GREEN provenance, version, no byte-identity claim. `cat README.md` `U-02/F-04`
- [x] 4.3 `scripts/verify-adapter-boundary.sh` RED leak detect; GREEN grep+`go list -json` (F-04). `bash scripts/verify-adapter-boundary.sh`
- [x] 4.4 `docs/v2/haro-especificacion-tecnica.md` §7 amend `Store.Transport()`/`TransportRepository`. `grep TransportRepository docs/v2/haro-especificacion-tecnica.md` `U-04`
- [x] 4.5 Gate `go build && go vet && go test -race -short && golangci-lint run`. `F-01..F-06,U-01..U-04`
