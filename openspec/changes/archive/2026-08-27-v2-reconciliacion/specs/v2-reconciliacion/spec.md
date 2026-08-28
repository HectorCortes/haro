# Specification of v2-reconciliacion

## Purpose

Reconcile the v2 regulations with Specs 9a–9c through exclusively documentary changes, verifiable against `shardeo-v2-deltas-acceptance.md` and the diff of the update.

## Requirements

### Requirement: Terminal/PTY recognized as an existing surface [v2-reconciliacion/F-01]

The Constitution XII.1 and the `terminal/*` row of the Specification §7 MUST recognize `terminal`/PTY as an implemented surface that preserves real PTY, human attach and presence, with reference to Spec 9c (`src/runtime/pty.ts`, `terminal.ts`, `attach.ts`; `SPECS.md` §9c).

#### Scenario: Implemented state
- GIVEN the final documents
- WHEN XII.1 and the `terminal/*` row are reviewed
- THEN `terminal`/PTY no longer appears as deferred or not implemented

#### Scenario: Explicit preservation
- GIVEN the new wording
- WHEN real PTY, human attach and presence are searched for
- THEN the three behaviors appear as existing ones to preserve

### Requirement: Bundle and admission recognized without enabling CAS [v2-reconciliacion/F-02]

The Constitution XII.2 MUST recognize as implemented the immutable bundle, SHA-256 manifest and admission test of Spec 9a (`src/runtime/bundle.ts`, `src/schema/bundle.ts`, `src/runtime/admission.ts`) and refer them to `v2-no-regresion`; the interaction CAS and multi-user XII.3 MUST remain deferred.

#### Scenario: Bundle out of deferred
- GIVEN the final Constitution
- WHEN XII.2 is reviewed
- THEN bundle, SHA-256 manifest and admission do not appear as deferred work

#### Scenario: Deferred items preserved
- GIVEN the final Constitution
- WHEN XII.2 and XII.3 are reviewed
- THEN interaction CAS and multi-user remain explicitly deferred

### Requirement: Transport chosen by capabilities [v2-reconciliacion/F-03]

VII.4 and the Specification §7 MUST abstractly define that each adapter chooses a native transport supported by its harness —stdio-RPC, local HTTP + SSE or JSONL—, declares it in capabilities and negotiates it in `initialize`. OpenCode HTTP+SSE MAY appear only as a non-normative example (`src/runtime/transport.ts`, `src/adapters/opencode.ts:296-326`, `src/adapters/opencode-http.ts`).

#### Scenario: Consistent abstract rule
- GIVEN both final documents
- WHEN VII.4 and §7 are inspected
- THEN both express selection by capabilities and negotiation in `initialize`

#### Scenario: HTTP+SSE is not a fallback
- GIVEN the wording about local HTTP + SSE
- WHEN a contextual textual assertion is executed
- THEN the word `fallback` does not qualify HTTP+SSE nor is it presented as an inferior route

### Requirement: Confined and traceable JSONL debt [v2-reconciliacion/F-04]

The Constitution XII MUST include a one-line note and the Specification §7 MUST detail that the OpenCode JSONL parser remains in `src/utils/agent.ts` (headless path; lines 1596–1713), and that `v2-adapter` MUST move it to the adapter.

#### Scenario: Exact reference
- GIVEN the final documents
- WHEN `src/utils/agent.ts`, `headless` and `v2-adapter` are searched for
- THEN the brief note exists in XII and the full detail exists in §7

#### Scenario: Undeclared debt resolved
- GIVEN the F-04 note
- WHEN its status is reviewed
- THEN the move is described as pending, not as already corrected behavior

### Requirement: Auditable and bounded update [v2-reconciliacion/U-01]

The PR diff MUST be the only authority of the update and MUST contain only changes in the two `docs/v2/` documents mappable to F-01..F-04; it MUST NOT create changelogs or additional documentary artifacts. The Go interfaces of §4 MUST remain conceptual, with TypeScript + ACP + zod as authorities if a clarification becomes necessary.

#### Scenario: Justifiable diff
- GIVEN the final PR diff
- WHEN each hunk is assigned to a criterion
- THEN all map exactly to F-01, F-02, F-03 or F-04 with their reason

#### Scenario: No scope creep
- GIVEN the final PR diff
- WHEN modified files and sections are enumerated
- THEN only the two documents and the agreed hunks appear, without code changes or `SPECS.md`