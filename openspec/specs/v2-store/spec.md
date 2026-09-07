# v2-store Specification

## Purpose

Complete the reference store with the remaining DDL, repositories, interchangeable backends, and atomic idempotent migration: all persistence access crosses repository interfaces behind a strict import gate, the 11-table §2 reference schema is exact with typed constraint sentinels, per-connection pragmas and RFC3339 UTC timestamps hold, SQLite and a mutex-backed fake satisfy one unchanged backend-neutral contract suite, and migration is transactional and idempotent.

## Constraints

The store MUST remain pure Go/no cgo without new dependencies; `database/sql` and `modernc.org/sqlite` MUST NOT be imported outside `internal/store`, and `internal/execution/report.go` MUST use store errors. This change SHALL create only the exact §2 `leases` and `interactions`; `leases` MUST retain its composite primary key without FK or CHECK, and `interactions` MUST retain its attempt FK, `permission|question` and `pending|resolved` CHECKs, and `UNIQUE(attempt_id, idempotency_key)`. `path_claims`, `attempt_transport`, `dag_hash`, and `base_commit` MUST be verified and MUST NEVER be redefined by this change. WAL and foreign-key enforcement MUST apply to every SQLite connection, and persisted timestamps MUST be RFC3339 UTC. Both backends MUST normalize failures to `ErrNotFound`, `ErrCheckViolation`, `ErrUniqueViolation`, and `ErrForeignKeyViolation`; the fake MUST NOT import a driver.

## Requirements

### Requirement: No direct SQLite access outside the store [F-01]

All persistence access MUST cross repository interfaces, and the import gate MUST reject either SQL import outside `internal/store`.

#### Scenario: Boundary gate
- GIVEN all Go packages
- WHEN the dependency gate inspects imports
- THEN only `internal/store` imports `database/sql` or `modernc.org/sqlite`

### Requirement: Complete reference schema and repositories [F-02]

Created databases MUST expose all 11 §2 tables and constraints. `Store` SHALL expose lease and interaction repositories. Acquisition MUST atomically issue retained per-step maximum fencing token plus one, initially one; renew and release MUST check holder and token.

#### Scenario: Exact schema ownership
- GIVEN a fresh database
- WHEN schema creation completes
- THEN all 11 reference tables match §2 and only `leases` and `interactions` are newly defined here

#### Scenario: Monotonic fencing
- GIVEN repeated successful acquisitions for one execution step
- WHEN each prior lease is released
- THEN tokens are 1, 2, and 3, and stale holder/token mutations fail closed

### Requirement: Connection pragmas and UTC timestamps [F-03]

Every SQLite connection MUST enforce foreign keys and file-backed databases MUST use WAL. Written timestamps MUST parse as RFC3339 UTC.

#### Scenario: Dedicated claim connection
- GIVEN `claim.Acquire` references an absent project through a dedicated connection
- WHEN the claim is inserted
- THEN `ErrForeignKeyViolation` is returned and `foreign_keys` is enabled on that connection

#### Scenario: Timestamp sampling
- GIVEN records containing each store-generated time field
- WHEN values are sampled
- THEN every value parses as RFC3339 with UTC offset `Z`

### Requirement: Interchangeable repository backend [U-01]

One unchanged shared contract suite MUST run state-machine, lease, claim, event, and interaction behavior against file-backed SQLite and a mutex-backed fake, including rollback and monotonic cursors.

#### Scenario: Dual-backend parity
- GIVEN factories for SQLite and fake stores
- WHEN the same suite runs against each factory without domain changes
- THEN both satisfy identical outcomes and error categories

### Requirement: Atomic idempotent migration [U-02]

Migration MUST be transactional and idempotent. A test-only seam MAY inject failure but MUST NOT alter production behavior.

#### Scenario: Repeated migration
- GIVEN an already migrated database containing records
- WHEN schema creation runs twice
- THEN both runs succeed and schema and records remain unchanged

#### Scenario: Mid-migration rollback
- GIVEN a fresh database and an injected failure after an earlier DDL statement
- WHEN migration runs and is retried without failure
- THEN the failed run leaves no partial schema and the retry creates the complete schema

### Requirement: Constraint rejection parity [U-03]

SQLite MUST reject invalid execution, step, attempt, interaction, and workspace enums at database level. Both backends MUST enforce FK and uniqueness constraints and return corresponding typed sentinels.

#### Scenario: Invalid constrained writes
- GIVEN valid parent rows in each backend
- WHEN invalid enums, duplicate interaction keys, or missing foreign keys are written
- THEN each write fails with `ErrCheckViolation`, `ErrUniqueViolation`, or `ErrForeignKeyViolation`, respectively

## Non-Requirements

Broker runtime, lease expiry/renewal loops, PTY, reporting, distribution, multi-user behavior, new dependencies, and sole-owned schema redefinition are outside this change.
