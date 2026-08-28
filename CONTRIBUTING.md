# Contribuir a Haro

Gracias por querer contribuir. Este documento describe el flujo de trabajo y los criterios que todo cambio debe cumplir para ser integrado.

## Flujo de trabajo

1. **Fork** el repositorio y clónalo localmente.
2. Crea una **rama** con un nombre descriptivo (`feat/broker-ipc`, `fix/race-store`, `docs/constitucion`).
3. Implementa el cambio en **work units pequeños** (PRs y commits pequeños, enfocados en una sola cosa).
4. Abre un **pull request** contra `main` describiendo el cambio (usa la plantilla incluida).
5. Espera la **revisión**. Se requiere **1 aprobación** para hacer merge.
6. El merge es **squash only**: cada PR se integra como un único commit con el mensaje de la rama.

**No** se requiere firma de commits ni DCO: los commits no firmados son bienvenidos.

## Convención de commits

Commits convencionales (`type(scope): descripción`), en español cuando el contenido lo esté:

- `feat(broker): persiste ejecuciones por proyecto`
- `fix(ipc): corrige reenvío de eventos tras reconexión`
- `docs(constitucion): aclara regla de capacidades`
- `refactor(store): extrae interfaz de repositorio`
- `test(adapter): cubre negociación de capacidades`
- `chore(ci): añade job de govulncheck`

Tipos usados: `feat`, `fix`, `docs`, `refactor`, `test`, `perf`, `chore`.

## Gates de integración

Un PR solo se integra si cumple todo lo siguiente:

- **CI verde**: `go build ./...`, `go vet ./...`, `go test ./... -race`, golangci-lint y govulncheck pasan en el PR.
- **Contrato respetado**: `deltas-acceptance.md` es el contrato de qué significa "completo". El PR debe **referenciar los IDs de los criterios** que toca (por ejemplo, `v2-broker/F-01`, `v2-adapter/U-03`) en su descripción.
- **Alcance acotado**: cambios pequeños y revisables. Si el PR toca varias specs, divídelo en varios PRs.
- **Documentación coherente**: si el cambio altera comportamiento normativo, debe reconciliarse con `docs/v2/` y con los criterios de `deltas-acceptance.md`.

## Cómo se estructura el desarrollo

Cada spec de `deltas-acceptance.md` se desarrolla como un **change SDD** bajo `openspec/`, con el ciclo `proposal → spec → design → tasks → apply → verify → archive`. Los cambios completados se archivan en `openspec/changes/archive/`; las specs vigentes viven en `openspec/specs/`.

- Los criterios de `deltas-acceptance.md` son la fuente de los criterios de aceptación de cada change.
- Un delta está completo solo cuando pasan todos sus criterios P0 y P1 y el change queda archivado.
- Revisa `docs/v2/haro-constitucion.md` y `docs/reference/` antes de tocar comportamiento: la constitución es normativa y la referencia es el oráculo del comportamiento v1 durante la migración.

## Reportar bugs y solicitar funcionalidades

- Bugs y funcionalidades: abre un issue usando las plantillas de `.github/ISSUE_TEMPLATE/`.
- Vulnerabilidades de seguridad: **no** abras un issue público; sigue [SECURITY.md](SECURITY.md).