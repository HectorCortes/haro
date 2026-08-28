# Shardeo — Especificaciones Funcionales del MVP

---

## Principio de orquestación

Shardeo ejecuta y persiste las transiciones solicitadas por un agente orquestador, pero no interpreta el significado semántico de las respuestas de los agentes ni decide qué camino debe seguir un workflow.

Las instrucciones generales del workflow definen cómo debe conducir el proceso el orquestador: cuándo solicitar aprobación al usuario, cómo interpretar el resultado de un step y cuándo reintentar, reabrir u omitir trabajo. Las instrucciones de cada step describen únicamente cómo debe ejecutar su tarea el agente o comando correspondiente.

Las decisiones del orquestador se materializan mediante comandos explícitos de Shardeo para que cada transición sea validada, persistida y trazable.

---

## Spec 1: Inicialización del proyecto

status: done

### Objetivo

Permitir que un usuario prepare cualquier repositorio para usar Shardeo, generando la estructura de directorios y archivos de configuración necesarios para definir workflows.

### Descripción funcional

El usuario instala Shardeo globalmente (`pnpm add -g shardeo`) y luego ejecuta `shardeo init` dentro de la raíz de un proyecto. Shardeo genera la carpeta `.shardeo/` con la estructura completa: `config.yaml`, y los directorios `workflows/`, `skills/`, `artifacts/` y `docs/`. Si la estructura ya existe, Shardeo informa al usuario sin sobrescribir nada.

### Criterios de aceptación

- `shardeo init` crea la estructura `.shardeo/` con todos los subdirectorios documentados (`workflows/`, `skills/`, `artifacts/`, `docs/`).
- Se genera un `config.yaml` con valores por defecto válidos.
- Si `.shardeo/` ya existe, el comando termina sin error y sin modificar el contenido existente, informando que el proyecto ya está inicializado.
- El comando falla con un mensaje claro si se ejecuta en un directorio donde no hay permisos de escritura.
- La salida del comando confirma la ruta de la estructura generada.

### Ejemplo de flujo de usuario

```
$ cd ~/projects/my-api
$ shardeo init
✓ Proyecto inicializado en /home/user/projects/my-api/.shardeo/
```

---

## Spec 2: Descubrimiento de workflows

status: done

### Objetivo

Permitir que el orquestador (o el usuario directamente) explore qué workflows están disponibles en el proyecto, entienda su propósito y conozca su estructura antes de ejecutarlos.

### Descripción funcional

El usuario ejecuta `shardeo workflows list` para obtener la lista de workflows definidos en `.shardeo/workflows/`, cada uno con su nombre y descripción extraídos del `workflow.yaml`. Para un workflow específico, ejecuta `shardeo workflows describe <workflow>`, que retorna el detalle completo: descripción, instrucciones generales (contenido del `instructions.md`), lista de steps con sus tipos, dependencias y artefactos producidos.

Ambos comandos validan la estructura YAML con Zod antes de presentar resultados. Si un workflow tiene errores de schema, aparece en el listado marcado como inválido, con el error específico.

### Criterios de aceptación

- `shardeo workflows list` muestra nombre y descripción de cada workflow encontrado en `.shardeo/workflows/`.
- Si no hay workflows definidos, el comando retorna un mensaje informativo (no un error).
- `shardeo workflows describe <workflow>` muestra: descripción, instrucciones generales, y para cada step: id, tipo, agentes configurados (si aplica), orden de ejecución (`depends_on`), validaciones de entrada (`requires`) y validaciones de salida (`produces`).
- Si el workflow referenciado no existe, el comando falla con un mensaje claro indicando los workflows disponibles.
- Un workflow con YAML malformado o que no pasa la validación de schema aparece en `list` marcado como inválido, y `describe` muestra el error de validación concreto.
- La salida es estructurada (JSON) para consumo del orquestador, con una opción de formato legible para uso humano directo.

### Ejemplo de flujo de usuario

```
$ shardeo workflows list
┌──────────────────┬──────────────────────────────────────────────┐
│ Workflow         │ Descripción                                  │
├──────────────────┼──────────────────────────────────────────────┤
│ backend-feature  │ Workflow para desarrollar una feature de     │
│                  │ backend, desde diseño hasta tests.           │
│ hotfix           │ Workflow para aplicar un fix crítico.        │
└──────────────────┴──────────────────────────────────────────────┘

$ shardeo workflows describe backend-feature
# backend-feature
Workflow para desarrollar una feature de backend...

Steps:
  1. architecture (agent) → produces: architecture.md, adr.md
  2. api-design (agent) → depends_on: architecture → requires: architecture.md → produces: api-contract.md
  3. database-design (agent) → depends_on: architecture → requires: architecture.md → produces: database-design-report.md
  4. lint (command) → depends_on: api-design, database-design
  5. implementation (agent) → depends_on: lint → requires: api-contract.md, database-design-report.md → produces: implementation-report.md
```

---

## Spec 3: Ejecución de un workflow con steps de tipo command

status: done

### Objetivo

Permitir la ejecución completa de un workflow que contenga únicamente steps de tipo `command`, validando el ciclo de vida completo: inicio, resolución de orden de ejecución (`depends_on`), validación de artefactos (`requires`/`produces`), y finalización.

### Descripción funcional

El usuario ejecuta `shardeo run <workflow>`, lo que registra la ejecución en la base de datos SQLite y retorna un `execution-id`. A partir de ahí, el flujo sigue el modelo de ejecución de Shardeo: el orquestador (o el usuario) consulta `shardeo steps next <execution-id>` para obtener los steps ejecutables, luego ejecuta cada uno con `shardeo step run <execution-id> <step-id>`.

Para steps de tipo `command`, Shardeo ejecuta el comando declarado, captura stdout/stderr y el código de retorno. Si el comando retorna exit code 0, Shardeo valida que los artefactos declarados en `produces` existan y marca el step como completado automáticamente. Si el comando falla o los artefactos no fueron generados, el step se marca como fallido.

`shardeo status <execution-id>` muestra el estado actual de la ejecución: qué steps se completaron, cuáles fallaron y cuáles están pendientes.

### Criterios de aceptación

- `shardeo run <workflow>` crea un registro de ejecución y retorna un `execution-id` único.
- `shardeo steps next <execution-id>` retorna los steps cuyos predecessores (`depends_on`) se han completado y que aún no han sido completados ni están en ejecución. Steps sin `depends_on` aparecen como disponibles inmediatamente.
- Si todos los steps pendientes tienen predecessores incompletos, retorna una lista vacía.
- `shardeo step run <execution-id> <step-id>` valida primero que los artefactos en `requires` existan. Si faltan, el step no se ejecuta y retorna un error con la lista de artefactos ausentes.
- Si la validación de `requires` pasa, ejecuta el comando, captura su salida y código de retorno.
- Un step de tipo `command` con exit code 0 y artefactos de `produces` generados correctamente se marca como completado sin intervención del orquestador.
- Un step con exit code distinto de 0 se marca como fallido, incluyendo stdout y stderr en la respuesta.
- Un step con exit code 0 pero artefactos de `produces` faltantes se marca como fallido con un mensaje que indica qué artefactos no se encontraron.
- `shardeo status <execution-id>` muestra el estado de cada step (pendiente, completado, fallido) y el estado general del workflow.
- No se puede ejecutar un step cuyos predecessores (`depends_on`) no se han completado; el comando retorna un error explícito.

### Ejemplo de flujo de usuario

```
$ shardeo run lint-only-workflow
Ejecución iniciada: exec-a1b2c3

$ shardeo steps next exec-a1b2c3
Steps disponibles:
  - lint (command)

$ shardeo step run exec-a1b2c3 lint
Ejecutando: npm run lint
Exit code: 0
Step completado.

$ shardeo status exec-a1b2c3
Workflow: lint-only-workflow
Estado: completado
Steps:
  ✓ lint — completado
```

---

## Spec 4: Ejecución de steps de tipo agent

status: implemented

### Objetivo

Permitir que Shardeo ejecute steps que requieren un agente de IA, resolviendo qué CLI utilizar, inyectando el contexto necesario y retornando el resultado al orquestador para que decida si el step está completo.

### Descripción funcional

Cuando el orquestador ejecuta `shardeo step run <execution-id> <step-id>` sobre un step de tipo `agent`, Shardeo vuelve a leer y validar el YAML completo del workflow antes de ejecutar el step. Si el formato o alguna regla de negocio no es válida, retorna el error sin invocar ningún CLI ni modificar el estado del step. Esta validación se repite en cada ejecución porque el workflow puede cambiar entre steps.

Después de validar el workflow, Shardeo:

1. Resuelve candidatos exclusivamente desde `steps[].agents`, en orden declarado. En v1, el único identificador válido es `opencode`; el modelo es opcional y opaco. Un candidato no disponible se omite sin crear intento. Después de una invocación iniciada, limpiada y con JSONL válido, cualquier error limpio hace fallback al siguiente candidato **sin clasificación semántica** de su causa.
2. Construye el contexto del step: instrucciones operacionales (generadas por Shardeo) + instrucciones de dominio (el `instructions.md` del step) + skills referenciados + artefactos requeridos.
3. Invoca el CLI en modo no interactivo y sin flags de bypass de permisos (`--auto` nunca se inyecta). El valor de modelo se pasa sin modificar; el CLI es responsable de la validación semántica.
4. Monitorea el timeout de inactividad configurable (default 5 minutos), que se reinicia con cualquier evento de salida. Si el timeout expira, el step termina con `permission_timeout`.
5. Detecta solicitudes de aprobación únicamente mediante fixtures exactos y versionados de OpenCode 1.17.18. En la versión 1, una coincidencia es terminal (`permission_required`). La resolución en vivo mediante adapters y sesiones gestionadas se define en la Spec 9 (modos `supervised` y `terminal`), ya implementada; la ruta `headless` de esta spec conserva su comportamiento salvo la migración de inactividad de la Spec 9.
6. Captura un baseline real antes de cada intento y conserva snapshots de archivos nuevos o modificados con SHA-256. El presupuesto acumulativo de contenido crudo es 1 MiB por step; excederlo produce `artifact_context_too_large`.
7. Persiste evidencia canónica y acotada. `DiagnosticRaw` es la única autoridad de bytes diagnósticos sin redactar: hasta 1 MiB conserva el frame exacto; si excede el límite, conserva prefijo, sufijo, tamaño y SHA-256. `AttemptEvidence`, las columnas legacy y la consola reciben únicamente proyecciones sanitizadas de hasta 16 KiB.
8. Cuando un error limpio habilita fallback, construye para el siguiente candidato un contexto extendido con las instrucciones, skills y artefactos requeridos originales, más una proyección sanitizada de snapshots, errores concretos del proveedor y referencias de intentos anteriores. Esta proyección nunca incluye evidencia cruda y tiene un presupuesto acumulado de 2 MiB por `step run`.

