# Haro — v2 Project Constitution

> Normative manifesto. It contains only business rules and technical definitions already decided — no research, no option comparison, no justification. It is the canonical reference against which any subsequent design or implementation decision is validated. Any new rule added in the future must be consistent with it; if a future decision contradicts something here, this document is updated explicitly, never silently ignored.

---

## I. Non-negotiable objectives

These objectives are fixed. Any concrete business rule can be redesigned; these objectives, not:

- **I.1** Total, auditable traceability of every execution.
- **I.2** Reproducibility: given the same context, the same attempt must be reconstructable.
- **I.3** Systematic fail-closed: on ambiguity, unknown capability or unrecognizable state, the system fails explicitly; it never assumes or guesses.
- **I.4** No mechanism may allow bypassing the permission policy.
- **I.5** Methodological neutrality: the core never decides branches, conditionals or the quality of a harness's work. That responsibility belongs exclusively to the external orchestrator agent and to the workflow's own instructions.
- **I.6** Haro coordinates and reports; it **never** integrates, publishes or discards work on its own. Any final integration action is the responsibility of whoever uses Haro.

## II. Harnesses

- **II.1** Initial target harnesses: OpenCode, Claude Code.
- **II.2** Architecture designed to incorporate, without modifying the core: Codex, Gemini CLI, GitHub Copilot CLI, Pi, and any other future agent CLI.
- **II.3** The core never knows provider-specific literals, endpoints, flags, event formats or field names. Any provider detail lives exclusively inside its adapter.
- **II.4** No data migration or compatibility is required with Shardeo v1's database or command surface.

## III. Distribution and environment

- **III.1** The distribution model is not tied to any particular ecosystem; it may change with respect to v1.
- **III.2** Local tool, single project/checkout, with no network infrastructure or own server except the local bridge to the harness.
- **III.3** Designed for a single user in the current state, without closing the door to a future expansion to multi-user concurrency. This translates to: the persistence layer must sit behind a repository interface, not direct SQLite access.

## IV. Domain model

### IV.1 Vocabulary

| Term | Definition |
|---|---|
| **Workflow** | Declarative YAML definition of a DAG of steps. |
| **Run / Execution** | A concrete execution instance of a workflow. It has a single `execution_id`. |
| **Step** | DAG node. Type: `command`, `agent`, or `workflow` (IV.4). |
| **Attempt** | A concrete attempt to run a step. There may be multiple attempts per step (retries, fallback between candidates). |
| **Generation** | Version of the artifacts produced by a step. Reopening a step invalidates its generation and that of everything depending on it (cascade). |
| **Harness** | External program (OpenCode, Claude Code, etc.) that does the actual work of an `agent`-type step. |
| **Adapter** | Component that translates Haro's internal contract into a specific harness's protocol. |
| **Broker** | Per-project daemon that manages active sessions and bridges the CLI and the adapters. |
| **path_claims** | Registry of write claims per project (IX). |

### IV.2 `step.mode` is not a closed enum

`headless` / `supervised` / `terminal` are not three separate implementations: they are convenience profiles derived from the **capabilities negotiated** with the adapter at `initialize` time. The effective mode of a step derives from which capabilities the adapter declared, not from an isolated choice of the system.

### IV.3 `Attempt` is transport-agnostic

`Attempt` stores only state, timings, result and evidence digest — nothing provider-specific. Any transport detail (native session identity, protocol-specific fields) lives in a per-adapter extension table, never in the `attempts` table.

### IV.4 `workflow` node type

A step can be of type `workflow`, in which case it references another YAML file instead of running a `command` or an `agent` directly. See Section X for its full behavior.

## V. Adapter (harness) contract

### V.1 Mandatory methods (every adapter must implement them)

```
Probe(ctx) → (available, version, declared capabilities)
Initialize(ctx, coreCapabilities) → (protocolVersion, harnessCapabilities)
NewSession(ctx, bundle) → SessionHandle
Session.Prompt(ctx, context) → event stream
Session.Cancel(ctx)
```

### V.2 Optional methods (gated by capability)

```
Session.RequestPermission(...)   — the harness calls TOWARD the core, not the other way around
Session.Terminal*(...)
Session.LoadPrevious(ctx, id)
```

### V.3 Capability rule

