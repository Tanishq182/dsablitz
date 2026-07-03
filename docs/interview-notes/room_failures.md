# Interview Notes: Room Failures & Production Recovery

This document catalogs realistic production failure scenarios, root causes, system solutions, trade-offs, and how to articulate these solutions in system design interviews.

---

## 1. Scenario: Simultaneous Joins (Race Condition)

### **Problem**
Two guest players click the "Join Room" button for the same room code at the exact same millisecond. If not handled, both could be admitted, violating the 1v1 limit and crashing the page.

### **Why it Happens**
Without database-level isolation, Thread A and Thread B read the same player count (1 player, the host) concurrently. Both threads validate that the room is not full and insert a player into seat 2, violating the table unique constraint `room_players_room_seat_unique` or admitting 3 players.

### **Solution**
Wrap the join validation and insertion in a transaction. Acquire a row lock on the room parent:
```sql
SELECT status, max_players FROM rooms WHERE code = $1 FOR UPDATE;
```
This forces Thread B to wait until Thread A commits. When Thread B resumes, it reads the updated player count (2) and rejects the join request with a clean "Lobby is full" error.

### **Tradeoffs**
* **Tradeoff**: Increases request latency slightly for blocked queries.
* **Justification**: Since rooms have a maximum of 2 players, the lock is held for less than 5ms, making the latency impact negligible compared to the cost of corrupting lobby state.

### **How to Explain in an Interview**
> *"To handle race conditions when multiple players try to join a lobby at once, I implemented pessimistic concurrency control. When a join request arrives, we start a transaction and lock the room's parent row using `SELECT FOR UPDATE`. This serializes all joining attempts for that specific room code. The first request successfully joins and increments the player count, while the second request is forced to wait, reads the updated full state, and fails gracefully with a 400 'Room full' message rather than a database unique-constraint failure."*

---

## 2. Scenario: Host Disconnects (Orphaned Room)

### **Problem**
The room host disconnects, closes the browser, or experiences a network blackout while in a lobby. The room remains in `waiting` or `ready` status indefinitely.

### **Why it Happens**
The database is unaware of network connection drops. If there is no active cleanup, the room remains in the database, blocking other players from joining and wasting database resources.

### **Solution**
1. **Heartbeat / Presence Layer**: When WebSockets are implemented, the WebSocket server tracks player presence. If the host connection is lost and not re-established within a grace period (e.g. 15 seconds), the server triggers the `LeaveRoom` service method.
2. **Database Expiry TTL**: The rooms table has an `expires_at` column. A background worker (cron) scans the database periodically and transitions expired rooms to `expired` status.

### **Tradeoffs**
* **Tradeoff**: Running a cron job adds minor query overhead to PostgreSQL.
* **Justification**: The query is index-backed and runs every 60 seconds, which keeps database load extremely low.

### **How to Explain in an Interview**
> *"Lobby presence is managed using a dual approach: transient state is backed by WebSockets, while the source of truth is stored in PostgreSQL. If a host disconnects, the WebSocket connection drop triggers a 15-second reconnection window. If the window expires, the socket coordinator calls the `LeaveRoom` service method, which disbands the lobby and marks it as `closed`. To catch edge cases (like a server crash that prevents the websocket cleanup from running), we have a background cron job that cleans up expired rooms based on their `expires_at` timestamp."*

---

## 3. Scenario: Duplicate Ready Request

### **Problem**
A player clicks the "Ready" button multiple times rapidly, sending duplicate HTTP requests.

### **Why it Happens**
The front-end fails to disable the button, or network lag causes the user to click repeatedly. If not handled, this could trigger multiple database writes or fire duplicate state-transition notifications.

### **Solution**
The `ToggleReady` service validates if the player's current status matches the requested status:
```go
if player.Status == targetStatus {
    return nil // No-op, success
}
```
If the status is already equal, it returns immediately without writing to the database.

### **Tradeoffs**
* **Tradeoff**: None. It is a simple, cost-free guard check.

### **How to Explain in an Interview**
> *"To protect against duplicate ready requests from button double-clicking, the `ToggleReady` service is designed to be idempotent. It acquires a row-level lock on the room, inspects the current player readiness status, and if it already matches the target state, it immediately returns a successful response without writing to the database or firing event triggers."*

