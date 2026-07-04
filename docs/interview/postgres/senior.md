# PostgreSQL Database - Senior Level

This document provides senior-level interview preparation material on PostgreSQL internal execution mechanics, focusing on Generalized Inverted Indexes (GIN), query execution diagnostics, row-level lock conflicts, and deadlock prevention architectures.

---

## Q&A Sets

### Q1: How does a Generalized Inverted Index (GIN) work internally in PostgreSQL, and how do you diagnose query bottlenecks using `EXPLAIN ANALYZE`?

#### Interviewer Intent
The interviewer wants to test your understanding of:
- Internal database index structures beyond basic B-Trees.
- GIN index architectures (inverted indexing of composite types like arrays, JSONB, or text documents).
- Advanced database performance diagnostics using execution plans.

#### Strong Answer
##### 1. Generalized Inverted Index (GIN) Internals
A B-Tree index maps a row value to a single database row pointer (TID). A **GIN (Generalized Inverted Index)** is an **inverted index**. It is designed to index composite values—such as arrays (e.g. `tags TEXT[]`) or JSONB objects—where a single row contains multiple elements.
- GIN breaks down the composite values of a column into individual elements (keys).
- It builds a B-Tree structure where the index keys are the *individual elements* (e.g., individual tags like `"graph"` or `"dp"`), rather than the entire array value.
- For each element key, GIN stores a list or a separate B-Tree of TIDs (row pointers) pointing to all rows that contain that element.
- When querying using array operators (such as `tags @> ARRAY['graph']` - contains "graph"), Postgres traverses the GIN index to find the `"graph"` key and immediately retrieves the list of matching TIDs.

```
GIN Index Structure
==============================================================
Tag Key ("dp")    ---> TID List: [Row_1, Row_14, Row_45]
Tag Key ("graph") ---> TID List: [Row_2, Row_14, Row_99]
```

##### 2. Diagnosing Bottlenecks with `EXPLAIN ANALYZE`
To diagnose query execution issues, prepend `EXPLAIN (ANALYZE, BUFFERS)` to your query. 
- **`EXPLAIN`**: Generates the static query plan estimated by the database statistics planner.
- **`ANALYZE`**: Actually executes the query, showing real execution times and row counts alongside the estimates.
- **`BUFFERS`**: Displays shared memory buffer hits, reads, and writes (L1 cache hits vs disk reads).

Key plan components to analyze:
1. **Sequential Scan (Seq Scan) vs Index Scan**: A Seq Scan indicates the database is reading the entire table from disk. If this happens on a large table, an index is missing or being ignored.
2. **Bitmap Index Scan / Bitmap Heap Scan**: GIN indexes typically execute via a Bitmap Index Scan. Postgres reads the index, builds a memory bitmap of matching page addresses, and then performs a Bitmap Heap Scan to fetch the actual rows, minimizing random I/O.
3. **Filter vs Index Cond**: "Index Cond" means the index was used to narrow the search. "Filter" means Postgres had to read rows and discard them in memory, which is a CPU bottleneck.
4. **Actual Rows vs Loops**: A large difference between estimated rows and actual rows indicates stale table statistics. Run `ANALYZE table_name` to update them.

#### Common Mistakes
- **Using GIN indexes for write-heavy columns**: GIN indexes have high update overhead. Inserting or updating an array with 10 elements requires updating 10 separate keys in the GIN index structure. Postgres uses a fast-update buffer to defer this, but under constant write pressure, GIN indexes cause write performance degradation.
- **Confusing GIN with GiST**: GiST (Generalized Search Tree) is best for geometric/spatial data or nearest-neighbor queries. GIN is superior for static composite lookups (arrays, JSONB) because it is faster to search, though slower to build.
- **Running `EXPLAIN` without `ANALYZE`**: Assuming the estimated plan is exactly what happened. The planner's estimates can be wrong due to stale statistics or parameter sniffing. Always use `ANALYZE` to see actual run times.

#### Follow-up Questions
1. What is the "fastupdate" parameter in GIN indexing? (It is a PostgreSQL setting that caches GIN index updates in a temporary list, merging them in bulk to reduce write write latency).
2. What are the advantages of combining `EXPLAIN` with `BUFFERS`? (It helps measure query cost in memory pages read from cache vs disk, which is more deterministic than execution time).

#### How DSAblitz demonstrates this concept
In DSAblitz, questions are categorized by tags (e.g. `dp`, `greedy`). Match sequences query questions matching specific tag sets. To prevent full table scans on the stateless question catalog, the schema defines a GIN index on the tags array.

#### Relevant code references
- `[000001_create_core_schema.up.sql:L134-L156](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L134-L156)`: The `questions` table definition with `tags TEXT[]`.
- `[000001_create_core_schema.up.sql:L269](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L269)`: GIN index creation: `CREATE INDEX idx_questions_tags_gin ON questions USING GIN(tags)`.

