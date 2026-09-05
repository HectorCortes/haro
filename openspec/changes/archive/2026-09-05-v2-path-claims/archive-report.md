# Archive Report: v2-path-claims (path_claims and workspace isolation)

**Change**: v2-path-claims
**Archived**: 2026-09-05
**Archived to**: `openspec/changes/archive/2026-09-05-v2-path-claims/`
**Mode**: hybrid (OpenSpec files + Engram)
**Spec synced to**: `openspec/specs/v2-path-claims/spec.md` (new domain — full spec, non-destructive)

## Final State (authoritative at close — outranks any intermediate snapshot)

- **Tasks**: 19/19 complete (`tasks.md` all `[x]`; Engram obs 2355). Task Completion Gate: PASS, no stale unchecked boxes.
- **Verification**: 10/10 requirements PASS, 12/12 scenarios PASS. Envelope `gentle-ai.verify-result/v1`, `evidence_revision` sha256:fc61e405c7599242c414558c82a339575e506b33ef026a1fcb796de7ae5fa980, verdict `pass_with_warnings`, blockers 0, critical_findings 0 (Engram obs 2359; file `verify-report.md`).
- **Gates green at close**: `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./... -race` exit 0 (14 packages), `golangci-lint run` 0 issues, gated git worktree E2E pass (`TestManagerThreatMatrix`, real git `worktree list`). CRITICAL findings: 0, blockers: 0. (per verify-report obs 2359, final — archive permitted.)
- **Runtime ledger**: apply attempt recorded passed (maintainer reset for changed-line budget, reason recorded); verify attempt settled complete.
- **Commits**: `a7bbaa2`, `31857a6`...`dd1742e` — work units: `feat(store)` (path_claims DDL + atomic repository), `feat(claim)` (canonical + policy), `feat(worktree,workflow)` (worktree manager + workspace inheritance), `feat(execution,cmd)` (worktree/claim lifecycle with CLI isolation); `docs(sdd)` (proposal/spec/design/exploration); this commit `chore(sdd)` (archive + spec sync).
- **Changed lines (REAL, recorded per orchestrator handoff)**: ~2634 changed lines; `size:exception` approved 2026-09-04 for delivery `single-pr`. Forecast was 950–1100; not a failure — recorded as the real number.
- **Strict TDD**: ACTIVE and followed — RED→GREEN→TRIANGULATE per task (evidence in apply-progress obs 2355; 19/19 tasks with test files; 11/11 RED test files verified present; re-verified under `-race`). Apply phase-contract validation PASS recorded (fresh gates: build/vet/test `-race -short` 15 packages/lint 0). No cosmetic fixes were needed after validation.

## Informational Warnings (verdict `pass_with_warnings`, non-blocking)

1. **Design-listed file-path deviation**: `internal/store/repositories.go`, `internal/execution/state.go`, `internal/cmd/output.go`, `internal/project/init.go` not modified; equivalents delivered in `store.go` (facade `PathClaims()`), `engine.go` (resync/remove inline), `execute.go` (`writeJSON` logical_conflict), `config.go` (sole `.haro/config.yaml` concern). File-path equivalences, not scope gaps.
2. **Test naming variants**: design expected unified `claim/*_test.go` names; delivered as `workspace_test.go`, `engine_claim_test.go`, `logical_conflict_test.go` — semantically identical.
3. **Real-git E2E scope**: gated `TestManagerThreatMatrix` covers worktree lifecycle only (Create/Remove/Prune/List); claim/conflict/steps-next lifecycle covered via `FakeManager`/`FakeRunner` integration — reported accurately in the verify report.

SUGGESTION (non-blocking, from verify): dedicated failure-release integration forcing `RunStep` with a failing runner (currently triangulated via defer code inspection + store owner-check).

## Sole Documented Amendment

- `docs/v2/haro-especificacion-tecnica.md` §7.3 — sole-ownership note for `path_claims`: `Acquire(ctx, PathClaim)`, idempotent `CREATE TABLE IF NOT EXISTS` plus partial active index, sole migration owner `v2-path-claims` — `v2-store` MUST NOT diverge; rollback retains schema. This is the only modified doc in the change; `deltas-acceptance.md` untouched.
- `.gitignore` gained `.haro/worktrees/` (managed worktrees ignored).

## Deferrals and Exclusions (intentional, recorded)

- `leases`, `interactions`, remaining 11-table DDL (`v2-store`); broker UDS/wait queue (`v2-broker`, `v2-ipc`); composition; PTY; reporting; distribution. All remain out of scope per proposal/design and owned by later v2 changes.

## Artifact Traceability (Engram observation IDs)

| Artifact | Engram obs ID | File in archive |
|----------|---------------|-----------------|
| exploration | 2345 | `openspec/changes/archive/2026-09-05-v2-path-claims/exploration.md` |
| proposal | 2347 | `openspec/changes/archive/2026-09-05-v2-path-claims/proposal.md` |
| spec | 2348 | `openspec/changes/archive/2026-09-05-v2-path-claims/specs/v2-path-claims/spec.md` |
| design | 2349 | `openspec/changes/archive/2026-09-05-v2-path-claims/design.md` |
| tasks | 2351 | `openspec/changes/archive/2026-09-05-v2-path-claims/tasks.md` |
| apply-progress | 2355 | `openspec/changes/archive/2026-09-05-v2-path-claims/apply-progress.md` |
| verify-report | 2359 | `openspec/changes/archive/2026-09-05-v2-path-claims/verify-report.md` |

## Review Gate

`reviewGate` structurally ABSENT — no review artifact was discovered for this candidate; receipt-driven development did not run for it. Archive proceeds under ordinary repository policy. Nothing to investigate.

## Archive Contents

- `proposal.md` ✅
- `exploration.md` ✅
- `specs/v2-path-claims/spec.md` ✅
- `design.md` ✅
- `tasks.md` ✅ (19/19 tasks complete, no stale unchecked boxes)
- `apply-progress.md` ✅
- `verify-report.md` ✅ (envelope YAML header intact — moved via `git mv`, content never re-written)
- `archive-report.md` ✅ (this file, additive)

## Source of Truth Updated

- `openspec/specs/v2-path-claims/spec.md` — new domain spec synced (full spec, non-destructive). The `rules.archive` "warn before merging destructive deltas" did not trigger: the delta is a full new-domain spec with no `REMOVED`/`RENAMED`/`MODIFIED` destructive sections.

## Mechanical Verification

- Spec sync: `diff -r` source delta vs `openspec/specs/v2-path-claims/spec.md` → empty (identical, exit 0); mode 644.
- Folder move: `diff -r` pre-move snapshot vs `openspec/changes/archive/2026-09-05-v2-path-claims/` → empty (identical, exit 0). `archive-report.md` excluded (did not exist in snapshot). Renames detected at 100% (untracked `verify-report.md` staged with `git add` before `git mv`).
- Active changes directory no longer contains `v2-path-claims` (only `archive/` remains).

## Next

- Post-archive: proceed to `v2-composicion` (workflow composition) per the contract tracking table order (`deltas-acceptance.md`). `v2-broker`/`v2-ipc` remain deferred per the CLI-direct architecture.