# System Design - Senior Level

This document details senior-level architecture discussions, focusing on database connection pool deadlocks, global lock ordering rules, deterministic row sorting, and production failure debugging.

---

## Q&A Set 1: Connection Pool Deadlocks & Transaction Context Propagation

### 1. Interviewer Intent
The interviewer wants to evaluate the candidate's understanding of database connection pool dynamics, nested transaction pitfalls, resource starvation, and how to design clean cross-module transactional APIs in high-traffic applications.

### 2. Strong Answer
In a modular monolith, operations often span multiple domain services. For instance, when starting a battle:
1. The `rooms.Service` begins a transaction to update the lobby state.
2. The `rooms.Service` calls the `battle.Service` to initialize the match.

If the `battle.Service` opens a new, nested transaction block, it must acquire a second database connection from the pool while the first connection remains open and locked by the parent room transaction.

```
PostgreSQL Connection Pool (Limit: 50)
┌────────────────────────────────────────────────────────┐
│ Connection 1 (Locked by Room Tx) ──► Waits for Battle  │
│ Connection 2 (Acquired by Battle) ──► Completes Battle │
└────────────────────────────────────────────────────────┘
```

Under high concurrent load, if 50 clients attempt to start battles at the same millisecond:
1. 50 parent room transactions are started, consuming all 50 connections in the pool.
2. To proceed, each transaction calls the battle service, which attempts to acquire a second connection from the pool.
3. Because the pool is empty, all battle calls block waiting for connections to release.
4. However, the connections cannot be released because the parent transactions are waiting for the battle calls to complete.

This results in a **Connection Pool Deadlock** (or pool starvation). The system freezes and all active transactions eventually time out.

We prevent this by enforcing a **Shared Transaction Context Rule**. Cross-module interfaces must accept the parent transaction handle (`pgx.Tx`) as an argument. All secondary writes run on this shared connection context, keeping connection usage to exactly **one** connection per user request.

### 3. Common Mistakes
* Spawning asynchronous goroutines that attempt to write to the database using the parent transaction handle, which causes race conditions and driver panics.
* Thinking that increasing the connection pool limit resolves the deadlock. It only delays it, as the system will still deadlock when the request volume exceeds the new pool size.
* Injecting repository dependencies across modules, bypassing service boundaries and exposing tables directly, which breaks modularity.

### 4. Follow-up Questions
* **How would you debug connection pool starvation in production?**
  * *Answer*: Monitor the pool metrics using `pgxpool.Stat()`. If `AcquireCount` and `EmptyWaitCount` are high while active connections are saturated, check the logs for nested transaction blocks or slow external calls running inside transactions.

### 5. How DSAblitz demonstrates this concept
In DSAblitz, transaction context is passed via `pgx.Tx` through the `BattleCoordinator` interface, ensuring that the room status update and battle initialization run on a single connection.

### 6. Relevant code references
* Shared transaction context propagation: [routes.go:L25-L35](file:///home/tanishq/dsablitz/backend/internal/server/routes.go#L25-L35)
* StartBattleTx coordinator execution: [service.go:L103-L154](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L103-L154)

### 7. Related documentation
* [Database Transaction Strategy](file:///home/tanishq/dsablitz/docs/database/transactions.md)
* [Transaction Boundaries Deep Dive](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)

---

## Q&A Set 2: Deadlock Prevention & Deterministic Row Locking

### 1. Interviewer Intent
The interviewer wants to assess if the candidate can design database transactions that touch multiple tables or rows without creating deadlock loops. They want to check if the candidate understands lock graphs and row sorting requirements.

### 2. Strong Answer
A database deadlock occurs when two concurrent transactions attempt to acquire locks on the same set of resources in different orders:
* Transaction A locks Row 1 and waits for Row 2.
* Transaction B locks Row 2 and waits for Row 1.

To prevent this, we enforce two strict design rules:
1. **Global Lock Hierarchy Rule**: Any transaction that acquires locks on multiple tables must lock them in a strict order:
   $$\text{rooms} \rightarrow \text{room\_players} \rightarrow \text{battles} \rightarrow \text{battle\_players} \rightarrow \text{battle\_question\_sequence}$$
2. **Deterministic Row Sorting Rule**: When locking multiple rows in the same table concurrently, the rows must be sorted by their primary key (e.g. `ORDER BY user_id ASC`) before applying the lock:
   ```sql
   SELECT * FROM battle_players WHERE battle_id = $1 ORDER BY user_id ASC FOR UPDATE
   ```
   This ensures that concurrent transactions always lock rows in the same order, converting potential deadlock loops into simple, serialized queues.

```
Transaction A (Locks User 1 -> Waits for User 2)
Transaction B (Locks User 1 -> Waits for User 2)
Result: Serialized Queue (Safe)
```

### 3. Common Mistakes
* Querying multiple rows using a random array order and locking them, which practically guarantees deadlocks under load.
* Forgetting that PostgreSQL aborts transactions permanently on any query error, meaning retry loops (like room code collisions) must run outside the transaction block.
* Applying locks on unindexed columns, which escalates row locks to full table locks, blocking all database read and write traffic.

### 4. Follow-up Questions
* **What is the difference between page locks, row locks, and table locks in PostgreSQL?**
  * *Answer*: Row locks (`FOR UPDATE`) lock specific rows. Page locks lock the disk page containing the rows. Table locks lock the entire table. Locks escalate if PostgreSQL runs out of memory or if queries lack index support, degrading performance.

### 5. How DSAblitz demonstrates this concept
In `CompleteBattle`, the battle service locks both players' scorecards. The repository query enforces `ORDER BY user_id ASC FOR UPDATE` to guarantee deterministic lock ordering.

### 6. Relevant code references
* Deterministic row locking query: [repository.go:L291-L313](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L291-L313)
* Room cleanup cleanup query (ORDER BY id ASC): [service.go:L427-L464](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L427-L464)

### 7. Related documentation
* [Database Schema Reference](file:///home/tanishq/dsablitz/docs/database/schema.md)
* [Database Indexing Design](file:///home/tanishq/dsablitz/docs/database/indexing.md)

---

## Key Takeaways
1. **Connection pool deadlocks** are caused by nested transactions acquiring multiple connections from the pool.
2. **Transaction context propagation** via `pgx.Tx` ensures that cross-module writes use exactly one connection.
3. **Lock hierarchies and row sorting** prevent database deadlock loops by serializing lock acquisition.

---

## Interview Questions
* **How would you resolve a failed migration that leaves the database in a "dirty" state?**
  * *Answer*: Fix the migration file and the database state manually, force the migration version to the correct index using `migrate force <version>`, and re-run the migration.
* **Why should retry loops run outside transaction blocks in PostgreSQL?**
  * *Answer*: PostgreSQL aborts the entire transaction on any query error, meaning retries inside the same block will fail. Retries must start a fresh transaction.

---

## Common Mistakes
* **Acquiring new connections in loops**: Opening transactions inside loops, which quickly exhausts the connection pool.
* **Unordered multi-row updates**: Updating multiple rows without sorting their primary keys, causing random deadlocks under high write loads.

---

## Related Documents
* [Modular Monolith Design ADR](file:///home/tanishq/dsablitz/docs/adr/0001_modular_monolith_design.md)
* [Room Transactions Deep Dive](file:///home/tanishq/dsablitz/docs/deep-dives/room_transactions.md)

---

## Lessons Learned
* **Row locks in room cleanup**: During load testing, periodic room cleanups caused deadlocks. We resolved this by sorting room IDs (`ORDER BY id ASC`) before applying the lock in `ExpireRooms`.
