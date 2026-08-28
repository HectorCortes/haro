# Apply Progress: v2-reconciliacion

**Change**: v2-reconciliacion
**Mode**: Strict TDD (documental [REV] — salvedad aprobada: sin código ni tests; disciplina strict TDD no aplica a tocar código sin test)
**Baseline commit**: `710bc6a` (chore: baseline docs v2 before reconciliation — docs were untracked on main; baseline committed to make `git diff` auditable; not part of work unit per se)
**Date**: 2026-08-27

## Completed Tasks (7/7) — Order C1→C2→T1→T2→T3→3.1→3.2

- [x] **1.1 C1 VII.4 F-03** — `docs/v2/shardeo-v2-constitucion.md:110` — Replaced fixed `JSON-RPC 2.0 sobre stdio … fallback … nunca general` with adapter-elige-declara-negocia text. Verify: `grep -q "declara en capacidades" docs/v2/shardeo-v2-constitucion.md` ✓; VII.4 line no longer contains `fallback` ( `grep "VII.4" … | grep -q fallback` → no match ); note: `fallback` still appears in unrelated sections `IV.1 Attempt` and `VI.3` (candidate fallback, not transport) — expected and out of scope.
- [x] **1.2 C2 XII F-01+F-02+F-04** — `docs/v2/shardeo-v2-constitucion.md:195-199` — Replaced `## XII. Diferido deliberadamente` block (XII.1 diferido, XII.2 Bundle+CAS) with `## XII. Estado implementado y diferidos deliberados` — XII.1 terminal/PTY ya implementado Spec 9c (PTY real, attach, presencia), XII.2 bundle/manifiesto/admisión Spec 9a → `v2-no-regresion`, CAS diferido, XII.3 preservado, XII.4 deuda `src/utils/agent.ts`. Verify: `grep -q "ya está implementado"` ✓, `grep -q "v2-no-regresion"` ✓, `grep -q "XII.4.*agent.ts"` ✓.
- [x] **2.1 T1 §7 F-03** — `docs/v2/shardeo-v2-especificacion-tecnica.md:535` — Replaced `Transporte: JSON-RPC 2.0 sobre stdio … (fallback …)` with adapter-elige text (`negocia en initialize`). Verify: `grep -q "negocia en .initialize."` ✓; `Transporte:` line no longer contains `fallback` ( `grep "^Transporte:" … | grep -q fallback` → no match ); `fallback` remains in schema `harness` fallback description — unrelated.
- [x] **2.2 T2 §7 F-01** — `docs/v2/shardeo-v2-especificacion-tecnica.md:546` — Row `terminal/*` `(opcional, diferido)` → `(opcional, implementado)` with PTY real, attach, presencia; Spec 9c. Verify: `grep -q "terminal.*implementado"` ✓, `grep -q "PTY real"` ✓.
- [x] **2.3 T3 §7 F-04** — `docs/v2/shardeo-v2-especificacion-tecnica.md:547` — Inserted `**Deuda confinada:** parser JSONL OpenCode en src/utils/agent.ts (headless 1596–1713); traslado pendiente v2-adapter/F-04.` Verify: `grep -q "Deuda confinada"` ✓, `grep -q "agent.ts"` ✓ (both docs).
- [x] **3.1 U-01 diff auditable** — `git diff --stat` shows 2 files, `git diff --name-only` shows only the two docs, no code/SPECS.md/§4/changelogs. `git diff | grep -c "^@@"` → 4 git hunks coalesced (C1, C2, T1, T2+T3). Logical hunks = 5 (C1, C2, T1, T2, T3); T2+T3 are adjacent and git coalesces them into one `@@` block — mapped F-01..F-04 all present, verified via grep diff mapping below. U-01 satisfied.
- [x] **3.2 Preservación CAS+XII.3** — CAS stays `permanece diferido`, XII.3 `no se implementa ahora; interfaz persistencia (III.3) no bloquea`. Verify: `grep -q "CAS.*diferido"` ✓, `grep -q "XII.3.*no se implementa ahora"` ✓ (constitución); also `grep -q "CAS.*diferido"` true in especificación? preserved via file (X-linked, but constitucion is authority).

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `docs/v2/shardeo-v2-constitucion.md` | Modified | C1 (VII.4 adapter-elige) + C2 (XII Estado implementado, XII.1/XII.2/XII.4) |
| `docs/v2/shardeo-v2-especificacion-tecnica.md` | Modified | T1 (Transporte adapter-elige) + T2 (terminal/* implementado) + T3 (Deuda confinada) |

No other files touched. `git diff --name-only` = 2 docs only.

## TDD Cycle Evidence (Strict TDD — documental [REV] salvedad)

Standard code TDD RED→GREEN→REFACTOR does not apply: this change is [REV] documentary only (no source, no tests to write), per task salvedad and design Threat Matrix N/A. Strict TDD discipline is satisfied by not touching code without tests.

| Task | RED (test written first) | GREEN (impl passes) | REFACTOR | Result |
|------|--------------------------|---------------------|----------|--------|
| 1.1 C1 F-03 | N/A — [REV] no code; spec scenario = grep aserción | grep "declara en capacidades" ✓, fallback removed from VII.4 | — | PASS (documental) |
| 1.2 C2 F-01/F-02/F-04 | N/A — [REV] | grep "ya está implementado" + "v2-no-regresion" + "XII.4.*agent.ts" ✓ | — | PASS |
| 2.1 T1 F-03 | N/A — [REV] | grep "negocia en .initialize." ✓, Transporte without fallback ✓ | — | PASS |
| 2.2 T2 F-01 | N/A — [REV] | grep "terminal.*implementado" + "PTY real" ✓ | — | PASS |
| 2.3 T3 F-04 | N/A — [REV] | grep "Deuda confinada" + "agent.ts" ✓ | — | PASS |
| 3.1 U-01 | N/A — diff auditable | git diff --stat 2 files, 4 git @@ (5 logical hunks), no code/SPECS.md ✓ | — | PASS |
| 3.2 Preservación | N/A — [REV] | grep "CAS.*diferido" + "XII.3.*no se implementa ahora" ✓ | — | PASS |

No tests executed via `npm test` (skipped per instruction; node-pty binding unavailable; not applicable to documental change).

## Work Unit Evidence

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `grep -n "declara en capacidades" docs/v2/*.md && git diff --stat` → `docs/v2/shardeo-v2-constitucion.md:110` match; `docs/v2/shardeo-v2-constitucion.md \| 9 +++++----` + `docs/v2/shardeo-v2-especificacion-tecnica.md \| 6 ++++--` (2 files, 9 ins, 6 del). Also per-task greps: C1 `declara en capacidades` ✓, C2 `ya está implementado`+`v2-no-regresion`+`XII.4.*agent.ts` ✓, T1 `negocia en .initialize.` ✓, T2 `terminal.*implementado`+`PTY real` ✓, T3 `Deuda confinada`+`agent.ts` ✓, 3.2 `CAS.*diferido`+`XII.3.*no se implementa ahora` ✓. |
| Runtime harness command/scenario and exact result | N/A — [REV] documental, solo grep/diff; no runtime boundary (no Broker, no adapter). Justificación: change no modifica código ni produce artefacto ejecutable. |
| Rollback boundary | `git revert HEAD` (2 docs) — revierte exactamente C1+C2+T1+T2+T3 sin tocar código, SPECS.md, §4, ni openspec artifacts (untracked). Baseline `710bc6a` queda intacto si se revierte el work unit commit. |

## Deviations from Design

None — implementation matches design.md verbatim (C1, C2, T1, T2, T3 redacción propuesta exacta; orden C1→C2→T1→T2→T3). Note: `fallback` word remains in unrelated sections (IV.1, VI.3, schema) — intentional, not transport-related; design only removes `fallback` qualification from VII.4/§7 Transporte.

## Issues Found

- `git diff | grep -c "^@@"` reports 4, not 5, because git coalesces adjacent T2 (table row) and T3 (inserted deuda note) into a single `@@` hunk (they share context lines). Logical hunks = 5 per design; git hunks = 4 but mapping F-01..F-04 remains auditable via `git diff` content.
- `fallback` grep verification as written in tasks.md would fail file-wide (due to candidate fallback); scoped verification (VII.4 / Transporte line) used instead.

## Remaining Tasks

None — 7/7 complete.

## Workload / PR Boundary

- Mode: single-pr
- Current work unit: 5 hunks + verificación U-01 — único work unit del change
- Boundary: `docs/v2/shardeo-v2-constitucion.md` (C1+C2) + `docs/v2/shardeo-v2-especificacion-tecnica.md` (T1+T2+T3)
- Estimated review budget impact: ~15 líneas netas (9 ins / 6 del), well under 400; single PR
- Chain strategy: no aplica

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
