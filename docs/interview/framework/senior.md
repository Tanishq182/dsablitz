# Framework & Patterns - Senior Level

This document details senior-level framework and operations patterns, focusing on Docker infrastructure, Redis persistence tuning, and database migration operations.

---

## Q&A Set 1: Development Infrastructure & Redis Persistence Tuning

### 1. Interviewer Intent
The interviewer wants to assess the candidate's understanding of development environments containerization, service orchestration via Docker Compose, and critical Redis persistence configurations (AOF vs RDB) for high-availability systems.

### 2. Strong Answer
In modern backend systems, Docker Compose structures local development environments, running multi-container services (such as PostgreSQL and Redis) under isolated networks with persistent storage volumes.

For key-value stores like Redis, configuring appropriate persistence is critical to prevent data loss. Redis supports two primary persistence options:
1. **RDB (Redis Database snapshots)**: Periodically writes point-in-time snapshots of the dataset to disk. While fast to load, a crash can lose up to 5 minutes of data.
2. **AOF (Append Only File)**: Logs every write command received by the server to a disk file. AOF is much more durable, losing at most 1 second of writes under the default `appendfsync everysec` configuration.

In our local development environment, we configure Redis with **AOF enabled**:
```yaml
redis:
  image: redis:7
  command: ["redis-server", "--appendonly", "yes"]
```
This ensures that session tokens and matchmaking lobby states survive container crashes or restarts, providing a reliable environment for testing.

### 3. Common Mistakes
* Leaving Redis in standard snapshotting mode (RDB) for session caches, causing user sessions to drop when container instances restart.
* Not defining appropriate health checks in the Compose file, allowing the application service to boot before PostgreSQL or Redis are ready to accept connections.
* Committing database credentials or API secrets directly into Docker Compose files instead of referencing environment variables.

### 4. Follow-up Questions
* **How does Redis AOF prevent file size ballooning over time?**
  * *Answer*: Redis uses an automatic background rewrite process (`BGREWRITEAOF`) to recreate the AOF file with the shortest sequence of commands needed to rebuild the current state.

### 5. How DSAblitz demonstrates this concept
Our Docker Compose file configures local development services, enabling Redis appendonly persistence and setting up service health checks.

### 6. Relevant code references
* Docker Compose services configuration: [docker-compose.yml:L1-L38](file:///home/tanishq/dsablitz/infra/docker-compose.yml#L1-L38)
* Redis appendonly configuration command: [docker-compose.yml:L20-L24](file:///home/tanishq/dsablitz/infra/docker-compose.yml#L20-L24)

### 7. Related documentation
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
* [Database Schema Reference](file:///home/tanishq/dsablitz/docs/database/schema.md)

---

## Q&A Set 2: Database Migration Operations with golang-migrate

### 1. Interviewer Intent
The interviewer wants to evaluate the candidate's experience in managing database schema changes, understanding migration versioning, and resolving pipeline blockages (like fixing a "dirty" migration state) in production databases.

### 2. Strong Answer
We manage database schema changes using sequential, raw SQL migration files executed by the `golang-migrate/migrate` framework.

The framework tracks migration state in PostgreSQL using a `schema_migrations` table containing two columns:
* `version` (bigint): The sequential identifier of the active schema state.
* `dirty` (boolean): Flagged as `true` if a migration fails during execution.

If a migration fails midway (e.g. due to a constraint violation or invalid syntax), the database marks the state as **dirty** and blocks all subsequent runs to prevent corruption.

```
schema_migrations table:
┌─────────────────┬────────────────┐
│ version: 000004 │ dirty: true    │ ◄── Blocks further runs
└─────────────────┴────────────────┘
```

To resolve a dirty state:
1. **Fix the Schema**: Manually execute SQL queries in the database to fix or revert the half-applied changes.
2. **Force Version**: Reset the migration tracking version using the CLI:
   ```bash
   migrate -path ./migrations -database "$DATABASE_URL" force <last_stable_version>
   ```
3. **Re-run Migration**: Deploy the corrected migration file to roll forward cleanly.

To ensure rollback safety, we write bi-directional migrations: every `up.sql` has a corresponding `down.sql` script that drops tables and constraints in reverse order of creation.

### 3. Common Mistakes
* Running migration scripts in production without dry-running them on staging databases to verify execution times and lock durations.
* Creating database indexes inside active migration transactions, which can lock the table and block user writes. Indexes must be created concurrently outside transactions.
* Deleting rows in `schema_migrations` manually to resolve errors, causing version tracking issues.

### 4. Follow-up Questions
* **How do you add a column to a table with millions of rows without locking it?**
  * *Answer*: Add the column as nullable first, populate the values in small batches to avoid long locks, and then apply the `NOT NULL` constraint.

### 5. How DSAblitz demonstrates this concept
Our migrations folder contains sequential SQL scripts with up/down behaviors, including partial index definitions.

### 6. Relevant code references
* Up migration scripts: [000004_fix_room_players_constraints.up.sql:L1-L18](file:///home/tanishq/dsablitz/backend/migrations/000004_fix_room_players_constraints.up.sql#L1-L18)
* Down migration scripts: [000004_fix_room_players_constraints.down.sql:L1-L10](file:///home/tanishq/dsablitz/backend/migrations/000004_fix_room_players_constraints.down.sql#L1-L10)

### 7. Related documentation
* [Migration Strategy & Operations](file:///home/tanishq/dsablitz/docs/database/migrations.md)
* [Database Schema Reference](file:///home/tanishq/dsablitz/docs/database/schema.md)

---

## Key Takeaways
1. **Redis appendonly AOF** ensures durability for session and lobby caches by logging every write command to disk.
2. **Failed migrations** flag `dirty = true` in PostgreSQL, requiring manual schema cleanup and `migrate force` commands to resolve.
3. **Up and Down migration scripts** must be written together to allow clean, transactional rollbacks during failures.

---

## Interview Questions
* **Why does golang-migrate use a dirty flag?**
  * *Answer*: To prevent the database from running subsequent migrations on top of a partially failed state, which could corrupt the schema.
* **What is the default fsync strategy for Redis AOF, and what are the tradeoffs?**
  * *Answer*: The default is `everysec`, which writes data to disk once per second. This balances high performance with low data loss (at most 1 second of writes).

---

## Common Mistakes
* **No rollback tests**: Deploying migrations without testing the `down.sql` rollback script, leaving the system vulnerable during deployment failures.
* **No lock timeout**: Running DDL changes on large tables without setting a lock timeout, allowing migrations to block user queries indefinitely.

---

## Related Documents
* [Database Indexing Design](file:///home/tanishq/dsablitz/docs/database/indexing.md)
* [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)

---

## Lessons Learned
* **Migration version collision**: Adopting a strict sequential registry and validating migration versions in pull requests prevented version collisions and stabilized deployment pipelines.