#### Related documentation
- [Database Indexing](file:///home/tanishq/dsablitz/docs/database/indexing.md)
- [Database Schema](file:///home/tanishq/dsablitz/docs/database/schema.md)

---

### Q2: How do pessimistic locks (`FOR UPDATE`) interact with composite indexes, and how does the Global Lock Ordering Rule prevent deadlock cycles when locking multiple tables?

#### Interviewer Intent
The interviewer wants to evaluate:
- Understanding of transactional concurrency control and isolation levels.
- Deep knowledge of row-level locking behavior in PostgreSQL (`FOR UPDATE`, `FOR SHARE`).
- Practical strategy for designing deadlock-free transactional workflows.

#### Strong Answer
##### 1. Pessimistic Row Locking (`FOR UPDATE`)
When a query executes `SELECT ... FOR UPDATE`, PostgreSQL acquires an exclusive row-level write lock on the matching records. Any concurrent transaction attempting to modify those rows or lock them using `FOR UPDATE` is blocked until the locking transaction commits or rolls back.
- **Index Interaction**: The database uses indexes to locate the rows to lock. If the query does not use an index, PostgreSQL is forced to run a **Sequential Scan**, locking *every single row* it evaluates in the table, even if they do not match the final `WHERE` filter. This drastically increases lock contention and blocks unrelated transactions.
- **Lock Ordering**: If multiple rows are locked concurrently, they must be locked in a **deterministic order** (e.g., `ORDER BY id ASC`). If Transaction A locks Row 1 then Row 2, and Transaction B locks Row 2 then Row 1, a **deadlock** occurs.

##### 2. The Global Lock Ordering Rule
In complex architectures where transactions span multiple tables, deadlocks can occur across tables. For example:
- Transaction A locks a row in `rooms` and wants to lock a row in `battles`.
- Transaction B locks a row in `battles` and wants to lock a row in `rooms`.
- Result: Permanent deadlock.

To prevent this, the system enforces a strict **Global Lock Ordering Rule**. All database transactions spanning multiple tables must acquire row locks in a predetermined, hierarchical order:
1. `rooms`
2. `room_players`
3. `battles`
4. `battle_players`
5. `battle_question_sequence`

By enforcing this hierarchy across all services, a transaction attempting to modify `battles` and `rooms` is forced to lock `rooms` first. If another transaction is already holding the lock on `rooms`, the incoming transaction blocks cleanly instead of creating a deadlock loop.

```
Transaction A (Wants Rooms & Battles) -> Locks Rooms -> Attempts Battles
                                                            | (Blocks)
Transaction B (Wants Rooms & Battles) -> Blocks on Rooms <---|
```

#### Common Mistakes
- **Locking rows without ordering**: Executing batch updates or bulk locks (e.g., in a background cleanup job) without sorting the IDs. If the cleanup job updates rows in arbitrary order, it will deadlock with active user transactions.
- **Assuming transaction isolation levels prevent deadlocks**: Many assume that setting the transaction isolation level to `SERIALIZABLE` or `REPEATABLE READ` prevents deadlocks. In reality, higher isolation levels do not prevent deadlocks; they simply convert lock waits into serialization failure errors, requiring application-level retries.
- **Locking too early or too broadly**: Acquiring `FOR UPDATE` locks on rows that are only needed for read-only checks, increasing database latency and blocking concurrent API requests.

#### Follow-up Questions
1. What is the difference between `FOR UPDATE` and `FOR NO KEY UPDATE` in PostgreSQL? (`FOR NO KEY UPDATE` does not lock foreign key constraints, allowing other transactions to insert rows referencing the locked row, which increases write concurrency).
2. How does the Postgres deadlock detector work? (It runs in the background. If a transaction waits for a lock longer than `deadlock_timeout` (typically 1 second), Postgres searches for dependency cycles in the lock graph and aborts one of the transactions).

#### How DSAblitz demonstrates this concept
In DSAblitz, rooms lifecycle management and battle initialization are highly concurrent. The codebase strictly enforces the Global Lock Ordering Rule.
- In `ExpireRooms`, expired rooms are sorted by ID before locking to prevent deadlocks: `ORDER BY id ASC FOR UPDATE`.
- In `CompleteBattle`, players are locked using deterministic ordering: `ORDER BY user_id ASC FOR UPDATE`.

#### Relevant code references
- `[PROJECT_CONTEXT.md:L61-L63](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L61-L63)`: Lock hierarchy definition.
- `[service.go:L430-L435](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L430-L435)`: ExpireRooms executing `ORDER BY id ASC FOR UPDATE`.
- `[repository.go:L291-L313](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L291-L313)`: `GetBattlePlayersTx` executing `ORDER BY user_id ASC FOR UPDATE` to secure player statistics lock-free.

#### Related documentation
- [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)
- [Transaction Boundaries](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)

---

## Key Takeaways
- **GIN indexes** act as inverted indexes, mapping composite elements (like array tags) to TIDs, speeding up element containment lookups at the expense of write overhead.
- **`EXPLAIN ANALYZE`** is the primary tool for database diagnostics, executing queries to return actual vs estimated execution times and memory page buffer stats.
- **`FOR UPDATE`** locks require deterministic ordering (e.g. `ORDER BY id ASC`) to prevent deadlock loops.
- Enforcing a strict **Global Lock Ordering Rule** across tables eliminates cross-table deadlock conditions in complex transactional systems.

## Interview Questions
1. Why does an index scan on a GIN index use a bitmap scan rather than a direct index scan?
2. Under what circumstances can a lock escalation occur in PostgreSQL? (Postgres does not escalate row locks to page/table locks; it is designed to hold millions of row locks without memory exhaustion).

## Common Mistakes
- Omitting deterministic ordering (`ORDER BY`) when acquiring row-level locks on multiple rows.
- Executing write transactions that lock tables out of order, violating the Global Lock Ordering Rule.

## Related Documents
- [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)
- [Database Transactions Guide](file:///home/tanishq/dsablitz/docs/database/transactions.md)

## Lessons Learned
- Establishing a clear lock ordering hierarchy in architectural guidelines prevents hard-to-debug runtime deadlock failures under high production concurrency.
