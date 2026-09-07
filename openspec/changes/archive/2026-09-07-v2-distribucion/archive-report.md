# Archive Report: v2-distribucion (Distribution — D08)

**Change**: v2-distribucion
**Archived**: 2026-09-07
**Archived to**: `openspec/changes/archive/2026-09-07-v2-distribucion/`
**Mode**: hybrid (OpenSpec files + Engram)
**Spec synced to**: `openspec/specs/v2-distribucion/spec.md` (new domain — full spec: Purpose, Constraints, Requirements, Non-Requirements)

## Goal

Deliver D08 criteria (`v2-distribucion/F-01`–`F-03`): Go-native Haro distribution with a reproducible `go install` mechanism and one CI-published static `haro` release asset, a permanent fail-closed gate against executable post-install hooks, and dependency-free offline `haro init` in a clean project. The historical npm-v1 contract text (`pnpm add -g shardeo`, `prepublishOnly`, `dist/index.js`) MUST remain byte-for-byte unchanged; the v2 Go interpretation is recorded as a normative, constitutionally grounded reinterpretation, not a contract rewrite.

## Final State (authoritative at close — outranks any intermediate snapshot)

```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:04893677e6336de1fc6fe77019664bbb83e36359dcb0f125a3e7bf4ed776e75b
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 4/4
scenarios: 7/7
```

