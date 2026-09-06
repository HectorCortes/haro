# Design: v2 workflow composition

## Technical Approach

Plan D05 in `execution.Engine.CreateExecution`: validate the root, call pure `internal/workflow/compose.Flatten(rootPath, readFile)`, then validate/hash before creating a worktree or row. Persist executable steps and hash transactionally. This satisfies Constitution X.1–X.5/I.3 and preserves v2-path-claims; one caller does not justify a planner facade.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Parent-relative `source` | Path risk | Resolve from including file; realpath must remain under `<root>/.haro/workflows` |
| Root contracts | Unused API | Forbid with `root_contract_forbidden` |
| Complete bindings | More declarations | Fail closed on incomplete, unknown, or unsatisfied mappings |
| Included workspace | Nested defaults | Shape-check only; use `system → root → flattened step` |
| Guards | Bounds valid large DAGs | Depth 16, expansions 256, flattened steps 256 |
| Canonical hash | Migration | Persist nullable `executions.dag_hash` |
| Zod parity | No Go zod | Manual `yaml.Node` checks; U-04 asserts code/field |
| Worktree/claims | Stable integration | Keep one worktree and existing claims/filtering |

## Data Flow

```text
CreateExecution
  Discover root → ValidateFile(root) → Flatten(rootPath, readFile)
  → canonical realpath cycle/guard checks → ValidateFlat → canonical hash
  → worktree.Create (once) → WithTx[execution(dag_hash) + flat steps]
  failure before worktree/rows; transaction failure → existing worktree cleanup

ReopenStep(target, cascade)
  stored flat steps → retained depends_on-descendant closure
                    → flattened produces→requires feeder closure
  → union reset/invalidation effects → resync
```

`Flatten` prefixes IDs (`a.b.x`), expands repeated files independently, rewrites contracts, and orders topologically by source order then ID. Artifact edges are not scheduling edges. Runtime re-flattens `Execution.WorkflowSource`, checks its hash, and resolves the namespaced payload; leaked `workflow` nodes fail `workflow_invalid`.

### Reopen cascade

`ReopenStep = (1) RETAINED depends_on-descendant reset closure (reopened step + transitive dependents → pending, generations invalidated) PLUS (2) fine-grained flattened artifact-edge traversal (walk the reopened step's required artifacts backward through produces→requires to the transitive upstream feeder producers, invalidating ONLY those feeders whose produces actually feed the reopened artifacts)`.

For F-06, node `a` contains `a.p1` (produces `artX`) and `a.p2` (produces `artY`); `b.q` requires `artX`. Reopening `b.q` invalidates/pends `a.p1`, not `a.p2`; every transitive `depends_on` dependent of `b.q` is also pended with generations invalidated. This retains the `s1 → s2 → s3` behavior pinned by `internal/execution/state_test.go`.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/workflow/parse.go` | Modify | Add contracts, `Source`/`Bindings`, and strict-validation nodes |
| `internal/workflow/validate.go` | Modify | Add `ValidateFile`, `ValidateFlat`, typed errors |
| `internal/workflow/{compose.go,hash.go}` | Create | Bounded flattening and canonical SHA-256 |
| `internal/execution/{engine.go,state.go}` | Modify | Plan/persist/rehydrate guard and dual cascade |
| `internal/store/{migrations.go,repositories.go,store.go}` | Modify | Migrate and map nullable `dag_hash` |
| `internal/cmd/execute.go` | Modify | Surface typed errors; exclude workflow nodes |
| `testdata/compose/**` | Create | Composition, contract, cycle, escape, guard, and workspace fixtures |

## Interfaces / Contracts

```go
type ValidationError struct { Code, Field, Message string }
type ReadFile func(string) ([]byte, error)
func Flatten(rootPath string, readFile ReadFile) (*FlatDAG, error)
type FlatDAG struct { Steps []Step; Hash string }
```

Stable codes are `cycle_detected`, `workflow_invalid`, `contract_violation`, `root_contract_forbidden`, `unknown_input`, `unknown_output`, `missing_binding`, and `guard_exceeded`. `ValidateFlat` owns the cross-file check rejecting parent `depends_on` or `requires` that targets a non-declared internal step of an included file, returning `contract_violation@steps[i].depends_on[j]` or `contract_violation@steps[i].requires[j]`. Hash input covers ordered executable fields, sorted maps, dependencies, artifacts, and effective workspace.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | U-01–U-04: repeatability, cycles, contracts, guards, schema parity/hash | Table-driven in-memory readers; exact code/field; hash twice |
| Integration | ALTER migration idempotence, atomic no-row failures, bindings, both cascade closures, inheritance | File-backed `t.TempDir` SQLite and fake worktree; retain descendant regression and add namespaced F-06 fixture |
| E2E | F-01–F-07, parallel `steps next`, one worktree/claims | Compose fixtures; real temporary repo where needed; `-race` |

## Threat Matrix

N/A — composition adds file parsing, not routing, shell/subprocess, VCS/PR automation, executable classification, or process integration; worktree/claims mechanics remain unchanged.

## Migration / Rollout

This is the store's first ALTER-based migration: unlike the plain `CREATE TABLE IF NOT EXISTS attempt_transport` precedent, `ALTER TABLE executions ADD COLUMN dag_hash TEXT` must be guarded with `PRAGMA table_info(executions)` (or an equivalent column-existence check) before execution. Because `migrate` runs from `store.Open`, the guard and ALTER path must be re-run-safe on every connection and preserve existing rows. Rollback removes additive files and engine/CLI wiring but retains nullable `dag_hash`. In-flight namespaced executions are orphaned and MUST NOT resume.

## Risks and Boundaries

Realpath containment blocks CRITICAL symlink escape. Canonical identity detects renamed-node cycles; guards bound collisions; contracts prevent miswiring; hashes detect drift. Out of scope: broker/IPC, PTY, reporting, distribution, leases/interactions, path-claim/worktree mechanics, authentication, schema sugar, and context transports.

## Open Questions

None.
