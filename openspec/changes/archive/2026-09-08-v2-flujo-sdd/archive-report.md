# Archive Report: v2-flujo-sdd (Development flow — D09)

**Change**: v2-flujo-sdd
**Archived**: 2026-09-08
**Archived to**: `openspec/changes/archive/2026-09-08-v2-flujo-sdd/`
**Mode**: hybrid (OpenSpec files + Engram)
**Spec synced to**: `openspec/specs/v2-flujo-sdd/spec.md` (new domain — full spec: Purpose, Constraints, Requirements, Non-Requirements)

## Goal

Deliver D09 criteria (`v2-flujo-sdd/F-01`–`F-03`, `U-01`, 4/4): rename `v2-flujo-gentle-ai` to `v2-flujo-sdd` in the contract; neutralize project tool references to the canonical **SDD flow** while preserving the sanctioned historical alias `previously v2-flujo-gentle-ai (D09)`; replace stale Node/npm/TypeScript development-flow runner guidance with the real pure-Go verification boundary; and add an executable, fail-closed criterion-to-test auditor. This is the last pending spec of the 11-spec v2 tracking table.

## Final State (authoritative at close — outranks any intermediate snapshot)

```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:61d71953588838490fa387e573023646d07644c40f699ac730600af44bd64baf
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 5/5
scenarios: 6/6
```

- **Tasks**: 20/20 complete (archived `tasks.md` all `[x]`, read in full in this phase). Task Completion Gate: PASS — the OpenSpec filesystem task artifact (the hybrid target of record for this project) has no stale unchecked implementation boxes.
- **Verification**: PASS WITH WARNINGS, 5/5 requirements, 6/6 scenarios, 0 blockers, 0 CRITICAL (Engram obs 2488; file `verify-report.md`, evidence revision `sha256:61d71953…4bd64baf`, verification evidence HEAD `36f4edc`). D09 track: F-01..F-03 + U-01 mapped 1:1 to the four named Go tests, plus the change's own TRACE-D09 requirement (5th requirement).
- **Gates green at close**: `go build ./...` exit 0 (empty output), `go vet ./...` exit 0, `go test ./... -race -count=1` exit 0 (14 packages passed, 2 packages reported no test files), `bash scripts/verify-traceability.sh` exit 0 (`TOTAL=92 STRICT=4 INFO=88`, default run executes all four D09 tests), `bash scripts/verify-store-boundary.sh` exit 0, `bash scripts/verify-distribution.sh` exit 0 (no v2-store/v2-distribucion regression), `golangci-lint run` exit 0 (0 issues). `govulncheck` is documented as a CI-only gate and was not locally requested; coverage analysis skipped per config (no threshold).
- **Non-blocking verify warnings (all consciously accepted, none block archive)**:
  1. Native `gentle-ai sdd-status` does not recognize the active flat `openspec/changes/v2-flujo-sdd/spec.md` (reports `specs: missing`). Irrelevant at close: the required spec was present and independently verified at the path supplied by the change; the change is now archived and the main spec is synced to `openspec/specs/v2-flujo-sdd/spec.md`.
  2. Engram observation `sdd/v2-flujo-sdd/tasks` (obs 2483) is a stale planning snapshot with unchecked boxes, while the filesystem `tasks.md` is authoritative and 20/20 checked. The hybrid backends are not fully synchronized for that upstream artifact; the filesystem-artifact task gate governs close and passes. Recorded, not blocking.
  3. `apply-progress.md` reports 10 U-01 leaf subtests; the current source contains 6 direct U-01 subtests. Documentation-accuracy drift only — root and fail-closed coverage still pass; not a coverage failure.
  4. `git diff --check` flags a blank line at EOF in the SDD `exploration.md` artifact. Referenced now in the archive trail; no product source file is affected (this write intentionally normalizes nothing in the archived artifact — the archive is byte-verbatim).
  5. `TestFlowDeliveryThroughGates` proves the documented review/delivery gate configuration and the archive receipt path, but no persisted PR review receipt was present in the active change artifacts. The sanctioned external review receipt for feature delivery is preserved out-of-band; a documentation-only archive push is the accepted receipt path here. Not blocking.
