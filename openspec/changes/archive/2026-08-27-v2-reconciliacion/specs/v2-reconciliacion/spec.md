# Especificación de v2-reconciliacion

## Propósito

Reconciliar la normativa v2 con Specs 9a–9c mediante cambios exclusivamente documentales, verificables contra `shardeo-v2-deltas-acceptance.md` y el diff de la actualización.

## Requirements

### Requirement: Terminal/PTY reconocido como superficie existente [v2-reconciliacion/F-01]

La Constitución XII.1 y la fila `terminal/*` de la Especificación §7 DEBEN reconocer `terminal`/PTY como superficie implementada que preserva PTY real, attach humano y presencia, con referencia a Spec 9c (`src/runtime/pty.ts`, `terminal.ts`, `attach.ts`; `SPECS.md` §9c).

#### Scenario: Estado implementado
- GIVEN los documentos finales
- WHEN se revisan XII.1 y la fila `terminal/*`
- THEN `terminal`/PTY ya no figura como diferido ni no implementado

#### Scenario: Preservación explícita
- GIVEN la nueva redacción
- WHEN se buscan PTY real, attach humano y presencia
- THEN los tres comportamientos aparecen como existentes a preservar

### Requirement: Bundle y admisión reconocidos sin habilitar CAS [v2-reconciliacion/F-02]

La Constitución XII.2 DEBE reconocer como implementados el bundle inmutable, manifiesto SHA-256 y prueba de admisión de Spec 9a (`src/runtime/bundle.ts`, `src/schema/bundle.ts`, `src/runtime/admission.ts`) y remitirlos a `v2-no-regresion`; el CAS de interacciones y XII.3 multiusuario DEBEN permanecer diferidos.

#### Scenario: Bundle fuera de diferido
- GIVEN la Constitución final
- WHEN se revisa XII.2
- THEN bundle, manifiesto SHA-256 y admisión no figuran como trabajo diferido

#### Scenario: Diferidos preservados
- GIVEN la Constitución final
- WHEN se revisan XII.2 y XII.3
- THEN CAS de interacciones y multiusuario siguen explícitamente diferidos

### Requirement: Transporte elegido por capacidades [v2-reconciliacion/F-03]

VII.4 y la Especificación §7 DEBEN definir abstractamente que cada adapter elige un transporte nativo soportado por su harness —stdio-RPC, HTTP local + SSE o JSONL—, lo declara en capacidades y lo negocia en `initialize`. OpenCode HTTP+SSE PUEDE aparecer solo como ejemplo no normativo (`src/runtime/transport.ts`, `src/adapters/opencode.ts:296-326`, `src/adapters/opencode-http.ts`).

#### Scenario: Regla abstracta consistente
- GIVEN ambos documentos finales
- WHEN se inspeccionan VII.4 y §7
- THEN ambos expresan selección por capacidades y negociación en `initialize`

#### Scenario: HTTP+SSE no es fallback
- GIVEN la redacción sobre HTTP local + SSE
- WHEN se ejecuta una aserción textual contextual
- THEN la palabra `fallback` no califica a HTTP+SSE ni este se presenta como vía inferior

### Requirement: Deuda JSONL confinada y trazable [v2-reconciliacion/F-04]

La Constitución XII DEBE incluir una nota de una línea y la Especificación §7 DEBE detallar que el parser JSONL de OpenCode permanece en `src/utils/agent.ts` (ruta headless; líneas 1596–1713), y que `v2-adapter` DEBE trasladarlo al adapter.

#### Scenario: Referencia exacta
- GIVEN los documentos finales
- WHEN se buscan `src/utils/agent.ts`, `headless` y `v2-adapter`
- THEN la nota breve existe en XII y el detalle completo existe en §7

#### Scenario: Deuda no declarada resuelta
- GIVEN la nota F-04
- WHEN se revisa su estado
- THEN el traslado se describe como pendiente, no como comportamiento ya corregido

### Requirement: Actualización auditable y acotada [v2-reconciliacion/U-01]

El diff del PR DEBE ser la única autoridad de la actualización y DEBE contener solo cambios en los dos documentos `docs/v2/` mapeables a F-01..F-04; NO DEBE crear changelogs ni artefactos documentales adicionales. Las interfaces Go de §4 DEBEN permanecer conceptuales, con TypeScript + ACP + zod como autoridades si resulta necesaria una aclaración.

#### Scenario: Diff justificable
- GIVEN el diff final del PR
- WHEN cada hunk se asigna a un criterio
- THEN todos se mapean exactamente a F-01, F-02, F-03 o F-04 con su motivo

#### Scenario: Sin ampliación de alcance
- GIVEN el diff final del PR
- WHEN se enumeran archivos y secciones modificados
- THEN solo aparecen los dos documentos y los hunks acordados, sin cambios de código ni `SPECS.md`
