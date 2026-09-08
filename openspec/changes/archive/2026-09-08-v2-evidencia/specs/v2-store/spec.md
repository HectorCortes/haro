# Delta for v2-store

## MODIFIED Requirements

### Requirement: Complete reference schema and repositories [F-02]

Created databases MUST expose all 11 §2 tables and constraints, including nullable `attempt_events.payload TEXT` alongside retained `payload_ref`. `Store` SHALL expose lease and interaction repositories. Acquisition MUST atomically issue retained per-step maximum fencing token plus one, initially one; renew and release MUST check holder and token.

(Previously: `attempt_events` exposed `payload_ref` but no inline `payload` column.)

#### Scenario: Exact schema ownership
- GIVEN a fresh database
- WHEN schema creation completes
- THEN all 11 reference tables match §2, including both attempt-event payload columns
- AND only `leases` and `interactions` are newly defined by the original store change

#### Scenario: Monotonic fencing
- GIVEN repeated successful acquisitions for one execution step
- WHEN each prior lease is released
- THEN tokens are 1, 2, and 3, and stale holder/token mutations fail closed

### Requirement: Interchangeable repository backend [U-01]

One unchanged shared contract suite MUST run state-machine, lease, claim, event, interaction, and nullable payload behavior against file-backed SQLite and a mutex-backed fake, including rollback and monotonic cursors.

(Previously: backend parity did not require inline attempt-event payload round trips.)

#### Scenario: Dual-backend parity
- GIVEN factories for SQLite and fake stores
- WHEN the same suite writes and reads null and non-null payloads without domain changes
- THEN both satisfy identical values, outcomes, and error categories

### Requirement: Atomic idempotent migration [U-02]

Migration MUST be transactional and idempotent. Existing databases MUST gain `attempt_events.payload TEXT` additively after a `PRAGMA table_info` probe; duplicate-column races MUST be accepted only through the duplicate-column error guard. A test-only seam MAY inject failure but MUST NOT alter production behavior.

(Previously: migration idempotency did not cover the additive payload column or duplicate-column race.)

#### Scenario: Repeated migration
- GIVEN a pre-payload database containing records
- WHEN schema creation runs repeatedly, including concurrent opens
- THEN all runs succeed with one nullable payload column and unchanged records

#### Scenario: Mid-migration rollback
- GIVEN a fresh database and an injected failure after an earlier DDL statement
- WHEN migration runs and is retried without failure
- THEN the failed run leaves no partial schema and the retry creates the complete schema
