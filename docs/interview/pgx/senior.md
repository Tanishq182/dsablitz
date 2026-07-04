# Pgx Database Driver - Senior Level

This document provides senior-level interview preparation material on the `pgx` Go driver, focusing on connection pool starvation, transactional deadlocks in modular architectures, wire protocol details (extended query protocol), binary encoding, and transaction retry boundaries.

---

## Q&A Sets

### Q1: How does sharing transaction contexts across modules prevent connection pool starvation and database-driver deadlocks?

#### Interviewer Intent
The interviewer wants to explore:
- Deep understanding of resource allocation and connection limits.
- Ability to diagnose connection pool deadlocks in highly concurrent environments.
- Knowledge of the Transaction Boundary Rule and its architectural importance in modular monoliths.

#### Strong Answer
##### 1. The Connection Pool Starvation Problem
In a modular monolith, separate domains (e.g. Rooms and Battles) require independent repositories. If a business workflow spans both domains, developers are often tempted to implement it by having one service start a transaction and then make a synchronous call to another service, which starts its own transaction.

If Service A starts transaction 1 (acquiring Connection 1 from the pool) and calls Service B, which tries to start transaction 2 (attempting to acquire Connection 2 from the pool), the system is safe under low load. However, under high concurrent traffic:
- 100 requests arrive. 100 threads execute Service A, acquiring all 100 available connections from the pool.
- Each thread now calls Service B synchronously.
- Service B attempts to start a new transaction, calling `pool.Begin()`.
- Since the connection pool is completely exhausted, all 100 requests block, waiting for a connection to become free.
- **Deadlock**: Connection 1 cannot be released until Service B completes. Service B cannot run because it is waiting for a connection to be released. The application freezes, leading to **connection pool starvation deadlock**.

```
[Request 1] ---> Service A (Acquires Conn 1) ---> Call Service B (Blocks waiting for Conn 2)
[Request 2] ---> Service A (Acquires Conn 2) ---> Call Service B (Blocks waiting for Conn 1)
========================= Connection Pool Exhausted (Deadlock!) =========================
```

##### 2. Mitigation via Transaction Propagation (Transaction Boundary Rule)
To resolve this, we enforce a strict **Transaction Boundary Rule**:
- We never start nested transactions across service calls.
- Instead of services starting their own internal transactions, the orchestration layer (or the initiating service) starts a single transaction.
- It propagates this single transaction handle (`pgx.Tx`) to all cross-module mutations.
- The downstream services execute their queries using the passed `tx` handle, which runs on the **same TCP socket connection** already allocated to the request. This eliminates the need to acquire a second connection from the pool, preventing starvation deadlocks.

#### Common Mistakes
- **Hiding transactions in contexts**: Storing `pgx.Tx` in a standard `context.Context` object and passing it implicitly. While this keeps signatures clean, it hides the transaction lifecycle, making it easy for developers to run queries on the wrong transaction or execute slow operations (like third-party API calls) while holding database connections. Explicit parameter passing is preferred for transparency.
- **Holding connections during network I/O**: Executing long-running HTTP client requests to external APIs while inside a database transaction block. This keeps the database connection busy and unavailable, causing connection pool exhaustion.
- **Mixing transaction types**: Mixing write transactions with read-only transactions in the same connection pool without proper rate limits.

#### Follow-up Questions
1. How does configuring `max_conn_lifetime` in pgx help prevent connection leaks? (It automatically closes and recreates connections that have been open too long, cleaning up stale resources).
2. How do you monitor connection pool health in pgx? (By reading `pool.Stat()`, which returns details on total connections, idle connections, and acquired connections).

#### How DSAblitz demonstrates this concept
In DSAblitz, room lifecycle events and battle starting run inside a single atomic database transaction. The `StartBattle` service method coordinates the write operations by passing the active transaction context `tx pgx.Tx` to the Battle coordinator, ensuring both modules share a single connection socket.

#### Relevant code references
- `[PROJECT_CONTEXT.md:L67-L70](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L67-L70)`: The Transaction Boundary Rule definition.
- `[service.go:L406-L414](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L406-L414)`: `StartBattle` passing `tx` to the battle coordinator instead of initiating a nested transaction.
- `[routes.go:L25-L35](file:///home/tanishq/dsablitz/backend/internal/server/routes.go#L25-L35)`: Adapter mapping the Rooms-owned coordinator interface to the Battle service transaction logic.

