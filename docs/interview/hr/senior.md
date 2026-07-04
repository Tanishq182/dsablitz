# HR - Senior Level

This document details senior-level leadership and culture questions, focusing on mentoring developers in database patterns and building quality standards.

---

## Q&A Set 1: Mentoring Developers in Database & Architecture Patterns

### 1. Interviewer Intent
The interviewer wants to evaluate the candidate's leadership style, empathy, teaching ability, and how they mentor junior developers in adopting complex backend patterns (like transaction isolation and dependency inversion).

### 2. Strong Answer
Mentoring junior developers requires explaining the "why" behind design decisions using codebase examples rather than just teaching abstract concepts.

For example, when mentoring a developer on **Transaction isolation and Connection Pool Deadlocks**:
1. **Explain the Risk**: I showed them how opening a nested transaction inside an active transaction block can consume multiple database connections, deadlocking the connection pool under load.
2. **Review Code Examples**: We walked through the `BattleCoordinator` interface, showing how propagating the transaction handle (`pgx.Tx`) through adapters keeps connection usage to exactly one connection per request.
3. **Visualize with Diagrams**: We drew diagrams showing the database connection pool limits to clarify why transactions must share connection contexts.

By using concrete code reviews and visual diagrams, the developer understood connection pool management and successfully applied these patterns to new service integrations.

### 3. Common Mistakes
* Directing junior developers to "just copy this pattern" without explaining the database design tradeoffs.
* Using overly abstract terminology without referencing specific codebase files or transactions.
* Criticizing implementation errors in code reviews without providing constructive refactoring examples.

### 4. Follow-up Questions
* **How do you verify a junior developer has understood the architecture patterns?**
  * *Answer*: I ask them to lead the next design review for a related feature or have them write tests simulating connection failures to verify their transaction rollback logic.

### 5. How DSAblitz demonstrates this concept
The dependency inversion setup is implemented via the coordinator adapter, which is a key learning reference for the team.

### 6. Relevant code references
* Shared transaction context adapter: [routes.go:L21-L35](file:///home/tanishq/dsablitz/backend/internal/server/routes.go#L21-L35)
* Coordinator interface contract: [models.go:L121-L124](file:///home/tanishq/dsablitz/backend/internal/rooms/models.go#L121-L124)

### 7. Related documentation
* [Dependency Graph Reference](file:///home/tanishq/dsablitz/docs/architecture/dependency_graph.md)
* [Transaction Boundaries Deep Dive](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)

---

## Q&A Set 2: Fostering a Culture of Documentation & Quality

### 1. Interviewer Intent
The interviewer wants to assess the candidate's ability to establish engineering standards, enforce code quality, and maintain operational documentation across the team.

### 2. Strong Answer
Fostering a quality-focused culture requires clear guidelines and automated checks in the deployment pipeline.

I established the **Living Documentation Policy** in our project context: documentation must evolve alongside the code. Every phase of development must update the system architecture, database ADRs, and runbooks before code is merged.

Additionally, I enforced database migration standards:
* Every schema change must include a matching rollback script (`down.sql`).
* Down migrations must drop tables and constraints in reverse order of creation to prevent foreign key constraint errors.
* Manual database hot-fixes in production are strictly prohibited; all changes must run through raw SQL migrations in CI/CD.

These standards, combined with automated migration validation in pull requests, keep our documentation accurate and protect our production database.

### 3. Common Mistakes
* Treating documentation as a post-release chore, resulting in stale API docs and outdated database references.
* Allowing manual table modifications in staging or production databases, bypassing the migration pipeline.
* Reviewing pull requests without verifying that migration scripts are backwards-compatible with running code.

### 4. Follow-up Questions
* **How do you handle developers who resist writing documentation?**
  * *Answer*: I explain how clear documentation prevents operational pages and outages. I also make updating documentation a required step in our pull request checklist.

### 5. How DSAblitz demonstrates this concept
The Living Documentation Policy header is placed at the top of the project context, and raw SQL migrations are defined in sequential order.

### 6. Relevant code references
* Living Documentation Policy definition: [PROJECT_CONTEXT.md:L5-L7](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L5-L7)
* Sequential raw SQL migration files: [000001_create_core_schema.up.sql](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql) and [000001_create_core_schema.down.sql](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.down.sql)

### 7. Related documentation
* [Documentation Quality Checklist](file:///home/tanishq/dsablitz/docs/checklists/documentation.md)
* [Migration Strategy & Operations](file:///home/tanishq/dsablitz/docs/database/migrations.md)

---

## Key Takeaways
1. **Mentorship** should explain connection pool limits and transaction sharing through concrete codebase examples.
2. **Living Documentation Policies** ensure system architecture, API contracts, and runbooks evolve alongside code changes.
3. **Database migrations** must be transactional and include matching rollback scripts.

---

## Interview Questions
* **How do you prevent database lock wait queues from building up during migration deployments?**
  * *Answer*: Enforce short lock timeouts (`SET local lock_timeout = '3s'`) on migration runs so they fail quickly instead of blocking application queries.
* **Why should database migrations run before new code is deployed?**
  * *Answer*: Running migrations first ensures the database schema supports the new application code. Schema additions must be backwards-compatible to prevent active server instances from failing.

---

## Common Mistakes
* **Stale runbooks**: Allowing architectural documentation to drift from the active code state, leading to operational confusion.
* **Non-transactional migrations**: Executing raw DDL updates outside transaction blocks, leaving the database in a partially migrated state if a statement fails.

---

## Related Documents
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
* [Database Schema Reference](file:///home/tanishq/dsablitz/docs/database/schema.md)

---

## Lessons Learned
* **Shared transaction ownership**: Mentoring the team on shared transaction context propagation resolved connection leaks and established a standard for cross-module integration.
