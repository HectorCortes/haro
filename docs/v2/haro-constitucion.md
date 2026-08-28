# Shardeo v2 — Constitución del proyecto

> Manifiesto normativo. Contiene únicamente reglas de negocio y definiciones técnicas ya decididas — no investigación, no comparación de opciones, no justificación. Es la referencia canónica contra la que se valida cualquier decisión de diseño o implementación posterior. Toda regla nueva que se agregue en el futuro debe ser consistente con esta; si una decisión futura contradice algo aquí, este documento se actualiza explícitamente, nunca se ignora en silencio.

---

## I. Objetivos no negociables

Estos objetivos son fijos. Cualquier regla de negocio concreta puede rediseñarse; estos objetivos, no:

- **I.1** Trazabilidad total y auditable de cada ejecución.
- **I.2** Reproducibilidad: dado el mismo contexto, el mismo intento debe poder reconstruirse.
- **I.3** Fail-closed sistemático: ante ambigüedad, capacidad desconocida o estado no reconocible, el sistema falla explícitamente; nunca asume ni adivina.
- **I.4** Ningún mecanismo puede permitir bypass de la política de permisos.
- **I.5** Neutralidad metodológica: el core nunca decide ramas, condicionales ni calidad del trabajo de un harness. Esa responsabilidad es exclusiva del agente orquestador externo y de las instrucciones del propio workflow.
- **I.6** Shardeo coordina y reporta; **nunca** integra, publica ni descarta trabajo por su cuenta. Toda acción de integración final es responsabilidad de quien usa Shardeo.

## II. Harnesses

- **II.1** Harnesses objetivo iniciales: OpenCode, Claude Code.
- **II.2** Arquitectura diseñada para incorporar, sin modificar el core: Codex, Gemini CLI, GitHub Copilot CLI, Pi, y cualquier otro CLI de agente futuro.
- **II.3** El core nunca conoce literales, endpoints, flags, formatos de evento o nombres de campos específicos de un proveedor. Cualquier detalle de proveedor vive exclusivamente dentro de su adapter.
- **II.4** No hay migración de datos ni compatibilidad requerida con la base de datos ni la superficie de comandos de Shardeo v1.

## III. Distribución y entorno

- **III.1** El modelo de distribución no está atado a ningún ecosistema en particular; puede cambiar respecto a v1.
- **III.2** Herramienta local, de un solo proyecto/checkout, sin infraestructura de red ni servidor propio salvo el puente local hacia el harness.
- **III.3** Diseño para un solo usuario en el estado actual, sin cerrar la puerta a una futura expansión a concurrencia multi-usuario. Esto se traduce en: la capa de persistencia debe estar detrás de una interfaz de repositorio, no de acceso directo a SQLite.

## IV. Modelo de dominio

### IV.1 Vocabulario

| Término | Definición |
|---|---|
| **Workflow** | Definición declarativa en YAML de un grafo (DAG) de steps. |
| **Run / Execution** | Una instancia concreta de ejecución de un workflow. Tiene un único `execution_id`. |
| **Step** | Nodo del DAG. Tipo: `command`, `agent`, o `workflow` (IV.4). |
| **Attempt** | Un intento concreto de ejecutar un step. Puede haber múltiples attempts por step (retries, fallback entre candidatos). |
| **Generación** | Versión de los artefactos producidos por un step. Reabrir un step invalida su generación y la de todo lo que dependa de ella (cascada). |
| **Harness** | Programa externo (OpenCode, Claude Code, etc.) que ejecuta el trabajo real de un step tipo `agent`. |
| **Adapter** | Componente que traduce el contrato interno de Shardeo al protocolo específico de un harness. |
| **Broker** | Daemon por proyecto que gestiona sesiones activas y hace de puente entre CLI y adapters. |
| **path_claims** | Registro de reclamos de escritura por proyecto (IX). |

### IV.2 `step.mode` no es un enum cerrado

`headless` / `supervised` / `terminal` no son tres implementaciones separadas: son perfiles de conveniencia derivados de las **capacidades negociadas** con el adapter en tiempo de `initialize`. El modo efectivo de un step se deriva de qué capacidades declaró el adapter, no de una elección aislada del sistema.

