# Haro — v2 Technical Specification

> Complements `haro-constitucion.md` (rules and principles) and `haro-arquitectura-consolidado.md` (research and architectural decision). This document contains only concrete technical artifacts: the workflow YAML schema, the persistence table DDL, the core Go interfaces, and the JSON-RPC method signatures of both internal protocols. It is the implementation reference — any divergence between the code and this document must be resolved by updating the document, not ignoring it.

---

## 1. Workflow file schema (YAML)

Expressed as JSON Schema (Draft 2020-12); the YAML is validated against its equivalent JSON representation.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://shardeo.dev/schema/workflow.json",
  "title": "Workflow",
  "type": "object",
  "required": ["version", "steps"],
  "additionalProperties": false,
  "properties": {
    "version": { "type": "integer", "const": 2 },
    "name": { "type": "string" },

    "inputs": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name", "satisfied_by"],
        "additionalProperties": false,
        "properties": {
          "name": { "type": "string", "pattern": "^[a-z][a-z0-9_]*$" },
          "satisfied_by": { "type": "string", "description": "id of the internal step that requires this input" }
        }
      }
    },

    "outputs": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name", "produced_by"],
        "additionalProperties": false,
        "properties": {
          "name": { "type": "string", "pattern": "^[a-z][a-z0-9_]*$" },
          "produced_by": { "type": "string", "description": "id of the internal step that produces this output" }
        }
      }
    },

    "workspace": { "$ref": "#/$defs/workspaceConfig" },

    "steps": {
      "type": "array",
      "minItems": 1,
      "items": { "$ref": "#/$defs/step" }
    }
  },

  "$defs": {
    "workspaceConfig": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "mode": { "type": "string", "enum": ["isolated", "shared"] },
        "on_logical_conflict": { "type": "string", "enum": ["block", "allow"], "default": "block" }
      }
    },

    "step": {
      "type": "object",
      "required": ["id", "type"],
      "properties": {
        "id": { "type": "string", "pattern": "^[a-z][a-z0-9_]*$" },
        "type": { "type": "string", "enum": ["command", "agent", "workflow"] },
        "depends_on": { "type": "array", "items": { "type": "string" }, "default": [] },
        "requires": { "type": "array", "items": { "type": "string" }, "default": [] },
        "produces": { "type": "array", "items": { "type": "string" }, "default": [] },
        "workspace": { "$ref": "#/$defs/workspaceConfig" }
      },
      "allOf": [
        {
          "if": { "properties": { "type": { "const": "command" } } },
          "then": { "required": ["run"], "properties": {
            "run": { "type": "string" },
            "env": { "type": "object", "additionalProperties": { "type": "string" } },
            "timeout_seconds": { "type": "integer", "minimum": 1 }
          } }
        },
        {
          "if": { "properties": { "type": { "const": "agent" } } },
          "then": { "required": ["harness", "instructions"], "properties": {
            "harness": {
              "type": "array",
              "minItems": 1,
              "items": { "type": "string" },
              "description": "ordered candidates; fallback without semantic classification to the next one if the current one fails"
            },
            "instructions": { "type": "string" },
            "mode": { "type": "string", "enum": ["headless", "supervised", "terminal"] },
            "timeout_seconds": { "type": "integer", "minimum": 1 }
          } }
        },
        {
          "if": { "properties": { "type": { "const": "workflow" } } },
          "then": { "required": ["source"], "properties": {
            "source": { "type": "string", "description": "path relative to another workflow file" },
            "bindings": {
              "type": "object",
              "description": "maps inputs declared by the included workflow to the parent's requires/produces",
              "additionalProperties": { "type": "string" }
            }
          } }
        }
      ]
    }
  }
}
```

### 1.1 Resolution notes

- `steps[].id` is unique within its own file. After flattening (X.2 of the constitution), the effective id of an internal step of a `workflow` node with `id: integrate_payments` is `integrate_payments.<internal_id>`.
- `inputs`/`outputs` are only valid in the file that is going to be **included**; a root file (invoked directly by the orchestrator) may omit them.
- `bindings` in a `workflow`-type step connects the parent's `requires`/`produces` with the `inputs`/`outputs` declared by the included file — it is what allows the parent to never wire against internal steps directly (Section X.3 of the constitution).

---

## 2. Persistence DDL (SQLite, WAL)

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE projects (
    id              TEXT PRIMARY KEY,          -- stable hash of the repo's absolute path
    root_path       TEXT NOT NULL,
    created_at      TEXT NOT NULL              -- ISO 8601
);

CREATE TABLE executions (
    id                  TEXT PRIMARY KEY,
    project_id          TEXT NOT NULL REFERENCES projects(id),
    workflow_source     TEXT NOT NULL,          -- path of the invoked root YAML
    status              TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed')),
    workspace_mode      TEXT NOT NULL CHECK (workspace_mode IN ('isolated','shared')),
    workspace_root      TEXT NOT NULL,          -- effective physical path (worktree or shared checkout)
    started_at          TEXT NOT NULL,
    ended_at            TEXT
);

CREATE INDEX idx_executions_project ON executions(project_id, status);

CREATE TABLE execution_steps (
    execution_id        TEXT NOT NULL REFERENCES executions(id),
    step_id             TEXT NOT NULL,          -- namespaced, e.g. "integrate_payments.charge"
    type                TEXT NOT NULL CHECK (type IN ('command','agent','workflow')),
    status               TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed','skipped')),
    depends_on           TEXT NOT NULL DEFAULT '[]',   -- JSON array of step_id
    requires             TEXT NOT NULL DEFAULT '[]',   -- JSON array of logical names
    produces             TEXT NOT NULL DEFAULT '[]',   -- JSON array of logical names
    workspace_mode        TEXT NOT NULL CHECK (workspace_mode IN ('isolated','shared')),
    current_generation    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (execution_id, step_id)
);

CREATE TABLE generations (
    id                   TEXT PRIMARY KEY,
    execution_id         TEXT NOT NULL,
    step_id              TEXT NOT NULL,
    number               INTEGER NOT NULL,
    created_at           TEXT NOT NULL,
    invalidated_at        TEXT,
    invalidated_by_step   TEXT,                 -- step_id whose reopen triggered the invalidation
    FOREIGN KEY (execution_id, step_id) REFERENCES execution_steps(execution_id, step_id),
    UNIQUE (execution_id, step_id, number)
);

CREATE TABLE attempts (
    id                   TEXT PRIMARY KEY,
    execution_id         TEXT NOT NULL,
    step_id              TEXT NOT NULL,
    generation_id         TEXT NOT NULL REFERENCES generations(id),
    status                TEXT NOT NULL CHECK (status IN ('running','completed','failed','cancelled')),
    started_at            TEXT NOT NULL,
    ended_at              TEXT,
    termination_reason     TEXT,
    result_digest          TEXT,                -- sha256 of the sanitized evidence
    FOREIGN KEY (execution_id, step_id) REFERENCES execution_steps(execution_id, step_id)
);

CREATE INDEX idx_attempts_step ON attempts(execution_id, step_id, status);

-- Transport/adapter-specific details, never in `attempts` (IV.3 of the constitution)
CREATE TABLE attempt_transport (
    attempt_id            TEXT PRIMARY KEY REFERENCES attempts(id),
    adapter_name           TEXT NOT NULL,        -- "opencode", "claudecode", "acp-generic", etc.
    native_session_id      TEXT,
    protocol_version       INTEGER,
    extra                  TEXT NOT NULL DEFAULT '{}'  -- free-form JSON, adapter's exclusive property
);

CREATE TABLE leases (
    execution_id          TEXT NOT NULL,
    step_id                TEXT NOT NULL,
    holder                 TEXT NOT NULL,        -- identifier of the process/broker holding the lease
    fencing_token           INTEGER NOT NULL,
    acquired_at              TEXT NOT NULL,
    expires_at               TEXT NOT NULL,
    PRIMARY KEY (execution_id, step_id)
);

CREATE TABLE step_transition_events (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id            TEXT NOT NULL,
    step_id                  TEXT NOT NULL,
    cursor                    INTEGER NOT NULL,    -- monotonic per (execution_id, step_id)
    from_status               TEXT,
    to_status                 TEXT NOT NULL,
    occurred_at               TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_step_events_cursor ON step_transition_events(execution_id, step_id, cursor);

CREATE TABLE attempt_events (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    attempt_id               TEXT NOT NULL REFERENCES attempts(id),
    cursor                    INTEGER NOT NULL,    -- monotonic per attempt_id
    event_type                TEXT NOT NULL,       -- "output_delta","permission_requested","interaction_resolved",...
    payload_ref                TEXT,                -- reference to sanitized evidence, never the full raw payload
    occurred_at                TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_attempt_events_cursor ON attempt_events(attempt_id, cursor);

CREATE TABLE interactions (
    id                       TEXT PRIMARY KEY,
    attempt_id                TEXT NOT NULL REFERENCES attempts(id),
    type                       TEXT NOT NULL CHECK (type IN ('permission','question')),
    status                     TEXT NOT NULL CHECK (status IN ('pending','resolved')),
    decision                   TEXT,
    idempotency_key             TEXT NOT NULL,
    resolved_at                 TEXT,
    UNIQUE (attempt_id, idempotency_key)
);

-- Coordination between concurrent executions (Section IX of the constitution)
CREATE TABLE path_claims (
    id                        INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id                 TEXT NOT NULL REFERENCES projects(id),
    logical_path                TEXT NOT NULL,     -- canonicalized, relative to the repo root
    mode                         TEXT NOT NULL,      -- "isolated:<execution_id>" | "shared"
    owner_execution_id           TEXT NOT NULL,
    owner_step_id                 TEXT NOT NULL,
    acquired_at                    TEXT NOT NULL,
    released_at                    TEXT
);

CREATE INDEX idx_path_claims_active ON path_claims(project_id, logical_path) WHERE released_at IS NULL;
```

