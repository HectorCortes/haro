# Haro

> **Harnesses Orchestrator** — orquestador de agentes de codificación IA a través de harnesses.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Haro estructura y gestiona la ejecución de workflows de desarrollo asistidos por IA. No es un agente ni un IDE: no reemplaza a OpenCode, Codex CLI, Claude Code ni otros agentes de codificación. Los trata como **harnesses** (runtimes de agente intercambiables) y les da estructura: qué ejecutar, con qué contexto, en qué orden y con qué runtime, separando la metodología (workflows), el conocimiento (skills y artefactos) y la ejecución (adapters por harness).

Haro es el **sucesor v2 de Shardeo**: conserva el modelo de workflows por pasos, dependencias, artefactos y trazabilidad, y lo reconstruye sobre un broker persistente por proyecto, IPC JSON-RPC entre CLI y broker, un contrato de adapter basado en ACP, aislamiento de workspace mediante `path_claims`, composición de workflows, reporte de cambios y un store sobre SQLite.

## Estado

**Pre-alpha — en migración activa de TypeScript a Go.** El contrato de "completo" es `deltas-acceptance.md` (92 criterios de aceptación organizados en specs, cada uno con un ID como `v2-no-regresion/F-01`). La especificación técnica v2 define el schema YAML, el DDL de SQLite, las interfaces y los protocolos; el código Go aún no existe (`go.mod` se creará en la fase spike). Aún no hay binario publicable ni comandos estables para usuarios.

**Stack objetivo:** Go puro, sin cgo (SQLite vía `modernc.org/sqlite`), CLI, JSON-RPC sobre socket Unix, YAML.

## Documentación

- [deltas-acceptance.md](deltas-acceptance.md) — criterios de aceptación verificables; el contrato de qué significa "completo".
- [docs/v2/haro-constitucion.md](docs/v2/haro-constitucion.md) — constitución normativa del proyecto.
- [docs/v2/haro-especificacion-tecnica.md](docs/v2/haro-especificacion-tecnica.md) — especificación técnica v2 (schema, DDL, interfaces, protocolos).
- [docs/reference/](docs/reference/) — referencia del comportamiento v1 (oráculo de la migración).

## Desarrollo

Requisitos: Go (la versión estable; `go.mod` se creará en la fase spike).

```sh
go test ./...
```

La CI ejecuta `go build ./...`, `go vet ./...`, `go test ./... -race`, golangci-lint y govulncheck en cada push a `main` y en cada pull request.

- [CONTRIBUTING.md](CONTRIBUTING.md) — cómo contribuir.
- [SECURITY.md](SECURITY.md) — cómo reportar vulnerabilidades.

## Licencia

MIT. Ver [LICENSE](LICENSE).