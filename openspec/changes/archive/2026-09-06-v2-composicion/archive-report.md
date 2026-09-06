# Archive Report: v2-composicion (workflow composition — D05)

**Change**: v2-composicion
**Archived**: 2026-09-06
**Archived to**: `openspec/changes/archive/2026-09-06-v2-composicion/`
**Mode**: hybrid (OpenSpec files + Engram)
**Spec synced to**: `openspec/specs/v2-composicion/spec.md` (new domain — full spec: Purpose, Constraints, Requirements, Non-Requirements)

## Final State (authoritative at close — outranks any intermediate snapshot)

```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:a5973fc1d174b3a2308f8e79f0a6431b85da445aaf0f6d5fb3cef844bac6ddc5
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 8/8
scenarios: 9/9
```

- **Tasks**: 22/22 complete (`tasks.md` all `[x]`; Engram obs 2387). Task Completion Gate: PASS, no stale unchecked boxes.
- **Verification**: 8/8 requirements PASS, 9/9 scenarios PASS. Envelope `gentle-ai.verify-result/v1`, `evidence_revision` sha256:a5973fc1d174b3a2308f8e79f0a6431b85da445aaf0f6d5fb3cef844bac6ddc5, verdict `pass_with_warnings`, blockers 0, critical_findings 0 (Engram obs 2391; file `verify-report.md`).
- **Gates green at close**: `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./... -race` exit 0 (14 packages + 1 root `[no test files]`), `golangci-lint run` 0 issues. CRITICAL findings: 0, blockers: 0. (per verify-report obs 2391, final — archive permitted.)
- **Warnings**: 4 non-blocking WARNINGs documented in `verify-report.md`: F-07 real-CLI `steps next` mimic gap; `unknown_output` unified to `unknown_input` (spec-allowed disjunction); field-path Contains-vs-exact variance; U-04 table representativeness. **No post-verify remediation was needed** — no later commits changed behavior after verification.
- **Delivery**: `single-pr` with maintainer-approved `size:exception` (review budget 200000); 2 ledger resets accepted by the maintainer for apply line overruns — all recorded in the native ledger.
- **Commits** (9 on branch `main`): `1269c2d` feat(workflow) strict parse and typed validation with contracts; `b161e3c` feat(workflow) bounded flatten and canonical hash; `68d4435` feat(store) dag_hash migration with idempotent ALTER; `1f5ba6b` feat(execution) composition planning, cascade and CLI guards; `7d15107` test(fixtures) composition fixtures and inheritance coverage; `00a4360` docs(sdd) finalize tasks and apply-progress; `bcb9e6e`/`87fdd90`/`74a855c` corrective test commits (contract rejection tables + F-04 bindings, TestMigrateDagHash_Idempotent, composed scheduling/workspace inheritance/hash verification). This commit `docs(sdd)` archives the change.
- **Strict TDD**: ACTIVE and followed — RED→GREEN→TRIANGULATE per task (evidence in apply-progress obs 2388; 22/22 tasks with test files; corrective re-run exactly once after validator evidence-truthfulness FAIL; no behavior changed, gates re-green).

## Informational Warnings (verdict `pass_with_warnings`, non-blocking)

1. **F-07 `steps next` via mimic, not production CLI path**: `TestCompose_F_StepsNext` replicates the deps-satisfied loop (`findNextPending`) instead of running `cmd.Execute` `steps next --json` against the composed fixture; the production handler's claim filtering (`isBlockedByClaims`) was triangulated separately by `TestLogicalConflict`. Risk: Low.
2. **`unknown_output` unified to `unknown_input`**: `compose.go` returns `unknown_input@steps[i].bindings.<name>` for any unknown key not in either set; the spec allows the disjunction `unknown_input|unknown_output`, so the unification is contract-conformant but reduces diagnostic granularity.
3. **Field-path exactness variance**: 5 tests assert exact field paths (`steps[0].bindings.unknownKey` etc.); remaining tests use `strings.Contains`. Both satisfy the spec literal, but exact assertions prove the field shape more strongly.
4. **U-04 schema parity table representative, not exhaustive**: covers `additionalProperties:false`, version, id pattern, workspace/mode enums, required fields, missing dep, cycle; not every per-field permutation listed in the spec. Triangulated via `TestParse_Strict` + `ValidateFile` checks.

SUGGESTIONS (non-blocking, from verify): log canonical hash hex via `t.Logf`; normalize Contains assertions to exact index assertions; add a composed `cmd.Execute` steps-next test; split `unknown_output` if the spec tightens.

## Sole Documented Amendment

