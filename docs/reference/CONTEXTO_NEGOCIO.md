# Project business context

Analyzed project: **Shardeo** (CLI, TypeScript). This document describes exclusively the functional context and business rules of the system, inferred from the actually implemented behavior (source code in `src/`, contracts in `SPECS.md` and `AGENTS.md`). Evidence references point to the files that demonstrate each rule; the document is self-contained and does not require reading them.

---

## 1. Executive summary

Shardeo is a command-line tool that **structures the execution of AI-assisted software development workflows**. It is not an agent or an IDE: it does not decide or reason about the work. Its function is to separate three responsibilities that are normally mixed together in a session with an AI agent:

- **The methodology** (what to do and in what order): defined by humans as declarative *workflows*.
- **The knowledge** (with what context): instructions, conventions and project artifacts.
- **The execution** (who does it): interchangeable AI agent runtimes.

An **orchestrator agent** (another AI CLI) drives the process step by step by asking Shardeo what can be executed, asking it to execute each work unit and deciding when a step is finished. Shardeo validates everything before executing, invokes the appropriate runtime with the correct context and persists traceable evidence of everything that happened.

The problem it solves: sessions with AI agents start from scratch, context is passed inconsistently, there is no record of what was executed or with what result, and switching tools means redoing the configuration. Shardeo turns a team's conventions into reproducible, auditable and resumable processes.

## 2. System objectives

**Main objective:** enable a team to execute development methodologies (design, API contracts, database, lint, implementation, verification, etc.) as structured workflows, with any agent runtime, without losing traceability or continuity.

**Secondary objectives:**

1. **Reproducibility:** any team member can execute the same workflow with the same context and the same validations.
2. **Full traceability:** every agent invocation is recorded (response, feedback, artifacts, tool used, completion reason, metrics).
3. **Resilience:** an interrupted workflow can be resumed from the last known state, even reconstructing lost artifacts.
4. **Provider independence:** models and CLIs are interchangeable runtimes; Shardeo does not compete with them.
5. **Deliberate security:** no permission shortcuts, no hidden interactivity, with explicit limits on what evidence is shown and what is retained.
6. **Methodological neutrality:** Shardeo does not interpret agent responses or incorporate business decisions from the user's domain; those decisions belong to the orchestrator and to the workflow instructions.

## 3. Actors

| Actor | Role |
|---|---|
| **Human user (engineer)** | Defines workflows, skills and conventions; approves permissions when the surface requires it; handles terminal handoffs; decides iterations through the orchestrator. |
| **Orchestrator agent** | AI CLI that queries the available workflows, starts executions, asks which steps are ready, orders their execution, interprets results according to the workflow instructions and materializes its decisions through explicit commands (`step complete`, `step reopen`, `step skip`, `step approve`). |
| **Executor agent (runtime / harness)** | AI process that performs the work of an `agent` step. In v1 the only supported one is OpenCode; the concrete model is optional and opaque to Shardeo. |
| **Shardeo (deterministic executor)** | Validates workflows, resolves the order, injects context, executes `command` steps, manages agent invocation, applies permission policies and persists state and evidence. Never decides methodology. |
| **Local supervisor (managed mode)** | Background process that owns a `supervised`/`terminal` attempt: freezes and admits the context, normalizes events, mediates interactions and applies timers. |

## 4. Domain concepts

- **Workflow.** Methodology declared in `.shardeo/workflows/<name>/workflow.yaml` plus a general instructions file addressed to the orchestrator. Constraints: mandatory name, at least one step, acyclic graph with an entry point, only allowed runtimes.

- **Step.** Work unit. Two types: `command` (deterministic process that self-completes on success) and `agent` (AI work that never self-completes). Properties with single responsibility: `depends_on` defines order; `requires` validates inputs at execution time; `produces` validates outputs after execution; `agents` lists runtime candidates in preference order; `skills` and `instructions` provide context; `mode` selects the execution surface.

- **Execution.** Concrete instance of a workflow with a unique identifier. All its steps are born pending and it has an aggregate state derived from the state of its steps.

- **Attempt.** Each runtime invocation within a step, including automatic fallback invocations and each manual re-execution. It is the minimum unit of evidence and audit.

- **Artifact.** Process knowledge document (plan, architecture decision, contract, report). It lives exclusively under `.shardeo/artifacts/` and is managed through `requires`/`produces`. It is distinguished from source code, which the agent modifies directly in the repository and Shardeo neither tracks nor validates.

- **Generation.** Current valid version of a step's result. Reopening a step invalidates its generation: the files remain on disk and in history for audit, but stop being valid as inputs until a new successful attempt produces a revalidated generation.

- **Execution mode.** Surface on which an `agent` step runs: `headless` (automated and blocking), `supervised` (a local supervisor mediates structured permissions) or `terminal` (a person controls a real terminal). They are not interchangeable variants: they change who is in command and how interactions are resolved.

- **Interaction.** Structured request from the harness that demands an external decision (the minimum supported is the permission). Each interaction announces its valid decisions; Shardeo never invents options or expands authority.

- **Context bundle.** Immutable per-attempt copy of all authorized materials, with cryptographic digests. Guarantees reproducibility even if the live files change during the attempt.

- **Sanitized evidence.** Everything visible is clean and bounded; raw diagnostic bytes have a single authorized depository with SHA-256 digest.

## 5. Business rules

Conventions: **High Confidence** = verified directly in code and/or automated tests; **Medium** = inferred from code plus specification with some nuance; **Low** = plausible inference with partial evidence. The codes cited in quotes (for example `permission_required`) are stable system completion reasons (`completion_reason`).

### Area A — Workflow definition and validation

#### RN-001 — Exclusive runtime in v1
**Rule:** the only valid agent identifier is `opencode`; declaring any other invalidates the whole workflow.
**Conditions:** schema validation on every workflow load.
**Result:** invalid workflow; no operation can use it until corrected.
**Confidence:** High. **Evidence:** `src/schema/workflow.ts` (`VALID_AGENT_IDENTIFIERS` whitelist), `src/utils/workflow.ts` (`validateStepAgents`).

#### RN-002 — Optional and opaque model
**Rule:** the model associated with a candidate is optional and is transferred without interpretation; the semantic validity of the model is the runtime's responsibility, not Shardeo's.
**Conditions:** resolution of the candidates of an `agent` step.
**Result:** the value passes intact to the invoked CLI.
**Confidence:** High. **Evidence:** `src/commands/steps.ts` (candidate resolution), SPECS.md Spec 4.