- **Delivery**: `single-pr` with maintainer pre-approved `size:exception` (review budget 200000, config review_budget_lines 20000). Delivered as direct pushes to `main` (no PR) per user preference; implementation commits are already in `main`, the orchestrator pushes this change's local archive commits after this phase.
- **Commits** (4 implementation already in `main` + this archive commit `docs(sdd)`, local only, no push):
  - `b3cd9f0` docs(contract): rename D09 to v2-flujo-sdd and correct stale runner guidance (deltas-acceptance.md, 13/13 swapped lines)
  - `254c375` docs(agents): drop tool prefix from artifact language rule (AGENTS.md:7)
  - `4e78511` feat(cmd): add D09 flow property tests with fail-closed traceability auditor (internal/cmd/flow_test.go, scripts/verify-traceability.sh)
  - `36f4edc` docs(sdd): mark v2-flujo-sdd tasks complete and record apply progress (change artifacts)
- **Strict TDD**: ACTIVE and followed — genuine RED→GREEN. Auditor + property tests written first and RED proven (script absent → exit 127 ×6; property tests fail against pre-edit docs baseline), then GREEN on the committed script/tests; final phase-5 gates all green. TDD Compliance 6/6 dimensions passed.

## Criteria Covered (criterion → test evidence)

| ID | Criterion | Spec requirement | Test evidence |
|----|-----------|------------------|---------------|
| F-01 | Each spec as an SDD change [PROC] P0 | Each spec follows the SDD lifecycle | `TestFlowEachSpecIsSDDChange` (5 archived-spec subtests): live name `v2-flujo-sdd`, alias line, 7-phase cycle in §Usage/F-01 criterion + tracking row, and required full archived artifact sets for the completed specs (flat and `specs/<name>/` layouts). PASS in `go test ./... -race -count=1`; strict mapping present. |
| F-02 | Verification via sdd-verify [PROC] P0 | Verification uses the real Go boundary | `TestFlowVerificationViaVerify`: race/local runners, E2E-vs-module wording, CI gate commands in §Usage/AGENTS.md/ci.yml; rejects `node --test`/`npm test`/`tsc --noEmit` on every flow surface. PASS; Go build/vet/lint pass. |
| F-03 | Delivery through the SDD flow [PROC] P0 | Delivery uses development gates | `TestFlowDeliveryThroughGates`: `.docs/initiatives/` + `tools/scripts/initiative/` absent; contract names `SDD flow gates (review receipts, delivery gates)`; AGENTS.md requires PR work units + green CI; ci.yml has pull-request review gate and push-to-main delivery gate; hybrid clause accepts docs-only archive pushes post-verify, feature code via reviewed PR. PASS. |
| U-01 | Criterion→test traceability [UNIT] P1 | Executable criterion-to-test audit | `TestFlowCriterionTraceability` (6 direct subtests, 4 roots + 2 fail-closed): real-root `--check-only` → exit 0 + `TOTAL=92 STRICT=4 INFO=88`; fixtures relative/absolute/non-git; duplicate ID → exit 1 naming `v2-distribucion/F-01`; removed mapping → exit 1 naming `v2-flujo-sdd/F-02` + `TestFlowVerificationViaVerify`. Default auditor execution proves all four tests pass. |
| TRACE-D09 | D09 traceability precedent | D09 traceability precedent | Exact 1:1 mapping F-01/F-02/F-03/U-01 → the four named tests, preserved in `spec.md`, `design.md`, `flow_test.go`, and auditor output. PASS. |

**Compliance summary**: 6/6 scenarios compliant (per verify-report).

## Files Changed (implementation, per apply-progress + verify-report + commit stats)

