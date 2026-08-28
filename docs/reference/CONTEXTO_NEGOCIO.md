# Contexto de negocio del proyecto

Proyecto analizado: **Shardeo** (CLI, TypeScript). Este documento describe exclusivamente el contexto funcional y las reglas de negocio del sistema, deducidas del comportamiento real implementado (código fuente en `src/`, contratos en `SPECS.md` y `AGENTS.md`). Las referencias de evidencia apuntan a los archivos que demuestran cada regla; el documento es autosuficiente y no requiere leerlos.

---

## 1. Resumen ejecutivo

Shardeo es una herramienta de línea de comandos que **estructura la ejecución de flujos de trabajo de desarrollo de software asistidos por IA**. No es un agente ni un IDE: no decide ni razona sobre el trabajo. Su función es separar tres responsabilidades que normalmente están mezcladas en una sesión con un agente de IA:

- **La metodología** (qué hacer y en qué orden): definida por humanos como *workflows* declarativos.
- **El conocimiento** (con qué contexto): instrucciones, convenciones y artefactos del proyecto.
- **La ejecución** (quién lo hace): runtimes de agentes de IA intercambiables.

Un **agente orquestador** (otro CLI de IA) conduce el proceso paso a paso consultando a Shardeo qué se puede ejecutar, pidiéndole que ejecute cada unidad de trabajo y decidiendo cuándo un paso está terminado. Shardeo valida todo antes de ejecutar, invoca al runtime adecuado con el contexto correcto y persiste evidencia trazable de todo lo ocurrido.

El problema que resuelve: las sesiones con agentes de IA parten de cero, el contexto se pasa de forma inconsistente, no hay registro de qué se ejecutó ni con qué resultado, y cambiar de herramienta implica rehacer la configuración. Shardeo convierte las convenciones de un equipo en procesos reproducibles, auditables y reanudables.

## 2. Objetivos del sistema

**Objetivo principal:** permitir que un equipo ejecute metodologías de desarrollo (diseño, contratos de API, base de datos, lint, implementación, verificación, etc.) como workflows estructurados, con cualquier runtime de agente, sin perder trazabilidad ni continuidad.

**Objetivos secundarios:**

1. **Reproducibilidad:** cualquier miembro del equipo puede ejecutar el mismo workflow con el mismo contexto y las mismas validaciones.
2. **Trazabilidad total:** cada invocación a un agente queda registrada (respuesta, feedback, artefactos, herramienta usada, motivo de finalización, métricas).
3. **Resiliencia:** un workflow interrumpido puede retomarse desde el último estado conocido, incluso reconstruyendo artefactos perdidos.
4. **Independencia del proveedor:** los modelos y CLIs son runtimes intercambiables; Shardeo no compite con ellos.
5. **Seguridad deliberada:** sin atajos de permisos, sin interactividad oculta, con límites explícitos sobre qué evidencia se muestra y qué se retiene.
6. **Neutralidad metodológica:** Shardeo no interpreta respuestas de agentes ni incorpora decisiones de negocio del dominio del usuario; esas decisiones pertenecen al orquestador y a las instrucciones del workflow.

## 3. Actores

| Actor | Función |
|---|---|
| **Usuario humano (ingeniero)** | Define workflows, skills y convenciones; aprueba permisos cuando la superficie lo exige; atiende handoffs de terminal; decide iteraciones a través del orquestador. |
| **Agente orquestador** | CLI de IA que consulta los workflows disponibles, inicia ejecuciones, pregunta qué steps están listos, ordena su ejecución, interpreta resultados según las instrucciones del workflow y materializa sus decisiones mediante comandos explícitos (`step complete`, `step reopen`, `step skip`, `step approve`). |
| **Agente ejecutor (runtime / harness)** | Proceso de IA que realiza el trabajo de un step `agent`. En v1 el único soportado es OpenCode; el modelo concreto es opcional y opaco para Shardeo. |
| **Shardeo (ejecutor determinista)** | Valida workflows, resuelve el orden, inyecta contexto, ejecuta steps `command`, gestiona la invocación de agentes, aplica políticas de permisos y persiste estado y evidencia. Nunca decide metodología. |
| **Supervisor local (modo gestionado)** | Proceso de fondo propietario de un intento `supervised`/`terminal`: congela y admite el contexto, normaliza eventos, media interacciones y aplica temporizadores. |

## 4. Conceptos del dominio

- **Workflow.** Metodología declarada en `.shardeo/workflows/<nombre>/workflow.yaml` más un archivo de instrucciones generales dirigido al orquestador. Restricciones: nombre obligatorio, al menos un step, grafo acíclico con punto de entrada, solo runtimes permitidos.

- **Step.** Unidad de trabajo. Dos tipos: `command` (proceso determinista que se autocompleta si tiene éxito) y `agent` (trabajo de IA que nunca se autocompleta). Propiedades con responsabilidad única: `depends_on` define orden; `requires` valida entradas al ejecutar; `produces` valida salidas tras ejecutar; `agents` lista candidatos de runtime en orden de preferencia; `skills` e `instructions` aportan contexto; `mode` elige la superficie de ejecución.

- **Ejecución (execution).** Instancia concreta de un workflow con identificador único. Todos sus steps nacen pendientes y posee un estado agregado derivado del estado de sus steps.

- **Intento (attempt).** Cada invocación de un runtime dentro de un step, incluidas las automáticas por fallback y cada re-ejecución manual. Es la unidad mínima de evidencia y auditoría.

- **Artefacto.** Documento de conocimiento del proceso (plan, decisión de arquitectura, contrato, reporte). Vive exclusivamente bajo `.shardeo/artifacts/` y se gestiona mediante `requires`/`produces`. Se distingue del código fuente, que el agente modifica directamente en el repositorio y Shardeo no trackea ni valida.

- **Generación.** Versión vigente del resultado de un step. Reabrir un step invalida su generación: los archivos permanecen en disco e historial para auditoría, pero dejan de ser válidos como entradas hasta que un nuevo intento exitoso produzca una generación revalidada.

- **Modo de ejecución.** Superficie sobre la que corre un step `agent`: `headless` (automatizado y bloqueante), `supervised` (un supervisor local media permisos estructurados) o `terminal` (una persona controla una terminal real). No son variantes intercambiables: cambian quién manda y cómo se resuelven las interacciones.

- **Interacción.** Solicitud estructurada del harness que exige una decisión externa (el mínimo soportado es el permiso). Cada interacción anuncia sus decisiones válidas; Shardeo nunca inventa opciones ni amplía autoridad.

- **Bundle de contexto.** Copia inmutable por intento de todos los materiales autorizados, con digests criptográficos. Garantiza reproducibilidad aunque los archivos vivos cambien durante el intento.

- **Evidencia sanitizada.** Todo lo visible está limpio y acotado; los bytes diagnósticos crudos tienen un único depósito autorizado con digest SHA-256.

## 5. Reglas de negocio

Convenciones: **Confianza Alta** = verificado directamente en código y/o pruebas automatizadas; **Media** = deducida de código más especificación con algún matiz; **Baja** = inferencia plausible con evidencia parcial. Los códigos citados entre comillas (por ejemplo `permission_required`) son razones de finalización (`completion_reason`) estables del sistema.

### Área A — Definición y validación de workflows

