# PostgreSQL Database - Graduate Level

This document provides graduate-level interview preparation material on PostgreSQL relational schema design, covering primary keys, foreign keys, referential integrity rules, and basic B-Tree indexing.

---

## Q&A Sets

### Q1: What is the difference between a Primary Key and a Foreign Key, and how do referential integrity actions like `ON DELETE CASCADE` and `ON DELETE RESTRICT` behave?

#### Interviewer Intent
The interviewer wants to verify:
- Core understanding of relational database models.
- Ability to define primary keys (uniqueness and non-nullability) and foreign keys (relations).
- Understanding of database-level referential integrity rules to prevent orphaned records.

#### Strong Answer
- **Primary Key (PK)**: A column (or set of columns) that uniquely identifies each row in a table. It is implicitly indexed and cannot contain `NULL` values.
- **Foreign Key (FK)**: A column (or set of columns) that establishes a link between data in two tables. It references the Primary Key of another table, ensuring that values in the child table correspond to valid rows in the parent table.

##### Referential Integrity Actions
When a row in a parent table is deleted, any referencing rows in child tables must be handled to prevent broken references (orphaned rows):
1. **`ON DELETE CASCADE`**: Automatically deletes referencing rows in the child table when the parent row is deleted. For example, if a `battle` row is deleted, all associated `battle_players` rows are automatically deleted.
2. **`ON DELETE RESTRICT`**: Prevents the parent row from being deleted if any child rows reference it. For example, if a room hosts a active user, deleting that host user is blocked by Postgres because it would leave the room host column pointing to a non-existent user.
3. **`ON DELETE SET NULL`**: Sets the child foreign key column to `NULL` when the parent row is deleted. For example, if a room is deleted, the `room_id` column in the `battles` table can be set to `NULL` to keep the battle records for history.

#### Common Mistakes
- **Assuming Foreign Keys are indexed automatically**: PostgreSQL does NOT automatically create indexes on foreign key columns. If you do not index them, joins and parent-row deletions can trigger slow sequential table scans.
- **Using Cascade Deletes carelessly**: Applying `ON DELETE CASCADE` everywhere can lead to accidental mass deletions. If a user is deleted and cascading is too deep, it can wipe out critical statistics, logs, and billing history.
- **Choosing the wrong UUID generation**: Choosing random UUIDs (`gen_random_uuid()`) is standard, but candidates should know that writing randomly distributed UUIDs can cause page fragmentation in primary key indexes.

#### Follow-up Questions
1. Can a table have multiple Foreign Keys? (Yes, referencing different tables or columns).
2. What is a Composite Primary Key? (A primary key made of two or more columns, e.g. `PRIMARY KEY (battle_id, sequence_index)`).

#### How DSAblitz demonstrates this concept
In DSAblitz, the core schema defines relationships between rooms, battles, and players. For example, if a battle is deleted, its player slots and question sequences are automatically cleaned up using `ON DELETE CASCADE`. If a host user exists, deleting the user is blocked using `ON DELETE RESTRICT`.

#### Relevant code references
- `[000001_create_core_schema.up.sql:L57-L72](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L57-L72)`: The `rooms` table definition, where `host_user_id` uses `REFERENCES users(id) ON DELETE RESTRICT`.
- `[000001_create_core_schema.up.sql:L109-L132](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L109-L132)`: The `battle_players` table definition, using `REFERENCES battles(id) ON DELETE CASCADE`.
- `[000003_add_battle_sequence_and_progression.up.sql:L5-L11](file:///home/tanishq/dsablitz/backend/migrations/000003_add_battle_sequence_and_progression.up.sql#L5-L11)`: Composite primary key `PRIMARY KEY (battle_id, sequence_index)` on the question sequence table.

