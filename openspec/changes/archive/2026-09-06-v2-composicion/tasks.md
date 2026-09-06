# Tasks: v2-composicion — Workflow composition (D05)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1500–2200 |
| 400-line budget risk | High |
| Chained PRs recommended | No (size:exception pre-approved, budget 200000) |
| Suggested split | Single PR (5 units) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Test | Harness | Rollback |
|------|------|------|---------|----------|
| 1 | Parse+Validate | `go test ./internal/workflow -run TestValidateFile` | N/A | `internal/workflow/parse.go` |
| 2 | Flatten+Hash | `go test ./internal/workflow -run TestFlatten` | ReadFile inject | `internal/workflow/compose.go` |
| 3 | dag_hash migrate | `go test ./internal/store -run TestMigrateDagHash` | `t.TempDir`×2 | `internal/store/migrations.go` |
| 4 | Engine+cascade+CLI | `go test ./internal/execution -run TestReopen -race` | `FakeWorktree` | `internal/execution/engine.go` |
| 5 | Fixtures+E2E | `go test ./... -run TestCompose -race` | `testdata/compose/**` | fixtures |

## Phase 1: Parse & Validation

- [x] 1.1 RED `TestParse_Strict`: unknown field→field-path error; `Inputs/Outputs`/`Source/Bindings` unmarshal; `go test -run TestParse_Strict`
- [x] 1.2 Modify `internal/workflow/parse.go`: add `Input/Output`, `Workflow.Inputs/Outputs`, `Step.Source/Bindings`; `yaml.Node`+`KnownFields(true)`
- [x] 1.3 RED `ValidationError{Code,Field,Message}`: `root_contract_forbidden@inputs|outputs`, `contract_violation@steps[i].depends_on|requires`
- [x] 1.4 Modify `internal/workflow/validate.go`: `ValidateFile`/`ValidateFlat`; codes `unknown_input|output`,`missing_binding`,`guard_exceeded`,`cycle_detected`

## Phase 2: Flatten & Hash

- [x] 2.1 RED `TestFlatten_Cycle` direct/transitive/self via renamed nodes; `TestFlatten_Guards` depth16/exp256/steps256; `go test -run TestFlatten_Cycle`
- [x] 2.2 Create `internal/workflow/compose.go`: `Flatten(rootPath,ReadFile)` — `Clean+EvalSymlinks` cycle, guards, `a.b.x`, bindings, `depends_on` via inputs, stable sort
- [x] 2.3 RED `TestHash_Repeatability`: same YAML twice → same Steps/order/Hash
- [x] 2.4 Create `internal/workflow/hash.go`: SHA-256 canonical ordered fields, sorted maps, deps, artifacts, workspace
- [x] 2.5 U-02/U-03/U-04 tables: exact Code/Field for cycles, contracts, schema parity (`additionalProperties:false`, version, types, patterns, source/bindings, workspace); `go test -run TestU -race`

## Phase 3: Store & Engine

- [x] 3.1 Modify `internal/store/migrations.go`+`repositories.go`+`store.go`: `PRAGMA table_info` → `ALTER dag_hash TEXT`, re-run-safe; `TestMigrateDagHash_Idempotent`
- [x] 3.2 RED `TestCreateExecution_NoRow`: cycle/contract/guard → no row, no worktree; `go test -run TestCreateExecution_NoRow`
- [x] 3.3 Modify `internal/execution/engine.go`: ValidateFile→Flatten→ValidateFlat→hash→`worktree.Create`→`WithTx`; `workflow_invalid` guard; re-flatten+hash verify on rehydrate
- [x] 3.4 RED cascade `TestReopen_RetainedDescendants` s1→s2→s3 pending (no regress `state_test.go`) + `TestReopen_NamespacedF06` a.p1/artX a.p2/artY b.q→artX → reopen b.q only a.p1; `go test -run TestReopen -race`
- [x] 3.5 Modify `internal/execution/state.go`: `ReopenStep` UNION (1) `depends_on` closure + (2) `produces→requires` feeder drill-down

## Phase 4: CLI, Fixtures, Inheritance

- [x] 4.1 Modify `internal/cmd/execute.go`: typed errors code+field; `steps next` filters `workflow` nodes
- [x] 4.2 Create `testdata/compose/**`: `basic`,`nested`,`reuse-twice`,`bindings`,`contract-violation`,`cycle-*`,`source-escape`,`symlink-escape`,`guards`,`workspace`
- [x] 4.3 Inheritance `system→root→step`: root-shared/isolated vs included defaults (not propagated) vs step overrides; one worktree, claims rewired; `go test -run TestWorkspace_Inheritance`
- [x] 4.4 `TestCompose_SingleWorktree`: one worktree per execution

## Phase 5: Verification

- [x] 5.1 U-01 `TestU01`: `basic`/`nested` hash twice identical; `go test -run TestU01`
- [x] 5.2 F-01–F-07 `TestCompose_F`: flat execution, namespaced reuse, contracts/bindings, containment/cycles/guards, cascade, parallel `steps next`; `go test ./... -run TestCompose_F -race`
- [x] 5.3 Guardrails `rg broker|PTY|leases`: broker/IPC/PTY/reporting/distribution/leases/interactions/path_claims/worktree/auth/`agents_command` untouched
- [x] 5.4 Gate `go vet`+`go test -race`+`golangci-lint run`; no execution/worktree on compose error