### IV.3 `Attempt` es agnóstico de transporte

`Attempt` almacena únicamente estado, tiempos, resultado y digest de evidencia — nada específico de un proveedor. Cualquier detalle de transporte (identidad nativa de sesión, campos propios de un protocolo) vive en una tabla de extensión por adapter, nunca en la tabla `attempts`.

### IV.4 Tipo de nodo `workflow`

Un step puede ser de tipo `workflow`, en cuyo caso referencia otro archivo YAML en lugar de ejecutar un `command` o un `agent` directamente. Ver Sección X para su comportamiento completo.

## V. Contrato del adapter (harness)

### V.1 Métodos obligatorios (todo adapter debe implementarlos)

```
Probe(ctx) → (disponible, versión, capacidades declaradas)
Initialize(ctx, coreCapabilities) → (protocolVersion, harnessCapabilities)
NewSession(ctx, bundle) → SessionHandle
Session.Prompt(ctx, contexto) → stream de eventos
Session.Cancel(ctx)
```

### V.2 Métodos opcionales (gateados por capability)

```
Session.RequestPermission(...)   — el harness llama HACIA el core, no al revés
Session.Terminal*(...)
Session.LoadPrevious(ctx, id)
```

### V.3 Regla de capacidades

- **V.3.1** Ningún método fuera de V.1 es obligatorio.
- **V.3.2** El core nunca invoca un método opcional sin haber verificado antes, en `Initialize`, que el adapter lo declaró soportado.
- **V.3.3** Nuevas capacidades se agregan de forma aditiva; nunca requieren incrementar la versión mayor del protocolo.
- **V.3.4** Un incremento de versión mayor del protocolo solo ocurre ante un cambio incompatible en los métodos obligatorios (V.1).

### V.4 Frontera de responsabilidad

- **V.4.1** El core es responsable de: ciclo de vida del step/attempt, políticas de permisos, persistencia, cascada de invalidación.
- **V.4.2** El adapter es responsable de: traducir el protocolo específico del harness al contrato de V.1/V.2.
- **V.4.3** El harness (proceso externo) es responsable de: todo el razonamiento, decisiones de implementación, e interpretación de instrucciones. Shardeo nunca interpreta ni evalúa ese razonamiento.

## VI. Ciclo de vida de una ejecución

```
pending → running → completed
                  ↘ failed → running (re-ejecución)
completed/failed → pending (reopen, invalida generación)
pending → skipped (solo si el trabajo aún no se ejecutó)
```

- **VI.1** Solo el poseedor del lease vigente de un step puede escribir su resultado.
- **VI.2** Un lease vencido queda invalidado por fencing token; ninguna escritura posterior a la expiración es válida aunque el proceso original siga vivo.
- **VI.3** El fallback entre candidatos de harness/modelo no clasifica errores semánticamente; es un mecanismo agnóstico de "intentar el siguiente candidato disponible".

## VII. Comunicación (IPC)

- **VII.1** CLI ↔ Broker: JSON-RPC 2.0 sobre socket Unix (o named pipe en Windows). Un broker por proyecto, identificado por la ruta del proyecto. Arranque perezoso: si el socket no responde, el CLI arranca el broker y reintenta.
- **VII.2** Un broker puede mantener múltiples sesiones concurrentes activas (múltiples ejecuciones en paralelo del mismo o distinto workflow, dentro del mismo proyecto).
- **VII.3** Distintos proyectos tienen brokers completamente independientes, sin coordinación entre sí.
- **VII.4** Broker ↔ Adapter ↔ Harness: el adapter elige el transporte nativo que su harness soporta —por ejemplo, stdio-RPC, HTTP local + SSE o JSONL—, lo declara en capacidades y lo negocia en `initialize`; el core no impone ni jerarquiza un transporte. (Ejemplo no normativo: OpenCode supervised usa HTTP local + SSE gestionado.)

## VIII. Modelo de eventos y approvals

