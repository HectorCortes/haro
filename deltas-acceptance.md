# Shardeo v2 — Criterios de aceptación de deltas

> Propósito: criterios de aceptación **verificables** para los deltas entre Shardeo v1 (Specs 1–9 entregadas en `main`) y la propuesta normativa v2 del proyecto. Este documento es **autocontenido**: no depende de documentos externos ni de sus secciones; cada criterio enuncia por completo el comportamiento esperado.
>
> **Decisión de stack (fijada): se continúa con TypeScript** sobre el codebase actual — no hay reescritura. Los contratos propuestos en la referencia de diseño (interfaces Go, JSON Schema) son conceptuales: se re-expresan en TypeScript con tipos estrictos y **ACP — Agent Client Protocol (estándar de Zed) — como estándar del protocolo de adapters** (ver Convenciones).
>
> Cada criterio es testeable a nivel de funcionamiento (comando real o integración) o a nivel unitario. Un criterio está **cumplido** solo cuando su verificación descrita pasa de forma reproducible en el flujo gentle-ai (ver §Uso con gentle-ai).

---

## Convenciones

- **Jerarquía de IDs — dos niveles**:

  ```
  id-spec > id-local
  ```

  - `id-spec` identifica la **spec** (un delta completo; coincide con el nombre del change SDD en gentle-ai). Ejemplo: `v2-broker`.
  - `id-local` identifica el criterio **dentro** de esa spec: `F-<n>` para criterios **funcionales** (E2E o integración) y `U-<n>` para **tests unitarios**. La numeración reinicia en cada spec.
  - Referencia completa: `v2-broker/F-01` (spec `v2-broker`, criterio funcional 1).

- **Tipos de verificación**: `[E2E]` comando real de punta a punta · `[INT]` integración con fixtures/subprocesos controlados · `[UNIT]` test unitario · `[REV]` revisión de documento · `[PROC]` criterio de proceso.
- **Prioridad**: `P0` bloqueante (sin esto el delta no existe) · `P1` importante · `P2` deseable.
- **Stack decidido — TypeScript** (evolución del codebase actual, sin reescritura): Node ≥ 20, ESM, `strict`. Runner de tests: `node --test` (suite explícita en `npm test`, ~550 casos). Typecheck: `tsc --noEmit`.
- **ACP como estándar del protocolo Broker↔Adapter (requisito explícito)**: el protocolo entre broker y adapters/harnesses se basa en **Agent Client Protocol** (estándar abierto impulsado por Zed) — JSON-RPC 2.0 sobre stdio, negociación de capacidades en `initialize`, y los métodos `initialize`, `session/new`, `session/prompt`, `session/update`, `session/cancel` y `session/request_permission` (`terminal/*` diferido). Los adapters traducen el protocolo nativo de cada harness a este contrato; el adapter genérico `acp-generic` lo implementa directamente cuando el harness ya habla ACP (ver `v2-adapter/U-03`).
- **Validación runtime — práctica vigente del repo**: la validación de schemas (workflows, config, payloads internos) sigue usando **zod** (en minúscula, la librería), ya establecido en v1 (`src/schema/*`); no se introduce JSON Schema ni validadores ad-hoc. Esto es práctica del repo, independiente del estándar ACP.
- **Mantenibilidad**: el código resultante debe conservar las reglas de la propuesta v2 adaptadas a TS: core sin literales de proveedor, fail-closed, presupuestos de evidencia, migraciones idempotentes y protocolo de adapters basado en ACP.
- **Estado**: cada criterio lleva checkbox de tracking; inician en `pendiente`.

### Specs (deltas)

| id-spec | Delta | Contenido |
|---|---|---|
| `v2-reconciliacion` | D00 | Reconciliación de la documentación normativa |
| `v2-no-regresion` | R00 | Preservación de Specs 1–9 |
| `v2-broker` | D01 | Broker persistente por proyecto |
| `v2-ipc` | D02 | IPC JSON-RPC CLI↔Broker y modelo de eventos |
| `v2-adapter` | D03 | Contrato de adapter v2 y multi-harness |
| `v2-path-claims` | D04 | path_claims y aislamiento de workspace |
| `v2-composicion` | D05 | Composición de workflows |
| `v2-reporte` | D06 | Reporte de cambios |
| `v2-store` | D07 | Persistencia tras interfaz de repositorio |
| `v2-distribucion` | D08 | Distribución |
| `v2-flujo-gentle-ai` | D09 | Flujo de desarrollo con gentle-ai |

---

## Uso con gentle-ai

El flujo de desarrollo ya **no** usa el pipeline de iniciativas (`.docs/initiatives/`, `tools/scripts/initiative/`). Cada spec de este archivo se implementa como un **change SDD** (o el equivalente gentle-ai) con el ciclo proposal → spec → design → tasks → apply → verify → archive. El nombre del change es el `id-spec`.

- Los criterios de este archivo son la **fuente de los criterios de aceptación** de cada spec del change.
- `sdd-verify` (o la verificación equivalente) ejecuta las verificaciones aquí descritas con `node --test` (`npm test`); los `[E2E]` son frontera pública, los `[UNIT]`/`[INT]` son tests del paquete/módulo.
- Un delta está completo solo cuando pasan todos sus criterios P0 y P1, y el change queda archivado.
- No se crean nuevas iniciativas; las existentes (p. ej. 008) se migran o cierran explícitamente.

---

# Spec: v2-reconciliacion — Reconciliación de la documentación normativa