---

## 4. Scenario: Duplicate Battle Start

### **Problem**
The room host clicks the "Start Battle" button twice, or sends concurrent requests due to network lag.

### **Why it Happens**
The first request is still processing the battle initialization (fetching questions, generating the sequence, inserting rows), so the room status has not yet transitioned to `in_battle`. The second request passes the `status == 'ready'` validation check.

### **Solution**
We execute the start-battle logic under a pessimistic row lock on the room.
1. The first transaction locks the room row, validates the `ready` status, transitions the room status to `in_battle`, creates the battle, and commits.
2. The second transaction blocks. Once the lock is released, it reads the status as `in_battle` (instead of `ready`).
3. Since it is already in battle, it queries the `battles` table for the active battle ID and returns it (idempotency), preventing a second battle from starting.

### **Tradeoffs**
* **Tradeoff**: Requires querying the battles table in the fallback case.
* **Justification**: Ensures a single match is played and prevents resources from being wasted on duplicate sequences.

### **How to Explain in an Interview**
> *"To ensure battle starting is idempotent, we lock the parent room row. The first request changes the room status to `in_battle` and creates the battle. The second request, which is serialized behind the lock, resumes and sees the room is already `in_battle`. Instead of failing or creating a second match, it queries the active battle for that room and returns its ID, ensuring the host is cleanly redirected to the same match."*

---

## 5. Scenario: Battle Creation Failure (Partial Transaction)

### **Problem**
During battle initiation, the database successfully updates the room status to `in_battle`, but writing the battle player slots or the 200-question sequence fails (e.g. database disconnects mid-way).

### **Why it Happens**
If operations are not grouped in a transaction boundary, the database enters a corrupt state: the room is marked as `in_battle`, but no battle rows or question sequences exist, rendering the game unplayable.

### **Solution**
We wrap both the Room update and Battle creation steps in a single atomic transaction. We pass the transaction handle (`tx`) from the Rooms service to the Battle module repository writes. If any insert fails, the transaction is rolled back completely.

### **Tradeoffs**
* **Tradeoff**: Holds the room lock slightly longer because we execute multiple database insertions inside the transaction.
* **Justification**: Battle start happens once per game, so the minor lock duration increase is fully worth the guarantee of database consistency.

### **How to Explain in an Interview**
> *"To prevent partial failures during game launch, we treat room status updates and battle creation as a single atomic unit of work. We execute all operations—updating the room status, inserting the battle record, mapping the player slots, and writing the question sequence—inside a single SQL transaction. If any database write fails, the entire transaction is rolled back, returning the lobby to its previous `ready` status so the host can safely retry."*

---

## 6. Scenario: Postgres Restarts Mid-Match

### **Problem**
The database crashes or restarts while a battle is active.

### **Why it Happens**
Active connections are severed, and in-flight transactions are aborted by the database server.

