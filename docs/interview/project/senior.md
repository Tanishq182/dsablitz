# DSAblitz Interview Prep: Senior SDE / Tech Lead Level

This document covers advanced database transaction boundaries, concurrency controls, deadlock prevention algorithms, and transaction recovery patterns in DSAblitz. It is tailored for Senior SDE and Tech Lead interview preparation.

---

## 🗂️ Q&A Sets

### Q1: Discuss the Transaction Boundary Rule and Shared Transaction Context in DSAblitz. How does propagating `pgx.Tx` across modules prevent connection pool starvation and database inconsistencies?

#### Interviewer Intent
The interviewer is checking your depth of knowledge regarding transactional boundaries, database connection pool management, and how to prevent silent partial-state commits (write skew/inconsistencies) or connection pool deadlocks in a modular monolith.

#### Strong Answer
In a modular monolith, operations often span multiple packages. For example, when launching a match, the system must update the room status (`waiting`/`ready` -> `in_battle` in the `rooms` module) and initialize the match progression records (inserting into `battles`, `battle_players`, and `battle_question_sequence` in the `battle` module) as a single atomic unit. 

If these writes occurred in separate transactions:
1. A database failure or application crash after updating the room status but before inserting the battle records would leave the database in an inconsistent state (a room marked `in_battle` without an associated battle record).
2. The players would be locked in a state they cannot progress through or leave.

To solve this, we enforce the **Transaction Boundary Rule** using a **Shared Transaction Context**:
* Instead of opening nested transactions in each service, inter-module interfaces accept a transaction handle (`pgx.Tx`) as an argument.
* The rooms service initiates the transaction: `s.repo.WithTransaction(ctx, func(tx pgx.Tx) error { ... })`.
* It performs room status writes, then passes the transaction handle `tx` to the `BattleCoordinator.StartBattle` interface call.
* The adapter routes the transaction context down into `battleService.StartBattleTx(ctx, tx, ...)` where the battle tables are inserted using the **same database connection**.

**Why is this critical for connection pool scalability?**
If the battle module opened its own transaction internally, it would request a second connection from the `pgxpool` connection pool. If the database pool is under heavy load and has no free connections, the battle service will block waiting for a connection to become available. However, the outer room transaction is holding its connection open while waiting for the battle service to return. This creates a **connection pool deadlock** (connection starvation) that hangs the entire application. Propagating the `pgx.Tx` context ensures all writes execute over a single connection, avoiding starvation and ensuring atomicity.

#### Common Mistakes
* **Using nested transactions**: Opening a new transaction block (`WithTransaction`) inside `battle.Service` when it is called from `rooms.Service`. This duplicates connection allocations and causes deadlocks.
* **Leaking `pgx.Tx` blindly**: Exposing raw database connections to controllers or client handlers. The transaction boundary must start and end strictly within the business logic service layer.

#### Follow-up Questions
* How does the system roll back the parent transaction if the battle service returns an error?
* How would you adapt this transaction context pattern if you transitioned the modular monolith to distributed microservices?

#### How DSAblitz demonstrates this concept
In `rooms/service.go`, `StartBattle` wraps all operations in a transaction block and passes `tx` into `battleCoordinator.StartBattle` which maps to `StartBattleTx` in `battle/service.go`.

#### Relevant code references
* [service.go:L406-L414](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L406-L414): Invoking `battleCoordinator.StartBattle` within the transaction block.
* [service.go:L103-L154](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L103-L154): The `StartBattleTx` execution block writing battle structures using the passed `tx pgx.Tx` handle.