#### RN-003 — Full revalidation on every operation
**Rule:** the complete YAML is re-read and revalidated before executing a step and on every relevant transition (listing, description, execution start, step query, complete, reopen, skip, resume), because the file can change between operations.
**Conditions:** any command that loads the workflow.
**Result:** if validation fails, the error is returned without invoking CLIs or modifying step or execution state.
**Confidence:** High. **Evidence:** `src/commands/steps.ts` (`cmdStepRun`, `cmdStepComplete`, `cmdStepReopen`, `cmdStepSkip`), `src/commands/resume.ts`.

#### RN-004 — Controlled forward compatibility
**Rule:** unknown top-level workflow fields are tolerated to allow evolution without breaking discovery; instead, the obsolete dependency field `after` is explicitly rejected with a message that directs the use of `depends_on`.
**Conditions:** workflow parsing.
**Result:** new fields do not break; `after` produces a specific validation error.
**Confidence:** High. **Evidence:** `src/schema/workflow.ts` (`.passthrough()` and `after` preprocessor).

#### RN-005 — Mandatory minimum content
**Rule:** every workflow requires a non-empty name and at least one step; without them it is invalid.
**Confidence:** High. **Evidence:** `src/schema/workflow.ts` (`WorkflowFile`).

### Area B — Graph and execution order

#### RN-006 — Unique identifiers
**Rule:** step IDs must be unique within the workflow; a duplicate makes references ambiguous and invalidates the workflow with an explicit error.
**Confidence:** High. **Evidence:** `src/utils/workflow.ts` (`analyzeWorkflowGraph`, code `duplicate_step_id`).

#### RN-007 — Existing dependency references
**Rule:** every `depends_on` must reference an existing step of the same workflow.
**Result:** otherwise the execution is rejected with the code `invalid_depends_on_reference` indicating the broken reference.
**Confidence:** High. **Evidence:** `src/utils/workflow.ts` (`analyzeWorkflowGraph`).

#### RN-008 — Mandatory entry point
**Rule:** at least one step without `depends_on` must exist; it is the workflow's entry gate.
**Result:** without an entry point, `shardeo run` refuses to start the execution (`no_entry_point`).
**Confidence:** High. **Evidence:** `src/utils/workflow.ts` (`analyzeWorkflowGraph`), `src/commands/run.ts`.

#### RN-009 — Acyclic graph
**Rule:** dependency cycles cannot exist; the error identifies the complete closed path of the steps involved.
**Confidence:** High. **Evidence:** `src/utils/workflow.ts` (iterative DFS with cycle detection and path).

#### RN-010 — depends_on as the only ordering criterion
**Rule:** only `depends_on` defines precedence. `requires` and `produces` validate data, but never create order or implicit parallelism.
**Confidence:** High. **Evidence:** `src/utils/dag.ts`, README, SPECS.md Specs 3/5.

#### RN-011 — Step availability
**Rule:** a step is available when all its declared predecessors reached a positive terminal state: `completed` or `skipped`. A skipped step satisfies the order even though it produces nothing.
**Conditions:** computation of `steps next`.
**Confidence:** High. **Evidence:** `src/utils/dag.ts` (`getReadySteps`), `src/db/queries.ts` (completed+skipped set), `src/commands/steps.ts::cmdStepsNext`.

#### RN-012 — Blocking by incomplete predecessors
**Rule:** attempting to execute a step whose predecessors are not terminal fails with an explicit error enumerating the incomplete predecessors; there is no forced execution.
**Confidence:** High. **Evidence:** `src/commands/steps.ts::cmdStepRun` (prior check).

#### RN-013 — Parallelism delegated to the orchestrator
**Rule:** if several steps are available simultaneously, they are all returned together; the decision to execute them in parallel or in series belongs to the orchestrator. Shardeo never executes anything in parallel by itself.
**Confidence:** High. **Evidence:** `src/commands/steps.ts::cmdStepsNext`, SPECS.md Spec 5.

### Area C — Basic step execution

#### RN-014 — Atomic execution creation
**Rule:** `shardeo run` validates the workflow and creates the execution together with all its steps in pending state in a single atomic operation, returning a unique identifier. An invalid workflow generates no record.
**Confidence:** High. **Evidence:** `src/commands/run.ts`, `src/db/queries.ts` (`createExecutionWithSteps`).

#### RN-015 — States that prevent execution
**Rule:** a step in `skipped` state cannot be executed; neither can an `agent` step already `completed`. `failed` or `pending` steps can be re-executed.
**Result:** error with the current state; no changes.
**Confidence:** High. **Evidence:** `src/commands/steps.ts::cmdStepRun`.

#### RN-016 — Self-completion exclusive to command steps
**Rule:** a `command` step is marked completed automatically when the process exits with code 0 and all its `produces` exist and are valid under `.shardeo/artifacts/`. If the command fails, or succeeds but the artifacts do not appear (or the snapshot exceeds limits), the step ends failed with the corresponding reason.
**Exceptions:** an empty command is considered trivially successful (code 0).
**Confidence:** High. **Evidence:** `src/commands/steps.ts::runCommandStep`.

#### RN-017 — Agent steps never self-complete
**Rule:** a successful agent invocation leaves the step in `running` state, not `completed`. Only the orchestrator can close it by invoking `step complete`, at which point Shardeo revalidates the artifacts declared in `produces`.
**Conditions:** `agent` type steps.
**Confidence:** High. **Evidence:** `src/commands/steps.ts::cmdStepComplete`, SPECS.md Spec 4.

#### RN-018 — Conditions to complete an agent step
**Rule:** `step complete` requires the step to be `running`, at least one attempt finished with a valid snapshot to exist, and the whole current set of `produces` to be present and safe. If anything is missing, the command fails and the step remains in progress.
**Confidence:** High. **Evidence:** `src/commands/steps.ts::cmdStepComplete` (`no_completed_attempts`, `no_snapshot_data`, `incomplete_generation_outputs`).

#### RN-019 — Input validation at execution time
**Rule:** the artifacts declared in `requires` are validated when executing the step (never when resolving the order). If they are missing, the step is not executed and the list of absent ones is returned. If they exist but come from invalidated generations, they are reported as stale (`stale_required_artifacts`) even though the physical file exists.
**Confidence:** High. **Evidence:** `src/utils/execution.ts` (`validateRequires`, `validateRequiresForExecution`).