El step NO se marca como completado automáticamente. El orquestador debe invocar `shardeo step complete <execution-id> <step-id>` para hacerlo. En ese momento, Shardeo valida que los artefactos declarados en `produces` existan bajo `.shardeo/artifacts/`. Si no existen, el comando falla y el step permanece en estado "en progreso".

### Criterios de aceptación

- Shardeo obtiene los candidatos exclusivamente de `steps[].agents`, recorre la lista en orden y usa el primer CLI soportado que esté disponible en el PATH.
- En v1, el único CLI soportado es `opencode`.
- Antes de ejecutar cada step, Shardeo vuelve a leer y valida el YAML completo del workflow, incluyendo su formato y reglas de negocio.
- Un workflow que declara cualquier agente distinto de `opencode` es inválido. La ejecución retorna un error antes de invocar un CLI o modificar el estado del step.
- Si un CLI soportado no está instalado o no está disponible, Shardeo intenta con el siguiente candidato declarado sin crear un intento para el candidato descartado durante el sondeo.
- Si ninguna sonda de disponibilidad supera la validación antes de invocar un CLI, el step falla con una respuesta que incluye evidencia sanitizada de todos los candidatos evaluados y no se crea ningún intento.
- Todo error limpio de OpenCode que cumpla los gates de proceso y JSONL se registra y avanza al siguiente candidato, sin inferir si proviene de cuota, contexto, modelo o proveedor.
- `permission_required`, `permission_timeout`, `artifact_context_too_large`, `adapter_contract_error`, `process_start_failed`, `process_cleanup_failed`, `context_error` y `persistence_error` son terminales en la versión 1 y no hacen fallback. Los modos gestionados que resuelven permisos en vivo (`supervised` y `terminal`) se definen en la Spec 9, ya implementada; la ruta `headless` de esta spec conserva estos códigos salvo que la inactividad pasa a reportarse como `process_inactivity_timeout` según la migración de la Spec 9.
- Cada invocación de un CLI se registra como un intento independiente del mismo step.
- Cada candidato invocado por fallback recibe un contexto extendido con las instrucciones, skills y artefactos requeridos originales, una proyección sanitizada de snapshots y errores previos, y referencias de intentos anteriores. La proyección nunca contiene evidencia cruda y su presupuesto acumulado es 2 MiB por `step run`.
- El contexto inyectado al CLI incluye, en este orden: instrucciones operacionales, instrucciones de dominio, skills, y contenido de los artefactos requeridos.
- El CLI se invoca en modo no interactivo con `shell: false`. No se inyecta `--auto` ni ningún otro flag de bypass de permisos.
- La respuesta visible se sanitiza y limita; la evidencia local acotada conserva el material de diagnóstico y su digest.
- `shardeo step complete <execution-id> <step-id>` valida `produces` bajo `.shardeo/artifacts/` con contención: rechaza paths absolutos, `..` y escapes por symlink; permite symlinks cuyo destino permanece dentro de la raíz.
- Para un step `agent`, `shardeo step complete` valida primero que todos los artefactos declarados en `produces` existan y sean seguros. Solo entonces marca el step como completado. Los steps cuyos predecesores declarados en `depends_on` estén completados pasan a estar disponibles en `steps next`; `requires` se valida únicamente al ejecutar el step.
- La respuesta completa o parcial, el CLI utilizado, el modelo y el motivo de finalización de cada intento se almacenan en la base de datos.
- Los snapshots de artefactos se capturan con hash SHA-256, excluyendo archivos sin cambios. El presupuesto acumulativo es 1 MiB por step.
- La validación de artefactos (`requires`/`produces`) rechaza paths absolutos, `..` y escapes por symlink para steps `agent` y `command`.

### Ejemplo de flujo de usuario

```
$ shardeo step run exec-a1b2c3 architecture
Resolviendo candidato... opencode (openai/gpt-4o) ✓
Inyectando contexto: instrucciones + 2 skills
Ejecutando en modo no interactivo...

Respuesta del agente:
  [proyección sanitizada de la respuesta de OpenCode]

$ shardeo step complete exec-a1b2c3 architecture
Validando artefactos:
  ✓ architecture.md
  ✓ adr.md
Step completado.
```

---

## Spec 5: Grafo de ejecución y resolución de paralelismo

status: pending

### Objetivo

Permitir que Shardeo resuelva el orden de ejecución de los steps basándose en el grafo definido por `depends_on`, y que informe al orquestador cuándo hay steps que pueden ejecutarse en paralelo.

### Descripción funcional

Shardeo construye un grafo dirigido acíclico (DAG) a partir de las relaciones `depends_on` entre steps. Cada propiedad tiene una sola responsabilidad: `depends_on` define orden, `requires` valida entradas, `produces` valida salidas. Solo `depends_on` participa en la construcción del grafo.

Al iniciar un workflow (`shardeo run`), Shardeo ejecuta tres validaciones sobre el grafo antes de registrar la ejecución:

1. Que exista al menos un step sin `depends_on` (punto de entrada del workflow).
2. Que no haya ciclos (A después de B, B después de A).
3. Que todo ID referenciado en `depends_on` corresponda a un step existente en el workflow.

Cuando el orquestador consulta `shardeo steps next`, Shardeo retorna todos los steps cuyos predecessores (`depends_on`) se han completado. Si hay múltiples steps disponibles simultáneamente, los retorna todos. La decisión de ejecutarlos en paralelo o secuencialmente es del orquestador. Shardeo no ejecuta nada en paralelo por sí mismo.

### Criterios de aceptación

- `shardeo run` valida el grafo al iniciar. Si no hay ningún step sin `depends_on`, rechaza la ejecución indicando que no hay punto de entrada.
- Si detecta un ciclo, rechaza la ejecución con un mensaje que identifica los steps involucrados.
- Si un `depends_on` referencia un step ID que no existe en el workflow, rechaza la ejecución con el código `invalid_depends_on_reference` e indica la referencia inválida.
- `shardeo steps next` retorna múltiples steps cuando sus predecessores (`depends_on`) se han completado simultáneamente (e.g., `api-design` y `database-design` después de completar `architecture`).
- Un step con `depends_on: [api-design, database-design]` solo aparece como disponible cuando ambos se han completado.
- Steps sin `depends_on` aparecen como disponibles inmediatamente al iniciar el workflow.
- La respuesta de `steps next` incluye metadata suficiente para que el orquestador decida: id del step, tipo, y lista de agentes configurados.
- La validación de `requires` (existencia de artefactos) ocurre al momento de ejecutar el step (`step run`), no al resolver el grafo.

### Ejemplo de flujo de usuario

```
$ shardeo run backend-feature
Validando grafo de ejecución... ✓
Ejecución iniciada: exec-x7y8z9

$ shardeo steps next exec-x7y8z9
Steps disponibles:
  - architecture (agent) — sin predecessores

  [orquestador completa 'architecture']

$ shardeo steps next exec-x7y8z9
Steps disponibles:
  - api-design (agent) — depends_on: architecture ✓
  - database-design (agent) — depends_on: architecture ✓

  [ambos pueden ejecutarse en paralelo]
```

---

## Spec 6: Re-ejecución de steps con contexto acumulado

status: pending

### Objetivo

Permitir que un step de tipo `agent` se ejecute múltiples veces antes de ser marcado como completado, entregando al agente el contexto de intentos anteriores y el feedback del usuario para que pueda iterar sobre su propio trabajo.

### Descripción funcional

La Spec 4 define completamente el fallback automático entre candidatos dentro de un mismo `step run`, incluido su contexto acumulado, evidencia sanitizada y límites. Esta spec no modifica ese comportamiento. Añade la re-ejecución manual posterior de un step no completado y el parámetro `--feedback`.

El feedback se envía como parámetro del comando: `shardeo step run <execution-id> <step-id> --feedback "El ADR no incluye alternativas evaluadas"`.

Cada invocación de un CLI se registra en la base de datos como un intento separado, vinculado al step y la ejecución. Esto incluye las invocaciones realizadas automáticamente por fallback. Cada intento almacena: número de intento, respuesta completa o parcial del agente, feedback recibido, artefactos generados, CLI utilizado y motivo de finalización.

### Criterios de aceptación

- Un step no completado puede re-ejecutarse con `shardeo step run` sin error.
- En la re-ejecución, el contexto entregado al CLI incluye los artefactos generados en intentos anteriores.
- Si se provee `--feedback`, el texto se incluye en el contexto entregado al agente, claramente delimitado de las instrucciones.
- Cada intento se registra como una entrada separada en la base de datos con: timestamp, número de intento, respuesta completa o parcial, feedback, artefactos generados, CLI utilizado y motivo de finalización.
- Los artefactos generados en una re-ejecución sobrescriben los de intentos anteriores en el filesystem (`.shardeo/artifacts/`), pero los anteriores permanecen registrados en la base de datos.
- `shardeo step complete` valida los artefactos del último intento, independientemente de cuántos intentos haya habido.

### Ejemplo de flujo de usuario

```
$ shardeo step run exec-x7y8z9 architecture
[...respuesta del agente...]

$ shardeo step complete exec-x7y8z9 architecture
✗ Artefacto faltante: adr.md

$ shardeo step run exec-x7y8z9 architecture --feedback "Falta el ADR. Debe incluir alternativas evaluadas y justificación de la decisión."
Intento #2
Contexto: instrucciones + skills + architecture.md (intento anterior) + feedback
[...respuesta del agente...]

$ shardeo step complete exec-x7y8z9 architecture
✓ architecture.md
✓ adr.md
Step completado.
```

---

## Spec 7: Persistencia de ejecuciones y reanudación de workflows

status: done

### Objetivo

Permitir que un workflow interrumpido (por error, cierre de terminal o cancelación) pueda retomarse desde el último estado conocido, sin perder el trabajo ya realizado.

### Descripción funcional

Shardeo persiste toda la metadata de ejecución en una base de datos SQLite embebida. Cuando el usuario ejecuta `shardeo resume <execution-id>`, Shardeo consulta la base de datos, identifica qué steps se completaron, verifica que sus artefactos sigan presentes en el filesystem, y retoma el workflow desde ese punto.

El estado general persistido de la ejecución debe mantenerse sincronizado con las transiciones de sus steps. Cuando todos los steps del workflow se han completado, Shardeo persiste `executions.status = completed`; `shardeo status` no puede limitarse a calcular y mostrar `completed` mientras la fila de la ejecución permanece en `running`. La actualización debe ser atómica con la transición que completa el workflow o idempotente y recuperable si se realiza inmediatamente después.

Si un artefacto fue completado en la base de datos pero su archivo no existe en `.shardeo/artifacts/`, Shardeo marca el step como "requiere reconstrucción" y entrega la respuesta anterior del agente como contexto para que el orquestador pueda reconstruirlo.

`shardeo status <execution-id>` muestra el estado completo de la ejecución incluyendo: steps completados (con timestamps y número de intentos), steps fallidos (con el error), steps pendientes y estado general del workflow.

