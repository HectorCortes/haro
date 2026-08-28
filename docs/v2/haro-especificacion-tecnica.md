# Shardeo v2 — Especificación técnica

> Complementa a `shardeo-v2-constitucion.md` (reglas y principios) y `shardeo-v2-arquitectura-consolidado.md` (investigación y decisión arquitectónica). Este documento contiene únicamente artefactos técnicos concretos: el schema del YAML de workflow, el DDL de las tablas de persistencia, las interfaces Go del core, y las firmas de los métodos JSON-RPC de ambos protocolos internos. Es la referencia de implementación — cualquier divergencia entre el código y este documento debe resolverse actualizando el documento, no ignorándolo.

---

## 1. Schema del archivo de workflow (YAML)

Expresado como JSON Schema (Draft 2020-12); el YAML se valida contra su representación JSON equivalente.

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
          "satisfied_by": { "type": "string", "description": "id de step interno que requiere este input" }
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
          "produced_by": { "type": "string", "description": "id de step interno que produce este output" }
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
              "description": "candidatos ordenados; fallback sin clasificación semántica al siguiente si el actual falla"
            },
            "instructions": { "type": "string" },
            "mode": { "type": "string", "enum": ["headless", "supervised", "terminal"] },
            "timeout_seconds": { "type": "integer", "minimum": 1 }
          } }
        },
        {
          "if": { "properties": { "type": { "const": "workflow" } } },
          "then": { "required": ["source"], "properties": {
            "source": { "type": "string", "description": "path relativo a otro archivo de workflow" },
            "bindings": {
              "type": "object",
              "description": "mapea inputs declarados por el workflow incluido a requires/produces del padre",
              "additionalProperties": { "type": "string" }
            }
          } }
        }
      ]
    }
  }
}
```

### 1.1 Notas de resolución

- `steps[].id` es único dentro de su propio archivo. Tras el aplanado (X.2 de la constitución), el id efectivo de un step interno de un nodo `workflow` con `id: integrate_payments` es `integrate_payments.<id_interno>`.
- `inputs`/`outputs` solo son válidos en el archivo que va a ser **incluido**; un archivo raíz (invocado directamente por el orquestador) puede omitirlos.
- `bindings` en un step tipo `workflow` conecta `requires`/`produces` del padre con los `inputs`/`outputs` declarados por el archivo incluido — es lo que permite que el padre nunca cablee contra steps internos directamente (Sección X.3 de la constitución).

---

## 2. DDL de persistencia (SQLite, WAL)

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE projects (
    id              TEXT PRIMARY KEY,          -- hash estable de la ruta absoluta del repo
    root_path       TEXT NOT NULL,
    created_at      TEXT NOT NULL              -- ISO 8601
);

CREATE TABLE executions (
    id                  TEXT PRIMARY KEY,
    project_id          TEXT NOT NULL REFERENCES projects(id),
    workflow_source     TEXT NOT NULL,          -- path del YAML raíz invocado
    status              TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed')),
    workspace_mode      TEXT NOT NULL CHECK (workspace_mode IN ('isolated','shared')),
    workspace_root      TEXT NOT NULL,          -- ruta física efectiva (worktree o checkout compartido)
    started_at          TEXT NOT NULL,
    ended_at            TEXT
);

CREATE INDEX idx_executions_project ON executions(project_id, status);

CREATE TABLE execution_steps (
    execution_id        TEXT NOT NULL REFERENCES executions(id),
    step_id             TEXT NOT NULL,          -- namespaced, p.ej. "integrate_payments.charge"
    type                TEXT NOT NULL CHECK (type IN ('command','agent','workflow')),
    status               TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed','skipped')),
    depends_on           TEXT NOT NULL DEFAULT '[]',   -- JSON array de step_id
    requires             TEXT NOT NULL DEFAULT '[]',   -- JSON array de nombres lógicos
    produces             TEXT NOT NULL DEFAULT '[]',   -- JSON array de nombres lógicos
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
    invalidated_by_step   TEXT,                 -- step_id cuyo reopen disparó la invalidación
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
    result_digest          TEXT,                -- sha256 de la evidencia sanitizada
    FOREIGN KEY (execution_id, step_id) REFERENCES execution_steps(execution_id, step_id)
);

CREATE INDEX idx_attempts_step ON attempts(execution_id, step_id, status);

-- Detalles específicos de transporte/adapter, nunca en `attempts` (IV.3 de la constitución)
CREATE TABLE attempt_transport (
    attempt_id            TEXT PRIMARY KEY REFERENCES attempts(id),
    adapter_name           TEXT NOT NULL,        -- "opencode", "claudecode", "acp-generic", etc.
    native_session_id      TEXT,
    protocol_version       INTEGER,
    extra                  TEXT NOT NULL DEFAULT '{}'  -- JSON libre, propiedad exclusiva del adapter
);

CREATE TABLE leases (
    execution_id          TEXT NOT NULL,
    step_id                TEXT NOT NULL,
    holder                 TEXT NOT NULL,        -- identificador de proceso/broker que sostiene el lease
    fencing_token           INTEGER NOT NULL,
    acquired_at              TEXT NOT NULL,
    expires_at               TEXT NOT NULL,
    PRIMARY KEY (execution_id, step_id)
);

CREATE TABLE step_transition_events (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id            TEXT NOT NULL,
    step_id                  TEXT NOT NULL,
    cursor                    INTEGER NOT NULL,    -- monotónico por (execution_id, step_id)
    from_status               TEXT,
    to_status                 TEXT NOT NULL,
    occurred_at               TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_step_events_cursor ON step_transition_events(execution_id, step_id, cursor);

CREATE TABLE attempt_events (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    attempt_id               TEXT NOT NULL REFERENCES attempts(id),
    cursor                    INTEGER NOT NULL,    -- monotónico por attempt_id
    event_type                TEXT NOT NULL,       -- "output_delta","permission_requested","interaction_resolved",...
    payload_ref                TEXT,                -- referencia a evidencia sanitizada, nunca el payload crudo completo
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

-- Coordinación entre ejecuciones concurrentes (Sección IX de la constitución)
CREATE TABLE path_claims (
    id                        INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id                 TEXT NOT NULL REFERENCES projects(id),
    logical_path                TEXT NOT NULL,     -- canonicalizado, relativo a la raíz del repo
    mode                         TEXT NOT NULL,      -- "isolated:<execution_id>" | "shared"
    owner_execution_id           TEXT NOT NULL,
    owner_step_id                 TEXT NOT NULL,
    acquired_at                    TEXT NOT NULL,
    released_at                    TEXT
);

CREATE INDEX idx_path_claims_active ON path_claims(project_id, logical_path) WHERE released_at IS NULL;
```