- **V.3.1** No method outside V.1 is mandatory.
- **V.3.2** The core never invokes an optional method without having previously verified, in `Initialize`, that the adapter declared it supported.
- **V.3.3** New capabilities are added additively; they never require incrementing the protocol major version.
- **V.3.4** A protocol major version increment only happens on an incompatible change to the mandatory methods (V.1).

### V.4 Responsibility boundary

- **V.4.1** The core is responsible for: step/attempt lifecycle, permission policies, persistence, invalidation cascade.
- **V.4.2** The adapter is responsible for: translating the harness's specific protocol to the V.1/V.2 contract.
- **V.4.3** The harness (external process) is responsible for: all reasoning, implementation decisions, and interpretation of instructions. Haro never interprets or evaluates that reasoning.

## VI. Execution lifecycle

```
pending → running → completed
                  ↘ failed → running (re-execution)
completed/failed → pending (reopen, invalidates generation)
pending → skipped (only if the work has not run yet)
```

- **VI.1** Only the holder of a step's current lease may write its result.
- **VI.2** An expired lease is invalidated by fencing token; no write after expiration is valid even if the original process is still alive.
- **VI.3** Fallback between harness/model candidates does not semantically classify errors; it is an agnostic "try the next available candidate" mechanism.

## VII. Communication (IPC)

- **VII.1** CLI ↔ Broker: JSON-RPC 2.0 over Unix socket (or named pipe on Windows). One broker per project, identified by the project path. Lazy startup: if the socket does not respond, the CLI starts the broker and retries.
- **VII.2** A broker can hold multiple concurrent active sessions (multiple parallel executions of the same or different workflow, within the same project).
- **VII.3** Different projects have completely independent brokers, with no coordination between them.
- **VII.4** Broker ↔ Adapter ↔ Harness: the adapter chooses the native transport its harness supports — for example, stdio-RPC, local HTTP + SSE or JSONL — declares it in capabilities and negotiates it in `initialize`; the core neither imposes nor ranks a transport. (Non-normative example: OpenCode supervised uses managed local HTTP + SSE.)

## VIII. Event model and approvals

- **VIII.1** Events are identified by a monotonic cursor per session/attempt.
- **VIII.2** No event carries a harness's full output; only references/deltas.
- **VIII.3** Interaction resolution (approval, answer to a question) is idempotent: re-sending the same resolution produces no duplicate effects.
- **VIII.4** Permission requests are originated by the harness calling toward the core (via its adapter), never the other way around.
- **VIII.5** The core applies the permission policy fail-closed: any unrecognizable action type, capability or decision is rejected by default.

## IX. Write coordination and workspace isolation

### IX.1 Principle

Haro coordinates to prevent two concurrent writers from colliding on the same resource, and reports what changed when finished. It never merges, publishes or discards work on its own (see I.6).

### IX.2 Physical isolation configuration, in three levels

```
system   → default: worktree per workflow
  └─ workflow → may set isolation for all its steps
       └─ step → may override the workflow level
```

- **IX.2.1** System default: each workflow execution runs in its own `git worktree`.
- **IX.2.2** A workflow may disable isolation (`shared`) for all its steps.
- **IX.2.3** An individual step may override the configuration inherited from its workflow.

### IX.3 Claim registry (`path_claims`)

- **IX.3.1** It is the only real non-collision guarantee mechanism; the physical isolation strategy (IX.2) only reduces when it needs to be invoked.
- **IX.3.2** Claims are made on the **logical identity** of a path within the repo (e.g. `src/foo.ts`), never on the physical path where it is actually written.
- **IX.3.3** Structure: `(project_id, logical_path, mode, owner, acquired_at)`, where `mode` is `isolated:<execution_id>` or `shared`.
- **IX.3.4** The logical path is canonicalized (resolved, symlink-free) and compared by prefix, so claiming a directory also blocks its children.
- **IX.3.5** A claim is released when the corresponding step finishes (success or failure), not when the whole execution ends.

### IX.4 Behavior matrix

| Case | Physical collision | Behavior |
|---|---|---|
| `isolated` vs `isolated`, same logical path | No | Allowed without blocking. Divergences, if any, are discovered when someone decides to integrate both worktrees — outside Haro's scope |
| `shared` vs `shared`, same logical path | Yes | The registry blocks: the second step does not start until the first claim is released |
| `isolated` vs `shared`, same logical path | Not physically, yes by intent | Governed by `on_logical_conflict`: `block` (default, fail-closed) or `allow` (explicit workflow opt-in) |
| Resources outside the repo (cache, test DB, unversioned artifacts) | N/A — always same path | Always treated as `shared`. The project config explicitly declares which paths are "of the repo" and which are "external" |