### Criterios de aceptación

- Toda ejecución de workflow persiste en SQLite: workflow, fecha de inicio, estado general.
- El estado general almacenado se actualiza después de cada transición que pueda cambiar el estado agregado del workflow.
- Cuando se completa el último step pendiente, `executions.status` queda persistido como `completed`; la respuesta de `shardeo status` y la fila almacenada no pueden divergir.
- Cada step ejecutado persiste: estado, timestamps, número de intentos, CLI utilizado.
- Cada intento persiste: respuesta del agente, feedback, artefactos generados, métricas (duración).
- `shardeo resume <execution-id>` retoma la ejecución sin re-ejecutar steps ya completados.
- Si un artefacto completado no existe en el filesystem, `resume` marca el step correspondiente para reconstrucción y entrega la respuesta del agente anterior como contexto.
- `shardeo status <execution-id>` muestra un resumen completo con estados, timestamps y número de intentos por step.
- Si el `execution-id` no existe, `resume` y `status` fallan con un mensaje claro.
- Un workflow cuyo último step se completó aparece con estado "completado" en `status`.

### Ejemplo de flujo de usuario

```
$ shardeo status exec-x7y8z9
Workflow: backend-feature
Estado: en progreso
Inicio: 2026-07-05 10:30:00

Steps:
  ✓ architecture — completado (2 intentos, 10:32)
  ✓ api-design — completado (1 intento, 10:45)
  ✓ database-design — completado (1 intento, 10:44)
  ✗ lint — fallido (exit code 1, 10:50)
  ○ implementation — pendiente

$ shardeo resume exec-x7y8z9
Retomando ejecución exec-x7y8z9...
Steps completados: 3/5
Verificando artefactos... ✓

$ shardeo steps next exec-x7y8z9
Steps disponibles:
  - lint (command) — último intento falló, puede re-ejecutarse
```

---

## Spec 8: Control de iteración dirigido por el orquestador

status: pending

### Objetivo

Permitir que el agente orquestador reabra trabajo completado u omita trabajo pendiente en respuesta al resultado de otros steps o a una decisión del usuario, sin incorporar condiciones de negocio ni decisiones metodológicas dentro de Shardeo.

### Descripción funcional

El grafo definido por `depends_on` permanece estático. El orquestador interpreta las instrucciones del workflow y las respuestas de los steps, y solicita a Shardeo transiciones explícitas cuando necesita iterar o tomar una rama opcional.

Si un step posterior detecta que el resultado de un predecessor debe corregirse, el orquestador ejecuta:

```
shardeo step reopen <execution-id> <step-id> --cascade --feedback "<motivo>"
```

Shardeo conserva los intentos anteriores, reabre el step, invalida su generación completada y reinicia el estado de los descendientes afectados. El feedback queda asociado a la reapertura y se incluye en el contexto del siguiente intento del step.

Los artefactos de una generación invalidada permanecen en el filesystem y en el historial para auditoría, pero no satisfacen `requires` ni permiten completar nuevamente el step de forma inmediata. Después de que finalice con éxito al menos un nuevo intento, `step complete` vuelve a validar el conjunto actual de artefactos declarados en `produces` y registra una nueva generación válida. Un artefacto puede conservar los mismos bytes si no necesitaba cambios; la validez pertenece a la nueva generación del step y no exige reescrituras artificiales de cada archivo.

Si las instrucciones del workflow indican que un step pendiente no es necesario, el orquestador ejecuta:

```
shardeo step skip <execution-id> <step-id> --reason "<motivo>"
```

El step pasa a estado `skipped`. Este estado se considera terminal al resolver dependencias `depends_on`, pero no genera artefactos ni satisface validaciones `requires`.

Shardeo no evalúa expresiones condicionales ni interpreta respuestas de agentes. La decisión de reabrir u omitir un step pertenece exclusivamente al orquestador; Shardeo valida y persiste la transición solicitada.

### Criterios de aceptación

- `shardeo step reopen <execution-id> <step-id> --cascade --feedback <texto>` permite reabrir un step completado o fallido.
- La reapertura conserva todos los intentos anteriores, sus respuestas, artefactos, feedback, timestamps y CLI utilizado.
- El step reabierto vuelve a estar disponible para ejecución cuando sus predecessors originales permanecen completados o `skipped`.
- El feedback de la reapertura se almacena y se incluye claramente delimitado en el contexto del siguiente intento.
- `--cascade` reinicia a estado pendiente todos los descendientes que estuvieran en progreso, completados o fallidos, invalida sus generaciones completadas y conserva su historial de intentos.
- Los artefactos de una generación invalidada no pueden satisfacer `requires` ni permitir `step complete` hasta que finalice con éxito al menos un nuevo intento del step y el conjunto actual de `produces` sea revalidado como una nueva generación. Los archivos que no requerían cambios pueden conservar sus bytes.
- Si la reapertura afecta descendientes y no se proporciona `--cascade`, el comando falla e informa qué steps serían invalidados.
- No se puede reabrir un step pendiente, en ejecución o `skipped`; el comando falla indicando su estado actual.
- `shardeo step skip <execution-id> <step-id> --reason <texto>` permite omitir un step pendiente que todavía no tenga intentos iniciados.
- No se puede omitir un step en ejecución, completado o fallido sin reabrir o resolver primero su estado actual.
- Un step `skipped` se considera terminal para resolver dependencias `depends_on`.
- Omitir un step no crea sus artefactos. Cualquier step posterior que los declare en `requires` falla normalmente al validar sus entradas.
- Tanto `reopen` como `skip` registran el motivo, actor, timestamp y transición de estado en la base de datos.
- `shardeo status <execution-id>` muestra steps reabiertos u omitidos, sus motivos y el historial de transiciones.
- `shardeo steps next <execution-id>` recalcula los steps disponibles después de cada reapertura u omisión.
- Las transiciones son rechazadas si el `execution-id` o el `step-id` no existen.

### Ejemplo de flujo de usuario

```
$ shardeo status exec-x7y8z9
Steps:
  ✓ implementation — completado (intento #1)
  ✗ verification — fallido: criterio AC-3 no cumplido

$ shardeo step reopen exec-x7y8z9 implementation \
    --cascade \
    --feedback "Verification failed: criterion AC-3 is not satisfied"

Step reabierto: implementation
Descendientes reiniciados:
  - verification

$ shardeo steps next exec-x7y8z9
Steps disponibles:
  - implementation (agent) — intento #2

$ shardeo step run exec-x7y8z9 implementation
Contexto: instrucciones + skills + artefactos anteriores + feedback de reapertura
```

---

## Spec 9: Superficies de ejecución e interacción gestionada para steps agent

status: implemented

### Objetivo

Definir una arquitectura multi-harness y neutral respecto del proveedor para
ejecutar steps `agent` en tres superficies distintas: `headless`, `supervised` y
`terminal`. Shardeo debe poder mediar permisos estructurados sin acoplar el core
a un protocolo concreto, entregar el control exclusivo de una terminal a una
persona cuando corresponda, entregar a cada intento gestionado un contexto
reproducible sin depender de los límites de argumentos del sistema operativo y
mantener separados los eventos de control de bajo volumen y el output completo.

Esta spec reemplaza la propuesta futura y no implementada de v2 basada en
`interactive: boolean`, antes documentada en AGENTS.md; no reemplaza el
comportamiento implementado de la Spec 4 en `headless`, salvo la migración
explícita del resultado de inactividad (ver §8). Se implementó en tres rebanadas
entregadas en `main`: `01-bundle-manifiesto-sondeo` (9a: bundle, manifiesto,
sondeo, resolución de modo, transporte, contención), `02-supervised-events` (9b:
supervisor, lease con fencing, IPC, broker de interacciones, eventos, output,
política de permisos, timeouts) y `03-terminal-pty` (9c: PTY adjuntable,
attach/reattach, timeout de presencia humana y handoff). Los contratos de
adapters, comandos y eventos aquí documentados son la autoridad de
comportamiento de los modos `supervised` y `terminal`; en `headless`, la Spec 4
sigue siendo la autoridad para la ruta no interactiva salvo la migración
documentada en §8.

### Resumen de decisiones

| Tema | Decisión |
|---|---|
| Superficies | Los modos son `headless`, `supervised` y `terminal`; no son variantes intercambiables de una misma sesión interactiva. |
| Herencia | `step.mode > workflow.mode > config.yaml defaults.agent_mode`; el valor por defecto es `headless`. |
| Override | `shardeo step run ... --mode <mode>` sobrescribe la configuración solo para ese intento. |
| Integración | Cada harness se integra mediante un adapter que sondea capacidades reales en tiempo de ejecución. |
| Contexto por intento | El core crea y congela un `context bundle` inmutable por intento gestionado, con un manifiesto neutral respecto del proveedor que conserva orden, límites y digests. |
| Transporte de contexto | El adapter selecciona un transporte anunciado y seguro. El contrato del core es el manifiesto, nunca una sintaxis del proveedor como `@path`. |
| Carga diferida | Referenciar contenido evita límites de tamaño del transporte y permite carga bajo demanda; todo contenido cargado sigue consumiendo tokens del modelo. |
| Supervisión | `supervised` usa un supervisor local en background que mantiene la sesión y media interacciones estructuradas. |
| Terminal | `terminal` crea una terminal adjuntable controlada únicamente por una persona. El orquestador no maneja ni interpreta su pantalla. |
| Comandos gestionados | En `supervised` y `terminal`, `step run` inicia el intento y retorna de inmediato; el seguimiento se realiza con `step events`, `step status` y `step output`. |
| Permisos | `step approve` resuelve un `interaction_id` normalizado usando solo una decisión anunciada por el adapter. |
| Coordinación | La señalización en vivo usa comunicación local explícita entre procesos. SQLite conserva estado y auditoría, pero no funciona como bus de mensajes. |
| Fallo seguro | Una capacidad ausente produce `unsupported_capability`; Shardeo no cambia de modo ni amplía permisos silenciosamente. |

### Terminología