#### RN-001 — Runtime exclusivo en v1
**Regla:** el único identificador de agente válido es `opencode`; declarar cualquier otro invalida todo el workflow.
**Condiciones:** validación de schema en cada carga del workflow.
**Resultado:** workflow inválido; ninguna operación puede usarlo hasta corregirlo.
**Confianza:** Alta. **Evidencia:** `src/schema/workflow.ts` (whitelist `VALID_AGENT_IDENTIFIERS`), `src/utils/workflow.ts` (`validateStepAgents`).

#### RN-002 — Modelo opcional y opaco
**Regla:** el modelo asociado a un candidato es opcional y se transfiere sin interpretar; la validez semántica del modelo es responsabilidad del runtime, no de Shardeo.
**Condiciones:** resolución de candidatos de un step `agent`.
**Resultado:** el valor pasa íntegro al CLI invocado.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts` (resolución de candidatos), SPECS.md Spec 4.

#### RN-003 — Revalidación íntegra en cada operación
**Regla:** el YAML completo se vuelve a leer y validar antes de ejecutar un step y en cada transición relevante (listado, descripción, inicio de ejecución, consulta de pasos, completar, reabrir, omitir, reanudar), porque el archivo puede cambiar entre operaciones.
**Condiciones:** cualquier comando que cargue el workflow.
**Resultado:** si la validación falla, se retorna el error sin invocar CLIs ni modificar estado de steps o ejecuciones.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts` (`cmdStepRun`, `cmdStepComplete`, `cmdStepReopen`, `cmdStepSkip`), `src/commands/resume.ts`.

#### RN-004 — Compatibilidad hacia adelante controlada
**Regla:** los campos desconocidos de nivel superior del workflow se toleran para permitir evolución sin romper el descubrimiento; en cambio, el campo de dependencias obsoleto `after` se rechaza explícitamente con un mensaje que dirige a usar `depends_on`.
**Condiciones:** parseo del workflow.
**Resultado:** los campos nuevos no rompen; `after` produce error de validación específico.
**Confianza:** Alta. **Evidencia:** `src/schema/workflow.ts` (`.passthrough()` y preprocesador de `after`).

#### RN-005 — Contenido mínimo obligatorio
**Regla:** todo workflow exige nombre no vacío y al menos un step; sin ellos es inválido.
**Confianza:** Alta. **Evidencia:** `src/schema/workflow.ts` (`WorkflowFile`).

### Área B — Grafo y orden de ejecución

#### RN-006 — Identificadores únicos
**Regla:** los IDs de steps deben ser únicos dentro del workflow; un duplicado hace ambiguas las referencias e invalida el workflow con error explícito.
**Confianza:** Alta. **Evidencia:** `src/utils/workflow.ts` (`analyzeWorkflowGraph`, código `duplicate_step_id`).

#### RN-007 — Referencias de dependencia existentes
**Regla:** todo `depends_on` debe referenciar un step existente del mismo workflow.
**Resultado:** en caso contrario la ejecución se rechaza con el código `invalid_depends_on_reference` indicando la referencia rota.
**Confianza:** Alta. **Evidencia:** `src/utils/workflow.ts` (`analyzeWorkflowGraph`).

#### RN-008 — Punto de entrada obligatorio
**Regla:** debe existir al menos un step sin `depends_on`; es la puerta de entrada del workflow.
**Resultado:** sin punto de entrada, `shardeo run` rechaza iniciar la ejecución (`no_entry_point`).
**Confianza:** Alta. **Evidencia:** `src/utils/workflow.ts` (`analyzeWorkflowGraph`), `src/commands/run.ts`.

#### RN-009 — Grafo acíclico
**Regla:** no pueden existir ciclos de dependencia; el error identifica la ruta cerrada completa de steps involucrados.
**Confianza:** Alta. **Evidencia:** `src/utils/workflow.ts` (DFS iterativa con detección de ciclo y ruta).

#### RN-010 — depends_on como único criterio de orden
**Regla:** solo `depends_on` define precedencia. `requires` y `produces` validan datos, pero nunca crean orden ni paralelismo implícito.
**Confianza:** Alta. **Evidencia:** `src/utils/dag.ts`, README, SPECS.md Specs 3/5.

#### RN-011 — Disponibilidad de un step
**Regla:** un step está disponible cuando todos sus predecessores declarados alcanzaron un estado terminal positivo: `completed` o `skipped`. Un step omitido satisface el orden aunque no produzca nada.
**Condiciones:** cómputo de `steps next`.
**Confianza:** Alta. **Evidencia:** `src/utils/dag.ts` (`getReadySteps`), `src/db/queries.ts` (conjunto completed+skipped), `src/commands/steps.ts::cmdStepsNext`.

#### RN-012 — Bloqueo por predecessores incompletos
**Regla:** intentar ejecutar un step cuyos predecessores no están terminalizados falla con error explícito enumerando los predecessores incompletos; no hay ejecución forzada.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::cmdStepRun` (verificación previa).

#### RN-013 — Paralelismo delegado al orquestador
**Regla:** si varios steps están disponibles simultáneamente, se retornan todos juntos; la decisión de ejecutarlos en paralelo o en serie pertenece al orquestador. Shardeo nunca ejecuta nada en paralelo por sí mismo.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::cmdStepsNext`, SPECS.md Spec 5.

### Área C — Ejecución básica de steps

#### RN-014 — Creación atómica de ejecuciones
**Regla:** `shardeo run` valida el workflow y crea la ejecución junto con todos sus steps en estado pendiente en una sola operación atómica, retornando un identificador único. Un workflow inválido no genera ningún registro.
**Confianza:** Alta. **Evidencia:** `src/commands/run.ts`, `src/db/queries.ts` (`createExecutionWithSteps`).

#### RN-015 — Estados que impiden ejecutar
**Regla:** no se puede ejecutar un step en estado `skipped`; tampoco un step `agent` ya `completed`. Los steps `failed` o `pending` sí pueden re-ejecutarse.
**Resultado:** error con el estado actual; sin cambios.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::cmdStepRun`.

#### RN-016 — Autocompletado exclusivo de steps command
**Regla:** un step `command` se marca completado automáticamente cuando el proceso termina con código 0 y todos sus `produces` existen y son válidos bajo `.shardeo/artifacts/`. Si el comando falla, o tiene éxito pero los artefactos no aparecen (o el snapshot excede límites), el step queda fallido con el motivo correspondiente.
**Excepciones:** un comando vacío se considera exitoso trivialmente (código 0).
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::runCommandStep`.

#### RN-017 — Los steps agent nunca se autocompletan
**Regla:** una invocación exitosa de un agente deja el step en estado `running`, no `completed`. Solo el orquestador puede cerrarlo invocando `step complete`, momento en que Shardeo revalida los artefactos declarados en `produces`.
**Condiciones:** steps tipo `agent`.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::cmdStepComplete`, SPECS.md Spec 4.

#### RN-018 — Condiciones para completar un agent
**Regla:** `step complete` exige que el step esté `running`, que exista al menos un intento finalizado con snapshot válido y que todo el conjunto actual de `produces` esté presente y sea seguro. Si falta algo, el comando falla y el step permanece en progreso.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::cmdStepComplete` (`no_completed_attempts`, `no_snapshot_data`, `incomplete_generation_outputs`).

#### RN-019 — Validación de entradas al ejecutar
**Regla:** los artefactos declarados en `requires` se validan al momento de ejecutar el step (nunca al resolver el orden). Si faltan, el step no se ejecuta y se retorna la lista de ausentes. Si existen pero provienen de generaciones invalidadas, se reportan como obsoletos (`stale_required_artifacts`) aunque el archivo físico exista.
**Confianza:** Alta. **Evidencia:** `src/utils/execution.ts` (`validateRequires`, `validateRequiresForExecution`).

