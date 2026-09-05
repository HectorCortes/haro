# Exploration: v2-path-claims (path_claims and workspace isolation)

## Current State

The system today has **no workspace isolation or path claim enforcement**. All executions run in the project root (`e.root`) and the engine hardcodes isolation:

- `internal/execution/engine.go:65,93` — `workspaceRoot := e.root`, `WorkspaceMode: "isolated"` hardcoded per execution and per step. No worktree creation, no `workspace_root` derivation, no `CommandRunner` cwd switching. `engine_transport_test.go` confirms tests inject isolated directly.
- `internal/workflow/parse.go` (50 lines) — `Workflow{Version, Name, Steps}` and `Step{ID,Type,Run,Env,TimeoutSeconds,DependsOn,Requires,Produces,Harness,Instructions,Mode}`. No `workspace`, `on_logical_conflict`, `inputs`/`outputs`, `source`/`bindings` fields.
- `internal/workflow/validate.go` (126 lines) — validates duplicate IDs, missing deps, cycles, entry point, step type, harness/instructions/mode for agents, run for commands. No workspace-mode validation, no `on_logical_conflict` values, no inheritance semantics.
- `internal/store/migrations.go` — 7 tables + `attempt_transport` (added by v2-adapter, sole owner). Missing per tech-spec §2: `leases`, `interactions`, `path_claims` (11-table reference). `Store` interface in `store.go` exposes `Projects, Executions, Steps, Attempts, Generations, Events, Transport` plus `WithTx`; no `PathClaims`, `Leases`, `Interactions`.
- `docs/v2/haro-especificacion-tecnica.md` §1 — canonical YAML schema already defines `workspaceConfig{mode: isolated|shared, on_logical_conflict: block|allow}` at workflow level and per step, plus §2 DDL `path_claims(id, project_id, logical_path, mode, owner_execution_id, owner_step_id, acquired_at, released_at)` and `idx_path_claims_active WHERE released_at IS NULL`, §5 `PathClaimRepository{Acquire, Release, ListActive}`. Code is behind spec.
- `docs/v2/haro-constitucion.md` §IX — normative: system default worktree per execution (IX.2.1), 3-level inheritance system→workflow→step (IX.2), claim registry `(project_id, logical_path, mode, owner, acquired_at)` with `isolated:<execution_id>|shared` (IX.3.3), prefix blocking, release on step finish (IX.3.5), behavior matrix (IX.4).
- `openspec/changes/archive/2026-08-30-v2-adapter/archive-report.md` — explicitly deferred `leases`, `interactions`, `path_claims` as owned by later changes (`v2-store`/`v2-broker`). `v2-adapter` took narrow ownership of `attempt_transport` via `CREATE TABLE IF NOT EXISTS` to avoid coupling. `v2-no-regresion` preserved `ValidateContainedPath` for `requires`/`produces` containment only.
- `internal/workflow/path.go` — `ValidateContainedPath(root, rel)` exists for artifact containment (EvalSymlinks walk, traversal rejection) but is **not** the claim canonicalization path. No `CanonicalizeLogicalPath(repoRoot, raw)` yet.
- Git invocation today — `internal/execution/runner.go:RealRunner.Run` via `exec.CommandContext` with explicit `argv[0]` (no shell), 300s ceiling, `cwd` param. No `git worktree` usage anywhere (`grep -r worktree` empty outside docs). `internal/cmd/execute.go` — `handleRun` creates execution directly, `handleStepRun` delegates to `Engine.RunStep` with no claim acquisition.

### What must be created vs extended

| Area | Status | Action |
|------|--------|--------|
| `path_claims` table + `PathClaimRepository` | Missing | Create (sole owner — see §Ownership) |
| `leases` + `interactions` tables | Missing | NOT in this change — owned by `v2-store`/`v2-broker` |
| `workspace` YAML fields + inheritance | Missing | Extend `parse.go`/`validate.go` |
| `CanonicalizeLogicalPath` + prefix checks | Missing | Create `internal/claim/` or `internal/workflow/canonical.go` |
| Worktree lifecycle (`git worktree add/remove`) | Missing | Create `internal/worktree/` |
| `Engine.CreateExecution` worktree + `workspace_root` persistence | Stubbed | Extend |
| `Engine.RunStep` Acquire/Release around step lifecycle | Missing | Extend (must release on success AND failure, defer) |
| Project config external paths | Skeleton only | Extend `.haro/config.yaml` parsing (deferred classification) |