| File | Action | What Was Done |
|------|--------|---------------|
| `deltas-acceptance.md` | Modified | Renamed D09 to `v2-flujo-sdd` (tracking table row L45/L492: id, content `Development flow`, status stayed pending until archive); title L456 with sanctioned alias `(previously v2-flujo-gentle-ai (D09))`; §Usage `## Usage — SDD flow` (L49–54) neutralized with real Go gates; L5/L25 Pure-Go stack lines; L7/L19 neutral phrasing; F-02/F-03 criterion wording (`SDD flow`). Exactly 13 swapped lines (5,7,19,25,45,49,51,54,456,463,466,467,492). No other spec rows/checkboxes touched. |
| `AGENTS.md` | Modified | L7 only: `including gentle-ai/openspec/SDD artifacts` → `including openspec/SDD artifacts`. 0 `gentle-ai` hits remain. |
| `scripts/verify-traceability.sh` | Created (+x) | Two-level fail-closed 92-criterion auditor: `set -euo pipefail`, script-derived default root (optional fixture root), `git -C` tracked discovery with `find` fallback; parses `# Spec: v2-*` + `### (F|U)-[0-9]+`; rejects duplicates/malformed/count≠92/specs≠11; strict mode requires D09's 4 mappings + test functions; informational tier classifies 59 IDs (`v2-broker`/`v2-ipc` = `deferred:cli-direct`; `v2-no-regresion`/`v2-adapter`/`v2-path-claims` = `archived-pending`); unknown ID fails; 29 completed-spec IDs require archived evidence accepting BOTH `<archive>/<dir>/spec.md` flat and `<archive>/<dir>/specs/<name>/spec.md` layouts. Default runs the 4 tests; `--check-only` skips to avoid recursion. Prints each ID/status + `TOTAL=92 STRICT=4 INFO=88`; fail-closed exit 0/1. |
| `internal/cmd/flow_test.go` | Created | Package `cmd`: `TestFlowEachSpecIsSDDChange` (F-01), `TestFlowVerificationViaVerify` (F-02), `TestFlowDeliveryThroughGates` (F-03), `TestFlowCriterionTraceability` (U-01). Repository-relative helpers and temp fixtures; toolchain discovery fix for go1.27 variadic positional+spread calls. |
| `go.mod` / `go.sum` | **Unchanged** | No new dependencies. |

## Key Decisions

- **Rename `v2-flujo-gentle-ai` → `v2-flujo-sdd` with a single sanctioned historical alias**: the third-ever periodic review established that the flow is tool-neutral. The tracking ID/title became `v2-flujo-sdd`; the historical name is preserved exactly once as `previously v2-flujo-gentle-ai (D09)` in the D09 title (L456) so searches and evidence hashes remain intact without rewriting archived history. This is a sanctioned, explicit choice — the alias is NOT removed.
- **Canonical neutral term = "SDD flow"**: all live prose uses `SDD flow` uniformly; `gentle-ai` appears only in the sanctioned alias in `deltas-acceptance.md` (1 hit), with 0 hits in `AGENTS.md`, `docs/v2/`, `scripts/`, `.github/`, `openspec/specs/`.
- **Two-level fail-closed auditor (92/4/88)**: strictly require and run D09's four named Go tests (`STRICT=4`); enumerate the other 88 IDs informationally (`INFO=88`) under an explicit allowlist — CLI-direct deferrals (`v2-broker`, `v2-ipc`) and archived-pending rows (`v2-no-regresion`, `v2-adapter`, `v2-path-claims`). Unknown/duplicate/malformed IDs and missing mappings fail closed, naming the criterion and test. The audit tribal knowledge is now executable and CI-gateable.
- **Archived-spec layout tolerance**: the auditor resolves completed-spec evidence under BOTH the flat `<archive>/<dir>/spec.md` layout (composicion, distribucion, reporte, store) and the `<archive>/<dir>/specs/<name>/spec.md` layout (reconciliacion, spike-go, no-regresion, adapter, path-claims), with `verify-report.md` at change-dir level — a MAJOR validator fix needed to pass strict U-01.
- **Runner = real Go boundary**: `sdd-verify` or equivalent runs `[E2E]` at the public boundary and `[UNIT]`/`[INT]` as module tests; the documented runner is `go test ./...` (locally) and `go test ./... -race` (CI), with gates `go build ./...`, `go vet ./...`, golangci-lint, govulncheck. Stale `node --test`/`npm`/`tsc --noEmit` guidance removed from the flow surface.
- **Cleanup strictly scoped to project components with immutability**: only `deltas-acceptance.md`, `AGENTS.md:7`, the new auditor script, and the new test file changed. `openspec/changes/archive/` (31+ historical verbatim `gentle-ai` matches and their `evidence_revision` hashes) and `docs/v2/` are immutable and untouched.

