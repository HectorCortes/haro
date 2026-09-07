# Tasks: v2-reporte — Change reporting

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 650–900 |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | Single PR (size:exception) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

`size:exception` pre-approved (budget 20000).

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Anchor+report+CLI | Single PR | `go test ./...` | `haro report <id> [--json]` | Revert report files |

## Phase 1: Anchor & Migration

- [x] 1.1 RED `internal/store/migrations_report_test.go` — PRAGMA idempotence + NullString fails pre-impl. `go test ./internal/store -run TestEnsureBaseCommitColumn`
- [x] 1.2 GREEN `internal/store/migrations.go` — `ensureBaseCommitColumn` PRAGMA race-tolerant ALTER `base_commit TEXT` after dag_hash, call `migrate`. `go test ./internal/store -run TestEnsureBaseCommitColumn`
- [x] 1.3 `internal/store/repositories.go,store.go` — `BaseCommit *string` NullString. `go test ./internal/store -run TestExecutionBaseCommitMapping`
- [x] 1.4 RED→GREEN `internal/execution/engine.go` — `CreateExecution` `git rev-parse HEAD` in `resolvedRoot` before worktree.Create; fail→no row; dirty ignored. `go test ./internal/execution -run TestCreateExecutionCapturesHEAD`

## Phase 2: Report Engine

- [x] 2.1 RED null-anchor — NULL `base_commit` terminal → `not_available` field `base_commit`. `internal/execution/report.go` `go test ./internal/execution -run TestReportNullAnchor`
- [x] 2.2 RED gating — unknown→`not_found`, pending/running→`not_completed`; shared→root, isolated existing→workspace, removed→`base..HEAD` no ls-files. `go test ./internal/execution -run TestReportStateAndWorkspace`
- [x] 2.3 GREEN `internal/execution/report.go` — `Report`/`ChangedFiles` broker-ready, fixed-argv `git diff --name-only --diff-filter=ADMR --find-renames -z` + `ls-files -z` (fallback +HEAD), NUL parse. `go test ./internal/execution -run TestReportWorkspaceSelection`
- [x] 2.4 GREEN normalization — clean/slash, reject absolute/`..`/symlink, filter `.haro/**`, dedupe, sort. `go test ./internal/execution -run TestReportNormalization`
- [x] 2.5 RED→GREEN U-01 table — git-gated `testing.Short` `t.TempDir`: added/mod/deleted/renamed/untracked/duplicate/`.haro`/escape; identical `--find-renames`. `go test ./internal/execution -run TestChangedFilesTable -short`
- [x] 2.6 Threat-matrix RED — repo selection (relative/absolute/outside/missing never `git -C`), commit state staged/unstaged read-only. `go test ./internal/execution -run TestReportThreatMatrix`

## Phase 3: CLI

- [x] 3.1 RED `internal/cmd/*report*_test.go` — `haro report <id> [--json]` FlagSet plain/JSON `{execution_id,base_commit,changed_files}` EPIPE. `go test ./internal/cmd -run TestReportCLI`
- [x] 3.2 GREEN `internal/cmd/execute.go` — route `report` delegate `Engine.Report`. `go test ./internal/cmd -run TestReportCLI`
- [x] 3.3 Read-only — Report leaves HEAD/index/status unchanged. `go test ./internal/execution -run TestReportReadOnly`

## Phase 4: E2E & Guardrails

- [x] 4.1 F-01/F-02 — one flattened terminal execution single report; untracked only on existing workspace. `go test ./internal/execution -run TestReportComposedE2E`
- [x] 4.2 F-03 oracle — vs `git diff --name-status --find-renames`+`status`; rename=new-path flags identical. `go test ./internal/execution -run TestReportGitOracle`
- [x] 4.3 Removed fallback — isolated removed `base..HEAD` root no untracked (lossy). `go test ./internal/execution -run TestReportRemovedFallback`
- [x] 4.4 CLI parity — plain vs JSON ordered equality includes nullable `base_commit`. `go test ./internal/cmd -run TestReportParityE2E`
- [x] 4.5 Guardrails — grep no broker/UDS/JSON-RPC/PTY/leases/interactions/agents_command, no new deps, no per-step, no worktree lifetime change. `grep -R broker internal/`
- [x] 4.6 Gate — `go vet ./... && go test ./... -race` + migration re-run. `go test ./... -count=1`
