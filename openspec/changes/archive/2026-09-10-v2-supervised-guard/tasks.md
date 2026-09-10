# Tasks: v2 Supervised Guard

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~155 (engine.go guard ~5; engine_test.go tests ~150) |
| 400-line budget risk | Low |
| Chained PRs recommended | No — small fail-closed change |
| Suggested split | 2 work-unit commits, direct push to main, no PR |
| Delivery strategy | single-pr; maintainer-pre-approved size:exception (200000-line budget); direct push to main |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Low

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|
| U1 | RED tests | `go test ./... -run TestEngine` | N/A — fail-closed rejection has no positive runtime path to harness | engine_test.go additions revert alone |
| U2 | Guard + gate | `go test ./... -race` | N/A — same reason; zero-attempt/zero-session assertions prove no adapter entry | engine.go guard + its tests revert as one unit |

Strict TDD: RED first, run `go test ./...` before GREEN. Criterion: `v2-no-regresion/F-02`, scenario "Unsupported declared execution modes". Threat matrix: only the adapter/process-boundary row applies (RED tests prove no adapter session); all other rows N/A per design.

## Phase 1: RED tests (internal/execution/engine_test.go)

- [x] 1.1 RED supervised rejection (F-02, scenario case 1): CreateExecution with `mode: supervised` workflow via `newAgentEngine` + `setupFakeManager` with available `fakeHarnessAdapter`. Assert: error contains `supervised mode not supported`; `assertStepStatus` step and execution `failed`; `assertAttemptEvidence(..., 0)`; adapter `NewSessionCount() == 0`. Verify: `go test ./...` — new test fails, rest green.
- [x] 1.2 RED terminal regression, store-seeded (F-02, scenario case 2): bypass CreateExecution (validation rejects terminal) — seed project row; `running` `store.Execution` with `WorkflowSource` pointing at terminal-mode workflow file on disk, shared/root workspace, nil `DagHash`; pending agent `store.ExecutionStep` with `[]` depends_on/requires/produces. `RunStep` re-parses the file (no Validate) and reaches the guard. Assert `terminal mode not supported`, step/execution `failed`, 0 attempts, 0 sessions. Verify: `go test ./...` — fails.

## Phase 2: GREEN guard (internal/execution/engine.go)

- [x] 2.1 Extend mode guard (~line 789) to one parameterized branch `mode == "terminal" || mode == "supervised"` with `reason := fmt.Sprintf("%s mode not supported", mode)`, preserving existing `failAttempt("no-attempt", ...)` + `fmt.Errorf` pattern. Nothing else changes (validate.go, fallback.go, adapters, store, CLI, docs, deltas-acceptance.md untouched). Verify: `go test ./...` fully green.

## Phase 3: Gates

- [x] 3.1 `go test ./...` green (full suite); `go test ./... -race` if locally feasible.
- [x] 3.2 `go vet ./...` and `go build ./...` clean.

## Phase 4: Commits (work-unit-commits)

- [x] 4.1 Unit 1: `test(execution): supervised mode fail-closed tests` — RED tests 1.1 + 1.2 (RED-by-design until unit 2 lands next).
- [x] 4.2 Unit 2: `feat(execution): reject supervised mode terminally at runtime` — guard 2.1; suite green at this commit.
- [x] 4.3 Confirm scope: `git status` shows only engine.go and engine_test.go; no docs(sdd) commit here (archive phase owns it).