## Limits Respected

- `deltas-acceptance.md` D09 literal criterion text unchanged except for tool references and stale runner wording (rename + neutralization); no other spec's rows/checkboxes touched; alias preserved.
- `docs/v2/` and `openspec/config.yaml` untouched.
- `go.mod`/`go.sum` unchanged; no new dependencies; no product behavior change (verify-not-redefine).
- `openspec/changes/archive/` immutable — `git diff --quiet b3cd9f0^..HEAD -- openspec/changes/archive` passes; 31 historical `gentle-ai` matches remain verbatim.
- `.atl/skill-registry.md` is out-of-scope generated tooling and retains its generated `gentle-ai` prefix (not under any required zero-hit path).
- No push performed; this phase makes local commits only.

## Sole Documented Amendment

- `deltas-acceptance.md` — D09 spec (`v2-flujo-sdd`, lines 456–473): all 4 criterion checkboxes (F-01..F-03, U-01) marked verified `[x]`; tracking row updated from `pending` to **complete** (4 criteria, 3 P0; verdict pass_with_warnings); "Last updated" summary line updated to 2026-09-08. No other spec rows/checkboxes touched.

## Artifact Traceability (Engram observation IDs)

| Artifact | Engram obs ID | File in archive |
|----------|---------------|-----------------|
| explore | 2478 | `openspec/changes/archive/2026-09-08-v2-flujo-sdd/exploration.md` |
| proposal | 2479 | `openspec/changes/archive/2026-09-08-v2-flujo-sdd/proposal.md` |
| spec | 2480 | `openspec/changes/archive/2026-09-08-v2-flujo-sdd/spec.md` |
| design | 2482 | `openspec/changes/archive/2026-09-08-v2-flujo-sdd/design.md` |
| tasks | 2483 (stale planning snapshot; filesystem tasks.md authoritative) | `openspec/changes/archive/2026-09-08-v2-flujo-sdd/tasks.md` |
| apply-progress | 2484 | `openspec/changes/archive/2026-09-08-v2-flujo-sdd/apply-progress.md` |
| verify-report | 2488 | `openspec/changes/archive/2026-09-08-v2-flujo-sdd/verify-report.md` |
| archive-report | (this save) | `openspec/changes/archive/2026-09-08-v2-flujo-sdd/archive-report.md` |

## Review Gate

