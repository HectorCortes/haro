# Proposal: v2-composicion (workflow composition)

## Intent

Deliver pending `deltas-acceptance.md` D05 (11 criteria): corrected **7×P0** (F-01,F-02,F-03,F-05,F-06,U-01,U-02) + **4×P1** (F-04,F-07,U-03,U-04), implementing constitution §X under sole ownership.

## Scope

### In Scope
- Own `type: workflow`, parent-relative `source`, `bindings`, and included-file `inputs/outputs`. Realpaths must remain within `root/.haro/workflows`; sibling/library directories inside it are allowed, while absolute paths and escapes fail.
- Root `inputs/outputs` fail as `root_contract_forbidden`. Every declared included input/output requires a binding; unknown, missing, or internally unsatisfied mappings fail closed with code and exact field.
- Before execution, recursively produce one namespaced DAG; detect realpath file cycles, rewire artifacts/dependencies, preserve parallelism, and cascade X.4 through flattened `produces→requires` edges.
- Root workflow workspace governs. Included workflow-level workspace is shape-validated but not propagated; internal step overrides remain, preserving `v2-path-claims` invariants.
- Fail before execution above depth 16, 256 file expansions, or 256 flattened steps.
- Persist nullable `executions.dag_hash` via idempotent migration, hashing canonical stable-topological DAG content.
- U-04 Go validation mirrors zod-equivalent `additionalProperties:false`, types, enums/patterns, non-empty steps, per-type conditionals, root prohibition, included targets, binding completeness/unknown keys, source containment, and workspace rules; tables assert exact field paths.

### Out of Scope
- Broker/UDS/JSON-RPC and waiting queues; PTY; `v2-reporte`; `v2-distribucion`; leases/interactions; `path_claims`/worktree mechanics; authentication/permission policy changes.
- `agents_command`/`agentsFile` sugar, `artifacts_dir` overrides, or new context transports.

## Capabilities

### New Capabilities
- `v2-composicion`: D05 contracts, flattening, hashing, cycles, and cascade.

### Modified Capabilities
- `v2-path-claims`: clarify root-governed inheritance without changing mechanics.

## Approach

Use Approach 1: before worktree/row creation, `CreateExecution` invokes pure `internal/workflow/compose.Flatten` (optionally shelled as `internal/planner`). Inject reads; canonicalize, validate, flatten, sort/hash, then atomically persist. Runtime workflow nodes are invalid.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/workflow/{parse,validate,compose,hash}` | New/Modified | Schema, validation, planning, hash |
| `internal/execution/{engine,state}` | Modified | Creation and cascade |
| `internal/store` migrations/repositories | Modified | `dag_hash` |
| `internal/cmd`, `testdata/compose` | Modified/New | Errors, fixtures |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| **CRITICAL** source symlink escape | High | Realpath then enforce workflows-root containment |
| Cycles; namespace/limit collisions | Med | Canonical keys, uniqueness checks, hard guards |
| Binding/cascade miswiring | High | Complete contracts and artifact-edge tables |
| Zod-fidelity or I/O regression | Med | Exact-field parity tables and injected reader |

## Rollback Plan

Revert additive files and engine/CLI wiring. Retain nullable `dag_hash` per `v2-path-claims` `archive-report.md:31`; no new table exists. In-flight namespaced executions become orphaned and must not resume.

## Dependencies

- Normative: `docs/reference/SPECS.md`, `docs/v2/haro-constitucion.md`, `docs/v2/haro-especificacion-tecnica.md`; analysis: this change's `exploration.md`.

## Success Criteria

- [ ] All 11 D05 criteria pass reproducibly, including twice-identical DAG/hash evidence and U-02–U-04 tables.
- [ ] Existing quality gates pass with no excluded-surface ownership leakage.