### 2.1 Notes

- All time columns are `TEXT` in ISO 8601 UTC format — SQLite has no native temporal type, and any timezone ambiguity in the store is avoided.
- `attempt_events.payload_ref` never contains the full raw output — it points to a sanitized blob/file managed outside the table (VIII.2 of the constitution).
- `path_claims` has no `FOREIGN KEY` to `executions` because a claim can survive at the project level even between different brokers sharing the same `project_id`; it is validated at the application level, not the schema level.

---

## 3. Go interfaces — domain

```go
package domain

type ExecutionID string
type StepID string   // namespaced after flattening: "integrate_payments.charge"
type AttemptID string
type GenerationID string
type ProjectID string

type StepType string
const (
    StepCommand  StepType = "command"
    StepAgent    StepType = "agent"
    StepWorkflow StepType = "workflow"
)

type ExecutionStatus string
const (
    ExecPending   ExecutionStatus = "pending"
    ExecRunning   ExecutionStatus = "running"
    ExecCompleted ExecutionStatus = "completed"
    ExecFailed    ExecutionStatus = "failed"
)

type StepStatus string
const (
    StepPending   StepStatus = "pending"
    StepRunning   StepStatus = "running"
    StepCompleted StepStatus = "completed"
    StepFailed    StepStatus = "failed"
    StepSkipped   StepStatus = "skipped"
)

type WorkspaceMode string
const (
    WorkspaceIsolated WorkspaceMode = "isolated"
    WorkspaceShared   WorkspaceMode = "shared"
)

type LogicalConflictPolicy string
const (
    ConflictBlock LogicalConflictPolicy = "block"
    ConflictAllow LogicalConflictPolicy = "allow"
)

type WorkspaceConfig struct {
    Mode              WorkspaceMode
    OnLogicalConflict LogicalConflictPolicy
}

type Step struct {
    ID        StepID
    Type      StepType
    DependsOn []StepID
    Requires  []string
    Produces  []string
    Workspace WorkspaceConfig

    Command  *CommandSpec  // present if Type == StepCommand
    Agent    *AgentSpec    // present if Type == StepAgent
    Workflow *WorkflowRef  // present if Type == StepWorkflow (compilation phase only, does not survive flattening)
}

type CommandSpec struct {
    Run            string
    Env            map[string]string
    TimeoutSeconds int
}

type AgentSpec struct {
    HarnessCandidates []string // fallback order
    Instructions      string
    Mode              string   // derived from negotiated capabilities, see Section 4
    TimeoutSeconds    int
}

type WorkflowRef struct {
    Source   string
    Bindings map[string]string // input/output name of the included -> parent's requires/produces
}

type Execution struct {
    ID             ExecutionID
    ProjectID      ProjectID
    WorkflowSource string
    Status         ExecutionStatus
    WorkspaceMode  WorkspaceMode
    WorkspaceRoot  string
    Steps          []Step
}

type Generation struct {
    ID              GenerationID
    ExecutionID     ExecutionID
    StepID          StepID
    Number          int
    InvalidatedBy   *StepID
}

type Attempt struct {
    ID               AttemptID
    ExecutionID      ExecutionID
    StepID           StepID
    GenerationID     GenerationID
    Status           string
    TerminationReason string
    ResultDigest      string
}
```

