# Archive Report: v2 Broker IPC

**Change**: `2026-09-11-v2-broker-ipc`
**Archived on**: 2026-09-20 → `openspec/changes/archive/2026-09-11-v2-broker-ipc/`
**Cycle verdict**: PASS WITH WARNINGS — implemented, verified, and archived.

## Final State (at close)

- **Tasks**: 39/39 complete. The persisted `tasks.md` in this archive has zero unchecked implementation tasks.
- **Verification**: `verify-report.md` (2026-09-20) reports `pass_with_warnings` with 21/21 requirements and 21/21 scenarios compliant, 0 critical findings, 0 blockers. The retrieved delta count is 9 `v2-broker` requirements plus 12 `v2-ipc` delta requirements; inherited base `v2-ipc/U-03` is a 13th IPC criterion revalidated by payload-doctrine tests and is not part of the 21 retrieved deltas.
- **Remediation closed**: C-01 (outgoing JSON-RPC result validation via `Dispatcher.validateResult` before response encoding, with valid round-trip, no-payload, malformed-field, and connection-recovery tests); W-01 (traceability note in the active `v2-ipc` delta identifying 12 delta requirements plus inherited base U-03); W-02 (socket collision hash-prefix extension after checking non-0600 occupancy); W-03 (one subscribed `ServeConn`/`Hub`/SQLite test receiving persisted status and interaction notifications in cursor order).
- **Hardening**: SQLite shared-cache mode removed; `_busy_timeout=5000`, `_txlock=immediate`, `SetMaxOpenConns(1)` single-connection stores, serialized open-time migrations.

## Gate Evidence (recorded in `apply-progress.md`, all cgroup-bounded)

| Evidence | Result |
|---|---|
| `haro-lock-5` — full non-race `go test -p=1 ./...` | exit 0 |
| `haro-lock-6`, `haro-lock-7` — two consecutive clean full race suites `go test -p=1 -race ./...` | exit 0 each |
| F-03 parallel-execution shakeouts: `haro-fix-23` 20/20 non-race; `haro-lock-3` 10/10 non-race; `haro-fix-24` and `haro-lock-4` 5/5 race each | exit 0 |
| `haro-lock-8` — `go vet -p=1 ./...` | exit 0 |
| `haro-lock-9` — Linux `go build -p=1 ./...`; `haro-lock-10` — Windows `GOOS=windows GOARCH=amd64 go build -p=1 ./...` | exit 0 each |
| `haro-fix-37` — `govulncheck ./...` | exit 0, no vulnerabilities |

`verify-report.md` independently re-verified the remediation with fresh bounded selectors for C-01, W-02, W-03, SQLite concurrency, build, and vet, all exit 0.

## Open Non-Blocking Items

- **W-04**: `golangci-lint run ./...` exits 1 with 21 project-wide findings (16 errcheck, 1 govet, 3 staticcheck, 1 unused), the same pre-existing set; outside the change-specific gates.
- **S-01**: suggestion to add a regression for retrying an identical approval after its attempt becomes terminal; not addressed.

## Spec Sync

| Domain | Action | Details |
|---|---|---|
| `v2-broker` | Created | `openspec/specs/v2-broker/spec.md` — 9 requirements (F-01..F-07, U-01, U-02) copied byte-identical from the delta (new capability; delta is the full spec). |
| `v2-ipc` | Updated | `openspec/specs/v2-ipc/spec.md` — 12 ADDED requirements appended (F-01..F-09, U-01, U-02, U-04); existing base requirement `U-03` and its scenarios preserved. Main spec now carries 13 IPC requirements. No destructive changes; the delta's non-normative traceability note was not merged (it is change-level editorial; its W-01 resolution is preserved in the archived delta and this report). |

## Review Gate

No bounded review transaction, ledger, or receipt existed for this candidate (`reviewGate` structurally absent; no review artifacts in the change folder). Archive proceeded under ordinary repository policy per the Native Review Receipt Gate; no review was discovered to read or block on.

## Defect and Contradiction Record

No unrankable source contradictions were found. `apply-progress.md` snapshots that recorded earlier pending states (e.g. the 38/39 gate stop and pre-hardening failures) are superseded by the final task artifact, `verify-report.md`, and the archived gate evidence; they remain in the archive as valid history.

## Operational Notes

- The working tree carries all changes **UNCOMMITTED**; no commit, push, or PR was authorized and none was performed.
- The archive move used plain `mv` because `git mv` was blocked by a stale `.git/index.lock` (0 bytes, dated 2026-09-19 23:54, no live git process at move time). Byte identity was verified by `diff -r` of the pre-move recursive snapshot against the archived tree: no differences.
- Constraint compliance: `deltas-acceptance.md` and `docs/v2/` untouched; no test suites run during archive; all archive commands were read-only bounded filesystem/git-read operations.

## Artifacts Read

- `openspec/changes/2026-09-11-v2-broker-ipc/proposal.md`
- `openspec/changes/2026-09-11-v2-broker-ipc/design.md`
- `openspec/changes/2026-09-11-v2-broker-ipc/tasks.md` (39/39, persisted artifact)
- `openspec/changes/2026-09-11-v2-broker-ipc/apply-progress.md` (intermediate snapshot)
- `openspec/changes/2026-09-11-v2-broker-ipc/verify-report.md` (verification snapshot)
- `openspec/changes/2026-09-11-v2-broker-ipc/specs/v2-broker/spec.md` and `specs/v2-ipc/spec.md` (delta specs)
- `openspec/specs/v2-ipc/spec.md` (existing main spec, merge target)