# PostgreSQL Database - SDE1 Level

This document provides production-level interview preparation material on PostgreSQL databases, focusing on data check constraints, data model consistency, and the application of partial (filtered) unique indexes.

---

## Q&A Sets

### Q1: What are check constraints, how do they differ from application-level validation, and how do they guarantee database-level integrity?

#### Interviewer Intent
The interviewer is looking to check:
- Understanding of defensive database design.
- Ability to analyze trade-offs between application-level checks and database-level constraints.
- Practical knowledge of using CHECK constraints for domain invariants (such as preventing self-friendship or validating state machines).

#### Strong Answer
A **CHECK constraint** is a database-level validation rule applied to one or more columns in a table. PostgreSQL evaluates this rule whenever a row is inserted or updated. If the condition evaluates to `FALSE`, the transaction aborts, the database rolls back any changes, and it returns an error to the application.

##### DB-Level vs Application-Level Validation
- **Application Validation**: Faster to evaluate (no DB roundtrip), and easy to format into user-friendly error messages. However, it can be bypassed by direct database access, admin scripts, batch migrations, or concurrent connection races.
- **Database CHECK Constraints**: Serve as the **final line of defense** for data integrity. They ensure that under no circumstances can invalid data (e.g. invalid status values, negative balances, self-referencing friendships) be written to the disk.

In a system like DSAblitz, CHECK constraints enforce business logic constraints:
- `friendships_distinct_users_check CHECK (requester_id <> addressee_id)`: Prevents a user from sending a friend request to themselves.
- `rooms_status_check CHECK (status IN ('waiting', 'ready', 'in_battle', 'closed', 'expired'))`: Enforces a strict state machine at the schema level.

#### Common Mistakes
- **Checking volatile functions**: Trying to use volatile functions (like `random()` or checking current time `now()`) inside CHECK constraints. PostgreSQL constraints must be deterministic.
- **Assuming CHECK constraints handle NULL values**: If a check constraint evaluates to `NULL` (e.g., comparing a nullable column like `CHECK (score >= 0)` when score is null), PostgreSQL treats it as passing. To prevent null values, you must pair the constraint with a `NOT NULL` modifier.
- **Putting complex cross-table checks in CHECK constraints**: CHECK constraints can only access columns in the current row. Attempting to validate conditions that span multiple tables requires triggers or transactions.

#### Follow-up Questions
1. What happens if you try to add a CHECK constraint to a table that already contains rows violating the new constraint? (The operation fails unless you specify `NOT VALID` to bypass check validation on existing rows).
2. How do you handle CHECK constraint violations in Go code? (By parsing the Postgres error code, typically `23514` for check violations, and returning custom application errors).

#### How DSAblitz demonstrates this concept
DSAblitz uses CHECK constraints to enforce values, status codes, and structural safety across users, rooms, battles, and friends.

#### Relevant code references
- `[000001_create_core_schema.up.sql:L24-L26](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L24-L26)`: Enforcing statuses and handle/display name length boundaries on the `users` table.
- `[000001_create_core_schema.up.sql:L53-L54](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L53-L54)`: Preventing self-friendship via `CHECK (requester_id <> addressee_id)`.
- `[000001_create_core_schema.up.sql:L68-L71](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L68-L71)`: Enforcing statuses, player capacity constraints, and game durations on the `rooms` table.