---

## 4. Adapter contract (Go)

```go
package adapter

import "context"

type Capabilities struct {
    ProtocolVersion int
    Permission      bool
    Terminal        bool
    LoadSession     bool
    Extra           map[string]any // namespaced, "_" prefix as in ACP
}

type ProbeResult struct {
    Available    bool
    Version      string
    Capabilities Capabilities
}

// Adapter is implemented once per harness (or generically by acp/ for any
// harness that speaks native Agent Client Protocol).
type Adapter interface {
    Probe(ctx context.Context) (ProbeResult, error)
    Initialize(ctx context.Context, core Capabilities) (Capabilities, error)
    NewSession(ctx context.Context, bundle SessionBundle, host SessionHost) (Session, error)
}

// SessionBundle is the immutable input context of an attempt.
type SessionBundle struct {
    Instructions string
    WorkspaceRoot string
    Requires     map[string]string // logical name -> resolved path
}

// Session is the handle of an in-flight attempt. Only the methods marked here as
// "mandatory" must always be implemented; the rest are invoked only if
// Capabilities declared them supported in Initialize.
type Session interface {
    // mandatory
    Prompt(ctx context.Context, input PromptInput) (<-chan SessionEvent, error)
    // mandatory
    Cancel(ctx context.Context) error

    // optional — requires Capabilities.LoadSession
    LoadPrevious(ctx context.Context, nativeSessionID string) error

    // optional — requires Capabilities.Terminal
    Terminal(ctx context.Context) (TerminalHandle, error)
}

// SessionHost is implemented by the broker and passed to the adapter in NewSession.
// It is the REVERSE channel: the harness (via the adapter) calls into the core, not the other way around.
type SessionHost interface {
    // mandatory to implement in the broker if Capabilities.Permission was negotiated
    RequestPermission(ctx context.Context, req PermissionRequest) (PermissionDecision, error)
}

type PromptInput struct {
    Text string
}

type SessionEvent struct {
    Cursor  int64
    Type    string // "output_delta" | "permission_requested" | "completed" | "failed"
    Payload []byte // JSON, shape defined in Section 6 (protocol)
}

type PermissionRequest struct {
    Kind        string
    Description string
    Options     []string
}

type PermissionDecision struct {
    Option string
}
```

