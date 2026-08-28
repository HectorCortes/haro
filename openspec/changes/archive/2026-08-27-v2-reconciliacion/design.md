# Design: Reconciliación normativa v2

## Enfoque técnico

Editar solo los dos documentos normativos contra Specs 9a–9c y el código verificado. El diff del PR será la autoridad U-01; no habrá changelog ni cambios de código, `SPECS.md` o §4.

## Decisiones de arquitectura y redacción

| Opción | Tradeoff | Decisión y motivo |
|---|---|---|
| Transporte fijo o selección por capacidades | Evita acoplar el core | El adapter elige, declara y negocia; OpenCode HTTP+SSE será ejemplo no normativo. |
| XII íntegramente diferida o particionada | Refleja Specs 9a/9c sin adelantar CAS | Separar implementado de CAS y multiusuario diferidos. |
| Changelog o diff | Evita duplicar autoridad | Usar solo el diff del PR para U-01. |

## Plan de edición por hunk

### `docs/v2/shardeo-v2-constitucion.md`

**Hunk C1 — VII.4, línea 110 — F-03**  
Actual: “**VII.4** Broker ↔ Adapter ↔ Harness: JSON-RPC 2.0 sobre stdio del subproceso cuando el harness lo soporta nativamente. HTTP local + SSE es un fallback válido de adapter para harnesses que no ofrecen stdio-RPC, nunca el mecanismo general.”  
Propuesto: “**VII.4** Broker ↔ Adapter ↔ Harness: el adapter elige el transporte nativo que su harness soporta —por ejemplo, stdio-RPC, HTTP local + SSE o JSONL—, lo declara en capacidades y lo negocia en `initialize`; el core no impone ni jerarquiza un transporte. (Ejemplo no normativo: OpenCode supervised usa HTTP local + SSE gestionado.)”

**Hunk C2 — XII, líneas 195–199 — F-01, F-02, F-04**  
Actual:
“## XII. Diferido deliberadamente (no implementar todavía)
- **XII.1** `terminal`/PTY — la arquitectura de sesiones (broker + adapter) debe permitir agregarlo después como una extensión de la superficie de una sesión existente, sin rediseño.
- **XII.2** Bundle con manifiesto SHA-256, prueba de admisión y CAS de interacciones — se introducen una vez el broker y al menos dos harnesses reales estén funcionando. Mientras tanto, contexto inmutable simple (copia + digest) es suficiente.
- **XII.3** Concurrencia multi-usuario compartida — no se implementa ahora; la interfaz de persistencia (III.3) se diseña para no bloquear esta evolución.”  
Propuesto:
“## XII. Estado implementado y diferidos deliberados
- **XII.1** `terminal`/PTY ya está implementado por Spec 9c (`src/runtime/pty.ts`, `src/runtime/terminal.ts`, `src/runtime/attach.ts`; `SPECS.md` §9c) como superficie existente a preservar: PTY real, attach humano y presencia.
- **XII.2** El bundle inmutable, su manifiesto SHA-256 y la prueba de admisión ya están implementados por Spec 9a (`src/runtime/bundle.ts`, `src/schema/bundle.ts`, `src/runtime/admission.ts`) y quedan bajo no-regresión (`v2-no-regresion`); el CAS de interacciones permanece diferido hasta su spec correspondiente.
- **XII.3** Concurrencia multi-usuario compartida — no se implementa ahora; la interfaz de persistencia (III.3) se diseña para no bloquear esta evolución.
- **XII.4** Deuda conocida: el parser JSONL de OpenCode permanece en `src/utils/agent.ts` (ruta headless) y `v2-adapter/F-04` debe trasladarlo al adapter.”

### `docs/v2/shardeo-v2-especificacion-tecnica.md`

**Hunk T1 — §7, línea 535 — F-03**  
Actual: “Transporte: JSON-RPC 2.0 sobre stdio del subproceso del harness (o HTTP local + SSE como fallback de adapter, ver VII.4 de la constitución). Reutiliza directamente la forma de métodos de Agent Client Protocol.”  
Propuesto: “Transporte: el adapter elige el transporte nativo que su harness soporta —por ejemplo, stdio-RPC, HTTP local + SSE o JSONL—, lo declara en capacidades y lo negocia en `initialize`. El contrato reutiliza la forma de métodos de Agent Client Protocol. (Ejemplo no normativo: OpenCode supervised usa HTTP local + SSE gestionado.)”

**Hunk T2 — §7, fila `terminal/*`, línea 546 — F-01**  
Actual: “| `terminal/*` *(opcional, diferido)* | — | — | Ver XII.1 de la constitución — no implementado en el primer corte |”.  
Propuesto: “| `terminal/*` *(opcional, implementado)* | — | — | PTY real, attach humano y presencia; superficie existente a preservar. Ver Spec 9c (`src/runtime/pty.ts`, `src/runtime/terminal.ts`, `src/runtime/attach.ts`; `SPECS.md` §9c). |”

**Hunk T3 — §7, después de la tabla, línea 547 — F-04**  
Actual: no existe nota.  
Propuesto: “**Deuda confinada:** el parser JSONL de OpenCode permanece en `src/utils/agent.ts` (ruta headless, líneas 1596–1713); su traslado al adapter está pendiente y `v2-adapter/F-04` debe completarlo.”

## Flujo, contratos y archivos

    Specs 9a–9c + código → hunks F-01..F-04 → diff del PR (U-01) → revisión

Solo cambian esos archivos. §4 permanece conceptual; TypeScript + ACP + zod siguen autoritativos, sin aclaración adicional.

## Orden y verificación

Aplicar C1 → C2 → T1 → T2 → T3. Revisar por criterio: HTTP+SSE sin `fallback`; terminal con PTY/attach/presencia; bundle/admisión remitidos a `v2-no-regresion`; CAS y XII.3 diferidos; dos notas F-04; solo dos documentos. Sin tests de código ni migración.

## Threat Matrix

N/A — el change es documental y no modifica routing, shell, subprocess, VCS/PR automation, clasificación de ejecutables ni integración de procesos.

## Criterio de completitud y preguntas abiertas

Completo cuando cada texto, ubicación, orden y mapeo F-01..F-04 puede aplicarse sin decisiones adicionales y el diff satisface U-01. Preguntas abiertas: ninguna.
