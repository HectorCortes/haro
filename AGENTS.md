# AGENTS.md — Convenciones para agentes IA

Este repositorio es sobre agentes de codificación IA y se trabaja en él con agentes. Estas convenciones aplican a cualquier agente (o humano) que edite el repo.

## Idioma

- El idioma del repositorio es **español**: documentación, artefactos técnicos, mensajes de commit, PRs e issues se escriben en español.
- Sigue la convención existente de los documentos: tono profesional, sin emojis, sin relleno.

## Fuentes de verdad

1. **`deltas-acceptance.md`** — el contrato: 92 criterios de aceptación organizados en specs, con IDs tipo `v2-no-regresion/F-01` (`F-<n>` funcional, `U-<n>` unitario). Un criterio está cumplido solo cuando su verificación pasa de forma reproducible.
2. **`docs/v2/haro-constitucion.md`** — normativa del proyecto (trazabilidad, fail-closed, neutralidad metodológica, core sin literales de proveedor, reglas de estabilidad).
3. **`docs/v2/haro-especificacion-tecnica.md`** — especificación técnica v2 (schema YAML, DDL SQLite, interfaces, protocolos JSON-RPC/ACP).
4. **`docs/reference/`** — comportamiento v1 (oráculo de la migración; los contratos de v1 se preservan salvo que la normativa v2 diga lo contrario).

En conflicto entre fuentes, la constitución y el contrato prevalecen. No modifiques los documentos existentes sin un cambio SDD que lo justifique.

## Stack

- **Go puro, sin cgo** (SQLite vía `modernc.org/sqlite`), CLI, JSON-RPC sobre socket Unix, YAML.
- Aún **no existe `go.mod`**: se creará en la fase spike. No lo generes por adelantado.
- **Postura de seguridad**: no se incorporan scripts de instalación de terceros ni hooks que ejecuten código externo en el repo (política del proyecto; la instalación de dependencias pasa por el gestor de paquetes estándar).

## Fuera de alcance

- **Terminal embebida / PTY**: diferida. No diseñar ni implementar nada en esa superficie.

## Flujo de desarrollo

- Cada spec de `deltas-acceptance.md` se desarrolla como un **change SDD** en `openspec/` (ciclo `proposal → spec → design → tasks → apply → verify → archive`); los cambios completados se archivan en `openspec/changes/archive/`.
- **Gate**: CI verde (`go build ./...`, `go vet ./...`, `go test ./... -race`, golangci-lint, govulncheck). Localmente, al menos `go test ./...`.
- **Commits convencionales** (`feat(...)`, `fix(...)`, `docs(...)`, `refactor(...)`, `test(...)`, `chore(...)`) y PRs pequeños como work units.
- Al añadir artefactos de build o cobertura, añade las entradas correspondientes al `.gitignore` sin borrar las existentes.