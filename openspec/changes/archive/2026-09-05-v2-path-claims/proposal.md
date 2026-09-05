# Proposal: v2-path-claims (path_claims and workspace isolation)

## Intent

Deliver constitution §IX and F-01–F-07/U-01–U-03: isolated worktrees by default; `system → workflow → step` inheritance; canonical prefix claims released after success or failure; isolated/isolated concurrency; `block|allow` isolated/shared policy; external-as-shared behavior; atomic Acquire and owner-checked Release.

## Scope

### In Scope
- Sole, idempotent `path_claims` DDL and `PathClaimRepository` (atomic `Acquire` via `BEGIN IMMEDIATE`/conflict semantics, `Release`, `ListActive`).
- `internal/claim/{canonical,policy}.go`: symlink-anchored canonicalization, boundary-safe prefixes, four-row matrix, external-as-shared.
- `internal/worktree/manager.go`: dependency-free `git worktree add/remove/prune` via `exec`.
- Workflow/step `workspace: {mode,on_logical_conflict}` parsing, validation, inheritance, isolated/block defaults; `.haro/config.yaml` `external_paths`.
- Engine: persist execution worktree/step roots; acquire before `pending→running`; defer release; filter blocked `steps next`; direct CLI returns structured `logical_conflict`.
- Technical-spec §7-style sole-ownership note.

### Out of Scope
- `leases`, `interactions`, full 11-table DDL (`v2-store`/`v2-broker`); broker UDS/wait queue; composition; PTY; reporting; distribution.

### Scope Slices

| Slice | Criteria |
|---|---|
| Worktrees and inheritance | F-01, F-02 |
| Canonical claims and lifecycle | F-03, F-04, U-01 |
| Conflict policy and externals | F-05, F-06, F-07, U-03 |
| SQLite atomicity | U-02 |

## Capabilities

### New Capabilities
- `v2-path-claims`: workspace isolation, logical claims, conflict policy, and claim lifecycle.

### Modified Capabilities
- None.

## Approach

Choose sole ownership of real SQLite `path_claims`, mirroring `attempt_transport`; deferral to a `v2-store` mock cannot prove U-02 atomicity. Create one worktree per execution and defer each claim release to step completion.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/{store,claim,worktree}` | New/Modified | DDL, repository, identity, policy, lifecycle |
| `internal/{workflow,execution,cmd,project}` | Modified | YAML/config, workspace roots, gating/errors |
| `docs/v2/haro-especificacion-tecnica.md` | Modified | Migration ownership contract |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Symlink escape/prefix confusion | High | Anchor resolved root; reject escapes; boundary table |
| Worktree leaks/git-less CI | Med | Idempotent cleanup/prune; `testing.Short()`/`LookPath`; fakes |
| SQLite race fidelity | Med | File-backed DB, never `:memory:`; concurrent tests |
| Migration divergence | Med | Sole-owner note; exact idempotent contract |
| External schema/default drift | Med | Minimal `external_paths`; empty list; preserve isolated/block defaults |

## Rollback Plan

Revert worktree creation, engine gating, and YAML/config fields; drop additive `path_claims` and its index. Re-running prior idempotent migrations remains safe; no destructive migration is introduced.

## Dependencies

- Git for worktree E2E; existing SQLite/YAML packages. Normative: constitution §IX and technical specification §§1,2,5.

## Success Criteria

- [ ] All ten mapped criteria pass reproducibly, including failure release, matrix, wrong-owner rejection, and file-backed concurrent Acquire.
- [ ] Existing tests remain green; no `leases`, `interactions`, broker queue, or composition ownership leaks into this change.
