# Apply Progress: v2-path-claims

**Change**: v2-path-claims
**Mode**: Strict TDD
**Delivery**: single-pr size:exception (approved 2026-09-04)
**Date**: 2026-09-05

## Summary

All 19 tasks implemented across 4 work units as single PR commits. Pure Go, no new dependencies, file-backed SQLite concurrency, git-gated E2E with Short skip, BEGIN IMMEDIATE atomicity, anchor EvalSymlinks containment, fixed-argv git manager, and claim-aware CLI.

## Work Unit Evidence

| Unit | Goal | Focused Test | Result | Runtime Harness | Result | Rollback Boundary |
|------|------|--------------|--------|-----------------|--------|-------------------|
| 1 | Canonical+policy | `go test ./internal/claim -race` | ok 1.0s | N/A pure funcs | N/A | `internal/claim/` |
| 2 | Store DDL+repo | `go test ./internal/store -run TestAcquire -race` | ok 1.3s | File DB 8 workers BEGIN IMMEDIATE single winner | ok 1.7s | `internal/store/migrations.go,claim.go,store.go` |
| 3 | Worktree+workflow | `go test ./internal/worktree -race` | ok 1.0s | gated git worktree list/prune | ok 1.0s (skip Short) | `internal/worktree/, workflow/, project/config.go` |
| 4 | Engine+CLI+docs | `go test ./internal/execution -run TestRunStepClaim -race` | ok 1.1s | temp git + claim logical_conflict + steps next filter | ok 1.2s | `internal/execution/, cmd/, .gitignore, docs/` |

