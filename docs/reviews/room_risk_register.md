# Risk Register: Rooms Module

This document catalogs identified risks, likelihoods, impacts, and mitigation plans for the **Rooms Module** implementation in **DSAblitz**.

---

## 1. High-Risk Items

### **Seat-Blocking & Rejoin Failures due to UNIQUE Constraints**
* **Description**: The table-level unique constraints in `room_players` (`UNIQUE (room_id, user_id)` and `UNIQUE (room_id, seat_number)`) prevent players who leave a room (status updated to `'left'`) from re-joining the room. It also prevents any other player from taking their seat.
* **Impact**: Critical. Break lobby interactions, causing lobbies to get permanently locked or players to be unable to join.
* **Mitigation**: Propose a new database migration to replace table-level unique constraints with partial unique indexes (e.g., `WHERE status IN ('joined', 'ready')`). In the interim, execute physical deletes of guest player rows when they leave.

### **Database Status Mismatch in Battle Module**
* **Description**: A mismatch exists between the Go constants in `internal/battle/models.go` (`"pending"`, `"completed"`) and the database constraints on `battles.status` (`'created'`, `'countdown'`, `'active'`, `'finished'`, `'aborted'`).
* **Impact**: Critical.
* **Status**: Resolved. Go constants in `internal/battle/models.go` have been aligned with the database schema (e.g. `StatusPending = "created"`, `StatusCompleted = "finished"`, and added `StatusCountdown` and `StatusAborted`).

### **Start Battle Race Conditions (Double Start)**
* **Description**: A host player double-clicks the "Start Battle" button or retries a slow network request, creating multiple concurrent transactions that attempt to initialize a battle for the same room.
* **Impact**: High. Can result in multiple battles created for the same room, inconsistent progression pointers, and database lockups.
* **Mitigation**: 
  1. Wrap room verification and battle creation in a single transaction that locks the room row via `SELECT ... FOR UPDATE`.
  2. Implement database-level idempotency by adding a partial unique index on `battles(room_id) WHERE status IN ('created', 'countdown', 'active')`.

### **Non-Atomic Battle Start Transaction (Distributed Pool Deadlock)**
* **Description**: The Battle coordinator call inside the Rooms service `StartBattle` transaction does not share the same SQL transaction (`pgx.Tx`), running its operations on a separate database connection. This breaks ACID atomicity (leaving battles created if room updates fail) and introduces connection-pool starvation deadlocks.
* **Impact**: Critical.
* **Status**: Resolved. The `BattleCoordinator` interface has been updated to accept `pgx.Tx` and pass it down, and the concrete adapter calls the new `StartBattleTx` inside the Battle service to execute queries on the shared transaction connection.

### **Hardcoded Status Mismatch in CompleteBattle Repository Call**
* **Description**: In `internal/battle/repository.go`, the SQL statement `SET status = 'completed'` hardcodes a value that violates the database check constraint (`CHECK (status IN ('created', 'countdown', 'active', 'finished', 'aborted'))`). Every battle completion will fail with a database error.
* **Impact**: Critical.
* **Status**: Resolved. Updated `CompleteBattle` in `internal/battle/repository.go` to use the correct domain constant `StatusCompleted` which translates to `'finished'`.

### **Deadlock in Expiry Cleanup due to Unordered Locks**
* **Description**: The query inside `rooms.Service.ExpireRooms` locks multiple expired rooms `FOR UPDATE` without an `ORDER BY` clause. Under load, concurrent cleanup runs can lock rows in different orders, leading to database deadlocks.
* **Impact**: Major.
* **Status**: Resolved. Added `ORDER BY id ASC` to the `FOR UPDATE` query in `ExpireRooms` to guarantee deterministic row lock acquisition order.

### **Room Code Generation Retry Loop Failure**
* **Description**: The retry loop for generating unique room codes runs inside the transaction. In PostgreSQL, if any query inside a transaction causes a unique constraint violation, the transaction is aborted and cannot execute further queries (like the subsequent retries).
* **Impact**: Minor.
* **Status**: Resolved. Moved the code generation retry loop outside the transaction block so each retry executes in a clean, fresh transaction context.

### **Leaving Room during Active Match**
* **Description**: A player can leave a room during a battle, which closes the room or marks them as left in `room_players` without completing or resigning the active battle correctly.
* **Impact**: Minor.
* **Status**: Resolved. Implemented a check in `LeaveRoom` that returns an error if `room.Status` is `StatusInBattle`, forcing users to abort or resign through the Battle module instead.

---

## 2. Medium-Risk Items

### **Orphaned Lobbies (Resource Leaks)**
* **Description**: Lobbies created by hosts who then close their tabs, disconnect, or crash will remain in `waiting` or `ready` status indefinitely.
* **Impact**: Medium. Clutters the database with active lobbies and allows users to join dead rooms.
* **Mitigation**: Implement a background ticker/cron job that scans for rooms where `status IN ('waiting', 'ready')` and `expires_at < NOW()`, transitioning them to `expired` and marking players as `left`.

### **Concurrent Guest Joins**
* **Description**: Multiple guests try to join the same public lobby code at the exact same millisecond.
* **Impact**: Medium. If not isolated, both players could be assigned seat 2, violating the database unique constraint and failing with raw 500 error pages.
* **Mitigation**: Implement strict transaction-level locks using `SELECT FOR UPDATE` on the `rooms` table, checking seating availability sequentially.

---

## 3. Low-Risk Items

### **Room Code Collision**
* **Description**: The random room code generator creates a code that is already in use by an active room.
* **Impact**: Low. Fails database insert due to unique constraint on `rooms.code`.
* **Mitigation**: Wrap room creation in a retry block (up to 3 attempts) that generates a new code upon detecting a unique key violation.