#### Related documentation
- [Database Schema](file:///home/tanishq/dsablitz/docs/database/schema.md)
- [Domain Invariants vs Room Rules](file:///home/tanishq/dsablitz/docs/deep-dives/domain_invariants_vs_room_rules.md)

---

### Q2: What is a Partial (Filtered) Index, and how does DSAblitz use it to enforce soft unique constraints on active states?

#### Interviewer Intent
The interviewer wants to see:
- Mastery of advanced indexing strategies in PostgreSQL.
- Understanding of partial unique indexes (`CREATE UNIQUE INDEX ... WHERE ...`).
- Ability to solve constraints where uniqueness is only required for "active" rows while allowing duplicates for "archived" or "historical" rows.

#### Strong Answer
A **Partial Index** is an index built over a subset of a table's rows, defined by a filter condition (e.g. `CREATE INDEX ... WHERE active = true`). This reduces the index size on disk, speeds up writes, and allows enforcing uniqueness selectively.

##### Enforcing "Soft" Uniqueness for Active States
In many applications, historical data must be preserved. For example, in a multiplayer lobby system:
- A user can join, leave, join a different seat, or get kicked.
- We want to record every state change in a table (e.g. `room_players`).
- **Core Rule**: A user can only occupy one seat at a time in a room, and a seat can only be occupied by one player at a time.
- **Problem**: Standard unique constraints like `UNIQUE(room_id, user_id)` prevent users from having historical records of joining and leaving the same room (since old records would conflict with new ones).

##### Solution: Partial Unique Indexes
We define a partial unique index that only applies to **active** players (e.g., players whose status is `joined` or `ready`):
```sql
CREATE UNIQUE INDEX idx_room_players_room_user_active 
ON room_players(room_id, user_id) 
WHERE status IN ('joined', 'ready');
```
PostgreSQL will only enforce uniqueness on rows matching the `WHERE` clause. A user can leave the room (transitioning status to `left`), and since `left` is not in the filter, another row for the same user joining that room can be inserted without violating the index.

```
Table: room_players
==============================================================
id  | room_id | user_id | seat | status   | Indexed?
--------------------------------------------------------------
U1  | Room_A  | User_9  | 1    | "left"   | No (Historic)
U2  | Room_A  | User_9  | 1    | "joined" | Yes (Active)
U3  | Room_A  | User_9  | 2    | "joined" | ERROR: Conflict!
```

#### Common Mistakes
- **Queries not using the partial index**: For the query optimizer to use a partial index, the query's `WHERE` clause must match or imply the index's filter. If you index `WHERE status IN ('joined', 'ready')` but query `WHERE status = 'left'`, Postgres cannot use that index.
- **Setting broad conditions**: Creating filters that change frequently (e.g. `WHERE expires_at > NOW()`). Since `NOW()` is non-deterministic, partial indexes cannot use dynamic functions; they must use static expressions.
- **Creating standard unique constraints concurrently**: Creating a standard table-level constraint while trying to implement partial uniqueness, which results in the standard constraint blocking inserts anyway.

#### Follow-up Questions
1. Can you create a partial index to save disk space? (Yes, by indexing only active, non-null, or flagged records, reducing index size and RAM usage).
2. How does PostgreSQL handle inserts that do not match the partial index filter? (They are inserted into the table but skipped by the index, bypass checking, and do not trigger conflicts).

#### How DSAblitz demonstrates this concept
During the Phase 5 Step 2 Audit, DSAblitz removed table-level constraints that blocked seats for left/kicked players. It replaced them with partial unique indexes to support re-joins and historical logging, and applied a partial unique index on `battles` to prevent multiple active battles inside a single room.

#### Relevant code references
- `[000004_fix_room_players_constraints.up.sql:L1-L18](file:///home/tanishq/dsablitz/backend/migrations/000004_fix_room_players_constraints.up.sql#L1-L18)`: Dropping strict table constraints and replacing them with partial unique indexes `idx_room_players_room_user_active`, `idx_room_players_room_seat_active`, and `idx_battles_active_room_unique`.

#### Related documentation
- [Database Indexing](file:///home/tanishq/dsablitz/docs/database/indexing.md)
- [Rooms Lifecycle Deep Dive](file:///home/tanishq/dsablitz/docs/deep-dives/room_state_machine.md)

---

## Key Takeaways
- **CHECK constraints** are evaluated on the database side for every write, serving as a strict guarantee of core business state invariants.
- **Partial Indexes** restrict index coverage using a static `WHERE` clause, minimizing memory consumption and index update costs.
- **Partial Unique Indexes** allow soft-state unique validation, enabling historical auditing while enforcing strict real-time concurrency rules (e.g. "one active battle per room").

## Interview Questions
1. Why is it impossible to use functions like `CURRENT_TIMESTAMP` or `random()` inside a partial index filter or CHECK constraint?
2. How does PostgreSQL resolve write conflicts when two concurrent transactions insert rows that violate a partial unique index?

## Common Mistakes
- Relying entirely on application-level checks to prevent invalid database states, resulting in dirty writes during race conditions.
- Misordering columns in composite partial indexes or missing the filter condition in SELECT queries.

## Related Documents
- [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)
- [Database Indexing Guide](file:///home/tanishq/dsablitz/docs/database/indexing.md)

## Lessons Learned
- Refactoring rigid table constraints into partial unique indexes allows the application to maintain historical traces of player actions while maintaining seat uniqueness invariants.
