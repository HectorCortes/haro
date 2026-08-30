# Archive Report: v2-adapter (adapter contract and multi-harness)

**Change**: v2-adapter
**Archived**: 2026-08-30
**Archived to**: `openspec/changes/archive/2026-08-30-v2-adapter/`
**Mode**: hybrid (OpenSpec files + Engram)
**Spec synced to**: `openspec/specs/v2-adapter/spec.md` (new domain — full spec, non-destructive)

## Final State (authoritative at close — outranks any intermediate snapshot)

- **Tasks**: 18/18 complete (`tasks.md` all `[x]`; Engram obs 1310). Task Completion Gate: PASS.
- **Verification**: 10/10 requirements PASS, 11/11 scenarios PASS. Envelope `gentle-ai.verify-result/v1`, `evidence_revision` sha256:db6aa9b603514d118cfc78cd88f160ec132ea56e2f3528cef6bb7017d9cafa7e, verdict `pass` (Engram obs 1313; file `verify-report.md`).
- **Gates green at close**: `go build ./...` 0, `go vet ./...` 0, `go test ./... -race -count=1` exit 0 (12 packages), `golangci-lint run` 0 issues, `bash scripts/verify-adapter-boundary.sh` pass (grep + import graph). CRITICAL findings: 0, blockers: 0. (per verify-report obs 1313, final — archive permitted.)
- **Commits**: `442349c`..`daa53cf` — 5 conventional commits: `feat(ipc)`, `feat(adapter)`, `feat(workflow,execution)`, `feat(adapter)`, `chore(sdd)` (tasks complete + apply-progress).
- **Changed lines (REAL, recorded per orchestrator handoff)**: ~3283 changed lines; `size:exception` approved 2026-08-30 for delivery `single-pr`. Forecast was 1900–2400; not a failure — recorded as the real number.
- **Strict TDD**: ACTIVE and followed — RED→GREEN→REFACTOR per task (evidence in apply-progress obs 1311; 18/18 tasks with test files; re-verified under `-race`). Apply phase-contract validation PASS recorded (obs 1312).
- **Cosmetic fix applied after apply validation (orchestrator)**: `testdata/fixtures/synthetic/v2.0.0-synthetic.1/README.md` no longer lists `claude-fixture.json` (file never existed); README now lists only `acp-fixture.json` and `opencode-fixture.jsonl`. Re-verified during verify; included in this archive commit.

## U-02 Fixture Provenance Record

- `testdata/fixtures/synthetic/v2.0.0-synthetic.1/` holds **synthetic neutral fixtures** (`acp-fixture.json`, `opencode-fixture.jsonl`) with a provenance README (version `v2.0.0-synthetic.1`, contents, usage).
- Original OpenCode 1.17.18 fixtures were **unavailable**; the README documents this and makes **no byte-identity claim**. Claude real-binary path remains opt-in via `HARO_TEST_CLAUDE_BINARY`, skipped under CI `testing.Short()`.

## Sole Documented Amendment

- `docs/v2/haro-especificacion-tecnica.md` §7.2 — `Store.Transport()`/`TransportRepository`, additive idempotent `attempt_transport` (`CREATE TABLE IF NOT EXISTS`), sole migration owner `v2-adapter` (overlap with `v2-store` avoided). This is the only modified doc in the change; `deltas-acceptance.md` untouched.

## Deferrals and Exclusions (intentional, recorded)

- Broker daemon/UDS and CLI↔Broker JSON-RPC (`v2-broker`, `v2-ipc`); `leases`, `interactions`, `path_claims`, composition, claims, reporting, distribution; PTY (`Terminal` false). All remain out of scope per proposal/design and owned by later v2 changes.

## Observations (informational, non-blocking)

- Coverage 61.1% total, changed-file average ~72.8% (informational; `coverage.available: false` in config). `internal/adapter/claude` 17.9% because real subprocess paths sit behind the opt-in binary — matches the constraint.
- SUGGESTION (non-blocking, from verify): second triangulation case for `TestManager_Probe`; boundary script narrowing on harness-name literals justified, no action needed.

## Artifact Traceability (Engram observation IDs)

| Artifact | Engram obs ID | File in archive |
|----------|---------------|-----------------|
| exploration | 1305 | `openspec/changes/archive/2026-08-30-v2-adapter/exploration.md` |
| proposal | 1306 | `openspec/changes/archive/2026-08-30-v2-adapter/proposal.md` |
| spec | 1308 | `openspec/changes/archive/2026-08-30-v2-adapter/specs/v2-adapter/spec.md` |
| design | 1309 | `openspec/changes/archive/2026-08-30-v2-adapter/design.md` |
| tasks | 1310 | `openspec/changes/archive/2026-08-30-v2-adapter/tasks.md` |
| apply-progress | 1311 | `openspec/changes/archive/2026-08-30-v2-adapter/apply-progress.md` |
| verify-report | 1313 | `openspec/changes/archive/2026-08-30-v2-adapter/verify-report.md` |

## Review Gate

`reviewGate` structurally ABSENT — no review artifact was discovered for this candidate; receipt-driven development did not run for it. Archive proceeds under ordinary repository policy. Nothing to investigate.

## Archive Contents

- `proposal.md` ✅
- `exploration.md` ✅
- `specs/v2-adapter/spec.md` ✅
- `design.md` ✅
- `tasks.md` ✅ (18/18 tasks complete, no stale unchecked boxes)
- `apply-progress.md` ✅
- `verify-report.md` ✅ (envelope YAML header intact — moved via `git mv`, content never re-written)
- `archive-report.md` ✅ (this file, additive)

## Source of Truth Updated

- `openspec/specs/v2-adapter/spec.md` — new domain spec synced (full spec, non-destructive). The `rules.archive` "warn before merging destructive deltas" did not trigger: the delta is a full new-domain spec with no `REMOVED`/`RENAMED`/`MODIFIED` destructive sections.

## Mechanical Verification

- Spec sync: `diff -r` source delta vs `openspec/specs/v2-adapter/spec.md` → empty (identical, exit 0).
- Folder move: `diff -r` pre-move snapshot vs `openspec/changes/archive/2026-08-30-v2-adapter/` → empty (identical, exit 0). `archive-report.md` excluded (did not exist in snapshot).
- Active changes directory no longer contains `v2-adapter` (only `archive/` remains).

## Next

- Post-archive: proceed to `v2-path-claims` (path_claims and workspace isolation) per the dependency map (tracking table order, `deltas-acceptance.md` line 487).