## Affected Areas

- `internal/workflow/parse.go` — add `Workspace *WorkspaceConfig` at workflow and step level; `WorkspaceConfig{Mode *string, OnLogicalConflict *string}` with yaml tags `workspace`; step needs `type==workflow` branching for `source`/`bindings` is out of scope but `workspace` field must coexist. Affects `Workflow` struct, `Step` struct.
- `internal/workflow/validate.go` — add workspace mode validation (`isolated|shared`), `on_logical_conflict` (`block|allow`, default `block`), inheritance check `system→workflow→step`, unknown field rejection if strict zod parity is required. ~30 lines.
- `internal/store/migrations.go` — add `path_claims` DDL idempotently (`CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS idx_path_claims_active`). Sole migration owner for this table (mirrors `attempt_transport` precedent). Must document overlap with `v2-store`'s promised 11-table DDL.
- `internal/store/store.go` — add `PathClaim struct` and `PathClaimRepository` interface, extend `Store` facade with `PathClaims() PathClaimRepository`, add `query` helper usage. ~40 lines + repo impl.
- `internal/store/claim.go` (new) — `Acquired bool, Conflict *PathClaim` atomic implementation via `INSERT ... ON CONFLICT` or `INSERT OR FAIL` + error mapping; `Release` ownership check (`owner_execution_id + owner_step_id` equality, no cross-owner release); `ListActive`. ~120 lines.
- `internal/claim/canonical.go` or `internal/workflow/canonical.go` (new) — `Canonicalize(repoRoot, raw string) (string, error)` handling `..`, absolutes, symlinks, prefix boundary (`src/` vs `src/foobar/`), evaluated against repo root. ~80 lines, shares logic with `ValidateContainedPath` but returns canonical logical path not just error.
- `internal/claim/policy.go` (new) — `ShouldBlock(existingMode, incomingMode string, conflictPolicy string, isExternal bool) bool` encoding the 4-row matrix (isolated/isolated allow, shared/shared block, isolated/shared gated by `on_logical_conflict`, external always shared). ~40 lines.
- `internal/worktree/manager.go` (new) — `Create(executionID, repoRoot) (worktreePath string, err)`, `Remove(worktreePath string) error`, `List()`, `Prune()` for leak recovery. Uses `RealRunner`-style `exec.CommandContext("git", "worktree", ...)` pattern. ~100 lines.
- `internal/execution/engine.go` — `CreateExecution` must resolve `WorkspaceMode`/`WorkspaceConfig` inheritance, create worktree when isolated, set `exec.WorkspaceRoot` to worktree path else `e.root`, persist `step.WorkspaceMode` per step, handle cleanup on failure. `RunStep` must `Canonicalize` each `produces`/`requires` (or explicit claims list — spec implies claims on logical paths touched), `Acquire` before `pending→running`, block polling or error if denied, `defer Release` tied to step finish (success + failure paths including `failAttempt`). Must integrate with `WithTx` for atomic attempt creation (reuse pattern from `runAgentStep`). ~150 lines changed.
- `internal/execution/engine_test.go`, `internal/store/*_test.go` — table-driven matrix, concurrency tests, canonicalization tables.
- `internal/project/config.go` (new or extended) — parse `.haro/config.yaml` for `system.workspace` defaults and `external_paths` list for F-07. Currently `init.go` only writes `version: 2`.
- `internal/cmd/execute.go` — `handleRun` may need `--workspace` override forwarding; `handleStepRun` must surface `blocked` error code when claim denied (retryable). `main.go` unchanged.
- `docs/v2/haro-especificacion-tecnica.md` — §7.2 style note for path_claims sole ownership (if not already explicit); `internal/store` interface doc.

## Approaches

### 1. Sole ownership of `path_claims` by v2-path-claims (narrow vertical slice)

Create `path_claims` table and `PathClaimRepository` exclusively in this change, via idempotent `CREATE TABLE IF NOT EXISTS` + `CREATE INDEX IF NOT EXISTS`, documented as sole owner mirroring `v2-adapter`'s `attempt_transport` precedent. `v2-store` will NOT recreate `path_claims`; its F-02 verification will assert the table exists and matches reference DDL regardless of who migrated it. `leases`/`interactions` remain owned by `v2-store`/`v2-broker`.