### Area D — Candidates, fallback and terminal reasons

#### RN-020 — Candidates only from agents[], in order
**Rule:** runtime candidates come exclusively from the step's `agents` list and are traversed in the declared order. Each candidate is probed lazily; an unavailable one is discarded without creating an attempt.
**Confidence:** High. **Evidence:** `src/commands/steps.ts` (`probeNextAgentCandidate`, `EvaluatedProbe` records).

#### RN-021 — No available candidates
**Rule:** if no probe passes validation before invoking, the step fails showing sanitized evidence of all evaluated candidates and no attempt is created.
**Confidence:** High. **Evidence:** `src/commands/steps.ts::runAgentStep` (response with `evaluated_probes` and `attempts: []`).

#### RN-022 — Strict success
**Rule:** an invocation is successful only if the process ended cleanly, the JSONL stream was valid and there was no error event with exit 0. In that case the attempt is persisted as a success and the step stays `running`.
**Confidence:** High. **Evidence:** `src/utils/agent.ts` (`invokeAgent`, `finishClosedProcess`).

#### RN-023 — Fallback without semantic classification
**Rule:** after an invocation started and cleaned up correctly with valid JSONL, any clean error advances to the next candidate without classifying its cause: quota, context, availability or model are treated the same (generic reason `unknown_error` with the concrete provider error preserved in sanitized evidence). The new attempt receives extended context with the original plus the projection of what happened.
**Business rationale:** OpenCode does not expose a stable error taxonomy; misclassifying them would be worse than not classifying them.
**Confidence:** High. **Evidence:** `src/utils/errors.ts`, `src/utils/agent.ts::buildFallbackEvidence`, SPECS.md Spec 4.

#### RN-024 — Enumerated terminal reasons
**Rule:** there is a closed list of reasons that prevent fallback and fail the step immediately: `permission_required`, `permission_detection_unsupported`, `permission_timeout`, `process_inactivity_timeout`, `interaction_timeout`, `artifact_context_too_large`, `adapter_contract_error`, `process_start_failed`, `process_cleanup_failed`, `context_error` and `persistence_error`.
**Confidence:** High. **Evidence:** `src/utils/errors.ts` (`TERMINAL_COMPLETION_REASONS`), used in `src/commands/steps.ts`.

#### RN-025 — Candidate exhaustion
**Rule:** if the last available candidate fails with a non-terminal reason, the step ends failed by exhaustion (`exhaustion`) with the detail of all the attempts made.
**Confidence:** High. **Evidence:** `src/commands/steps.ts::runAgentStep`.

#### RN-026 — Every invocation is an independent attempt
**Rule:** every CLI invocation (manual, by fallback or by re-execution) is recorded as a separate attempt with number, full or partial response, feedback, generated artifacts, runtime and model used, completion reason and time metrics.
**Confidence:** High. **Evidence:** `src/db/queries.ts` (`finalizeAttemptCompleted`, `finalizeAttemptAndInsertNext`, `finalizeAttemptAndFailStep`), SPECS.md Spec 6.

### Area E — Context delivered to the agent

#### RN-027 — Contractual context order
**Rule:** the context injected into the agent always respects this order: operational instructions (generated by Shardeo), step domain instructions, skills, required artifacts, artifacts generated in previous attempts, reopen feedback, user feedback and, lastly, fallback evidence if any.
**Confidence:** High. **Evidence:** `src/utils/agent.ts` (`assembleAgentContext`, `buildContextWithContainment`).

#### RN-028 — Size budgets per component
**Rule:** each context component has maximum limits: instructions 256 KiB (per file and cumulative); skills 256 KiB per file and 1 MiB cumulative; required artifacts 1 MiB per file and cumulative; artifacts from previous attempts 2 MiB cumulative. Exceeding any budget invalidates the context and the step fails closed with `context_error` before invoking the agent. Non-UTF-8 binary content is replaced by a marker with its size and digest.
**Confidence:** High. **Evidence:** `src/utils/agent.ts` (`CONTEXT_LIMITS`, `readContainedText`, `decodePriorArtifactText`).

#### RN-029 — Iteration with memory
**Rule:** a non-completed step can be re-executed without limit. Each re-execution receives the artifacts generated in previous attempts and, if provided, the user feedback clearly delimited within the context. Feedback must be non-empty text.
**Confidence:** High. **Evidence:** `src/commands/steps.ts::cmdStepRun` (`--feedback`), `getPriorAttemptArtifacts`, SPECS.md Spec 6.

### Area F — Timeouts and activity

#### RN-030 — Resettable inactivity timeout
**Rule:** an agent invocation ends with `process_inactivity_timeout` when it produces no observable activity during the configured period (5 minutes by default, project-configurable). Any process output event restarts the counter; an internal heartbeat does not count as activity.
**Confidence:** High. **Evidence:** `src/utils/agent.ts` (inactivity timer, `DEFAULT_INACTIVITY_TIMEOUT_MS`), `src/schema/config.yaml` via `src/schema/config.ts`.

#### RN-031 — Fixed ceiling for commands
**Rule:** `command` steps have an absolute maximum execution time of 300 seconds, not configurable at step level.
**Confidence:** High. **Evidence:** `src/commands/steps.ts::runCommandStep` (fixed timeout), `src/utils/execution.ts` (`runCommand`).

#### RN-032 — Independent timers per surface
**Rule:** there are three timers with distinct counters and errors: process inactivity (applies to all three surfaces), interaction decision (only `supervised`) and human presence (only `terminal`). When any expires, safe shutdown preserving evidence is requested; if shutdown cannot be confirmed, the attempt is marked `orphaned` instead of being declared finished.
**Confidence:** High. **Evidence:** SPECS.md Spec 9 (timeout table), `src/runtime/terminal.ts`, `src/runtime/supervisor.js`.

### Area G — Permissions and interactions

#### RN-033 — Permission detection only by exact signals
**Rule:** in headless mode, an agent approval request is recognized only through exact, versioned harness signals. A match terminates the attempt with `permission_required`. An ambiguous or unrecognized output is not guessed: it fails closed with `permission_detection_unsupported` or `adapter_contract_error`.
**Business rationale:** prefer failing over authorizing something without certainty.
**Confidence:** High. **Evidence:** `src/adapters/opencode.ts` (detector with version identity and self-rejection signal), `src/utils/agent.ts` (detector consumption).