### 2.1 Notas

- Todas las columnas de tiempo son `TEXT` en formato ISO 8601 UTC — SQLite no tiene tipo temporal nativo, y se evita cualquier ambigüedad de zona horaria en el store.
- `attempt_events.payload_ref` nunca contiene el output crudo completo — apunta a un blob/archivo sanitizado gestionado fuera de la tabla (VIII.2 de la constitución).
- `path_claims` no tiene `FOREIGN KEY` hacia `executions` porque un reclamo puede sobrevivir a nivel de proyecto incluso entre brokers distintos que comparten el mismo `project_id`; se valida a nivel de aplicación, no de esquema.

---

## 3. Interfaces Go — dominio

```go
package domain

type ExecutionID string
type StepID string   // namespaced tras aplanado: "integrate_payments.charge"
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

    Command  *CommandSpec  // presente si Type == StepCommand
    Agent    *AgentSpec    // presente si Type == StepAgent
    Workflow *WorkflowRef  // presente si Type == StepWorkflow (solo en fase de compilación, no sobrevive al aplanado)
}

type CommandSpec struct {
    Run            string
    Env            map[string]string
    TimeoutSeconds int
}

type AgentSpec struct {
    HarnessCandidates []string // orden de fallback
    Instructions      string
    Mode              string   // derivado de capacidades negociadas, ver Sección 4
    TimeoutSeconds    int
}

type WorkflowRef struct {
    Source   string
    Bindings map[string]string // nombre de input/output del incluido -> requires/produces del padre
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

## 4. Contrato del adapter (Go)

```go
package adapter