La documentación normativa v2 exige actualizarse explícitamente ante contradicciones con la realidad. Antes de implementar cualquier otra spec, debe reconciliarse con el estado actual del repo. Criterios sobre ese documento, verificables por revisión del diff.

### F-01 — La superficie terminal/PTY deja de listarse como trabajo diferido [REV] · P0 · [x]
**Criterio**: La documentación normativa v2 reconoce que la superficie `terminal`/PTY **ya está implementada** (PTY real, attach humano, presencia) y la trata como superficie existente a preservar, no como trabajo diferido.
**Verificación**: Revisión del documento: la superficie `terminal`/PTY ya no figura en la lista de trabajo diferido.

### F-02 — El bundle inmutable con manifiesto deja de listarse como trabajo diferido [REV] · P0 · [x]
**Criterio**: La documentación normativa v2 reconoce que el bundle inmutable con manifiesto SHA-256 y prueba de admisión **ya está implementado** y lo remite a no-regresión (`v2-no-regresion`), no a trabajo diferido.
**Verificación**: Revisión del documento: el bundle con manifiesto SHA-256 ya no figura en la lista de trabajo diferido.

### F-03 — El transporte Broker↔Adapter no se dicta desde el core [REV] · P0 · [x]
**Criterio**: La documentación normativa v2 no dicta el transporte Broker↔Adapter: lo elige el adapter según lo que su harness soporta nativamente (stdio-RPC, HTTP local + SSE, JSONL), se declara en capacidades y se negocia en `initialize`. HTTP local + SSE **no es "fallback"** sino vía nativa válida cuando el harness la expone (caso real: OpenCode supervised usa servidor HTTP/SSE gestionado).
**Verificación**: Revisión del documento: ya no califica HTTP+SSE como mecanismo de fallback "nunca general".

### F-04 — Deuda conocida del parser JSONL registrada [REV] · P1 · [x]
**Criterio**: La documentación normativa v2 registra la excepción confinada vigente: el parser JSONL de OpenCode vive en `src/utils/agent.ts` (ruta headless) y su traslado al adapter es deuda pendiente que la spec `v2-adapter` debe cerrar.
**Verificación**: Revisión del documento: existe una nota de deuda conocida con la referencia exacta.

### U-01 — Diff auditable de la actualización normativa [REV] · P1 · [x]
**Criterio**: Existe un diff (o changelog) de la actualización que muestra exactamente qué reglas cambiaron y por qué — nunca una edición silenciosa.
**Verificación**: Revisión del diff de la actualización: solo los cambios descritos en F-01..F-04 de esta spec, cada uno con su justificación.

---

# Spec: v2-no-regresion — Preservación de Specs 1–9

Sin reescritura, estos criterios son la salvaguarda de que las specs v2 no rompen el comportamiento probado: la suite existente (~550 casos) debe seguir verde en cada cambio. Verificaciones `[E2E]`/`[INT]` contra el sistema; `[UNIT]` contra la suite vigente.

### F-01 — Init idempotente (Spec 1) [E2E] · P0 · [ ]
**Criterio**: `shardeo init` crea `.shardeo/` completo en un proyecto limpio; ejecutado dos veces no modifica nada existente y no falla.
**Verificación**: init en directorio temporal vacío → estructura esperada; init de nuevo → mismo estado, salida informativa, exit 0.

### F-02 — Descubrimiento y validación de workflows (Spec 2) [E2E] · P0 · [ ]
**Criterio**: `workflows list` y `workflows describe` devuelven salida JSON estructurada; un workflow con YAML inválido aparece marcado inválido en `list` y `describe` reporta el error de validación concreto.
**Verificación**: fixture de workflow válido e inválido en `.shardeo/workflows/` → salidas esperadas.

### F-03 — Ciclo command completo (Spec 3) [E2E] · P0 · [ ]
**Criterio**: `run` + `steps next` + `step run` + `status`: exit 0 con `produces` → `completed` automático; exit ≠ 0 → `failed` con stdout/stderr; exit 0 sin `produces` → `failed` listando los artefactos faltantes; `depends_on` insatisfecho → error explícito.
**Verificación**: workflow command de prueba que cubre los cuatro casos.

### F-04 — Ciclo agent headless con fallback (Spec 4) [E2E] · P0 · [ ]
**Criterio**: `step run` agent headless sigue el ciclo probe → lease → invoke → finalize; ante error limpio hace fallback al siguiente candidato **sin clasificación semántica**; ante error terminal (`permission_required`, `process_start_failed`, etc.) no hay fallback y el step queda `failed`.
**Verificación**: adapters de test controlados que producen éxito, error limpio y error terminal.

### F-05 — Re-ejecución con feedback (Spec 6) [INT] · P0 · [ ]
**Criterio**: `step run --feedback` entrega el feedback delimitado junto con la respuesta parcial previa; el intento se registra como reconstrucción con su historial.
**Verificación**: fixture con dos intentos: el segundo recibe el feedback textual íntegro y el contexto previo acotado.

### F-06 — Resume y reconciliación (Spec 7) [INT] · P0 · [ ]
**Criterio**: `resume` reconcilia intentos expirados, detecta artefactos faltantes (`reconstruction_required`), entrega la respuesta previa como contexto para reconstruir y devuelve guía de continuación.
**Verificación**: ejecución detenida a mitad → `resume` → estados y guía esperados.

### F-07 — Reopen/skip con generaciones (Spec 8) [INT] · P0 · [ ]
**Criterio**: `step reopen --cascade` invalida la generación vigente (los archivos quedan pero dejan de satisfacer `requires`/`complete`) y reinicia descendientes conservando historial; `step skip --reason` solo aplica a pendientes sin intentos; todo queda en `step_transition_events`.
**Verificación**: escenario de 3 steps encadenados; reopen del primero → cascada y auditoría verificadas.

