# Proposal: v2 adapter contract and multi-harness

## Intent

Establish the F-01–F-06/U-01–U-04 transport-agnostic boundary: close OpenCode JSONL debt, confine provider literals, negotiate fail-closed, and add contract tests, Claude/ACP adapters, bounded fallback, and neutral attempts.

## Scope

### In Scope
- CLI-direct `AdapterManager`: single prior `initialize`, capability-gated sessions, unclassified ordered fallback, sanitized context ≤2 MiB.
- Agent wiring; three adapters; neutral synthetic fixtures and provenance README (originals unavailable).
- Contract suite; real-binary opt-in via `HARO_TEST_CLAUDE_BINARY`, skipped under CI `testing.Short()`.
- Sole ownership of `attempt_transport`: repository plus idempotent `CREATE TABLE IF NOT EXISTS`.
- `scripts/verify-adapter-boundary.sh` provider-literal/import gate.

### Out of Scope
- Broker daemon/UDS and CLI↔Broker JSON-RPC (`v2-broker`, `v2-ipc`); lifecycle later moves behind stable interfaces.
- Full 11-table DDL beyond `attempt_transport` (`leases`, `interactions`, `path_claims`), composition, claims, reporting, distribution, and PTY F-10.

### Scope Slices
| Criteria | Slice |
|---|---|
| F-01 | Single prior initialize |
| F-02 | Optional-method gating |
| F-03 | Fail-closed permission negotiation |
| F-04 | OpenCode parser and boundary gate |
| F-05 | Claude adapter and contract E2E |
| F-06 | Ordered bounded fallback |
| U-01 | Capability combination/drift tables |
| U-02 | Versioned public contract suite |
| U-03 | Bijective ACP translation/fixtures |
| U-04 | `attempts` plus extension table |

## Capabilities

### New Capabilities
- `v2-adapter`: Contract, adapters, negotiation, fallback, tests, and transport extension.

### Modified Capabilities
None.

## Approach

Engine uses an in-process manager and stable interfaces; adapters choose transport. NDJSON/JSON-RPC is limited to 10 MiB. Redaction precedes accumulation; additive capabilities round-trip; attempt/transport rows commit transactionally.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/adapter/`, `testdata/` | New | Adapters, fixtures |
| `internal/execution/`, `internal/workflow/` | Modified | Agent fallback |
| `internal/store/` | Modified | Transport extension |
| `scripts/verify-adapter-boundary.sh` | New | F-04 CI gate |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Double migration ownership | High | Single owner; idempotent schema test |
| Secret accumulation | Medium | Redact before every accumulation |
| Oversized RPC | Medium | Enforce 10 MiB limit |
| Capability/provider drift | Medium | Drift table; CI literal gate |
| Fixture gaps/re-home | Medium | Provenance/opt-in; stable interfaces |

## Rollback Plan

Disable agent wiring and revert isolated code. Keep the additive, inert `attempt_transport` table; no destructive migration is required.

## Dependencies

- `deltas-acceptance.md`; constitution §§II/V/VI/VII; technical specification §§1/2/4/7.

## Success Criteria

- [ ] All criteria pass, including contract/ACP/Claude fixtures and boundary gate.
- [ ] Fallback remains sanitized and ≤2 MiB; RPC rejects payloads above 10 MiB.
- [ ] `attempts` stays transport-neutral and existing command behavior remains green.