#### RN-034 — Never a permission bypass
**Rule:** agents are invoked in non-interactive mode without shell and without permission-skipping flags; those flags are never injected by the system. If a step fails because the agent needed to ask, the problem is solved by improving the instructions or using `supervised`/`terminal`.
**Confidence:** High. **Evidence:** `src/utils/agent.ts` (`spawn` with `shell: false`), README, AGENTS.md.

#### RN-035 — Layered permission policy and announced decisions
**Rule:** the permission policy is inherited by layers (`step > workflow > config`, default value `prompt`; lists are not mixed between layers). In managed modes, any automatic authorization requires exact normalized capability matching and that the decision is announced for the concrete interaction; `deny` always precedes any automatic authorization; an unknown capability never matches authorization rules and uses the safe default; a rule requesting a non-announced decision fails without degrading. `allow_once` authorizes exclusively the current request.
**Confidence:** High. **Evidence:** SPECS.md Spec 9 (permission policy), `src/runtime/supervisor.js` (policy application with actor `policy`).

#### RN-036 — Unique and idempotent resolution
**Rule:** each interaction transitions once from pending to resolved through compare-and-swap. Repeating the same decision returns the stored result without re-forwarding it; attempting a different decision after resolution fails with `interaction_already_resolved`. Every automatic resolution is recorded with actor `policy` and a reference to the applied rule.
**Confidence:** High. **Evidence:** SPECS.md Spec 9 (idempotent resolution), interaction broker.

### Area H — Evidence, sanitization and data

#### RN-037 — Bounded visible output and single raw authority
**Rule:** everything shown on the console and persisted in standard projections is sanitized (no host absolute paths or control characters) and limited to 16 KiB per attempt. Unredacted diagnostic bytes have a single authorized depository (`DiagnosticRaw`): up to 1 MiB the exact frame is kept; above that, prefix, suffix, original size and SHA-256 digest are stored.
**Confidence:** High. **Evidence:** `src/utils/agent.ts` (`MAX_CONSOLE_BYTES`, `MAX_DIAGNOSTIC_RAW_BYTES`, `encodeDiagnosticRaw`), README.

#### RN-038 — Bounded fallback context
**Rule:** the projection delivered to the next candidate after a failure with fallback has a cumulative budget of 2 MiB per step execution; it includes concrete provider errors and references to previous attempts, never raw evidence. Exceeding the budget terminates with `artifact_context_too_large`.
**Confidence:** High. **Evidence:** `src/utils/agent.ts` (`MAX_FALLBACK_BYTES`, `buildFallbackEvidence`), `src/commands/steps.ts::runAgentStep`.

#### RN-039 — Snapshots with cumulative budget
**Rule:** before each attempt a baseline of the artifacts declared in `produces` is captured; afterwards only new or modified files are snapshotted, each with its SHA-256. The cumulative content per step has a ceiling of 1 MiB: exceeding it produces `artifact_context_too_large` and fails the attempt.
**Confidence:** High. **Evidence:** `src/utils/agent.ts` (`MAX_STEP_SNAPSHOT_BYTES`), `src/commands/steps.ts::runAgentStep`.

### Area I — Artifacts

#### RN-040 — Single root and strict containment
**Rule:** all artifacts live exclusively under `.shardeo/artifacts/`. Paths declared in `requires` and `produces` reject absolute paths, `..` segments and escapes through symlinks, for both `agent` and `command` steps. A symlink whose target stays within the root does not escape.
**Business rationale:** artifacts are the currency of exchange between steps; their integrity and predictable location sustain traceability and security.
**Confidence:** High. **Evidence:** `src/utils/agent.ts` (`validateContainedPath`, `isParentEscape`), `src/utils/execution.ts`.

#### RN-041 — Generational validity of artifacts
**Rule:** a required artifact is only valid if it comes from the producer whose current generation matches the current state (step completed, no invalidation) and whose generation manifest (paths, sizes, digests) matches the physical files. An orphan artifact or one from an invalidated generation is reported as stale even though it exists on disk.
**Confidence:** High. **Evidence:** `src/utils/execution.ts::validateRequiresForExecution`, `src/db/queries.ts` (`step_generation_invalidations`).

#### RN-042 — Overwriting and history
**Rule:** artifacts generated in a re-execution overwrite those from previous attempts on the filesystem, but the previous versions remain recorded in the database as evidence. The `produces` validation at completion always applies to the current set.
**Confidence:** High. **Evidence:** SPECS.md Spec 6, `getPriorAttemptArtifacts` in `src/db/queries.ts`.

#### RN-043 — Nature of produced artifacts
**Rule:** to validate a generation, every produced artifact must be a regular file within the root; symlinks and other file types are rejected in that validation.
**Note:** the written contract contemplated allowing internal symlinks; the code implemented the stricter criterion. See section 12.
**Confidence:** Medium. **Evidence:** `src/utils/execution.ts::inspectProducedArtifacts` versus SPECS.md Spec 4.

### Area J — Orchestrator-driven iteration

#### RN-044 — Skip only virgin work
**Rule:** `step skip` only admits pending steps with no attempt started and no valid generation; it requires a mandatory reason. The result is terminal.
**Confidence:** High. **Evidence:** `src/db/queries.ts::skipStepAtomic`, `src/commands/steps.ts::cmdStepSkip`.

#### RN-045 — Functional effect of skipping
**Rule:** a skipped step satisfies the order (releases its dependents) but generates no artifacts and does not satisfy `requires` validations: any dependent that needs its outputs will fail normally when validating inputs.
**Confidence:** High. **Evidence:** SPECS.md Spec 8, RN-011 and RN-019.

#### RN-046 — Reopen only finished work
**Rule:** `step reopen` admits only `completed` or `failed` steps, with mandatory feedback (non-empty text up to 16 KiB). Pending, running or skipped steps cannot be reopened.
**Confidence:** High. **Evidence:** `src/db/queries.ts::reopenStepAtomic`, `src/commands/steps.ts::cmdStepReopen`.

#### RN-047 — Mandatory cascade when affected descendants exist
**Rule:** if reopening a step would invalidate descendants in `running`, `completed` or `failed` state and `--cascade` is not provided, the command fails enumerating which steps would be restarted. With cascade, the affected descendants return to pending, their generation number is incremented and the invalidation is recorded; their attempt history is preserved.
**Confidence:** High. **Evidence:** `src/db/queries.ts::reopenStepAtomic` (`cascade_required`, `cascade_reset`).