### F-08 — Bundle inmutable y admisión (Spec 9a) [INT] · P0 · [ ]
**Criterio**: cada intento materializa un bundle con manifiesto (orden semántico, roles, bytes + SHA-256); la prueba de admisión es obligatoria antes de trabajo del harness; drift del contexto → fallo cerrado.
**Verificación**: fixture donde el contexto cambia entre congelación y admisión → intento falla con `adapter_contract_error`/drift.

### F-09 — Supervised completo (Spec 9b) [E2E] · P0 · [ ]
**Criterio**: supervisor con lease + fencing, IPC UDS, eventos con cursor persistidos antes de visibles, interacción CAS idempotente, `step approve` solo con decisiones de `available_decisions`, política de permisos fail-closed (desconocido → rechazo).
**Verificación**: harness simulado que solicita permiso → `step events` → `step approve` → resolución única; reenvío de la misma resolución no duplica efectos.

### F-10 — Terminal y attach humano (Spec 9c) [E2E] · P0 · [ ]
**Criterio**: `step run --mode terminal` crea PTY adjuntable y retorna `awaiting_human` + comando de attach; detach no mata al hijo; reattach ilimitado dentro de `human_presence_seconds`; el orquestador nunca escribe teclas ni interpreta la pantalla.
**Verificación**: PTY real con harness de prueba; attach/detach/reattach y cierre.

### F-11 — Detección adapter-native de permisos [INT] · P1 · [ ]
**Criterio**: la detección de solicitudes de permiso funciona con las fixtures versionadas (OpenCode 1.17.18) y el puente live (1.18.x); evidencia ambigua o desconocida → fallo cerrado, nunca adivinanza.
**Verificación**: suite de fixtures existente portada sin cambios de comportamiento.

### F-12 — Presupuestos de evidencia y sanitización [INT] · P0 · [ ]
**Criterio**: proyecciones visibles/evidencia ≤ 16 KiB; `DiagnosticRaw` ≤ 1 MiB (autoridad de bytes crudos, con prefijo+sufijo+SHA-256 si excede); snapshots ≤ 1 MiB; contexto de fallback ≤ 2 MiB; redacción de credenciales por patrones en toda salida proyectada.
**Verificación**: fixtures con outputs gigantes y con credenciales (Bearer, Basic, tokens) → proyecciones acotadas y redactadas.

### F-13 — Contención de paths [INT] · P0 · [ ]
**Criterio**: rechazo de rutas absolutas, `..` y escapes por symlink en workflows, instrucciones, skills, artefactos y bundles; symlinks internos permitidos solo si su destino real queda dentro.
**Verificación**: tabla de paths maliciosos contra `validateContainedPath` equivalente.

### F-14 — Contrato de salida CLI [E2E] · P0 · [ ]
**Criterio**: toda salida de comando es JSON estructurado; errores `{error, code}` con exit 1; salida EPIPE-safe (exit 0 si el consumidor cierra el pipe); argumentos posicionales extra rechazados respetando `--`.
**Verificación**: script que consume con pipe cerrado y pasa argumentos extra.

### U-01 — Suite vigente en verde [UNIT] · P0 · [ ]
**Criterio**: la suite existente (~550 casos, 74 suites) pasa completa en `npm test` y `npm run typecheck` queda limpio; ambos siguen en verde tras cada spec v2.
**Verificación**: `npm test` y `npm run typecheck` en verde.

### U-02 — Fixtures neutrales reutilizadas [UNIT] · P0 · [ ]
**Criterio**: las fixtures JSONL de OpenCode v1.17.18 y el servidor simulado v1.18.16 se reutilizan tal cual (formato neutral al lenguaje).
**Verificación**: los mismos archivos de `test/fixtures/` referenciados por los tests del stack nuevo.

### U-03 — Máquina de estados con cobertura vigente [UNIT] · P0 · [ ]
**Criterio**: transiciones atómicas (claim, finalize, reopen, skip), idempotencia de resolución y fencing de leases mantienen su cobertura unitaria (hoy en `src/db/queries.ts`) y se extienden con los cambios v2.
**Verificación**: tabla de transiciones cubierta caso a caso, en verde.

---

# Spec: v2-broker — Broker persistente por proyecto

### F-01 — Broker único por proyecto [E2E] · P0 · [ ]
**Criterio**: existe un broker por proyecto identificado por la ruta canónica del proyecto; CLI↔Broker por socket Unix (o named pipe en Windows).
**Verificación**: `execution.start` desde el CLI contra un broker vivo responde por el socket; dos invocaciones desde el mismo repo usan el mismo broker.

### F-02 — Arranque perezoso [E2E] · P0 · [ ]
**Criterio**: si el socket no responde, el CLI arranca el broker y reintenta; el broker sobrevive a la invocación CLI que lo arrancó.
**Verificación**: matar el broker → primera invocación CLI lo relanza y completa su operación; el proceso broker sigue vivo tras terminar el CLI.

### F-03 — Sesiones concurrentes [E2E] · P0 · [ ]
**Criterio**: un broker mantiene múltiples sesiones activas: dos ejecuciones en paralelo (mismo o distinto workflow, mismo proyecto) progresan sin interferencia de estado.
**Verificación**: lanzar dos ejecuciones simultáneas; ambas completan y cada una persiste su propio estado sin pisarse.