#### Related documentation
* [PROJECT_CONTEXT.md:L67-L70](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L67-L70)
* [transaction_boundaries.md](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)
* [room_transactions.md](file:///home/tanishq/dsablitz/docs/deep-dives/room_transactions.md)

---

### Q2: Explain how DSAblitz prevents race conditions and deadlocks under high concurrency. Discuss the Global Lock Ordering Rule and the Deterministic Row Locking Rule.

#### Interviewer Intent
The interviewer wants to evaluate your understanding of locking mechanics, database deadlock conditions (mutual exclusion, hold and wait, no preemption, circular wait), and how to programmatically write deadlock-free locking queries.

#### Strong Answer
In a real-time multiplayer application, concurrent requests modify the same entities (e.g., players submitting answers, leaving rooms, or completing battles simultaneously). We use pessimistic locking (`SELECT ... FOR UPDATE`) to block concurrent writes and prevent race conditions. However, acquiring locks on multiple rows across different tables can cause deadlocks if transactions acquire them in different orders.

To guarantee deadlock-free execution under load, we enforce two concurrency design rules:

#### 1. Global Lock Ordering Rule
Any transaction that acquires locks across multiple tables must lock them in a strict top-down order. The global lock hierarchy in DSAblitz is:
$$\text{rooms} \rightarrow \text{room\_players} \rightarrow \text{battles} \rightarrow \text{battle\_players} \rightarrow \text{battle\_question\_sequence}$$
By mandating this sequence, we break the circular wait condition. For example, a transaction initializing a battle locks the parent `rooms` row first, then the `room_players` rows, before inserting the `battles` rows.

#### 2. Deterministic Row Locking Rule
When locking multiple rows in the **same table** (such as batch cleanups or locking both players in a battle), rows must be locked in a sorted, deterministic order (e.g., `ORDER BY id ASC`).
* If Player 1 and Player 2 are in a match, and Transaction A locks Player 1 then Player 2, while Transaction B locks Player 2 then Player 1, they will deadlock.
* To prevent this in `CompleteBattle`, the players are loaded and locked using a query sorted by `user_id ASC`.
* In `ExpireRooms`, the cleanup task locks expired lobbies using `ORDER BY id ASC` before applying `FOR UPDATE`:
  ```sql
  SELECT id FROM rooms
  WHERE status IN ('waiting', 'ready') AND expires_at < NOW()
  ORDER BY id ASC
  FOR UPDATE
  ```

Sorting the target IDs before locking guarantees that concurrent transactions lock rows in the exact same sequence, eliminating circular deadlocks.

#### Common Mistakes
* **Locking parent tables after child tables**: Writing business logic that locks `battle_players` and subsequently locks `battles` or `rooms` to update statuses.
* **Unordered batch locking**: Executing a query like `SELECT * FROM battle_players WHERE battle_id = $1 FOR UPDATE` without sorting. If the database engine returns rows in a non-deterministic order (due to index changes or updates), a deadlock can occur.

#### Follow-up Questions
* Why does the `SubmitAnswer` function only lock the individual `battle_players` row instead of the entire table or match?
* How does Postgres handle deadlock detection internally, and how does it choose which transaction to abort?

#### How DSAblitz demonstrates this concept
In `rooms/service.go` and `battle/service.go`, transactions obtain lock ordering sequentially, and repository cleanups sort primary keys before locking.

#### Relevant code references
* [service.go:L427-L464](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L427-L464): `ExpireRooms` sorting room IDs ASC before locking `FOR UPDATE`.
* [service.go:L193-L197](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L193-L197): Row locking player records in `SubmitAnswer`.
* [service.go:L314-L318](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L314-L318): Loading and locking players in `CompleteBattle` (where the repository query enforces `ORDER BY user_id ASC` before locking).

#### Related documentation
* [PROJECT_CONTEXT.md:L61-L63](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L61-L63)
* [PROJECT_CONTEXT.md:L71-L72](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L71-L72)

---

### Q3: What is the Postgres Abort & Retry Boundary Rule? Explain why transaction retries must be positioned outside the transaction block.

#### Interviewer Intent
The interviewer wants to see if you understand the internal states of PostgreSQL transactions and database drivers, particularly how transaction invalidation occurs upon error and how to structure robust retry logic.

#### Strong Answer
In PostgreSQL, transactions are highly sensitive to errors. If any query within a transaction block encounters a failure (e.g., a primary key duplicate violation, serialization failure, or lock timeout), the PostgreSQL server marks the entire transaction as permanently aborted. 

Once aborted, the connection rejects all subsequent SQL commands within that transaction block, returning the error: `ERROR: current transaction is aborted, commands ignored until end of transaction block`. The transaction can only be rolled back.

This characteristic dictates the **Postgres Abort & Retry Boundary Rule**:
> Any loop designed to retry an operation upon failure (such as handling primary key collisions or optimistic locking conflicts) **must execute outside the transaction block**.

If the retry loop is inside the transaction block:
1. The first query failure aborts the transaction.
2. The next iteration of the loop tries to run a query on the same connection in the same transaction block.
3. This query fails immediately with the "current transaction is aborted" error, making the retry loop completely useless.

In DSAblitz, when creating a room:
1. We generate a random 6-character room code. There is a small chance of code collision.
2. The retry loop runs up to 3 times **outside** the transaction.
3. Inside each loop iteration, we open a new transaction `repo.WithTransaction`.
4. If `InsertRoom` returns a collision error, the transaction rolls back, releasing the connection and clearing the transaction state.
5. The loop moves to the next iteration and initiates a **brand new transaction** on a clean connection.

```mermaid
graph TD
    subgraph Bad Design: Retry Inside Transaction
        TxStart1[Begin Transaction] --> Loop1[Start Retry Loop]
        Loop1 --> Query1[Insert Room Code]
        Query1 -- Collision Error --> Abort1[Postgres Aborts Tx]
        Abort1 --> LoopNext1[Retry Attempt 2]
        LoopNext1 --> Query2[Insert Different Room Code]
        Query2 --> Fails1[FAILS IMMEDIATELY: Transaction is aborted]
    end
    
    subgraph Correct Design: Retry Outside Transaction
        Loop2[Start Retry Loop] --> TxStart2[Begin Fresh Transaction]
        TxStart2 --> Query3[Insert Room Code]
        Query3 -- Collision Error --> Rollback2[Rollback Transaction]
        Rollback2 --> LoopNext2[Retry Attempt 2]
        LoopNext2 --> TxStart3[Begin Brand New Transaction]
        TxStart3 --> Query4[Insert Different Room Code]
        Query4 -- Success --> Commit[Commit Transaction]
    end
```

#### Common Mistakes
* **Placing the retry loop inside the transaction callback**: Putting a loop inside Go's `db.WithTransaction(func(tx pgx.Tx) { ... retry loop ... })` block. The connection's transaction is aborted on the first failure, causing all subsequent loop iterations to fail.
* **Leaking aborted transaction connections**: Failing to roll back an aborted transaction before returning a connection to the pool, which can corrupt the connection state for the next lease.

#### Follow-up Questions
* What is the performance overhead of starting and rolling back transactions during a collision retry?
* How does pgx manage rolling back transactions on context cancellations?

#### How DSAblitz demonstrates this concept
In `rooms/service.go`, the `CreateRoom` method implements the 3-attempt room code generation retry loop outside of the `repo.WithTransaction` call.

#### Relevant code references
* [service.go:L56-L113](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L56-L113): The retry loop in `CreateRoom` spanning outside of `s.repo.WithTransaction`.

#### Related documentation
* [PROJECT_CONTEXT.md:L74-L76](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L74-L76)
* [room_state_machine.md](file:///home/tanishq/dsablitz/docs/deep-dives/room_state_machine.md)

---

## 📌 Key Takeaways
* **Shared Tx Context**: Propagating `pgx.Tx` avoids nested transactions, prevents connection pool starvation, and guarantees multi-module atomicity.
* **Deterministic Row Locking**: Sorting records (e.g. `ORDER BY id ASC`) before locking `FOR UPDATE` prevents deadlock cycles.
* **Outside Retry Boundary**: Retries on transaction failures must run outside the transaction block because PostgreSQL invalidates transactions immediately upon error.

## ❓ Interview Questions
1. Describe how a connection pool deadlock occurs, and how you would architecture system services to prevent it.
2. How does the order of table row locking affect the occurrence of database deadlocks?
3. Why does PostgreSQL reject further queries inside a transaction once a single query fails?

## ⚠️ Common Mistakes
* Opening multiple transaction blocks concurrently on the same runtime thread, leading to connection exhaustion.
* Batch processing records with database locks without sorting their IDs, resulting in random production deadlocks.

## 🔗 Related Documents
* [transaction_boundaries.md](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)
* [room_transactions.md](file:///home/tanishq/dsablitz/docs/deep-dives/room_transactions.md)

## 💡 Lessons Learned
* System scalability depends heavily on keeping transaction lifecycles short and database connection counts minimal.
* Pessimistic concurrency controls must be paired with strict locking hierarchies and deterministic record sorting to maintain runtime stability.
