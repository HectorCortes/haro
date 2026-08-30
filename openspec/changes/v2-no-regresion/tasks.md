# Tasks: v2-no-regresion

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated lines | 2200–2800 (wf300 store500 proj100 exec700 cli400 tests600+) |
| 400-line risk | High |
| Chained PRs | Yes |
| Split | Single PR `size:exception` (6 units) |
| Delivery | single-pr |
| Chain | size-exception |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | PR | Focused test | Runtime harness | Rollback |
|------|------|----|--------------|-----------------|----------|
| 1 | Workflow DAG/containment | 1 | `go test ./internal/workflow -run TestParse\|Discover\|Validate\|Containment` | `t.TempDir` list | `internal/workflow/*` |
| 2 | Store DDL/idempotent | 1 | `go test ./internal/store -run TestMigrations` | file DB WAL+FK | `internal/store/*` |
| 3 | Project init | 1 | `go test ./internal/project -run TestInitIdempotent` | `haro init` twice | `internal/project/*` |
| 4 | Engine+budgets+Runner | 1 | `go test ./internal/execution -run TestCommandCycle\|Evidence` | fake 4 cases | `internal/execution/*` |
| 5 | CLI JSON/EPIPE/`--` | 1 | `go test ./internal/cmd -run TestCLI` | closed `os.Pipe` | `internal/cmd/*`+`main.go` |
| 6 | E2E+gate | 1 | `go test ./... -race` | `vet && lint && vulncheck` | `testdata/*` |

## Phase 1: Workflow & Store

- [x] 1.1 RED parse env/timeout/depends_on/requires/produces → GREEN `workflow/parse.go` `TestParse` [F-02]
- [x] 1.2 RED discover `.haro/workflows/*/workflow.yaml` sorted → GREEN `workflow/discover.go` `TestDiscover` [F-02]
- [x] 1.3 RED validate dup IDs/missing/cycle/no-entry → GREEN `workflow/validate.go` `TestValidate` [F-02]
- [x] 1.4 RED containment absolute/`..`/symlink `EvalSymlinks` → GREEN `workflow/path.go` `TestContainment` [F-13]
- [x] 1.5 RED Store `WithTx` only `store` imports `database/sql` → GREEN `store/store.go` [U-01]
- [x] 1.6 RED 7 tables CHECKs WAL FK idempotent → GREEN `store/migrations.go`+`sqlite.go` `TestMigrations` [F-03,F-07]

## Phase 2: Project & Runner

- [x] 2.1 RED `TestInitIdempotent` bytes same → GREEN `project/init.go` `.haro/*` [F-01]
- [x] 2.2 RED quoted-argv no shell `;|$()` timeout300s → GREEN `execution/runner.go` `CommandRunner.Run` [F-03]
- [x] 2.3 RED 16KiB/1MiB/2MiB Bearer/Basic/token → GREEN `execution/evidence.go` `TestEvidenceBudgets` [F-12]

## Phase 3: Engine & State

- [x] 3.1 RED fake 4: exit0 ok / nonzero fail / missing fail / unmet unexecuted → GREEN `execution/engine.go` [F-03]
- [x] 3.2 RED delimited feedback bounded prior → GREEN feedback `TestFeedback` [F-05]
- [x] 3.3 RED cascade 3 steps reopen generations pending files keep; skip virgin → GREEN `execution/state.go` `TestReopenSkip` [F-07]
- [x] 3.4 RED idempotent pending→running→completed|failed→pending pending→skipped → GREEN transitions `TestStateMachine` [U-03]

## Phase 4: CLI

- [x] 4.1 RED `flag.NewFlagSet(ContinueOnError)` 8 leaves `--` `unexpected_argument` → GREEN `cmd/execute.go` `Execute` [F-14]
- [x] 4.2 RED JSON `{error,code}/1` EPIPE→0 → GREEN `cmd/output.go` `TestCLIContract` [F-14]
- [x] 4.3 RED thin `main.go` `cmd.Execute` no provider → GREEN `main.go` `go vet` [F-14]

## Phase 5: Integration & Gate

- [x] 5.1 RED `TestWorkflowsDiscovery`/`TestCommandCycle` via `Execute` file DB fake → GREEN wiring [F-02,F-03]
- [x] 5.2 RED `TestInitIdempotent` `status` stale produces invalid → GREEN E2E [F-01,F-07]
- [x] 5.3 Verify `docs/` `config.yaml` `.gitignore` unchanged + `go test -race && vet && lint && vulncheck` [U-01]

Plan: workflow→store→project→execution→cmd→e2e→gate
