# Delta for v2-no-regresion

## MODIFIED Requirements

### Requirement: Workflow discovery and validation [v2-no-regresion/F-02] — P0 [E2E]

`workflows list/describe` MUST return structured JSON, flag malformed/invalid YAML, and report concrete errors: duplicates, missing dependencies, cycles, or no entry point. Workflow validation MUST continue to accept `supervised` as a valid declared mode. At runtime, `step run` MUST reject an agent step declared with `mode: supervised` before harness intersection, fallback, or adapter invocation; the step MUST end `failed` with the distinct reason `supervised mode not supported`, no attempt MUST be created, no adapter MUST be invoked, no fallback MUST run, and execution MUST NOT silently downgrade to headless. Agent steps declared with `mode: terminal` MUST retain their existing terminal rejection behavior.

(Previously: F-02 covered workflow discovery and validation without requiring fail-closed runtime rejection of supervised mode.)

#### Scenario: Discovery
- GIVEN mixed workflow YAML
- WHEN both run
- THEN details and errors appear

#### Scenario: Unsupported declared execution modes
- GIVEN a valid workflow declaring an agent step with `mode: supervised` and the existing `mode: terminal` rejection case
- WHEN `step run` executes each step
- THEN the supervised step ends `failed` with reason `supervised mode not supported`
- AND no attempt is created, no adapter is invoked, no fallback runs, and no headless downgrade occurs
- AND the terminal step retains its existing terminal rejection behavior
