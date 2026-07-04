# Pgx Database Driver - SDE1 Level

This document provides production-level interview preparation material on the `pgx` Go driver, focusing on transaction boundary management, transaction context propagation, and pessimistic locking.

---

## Q&A Sets

### Q1: How does the `WithTransaction` pattern manage database transaction scopes, and how do we propagate transaction handles (`pgx.Tx`) across distinct service modules?

#### Interviewer Intent
The interviewer wants to evaluate:
- Understanding of transactional atomicity and database isolation.
- Design patterns for clean database transaction control in Go.
- Knowledge of transaction propagation rules to prevent nested transactions or connection leaks.

#### Strong Answer
##### 1. The `WithTransaction` Pattern
To ensure atomic execution (where all database modifications succeed or all fail together), we run queries inside a transaction. Managing the boilerplate of `Begin`, `Commit`, and `Rollback` manually can lead to bugs if rollbacks are missed during error returns.
The **`WithTransaction`** helper pattern abstracts this lifecycle:
- It calls `r.db.Begin(ctx)` to start a transaction.
- It uses a `defer tx.Rollback(ctx)` block. If the inner function returns an error, the transaction rolls back. If the transaction committed successfully, the deferred rollback is a safe no-op.
- If the inner function completes without error, it calls `tx.Commit(ctx)`.

```go
func (r *Repository) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
    tx, err := r.db.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback(ctx) // Safe noop if committed

    if err := fn(tx); err != nil {
        return err // Rollback triggered here
    }

    if err := tx.Commit(ctx); err != nil {
        return fmt.Errorf("commit transaction: %w", err)
    }
    return nil
}
```

##### 2. Transaction Context Propagation
In a modular system, business workflows can span multiple domains. For example, starting a battle requires changing a room's status (Rooms domain) and initializing a battle record (Battle domain).
- **Rule**: Both operations must execute inside the same database transaction.
- **Propagation**: To coordinate this, interfaces for cross-module mutations accept the parent transaction handle (`pgx.Tx`) as a parameter. By executing their queries on the passed `tx` handle instead of the raw connection pool, both modules run on the same database connection and within the same atomic scope.

#### Common Mistakes
- **Starting nested transactions**: Attempting to call `db.Begin()` within a function that is already running inside an active transaction. PostgreSQL does not support nested transactions natively; attempting this triggers a syntax error or starts an independent connection, which can lead to deadlocks.
- **Executing queries on the pool instead of the transaction**: Using `r.db.Exec` instead of `tx.Exec` inside a transaction block. The queries run outside the transaction boundary and cannot see uncommitted changes, breaking consistency.
- **Not passing context correctly**: Omitting the `context.Context` parameter or passing a cancelled context, causing the driver to abort the transaction.

#### Follow-up Questions
1. How do you implement savepoints in pgx if you need nested transaction-like behavior? (By using `tx.Begin(ctx)` which pgx maps to database `SAVEPOINT` commands).
2. What happens if a panic occurs inside the `WithTransaction` function block? (The deferred `tx.Rollback(ctx)` executes during panic unwinding, rolling back the transaction before the panic propagates).

#### How DSAblitz demonstrates this concept
In DSAblitz, `StartBattle` transitions a room's status and initializes battle sequences atomically. The `rooms.Service` orchestrates the transaction context and passes the active `pgx.Tx` to the `BattleCoordinator` interface implementation (`StartBattleTx`), executing all writes on a single connection.

#### Relevant code references
- `[repository.go:L27-L43](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L27-L43)`: The `WithTransaction` pattern in the battle repository.
- `[service.go:L338-L424](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L338-L424)`: `StartBattle` starting a transaction and propagating `tx` to the battle coordinator adapter.
- `[routes.go:L21-L35](file:///home/tanishq/dsablitz/backend/internal/server/routes.go#L21-L35)`: The `battleCoordinatorAdapter` forwarding the parent transaction `tx` to `StartBattleTx`.

#### Related documentation
- [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)
- [Transaction Boundaries](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)

---

### Q2: How do you implement pessimistic row locking using pgx, and how do you handle transaction failures and rollbacks cleanly in Go?