---

## 5. Go interfaces — persistence (repository)

```go
package store

import "context"

// Store groups all repositories. No layer outside `store/` accesses SQLite
// directly (III.3 of the constitution: keeps the door open to another backend).
type Store interface {
    Executions() ExecutionRepository
    Steps() StepRepository
    Attempts() AttemptRepository
    Generations() GenerationRepository
    Events() EventRepository
    Leases() LeaseRepository
    PathClaims() PathClaimRepository
}

type PathClaim struct {
    ProjectID     string
    LogicalPath   string
    Mode          string // "isolated:<execution_id>" | "shared"
    OwnerExecution string
    OwnerStep      string
}

type PathClaimRepository interface {
    // Acquire is atomic (INSERT ... ON CONFLICT). If an incompatible active claim
    // already exists, Acquired=false and Conflict describes the current owner.
    Acquire(ctx context.Context, claim PathClaim) (acquired bool, conflict *PathClaim, err error)
    Release(ctx context.Context, projectID, logicalPath, ownerExecution, ownerStep string) error
    ListActive(ctx context.Context, projectID string) ([]PathClaim, error)
}

type LeaseRepository interface {
    // Acquire fails if there is an unexpired lease held by another holder.
    Acquire(ctx context.Context, executionID, stepID, holder string) (fencingToken int64, err error)
    Renew(ctx context.Context, executionID, stepID, holder string, fencingToken int64) error
    Release(ctx context.Context, executionID, stepID, holder string, fencingToken int64) error
}
```

---

## 6. JSON-RPC protocol — CLI ↔ Broker

Transport: JSON-RPC 2.0 over Unix socket. One broker per project.

