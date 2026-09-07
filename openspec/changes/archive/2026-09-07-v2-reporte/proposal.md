# Proposal: v2-reporte (change reporting)

## Intent

Deliver `v2-reporte` F-01–F-03/U-01 (1×P0, 3×P1), constitution §IX.5.1–IX.5.2/I.6, and technical specification §6 `execution.report`.

## Scope

### In Scope
- At `CreateExecution`, capture `git rev-parse HEAD` in `resolvedRoot` as nullable `executions.base_commit`; dirty files are excluded. Own the idempotent `PRAGMA table_info`-guarded ALTER like `dag_hash`.
- Add execution-scoped, side-effect-free `ChangedFiles(ctx, executionID)`/`Report` for terminal executions; unknown/non-terminal fail as `not_found`/`not_completed`. Fixed-argv `git diff --name-only --diff-filter=ADMR --find-renames` plus `git ls-files --others --exclude-standard` excludes `.haro/**`, emits rename new paths, rejects `../` escapes, de-duplicates, and sorts canonical repo-relative paths.
- Shared mode runs at repo root. Isolated mode uses an existing `workspace_root`; after terminal removal it falls back to repo-root diff from `base_commit`, scanning untracked files only when the workspace exists. Keep current removal timing.
- Add CLI-direct `haro report <execution_id> [--json]`; plain is one path per line; JSON is `{execution_id, base_commit, changed_files}`. Keep it broker-ready.
- Add git-gated, `testing.Short`-aware temp-repo tables for new/modified/deleted/renamed/untracked files, store/CLI tests, and E2E proof for F-01/F-02 top-level composition and F-03 Git-oracle accuracy.

### Out of Scope
- no content diffs/line stats; no per-step/per-workflow sub-reports (F-02); no commit/push/merge/publish/discard (I.6 — v2-distribucion owns publishing); no broker/UDS/JSON-RPC in this change; no PTY; no leases/interactions or other v2-store DDL beyond base_commit; no new dependencies (pure Go/no cgo); no agents_command/agentsFile; no worktree-lifetime change.

## Capabilities

### New Capabilities
- `v2-reporte`: deterministic, read-only top-level execution change reporting.

### Modified Capabilities
- None.

## Approach

Use exploration Approach 1: compute on demand after terminal state; “computed once” means one execution-level report. `base_commit` anchors the diff; untracked union matches Git; read-only commands satisfy I.6. Approach 2 persistence remains an additive follow-up if literal write-once snapshots are needed. JSON-RPC `execution.report` is deferred to `v2-broker`/`v2-ipc` as a thin wrapper.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/store` | Modified | Migration/repositories |
| `internal/execution` | Modified/New | Anchor/report |
| `internal/cmd/{execute,output}.go` | Modified | CLI/output |
| `internal/worktree` | Unchanged | Removal policy |
| tests | New | Tables/E2E |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Removed isolated worktree loses final/untracked state | High | Fallback, binding tests, optional later persistence |
| Untracked/rename/path oracle drift | Med | Union, rename-new-path rule, canonicalization, Git comparison |
| Dirty starting tree surprises users | Med | Document committed-HEAD semantics |

## Rollback Plan

Revert CLI/engine/repository wiring; retain nullable `base_commit` per `dag_hash` precedent (or drop it). No stored report can be orphaned.

## Dependencies

- Normative: `docs/reference/SPECS.md`, `docs/v2/haro-constitucion.md`, `docs/v2/haro-especificacion-tecnica.md`; analysis: `exploration.md`; Git binary.

## Success Criteria

- [ ] F-01–F-03 and U-01 pass reproducibly, including top-level-only reporting and Git-oracle accuracy.
- [ ] All commands remain read-only and existing quality gates pass without ownership leakage.
