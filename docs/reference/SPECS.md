# Shardeo — Functional Specifications of the MVP

---

## Orchestration principle

Shardeo executes and persists the transitions requested by an orchestrating agent, but does not interpret the semantic meaning of agent responses nor decide which path a workflow should follow.

The workflow's general instructions define how the orchestrator should conduct the process: when to ask the user for approval, how to interpret a step's outcome, and when to retry, reopen, or skip work. Each step's instructions describe only how the corresponding agent or command should execute its task.

The orchestrator's decisions materialize through explicit Shardeo commands so that each transition is validated, persisted, and traceable.

---

## Spec 1: Project initialization

status: done

### Objective

Let a user prepare any repository to use Shardeo, generating the directory structure and configuration files needed to define workflows.

### Functional description

The user installs Shardeo globally (`pnpm add -g shardeo`) and then runs `shardeo init` inside the root of a project. Shardeo generates the `.shardeo/` folder with the complete structure: `config.yaml`, and the `workflows/`, `skills/`, `artifacts/`, and `docs/` directories. If the structure already exists, Shardeo informs the user without overwriting anything.

### Acceptance criteria

- `shardeo init` creates the `.shardeo/` structure with all documented subdirectories (`workflows/`, `skills/`, `artifacts/`, `docs/`).
- A `config.yaml` with valid default values is generated.
- If `.shardeo/` already exists, the command exits without error and without modifying the existing content, reporting that the project is already initialized.
- The command fails with a clear message if run in a directory without write permissions.
- The command's output confirms the path of the generated structure.

### Example user flow

```
$ cd ~/projects/my-api
$ shardeo init
✓ Project initialized at /home/user/projects/my-api/.shardeo/
```

---

## Spec 2: Workflow discovery

status: done

### Objective

Let the orchestrator (or the user directly) explore which workflows are available in the project, understand their purpose, and know their structure before running them.

### Functional description

The user runs `shardeo workflows list` to get the list of workflows defined in `.shardeo/workflows/`, each with its name and description extracted from the `workflow.yaml`. For a specific workflow, they run `shardeo workflows describe <workflow>`, which returns the full detail: description, general instructions (content of the `instructions.md`), and the list of steps with their types, dependencies, and produced artifacts.

Both commands validate the YAML structure with Zod before presenting results. If a workflow has schema errors, it appears in the listing marked as invalid, with the specific error.

### Acceptance criteria

- `shardeo workflows list` shows the name and description of each workflow found in `.shardeo/workflows/`.
- If no workflows are defined, the command returns an informative message (not an error).
- `shardeo workflows describe <workflow>` shows: description, general instructions, and for each step: id, type, configured agents (if applicable), execution order (`depends_on`), input validations (`requires`), and output validations (`produces`).
- If the referenced workflow does not exist, the command fails with a clear message listing the available workflows.
- A workflow with malformed YAML or that fails schema validation appears in `list` marked as invalid, and `describe` shows the concrete validation error.
- The output is structured (JSON) for orchestrator consumption, with a readable format option for direct human use.

### Example user flow

```
$ shardeo workflows list
┌──────────────────┬──────────────────────────────────────────────┐
│ Workflow         │ Description                                  │
├──────────────────┼──────────────────────────────────────────────┤
│ backend-feature  │ Workflow to develop a backend feature, from  │
│                  │ design to tests.                             │
│ hotfix           │ Workflow to apply a critical fix.            │
└──────────────────┴──────────────────────────────────────────────┘

$ shardeo workflows describe backend-feature
# backend-feature
Workflow to develop a backend feature...

Steps:
  1. architecture (agent) → produces: architecture.md, adr.md
  2. api-design (agent) → depends_on: architecture → requires: architecture.md → produces: api-contract.md
  3. database-design (agent) → depends_on: architecture → requires: architecture.md → produces: database-design-report.md
  4. lint (command) → depends_on: api-design, database-design
  5. implementation (agent) → depends_on: lint → requires: api-contract.md, database-design-report.md → produces: implementation-report.md
```

---

## Spec 3: Running a workflow with command-type steps

status: done

### Objective

Allow the complete execution of a workflow that contains only `command`-type steps, validating the full lifecycle: start, execution order resolution (`depends_on`), artifact validation (`requires`/`produces`), and completion.

### Functional description

The user runs `shardeo run <workflow>`, which records the execution in the SQLite database and returns an `execution-id`. From there, the flow follows Shardeo's execution model: the orchestrator (or the user) queries `shardeo steps next <execution-id>` to get the executable steps, then runs each one with `shardeo step run <execution-id> <step-id>`.

For `command`-type steps, Shardeo executes the declared command and captures stdout/stderr and the exit code. If the command returns exit code 0, Shardeo validates that the artifacts declared in `produces` exist and marks the step as completed automatically. If the command fails or the artifacts were not generated, the step is marked as failed.

`shardeo status <execution-id>` shows the current state of the execution: which steps were completed, which failed, and which are pending.

### Acceptance criteria

- `shardeo run <workflow>` creates an execution record and returns a unique `execution-id`.
- `shardeo steps next <execution-id>` returns the steps whose predecessors (`depends_on`) are completed and that are not yet completed or in execution. Steps without `depends_on` appear as immediately available.
- If all pending steps have incomplete predecessors, it returns an empty list.
- `shardeo step run <execution-id> <step-id>` first validates that the artifacts in `requires` exist. If they are missing, the step is not executed and it returns an error with the list of missing artifacts.
- If the `requires` validation passes, it executes the command and captures its output and exit code.
- A `command`-type step with exit code 0 and `produces` artifacts generated correctly is marked as completed without orchestrator intervention.
- A step with a non-zero exit code is marked as failed, including stdout and stderr in the response.
- A step with exit code 0 but missing `produces` artifacts is marked as failed with a message indicating which artifacts were not found.
- `shardeo status <execution-id>` shows the state of each step (pending, completed, failed) and the overall workflow state.
- A step whose predecessors (`depends_on`) are not completed cannot be executed; the command returns an explicit error.

### Example user flow

```
$ shardeo run lint-only-workflow
Execution started: exec-a1b2c3

$ shardeo steps next exec-a1b2c3
Available steps:
  - lint (command)

$ shardeo step run exec-a1b2c3 lint
Running: npm run lint
Exit code: 0
Step completed.

$ shardeo status exec-a1b2c3
Workflow: lint-only-workflow
Status: completed
Steps:
  ✓ lint — completed
```

---

## Spec 4: Executing agent-type steps

status: implemented

### Objective

Let Shardeo execute steps that require an AI agent, resolving which CLI to use, injecting the necessary context, and returning the result to the orchestrator so it can decide whether the step is complete.

### Functional description

When the orchestrator runs `shardeo step run <execution-id> <step-id>` on an `agent`-type step, Shardeo re-reads and validates the complete workflow YAML before executing the step. If the format or any business rule is invalid, it returns the error without invoking any CLI or modifying the step state. This validation is repeated on every execution because the workflow can change between steps.

After validating the workflow, Shardeo:

1. Resolves candidates exclusively from `steps[].agents`, in declared order. In v1, the only valid identifier is `opencode`; the model is optional and opaque. An unavailable candidate is skipped without creating an attempt. After an invocation that started, was cleaned up, and produced valid JSONL, any clean error falls back to the next candidate **without semantic classification** of its cause.
2. Builds the step context: operational instructions (generated by Shardeo) + domain instructions (the step's `instructions.md`) + referenced skills + required artifacts.
3. Invokes the CLI in non-interactive mode and without permission-bypass flags (`--auto` is never injected). The model value is passed unmodified; the CLI is responsible for semantic validation.
4. Monitors the configurable inactivity timeout (default 5 minutes), which resets on any output event. If the timeout expires, the step terminates with `permission_timeout`.
5. Detects approval requests only through exact, versioned fixtures of OpenCode 1.17.18. In version 1, a match is terminal (`permission_required`). Live resolution through adapters and managed sessions is defined in Spec 9 (modes `supervised` and `terminal`), already implemented; the `headless` path of this spec keeps its behavior except for the Spec 9 inactivity migration.
6. Captures a real baseline before each attempt and keeps snapshots of new or modified files with SHA-256. The cumulative raw content budget is 1 MiB per step; exceeding it produces `artifact_context_too_large`.
7. Persists canonical, bounded evidence. `DiagnosticRaw` is the only authority for unredacted diagnostic bytes: up to 1 MiB it keeps the exact frame; if it exceeds the limit, it keeps prefix, suffix, size, and SHA-256. `AttemptEvidence`, the legacy columns, and the console receive only sanitized projections of up to 16 KiB.
8. When a clean error enables fallback, it builds for the next candidate an extended context with the original instructions, skills, and required artifacts, plus a sanitized projection of snapshots, concrete provider errors, and references to previous attempts. This projection never includes raw evidence and has an accumulated budget of 2 MiB per `step run`.

The step is NOT marked as completed automatically. The orchestrator must invoke `shardeo step complete <execution-id> <step-id>` to do so. At that moment, Shardeo validates that the artifacts declared in `produces` exist under `.shardeo/artifacts/`. If they do not exist, the command fails and the step remains in the "in progress" state.

### Acceptance criteria

- Shardeo gets the candidates exclusively from `steps[].agents`, walks the list in order, and uses the first supported CLI available in the PATH.
- In v1, the only supported CLI is `opencode`.
- Before executing each step, Shardeo re-reads and validates the complete workflow YAML, including its format and business rules.
- A workflow that declares any agent other than `opencode` is invalid. Execution returns an error before invoking a CLI or modifying the step state.
- If a supported CLI is not installed or unavailable, Shardeo tries the next declared candidate without creating an attempt for the discarded candidate during probing.
- If no availability probe passes validation before invoking a CLI, the step fails with a response that includes sanitized evidence of all evaluated candidates and no attempt is created.
- Every clean OpenCode error that meets the process and JSONL gates is recorded and advances to the next candidate, without inferring whether it comes from quota, context, model, or provider.
- `permission_required`, `permission_timeout`, `artifact_context_too_large`, `adapter_contract_error`, `process_start_failed`, `process_cleanup_failed`, `context_error`, and `persistence_error` are terminal in version 1 and do not fall back. The managed modes that resolve permissions live (`supervised` and `terminal`) are defined in Spec 9, already implemented; the `headless` path of this spec keeps these codes except that inactivity is reported as `process_inactivity_timeout` per the Spec 9 migration.
- Each CLI invocation is recorded as an independent attempt of the same step.
- Each candidate invoked by fallback receives an extended context with the original instructions, skills, and required artifacts, a sanitized projection of snapshots and previous errors, and references to previous attempts. The projection never contains raw evidence and its accumulated budget is 2 MiB per `step run`.
- The context injected into the CLI includes, in this order: operational instructions, domain instructions, skills, and the content of the required artifacts.
- The CLI is invoked in non-interactive mode with `shell: false`. Neither `--auto` nor any other permission-bypass flag is injected.
- The visible response is sanitized and limited; the bounded local evidence keeps the diagnostic material and its digest.
- `shardeo step complete <execution-id> <step-id>` validates `produces` under `.shardeo/artifacts/` with containment: it rejects absolute paths, `..`, and symlink escapes; it allows symlinks whose destination stays within the root.
- For an `agent` step, `shardeo step complete` first validates that all artifacts declared in `produces` exist and are safe. Only then does it mark the step as completed. Steps whose predecessors declared in `depends_on` are completed become available in `steps next`; `requires` is validated only when executing the step.
- The complete or partial response, the CLI used, the model, and the completion reason of each attempt are stored in the database.
- Artifact snapshots are captured with SHA-256 hashing, excluding unchanged files. The cumulative budget is 1 MiB per step.
- Artifact validation (`requires`/`produces`) rejects absolute paths, `..`, and symlink escapes for `agent` and `command` steps.

### Example user flow

```
$ shardeo step run exec-a1b2c3 architecture
Resolving candidate... opencode (openai/gpt-4o) ✓
Injecting context: instructions + 2 skills
Running in non-interactive mode...

Agent response:
  [sanitized projection of the OpenCode response]

$ shardeo step complete exec-a1b2c3 architecture
Validating artifacts:
  ✓ architecture.md
  ✓ adr.md
Step completed.
```

---

## Spec 5: Execution graph and parallelism resolution

status: pending

### Objective

Let Shardeo resolve the execution order of steps based on the graph defined by `depends_on`, and report to the orchestrator when there are steps that can run in parallel.

### Functional description

Shardeo builds a directed acyclic graph (DAG) from the `depends_on` relationships between steps. Each property has a single responsibility: `depends_on` defines order, `requires` validates inputs, `produces` validates outputs. Only `depends_on` participates in graph construction.

When starting a workflow (`shardeo run`), Shardeo performs three validations on the graph before recording the execution:

1. That at least one step without `depends_on` exists (the workflow's entry point).
2. That there are no cycles (A after B, B after A).
3. That every ID referenced in `depends_on` corresponds to a step that exists in the workflow.

When the orchestrator queries `shardeo steps next`, Shardeo returns all steps whose predecessors (`depends_on`) are completed. If multiple steps are available simultaneously, it returns them all. The decision to run them in parallel or sequentially belongs to the orchestrator. Shardeo does not run anything in parallel by itself.

### Acceptance criteria

- `shardeo run` validates the graph on start. If there is no step without `depends_on`, it rejects the execution indicating there is no entry point.
- If it detects a cycle, it rejects the execution with a message identifying the steps involved.
- If a `depends_on` references a step ID that does not exist in the workflow, it rejects the execution with the code `invalid_depends_on_reference` and indicates the invalid reference.
- `shardeo steps next` returns multiple steps when their predecessors (`depends_on`) are completed simultaneously (e.g., `api-design` and `database-design` after completing `architecture`).
- A step with `depends_on: [api-design, database-design]` only appears as available when both are completed.
- Steps without `depends_on` appear as available immediately when the workflow starts.
- The `steps next` response includes enough metadata for the orchestrator to decide: step id, type, and the list of configured agents.
- The `requires` validation (artifact existence) happens when executing the step (`step run`), not when resolving the graph.

### Example user flow

```
$ shardeo run backend-feature
Validating execution graph... ✓
Execution started: exec-x7y8z9

$ shardeo steps next exec-x7y8z9
Available steps:
  - architecture (agent) — no predecessors

  [orchestrator completes 'architecture']

$ shardeo steps next exec-x7y8z9
Available steps:
  - api-design (agent) — depends_on: architecture ✓
  - database-design (agent) — depends_on: architecture ✓

  [both can run in parallel]
```

---

## Spec 6: Step re-execution with accumulated context

status: pending

### Objective

Let an `agent`-type step run multiple times before being marked as completed, delivering to the agent the context of previous attempts and the user's feedback so it can iterate over its own work.

### Functional description

Spec 4 fully defines the automatic fallback between candidates within the same `step run`, including its accumulated context, sanitized evidence, and limits. This spec does not modify that behavior. It adds the later manual re-execution of an uncompleted step and the `--feedback` parameter.

Feedback is sent as a command parameter: `shardeo step run <execution-id> <step-id> --feedback "The ADR does not include evaluated alternatives"`.

Each CLI invocation is recorded in the database as a separate attempt, linked to the step and the execution. This includes invocations made automatically by fallback. Each attempt stores: attempt number, complete or partial agent response, received feedback, generated artifacts, CLI used, and completion reason.

### Acceptance criteria

- An uncompleted step can be re-executed with `shardeo step run` without error.
- On re-execution, the context delivered to the CLI includes the artifacts generated in previous attempts.
- If `--feedback` is provided, the text is included in the context delivered to the agent, clearly delimited from the instructions.
- Each attempt is recorded as a separate entry in the database with: timestamp, attempt number, complete or partial response, feedback, generated artifacts, CLI used, and completion reason.
- Artifacts generated in a re-execution overwrite those of previous attempts on the filesystem (`.shardeo/artifacts/`), but the previous ones remain recorded in the database.
- `shardeo step complete` validates the artifacts of the last attempt, regardless of how many attempts there have been.

### Example user flow

```
$ shardeo step run exec-x7y8z9 architecture
[...agent response...]

$ shardeo step complete exec-x7y8z9 architecture
✗ Missing artifact: adr.md

$ shardeo step run exec-x7y8z9 architecture --feedback "The ADR is missing. It must include evaluated alternatives and the justification of the decision."
Attempt #2
Context: instructions + skills + architecture.md (previous attempt) + feedback
[...agent response...]

$ shardeo step complete exec-x7y8z9 architecture
✓ architecture.md
✓ adr.md
Step completed.
```

---

## Spec 7: Execution persistence and workflow resumption

status: done

### Objective

Let an interrupted workflow (due to an error, terminal closure, or cancellation) be resumed from the last known state, without losing the work already done.

### Functional description

Shardeo persists all execution metadata in an embedded SQLite database. When the user runs `shardeo resume <execution-id>`, Shardeo queries the database, identifies which steps were completed, verifies that their artifacts are still present on the filesystem, and resumes the workflow from that point.

The persisted overall state of the execution must stay synchronized with its steps' transitions. When all steps of the workflow are completed, Shardeo persists `executions.status = completed`; `shardeo status` cannot merely compute and display `completed` while the execution row remains in `running`. The update must be atomic with the transition that completes the workflow, or idempotent and recoverable if performed immediately after.

If an artifact was completed in the database but its file does not exist in `.shardeo/artifacts/`, Shardeo marks the step as "requires reconstruction" and delivers the previous agent response as context so the orchestrator can reconstruct it.

`shardeo status <execution-id>` shows the complete execution state including: completed steps (with timestamps and attempt counts), failed steps (with the error), pending steps, and the overall workflow state.

### Acceptance criteria

- Every workflow execution is persisted in SQLite: workflow, start date, overall state.
- The stored overall state is updated after every transition that can change the workflow's aggregate state.
- When the last pending step is completed, `executions.status` is persisted as `completed`; the `shardeo status` response and the stored row cannot diverge.
- Each executed step persists: state, timestamps, attempt count, CLI used.
- Each attempt persists: agent response, feedback, generated artifacts, metrics (duration).
- `shardeo resume <execution-id>` resumes the execution without re-running already completed steps.
- If a completed artifact does not exist on the filesystem, `resume` marks the corresponding step for reconstruction and delivers the previous agent response as context.
- `shardeo status <execution-id>` shows a complete summary with states, timestamps, and attempt counts per step.
- If the `execution-id` does not exist, `resume` and `status` fail with a clear message.
- A workflow whose last step was completed appears with a "completed" state in `status`.

### Example user flow

```
$ shardeo status exec-x7y8z9
Workflow: backend-feature
Status: in progress
Start: 2026-07-05 10:30:00

Steps:
  ✓ architecture — completed (2 attempts, 10:32)
  ✓ api-design — completed (1 attempt, 10:45)
  ✓ database-design — completed (1 attempt, 10:44)
  ✗ lint — failed (exit code 1, 10:50)
  ○ implementation — pending

$ shardeo resume exec-x7y8z9
Resuming execution exec-x7y8z9...
Steps completed: 3/5
Verifying artifacts... ✓

$ shardeo steps next exec-x7y8z9
Available steps:
  - lint (command) — last attempt failed, can be re-run
```

---

## Spec 8: Orchestrator-driven iteration control

status: pending

### Objective

Allow the orchestrating agent to reopen completed work or skip pending work in response to the outcome of other steps or a user decision, without embedding business conditions or methodological decisions inside Shardeo.

### Functional description

The graph defined by `depends_on` remains static. The orchestrator interprets the workflow instructions and the steps' responses, and requests explicit transitions from Shardeo when it needs to iterate or take an optional branch.

If a later step detects that the result of a predecessor must be corrected, the orchestrator runs:

```
shardeo step reopen <execution-id> <step-id> --cascade --feedback "<reason>"
```

Shardeo keeps the previous attempts, reopens the step, invalidates its completed generation, and resets the state of the affected descendants. The feedback stays associated with the reopening and is included in the context of the step's next attempt.

The artifacts of an invalidated generation remain on the filesystem and in the history for audit, but do not satisfy `requires` nor allow the step to be completed again immediately. After at least one new attempt finishes successfully, `step complete` revalidates the current set of artifacts declared in `produces` and records a new valid generation. An artifact can keep the same bytes if it needed no changes; validity belongs to the step's new generation and does not require artificial rewrites of every file.

If the workflow instructions indicate that a pending step is not needed, the orchestrator runs:

```
shardeo step skip <execution-id> <step-id> --reason "<reason>"
```

The step moves to the `skipped` state. This state is considered terminal when resolving `depends_on` dependencies, but it generates no artifacts and does not satisfy `requires` validations.

Shardeo does not evaluate conditional expressions nor interpret agent responses. The decision to reopen or skip a step belongs exclusively to the orchestrator; Shardeo validates and persists the requested transition.

### Acceptance criteria

- `shardeo step reopen <execution-id> <step-id> --cascade --feedback <text>` allows reopening a completed or failed step.
- The reopening keeps all previous attempts, their responses, artifacts, feedback, timestamps, and CLI used.
- The reopened step becomes available for execution again when its original predecessors remain completed or `skipped`.
- The reopening feedback is stored and included clearly delimited in the context of the next attempt.
- `--cascade` resets to pending state all descendants that were in progress, completed, or failed, invalidates their completed generations, and keeps their attempt history.
- The artifacts of an invalidated generation cannot satisfy `requires` nor allow `step complete` until at least one new attempt of the step finishes successfully and the current `produces` set is revalidated as a new generation. Files that needed no changes can keep their bytes.
- If the reopening affects descendants and `--cascade` is not provided, the command fails and reports which steps would be invalidated.
- A pending, in-execution, or `skipped` step cannot be reopened; the command fails indicating its current state.
- `shardeo step skip <execution-id> <step-id> --reason <text>` allows skipping a pending step that does not yet have started attempts.
- A step in execution, completed, or failed cannot be skipped without first reopening or resolving its current state.
- A `skipped` step is considered terminal when resolving `depends_on` dependencies.
- Skipping a step does not create its artifacts. Any later step that declares them in `requires` fails normally when validating its inputs.
- Both `reopen` and `skip` record the reason, actor, timestamp, and state transition in the database.
- `shardeo status <execution-id>` shows reopened or skipped steps, their reasons, and the transition history.
- `shardeo steps next <execution-id>` recomputes the available steps after each reopening or skip.
- Transitions are rejected if the `execution-id` or the `step-id` does not exist.

### Example user flow

```
$ shardeo status exec-x7y8z9
Steps:
  ✓ implementation — completed (attempt #1)
  ✗ verification — failed: criterion AC-3 not met

$ shardeo step reopen exec-x7y8z9 implementation \
    --cascade \
    --feedback "Verification failed: criterion AC-3 is not satisfied"

Step reopened: implementation
Descendants reset:
  - verification

$ shardeo steps next exec-x7y8z9
Available steps:
  - implementation (agent) — attempt #2

$ shardeo step run exec-x7y8z9 implementation
Context: instructions + skills + previous artifacts + reopen feedback
```

---

## Spec 9: Execution surfaces and managed interaction for agent steps

status: implemented

### Objective

Define a multi-harness, provider-neutral architecture for running `agent` steps on three distinct surfaces: `headless`, `supervised`, and `terminal`. Shardeo must be able to mediate structured permissions without coupling the core to a specific protocol, hand exclusive control of a terminal to a person when appropriate, deliver reproducible context to each managed attempt without depending on operating-system argument limits, and keep low-volume control events and full output separate.

This spec replaces the future, unimplemented v2 proposal based on `interactive: boolean`, previously documented in AGENTS.md; it does not replace the implemented Spec 4 behavior in `headless`, except for the explicit migration of the inactivity outcome (see §8). It was implemented in three slices delivered on `main`: `01-bundle-manifiesto-sondeo` (9a: bundle, manifest, probing, mode resolution, transport, containment), `02-supervised-events` (9b: supervisor, lease with fencing, IPC, interaction broker, events, output, permission policy, timeouts), and `03-terminal-pty` (9c: attachable PTY, attach/reattach, human-presence timeout, and handoff). The adapter, command, and event contracts documented here are the behavioral authority of the `supervised` and `terminal` modes; in `headless`, Spec 4 remains the authority for the non-interactive path except for the migration documented in §8.

### Decision summary

| Topic | Decision |
|---|---|
| Surfaces | The modes are `headless`, `supervised`, and `terminal`; they are not interchangeable variants of a single interactive session. |
| Inheritance | `step.mode > workflow.mode > config.yaml defaults.agent_mode`; the default value is `headless`. |
| Override | `shardeo step run ... --mode <mode>` overrides the configuration only for that attempt. |
| Integration | Each harness is integrated through an adapter that probes real capabilities at runtime. |
| Context per attempt | The core creates and freezes an immutable `context bundle` per managed attempt, with a provider-neutral manifest that preserves order, limits, and digests. |
| Context transport | The adapter selects an announced and safe transport. The core's contract is the manifest, never a provider syntax such as `@path`. |
| Lazy loading | Referencing content avoids transport size limits and enables on-demand loading; all loaded content still consumes model tokens. |
| Supervision | `supervised` uses a local background supervisor that maintains the session and mediates structured interactions. |
| Terminal | `terminal` creates an attachable terminal controlled only by a person. The orchestrator neither handles nor interprets its screen. |
| Managed commands | In `supervised` and `terminal`, `step run` starts the attempt and returns immediately; tracking is done with `step events`, `step status`, and `step output`. |
| Permissions | `step approve` resolves a normalized `interaction_id` using only a decision announced by the adapter. |
| Coordination | Live signaling uses explicit local inter-process communication. SQLite keeps state and audit records, but does not act as a message bus. |
| Fail-safe | A missing capability produces `unsupported_capability`; Shardeo neither changes mode nor silently broadens permissions. |

### Terminology

| Term | Meaning in this spec |
|---|---|
| Spec | `Specification` (a documented contract of behavior and acceptance criteria). |
| Harness | Agent product or runtime that performs the work, e.g., OpenCode today and other harnesses in the future. |
| Adapter | Integration boundary that knows a harness's launch, probing, and native protocols. |
| CLI | `Command-Line Interface` (a program operated through text commands). It is one possible surface of a harness, not the core contract. |
| API | `Application Programming Interface` (a structured contract between programs). |
| SDK | `Software Development Kit` (a library and tooling a provider offers for integrations). |
| PTY | `Pseudoterminal` (a device pair that provides real terminal semantics to a process). |
| TUI | `Terminal User Interface` (an interactive screen meant for a person). |
| IPC | `Inter-Process Communication` (a local channel to send commands to the active supervisor). |
| Supervisor | Local background process that owns a managed session, normalizes events, and applies resolutions through the adapter. |
| Interaction | Structured harness request that requires an external decision or action. The minimum supported kind is `permission`. |
| Control event | Low-volume, normalized event describing semantic changes of the attempt. It does not contain the full output. |
| Cursor | Opaque, persisted identifier that lets event polling continue without losing or double-processing events. |
| Context bundle | A package of context: an immutable, contained copy of the materials authorized for an attempt, together with its manifest. |
| Context manifest | Neutral contract that identifies the bundle and describes each entry without imposing how a harness must admit it. |
| Context admission | Proof produced by the adapter that the mandatory entries became available to the session through the selected transport. |
| SHA-256 | `Secure Hash Algorithm 256-bit` (a cryptographic digest used to verify identity and integrity). |

The interaction types form an extensible contract. This spec defines the complete
flow for `permission` and reserves `question`, `authentication`, and
`terminal_handoff` for adapters that announce explicit support. Declaring a type
does not imply that Shardeo can already resolve it.

### Architecture and responsibilities

#### Shardeo core

The core validates configuration, resolves the effective mode, selects the
adapter, builds and verifies the context bundle, applies policies over normalized
data, persists state, and exposes the public commands. It knows nothing about
provider-specific endpoints, flags, events, reference syntax, or response values.

#### Harness adapter

The adapter encapsulates all harness-specific behavior:

- Probes the capabilities of the selected installation at runtime.
- Launches the process or session with the mechanism appropriate for the mode.
- Selects an announced context transport, admits the manifest entries, and
  returns verifiable evidence of availability.
- Converts native events into normalized interactions and events.
- Translates a normalized decision announced in `available_decisions` to the
  native protocol.
- Declares whether it can resume or reconcile a session after a crash.
- Exposes only policy metadata it can map unambiguously.

The core does not assume that a harness emits `JSONL (JSON Lines, based on
JavaScript Object Notation: a format of one structured object per line)` nor that
it accepts decisions through standard input. It also does not hardcode fixture
versions, command flags, or provider-specific responses.

Launching is the adapter's responsibility. It must prefer direct execution with
an `argv (argument vector: a structured list of arguments passed to the process)`
and avoid shell interpolation. If
a harness requires a shell, the adapter must justify it and safely escape
untrusted values. Activating a shell does not create a PTY; the `terminal` mode
must allocate a real PTY explicitly.

#### Context bundle and manifest

For each `supervised` or `terminal` attempt, the supervisor, as a component of the
core, materializes a bundle before starting the provider session. The bundle
contains per-attempt copies, not mutable references to the project's live files.
Once its digests are computed and the manifest is written, both are frozen. This
avoids sending the combined context as a single argument, exceeding
operating-system limits, and losing reproducibility if a file changes during the
attempt.

The manifest preserves the semantic order and boundaries required by Spec 4:
operational instructions, domain instructions, skills, required artifacts, and
prior or fallback context when applicable. The `order` field defines that total
order and each entry keeps its own file. An adapter cannot concatenate, reorder,
nor omit entries in a way that alters those boundaries. `role` values are stable,
adapter-neutral logical identifiers; a transport may map them, but not
reinterpret them.

Provider-neutral example:

```json
{
  "schema_version": 1,
  "bundle_id": "bundle-attempt-7",
  "execution_id": "exec-abc",
  "step_id": "architecture",
  "attempt_id": "attempt-7",
  "created_at": "2026-07-26T12:00:00.000Z",
  "created_by": "shardeo",
  "total_bytes": 42071,
  "entries": [
    {
      "order": 10,
      "role": "operational_instructions",
      "relative_path": "entries/010-operational.md",
      "required": true,
      "loading": "eager",
      "bytes": 2841,
      "sha256": "d4b5a901d4b5a901d4b5a901d4b5a901d4b5a901d4b5a901d4b5a901d4b5a901"
    },
    {
      "order": 20,
      "role": "domain_instructions",
      "relative_path": "entries/020-domain.md",
      "required": true,
      "loading": "eager",
      "bytes": 4970,
      "sha256": "8c215e3f8c215e3f8c215e3f8c215e3f8c215e3f8c215e3f8c215e3f8c215e3f"
    },
    {
      "order": 30,
      "role": "skill",
      "relative_path": "entries/030-skill-hexagonal.md",
      "required": true,
      "loading": "on_demand",
      "bytes": 6230,
      "sha256": "472e81bc472e81bc472e81bc472e81bc472e81bc472e81bc472e81bc472e81bc"
    },
    {
      "order": 40,
      "role": "required_artifact",
      "relative_path": "entries/040-architecture.md",
      "required": true,
      "loading": "on_demand",
      "bytes": 18720,
      "sha256": "779a0c12779a0c12779a0c12779a0c12779a0c12779a0c12779a0c12779a0c12"
    },
    {
      "order": 50,
      "role": "prior_attempt_context",
      "relative_path": "entries/050-prior-attempt.md",
      "required": false,
      "loading": "on_demand",
      "bytes": 9310,
      "sha256": "f13c721df13c721df13c721df13c721df13c721df13c721df13c721df13c721d"
    }
  ]
}
```

Operational and domain instructions are mandatory and `eager`. Required skills
and required artifacts are mandatory references: before work begins, the adapter
must prove that the session can obtain the exact bytes identified by the
manifest. Optional or prior material is loaded `on_demand`. `on_demand` defers
the byte transfer, but does not make an entry with `required: true` optional nor
allow the agent to skip contractually required material. If the transport cannot
prove the availability of all mandatory entries, the attempt fails closed before
provider work.

The bundle reduces the initial transport size and avoids loading unnecessary
optional material. It does not by itself reduce token usage: when an entry is
injected or the agent reads it, its content consumes model tokens like any other
context.

The core validates the admission proof against `bundle_id`, the manifest digest,
and the digests of the mandatory entries before allowing provider work. The proof
demonstrates availability and byte identity, not that the content is free for the
model nor that a mandatory entry can be ignored.

#### Managed-attempt supervisor

Each `supervised` or `terminal` attempt has a single owning supervisor. The
supervisor holds the process or session handle, captures output, receives
commands over IPC, renews its lease, and publishes persisted control events.
`step run` waits for a readiness handshake confirming a frozen bundle, admitted
context, and started session; then it returns.

#### Interaction broker

The broker normalizes native requests, assigns an `interaction_id`, records
`available_decisions`, and applies a resolution exactly once. The normalized
interaction preserves the minimum needed to decide without exposing the
provider's internal contracts as part of Shardeo's public API.

#### State and output store

SQLite keeps attempts, sessions, leases, interactions, decisions, cursors, and
audit records. A contained file store under the runtime root keeps the full
output per attempt. Neither replaces the IPC channel used for real-time
signaling.

#### Relationship with Traycer

The architecture takes from Traycer only the separation between harnesses and
the mediated interaction pattern. It does not assume that Traycer limits OpenCode
to a TUI, nor does it depend on private details of its implementation.

### Capability probing

The adapter produces a capability manifest after consulting the actual
installation that will be used. Probing can validate commands, endpoints,
protocol negotiation, or callbacks, but cannot infer support from a declared
semantic version alone.

Illustrative TypeScript shape:

```ts
type ExecutionMode = "headless" | "supervised" | "terminal";
type ContextTransport =
  | "direct_injection"
  | "file_reference"
  | "api_attachment"
  | "tool_read";
type InteractionKind =
  | "permission"
  | "question"
  | "authentication"
  | "terminal_handoff";

type DecisionScope = "request" | "session" | "resource";

interface ModeCapability {
  supported: boolean;
  reason?: string;
  interaction_kinds: InteractionKind[];
  decision_scopes: DecisionScope[];
}

interface ContextTransportCapability {
  supported: boolean;
  modes: ExecutionMode[];
  loading: Array<"eager" | "on_demand">;
  max_total_bytes: number;
  max_entry_bytes: number;
  proves_required_availability: boolean;
  verifies_sha256: boolean;
  scoped_read_permission: "supported" | "unsupported" | "not_applicable";
  reason?: string;
}

interface ContextAdmissionReceipt {
  bundle_id: string;
  manifest_sha256: string;
  transport: ContextTransport;
  admitted_required_entries: Array<{
    relative_path: string;
    sha256: string;
  }>;
}

interface HarnessCapabilityManifest {
  adapter_id: string;
  harness_identity: string;
  probed_at: string;
  modes: Record<ExecutionMode, ModeCapability>;
  context_transports: Record<
    ContextTransport,
    ContextTransportCapability
  >;
  policy_capabilities: string[];
  session_resume: "supported" | "unsupported";
  interaction_reconciliation: "supported" | "unsupported";
}

interface AvailableDecision {
  id: string;
  effect: "allow" | "deny";
  scope: DecisionScope;
}

interface NormalizedInteraction {
  interaction_id: string;
  kind: InteractionKind;
  summary: string;
  permission_capability?: string;
  resource?: Record<string, unknown>;
  available_decisions: AvailableDecision[];
}
```

The capability manifest describes support, not authorization. That an adapter
supports a `session`-scoped decision does not authorize Shardeo to select it.
Each interaction announces the valid subset in `available_decisions`.

The adapter chooses a transport that supports the effective mode, the loading
strategies, and the bundle's real limits. It must also be able to verify SHA-256
and prove the availability of mandatory entries. `direct_injection` is valid only
if the payload fits safely; `file_reference`, `api_attachment`, and `tool_read`
may defer loading, but do not relax admission nor the read policy. If no
combination satisfies the manifest, Shardeo fails with `unsupported_capability`
before starting provider work.

If the requested mode is not supported by the selected adapter and installation,
`step run` fails with `unsupported_capability` before launching the harness
process. The error includes `adapter_id`, `harness_identity`, `requested_mode`,
and a sanitized reason. There is no silent fallback to `terminal`, to another
mode, or to broader permissions.

### Non-normative adapter mappings

These examples explain how concrete adapters could be implemented; they are not
part of the core contract:

- OpenCode could use a managed server, a structured stream via
  `SSE (Server-Sent Events: events sent by the server over a persistent
  connection)`, and a permission response via
  `HTTP (Hypertext Transfer Protocol: a communication protocol for web resources
  and operations)`.
- Codex could use its app-server and
  `JSON-RPC (JavaScript Object Notation Remote Procedure Call: a remote call
  protocol with structured messages)` for approval requests and responses.
- A future harness could use SDK callbacks, hooks, or another structured
  protocol.
- For context, OpenCode could resolve entries via `@path` references or a
  managed session; another harness could use native attachments, API or SDK
  payloads, tool-based reads, or direct injection.

The adapter must hide native event names, paths, methods, and response values.
None of these examples obliges other adapters to reproduce the same transport.
In particular, `@path` does not appear in the manifest nor in the core: it is
only a possible internal translation of the OpenCode adapter.

Non-normative example for an OpenCode terminal. The working directory stays in
the assigned project or worktree; the bundle lives under a contained runtime
root and is referenced with a relative path:

```bash
# Direct start of the TUI with a short bootstrap created by the adapter.
opencode <project-or-worktree> \
  --prompt 'Load @.shardeo/runtime/attempt-7/context/manifest.json and follow the declared order.'

# Preferred: attach the TUI to an already created session on a managed server.
opencode attach <managed-server-url>
```

In the second variant, the adapter first attaches the manifest through the
session's API or SDK and opens the TUI against the managed server; it selects
the exact session through a verified native capability or fails with
`unsupported_capability`, without asking the person to infer it. It does not
re-transport the context in the prompt. It is the preferred option when the
session already exists. The commands are illustrative and do not turn OpenCode
flags or references into normative Shardeo contract.

### Mode resolution

The declarative configuration uses a single `mode` property:

```text
step.mode > workflow.mode > config.yaml defaults.agent_mode
```

If no layer declares it, the effective mode is `headless`. The `--mode` flag
overrides the effective value only for the attempt started by that command.

```yaml
# config.yaml
defaults:
  agent_mode: headless

# workflow.yaml
name: backend-feature
mode: supervised
steps:
  - id: architecture
    type: agent
    mode: terminal
    instructions: steps/architecture/instructions.md
```

`command` steps do not use `agent_mode` nor accept these surfaces.

### Behavior by mode

| Aspect | `headless` | `supervised` | `terminal` |
|---|---|---|---|
| Primary control | Process invoked by `step run` | Shardeo supervisor | Person attached to a managed PTY |
| Return of `step run` | Blocks until finished, as in Spec 4 | Returns after starting the supervisor | Returns after starting the supervisor and preparing the PTY |
| Interaction | Not resolved live if the adapter cannot do so | Structured events and mediated decisions | Native human interaction in the terminal |
| Unresolvable permission | Terminates with `permission_required` | Remains in `awaiting_interaction` | Decided by the person inside the harness |
| Orchestrator usage | Receives the final result | Polls events and issues explicit decisions | Only requests the mode and communicates the handoff |
| Terminal screen | Not required | Not required | Never controlled or interpreted by an agent |
| Context | Direct injection from Spec 4 while it remains safe and supported | Bundle frozen and admitted before opening the session | Bundle frozen and adapter bootstrap before `awaiting_human` |
| Full output | Evidence and projection per Spec 4 | On-disk buffer, queryable | On-disk buffer, queryable |

#### `headless` mode

`headless` preserves Spec 4 behavior except for the explicit inactivity outcome
migration described in this spec. `step run` remains blocking and the adapter
executes the existing non-interactive path. When it detects a request it cannot
resolve through a supported policy, it terminates the attempt with
`permission_required`. Since Spec 9's implementation, an expiry due to lack of
activity changes from `permission_timeout` to `process_inactivity_timeout`. This
spec does not turn OpenCode fixtures into a supervision mechanism nor change the
v1 fallback rules. The implemented adapter may continue injecting directly the
combined context through standard input when safe and supported. Adopting bundles
for `supervised` and `terminal` does not silently change that path. A `headless`
adapter can only use bundles after announcing and validating explicit support
for the chosen transport.

#### `supervised` mode

1. `step run` resolves configuration, adapter, and mode.
2. The adapter probes the capabilities of the selected installation.
3. Shardeo creates the attempt identity and acquires an owner lease.
4. Shardeo starts the supervisor.
5. The supervisor creates and freezes the attempt's bundle, verifies its limits
   and integrity, and persists its identity.
6. The adapter selects an announced transport, verifies the digests, and returns
   proof that all mandatory entries became available.
7. Only then does the supervisor open the native session and start capturing the
   full output; the readiness handshake confirms these steps.
8. `step run` returns `attempt_id`, `session_id`, state, bundle identity,
   transport, and initial cursor.
9. The adapter receives a native request and normalizes it as
   `interaction_required`.
10. The supervisor persists the event and moves to `awaiting_interaction`
    without closing the session.
11. The orchestrator polls `step events` and selects a decision included in
    `available_decisions`.
12. `step approve` sends the resolution to the supervisor over IPC.
13. The supervisor performs an idempotent compare-and-set resolution, asks the
    adapter to translate it, and publishes `interaction_resolved`.

A resolution does not write generic bytes to standard input. The adapter uses
the API, SDK, or structured protocol it announced during probing.

#### `terminal` mode

1. `step run` creates the attempt identity, a supervisor, and an attachable PTY
   in the background.
2. The supervisor creates and freezes the bundle before starting the provider
   session.
3. The adapter verifies the digests and admits the context through a native
   mechanism, or starts the harness with a short bootstrap instruction pointing
   at the manifest and preserving the declared order and mandatory nature.
4. Only after that admission does the harness become active inside the PTY and
   the supervisor moves to `awaiting_human`.
5. `step run` returns the attempt identity, the bundle identity, and the exact
   attach command.
6. The orchestrator shows the handoff; it does not write the bootstrap, does not
   send keystrokes, does not capture the screen, and does not attempt to
   interpret the TUI.
7. The person opens another terminal and runs
   `shardeo step attach <execution-id> <step-id> --attempt-id <attempt-id>`.
8. On disconnect, the child process stays alive and the supervisor returns to
   `awaiting_human`. The person can run the same command to reattach while the
   attempt remains active and the no-human-presence window has not expired.
9. When the harness finishes, the supervisor closes the PTY, persists the result,
   and publishes the attempt's terminal event.

The portable mode requires another terminal because the orchestrating harness
may remain open. Suspending the current terminal, integrating Tmux or Zellij, or
installing harness plugins are future UX improvements, not part of the central
contract.

### Command and event contract

Managed commands always use the identity returned by `step run`. The following
examples belong to the same attempt.

```bash
$ shardeo step run exec-abc architecture --mode supervised
{"type":"attempt_started","execution_id":"exec-abc","step_id":"architecture","attempt_id":"attempt-7","session_id":"session-7","mode":"supervised","state":"running","event_cursor":"event-0","context_bundle":{"bundle_id":"bundle-attempt-7","manifest_sha256":"a45192efa45192efa45192efa45192efa45192efa45192efa45192efa45192ef","transport":"api_attachment","required_entries_admitted":4}}

$ shardeo step events exec-abc architecture --attempt-id attempt-7 --after event-0 --limit 50
{"events":[{"cursor":"event-1","type":"interaction_required","interaction":{"interaction_id":"interaction-42","kind":"permission","summary":"Execute package installation","permission_capability":"process.execute","resource":{"command":"pnpm install"},"available_decisions":[{"id":"allow_once","effect":"allow","scope":"request"},{"id":"deny","effect":"deny","scope":"request"}]}}],"next_cursor":"event-1","has_more":false}

$ shardeo step status exec-abc architecture --attempt-id attempt-7
{"attempt_id":"attempt-7","state":"awaiting_interaction","pending_interaction_id":"interaction-42","event_cursor":"event-1"}

$ shardeo step output exec-abc architecture --attempt-id attempt-7 --tail 100
[sanitized projection of the last 100 lines]

$ shardeo step approve exec-abc architecture --interaction-id interaction-42 --decision allow_once
{"type":"interaction_resolved","interaction_id":"interaction-42","decision":"allow_once","actor":"orchestrator","state":"running","cursor":"event-2"}

$ shardeo step events exec-abc architecture --attempt-id attempt-7 --after event-1 --limit 50
{"events":[{"cursor":"event-2","type":"interaction_resolved","interaction_id":"interaction-42","decision":"allow_once"},{"cursor":"event-3","type":"attempt_completed","exit_code":0}],"next_cursor":"event-3","has_more":false}

$ shardeo step output exec-abc architecture --attempt-id attempt-7 --full
[stream of the retained and sanitized full output]
```

`--after` is exclusive: it returns events after the given cursor. Repeating the
query with the same cursor returns the same stable set within retention. The
consumer stores `next_cursor` only after processing the complete response.
Ordering is defined per attempt, not by timestamp.

`step status` provides a current view and may omit intermediate transitions;
`step events` is the source for consuming semantic transitions. `step output` is
the only interface for reading high-volume output and does not advance the event
cursor. `--tail <lines>` returns a sanitized instantaneous view and can be used
while the attempt is active or after it finishes. `--full` is only accepted for a
terminal attempt and streams the entire retained and sanitized output; if the
retention policy dropped content, the response includes truncation metadata.
Each `interaction_id` is unique and bound to the execution, step, and attempt
that originated it; `step approve` rejects any identity crossing.

The minimum events are:

| Event | Purpose |
|---|---|
| `attempt_started` | Confirms identity, mode, initial cursor, `bundle_id`, manifest digest, transport, and number of admitted mandatory entries. It does not include bundle content. |
| `attempt_state_changed` | Publishes a relevant state transition. |
| `interaction_required` | Presents an interaction and its available decisions. |
| `interaction_resolved` | Records the applied decision and its actor. |
| `output_available` | Indicates that queryable output exists without including it. |
| `attempt_completed` | Publishes successful completion and a bounded summary. |
| `attempt_failed` | Publishes a structured error and a bounded summary. |

### Human handoff in terminal

```bash
$ shardeo step run exec-abc architecture --mode terminal
{"type":"attempt_started","execution_id":"exec-abc","step_id":"architecture","attempt_id":"attempt-8","session_id":"session-8","mode":"terminal","state":"awaiting_human","event_cursor":"event-0","context_bundle":{"bundle_id":"bundle-attempt-8","manifest_sha256":"c12f09abc12f09abc12f09abc12f09abc12f09abc12f09abc12f09abc12f09ab","transport":"file_reference","required_entries_admitted":4},"attach_command":"shardeo step attach exec-abc architecture --attempt-id attempt-8"}

# The person runs this in another terminal:
$ shardeo step attach exec-abc architecture --attempt-id attempt-8
Attached to attempt-8. Use the configured detach sequence to keep the child alive.
```

If no single live `terminal` attempt exists for that execution and step, the
command fails explicitly and lists the candidate identities; it never attaches to
a session through ambiguous inference. Detach does not send a termination signal
to the child.

### Permission policy

Shardeo replaces rules based on path or command patterns with policies over
normalized capabilities that the adapter can map precisely. The closest
declaration replaces the whole object:

```text
step.permission_policy > workflow.permission_policy > config.yaml defaults.permission_policy
```

The default value is `prompt`. Lists are not merged across layers because an
implicit combination could broaden authority.

```yaml
permission_policy:
  mode: rules
  default: prompt
  rules:
    - capability: filesystem.read
      decision: allow_once
    - capability: process.execute
      decision: deny
```

| Policy mode | Behavior |
|---|---|
| `prompt` | Every supported request is published for explicit decision. |
| `deny` | Every request the adapter can safely deny is denied; if it cannot, the attempt fails closed. |
| `rules` | Applies only rules with an exact normalized capability; `default` must be `prompt` or `deny`. |

An automatic rule can only select a decision present in `available_decisions`.
`allow_once` authorizes exclusively the current request. A persistent or
session-wide approval is only available if the adapter announces that decision
and scope for the concrete interaction. If a rule requests a missing decision,
the result is `unsupported_policy_decision`; Shardeo does not degrade it to
another decision.

`deny` always takes precedence over an automatic authorization that could apply
to the same capability. An unknown capability or an ambiguous mapping never
matches an authorization rule; it uses the safe `default`. Every automatic
resolution publishes `interaction_resolved` with `actor: "policy"` and
references the applied rule.

The initial capability vocabulary includes `filesystem.read`, `filesystem.write`,
`process.execute`, `network.request`, and `unknown`. An adapter can announce
namespaced extensions. The core compares exact identifiers and does not
interpret provider-specific paths, commands, or payloads to create its own
authorizations.

### Persistence, concurrency, and recovery

#### Bundle integrity, permissions, and retention

- The core resolves `realpath` for both the root and each entry, requires
  contained relative paths, and rejects absolute paths, `..` segments, and
  symlink escapes before copying or admitting content.
- Each entry and the whole bundle have configurable byte limits. An entry or sum
  exceeding its limit fails before starting provider work; mandatory context is
  never truncated.
- Files are created with restrictive local permissions where the operating
  system allows it. The bundle does not incorporate secrets or additional
  sources: only material already authorized for the step.
- The adapter recomputes and verifies each digest before admitting an entry. A
  difference from the manifest is treated as drift or tampering, invalidates the
  admission, and fails closed.
- If the provider supports scoped permission rules, the adapter preauthorizes
  read access only to the exact bundle root. It never grants broad project or
  filesystem read access to ease `file_reference` or `tool_read`.
- If it cannot express that exact scope, the adapter must use direct injection,
  attachments, or another native mechanism that does not require broadening
  permissions. The absence of a safe option produces `unsupported_capability`.
- The manifest, the admission proof, and the digests are linked to the attempt's
  evidence. Copies are retained with size and age limits for the period needed
  for audit, resumption, or recovery, and are then removed through verifiable
  cleanup. An active or `orphaned` attempt does not lose its bundle while it can
  still be recovered.

#### Single ownership and lease

- Only one supervisor can own a managed attempt.
- The lease includes attempt identity, fencing generation, owning process
  identity, and heartbeat.
- Every IPC command must target the active endpoint and generation. A supervisor
  with a previous generation cannot persist events nor apply decisions.
- Before replacing an expired lease, Shardeo verifies that the owner is no
  longer active. A reused process identifier is not sufficient evidence to kill
  a process.

#### Idempotent resolution

An interaction transitions via compare-and-set from `pending` to `resolving` and
then to `resolved` or `resolution_failed`. Repeating the same decision returns
the stored result without re-sending it to the harness. Attempting a different
decision after the resolution is acquired returns `interaction_already_resolved`
with the current decision.

Applying to the provider and persisting do not constitute a distributed
transaction. If the supervisor crashes in `resolving`, it only reconciles or
re-sends when the adapter announces explicit, idempotent support. Otherwise it
marks the attempt `orphaned`, keeps the evidence, and requires human recovery;
it never repeats an authorization blindly.

#### Restart and cleanup

When starting or querying an execution, Shardeo identifies expired leases and
absent supervisors. It can reconnect a session only when the adapter announces
`session_resume: supported` and validates the native identity. If it cannot
prove a safe resumption, it marks the attempt `orphaned`, closes only resources
whose ownership it can demonstrate, and keeps output, events, and interactions
for audit.

Cleanup of sockets, lease files, and PTYs happens after persisting the terminal
state. Restarting Shardeo does not automatically turn an `orphaned` attempt into
failed nor create a second supervisor.

### Timeouts and waiting states

The three timers are independent and configurable. They can share the same
default value of 300 seconds, but never the same counter. The process inactivity
timer applies to all three surfaces; the decision timer applies only to
`supervised`, and the human presence timer only to `terminal`:

| Timeout | Runs when | Paused when | Result on expiry |
|---|---|---|---|
| `process_inactivity_seconds` | The harness should be making progress and produces no observable activity | In `supervised`, an interaction is pending; in `terminal`, a human attach is awaited | `process_inactivity_timeout` |
| `interaction_decision_seconds` | The state is `awaiting_interaction` | The interaction enters resolution | `interaction_timeout` |
| `human_presence_seconds` | A `terminal` attempt is active with no person attached, both before the first attach and after a detach | A person is attached | `human_presence_timeout` |

Observable activity means output or a native progress event recognized by the
adapter; an internal heartbeat does not count. Each expiry generates a control
event, asks the adapter for a safe shutdown, and keeps the evidence. If the
shutdown cannot be confirmed, the attempt moves to `orphaned` instead of being
declared finished.

### Control events and full output

Control events contain only identities, states, interactions, decisions, bundle
identity, manifest digest, transport, counts, and bounded summaries. They never
include the full manifest nor the content of its entries. The supervisor
persists the event and its cursor before making it visible. Cursors are
monotonic within the attempt and a uniqueness constraint prevents duplicating
the same semantic event during an internal retry.

The operational output is sanitized incrementally before being written to disk
per attempt and is subject to configurable size and age limits. Paths are
derived only from validated identifiers and must remain under a Shardeo runtime
root; absolute paths, `..`, and symlink escapes are rejected. `step output`
never reveals an internal path as authority for the consumer to open files
directly.

Buffer retention or truncation generates visible metadata, not an attempt
completion. `--tail` returns the last retained lines as a bounded response.
`--full` streams the whole retained buffer once the attempt reaches a terminal
state and is not subject to the evidence projection limit. Evidence follows the
Spec 4 rules: summaries, events, normal command output, and persisted
projections are limited to 16
`KiB (kibibytes: binary units of 1024 bytes)`; `DiagnosticRaw` is the only raw
diagnostic authority, bounded to 1
`MiB (mebibyte: binary unit of 1,048,576 bytes)` and, when the limit is
exceeded, keeps prefix, suffix, original size, and SHA-256 digest. The sanitized
operational buffer does not extend the persisted evidence nor become a second
diagnostic authority.

### Surface comparison

| Criterion | `headless` | `supervised` | `terminal` |
|---|---|---|---|
| Primary case | Automation without live interaction | Machine control through a structured protocol | Human control of a native terminal |
| v1 compatibility | Preserves Spec 4 except the explicit migration of `permission_timeout` to `process_inactivity_timeout` | New adapter capability | New adapter capability |
| Session owner | `step run` | Local supervisor | Local supervisor; the person controls the PTY |
| Permission resolution | Supported policy or terminal termination | `step approve` with an announced decision | The person responds in the native interface |
| Decision channel | None universal | API, SDK, or native protocol through the adapter | Human input on the PTY |
| Tracking | Blocking result | Control polling and on-demand output | Control polling, attach, and on-demand output |
| Risk avoided | Permission bypass | Coupling to a provider's events | Fragile automation or screen scraping of a TUI |

### Acceptance criteria

- Spec 9 is implemented and is the behavioral authority of the `supervised` and
  `terminal` modes; in `headless`, Spec 4 remains the authority of the
  non-interactive path except for the inactivity outcome migration. Spec 9
  replaces only the future, unimplemented
  `interactive: boolean` proposal previously documented in AGENTS.md with
  `headless`, `supervised`, and `terminal`.
- The mode is resolved via
  `step.mode > workflow.mode > config.yaml defaults.agent_mode`, with `headless`
  as default, and `--mode` applies only to the current attempt.
- The core depends on a neutral adapter contract and contains no endpoints,
  flags, fixtures, event names, nor response values of OpenCode, Codex, or any
  other provider.
- The core's context contract is a neutral manifest with `bundle_id`,
  `attempt_id`, creation metadata, and ordered entries; it never contains nor
  requires provider-specific syntax such as `@path`.
- Each entry keeps its logical role, contained relative path, mandatory nature,
  `eager` or `on_demand` loading strategy, size in bytes, and SHA-256 digest.
- Order and boundaries are those of Spec 4: operational instructions, domain
  instructions, skills, required artifacts, and prior or fallback context when
  applicable.
- Operational and domain instructions are mandatory and `eager`; required
  skills and artifacts remain mandatory even when admitted by reference.
  `on_demand` does not allow skipping contractual material.
- Lazy loading can reduce initial transport bytes, but every loaded content
  consumes model tokens; the manifest does not declare that content free of
  context cost.
- The adapter probes the real installation at runtime; a declared version is not
  enough to claim capability.
- Transport selection considers mode, loading strategies, per-entry and total
  limits, digest verification, mandatory availability proof, and scoped read
  permission support.
- An unsupported mode fails before launch with `unsupported_capability`, without
  switching surfaces or granting permissions.
- `headless` preserves Spec 4 behavior, including `permission_required` when the
  adapter cannot resolve the interaction and direct injection through standard
  input when it remains safe and supported, except for the explicit inactivity
  outcome migration from `permission_timeout` to `process_inactivity_timeout`
  (implemented).
- Adopting bundles does not silently change the v1 `headless` path; an adapter
  must announce explicit support before using them in that mode.
- In `supervised`, `step run` starts a supervisor and returns immediately with
  `attempt_id`, `session_id`, state, bundle identity, transport, and initial
  cursor, after freezing and admitting the context and opening the session.
- Before starting `supervised` work, the adapter proves that all mandatory
  entries and their exact bytes are available to the session.
- A supervised permission request is published as `interaction_required` with
  `kind: "permission"`, `interaction_id`, and `available_decisions`.
- `step approve` accepts only a decision announced for that interaction and the
  adapter translates it through its native protocol; there is no universal
  standard-input injection.
- In `terminal`, `step run` creates an attachable PTY, returns `awaiting_human`
  and the command `shardeo step attach <execution-id> <step-id> --attempt-id
  <attempt-id>`.
- Before `awaiting_human`, Shardeo freezes the bundle and the adapter admits the
  context or writes the short bootstrap when starting the harness. The
  orchestrator never writes that bootstrap into the TUI.
- Only one person controls the terminal. The orchestrator does not send
  keystrokes, read screen cells, nor perform screen scraping.
- Detach does not kill the child and reattach works while the session remains
  alive.
- `step events` supports polling by persisted cursor without semantic loss or
  duplication; `step status` does not replace the event history.
- `step output --tail <lines>` returns a sanitized instantaneous view during or
  after the attempt. `step output --full` is only accepted for terminal attempts
  and streams the entire retained and sanitized buffer, with truncation metadata
  when applicable. This channel keeps path containment and bounded retention,
  and does not alter the limits nor the authority of the evidence defined by
  Spec 4.
- Each bundle uses immutable per-attempt copies, contained paths verified by
  `realpath`, per-entry and total limits, restrictive local permissions, and
  drift or tampering detection before admission.
- Bundle retention and removal are bounded and tied to the attempt's evidence,
  resumption, and recovery; no secret material is added beyond what was already
  authorized for the step.
- Preauthorized reads, when they exist, are limited to the exact bundle. No
  adapter silently grants broad project or filesystem access; if it cannot admit
  the context without that broadening, it fails closed.
- `attempt_started` records bundle identity, manifest digest, transport, and the
  number of admitted mandatory entries, without emitting full content in control
  events.
- Commands to the supervisor use explicit local IPC. SQLite keeps state and
  audit records, but is not the live signaling channel.
- A lease with fencing guarantees a single owning supervisor per attempt.
- Compare-and-set resolution is idempotent; a retry does not re-send an already
  applied decision and a conflicting decision fails explicitly.
- A stale supervisor or a crash during resolution is recovered only when the
  adapter can prove safe resume or reconciliation; otherwise the attempt moves
  to `orphaned` without repeating authorizations.
- The process inactivity timeout applies to `headless`, `supervised`, and
  `terminal`; the decision timeout applies only to `supervised` and the human
  presence timeout only to `terminal`. The three use distinct counters and
  errors.
- The default policy is `prompt`; `deny` is supported and every automatic
  authorization requires an exact mapping and a decision announced by the
  adapter.
- Persistent- or session-scoped approvals exist only when the adapter announces
  them for the concrete interaction.
- Launching does not assume a shell or a PTY. The adapter avoids shell
  interpolation unless justified and creates a real PTY for `terminal`.

### Concise complete flows

#### Supervised permission

```bash
$ shardeo step run exec-abc implementation --mode supervised
{"type":"attempt_started","attempt_id":"attempt-9","session_id":"session-9","state":"running","event_cursor":"event-0","context_bundle":{"bundle_id":"bundle-attempt-9","manifest_sha256":"98a1ce2098a1ce2098a1ce2098a1ce2098a1ce2098a1ce2098a1ce2098a1ce20","transport":"api_attachment","required_entries_admitted":4}}

$ shardeo step events exec-abc implementation --attempt-id attempt-9 --after event-0 --limit 50
{"events":[{"cursor":"event-1","type":"interaction_required","interaction":{"interaction_id":"interaction-51","kind":"permission","available_decisions":[{"id":"allow_once","effect":"allow","scope":"request"},{"id":"deny","effect":"deny","scope":"request"}]}}],"next_cursor":"event-1","has_more":false}

$ shardeo step approve exec-abc implementation --interaction-id interaction-51 --decision deny
{"type":"interaction_resolved","interaction_id":"interaction-51","decision":"deny","state":"running","cursor":"event-2"}

$ shardeo step events exec-abc implementation --attempt-id attempt-9 --after event-2 --limit 50
{"events":[{"cursor":"event-3","type":"attempt_completed","exit_code":0}],"next_cursor":"event-3","has_more":false}
```

#### Human handoff

```bash
$ shardeo step run exec-abc architecture --mode terminal
{"type":"attempt_started","attempt_id":"attempt-10","session_id":"session-10","state":"awaiting_human","context_bundle":{"bundle_id":"bundle-attempt-10","manifest_sha256":"ef71a0c4ef71a0c4ef71a0c4ef71a0c4ef71a0c4ef71a0c4ef71a0c4ef71a0c4","transport":"file_reference","required_entries_admitted":4},"attach_command":"shardeo step attach exec-abc architecture --attempt-id attempt-10"}

# In another terminal, run by the person:
$ shardeo step attach exec-abc architecture --attempt-id attempt-10
Attached to attempt-10. Use the configured detach sequence to keep the child alive.

# The orchestrator only observes the status:
$ shardeo step status exec-abc architecture --attempt-id attempt-10
{"attempt_id":"attempt-10","state":"running","human_attached":true,"event_cursor":"event-2"}
```

## Spec 10: Semantic model/provider override in `step run` with fallback to `steps[].agents`

status: draft

### Objective

Let the orchestrator scale or change the provider/model/variant of a step without editing the `workflow.yaml`, keeping `steps[].agents` as the versionable, traceable default. The CLI override takes priority over the hardcoded list; if the override fails due to the chosen provider/model, Shardeo automatically falls back to the candidates declared in `steps[].agents`.

This approach replaces the literal port of `custom-tools.json` / `initiative_tier_resolver` / `model-tier-agents.ts` to Shardeo: the workflow declares `agents: [{opencode: deepseek/deepseek-v4-flash}]` as the baseline (e.g., tier `medium`), and the orchestrator scales up to `openai/gpt-5.6-sol + xhigh` via `step run` flags, without rewriting YAML or introducing a declarative `agents_command` in the schema.

### Functional description

The workflow declares its candidates hardcoded in `steps[].agents`, as in Spec 4 (only `opencode` in v1, with opaque `model` and optional `variant`). That list is the default and the fallback source. The orchestrator can, on any `shardeo step run`, specify a direct override:

```bash
shardeo step run <execution-id> <step-id> \
  --harness opencode \
  --provider openrouter/deepseek \
  --model deepseek-v4-flash \
  --variant max \
  [--feedback "<text>"]
```

Shardeo validates the flags, prepends the override candidate to the `steps[].agents` list (deduplicating an identical `harness+provider/model+variant`), and executes with the same probe/claim/invoke/heartbeat machinery of Spec 4. If the override fails due to the provider/model, Shardeo automatically continues with the next candidate of `steps[].agents` without requiring orchestrator intervention. If the orchestrator does not specify an override, behavior is identical to Spec 4.

Outside Spec 10, the orchestrator can resolve the model dynamically via DAG composition — without schema sugar — using a `command` that writes `resolved.json` and a later `agent` that depends on it. Neither `agents_command` nor `agentsFile` is added to the schema.

Relationship with the artifacts destination: Spec 10 does not change the artifact validation root nor the containment contract. `.shardeo/artifacts` remains the only root validated by `validateProduces` / `validateRequires` / `validateContainedPath`. Projection toward `.docs/initiatives/**` or any other versioned root is, when needed (e.g., migrating the initiatives pipeline), an explicit `type: command` in the DAG that copies/promotes the artifact, or a dedicated command that resolves the step's path. That decision is recorded in `.docs/spikes/02_migracion-pipeline-initiatives-moldeable.md` and is not part of this spec's acceptance criteria.

### Schema and validation

- `steps[].agents` is extended to support `variant` without breaking compatibility: each entry accepts a `string` (`"opencode"`), a `Record<string, string>` (`{opencode: "openai/gpt-4.1"}`), or a `Record<string, {model: string, variant?: string}>` (`{opencode: {model: "openai/gpt-4.1", variant: "high"}}`). The `VALID_AGENT_IDENTIFIERS` whitelist remains restricted to `opencode` in v1.
- CLI flags are validated before any probe or claim:
  - `--harness` only accepts `opencode` in v1; any other value is `workflow_invalid`.
  - `--provider` must have the form `provider/model` with no spaces or `NUL`; environment tokens are not allowed.
  - `--model` is required when `--provider` or `--variant` is specified.
  - `--variant` without `--model` is a validation error.
  - `--variant` is a string without `NUL` and of bounded length (max 64 UTF-8 bytes).
  - `--harness` alone without `--model` is an error; the override is atomic (harness + model, optionally provider and variant) or it does not exist.
  - If `--provider` is specified, the effective model is `provider/model`; otherwise, it is `model` as-is. Both formats are not accepted simultaneously in a contradictory way.
- A workflow with invalid `agents` remains invalid with the same Spec 4 rejection; the override does not make a malformed workflow valid outside the override.

### Candidate resolution and variants

- When an override exists, the synthetic candidate `({harness: --harness, model: "<provider/model or model>", variant: --variant})` is prepended to the resolved `steps[].agents` list of the workflow, preserving the declared order of `steps[].agents` for fallback. If the synthetic candidate exactly matches (harness + model + variant) an entry of `steps[].agents`, it is deduplicated and appears only once at the beginning.
- `invokeAgent` (`src/utils/agent.ts`) passes `--model <model>` and, when `variant` is present and non-empty, `--variant <variant>` to the `opencode` `spawn` with `shell: false`, `detached`, and unmodified `stdio`. The flag order is `run --format json [--model <model>] [--variant <variant>]`.
- The implementation does not introduce a new context transport type nor modify `assembleAgentContext` / `getArtifactsDir` / `captureArtifactSnapshot`.

### Fallback

- If the attempt with the override candidate fails and `completionReason` is `unknown_error` with a non-null `providerError` (an error coming from the provider/model, e.g., quota, nonexistent model, auth) or `process_start_failed` attributable to the model/provider, Shardeo treats it as fallback and continues with the next candidate of `steps[].agents` using the same `fallbackContext` of up to 2 MiB and the 1 MiB snapshot budget of Spec 4.
- The following `completionReason` values are **terminal** and do not fall back to the next candidate, even when they come from an override: `permission_required`, `permission_timeout`, `context_error`, `artifact_context_too_large`, `adapter_contract_error`, `process_cleanup_failed`, `persistence_error`. `exhaustion` closes the `step run`.
- When the override fails and at least one fallback candidate is available, the provider error is preserved in the attempt's sanitized evidence (up to 16 KiB, `DiagnosticRaw` up to 1 MiB) and the next attempt receives the sanitized projection as `fallbackContext`.
- If the override succeeds, the step remains `running` and requires `step complete` as in Spec 4.

### Persistence and observability

- An idempotent `migrateSpec10` migration is added that creates `step_attempts.variant TEXT` (nullable, no default) compatible with existing databases, with `SQLITE_BUSY` retries as in previous migrations.
- Each attempt persists `agent_used` (harness), `model` (effective `provider/model`), and `variant` (literal value or `NULL` when not specified). `getExecutionStatusSummary` / `shardeo status` expose `variant` per attempt without breaking consumers that ignore it.
- The `steps next` and `workflows describe` summaries do not change; the override belongs to the `step run`, not to the workflow.

### Acceptance criteria

- `shardeo step run <execution-id> <step-id> --harness opencode --provider openrouter/deepseek --model deepseek-v4-flash --variant max` runs the step with that model/variant and persists it; the same step without flags uses `steps[].agents` unchanged with respect to Spec 4.
- The declaration `agents: [{opencode: {model: "openai/gpt-4.1", variant: "high"}}]` in the workflow is valid and is equivalent to `--model openai/gpt-4.1 --variant high` as fallback, with the same underlying `invokeAgent`.
- A workflow that declares `agents: ["opencode"]` without a model remains valid; `variant` is optional and not required at any level.
- Incomplete or invalid flags (`--variant` without `--model`, `--provider` without `--model`, unsupported `--harness`, malformed `provider`, `variant` with `NUL` or empty) fail before probe/claim with a structured error and create no attempt.
- When the override fails with `unknown_error` + non-null `providerError`, Shardeo automatically falls back to the next candidate of `steps[].agents` in the same `step run`; the failed attempt remains persisted with `completion_reason: unknown_error` and `decision: fallback`.
- When the override fails with `permission_required`, `context_error`, `artifact_context_too_large`, `adapter_contract_error`, `process_cleanup_failed`, or `persistence_error`, the step terminates with a terminal `completion_reason`, `decision: terminal`, and does not fall back to the next candidate.
- If the override is identical to a candidate of `steps[].agents`, it runs only once (deduplicated) and the attempt is not duplicated.
- `variant` is persisted per attempt (`TEXT`, nullable) and appears in `shardeo status`; a database created before this spec keeps working after the migration and previous attempts show `variant: null`.
- The behavior of `step complete`, `reopen --cascade`, `skip`, `steps next`, `resume`, and the `requires`/`produces` validation under `.shardeo/artifacts` remains unchanged with respect to Specs 4, 6, 7, and 8.

### Example user flow

```bash
# workflow declares medium as the versionable default
# .shardeo/workflows/initiative-spec/workflow.yaml
# steps:
#   - id: implement
#     type: agent
#     agents: [{opencode: {model: deepseek/deepseek-v4-flash, variant: max}}]

# Orchestrator keeps medium as the default
$ shardeo step run exec-abc implement
→ uses deepseek/deepseek-v4-flash (max) from steps[].agents

# Scaled up to sota without editing the YAML
$ shardeo step run exec-abc implement \
    --harness opencode --provider openai --model gpt-5.6-sol --variant xhigh
→ prepends openai/gpt-5.6-sol (xhigh); if it fails due to quota/model, falls back to deepseek/deepseek-v4-flash

# Alternative declarative form (equivalent to the override) inside the workflow
# agents: [{opencode: {model: "openai/gpt-5.6-sol", variant: "xhigh"}}, {opencode: deepseek/deepseek-v4-flash}]
```

### Non-goals

- Porting `custom-tools.json` / `initiative_tier_resolver` / `model-tier-agents.ts` to Shardeo or introducing a semantic `tier` in the workflow schema. Tiers live in the orchestrator and are translated into `provider/model+variant` when invoking `step run`.
- Adding `agents_command` or `agentsFile` to the schema to resolve candidates dynamically via script. The recommended composition is a `type: command` that writes `.shardeo/artifacts/resolved.json` followed by an `agent` that depends on it; the orchestrator reads the JSON and decides the override per `step run`.
- Changing the artifacts root (`getArtifactsDir`), path containment, or the snapshot/evidence budget of Spec 4. Promotion to `.docs/initiatives/**` is done with an explicit `command` in the DAG (see `.docs/spikes/02_migracion-pipeline-initiatives-moldeable.md`), not with a configurable `artifacts_dir` in this spec.
- Changing the workflow validation order: the full YAML load and validation is repeated on each `step run` before resolving the override, as in Spec 4.
- Introducing a new context transport or modifying the `fallbackContext` format beyond what Spec 4 already defines.