| Término | Significado en esta spec |
|---|---|
| Spec | `Specification` (especificación: contrato documentado de comportamiento y criterios de aceptación). |
| Harness | Producto o runtime de agente que ejecuta el trabajo, por ejemplo OpenCode hoy y otros harnesses en el futuro. |
| Adapter | Límite de integración que conoce el lanzamiento, el sondeo y los protocolos nativos de un harness. |
| CLI | `Command-Line Interface` (interfaz de línea de comandos: programa operado mediante comandos de texto). Es una posible superficie de un harness, no el contrato del core. |
| API | `Application Programming Interface` (interfaz de programación de aplicaciones: contrato estructurado entre programas). |
| SDK | `Software Development Kit` (kit de desarrollo de software: biblioteca y herramientas que un proveedor ofrece para integraciones). |
| PTY | `Pseudoterminal` (pseudoterminal: par de dispositivos que proporciona semántica real de terminal a un proceso). |
| TUI | `Terminal User Interface` (interfaz de usuario en terminal: pantalla interactiva destinada a una persona). |
| IPC | `Inter-Process Communication` (comunicación entre procesos: canal local para enviar comandos al supervisor activo). |
| Supervisor | Proceso local en background que posee una sesión gestionada, normaliza eventos y aplica resoluciones mediante el adapter. |
| Interacción | Solicitud estructurada del harness que requiere una decisión o una acción externa. El mínimo soportado es `permission`. |
| Evento de control | Evento normalizado y de bajo volumen que describe cambios semánticos del intento. No contiene el output completo. |
| Cursor | Identificador opaco y persistido que permite continuar el polling de eventos sin perderlos ni procesarlos dos veces. |
| Context bundle | Paquete de contexto: copia inmutable y contenida de los materiales autorizados para un intento, junto con su manifiesto. |
| Manifiesto de contexto | Contrato neutral que identifica el bundle y describe cada entrada sin imponer cómo debe admitirla un harness. |
| Admisión de contexto | Prueba producida por el adapter de que las entradas obligatorias quedaron disponibles para la sesión mediante el transporte seleccionado. |
| SHA-256 | `Secure Hash Algorithm 256-bit` (algoritmo de hash seguro de 256 bits: digest criptográfico usado para verificar identidad e integridad). |

Los tipos de interacción forman un contrato extensible. Esta spec define el flujo
completo para `permission` y reserva `question`, `authentication` y
`terminal_handoff` para adapters que anuncien soporte explícito. Declarar un tipo
no implica que Shardeo ya pueda resolverlo.

### Arquitectura y responsabilidades

#### Core de Shardeo

El core valida la configuración, resuelve el modo efectivo, selecciona el
adapter, construye y verifica el bundle de contexto, aplica políticas sobre datos
normalizados, persiste el estado y expone los comandos públicos. No conoce
endpoints, flags, eventos, sintaxis de referencias ni valores de respuesta
propios de un proveedor.

#### Harness adapter

El adapter encapsula todo comportamiento específico del harness:

- Sondea en runtime las capacidades de la instalación seleccionada.
- Lanza el proceso o la sesión con el mecanismo adecuado para el modo.
- Selecciona un transporte de contexto anunciado, admite las entradas del
  manifiesto y devuelve evidencia verificable de disponibilidad.
- Convierte eventos nativos en interacciones y eventos normalizados.
- Traduce una decisión normalizada anunciada en `available_decisions` al
  protocolo nativo.
- Declara si puede reanudar o reconciliar una sesión después de una caída.
- Expone únicamente metadatos de política que puede mapear sin ambigüedad.

El core no presupone que un harness emita `JSONL (JSON Lines, basado en
JavaScript Object Notation: formato de un objeto estructurado por línea)` ni que
acepte decisiones por entrada estándar. Tampoco codifica versiones de fixtures,
flags de comandos o respuestas específicas del proveedor.

El lanzamiento es responsabilidad del adapter. Debe preferir la ejecución
directa con un `argv (argument vector: lista estructurada de argumentos entregada
al proceso)` y evitar interpolación de shell. Si
un harness requiere un shell, el adapter debe justificarlo y escapar de forma
segura los valores no confiables. Activar un shell no crea una PTY; el modo
`terminal` debe asignar una PTY real de forma explícita.

#### Bundle y manifiesto de contexto

Para cada intento `supervised` o `terminal`, el supervisor, como componente del
core, materializa un bundle antes de iniciar la sesión del proveedor. El bundle
contiene copias por intento, no referencias mutables a los archivos vivos del
proyecto. Una vez calculados sus digests y escrito el manifiesto, ambos quedan
congelados. Esto evita enviar el contexto combinado como un único argumento,
superar límites del sistema operativo y perder reproducibilidad si un archivo
cambia durante el intento.

El manifiesto preserva el orden semántico y las fronteras exigidos por la Spec 4:
instrucciones operacionales, instrucciones de dominio, skills, artefactos
requeridos y contexto previo o de fallback cuando corresponda. El campo `order`
define ese orden total y cada entrada conserva su propio archivo. Un adapter no
puede concatenar, reordenar ni omitir entradas de forma que altere esas
fronteras. Los valores de `role` son identificadores lógicos estables y neutrales
respecto del adapter; un transporte puede mapearlos, pero no reinterpretarlos.

Ejemplo neutral respecto del proveedor:

```json
{
  "schema_version": 1,
  "bundle_id": "bundle-attempt-7",
  "execution_id": "exec-abc",
  "step_id": "architecture",
  "attempt_id": "attempt-7",
  "created_at": "2026-07-26T12:00:00.000Z",
  "created_by": "shardeo",
  "total_bytes": 42071,
  "entries": [
    {
      "order": 10,
      "role": "operational_instructions",
      "relative_path": "entries/010-operational.md",
      "required": true,
      "loading": "eager",
      "bytes": 2841,
      "sha256": "d4b5a901d4b5a901d4b5a901d4b5a901d4b5a901d4b5a901d4b5a901d4b5a901"
    },
    {
      "order": 20,
      "role": "domain_instructions",
      "relative_path": "entries/020-domain.md",
      "required": true,
      "loading": "eager",
      "bytes": 4970,
      "sha256": "8c215e3f8c215e3f8c215e3f8c215e3f8c215e3f8c215e3f8c215e3f8c215e3f"
    },
    {
      "order": 30,
      "role": "skill",
      "relative_path": "entries/030-skill-hexagonal.md",
      "required": true,
      "loading": "on_demand",
      "bytes": 6230,
      "sha256": "472e81bc472e81bc472e81bc472e81bc472e81bc472e81bc472e81bc472e81bc"
    },
    {
      "order": 40,
      "role": "required_artifact",
      "relative_path": "entries/040-architecture.md",
      "required": true,
      "loading": "on_demand",
      "bytes": 18720,
      "sha256": "779a0c12779a0c12779a0c12779a0c12779a0c12779a0c12779a0c12779a0c12"
    },
    {
      "order": 50,
      "role": "prior_attempt_context",
      "relative_path": "entries/050-prior-attempt.md",
      "required": false,
      "loading": "on_demand",
      "bytes": 9310,
      "sha256": "f13c721df13c721df13c721df13c721df13c721df13c721df13c721df13c721d"
    }
  ]
}
```

Las instrucciones operacionales y de dominio son obligatorias y `eager`. Los
skills requeridos y artefactos requeridos son referencias obligatorias: antes de
que comience el trabajo, el adapter debe probar que la sesión puede obtener los
bytes exactos identificados por el manifiesto. El material opcional o previo se
carga `on_demand`. `on_demand` difiere la transferencia de bytes, pero no vuelve
opcional una entrada con `required: true` ni permite al agente omitir material
exigido por el contrato. Si el transporte no puede probar la disponibilidad de
todas las entradas obligatorias, el intento falla cerrado antes del trabajo del
proveedor.

El bundle reduce el tamaño del transporte inicial y permite no cargar material
opcional innecesario. No reduce por sí mismo el uso de tokens: cuando una entrada
se inyecta o el agente la lee, su contenido consume tokens del modelo como
cualquier otro contexto.

El core valida la prueba de admisión contra `bundle_id`, digest del manifiesto y
digests de las entradas obligatorias antes de permitir trabajo del proveedor. La
prueba demuestra disponibilidad e identidad de bytes, no que el contenido sea
gratuito para el modelo ni que una entrada obligatoria pueda ignorarse.

#### Supervisor de intentos gestionados

Cada intento `supervised` o `terminal` tiene un único supervisor propietario. El
supervisor mantiene el handle del proceso o de la sesión, captura output, recibe
comandos por IPC, renueva su lease y publica eventos de control persistidos.
`step run` espera un handshake de readiness que confirma bundle congelado,
contexto admitido y sesión iniciada; después retorna.

#### Broker de interacción

El broker normaliza solicitudes nativas, asigna un `interaction_id`, registra
`available_decisions` y aplica una resolución una sola vez. La interacción
normalizada preserva el mínimo necesario para decidir sin exponer contratos
internos del proveedor como parte de la API pública de Shardeo.

#### Almacén de estado y output

SQLite conserva intentos, sesiones, leases, interacciones, decisiones, cursores
y auditoría. Un almacén de archivos contenido bajo la raíz de runtime conserva
el output completo por intento. Ninguno reemplaza el canal IPC usado para
señalización en tiempo real.

#### Relación con Traycer

La arquitectura toma de Traycer únicamente la separación entre harnesses y el
patrón de interacción mediada. No asume que Traycer limite OpenCode a una TUI ni
depende de detalles privados de su implementación.

### Sondeo de capacidades

El adapter produce un manifiesto de capacidades después de consultar la
instalación real que se utilizará. El sondeo puede validar comandos, endpoints,
negociación de protocolo o callbacks, pero no puede inferir soporte solo desde
una versión semántica declarada.

Forma ilustrativa en TypeScript:

```ts
type ExecutionMode = "headless" | "supervised" | "terminal";
type ContextTransport =
  | "direct_injection"
  | "file_reference"
  | "api_attachment"
  | "tool_read";
type InteractionKind =
  | "permission"
  | "question"
  | "authentication"
  | "terminal_handoff";

type DecisionScope = "request" | "session" | "resource";

interface ModeCapability {
  supported: boolean;
  reason?: string;
  interaction_kinds: InteractionKind[];
  decision_scopes: DecisionScope[];
}

interface ContextTransportCapability {
  supported: boolean;
  modes: ExecutionMode[];
  loading: Array<"eager" | "on_demand">;
  max_total_bytes: number;
  max_entry_bytes: number;
  proves_required_availability: boolean;
  verifies_sha256: boolean;
  scoped_read_permission: "supported" | "unsupported" | "not_applicable";
  reason?: string;
}

interface ContextAdmissionReceipt {
  bundle_id: string;
  manifest_sha256: string;
  transport: ContextTransport;
  admitted_required_entries: Array<{
    relative_path: string;
    sha256: string;
  }>;
}

interface HarnessCapabilityManifest {
  adapter_id: string;
  harness_identity: string;
  probed_at: string;
  modes: Record<ExecutionMode, ModeCapability>;
  context_transports: Record<
    ContextTransport,
    ContextTransportCapability
  >;
  policy_capabilities: string[];
  session_resume: "supported" | "unsupported";
  interaction_reconciliation: "supported" | "unsupported";
}

interface AvailableDecision {
  id: string;
  effect: "allow" | "deny";
  scope: DecisionScope;
}

interface NormalizedInteraction {
  interaction_id: string;
  kind: InteractionKind;
  summary: string;
  permission_capability?: string;
  resource?: Record<string, unknown>;
  available_decisions: AvailableDecision[];
}
```

