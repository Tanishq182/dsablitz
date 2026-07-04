# Concurrency Controls - SDE1 Level Interview Prep

This guide covers lock ordering hierarchies to prevent circular deadlocks, transaction boundary rules, and connection pool starvation mitigation in Go-based modular monoliths.

---

## Q&A Sets

### Q1: What is a cyclic deadlock in database transactions, and how does enforcing a global lock ordering hierarchy across multiple tables prevent it?

#### Interviewer Intent
Evaluate the candidate's understanding of deadlock conditions (specifically circular wait), database lock contention, and architectural strategies to prevent deadlocks in systems with multi-table mutations.

#### Strong Answer
A cyclic deadlock occurs when two or more transactions are unable to proceed because each is waiting for a lock held by the other. For example:
* **Transaction A** locks Row X in the `rooms` table, and then attempts to lock Row Y in the `room_players` table.
* **Transaction B** (running concurrently) locks Row Y in the `room_players` table, and then attempts to lock Row X in the `rooms` table.

Neither transaction can complete; Transaction A waits for Transaction B to release Row Y, while Transaction B waits for Transaction A to release Row X. PostgreSQL detects this cycle and aborts one of the transactions.

To prevent cyclic deadlocks, we enforce a **Global Lock Ordering Hierarchy**. We define a strict sequence in which resources must be locked:
$$\text{rooms} \rightarrow \text{room\_players} \rightarrow \text{battles} \rightarrow \text{battle\_players} \rightarrow \text{battle\_question\_sequence}$$

If every transaction in our system follows this exact ordering:
* Transaction A locks Row X in `rooms`.
* Transaction B, wanting to update `room_players` and `rooms`, is forced by lock ordering rules to lock Row X in `rooms` **before** locking Row Y in `room_players`.
* Transaction B blocks waiting for the `rooms` lock. Transaction A successfully acquires `room_players` and completes, releasing its locks. Transaction B then acquires the locks and completes.

By serializing access at the highest common ancestor in the hierarchy, we eliminate the circular wait condition, preventing deadlocks entirely.

```mermaid
graph TD
    subgraph Lock Hierarchy (Order of Acquisition)
        R[rooms] --> RP[room_players]
        RP --> B[battles]
        B --> BP[battle_players]
        BP --> BQS[battle_question_sequence]
    end
    style R fill:#f9f,stroke:#333,stroke-width:2px
```

#### Common Mistakes
* Believing that database isolation levels (like `REPEATABLE READ` or `SERIALIZABLE`) eliminate the need for lock ordering (they do not; higher isolation levels just throw serialization failure errors, requiring complex application-level retries).
* Lock ordering table rows arbitrarily based on execution paths, leading to random deadlock spikes under load.
* Forgetting that locks are acquired implicitly by `INSERT`, `UPDATE`, and `DELETE` queries, not just explicit `SELECT ... FOR UPDATE` statements.

#### Follow-up Questions
* What is the difference between a shared lock (select) and an exclusive lock (update) in PostgreSQL?
* How does PostgreSQL detect deadlocks, and what is the default deadlock detection timeout? (Postgres runs a background deadlock detection thread; default is `deadlock_timeout = 1s`).
* If Transaction A locks a row in `rooms`, and Transaction B wants to update an unrelated row in `room_players`, will Transaction B block? (No, locks are row-level, not table-level).

#### How DSAblitz demonstrates this concept
DSAblitz documents and enforces this rule in `PROJECT_CONTEXT.md` (lines 61-63). Any multi-table transaction (such as starting a battle or leaving/expiring rooms) locks rows according to this strict hierarchy. For instance, `rooms/service.go` locks the room and gets active players (locking `rooms` first, then `room_players`), before calling the battle coordinator to initialize battle players.

#### Relevant code references
* [service.go:L341-L417](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L341-L417) - StartBattle locking rooms first, then active players, then calling battle coordinator.
* [service.go:L102-L154](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L102-L154) - Battle service initializing `battles` and `battle_players` inside the shared transaction handle downstream.

