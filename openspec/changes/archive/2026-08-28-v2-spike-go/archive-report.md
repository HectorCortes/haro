# Archive Report: v2-spike-go

**Change**: v2-spike-go (M0 Go spike)
**Archived to**: `openspec/changes/archive/2026-08-28-v2-spike-go/`
**Date**: 2026-08-28
**Artifact store mode**: hybrid (Engram + OpenSpec)
**Archived by**: sdd-archive subagent

## Final State (at close)

The change is CLOSED. All 9 implementation tasks completed; verification PASSED (envelope `gentle-ai.verify-result/v1` validated by `gentle-ai sdd-verify-validate` → verdict pass); specs synced to main source of truth; change folder moved to archive. This report is the terminal record — it describes the state AT CLOSE, not intermediate snapshots (`apply-progress.md` / `verify-report.md` are historical snapshots; their "pending/blocked/incomplete" claims are invalid for the close moment). Per the orchestrator handoff (FINAL-STATE FACTS, rank 3 over intermediate snapshots), the following are the authoritative close facts.

### Verification
- **Verdict**: PASS. Envelope `schema: gentle-ai.verify-result/v1`, `evidence_revision: sha256:b49bb79e747e5a8dbc3f0e44adfa6900c7bf733d3cfc2b05adb5f2ef2e0f1581`, admitted by `gentle-ai sdd-verify-validate`.
- **Requirements**: 10/10 (F-01..F-07, U-01..U-03)
- **Scenarios**: 10/10 compliant
- **Gates at close (all exit 0)**: `go build ./...` 0, `go vet ./...` 0, `go test ./... -race` 0, `golangci-lint run` 0, `govulncheck ./...` 0, `CGO_ENABLED=0 go build ./...` 0.
- **CRITICAL findings**: 0; **WARNING findings**: 1 (documented deviation, non-blocking); only SUGGESTIONs recorded.