El manifiesto de capacidades describe soporte, no autorización. Que un adapter
soporte una decisión de alcance `session` no autoriza a Shardeo a seleccionarla.
Cada interacción anuncia el subconjunto válido en `available_decisions`.

El adapter elige un transporte que soporte el modo efectivo, las estrategias de
carga y los límites reales del bundle. También debe poder verificar SHA-256 y
probar la disponibilidad de entradas obligatorias. `direct_injection` es válido
solo si el payload cabe de forma segura; `file_reference`, `api_attachment` y
`tool_read` pueden diferir la carga, pero no relajan la admisión ni la política de
lectura. Si ninguna combinación satisface el manifiesto, Shardeo falla con
`unsupported_capability` antes de iniciar trabajo del proveedor.

Si el modo solicitado no está soportado por el adapter y la instalación
seleccionados, `step run` falla con `unsupported_capability` antes de lanzar el
proceso del harness. El error incluye `adapter_id`, `harness_identity`,
`requested_mode` y una razón sanitizada. No existe fallback silencioso a
`terminal`, a otro modo ni a permisos más amplios.

### Mapeos de adapters no normativos

Estos ejemplos explican cómo podrían implementarse adapters concretos; no forman
parte del contrato del core:

- OpenCode puede usar un servidor gestionado, un stream estructurado mediante
  `SSE (Server-Sent Events: eventos enviados por el servidor sobre una conexión
  persistente)` y una respuesta de permiso mediante
  `HTTP (Hypertext Transfer Protocol: protocolo de comunicación para recursos y
  operaciones web)`.
- Codex puede usar su app-server y
  `JSON-RPC (JavaScript Object Notation Remote Procedure Call: protocolo de
  llamadas remotas con mensajes estructurados)` para solicitudes y respuestas de
  aprobación.
- Un harness futuro puede usar callbacks de un SDK, hooks u otro protocolo
  estructurado.
- Para contexto, OpenCode puede resolver entradas mediante referencias `@path` o
  una sesión gestionada; otro harness puede usar attachments nativos, payloads de
  API o SDK, lecturas mediante tools, o inyección directa.

El adapter debe ocultar nombres de eventos, rutas, métodos y valores de respuesta
nativos. Ninguno de estos ejemplos obliga a otros adapters a reproducir el mismo
transporte. En particular, `@path` no aparece en el manifiesto ni en el core: es
solo una posible traducción interna del adapter de OpenCode.

Ejemplo no normativo para una terminal de OpenCode. El directorio de trabajo
permanece en el proyecto o worktree asignado; el bundle vive bajo una raíz de
runtime contenida y se referencia mediante un path relativo:

```bash
# Inicio directo de la TUI con un bootstrap corto creado por el adapter.
opencode <project-or-worktree> \
  --prompt 'Carga @.shardeo/runtime/attempt-7/context/manifest.json y sigue el orden declarado.'

# Preferido: attach de la TUI a una sesión ya creada en un servidor gestionado.
opencode attach <managed-server-url>
```

En la segunda variante, el adapter adjunta previamente el manifiesto mediante la
API o el SDK de la sesión y abre la TUI contra el servidor gestionado; selecciona
la sesión exacta mediante una capacidad nativa comprobada o falla con
`unsupported_capability`, sin pedir a la persona que la infiera. No vuelve a
transportar el contexto en el prompt. Es la opción preferida cuando la sesión ya
existe. Los comandos son ilustrativos y no convierten flags ni referencias de
OpenCode en contrato normativo de Shardeo.

### Resolución del modo

La configuración declarativa usa una única propiedad `mode`:

```text
step.mode > workflow.mode > config.yaml defaults.agent_mode
```

Si ninguna capa la declara, el modo efectivo es `headless`. El flag `--mode`
sobrescribe el valor efectivo únicamente para el intento iniciado por ese
comando.

```yaml
# config.yaml
defaults:
  agent_mode: headless

# workflow.yaml
name: backend-feature
mode: supervised
steps:
  - id: architecture
    type: agent
    mode: terminal
    instructions: steps/architecture/instructions.md
```

Los steps `command` no usan `agent_mode` ni aceptan estas superficies.

### Comportamiento por modo

| Aspecto | `headless` | `supervised` | `terminal` |
|---|---|---|---|
| Control principal | Proceso invocado por `step run` | Supervisor de Shardeo | Persona conectada a una PTY gestionada |
| Retorno de `step run` | Bloquea hasta terminar, como en Spec 4 | Retorna tras iniciar el supervisor | Retorna tras iniciar el supervisor y preparar la PTY |
| Interacción | No se resuelve en vivo si el adapter no puede hacerlo | Eventos estructurados y decisiones mediadas | Interacción humana nativa en terminal |
| Permiso no resoluble | Termina con `permission_required` | Queda en `awaiting_interaction` | Lo decide la persona dentro del harness |
| Uso por el orquestador | Recibe resultado final | Consulta eventos y emite decisiones explícitas | Solo solicita el modo y comunica el handoff |
| Pantalla de terminal | No requerida | No requerida | Nunca se controla ni se interpreta por un agente |
| Contexto | Inyección directa de Spec 4 mientras siga siendo segura y soportada | Bundle congelado y admitido antes de abrir la sesión | Bundle congelado y bootstrap del adapter antes de `awaiting_human` |
| Output completo | Evidencia y proyección según Spec 4 | Buffer en disco, consultable | Buffer en disco, consultable |

#### Modo `headless`

`headless` conserva el comportamiento de la Spec 4 salvo la migración explícita
del resultado de inactividad descrita en esta spec. `step run` permanece
bloqueante y el adapter ejecuta la ruta no interactiva existente. Cuando detecta
una solicitud que no puede resolver mediante una política soportada, termina el
intento con `permission_required`. Desde la implementación de Spec 9, una
expiración por falta de actividad pasa de `permission_timeout` a
`process_inactivity_timeout`. Esta spec no
convierte fixtures de OpenCode en un mecanismo de supervisión ni cambia las reglas
de fallback de v1. El adapter implementado puede continuar inyectando directamente
el contexto combinado por entrada estándar cuando sea seguro y esté soportado.
Adoptar bundles para `supervised` y `terminal` no cambia silenciosamente esa ruta.
Un adapter `headless` solo puede usar bundles después de anunciar y validar
soporte explícito para el transporte elegido.

#### Modo `supervised`

1. `step run` resuelve configuración, adapter y modo.
2. El adapter sondea capacidades de la instalación seleccionada.
3. Shardeo crea la identidad del intento y adquiere un lease de propietario.
4. Shardeo inicia el supervisor.
5. El supervisor crea y congela el bundle del intento, verifica sus límites e
   integridad y persiste su identidad.
6. El adapter selecciona un transporte anunciado, verifica los digests y devuelve
   prueba de que todas las entradas obligatorias quedaron disponibles.
7. Solo entonces el supervisor abre la sesión nativa y comienza a capturar el
   output completo; el handshake de readiness confirma estos pasos.
8. `step run` retorna `attempt_id`, `session_id`, estado, identidad del bundle,
   transporte y cursor inicial.
9. El adapter recibe una solicitud nativa y la normaliza como
   `interaction_required`.
10. El supervisor persiste el evento y pasa a `awaiting_interaction` sin cerrar la
   sesión.
11. El orquestador consulta `step events` y selecciona una decisión incluida en
   `available_decisions`.
12. `step approve` envía la resolución al supervisor por IPC.
13. El supervisor realiza una resolución compare-and-set idempotente, pide al
    adapter traducirla y publica `interaction_resolved`.

Una resolución no escribe bytes genéricos en la entrada estándar. El adapter usa
la API, el SDK o el protocolo estructurado que anunció durante el sondeo.

#### Modo `terminal`

1. `step run` crea la identidad del intento, un supervisor y una PTY adjuntable en
   background.
2. El supervisor crea y congela el bundle antes de iniciar la sesión del
   proveedor.
3. El adapter verifica los digests y admite el contexto mediante un mecanismo
   nativo o inicia el harness con una instrucción bootstrap corta que apunta al
   manifiesto y preserva el orden y la obligatoriedad declarados.
4. Solo después de esa admisión el harness queda activo dentro de la PTY y el
   supervisor pasa a `awaiting_human`.
5. `step run` retorna la identidad del intento, la identidad del bundle y el
   comando exacto de attach.
6. El orquestador muestra el handoff; no escribe el bootstrap, no ejecuta teclas,
   no captura la pantalla y
   no intenta interpretar la TUI.
7. La persona abre otra terminal y ejecuta
   `shardeo step attach <execution-id> <step-id> --attempt-id <attempt-id>`.
8. Al desconectarse, el proceso hijo continúa vivo y el supervisor vuelve a
   `awaiting_human`. La persona puede ejecutar el mismo comando para reattach
   mientras el intento siga activo y no expire el plazo sin presencia humana.
9. Al terminar el harness, el supervisor cierra la PTY, persiste el resultado y
   publica el evento terminal del intento.

El modo portable requiere otra terminal porque el harness orquestador puede
seguir abierto. Suspender la terminal actual, integrar Tmux o Zellij, o instalar
plugins de harness son mejoras futuras de experiencia de usuario, no parte del
contrato central.

### Contrato de comandos y eventos

Los comandos gestionados usan siempre la identidad retornada por `step run`. Los
ejemplos siguientes pertenecen al mismo intento.

```bash
$ shardeo step run exec-abc architecture --mode supervised
{"type":"attempt_started","execution_id":"exec-abc","step_id":"architecture","attempt_id":"attempt-7","session_id":"session-7","mode":"supervised","state":"running","event_cursor":"event-0","context_bundle":{"bundle_id":"bundle-attempt-7","manifest_sha256":"a45192efa45192efa45192efa45192efa45192efa45192efa45192efa45192ef","transport":"api_attachment","required_entries_admitted":4}}

$ shardeo step events exec-abc architecture --attempt-id attempt-7 --after event-0 --limit 50
{"events":[{"cursor":"event-1","type":"interaction_required","interaction":{"interaction_id":"interaction-42","kind":"permission","summary":"Execute package installation","permission_capability":"process.execute","resource":{"command":"pnpm install"},"available_decisions":[{"id":"allow_once","effect":"allow","scope":"request"},{"id":"deny","effect":"deny","scope":"request"}]}}],"next_cursor":"event-1","has_more":false}

$ shardeo step status exec-abc architecture --attempt-id attempt-7
{"attempt_id":"attempt-7","state":"awaiting_interaction","pending_interaction_id":"interaction-42","event_cursor":"event-1"}

$ shardeo step output exec-abc architecture --attempt-id attempt-7 --tail 100
[proyección sanitizada de las últimas 100 líneas]

$ shardeo step approve exec-abc architecture --interaction-id interaction-42 --decision allow_once
{"type":"interaction_resolved","interaction_id":"interaction-42","decision":"allow_once","actor":"orchestrator","state":"running","cursor":"event-2"}

$ shardeo step events exec-abc architecture --attempt-id attempt-7 --after event-1 --limit 50
{"events":[{"cursor":"event-2","type":"interaction_resolved","interaction_id":"interaction-42","decision":"allow_once"},{"cursor":"event-3","type":"attempt_completed","exit_code":0}],"next_cursor":"event-3","has_more":false}

$ shardeo step output exec-abc architecture --attempt-id attempt-7 --full
[stream del output completo retenido y sanitizado]
```

