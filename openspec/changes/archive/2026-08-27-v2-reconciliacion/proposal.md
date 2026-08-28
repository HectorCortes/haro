# Proposal: v2 normative reconciliation

## Intent

Eliminate contradictions between `docs/v2/` and Specs 9a–9c before the remaining v2 deltas.

## Scope

### In Scope
- Fix terminal/PTY, bundle/admission and adapter-native transport.
- Record the JSONL debt and bound each hunk to F-01..F-04.

### Out of Scope
- Code, `SPECS.md`, other v2 changes and new changelogs.
- Interaction CAS and multi-user (`XII.3`), which remain deferred.
- Conceptual Go interfaces of §4; TypeScript + ACP + zod remain authoritative.

## Capabilities

### New Capabilities
- `v2-reconciliacion`: coherence of the v2 regulations.

### Modified Capabilities
- None.

## Approach

- Partition `XII.2`: recognize bundle/manifest/admission and defer CAS.
- Make `VII.4` and §7 abstract: native transport declared and negotiated in `initialize`; OpenCode HTTP+SSE will be a non-normative example.
- Annotate F-04 in Constitution XII and detail it in §7 with `src/utils/agent.ts` (headless).
- Use the PR diff as U-01 authority, without artifacts outside `docs/v2/`.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `docs/v2/shardeo-v2-constitucion.md` | Modified | Reconcile VII.4 and XII.1–XII.2; record F-04. |
| `docs/v2/shardeo-v2-especificacion-tecnica.md` | Modified | Align §7 and document the exact debt. |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Diff scope creep | Medium | Map each hunk to an ID F-01..F-04. |
| Confusing CAS with implemented bundle | Medium | Keep CAS explicitly deferred. |

## Rollback Plan

Revert the hunks of both documents; there is no runtime state.

## Dependencies

- `shardeo-v2-deltas-acceptance.md` (authority) and Specs 9a–9c.

## Success Criteria

- [ ] **F-01**: The v2 normative documentation recognizes that the `terminal`/PTY surface **is already implemented** (real PTY, human attach, presence) and treats it as an existing surface to preserve, not as deferred work.
- [ ] **F-02**: The v2 normative documentation recognizes that the immutable bundle with SHA-256 manifest and admission test **is already implemented** and refers it to no-regression (`v2-no-regresion`), not to deferred work.
- [ ] **F-03**: The v2 normative documentation does not dictate the Broker↔Adapter transport: the adapter chooses it according to what its harness natively supports (stdio-RPC, local HTTP + SSE, JSONL), it is declared in capabilities and negotiated in `initialize`. Local HTTP + SSE is **not a "fallback"** but a valid native route when the harness exposes it (real case: OpenCode supervised uses a managed HTTP/SSE server).
- [ ] **F-04**: The v2 normative documentation records the current confined exception: the OpenCode JSONL parser lives in `src/utils/agent.ts` (headless path) and its move to the adapter is pending debt that the `v2-adapter` spec must close.
- [ ] **U-01**: There is a diff (or changelog) of the update that shows exactly which rules changed and why — never a silent edit.