#### Related documentation
- [Database Schema](file:///home/tanishq/dsablitz/docs/database/schema.md)
- [Database Migrations](file:///home/tanishq/dsablitz/docs/database/migrations.md)

---

### Q2: What is a B-Tree index, and how do composite indexes speed up queries with multiple search criteria?

#### Interviewer Intent
The interviewer is looking for:
- Understanding of the default indexing structure in PostgreSQL (B-Tree).
- Knowledge of search complexity (from $O(N)$ sequential scans to $O(\log N)$ index lookups).
- Understanding of how composite indexes (indexes on multiple columns) behave and their prefix matching rules.

#### Strong Answer
##### 1. B-Tree Indexes
By default, PostgreSQL creates a **B-Tree (Balanced Tree)** index when you run `CREATE INDEX`. A B-Tree index maintains data in a sorted, balanced tree structure. This allows Postgres to locate rows in $O(\log N)$ search complexity rather than executing a full-table scan of $O(N)$. It is highly efficient for:
- Exact equality matches (`=`).
- Range queries (`<`, `<=`, `>`, `>=`).
- Sorting (`ORDER BY`).

##### 2. Composite Indexes
A **Composite Index** is an index created on multiple columns in a specific order (e.g., `CREATE INDEX ON users(status, created_at)`).
- **Ordering Matters**: The index is sorted first by the first column, and then by the second column within matches of the first.
- **Left-to-Right Prefix Rule**: A query can use a composite index only if the query filters include the left-most prefix of the index columns.
  - A query filtering by `WHERE status = 'active' AND created_at > NOW()` **can** use the index.
  - A query filtering by `WHERE status = 'active'` **can** use the index.
  - A query filtering *only* by `WHERE created_at > NOW()` **cannot** use the index efficiently, because the index is sorted primarily by `status`.

```
Composite Index: (status, created_at)
--------------------------------------
("active",  2026-07-01)
("active",  2026-07-02)
("deleted", 2026-06-15)
("deleted", 2026-07-01)
```

#### Common Mistakes
- **Creating separate single-column indexes instead of a composite index**: Creating an index on `status` and another on `created_at` when queries always filter by both. While Postgres can perform bitmap index scans to combine them, a single composite index is much faster.
- **Ignoring the Column Order**: Registering a composite index on `(created_at, status)` when the application primarily searches by `status`. This violates the left-most prefix rule for searches that only supply `status`.
- **Indexing columns with low cardinality**: Creating an index on a boolean column (e.g. `is_active`). Since half the table matches `true`, the planner will ignore the index and perform a faster sequential scan.

#### Follow-up Questions
1. Why does the Postgres planner sometimes choose a full table scan even if an index exists? (If the table is very small, or if the query filters return a large percentage of the table rows, scanning the table directly is faster than reading the index first).
2. How does index write overhead affect update/insert performance? (Every write, delete, or update of indexed columns requires modifying the B-Tree structure, adding latency to write transactions).

#### How DSAblitz demonstrates this concept
In DSAblitz, tables are indexed to support rapid lookups. For example, matchmaking queries looking for rooms with specific status values use composite indexes. The `idx_users_status_created_at` index speeds up user management, while `idx_rooms_status_created_at` optimizes lobby listing.

#### Relevant code references
- `[000001_create_core_schema.up.sql:L244](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L244)`: Composite index `idx_users_status_created_at` on `users(status, created_at)`.
- `[000001_create_core_schema.up.sql:L252](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L252)`: Composite index `idx_rooms_status_created_at` on `rooms(status, created_at)`.

#### Related documentation
- [Database Indexing](file:///home/tanishq/dsablitz/docs/database/indexing.md)
- [Database Schema](file:///home/tanishq/dsablitz/docs/database/schema.md)

---

## Key Takeaways
- **Primary Keys** enforce identity and uniqueness. **Foreign Keys** preserve referential integrity using actions like `ON DELETE CASCADE` and `ON DELETE RESTRICT`.
- **B-Tree indexes** sort columns for $O(\log N)$ range and equality lookups.
- **Composite indexes** require left-most prefix matching. Column order is critical to ensuring queries use the index.

## Interview Questions
1. Explain how a cascade delete behaves when several child tables have foreign keys with cascading enabled.
2. In what scenarios is a composite index on `(A, B)` not useful for queries filtering on `B`?

## Common Mistakes
- Not indexing foreign key columns, which causes slow sequential scans on parent deletions.
- Creating too many indexes, which degrades write, update, and insert throughput.

## Related Documents
- [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)
- [Database Indexing Guide](file:///home/tanishq/dsablitz/docs/database/indexing.md)

## Lessons Learned
- Decoupling user identities using database foreign keys prevents dangling references and enforces strict system state boundaries.
