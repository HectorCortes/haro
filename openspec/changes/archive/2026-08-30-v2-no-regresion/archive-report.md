# Archive Report: v2-no-regresion (Preservation of Specs 1–9 — Go re-expression)

**Change**: v2-no-regresion
**Archived**: 2026-08-30
**Archived to**: `openspec/changes/archive/2026-08-30-v2-no-regresion/`
**Mode**: hybrid (OpenSpec files + Engram)
**Spec synced to**: `openspec/specs/v2-no-regresion/spec.md` (new domain — full spec, non-destructive)

## Final State (authoritative at close — outranks any intermediate snapshot)

- **Tasks**: 19/19 complete (`tasks.md` all `[x]`; Engram obs 1254). Task Completion Gate: PASS.
- **Verification**: 10/10 requirements PASS, 11/11 scenarios PASS. Envelope `gentle-ai.verify-result/v1`, `evidence_revision` sha256:3d0378308359060e1e2a1e1beecb7a1cf59181f87f3b4ce7b7412ec33b185a5d, validated by `gentle-ai sdd-verify-validate` → verdict `pass` (Engram obs 1299; file `verify-report.md`).
- **Gates green at close**: `go build ./...` 0, `go vet ./...` 0, `go test ./... -race` 0, `golangci-lint run` 0 issues, `govulncheck ./...` 0. CRITICAL findings: 0. (per verify-report obs 1299, final — no CRITICAL, archive permitted.)
- **Commits**: `01cfa2e`..`cdc667a` — 6 conventional commits: `feat(workflow)`, `feat(store)`, `feat(project)`, `feat(execution)`, `feat(cmd)`, `chore(gate)`.
- **Changed lines (REAL, recorded per orchestrator handoff)**: 33 files, 5041 insertions(+), 5 deletions(-). Forecast was 2200–2800; within approved `size:exception` budget 20000 (review budget). Not a failure — recorded as the real number.
- **Strict TDD**: ACTIVE and followed — RED→GREEN→TRIANGULATE per task (evidence in apply-progress obs 1294; 19/19 tasks with test files; re-verified under `-race`).

## Deferrals and Exclusions (intentional, recorded)

- **U-02** (neutral OpenCode fixtures as-is): DEFERRED to `v2-adapter` by explicit user decision (2026-08-28). This change does NOT include fixture work; no `testdata/opencode` directory exists; `grep -R opencode|claude internal --include=*.go` is empty. Recorded in spec (obs 1251) and verify-report (obs 1299).
- F-04/F-11, agent F-05/F-12, F-06, F-08, F-09, F-10 (PTY): owned by later v2 changes (`v2-adapter`/`v2-broker`/`v2-ipc`/`v2-store`/etc.) — out of scope by cut list.
- `docs/v2/*`, `deltas-acceptance.md`, `openspec/config.yaml`, `.gitignore` UNCHANGED (verified via `git diff efe4138..HEAD` empty for those paths).

## Observations (WARNING-level, non-blocking, informational)

- Coverage total 64.2% (informational; `coverage.available: false` in config, no threshold). Store repository accessors low (21%) because exercised via engine integration, not direct unit — not a P0 gap.

## Artifact Traceability (Engram observation IDs)

| Artifact | Engram obs ID | File in archive |
|----------|---------------|-----------------|
| proposal | 1250 | `openspec/changes/archive/2026-08-30-v2-no-regresion/proposal.md` |
| spec | 1251 | `openspec/changes/archive/2026-08-30-v2-no-regresion/specs/v2-no-regresion/spec.md` |
| design | 1252 | `openspec/changes/archive/2026-08-30-v2-no-regresion/design.md` |
| tasks | 1254 | `openspec/changes/archive/2026-08-30-v2-no-regresion/tasks.md` |
| apply-progress | 1294 | (Engram only — not a file artifact) |
| verify-report | 1299 | `openspec/changes/archive/2026-08-30-v2-no-regresion/verify-report.md` |

## Review Gate

`reviewGate` structurally ABSENT — no review artifact was discovered for this candidate; receipt-driven development did not run. Archive proceeds under ordinary repository policy. No `reviewOffer` asserted (kill switch off / not started). No review recorded; nothing to investigate.

## Archive Contents

- `proposal.md` ✅
- `specs/v2-no-regresion/spec.md` ✅
- `design.md` ✅
- `tasks.md` ✅ (19/19 tasks complete, no stale unchecked boxes)
- `verify-report.md` ✅ (envelope YAML header intact — moved via `git mv`, content never re-written)
- `archive-report.md` ✅ (this file, additive)

## Source of Truth Updated

- `openspec/specs/v2-no-regresion/spec.md` — new domain spec synced (full spec, non-destructive). Mirrors how `2026-08-28-v2-spike-go` synced `openspec/specs/v2-spike-go/spec.md`. The `rules.archive` "warn before merging destructive deltas" did not trigger: the delta is a full new-domain spec with no `REMOVED`/`RENAMED`/`MODIFIED` destructive sections.

## Mechanical Verification

- Spec sync: `diff -r` source delta vs `openspec/specs/v2-no-regresion/spec.md` → empty (identical).
- Folder move: `diff -r` pre-move snapshot vs `openspec/changes/archive/2026-08-30-v2-no-regresion/` → empty (identical). `archive-report.md` excluded (did not exist in snapshot).
- Active changes directory no longer contains `v2-no-regresion`.

## Next

- Post-archive: proceed to `v2-adapter` (owns U-02 fixture provenance) per dependency map.
