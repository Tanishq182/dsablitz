# Deep Dive: Room State Machine

This document defines the formal state machine, transitions, database-level invariants, and runtime business rules for Rooms in **DSAblitz**.

---

## 1. Room States

A Room progresses through the following states (defined by the database constraint `rooms_status_check`):

* **waiting** (Default): The room is open for players to join. It requires exactly 1 host player and can accept up to 1 guest player.
* **ready**: The room is full (2 players), and both players have set their status in `room_players` to `ready`.
* **in_battle**: The host has started a battle. Gameplay is active. No joins or leaves are permitted on the room presence layer.
* **closed** (Terminal): The host has disbanded/left the room, or the room has been shut down. No further actions are allowed.
* **expired** (Terminal): The room exceeded its idle time limit (`expires_at`) before starting a battle.

---

## 2. State Transition Matrix

The table below defines the valid transitions between Room states, their triggers, and the associated side effects.

| Source State | Target State | Triggering Event | Validations / Preconditions | DB Side Effects & Updates |
| :--- | :--- | :--- | :--- | :--- |
| *None* | **waiting** | Host creates room | Input check: duration ∈ {120, 300} | Insert Room (`status = 'waiting'`). Insert Host Player (`seat = 1, status = 'joined'`). |
| **waiting** | **ready** | Player toggles ready | Active player count = 2 AND both players `ready` | Update Room (`status = 'ready'`). |
| **waiting** | **closed** | Host leaves room | Leaving user is room host | Update Room (`status = 'closed'`). Update all players (`status = 'left', left_at = NOW()`). |
| **waiting** | **expired** | Expiry cron runs | `NOW() > expires_at` | Update Room (`status = 'expired'`). |
| **ready** | **waiting** | Player toggles unready | Either player toggles ready to `false` | Update Room (`status = 'waiting'`). |
| **ready** | **waiting** | Guest player leaves | Guest user leaves room | Update Guest (`status = 'left', left_at = NOW()`). Update Room (`status = 'waiting'`). |
| **ready** | **in_battle** | Host starts battle | Host user initiates; both players `ready` | Call Battle service. Insert Battle and Sequence. Update Room (`status = 'in_battle'`). |
| **ready** | **closed** | Host leaves room | Leaving user is room host | Update Room (`status = 'closed'`). Update all players (`status = 'left', left_at = NOW()`). |
| **ready** | **expired** | Expiry cron runs | `NOW() > expires_at` | Update Room (`status = 'expired'`). |
| **in_battle** | **waiting** | Battle finishes | Battle status transitions to completed/aborted | Reset all active players (`status = 'joined'`). Update Room (`status = 'waiting'`). |
| **in_battle** | **closed** | Battle ends + Disband | Host leaves or closes room | Update Room (`status = 'closed'`). Update all players (`status = 'left'`). |

---

## 3. Player Presence vs. Room Status

The status of the room is a computed function of the individual statuses of its participants in `room_players`:

* **Room is `waiting`**:
  - Participant 1 (Host): status = `joined` or `ready`.
  - Participant 2 (Guest): status = `joined` (or row does not exist).
* **Room is `ready`**:
  - Participant 1 (Host): status = `ready`.
  - Participant 2 (Guest): status = `ready`.
* **Room is `in_battle`**:
  - Participant 1 (Host): status = `ready` (or locked/active).
  - Participant 2 (Guest): status = `ready` (or locked/active).
* **Room is `closed` / `expired`**:
  - All participants: status = `left` or `kicked` with `left_at` set.

---

## 4. Key Invariants & Safeguards

To maintain consistency and prevent illegal state progressions, the following business rules are enforced programmatically:

1. **No Mid-Game Joining**:
   - In `in_battle`, `closed`, or `expired` states, calls to `JoinRoom` must be rejected immediately at the database query phase via pessimistic checking.
2. **Strict Seating**:
   - Seat numbers are restricted to `1` (Host) and `2` (Guest). A joining user cannot take a seat that is already occupied by a player in `joined` or `ready` status.
3. **Pessimistic Isolation**:
   - All state transition logic must run within database transactions wrapped in a `SELECT FOR UPDATE` lock on the specific room row. This prevents two concurrent operations (e.g., Guest readying up while Host leaves) from producing inconsistent mixed states.