- **VIII.1** Los eventos se identifican por cursor monotónico por sesión/attempt.
- **VIII.2** Ningún evento carga la salida completa de un harness; solo referencias/deltas.
- **VIII.3** La resolución de una interacción (approval, respuesta a pregunta) es idempotente: reenviar la misma resolución no produce efectos duplicados.
- **VIII.4** Las solicitudes de permiso las origina el harness llamando hacia el core (vía su adapter), nunca al revés.
- **VIII.5** El core aplica la política de permisos de forma fail-closed: cualquier tipo de acción, capacidad o decisión no reconocible se rechaza por defecto.

## IX. Coordinación de escritura y aislamiento de workspace

### IX.1 Principio

Shardeo coordina para evitar que dos escritores concurrentes choquen sobre el mismo recurso, y reporta qué cambió al terminar. Nunca mergea, publica ni descarta trabajo por su cuenta (ver I.6).

### IX.2 Configuración de aislamiento físico, en tres niveles

```
sistema  → default: worktree por workflow
  └─ workflow → puede fijar aislamiento para todos sus steps
       └─ step → puede sobreescribir el nivel workflow
```

- **IX.2.1** Default del sistema: cada ejecución de un workflow corre en su propio `git worktree`.
- **IX.2.2** Un workflow puede desactivar el aislamiento (`shared`) para todos sus steps.
- **IX.2.3** Un step individual puede sobreescribir la configuración heredada de su workflow.

### IX.3 Registro de reclamos (`path_claims`)

- **IX.3.1** Es el único mecanismo de garantía real de no-colisión; la estrategia física de aislamiento (IX.2) solo reduce cuándo hace falta invocarlo.
- **IX.3.2** Los reclamos se hacen sobre **identidad lógica** de path dentro del repo (p. ej. `src/foo.ts`), nunca sobre la ruta física donde efectivamente se escribe.
- **IX.3.3** Estructura: `(project_id, logical_path, mode, owner, acquired_at)`, donde `mode` es `isolated:<execution_id>` o `shared`.
- **IX.3.4** El path lógico se canonicaliza (resuelto, sin symlinks) y se compara por prefijo, de forma que reclamar un directorio bloquea también a sus hijos.
- **IX.3.5** Un reclamo se libera al terminar el step correspondiente (éxito o fallo), no al terminar la ejecución completa.

### IX.4 Matriz de comportamiento

| Caso | Colisión física | Comportamiento |
|---|---|---|
| `isolated` vs `isolated`, mismo path lógico | No | Permitido sin bloqueo. Divergencias, si las hay, se descubren cuando alguien decida integrar ambos worktrees — fuera del alcance de Shardeo |
| `shared` vs `shared`, mismo path lógico | Sí | El registro bloquea: el segundo step no arranca hasta que el primer reclamo se libere |
| `isolated` vs `shared`, mismo path lógico | No físicamente, sí de intención | Gobernado por `on_logical_conflict`: `block` (default, fail-closed) o `allow` (opt-in explícito por workflow) |
| Recursos fuera del repo (caché, DB de test, artefactos no versionados) | N/A — misma ruta siempre | Se tratan siempre como `shared`. La config del proyecto declara explícitamente qué paths son "del repo" y cuáles son "externos" |

### IX.5 Reporte de cambios

- **IX.5.1** Al terminar la ejecución de nivel superior (la invocada por el orquestador externo), Shardeo calcula y muestra qué archivos cambiaron respecto al punto de partida.
- **IX.5.2** El reporte se calcula una sola vez, a nivel de la ejecución de nivel superior — nunca por cada workflow anidado internamente (ver X).

## X. Composición de workflows (workflows anidados)

### X.1 Modelo

- **X.1.1** Un step de tipo `workflow` referencia otro archivo YAML.
- **X.1.2** La composición ocurre en tiempo de **planificación/compilación**, no en tiempo de ejecución. No existen ejecuciones "hijas": el resultado es un único DAG plano bajo un único `execution_id`.
- **X.1.3** La paralelización entre nodos `workflow` incluidos usa exactamente el mismo mecanismo de DAG que cualquier otro step (nodos sin `depends_on` entre sí corren en paralelo). Configurarlo correctamente es responsabilidad del usuario.

### X.2 Namespacing

