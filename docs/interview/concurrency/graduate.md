# Concurrency Controls - Graduate Level Interview Prep

This guide covers basic race conditions in multiplayer systems, optimistic versus pessimistic locking paradigms, and using single-row pessimistic locks to prevent double-submissions or score tampering.

---

## Q&A Sets

### Q1: What is a race condition in a real-time multiplayer system, and what specific bugs can occur if concurrency is not controlled?

#### Interviewer Intent
Assess the candidate's understanding of concurrent state mutation issues, the stateless/stateful nature of web applications, and typical vulnerabilities in competitive online platforms.

#### Strong Answer
A race condition occurs when multiple threads or concurrent requests attempt to read and write to the same shared resource simultaneously, and the final state of the resource depends on the order or timing of execution.

In a real-time multiplayer competitive platform like DSAblitz, failing to handle concurrency can result in several severe bugs:
1. **Duplicate Submissions**: A player double-clicks the "Submit" button for a question. If the server processes both requests concurrently, it might read the player's progression state twice before updating it. As a result, the player could get points twice for the same correct answer, or advance past multiple questions incorrectly.
2. **Score Tampering**: If two players in a battle submit answers at the exact same moment, and the server updates their scores in parallel without locks, one update might overwrite the other (the "Lost Update" problem), leading to incorrect final scores.
3. **Lobby Overfill**: A game lobby has a maximum capacity of 2 players. If two players click "Join" at the exact same time, and the server reads the lobby size as 1 for both requests before committing the inserts, both joins will succeed, resulting in 3 players in a 1v1 lobby.

#### Common Mistakes
* Assuming that single-threaded web frameworks (like Node.js) or standard Go goroutines are immune to race conditions (they are not; even if the server code runs concurrently, the database is shared and prone to race conditions).
* Believing that client-side validation (like disabling a button after clicking) is sufficient to prevent concurrent request abuses (attackers can bypass clients and send raw concurrent HTTP requests).
* Thinking that database auto-increment fields resolve all concurrency issues.

#### Follow-up Questions
* What is the "Lost Update" problem in databases, and how does it happen?
* How does client-side request rate-limiting interact with backend concurrency controls?
* If an API endpoint is stateless, does it mean we don't have to worry about race conditions? (No, because the state is stored in the shared database).

#### How DSAblitz demonstrates this concept
DSAblitz controls concurrency in `SubmitAnswer` inside `backend/internal/battle/service.go`. It rejects duplicate submissions by checking a client-supplied monotonic submission index against the player's server-computed expected index, and locks the player row to prevent double-submits.

#### Relevant code references
* [service.go:L226-L233](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L226-L233) - Expected submission index check.
* [service.go:L193-L198](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L193-L198) - Pessimistic row locking on player progression state.