import "context"

type Capabilities struct {
    ProtocolVersion int
    Permission      bool
    Terminal        bool
    LoadSession     bool
    Extra           map[string]any // namespaced, prefijo "_" como en ACP
}

type ProbeResult struct {
    Available    bool
    Version      string
    Capabilities Capabilities
}

// Adapter es implementado una vez por harness (o genéricamente por acp/ para cualquier
// harness que hable Agent Client Protocol nativo).
type Adapter interface {
    Probe(ctx context.Context) (ProbeResult, error)
    Initialize(ctx context.Context, core Capabilities) (Capabilities, error)
    NewSession(ctx context.Context, bundle SessionBundle, host SessionHost) (Session, error)
}

// SessionBundle es el contexto inmutable de entrada de un attempt.
type SessionBundle struct {
    Instructions string
    WorkspaceRoot string
    Requires     map[string]string // nombre lógico -> path resuelto
}

// Session es el handle de un attempt en curso. Solo los métodos marcados aquí como
// "obligatorio" deben implementarse siempre; el resto se invoca únicamente si
// Capabilities lo declaró soportado en Initialize.
type Session interface {
    // obligatorio
    Prompt(ctx context.Context, input PromptInput) (<-chan SessionEvent, error)
    // obligatorio
    Cancel(ctx context.Context) error

    // opcional — requiere Capabilities.LoadSession
    LoadPrevious(ctx context.Context, nativeSessionID string) error

    // opcional — requiere Capabilities.Terminal
    Terminal(ctx context.Context) (TerminalHandle, error)
}

// SessionHost lo implementa el broker y se le pasa al adapter en NewSession.
// Es el canal INVERSO: el harness (vía el adapter) llama hacia el core, no al revés.
type SessionHost interface {
    // obligatorio de implementar en el broker si Capabilities.Permission fue negociado
    RequestPermission(ctx context.Context, req PermissionRequest) (PermissionDecision, error)
}

type PromptInput struct {
    Text string
}