`--after` es exclusivo: retorna eventos posteriores al cursor indicado. Repetir
la consulta con el mismo cursor retorna el mismo conjunto estable dentro de la
retención. El consumidor guarda `next_cursor` solo después de procesar la
respuesta completa. El orden se define por intento, no por timestamp.

`step status` entrega una vista actual y puede omitir transiciones intermedias;
`step events` es la fuente para consumir transiciones semánticas. `step output`
es la única interfaz para leer output de alto volumen y no avanza el cursor de
eventos. `--tail <líneas>` devuelve una vista instantánea sanitizada y puede
usarse mientras el intento está activo o después de su finalización. `--full`
solo se acepta para un intento terminal y transmite mediante streaming todo el
output retenido y sanitizado; si la política de retención eliminó contenido, la
respuesta incluye metadata de truncado. Cada `interaction_id` es único y queda
vinculado al execution, step e intento que lo originaron; `step approve` rechaza
cualquier cruce de identidad.

Los eventos mínimos son:

| Evento | Propósito |
|---|---|
| `attempt_started` | Confirma identidad, modo, cursor inicial, `bundle_id`, digest del manifiesto, transporte y cantidad de entradas obligatorias admitidas. No incluye contenido del bundle. |
| `attempt_state_changed` | Publica una transición relevante de estado. |
| `interaction_required` | Presenta una interacción y sus decisiones disponibles. |
| `interaction_resolved` | Registra la decisión aplicada y su actor. |
| `output_available` | Indica que existe output consultable sin incluirlo. |
| `attempt_completed` | Publica finalización exitosa y resumen acotado. |
| `attempt_failed` | Publica error estructurado y resumen acotado. |

### Handoff humano en terminal

```bash
$ shardeo step run exec-abc architecture --mode terminal
{"type":"attempt_started","execution_id":"exec-abc","step_id":"architecture","attempt_id":"attempt-8","session_id":"session-8","mode":"terminal","state":"awaiting_human","event_cursor":"event-0","context_bundle":{"bundle_id":"bundle-attempt-8","manifest_sha256":"c12f09abc12f09abc12f09abc12f09abc12f09abc12f09abc12f09abc12f09ab","transport":"file_reference","required_entries_admitted":4},"attach_command":"shardeo step attach exec-abc architecture --attempt-id attempt-8"}

# La persona ejecuta esto en otra terminal:
$ shardeo step attach exec-abc architecture --attempt-id attempt-8
Attached to attempt-8. Use the configured detach sequence to keep the child alive.
```

Si no existe un único intento `terminal` vivo para ese execution y step, el
comando falla de forma explícita y lista las identidades candidatas; nunca se
adjunta a una sesión por inferencia ambigua. El detach no envía una señal de
terminación al hijo.

### Política de permisos

Shardeo reemplaza reglas basadas en patrones de paths o comandos por políticas
sobre capacidades normalizadas que el adapter puede mapear con precisión. La
declaración más cercana reemplaza el objeto completo:

```text
step.permission_policy > workflow.permission_policy > config.yaml defaults.permission_policy
```

El valor por defecto es `prompt`. No se mezclan listas entre capas porque una
combinación implícita puede ampliar autoridad.

```yaml
permission_policy:
  mode: rules
  default: prompt
  rules:
    - capability: filesystem.read
      decision: allow_once
    - capability: process.execute
      decision: deny
```

| Modo de política | Comportamiento |
|---|---|
| `prompt` | Toda solicitud soportada se publica para decisión explícita. |
| `deny` | Toda solicitud que el adapter pueda rechazar con seguridad se rechaza; si no puede, el intento falla cerrado. |
| `rules` | Aplica solo reglas con una capacidad normalizada exacta; `default` debe ser `prompt` o `deny`. |

Una regla automática solo puede seleccionar una decisión presente en
`available_decisions`. `allow_once` autoriza exclusivamente la solicitud actual.
Una aprobación persistente o para toda la sesión solo está disponible si el
adapter anuncia esa decisión y ese alcance para la interacción concreta. Si una
regla solicita una decisión ausente, el resultado es
`unsupported_policy_decision`; Shardeo no la degrada a otra decisión.

`deny` siempre tiene precedencia sobre una autorización automática que pudiera
aplicar a la misma capacidad. Una capacidad desconocida o un mapeo ambiguo nunca
coincide con una regla de autorización; usa el `default` seguro. Toda resolución
automática publica `interaction_resolved` con `actor: "policy"` y referencia la
regla aplicada.

El vocabulario inicial de capacidades incluye `filesystem.read`,
`filesystem.write`, `process.execute`, `network.request` y `unknown`. Un adapter
puede anunciar extensiones namespaced. El core compara identificadores exactos y
no interpreta paths, comandos ni payloads específicos del proveedor para crear
autorizaciones propias.

### Persistencia, concurrencia y recuperación

#### Integridad, permisos y retención del bundle

- El core resuelve `realpath` tanto para la raíz como para cada entrada, exige
  paths relativos contenidos y rechaza paths absolutos, segmentos `..` y escapes
  mediante symlinks antes de copiar o admitir contenido.
- Cada entrada y el bundle completo tienen límites configurables de bytes. Una
  entrada o suma que exceda su límite falla antes de iniciar trabajo del
  proveedor; no se trunca contexto obligatorio.
- Los archivos se crean con permisos locales restrictivos donde el sistema
  operativo lo permita. El bundle no incorpora secretos ni fuentes adicionales:
  solo material ya autorizado para el step.
- El adapter recalcula y verifica cada digest antes de admitir una entrada. Una
  diferencia respecto del manifiesto se trata como drift o manipulación, invalida
  la admisión y falla cerrado.
- Si el proveedor soporta reglas de permiso acotadas, el adapter preautoriza solo
  lectura de la raíz exacta del bundle. Nunca concede lectura amplia del proyecto
  o del filesystem para facilitar `file_reference` o `tool_read`.
- Si no puede expresar ese alcance exacto, el adapter debe usar inyección directa,
  attachments u otro mecanismo nativo que no requiera ampliar permisos. La
  ausencia de una opción segura produce `unsupported_capability`.
- El manifiesto, la prueba de admisión y los digests se vinculan a la evidencia
  del intento. Las copias se retienen con límites de tamaño y antigüedad durante
  el periodo necesario para auditoría, reanudación o recuperación, y después se
  eliminan mediante cleanup verificable. Un intento activo u `orphaned` no pierde
  su bundle mientras todavía pueda recuperarse.

#### Propiedad única y lease

- Solo un supervisor puede poseer un intento gestionado.
- El lease incluye identidad del intento, generación de fencing, identidad del
  proceso propietario y heartbeat.
- Todo comando IPC debe dirigirse al endpoint y a la generación activos. Un
  supervisor con una generación anterior no puede persistir eventos ni aplicar
  decisiones.
- Antes de reemplazar un lease vencido, Shardeo verifica que el propietario no
  siga activo. Un identificador de proceso reutilizado no es evidencia suficiente
  para matar un proceso.

#### Resolución idempotente

Una interacción transiciona mediante compare-and-set desde `pending` a
`resolving` y después a `resolved` o `resolution_failed`. Repetir la misma
decisión retorna el resultado almacenado sin reenviarla al harness. Intentar una
decisión distinta después de adquirir la resolución retorna
`interaction_already_resolved` con la decisión vigente.

La aplicación al proveedor y la persistencia no constituyen una transacción
distribuida. Si el supervisor cae en `resolving`, solo reconcilia o reenvía cuando
el adapter anuncia soporte explícito e idempotente. En caso contrario marca el
intento `orphaned`, conserva la evidencia y exige recuperación humana; nunca
repite una autorización a ciegas.

#### Restart y cleanup

Al iniciar o consultar una ejecución, Shardeo identifica leases vencidos y
supervisores ausentes. Puede reconectar una sesión solo cuando el adapter anuncia
`session_resume: supported` y valida la identidad nativa. Si no puede probar una
reanudación segura, marca el intento `orphaned`, cierra únicamente recursos cuya
propiedad pueda demostrar y conserva output, eventos e interacciones para
auditoría.

El cleanup de sockets, archivos de lease y PTYs ocurre después de persistir el
estado terminal. El restart de Shardeo no transforma automáticamente un intento
`orphaned` en fallido ni crea un segundo supervisor.

### Timeouts y estados de espera

Los tres temporizadores son independientes y configurables. Pueden compartir el
mismo valor por defecto de 300 segundos, pero nunca el mismo contador. El
temporizador de inactividad del proceso aplica a las tres superficies; el de
decisión aplica solo a `supervised` y el de presencia humana solo a `terminal`:

| Timeout | Corre cuando | Se pausa cuando | Resultado al expirar |
|---|---|---|---|
| `process_inactivity_seconds` | El harness debería estar progresando y no produce actividad observable | En `supervised`, existe una interacción pendiente; en `terminal`, se espera attach humano | `process_inactivity_timeout` |
| `interaction_decision_seconds` | El estado es `awaiting_interaction` | La interacción entra en resolución | `interaction_timeout` |
| `human_presence_seconds` | Un intento `terminal` está activo sin una persona adjunta, tanto antes del primer attach como después de un detach | Hay una persona adjunta | `human_presence_timeout` |

Actividad observable significa output o un evento nativo de progreso reconocido
por el adapter; un heartbeat interno no cuenta. Cada expiración genera un evento
de control, solicita al adapter el cierre seguro y conserva la evidencia. Si el
cierre no puede confirmarse, el intento pasa a `orphaned` en lugar de declararse
terminado.

### Eventos de control y output completo

Los eventos de control contienen solo identidades, estados, interacciones,
decisiones, identidad del bundle, digest del manifiesto, transporte, conteos y
resúmenes acotados. Nunca incluyen el manifiesto completo ni el contenido de sus
entradas. El supervisor persiste el evento y su cursor antes de hacerlo visible.
Los cursores son monotónicos dentro del intento y una restricción de unicidad
impide duplicar el mismo evento semántico durante un retry interno.

El output operativo se sanitiza de forma incremental antes de escribirse en disco
por intento y queda sujeto a límites configurables de tamaño y antigüedad. Las
rutas se derivan únicamente de identificadores validados y deben permanecer bajo
una raíz de runtime de Shardeo; se rechazan paths absolutos, `..` y escapes
mediante symlinks. `step output` nunca revela una ruta interna como autoridad para
que el consumidor abra archivos directamente.

