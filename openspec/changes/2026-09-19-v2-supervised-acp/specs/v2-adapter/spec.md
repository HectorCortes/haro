# Delta for v2-adapter

## MODIFIED Requirements

### Requirement: Prior initialization [v2-adapter/F-01]

Each CLI invocation MUST construct one manager from project configuration, probe and initialize each usable harness once before `session/*`, and inject it into all agent-capable engines. Sessions MUST use direct execution with fixed argv and no shell. OpenCode and Claude MUST preserve their headless registration; `acp-generic` MUST be registered only when its real managed-session contract is available and MUST initialize exactly once per subprocess before ACP session methods.

(Previously: ACP remained unregistered until a real session contract existed.)

#### Scenario: CLI lifecycle (`TestAgentManagerCLIInjection`)
- GIVEN enabled configuration and any agent-capable run site
- WHEN the CLI creates its execution engine
- THEN one manager is injected and each usable harness initializes once before its first session

#### Scenario: Real OpenCode, Claude, and ACP sessions
- GIVEN existing headless fixtures and a compliant ACP executable fixture
- WHEN each registered session runs or cancels
- THEN headless behavior is unchanged and ACP follows one ordered managed-session cycle

### Requirement: Adapter boundary [v2-adapter/F-04]

`.haro/config.yaml` MUST accept strict harness records. An `acp-generic` record MUST separate executable, ordered argv list, and allowed environment; it MUST NOT accept a shell command string or interpolate values. Unknown fields, invalid values, provider literals outside adapters, and oversized protocol messages MUST fail closed.

(Previously: harness records exposed binary, environment, timeout, and enabled fields without an ACP-specific argv contract.)

#### Scenario: Boundary gate
- GIVEN sources, shell-like input, and an oversized message
- WHEN boundary and parser checks run
- THEN provider leakage, shell configuration, and the oversized message are rejected

#### Scenario: Strict ACP launch configuration
- GIVEN executable, ordered argv, allowed environment, and unknown record fields
- WHEN project configuration loads and launches
- THEN exact argv/environment pass directly while unknown or interpolated input fails

### Requirement: Generic ACP adapter [v2-adapter/U-03]

The registered `acp-generic` adapter MUST implement JSON-RPC 2.0 over stdio and translate `initialize`, `session/new`, `session/prompt`, `session/update`, `session/cancel`, and negotiated `session/request_permission` bijectively. It MUST validate capabilities and every inbound/outbound payload and MUST fail closed on unknown, malformed, out-of-order, or unsupported protocol behavior.

(Previously: only translation round trips were required; no real registered ACP session existed.)

#### Scenario: Full ACP fixture cycle
- GIVEN valid, malformed, unknown, and unsupported ACP fixtures
- WHEN capability negotiation and the complete session cycle run
- THEN valid messages round-trip and invalid cases terminate without unauthorized effects
