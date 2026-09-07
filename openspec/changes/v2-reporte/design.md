# Design: v2 change reporting

## Technical Approach

Implement proposal Approach 1 for F-01–F-03/U-01 (`deltas-acceptance.md:392-408`): persist the committed anchor and compute one terminal execution report on demand. `internal/execution` invokes Git without a shell and exposes a broker-ready method to CLI-direct `haro report`. This preserves Constitution IX.5/I.6 (`haro-constitucion.md:155-158,16`) while deferring §6 JSON-RPC (`haro-especificacion-tecnica.md:513-518`).

## Architecture Decisions

| Option | Tradeoff | Decision and rationale |
|---|---|---|
| Committed `HEAD` | Ignores pre-existing dirt | Capture `git rev-parse HEAD`; it is the specified stable anchor. |
| On-demand vs persisted | Shared state may change | Compute on demand; persistence is future additive work, not v1. |
| Tracked diff plus untracked scan | Two subprocesses | Union both Git oracles to include new files. |
| Rename paths | Loses old name | Emit destination only, matching `--name-only --find-renames`. |
| Canonical output | Rejects suspicious output | Clean, slash-normalize, dedupe, sort; fail on escape. |
| Haro internals | Omits artifacts | Exclude `.haro` and `.haro/**`. |
| Report state errors | No partial results | Stable `not_found` and `not_completed` codes fail closed. |
| Removed isolated workspace | Cannot recover untracked files | Diff `base_commit..HEAD` at repo root; scan untracked only with an existing workspace. |
| Worktree lifetime | Removed state is lossy | Keep removal at `engine.go:1018-1023`; binding tests document it. |
| Read-only Git | Requires Git | Use only fixed-argv `rev-parse`, `diff`, `ls-files`; never mutate (I.6). |

## Data Flow

```text
CreateExecution: validate/flatten → resolve root → git rev-parse HEAD (cmd.Dir=root)
  → worktree.Create → WithTx[execution(base_commit) + flattened steps]

haro report <id> → store.Open → Engine.Report → execution lookup → terminal gate
  → shared/existing isolated: cwd=workspace, diff(base), ls-files(untracked)
  → removed isolated: cwd=repo root, diff(base, HEAD), no untracked
  → normalize/filter/sort → plain lines or JSON
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/store/{migrations,repositories,store}.go` | Modify | Add/map nullable `base_commit`; own migration. |
| `internal/execution/engine.go` | Modify | Capture anchor; preserve terminal removal. |
| `internal/execution/report.go` | Create | Errors, workspace selection, Git, normalization. |
| `internal/cmd/execute.go` | Modify | Route/parse report; use output helpers. |
| `internal/{store,execution,cmd}/*report*_test.go` | Create | Migration, Git tables, CLI, and composed E2E coverage. |

## Interfaces / Contracts

```go
type Execution struct { /* existing fields */ BaseCommit *string }
type Report struct { ExecutionID string; BaseCommit *string; ChangedFiles []string }
func (e *Engine) ChangedFiles(context.Context, string) ([]string, error)
func (e *Engine) Report(context.Context, string) (*Report, error)
```

After validation/flattening and before `worktree.Manager.Create`, resolve `e.root` with the manager's `EvalSymlinks`/clean pattern and run `git rev-parse HEAD` with `cmd.Dir=resolvedRoot`; failure creates no execution/worktree. `ensureBaseCommitColumn`, after `ensureDagHashColumn`, checks `PRAGMA table_info(executions)` then race-tolerantly runs `ALTER TABLE executions ADD COLUMN base_commit TEXT`. INSERT/SELECT and `sql.NullString` mapping follow `dag_hash`; ownership follows §7.2–7.3 (`haro-especificacion-tecnica.md:556-562`).

`Report` loads execution/project, requires `completed|failed` and a non-null anchor, and confines workspace to project root. Existing workspace argv is `git diff --name-only --diff-filter=ADMR --find-renames -z <base> --` plus `git ls-files --others --exclude-standard -z`; removed fallback adds `HEAD` before `--` and omits `ls-files`. NUL entries use claim-style clean/absolute/`..`/symlink checks; filter `.haro`, dedupe, sort. CLI uses `flag.FlagSet`, `writeJSON`/`writeJSONError`; JSON is `{execution_id,base_commit,changed_files}`, plain is one path per line.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | U-01 normalization/change table | Git-gated, short-aware temp repos: added, modified, deleted, renamed, untracked, duplicate, `.haro`, escape. |
| Integration | Migration and workspaces | File SQLite; idempotence, dirty anchor, shared/isolated/fallback, null anchor. |
| E2E | F-01/F-02/F-03, CLI parity | One composed execution/report, Git oracle, plain/JSON equality; HEAD/status unchanged. |

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: no executable classification | Paths are data only | None |
| Git repository selection | Applicable | Canonical engine-owned `cmd.Dir`; never `git -C`; bad roots fail | Relative/absolute roots, outside workspace, missing repo |
| Commit state | Applicable | Read staged/unstaged/empty-index state without mutation; never `commit -a` | Staged, unstaged, empty index; assert HEAD/index/status unchanged |
| Push state | N/A: no push command | No remote operation | None |
| PR commands | N/A: no PR automation | No composed command | None |

## Migration / Rollout

Nullable migration needs no backfill or flag. Rollback reverts CLI/engine/repository wiring; retain `base_commit` per `dag_hash` precedent or drop it. No report is orphaned.

## Risks and Boundaries

Removed isolated worktrees irreversibly lose untracked/uncommitted final state; fallback and tests make this binding without changing removal timing. Git rename/path behavior is controlled by fixed argv and canonicalization. Dirty starts intentionally anchor committed HEAD. Out of scope: content/line diffs, per-step/workflow or persisted reports, commit/push/merge/publish/discard, broker/UDS/JSON-RPC, PTY, leases/interactions, other DDL, new dependencies, agents artifacts, and worktree-lifetime changes.

## Open Questions

None.