- Pros: Unblocks U-02 atomic Acquire immediately (cannot test concurrency without a real table); preserves incremental SDD vertical slices; avoids circular dependency `v2-path-claims → v2-store`; proven pattern (attempt_transport migration succeeded, 3283 lines delivered, single owner, no divergence); keeps change small and auditable.
- Cons: Splits 11-table DDL across two changes; requires explicit contract in archive reports to prevent `v2-store` from re-creating the table with divergent SQL; `v2-store` must use `IF NOT EXISTS` defensively.
- Effort: Low

### 2. Defer `path_claims` DDL to v2-store, mock it in v2-path-claims

Implement an in-memory fake `PathClaimRepository` for U-02/U-03 only, asserting interfaces but not persisting to SQLite.

- Pros: Keeps DDL ownership pure (single 11-table migration in `v2-store`); slightly smaller diff now.
- Cons: U-02 requires testing atomic `INSERT ON CONFLICT` against `modernc.org/sqlite` with concurrent workers — fake cannot prove atomicity; forces `v2-store` to land before `v2-path-claims` can be verified, violating the tracking order (D04 before D07) and creating a blocking dependency; fake vs real behavior divergence risk for prefix checks; violates constitution X.3.1 stability (logical path identity) which deserves a real schema early.
- Effort: Medium (fake + later rewrite to real)

### 3. Full 11-table ownership in v2-path-claims

Take all missing tables (`leases`, `interactions`, `path_claims`) now.

- Pros: Single migration lands the complete §2 DDL; no coordination needed later.
- Cons: Expands scope 3x (lease fencing, interaction CAS are separate specs with their own invariants); duplicates `v2-store`/`v2-broker` charter; review budget risk (v2-adapter already hit 3283 lines, `size:exception`); violates SDD principle of narrow vertical slice.
- Effort: High

### 4. Worktree lifecycle location

Two sub-options evaluated:

- **A: Create worktree in `CreateExecution`, remove on execution terminal.** — Aligns with F-01 (`workspace_root` is the worktree for the whole execution). Simple: one `git worktree add` per execution. Cleanup in `resyncExecution` when status `completed|failed`, plus orphan prune on startup.
- **B: Create lazily on first `RunStep`** — saves worktree for executions that never run, but F-01 verification (`git worktree list` shows new worktree per execution immediately after `run`) expects A.

**Recommendation: A.** F-01 is per-execution, not per-step. Laziness would make test harness timing-sensitive.

### 5. Release lifecycle

- **Option R1: `defer Release` inside `RunStep` after Acquire** — covers success, failure, panic, context cancellation. Pair with `steps.next` blocking check that queries `ListActive` before presenting a step as runnable; broker variant would poll/wait instead of returning error.
- **Option R2: Release in separate `CompleteExecution` step** — delays release until whole execution ends, violates F-04 (release on step finish, not execution end) and would deadlock shared/shared concurrency.

**Recommendation: R1** — mandatory `defer` in both `RunStep` and `runAgentStep` paths.

## Recommendation

**Adopt Approach 1 (sole `path_claims` ownership) + Worktree A + Release R1.**

Justification:

- Constitution §IX.3 already defines the claim tuple and behavior matrix; tech-spec §2 §5 normatively define `path_claims` DDL and `PathClaimRepository`. The table is not speculative — it is specified and already excluded from `v2-adapter` precisely because it belongs to a dedicated change. Taking it now is consistent with precedent: `v2-adapter` took `attempt_transport` with `IF NOT EXISTS` and was archived successfully; the report notes overlap as `MR-SHARE-1` non-destructive overlap and requires downstream to not diverge.
- U-02 cannot be verified without a real SQLite table; `modernc.org/sqlite` serializes via `sql.DB` with `cache=shared` but still honors UNIQUE/ON CONFLICT. A fake would hide the exact property under test (one winner). The spec says "INSERT ... ON CONFLICT" — this must be exercised with `*sql.DB` + goroutines (`t.TempDir()` file, not `:memory:`) and `-race`.
- Keeping `leases`/`interactions` out preserves the `v2-store` boundary (F-01 no direct access, F-02 complete DDL, WAL+foreign_keys, interchangeable backend U-01). The only overlap to document is one table, not three.
- Worktree per execution matches F-01 wording verbatim ("effective `workspace_root` is that worktree") and the constitution IX.2.1; worktree naming should be deterministic and collision-free (e.g., `.haro/worktrees/<executionID>` or `/tmp/haro-worktree-<hash>` if repo-local path pollutes git). `git worktree add --detach <path> HEAD` is the minimal invocation, via `CommandRunner` or direct `exec.CommandContext` in `worktree.Manager` with `cwd = repoRoot`. Cleanup must be idempotent: `git worktree remove --force <path>` on execution completion, plus `git worktree prune` during `Init`/`Open` for orphans. Interaction with `CommandRunner cwd`: `Run` should use the execution's `WorkspaceRoot` (derived, not `e.root`) for command steps, and `SessionBundle.WorkspaceRoot` for agent steps.
- Canonicalization must reuse and extend `ValidateContainedPath` logic: `filepath.Clean`, reject absolutes, lexical `..` check, then `EvalSymlinks` on the joined repo-root path, then prefix comparison with boundary guard (`strings.HasPrefix(candidate, claimed+"/") || candidate == claimed` after ensuring both are clean canonical). Edge: `src` claiming must block `src/foo.ts` but not `srcfoo.ts` nor `src/foobar/`. Symlink escapes must be rejected, not silently resolved inside.
- Concurrency test harness for U-02: file-backed SQLite (`t.TempDir()/claims.db`, `store.Open`), same `projectID = repoRoot` (canonical, symlink-resolved), same `logical_path = "src/a"`, `mode = "shared"` for both workers, goroutine count 8–16, `sync.WaitGroup` + `errCh`, assert exactly 1 `Acquired=true`. Use `WithTx` + `INSERT` with unique partial index trick if needed; simplest is SQLite `UNIQUE(project_id, logical_path) WHERE released_at IS NULL` semantics emulated by the repo (since SQLite partial indexes cannot be created via `IF NOT EXISTS` without migration complexity — design choice: either enforce uniqueness via app-level check inside transaction or create partial index explicitly). The tech-spec DDL uses a partial index; production `Acquire` should `SELECT FOR` active prefix conflicts first, then `INSERT` inside the same `Tx`, relying on SQLite's `BEGIN IMMEDIATE` to serialize (modernc honors this per connection; file-backed with `cache=shared` is sufficient for unit tests). Document this as a transaction-isolation decision.
- YAML schema extension: define `type WorkspaceConfig struct { Mode *string `yaml:"mode"`; OnLogicalConflict *string `yaml:"on_logical_conflict"` }` on both `Workflow` and `Step`. Validation resolves inheritance: `system` default `isolated` (`internal/project/config.go` will later allow override; for now default isolated), workflow overrides system, step overrides workflow. When `OnLogicalConflict` is absent it defaults to `block` only when the effective conflict is `isolated/shared` (not for same-mode). Reject `on_logical_conflict` outside `block|allow`, reject `mode` outside `isolated|shared`.
- External resources F-07: requires project config to list external logical prefixes (e.g., `external_paths: ["cache/", ".tmp/"]`). Since `.haro/config.yaml` is currently skeletal, this change should add minimal parsing (yaml.v3 already a dep, no new dependency) and rule "if canonical logical_path has prefix in external list → treat mode as `shared` regardless of step's effective mode." Keep list empty by default; test via table-driven `isExternal(canonical, list)`.
- Scope boundaries to enforce:
  - `v2-store`: owns `leases`, `interactions`, full 11-table `TestMigrations` final assertion, interchangeable backend U-01, WAL/foreign_keys, idempotent schema, enum CHECKs. Must not touch `path_claims` DDL after this change (use `IF NOT EXISTS`).
  - `v2-broker`: owns daemon/UDS lifecycle, fencing, `leases` runtime.
  - `v2-composicion`: owns `type: workflow` flattening, namespacing, `inputs/outputs/bindings`, static cycle over file graph.
  - `v2-path-claims` owns ONLY: `path_claims` table/repo, claim canonicalization + prefix policy, Acquire/Release, behavior matrix tests, workspace isolation (worktree + YAML inheritance), external path classification. No broker runtime change except claim gating in `Engine.RunStep` / `Steps.Next` filtering.

## Risks