#### Interviewer Intent
The interviewer wants to confirm that you:
- Know how to acquire row-level locks using `SELECT ... FOR UPDATE` via pgx.
- Understand pgx-specific query methods (`QueryRow`, `Scan`).
- Know how to clean up resources during database errors.

#### Strong Answer
##### 1. Pessimistic Locking with pgx
Pessimistic locking secures a database row within a transaction to prevent concurrent updates. To execute this in pgx, run a `FOR UPDATE` query on the transaction handle (`tx`):
- Use `tx.QueryRow(ctx, sql, params...)` to fetch the row and acquire the lock.
- Call `.Scan(...)` to map the row columns to local Go struct fields.
- The row remains locked until the transaction commits or rolls back.

##### 2. Clean Error Handling & Rollbacks
If a query fails (e.g. key constraint violation, connection timeout), pgx returns a non-nil error. 
- In Go, we immediately check `if err != nil`.
- If an error is returned inside our transaction wrapper, we return it. The deferred `tx.Rollback(ctx)` is executed, sending a `ROLLBACK` command to the database.
- Once a query error occurs inside a PostgreSQL transaction, PostgreSQL marks the transaction as aborted. Any subsequent queries on that same transaction handle will fail with the error `current transaction is aborted, commands ignored until end of transaction block`. Therefore, you must exit the transaction block immediately upon encountering any query error.

#### Common Mistakes
- **Ignoring `pgx.ErrNoRows`**: When a query returns no rows, `QueryRow().Scan()` returns `pgx.ErrNoRows`. If you do not check for this specifically, it can propagate as a generic database failure.
- **Forgetting to scan query results**: Executing a select query without calling `.Scan(...)` or closing the rows iterator. This leaves the database connection in a busy state, blocking it from executing subsequent queries.
- **Attempting queries on aborted transactions**: Trying to log the error to a database table using the same transaction handle after a query fails. The log write will be ignored because the transaction is already aborted.

#### Follow-up Questions
1. What is the difference between `tx.Exec` and `tx.QueryRow` in pgx? (`tx.Exec` is for commands that do not return rows, like inserts or updates; `tx.QueryRow` is for queries that return a single row).
2. How do you distinguish a connection timeout error from a duplicate key error in pgx? (By using driver-specific error casting, e.g. checking for PostgreSQL error codes via `*pgconn.PgError`).

#### How DSAblitz demonstrates this concept
In DSAblitz, players submit answers concurrently. To prevent race conditions during score updates, the battle repository locks the player's progression row pessimisticly using `GetBattlePlayerForUpdate`. Any query failure aborts the transaction, rolling back the score update safely.

#### Relevant code references
- `[repository.go:L119-L128](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L119-L128)`: `GetBattlePlayerForUpdate` executing `SELECT ... FOR UPDATE` and scanning the row.
- `[service.go:L192-L204](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L192-L204)`: Executing the transaction block and locking the player row.

#### Related documentation
- [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)
- [Submission Lifecycle](file:///home/tanishq/dsablitz/docs/deep-dives/submission_lifecycle.md)

---

## Key Takeaways
- **`WithTransaction`** abstracts transaction lifecycles, ensuring a `ROLLBACK` is executed if any query errors out.
- Cross-module mutations share a database transaction by propagating the **`pgx.Tx`** handle.
- Once any query fails inside a PostgreSQL transaction, the transaction is permanently aborted; the application must exit the block immediately.

## Interview Questions
1. How does pgx handle panics that occur inside a transaction block?
2. What is the significance of the error message "current transaction is aborted, commands ignored until end of transaction block"?

## Common Mistakes
- Executing queries on the connection pool (`db`) instead of the transaction handle (`tx`) inside a transaction block.
- Forgetting to handle `pgx.ErrNoRows` separately from other database query failures.

## Related Documents
- [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)
- [Database Transactions Guide](file:///home/tanishq/dsablitz/docs/database/transactions.md)

## Lessons Learned
- Propagating transaction handles across boundary interfaces prevents connection leaks and ensures that all cross-module mutations succeed or fail as a single unit.