#### RN-048 — Generational invalidation on reopen
**Rule:** reopening invalidates the step's current generation: its artifacts remain on disk and in history for audit, but they do not satisfy `requires` nor allow completing again until at least one new attempt finishes successfully and the current `produces` set is revalidated as a new generation. Files that did not need changes can keep the same bytes: validity belongs to the generation, not to the rewrite.
**Confidence:** High. **Evidence:** `src/db/queries.ts` (`insertGenerationInvalidation`), SPECS.md Spec 8.

#### RN-049 — Occupancy and reconciliation
**Rule:** a reopen or skip transition fails if a live attempt with a valid lease exists (step occupied). Attempts with expired leases are automatically reconciled as interrupted before proceeding. Every transition records reason, actor, timestamp and origin/destination states in an audit log.
**Confidence:** High. **Evidence:** `src/db/queries.ts` (`reopenStepAtomic`, `reconcileInterruptedAttempt`, `step_transition_events`).

### Area K — Aggregate state and resumption

#### RN-050 — Transactionally derived aggregate state
**Rule:** the execution state is recalculated with every step transition, within the same transaction: if any step failed, the execution is `failed`; if all steps are `completed` or `skipped`, it is `completed`; in any other case, `running`. The status query never shows an aggregate that diverges from the stored one.
**Confidence:** High. **Evidence:** `src/db/queries.ts::syncExecutionStatusInTransaction`, SPECS.md Spec 7.

#### RN-051 — Safe resumption
**Rule:** `resume` requires the associated workflow not to have changed since the execution started (same step set); if it changed, it fails with `workflow_changed`. It reconciles interrupted attempts (expired leases), does not re-execute completed steps and returns explicit continuation guidance.
**Confidence:** High. **Evidence:** `src/commands/resume.ts`.

#### RN-052 — Reconstruction of lost artifacts
**Rule:** if a completed step lost its files on disk, `resume` marks it as "requires reconstruction" with the list of missing files and delivers the agent's last response as context to reconstruct them without starting from scratch.
**Confidence:** High. **Evidence:** `src/commands/resume.ts` (`markStepReconstructionRequired`, `previous_agent_response`), SPECS.md Spec 7.

### Area L — Execution surfaces

#### RN-053 — Layered mode resolution
**Rule:** the effective mode of an `agent` step is resolved as: explicit command override (only for that attempt) over `step.mode`, then `workflow.mode`, then the project config default; if nobody declares it, `headless`. An invalid value in any layer is an error. `command` steps ignore modes entirely.
**Confidence:** High. **Evidence:** `src/utils/mode.ts::resolveEffectiveMode`, `src/schema/mode.ts`, SPECS.md Spec 9.

#### RN-054 — Probed capabilities and closed failure
**Rule:** managed modes require exactly one adapter candidate and that it announces real mode support after probing the concrete installation. An unsupported mode or capability produces `unsupported_capability` before launching provider work; there is never a silent surface change or permission expansion.
**Confidence:** High. **Evidence:** `src/commands/steps.ts::cmdStepRun` (prior probe), `src/runtime/terminal.ts` (`terminalCapabilityError`).

#### RN-055 — Supervised: non-blocking start and event-based tracking
**Rule:** in `supervised`, starting the attempt freezes and admits the context and opens the session; then the command returns immediately with identities and initial cursor. Tracking is done by querying structured events with monotonic cursors (no semantic losses or duplicates) and reading output on demand; control events contain identities and summaries, never the full output.
**Confidence:** High. **Evidence:** SPECS.md Spec 9 (command and event contract), `src/runtime/supervisor.js`.

#### RN-056 — Terminal: exclusively human control
**Rule:** the `terminal` mode delivers a real terminal to a person. The orchestrator only communicates the handoff: it never writes keys, does not read the screen or scrape the interface. Connecting requires the exact identifier of the single live attempt (never attached by ambiguous inference); disconnecting does not kill the process and reconnection is possible while the attempt remains active within its human presence window.
**Confidence:** High. **Evidence:** SPECS.md Spec 9 (human handoff), `src/runtime/terminal.ts`.

#### RN-057 — Orphaned attempts under human custody
**Rule:** an attempt whose shutdown could not be confirmed stays `orphaned`: it never auto-marks itself as failed, is not automatically restarted and never repeats an authorization blindly; it keeps all its evidence and requires human recovery. Restarting Shardeo does not turn orphans into failures nor create second supervisors.
**Confidence:** High. **Evidence:** SPECS.md Spec 9 (persistence, concurrency and recovery), state machine of `src/runtime/terminal.ts`.

### Area M — Managed context integrity

#### RN-058 — Immutable bundle with proven admission
**Rule:** in managed modes, the context is materialized as immutable per-attempt copies with SHA-256 digests and preserved contractual order. Before the provider works, the adapter must prove that all mandatory inputs became available with those exact bytes; any difference is treated as drift or tampering and fails closed. Mandatory context is never truncated: an over-limit failure happens before starting.
**Confidence:** High. **Evidence:** SPECS.md Spec 9 (bundle and manifest), `src/runtime/bundle.ts`.

#### RN-059 — Minimal pre-authorized reading
**Rule:** if the provider allows pre-authorizing reads, the scope is only the exact bundle root; never broad access to the project or the filesystem. If that safe scope cannot be expressed, another native mechanism is used or it fails with `unsupported_capability`.
**Confidence:** High. **Evidence:** SPECS.md Spec 9 (bundle integrity and permissions).

#### RN-060 — Uniform exit codes
**Rule:** every command ends with code 0 on success and 1 on error, consistently for programmatic consumption by the orchestrator.
**Confidence:** High. **Evidence:** `src/utils/errors.ts` (`ExitCode`).

## 6. Main business flows

### Flow F1 — Project preparation
1. **Initial condition:** any repository without `.shardeo/`.
2. **Actors:** human user + Shardeo.
3. **Steps:** the user installs the CLI globally and runs the initialization; Shardeo creates the configuration structure (workflows, skills, artifacts, docs) with valid defaults.
4. **Rules:** total idempotency: if the structure exists, it reports without overwriting anything; without write permissions it fails with a clear message.
5. **Result:** project ready to define workflows.
6. **Exceptions:** pre-existing structure (non-destructive); lack of permissions.
7. **References:** RN-060. **Evidence:** SPECS.md Spec 1.