### F-04 — Brokers independientes por proyecto [E2E] · P1 · [ ]
**Criterio**: distintos proyectos tienen brokers completamente independientes, sin coordinación entre sí.
**Verificación**: operaciones simultáneas en dos repos distintos no comparten socket ni estado; detener uno no afecta al otro.

### F-05 — No duplicación de broker [E2E] · P1 · [ ]
**Criterio**: una segunda invocación CLI con socket vivo encuentra el broker existente y no lanza otro.
**Verificación**: conteo de procesos broker antes/después de N invocaciones → 1.

### F-06 — Muerte del broker y fencing [E2E] · P0 · [ ]
**Criterio**: si el broker muere con sesiones activas, los leases vencen y ninguna escritura posterior a la expiración es válida aunque el proceso original siga vivo; `resume` detecta el estado y guía la recuperación.
**Verificación**: matar el broker a mitad de un intento → intentos expirados; reintento de escritura con fencing token viejo → rechazado.

### F-07 — Apagado limpio [E2E] · P1 · [ ]
**Criterio**: el broker se detiene limpiamente ante señal de terminación: libera leases y sockets; no deja sockets zombies que bloqueen el siguiente arranque.
**Verificación**: señal TERM → proceso sale, socket eliminado, arranque posterior inmediato funciona.

### U-01 — Derivación del socket path [UNIT] · P1 · [ ]
**Criterio**: el path del socket se deriva de la ruta canónica del proyecto (hash estable): mismo repo → mismo socket; repos distintos → sockets distintos; la ruta no excede límites de longitud del sistema.
**Verificación**: tabla de rutas canónicas (incl. symlinks, trailing slashes) → derivación estable y única.

### U-02 — Framing robusto [UNIT] · P1 · [ ]
**Criterio**: el framing del socket (JSON-RPC 2.0) rechaza mensajes malformados y oversized sin romper la conexión, y soporta conexiones concurrentes.
**Verificación**: tests de framing con mensajes inválidos, > límite, y N clientes simultáneos.

---

# Spec: v2-ipc — IPC JSON-RPC CLI↔Broker y modelo de eventos

### F-01 — execution.start [E2E] · P0 · [ ]
**Criterio**: `execution.start` valida el workflow (schema + grafo + ciclos), crea la ejecución y retorna `execution_id`.
**Verificación**: workflow válido → id; workflow con ciclo/step inexistente → error estructurado sin ejecución creada.

### F-02 — execution.status [E2E] · P1 · [ ]
**Criterio**: `execution.status` devuelve el estado agregado de la ejecución y el resumen de steps.
**Verificación**: ejecución en progreso → estados por step coherentes con las transiciones ocurridas.

### F-03 — step.run no bloqueante [E2E] · P0 · [ ]
**Criterio**: `step.run` para steps agent retorna `{attempt_id, cursor}` sin bloquear; el progreso se consume por `step.events`.
**Verificación**: step agent lento → `step.run` retorna inmediatamente; eventos posteriores aparecen vía `step.events`.

### F-04 — step.events con paginación estable [E2E] · P0 · [ ]
**Criterio**: `step.events {since_cursor}` devuelve los eventos posteriores y `next_cursor`; reintentar la misma consulta devuelve exactamente los mismos eventos (sin duplicados) y avanzar el cursor nunca pierde eventos.
**Verificación**: secuencia de eventos conocida (fixture) consumida con reintentos y saltos de cursor.

### F-05 — step.approve idempotente [E2E] · P0 · [ ]
**Criterio**: `step.approve` resuelve una interacción y es idempotente: reenviar la misma resolución no produce efectos duplicados.
**Verificación**: aprobar dos veces la misma interacción con la misma clave → un único efecto observable en el harness y en el store.

### F-06 — step.cancel [E2E] · P1 · [ ]
**Criterio**: `step.cancel` cancela el attempt en curso y el harness recibe la cancelación; el intento queda `cancelled`/`failed` según la semántica definida, sin escrituras posteriores válidas.
**Verificación**: attempt activo → cancel → harness notificado, estado persistido.

### F-07 — step.reopen con invalidación [E2E] · P0 · [ ]
**Criterio**: `step.reopen` invalida la generación vigente y devuelve la lista `{invalidated: []}` de steps afectados por la cascada de generaciones.
**Verificación**: cadena de 3 steps → reopen del primero → `invalidated` lista los descendientes reales.

### F-08 — step.reopen con feedback opcional [E2E] · P1 · [ ]
**Criterio**: `step.reopen` acepta feedback opcional para la re-ejecución (preservación del comportamiento v1 de `--feedback`).
**Verificación**: reopen con feedback → el próximo intento del step recibe el feedback íntegro.

### F-09 — Notificaciones al CLI [E2E] · P1 · [ ]
**Criterio**: el CLI recibe `step.status_changed` e `step.interaction_required` del broker mientras consume eventos.
**Verificación**: suscripción de prueba recibe las notificaciones en el orden esperado con su cursor.

### U-01 — Cursor monotónico y persistencia previa [UNIT] · P0 · [ ]
**Criterio**: el cursor es monotónico por sesión/attempt; el evento se persiste **antes** de hacerse visible (un consumidor nunca ve un hueco ni un evento no persistido).
**Verificación**: test de crash entre persistencia y publicación → sin eventos visibles no persistidos.

### U-02 — CAS de interacciones [UNIT] · P0 · [ ]
**Criterio**: la resolución de interacción es atómica: misma `idempotency_key` → mismo resultado sin re-ejecución; clave distinta sobre la misma interacción → rechazada; cruce de identidad (attempt equivocado) → rechazado.
**Verificación**: tabla de casos (pendiente/resuelta/duplicada/ajena).

