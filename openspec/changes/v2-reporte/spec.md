# Delta for v2-reporte

## ADDED Requirements

### Requirement: Committed starting-point anchor [F-01; Constitution I.3, IX.5.1; Technical Specification §2]

At `CreateExecution`, the system MUST capture `git rev-parse HEAD` in the resolved repository root as nullable `executions.base_commit`; pre-existing dirty changes MUST NOT alter this committed anchor. `v2-reporte` SHALL solely own an idempotent, `PRAGMA table_info`-guarded column migration; existing rows MAY remain null.

#### Scenario: Creation and migration
- GIVEN a dirty repository and a database with or without `base_commit`
- WHEN migration and execution creation run repeatedly
- THEN one nullable column exists and the execution stores the committed `HEAD`

### Requirement: One on-demand top-level report [F-01, F-02; Constitution IX.5.1–IX.5.2, X.1.2]

A terminal top-level execution MUST expose one execution-scoped, side-effect-free report containing `changed_files`. It SHALL be computed on demand, MUST NOT be persisted in v1, and MUST NOT produce per-step or nested-workflow reports; flattened composition retains one `execution_id`.

#### Scenario: Composed execution completes
- GIVEN a terminal execution containing flattened nested workflows
- WHEN its report is requested
- THEN one report covers the complete execution and no internal sub-report exists

#### Scenario: Report errors
- GIVEN an unknown, pending, or running execution
- WHEN its report is requested
- THEN the error code is respectively `not_found` or `not_completed`

### Requirement: Git-accurate deterministic changed files [F-03, U-01; Constitution I.3, IX.5.1]

The report MUST equal the union of tracked `git diff --name-only --diff-filter=ADMR --find-renames` results and `git ls-files --others --exclude-standard`. It MUST include new/untracked, modified, deleted, and renamed files; a rename SHALL emit only its new path. Results MUST be lexicographically sorted, de-duplicated canonical repository-relative paths. `.haro/**` MUST be excluded, and any `../` escape MUST fail closed.

#### Scenario: Real Git change table
- GIVEN temporary Git repositories with base commits and final new, modified, deleted, renamed, and untracked states
- WHEN each report is compared with real `git status` and `git diff --name-status --find-renames`
- THEN paths match the Git oracle, with only each rename destination reported

#### Scenario: Filtering and canonicalization
- GIVEN duplicate, `.haro/**`, or repository-escaping candidates
- WHEN changed files are normalized
- THEN duplicates and `.haro/**` are absent and an escape is rejected

### Requirement: Workspace-mode report source [F-01, F-03; Constitution IX.2, IX.5.1]

Shared mode MUST diff at repository root. Isolated mode MUST use an existing `workspace_root`; after terminal removal it SHALL fall back to a repository-root diff from `base_commit`, and SHALL scan untracked files only while the workspace exists. Report behavior MUST NOT change worktree-removal timing.

#### Scenario: Isolated workspace availability
- GIVEN isolated terminal executions with an existing or removed workspace
- WHEN each report is requested
- THEN the existing workspace is inspected and the removed workspace uses the committed repo-root fallback without untracked scanning

### Requirement: CLI report surface [F-01; Technical Specification §6 `execution.report`]

`haro report <execution_id> [--json]` MUST delegate to the execution report. Plain output SHALL contain one changed path per line; JSON MUST be `{execution_id, base_commit, changed_files}`. The engine operation SHALL remain suitable for a future thin `execution.report` wrapper.

#### Scenario: Plain and JSON output
- GIVEN a terminal execution with changed files
- WHEN report is requested in plain and JSON modes
- THEN both expose the same ordered paths and JSON includes the execution ID and nullable base commit

### Requirement: Strictly read-only reporting [F-01–F-03, U-01; Constitution I.6]

Reporting MUST use only read-only Git inspection, MUST NOT mutate repository state, and MUST preserve pure Go/no-cgo operation without new dependencies; use of the Git binary is permitted.

#### Scenario: Repository remains unchanged
- GIVEN any reportable execution
- WHEN reporting succeeds or fails
- THEN no commit, push, merge, reset, publish, integration, or discard action occurs

## Non-Requirements

- No content diffs, line statistics, per-step reports, persisted `changed_files`, or Approach 2 snapshot storage; persistence MAY be a future additive change.
- No broker, UDS, JSON-RPC transport, PTY, leases/interactions, other v2-store DDL, `agents_command`/`agentsFile`, new dependency, or worktree-lifetime change.