### Flow F2 — Discovery
1. **Initial condition:** workflows defined in the project.
2. **Actors:** orchestrator or human + Shardeo.
3. **Steps:** list workflows with name and description; describe one to see general instructions and steps with type, dependencies, inputs and outputs.
4. **Rules:** both commands validate the YAML before responding; an invalid workflow appears marked as such with its concrete error; having no workflows is information, not an error.
5. **Result:** the orchestrator can present informed options to the user.
6. **Exceptions:** non-existent workflow (error with list of available ones); malformed YAML (marked, not hidden).
7. **References:** RN-003. **Evidence:** SPECS.md Spec 2.

### Flow F3 — Workflow execution
1. **Initial condition:** valid workflow.
2. **Actors:** orchestrator (conductor), executor agent or deterministic processes, Shardeo.
3. **Steps:** start execution (unique identifier, pending steps) → query available steps → execute each step → complete the `agent` ones → repeat until finished.
4. **Rules:** RN-006 to RN-019 (graph, order, command self-completion, manual agent closure).
5. **Result:** execution `completed` when all steps reach a positive terminal state.
6. **Exceptions:** step failure (`failed` infects the aggregate, RN-050); workflow modified mid-execution (RN-051).

### Flow F4 — Headless agent step with fallback
1. **Initial condition:** `agent` step available in headless mode.
2. **Actors:** orchestrator, Shardeo, OpenCode runtime (candidates).
3. **Steps:** revalidate workflow → validate inputs → probe candidates in order → build bounded context → invoke without permission bypass → classify the result (success / fallback / terminal) → persist attempt with sanitized evidence → if fallback, repeat with extended context.
4. **Rules:** RN-020 to RN-026, RN-027 to RN-039.
5. **Expected result:** success leaves the step `running` awaiting explicit closure.
6. **Exceptions:** permission requested (`permission_required`), inactivity (`process_inactivity_timeout`), budget exceeded (`artifact_context_too_large`), invalid context (`context_error`), harness contract violated (`adapter_contract_error`), exhaustion (`exhaustion`).
7. **Evidence:** `src/commands/steps.ts::runAgentStep`, `src/utils/agent.ts`.

### Flow F5 — Directed iteration (reopen / skip)
1. **Initial condition:** the result of a later step reveals incorrect previous work, or a pending step turns out unnecessary according to the workflow instructions.
2. **Actors:** orchestrator decides; Shardeo validates and persists; the user approves according to the workflow instructions.
3. **Steps:** reopen with feedback (with cascade if affected descendants exist) or skip with reason → re-execute → re-complete.
4. **Rules:** RN-044 to RN-049.
5. **Result:** new current generation of the corrected work; descendants coherently restarted; complete history for audit.
6. **Exceptions:** required cascade not provided; occupied step; unsupported states.
7. **Business note:** Shardeo never decides when to iterate; it only materializes the orchestrator's decision (SPECS.md Spec 8).

### Flow F6 — Resumption after interruption
1. **Initial condition:** execution stopped by error, terminal closure or cancellation.
2. **Actors:** orchestrator/human + Shardeo.
3. **Steps:** resume → reconcile interrupted attempts → verify artifacts of completed ones → mark reconstructions → continue with pending steps delivering previous responses as context.
4. **Rules:** RN-050 to RN-052, RN-029.
5. **Result:** continuity without repeating completed work or losing accumulated knowledge.
6. **Exceptions:** changed workflow (blocking); lost artifacts (reconstruction path).

### Flow F7 — Supervised permission
1. **Initial condition:** agent step configured (or forced per attempt) in `supervised` mode.
2. **Actors:** orchestrator, local supervisor, harness, permission policy.
3. **Steps:** start with frozen and admitted context → immediate return → the harness asks for permission → the request is published as an interaction with valid decisions → the orchestrator queries events and issues an announced decision → idempotent resolution applied by native protocol.
4. **Rules:** RN-032, RN-035, RN-036, RN-054, RN-055, RN-058.
5. **Result:** mediated permission without exposing the provider's internal contracts or expanding authority.
6. **Exceptions:** decision timeout (`interaction_timeout`); crash during resolution (orphan, RN-057); missing capability (`unsupported_capability`).

### Flow F8 — Human handoff in terminal
1. **Initial condition:** agent step in `terminal` mode.
2. **Actors:** orchestrator (only communicates), person (controls), supervisor.
3. **Steps:** start with attachable PTY → return with the exact connection command → the person connects from another terminal and works natively → when the harness finishes, the supervisor persists the result.
4. **Rules:** RN-032, RN-054, RN-056, RN-058.
5. **Result:** real interactive work without fragile interface automation.
6. **Exceptions:** expiry without human presence; multiple reconnections allowed; ambiguous attempt identity (explicit failure).

## 7. States and transitions

### Step (within an execution)

| State | Functional meaning |
|---|---|
| `pending` | Waiting for its predecessors to reach a positive terminal state. |
| `running` | Has at least one live attempt or one finished attempt pending closure; a successful agent leaves the step here. |
| `completed` | Accepted work with validated current generation. |
| `failed` | The last attempt ended in a terminal reason or the command failed. |
| `skipped` | Skipped by orchestrator decision; terminal and with no production. |

Allowed transitions:

- `pending → running` (execution claimed with lease).
- `running → completed` (only `step complete` for agents, validating produces; automatic for successful command).
- `running → failed` (terminal reason, command failure or exhaustion).
- `failed → running` (direct re-execution allowed).
- `completed → pending` and `failed → pending` (reopen with feedback; invalidates generation).
- `pending → skipped` (skip only virgin work).

Forbidden transitions: executing `skipped` steps or `completed` agents; completing non-running steps; skipping steps with attempts or in any state other than pending; reopening `pending`, `running` or `skipped`. The terminal states of a step are only abandoned through the explicit reopen/skip transitions described.

### Execution (aggregate)

`running → completed` when all its steps are `completed`/`skipped`; `running → failed` if any failed. Reopening can return a closed execution to `running` (the aggregate is recalculated transactionally). Forbidden: divergence between what is shown and what is stored.

### Managed attempt (`supervised` / `terminal`)

States: `starting`, `awaiting_human`, `running`, and terminal `completed`, `failed`, `orphaned`.

