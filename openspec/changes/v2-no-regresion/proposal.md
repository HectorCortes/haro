# Proposal: Preservation of Specs 1–9 — Go re-expression

## Intent

Preserve v1 semantics through `haro init`, `workflows list/describe`, `run`, `steps next`, `step run/reopen/skip`, and `status`. A minimal Store replaces §6 Broker/JSON-RPC under Constitution II.4.

## Scope

### In Scope
- Idempotent init; discovery/validation; workflow fields; DAG validation; `validateContainedPath`.
- Command-cycle cases, dependency guard, feedback, generations/cascade/skip audit, budgets/redaction, JSON/EPIPE/`--`, state-machine tests.
- Store-backed idempotent tables: projects, executions, execution_steps, generations, attempts, attempt_events, step_transition_events.
- Strict TDD; neutral fixtures; Go gate.

### Out of Scope
- Agent/supervised/bundle/permission/diagnostics → adapter/broker/IPC; resume → broker/store; PTY deferred.
- Full DDL/backends, claims, composition, reporting, distribution → owners.
- TypeScript/verification governance debt → separate reconciliation (spike evidence); normative docs untouched.

## Capabilities

### New Capabilities
- `v2-no-regresion`: Go command-surface preservation.

### Modified Capabilities
- None.

## Approach

Use one PR, strict TDD, stdlib `flag`, pure Go/no cgo, provider-neutral core, Store interfaces, idempotent migrations, and immediate budgets.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `main.go`, `cmd/haro/` | Modified/New | CLI. |
| `internal/workflow/` | Modified | Schema/DAG/paths. |
| `internal/store/`, `internal/execution/` | Modified/New | State. |
| `testdata/` | Modified | Fixtures. |

## Risks

| Risk | L | Mitigation |
|---|---|---|
| Governance drift | H | Mapping/debt. |
| DDL lock-in | M | Store. |
| Scope bleed | M | Cut-list grep. |
| CLI ambiguity | L | Stdlib `flag`. |
| Budget regression | M | Fixture tables. |

## Rollback Plan

Revert commits; tables are idempotent; no external state.

## Dependencies

- `deltas-acceptance.md`; Constitution; technical specification; dependencies.

## Success Criteria and Contract Mapping

Proofs: **A** `go test ./... -race -run`; **B** `go vet ./... && golangci-lint run && govulncheck ./...`.

| Contract | Outcome / proof |
|---|---|
| F-01 [E2E] | [ ] init; `TestInitIdempotent` |
| F-02 [E2E] | [ ] discovery; `TestWorkflowsDiscovery` |
| F-03 [E2E] | [ ] command cycle; `TestCommandCycle` |
| F-04 [E2E] | Deferred → adapter |
| F-05 [INT] | [ ] feedback; `TestFeedback`; agent deferred |
| F-06 [INT] | Deferred → broker/store |
| F-07 [INT] | [ ] generations; `TestReopenSkip` |
| F-08 [INT] | Deferred → adapter/store |
| F-09 [E2E] | Deferred → broker/IPC/adapter |
| F-10 [E2E] | Deferred → PTY policy |
| F-11 [INT] | Deferred → adapter |
| F-12 [INT] | [ ] budgets; `TestEvidenceBudgets`; agent deferred |
| F-13 [INT] | [ ] containment; `TestContainment` |
| F-14 [E2E] | [ ] JSON/EPIPE/args; `TestCLIContract` |
| U-01 [UNIT] | [ ] Go suite/gates; A+B |
| U-02 [UNIT] | Deferred → adapter (no consumption surface; originals unavailable) |
| U-03 [UNIT] | [ ] transitions; `TestStateMachine`; fencing deferred |