### Commits
- **Range**: `aa13462..c993285` (8 conventional commits over `origin/main`):
  1. `feat(go):` bootstrap module and main
  2. `feat(workflow):` parse proof
  3. `feat(store):` sqlite proof
  4. `feat(ipc):` health proof
  5. `feat(adapter):` capabilities proof
  6. `chore(config):` Go config and gitignore
  7. `fix(lint):` handle errcheck
  8. `chore(sdd):` SDD bookkeeping (this archive's parent bookkeeping commit)
- **Evidence revision**: `c993285` (HEAD at close), matching the verify-report envelope.

### Deliverable deviations (documented, non-blocking)
- **Go version**: `go.mod` declares `go 1.25.0` (design said 1.23; spec F-01 floor is `>=1.23`). Required by `modernc.org/sqlite v1.57.0` transitive deps (vet fails with 1.23). Satisfies spec floor; no spec break. Recorded in apply-progress obs 1234.
- **`openspec/config.yaml`**: rewritten to Go truth — `strict_tdd: true`, `testing.runner.command: "go test ./..."`, `review_budget_lines: 20000`, `execution.mode: auto`, `artifact_store: both`, `delivery_strategy: single-pr` preserved; linter `golangci-lint run`, checker `go vet ./...`.
- **`.gitignore`**: appended `*.out`, `coverage.*`, `*.cover`; all 7 original lines preserved (verified additive-only).
- **`docs/v2/*` and `deltas-acceptance.md`**: untouched — `git diff origin/main..HEAD -- docs/ deltas-acceptance.md` empty.

## Criteria Compliance (final)
| Criterion | Status | Evidence |
|-----------|--------|----------|
| F-01 Buildable bootstrap | ✅ COMPLIANT | `go build ./...` 0, `go build -o haro .` → `haro v0.0.0-spike`, `CGO_ENABLED=0` 0, module `github.com/HectorCortes/haro`, pure Go no cgo |
| F-02 Vet-clean bootstrap | ✅ COMPLIANT | `go vet ./...` 0 |
| F-03 Race-clean tests | ✅ COMPLIANT | `go test ./... -race` 0, 8 tests pass, no races |
| F-04 Lint-clean bootstrap | ✅ COMPLIANT | `golangci-lint run` 0 issues |
| F-05 Vulnerability-clean deps | ✅ COMPLIANT | `govulncheck ./...` 0 |
| U-01 Go-aligned config | ✅ COMPLIANT | config + gitignore verified additive |
| U-02 Sample workflow parsing | ✅ COMPLIANT | `TestParse/valid_workflow` PASS, `version==2`, `steps[0].id=="build"` |
| F-06 Pure-Go SQLite roundtrip | ✅ COMPLIANT | `TestSQLiteRoundtrip` PASS, FK==1, WAL success-only, insert/select match |
| F-07 Unix-socket health roundtrip | ✅ COMPLIANT | `TestHealthRoundtrip` PASS, `h.sock` + `t.TempDir()`, JSON-RPC 2.0 |
| U-03 Capabilities JSON preservation | ✅ COMPLIANT | `TestCapabilitiesRoundtrip` PASS, protocol/permission/`_`-prefixed extras preserved |

## Gates
- **Native Review Receipt Gate**: `reviewGate` structurally ABSENT (no `review/` artifact discovered for this candidate). Archive proceeds under ordinary repository policy. Verify passed; no review gate to satisfy.
- **Task Completion Gate**: `tasks.md` 9/9 implementation tasks checked `[x]`. PASS. No stale unchecked tasks.
- **Strict-vs-OpenSpec policy**: no CRITICAL issues; no incomplete tasks. No override needed.

## Spec Sync
- **Domain**: `v2-spike-go`
- **Action**: Created main spec. This change is a bootstrap, NOT a scored spec in `deltas-acceptance.md`; the delta spec is a full spec, not a delta with ADDED/MODIFIED/REMOVED markers. Followed the convention of the archived `v2-reconciliacion` (`openspec/specs/v2-reconciliacion/spec.md` was also created by mechanical copy).
- **Target**: `openspec/specs/v2-spike-go/spec.md`
- **Mechanical copy**: shell `cp` to temp + `diff -r` readback EMPTY (byte-identical), then `mv` temp to target. 10 requirements (F-01..F-07, U-01..U-03) merged into source of truth.
- **Destructive-merge warning**: not triggered — this is a CREATE, not a destructive merge; `rules.archive` "Warn before merging destructive deltas" N/A.

## Archive Contents (verified by `diff -r`, byte-identical to pre-move snapshot)
- proposal.md ✅
- design.md ✅
- specs/v2-spike-go/spec.md ✅
- tasks.md ✅ (9/9 complete)
- verify-report.md ✅ (envelope YAML header kept intact)
- archive-report.md ✅ (this file, additive — excluded from `diff -r`)
- apply-progress: **Engram-only** (obs 1234) — not on disk. This change's pipeline persisted apply-progress to Engram (unlike the reference `v2-reconciliacion`, which wrote it to disk). No on-disk `apply-progress.md` was fabricated; it is fully traceable via Engram.

## Mechanical Copy Verification
- Spec sync `diff -r` (delta spec vs temp copy): EMPTY — byte-identical. Status 0.
- Move `diff -r` (pre-move snapshot vs `openspec/changes/archive/2026-08-28-v2-spike-go/`): EMPTY — byte-identical. Status 0.
- Both verbatim diff outputs (empty) are reproduced in the sdd-archive phase result returned to the orchestrator.

## Traceability
- **Artifact store**: hybrid. OpenSpec filesystem artifacts moved; Engram observations recorded for full traceability.
- **Engram observation IDs** (all read for this archive):
  - proposal 1224
  - spec 1225
  - design 1226
  - tasks 1229
  - apply-progress 1234
  - verify-report 1242
- **OpenSpec paths**:
  - `openspec/changes/archive/2026-08-28-v2-spike-go/` (all artifacts)
  - `openspec/specs/v2-spike-go/spec.md` (synced main spec)
- **Engram topic**: `sdd/v2-spike-go/archive-report` (this report, `capture_prompt: false`)

## Observations (non-blocking, recorded per handoff)
- **Cosmetic nit 1**: apply-progress obs 1234 contains self-referential wording ("See previous observation #1234...") — no functional impact.
- **Cosmetic nit 2**: `tasks.md` task 1.1 text still says `go 1.23` while the implemented `go.mod` is `go 1.25.0` (documented deviation above). Non-blocking; the actual committed `go.mod` is authoritative.
- **Carry-forward constraint**: Go 1.25 floor is the only lasting constraint; later v2 changes must not regress to 1.23 without downgrading `modernc.org/sqlite`.

## Next Recommended
First scored v2 change per `deltas-acceptance.md` (e.g., `v2-no-regresion` or the next contracted spec). The bootstrap integrations (YAML, SQLite, Unix-socket JSON-RPC, Capabilities shape) are proven and reduce later-spec bootstrap uncertainty.

## Key Learnings
1. `v2-spike-go` closed as a pure-Go bootstrap spike with a byte-identical mechanical archive copy and a created (not merged) main spec.
2. Hybrid archive requires both the OpenSpec filesystem move (`git mv`, empty `diff -r`) and an Engram topic persist with `capture_prompt: false`.
3. A missing `reviewGate` key means no receipt exists — archive proceeds under ordinary policy, not a defect to investigate.
4. A bootstrap spec outside `deltas-acceptance.md` is synced to `openspec/specs/` as a full spec via mechanical copy, mirroring the `v2-reconciliacion` reference.
