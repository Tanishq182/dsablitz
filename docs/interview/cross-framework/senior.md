# DSAblitz Interview Prep: Cross-Framework Comparisons (Senior Level)

This document provides in-depth comparisons between different systems architectures, scaling limits, and concurrency models for senior engineers, reflecting the production-grade design decisions made in **DSAblitz**.

---

## Q&A 1: Modular Monolith vs. Microservices (Transaction Boundaries & Scaling)

### Interviewer Intent
The interviewer wants to see if you can critically evaluate modular monoliths vs. microservices. They will test your knowledge on database transaction boundaries, cross-module coupling, network latency, connection pool starvation, and the operational complexity of distributed transactions (Sagas vs. 2PC) compared to local SQL transactions.

### Strong Answer
A **Modular Monolith** was chosen for the MVP of DSAblitz to maximize transactional consistency and minimize latency. In a real-time multiplayer application, starting a match must be an atomic operation: the lobby status must change, players must be assigned, and the battle sequence must be persisted. 

```
Modular Monolith Transaction Boundary:
[Rooms Module] --(Passes pgx.Tx)--> [Battle Coordinator Adapter] --(Passes pgx.Tx)--> [Battle Module]
   └── Runs inside a single connection block. Commit/Rollback is fully atomic.

Microservices Distributed Boundary:
[Rooms Service] --(gRPC StartBattle)--> [Battle Service] 
   └── Requires Saga Pattern or Outbox. Eventually Consistent. High failure complexity.
```

#### 1. Transaction Boundary and Consistency
- **Monolith (DSAblitz)**: We enforce a **Transaction Boundary Rule**. Cross-module gameplay actions (e.g. initializing a battle from a room) run within a single atomic database transaction. We propagate the parent transaction context (`pgx.Tx`) through interfaces. If the Battle module fails to insert the question sequence, the Rooms module's status change to `in_battle` is rolled back automatically.
- **Microservices**: To achieve the same atomicity, we would need to implement a Saga Pattern (orchestrated/choreographed) or a Two-Phase Commit (2PC). This introduces distributed consensus problems, increases latency (multiple network roundtrips), and requires compensation logic (e.g. manually deleting the battle record if the room fails to update).

#### 2. Dependency Inversion and Coupling
- To prevent architectural rot, we enforce a strict **Dependency Inversion Rule**: the Rooms module must never import the concrete Battle module. Instead, it interacts through a Rooms-owned interface, `BattleCoordinator`. This keeps modules decoupled. If we decide to split the Battle engine into a standalone service in V2, we can swap the local adapter with a gRPC or HTTP client implementation without altering any business logic inside the Rooms module.

#### 3. Connection Pool Starvation
- Passing `pgx.Tx` across modules locks a database connection for the duration of the transaction. If a module performs network requests or slow CPU operations inside the transaction block, it starves the connection pool, crashing the system under load. We mitigate this by keeping transactional blocks extremely short, executing only fast SQL operations, and performing stateless calculations outside of the transaction boundary.

### Common Mistakes
- **Nested Transactions**: Attempting to start a transaction (`tx.Begin()`) inside an active transaction. PostgreSQL does not support nested transactions natively; it will fail. In DSAblitz, we pass the parent transaction context `pgx.Tx` directly to downstream repositories instead of opening new transactions.
- **Microservices as a Default**: Assuming microservices are always superior. A microservice architecture would significantly increase network overhead, making sub-100ms real-time status syncs harder to achieve while introducing complex distributed failure scenarios.

### Follow-up Questions
1. *How would you refactor the Rooms-Battle boundary to use the Transactional Outbox pattern if we migrated to microservices?*
2. *How do you prevent connection-pool deadlocks when multiple transactions compete for connections under heavy concurrency?*

