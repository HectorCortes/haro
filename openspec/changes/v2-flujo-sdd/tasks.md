# Tasks: v2 SDD flow (D09)

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 350–550 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No (single PR; size:exception pre-approved, 200000; direct push) |
| Suggested split | Single PR |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Medium

### Work Units

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|
| 1 | Docs neutralization | grep per task | N/A — docs-only | revert both docs |
| 2 | Auditor + 4 property tests | `go test ./internal/cmd -run '^TestFlow' -count=1` | `bash scripts/verify-traceability.sh` (real root + fixtures) | delete script + `flow_test.go` |

## Phase 1: Contract edits (deltas-acceptance.md)

- [x] 1.1 L45+L492: id → `v2-flujo-sdd`, content → `Development flow`, status stays `pending`; no other row.
- [x] 1.2 L456: `# Spec: v2-flujo-sdd — Development flow (previously v2-flujo-gentle-ai (D09))`.
- [x] 1.3 §Usage L49–54: heading `## Usage — SDD flow`; L51 neutral; L54 → `go test ./...` (`-race` in CI), E2E vs UNIT/INT layers, gates: build, vet, golangci-lint, govulncheck.
- [x] 1.4 L5+L25 → design's Pure Go/no-cgo stack lines.
- [x] 1.5 L7 → `...in the SDD flow (see §Usage).`; L19 → `(it matches the SDD change name).`
- [x] 1.6 L463 → `verification of the SDD flow`; L466 heading → `Delivery through the SDD flow`; L467 → `SDD flow gates (review receipts, delivery gates)`.
- [x] 1.7 Guard: diff only listed lines; L26–28, L147–148, other specs verbatim.

## Phase 2: AGENTS.md

- [x] 2.1 L7: drop `gentle-ai/` → `including openspec/SDD artifacts`. Only L7.

## Phase 3: Auditor RED→GREEN (scripts/verify-traceability.sh)

- [x] 3.1 RED: create `flow_test.go` (package `cmd`) with `TestFlowCriterionTraceability`: real-root `--check-only` → exit 0 + `TOTAL=92 STRICT=4 INFO=88`; fixtures: relative/absolute/non-git roots, duplicate ID → exit 1 naming ID, removed mapping → fail with criterion+test.
- [x] 3.2 GREEN script (+x), store-gate style: `set -euo pipefail`; script-derived root, optional fixture root; `git -C` discovery, `find` fallback; parse `# Spec: v2-*` + `### (F|U)-[0-9]+`; reject duplicates/malformed/count≠92; strict: D09's 4 mappings+functions; informational: 59-ID allowlist (`v2-broker`, `v2-ipc` = `deferred:cli-direct`; `v2-no-regresion`, `v2-adapter`, `v2-path-claims` = `archived-pending`), unknown fails; print IDs; default runs 4 tests, `--check-only` skips; fail-closed exit 0/1.
- [x] 3.3 MAJOR validator fix: archived-spec evidence accepts BOTH `<archive>/spec.md` flat (composicion, distribucion, reporte, store) AND `<archive>/specs/<name>/spec.md` (reconciliacion, spike-go, no-regresion, adapter, path-claims); `verify-report.md` at change-dir level. RED fixture: non-allowlisted complete `v2-reconciliacion` must resolve via dir layout.

## Phase 4: Property tests RED→GREEN (internal/cmd/flow_test.go)

- [x] 4.1 `TestFlowEachSpecIsSDDChange` (F-01): live name `v2-flujo-sdd`, alias line, 7-phase cycle. RED vs `git stash` baseline.
- [x] 4.2 `TestFlowVerificationViaVerify` (F-02): race/local runners, E2E-vs-module wording, CI gates; rejects `node --test`/`npm`.
- [x] 4.3 `TestFlowDeliveryThroughGates` (F-03; covers validator MINOR): `.docs/initiatives/` + `tools/scripts/initiative/` absent; contract/AGENTS/CI name review + delivery gates; hybrid: docs-only push post-verify accepted, feature code via reviewed PR.
- [x] 4.4 Focused run green; script default executes all four tests; real-root `--check-only` green.

## Phase 5: Final gates

- [x] 5.1 `go build ./...`; `go vet ./...`; `go test ./... -race -count=1`; `golangci-lint run`.
- [x] 5.2 `bash scripts/verify-traceability.sh` (default) exit 0.
- [x] 5.3 No regression: `verify-store-boundary.sh` + `verify-distribution.sh`.
- [x] 5.4 Grep decision: alias stays literal (F-01 mandates `previously v2-flujo-gentle-ai (D09)`); gate = exactly one `gentle-ai` hit in deltas-acceptance.md (the alias line), 0 in AGENTS.md.
- [x] 5.5 `git status --porcelain`: only 4 intended files.

## Non-Goals

openspec/ archive, docs/v2/, product code, new deps, PTY, other specs, AGENTS.md:L23, row flips at archive.
