# Battle Lifecycle & State Machine

This document defines the lifecycle states, state transitions, and module ownership boundaries of a multiplayer coding match (Battle) in **DSAblitz**.

---

## 1. Battle State Machine

A Battle progresses through three distinct states: `pending`, `active`, and `completed`.

```
        Room lobby full / Game Launch
                    │
                    ▼
            ┌───────────────┐
            │    Pending    │ (Sequence pre-generated, players initialized)
            └───────┬───────┘
                    │ Countdown ends (e.g., 5 seconds)
                    ▼
            ┌───────────────┐
            │    Active     │ (Match timer ticks, players progress)
            └───────┬───────┘
                    │ Match timer ends / Both players finish / Admin abort
                    ▼
            ┌───────────────┐
            │   Completed   │ (Scores frozen, ratings recalculated)
            └───────────────┘
```

### **State Descriptions**
* **Pending**: The battle is created in Postgres, player slots are initialized, and the 200-question sequence is deterministically pre-generated. The game client displays a countdown (e.g., 5 seconds). No submissions are accepted in this state.
* **Active**: Players receive questions and submit answers. Pointers increment dynamically. Match duration (5 or 10 minutes) counts down.
* **Completed**: Submissions are closed. Scores are frozen, the winner is determined, and player ratings are updated in the database.

---

## 2. Event & State Ownership

### **Timer Expiry**
* **Owner**: **Rooms Module** (WebSockets connection loop).
* **Mechanism**: The WebSocket server maintains the match timer in Redis or memory. When the timer expires, the Rooms module triggers the battle termination logic.

### **Battle Completion**
* **Owner**: **Battle Module** (Persisted State).
* **Mechanism**: The Rooms module calls `battleService.CompleteBattle(ctx, battleID)`. The Battle service updates the battle status to `completed` in PostgreSQL, freezing scores and submissions.

### **Rating Updates**
* **Owner**: **Users Module** (Rating Logic).
* **Mechanism**: Upon transition to `completed`, the Battle Service calculates the winner, retrieves the players' original ratings, calls the Users Module to compute rating changes, and persists the new ratings.

### **Reconnect Handling (Future)**
* **Owner**: **Rooms Module** (WebSocket Hub).
* **Mechanism**: When a disconnected player reconnects, the WebSocket connection fetches the active room data. It then queries the Battle Module (`battleService.GetNextQuestion`) to retrieve the current pointer, allowing the player to resume the battle.