La retención o truncado del buffer genera metadata visible, no una finalización
del intento. `--tail` devuelve las últimas líneas retenidas como una respuesta
acotada. `--full` transmite por streaming todo el buffer retenido una vez que el
intento alcanza un estado terminal y no está sujeto al límite de las proyecciones
de evidencia. La evidencia sigue las reglas de Spec 4: los resúmenes, eventos,
salida normal de comandos y proyecciones persistidas se limitan a 16
`KiB (kibibytes: unidades binarias de 1024 bytes)`; `DiagnosticRaw` es la única
autoridad diagnóstica de bytes crudos, acotada hasta 1
`MiB (mebibyte: unidad binaria de 1 048 576 bytes)` y, al exceder el límite,
conserva prefijo, sufijo, tamaño original y digest SHA-256. El buffer operativo
sanitizado no amplía la evidencia persistida ni se convierte en una segunda
autoridad diagnóstica.

### Comparación de superficies

| Criterio | `headless` | `supervised` | `terminal` |
|---|---|---|---|
| Caso principal | Automatización sin interacción en vivo | Control de máquina mediante protocolo estructurado | Control humano de una terminal nativa |
| Compatibilidad v1 | Preserva Spec 4 salvo la migración explícita de `permission_timeout` a `process_inactivity_timeout` | Nueva capacidad del adapter | Nueva capacidad del adapter |
| Propietario de la sesión | `step run` | Supervisor local | Supervisor local; persona controla la PTY |
| Resolución de permisos | Política soportada o finalización terminal | `step approve` con decisión anunciada | La persona responde en la interfaz nativa |
| Canal de decisión | Ninguno universal | API, SDK o protocolo nativo a través del adapter | Entrada humana en PTY |
| Seguimiento | Resultado bloqueante | Polling de control y output bajo demanda | Polling de control, attach y output bajo demanda |
| Riesgo evitado | Bypass de permisos | Acoplamiento a eventos de un proveedor | Automatización frágil o screen scraping de una TUI |

### Criterios de aceptación

- Spec 9 está implementada y es la autoridad de comportamiento de los modos
  `supervised` y `terminal`; en `headless`, la Spec 4 sigue siendo la autoridad de
  la ruta no interactiva salvo la migración del resultado de inactividad. Spec 9
  reemplaza únicamente la propuesta futura no
  implementada de `interactive: boolean` antes documentada en AGENTS.md por
  `headless`, `supervised` y `terminal`.
- El modo se resuelve mediante
  `step.mode > workflow.mode > config.yaml defaults.agent_mode`, con `headless`
  como default, y `--mode` aplica solo al intento actual.
- El core depende de un contrato de adapter neutral y no contiene endpoints,
  flags, fixtures, nombres de eventos ni valores de respuesta de OpenCode, Codex
  u otro proveedor.
- El contrato de contexto del core es un manifiesto neutral con `bundle_id`,
  `attempt_id`, metadata de creación y entradas ordenadas; nunca contiene ni
  requiere sintaxis específica como `@path`.
- Cada entrada conserva su rol lógico, path relativo contenido, obligatoriedad,
  estrategia `eager` u `on_demand`, tamaño en bytes y digest SHA-256.
- El orden y las fronteras son los de Spec 4: instrucciones operacionales,
  instrucciones de dominio, skills, artefactos requeridos y contexto previo o de
  fallback cuando aplique.
- Las instrucciones operacionales y de dominio son obligatorias y `eager`; los
  skills y artefactos requeridos siguen siendo obligatorios aunque se admitan por
  referencia. `on_demand` no permite omitir material contractual.
- La carga diferida puede reducir bytes del transporte inicial, pero todo
  contenido cargado consume tokens del modelo; el manifiesto no declara ese
  contenido libre de costo de contexto.
- El adapter sondea la instalación real en runtime; una versión declarada no
  basta para afirmar capacidad.
- La selección de transporte considera modo, estrategias de carga, límites por
  entrada y totales, verificación de digests, prueba de disponibilidad obligatoria
  y soporte de permisos de lectura acotados.
- Un modo no soportado falla antes del lanzamiento con
  `unsupported_capability`, sin cambiar de superficie ni conceder permisos.
- `headless` preserva el comportamiento de Spec 4, incluido
  `permission_required` cuando el adapter no puede resolver la interacción y la
  inyección directa por entrada estándar cuando siga siendo segura y soportada,
  salvo la migración explícita del resultado de inactividad desde
  `permission_timeout` a `process_inactivity_timeout` (implementada).
- La adopción de bundles no cambia silenciosamente la ruta `headless` de v1; un
  adapter debe anunciar soporte explícito antes de usarlos en ese modo.
- En `supervised`, `step run` inicia un supervisor y retorna de inmediato con
  `attempt_id`, `session_id`, estado, identidad del bundle, transporte y cursor
  inicial, después de congelar y admitir el contexto y abrir la sesión.
- Antes de iniciar trabajo `supervised`, el adapter prueba que todas las entradas
  obligatorias y sus bytes exactos están disponibles para la sesión.
- Una solicitud de permiso supervisada se publica como `interaction_required`
  con `kind: "permission"`, `interaction_id` y `available_decisions`.
- `step approve` acepta solo una decisión anunciada para esa interacción y el
  adapter la traduce mediante su protocolo nativo; no existe inyección universal
  por entrada estándar.
- En `terminal`, `step run` crea una PTY adjuntable, retorna `awaiting_human` y el
  comando `shardeo step attach <execution-id> <step-id> --attempt-id <attempt-id>`.
- Antes de `awaiting_human`, Shardeo congela el bundle y el adapter admite el
  contexto o escribe el bootstrap corto al iniciar el harness. El orquestador
  nunca escribe ese bootstrap en la TUI.
- Solo una persona controla la terminal. El orquestador no envía teclas, no lee
  celdas de pantalla y no realiza screen scraping.
- Detach no mata al hijo y reattach funciona mientras la sesión permanezca viva.
- `step events` soporta polling por cursor persistido sin pérdida ni duplicación
  semántica; `step status` no sustituye el historial de eventos.
- `step output --tail <líneas>` devuelve una vista instantánea sanitizada durante
  o después del intento. `step output --full` solo se acepta para intentos
  terminales y transmite por streaming todo el buffer retenido y sanitizado, con
  metadata de truncado cuando corresponda. Este canal mantiene contención de
  paths y retención acotada, y no altera los límites ni la autoridad de la
  evidencia definida por Spec 4.
- Cada bundle usa copias inmutables por intento, paths contenidos verificados por
  `realpath`, límites por entrada y totales, permisos locales restrictivos y
  detección de drift o manipulación antes de la admisión.
- La retención y eliminación del bundle están acotadas y vinculadas a evidencia,
  reanudación y recuperación del intento; no se añade material secreto que no
  estuviera ya autorizado para el step.
- La lectura preautorizada, cuando exista, se limita al bundle exacto. Ningún
  adapter concede silenciosamente acceso amplio al proyecto o al filesystem; si
  no puede admitir el contexto sin esa ampliación, falla cerrado.
- `attempt_started` registra identidad del bundle, digest del manifiesto,
  transporte y cantidad de entradas obligatorias admitidas, sin emitir contenido
  completo en eventos de control.
- Los comandos al supervisor usan IPC local explícito. SQLite conserva estado y
  auditoría, pero no es el canal de señalización en vivo.
- Un lease con fencing garantiza un solo supervisor propietario por intento.
- La resolución compare-and-set es idempotente; un retry no reenvía una decisión
  ya aplicada y una decisión conflictiva falla de forma explícita.
- Un supervisor stale o una caída durante la resolución se recuperan solo cuando
  el adapter puede probar resume o reconciliación segura; en caso contrario el
  intento pasa a `orphaned` sin repetir autorizaciones.
- El timeout de inactividad del proceso aplica a `headless`, `supervised` y
  `terminal`; el timeout de decisión aplica solo a `supervised` y el de presencia
  humana solo a `terminal`. Los tres usan contadores y errores distintos.
- La política por defecto es `prompt`; `deny` está soportado y toda autorización
  automática requiere un mapeo exacto y una decisión anunciada por el adapter.
- Las aprobaciones de alcance persistente o de sesión solo existen cuando el
  adapter las anuncia para la interacción concreta.
- El lanzamiento no presupone shell ni PTY. El adapter evita interpolación de
  shell salvo requisito justificado y crea una PTY real para `terminal`.

### Flujos completos concisos

#### Permiso supervisado

```bash
$ shardeo step run exec-abc implementation --mode supervised
{"type":"attempt_started","attempt_id":"attempt-9","session_id":"session-9","state":"running","event_cursor":"event-0","context_bundle":{"bundle_id":"bundle-attempt-9","manifest_sha256":"98a1ce2098a1ce2098a1ce2098a1ce2098a1ce2098a1ce2098a1ce2098a1ce20","transport":"api_attachment","required_entries_admitted":4}}

$ shardeo step events exec-abc implementation --attempt-id attempt-9 --after event-0 --limit 50
{"events":[{"cursor":"event-1","type":"interaction_required","interaction":{"interaction_id":"interaction-51","kind":"permission","available_decisions":[{"id":"allow_once","effect":"allow","scope":"request"},{"id":"deny","effect":"deny","scope":"request"}]}}],"next_cursor":"event-1","has_more":false}

$ shardeo step approve exec-abc implementation --interaction-id interaction-51 --decision deny
{"type":"interaction_resolved","interaction_id":"interaction-51","decision":"deny","state":"running","cursor":"event-2"}

$ shardeo step events exec-abc implementation --attempt-id attempt-9 --after event-2 --limit 50
{"events":[{"cursor":"event-3","type":"attempt_completed","exit_code":0}],"next_cursor":"event-3","has_more":false}
```

#### Handoff humano

```bash
$ shardeo step run exec-abc architecture --mode terminal
{"type":"attempt_started","attempt_id":"attempt-10","session_id":"session-10","state":"awaiting_human","context_bundle":{"bundle_id":"bundle-attempt-10","manifest_sha256":"ef71a0c4ef71a0c4ef71a0c4ef71a0c4ef71a0c4ef71a0c4ef71a0c4ef71a0c4","transport":"file_reference","required_entries_admitted":4},"attach_command":"shardeo step attach exec-abc architecture --attempt-id attempt-10"}

# En otra terminal, ejecutado por la persona:
$ shardeo step attach exec-abc architecture --attempt-id attempt-10
Attached to attempt-10. Use the configured detach sequence to keep the child alive.

# El orquestador solo observa el estado:
$ shardeo step status exec-abc architecture --attempt-id attempt-10
{"attempt_id":"attempt-10","state":"running","human_attached":true,"event_cursor":"event-2"}
```

## Spec 10: Override semántico de modelo/proveedor en `step run` con fallback a `steps[].agents`

status: draft

### Objetivo

