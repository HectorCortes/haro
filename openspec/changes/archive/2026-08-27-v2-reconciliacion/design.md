# Design: v2 normative reconciliation

## Technical approach

Edit only the two normative documents against Specs 9a–9c and the verified code. The PR diff will be the U-01 authority; there will be no changelog and no changes to code, `SPECS.md` or §4.

## Architecture and wording decisions

| Option | Tradeoff | Decision and rationale |
|---|---|---|
| Fixed transport or selection by capabilities | Avoids coupling the core | The adapter chooses, declares and negotiates; OpenCode HTTP+SSE will be a non-normative example. |
| XII entirely deferred or partitioned | Reflects Specs 9a/9c without advancing CAS | Separate implemented from CAS and multi-user deferred. |
| Changelog or diff | Avoids duplicating authority | Use only the PR diff for U-01. |

## Per-hunk edit plan

### `docs/v2/shardeo-v2-constitucion.md`

**Hunk C1 — VII.4, line 110 — F-03**  
Current: "**VII.4** Broker ↔ Adapter ↔ Harness: JSON-RPC 2.0 over the subprocess stdio when the harness natively supports it. Local HTTP + SSE is a valid adapter fallback for harnesses that do not offer stdio-RPC, never the general mechanism."  
Proposed: "**VII.4** Broker ↔ Adapter ↔ Harness: the adapter chooses the native transport its harness supports — for example, stdio-RPC, local HTTP + SSE or JSONL —, declares it in capabilities and negotiates it in `initialize`; the core neither imposes nor ranks a transport. (Non-normative example: OpenCode supervised uses managed local HTTP + SSE.)"

**Hunk C2 — XII, lines 195–199 — F-01, F-02, F-04**  
Current:
"## XII. Deliberately deferred (do not implement yet)
- **XII.1** `terminal`/PTY — the session architecture (broker + adapter) must allow adding it later as an extension of an existing session's surface, without redesign.
- **XII.2** Bundle with SHA-256 manifest, admission test and interaction CAS — introduced once the broker and at least two real harnesses are working. Meanwhile, simple immutable context (copy + digest) is enough.
- **XII.3** Shared multi-user concurrency — not implemented now; the persistence interface (III.3) is designed not to block this evolution."  
Proposed:
"## XII. Implemented state and deliberate deferrals
- **XII.1** `terminal`/PTY is already implemented by Spec 9c (`src/runtime/pty.ts`, `src/runtime/terminal.ts`, `src/runtime/attach.ts`; `SPECS.md` §9c) as an existing surface to preserve: real PTY, human attach and presence.
- **XII.2** The immutable bundle, its SHA-256 manifest and the admission test are already implemented by Spec 9a (`src/runtime/bundle.ts`, `src/schema/bundle.ts`, `src/runtime/admission.ts`) and fall under no-regression (`v2-no-regresion`); the interaction CAS remains deferred until its corresponding spec.
- **XII.3** Shared multi-user concurrency — not implemented now; the persistence interface (III.3) is designed not to block this evolution.
- **XII.4** Known debt: the OpenCode JSONL parser remains in `src/utils/agent.ts` (headless path) and `v2-adapter/F-04` must move it to the adapter."

### `docs/v2/shardeo-v2-especificacion-tecnica.md`

**Hunk T1 — §7, line 535 — F-03**  
Current: "Transport: JSON-RPC 2.0 over the harness subprocess stdio (or local HTTP + SSE as an adapter fallback, see VII.4 of the constitution). It directly reuses the method shape of Agent Client Protocol."  
Proposed: "Transport: the adapter chooses the native transport its harness supports — for example, stdio-RPC, local HTTP + SSE or JSONL —, declares it in capabilities and negotiates it in `initialize`. The contract reuses the method shape of Agent Client Protocol. (Non-normative example: OpenCode supervised uses managed local HTTP + SSE.)"

**Hunk T2 — §7, `terminal/*` row, line 546 — F-01**  
Current: "| `terminal/*` *(optional, deferred)* | — | — | See XII.1 of the constitution — not implemented in the first cut |".  
Proposed: "| `terminal/*` *(optional, implemented)* | — | — | Real PTY, human attach and presence; existing surface to preserve. See Spec 9c (`src/runtime/pty.ts`, `src/runtime/terminal.ts`, `src/runtime/attach.ts`; `SPECS.md` §9c). |"

**Hunk T3 — §7, after the table, line 547 — F-04**  
Current: no note exists.  
Proposed: "**Confined debt:** the OpenCode JSONL parser remains in `src/utils/agent.ts` (headless path, lines 1596–1713); its move to the adapter is pending and `v2-adapter/F-04` must complete it."

## Flow, contracts and files

    Specs 9a–9c + code → hunks F-01..F-04 → PR diff (U-01) → review

Only those files change. §4 remains conceptual; TypeScript + ACP + zod remain authoritative, without additional clarification.

## Order and verification

Apply C1 → C2 → T1 → T2 → T3. Review per criterion: HTTP+SSE without `fallback`; terminal with PTY/attach/presence; bundle/admission referred to `v2-no-regresion`; CAS and XII.3 deferred; two F-04 notes; only two documents. No code tests or migration.

## Threat Matrix

N/A — the change is documentary and does not modify routing, shell, subprocess, VCS/PR automation, executable classification or process integration.

## Completeness criterion and open questions

Complete when each text, location, order and F-01..F-04 mapping can be applied without additional decisions and the diff satisfies U-01. Open questions: none.