### Área D — Candidatos, fallback y terminales

#### RN-020 — Candidatos solo desde agents[], en orden
**Regla:** los candidatos de runtime salen exclusivamente de la lista `agents` del step y se recorren en el orden declarado. Cada candidato se sondea perezosamente; uno no disponible se descarta sin crear intento.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts` (`probeNextAgentCandidate`, registros `EvaluatedProbe`).

#### RN-021 — Sin candidatos disponibles
**Regla:** si ninguna sonda supera la validación antes de invocar, el step falla mostrando evidencia sanitizada de todos los candidatos evaluados y no se crea ningún intento.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::runAgentStep` (respuesta con `evaluated_probes` y `attempts: []`).

#### RN-022 — Éxito estricto
**Regla:** una invocación es exitosa solo si el proceso terminó limpio, el stream JSONL fue válido y no hubo evento de error con salida 0. En ese caso el intento se persiste como éxito y el step queda `running`.
**Confianza:** Alta. **Evidencia:** `src/utils/agent.ts` (`invokeAgent`, `finishClosedProcess`).

#### RN-023 — Fallback sin clasificación semántica
**Regla:** tras una invocación iniciada y limpiada correctamente con JSONL válido, cualquier error limpio hace avanzar al siguiente candidato sin clasificar su causa: cuota, contexto, disponibilidad o modelo se tratan igual (razón genérica `unknown_error` con el error concreto del proveedor preservado en evidencia sanitizada). El nuevo intento recibe un contexto extendido con lo original más la proyección de lo ocurrido.
**Motivo de negocio:** OpenCode no expone una taxonomía estable de errores; clasificarlos mal sería peor que no clasificarlos.
**Confianza:** Alta. **Evidencia:** `src/utils/errors.ts`, `src/utils/agent.ts::buildFallbackEvidence`, SPECS.md Spec 4.

#### RN-024 — Razones terminales enumeradas
**Regla:** existe una lista cerrada de motivos que impiden el fallback y hacen fallar el step de inmediato: `permission_required`, `permission_detection_unsupported`, `permission_timeout`, `process_inactivity_timeout`, `interaction_timeout`, `artifact_context_too_large`, `adapter_contract_error`, `process_start_failed`, `process_cleanup_failed`, `context_error` y `persistence_error`.
**Confianza:** Alta. **Evidencia:** `src/utils/errors.ts` (`TERMINAL_COMPLETION_REASONS`), usada en `src/commands/steps.ts`.

#### RN-025 — Agotamiento de candidatos
**Regla:** si el último candidato disponible falla con motivo no terminal, el step termina fallido por agotamiento (`exhaustion`) con el detalle de todos los intentos realizados.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::runAgentStep`.

#### RN-026 — Cada invocación es un intento independiente
**Regla:** toda invocación de CLI (manual, por fallback o por re-ejecución) se registra como intento separado con número, respuesta completa o parcial, feedback, artefactos generados, runtime y modelo usados, motivo de finalización y métricas de tiempo.
**Confianza:** Alta. **Evidencia:** `src/db/queries.ts` (`finalizeAttemptCompleted`, `finalizeAttemptAndInsertNext`, `finalizeAttemptAndFailStep`), SPECS.md Spec 6.

### Área E — Contexto entregado al agente

#### RN-027 — Orden contractual del contexto
**Regla:** el contexto inyectado al agente respeta siempre este orden: instrucciones operacionales (generadas por Shardeo), instrucciones de dominio del step, skills, artefactos requeridos, artefactos generados en intentos previos, feedback de reapertura, feedback del usuario y, por último, evidencia de fallback si la hay.
**Confianza:** Alta. **Evidencia:** `src/utils/agent.ts` (`assembleAgentContext`, `buildContextWithContainment`).

#### RN-028 — Presupuestos de tamaño por componente
**Regla:** cada componente del contexto tiene límites máximos: instrucciones 256 KiB (por archivo y acumulado); skills 256 KiB por archivo y 1 MiB acumulado; artefactos requeridos 1 MiB por archivo y acumulado; artefactos de intentos previos 2 MiB acumulado. Exceder cualquier presupuesto invalida el contexto y el step falla cerrado con `context_error` antes de invocar al agente. El contenido binario no-UTF-8 se sustituye por un marcador con su tamaño y digest.
**Confianza:** Alta. **Evidencia:** `src/utils/agent.ts` (`CONTEXT_LIMITS`, `readContainedText`, `decodePriorArtifactText`).

#### RN-029 — Iteración con memoria
**Regla:** un step no completado puede re-ejecutarse sin límite de veces. Cada re-ejecución recibe los artefactos generados en intentos anteriores y, si se provee, el feedback del usuario claramente delimitado dentro del contexto. El feedback es obligatoriamente texto no vacío.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::cmdStepRun` (`--feedback`), `getPriorAttemptArtifacts`, SPECS.md Spec 6.

### Área F — Tiempos y actividad

#### RN-030 — Timeout de inactividad reiniciable
**Regla:** una invocación de agente termina con `process_inactivity_timeout` cuando no produce actividad observable durante el periodo configurado (5 minutos por defecto, configurable por proyecto). Cualquier evento de salida del proceso reinicia el contador; un latido interno no cuenta como actividad.
**Confianza:** Alta. **Evidencia:** `src/utils/agent.ts` (temporizador de inactividad, `DEFAULT_INACTIVITY_TIMEOUT_MS`), `src/schema/config.yaml` vía `src/schema/config.ts`.