#### Related documentation
* [deep-dives/transaction_boundaries.md](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)
* [database/transactions.md](file:///home/tanishq/dsablitz/docs/database/transactions.md)

---

### Q2: What is database connection pool starvation, and how does propagating a single transaction context prevent it?

#### Interviewer Intent
Assess knowledge of connection pool tuning, database drivers, context propagation, and how nested database transactions cause system-wide lockups.

#### Strong Answer
**Connection pool starvation** is a scenario where all available database connections in a server's connection pool are acquired and held by active transactions, and those transactions are blocked waiting for new connections to become available. This leads to a permanent hang or timeout.

This frequently happens when using **nested transactions**:
1. Suppose our connection pool limit is 10.
2. The Rooms service starts a transaction (`BEGIN`), acquiring **Connection 1**.
3. It performs some queries and then calls the Battle service.
4. If the Battle service starts its own independent transaction (`BEGIN`), it will request a new connection (**Connection 2**) from the pool.
5. If 10 concurrent requests execute this flow at the same time:
   * 10 parent transactions are started, consuming **all 10 connections** from the pool.
   * None of these 10 active connections can proceed because they are blocked waiting to acquire a second connection from the pool to execute the Battle transaction.
   * The system is deadlocked (starved).

To prevent connection pool starvation and enforce atomic boundaries in our modular monolith, we use the **Shared Transaction Context** pattern:
* Services do not open nested transactions.
* The calling service starts the transaction using `repo.WithTransaction(...)` and receives a transaction context handler `tx pgx.Tx`.
* Any cross-module method signatures accept this `tx` handle:
  ```go
  StartBattle(ctx context.Context, tx pgx.Tx, ...)
  ```
* All downstream query executions use this shared `tx` handle. This ensures that the entire request executes on **exactly one database connection**, preventing starvation, reducing transaction overhead, and guaranteeing atomicity.

#### Common Mistakes
* Starting new transaction blocks (`tx.Begin()`) inside an existing transaction callback, which PostgreSQL does not support natively (it requires savepoints) and consumes extra connections if done via a pool.
* Failing to pass the parent `tx` handle to a downstream method, causing that query to execute outside the transaction block, leading to dirty reads or isolation leaks.
* Allocating connection pools that are too small for the thread/goroutine count without setting appropriate connection acquisition timeouts.

#### Follow-up Questions
* How do PostgreSQL Savepoints differ from full transactions?
* What parameters in Go's `pgxpool.Config` control the minimum and maximum connection limits? (e.g., `MaxConns`, `MinConns`).
* What happens to a shared transaction connection if a network partition occurs mid-request? (The connection is closed, and the database automatically rolls back the uncommitted transaction).

#### How DSAblitz demonstrates this concept
DSAblitz enforces the Transaction Boundary Rule. Replaced nested transactions by propagating the parent transaction context (`pgx.Tx`) through `BattleCoordinator` into `StartBattleTx`. The database connection is preserved throughout the cross-module call, resolving connection pool deadlock issues.

#### Relevant code references
* [service.go:L12-L13](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L12-L13) - Import of `pgx`.
* [service.go:L406-L414](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L406-L414) - Rooms service passing `tx pgx.Tx` into `battleCoordinator.StartBattle`.
* [service.go:L102-L103](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L102-L103) - Battle service receiving `tx pgx.Tx` and executing queries using it.

#### Related documentation
* [deep-dives/transaction_boundaries.md](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)
* [deep-dives/room_transactions.md](file:///home/tanishq/dsablitz/docs/deep-dives/room_transactions.md)

---

## Key Takeaways
* **Circular Deadlocks**: Occur when transactions acquire locks on multiple tables in conflicting orders. Enforcing a global lock hierarchy eliminates this issue.
* **Shared Context**: Passing `pgx.Tx` across module boundaries coordinates multiple updates in a single connection, avoiding connection pool deadlocks and starvation.
* **Decoupled Architecture**: Using interfaces like `BattleCoordinator` prevents circular imports while maintaining transactional boundaries across domain boundaries.

## Interview Questions
1. Why is lock order consistency important across all services in a database-backed system?
2. What is database connection pool starvation, and how do nested transactions trigger it?
3. How do you design Go interfaces to support transactional mutations across module boundaries?

## Common Mistakes
* Opening nested transaction blocks in sub-services, exhausting database connections.
* Neglecting the lock hierarchy, causing cyclic deadlock errors that are difficult to reproduce in local environments.
* Storing transaction objects in struct fields instead of passing them explicitly through parameters, creating goroutine race conditions.

## Related Documents
* [database/transactions.md](file:///home/tanishq/dsablitz/docs/database/transactions.md)
* [deep-dives/transaction_boundaries.md](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)
* [architecture/module_boundaries.md](file:///home/tanishq/dsablitz/docs/architecture/module_boundaries.md)

## Lessons Learned
* Ensure that the parent connection handle is always passed explicitly to all query executors to preserve transaction boundaries.
* Document a clear order of operations for multi-table transactions to prevent deadlocks from occurring in production under heavy loads.
