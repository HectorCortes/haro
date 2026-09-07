# Archive Report: v2-reporte (change reporting — D06)

**Change**: v2-reporte
**Archived**: 2026-09-07
**Archived to**: `openspec/changes/archive/2026-09-07-v2-reporte/`
**Mode**: hybrid (OpenSpec files + Engram)
**Spec synced to**: `openspec/specs/v2-reporte/spec.md` (new domain — full spec: Purpose, Constraints, Requirements, Non-Requirements)

## Goal

Deliver D06 change reporting (F-01..F-03, U-01): capture a committed starting-point anchor at execution creation, expose one execution-scoped on-demand changed-files report accurate against real Git, and surface it through CLI-direct `haro report` — read-only, deterministic, and ready for a future thin `execution.report` wrapper.

## Final State (authoritative at close — outranks any intermediate snapshot)

```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:ccc55e5414e5e068d95332adf205530e05122ddf6908c6889bcbe99fc3f0b42e
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 6/6
scenarios: 8/8
```

- **Tasks**: 19/19 complete (`tasks.md` all `[x]`; Engram obs 2412). Task Completion Gate: PASS, no stale unchecked boxes.
- **Verification**: 6/6 requirements PASS, 8/8 scenarios PASS. Envelope `gentle-ai.verify-result/v1`, `evidence_revision` sha256:ccc55e5414e5e068d95332adf205530e05122ddf6908c6889bcbe99fc3f0b42e, verdict `pass_with_warnings`, blockers 0, critical_findings 0 (Engram obs 2416; file `verify-report.md`, evidence revision = commit `962f7b3`). D06 criteria trace: 4/4 (F-01..F-03, U-01) with passing evidence.
- **Gates green at close**: `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./... -race -count=1` exit 0 (14 packages, 1 root `[no test files]`), `golangci-lint run` 0 issues, `govulncheck ./...` clean. CRITICAL findings: 0, blockers: 0.
- **Warnings**: 3 non-blocking WARNINGs at close (no post-verify remediation was needed): (1) silent `ls-files` failure swallowed as "no untracked files" (`report.go` — fail-closed consideration per Constitution I.3, risk Low); (2) workspace existence decided by `os.Stat` alone (`report.go` — stale empty dir would fail closed with `not a git repository` instead of falling back, risk Low); (3) nullable `base_commit` for non-git roots yields `not_available` instead of failing `CreateExecution` — spec-conformant nullable semantics by design (existing rows MAY remain null). SUGGESTIONs recorded for future hardening: propagate `ls-files` errors, probe `.git` presence before treating a workspace as existing.
- **Delivery**: `single-pr` with maintainer-approved `size:exception` (review budget 200000, config review_budget_lines 20000); 1 ledger reset accepted by the maintainer for the apply line overrun (2937 > 1200). Diff ~2927 lines (≈2445 code/tests + docs), within the exception.
- **Commits** (4 on branch `feat/v2-reporte`, stacked on `feat/v2-composicion` — PR #5 open; both delivered as separate PRs): `2061e40` feat(store) base_commit anchor migration + HEAD capture on CreateExecution; `2dc790d` feat(execution) execution-scoped read-only report engine with deterministic git oracle; `02734dc` feat(cmd) wire `haro report <execution_id> [--json]` delegating to Engine.Report; `962f7b3` test(reporte) E2E and guardrails for deterministic change reporting. This commit `docs(sdd)` archives the change.
- **Strict TDD**: ACTIVE and followed — RED→GREEN per task (evidence in apply-progress obs 2413; 19/19 tasks with test files; TDD Compliance 6/6 checks; no behavior changes after verification).

## Informational Warnings (verdict `pass_with_warnings`, non-blocking)

1. **Silent `ls-files` failure swallowed** (`report.go`): `if lsOut, err := runGit(...); err == nil { ... }` treats a failed `ls-files` as "no untracked files", hiding untracked files without an error. Constitution I.3 (fail explicitly) would surface it. Risk Low — `ls-files` rarely fails where `diff` succeeds; no negative test exists. Recommendation: propagate as `fmt.Errorf("git ls-files: %w", err)`.
2. **Workspace existence via `os.Stat` alone** (`report.go`): a left-behind empty `.haro/worktrees/<id>` directory is treated as an existing workspace, so `runGit` fails with `not a git repository` instead of the graceful `base_commit..HEAD` repo-root fallback. Fail-closed (error, not silent data loss), risk Low. Recommendation: probe `os.Stat(filepath.Join(ws, ".git"))` or `git rev-parse --is-inside-work-tree`.
3. **Nullable `base_commit` for non-git roots**: `captureBaseCommit` treats "not a git repository"/missing git as nil anchor; terminal legacy/non-git executions then report `not_available` (field `base_commit`). Spec-conformant (`existing rows MAY remain null`), fail-closed for reporting; pass-with-note, not a defect.

## Sole Documented Amendment

- `deltas-acceptance.md` — D06 spec (`v2-reporte`, lines 392–409): all 4 criterion checkboxes (F-01..F-03, U-01) marked verified `[x]`; tracking row updated from `pending` to **complete** (4 criteria, 1 P0; verdict pass_with_warnings); "Last updated" summary line updated to 2026-09-07. No implementation-time doc amendments: no broker/UDS/JSON-RPC/PTY/leases/interactions/`agents_command`/`agentsFile` code and no changes to existing normative docs (guardrail greps + empty `go.mod` diff).

## Files Changed (implementation, per apply-progress)

| File | Action | What Was Done |
|---|---|---|
| `internal/store/migrations.go` | Modified | `ensureBaseCommitColumn` PRAGMA-guarded race-tolerant `ALTER TABLE executions ADD COLUMN base_commit TEXT` after `dag_hash` |
| `internal/store/store.go` | Modified | `Execution.BaseCommit *string` |
| `internal/store/repositories.go` | Modified | Create/Get `BaseCommit` NullString with `isMissingColumn` fallback for legacy DBs |
| `internal/execution/engine.go` | Modified | Capture `git rev-parse HEAD` in `resolvedRoot` before `worktree.Create`; fail → no row/worktree; dirty ignored; symlink clean; worktree removal timing unchanged |
| `internal/execution/report.go` | Created | `Report`/`ChangedFiles` execution-scoped read-only; terminal gate; nullable guard `not_available`; fixed-argv `git diff --name-only --diff-filter=ADMR --find-renames -z` + `git ls-files --others --exclude-standard -z`; NUL parse; `normalizePaths` via `claim.Canonicalize`; `.haro/**` filter; dedupe; sort; workspace selection shared/existing-isolated/removed-fallback (`base..HEAD`, no `ls-files`) |
| `internal/cmd/execute.go` | Modified | Route `report` delegating `Engine.Report`/`ChangedFiles`; `flag.FlagSet --json`; `--` terminator; EPIPE-safe `writeJSON`/`writeJSONError`; plain one path per line; JSON `{execution_id, base_commit, changed_files}` |
| 13 test files (store 1, execution 10, cmd 2) | Created | Migration PRAGMA idempotence, HEAD anchor, null-anchor, state/workspace, NUL workspace selection, normalization, U-01 8-case change table, threat matrix, read-only, composed E2E, git oracle, removed fallback, CLI plain/JSON parity |

## Deferrals and Exclusions (intentional, recorded)

- `v2-store` (persistence behind a repository interface — full DDL: `leases`, `interactions`, `step_transition_events`, repository facade), `v2-broker`/`v2-ipc` (broker UDS/wait queues, JSON-RPC `execution.report` — deferred per CLI-direct architecture; `Report` remains broker-ready as a thin future wrapper), `v2-distribucion`, PTY, auth policy. Report persistence (Approach 2 snapshot storage) MAY be a future additive change — explicitly not v1.

## Artifact Traceability (Engram observation IDs)

| Artifact | Engram obs ID | File in archive |
|----------|---------------|-----------------|
| explore | 2405 | `openspec/changes/archive/2026-09-07-v2-reporte/exploration.md` |
| proposal | 2406 | `openspec/changes/archive/2026-09-07-v2-reporte/proposal.md` |
| spec | 2407 | `openspec/changes/archive/2026-09-07-v2-reporte/spec.md` |
| design | 2410 | `openspec/changes/archive/2026-09-07-v2-reporte/design.md` |
| tasks | 2412 | `openspec/changes/archive/2026-09-07-v2-reporte/tasks.md` |
| apply-progress | 2413 | `openspec/changes/archive/2026-09-07-v2-reporte/apply-progress.md` |
| verify-report | 2416 | `openspec/changes/archive/2026-09-07-v2-reporte/verify-report.md` |
| archive-report | (this save) | `openspec/changes/archive/2026-09-07-v2-reporte/archive-report.md` |

## Review Gate

`reviewGate` structurally ABSENT — no review artifact was discovered for this candidate; receipt-driven development did not run for it. Archive proceeds under ordinary repository policy. Nothing to investigate.

## Archive Contents

- `proposal.md` ✅
- `exploration.md` ✅
- `spec.md` ✅ (delta spec, verbatim)
- `design.md` ✅
- `tasks.md` ✅ (19/19 tasks complete, no stale unchecked boxes)
- `apply-progress.md` ✅
- `verify-report.md` ✅ (envelope YAML header intact — staged and moved via `git mv`, content never re-written)
- `archive-report.md` ✅ (this file, additive)

## Source of Truth Updated

- `openspec/specs/v2-reporte/spec.md` — new domain spec created. The change's delta spec is ADDED-only (6 requirements, no MODIFIED/REMOVED/RENAMED), so the main spec was composed as a full spec (Purpose, Constraints, Requirements, Non-Requirements) per the archive convention, modeled on `openspec/specs/v2-composicion/spec.md`: Purpose and Constraints composed from the delta's normative anchors; the 6 ADDED requirements carried over verbatim with scenarios; Non-Requirements section carried over verbatim. Carried-over bodies verified byte-identical against the delta by `diff` (heading `## ADDED Requirements` renamed to `## Requirements`; delta section markers dropped as delta bookkeeping). The `rules.archive` "warn before merging destructive deltas" did not trigger: no `REMOVED`/`RENAMED` sections exist and the sync is purely additive.
- `deltas-acceptance.md` — D06 criteria checkboxes, tracking row, and summary line updated (see Sole Documented Amendment).

## Mechanical Verification

- Folder move: `git add openspec/changes/v2-reporte/verify-report.md` (untracked) then `git mv openspec/changes/v2-reporte openspec/changes/archive/2026-09-07-v2-reporte`. Pre-move recursive snapshot vs archived tree, `diff -r -x archive-report.md` → **empty (exit 0)**: byte-identical, no truncation or alteration. `archive-report.md` excluded because it did not exist in the source change folder (additive-only).
- Source directory confirmed gone after the move; active changes directory contains only `archive/`.
- Spec sync: header (Purpose/Constraints) composed by the model; requirement and Non-Requirement bodies copied with shell `sed` extraction, never re-typed, and byte-verified with `diff` (verbatim diff output in phase result).

## Next

- Post-archive: proceed to **`v2-store`** (persistence behind a repository interface) — the next pending row in the `deltas-acceptance.md` tracking table after `v2-reporte` (6 criteria, 3 P0: no direct SQLite access outside the store, complete v2 DDL incl. `leases`/`interactions`/`step_transition_events`, interchangeable backend, idempotent schema, enum CHECKs). `v2-broker`/`v2-ipc` remain deferred per the CLI-direct architecture. The `v2-no-regresion`, `v2-adapter`, and `v2-path-claims` rows remain `pending` as recorded — their archive cycles explicitly left `deltas-acceptance.md` untouched (tracking-row updates began with `v2-composicion`); `v2-distribucion` and `v2-flujo-gentle-ai` remain later work.
