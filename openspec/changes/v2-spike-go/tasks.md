# Tasks: M0 Go spike (v2-spike-go)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 450–550 |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: High

Project budget 20000 > est. 500, so single PR approved despite generic 400 guard.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Bootstrap module+main | PR 1 | `go build ./... && go vet ./...` | `go build -o haro . && ./haro` | `go.mod`, `go.sum`, `main.go` |
| 2 | Workflow parse | PR 1 | `go test ./internal/workflow -race` | N/A file parse | `internal/workflow/`, `testdata/` |
| 3 | SQLite proof | PR 1 | `go test ./internal/store -race` | N/A memory DB | `internal/store/` |
| 4 | IPC health | PR 1 | `go test ./internal/ipc -race` | `net.Dial unix h.sock` | `internal/ipc/` |
| 5 | Capabilities | PR 1 | `go test ./internal/adapter -race` | N/A JSON | `internal/adapter/` |
| 6 | Config+gitignore | PR 1 | `go test ./... -race` | N/A static | `openspec/config.yaml`, `.gitignore` |

## Phase 1: Bootstrap

- [ ] 1.1 Create `go.mod` (`github.com/HectorCortes/haro`, `go 1.23`) + `go get modernc.org/sqlite gopkg.in/yaml.v3 && go mod tidy`. Covers F-01,F-05. Verify: `CGO_ENABLED=0 go build ./...`.
- [ ] 1.2 Create `main.go` (print `haro v0.0.0-spike`, exit 0). Deps: 1.1. Covers F-01. Verify: `go build -o haro . && ./haro`; `go vet ./...`.

## Phase 2: Core Proofs

- [ ] 2.1 Create `testdata/sample-workflow.yaml` + `internal/workflow/parse.go` + `parse_test.go` (valid/wrong version/empty). Deps: 1.1. Covers U-02. Verify: `go test ./internal/workflow -race`.
- [ ] 2.2 Create `internal/store/sqlite_test.go` (WAL success-only, FK=1, create/insert/select). Deps: 1.1. Covers F-06. Verify: `go test ./internal/store -race`.
- [ ] 2.3 Create `internal/ipc/health.go` + `health_test.go` (short `h.sock`, cleanup). Deps: 1.1. Covers F-07. Verify: `go test ./internal/ipc -race`.
- [ ] 2.4 Create `internal/adapter/capabilities.go` + `capabilities_test.go` (roundtrip preserves `_` extra). Deps: 1.1. Covers U-03. Verify: `go test ./internal/adapter -race`.

## Phase 3: Config

- [ ] 3.1 Rewrite `openspec/config.yaml` (Go context, keep `auto`/`both`/`single-pr`, budget 20000, `strict_tdd true`, runner `go test ./...`, linter `golangci-lint run`, checker `go vet ./...`). Deps: 2.1-2.4. Covers U-01. Verify: no `npm`/`tsc`.
- [ ] 3.2 Append `.gitignore` (`*.out`,`coverage.*`,`*.cover`), preserve 7 lines. Deps: 3.1. Covers U-01. Verify: diff only additions.

## Phase 4: Gate

- [ ] 4.1 Run `go build ./...`, `go build -o haro .`, `go vet ./...`, `go test ./... -race`, `golangci-lint run`, `govulncheck ./...`. Deps: all. Covers F-01..F-07,U-01..U-03. Verify: all exit 0.

Coverage: F-01:1.1,1.2,4.1 F-02:3.1,4.1 F-03:2.1-2.4,4.1 F-04:3.1,4.1 F-05:1.1,4.1 U-01:3.1,3.2,4.1 U-02:2.1,4.1 F-06:2.2,4.1 F-07:2.3,4.1 U-03:2.4,4.1 — Threat matrix N/A.

Commits: 1:`feat(go): bootstrap module and main` 2:`feat(workflow): parse proof` 3:`feat(store): sqlite proof` 4:`feat(ipc): health proof` 5:`feat(adapter): capabilities proof` 6:`chore(config): Go config and gitignore`