### How DSAblitz demonstrates this concept
DSAblitz utilizes a modular monolith architecture with strict interface boundaries and shared database transaction contexts.
- **Dependency Inversion**: The Rooms module defines `BattleCoordinator` in [models.go:L121-L124](file:///home/tanishq/dsablitz/backend/internal/rooms/models.go#L121-L124).
- **Wiring the Adapter**: The adapter is implemented in `server` and maps room structures to battle structures in [routes.go:L21-L35](file:///home/tanishq/dsablitz/backend/internal/server/routes.go#L21-L35).
- **Shared Transaction Execution**: The atomic database execution spans modules in [service.go:L338-L424](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L338-L424).

### Related Documentation
- [Modular Monolith Design](file:///home/tanishq/dsablitz/docs/adr/0001_modular_monolith_design.md)
- [Module Boundaries](file:///home/tanishq/dsablitz/docs/architecture/module_boundaries.md)

---

## Q&A 2: Distributed Cache Invalidation vs. Single-Node In-Memory Caches

### Interviewer Intent
The interviewer wants to discuss cache coherency in horizontal scaling environments. They will check your understanding of split-brain caching, cache invalidation strategies (write-through, write-around, pub/sub invalidation), and the network/concurrency limits of distributed caches.

### Strong Answer
In a single-node setup, an in-memory cache managed by a `sync.RWMutex` (as implemented in the Questions module of DSAblitz) is highly efficient. However, when we scale the application horizontally to multiple nodes, we face the **Cache Coherency Problem**:

```
[User Request] ---> [Node A] (Updates Question DB -> Updates Node A RAM Cache)
                      └── [Node B] (Unaware of change -> Serves stale RAM Cache)
```

If Node A handles an Admin CRUD API update and updates PostgreSQL, Node B's local cache remains stale, causing inconsistent gameplay. We analyzed the following solutions to address this scaling limit:

1. **Short TTLs (Eventually Consistent)**: Let the local cache expire after 1 minute. Simple to implement, but players might see different questions or validation results for up to 60 seconds, which is unacceptable for a competitive platform.
2. **Centralized Redis Cache**: Read all questions from Redis. This eliminates split-brain but introduces 1-3ms network roundtrips and serialization costs for every gameplay query.
3. **Redis Pub/Sub Cache Invalidation (DSAblitz V2 Target)**: Keep the read-heavy local in-memory cache for nanosecond lookup speeds. When an Admin updates a question, Node A publishes an invalidation event (`question_invalidated`) to a Redis Pub/Sub channel. All running nodes subscribe to this channel. Upon receiving the event, each node invalidates its local memory copy or reloads the active question bank.

This third option provides the best of both worlds: local read performance (nanosecond speeds) and distributed cache consistency (sub-10ms propagation latency).

### Common Mistakes
- **Ignoring the Network Overhead of Redis**: Assuming a centralized Redis cache is always better than an in-memory cache. For read-only catalogs like questions, querying Redis hundreds of times per second adds unnecessary latency compared to local memory lookups.
- **Race Conditions in Pub/Sub Invalidation**: Assuming the invalidation message always arrives before the next read query. If the invalidation is delayed, a read might fetch stale data. We mitigate this by including a version timestamp in the invalidation message to ensure late-arriving queries do not override newer database states.

### Follow-up Questions
1. *What happens to your nodes if the Redis server goes down? Does the Pub/Sub invalidation fail gracefully?*
2. *How does the Questions seeder in DSAblitz ensure idempotency when reloading the cache at startup?*

### How DSAblitz demonstrates this concept
DSAblitz documents this exact scaling path and manages cache states.
- **In-Memory Cache Loading**: Defined in [service.go:L31-L46](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L31-L46).
- **V2 Distributed Cache Invalidation**: Outlined as Phase 5 Technical Debt in [PROJECT_CONTEXT.md:L82-L86](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L82-L86).

### Related Documentation
- [Cache Design](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)
- [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)

---

## Q&A 3: Concurrency Control (Pessimistic vs. Optimistic vs. Distributed Locks)

### Interviewer Intent
The interviewer wants to analyze your concurrency engineering capabilities. They will look for deep analysis of pessimistic row locking (`SELECT ... FOR UPDATE`), optimistic locking (version columns), and distributed locks (Redlock), specifically regarding scalability, connection usage, deadlock risks, and contention.

### Strong Answer
For high-contention operations like 1v1 match submissions (where users submit answers in sub-second intervals), choosing the right concurrency control strategy is critical:

| Strategy | Implementation | Pros | Cons / Scaling Limits |
| :--- | :--- | :--- | :--- |
| **Pessimistic Row Locking** | `SELECT ... FOR UPDATE` | Guarantees serialization at the DB layer; prevents race conditions; simple implementation. | Locks database connections; can cause starvation under extreme workloads. |
| **Optimistic Concurrency Control (OCC)** | `WHERE version = current_version` | No DB-level locks; high performance under low contention. | Aborts/retries on conflict; high overhead in high-contention game loops. |
| **Distributed Locks (Redlock)** | Redis mutexes with lease TTLs | Offloads lock contention from DB to Redis; scale across DB nodes. | Requires network overhead; lock lease expiration bugs; high architectural complexity. |

#### 1. Pessimistic Locking in DSAblitz
We chose **Pessimistic Row Locking** because competitive gameplay requires absolute consistency (no score double-counting or out-of-order state progressions).
- When a user submits an answer, we execute `GetBattlePlayerForUpdate` which runs `SELECT ... FOR UPDATE` on `battle_players`.
- This serializes concurrent requests for that user. A second rapid submission will block until the first commits, read the updated submission counter, and be rejected as a duplicate.

#### 2. Deadlock Prevention
To prevent deadlocks when locking multiple resources, we enforce a **Global Lock Ordering Rule**:
- Any transaction spanning multiple tables must acquire row locks in this exact hierarchy: `rooms` -> `room_players` -> `battles` -> `battle_players` -> `battle_question_sequence`.
- In `ExpireRooms`, we sort target rows deterministically using `ORDER BY id ASC` before locking, ensuring concurrent routines never lock resources in reverse order, which would cause deadlock cycles.

### Common Mistakes
- **Performing External I/O Inside Lock Blocks**: Making a network call or validating an answer via an external API while holding a `FOR UPDATE` lock. This holds the database connection open, quickly starving the pool and freezing the entire application.
- **Relying on Application-Level Mutexes in Distributed Environments**: Using a Go `sync.Mutex` to protect submission logic. This works on a single server, but as soon as the app scales to two instances behind a load balancer, the mutex is bypassed, allowing concurrent race conditions.

### Follow-up Questions
1. *Why do we use a monotonic submission index verification alongside pessimistic locking?*
2. *What is the difference between PostgreSQL `FOR UPDATE` and `FOR SHARE` locks?*

### How DSAblitz demonstrates this concept
DSAblitz implements these concurrency controls strictly.
- **Pessimistic Row Locking**: Implemented in [service.go:L192-L198](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L192-L198) when locking the player's progression row.
- **Monotonic Submission Index Guard**: Enforced at [service.go:L227-L233](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L227-L233).
- **Deterministic Sort in ExpireRooms**: Enforced at [service.go:L427-L440](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L427-L440).

### Related Documentation
- [Websocket Concurrency](file:///home/tanishq/dsablitz/docs/deep-dives/websocket_concurrency.md)
- [Room Transactions](file:///home/tanishq/dsablitz/docs/deep-dives/room_transactions.md)

---

## Key Takeaways
- **Atomic Boundaries**: Modular monoliths utilize single database transaction scopes to maintain consistency, avoiding the distributed coordination complexity of microservices.
- **Local Read Caching**: In-memory caches are optimal for read-heavy static catalogs, but require distributed invalidation hooks (like Redis Pub/Sub) when scaled horizontally.
- **Database-Level Concurrency**: Concurrency control must reside at the persistent layer (database `SELECT ... FOR UPDATE`) in distributed systems to prevent race conditions across nodes.

## Interview Questions
1. *Compare the consistency and latency tradeoffs of a Saga pattern vs. a local Postgres transaction.*
2. *How do you prevent deadlocks when executing batch room cleanups with concurrent user updates?*
3. *How would you design a Redis Pub/Sub mechanism to sync local cache invalidations across 10 application nodes?*

## Common Mistakes
1. **Network Requests inside DB Transactions**: Calling external endpoints while holding connection locks, leading to connection starvation.
2. **Missing Lock Ordering**: Acquiring row locks in random order, leading to database deadlocks under high load.
3. **Application-only Locks in Distributed Deployments**: Relying on local memory locks (`sync.Mutex`) when running multiple horizontal containers.

## Related Documents
- [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)
- [Cache Design](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)
- [Transaction Boundaries](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)

## Lessons Learned
- Decoupling modules with clean interface definitions allows starting as a simple modular monolith and migrating to microservices in the future.
- Row-locking queries must order their records deterministically (`ORDER BY id ASC`) before locking to avoid deadlock cycles.
