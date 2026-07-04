# System Design - Graduate / Intern Level

This document contains detailed system design interview notes for Graduate/Intern level roles, focusing on in-memory caching, database transactions, and the basic request-response path.

---

## Q&A Set 1: In-Memory Caching vs Relational Databases

### 1. Interviewer Intent
The interviewer wants to assess whether the candidate understands the fundamental difference in latency, throughput, and scalability between in-memory structures and physical disk-backed databases like PostgreSQL. They want to check if the candidate knows when to apply caching to offload database query load.

### 2. Strong Answer
An in-memory cache stores data directly in the application's random-access memory (RAM), which offers sub-microsecond access times (typically ~50 nanoseconds). In contrast, query processing in a relational database like PostgreSQL involves establishing TCP connection handshakes (unless using pooled connections), parsing SQL statements, executing query plans, searching B-tree indexes, and potentially performing physical disk read operations, resulting in millisecond-range latency.

For static or read-heavy catalogs, fetching data directly from PostgreSQL on every request degrades read throughput and starves connection pools. Caching this data in RAM decouples read operations from the database entirely.

In a single-instance system, a local thread-safe mapping guarded by a read-write mutex (`sync.RWMutex`) provides optimal speed. On cache misses, the system uses a **Lazy Read-Through Cache** mechanism to retrieve the data from the database, populate the cache, and return it to the client.

### 3. Common Mistakes
* Assuming that distributed caching solutions like Redis are as fast as local in-memory caching. Redis still requires a network roundtrip (typically ~1ms) and network serialization, while local RAM lookups take nanoseconds.
* Forgetting to make in-memory cache structures thread-safe, leading to race conditions and memory corruption when multiple concurrent goroutines mutate or access maps.
* Not implementing database read-through fallbacks on cache misses, which can cause requests to fail if the cache fails to populate at startup.

### 4. Follow-up Questions
* **How does the cache remain consistent if the database is updated?**
  * *Answer*: Since the question bank in our MVP is updated only through seeds, we load the cache at startup. For real-time updates in a clustered environment, we would publish invalidation events via Redis Pub/Sub to trigger updates across nodes.

### 5. How DSAblitz demonstrates this concept
In DSAblitz, the static question bank catalog is loaded into RAM at server startup. The `questions.Service` manages this in-memory cache using a map and `sync.RWMutex` to guard concurrent reads.

### 6. Relevant code references
* Cache lookup and lazy fallback: [service.go:L48-L57](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L48-L57)
* Cache initialization at startup: [service.go:L31-L46](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L31-L46)

### 7. Related documentation
* [Questions Cache Design](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)
* [Database Indexing](file:///home/tanishq/dsablitz/docs/database/indexing.md)

---

## Q&A Set 2: Database Transactions for Lobby Creation

### 1. Interviewer Intent
The interviewer wants to assess if the candidate understands the concept of database transactions, the ACID properties (specifically Atomicity), and how to apply them to prevent inconsistent states in multiplayer lobbies.

### 2. Strong Answer
A database transaction is a sequence of SQL operations treated as a single logical unit of work. It adheres to **ACID** properties, ensuring that either all operations succeed or none do (**Atomicity**).

In a multiplayer game lobby, when a host creates a room, two operations must happen concurrently:
1. Insert the metadata for the new room into the `rooms` table.
2. Insert the host player record into the `room_players` table.

If these inserts are executed outside a transaction and the server crashes midway, an empty room might be created without a host player, creating an orphaned state. Wrapping these statements in an atomic transaction ensures both tables are updated together, maintaining system consistency.

### 3. Common Mistakes
* Writing sequential database inserts on separate connections without a transaction block, leaving the system vulnerable to split-brain states when one of the queries fails.
* Not defining appropriate foreign key constraints (e.g. `ON DELETE CASCADE`), which could cause orphan rows when parent records are dropped.
* Executing slow external network calls (such as API integration requests) inside transaction blocks, which holds database connections open and causes connection pool starvation.

### 4. Follow-up Questions
* **What happens if a room code collision occurs during creation?**
  * *Answer*: The transaction fails and rolls back. The room code generation retry logic must run outside the transaction loop so each retry starts with a clean database transaction context.

### 5. How DSAblitz demonstrates this concept
The `rooms.Service` executes both the room creation and the initial player seating within a transaction using the `WithTransaction` helper.

### 6. Relevant code references
* CreateRoom atomic transaction block: [service.go:L56-L104](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L56-L104)
* Room validation check before write: [models.go:L52-L76](file:///home/tanishq/dsablitz/backend/internal/rooms/models.go#L52-L76)

### 7. Related documentation
* [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)
* [Room Transactions Deep Dive](file:///home/tanishq/dsablitz/docs/deep-dives/room_transactions.md)

---

## Key Takeaways
1. **In-memory cache lookups** run in nanoseconds, eliminating database disk I/O and networking overhead for read-heavy static data.
2. **Lazy read-through fallbacks** prevent service failures during cache misses by querying the database directly.
3. **Database transactions** enforce atomicity, ensuring related entities (like lobbies and players) are inserted together.

---

## Interview Questions
* **Why is standard memory map lookup not thread-safe in Go, and how do we resolve it?**
  * *Answer*: Go maps do not support concurrent reads and writes. We use `sync.RWMutex` to serialize write operations using `Lock()` while allowing concurrent read operations using `RLock()`.
* **What is connection starvation in a relational database?**
  * *Answer*: It occurs when transactions take too long to complete, consuming all available connections in the pool. New requests must block and wait, leading to timeout failures.

---

## Common Mistakes
* **Lack of mutex synchronization**: Accessing shared cache maps without RLock/WLock, leading to fatal runtime panic errors under load.
* **Orphaned states**: Mutating related tables sequentially outside a database transaction, leaving corrupted states if a write fails.

---

## Related Documents
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
* [Database Schema Reference](file:///home/tanishq/dsablitz/docs/database/schema.md)

---

## Lessons Learned
* **Read Committed Isolation**: In our postgres configuration, read operations can see dirty reads unless explicit locks are used. We migrated critical state checks to run within transaction boundaries, protecting player stats from concurrency issues.
