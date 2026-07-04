# Behavioral - Senior Level

This document details senior-level behavioral scenarios, focusing on leading architectural alignments, managing system technical debt, and guiding teams through core engineering selections.

---

## Scenario 1: Managing Long-Term Architectural Technical Debt

### 1. Interviewer Intent
The interviewer wants to assess the candidate's technical foresight, strategic planning, ability to manage technical debt pragmatically, and how they guide a team when scaling an architecture from single-node to clustered systems.

### 2. Strong Answer
In our Questions module, the question bank is read-only and loaded into a thread-safe local cache map at startup. This design is highly optimized for a single instance, delivering sub-microsecond read speeds (~50ns) and offloading all catalog read traffic from PostgreSQL.

However, as we planned for horizontal scaling (multi-instance behind a load balancer), this design introduced architectural technical debt: if an admin updates a question, Node A writes to PostgreSQL and updates its local cache, but Node B and Node C continue serving stale data from their local RAM.

I managed this technical debt by taking a multi-step approach:
1. **Document and Prioritize**: I documented the issue as a Phase 5 technical debt item in our core documentation, detailing the limitations and tradeoffs of the local cache.
2. **Align the Team**: I led a session explaining the options: query the database directly (too slow), use Redis for all reads (adds network latency), or keep local caching but implement a **Redis Pub/Sub invalidation hook** in V2.
3. **Establish the Future Path**: We agreed to keep the high-speed local cache for the MVP. When admin APIs are introduced, Node A will publish an invalidation event via Redis Pub/Sub, and other nodes will subscribe to the channel, reload the question from PostgreSQL, and update their local cache maps.

This balanced MVP simplicity with a clear path to cluster scaling.

### 3. Common Mistakes
* Implementing complex, distributed caching mechanisms before they are required, adding unnecessary infrastructure complexity to the MVP.
* Failing to document architectural technical debt, leaving the team unaware of scaling boundaries.
* Choosing design patterns without analyzing latency profiles (e.g. replacing sub-microsecond local reads with millisecond Redis reads for static configurations).

### 4. Follow-up Questions
* **How does the lazy read-through fallback help during cache misses?**
  * *Answer*: If a question ID is not found in the local cache map, the service queries PostgreSQL directly. This ensures the system continues working even if the cache fails to populate.

### 5. How DSAblitz demonstrates this concept
The Questions cache uses a thread-safe local mapping, and the documentation lists the Redis Pub/Sub invalidation hook as a V2 technical debt item.

### 6. Relevant code references
* Lazy read-through cache lookup: [service.go:L48-L57](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L48-L57)
* Cache design roadmap: [PROJECT_CONTEXT.md:L82-L86](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L82-L86)

### 7. Related documentation
* [Questions Cache Design](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)

---

## Scenario 2: Guiding Consensus on Database Driver Selection

### 1. Interviewer Intent
The interviewer wants to evaluate the candidate's technical leadership, deep driver knowledge, capacity to compare raw libraries vs abstractions, and how they guide a team to a consensus on core architecture decisions.

### 2. Strong Answer
During the scaffolding phase of DSAblitz, our engineering team debated whether to use Go's standard `database/sql` driver library or the PostgreSQL-native `jackc/pgx/v5` driver.

Some team members favored `database/sql` because it is part of the standard library and allows switching SQL databases (like MySQL) easily. I led the discussion and presented a comparative analysis:
1. **Performance**: `database/sql` uses `interface{}` parameter binding, causing interface allocation overheads. `pgx` uses type-safe parameters, reducing allocation overhead.
2. **Native Features**: `pgx` natively supports PostgreSQL-specific features like array types (`tags && $1`), copy protocols, and advanced connection pooling metrics (`pgxpool.Stat`), which `database/sql` lacks.
3. **Database Portability Myth**: Switching database engines in production is extremely rare and requires rewriting SQL queries, schemas, and transactions anyway. Designing for database portability adds abstraction overhead without real-world benefits.

Based on this analysis, I guided the team to a consensus: we chose `jackc/pgx/v5` as our core database driver to optimize performance and leverage PostgreSQL's native features.

### 3. Common Mistakes
* Making driver selections based on personal preference or popularity without compiling a list of technical tradeoffs.
* Failing to document driver selections, leaving future developers unaware of the architectural reasoning.
* Over-engineering database abstractions (like generic ORMs) that degrade performance and obscure database-level transactions.

### 4. Follow-up Questions
* **How does pgx support array intersections in PostgreSQL?**
  * *Answer*: It maps Go slices directly to PostgreSQL array types, allowing us to run query filters like `tags && $1` without manual string formatting.

### 5. How DSAblitz demonstrates this concept
The database connection pool is configured using `pgxpool` in the platform package.

### 6. Relevant code references
* PGX Connection Pool Setup: [database.go:L15-L31](file:///home/tanishq/dsablitz/backend/internal/platform/database/database.go#L15-L31)
* Array intersection query in repository: [repository.go:L34-L55](file:///home/tanishq/dsablitz/backend/internal/questions/repository.go#L34-L55)

### 7. Related documentation
* [Database Schema Reference](file:///home/tanishq/dsablitz/docs/database/schema.md)
* [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)

---

## Key Takeaways
1. **Local in-memory caches** provide sub-microsecond speeds; cluster scaling requires invalidation patterns like Redis Pub/Sub.
2. **PostgreSQL-native drivers** (`pgx`) outperform standard abstractions by supporting type-safe parameters and native connection pools.
3. **Architectural alignments** should prioritize real-world performance over generic abstractions.

---

## Interview Questions
* **Why does pgx stand out for Go applications using PostgreSQL?**
  * *Answer*: It offers direct binary protocol communication, type-safe array mapping, and native connection pool metrics.
* **What are the risks of using distributed Redis caches for static configuration data?**
  * *Answer*: It introduces network dependency and latency (milliseconds vs nanoseconds), which can cause request failures if Redis goes down.

---

## Common Mistakes
* **Portability over-engineering**: Abstracting the database driver to support multiple SQL engines, losing access to PostgreSQL-specific optimizations.
* **Distributed caching overhead**: Replacing local memory caches with distributed ones for static catalogs, adding network latency to every read.

---

## Related Documents
* [Modular Monolith Design ADR](file:///home/tanishq/dsablitz/docs/adr/0001_modular_monolith_design.md)
* [Database Indexing Design](file:///home/tanishq/dsablitz/docs/database/indexing.md)

---

## Lessons Learned
* **Driver consistency**: Selecting `jackc/pgx/v5` enabled type-safe operations and connection pool management, simplifying the implementation of pessimistic row locking.