- `deltas-acceptance.md` — D05 spec (`v2-composicion`, lines 344–388): all 11 criterion checkboxes (F-01..F-07, U-01..U-04) marked verified `[x]`; tracking row updated from `pending` to `complete` (verdict pass_with_warnings); "Last updated" summary line updated. No implementation-time doc amendments: the verify report confirms no new broker/PTY/reporting/distribution/leases/interactions/`agents_command`/`agentsFile`/`artifacts_dir` code and no changes to existing normative docs.

## Deferrals and Exclusions (intentional, recorded)

- `v2-broker`/`v2-ipc` (broker UDS/wait queues, JSON-RPC IPC) — deferred per the CLI-direct architecture.
- `v2-store` (remaining DDL: `leases`, `interactions`, full repository facade), `v2-reporte` (change reporting), `v2-distribucion` (distribution), PTY, auth policy, schema sugar, context transports. All remain out of scope per proposal/design and owned by later v2 changes.

## Artifact Traceability (Engram observation IDs)

| Artifact | Engram obs ID | File in archive |
|----------|---------------|-----------------|
| explore | 2382 | `openspec/changes/archive/2026-09-06-v2-composicion/exploration.md` |
| proposal | 2383 | `openspec/changes/archive/2026-09-06-v2-composicion/proposal.md` |
| spec | 2384 | `openspec/changes/archive/2026-09-06-v2-composicion/spec.md` |
| design | 2385 | `openspec/changes/archive/2026-09-06-v2-composicion/design.md` |
| design validation | 2386 | (validation note, not a change artifact) |
| tasks | 2387 | `openspec/changes/archive/2026-09-06-v2-composicion/tasks.md` |
| apply-progress | 2388 | `openspec/changes/archive/2026-09-06-v2-composicion/apply-progress.md` |
| verify-report | 2391 | `openspec/changes/archive/2026-09-06-v2-composicion/verify-report.md` |
| archive-report | (this save) | `openspec/changes/archive/2026-09-06-v2-composicion/archive-report.md` |

## Review Gate

`reviewGate` structurally ABSENT — no review artifact was discovered for this candidate; receipt-driven development did not run for it. Archive proceeds under ordinary repository policy. Nothing to investigate.

## Archive Contents

- `proposal.md` ✅
- `exploration.md` ✅
- `spec.md` ✅ (delta spec, verbatim)
- `design.md` ✅
- `tasks.md` ✅ (22/22 tasks complete, no stale unchecked boxes)
- `apply-progress.md` ✅
- `verify-report.md` ✅ (envelope YAML header intact — moved via `git mv`, content never re-written)
- `archive-report.md` ✅ (this file, additive)

## Source of Truth Updated

- `openspec/specs/v2-composicion/spec.md` — new domain spec created. The change's delta spec is a genuine delta (`ADDED` 7 requirements + `MODIFIED` 1), so the main spec was composed as a full spec (Purpose, Constraints, Requirements, Non-Requirements) per the archive convention, modeled on `openspec/specs/v2-path-claims/spec.md`: ADDED requirements carried over verbatim with scenarios; the MODIFIED requirement replaced with its updated text (delta `(Previously: ...)` note and section markers dropped as delta bookkeeping); Non-Requirements section carried over. The `rules.archive` "warn before merging destructive deltas" did not trigger: no `REMOVED`/`RENAMED` sections exist and the merge is purely additive.
- `deltas-acceptance.md` — D05 criteria checkboxes, tracking row, and summary line updated (see Sole Documented Amendment).

## Mechanical Verification

- Folder move: `git mv openspec/changes/v2-composicion openspec/changes/archive/2026-09-06-v2-composicion` (untracked `verify-report.md` and modified `apply-progress.md` staged with `git add` before the move). Pre-move recursive snapshot vs archived tree, `diff -r -x archive-report.md` → **empty (exit 0)**: byte-identical, no truncation or alteration. `archive-report.md` excluded because it did not exist in the source snapshot (additive-only).
- Source directory confirmed gone after the move; active changes directory contains only `archive/`.
- Spec sync: not a byte copy — the delta spec is an ADDED/MODIFIED delta, so `openspec/specs/v2-composicion/spec.md` is the composed full spec (transformation documented above, verified by review against the delta; contents preserved verbatim at requirement/scenario level).

## Next

- Post-archive: proceed to **`v2-reporte`** (change reporting) — the next row in the `deltas-acceptance.md` tracking table after `v2-composicion` and CLI-direct compatible (git-based change report at top-level execution completion). `v2-store` (persistence behind a repository interface) is the alternative candidate; `v2-broker`/`v2-ipc` remain deferred per the CLI-direct architecture.