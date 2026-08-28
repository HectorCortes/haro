# Política de seguridad de Haro

## Reportar una vulnerabilidad

Por favor, **no abras un issue público** para reportar vulnerabilidades de seguridad. Usa **GitHub Private Vulnerability Reporting** en este repositorio (pestaña *Security → Report a vulnerability*), que garantiza que el reporte solo sea visible para el mantenedor.

## Qué reportar

Reporta por esta vía cualquier vulnerabilidad que afecte la seguridad de Haro, incluyendo:

- Vulnerabilidades en el CLI o en el manejo de entradas (parsing de YAML, IPC JSON-RPC, argumentos).
- Ejecución de código no intencionada (inyección de comandos, rutas no sanitizadas, payloads de harness).
- Fallos en el manejo de permisos y aprobaciones (steps `agent`, mediación de solicitudes de permiso).
- Aislamiento de workspace y `path_claims` (acceso a archivos fuera del alcance declarado).
- Vulnerabilidades en dependencias (incluidas las de la cadena de build, CVE conocidas).
- Problemas de persistencia (SQLite, datos de ejecución) que expongan información.

**No** reportes por esta vía bugs funcionales ordinarios, comportamientos esperados de la especificación ni problemas de UX: esos van como issues normales (usa las plantillas de `.github/ISSUE_TEMPLATE/`).

## Compromiso de respuesta

Haro es mantenido por un único mantenedor. Al recibir un reporte:

- Se responde **lo antes posible** (típicamente en pocos días), reconociendo el reporte y evaluando su alcance.
- Se trabaja en una corrección y, si aplica, en un aviso público coordinado.
- **No se promete un SLA** de respuesta ni un calendario de corrección.
- **No se ofrecen recompensas** por el reporte.

Incluye en el reporte: descripción, pasos para reproducir, impacto estimado y versión o commit afectado. Cuanta más información aportes, más rápido se podrá evaluar.