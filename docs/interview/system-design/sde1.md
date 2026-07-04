# System Design - SDE1 Production Level

This document contains detailed system design interview notes for SDE1 level roles, focusing on concurrency control, pessimistic row locking, and modular monolith dependency inversion.

---

## Q&A Set 1: Concurrency Control in Rapid-Fire Gameplay

### 1. Interviewer Intent
The interviewer wants to assess if the candidate can design concurrency controls to protect player progress and scores under high concurrent write loads (like multiple players submitting answers in the same millisecond). They want to check if the candidate understands row-level locking and client-server sequence validation.

### 2. Strong Answer
To secure player progression and prevent double scoring or out-of-order execution, we implement **Pessimistic Row-Level Locking** alongside client-side synchronization counters.

When a client submits an answer, the backend begins a transaction and locks the specific player's row in the `battle_players` table:
```sql
SELECT * FROM battle_players WHERE battle_id = $1 AND user_id = $2 FOR UPDATE
```
This serializes any other incoming submissions for the same player, blocking concurrent operations until the first transaction commits or rolls back.

Inside this lock boundary, we verify a client-supplied monotonic `submissionIndex` against the server's expected index ($\text{correct\_count} + \text{incorrect\_count} + 1$).
If the client index is less than expected, we reject it as a duplicate. If it is greater, we reject it as an out-of-order submission. We also verify the uniqueness of the answer against previous submissions for that specific question. Once validated, the server updates progression indices, writes the answer log to an append-only `submissions` table, and commits the transaction.

### 3. Common Mistakes
* Relying on client-supplied timestamps to order submissions, which are easily spoofed by players or skewed by network latency.
* Running validations outside the database transaction, creating race conditions where two concurrent requests see the same state and execute double writes.
* Querying or updating the heavy `questions` catalog table under active gameplay transaction locks, causing lock escalation and database thread exhaustion.

### 4. Follow-up Questions
* **Why not use Optimistic Concurrency Control (OCC) with version columns?**
  * *Answer*: OCC works well when writes are rare and conflicts are low. In a 1v1 rapid-fire battle, write contention on the player scorecard is high. OCC would cause frequent serialization failures and transaction retries, increasing latency. Pessimistic locking blocks concurrent requests, keeping execution time low.

### 5. How DSAblitz demonstrates this concept
The `battle.Service` coordinates answer validation and scoring inside a row-locked transaction block in `SubmitAnswer`.

### 6. Relevant code references
* Pessimistic row locking and index validation: [service.go:L193-L245](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L193-L245)
* Database lock query: [repository.go:L119-L128](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L119-L128)

### 7. Related documentation
* [Submission Lifecycle](file:///home/tanishq/dsablitz/docs/deep-dives/submission_lifecycle.md)
* [Idempotency and Submissions](file:///home/tanishq/dsablitz/docs/deep-dives/idempotency.md)

---

## Q&A Set 2: Dependency Inversion for Module Boundaries

### 1. Interviewer Intent
The interviewer wants to assess if the candidate understands clean architecture principles, modular design, and how to prevent circular dependency problems in Go modular monoliths.

### 2. Strong Answer
In a modular monolith, clean boundaries prevent code entanglement. For example, the `rooms` module coordinates lobbies, while the `battle` module handles gameplay execution. To launch a match, the `rooms` service must trigger battle initialization.

However, if `rooms` imports `battle` directly, and `battle` needs to import `rooms` to update lobby states later, a circular dependency occurs. Go prohibits circular imports and fails compilation.

We resolve this using the **Dependency Inversion Principle**. The `rooms` module defines an abstract `BattleCoordinator` interface, declaring only the methods it needs. The `battle` module adapter implements this interface. At startup, the server boots both services and injects the adapter into the `rooms` service. The `rooms` module remains completely unaware of the concrete `battle` package, maintaining clear boundaries.

```
┌──────────────┐             ┌─────────────────────┐
│    rooms     │────────────►│  BattleCoordinator  │ (Interface)
└──────────────┘             └─────────────────────┘
                                        ▲
                                        │ (Implements)
                             ┌─────────────────────┐
                             │  battleCoordinator  │ (Adapter)
                             │       Adapter       │
                             └──────────┬──────────┘
                                        │ (Calls)
                                        ▼
                             ┌─────────────────────┐
                             │    battle.Service   │
                             └─────────────────────┘
```

### 3. Common Mistakes
* Defining interfaces in the package that *implements* them rather than the package that *uses* them, violating the dependency inversion flow.
* Creating cross-module transaction hooks that open separate database connections, leading to partial-failure states if one transaction rolls back.
* Allowing direct imports of repository packages across modules, bypassing domain service gates and exposing internal tables.

### 4. Follow-up Questions
* **How do we handle atomic database updates across module boundaries?**
  * *Answer*: The interface methods accept the parent transaction handle (`pgx.Tx`). This allows the secondary module to execute database writes on the same connection, ensuring both modules commit or rollback together.

### 5. How DSAblitz demonstrates this concept
In `rooms.Service`, the `battleCoordinator` interface coordinates battle launches. The concrete wiring is implemented in `server.registerRoutes` using `battleCoordinatorAdapter`.

### 6. Relevant code references
* Dependency adapter implementation: [routes.go:L21-L35](file:///home/tanishq/dsablitz/backend/internal/server/routes.go#L21-L35)
* Interface definition in Rooms: [models.go:L121-L124](file:///home/tanishq/dsablitz/backend/internal/rooms/models.go#L121-L124)
* Calling the coordinator within room transactions: [service.go:L406-L414](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L406-L414)

### 7. Related documentation
* [Modular Monolith Design ADR](file:///home/tanishq/dsablitz/docs/adr/0001_modular_monolith_design.md)
* [Module Boundary Interactions](file:///home/tanishq/dsablitz/docs/architecture/module_interactions.md)

---

## Key Takeaways
1. **Pessimistic row locking** (`FOR UPDATE`) serializes concurrent updates to sensitive rows, preventing race conditions.
2. **Monotonic index validation** blocks duplicate or out-of-order updates at the application layer.
3. **Dependency inversion interfaces** decouple modules, preventing circular imports in Go.

---

## Interview Questions
* **How does PostgreSQL handle locking when multiple requests target the same row concurrently?**
  * *Answer*: The first transaction acquires the lock and proceeds. Subsequent transactions block on the `FOR UPDATE` query until the first transaction commits or rolls back.
* **Why should interfaces be owned by the consumer package?**
  * *Answer*: The consumer package defines what it needs from the external dependency. This decouples the consumer from specific implementations, allowing easy adapter swaps.

---

## Common Mistakes
* **No transaction context propagation**: Calling another module's database methods without passing the `pgx.Tx` handle, resulting in orphaned records.
* **Non-deterministic row locking**: Locking multiple records without sorting their IDs first, causing deadlocks under high load.

---

## Related Documents
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
* [Room Transactions Deep Dive](file:///home/tanishq/dsablitz/docs/deep-dives/room_transactions.md)

---

## Lessons Learned
* **Shared transaction context**: Initially, the battle setup was executed in a separate transaction from room creation, leading to orphaned rooms when the battle insert failed. We resolved this by passing the transaction context through the `BattleCoordinator` adapter interface.
