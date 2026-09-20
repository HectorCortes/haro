# Delta for v2-no-regresion

## MODIFIED Requirements

### Requirement: Workflow discovery and validation [v2-no-regresion/F-02,F-08,F-09,F-11] — P0 [E2E]

`workflows list/describe` MUST return structured JSON, flag malformed/invalid YAML, and report duplicates, missing dependencies, cycles, or no entry point. Workflow validation MUST accept `supervised`. At runtime, `step run` MUST enable `supervised` only for an admitted, ready, fenced ACP managed session that satisfies F-08/F-09/F-11; any missing, malformed, unknown, drifted, or unsupported prerequisite MUST fail before provider work without fallback or headless downgrade. Agent steps declared with `mode: terminal` MUST retain their existing rejection behavior.

(Previously: runtime rejected every supervised step before creating an attempt.)

#### Scenario: Discovery
- GIVEN mixed workflow YAML
- WHEN both run
- THEN details and errors appear

#### Scenario: Proven supervised mode and retained terminal guard
- GIVEN valid supervised ACP proof, incomplete proof, and a terminal request
- WHEN `step run` executes each step
- THEN only the fully proven supervised request returns an admitted ready attempt
- AND incomplete supervised requests fail without fallback, adapter work, or headless downgrade
- AND the terminal step retains its existing terminal rejection behavior