Permitir que el orquestador escale o cambie el proveedor/modelo/variante de un step sin editar el `workflow.yaml`, manteniendo `steps[].agents` como default versionable y trazable. El override de CLI es prioritario sobre la lista hardcodeada; si el override falla por causa del proveedor/modelo elegido, Shardeo hace fallback automático a los candidatos declarados en `steps[].agents`.

Este enfoque reemplaza el port literal de `custom-tools.json` / `initiative_tier_resolver` / `model-tier-agents.ts` a Shardeo: el workflow declara `agents: [{opencode: deepseek/deepseek-v4-flash}]` como baseline (ej. tier `medium`), y el orquestador escala a `openai/gpt-5.6-sol + xhigh` vía flags de `step run`, sin reescribir YAML ni introducir un `agents_command` declarativo en el schema.

### Descripción funcional

El workflow declara sus candidatos de forma hardcodeada en `steps[].agents`, igual que en Spec 4 (solo `opencode` en v1, con `model` opaco y `variant` opcional). Esa lista es el default y la fuente de fallback. El orquestador puede, en cualquier `shardeo step run`, especificar un override directo:

```bash
shardeo step run <execution-id> <step-id> \
  --harness opencode \
  --provider openrouter/deepseek \
  --model deepseek-v4-flash \
  --variant max \
  [--feedback "<texto>"]
```

Shardeo valida los flags, antepone el candidato de override a la lista `steps[].agents` (deduplicando `harness+provider/model+variant` idéntico), y ejecuta con la misma máquina de probe/claim/invoke/heartbeat de Spec 4. Si el override falla por causa del proveedor/modelo, Shardeo continúa automáticamente con el siguiente candidato de `steps[].agents` sin requerir intervención del orquestador. Si el orquestador no especifica override, el comportamiento es idéntico a Spec 4.

Fuera de Spec 10, el orquestador puede resolver dinámicamente el modelo vía composición del DAG —sin azúcar en el schema— usando un `command` que escribe `resolved.json` y un `agent` posterior que depende de él. No se añade `agents_command` ni `agentsFile` al schema.

Relación con el destino de artefactos: Spec 10 no cambia la raíz de validación de artefactos ni el contrato de contención. `.shardeo/artifacts` permanece como única raíz validada por `validateProduces` / `validateRequires` / `validateContainedPath`. La proyección hacia `.docs/initiatives/**` o cualquier otra raíz versionada es, cuando se necesite (ej. migración del pipeline de initiatives), un `type: command` explícito en el DAG que copia/promociona el artefacto, o un comando dedicado que resuelve la ruta del step. Esa decisión se registra en `.docs/spikes/02_migracion-pipeline-initiatives-moldeable.md` y no forma parte del criterio de aceptación de esta spec.

### Schema y validación

- `steps[].agents` se extiende para soportar `variant` sin romper compatibilidad: cada entrada admite `string` (`"opencode"`), `Record<string, string>` (`{opencode: "openai/gpt-4.1"}`) o `Record<string, {model: string, variant?: string}>` (`{opencode: {model: "openai/gpt-4.1", variant: "high"}}`). La whitelist de `VALID_AGENT_IDENTIFIERS` sigue restringida a `opencode` en v1.
- Los flags de CLI se validan antes de cualquier probe o claim:
  - `--harness` solo admite `opencode` en v1; cualquier otro valor es `workflow_invalid`.
  - `--provider` debe tener forma `provider/model` sin espacios ni `NUL`; no se permiten tokens de entorno.
  - `--model` es requerido si se especifica `--provider` o `--variant`.
  - `--variant` sin `--model` es error de validación.
  - `--variant` es un string sin `NUL` y de longitud acotada (máx. 64 bytes UTF-8).
  - `--harness` aislado sin `--model` es error; el override es atómico (harness + modelo, opcionalmente provider y variant) o no existe.
  - Si se especifica `--provider`, el modelo efectivo es `provider/model`; de lo contrario, es `model` tal cual. No se admiten ambos formatos simultáneamente de forma contradictoria.
- Un workflow con `agents` inválido sigue siendo inválido con el mismo rechazo de Spec 4; el override no vuelve válido un workflow malformado fuera del override.

### Resolución de candidatos y variantes

- Cuando existe override, el candidato sintético `({harness: --harness, model: "<provider/model o model>", variant: --variant})` se antepone a la lista `steps[].agents` resuelta del workflow, preservando el orden declarado de `steps[].agents` para el fallback. Si el candidato sintético coincide exactamente (harness + modelo + variante) con una entrada de `steps[].agents`, se deduplica y aparece una sola vez al inicio.
- `invokeAgent` (`src/utils/agent.ts`) pasa `--model <model>` y, cuando `variant` está presente y no es vacío, `--variant <variant>` al `spawn` de `opencode` con `shell: false`, `detached` y `stdio` inalterados. El orden de flags es `run --format json [--model <model>] [--variant <variant>]`.
- La implementación no introduce un nuevo tipo de transporte de contexto ni modifica `assembleAgentContext` / `getArtifactsDir` / `captureArtifactSnapshot`.

### Fallback

- Si el intento con el candidato de override falla y `completionReason` es `unknown_error` con `providerError` no nulo (error proveniente del proveedor/modelo, ej. cuota, modelo inexistente, auth) o `process_start_failed` atribuible al modelo/proveedor, Shardeo lo trata como fallback y continúa con el siguiente candidato de `steps[].agents` usando el mismo contexto de `fallbackContext` de hasta 2 MiB y presupuesto de snapshot de 1 MiB de Spec 4.
- Los siguientes `completionReason` son **terminales** y no hacen fallback al siguiente candidato, aunque provengan de un override: `permission_required`, `permission_timeout`, `context_error`, `artifact_context_too_large`, `adapter_contract_error`, `process_cleanup_failed`, `persistence_error`. `exhaustion` cierra el `step run`.
- Cuando el override falla y existe al menos un candidato de fallback disponible, el error del proveedor se preserva en la evidencia sanitizada del intento (hasta 16 KiB, `DiagnosticRaw` hasta 1 MiB) y el siguiente intento recibe la proyección sanitizada como `fallbackContext`.
- Si el override tiene éxito, el step queda `running` y requiere `step complete` como en Spec 4.

### Persistencia y observabilidad

- Se añade migración idempotente `migrateSpec10` que crea `step_attempts.variant TEXT` (nullable, sin default) compatible con bases existentes, con reintentos `SQLITE_BUSY` como en migraciones previas.
- Cada intento persiste `agent_used` (harness), `model` (`provider/model` efectivo) y `variant` (valor literal o `NULL` cuando no se especificó). `getExecutionStatusSummary` / `shardeo status` exponen `variant` por intento sin romper consumidores que lo ignoren.
- El resumen de `steps next` y `workflows describe` no cambian; el override es propiedad del `step run`, no del workflow.

### Criterios de aceptación

- `shardeo step run <execution-id> <step-id> --harness opencode --provider openrouter/deepseek --model deepseek-v4-flash --variant max` ejecuta el step con ese modelo/variante y lo persiste; el mismo step sin flags usa `steps[].agents` sin cambios respecto a Spec 4.
- La declaración `agents: [{opencode: {model: "openai/gpt-4.1", variant: "high"}}]` en el workflow es válida y equivale a `--model openai/gpt-4.1 --variant high` como fallback, con el mismo `invokeAgent` subyacente.
- Un workflow que declara `agents: ["opencode"]` sin modelo sigue siendo válido; `variant` es opcional y no se requiere en ningún nivel.
- Flags incompletos o inválidos (`--variant` sin `--model`, `--provider` sin `--model`, `--harness` no soportado, `provider` mal formado, `variant` con `NUL` o vacío) fallan antes de probe/claim con error estructurado y no crean intento.
- Cuando el override falla con `unknown_error` + `providerError` no nulo, Shardeo hace fallback automático al siguiente candidato de `steps[].agents` en el mismo `step run`; el intento fallido queda persistido con `completion_reason: unknown_error` y `decision: fallback`.
- Cuando el override falla con `permission_required`, `context_error`, `artifact_context_too_large`, `adapter_contract_error`, `process_cleanup_failed` o `persistence_error`, el step termina con `completion_reason` terminal, `decision: terminal` y no hace fallback al siguiente candidato.
- Si el override es idéntico a un candidato de `steps[].agents`, se ejecuta una sola vez (deduplicado) y no se duplica el intento.
- `variant` se persiste por intento (`TEXT`, nullable) y aparece en `shardeo status`; una base creada antes de esta spec sigue funcionando tras la migración y los intentos previos muestran `variant: null`.
- El comportamiento de `step complete`, `reopen --cascade`, `skip`, `steps next`, `resume` y la validación de `requires`/`produces` bajo `.shardeo/artifacts` permanece sin cambios respecto a Specs 4, 6, 7 y 8.

### Ejemplo de flujo de usuario

```bash
# workflow declara medium como default versionable
# .shardeo/workflows/initiative-spec/workflow.yaml
# steps:
#   - id: implement
#     type: agent
#     agents: [{opencode: {model: deepseek/deepseek-v4-flash, variant: max}}]

# Orquestador mantiene medium por defecto
$ shardeo step run exec-abc implement
→ usa deepseek/deepseek-v4-flash (max) de steps[].agents

# Escalado a sota sin editar YAML
$ shardeo step run exec-abc implement \
    --harness opencode --provider openai --model gpt-5.6-sol --variant xhigh
→ antepone openai/gpt-5.6-sol (xhigh); si falla por cuota/modelo, fallback a deepseek/deepseek-v4-flash

# Declarativo alternativo (equivalente al override) dentro del workflow
# agents: [{opencode: {model: "openai/gpt-5.6-sol", variant: "xhigh"}}, {opencode: deepseek/deepseek-v4-flash}]
```

### No objetivos

- Portar `custom-tools.json` / `initiative_tier_resolver` / `model-tier-agents.ts` a Shardeo ni introducir un `tier` semántico en el schema del workflow. Los tiers viven en el orquestador y se traducen a `provider/model+variant` al invocar `step run`.
- Añadir `agents_command` o `agentsFile` al schema para resolver candidatos dinámicamente vía script. La composición recomendada es un `type: command` que escribe `.shardeo/artifacts/resolved.json` seguido de un `agent` que depende de él; el orquestador lee el JSON y decide el override por `step run`.
- Cambiar la raíz de artefactos (`getArtifactsDir`), la contención de paths o el presupuesto de snapshots/evidencia de Spec 4. La promoción a `.docs/initiatives/**` se hace con un `command` explícito en el DAG (ver `.docs/spikes/02_migracion-pipeline-initiatives-moldeable.md`), no con un `artifacts_dir` configurable en esta spec.
- Cambiar el orden de validación del workflow: la carga y validación completa del YAML se repite en cada `step run` antes de resolver el override, igual que en Spec 4.
- Introducir un nuevo transporte de contexto ni modificar el formato de `fallbackContext` más allá de lo ya definido por Spec 4.