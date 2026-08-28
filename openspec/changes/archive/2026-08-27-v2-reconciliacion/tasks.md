# Tasks: Reconciliación normativa v2

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 40–60 (2 docs) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | 5 hunks + verificación U-01 | PR 1 | `grep -n "declara en capacidades" docs/v2/*.md && git diff --stat` | N/A — [REV] documental, solo grep/diff | `git revert HEAD` (2 docs) |

## Phase 1: Constitución (C1 → C2)

- [x] 1.1 **C1 VII.4 F-03** `docs/v2/shardeo-v2-constitucion.md:110` reemplazar `JSON-RPC 2.0 sobre stdio … HTTP local + SSE es un fallback … nunca general` por texto adapter-elige-declara-negocia (`initialize`, no jerarquiza) + ejemplo OpenCode supervised HTTP+SSE. → F-03. Verify: `grep -q "declara en capacidades" && ! grep -q "fallback" `.
- [x] 1.2 **C2 XII F-01+F-02+F-04** `docs/v2/shardeo-v2-constitucion.md:195-199` reemplazar bloque `XII. Diferido … XII.1 terminal diferido / XII.2 Bundle+CAS / XII.3` por `XII. Estado implementado y diferidos — XII.1 terminal/PTY ya implementado Spec 9c (pty.ts/terminal.ts/attach.ts) PTY real+attach+presencia — XII.2 bundle+manifiesto+admisión Spec 9a → v2-no-regresion, CAS diferido — XII.3 preservado — XII.4 deuda JSONL src/utils/agent.ts headless → v2-adapter/F-04`. → F-01,F-02,F-04. Verify: `grep -q "ya está implementado" && grep -q "v2-no-regresion" && grep -q "XII.4.*agent.ts"`.

## Phase 2: Especificación §7 (T1 → T3)

- [x] 2.1 **T1 §7 F-03** `docs/v2/shardeo-v2-especificacion-tecnica.md:535` reemplazar `Transporte: JSON-RPC 2.0 sobre stdio … (fallback …)` por `el adapter elige transporte nativo (stdio-RPC/HTTP+SSE/JSONL), declara capacidades, negocia en initialize` + ejemplo OpenCode. → F-03. Verify: `grep -q "negocia en .initialize." && ! grep -q "fallback"`.
- [x] 2.2 **T2 §7 F-01** `docs/v2/shardeo-v2-especificacion-tecnica.md:546` fila `terminal/*` reemplazar `*(opcional, diferido)* … Ver XII.1 — no implementado` por `*(opcional, implementado)* … PTY real, attach humano y presencia; Spec 9c (pty.ts/terminal.ts/attach.ts)`. → F-01. Verify: `grep -q "terminal.*implementado" && grep -q "PTY real"`.
- [x] 2.3 **T3 §7 F-04** `docs/v2/shardeo-v2-especificacion-tecnica.md:547` insertar `**Deuda confinada:** parser JSONL OpenCode en src/utils/agent.ts (headless 1596–1713); traslado pendiente v2-adapter/F-04.` → F-04. Verify: `grep -q "Deuda confinada" && grep -q "agent.ts"`.

## Phase 3: Verificación documental (U-01 + preservación)

- [x] 3.1 **U-01 diff auditable** `git diff --name-only` solo `docs/v2/shardeo-v2-constitucion.md` + `docs/v2/shardeo-v2-especificacion-tecnica.md`; 5 hunks mapeados F-01..F-04, sin código/SPECS.md/§4/changelogs. → U-01. Verify: `git diff --stat && git diff | grep -c "^@@"` =5.
- [x] 3.2 **Preservación CAS+XII.3** CAS sigue `permanece diferido` y XII.3 `no se implementa ahora; interfaz persistencia (III.3) no bloquea`. → F-02 preservación. Verify: `grep -q "CAS.*diferido" && grep -q "XII.3.*no se implementa ahora"`.

> Orden C1→C2→T1→T2→T3→U-01. Sin código/tests. Rollback: revert commit.