`reviewGate` structurally ABSENT — no review artifact was discovered for this candidate (the verify-report WARNING 5 is a no-persisted-PR-receipt note, not a gate value; native structured status carried no `reviewGate` key, and D09 F-03's "review receipts, delivery gates" contract is satisfied by the documented gate configuration asserted by `TestFlowDeliveryThroughGates`). Receipt-driven development did not run for this candidate. Archive proceeds under ordinary repository policy. Nothing to investigate; absence of `reviewGate` is not a defect.

## Archive Contents

- `proposal.md` ✅
- `exploration.md` ✅ (verbatim; the known trailing-blank EOF diff-check warning is preserved as-is — no post-hoc rewrite of archived content)
- `spec.md` ✅ (delta spec, verbatim)
- `design.md` ✅
- `tasks.md` ✅ (20/20 tasks complete, no stale unchecked boxes)
- `apply-progress.md` ✅
- `verify-report.md` ✅ (envelope YAML header intact — staged then moved via `git mv`, content never re-written)
- `archive-report.md` ✅ (this file, additive)

## Source of Truth Updated

- `openspec/specs/v2-flujo-sdd/spec.md` — new domain spec created. The change's delta spec is ADDED-only (5 requirements: F-01, F-02, F-03, U-01, TRACE-D09; no MODIFIED/REMOVED/RENAMED), so the main spec was composed as a full spec (Purpose, Constraints, Requirements, Non-Requirements) per the archive convention, modeled on `openspec/specs/v2-distribucion/spec.md` and the rest of the series. Generation was mechanical: the delta was transformed by exactly two header edits (`# Delta for v2-flujo-sdd` → `# v2-flujo-sdd Specification`; `## ADDED Requirements` → `## Requirements`); every body section (Purpose, Constraints, all 5 requirement blocks with scenarios, Non-Requirements) is byte-identical to the delta, verified by byte comparison (empty diff) against the re-computed expected bytes. The transformation method was validated by re-deriving the committed `v2-distribucion` main spec from its archived delta (empty diff). The `rules.archive` "warn before merging destructive deltas" did not trigger: no `REMOVED`/`RENAMED` sections exist and the sync is purely additive.
- `deltas-acceptance.md` — D09 criteria checkboxes (4), tracking row (`pending` → **complete**), and summary line updated (see Sole Documented Amendment).

## Mechanical Verification

- Folder move: staged the initially-untracked `verify-report.md`, then `git mv openspec/changes/v2-flujo-sdd openspec/changes/archive/2026-09-08-v2-flujo-sdd`. Pre-move recursive snapshot vs archived tree, `diff -r` → **empty (exit 0)**: byte-identical, no truncation or alteration. `archive-report.md` excluded because it did not exist in the source change folder (additive-only). Source directory confirmed gone; active changes directory contains only `archive/`.
- Spec sync: mechanical two-header transformation (byte-faithful, body unchanged), verified by byte comparison against the re-computed expected bytes → empty diff. Atomic `mv` into `openspec/specs/v2-flujo-sdd/spec.md`, permissions aligned to series (664).

## Next

- **All 92-criterion series status at close**: `v2-store`, `v2-distribucion`, `v2-flujo-sdd` are now **complete** (plus earlier `v2-reconciliacion`, `v2-composicion`, `v2-reporte`); `v2-broker`/`v2-ipc` remain **pending** (deferred per the CLI-direct architecture), and `v2-no-regresion`/`v2-adapter`/`v2-path-claims` remain **pending** in the tracking table with their archive cycles documented. This change closes the last originally-requested development-flow spec; no spec in the table is left unaccounted — every row is either complete or a recorded pending/deferred/pending-differed status.
- **Tracked technical debt**: validate the real release path end-to-end with the first `v*` tag (workflow exact-tag install, downloaded-asset checksum/static/link/offline-init proof, post-propagation `@latest` smoke) — carried from `v2-distribucion`, still outstanding at this archive.
- **Pending normative docs note (deferred, not applied here)**: `docs/v2/haro-especificacion-tecnica.md` §7 sole-owner table could note that `leases`/`interactions` are now owned by `v2-store` and `path_claims`/`attempt_transport`/`dag_hash`/`base_commit` by their owners — a normative docs update deferred per "do not edit docs without an SDD change"; still pending at this archive.
- **Optional contract decision for another session**: review why `v2-no-regresion`, `v2-adapter`, and `v2-path-claims` are archived but still marked `pending` in the tracking table — a deliberate contract/status decision for the orchestrator, not applied here.
- Delivery: the orchestrator performs the direct `main` push of the accumulated local commits (including this archive commit) after this phase, per the pre-approved `size:exception` and the user's no-PR delivery preference.