Valid transitions: `starting → awaiting_human|failed|orphaned`; `awaiting_human → running|completed|failed|orphaned`; `running → awaiting_human|completed|failed|orphaned`. The three terminal states have no exit (an orphan never comes back to life by itself). **Evidence:** state machine in `src/runtime/terminal.ts`.

### Interaction

`pending → resolving → resolved | resolution_failed`, through idempotent compare-and-swap; a contrary decision after resolution fails. **Evidence:** SPECS.md Spec 9.

## 8. Permissions and restrictions

| Actor | Action | Condition |
|---|---|---|
| Orchestrator | Start, query and execute steps | Valid workflow; terminalized predecessors; step not skipped nor (if agent) completed |
| Orchestrator | Complete an agent step | Step `running`; complete and safe produces |
| Orchestrator | Reopen / skip | Admitted states (RN-044, RN-046); mandatory feedback/reason; cascade when applicable |
| Orchestrator | Approve supervised interactions | Only decisions announced by that interaction |
| Automatic policy | Authorize/reject permissions | Exact capability match + announced decision; `deny` precedes; actor recorded as `policy` |
| Person | Control terminal in `terminal` mode | Attach with the exact identity of the live attempt |
| Executor agent | Write code in the repository | Free, guided by instructions; Shardeo does not track code |
| Executor agent | Leave artifacts | Only under `.shardeo/artifacts/` with strict containment |
| Shardeo | Decide methodology (branches, conditionals, quality) | Never: that authority belongs to the orchestrator and to the workflow instructions |
| Any process | Permission bypass | Forbidden by design: no skipping flags or interactive shell |

## 9. Calculations and business decisions

**Operational numeric limits** (constants verified in `src/utils/agent.ts`):

| Limit | Value | Effect when exceeded |
|---|---|---|
| Visible output/projections | 16 KiB per attempt | Sanitized truncation, never an error |
| Raw diagnostics (`DiagnosticRaw`) | Exactly 1 MiB; above that prefix+suffix+size+SHA-256 | Bounded retention with digest |
| Cumulative snapshot per step | 1 MiB of new/modified content | Attempt fails (`artifact_context_too_large`) |
| Fallback context | 2 MiB cumulative per step execution | Closed failure |
| Instructions (per file and total) | 256 KiB | Invalid context (`context_error`) before invoking |
| Skills | 256 KiB/file, 1 MiB cumulative | Same |
| Required artifacts | 1 MiB per file and cumulative | Same |
| Cumulative prior artifacts | 2 MiB | Same |
| Agent inactivity | 300 000 ms by default (configurable) | `process_inactivity_timeout` |
| Command execution | Fixed 300 s | Step failure |
| Candidate probe | 5 s | Candidate discarded |
| Attempt lease / heartbeat | 30 s / every 10 s | Reconciliation of interrupted ones |

**Derived decisions:**

- *Attempt result classification:* strict success → `success`; reason in terminal list → `terminal`; any other clean error → `fallback` (RN-022 to RN-025).
- *Availability:* `depends_on ⊆ {completed ∪ skipped}` and own state `pending` (RN-011).
- *Execution aggregate:* `failed > completed = all terminalized positive > running` in that order of precedence (RN-050).
- *Mode resolution:* the first layer that declares wins; default `headless` (RN-053).
- *Deterministic order:* the descendants affected by a cascade are processed in stable UTF-8 binary order, guaranteeing reproducible audit.
- *Security priority over uncertainty:* in the face of ambiguity (unrecognizable permission, unknown capability, non-announced decision, bundle drift), the system fails closed instead of guessing.

## 10. Special cases and exceptions

- **Empty command:** a `command` step without command text completes trivially with code 0 (useful as markers or synchronization points in the graph). Evidence: `runCommandStep`.
- **Non-existent vs. failed candidates:** non-installed candidates generate no attempt and do not count as a provider failure; only sanitized probe evidence is recorded. Evidence: RN-020/RN-021.
- **Harness self-rejection:** when OpenCode itself rejects a permission in non-interactive mode, the versioned signal is recognized and treated as denied-permission evidence, not as a generic error. Evidence: commit H-LIVE-03, `src/adapters/opencode.ts`.
- **Binary content in prior artifacts:** if an artifact from a previous attempt is not valid UTF-8 text, it is delivered to the next attempt as a marker with size and digest instead of the content. Evidence: `decodePriorArtifactText`.
- **Visible invalid workflows:** they are not hidden in the listing: they appear marked as invalid with their error, so the author can correct them. Evidence: SPECS.md Spec 2.
- **Error without entry point with latent cycle:** the missing-entry-point message adds, when applicable, the path of the detected cycle, helping diagnose broken graphs. Evidence: `analyzeWorkflowGraph`.
- **Non-destructive terminal disconnect:** detaching from the PTY keeps the child process alive; the person can reconnect while the attempt stays within its human presence window. Evidence: RN-056.
- **System restart does not alter orphans:** starting the tool again does not turn orphans into failures or duplicate supervisors; recovery is always deliberate. Evidence: RN-057.

## 11. Business-relevant integrations

- **OpenCode (v1 execution runtime).** Business role: executing arm of the `agent` steps. Its structured output contract (JSONL) defines what "clean invocation" means; its exact, versioned signals enable permission request detection in headless; in managed modes it provides session server, events and native approval resolution, and a real attachable terminal. The concrete model it uses underneath is irrelevant to Shardeo (interchangeability).
- **Model providers (behind OpenCode).** Business role: variable capacity (quota, availability) that motivates the ordered candidate list and the fallback without semantic classification: the system assumes any provider can fail at any time.
- **Local embedded store (SQLite).** Business role: the project's institutional memory — traceability, resilience (resumption, reconciliation) and observability. It is not a real-time signaling bus; that role is played by explicit local inter-process communication.

## 12. Ambiguities and unconfirmed rules