- **Worktree leak on crash / CI hermeticity** — `git worktree` state is filesystem + `.git/worktrees/`. If the process dies before `Remove`, orphans remain. Mitigation: use `t.TempDir()` worktrees in tests (`git worktree add <tmpDir>/wt HEAD`), defer cleanup in tests, and implement startup prune (`git worktree prune` + remove directories not in `executions` table with `status in (running,pending)`). CI must have `git` available; skip `F-01` worktree E2E under `testing.Short()` or when `exec.LookPath("git")` fails, but keep unit coverage for `WorktreeManager` via `FakeRunner`.
- **Symlink escape in canonicalization (CRITICAL)** — Incorrect `EvalSymlinks` handling can let `src/link -> /etc` bypass prefix check. Must anchor both `repoRoot` and `candidate` via `EvalSymlinks(repoRoot)` then `EvalSymlinks(repoRoot + "/" + logical)`, reject if resolved target does not have prefix `evalRoot + "/"` or `== evalRoot`. Test matrix must include: `src/foo` vs `src/foobar` (no collision), `src/` vs `src/foo.ts` (collision), symlink inside repo pointing outside, symlink to sibling, `a/../b` normalization, absolute path rejection. This is the highest security-adjacent risk.
- **Deadlock / blocking semantics (F-04 vs broker concurrency)** — CLI-direct mode has no broker to block/wait. Spec F-04 says second step "does not start until first claim released." Without broker, `RunStep` must either return a structured `blocked{owner}` error (client retries poll) or block internally with `time.Wait` + backoff. The E2E verification expects blocking, which implies broker or polling in `steps next` filtering. Clarify in proposal: for CLI-direct, `step.run` returns `code: logical_conflict` with owner, and `steps next` filters blocked steps; broker later will implement waiting queue. Without this, E2E will flake.
- **Migration ownership divergence** — If `v2-store` later runs a non-`IF NOT EXISTS` DDL that redefines `path_claims` differently, tests break. Must document in both changes that `path_claims` is `IF NOT EXISTS` and any DDL evolution requires a coordinated migration file, not table recreation.
- **SQLite concurrency fidelity** — `modernc.org/sqlite` with `file:?cache=shared` does serialize via connections, but `ON CONFLICT` behavior depends on whether the constraint is a UNIQUE index or application check. A race-free `Acquire` must be a single transaction with `BEGIN IMMEDIATE`; testing with `:memory:` hides this (different journal mode). Use file-backed DB in U-02.
- **External resource classification ambiguity** — Without a concrete config schema, F-07 is untestable. Need a minimal schema proposal: `.haro/config.yaml` gains `external_paths: []string` (relative logical prefixes). Until `v2-store` lands full config, F-07 tests should inject via `t.TempDir` config file.
- **Workspace inheritance default divergence** — Constitution says system default is worktree per execution (isolated). If workflow omits `workspace`, current code already defaults isolated. Must not silently change to shared; apply must preserve existing `workspace_mode` column CHECK `isolated|shared`.
- **No cgo / git availability gate** — Worktree via `git` binary is explicitly allowed (no cgo, no new Go dependency). Tests invoking real `git` must guard with `testing.Short()` or `git --version` check; `FakeRunner`/`FakeWorktreeManager` for unit.

## Ready for Proposal

Yes — exploration is complete and self-contained. No additional spikes needed. The next step is `sdd-propose` to formalize scope (`path_claims` sole ownership, worktree per execution, YAML inheritance, acquire/release lifecycle), non-goals (leases/interactions/composition/reporting/broker daemon), and a rollback plan (drop `path_claims` table + revert worktree creation; idempotent migration allows safe re-run).

## Appendix: Verification mapping for proposal

| Spec criterion | Evidence shape | Layer |
|---|---|---|
| F-01 worktree per execution | `git worktree list` shows worktree, `workspace_root` equals worktree, command output inside worktree | E2E (`git` gate) + unit via FakeWorktree |
| F-02 3-level inheritance | Three YAML fixtures → resolved `workspace_root` assertion | E2E/INT via Engine |
| F-03 prefix identity | `src/` blocks `src/foo.ts`, not `src/foobar/` | E2E + unit policy table |
| F-04 shared blocks until release | slow A blocks B, release on success + failure both unblock | E2E with goroutines + Store |
| F-05 isolated/isolated allow | both start concurrently | E2E concurrent RunStep |
| F-06 on_logical_conflict block/allow | workflow `allow` fixture starts, default `block` does not | E2E + unit matrix |
| F-07 external always shared | external prefix list → 2 isolated claims still block | E2E + unit isExternal table |
| U-01 canonicalization | edge table (symlink, `..`, absolute, prefix boundary) | UNIT table-driven `Canonicalize` |
| U-02 atomic Acquire | workers+file DB → one winner | UNIT concurrency with `-race` |
| U-03 behavior matrix + wrong owner reject | 4-row table + Release wrong owner error | UNIT table + owner check |