- **Tasks**: 13/13 complete (`tasks.md` all `[x]`, read in full in this phase; Engram obs 2466). Task Completion Gate: PASS, no stale unchecked implementation boxes.
- **Verification**: PASS WITH WARNINGS, 4/4 requirements, 7/7 scenarios (5 runtime-complete, 2 partial external/audit gates), 0 blockers, 0 CRITICAL (Engram obs 2468; file `verify-report.md`, evidence revision `sha256:04893677…e75b`, verification date 2026-09-07, evidence commit `0767c6b`). D08 criteria trace: 3/3 (F-01..F-03) with covering tests; plus the change's own TRACE-D08 requirement (4th requirement).
- **Gates green at close**: `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./... -race -count=1` exit 0 (16 package entries: 14 passed, 2 `[no test files]` — the apply artifact's older "12 packages" wording is an under-count, not a failing gate), `golangci-lint run` 0 issues, `bash scripts/verify-distribution.sh` exit 0, `bash scripts/verify-store-boundary.sh` exit 0 (no v2-store regression), workflow YAML parse of both `ci.yml` and `release.yml` exit 0, structural workflow policy audit exit 0. CRITICAL: 0, blockers: 0.
- **Honest release warning carried to archive**: the `@latest` smoke test resolved the currently published pseudo-version `v0.0.0-20260907164447-5cb547699fc1`, not this untagged HEAD (`0767c6b`, six commits ahead of `origin/main`). No `v*` tag or GitHub Release exists yet, so the exact-tag install and the downloaded release-asset check under `release.yml` are structurally verified but not end-to-end executed. The remaining manual post-tag proof (first `v*` tag → workflow runs exact-tag `GOPROXY=direct` clean-`GOBIN` install → download `haro`/`haro.sha256` → checksum verify → static-link `file` check → offline `./haro init` → post-propagation `@latest` smoke) is recorded as a tracked follow-up, not a blocker. Per `verify-report` WARNING 1 and SUGGESTION 1: after tagging, record the release-asset checksum and the post-proxy `@latest` result in the archived change evidence.
- **Delivery**: `single-pr` with maintainer pre-approved `size:exception` (review budget 200000, config review_budget_lines 20000). Delivered as direct pushes to `main` (no PR) per user preference, branchless; implementation commits are already in `main`, the orchestrator pushes after this phase.
- **Commits** (6 implementation + this `docs(sdd)` archive commit, all local except the already-pushed implementation commits):
  - `2a534bc` test(cmd): prove offline idempotent init with real binary (D08 F-03)
  - `18a962f` chore(scripts): add F-02 hook-free distribution gate wired into CI
  - `e6c2a4e` ci(release): publish static haro binary on v* tags with clean install proof
  - `0ee58f4` docs(readme): document installation, release assets and CI verification policy
  - `9c52ed7` chore(sdd): mark v2-distribucion tasks complete and record apply progress
  - `0767c6b` docs(sdd): track v2-distribucion change artifacts
- **Strict TDD**: ACTIVE and followed — genuine RED→GREEN for the F-02 gate (task 2.1 RED confirmed: clean fixture expected exit 0, script absent → exit 127); F-03 and structural config/docs tasks explicitly recorded as approval/structural (N/A RED) because production `init.go` and CI behavior pre-existed and were verified-not-redefined. TDD Compliance 6/6 dimensions passed.

## Criteria Covered (criterion → test evidence)

| ID | Criterion | Spec requirement | Test evidence |
|----|-----------|------------------|---------------|
| F-01 | End-to-end installation [E2E] P0 | Go-native end-to-end installation | `module-install-latest` smoke: clean `GOBIN`, networked `go install github.com/HectorCortes/haro@latest`, executable invoked from empty dir (exit 0) + local `CGO_ENABLED=0` static build and offline execution + `.github/workflows/release.yml` structurally verified (exact-tag `GOPROXY=direct` install, static build, checksum, `gh release create --verify-tag`/upload, `contents: write` only on the dependent release job). ⚠️ Honest gap: no `v*` tag exists yet, so the published-asset path is not end-to-end validated (manual post-tag check). |
| F-02 | No post-install scripts [E2E] P1 | No executable installation hooks | `TestDistributionGateCleanFixture` + `TestDistributionGateRejectsPackageArtifacts` (package.json, npm-shrinkwrap.json) + `TestDistributionGateRejectsInstallHooks` (preinstall.sh, install.sh, postinstall.sh) — 11 fixture cases pass; `bash scripts/verify-distribution.sh` at repo root exit 0; allow-list (requirements.txt, CMakeLists.txt, executable *.md/MDX, README.sh) passes; wired as fail-closed step in `ci.yml` `build-and-test` (`contents: read` retained). |
| F-03 | init without additional dependencies [E2E] P1 | Dependency-free offline initialization | `TestDistributionOfflineInitE2E` (real `CGO_ENABLED=0` binary, empty temp project, `GOPROXY=off`, exit 0, `.haro/{config.yaml,workflows,skills,artifacts,docs}` present) + `TestDistributionOfflineInitIdempotentE2E` (second offline run preserves custom config bytes and `docs/user.md`). Production `internal/project/init.go` unchanged (verify-not-redefine). |
| TRACE-D08 | Constitutional interpretation and traceability | Constitutional interpretation and traceability | Spec documents `go install` as the Constitution §III.1 equivalent of F-01's historical `pnpm add -g shardeo`; `deltas-acceptance.md` F-01 text unchanged (byte-for-byte); exact 1:1 mapping F-01/F-02/F-03 recorded here and in `openspec/specs/v2-distribucion/spec.md`. |

**Compliance summary**: 7/7 scenarios evidenced (5 runtime-complete, 2 partial external/audit gates: tagged-release publication and the archive-time traceability record — the latter is closed by this archive). No scenario failed.

## Files Changed (implementation, per apply-progress + verify-report)

| File | Action | What Was Done |
|---|---|---|
| `internal/cmd/distribution_e2e_test.go` | Created | Real-binary offline/idempotent init E2E (F-03) + adversarial F-02 gate fixtures (clean, 2 package artifacts, 3 hooks, 5 allow-list files); 6 top-level tests, 11 fixture cases |
| `scripts/verify-distribution.sh` | Created (+x) | Fail-closed exact-name gate: rejects `package.json`, `npm-shrinkwrap.json`, `preinstall.sh`, `install.sh`, `postinstall.sh`; `set -euo pipefail`, optional root defaulting to repo root, `git grep` + non-git fallback; style of `verify-store-boundary.sh` |
| `.github/workflows/ci.yml` | Modified | `bash scripts/verify-distribution.sh` added to `build-and-test`; triggers and `contents: read` retained (obsolete "go.mod does not exist" comment left in place — no runtime effect) |
| `.github/workflows/release.yml` | Created | `on.push.tags: ['v*']`; workflow `contents: read`; `verify` job: gate + `go test ./... -race -count=1` + clean-`GOBIN` `GOPROXY=direct go install github.com/HectorCortes/haro@$GITHUB_REF_NAME` executed from empty dir; `release` job (`needs: verify`, sole `contents: write`): `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/haro .`, `sha256sum`, `gh release create --verify-tag --generate-notes` + `gh release upload --clobber` |
| `README.md` | Modified | Stale "go.mod will be created in the spike phase" / "no publishable binary" text removed; Installation rewritten: `go install github.com/HectorCortes/haro@latest` and release asset `chmod +x haro && ./haro init`; release verification policy documented (networked install not run in ordinary CI, exact-tag `GOPROXY=direct` in release workflow, post-release `@latest` manual check) |
| `openspec/changes/v2-distribucion/tasks.md` | Checkboxes [x] | 13/13 tasks marked complete (commit 9c52ed7) |
| `internal/project/init.go` | **Unchanged** | Verified identical to pre-change revision `5cb5476` (verify-not-redefine; F-03 approval tests) |
| `go.mod` / `go.sum` | **Unchanged** | No new dependencies |

## Key Decisions

- **`go install` = constitutional reinterpretation of F-01 (Constitution §III.1)**: F-01's literal npm wording ("global npm package `pnpm add -g shardeo`", `prepublishOnly`, `dist/index.js`) is a v1-era distribution model. Constitution §III.1 states "The distribution model is not tied to any particular ecosystem; it may change with respect to v1." The spec therefore SHALL reinterpret F-01 as `go install github.com/HectorCortes/haro@latest` plus a CI-published static binary release, with `haro` as the v2 command. The contract text in `deltas-acceptance.md` is NEVER edited — only checkboxes/tracking row/date note — and the interpretation lives in the spec and this archive trail (TRACE-D08). This is a normative, traceable reinterpretation per the constitution, not a silent contract rewrite.
- **Separate tag-only release workflow with isolated permissions**: ordinary `ci.yml` stays `contents: read`; `release.yml` triggers only on `v*` and grants `contents: write` solely to the dependent `release` job (`needs: verify`). Exact-tag `GOPROXY=direct` network proof runs only in the release workflow, isolating proxy/tag timing fragility from PR CI; post-release `@latest` remains a documented manual check.
- **F-02 fail-closed exact-name gate**: rejects exactly `package.json`, `npm-shrinkwrap.json`, `preinstall.sh`, `install.sh`, `postinstall.sh` (tracked via `git grep` and non-git fallback), with adversarial fixtures proving rejection and an allow-list (requirements.txt, CMakeLists.txt, executable Markdown/MDX, README.sh) proving no false positives. The absence of hooks is a permanent regression gate, not permission to introduce them.
- **F-03 verify-not-redefine**: `internal/project/init.go` already performs only local filesystem operations (`Init(string) error`); the change proves offline real-binary init with `GOPROXY=off` without touching production code — approval tests documented as such in the TDD evidence.
- **Plain Go + `gh` release mechanics**: no GoReleaser/Homebrew/npm wrapper; a single stripped static binary artifact plus SHA-256 checksum explicitly scoped to the root `main.go` entry point. Single runner-platform binary is explicit v2 scope; multi-arch is deferred.

## Limits Respected

- `deltas-acceptance.md` D08 literal criteria text byte-for-byte unchanged (F-01 still says `pnpm add -g shardeo`); only the 3 checkboxes, tracking row (`pending` → **complete**), and "Last updated" note changed by this archive. No rows/checkboxes of other specs touched.
- No GoReleaser, Homebrew, npm wrappers, `shardeo doctor`/`setup`, or CLI registration in `config.yaml` (explicit Non-Requirements).
- `internal/project/init.go` untouched; `go.mod`/`go.sum` unchanged; no new dependencies.
- No broker/IPC/PTY, no multi-arch, no network infrastructure in ordinary CI; `ci.yml` permissions not escalated.
- No edits to `docs/v2/` or `openspec/config.yaml`. Constitution §III.1 (line 27 of `docs/v2/haro-constitucion.md`) was read and quoted only.

## Deferrals and Exclusions (intentional, recorded)

- `v2-broker`/`v2-ipc` remain deferred per the CLI-direct architecture.
- `v2-no-regresion`, `v2-adapter`, `v2-path-claims` rows remain `pending` as recorded; their tracking-row updates were begun later in the series and their checkboxes were left untouched by previous archives — this change touches only the `v2-distribucion` row and checkboxes.
- `v2-flujo-gentle-ai` remains `pending` — it is the only remaining uncompleted spec in the tracking table whose predecessor deferrals do not apply (see Next).
- Networked publish proof for this untagged HEAD remains external by design (tracked follow-up with the first `v*` tag).

## Sole Documented Amendment

- `deltas-acceptance.md` — D08 spec (`v2-distribucion`, lines 440–452): all 3 criterion checkboxes (F-01..F-03) marked verified `[x]`; tracking row updated from `pending` to **complete** (3 criteria, 1 P0; verdict pass_with_warnings); "Last updated" summary line updated to 2026-09-07. No implementation-time doc amendments: F-01 literal text untouched, `go.mod` unchanged, no npm artifacts, `docs/v2/` untouched.

## Artifact Traceability (Engram observation IDs)

| Artifact | Engram obs ID | File in archive |
|----------|---------------|-----------------|
| explore | 2460 | `openspec/changes/archive/2026-09-07-v2-distribucion/exploration.md` |
| proposal | 2461 | `openspec/changes/archive/2026-09-07-v2-distribucion/proposal.md` |
| spec | 2462 | `openspec/changes/archive/2026-09-07-v2-distribucion/spec.md` |
| design | 2464 | `openspec/changes/archive/2026-09-07-v2-distribucion/design.md` |
| tasks | 2466 | `openspec/changes/archive/2026-09-07-v2-distribucion/tasks.md` |
| apply-progress | 2467 | `openspec/changes/archive/2026-09-07-v2-distribucion/apply-progress.md` |
| verify-report | 2468 | `openspec/changes/archive/2026-09-07-v2-distribucion/verify-report.md` |
| archive-report | (this save) | `openspec/changes/archive/2026-09-07-v2-distribucion/archive-report.md` |

## Review Gate

`reviewGate` structurally ABSENT — no review artifact was discovered for this candidate (native status: `reviewPolicy`, `reviewLedger`, `reviewReceipt`, `reviewBundle`, `reviewContext`, `reviewState` all missing; `reviewOffer` absent; `blockedReasons` empty). Receipt-driven development did not run for this candidate. Archive proceeds under ordinary repository policy. Nothing to investigate.

## Archive Contents

- `proposal.md` ✅
- `exploration.md` ✅
- `spec.md` ✅ (delta spec, verbatim)
- `design.md` ✅
- `tasks.md` ✅ (13/13 tasks complete, no stale unchecked boxes)
- `apply-progress.md` ✅
- `verify-report.md` ✅ (envelope YAML header intact — staged then moved via `git mv`, content never re-written)
- `archive-report.md` ✅ (this file, additive)

## Source of Truth Updated

- `openspec/specs/v2-distribucion/spec.md` — new domain spec created. The change's delta spec is ADDED-only (4 requirements, no MODIFIED/REMOVED/RENAMED), so the main spec was composed as a full spec (Purpose, Constraints, Requirements, Non-Requirements) per the archive convention, modeled on `openspec/specs/v2-store/spec.md` and `openspec/specs/v2-reporte/spec.md`. Generation was mechanical: the delta was transformed by exactly two header edits (`# Delta for v2-distribucion` → `# v2-distribucion Specification`; `## ADDED Requirements` → `## Requirements`); every body section (Purpose, Constraints, all 4 requirement blocks with scenarios, Non-Requirements) is byte-identical to the delta, verified by `diff` (empty) against the transformed expectation. The `rules.archive` "warn before merging destructive deltas" did not trigger: no `REMOVED`/`RENAMED` sections exist and the sync is purely additive.
- `deltas-acceptance.md` — D08 criteria checkboxes, tracking row, and summary line updated (see Sole Documented Amendment).

## Mechanical Verification

- Folder move: staged the initially-untracked `verify-report.md`, then `git mv openspec/changes/v2-distribucion openspec/changes/archive/2026-09-07-v2-distribucion`. Pre-move recursive snapshot vs archived tree, `diff -r` → **empty (exit 0)**: byte-identical, no truncation or alteration. `archive-report.md` excluded because it did not exist in the source change folder (additive-only).
- Source directory confirmed gone after the move; active changes directory contains only `archive/`.
- Spec sync: mechanical two-header transformation, verifier recomputed the expected transformed bytes and `diff -r` → **empty (exit 0)**; verbatim diff output in phase result.
- `deltas-acceptance.md`: exactly 3 checkboxes marked `[x]`, tracking row `complete`, "Last updated" updated; `git diff` confirmed no other spec rows/checkboxes touched and F-01's literal npm text untouched.

## Next

- Post-archive: the only remaining uncompleted spec in the tracking table is **`v2-flujo-gentle-ai`** (development flow with gentle-ai; 4 criteria, 3 P0, pending). Recommended next SDD change after delivery. `v2-broker`/`v2-ipc` remain deferred per the CLI-direct architecture; `v2-no-regresion`, `v2-adapter`, and `v2-path-claims` rows remain `pending` as recorded, with their archive cycles having left `deltas-acceptance.md` untouched.
- **Tracked technical debt**: validate the real release path end-to-end with the first `v*` tag (workflow exact-tag install, downloaded-asset checksum/static/link/offline-init proof, post-propagation `@latest` smoke), then record the release-asset checksum and result in this archived change's evidence (per verify-report SUGGESTION 1).
- **Pending normative docs note (deferred, not applied here)**: `docs/v2/haro-especificacion-tecnica.md` §7 sole-owner table could be extended to note that `leases` and `interactions` are now owned by `v2-store` — a normative docs update deferred out of the v2-store change per the "do not edit docs without an SDD change" rule; still pending at this archive.
- Delivery: the orchestrator performs the direct `main` push of the accumulated local commits after this archive phase (per the pre-approved `size:exception` and the user's no-PR delivery preference).