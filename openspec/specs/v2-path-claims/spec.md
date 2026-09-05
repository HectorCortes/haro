# v2-path-claims Specification

## Purpose

Define path coordination.

## Constraints

Code MUST be pure Go/no cgo without new dependencies; Git is allowed. Worktrees MUST be per-execution, removed at terminal state, and orphan-pruned. Canonicalization MUST anchor symlinks to repository root and reject escapes. Concurrency tests MUST use file-backed SQLite, not `:memory:`. This change solely owns idempotent `path_claims` DDL (`CREATE TABLE IF NOT EXISTS` plus partial active index); `v2-store` MUST NOT diverge. Defaults MUST remain `isolated` and `block`. `.haro/config.yaml` MAY add only `external_paths`. CLI-direct conflicts MUST return structured `logical_conflict` with owner and be filtered from `steps next`; broker waiting is deferred. Leases, interactions, other DDL, broker/UDS/queues, composition, PTY, reporting, and distribution are excluded.

## Requirements

### Requirement: Default worktree [v2-path-claims/F-01]
Each execution MUST default to a distinct Git worktree as its effective `workspace_root`.

#### Scenario: Visible worktree
- GIVEN defaults
- WHEN execution starts
- THEN `git worktree list` shows its worktree and the step runs there

### Requirement: Three-level inheritance [v2-path-claims/F-02]
Workspace mode MUST resolve `system → workflow → step`; workflow `shared` MUST disable isolation unless overridden per step.

#### Scenario: Inheritance fixtures
- GIVEN workflow-shared, isolated/step-shared, and shared/step-isolated fixtures
- WHEN each fixture resolves
- THEN each step receives the expected checkout or worktree root

### Requirement: Canonical claims [v2-path-claims/F-03]
Claims MUST use canonical, relative, symlink-free identity with boundary-aware prefixes.

#### Scenario: Prefix boundary
- GIVEN an active claim on `src/`
- WHEN `src/foo.ts` and `src/foobar/` are claimed
- THEN the child conflicts and the sibling does not

### Requirement: Shared serialization [v2-path-claims/F-04]
Shared conflicts MUST NOT start before release; claims MUST release on step success or failure.

#### Scenario: Release after success
- GIVEN slow shared A owns `src/` and B requests it
- WHEN A succeeds
- THEN `logical_conflict` is returned and `steps next` omits B
- AND B starts after release

#### Scenario: Release after failure
- GIVEN slow shared A owns `src/` and B requests it
- WHEN A fails
- THEN B starts only after A releases the claim

### Requirement: Isolated claims coexist [v2-path-claims/F-05]
Isolated executions MUST coexist on the same logical path.

#### Scenario: Concurrent isolation
- GIVEN two isolated steps claiming `src/`
- WHEN both start concurrently
- THEN both start and no conflict is recorded

### Requirement: Mixed-mode policy [v2-path-claims/F-06]
A mixed isolated/shared collision MUST block by default and allow only explicit workflow `on_logical_conflict: allow`.

#### Scenario: Block and allow fixtures
- GIVEN default-block and explicit-allow mixed-mode fixtures
- WHEN each second step requests the same path
- THEN the block fixture is denied and the allow fixture starts

### Requirement: External paths [v2-path-claims/F-07]
Paths under an `external_paths` prefix MUST be shared regardless of workspace mode.

#### Scenario: External collision
- GIVEN isolated steps claim one configured external cache prefix
- WHEN the second requests its claim
- THEN it is blocked as shared

### Requirement: Canonicalization [v2-path-claims/U-01]
Canonicalization MUST prevent false matches and repository escapes.

#### Scenario: Edge table
- GIVEN internal/external symlinks, `..`, absolutes, and prefix-boundary rows
- WHEN each path is canonicalized and compared
- THEN internal paths normalize, escapes and absolutes fail, and boundaries stay distinct

### Requirement: Atomic acquisition [v2-path-claims/U-02]
`Acquire` MUST atomically INSERT-on-conflict: one incompatible requester wins; losers receive `Acquired=false` and its owner.

#### Scenario: Concurrency
- GIVEN incompatible owners share a file-backed SQLite database
- WHEN they acquire the same logical path concurrently
- THEN exactly one wins and every loser reports that owner

### Requirement: Conflict matrix and release [v2-path-claims/U-03]
Policy MUST allow isolated/isolated, block shared/shared, and honor block/allow for mixed modes; `Release` MUST reject non-owners.

#### Scenario: Four-row matrix
- GIVEN the four mode-policy combinations
- WHEN conflict policy is evaluated
- THEN outcomes are allow, block, block, and allow respectively

#### Scenario: Wrong-owner release
- GIVEN a claim owned by one execution and step
- WHEN another owner releases it
- THEN release is rejected and the claim remains active
