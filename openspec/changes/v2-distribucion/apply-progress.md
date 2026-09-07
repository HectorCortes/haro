# Apply Progress: v2-distribucion (D08)

**Mode**: Strict TDD (go test ./...)
**Delivery**: single-pr, size:exception pre-approved (200000), direct push to main, no PRs.
**Tasks**: 13/13 complete (1.1 → 5.1).

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | `internal/cmd/distribution_e2e_test.go` | E2E (real binary) | ✅ `go test ./internal/cmd -count=1` ok | ➖ Approval test — behavior pre-exists (`project.Init` unchanged, verify-not-redefine) | ✅ Passed (2/2 tests, 16.5s) | ✅ 2 scenarios (clean + idempotent) | ➖ None needed |
| 1.2 | `internal/cmd/distribution_e2e_test.go` | E2E | ✅ baseline ok | ➖ Approval test (see 1.1) | ✅ Passed — exit 0, all 5 paths, `GOPROXY=off` | covered by 1.1 triangulation | ➖ None needed |
| 1.3 | `internal/cmd/distribution_e2e_test.go` | E2E | ✅ baseline ok | ➖ Approval test (see 1.1) | ✅ Passed — custom config bytes + docs/user.md preserved on rerun | covered by 1.1 triangulation | ➖ None needed |
| 2.1 | `internal/cmd/distribution_e2e_test.go` (gate fixtures) | Integration (bash gate) | ✅ baseline ok | ✅ Written — clean fixture expected exit 0, script absent → exit 127, FAIL confirmed | ✅ Passed (7 fixture tests / 8 cases) | ✅ 5 rejected names + 5 allow-list files | ➖ None needed |
| 2.2 | `scripts/verify-distribution.sh` | Script | ✅ fixtures RED before | ➖ (was the GREEN of 2.1) | ✅ Passed — fixtures green; repo-root run exits 0 | via 2.1 fixtures | ➖ None needed |
| 2.3 | `.github/workflows/ci.yml` | CI config | ➖ N/A (structural) | ➖ Structural — no unit-testable logic | ✅ YAML parse OK; gate step added to `build-and-test`, triggers/`contents: read` retained | ➖ Triangulation skipped: purely structural config edit | ➖ None needed |
| 3.1 | `.github/workflows/release.yml` | CI config | ➖ N/A (new file, not triggered — no tag exists) | ➖ Structural | ✅ YAML parse OK; `on.push.tags: ['v*']`, workflow `contents: read`, only checkout@v7/setup-go@v7 | ➖ Triangulation skipped: purely structural | ➖ None needed |
| 3.2 | `.github/workflows/release.yml` | CI config | ➖ N/A (structural) | ➖ Structural | ✅ verify job: gate + `go test ./... -race -count=1` + clean-`GOBIN` `GOPROXY=direct` exact-tag install executed from empty dir | ➖ Triangulation skipped: structural | ➖ None needed |
| 3.3 | `.github/workflows/release.yml` | CI config | ➖ N/A (structural) | ➖ Structural | ✅ release job `needs: verify`, sole `contents: write`; build/checksum/`gh release create --verify-tag --generate-notes` + `upload --clobber` | ➖ Triangulation skipped: structural | ➖ None needed |
| 4.1 | `README.md` | Docs | ➖ N/A | ➖ Structural | ✅ stale "go.mod will be created" / "no publishable binary" removed; Installation added | ➖ Triangulation skipped: docs | ➖ None needed |
| 4.2 | `README.md` | Docs | ➖ N/A | ➖ Structural | ✅ networked-install CI policy documented (Release verification notes) | ➖ Triangulation skipped: docs | ➖ None needed |
| 4.3 | `deltas-acceptance.md` | Guard | ➖ N/A | ➖ Structural | ✅ F-01 literal text untouched (`git diff` on file: none) | ➖ Guard verified | ➖ None needed |
| 5.1 | Full gates | Gates | — | — | ✅ all green (below) | — | — |

## Work Unit Evidence

### U1 — Offline init E2E
- **Focused test**: `go test ./internal/cmd -run TestDistribution -count=1` → `ok ... 1.283s` (2 E2E tests PASS)
- **Runtime harness**: real binary built with `CGO_ENABLED=0 go build -o <tmp>/haro .`, run twice in `t.TempDir()` with `GOPROXY=off`; exit 0; `.haro/config.yaml` + `workflows/ skills/ artifacts/ docs/`; custom config bytes and `docs/user.md` preserved. Release-flag binary also sanity-run offline in a temp dir: `{"status":"initialized"}`.
- **Rollback boundary**: delete `internal/cmd/distribution_e2e_test.go` (commit 2a534bc). No production files touched.

### U2 — F-02 gate
- **Focused test**: `go test ./internal/cmd -run TestDistributionGate -count=1` → PASS (8 cases: clean, 2 package artifacts, 3 hooks, allow-list ×5 files)
- **Runtime harness**: `bash scripts/verify-distribution.sh` at repo root → exit 0; RED pre-GREEN proven (exit 127, script absent). Repo-root gate passing after GREEN.
- **Rollback boundary**: delete `scripts/verify-distribution.sh`, revert gate fixtures in `distribution_e2e_test.go`, revert ci.yml step (commit 18a962f).

### U3 — Release + README
- **Focused test**: `go build ./... && go vet ./...` → both clean
- **Runtime harness**: `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tmp/opencode/haro-dist .` → `file`: "statically linked, Go BuildID…, stripped"; `haro init` offline succeeds. Workflow cannot be fired locally (no tag) — validated by YAML parse + design review only.
- **Rollback boundary**: delete `.github/workflows/release.yml`, revert `README.md` (commits e6c2a4e, 0ee58f4).

## Files Changed

| File | Action | Commit |
|---|---|---|
| `internal/cmd/distribution_e2e_test.go` | Created | 2a534bc, 18a962f |
| `scripts/verify-distribution.sh` | Created (+x) | 18a962f |
| `.github/workflows/ci.yml` | Modified (gate step) | 18a962f |
| `.github/workflows/release.yml` | Created | e6c2a4e |
| `README.md` | Modified (installation + status) | 0ee58f4 |
| `openspec/changes/v2-distribucion/tasks.md` | Checkboxes [x] | (this commit) |

## Final Gates (task 5.1)

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./... -race -count=1` ✅ (all 12 packages ok)
- `golangci-lint run` ✅ (0 issues; installed locally)
- `bash scripts/verify-distribution.sh` ✅ (no regression risk; repo clean)
- `bash scripts/verify-store-boundary.sh` ✅ (no v2-store regression)
- `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"` → statically linked, stripped, offline init works
- `git status --porcelain` → only intended files (pre-existing untracked: `.atl/`, `.engram/`, openspec change dir committed with this progress, and a pre-existing untracked `testdata/compose/symlink-escape/.../link.yaml` leftover from before this change — untouched)
- No new dependencies (`go.mod` untouched); `internal/project/init.go` untouched; `deltas-acceptance.md` untouched.

## Deviations from Design

None — implementation matches design.md (gate exact-name classification with find fallback, release.yml job split and permissions, README install paths, no networked install in ordinary CI).

## Issues Found

- Pre-existing untracked file `testdata/compose/symlink-escape/.haro/workflows/lib/link.yaml` (leftover from before this change); left untouched and uncommitted.

## Status

13/13 tasks complete. Ready for sdd-verify.