#### Related documentation
- [Transaction Boundaries](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md)
- [Room Transactions](file:///home/tanishq/dsablitz/docs/deep-dives/room_transactions.md)

---

### Q2: How does `pgx` interact with PostgreSQL at the protocol level, and how do we design retry boundaries in Go to handle aborted transactions?

#### Interviewer Intent
The interviewer is looking for:
- Understanding of the PostgreSQL Frontend/Backend protocol (specifically Extended Query Protocol).
- Knowledge of prepared statements, statement caching, and binary format encoding.
- Mastery of transaction retry design patterns when dealing with transient failures (such as serialization failures, deadlocks, or connection drops).

#### Strong Answer
##### 1. Protocol Level & Serialization
`pgx` communicates with PostgreSQL using the native frontend/backend protocol. It defaults to the **Extended Query Protocol**:
- **Parse**: Sends the query string with placeholders (e.g. `$1`) to the database. PostgreSQL compiles it and stores the query plan as a prepared statement.
- **Describe**: PostgreSQL returns the parameter types and column descriptions.
- **Bind**: `pgx` sends the parameter values. `pgx` encodes parameters in **binary format** rather than text format. Binary encoding (e.g. representing integers as 4/8 byte payloads or UUIDs as 16-byte raw data) reduces network bandwidth and CPU parsing overhead on the database server.
- **Execute**: The database runs the prepared statement and returns the rows.

`pgx` automatically manages a **statement cache** on every connection in the pool. When a query is run repeatedly, pgx skips the Parse/Describe phase and executes the cached prepared statement directly.

##### 2. Postgres Abort & Retry Boundary Rule
When a query fails inside a PostgreSQL transaction (e.g. database serialization conflict or lock acquisition timeout), PostgreSQL aborts the transaction. Any subsequent query on that transaction returns the error `current transaction is aborted`. The transaction cannot be rescued; it must be rolled back.

To recover from these failures, we must design a **Retry Loop**:
- **Rule**: The retry loop must be executed **outside** the transaction block. Since the transaction is permanently aborted upon error, each retry attempt must start a clean, fresh transaction.
- In Go, we wrap the transaction block in a retry loop. If a retryable error occurs (e.g. Postgres error code `40001` for serialization failure or `40P01` for deadlock), we wait briefly (using exponential backoff) and start a new transaction.

```go
func RunWithRetry(ctx context.Context, db *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
    for i := 0; i < MaxRetries; i++ {
        err := db.BeginFunc(ctx, func(tx pgx.Tx) error {
            return fn(tx) // If this fails, BeginFunc automatically rolls back
        })
        if err == nil {
            return nil // Success
        }
        
        // Check if error is retryable
        if isRetryableError(err) {
            time.Sleep(backoff(i))
            continue // Retry with a clean transaction
        }
        return err // Fatal error, abort
    }
    return ErrMaxRetriesExceeded
}
```

#### Common Mistakes
- **Placing the retry loop inside the transaction block**: Attempting to retry a query using the same aborted transaction handle. This leads to persistent failures.
- **Hardcoding statement caching configuration**: Not configuring statement cache limits for applications that generate dynamic SQL strings, causing statement cache pollution and memory bloat on both client and database servers.
- **Ignoring context cancellation inside retries**: Retrying queries indefinitely after the client has disconnected, wasting database connections and CPU cycles.

#### Follow-up Questions
1. What is the difference between PostgreSQL's Simple Query Protocol and Extended Query Protocol? (Simple protocol sends the raw query string and parameters together as text in a single roundtrip, whereas Extended splits them into multi-step binary calls).
2. How do you identify specific PostgreSQL errors in Go? (By casting the error to `*pgconn.PgError` and checking the 5-character SQLState code).

#### How DSAblitz demonstrates this concept
In DSAblitz, room code generation generates random strings. If a room code collision occurs, the database throws a unique constraint violation. Rather than executing the retry loop inside the transaction, the `CreateRoom` service method places the retry loop outside the transaction block, ensuring that each retry attempt operates on a fresh, clean transaction.

#### Relevant code references
- `[PROJECT_CONTEXT.md:L74-L75](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L74-L75)`: The Postgres Abort & Retry Boundary Rule.
- `[service.go:L56-L113](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L56-L113)`: `CreateRoom` loop executing `WithTransaction` on each retry attempt to recover from code collisions safely.

#### Related documentation
- [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)
- [Rooms Lifecycle Deep Dive](file:///home/tanishq/dsablitz/docs/deep-dives/room_state_machine.md)

---

## Key Takeaways
- Executing multiple transactions within a single request thread can exhaust connection pools and trigger **starvation deadlocks**.
- Solve connection starvation by propagating the **`pgx.Tx`** handle to share a single connection socket across service modules.
- `pgx` utilizes the **Extended Query Protocol** and binary encoding to optimize network I/O and query compilation overhead.
- Because query failures abort PostgreSQL transactions permanently, **retry loops** must execute outside the transaction block.

## Interview Questions
1. Explain how pgx's connection-level statement cache improves performance, and how it behaves under dynamic SQL generation.
2. How would you design a retry decorator in Go that parses PostgreSQL `PgError` codes to handle serialization failures?

## Common Mistakes
- Implementing retry logic inside transaction blocks, leading to persistent "transaction is aborted" errors.
- Performing long-running background tasks or third-party API calls while holding an active transaction handle.

## Related Documents
- [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)
- [Database Transactions Guide](file:///home/tanishq/dsablitz/docs/database/transactions.md)

## Lessons Learned
- Structuring retry loops outside the boundaries of database transactions prevents cascading connection drops and ensures clean recovery paths for transient conflicts.
