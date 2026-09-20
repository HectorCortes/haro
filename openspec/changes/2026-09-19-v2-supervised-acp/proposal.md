# Proposal: Supervised Managed Sessions through ACP

## Intent

Enable broker-owned `mode: supervised` ACP sessions without headless downgrade. Keep the guard until the complete path is proven, following `SPECS.md` §9, the v2 Constitution, and technical specification.

## Scope

### In Scope
- Launch `acp-generic` without a shell over JSON-RPC 2.0 stdio, with fixed arguments/environment, one `initialize`, and the ACP cycle.
- Freeze ordered instructions, skills, required artifacts, and prior/fallback context as contained attempt entries; admit only receipts matching bundle, manifest, and mandatory-entry SHA-256 identities.
- Persist session identity/state (`starting`, `ready`, `running`, `awaiting_interaction`, `cancelling`, terminal/`orphaned`), endpoint generation, lease heartbeat, admission, and transport identity.
- Retain sanitized output in a contained file with configurable age/size; expose bounded `step output --tail` and terminal-only streaming `--full`, never paths.
- Configure independent inactivity and decision timers, defaulting to 300 seconds.
- Use `pending → resolving → resolved|resolution_failed`; after an uncertain provider application, orphan unless negotiated idempotent resume is proven.

### Out of Scope
- Terminal/PTY, OpenCode HTTP/SSE, headless-adapter changes, broad normative rewrites, automatic orphan restart, or authorization replay.

## Capabilities

### New Capabilities
- `v2-supervised-acp`: ACP admission, supervision, output, timeouts, and recovery.

### Modified Capabilities
- `v2-no-regresion`: replace only the supervised guard after F-08/F-09/F-11 proof.
- `v2-adapter`: register real `acp-generic`; extend U-03 beyond translations.
- `v2-ipc`: allow supervised `step.run`; preserve F-03–F-06/U-01–U-04.

## Approach

Add a broker-native supervisor behind `Engine`, `EventSink`, fenced repositories, and `BrokerSessionHost`; ACP remains adapter-owned. After restart, reconnect only with negotiated resume and validated identity; otherwise persist `orphaned`, retain evidence, and reject stale writes.

## Acceptance Mapping

| Target | Scope |
|---|---|
| `v2-no-regresion/F-08,F-09,F-11` | Admission, E2E, permission detection |
| `v2-adapter/U-03` | Full ACP fixture cycle |
| `v2-broker/F-06,F-07`; `v2-ipc/F-03–F-06,U-01–U-04` | Required lifecycle contracts |

## Affected Areas

| Area | Impact |
|---|---|
| `internal/{execution,broker,adapter/acp,store,ipc}` | Runtime, transport, persistence, output |

## Risks

| Risk | Mitigation |
|---|---|
| Partial enablement bypasses policy | Keep guard through E2E readiness proof |
| Crash duplicates authorization | Persist resolving state; orphan unless safe resume |
| Output/drift leaks or corrupts evidence | Containment, digests, sanitization, limits |

## Rollout and Rollback

Ship schema and dormant ACP capability first; enable after full tests. Roll back by unregistering `acp-generic` and restoring the guard while retaining audit records.

## Proposal Question Round

Confirm executable/argument schema, retention defaults, and whether CLI-guided orphan recovery ships now or with `resume`; other choices above are proposed defaults.

## Dependencies

- Completed broker/IPC seams and pure-Go SQLite.

## Success Criteria

- [ ] ACP fixtures admit exact bytes, return early, persist-before-publish, resolve once, and settle under fencing.
- [ ] Drift, malformed ACP, timeout, death, stale generation, unknown decision, and unsafe resume fail closed without duplicate effects.