### U-03 — Payload_ref sin output crudo [UNIT] · P1 · [ ]
**Criterio**: `attempt_events.payload_ref` referencia evidencia sanitizada gestionada fuera de la tabla; nunca contiene el output crudo completo.
**Verificación**: inspección del store tras un intento → los eventos solo contienen referencias/deltas, no blobs completos.

### U-04 — Payloads JSON-RPC validados en la frontera [UNIT] · P1 · [ ]
**Criterio**: todo payload entrante y saliente de los protocolos JSON-RPC (CLI↔Broker y Broker↔Adapter) se valida en la frontera contra su schema (zod, práctica vigente del repo); payload desconocido o malformado → error estructurado y fail-closed, sin efectos sobre el estado.
**Verificación**: tabla de payloads malformados/desconocidos → rechazo con código estable y mensaje con path del campo.

---

# Spec: v2-adapter — Contrato de adapter v2 y multi-harness

### F-01 — initialize único y previo [E2E] · P0 · [ ]
**Criterio**: `initialize` se ejecuta una sola vez por subproceso, antes de cualquier `session/*`; negocia `protocolVersion` y capacidades en ambas direcciones.
**Verificación**: harness de prueba registra el orden de llamadas → initialize exactamente una vez y primero.

### F-02 — Nunca invocar lo no anunciado [E2E] · P0 · [ ]
**Criterio**: el broker nunca invoca un método opcional que el adapter no declaró en `Initialize`.
**Verificación**: adapter sin `Terminal`/`LoadSession` → el broker no emite esos métodos; intento de uso desde el CLI → error `unsupported_capability`.

### F-03 — request_permission solo si fue negociado [E2E] · P0 · [ ]
**Criterio**: el harness solo llama a `session/request_permission` si el broker anunció `Permission: true`; si no, resuelve con su política por defecto o falla (fail-closed).
**Verificación**: broker sin permiso negociado + harness que lo necesita → el harness falla cerrado, nunca se salta la política.

### F-04 — Cierre de la deuda del parser JSONL [INT] · P0 · [ ]
**Criterio**: el parser del stream JSONL de OpenCode vive en el adapter OpenCode, no en el core; el core no contiene ningún literal de proveedor.
**Verificación**: script de verificación (grep/import graph) que falla si fuera de `adapters/` aparece `opencode`, endpoints, flags o nombres de eventos del proveedor.

### F-05 — Adapter Claude Code con contract tests [E2E] · P1 · [ ]
**Criterio**: existe un adapter Claude Code que pasa la suite de contract tests de frontera pública contra el binario real, o contra fixtures grabadas solo si el binario real no es viable en CI (caso justificado).
**Verificación**: la suite `step run → subproceso real → protocolo → salida JSON pública → store → settle` corre contra `claude` (o sus fixtures grabadas) y pasa.

### F-06 — Fallback entre harnesses sin clasificación semántica [E2E] · P0 · [ ]
**Criterio**: un step con `harness: [opencode, claudecode]` (o inverso) hace fallback al siguiente candidato ante error limpio, sin clasificar la causa; al agotar candidatos el step falla con evidencia sanitizada de cada candidato.
**Verificación**: primer candidato falla limpio → el segundo recibe el contexto acumulado acotado (≤ 2 MiB) y ejecuta; ambos fallan → step `failed` con evidencia de ambos.

### U-01 — Negociación de capacidades por tabla [UNIT] · P0 · [ ]
**Criterio**: cada método opcional solo se invoca si fue anunciado; las capacidades nuevas son aditivas sin incrementar la versión mayor del protocolo; la versión mayor solo cambia ante cambios incompatibles en los métodos obligatorios.
**Verificación**: tests de tabla con combinaciones de capacidades (vacías, parciales, completas, futuras aditivas).

### U-02 — Suite de contract tests de primera clase [UNIT] · P1 · [ ]
**Criterio**: la suite de contract tests de frontera pública existe como artefacto de primera clase del repositorio desde el inicio.
**Verificación**: el repositorio contiene la suite versionada y ejecutable, no un documento de intención.

### U-03 — Adapter ACP genérico (estándar de Zed) [INT] · P1 · [ ]
**Criterio**: el adapter "acp-generic" traduce el contrato interno a **ACP (Agent Client Protocol, estándar abierto impulsado por Zed)** y viceversa (initialize, session/new, session/prompt, session/update, session/cancel, session/request_permission) con fixtures del protocolo.
**Verificación**: harness ACP simulado (fixtures) → el adapter completa el ciclo completo y las traducciones de capacidades son biyectivas.

### U-04 — Attempt agnóstico de transporte [UNIT] · P0 · [ ]
**Criterio**: `attempts` no contiene campos de transporte; los detalles (native_session_id, protocol_version, extra) viven en la tabla de extensión por adapter.
**Verificación**: schema test: insertar un attempt sin transporte y con transporte → ambos válidos; los campos nativos nunca aparecen en `attempts`.

---

# Spec: v2-path-claims — path_claims y aislamiento de workspace

### F-01 — Worktree por ejecución por defecto [E2E] · P0 · [ ]
**Criterio**: el default del sistema es que cada ejecución de un workflow corre en su propio `git worktree`; el `workspace_root` efectivo es ese worktree.
**Verificación**: `git worktree list` muestra un worktree nuevo por ejecución; el trabajo del step ocurre dentro de él.

