# Room Transaction Boundary & Locking Strategy

This document details the transaction flow, database locking mechanism, and concurrency rules applied to room lifecycle operations in **DSAblitz**.

---

## 1. Complete Transaction Flow

Every lifecycle-altering operation (e.g. Join Room, Toggle Ready, Leave Room, Start Battle) follows a strict transactional flow:

```
                  Client Request (e.g., JoinRoom)
                                │
                                ▼
                         [ Begin Tx ]
                                │
                                ▼
         [ SELECT * FROM rooms WHERE code = ? FOR UPDATE ]
             (Acquires Row-Level Lock on Room Parent)
                                │
                                ▼
      [ SELECT * FROM room_players WHERE room_id = ? FOR UPDATE ]
           (Loads current player statuses & seat layout)
                                │
                                ▼
                  [ Validate Business Rules ]
            - Is room waiting? Is it full?
            - Does the user already exist in this lobby?
                                │
                                ├─ (Rule Violated) ──► [ Rollback Tx ] (Close connection)
                                │
                                ▼
           [ Write Updates & Insertions to Database ]
            - Insert new room_player row (assign seat 2)
            - Update room status if transitions are triggered
                                │
                                ▼
                         [ Commit Tx ]
             (Releases all acquired row locks on DB)
```

---

## 2. Row Locking Mechanics

* **Query**: `SELECT ... FOR UPDATE`
* **Target**: The specific row in the `rooms` table corresponding to the lobby being modified.
* **Mechanism**: PostgreSQL locks the specific row in index order. Any other client attempting to read that row using `FOR UPDATE` or trying to update it will block until the transaction holding the lock either commits or rolls back.
* **Why not Table-Level Locks?** Table-level locks (`LOCK TABLE rooms IN EXCLUSIVE MODE`) would serialise *all* room activities across the entire system, creating severe performance bottlenecks. Row-level locks restrict locking strictly to the two players competing in that specific room.

---

## 3. Concurrency Protection & Deadlock Avoidance

### **Global Lock Ordering Rule**

To completely prevent deadlocks in transactions spanning multiple entities, the entire backend enforces a strict **Global Lock Ordering Rule**. If a transaction needs to lock or write to multiple tables, it MUST acquire locks in this exact order:

```
    rooms
      │
      ▼
  room_players
      │
      ▼
   battles
      │
      ▼
 battle_players
      │
      ▼
battle_question_sequence
```

#### **Why Deterministic Ordering Prevents Deadlocks**
A deadlock occurs when two concurrent transactions try to acquire the same set of locks but in a different order. For example:
- **Transaction A** locks Table X and waits for Table Y.
- **Transaction B** locks Table Y and waits for Table X.
Both transactions block indefinitely waiting for each other, forcing the database engine to abort one of them.

By enforcing a deterministic lock ordering, we guarantee that all transactions acquire locks in the same direction. If Transaction A is locking `rooms` and moving down to `battles`, and Transaction B is also locking `rooms` and moving down to `battles`, Transaction B will block at the very first step (`rooms`), allowing Transaction A to acquire all its locks, complete, and release them. No cycle can form.

#### **Rules for Future Modules**
Any future module (e.g. matchmaking, statistics, friendships, tournaments) that initiates or interacts with rooms or battles within a database transaction must strictly conform to this hierarchy:
- If a tournament transaction updates tournament brackets AND room states, and tournament is not in the hierarchy, it must either run outside the room transaction or define a strict lock order with respect to the room table (e.g., `tournaments` -> `rooms` -> ...).
- No transaction may lock a child table (e.g., `battle_players`) and then attempt to lock a parent table (e.g., `battles` or `rooms`) in the same transaction.

#### **Deadlock Scenario If Violated**
Suppose a developer updates player battle scores and decides to log this in the room status.
- **Transaction 1 (Start Battle)**: Locks `rooms` row first (ordering correct), then starts battle and attempts to lock `battle_players` for initialization.
- **Transaction 2 (Submission Score Update)**: Locks `battle_players` first to update scores, and then queries/locks the associated `rooms` table to see if the room status should be modified.
If these two run concurrently:
- Transaction 1 holds lock on `rooms` and waits for lock on `battle_players`.
- Transaction 2 holds lock on `battle_players` and waits for lock on `rooms`.
*Result*: Both transactions are deadlocked. PostgreSQL will raise a `40P01` deadlock detected error and abort one of them, resulting in failed matches or score losses. Enforcing the Global Lock Ordering prevents this completely.

---

## 4. Rollback and Isolation

* **Isolation Level**: Read Committed (Postgres default) combined with explicit row locks (`FOR UPDATE`) provides serializable isolation for locked rows.
* **Rollback Protocol**: In Go, the database connection uses a deferred function call to rollback:
  ```go
  tx, err := r.db.Begin(ctx)
  if err != nil {
      return err
  }
  defer tx.Rollback(ctx) // Safe no-op if Tx is committed
  
  // ... execute writes ...
  
  if err := tx.Commit(ctx); err != nil {
      return err
  }
  ```
  If any validation fails or database operation fails mid-transaction, `tx.Rollback(ctx)` is triggered automatically on function exit, ensuring partial updates are never persisted.