Full gate: `go build ./...` ok, `go vet ./...` ok, `go test ./... -race -short` ok (all 14 packages), `golangci-lint run` 0 issues.

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | `internal/claim/canonical_test.go` | Unit | ✅ 13/13 baseline | ✅ undefined Canonicalize | ✅ Passed | ✅ anchored root vs src/link->/etc | ✅ Clean |
| 1.2 | `internal/claim/canonical.go` | Unit | N/A new | ✅ Written | ✅ Passed | ✅ 3 cases symlink/escape | ✅ Clean |
| 1.3 | `internal/claim/canonical_test.go` | Unit | ✅ 1.0s | ✅ Written | ✅ Passed | ✅ 2 symlink escapes | ✅ Clean |
| 1.4 | `internal/claim/policy_test.go` | Unit | ✅ baseline | ✅ undefined ShouldBlock | ✅ Passed | ✅ external boundary | ✅ Clean |
| 1.5 | `internal/claim/policy.go` | Unit | N/A | ✅ Written | ✅ Passed | ✅ 4-row matrix+IsExternal | ✅ Clean |
| 1.6 | `internal/store/claim_test.go` | Integration | ✅ 0.1s prior store | ✅ undefined PathClaim | ✅ Passed | ✅ isolated coexist | ✅ Clean |
| 1.7 | `internal/store/claim.go` | Integration | N/A | ✅ Written | ✅ Passed | ✅ 8-worker file DB | ✅ Clean |
| 2.1 | `internal/workflow/workspace_test.go` | Unit | ✅ baseline | ✅ undefined Workspace | ✅ Passed | ✅ defaults | ✅ Clean |
| 2.2 | `internal/workflow/parse.go` | Unit | N/A | ✅ Written | ✅ Passed | ✅ resolver isolated/block | ✅ Clean |
| 2.3 | `internal/worktree/manager_test.go` | Unit | ✅ N/A new | ✅ undefined FakeManager | ✅ Passed | ✅ threat-matrix gated git | ✅ Clean |
| 2.4 | `internal/worktree/manager.go` | Unit | N/A | ✅ Written | ✅ Passed | ✅ fixed argv/resolved cwd/idempotent | ✅ Clean |
| 2.5 | `internal/project/config_test.go` | Unit | ✅ baseline | ✅ undefined LoadConfig | ✅ Passed | ✅ empty default | ✅ Clean |
| 3.1 | `internal/execution/engine_claim_test.go` | Integration | ✅ prior exec 1.2s | ✅ undefined SetWorktreeManager | ✅ Passed | ✅ shared vs isolated worktree | ✅ Clean |
| 3.2 | `internal/execution/engine.go` | Integration | N/A | ✅ Written | ✅ Passed | ✅ prune/resync Remove | ✅ Clean |
| 3.3 | `internal/execution/engine_claim_test.go` | Integration | ✅ | ✅ Written | ✅ Passed | ✅ wrong-owner reject | ✅ Clean |
| 3.4 | `internal/execution/engine.go` | Integration | N/A | ✅ Written | ✅ Passed | ✅ failure defer Release | ✅ Clean |
| 3.5 | `internal/cmd/logical_conflict_test.go` | Integration | ✅ baseline | ✅ code run_failed want logical_conflict | ✅ Passed | ✅ steps next filter | ✅ Clean |
| 4.1 | E2E gated | E2E | ✅ 14 pkgs | ✅ Short skip | ✅ Passed | ✅ temp git scenarios | ✅ Clean |
| 4.2 | `.gitignore` + docs | Docs | N/A | ✅ Written | ✅ Passed | ➖ Single | ✅ Clean |

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/claim/canonical.go` | Created | Root-anchored EvalSymlinks, rejects absolutes/.. escapes, boundary predicate IsPrefix/Overlaps |
| `internal/claim/policy.go` | Created | ShouldBlock 4-row matrix + IsExternal forces shared |
| `internal/store/migrations.go` | Modified | Added path_claims DDL with FK/NOT NULL + partial index idx_path_claims_active |
| `internal/store/claim.go` | Created | PathClaimRepository with BEGIN IMMEDIATE conflict scan INSERT single winner, owner-checked Release, ListActive, Store.PathClaims |
| `internal/store/store.go` | Modified | Added PathClaims() to Store facade |
| `internal/workflow/parse.go` | Modified | Added WorkspaceConfig + ResolveWorkspace system→workflow→step (isolated/block defaults) |
| `internal/workflow/validate.go` | Modified | Validation for mode isolated\|shared and on_logical_conflict block\|allow |
| `internal/worktree/manager.go` | Created | RealManager with fixed argv git worktree add/remove/prune, resolved cwd, idempotent, FakeManager |
| `internal/project/config.go` | Created | LoadConfig for .haro/config.yaml external_paths empty default |
| `internal/execution/engine.go` | Modified | Injected worktree manager, CreateExecution inheritance+worktree, acquire/release wrapper canonicalize, defer Release success/failure, logical_conflict owner error, wtRoot runner cwd, resync Remove, startup prune |
| `internal/execution/engine_claim_test.go` | Created | RED→GREEN for workspace and claim lifecycle |
| `internal/cmd/execute.go` | Modified | logical_conflict structured output with owner fields, claim-aware steps next filter via Overlaps/ShouldBlock/IsExternal |
| `internal/cmd/logical_conflict_test.go` | Created | RED→GREEN for logical_conflict and filtered steps next |
| `.gitignore` | Modified | Added .haro/worktrees/ |
| `docs/v2/haro-especificacion-tecnica.md` | Modified | Added §7.3 sole-ownership note for path_claims (Acquire ctx,PathClaim, retain schema) |
| Tests: `internal/claim/*_test.go`, `internal/store/claim_test.go`, `internal/workflow/workspace_test.go`, `internal/worktree/manager_test.go`, `internal/project/config_test.go` | Created/Modified | Strict TDD coverage |

## Deviations from Design

None — implementation matches design. Acquire signature `Acquire(ctx, PathClaim)` as implemented; on_logical_conflict defaults to block (allow path tested via ShouldBlock unit, not store repo, per spec allow only via explicit allow fixture). Manager.Create uses `--detach HEAD` as per design.

## Issues Found

None. Concurrency fidelity required file-backed DB with `file:?cache=shared` and `BEGIN IMMEDIATE` via dedicated Conn; validated with 8-worker racetest. Symlink escape anchoring verified via src/link->/etc triangulate.

## Remaining Tasks

None — 19/19 complete.

## Workload / PR Boundary

- Mode: single PR size:exception
- Work units as commits: 4 (canonical+policy, store, worktree+workflow+config, engine+CLI+docs)
- Boundary: change starts from no-worktree/no-claim baseline, ends with full workspace isolation and claim coordination, rollback retains path_claims schema
- Estimated review budget impact: 950-1100 lines, single PR approved

## Status

19/19 tasks complete. Ready for verify (`go test ./... -race`).

## Verification Commands

- `go test ./internal/claim -race` → ok
- `go test ./internal/store -run TestAcquire -race` → ok
- `go test ./internal/worktree -race` → ok (gated git)
- `go test ./internal/execution -run TestRunStepClaim -race` → ok
- `go test ./internal/project -race` → ok
- `go test ./internal/workflow -run TestWorkspace -race` → ok
- `go test ./internal/cmd -run TestLogicalConflict -race` → ok
- `go test ./... -race -short` → ok all packages
- `go build ./... && go vet ./...` → ok
- `golangci-lint run` → 0 issues
