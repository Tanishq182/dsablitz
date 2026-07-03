# Interview Notes: Rooms Module

This document outlines key technical discussions, architectural decisions, trade-offs, and database design caveats for the **Rooms Module** in **DSAblitz**.

---

## 1. Architectural Question: Room vs. Battle Separation

### Q: Why separate Room and Battle into different tables and modules?
**A:**
1. **Separation of Concerns**: Rooms manage human presence, lobby states, matchmaking, and WebSockets connection lifecycle (transient infrastructure state). Battles own gameplay execution: question streaming, scoring algorithms, submission logs, and Elo calculations (durable transactional state).
2. **One-to-Many Relationship**: A single persistent Room (e.g., a "rematch lobby") can host multiple sequential Battles. If we merged them, we would need to constantly duplicate lobby details or wipe out historical match statistics.
3. **Write Isolation**: Battles have high-frequency writes (submissions, pointer updates). Lobby presence is low-frequency (joins, leaves, ready toggles). Keeping them separate prevents database lock contention between active game submissions and room state lookups.

---

## 2. Database Constraint Conflict (Crucial Finding)

### The Issue
The existing `room_players` table schema includes two strict unique constraints:
```sql
CONSTRAINT room_players_room_user_unique UNIQUE (room_id, user_id),
CONSTRAINT room_players_room_seat_unique UNIQUE (room_id, seat_number)
```
Simultaneously, the table allows player status to transition to `'left'` or `'kicked'`, and includes a `left_at` timestamp.

If a player leaves a room and their record's status is simply updated to `'left'`:
1. The row remains in the database.
2. The user **cannot rejoin** the room, because the unique constraint `UNIQUE (room_id, user_id)` is violated.
3. The seat remains **blocked**, because the unique constraint `UNIQUE (room_id, seat_number)` prevents any new player from taking that seat (e.g., seat 2).

### The Solutions

#### Option A: Deleting Player Rows on Leave (No Migration Required)
* **Mechanism**: When a player leaves or is kicked from a room, the backend deletes their `room_players` record instead of updating it to `left`.
* **Pros**: Simple, does not require updating the database schema or writing new migration scripts.
* **Cons**: We lose historical tracking of who joined and left the room before the battle started. However, rooms are short-lived lobbies, so historical tracking of leaves is not a core requirement.

#### Option B: Partial Unique Indexes (Production-Grade, Recommended)
* **Mechanism**: Remove the table-level unique constraints and replace them with partial indexes that ignore historical rows:
  ```sql
  ALTER TABLE room_players DROP CONSTRAINT room_players_room_user_unique;
  ALTER TABLE room_players DROP CONSTRAINT room_players_room_seat_unique;
  
  CREATE UNIQUE INDEX idx_room_players_room_user_active 
    ON room_players(room_id, user_id) 
    WHERE status IN ('joined', 'ready');
    
  CREATE UNIQUE INDEX idx_room_players_room_seat_active 
    ON room_players(room_id, seat_number) 
    WHERE status IN ('joined', 'ready');
  ```
* **Pros**: Preserves history while keeping database integrity intact.
* **Cons**: Requires executing a database schema migration.

---

## 3. Idempotency & Concurrency Design

### Q: How do we prevent a host from starting multiple battles for the same room simultaneously?
**A:**
1. **Application-level Pessimistic Lock**: When `StartBattle` is called, the Rooms service opens a transaction and queries the room row with `SELECT ... FOR UPDATE`. If the room's status is already `in_battle`, it aborts or returns the active battle.
2. **Database-level Constraint (Idempotency Index)**: We can introduce a partial unique index on the `battles` table to guarantee that a room can have at most one active battle at any given time:
   ```sql
   CREATE UNIQUE INDEX idx_battles_active_room_unique 
     ON battles(room_id) 
     WHERE status IN ('created', 'countdown', 'active');
   ```
   If a race condition slips past the application lock, the database will raise a unique key violation, preventing duplicate battle rows.

---

## 4. Possible Interviewer Q&A

### Q: How do we generate unique room codes?
**A:** We use a cryptographically secure random code generator that creates a short alphanumeric uppercase string (e.g., 6 characters like `DSAXYZ`). To handle collisions (which are rare but possible), we can retry the generation up to 3 times in a loop, checking the database for existence, or rely on a `UNIQUE` index constraint violation on `rooms.code` to trigger a retry.

### Q: What happens if a player disconnects during a match?
**A:** Disconnection does not change the room state or the battle state. The player's WebSocket connection drops, but their database entries in `room_players` and `battle_players` remain active. Upon reconnecting, the client queries the active room status, retrieves the battle ID, and continues requesting questions using their current pointer.
