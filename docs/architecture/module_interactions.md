# Runtime Module Interactions

This document answers the primary question: **How do the modular monolithic modules interact at runtime, and what are their transaction and data ownership boundaries?**

---

## 1. Runtime Collaboration Paths

Core modules remain completely decoupled in terms of source dependency compile-time limits, but collaborate at runtime through interface adapters. There are two primary runtime interaction paths:

### Path A: Room Starting a Battle (Transaction Propagation)
When a room becomes full and both players are ready, the `rooms` module initiates the battle:
1. **Lobby Context**: The `rooms.Service` begins a database transaction.
2. **Adapter Handshake**: It calls `BattleCoordinator.StartBattleTx` ([service.go](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L337)), passing the current `tx pgx.Tx` handle to propagate the transaction context.
3. **Gameplay Context**: The `battle.Service` writes gameplay rows (initializing `battles`, sequence mappings, and `battle_players` slots) on the *same transaction*.
4. **Completion**: The `rooms.Service` transitions the room status to `in_battle` and commits. If any database write fails, the entire transaction rolls back.

```
┌──────────────┐         StartBattleTx(tx)         ┌──────────────┐
│ rooms.Service│ ────────────────────────────────> │battle.Service│
└──────┬───────┘                                   └──────┬───────┘
       │ (Locks room row)                                 │ (Inserts battles on tx)
       ▼                                                  ▼
[ Commit or Rollback both Room and Battle states atomically ]
```

### Path B: Battle Evaluating Answers (Stateless Dependency)
During gameplay, a player submits an answer:
1. **Pessimistic Lock**: The `battle.Service` begins a transaction and locks the individual player's progress row.
2. **Stateless Lookup**: It calls `QuestionsService.GetSanitizedQuestion` to get difficulty and metadata.
3. **Validation**: It calls `QuestionsService.ValidateAnswer` ([service.go](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L246)) to run regex and content matches.
4. **No Shared Transaction**: Because `QuestionsService` operations are purely read-only and CPU-bound (resolved against the thread-safe local cache), these queries run outside the battle database transaction.

---

## 2. Ownership Boundaries & Data Flow

* **Room Module**:
  * *Owns*: `rooms`, `room_players` tables.
  * *Boundary*: Dictates who is allowed to enter a battle.
* **Battle Module**:
  * *Owns*: `battles`, `battle_players`, `battle_question_sequence`, and `submissions` tables.
  * *Boundary*: Dictates progression, attempts count, scoring, and winners.
* **Questions Module**:
  * *Owns*: In-memory questions map, CSV importing runtime.
  * *Boundary*: Dictates whether an answer matches structural patterns.

---

## 3. Cross-References
* For detail on transaction boundaries, see [transaction_boundaries.md](file:///home/tanishq/dsablitz/docs/deep-dives/transaction_boundaries.md).
* For detailed database mapping, see [schema.md](file:///home/tanishq/dsablitz/docs/database/schema.md).
* For step-by-step lifecycles, see [request_lifecycle.md](file:///home/tanishq/dsablitz/docs/architecture/request_lifecycle.md).