### F-02 — Configuración de aislamiento en 3 niveles [E2E] · P0 · [ ]
**Criterio**: `workspace.mode` se resuelve por herencia `sistema → workflow → step`, con override por step; `workspace.shared` en workflow desactiva el aislamiento para todos sus steps salvo override.
**Verificación**: tres fixtures (workflow shared, step shared con workflow isolated, step isolated con workflow shared) → workspace_root correcto en cada caso.

### F-03 — Reclamos por identidad lógica y prefijo [E2E] · P1 · [ ]
**Criterio**: los reclamos se hacen sobre la identidad lógica del path (relativa, canonicalizada, sin symlinks) y reclamar un directorio bloquea a sus hijos por comparación de prefijo.
**Verificación**: reclamar `src/` → un segundo reclamo de `src/foo.ts` reporta conflicto; `src/foobar/` no colisiona con `src/foo/` (borde de prefijo).

### F-04 — shared vs shared bloquea [E2E] · P0 · [ ]
**Criterio**: dos steps `shared` con el mismo path lógico: el segundo no arranca hasta que el primer reclamo se libera; la liberación ocurre al terminar el step (éxito o fallo), no al terminar la ejecución.
**Verificación**: step A lento con reclamo de `src/` → step B con `src/` queda bloqueado; A termina (éxito) → B arranca; caso fallo → B arranca igualmente.

### F-05 — isolated vs isolated permite [E2E] · P1 · [ ]
**Criterio**: dos steps `isolated` con el mismo path lógico corren sin bloqueo; las divergencias se descubren solo en la integración, fuera del alcance de Shardeo.
**Verificación**: dos steps isolated concurrentes con `src/` → ambos arrancan; no se registra ningún conflicto.

### F-06 — isolated vs shared gobernado por on_logical_conflict [E2E] · P0 · [ ]
**Criterio**: `isolated` vs `shared` sobre el mismo path: `on_logical_conflict: block` (default) impide el arranque; `allow` (opt-in explícito del workflow) lo permite.
**Verificación**: fixture block → segundo step bloqueado; fixture allow → arranca.

### F-07 — Recursos externos siempre shared [E2E] · P1 · [ ]
**Criterio**: los paths declarados como externos al repo en la configuración del proyecto se tratan siempre como `shared`.
**Verificación**: path externo (p. ej. caché) reclamado por dos steps → bloqueo compartido aunque ambos sean isolated.

### U-01 — Canonicalización del path lógico [UNIT] · P0 · [ ]
**Criterio**: la canonicalización (resuelta, sin symlinks, relativa al repo) maneja symlinks, `..`, absolutos y bordes de prefijo sin falsos positivos ni escapes.
**Verificación**: tabla de casos límite incluyendo `src/foo` vs `src/foobar`, symlink dentro/fuera del repo.

### U-02 — Acquire atómico [UNIT] · P0 · [ ]
**Criterio**: `Acquire` es atómico (INSERT ... ON CONFLICT): dos adquisiciones concurrentes incompatibles → una gana, la otra recibe `Acquired=false` con el dueño actual.
**Verificación**: test de concurrencia (workers / `Promise.all` sobre la misma base) con el mismo path y modos incompatibles.

### U-03 — Matriz de comportamiento completa [UNIT] · P1 · [ ]
**Criterio**: las cuatro filas de la matriz de comportamiento (isolated/isolated, shared/shared, isolated/shared block, isolated/shared allow) tienen tests de tabla; `Release` con owner incorrecto no libera el reclamo de otro.
**Verificación**: tests de tabla parametrizados por modo y policy; release con owner/step ajeno → rechazado.

---

# Spec: v2-composicion — Composición de workflows

### F-01 — DAG plano bajo un único execution_id [E2E] · P0 · [ ]
**Criterio**: un step `type: workflow` referencia otro YAML; la composición ocurre en planificación, el resultado es un único DAG plano bajo un único `execution_id`, sin ejecuciones hijas.
**Verificación**: workflow con un nodo workflow → `steps next` expone los steps internos aplanados; `status` no muestra ninguna ejecución hija.

### F-02 — Namespacing de steps internos [E2E] · P0 · [ ]
**Criterio**: los steps internos se identifican `<nodo>.<step_interno>`; incluir el mismo workflow dos veces con nodos distintos no colisiona.
**Verificación**: workflow con dos nodos al mismo archivo → ids `a.x`, `a.y`, `b.x`, `b.y` coexisten y ejecutan independientemente.

### F-03 — Contrato inputs/outputs explícito [E2E] · P0 · [ ]
**Criterio**: `inputs`/`outputs` solo son válidos en el archivo incluido; el padre cablea (`depends_on`, `requires`) solo contra ese contrato, nunca contra steps internos.
**Verificación**: padre que referencia un step interno del incluido (no declarado como input/output) → error de validación claro.

### F-04 — Bindings del nodo workflow [INT] · P1 · [ ]
**Criterio**: `bindings` conecta `requires`/`produces` del padre con los `inputs`/`outputs` del incluido y la resolución de artefactos respeta esos mapeos.
**Verificación**: fixture con bindings → los artefactos del padre alimentan los inputs correctos del incluido y viceversa.

### F-05 — Detección estática de ciclos [E2E] · P0 · [ ]
**Criterio**: un workflow no puede incluirse a sí mismo, directa o transitivamente; la detección es estática, sobre el grafo de referencias entre archivos, antes de aplanar.
**Verificación**: fixture con auto-inclusión directa y transitiva → `run` falla con error de ciclo, sin ejecución creada.