| Method | Direction | Params | Result |
|---|---|---|---|
| `execution.start` | CLI → Broker | `{workflow_path: string, workspace_override?: WorkspaceConfig}` | `{execution_id: string}` |
| `execution.status` | CLI → Broker | `{execution_id: string}` | `{status: ExecutionStatus, steps: StepSummary[]}` |
| `execution.report` | CLI → Broker | `{execution_id: string}` | `{changed_files: string[]}` |
| `step.run` | CLI → Broker | `{execution_id, step_id, mode?: "headless"\|"supervised"}` | `{attempt_id: string, cursor: int}` |
| `step.events` | CLI → Broker | `{execution_id, step_id, since_cursor: int}` | `{events: SessionEvent[], next_cursor: int}` |
| `step.approve` | CLI → Broker | `{execution_id, step_id, interaction_id, decision: string, idempotency_key: string}` | `{resolved: bool}` |
| `step.cancel` | CLI → Broker | `{execution_id, step_id}` | `{}` |
| `step.reopen` | CLI → Broker | `{execution_id, step_id}` | `{invalidated: string[]}` |

Notifications (Broker → CLI, no response expected):

| Notification | Params |
|---|---|
| `step.status_changed` | `{execution_id, step_id, from: StepStatus, to: StepStatus, cursor: int}` |
| `step.interaction_required` | `{execution_id, step_id, interaction_id, kind: "permission"\|"question", description: string, options: string[]}` |

---

## 7. JSON-RPC protocol — Broker ↔ Adapter/Harness (ACP-based)

Transport: the adapter chooses the native transport its harness supports — for example, stdio-RPC, local HTTP + SSE or JSONL — declares it in capabilities and negotiates it in `initialize`. The contract reuses the method shape of Agent Client Protocol. (Non-normative example: OpenCode supervised uses managed local HTTP + SSE.)

| Method | Direction | Params (summary) | Result / Notification |
|---|---|---|---|
| `initialize` | Broker → Harness | `{protocolVersion: int, clientCapabilities: Capabilities}` | `{protocolVersion: int, agentCapabilities: Capabilities}` |
| `session/new` | Broker → Harness | `{instructions: string, workspaceRoot: string, requires: object}` | `{sessionId: string}` |
| `session/prompt` | Broker → Harness | `{sessionId: string, text: string}` | `{}` (result arrives via notifications) |
| `session/update` | Harness → Broker | `{sessionId, cursor: int, delta: object}` | notification, no response |
| `session/cancel` | Broker → Harness | `{sessionId: string}` | `{}` |
| `session/request_permission` | Harness → Broker | `{sessionId, kind: string, description: string, options: string[]}` | `{option: string}` |
| `session/load` *(optional)* | Broker → Harness | `{sessionId: string, nativeSessionId: string}` | `{}` |
| `terminal/*` *(optional, implemented)* | — | — | Real PTY, human attach and presence; existing surface to preserve. See Spec 9c (`src/runtime/pty.ts`, `src/runtime/terminal.ts`, `src/runtime/attach.ts`; `SPECS.md` §9c). |

**Confined debt:** the OpenCode JSONL parser remains in `src/utils/agent.ts` (headless path, lines 1596–1713); moving it to the adapter is pending and `v2-adapter/F-04` must complete it.

### 7.1 Negotiation rule

- `initialize` runs exactly once per subprocess, before any `session/*`.
- The broker never calls a method not announced in `agentCapabilities`.
- The harness never calls `session/request_permission` if the broker did not announce `Permission: true` in `clientCapabilities` — in that case, any permission need must be resolved with the harness's default policy or fail (fail-closed, per V.4 and VIII.5 of the constitution).

### 7.2 Transport persistence (v2-adapter)

Per-attempt adapter details are persisted via `Store.Transport()` (`TransportRepository`) backed by `attempt_transport` (additive, idempotent `CREATE TABLE IF NOT EXISTS`, sole owner `v2-adapter`). `attempts` remains transport-neutral; `attempt_transport` holds `adapter_name`, `native_session_id`, `protocol_version`, and `extra` JSON. Engine inserts the attempt and its optional transport row in a single `WithTx`.

---

## 8. Traceability with the constitution

Each artifact in this document directly references a rule of `haro-constitucion.md`:

| Artifact | Rule it implements |
|---|---|
| Section 1 (YAML schema) — `workflow` block in `steps[].type` | X.1, X.3 |
| Section 2 — `path_claims` table | IX.3 |
| Section 2 — `attempt_transport` table | IV.3 |
| Section 3 — `Step.Workflow *WorkflowRef` | X.1, X.2 |
| Section 4 — `Adapter`/`Session`/`SessionHost` | V.1, V.2, V.3, V.4 |
| Section 5 — `PathClaimRepository` | IX.3, IX.4 |
| Section 6 — `step.reopen` | X.4, VI |
| Section 7 — methods table | VII, VIII.4 |