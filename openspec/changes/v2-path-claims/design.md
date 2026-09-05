# Design: v2 path claims and workspace isolation

## Technical Approach

`execution.Engine` will own worktree and claim lifecycles. `CreateExecution` resolves `system → workflow → step` settings (defaults `isolated/block`), creates `.haro/worktrees/<executionID>` when needed, and persists effective mode/root. Before command or agent execution crosses `pending → running`, Engine canonicalizes/deduplicates `requires ∪ produces`, acquires claims, and defers owner-checked release on every return. Terminal resync removes the worktree; startup prunes orphans.

`v2-path-claims` solely owns idempotent `path_claims` DDL, following archived `attempt_transport` migration/`WithTx` patterns. Broker waiting is deferred: CLI returns owner-bearing `logical_conflict`; `steps next` omits blocked steps.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Real `path_claims` now | Splits 11-table migration; proves atomicity | Chosen; `v2-store` verifies, never redefines |
| Worktree in `CreateExecution` | Eager cost; stable F-01 root | Chosen over lazy creation |
| R1 deferred release | Shared wrapper; covers every return | Chosen over execution-end release |
| `exec.CommandContext` Git manager | External Git; no dependency/shell | Chosen with `FakeWorktree` |
| `internal/claim` identity | Overlaps artifact containment | Chosen for stable logical identity |

## Data Flow

```text
config + workflow → resolve settings → worktree.Create → execution/steps Store
RunStep → canonicalize paths → BEGIN IMMEDIATE → scan active prefixes → INSERT
        → pending→running → runner/adapter at step workspace_root → defer Release
        → terminal resync → worktree.Remove; startup → worktree.Prune
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/worktree/manager.go` | Create | Fixed-argv Git add/remove/prune, resolved cwd, idempotent cleanup |
| `internal/claim/{canonical.go,policy.go}` | Create | Clean/reject absolute or escaping paths, root-anchored `EvalSymlinks`, boundary predicate (`a==b || HasPrefix(a,b+separator)`), four-row policy, external-as-shared |
| `internal/store/{migrations.go,store.go,claim.go,repositories.go}` | Modify/Create | DDL, repository/facade, immediate transaction, step root |
| `internal/workflow/{parse.go,validate.go}` | Modify | Pointer `WorkspaceConfig` fields, value validation, inheritance resolver |
| `internal/project/{config.go,init.go}` | Create/Modify | Minimal `external_paths`; preserve configs/empty default |
| `internal/execution/{engine.go,state.go}` | Modify | Inject manager, create/cleanup worktrees, effective cwd, acquire/release wrapper |
| `internal/cmd/{execute.go,output.go}` | Modify | Owner-bearing typed conflict output and claim-aware `steps next` |
| Package tests, `testdata/` | Create/Modify | RED fixtures and unit/integration/E2E coverage |
| `.gitignore`, `docs/v2/haro-especificacion-tecnica.md` | Modify | Ignore managed worktrees; add §7-style sole-ownership/API note |

## Interfaces / Contracts

`WorkspaceConfig{Mode, OnLogicalConflict *string}` appears on workflow and step. `ExecutionStep` stores effective mode/root: shared uses repo root; isolated uses execution worktree. `PathClaim` columns are `id, project_id, logical_path, mode, owner_execution_id, owner_step_id, acquired_at, released_at`; mode is `isolated:<executionID>` or `shared` (including externals). `Acquire(ctx, claim, policy)` returns `(bool, *PathClaim, error)`; `Release`/`ListActive` follow §5.

Migration uses `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS idx_path_claims_active ON path_claims(project_id, logical_path) WHERE released_at IS NULL`. `Acquire` reserves a SQLite connection, runs `BEGIN IMMEDIATE`, scans unreleased prefix overlaps, applies `ShouldBlock`, and inserts before commit. Incompatible contenders serialize to one winner; isolated claims coexist. Release matches project/path/execution/step and errors on zero rows.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | Canonical/symlink edges, prefixes, policy/external matrix, wrong-owner release, Git argv/idempotence | Table-driven RED → GREEN → TRIANGULATE; `t.TempDir`, fakes |
| Integration | Inheritance fixtures and persisted roots; `external_paths`; 8–16 concurrent Acquire calls | File-backed `t.TempDir()/claims.db`, goroutines, exactly one incompatible winner, `-race`; never `:memory:` |
| E2E | `git worktree list`; block/allow; isolated concurrency; release after success and failure | Real temporary Git repo; skip under `testing.Short()` or failed `exec.LookPath("git")` |

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: no executable classifier | Commands remain explicit argv | N/A |
| Git repository selection | Applicable | Resolved root is `cmd.Dir`; generated destination cannot escape; no `git -C`; invalid root fails closed | Relative/absolute/symlink roots; fixed argv/cwd; escape rejection |
| Commit state | N/A: no staging or commit operation | Worktree starts from explicit `HEAD` | N/A |
| Push state | N/A: no push operation | None | N/A |
| PR commands | N/A: no PR automation | None | N/A |

## Migration / Rollout

Migration/index are idempotent; a guarded migration adds step root. Rollback removes gating/worktrees but retains schema. Leases, interactions, remaining 11-table DDL, broker/UDS/wait queue, composition, PTY, reporting, and distribution are out of scope; `v2-store` must not diverge.

## Risks

| Risk | Mitigation |
|---|---|
| CRITICAL symlink escape | Resolve root/candidate and reject boundary escape; adversarial table |
| Worktree leak or Git-less CI | Terminal cleanup, startup prune, fakes, Git-gated E2E |
| SQLite concurrency mismatch | File-backed WAL database, `BEGIN IMMEDIATE`, goroutines and `-race` |
| Migration divergence | Sole-owner note and repeat-schema test |
| External schema drift | Only `external_paths`; canonical prefix table |
| Inheritance default drift | Explicit isolated/block resolver tests |

## Open Questions

None.
