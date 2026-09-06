# Delta Specification: v2-composicion

## ADDED Requirements

### Requirement: Planning-time flattened execution [D05 F-01, F-02; Constitution X.1.1–X.1.2, X.2]

Before execution creation, the system MUST recursively replace `workflow` nodes, persist executable steps under one `execution_id`, namespace IDs as `<node>.<internal>`, and keep repeated inclusions independent.

#### Scenario: Reused workflow
- GIVEN `a` and `b` include one file containing `x` and `y`
- WHEN the root workflow is planned
- THEN one execution contains all four namespaced IDs and no child/runtime workflow node

### Requirement: Explicit contracts and bindings [D05 F-03, F-04, U-03; Constitution X.3; Technical Specification §1]

Only included files MAY declare contracts; root `inputs`/`outputs` MUST fail `root_contract_forbidden`. Parents MUST wire contract names only. Declarations targeting non-empty `requires`/`produces` MUST have bindings that rewrite dependencies and artifacts.

#### Scenario: Bound artifacts
- GIVEN valid contracts and complete bindings
- WHEN composition flattens the workflow
- THEN parent artifacts feed consumers and internal products feed the parent

#### Scenario: Contract rejection table
- GIVEN root contracts, direct internal references, unknown/missing bindings, or unsatisfied internal mappings
- WHEN validated
- THEN mappings are `root_contract_forbidden@inputs|outputs`, `contract_violation@steps[i].depends_on[j]|requires[j]`, `unknown_input|unknown_output|missing_binding@steps[i].bindings.<name>`, and `contract_violation@inputs[i].satisfied_by|outputs[i].produced_by`

### Requirement: Contained, bounded, acyclic inclusion [D05 F-05, U-02; Constitution I.3, X.5; Technical Specification §1 `source`]

`source` MUST be including-file-relative and its realpath MUST remain within `root/.haro/workflows`; sibling/library paths inside MAY resolve. Absolutes, escapes, realpath cycles, depth >16, expansions >256, or flattened steps >256 MUST fail before execution/worktree creation.

#### Scenario: Inclusion safety table
- GIVEN valid siblings, absolute/escaping paths, direct/transitive/self cycles including different node names for one file, and exceeded guards
- WHEN planned
- THEN siblings flatten and invalid rows return stable codes with exact `steps[i].source` or guard fields

### Requirement: Boundary-independent cascade [D05 F-06; Constitution X.4]

Reopening MUST cascade over flattened `produces→requires`, independently of public outputs, invalidating only transitive feeders.

#### Scenario: Fine-grained invalidation
- GIVEN only one of two internal producers feeds a reopened step
- WHEN that step is reopened
- THEN only its producer chain is invalidated

### Requirement: DAG parallelism [D05 F-07; Constitution X.1.3]

Flattened steps MUST use the existing `depends_on` scheduler; artifacts MUST NOT become ordering edges.

#### Scenario: Independent inclusions
- GIVEN two workflow nodes without dependencies
- WHEN querying `steps next`
- THEN ready internal steps from both namespaces are returned

### Requirement: Pure canonical DAG and persisted hash [D05 U-01; Constitution I.1–I.2; Technical Specification §§2–3]

Flattening MUST be pure and stably topological. Its canonical hash MUST persist in nullable, idempotently migrated `executions.dag_hash`.

#### Scenario: Repeatability
- GIVEN identical nested YAML bytes
- WHEN flattened twice
- THEN both structures, orders, and DAG hashes are identical

### Requirement: Zod-equivalent strict validation [D05 U-04; Constitution I.3; Technical Specification §1]

The Go validator MUST mirror zod-equivalent strictness and return stable codes with exact fields. U-04 tables MUST cover: `additionalProperties:false`; version; object/array/string/integer types; enums/patterns; non-empty steps; command `run`/`env`/timeout; agent `harness`/`instructions`/`mode`/timeout; workflow `source`/`bindings`; root contracts; included targets; binding completeness/unknown keys; source containment; workspace shape.

#### Scenario: Schema parity table
- GIVEN invalid rows for each rule and valid rows for each step type
- WHEN Go validation executes
- THEN invalid rows match zod-equivalent code and field while valid rows pass
- AND composition errors create no execution/worktree

## MODIFIED Requirements

### Requirement: Three-level inheritance [v2-path-claims/F-02; D05 F-01; Constitution IX.2, X.1.2]

Workspace MUST resolve `system → root workflow → flattened step`; included workspace MUST be shape-valid but MUST NOT propagate, while step overrides MUST persist. Composition MUST retain one worktree per execution, claim acquisition, and `steps next` filtering of `logical_conflict`.
(Previously: inheritance did not distinguish root from included workflow configuration.)

#### Scenario: Composed inheritance fixtures
- GIVEN root-shared/root-isolated modes, included defaults, and internal overrides
- WHEN each fixture resolves
- THEN root policy governs except step overrides, one execution worktree exists, and claims remain unchanged

## Non-Requirements

This change MUST NOT add broker/UDS/JSON-RPC queues, PTY, reporting, distribution, leases/interactions, path-claim/worktree mechanics, authentication policy, `agents_command`/`agentsFile`, `artifacts_dir`, or context transports (v1 Spec 10).
