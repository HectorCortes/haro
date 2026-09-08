# v2-flujo-sdd Specification

## Purpose

Formalize the project's tool-neutral **SDD flow**, previously `v2-flujo-gentle-ai` (D09), correct stale Node/npm runner guidance to the real Go verification boundary, and make criterion-to-test traceability executable.

## Constraints

- Neutralized references MUST use **SDD flow** uniformly while preserving the historical alias.
- D09 criterion text MUST remain literal except for tool references and stale runner wording.
- The 31 historical `openspec/changes/archive/` matches MUST remain verbatim and their `evidence_revision` hashes MUST remain intact; `docs/v2/` MUST NOT change.
- Only `AGENTS.md:7` MAY change in `AGENTS.md`; no dependencies, product behavior, v1-oracle text, or other delta specifications MAY change.
- Verification MUST remain pure Go/no cgo. The catalog auditor MUST allowlist CLI-direct deferrals (`v2-broker`, `v2-ipc`) and archived-pending rows.

## Requirements

### Requirement: Each spec follows the SDD lifecycle [v2-flujo-sdd/F-01]

Each contract spec MUST be developed as one SDD change through `proposal → spec → design → tasks → apply → verify → archive`, using its criteria as acceptance sources. This D09 change MUST demonstrate that lifecycle without tool-specific wording.

#### Scenario: Neutral lifecycle and preserved alias
- GIVEN the live contract and the D09 change
- WHEN lifecycle and naming are audited
- THEN D09 uses `v2-flujo-sdd` and “SDD flow” through the complete cycle
- AND records “previously `v2-flujo-gentle-ai` (D09)” without changing archived history

### Requirement: Verification uses the real Go boundary [v2-flujo-sdd/F-02]

`sdd-verify` or an equivalent verification MUST run `[E2E]` checks at the public boundary and `[UNIT]`/`[INT]` checks as module tests. The documented CI runner MUST be `go test ./... -race`, with no Node/npm runner claim.

#### Scenario: Go verification layers
- GIVEN the flow guidance and verification evidence
- WHEN their runner and test layers are inspected
- THEN they identify the Go race suite and the public-boundary/module distinction
- AND contain neutral SDD flow wording

### Requirement: Delivery uses development gates [v2-flujo-sdd/F-03]

Every delta MUST pass review receipts and delivery gates rather than an initiatives pipeline. Feature code SHALL use reviewed PRs; documentation-only archive records MAY be pushed directly after verification. No new `.docs/initiatives/` path MAY exist.

#### Scenario: Valid hybrid delivery
- GIVEN verified feature delivery and a documentation-only archive step
- WHEN delivery evidence is reviewed
- THEN the feature has PR review and gate receipts while the archive push is accepted
- AND `.docs/initiatives/` remains absent

### Requirement: Executable criterion-to-test audit [v2-flujo-sdd/U-01]

Every contract criterion MUST have at least one named test. `scripts/verify-traceability.sh` MUST enumerate all 92 IDs, strictly require and run D09's four named Go tests, and report the other 88 informationally under the approved deferral/pending allowlist.

#### Scenario: Allowlisted catalog passes
- GIVEN all four D09 tests exist and pass and known non-D09 gaps are allowlisted
- WHEN the auditor runs
- THEN it exits successfully and reports all 92 IDs without hiding classifications

#### Scenario: Missing D09 test fails closed
- GIVEN any required D09 test is absent or failing
- WHEN the auditor runs
- THEN it exits non-zero and identifies the affected criterion and test

### Requirement: D09 traceability precedent [v2-flujo-sdd/TRACE-D09]

The change MUST preserve this exact criterion-to-test mapping:

| Criterion | Named Go test |
|---|---|
| F-01 | `TestFlowEachSpecIsSDDChange` |
| F-02 | `TestFlowVerificationViaVerify` |
| F-03 | `TestFlowDeliveryThroughGates` |
| U-01 | `TestFlowCriterionTraceability` |

#### Scenario: Complete D09 mapping
- GIVEN `internal/cmd/flow_test.go` and this table
- WHEN D09 traceability is audited
- THEN all four criteria map one-to-one to existing passing tests

## Non-Requirements

Historical OpenSpec archives, `docs/v2/`, product code, initiative-pipeline implementation, D01/D02 (`v2-broker`/`v2-ipc`), archived-pending row completion, PTY, new dependencies, other specs, and rewriting the v1 oracle are outside this change.
