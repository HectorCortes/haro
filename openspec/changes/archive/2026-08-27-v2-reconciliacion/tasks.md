# Tasks: v2 normative reconciliation

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
| 1 | 5 hunks + U-01 verification | PR 1 | `grep -n "declares it in capabilities" docs/v2/*.md && git diff --stat` | N/A — [REV] documentary, grep/diff only | `git revert HEAD` (2 docs) |

## Phase 1: Constitution (C1 → C2)

- [x] 1.1 **C1 VII.4 F-03** `docs/v2/shardeo-v2-constitucion.md:110` replace `JSON-RPC 2.0 over subprocess stdio … local HTTP + SSE is a valid adapter fallback … never the general mechanism` with adapter-chooses-declares-negotiates text (`initialize`, no ranking) + OpenCode supervised HTTP+SSE example. → F-03. Verify: `grep -q "declares it in capabilities" && ! grep -q "fallback" `.
- [x] 1.2 **C2 XII F-01+F-02+F-04** `docs/v2/shardeo-v2-constitucion.md:195-199` replace block `XII. Deliberately deferred … XII.1 terminal deferred / XII.2 Bundle+CAS / XII.3` with `XII. Implemented state and deliberate deferrals — XII.1 terminal/PTY already implemented Spec 9c (pty.ts/terminal.ts/attach.ts) real PTY+attach+presence — XII.2 bundle+manifest+admission Spec 9a → v2-no-regresion, CAS deferred — XII.3 preserved — XII.4 JSONL debt src/utils/agent.ts headless → v2-adapter/F-04`. → F-01,F-02,F-04. Verify: `grep -q "already implemented" && grep -q "v2-no-regresion" && grep -q "XII.4.*agent.ts"`.

## Phase 2: Specification §7 (T1 → T3)

- [x] 2.1 **T1 §7 F-03** `docs/v2/shardeo-v2-especificacion-tecnica.md:535` replace `Transport: JSON-RPC 2.0 over stdio … (fallback …)` with `the adapter chooses the native transport (stdio-RPC/HTTP+SSE/JSONL), declares capabilities, negotiates in initialize` + OpenCode example. → F-03. Verify: `grep -q "negotiates it in .initialize." && ! grep -q "fallback"`.
- [x] 2.2 **T2 §7 F-01** `docs/v2/shardeo-v2-especificacion-tecnica.md:546` `terminal/*` row replace `*(optional, deferred)* … See XII.1 — not implemented` with `*(optional, implemented)* … real PTY, human attach and presence; Spec 9c (pty.ts/terminal.ts/attach.ts)`. → F-01. Verify: `grep -q "terminal.*implemented" && grep -q "Real PTY"`.
- [x] 2.3 **T3 §7 F-04** `docs/v2/shardeo-v2-especificacion-tecnica.md:547` insert `**Confined debt:** OpenCode JSONL parser in src/utils/agent.ts (headless 1596–1713); move pending v2-adapter/F-04.` → F-04. Verify: `grep -q "Confined debt" && grep -q "agent.ts"`.

## Phase 3: Documentary verification (U-01 + preservation)

- [x] 3.1 **U-01 auditable diff** `git diff --name-only` only `docs/v2/shardeo-v2-constitucion.md` + `docs/v2/shardeo-v2-especificacion-tecnica.md`; 5 hunks mapped F-01..F-04, without code/SPECS.md/§4/changelogs. → U-01. Verify: `git diff --stat && git diff | grep -c "^@@"` =5.
- [x] 3.2 **CAS+XII.3 preservation** CAS keeps `remains deferred` and XII.3 `not implemented now; persistence interface (III.3) does not block`. → F-02 preservation. Verify: `grep -q "CAS.*deferred" && grep -q "XII.3.*not implemented now"`.

> Order C1→C2→T1→T2→T3→U-01. No code/tests. Rollback: revert commit.