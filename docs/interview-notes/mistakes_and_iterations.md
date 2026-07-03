# Engineering Log: Mistakes & Architectural Iterations

This log documents key structural modifications and refactoring iterations executed during the Questions and Battle modules implementation in Phase 5.

---

## 1. Relocating Active Gameplay State
* **Mistake**: The initial draft of the Questions module managed player pointers, submission logging, and battle sequence tracking.
* **Why it changed**: This violated the Single Responsibility Principle and module boundaries. The Questions module represents static, read-only configuration. Placing active session logs inside it coupled static lookups to mutable transactions.
* **Refactoring**: Created the stateful Battle Module. All mutable columns (`current_question_index`, `current_question_attempts`, `score`), sequence lists (`battle_question_sequence`), and audit tables (`submissions`) were moved to the Battle package. The Questions module became 100% stateless.

---

## 2. Transitioning to Type-Safe UUIDs
* **Mistake**: Question IDs and user relations were initially represented as `string` types in Go entities. Database lookups cast Postgres UUID columns to text (`id::TEXT`) to support Go scanning.
* **Why it changed**: Representing IDs as strings was error-prone and bypassed Go's compiler type checks. Cast operators in SQL queries bypass type validation and add string parsing overhead.
* **Refactoring**: Updated all database entities, DTO structs, repository scanners, and seeder parsers to use `github.com/google/uuid.UUID`. The pgx driver maps Postgres UUID types to this structure natively.

---

## 3. Refining Domain Invariant Separation
* **Mistake**: Initial validation rules (like checking if an MCQ question had exactly 4 options or if the correct answer existed inside the options list) were placed directly inside the `Validate()` method of the `Question` model.
* **Why it changed**: Checking if an MCQ has 4 options is an application-level business policy, not a structural model invariant. In the future, the product might allow 3-option or 5-option MCQs. Placing these checks in the entity itself prevents valid extensions.
* **Refactoring**: Kept only structural domain invariants in the model (e.g. valid UUID structure, non-empty prompt text, difficulty range 1-5). Moved options counts and answer key checks to the `seeder.go` ingestion pipeline.

---

## 4. Upsert Seeding vs. UNIQUE Constraint
* **Mistake**: The initial design proposed an `ON CONFLICT (title) DO UPDATE` seeding clause, requiring a `UNIQUE(title)` constraint on the `questions` table.
* **Why it changed**: Question titles are not guaranteed to be unique. Multiple questions might share the title "Binary Search Complexity" while having different difficulty levels or prompts.
* **Refactoring**: Eliminated the `UNIQUE(title)` database constraint. Assigned predefined, stable UUIDs to questions in the JSON seed file. The seeder resolves conflicts on the primary key `id` (`ON CONFLICT (id) DO UPDATE`), guaranteeing safe, idempotent seeding.

---

## 5. Propagating pgx.Tx to Battle Coordinator
* **Mistake**: The Battle Coordinator API didn't accept the parent transaction, which caused nested transactions on separate database connections.
* **Why it changed**: Nested transactions break ACID atomicity, leave orphaned battles if the parent transaction rolls back, and exhaust the pgxpool connection pool, leading to starvation deadlocks under high concurrency.
* **Refactoring**: Updated the `BattleCoordinator` interface to accept `pgx.Tx` and pass it down, and implemented `StartBattleTx` inside the Battle service to execute operations on the same parent database transaction connection.

---

## 6. Deterministic Row Locking in ExpireRooms
* **Mistake**: The `ExpireRooms` database query locked expired room rows using `FOR UPDATE` without an `ORDER BY` clause.
* **Why it changed**: Locking rows in non-deterministic order concurrently across multiple cleanups led to database deadlocks.
* **Refactoring**: Added `ORDER BY id ASC` before the `FOR UPDATE` locking clause to enforce a strict, deterministic lock acquisition order.

---

## 7. Moving Retry Loops Outside Transactions
* **Mistake**: The room code generation retry loop was executed inside the transaction block.
* **Why it changed**: PostgreSQL immediately aborts a transaction upon encountering a unique constraint violation, meaning subsequent retries inside the same transaction will fail with a transaction aborted error.
* **Refactoring**: Moved the retry loop outside the transaction boundary. Each retry attempt now starts a fresh, clean transaction block, allowing room code generation collisions to be resolved gracefully.

---

## 8. Restricting LeaveRoom during Battle
* **Mistake**: Players could call `LeaveRoom` to exit a room lobby while an active battle was underway, causing inconsistent state.
* **Why it changed**: It bypassed the Battle module's progression, scoring, and resignation flow.
* **Refactoring**: Added a guard condition in `LeaveRoom` to reject room exits if `room.Status` is `StatusInBattle`, forcing players to resign or abort using the Battle module instead.