### F-06 — Cascada que ignora la frontera [E2E] · P0 · [ ]
**Criterio**: la cascada de invalidación opera sobre el grafo aplanado de generaciones: reabrir un step invalida solo los steps internos cuyos `produces` alimentan realmente lo reabierto, con granularidad fina, independientemente del contrato `outputs` declarado.
**Verificación**: nodo con dos steps internos donde solo uno alimenta lo reabierto → `step.reopen` invalida solo ese step interno, no el otro ni los de otros nodos.

### F-07 — Paralelización por el mismo DAG [E2E] · P1 · [ ]
**Criterio**: los nodos `workflow` sin `depends_on` entre sí son paralelizables por el mismo mecanismo que cualquier step.
**Verificación**: dos nodos workflow sin dependencias → ambos aparecen en `steps next` como disponibles.

### U-01 — Aplanado puro y reproducible [UNIT] · P0 · [ ]
**Criterio**: el aplanado es una función pura: mismos archivos YAML → mismo DAG plano; incluye namespacing, contrato y orden topológico estable.
**Verificación**: tests de igualdad estructural (hash del DAG) sobre fixtures de composición anidada, ejecutados dos veces.

### U-02 — Ciclos estáticos por tabla [UNIT] · P0 · [ ]
**Criterio**: la detección de ciclos cubre directo, transitivo y auto-inclusión con distinto nombre de nodo vs archivo.
**Verificación**: tabla de fixtures de referencia entre archivos → resultado esperado en cada caso.

### U-03 — Validación de contrato por tabla [UNIT] · P1 · [ ]
**Criterio**: cablear contra un step interno no declarado, referenciar un input/output inexistente o declarar `inputs`/`outputs` en un archivo raíz → errores de validación específicos.
**Verificación**: tabla de YAML inválidos → código de error y mensaje esperados.

### U-04 — Schema v2 validado con zod (no JSON Schema) [UNIT] · P1 · [ ]
**Criterio**: el schema del workflow v2 (version, steps, workspace, inputs/outputs, bindings y condicionales por tipo de step) se define con zod (práctica vigente del repo); el YAML se valida contra la representación zod equivalente, no contra JSON Schema; los errores nombran el campo exacto.
**Verificación**: tabla de YAML inválidos → mensajes de error con path del campo y código estable.

---

# Spec: v2-reporte — Reporte de cambios

### F-01 — Reporte al terminar la ejecución de nivel superior [E2E] · P0 · [ ]
**Criterio**: al terminar la ejecución de nivel superior, el sistema calcula y muestra qué archivos cambiaron respecto al punto de partida; `execution.report` devuelve `changed_files`.
**Verificación**: ejecución que crea/modifica/elimina archivos → el reporte lista exactamente esos cambios.

### F-02 — Calculado una sola vez, solo a nivel superior [E2E] · P1 · [ ]
**Criterio**: el reporte se calcula una sola vez, a nivel de la ejecución de nivel superior — nunca por cada workflow anidado internamente.
**Verificación**: ejecución con nodos workflow anidados → un único reporte al final; ningún sub-reporte intermedio.

### F-03 — Exactitud respecto al git real [INT] · P1 · [ ]
**Criterio**: el reporte es exacto respecto al estado real del workspace: archivos nuevos, modificados, eliminados (y renombrados si aplica), contra el punto de partida.
**Verificación**: fixture con los cuatro tipos de cambio → el reporte coincide con `git status`/`git diff --name-status` del workspace.

### U-01 — Cálculo de diff por tabla [UNIT] · P1 · [ ]
**Criterio**: el cálculo de `changed_files` compara el punto de partida (base/commit) con el estado final y cubre nuevos, modificados, eliminados y renombrados.
**Verificación**: tests de tabla con repos de prueba (git real en memoria o directorio temporal).

---

# Spec: v2-store — Persistencia tras interfaz de repositorio

### F-01 — Ningún acceso directo fuera del store [INT] · P0 · [ ]
**Criterio**: ninguna capa fuera del store accede a SQLite directamente; todo acceso pasa por las interfaces de repositorio.
**Verificación**: script de verificación de dependencias (import graph / paquetes) que falla si un módulo de dominio o CLI importa el driver de persistencia.

### F-02 — DDL v2 completo [INT] · P0 · [ ]
**Criterio**: el esquema implementa las tablas de referencia: `projects`, `executions`, `execution_steps`, `generations`, `attempts`, `attempt_transport`, `leases`, `step_transition_events`, `attempt_events`, `interactions`, `path_claims` — con sus constraints y CHECK de enums.
**Verificación**: inspección del esquema creado por el sistema (sin migraciones manuales) contra el DDL de referencia.

### F-03 — WAL y timestamps [INT] · P1 · [ ]
**Criterio**: `PRAGMA journal_mode = WAL`, `PRAGMA foreign_keys = ON`; todas las columnas de tiempo son ISO 8601 UTC.
**Verificación**: consulta de pragmas y muestreo de valores de tiempo en registros creados.

### U-01 — Backend intercambiable [UNIT] · P0 · [ ]
**Criterio**: la suite de dominio (máquina de estados, leases, claims, eventos) corre contra un backend alternativo (in-memory/fake) que implementa las mismas interfaces, sin cambios en el código de dominio (puerta abierta a concurrencia futura).
**Verificación**: la suite completa pasa contra el backend fake y contra SQLite con los mismos tests.

### U-02 — Esquema idempotente [UNIT] · P1 · [ ]
**Criterio**: la creación/migración del esquema es idempotente y transaccional (re-ejecución no rompe, fallo a mitad no deja estado parcial).
**Verificación**: crear dos veces + simular fallo a mitad de migración → estado consistente.