#### Related documentation
* [deep-dives/idempotency.md](file:///home/tanishq/dsablitz/docs/deep-dives/idempotency.md)
* [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)

---

### Q2: Compare Optimistic Locking and Pessimistic Locking. What are their trade-offs, and why does our application choose pessimistic row locking?

#### Interviewer Intent
Verify the candidate's understanding of database locking paradigms, performance trade-offs under high write contention, and transaction isolation.

#### Strong Answer
Optimistic and pessimistic locking are two strategies used to prevent conflicting updates in a database:

| Feature | Optimistic Locking | Pessimistic Locking |
| :--- | :--- | :--- |
| **Mechanism** | Uses a version column or timestamp. When updating, it checks: `WHERE version = expected_version`. If another transaction updated the row, the version won't match, and the update fails, requiring an application-level retry. | Acquires a database lock on the row immediately when reading (e.g., `SELECT ... FOR UPDATE`). Any concurrent transaction attempting to read or write the same row is blocked until the locking transaction commits or rolls back. |
| **Contention** | Best for **low contention** (reads are frequent, writes rarely conflict). | Best for **high contention** (frequent writes on the same rows). |
| **Performance** | No locking overhead; fast execution if conflict rates are low. | Higher lock acquisition overhead; blocks concurrent readers/writers. |
| **Failure Handling** | Fails at commit time. Application must handle retries explicitly. | Fails or blocks at read time. Database serializes access automatically. |

In DSAblitz, gameplay updates are **highly contentious**: players submit answers in rapid succession in a short duration (5-10 minutes). If we used optimistic locking, high-speed clicking or rapid sub-second submissions would frequently fail version checks, leading to high transaction rollback rates and constant application-level retry overhead, degrading gameplay responsiveness. 

By choosing **pessimistic row locking** (using `SELECT ... FOR UPDATE` on `battle_players`), we serialize the player's submission updates. Concurrent requests wait in a queue for the lock, guaranteeing that each submission executes against a stable, locked state, preventing conflicts and retries.

#### Common Mistakes
* Recommending optimistic locking for real-time multiplayer systems without considering the CPU and network overhead of constant retry loops under high contention.
* Believing that pessimistic locking blocks all database reads (it only blocks other pessimistic reads/writes; standard `SELECT` queries that do not request a lock can still read the row, depending on isolation level).
* Forgetting that pessimistic locks must be executed inside a database transaction block to be effective.

#### Follow-up Questions
* What is a "phantom read", and how does it differ from a non-repeatable read?
* What happens if a transaction holding a pessimistic lock takes too long to execute? (It blocks other transactions, leading to connection exhaustion or timeouts).
* How do you configure lock timeouts in PostgreSQL to prevent infinite blocking? (Using `SET local lock_timeout`).

#### How DSAblitz demonstrates this concept
DSAblitz enforces pessimistic locking in the database layer. In `backend/internal/battle/repository.go`, `GetBattlePlayerForUpdate` uses `FOR UPDATE` to lock the target player's row inside the current transaction block.

#### Relevant code references
* [repository.go:L119-L128](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L119-L128) - `GetBattlePlayerForUpdate` utilizing `FOR UPDATE`.
* [service.go:L193-L198](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L193-L198) - Acquiring the pessimistic lock during answer submissions.

#### Related documentation
* [database/transactions.md](file:///home/tanishq/dsablitz/docs/database/transactions.md)
* [deep-dives/transaction_boundaries.md](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)

---

## Key Takeaways
* **Race Conditions in Games**: Multi-threaded game loops and concurrent API requests can lead to double scoring, lobby overfill, and lost updates if not protected.
* **Pessimistic Locking (`SELECT FOR UPDATE`)**: Ideal for high-contention scenarios like real-time quizzes, as it serializes updates and prevents the overhead of optimistic retry failures.
* **Client Validation Is Insufficient**: Backend state validation (e.g. index checks) and database locks are required to guarantee system integrity against attackers or lag.

## Interview Questions
1. Why is pessimistic locking preferred over optimistic locking in a high-contention real-time battle system?
2. What database query clause is used in PostgreSQL to acquire a pessimistic lock on a row?
3. How can double-submitting a quiz answer lead to score corruption if no locks are applied?

## Common Mistakes
* Relying solely on frontend UI button disabling to prevent double-form submission.
* Believing that using Go channels or sync mutexes on a single server instance is sufficient to prevent race conditions when running multiple instances of a service.
* Acquiring pessimistic locks without executing them inside a database transaction block.

## Related Documents
* [database/transactions.md](file:///home/tanishq/dsablitz/docs/database/transactions.md)
* [deep-dives/transaction_boundaries.md](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)
* [flows/submission_flow.md](file:///home/tanishq/dsablitz/docs/flows/submission_flow.md)

## Lessons Learned
* In high-concurrency systems, always serialize writes on specific entities using row-level locks to maintain strict consistency.
* Validate state invariants (like index counters) server-side inside locked transaction blocks to catch out-of-order execution or malicious requests.
