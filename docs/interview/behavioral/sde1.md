# Behavioral - SDE1 Production Level

This document contains behavioral interview notes and scenarios for SDE1 level candidates, focusing on leading technical debates and identifying and resolving system-level technical debt.

---

## Scenario 1: Debate on Pessimistic Locking vs Optimistic Concurrency Control

### 1. Interviewer Intent
The interviewer wants to assess the candidate's analytical skills, ability to weigh engineering tradeoffs under different workloads, and communication style when debating technical approaches with peers.

### 2. Strong Answer
During the design of the gameplay battle engine, my team debated whether to use **Optimistic Concurrency Control (OCC)** or **Pessimistic Row-Level Locking** (`FOR UPDATE`) to protect player progression and scores.

A peer argued for OCC, pointing out that it doesn't block database connections and is easier to implement using version check columns. I agreed that OCC is lightweight, but pointed out that in a 1v1 rapid-fire coding match, write contention on the player scorecard is high. Both players submit answers rapidly, and rating updates occur concurrently.

Under high contention, OCC leads to frequent write conflicts, triggering retry loops that increase latency and CPU load. I presented these database patterns to the team, and we reached a consensus:
* Use **Pessimistic Row Locking** (`FOR UPDATE`) for active gameplay writes (`battle_players`) to serialize score updates and keep latency low.
* Use **Stateless In-Memory Caching** for static catalog reads (`questions`) to eliminate read contention entirely.

This hybrid approach optimized both write safety and read throughput.

### 3. Common Mistakes
* Claiming one technology or locking pattern is "always better" without evaluating the application's workload and contention patterns.
* Getting personal or stubborn during the debate rather than using data and engineering tradeoffs to reach a consensus.
* Ignoring database connection pool metrics and lock wait durations when advocating for pessimistic locking.

### 4. Follow-up Questions
* **How did you ensure the pessimistic locks did not cause database deadlocks?**
  * *Answer*: We established a global lock ordering hierarchy (e.g. `rooms` -> `battles` -> `battle_players`) and sorted player IDs deterministically before applying locks.

### 5. How DSAblitz demonstrates this concept
Pessimistic row locking is applied to player scorecards in `SubmitAnswer` to serialize writes, while the static question bank is read from an in-memory cache.

### 6. Relevant code references
* Pessimistic lock on scorecard: [service.go:L193-L198](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L193-L198)
* Database lock query: [repository.go:L119-L128](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L119-L128)

### 7. Related documentation
* [Database Transaction Strategy](file:///home/tanishq/dsablitz/docs/database/transactions.md)
* [Questions Cache Design](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)

---

## Scenario 2: Resolving Technical Debt in Transaction Blocks

### 1. Interviewer Intent
The interviewer wants to see if the candidate takes initiative in identifying design flaws, understands the operational risks of technical debt, and can propose and execute clean refactoring plans.

### 2. Strong Answer
In our matchmaking module, creating a room involves generating a unique 6-character code. If a code collision occurs (the code already exists), the server should generate a new code and try again.

Initially, this retry loop ran inside the database transaction block. However, PostgreSQL aborts the entire transaction on any query error (like a unique constraint violation or search error). Running retry loops inside an aborted transaction caused subsequent queries to fail, crashing the connection.

I identified this technical debt and proposed a refactoring plan:
1. Move the retry loop **outside** of the database transaction block.
2. In each iteration, start a fresh, clean transaction to check for collisions and insert the room.
3. If a collision is detected, roll back the transaction, increment the retry counter, generate a new code, and start a new transaction.

This resolved the aborted transaction issue and made the room generation engine reliable.

### 3. Common Mistakes
* Postponing critical reliability refactors in favor of shipping new features, leaving severe database bugs in production.
* Writing complex workaround code in the application layer instead of fixing the root database transaction boundaries.
* Not documenting technical debt, leaving other developers unaware of transaction lifecycle constraints.

### 4. Follow-up Questions
* **What is the limit of your retry loop, and what happens if all retries fail?**
  * *Answer*: The loop runs up to 3 times. If all retries fail, the system returns a clear error to the client, preventing infinite loops and database resource exhaustion.

### 5. How DSAblitz demonstrates this concept
In `rooms.Service.CreateRoom`, the room code generation loop runs outside the transaction block, starting a clean transaction for each attempt.

### 6. Relevant code references
* CreateRoom outside-transaction retry loop: [service.go:L56-L113](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L56-L113)

### 7. Related documentation
* [PROJECT_CONTEXT.md Postgres Retry Boundary Rule](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L74-L75)
* [Room Transactions Deep Dive](file:///home/tanishq/dsablitz/docs/deep-dives/room_transactions.md)

---

## Key Takeaways
1. **Pessimistic locking** is preferred over OCC under high-contention write workloads to minimize transaction retries and keep latency low.
2. **PostgreSQL aborts transactions permanently** on any query error; retry loops must execute outside the transaction block.
3. **Collaborative debates** are best resolved by evaluating engineering tradeoffs against specific application workloads.

---

## Interview Questions
* **Why does PostgreSQL reject queries executed after a transaction has encountered an error?**
  * *Answer*: PostgreSQL enters an aborted state to protect data consistency, requiring a `ROLLBACK` to clear the state.
* **How do we avoid lock escalation in high-traffic databases?**
  * *Answer*: Enforce indexing on locked columns, keep transactions short, and release locks quickly.

---

## Common Mistakes
* **Retrying inside aborted transactions**: Running retry loops within a failed PostgreSQL transaction block, causing subsequent queries to fail.
* **Underestimating write contention**: Using OCC under high concurrent write loads, leading to latency spikes due to collision retries.

---

## Related Documents
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
* [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)

---

## Lessons Learned
* **Postgres Retry Boundary Rule**: Moving the room code generation retry logic outside of the transaction block resolved database connection errors and stabilized the matchmaking lobby creation.
