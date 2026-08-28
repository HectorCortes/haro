# Proposal: Reconciliación normativa v2

## Intent

Eliminar contradicciones entre `docs/v2/` y Specs 9a–9c antes de los demás deltas v2.

## Scope

### In Scope
- Corregir terminal/PTY, bundle/admisión y transporte adapter-native.
- Registrar la deuda JSONL y limitar cada hunk a F-01..F-04.

### Out of Scope
- Código, `SPECS.md`, otros changes v2 y changelogs nuevos.
- CAS de interacciones y multiusuario (`XII.3`), que siguen diferidos.
- Interfaces Go conceptuales de §4; TypeScript + ACP + zod siguen autoritativos.

## Capabilities

### New Capabilities
- `v2-reconciliacion`: coherencia de la normativa v2.

### Modified Capabilities
- None.

## Approach

- Particionar `XII.2`: reconocer bundle/manifiesto/admisión y diferir CAS.
- Hacer `VII.4` y §7 abstractos: transporte nativo declarado y negociado en `initialize`; OpenCode HTTP+SSE será ejemplo no normativo.
- Anotar F-04 en Constitución XII y detallarla en §7 con `src/utils/agent.ts` (headless).
- Usar el diff del PR como autoridad U-01, sin artefactos fuera de `docs/v2/`.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `docs/v2/shardeo-v2-constitucion.md` | Modified | Reconciliar VII.4 y XII.1–XII.2; registrar F-04. |
| `docs/v2/shardeo-v2-especificacion-tecnica.md` | Modified | Alinear §7 y documentar la deuda exacta. |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Ampliación del diff | Med | Mapear cada hunk a un ID F-01..F-04. |
| Confundir CAS con bundle implementado | Med | Mantener CAS explícitamente diferido. |

## Rollback Plan

Revertir los hunks de ambos documentos; no existe estado runtime.

## Dependencies

- `shardeo-v2-deltas-acceptance.md` (autoridad) y Specs 9a–9c.

## Success Criteria

- [ ] **F-01**: La documentación normativa v2 reconoce que la superficie `terminal`/PTY **ya está implementada** (PTY real, attach humano, presencia) y la trata como superficie existente a preservar, no como trabajo diferido.
- [ ] **F-02**: La documentación normativa v2 reconoce que el bundle inmutable con manifiesto SHA-256 y prueba de admisión **ya está implementado** y lo remite a no-regresión (`v2-no-regresion`), no a trabajo diferido.
- [ ] **F-03**: La documentación normativa v2 no dicta el transporte Broker↔Adapter: lo elige el adapter según lo que su harness soporta nativamente (stdio-RPC, HTTP local + SSE, JSONL), se declara en capacidades y se negocia en `initialize`. HTTP local + SSE **no es "fallback"** sino vía nativa válida cuando el harness la expone (caso real: OpenCode supervised usa servidor HTTP/SSE gestionado).
- [ ] **F-04**: La documentación normativa v2 registra la excepción confinada vigente: el parser JSONL de OpenCode vive en `src/utils/agent.ts` (ruta headless) y su traslado al adapter es deuda pendiente que la spec `v2-adapter` debe cerrar.
- [ ] **U-01**: Existe un diff (o changelog) de la actualización que muestra exactamente qué reglas cambiaron y por qué — nunca una edición silenciosa.