### U-03 — CHECK de enums [UNIT] · P1 · [ ]
**Criterio**: los enums (`status` de executions/steps/attempts, `type` de steps, modos de workspace) rechazan valores inválidos a nivel de base de datos.
**Verificación**: INSERT con valor inválido → error de constraint.

---

# Spec: v2-distribucion — Distribución

### F-01 — Instalación de punta a punta [E2E] · P0 · [ ]
**Criterio**: el mecanismo de distribución es el vigente — paquete npm global (`pnpm add -g shardeo`), bin `dist/index.js`, `files: ["dist"]`, `prepublishOnly` con build — y la instalación funciona en un proyecto limpio sin pasos manuales adicionales.
**Verificación**: instalación en entorno limpio → `shardeo` resoluble y ejecutable; `npm run build` produce `dist/` completo.

### F-02 — Sin scripts post-install [E2E] · P1 · [ ]
**Criterio**: la instalación no ejecuta scripts post-install que corran código (vector de ataque conocido; pnpm los desactiva por defecto).
**Verificación**: revisión del paquete/artefacto de distribución → sin hooks de instalación ejecutables.

### F-03 — init sin dependencias adicionales [E2E] · P1 · [ ]
**Criterio**: `shardeo init` funciona en un proyecto sin ninguna dependencia previa instalada (herramienta local de un solo checkout, sin infraestructura de red).
**Verificación**: proyecto vacío → init completo; ninguna llamada de red requerida (verificable por red deshabilitada).

---

# Spec: v2-flujo-gentle-ai — Flujo de desarrollo con gentle-ai

### F-01 — Cada spec como change SDD [PROC] · P0 · [ ]
**Criterio**: cada spec de este archivo (v2-reconciliacion, v2-no-regresion, v2-broker…v2-distribucion) se desarrolla como un change SDD con proposal → spec → design → tasks → apply → verify → archive; los criterios de este archivo son la fuente de los criterios de aceptación de cada spec.
**Verificación**: existe un change por spec con trazabilidad criterio → spec → tests.

### F-02 — Verificación vía sdd-verify [PROC] · P0 · [ ]
**Criterio**: `sdd-verify` (o la verificación equivalente del flujo gentle-ai) ejecuta las verificaciones descritas: los `[E2E]` como frontera pública, los `[UNIT]`/`[INT]` como tests del módulo.
**Verificación**: el reporte de verificación de cada change lista los criterios cubiertos con su evidencia.

### F-03 — Entrega por el flujo gentle-ai [PROC] · P0 · [ ]
**Criterio**: la entrega de cada delta pasa por las gates del flujo gentle-ai (review receipts, delivery gates), no por el pipeline de iniciativas; no se crean nuevas iniciativas.
**Verificación**: historial de entregas con receipts; ausencia de `.docs/initiatives/` nuevas.

### U-01 — Trazabilidad criterio→test [UNIT] · P1 · [ ]
**Criterio**: cada criterio de este archivo tiene al menos un test nombrado que lo verifica (mapeo explícito, p. ej. tabla en la spec del change).
**Verificación**: script de auditoría que recorre los IDs de criterios y confirma su test correspondiente existe y pasa.

---

## Resumen de tracking

> **Fuente única de tracking**: este documento. Cada criterio lleva checkbox (`- [ ]` pendiente / `- [x]` cumplido) en su encabezado. La columna **Estado** de la tabla refleja la fase SDD del change de cada spec — `pendiente → proposal → spec → design → tasks → apply → verify → archive → completo` — y se actualiza al cierre de cada fase. El detalle fino (artefactos, evidencia, decisiones) vive en los artifacts del change (openspec/ + Engram).

| id-spec | Contenido | Criterios | P0 | Estado |
|---|---|---|---|---|
| `v2-reconciliacion` | Reconciliación de la documentación normativa | 5 | 3 | **completo** |
| `v2-no-regresion` | Preservación Specs 1–9 | 17 | 14 | pendiente |
| `v2-broker` | Broker persistente por proyecto | 9 | 4 | pendiente |
| `v2-ipc` | IPC JSON-RPC CLI↔Broker y eventos | 13 | 6 | pendiente |
| `v2-adapter` | Contrato de adapter v2 y multi-harness | 10 | 6 | pendiente |
| `v2-path-claims` | path_claims y aislamiento de workspace | 10 | 5 | pendiente |
| `v2-composicion` | Composición de workflows | 11 | 6 | pendiente |
| `v2-reporte` | Reporte de cambios | 4 | 1 | pendiente |
| `v2-store` | Persistencia tras interfaz de repositorio | 6 | 3 | pendiente |
| `v2-distribucion` | Distribución | 3 | 1 | pendiente |
| `v2-flujo-gentle-ai` | Flujo de desarrollo con gentle-ai | 4 | 3 | pendiente |
| **Total** | | **92** | **52** | |

Última actualización: 2026-08-27 — `v2-reconciliacion` **completo** (5/5 criterios, verify PASS, archivado en `openspec/changes/archive/2026-08-27-v2-reconciliacion/`).

## Fuera de alcance (explícitamente no cubierto)

- `shardeo doctor`, `shardeo setup` y el registro de CLIs en `config.yaml` (roadmap "Futuro: v2" de AGENTS.md): no forman parte de la propuesta normativa v2; se gestionan como specs aparte si se deciden.
- Concurrencia multi-usuario compartida: diferido; `v2-store/U-01` solo garantiza que la interfaz no lo bloquee.
- Bundle CAS de interacciones (parte no implementada): diferido; `v2-no-regresion/F-08` cubre solo lo ya existente (Spec 9a).