#### RN-031 — Techo fijo para comandos
**Regla:** los steps `command` tienen un tiempo máximo absoluto de ejecución de 300 segundos, no configurable a nivel de step.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::runCommandStep` (timeout fijo), `src/utils/execution.ts` (`runCommand`).

#### RN-032 — Temporizadores independientes por superficie
**Regla:** existen tres temporizadores con contadores y errores distintos: inactividad del proceso (aplica a las tres superficies), decisión de interacción (solo `supervised`) y presencia humana (solo `terminal`). Al expirar cualquiera se solicita el cierre seguro conservando evidencia; si el cierre no puede confirmarse, el intento se marca `orphaned` en lugar de declararse terminado.
**Confianza:** Alta. **Evidencia:** SPECS.md Spec 9 (tabla de timeouts), `src/runtime/terminal.ts`, `src/runtime/supervisor.js`.

### Área G — Permisos e interacciones

#### RN-033 — Detección de permisos solo por señales exactas
**Regla:** en modo headless, una solicitud de aprobación del agente se reconoce únicamente mediante señales exactas y versionadas del harness. Una coincidencia terminaliza el intento con `permission_required`. Una salida ambigua o no reconocida no se adivina: falla cerrada con `permission_detection_unsupported` o `adapter_contract_error`.
**Motivo de negocio:** preferir fallar antes que autorizar algo sin certeza.
**Confianza:** Alta. **Evidencia:** `src/adapters/opencode.ts` (detector con identidad de versión y señal de auto-rechazo), `src/utils/agent.ts` (consumo del detector).

#### RN-034 — Nunca bypass de permisos
**Regla:** los agentes se invocan en modo no interactivo sin shell y sin banderas de omisión de permisos; esas banderas nunca se inyectan por el sistema. Si un step falla porque el agente necesitó preguntar, el problema se resuelve mejorando las instrucciones o usando `supervised`/`terminal`.
**Confianza:** Alta. **Evidencia:** `src/utils/agent.ts` (`spawn` con `shell: false`), README, AGENTS.md.

#### RN-035 — Política de permisos por capas y decisiones anunciadas
**Regla:** la política de permisos se hereda por capas (`step > workflow > config`, valor por defecto `prompt`; no se mezclan listas entre capas). En modos gestionados, toda autorización automática exige coincidencia exacta de capacidad normalizada y que esa decisión esté anunciada para la interacción concreta; `deny` siempre precede a cualquier autorización automática; una capacidad desconocida jamás coincide con reglas de autorización y usa el default seguro; una regla que pide una decisión no anunciada falla sin degradarse. `allow_once` autoriza exclusivamente la solicitud actual.
**Confianza:** Alta. **Evidencia:** SPECS.md Spec 9 (política de permisos), `src/runtime/supervisor.js` (aplicación de política con actor `policy`).

#### RN-036 — Resolución única e idempotente
**Regla:** cada interacción transiciona una sola vez de pendiente a resuelta mediante comparación-y-asignación. Repetir la misma decisión retorna el resultado almacenado sin reenviarla; intentar una decisión distinta tras resolver falla con `interaction_already_resolved`. Toda resolución automática queda registrada con actor `policy` y referencia a la regla aplicada.
**Confianza:** Alta. **Evidencia:** SPECS.md Spec 9 (resolución idempotente), broker de interacciones.

### Área H — Evidencia, sanitización y datos

#### RN-037 — Salida visible acotada y única autoridad cruda
**Regla:** todo lo que se muestra en consola y se persiste en proyecciones estándar está sanitizado (sin rutas absolutas del host ni caracteres de control) y limitado a 16 KiB por intento. Los bytes diagnósticos sin redactar tienen un único depósito autorizado (`DiagnosticRaw`): hasta 1 MiB se conserva el frame exacto; por encima se guardan prefijo, sufijo, tamaño original y digest SHA-256.
**Confianza:** Alta. **Evidencia:** `src/utils/agent.ts` (`MAX_CONSOLE_BYTES`, `MAX_DIAGNOSTIC_RAW_BYTES`, `encodeDiagnosticRaw`), README.

#### RN-038 — Contexto de fallback acotado
**Regla:** la proyección entregada al siguiente candidato tras un fallo con fallback tiene un presupuesto acumulado de 2 MiB por ejecución del step; incluye errores concretos del proveedor y referencias a intentos previos, nunca evidencia cruda. Exceder el presupuesto terminaliza con `artifact_context_too_large`.
**Confianza:** Alta. **Evidencia:** `src/utils/agent.ts` (`MAX_FALLBACK_BYTES`, `buildFallbackEvidence`), `src/commands/steps.ts::runAgentStep`.

#### RN-039 — Snapshots con presupuesto acumulado
**Regla:** antes de cada intento se captura un baseline de los artefactos declarados en `produces`; después solo se snapshottean archivos nuevos o modificados, cada uno con su SHA-256. El contenido acumulado por step tiene techo de 1 MiB: superarlo produce `artifact_context_too_large` y falla el intento.
**Confianza:** Alta. **Evidencia:** `src/utils/agent.ts` (`MAX_STEP_SNAPSHOT_BYTES`), `src/commands/steps.ts::runAgentStep`.

### Área I — Artefactos

#### RN-040 — Raíz única y contención estricta
**Regla:** todos los artefactos viven exclusivamente bajo `.shardeo/artifacts/`. Las rutas declaradas en `requires` y `produces` rechazan rutas absolutas, segmentos `..` y escapes mediante symlinks, tanto para steps `agent` como `command`. Un symlink cuyo destino permanece dentro de la raíz no escapa.
**Motivo de negocio:** los artefactos son la moneda de intercambio entre steps; su integridad y ubicación predecible sostienen la trazabilidad y la seguridad.
**Confianza:** Alta. **Evidencia:** `src/utils/agent.ts` (`validateContainedPath`, `isParentEscape`), `src/utils/execution.ts`.

#### RN-041 — Vigencia generacional de los artefactos
**Regla:** un artefacto requerido solo es válido si proviene del productor cuya generación vigente coincide con el estado actual (step completado, sin invalidación) y cuyo manifiesto de generación (rutas, tamaños, digests) coincide con lo físico. Un artefacto huérfano o proveniente de una generación invalidada se reporta como obsoleto aunque exista en disco.
**Confianza:** Alta. **Evidencia:** `src/utils/execution.ts::validateRequiresForExecution`, `src/db/queries.ts` (`step_generation_invalidations`).

#### RN-042 — Sobrescritura e historial
**Regla:** los artefactos generados en una re-ejecución sobrescriben los de intentos anteriores en el filesystem, pero las versiones anteriores permanecen registradas en la base de datos como evidencia. La validación de `produces` al completar recae siempre sobre el conjunto actual.
**Confianza:** Alta. **Evidencia:** SPECS.md Spec 6, `getPriorAttemptArtifacts` en `src/db/queries.ts`.

#### RN-043 — Naturaleza de los artefactos producidos
**Regla:** para validar una generación se exige que cada artefacto producido sea un archivo regular dentro de la raíz; los symlinks y otros tipos de archivo son rechazados en esa validación.
**Nota:** el contrato escrito contemplaba permitir symlinks internos; el código implementó el criterio más estricto. Ver sección 12.
**Confianza:** Media. **Evidencia:** `src/utils/execution.ts::inspectProducedArtifacts` frente a SPECS.md Spec 4.

### Área J — Iteración dirigida por el orquestador

#### RN-044 — Omisión solo de trabajo virgen
**Regla:** `step skip` solo admite steps pendientes que no tengan ningún intento iniciado y sin generación válida; requiere motivo obligatorio. El resultado es terminal.
**Confianza:** Alta. **Evidencia:** `src/db/queries.ts::skipStepAtomic`, `src/commands/steps.ts::cmdStepSkip`.

#### RN-045 — Efecto funcional de omitir
**Regla:** un step omitido satisface el orden (libera a sus dependientes) pero no genera artefactos ni satisface validaciones `requires`: cualquier dependiente que necesite sus salidas fallará normalmente al validar entradas.
**Confianza:** Alta. **Evidencia:** SPECS.md Spec 8, RN-011 y RN-019.

#### RN-046 — Reapertura solo de trabajo terminado
**Regla:** `step reopen` admite únicamente steps `completed` o `failed`, con feedback obligatorio (texto no vacío hasta 16 KiB). No se pueden reabrir steps pendientes, en ejecución u omitidos.
**Confianza:** Alta. **Evidencia:** `src/db/queries.ts::reopenStepAtomic`, `src/commands/steps.ts::cmdStepReopen`.

#### RN-047 — Cascada obligatoria ante descendientes afectados
**Regla:** si reabrir un step invalidaría descendientes en estado `running`, `completed` o `failed` y no se provee `--cascade`, el comando falla enumerando qué steps serían reiniciados. Con cascada, los descendientes afectados vuelven a pendiente, se incrementa su número de generación y se registra la invalidación; su historial de intentos se conserva.
**Confianza:** Alta. **Evidencia:** `src/db/queries.ts::reopenStepAtomic` (`cascade_required`, `cascade_reset`).

#### RN-048 — Invalidación generacional al reabrir
**Regla:** reabrir invalida la generación vigente del step: sus artefactos permanecen en disco e historial para auditoría, pero no satisfacen `requires` ni permiten completar de nuevo hasta que finalice con éxito al menos un nuevo intento y el conjunto actual de `produces` sea revalidado como nueva generación. Los archivos que no necesitaban cambios pueden conservar los mismos bytes: la validez pertenece a la generación, no a la reescritura.
**Confianza:** Alta. **Evidencia:** `src/db/queries.ts` (`insertGenerationInvalidation`), SPECS.md Spec 8.

#### RN-049 — Ocupación y conciliación
**Regla:** una transición de reapertura u omisión falla si existe un intento vivo con lease vigente (step ocupado). Los intentos con lease expirado se concilian automáticamente como interrumpidos antes de proceder. Toda transición registra motivo, actor, marca temporal y estados origen/destino en un registro de auditoría.
**Confianza:** Alta. **Evidencia:** `src/db/queries.ts` (`reopenStepAtomic`, `reconcileInterruptedAttempt`, `step_transition_events`).

### Área K — Estado agregado y reanudación

#### RN-050 — Estado agregado derivado transaccionalmente
**Regla:** el estado de la ejecución se recalcula con cada transición de step, dentro de la misma transacción: si algún step falló, la ejecución está `failed`; si todos los steps están `completed` o `skipped`, está `completed`; en cualquier otro caso, `running`. La consulta de estado nunca muestra un agregado que diverja del almacenado.
**Confianza:** Alta. **Evidencia:** `src/db/queries.ts::syncExecutionStatusInTransaction`, SPECS.md Spec 7.

#### RN-051 — Reanudación segura
**Regla:** `resume` exige que el workflow asociado no haya cambiado desde el inicio de la ejecución (mismo conjunto de steps); si cambió, falla con `workflow_changed`. Concilia intentos interrumpidos (leases expirados), no re-ejecuta steps completados y retorna guía explícita de continuación.
**Confianza:** Alta. **Evidencia:** `src/commands/resume.ts`.

#### RN-052 — Reconstrucción de artefactos perdidos
**Regla:** si un step completado perdió sus archivos en disco, `resume` lo marca como "requiere reconstrucción" con la lista de faltantes y entrega la última respuesta del agente como contexto para reconstruirlos sin partir de cero.
**Confianza:** Alta. **Evidencia:** `src/commands/resume.ts` (`markStepReconstructionRequired`, `previous_agent_response`), SPECS.md Spec 7.

### Área L — Superficies de ejecución

#### RN-053 — Resolución de modo por capas
**Regla:** el modo efectivo de un step `agent` se resuelve como: override explícito del comando (solo para ese intento) sobre `step.mode`, luego `workflow.mode`, luego el default de config del proyecto; si nadie declara, `headless`. Un valor inválido en cualquier capa es error. Los steps `command` ignoran por completo los modos.
**Confianza:** Alta. **Evidencia:** `src/utils/mode.ts::resolveEffectiveMode`, `src/schema/mode.ts`, SPECS.md Spec 9.

#### RN-054 — Capacidades sondeadas y fallo cerrado
**Regla:** los modos gestionados exigen exactamente un candidato adapter y que este anuncie soporte real del modo tras sondear la instalación concreta. Un modo o capacidad no soportados producen `unsupported_capability` antes de lanzar trabajo del proveedor; nunca hay cambio silencioso de superficie ni ampliación de permisos.
**Confianza:** Alta. **Evidencia:** `src/commands/steps.ts::cmdStepRun` (sondeo previo), `src/runtime/terminal.ts` (`terminalCapabilityError`).

#### RN-055 — Supervised: arranque no bloqueante y seguimiento por eventos
**Regla:** en `supervised`, iniciar el intento congela y admite el contexto y abre la sesión; entonces el comando retorna de inmediato con identidades y cursor inicial. El seguimiento se hace consultando eventos estructurados con cursores monotónicos (sin pérdidas ni duplicados semánticos) y leyendo salida bajo demanda; los eventos de control contienen identidades y resúmenes, jamás la salida completa.
**Confianza:** Alta. **Evidencia:** SPECS.md Spec 9 (contrato de comandos y eventos), `src/runtime/supervisor.js`.

#### RN-056 — Terminal: control exclusivamente humano
**Regla:** el modo `terminal` entrega una terminal real a una persona. El orquestador solo comunica el handoff: nunca escribe teclas, no lee pantalla ni hace scraping de la interfaz. Conectarse exige el identificador exacto del único intento vivo (nunca se adjunta por inferencia ambigua); desconectarse no mata el proceso y se puede reconectar mientras el intento siga activo dentro de su plazo de presencia humana.
**Confianza:** Alta. **Evidencia:** SPECS.md Spec 9 (handoff humano), `src/runtime/terminal.ts`.

#### RN-057 — Intentos huérfanos bajo tutela humana
**Regla:** un intento cuyo cierre no pudo confirmarse queda `orphaned`: nunca se auto-marca como fallido, no se reinicia automáticamente ni se repite una autorización a ciegas; conserva toda su evidencia y exige recuperación humana. Reiniciar Shardeo tampoco convierte huérfanos en fallidos ni crea segundos supervisores.
**Confianza:** Alta. **Evidencia:** SPECS.md Spec 9 (persistencia, concurrencia y recuperación), máquina de estados de `src/runtime/terminal.ts`.

### Área M — Integridad del contexto gestionado

#### RN-058 — Bundle inmutable con admisión probada
**Regla:** en modos gestionados, el contexto se materializa como copias inmutables por intento con digests SHA-256 y orden contractual preservado. Antes de que el proveedor trabaje, el adapter debe probar que todas las entradas obligatorias quedaron disponibles con esos bytes exactos; cualquier diferencia se trata como drift o manipulación y falla cerrado. No se trunca contexto obligatorio: un exceso de límite falla antes de empezar.
**Confianza:** Alta. **Evidencia:** SPECS.md Spec 9 (bundle y manifiesto), `src/runtime/bundle.ts`.

#### RN-059 — Lectura preautorizada mínima
**Regla:** si el proveedor permite preautorizar lecturas, el alcance es únicamente la raíz exacta del bundle; nunca acceso amplio al proyecto o al sistema de archivos. Si no puede expresarse ese alcance seguro, se usa otro mecanismo nativo o se falla con `unsupported_capability`.
**Confianza:** Alta. **Evidencia:** SPECS.md Spec 9 (integridad y permisos del bundle).

#### RN-060 — Códigos de salida uniformes
**Regla:** todo comando termina con código 0 en éxito y 1 en error, de forma consistente para consumo programático del orquestador.
**Confianza:** Alta. **Evidencia:** `src/utils/errors.ts` (`ExitCode`).

## 6. Flujos de negocio principales

### Flujo F1 — Preparación del proyecto
1. **Condición inicial:** repositorio cualquiera sin `.shardeo/`.
2. **Actores:** usuario humano + Shardeo.
3. **Pasos:** el usuario instala el CLI globalmente y ejecuta la inicialización; Shardeo crea la estructura de configuración (workflows, skills, artifacts, docs) con valores por defecto válidos.
4. **Reglas:** idempotencia total: si la estructura existe, informa sin sobrescribir nada; sin permisos de escritura falla con mensaje claro.
5. **Resultado:** proyecto listo para definir workflows.
6. **Excepciones:** estructura preexistente (no destructivo); falta de permisos.
7. **Referencias:** RN-060. **Evidencia:** SPECS.md Spec 1.

### Flujo F2 — Descubrimiento
1. **Condición inicial:** workflows definidos en el proyecto.
2. **Actores:** orquestador o humano + Shardeo.
3. **Pasos:** listar workflows con nombre y descripción; describir uno para ver instrucciones generales y steps con tipo, dependencias, entradas y salidas.
4. **Reglas:** ambos comandos validan el YAML antes de responder; un workflow inválido aparece marcado como tal con su error concreto; no haber workflows es información, no error.
5. **Resultado:** el orquestador puede presentar opciones informadas al usuario.
6. **Excepciones:** workflow inexistente (error con lista de disponibles); YAML malformado (marcado, no oculto).
7. **Referencias:** RN-003. **Evidencia:** SPECS.md Spec 2.

### Flujo F3 — Ejecución de un workflow
1. **Condición inicial:** workflow válido.
2. **Actores:** orquestador (conductor), agente ejecutor o procesos deterministas, Shardeo.
3. **Pasos:** iniciar ejecución (identificador único, steps pendientes) → consultar pasos disponibles → ejecutar cada paso → completar los `agent` → repetir hasta terminar.
4. **Reglas:** RN-006 a RN-019 (grafo, orden, autocompletado de command, cierre manual de agent).
5. **Resultado:** ejecución `completed` cuando todos los steps alcanzan estado terminal positivo.
6. **Excepciones:** fallo de un step (`failed` contagia el agregado, RN-050); workflow modificado en medio de la ejecución (RN-051).

### Flujo F4 — Step agent headless con fallback
1. **Condición inicial:** step `agent` disponible en modo headless.
2. **Actores:** orquestador, Shardeo, runtime OpenCode (candidatos).
3. **Pasos:** revalidar workflow → validar entradas → sondear candidatos en orden → construir contexto acotado → invocar sin bypass de permisos → clasificar el resultado (éxito / fallback / terminal) → persistir intento con evidencia sanitizada → si hay fallback, repetir con contexto extendido.
4. **Reglas:** RN-020 a RN-026, RN-027 a RN-039.
5. **Resultado esperado:** éxito deja el step `running` a la espera del cierre explícito.
6. **Excepciones:** permiso solicitado (`permission_required`), inactividad (`process_inactivity_timeout`), presupuesto excedido (`artifact_context_too_large`), contexto inválido (`context_error`), contrato del harness violado (`adapter_contract_error`), agotamiento (`exhaustion`).
7. **Evidencia:** `src/commands/steps.ts::runAgentStep`, `src/utils/agent.ts`.

### Flujo F5 — Iteración dirigida (reabrir / omitir)
1. **Condición inicial:** resultado de un step posterior revela trabajo previo incorrecto, o un step pendiente resulta innecesario según las instrucciones del workflow.
2. **Actores:** orquestador decide; Shardeo valida y persiste; el usuario aprueba según las instrucciones del workflow.
3. **Pasos:** reabrir con feedback (con cascada si hay descendientes afectados) u omitir con motivo → reejecutar → recompletar.
4. **Reglas:** RN-044 a RN-049.
5. **Resultado:** nueva generación vigente del trabajo corregido; descendientes coherentemente reiniciados; historial íntegro para auditoría.
6. **Excepciones:** cascada requerida no provista; step ocupado; estados no admitidos.
7. **Nota de negocio:** Shardeo nunca decide cuándo iterar; solo materializa la decisión del orquestador (SPECS.md Spec 8).

### Flujo F6 — Reanudación tras interrupción
1. **Condición inicial:** ejecución detenida por error, cierre de terminal o cancelación.
2. **Actores:** orquestador/humano + Shardeo.
3. **Pasos:** reanudar → conciliar intentos interrumpidos → verificar artefactos de completados → marcar reconstrucciones → continuar con los pasos pendientes entregando respuestas previas como contexto.
4. **Reglas:** RN-050 a RN-052, RN-029.
5. **Resultado:** continuidad sin repetir trabajo completado ni perder conocimiento acumulado.
6. **Excepciones:** workflow cambiado (bloqueo); artefactos perdidos (ruta de reconstrucción).

### Flujo F7 — Permiso supervisado
1. **Condición inicial:** step agent configurado (o forzado por intento) en modo `supervised`.
2. **Actores:** orquestador, supervisor local, harness, política de permisos.
3. **Pasos:** inicio con contexto congelado y admitido → retorno inmediato → el harness pide permiso → la solicitud se publica como interacción con decisiones válidas → el orquestador consulta eventos y emite una decisión anunciada → resolución idempotente aplicada por protocolo nativo.
4. **Reglas:** RN-032, RN-035, RN-036, RN-054, RN-055, RN-058.
5. **Resultado:** permiso mediado sin exponer contratos internos del proveedor ni ampliar autoridad.
6. **Excepciones:** timeout de decisión (`interaction_timeout`); caída durante la resolución (huérfano, RN-057); capacidad ausente (`unsupported_capability`).

### Flujo F8 — Handoff humano en terminal
1. **Condición inicial:** step agent en modo `terminal`.
2. **Actores:** orquestador (solo comunica), persona (controla), supervisor.
3. **Pasos:** inicio con PTY adjuntable → retorno con el comando exacto de conexión → la persona conecta en otra terminal y trabaja de forma nativa → al terminar el harness, el supervisor persiste el resultado.
4. **Reglas:** RN-032, RN-054, RN-056, RN-058.
5. **Resultado:** trabajo interactivo real sin automatización frágil de interfaces.
6. **Excepciones:** expiración sin presencia humana; reconexiones múltiples permitidas; identidad ambigua de intento (fallo explícito).

## 7. Estados y transiciones

### Step (dentro de una ejecución)

| Estado | Significado funcional |
|---|---|
| `pending` | Esperando que sus predecessores alcancen estado terminal positivo. |
| `running` | Tiene al menos un intento vivo o finalizado pendiente de cierre; un agente exitoso deja aquí al step. |
| `completed` | Trabajo aceptado con generación vigente validada. |
| `failed` | Último intento terminó en motivo terminal o el comando falló. |
| `skipped` | Omitido por decisión del orquestador; terminal y sin producción. |

Transiciones permitidas:

- `pending → running` (ejecución reclamada con lease).
- `running → completed` (solo `step complete` para agents, validando produces; automático para command exitoso).
- `running → failed` (motivo terminal, fallo de comando o agotamiento).
- `failed → running` (re-ejecución directa permitida).
- `completed → pending` y `failed → pending` (reapertura con feedback; invalida generación).
- `pending → skipped` (omisión solo de trabajo virgen).

Transiciones prohibidas: ejecutar steps `skipped` o agents `completed`; completar steps no running; omitir steps con intentos o en cualquier estado distinto de pendiente; reabrir `pending`, `running` o `skipped`. Los estados terminales del step solo se abandonan mediante las transiciones explícitas de reapertura/omisión descritas.

### Ejecución (agregado)

`running → completed` cuando todos sus steps están `completed`/`skipped`; `running → failed` si alguno falló. La reapertura puede devolver una ejecución cerrada a `running` (el agregado se recalcula transaccionalmente). Prohibido: divergencia entre lo mostrado y lo almacenado.

### Intento gestionado (`supervised` / `terminal`)

Estados: `starting`, `awaiting_human`, `running`, y terminales `completed`, `failed`, `orphaned`.

Transiciones válidas: `starting → awaiting_human|failed|orphaned`; `awaiting_human → running|completed|failed|orphaned`; `running → awaiting_human|completed|failed|orphaned`. Los tres estados terminales no tienen salida (un huérfano jamás vuelve a la vida por sí solo). **Evidencia:** máquina de estados en `src/runtime/terminal.ts`.

### Interacción

`pending → resolving → resolved | resolution_failed`, mediante comparación-y-asignación idempotente; una decisión contraria tras resolver falla. **Evidencia:** SPECS.md Spec 9.

## 8. Permisos y restricciones

| Actor | Acción | Condición |
|---|---|---|
| Orquestador | Iniciar, consultar y ejecutar steps | Workflow válido; predecessores terminalizados; step no omitido ni (si es agent) completado |
| Orquestador | Completar un agent | Step `running`; produce completo y seguro |
| Orquestador | Reabrir / omitir | Estados admitidos (RN-044, RN-046); feedback/motivo obligatorios; cascada si corresponde |
| Orquestador | Aprobar interacciones supervisadas | Solo decisiones anunciadas por esa interacción |
| Política automática | Autorizar/rechazar permisos | Coincidencia exacta de capacidad + decisión anunciada; `deny` precede; actor registrado como `policy` |
| Persona | Controlar terminal en modo `terminal` | Adjuntarse con identidad exacta del intento vivo |
| Agente ejecutor | Escribir código en el repositorio | Libre, guiado por instrucciones; Shardeo no trackea el código |
| Agente ejecutor | Dejar artefactos | Solo bajo `.shardeo/artifacts/` con contención estricta |
| Shardeo | Decidir metodología (ramas, condicionales, calidad) | Nunca: esa autoridad pertenece al orquestador y a las instrucciones del workflow |
| Cualquier proceso | Bypass de permisos | Prohibido por diseño: sin banderas de omisión ni shell interactivo |

## 9. Cálculos y decisiones de negocio

**Límites numéricos operativos** (constantes verificadas en `src/utils/agent.ts`):

| Límite | Valor | Efecto al exceder |
|---|---|---|
| Salida visible/proyecciones | 16 KiB por intento | Truncado sanitizado, nunca error |
| Diagnóstico crudo (`DiagnosticRaw`) | 1 MiB exacto; encima prefijo+sufijo+tamaño+SHA-256 | Conservación acotada con digest |
| Snapshot acumulado por step | 1 MiB de contenido nuevo/modificado | Falla el intento (`artifact_context_too_large`) |
| Contexto de fallback | 2 MiB acumulado por ejecución de step | Falla cerrada |
| Instrucciones (por archivo y total) | 256 KiB | Contexto inválido (`context_error`) antes de invocar |
| Skills | 256 KiB/archivo, 1 MiB acumulado | Igual |
| Artefactos requeridos | 1 MiB/archivo y acumulado | Igual |
| Artefactos previos acumulados | 2 MiB | Igual |
| Inactividad de agente | 300 000 ms por defecto (configurable) | `process_inactivity_timeout` |
| Ejecución de comando | 300 s fijos | Fallo del step |
| Sondeo de candidato | 5 s | Candidato descartado |
| Lease de intento / latido | 30 s / cada 10 s | Conciliación de interrumpidos |

**Decisiones derivadas:**

- *Clasificación de resultado de un intento:* éxito estricto → `success`; motivo en lista terminal → `terminal`; cualquier otro error limpio → `fallback` (RN-022 a RN-025).
- *Disponibilidad:* `depends_on ⊆ {completed ∪ skipped}` y propio estado `pending` (RN-011).
- *Agregado de ejecución:* `failed > completed = todos terminalizados positivos > running` en ese orden de precedencia (RN-050).
- *Resolución de modo:* primera capa que declare gana; default `headless` (RN-053).
- *Orden determinista:* los descendientes afectados por una cascada se procesan en orden binario UTF-8 estable, garantizando auditoría reproducible.
- *Prioridad de seguridad ante incertidumbre:* ante ambigüedad (permiso no reconocible, capacidad desconocida, decisión no anunciada, drift de bundle), el sistema falla cerrado en lugar de adivinar.

## 10. Casos especiales y excepciones

- **Comando vacío:** un step `command` sin texto de comando completa trivialmente con código 0 (útil como marcadores o puntos de sincronización en el grafo). Evidencia: `runCommandStep`.
- **Candidatos inexistentes vs. fallidos:** los candidatos no instalados no generan intento ni cuentan como fallo del proveedor; solo se registra evidencia sanitizada de la sonda. Evidencia: RN-020/RN-021.
- **Auto-rechazo del harness:** cuando OpenCode rechaza por sí mismo un permiso en modo no interactivo, la señal versionada se reconoce y se trata como evidencia de permiso denegado, no como error genérico. Evidencia: commit H-LIVE-03, `src/adapters/opencode.ts`.
- **Contenido binario en artefactos previos:** si un artefacto de un intento anterior no es texto UTF-8 válido, se entrega al siguiente intento como marcador con tamaño y digest en lugar del contenido. Evidencia: `decodePriorArtifactText`.
- **Workflows inválidos visibles:** en el listado no se ocultan: aparecen marcados como inválidos con su error, para que el autor pueda corregirlos. Evidencia: SPECS.md Spec 2.
- **Error sin punto de entrada con ciclo latente:** el mensaje de falta de punto de entrada añade, cuando aplica, la ruta del ciclo detectada, ayudando a diagnosticar grafos roto. Evidencia: `analyzeWorkflowGraph`.
- **Desconexión en terminal no destructiva:** separarse de la PTY mantiene vivo el proceso hijo; la persona puede re-conectarse mientras el intento siga dentro de su plazo de presencia humana. Evidencia: RN-056.
- **Reinicio del sistema no altera huérfanos:** arrancar de nuevo la herramienta no convierte huérfanos en fallidos ni duplica supervisores; la recuperación es siempre deliberada. Evidencia: RN-057.

## 11. Integraciones relevantes para el negocio

- **OpenCode (runtime de ejecución v1).** Rol de negocio: brazo executor de los steps `agent`. Su contrato de salida estructurada (JSONL) define qué significa "invocación limpia"; sus señales exactas y versionadas habilitan la detección de solicitudes de permiso en headless; en modos gestionados aporta servidor de sesión, eventos y resolución nativa de aprobaciones, y terminal real adjuntable. El modelo concreto que use por debajo es irrelevante para Shardeo (intercambiabilidad).
- **Proveedores de modelos (detrás de OpenCode).** Rol de negocio: capacidad variable (cuota, disponibilidad) que motiva la lista ordenada de candidatos y el fallback sin clasificación semántica: el sistema asume que cualquier proveedor puede fallar en cualquier momento.
- **Almacén embebido local (SQLite).** Rol de negocio: memoria institucional del proyecto — trazabilidad, resiliencia (reanudación, conciliación) y observabilidad. No es un bus de señalización en tiempo real; ese papel lo cumple la comunicación local explícita entre procesos.

## 12. Ambigüedades y reglas no confirmadas

1. **Estado documental rezagado respecto del código.** SPECS.md marca las Specs 5, 6 y 8 como `pending`, pero el comportamiento está implementado y probado (validación de grafo en cada carga; re-ejecución con contexto acumulado; reopen/skip con generaciones). Posible interpretación: la documentación de estados no se actualizó. Evidencia: comandos y pruebas existentes (`test/spec8-iteration-control.test.mjs`, `test/utils/dag-execution.test.mjs`). Incertidumbre: cuál es la fuente de verdad pretendida para futuras decisiones.
2. **Spec 10 (override de modelo por flags) es borrador.** Documentada pero no implementada: los flags propuestos no existen en el comando de ejecución actual. No debe asumirse disponible. Evidencia: firma de opciones de `cmdStepRun` frente a SPECS.md Spec 10 (`status: draft`).
3. **Symlinks en artefactos producidos.** El contrato escrito permite symlinks cuyo destino permanece dentro de la raíz; la captura de generación implementada rechaza symlinks como artefacto producido (exige archivo regular). Posible endurecimiento posterior deliberado. Evidencia: `inspectProducedArtifacts` frente a SPECS.md Spec 4. Incertidumbre: cuál criterio prevalecerá.
4. **Prototipo heredado divergente.** Existe un prototipo Python antiguo en la raíz (`shardeo.py`) con semánticas distintas (mapa fijo de CLIs, disponibilidad basada en productores completados en lugar del grafo, timeout fijo). No representa el comportamiento actual; el sistema vigente es el TypeScript. Evidencia: comparación directa de ambos códigos.
5. **Persistencia del código `permission_timeout`.** Sigue en la lista de motivos terminales aunque la inactividad migró a `process_inactivity_timeout`; probablemente se conserva por compatibilidad con rutas gestionadas antiguas. Incertidumbre: si alguna ruta activa aún lo emite.
6. **Lectura del modo a nivel de workflow.** En el comando de ejecución, el modo declarado a nivel workflow se recupera releyendo el YAML crudo con comentarios de "mejor esfuerzo" en el código; el cableado parece parcial. Evidencia: bloque correspondiente de `cmdStepRun`. Incertidumbre: si la herencia `workflow.mode` está plenamente garantizada en todos los caminos.
7. **Flag `--human` aceptado pero sin efecto** en varios comandos (parámetro ignorado). Posible vestigio de una superficie de formato humano planificada. Evidencia: firmas `_opts: { human?: boolean }`.

## 13. Resumen de reglas

| ID | Regla | Área | Confianza |
|---|---|---|---|
| RN-001 | Solo `opencode` como runtime en v1; otro identificador invalida el workflow | Workflows | Alta |
| RN-002 | Modelo opcional y opaco, transferido sin interpretar | Workflows | Alta |
| RN-003 | Revalidación íntegra del YAML en cada operación | Workflows | Alta |
| RN-004 | Campos desconocidos tolerados; campo `after` prohibido | Workflows | Alta |
| RN-005 | Nombre y al menos un step obligatorios | Workflows | Alta |
| RN-006 | IDs de steps únicos | Grafo | Alta |
| RN-007 | depends_on solo referencia steps existentes | Grafo | Alta |
| RN-008 | Punto de entrada obligatorio (step sin dependencias) | Grafo | Alta |
| RN-009 | Grafo acíclico con diagnóstico de ruta | Grafo | Alta |
| RN-010 | depends_on único criterio de orden | Grafo | Alta |
| RN-011 | Disponible = predecessores completed/skipped | Grafo | Alta |
| RN-012 | Bloqueo explícito por predecessores incompletos | Ejecución | Alta |
| RN-013 | Paralelismo decidido por el orquestador | Grafo | Alta |
| RN-014 | Creación atómica de ejecución; inválido no crea nada | Ejecución | Alta |
| RN-015 | skipped/completed(agent) impiden re-ejecutar | Ejecución | Alta |
| RN-016 | Command autocompleta con exit 0 + produces válidos | Ejecución | Alta |
| RN-017 | Agent nunca autocompleta; cierre explícito del orquestador | Ejecución | Alta |
| RN-018 | Complete exige running, intento con snapshot y produces íntegro | Ejecución | Alta |
| RN-019 | requires valida al ejecutar; obsoletos detectados por generación | Artefactos | Alta |
| RN-020 | Candidatos desde agents[] en orden; sondeo perezoso sin intento | Fallback | Alta |
| RN-021 | Sin candidatos: fallo con sondas evaluadas y cero intentos | Fallback | Alta |
| RN-022 | Éxito estricto (JSONL válido, sin error, exit 0) deja running | Fallback | Alta |
| RN-023 | Error limpio hace fallback sin clasificación semántica | Fallback | Alta |
| RN-024 | Lista cerrada de motivos terminales sin fallback | Fallback | Alta |
| RN-025 | Agotamiento de candidatos cierra con exhaustion | Fallback | Alta |
| RN-026 | Cada invocación persiste como intento independiente | Persistencia | Alta |
| RN-027 | Orden contractual del contexto inyectado | Contexto | Alta |
| RN-028 | Presupuestos de tamaño por componente; exceso falla cerrado | Contexto | Alta |
| RN-029 | Re-ejecuciones ilimitadas con artefactos previos y feedback delimitado | Contexto | Alta |
| RN-030 | Inactividad 5 min reiniciable termina con process_inactivity_timeout | Tiempos | Alta |
| RN-031 | Techo fijo de 300 s para comandos | Tiempos | Alta |
| RN-032 | Tres temporizadores independientes; cierre dudoso = orphaned | Tiempos | Alta |
| RN-033 | Detección de permisos solo por señales exactas; ambigüedad falla cerrada | Permisos | Alta |
| RN-034 | Sin bypass de permisos ni shell interactivo | Permisos | Alta |
| RN-035 | Política por capas; solo decisiones anunciadas; deny precede | Permisos | Alta |
| RN-036 | Resolución única idempotente; conflicto explícito | Permisos | Alta |
| RN-037 | Visible ≤16 KiB sanitizado; crudo solo en depósito autorizado ≤1 MiB | Evidencia | Alta |
| RN-038 | Fallback context ≤2 MiB sanitizado, sin evidencia cruda | Evidencia | Alta |
| RN-039 | Snapshots SHA-256 con presupuesto de 1 MiB por step | Evidencia | Alta |
| RN-040 | Artefactos bajo raíz única con contención estricta | Artefactos | Alta |
| RN-041 | Vigencia generacional; invalidado = obsoleto aunque exista | Artefactos | Alta |
| RN-042 | Re-ejecución sobrescribe archivos; historial permanece | Artefactos | Alta |
| RN-043 | Producido debe ser archivo regular contenido | Artefactos | Media |
| RN-044 | Skip solo trabajo virgen pendiente; terminal | Iteración | Alta |
| RN-045 | skipped libera orden pero no satisface requires | Iteración | Alta |
| RN-046 | Reopen solo completed/failed con feedback obligatorio | Iteración | Alta |
| RN-047 | Cascada obligatoria ante descendientes afectados | Iteración | Alta |
| RN-048 | Reapertura invalida generación; validez nueva tras nuevo éxito | Iteración | Alta |
| RN-049 | Lease vivo bloquea; expirado se concilia; auditoría de transiciones | Iteración | Alta |
| RN-050 | Agregado derivado transaccionalmente (failed domina) | Estado | Alta |
| RN-051 | Resume exige workflow sin cambios; no repite completados | Reanudación | Alta |
| RN-052 | Artefacto perdido marca reconstrucción con respuesta previa | Reanudación | Alta |
| RN-053 | Modo por capas con default headless; override solo al intento | Modos | Alta |
| RN-054 | Capacidades sondeadas; unsupported_capability sin fallback silencioso | Modos | Alta |
| RN-055 | Supervised no bloqueante; eventos con cursores monotónicos | Modos | Alta |
| RN-056 | Terminal controlada solo por humanos; attach exacto; detach no mata | Modos | Alta |
| RN-057 | Huérfanos: nunca auto-fallidos ni re-autorización a ciegas | Modos | Alta |
| RN-058 | Bundle inmutable con admisión probada; drift falla cerrado | Integridad | Alta |
| RN-059 | Lectura preautorizada limitada al bundle exacto | Integridad | Alta |
| RN-060 | Códigos de salida uniformes 0/1 | Convención | Alta |