- **X.2.1** Los steps internos de un workflow incluido se identifican con el prefijo del nombre del nodo que lo incluyó: `<nombre_nodo>.<step_interno>`.
- **X.2.2** Esto permite incluir el mismo workflow varias veces en un mismo padre, cada instancia con namespace propio, sin colisión de ids.

### X.3 Frontera explícita

- **X.3.1** Todo workflow incluible declara su propio contrato `inputs`/`outputs`, mapeado a steps internos concretos.
- **X.3.2** El workflow que incluye a otro solo puede cablear (`depends_on`, `requires`) contra ese contrato declarado, nunca directamente contra steps internos del workflow incluido.

### X.4 La cascada de invalidación ignora la frontera de composición

- **X.4.1** La cascada de invalidación al reabrir un step opera sobre el grafo real de generaciones/artefactos, ya aplanado — no sobre la estructura declarada de `inputs`/`outputs`.
- **X.4.2** Reabrir algo invalida únicamente los steps internos de un workflow incluido cuyos `produces` alimentan realmente lo reabierto, con la misma granularidad fina que ya aplica entre steps normales — independientemente de qué se haya declarado como `outputs` público en X.3.
- **X.4.3** El contrato `inputs`/`outputs` (X.3) y el comportamiento de la cascada (X.4) son reglas independientes; una no condiciona a la otra.

### X.5 Ciclos

- **X.5.1** Un workflow no puede incluirse a sí mismo, directa o transitivamente.
- **X.5.2** La detección de ciclos se resuelve estáticamente, sobre el grafo de referencias entre archivos, antes de aplanar. No requiere ningún mecanismo en tiempo de ejecución.

## XI. Testing

- **XI.1** Todo adapter nuevo debe pasar una suite de contract tests de frontera pública antes de integrarse: `step run → subproceso real del harness → protocolo → salida JSON pública → store → settle`.
- **XI.2** Esta suite debe ejecutarse contra un binario real del harness, no solo contra fixtures o mocks, salvo en los casos donde depender de un binario real en CI no sea viable (para esos, se usan fixtures grabadas).
- **XI.3** La suite de contract tests es un artefacto de primera clase del repositorio desde el inicio, no una incorporación tardía.

## XII. Estado implementado y diferidos deliberados

- **XII.1** `terminal`/PTY ya está implementado por Spec 9c (`src/runtime/pty.ts`, `src/runtime/terminal.ts`, `src/runtime/attach.ts`; `SPECS.md` §9c) como superficie existente a preservar: PTY real, attach humano y presencia.
- **XII.2** El bundle inmutable, su manifiesto SHA-256 y la prueba de admisión ya están implementados por Spec 9a (`src/runtime/bundle.ts`, `src/schema/bundle.ts`, `src/runtime/admission.ts`) y quedan bajo no-regresión (`v2-no-regresion`); el CAS de interacciones permanece diferido hasta su spec correspondiente.
- **XII.3** Concurrencia multi-usuario compartida — no se implementa ahora; la interfaz de persistencia (III.3) se diseña para no bloquear esta evolución.
- **XII.4** Deuda conocida: el parser JSONL de OpenCode permanece en `src/utils/agent.ts` (ruta headless) y `v2-adapter/F-04` debe trasladarlo al adapter.

## XIII. Reglas de estabilidad (difícil de cambiar después, diseñar bien desde el inicio)

- **XIII.1** El contrato del adapter (Sección V) — cambiarlo obliga a reescribir todos los adapters existentes.
- **XIII.2** El formato de los eventos persistidos (cursor monotónico, forma del payload) — es la base de auditoría y reanudación; cambiarlo rompe compatibilidad con datos históricos.
- **XIII.3** La regla de que el core nunca conoce literales de un proveedor (II.3) — es disciplina de código, no una feature; una sola excepción contamina el resto del sistema.
- **XIII.4** La identidad lógica de path en `path_claims` (IX.3.2) — cambiarla obliga a reinterpretar todo el historial de coordinación.
- **XIII.5** El contrato explícito `inputs`/`outputs` de un workflow incluido (X.3) — cambiarlo rompe la compatibilidad de cualquier workflow que ya incluya a otro.
