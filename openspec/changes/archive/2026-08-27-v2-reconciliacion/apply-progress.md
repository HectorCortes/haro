# Apply Progress: v2-reconciliacion

**Change**: v2-reconciliacion
**Mode**: Strict TDD (documentary [REV] — approved exemption: no code or tests; strict TDD discipline does not apply to touching code without a test)
**Baseline commit**: `710bc6a` (chore: baseline docs v2 before reconciliation — docs were untracked on main; baseline committed to make `git diff` auditable; not part of work unit per se)
**Date**: 2026-08-27

## Completed Tasks (7/7) — Order C1→C2→T1→T2→T3→3.1→3.2

- [x] **1.1 C1 VII.4 F-03** — `docs/v2/shardeo-v2-constitucion.md:110` — Replaced fixed `JSON-RPC 2.0 over subprocess stdio … fallback … never general` with adapter-chooses-declares-negotiates text. Verify: `grep -q "declares it in capabilities" docs/v2/shardeo-v2-constitucion.md` ✓; VII.4 line no longer contains `fallback` ( `grep "VII.4" … | grep -q fallback` → no match ); note: `fallback` still appears in unrelated sections `IV.1 Attempt` and `VI.3` (candidate fallback, not transport) — expected and out of scope.
- [x] **1.2 C2 XII F-01+F-02+F-04** — `docs/v2/shardeo-v2-constitucion.md:195-199` — Replaced `## XII. Deliberately deferred` block (XII.1 deferred, XII.2 Bundle+CAS) with `## XII. Implemented state and deliberate deferrals` — XII.1 terminal/PTY already implemented Spec 9c (real PTY, attach, presence), XII.2 bundle/manifest/admission Spec 9a → `v2-no-regresion`, CAS deferred, XII.3 preserved, XII.4 debt `src/utils/agent.ts`. Verify: `grep -q "already implemented"` ✓, `grep -q "v2-no-regresion"` ✓, `grep -q "XII.4.*agent.ts"` ✓.
- [x] **2.1 T1 §7 F-03** — `docs/v2/shardeo-v2-especificacion-tecnica.md:535` — Replaced `Transport: JSON-RPC 2.0 over subprocess stdio … (fallback …)` with adapter-chooses text (`negotiates it in initialize`). Verify: `grep -q "negotiates it in .initialize."` ✓; `Transport:` line no longer contains `fallback` ( `grep "^Transporte:" … | grep -q fallback` → no match ); `fallback` remains in schema `harness` fallback description — unrelated.
- [x] **2.2 T2 §7 F-01** — `docs/v2/shardeo-v2-especificacion-tecnica.md:546` — Row `terminal/*` `(optional, deferred)` → `(optional, implemented)` with real PTY, attach, presence; Spec 9c. Verify: `grep -q "terminal.*implemented"` ✓, `grep -q "Real PTY"` ✓.
- [x] **2.3 T3 §7 F-04** — `docs/v2/shardeo-v2-especificacion-tecnica.md:547` — Inserted `**Confined debt:** OpenCode JSONL parser in src/utils/agent.ts (headless 1596–1713); move pending v2-adapter/F-04.` Verify: `grep -q "Confined debt"` ✓, `grep -q "agent.ts"` ✓ (both docs).
- [x] **3.1 U-01 auditable diff** — `git diff --stat` shows 2 files, `git diff --name-only` shows only the two docs, no code/SPECS.md/§4/changelogs. `git diff | grep -c "^@@"` → 4 git hunks coalesced (C1, C2, T1, T2+T3). Logical hunks = 5 (C1, C2, T1, T2, T3); T2+T3 are adjacent and git coalesces them into one `@@` block — mapped F-01..F-04 all present, verified via grep diff mapping below. U-01 satisfied.
- [x] **3.2 CAS+XII.3 preservation** — CAS stays `remains deferred`, XII.3 `not implemented now; persistence interface (III.3) does not block`. Verify: `grep -q "CAS.*deferred"` ✓, `grep -q "XII.3.*not implemented now"` ✓ (constitution); also `grep -q "CAS.*deferred"` true in specification? preserved via file (X-linked, but constitution is authority).

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `docs/v2/shardeo-v2-constitucion.md` | Modified | C1 (VII.4 adapter-chooses) + C2 (XII implemented state, XII.1/XII.2/XII.4) |
| `docs/v2/shardeo-v2-especificacion-tecnica.md` | Modified | T1 (Transport adapter-chooses) + T2 (terminal/* implemented) + T3 (Confined debt) |

No other files touched. `git diff --name-only` = 2 docs only.

## TDD Cycle Evidence (Strict TDD — documentary [REV] exemption)

Standard code TDD RED→GREEN→REFACTOR does not apply: this change is [REV] documentary only (no source, no tests to write), per task exemption and design Threat Matrix N/A. Strict TDD discipline is satisfied by not touching code without tests.

| Task | RED (test written first) | GREEN (impl passes) | REFACTOR | Result |
|------|--------------------------|---------------------|----------|--------|
| 1.1 C1 F-03 | N/A — [REV] no code; spec scenario = grep assertion | grep "declares it in capabilities" ✓, fallback removed from VII.4 | — | PASS (documentary) |
| 1.2 C2 F-01/F-02/F-04 | N/A — [REV] | grep "already implemented" + "v2-no-regresion" + "XII.4.*agent.ts" ✓ | — | PASS |
| 2.1 T1 F-03 | N/A — [REV] | grep "negotiates it in .initialize." ✓, Transport without fallback ✓ | — | PASS |
| 2.2 T2 F-01 | N/A — [REV] | grep "terminal.*implemented" + "Real PTY" ✓ | — | PASS |
| 2.3 T3 F-04 | N/A — [REV] | grep "Confined debt" + "agent.ts" ✓ | — | PASS |
| 3.1 U-01 | N/A — auditable diff | git diff --stat 2 files, 4 git @@ (5 logical hunks), no code/SPECS.md ✓ | — | PASS |
| 3.2 Preservation | N/A — [REV] | grep "CAS.*deferred" + "XII.3.*not implemented now" ✓ | — | PASS |

No tests executed via `npm test` (skipped per instruction; node-pty binding unavailable; not applicable to documentary change).

## Work Unit Evidence

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `grep -n "declares it in capabilities" docs/v2/*.md && git diff --stat` → `docs/v2/shardeo-v2-constitucion.md:110` match; `docs/v2/shardeo-v2-constitucion.md \| 9 +++++----` + `docs/v2/shardeo-v2-especificacion-tecnica.md \| 6 ++++--` (2 files, 9 ins, 6 del). Also per-task greps: C1 `declares it in capabilities` ✓, C2 `already implemented`+`v2-no-regresion`+`XII.4.*agent.ts` ✓, T1 `negotiates it in .initialize.` ✓, T2 `terminal.*implemented`+`Real PTY` ✓, T3 `Confined debt`+`agent.ts` ✓, 3.2 `CAS.*deferred`+`XII.3.*not implemented now` ✓. |
| Runtime harness command/scenario and exact result | N/A — [REV] documentary, grep/diff only; no runtime boundary (no Broker, no adapter). Justification: the change does not modify code nor produce an executable artifact. |
| Rollback boundary | `git revert HEAD` (2 docs) — reverts exactly C1+C2+T1+T2+T3 without touching code, SPECS.md, §4, or openspec artifacts (untracked). Baseline `710bc6a` remains intact if the work unit commit is reverted. |

## Deviations from Design

None — implementation matches design.md verbatim (C1, C2, T1, T2, T3 exact proposed wording; order C1→C2→T1→T2→T3). Note: `fallback` word remains in unrelated sections (IV.1, VI.3, schema) — intentional, not transport-related; design only removes `fallback` qualification from VII.4/§7 Transport.

## Issues Found

- `git diff | grep -c "^@@"` reports 4, not 5, because git coalesces adjacent T2 (table row) and T3 (inserted debt note) into a single `@@` hunk (they share context lines). Logical hunks = 5 per design; git hunks = 4 but mapping F-01..F-04 remains auditable via `git diff` content.
- `fallback` grep verification as written in tasks.md would fail file-wide (due to candidate fallback); scoped verification (VII.4 / Transport line) used instead.

## Remaining Tasks

None — 7/7 complete.

## Workload / PR Boundary

- Mode: single-pr
- Current work unit: 5 hunks + U-01 verification — the change's only work unit
- Boundary: `docs/v2/shardeo-v2-constitucion.md` (C1+C2) + `docs/v2/shardeo-v2-especificacion-tecnica.md` (T1+T2+T3)
- Estimated review budget impact: ~15 net lines (9 ins / 6 del), well under 400; single PR
- Chain strategy: not applicable

## Status

7/7 tasks complete. Ready for verify (sdd-verify). Commit pending as single work unit `docs(v2): reconcile normative docs with Specs 9a-9c`.

## Verification Outputs (pre-commit)

```
git diff --stat →
 docs/v2/shardeo-v2-constitucion.md           | 9 +++++----
 docs/v2/shardeo-v2-especificacion-tecnica.md | 6 ++++--
 2 files changed, 9 insertions(+), 6 deletions(-)

git diff --name-only →
 docs/v2/shardeo-v2-constitucion.md
 docs/v2/shardeo-v2-especificacion-tecnica.md

grep checks: all passed (see Completed Tasks)
```