### **Solution**
Our backend services use connection pooling ([database.Manager](file:///home/tanishq/dsablitz/backend/internal/platform/database/server.go)) with automatic reconnection logic. Ongoing transactions will fail, but subsequent requests will acquire a new connection from the pool. Players will experience a brief error, but when they retry, they can fetch their current state from the last committed checkpoint.

### **Tradeoffs**
* **Tradeoff**: Ongoing active submissions during the crash will be lost.
* **Justification**: In-memory state recovery from local Go memory is not reliable across restarts; relying on committed database checkpoints is the only safe approach.

### **How to Explain in an Interview**
> *"If PostgreSQL restarts during a match, any active transaction is rolled back by the database, and the client receives a connection error. However, because our player progression pointers are saved after each successful submission and our connection pool automatically reconnects, the game recovers immediately. Upon reconnecting, the client retries the request, retrieves the last committed state from the database, and the player resumes the match from the exact question they were on."*

---

## 7. Scenario: WebSocket Disconnect During Countdown

### **Problem**
During the 5-second battle countdown, a player's WebSocket connection drops.

### **Why it Happens**
The player loses internet connection or their browser tab crashes immediately after the battle is initialized.

### **Solution**
The battle status is already marked as `active` (or `countdown`/`created`) in the database. The disconnection does not abort the battle. The disconnected player has until the match timer runs out (e.g. 5 or 10 minutes) to reconnect. The WebSocket hub preserves their session, and when they reconnect, they fetch the active battle sequence and resume.

### **Tradeoffs**
* **Tradeoff**: The active player keeps playing and gets an advantage, but this is standard for online competitive games.

### **How to Explain in an Interview**
> *"A WebSocket disconnection during the countdown does not stop the match. The battle is already committed to the database. The disconnected player can reconnect to the socket server at any time during the match. The server will resolve their active room, fetch their current progression index from the database, and stream the next question, allowing them to catch up."*

---

## 8. Scenario: Distributed Connection-Pool Deadlock (Nested Transactions)

### **Problem**
Under high concurrent match launch traffic, the server hangs, runs out of database connections, and eventually starts throwing connection timeout errors.

### **Why it Happens**
When a transaction inside the Rooms service (which holds a locked room row) calls the Battle service over a decoupled interface, the Battle service starts its own transaction. Because they are not sharing the same transaction context, the Battle service acquires a second connection from the pool.
If the database pool size is $N$ and $N$ rooms attempt to start a battle concurrently:
1. All $N$ connections are acquired by the Rooms transactions.
2. Each Rooms transaction locks its room row and calls `StartBattle`.
3. The Battle service attempts to acquire a second connection to start its transaction.
4. Since the connection pool is entirely exhausted, all threads block indefinitely.
*Result: Distributed connection-pool deadlock.*

### **Solution**
Pass the transaction handle `tx pgx.Tx` down through the interface context (i.e. `BattleCoordinator.StartBattleTx`). The Battle module executes all its repository statements on the exact same connection holding the parent room lock.

### **Tradeoffs**
* **Tradeoff**: Binds package interfaces slightly closer to the specific driver (`pgx.Tx`), but is fully justified by the deadlock prevention.

### **How to Explain in an Interview**
> *"To avoid connection-pool starvation and distributed deadlocks, we prevent nested, independent transactions during cross-module coordination. If Module A opens a transaction and calls Module B synchronously, we pass the transaction handle (`tx`) through the interface. This ensures both modules execute their statements on the same database connection, avoiding deadlock loops and maintaining atomic transactional boundaries."*

---

## 9. Scenario: Batch Row-Locking Deadlock

### **Problem**
During periodic background room expiration cleanups, the database throws `40P01` (deadlock detected) errors, aborting the cleanup process.

### **Why it Happens**
The cleanup cron runs `SELECT ... FOR UPDATE` on multiple expired rooms. If two cleanup workers run concurrently (or one cleanup worker intersects with another batch query) and PostgreSQL returns the rows in different order (e.g. `[A, B]` on one thread and `[B, A]` on another), the transactions will deadlock.

### **Solution**
Add `ORDER BY id ASC` to all queries that acquire locks on multiple rows. This guarantees that all transactions lock rows in the exact same physical order, eliminating the possibility of locking cycles.

### **How to Explain in an Interview**
> *"Locking multiple rows concurrently in a database is a classic source of deadlocks. If Transaction 1 locks Room A then Room B, and Transaction 2 locks Room B then Room A, they will deadlock. We mitigate this by always ordering the select query deterministically (e.g. `ORDER BY id ASC`) before applying `FOR UPDATE`, forcing all concurrent transactions to lock rows in the same order."*

---

## 10. Scenario: Transaction Abort on Room Code Collision

### **Problem**
During room creation, the code generator hits a collision (rare but possible). The retry loop inside the transaction block fails to recover and crashes with a transaction abort error.

### **Why it Happens**
In PostgreSQL, if a query inside a transaction block raises an error (such as a unique key violation on `rooms.code`), the entire transaction is marked as aborted. The database rejects any subsequent commands in that transaction, meaning you cannot run a retry loop or select queries within the same transaction context.

### **Solution**
Move the retry loop outside the transaction block. If an attempt to create a room fails with a unique constraint error, we rollback, start a brand-new transaction, and generate a new code.

### **How to Explain in an Interview**
> *"Unlike some databases that allow recovery from query errors within a transaction, PostgreSQL immediately marks the transaction as aborted upon any constraint failure. To handle unique code collisions, we must run the retry loop outside the transaction. Each retry attempt opens a new, clean transaction, allowing us to recover gracefully from collisions without raising 500 error pages."*

