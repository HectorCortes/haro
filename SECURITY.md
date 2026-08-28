# Haro security policy

## Reporting a vulnerability

Please, **do not open a public issue** to report security vulnerabilities. Use **GitHub Private Vulnerability Reporting** on this repository (*Security → Report a vulnerability* tab), which guarantees that the report is only visible to the maintainer.

## What to report

Report through this channel any vulnerability that affects Haro's security, including:

- Vulnerabilities in the CLI or in input handling (YAML parsing, JSON-RPC IPC, arguments).
- Unintended code execution (command injection, unsanitized paths, harness payloads).
- Failures in permission and approval handling (`agent` steps, permission request mediation).
- Workspace isolation and `path_claims` (access to files outside the declared scope).
- Vulnerabilities in dependencies (including the build chain, known CVEs).
- Persistence issues (SQLite, execution data) that expose information.

**Do not** report through this channel ordinary functional bugs, expected specification behavior or UX issues: those go as regular issues (use the templates in `.github/ISSUE_TEMPLATE/`).

## Response commitment

Haro is maintained by a single maintainer. Upon receiving a report:

- A response is provided **as soon as possible** (typically within a few days), acknowledging the report and assessing its scope.
- Work is done on a fix and, if applicable, on a coordinated public notice.
- **No SLA** for response or a fix timeline is promised.
- **No rewards** are offered for the report.

Include in the report: description, steps to reproduce, estimated impact and affected version or commit. The more information you provide, the faster it can be assessed.