type SessionEvent struct {
    Cursor  int64
    Type    string // "output_delta" | "permission_requested" | "completed" | "failed"
    Payload []byte // JSON, forma definida por Sección 6 (protocolo)
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

## 5. Interfaces Go — persistencia (repositorio)

```go
package store

import "context"

// Store agrupa todos los repositorios. Ninguna capa fuera de `store/` accede a SQLite
// directamente (III.3 de la constitución: mantiene abierta la puerta a otro backend).
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
    // Acquire es atómico (INSERT ... ON CONFLICT). Si ya existe un reclamo activo
    // incompatible, Acquired=false y Conflict describe al dueño actual.
    Acquire(ctx context.Context, claim PathClaim) (acquired bool, conflict *PathClaim, err error)
    Release(ctx context.Context, projectID, logicalPath, ownerExecution, ownerStep string) error
    ListActive(ctx context.Context, projectID string) ([]PathClaim, error)
}

type LeaseRepository interface {
    // Acquire falla si existe un lease vigente de otro holder no vencido.
    Acquire(ctx context.Context, executionID, stepID, holder string) (fencingToken int64, err error)
    Renew(ctx context.Context, executionID, stepID, holder string, fencingToken int64) error
    Release(ctx context.Context, executionID, stepID, holder string, fencingToken int64) error
}
```

---

## 6. Protocolo JSON-RPC — CLI ↔ Broker

Transporte: JSON-RPC 2.0 sobre socket Unix. Un broker por proyecto.

| Método | Dirección | Params | Result |
|---|---|---|---|
| `execution.start` | CLI → Broker | `{workflow_path: string, workspace_override?: WorkspaceConfig}` | `{execution_id: string}` |
| `execution.status` | CLI → Broker | `{execution_id: string}` | `{status: ExecutionStatus, steps: StepSummary[]}` |
| `execution.report` | CLI → Broker | `{execution_id: string}` | `{changed_files: string[]}` |
| `step.run` | CLI → Broker | `{execution_id, step_id, mode?: "headless"\|"supervised"}` | `{attempt_id: string, cursor: int}` |
| `step.events` | CLI → Broker | `{execution_id, step_id, since_cursor: int}` | `{events: SessionEvent[], next_cursor: int}` |
| `step.approve` | CLI → Broker | `{execution_id, step_id, interaction_id, decision: string, idempotency_key: string}` | `{resolved: bool}` |
| `step.cancel` | CLI → Broker | `{execution_id, step_id}` | `{}` |
| `step.reopen` | CLI → Broker | `{execution_id, step_id}` | `{invalidated: string[]}` |

Notificaciones (Broker → CLI, sin respuesta esperada):

| Notificación | Params |
|---|---|
| `step.status_changed` | `{execution_id, step_id, from: StepStatus, to: StepStatus, cursor: int}` |
| `step.interaction_required` | `{execution_id, step_id, interaction_id, kind: "permission"\|"question", description: string, options: string[]}` |

---

## 7. Protocolo JSON-RPC — Broker ↔ Adapter/Harness (basado en ACP)

Transporte: el adapter elige el transporte nativo que su harness soporta —por ejemplo, stdio-RPC, HTTP local + SSE o JSONL—, lo declara en capacidades y lo negocia en `initialize`. El contrato reutiliza la forma de métodos de Agent Client Protocol. (Ejemplo no normativo: OpenCode supervised usa HTTP local + SSE gestionado.)

| Método | Dirección | Params (resumen) | Result / Notificación |
|---|---|---|---|
| `initialize` | Broker → Harness | `{protocolVersion: int, clientCapabilities: Capabilities}` | `{protocolVersion: int, agentCapabilities: Capabilities}` |
| `session/new` | Broker → Harness | `{instructions: string, workspaceRoot: string, requires: object}` | `{sessionId: string}` |
| `session/prompt` | Broker → Harness | `{sessionId: string, text: string}` | `{}` (resultado llega por notificaciones) |
| `session/update` | Harness → Broker | `{sessionId, cursor: int, delta: object}` | notificación, sin respuesta |
| `session/cancel` | Broker → Harness | `{sessionId: string}` | `{}` |
| `session/request_permission` | Harness → Broker | `{sessionId, kind: string, description: string, options: string[]}` | `{option: string}` |
| `session/load` *(opcional)* | Broker → Harness | `{sessionId: string, nativeSessionId: string}` | `{}` |
| `terminal/*` *(opcional, implementado)* | — | — | PTY real, attach humano y presencia; superficie existente a preservar. Ver Spec 9c (`src/runtime/pty.ts`, `src/runtime/terminal.ts`, `src/runtime/attach.ts`; `SPECS.md` §9c). |

**Deuda confinada:** el parser JSONL de OpenCode permanece en `src/utils/agent.ts` (ruta headless, líneas 1596–1713); su traslado al adapter está pendiente y `v2-adapter/F-04` debe completarlo.

### 7.1 Regla de negociación

- `initialize` se ejecuta una única vez por subproceso, antes de cualquier `session/*`.
- El broker nunca llama a un método no anunciado en `agentCapabilities`.
- El harness nunca llama a `session/request_permission` si el broker no anunció `Permission: true` en `clientCapabilities` — en ese caso, cualquier necesidad de permiso debe resolverse con la política por defecto del harness o fallar (fail-closed, según V.4 y VIII.5 de la constitución).

---

## 8. Trazabilidad con la constitución

Cada artefacto de este documento referencia directamente una regla de `shardeo-v2-constitucion.md`:

| Artefacto | Regla que implementa |
|---|---|
| Sección 1 (schema YAML) — bloque `workflow` en `steps[].type` | X.1, X.3 |
| Sección 2 — tabla `path_claims` | IX.3 |
| Sección 2 — tabla `attempt_transport` | IV.3 |
| Sección 3 — `Step.Workflow *WorkflowRef` | X.1, X.2 |
| Sección 4 — `Adapter`/`Session`/`SessionHost` | V.1, V.2, V.3, V.4 |
| Sección 5 — `PathClaimRepository` | IX.3, IX.4 |
| Sección 6 — `step.reopen` | X.4, VI |
| Sección 7 — tabla de métodos | VII, VIII.4 |