### IX.5 Change reporting

- **IX.5.1** When the top-level execution (the one invoked by the external orchestrator) finishes, Haro computes and shows which files changed relative to the starting point.
- **IX.5.2** The report is computed once, at the top-level execution level — never per internally nested workflow (see X).

## X. Workflow composition (nested workflows)

### X.1 Model

- **X.1.1** A `workflow`-type step references another YAML file.
- **X.1.2** Composition happens at **planning/compilation** time, not at runtime. There are no "child" executions: the result is a single flat DAG under a single `execution_id`.
- **X.1.3** Parallelization between included `workflow` nodes uses exactly the same DAG mechanism as any other step (nodes without `depends_on` between them run in parallel). Configuring it correctly is the user's responsibility.

### X.2 Namespacing

- **X.2.1** The internal steps of an included workflow are identified with the prefix of the name of the node that included it: `<node_name>.<internal_step>`.
- **X.2.2** This allows including the same workflow several times in the same parent, each instance with its own namespace, without id collisions.

### X.3 Explicit boundary

- **X.3.1** Every includable workflow declares its own `inputs`/`outputs` contract, mapped to concrete internal steps.
- **X.3.2** The workflow including another can only wire (`depends_on`, `requires`) against that declared contract, never directly against internal steps of the included workflow.

### X.4 The invalidation cascade ignores the composition boundary

- **X.4.1** The invalidation cascade when reopening a step operates on the real generation/artifact graph, already flattened — not on the declared `inputs`/`outputs` structure.
- **X.4.2** Reopening something invalidates only the internal steps of an included workflow whose `produces` actually feed what was reopened, with the same fine granularity already applied between normal steps — regardless of what was declared as public `outputs` in X.3.
- **X.4.3** The `inputs`/`outputs` contract (X.3) and the cascade behavior (X.4) are independent rules; one does not condition the other.

### X.5 Cycles

- **X.5.1** A workflow cannot include itself, directly or transitively.
- **X.5.2** Cycle detection is resolved statically, over the file-reference graph, before flattening. It requires no runtime mechanism.

## XI. Testing

- **XI.1** Every new adapter must pass a public-boundary contract test suite before being integrated: `step run → real harness subprocess → protocol → public JSON output → store → settle`.
- **XI.2** This suite must run against a real harness binary, not only against fixtures or mocks, except in cases where depending on a real binary in CI is not viable (for those, recorded fixtures are used).
- **XI.3** The contract test suite is a first-class repository artifact from the start, not a late addition.

## XII. Implemented state and deliberate deferrals

- **XII.1** `terminal`/PTY is already implemented by Spec 9c (`src/runtime/pty.ts`, `src/runtime/terminal.ts`, `src/runtime/attach.ts`; `SPECS.md` §9c) as an existing surface to preserve: real PTY, human attach and presence.
- **XII.2** The immutable bundle, its SHA-256 manifest and the admission test are already implemented by Spec 9a (`src/runtime/bundle.ts`, `src/schema/bundle.ts`, `src/runtime/admission.ts`) and fall under no-regression (`v2-no-regresion`); the interaction CAS remains deferred until its corresponding spec.
- **XII.3** Shared multi-user concurrency — not implemented now; the persistence interface (III.3) is designed not to block this evolution.
- **XII.4** Known debt: the OpenCode JSONL parser remains in `src/utils/agent.ts` (headless path) and `v2-adapter/F-04` must move it to the adapter.

## XIII. Stability rules (hard to change later, design well from the start)

- **XIII.1** The adapter contract (Section V) — changing it forces rewriting all existing adapters.
- **XIII.2** The format of persisted events (monotonic cursor, payload shape) — it is the basis of audit and resumption; changing it breaks compatibility with historical data.
- **XIII.3** The rule that the core never knows provider literals (II.3) — it is code discipline, not a feature; a single exception contaminates the rest of the system.
- **XIII.4** The logical path identity in `path_claims` (IX.3.2) — changing it forces reinterpreting the entire coordination history.
- **XIII.5** The explicit `inputs`/`outputs` contract of an included workflow (X.3) — changing it breaks the compatibility of any workflow that already includes another.