1. **Documentation state lagging behind code.** SPECS.md marks Specs 5, 6 and 8 as `pending`, but the behavior is implemented and tested (graph validation on every load; re-execution with accumulated context; reopen/skip with generations). Possible interpretation: the status documentation was not updated. Evidence: existing commands and tests (`test/spec8-iteration-control.test.mjs`, `test/utils/dag-execution.test.mjs`). Uncertainty: which is the intended source of truth for future decisions.
2. **Spec 10 (model override by flags) is a draft.** Documented but not implemented: the proposed flags do not exist in the current execution command. It must not be assumed available. Evidence: `cmdStepRun` options signature versus SPECS.md Spec 10 (`status: draft`).
3. **Symlinks in produced artifacts.** The written contract allows symlinks whose target stays within the root; the implemented generation capture rejects symlinks as a produced artifact (requires a regular file). Possibly a later deliberate hardening. Evidence: `inspectProducedArtifacts` versus SPECS.md Spec 4. Uncertainty: which criterion will prevail.
4. **Divergent legacy prototype.** There is an old Python prototype at the root (`shardeo.py`) with different semantics (fixed CLI map, availability based on completed producers instead of the graph, fixed timeout). It does not represent current behavior; the current system is the TypeScript one. Evidence: direct comparison of both codes.
5. **Persistence of the `permission_timeout` code.** It remains in the terminal reasons list although inactivity migrated to `process_inactivity_timeout`; it is probably kept for compatibility with old managed paths. Uncertainty: whether any active path still emits it.
6. **Workflow-level mode reading.** In the execution command, the workflow-level declared mode is retrieved by re-reading the raw YAML with "best effort" comments in the code; the wiring seems partial. Evidence: corresponding block of `cmdStepRun`. Uncertainty: whether `workflow.mode` inheritance is fully guaranteed on all paths.
7. **`--human` flag accepted but without effect** in several commands (ignored parameter). Possible vestige of a planned human formatting surface. Evidence: `_opts: { human?: boolean }` signatures.

## 13. Rules summary

| ID | Rule | Area | Confidence |
|---|---|---|---|
| RN-001 | Only `opencode` as runtime in v1; another identifier invalidates the workflow | Workflows | High |
| RN-002 | Optional and opaque model, transferred without interpretation | Workflows | High |
| RN-003 | Full YAML revalidation on every operation | Workflows | High |
| RN-004 | Unknown fields tolerated; `after` field forbidden | Workflows | High |
| RN-005 | Name and at least one step mandatory | Workflows | High |
| RN-006 | Unique step IDs | Graph | High |
| RN-007 | depends_on only references existing steps | Graph | High |
| RN-008 | Mandatory entry point (step without dependencies) | Graph | High |
| RN-009 | Acyclic graph with path diagnosis | Graph | High |
| RN-010 | depends_on as the only ordering criterion | Graph | High |
| RN-011 | Available = predecessors completed/skipped | Graph | High |
| RN-012 | Explicit blocking by incomplete predecessors | Execution | High |
| RN-013 | Parallelism decided by the orchestrator | Graph | High |
| RN-014 | Atomic execution creation; invalid creates nothing | Execution | High |
| RN-015 | skipped/completed(agent) prevent re-execution | Execution | High |
| RN-016 | Command self-completes with exit 0 + valid produces | Execution | High |
| RN-017 | Agent never self-completes; explicit orchestrator closure | Execution | High |
| RN-018 | Complete requires running, attempt with snapshot and complete produces | Execution | High |
| RN-019 | requires validates at execution; stale detected by generation | Artifacts | High |
| RN-020 | Candidates from agents[] in order; lazy probe without attempt | Fallback | High |
| RN-021 | No candidates: failure with evaluated probes and zero attempts | Fallback | High |
| RN-022 | Strict success (valid JSONL, no error, exit 0) leaves running | Fallback | High |
| RN-023 | Clean error triggers fallback without semantic classification | Fallback | High |
| RN-024 | Closed list of terminal reasons without fallback | Fallback | High |
| RN-025 | Candidate exhaustion closes with exhaustion | Fallback | High |
| RN-026 | Every invocation persisted as an independent attempt | Persistence | High |
| RN-027 | Contractual order of injected context | Context | High |
| RN-028 | Size budgets per component; excess fails closed | Context | High |
| RN-029 | Unlimited re-executions with prior artifacts and delimited feedback | Context | High |
| RN-030 | Resettable 5-min inactivity ends with process_inactivity_timeout | Timeouts | High |
| RN-031 | Fixed 300 s ceiling for commands | Timeouts | High |
| RN-032 | Three independent timers; doubtful closure = orphaned | Timeouts | High |
| RN-033 | Permission detection only by exact signals; ambiguity fails closed | Permissions | High |
| RN-034 | No permission bypass or interactive shell | Permissions | High |
| RN-035 | Layered policy; only announced decisions; deny precedes | Permissions | High |
| RN-036 | Unique idempotent resolution; explicit conflict | Permissions | High |
| RN-037 | Visible ≤16 KiB sanitized; raw only in authorized depository ≤1 MiB | Evidence | High |
| RN-038 | Fallback context ≤2 MiB sanitized, without raw evidence | Evidence | High |
| RN-039 | SHA-256 snapshots with 1 MiB budget per step | Evidence | High |
| RN-040 | Artifacts under single root with strict containment | Artifacts | High |
| RN-041 | Generational validity; invalidated = stale even if it exists | Artifacts | High |
| RN-042 | Re-execution overwrites files; history remains | Artifacts | High |
| RN-043 | Produced must be a contained regular file | Artifacts | Medium |
| RN-044 | Skip only virgin pending work; terminal | Iteration | High |
| RN-045 | skipped releases order but does not satisfy requires | Iteration | High |
| RN-046 | Reopen only completed/failed with mandatory feedback | Iteration | High |
| RN-047 | Mandatory cascade when affected descendants exist | Iteration | High |
| RN-048 | Reopen invalidates generation; new validity after new success | Iteration | High |
| RN-049 | Live lease blocks; expired reconciled; transition audit | Iteration | High |
| RN-050 | Transactionally derived aggregate (failed dominates) | State | High |
| RN-051 | Resume requires unchanged workflow; does not repeat completed | Resumption | High |
| RN-052 | Lost artifact marks reconstruction with previous response | Resumption | High |
| RN-053 | Layered mode with headless default; override only per attempt | Modes | High |
| RN-054 | Probed capabilities; unsupported_capability without silent fallback | Modes | High |
| RN-055 | Supervised non-blocking; events with monotonic cursors | Modes | High |
| RN-056 | Terminal controlled only by humans; exact attach; detach does not kill | Modes | High |
| RN-057 | Orphans: never auto-failed nor blind re-authorization | Modes | High |
| RN-058 | Immutable bundle with proven admission; drift fails closed | Integrity | High |
| RN-059 | Pre-authorized reading limited to the exact bundle | Integrity | High |
| RN-060 | Uniform exit codes 0/1 | Convention | High |