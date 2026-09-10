# Apply Progress: v2 Supervised Guard

Date: 2026-09-10 · Mode: Strict TDD (`go test ./...`) · Delivery: `single-pr` with maintainer pre-approved `size:exception` (200000-line budget; ~155-line forecast) · Direct commits to `main`, not pushed (orchestrator pushes after verify).

## Status

8/8 tasks complete (tasks.md checkbox count: 8 checked, 0 open). Ready for verify.

## Commits (one per unit, local on `main`, NOT pushed)

| Unit | Commit | Message |
|---|---|---|
| U1 tests | `95f7f1b` | `test(execution): supervised mode fail-closed tests` |
| U2 guard | `fa7a2b6` | `feat(execution): reject supervised mode terminally at runtime` |

## TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.1 | `internal/execution/engine_test.go` (`TestSupervisedAgentStepFailsClosed`) | Integration (engine+store) | ✅ baseline `go test ./...` all green | ✅ `go test ./...`: only this test failed — `supervised mode must fail closed` (silent headless downgrade); 14 packages ok | ✅ `ok github.com/HectorCortes/haro/internal/execution 3.408s` | ✅ error contains `supervised mode not supported`; step+execution `failed`; `assertAttemptEvidence(..., 0)`; `NewSessionCount() == 0` with a usable adapter available | ➖ none needed |
| 1.2 | `internal/execution/engine_test.go` (`TestTerminalAgentStepRegressionStoreSeeded`) | Integration (engine+store) | ✅ same baseline | ⚠️ green from start BY DESIGN: regression pin of existing terminal rejection (cannot be RED without breaking terminal behavior); phase-level RED came from 1.1. Honest deviation from tasks.md "fails" wording, recorded here | ✅ same green run | ✅ store-seeded path (project row + running Execution, nil `DagHash`, `WorkflowSource` re-parse without Validate, pending agent step `[]` arrays) proves the guard is reachable without validation; zero attempts/sessions | ➖ none needed |
| 2.1 | `internal/execution/engine.go` (guard ~789) | Unit (engine) | ✅ | ✅ RED above | ✅ both tests pass; full `go test ./...` 15/15 packages ok | ✅ one parameterized branch `mode == "terminal" || mode == "supervised"`, reason `fmt.Sprintf("%s mode not supported", mode)`, `failAttempt("no-attempt")` + `fmt.Errorf` preserved | ✅ exact design block; no cleanup needed |
| 3.1 | gates | — | — | ➖ | ✅ `go test ./...` 15/15 ok; `go test ./... -race` 15 ok, 0 FAIL (ran locally) | ➖ | ➖ |
| 3.2 | gates | — | — | ➖ | ✅ `go vet ./...` clean; `go build ./...` clean | ➖ | ➖ |

## Work Unit Evidence

| Unit | Focused test command + result | Runtime harness + result | Rollback boundary |
|---|---|---|---|
| U1 | `go test ./...` → only `TestSupervisedAgentStepFailsClosed` failing (RED-by-design), 14 packages ok | N/A — fail-closed rejection has no positive runtime path to harness; zero-attempt/zero-session assertions prove no adapter entry | `engine_test.go` additions revert alone |
| U2 | `go test ./...` → 15/15 packages ok; `go test ./... -race` → 15 ok, 0 FAIL; `go vet ./...` clean; `go build ./...` clean | N/A — same reason; guard precedes harness configuration and intersection | `engine.go` guard + its tests revert as one unit |

## Final Gate — all four commands

- `go test ./...` → **15/15 packages ok** (execution 3.408s)
- `go test ./... -race` → **15 ok, 0 FAIL** (ran locally)
- `go build ./...` → **ok**
- `go vet ./...` → **clean**

## Key discoveries / gotchas

- The runtime terminal guard is only reachable through validation-bypassing paths: `CreateExecution` rejects `mode: terminal` during `ValidateFile`, so the terminal regression test must seed the execution directly through the store (project row + running Execution with `WorkflowSource`, nil `DagHash` skips hash re-flattening, pending agent step with `[]` dependency/require/produce arrays). This confirms the engine guard is the real enforcement boundary (engine re-parses the workflow with `workflow.Parse`, no Validate).
- Store-seeding recipe used (design.md): `Projects().Create(root, root)`; running `store.Execution` with `WorkflowSource=<root>/.haro/workflows/agentwf/workflow.yaml`, `WorkspaceMode=shared`, `WorkspaceRoot=root`, nil `DagHash`, nil `BaseCommit`; pending agent `ExecutionStep` with `"[]"` `DependsOn`/`Requires`/`Produces`, `WorkspaceMode=shared`, `CurrentGeneration=0`.
- Scope confirmed: combined diff `fd15ca7..fa7a2b6` touches only `internal/execution/engine.go` (+4/−3) and `internal/execution/engine_test.go` (+135). validate.go, fallback.go, adapters, store, CLI, docs, `deltas-acceptance.md` untouched; no new acceptance IDs.
- Delivery: `single-pr`, maintainer-pre-approved `size:exception` (200000-line budget), direct push to `main`, no PR; commits made locally, orchestrator pushes after verify+archive.
- Rollback boundary: revert both commits as one unit (guard + its tests).