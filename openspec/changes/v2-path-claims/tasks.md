# Tasks: v2-path-claims (path_claims and workspace isolation)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 950–1100 |
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
| 1 | Canonical+policy | PR1 | `go test ./internal/claim -race` | N/A pure funcs | `internal/claim/` |
| 2 | Store DDL+repo | PR1 | `go test ./internal/store -run TestAcquire -race` | File DB | `internal/store/migrations.go,claim.go,store.go` |
| 3 | Worktree+workflow | PR1 | `go test ./internal/worktree -race` | gated git | `internal/worktree/, workflow/, project/config.go` |
| 4 | Engine+CLI+docs | PR1 | `go test ./internal/execution -run TestRunStepClaim -race` | temp git | `internal/execution/, cmd/, .gitignore, docs/` |

## Phase 1: Foundation

- [x] 1.1 RED `claim/canonical_test.go` symlinks/`..`/absolutes/`src/` vs `src/foobar/` → fail escapes. `go test ./internal/claim -run TestCanonical -race` [U-01,F-03]
- [x] 1.2 GREEN `claim/canonical.go` EvalSymlinks anchored, boundary check. `go test ./internal/claim -race` [U-01,F-03]
- [x] 1.3 TRIANGULATE anchored root vs `src/link->/etc`. `go test ./internal/claim -race` [U-01]
- [x] 1.4 RED `claim/policy_test.go` 4-row matrix + external-shared. `go test ./internal/claim -run TestPolicy -race` [U-03,F-05,F-06,F-07]
- [x] 1.5 GREEN `claim/policy.go` `ShouldBlock`+`IsExternal` forces shared. `go test ./internal/claim -race` [U-03,F-05,F-06,F-07]
- [x] 1.6 RED `store/claim_test.go` DDL FK/NOT NULL/partial index; file DB concurrency one winner. `go test ./internal/store -race` [U-02,U-03]
- [x] 1.7 GREEN `store/migrations.go` DDL FK+index; claim.go BEGIN IMMEDIATE repo; Acquire=(ctx,PathClaim). `go test ./internal/store -race` [U-02,U-03]

## Phase 2: Core

- [x] 2.1 RED `workflow/parse_test.go` fixtures isolated/block defaults, invalid mode/allow. `go test ./internal/workflow -run TestWorkspace -race` [F-02,F-06]
- [x] 2.2 GREEN `workflow/parse.go` WorkspaceConfig+resolver system→workflow→step. `go test ./internal/workflow -race` [F-02,F-06]
- [x] 2.3 RED `worktree/manager_test.go` fixed argv git, FakeWorktree, threat-matrix. `go test ./internal/worktree -race` [F-01]
- [x] 2.4 GREEN `worktree/manager.go` `Create/Remove/Prune` exec.CommandContext idempotent. `go test ./internal/worktree -race` [F-01]
- [x] 2.5 RED+GREEN `project/config_test.go` `.haro/config.yaml` `external_paths` empty default. `go test ./internal/project -race` [F-07]

## Phase 3: Integration

- [x] 3.1 RED `execution/engine_test.go` CreateExecution persists WorkspaceRoot/Mode per execution+step. `go test ./internal/execution -run TestCreateExecutionWorkspace -race` [F-01,F-02]
- [x] 3.2 GREEN `execution/engine.go` worktree .haro/worktrees/<id>, resync Remove, prune. `go test ./internal/execution -race` [F-01,F-02,F-04]
- [x] 3.3 RED `execution/engine_test.go` Canonicalize+Acquire, logical_conflict, defer Release, wrong-owner reject. `go test ./internal/execution -run TestRunStepClaim -race` [F-03,F-04,U-03]
- [x] 3.4 GREEN `execution/engine.go` wrapper defer Release; TRIANGULATE failure. `go test ./internal/execution -race` [F-03,F-04,U-02,U-03]
- [x] 3.5 RED+GREEN `cmd/execute_test.go`+`output.go` logical_conflict + steps next filter. `go test ./internal/cmd -run TestLogicalConflict -race` [F-04,F-06]

## Phase 4: Verification

- [x] 4.1 TRIANGULATE E2E temp git repo all scenarios gated Short. `go test ./... -race -short` [F-01..F-07,U-01..U-03]
- [x] 4.2 GREEN `.gitignore` .haro/worktrees/; docs §7 sole-ownership (Acquire ctx,PathClaim, retain schema). `go build ./... && go vet ./... && go test ./... -race` [scope]
