# Future Direction: Domain Events in Room & Battle Lifecycle

This document outlines how the **DSAblitz** architecture is designed to support **Domain Events** in the future without requiring major refactoring, and details why the MVP intentionally uses synchronous service calls instead of an event bus.

---

## 1. Planned Domain Events

As the application grows, state transitions will publish events to notify interested modules. The primary domain events identified for the Room and Battle lifecycle include:

* **`RoomCreated`**: Published when a host creates a lobby.
* **`PlayerJoined`**: Published when a guest successfully joins the lobby.
* **`PlayerReady`**: Published when a player toggles their ready status.
* **`RoomReady`**: Published when both players in a lobby are ready.
* **`BattleStarted`**: Published when the battle is initialized and the question sequence is generated.
* **`BattleFinished`**: Published when the battle ends, rating updates are calculated, and scores are frozen.
* **`RoomClosed` / `RoomExpired`**: Published when a lobby is disbanded, closed by the host, or times out.

---

## 2. Intended Consumers

A decoupled event system will allow multiple downstream consumers to react to these events asynchronously:

```
┌─────────────────────────────────┐
│          Rooms/Battle           │
│         State Machine           │
└────────────────┬────────────────┘
                 │ Publishes
                 ▼
     [ Domain Events Channel ]
                 │
  ┌──────────────┼──────────────┬──────────────┐
  ▼              ▼              ▼              ▼
┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐
│WebSockets │  │Analytics  │  │Leaderboard│  │Social/Chat│
│ Presence  │  │ & Audit   │  │ & Elo     │  │Integrations
└───────────┘  └───────────┘  └───────────┘  └───────────┘
```

1. **WebSockets Presence Layer**:
   - Listens to `PlayerJoined`, `PlayerReady`, `RoomReady`, and `BattleStarted` to broadcast real-time state changes to connected clients in the lobby.
2. **Analytics & BI Tools**:
   - Consumes `RoomCreated`, `BattleStarted`, and `BattleFinished` to track user engagement, average lobby durations, and player retention.
3. **Achievements & Streaks Engine**:
   - Listens to `BattleFinished` to evaluate streak increments, daily challenges, and award achievement badges to users.
4. **Audit Logs & Anti-Cheat**:
   - Inspects `BattleStarted` and `BattleFinished` metadata to monitor question submission patterns and flag suspicious click rates.
5. **Discord/Slack Integrations**:
   - Broadcasts match completions to community channels.

---

## 3. Why the MVP Prefers Synchronous Service Calls

Instead of introducing an event bus (e.g. Redis Pub/Sub, Kafka, RabbitMQ) today, the MVP uses direct service calls (decoupled via Go interfaces like `rooms.BattleStarter`). 

### **The Tradeoffs Explained**

| Aspect | Direct Service Calls (MVP) | Event Bus (Future) |
| :--- | :--- | :--- |
| **Complexity** | **Very Low**: Standard method invocation. Easy to debug using standard IDE stack traces. | **High**: Requires message brokers, event serialization, error-channel handling, and retry dead-letter queues. |
| **Transaction Boundary** | **Atomic**: The database transaction is committed only when the child call succeeds. Clean rollbacks. | **Eventual Consistency**: Requires handling partial failures (e.g., Room state changes but Battle failed to start) via Sagas or Outbox patterns. |
| **Runtime Latency** | **Negligible**: Low microsecond execution in-process. | **Variable**: Network hops to/from message brokers. |
| **Scalability** | **Limited**: Single-binary bounded execution thread. | **High**: Multiple microservices can scale independently and consume events at their own pace. |

### **DSAblitz Decision Rationale**
For a 1v1 competitive coding platform starting its MVP phase, **reliability and database consistency are critical**. If a host starts a battle, we cannot afford to enter a state where the lobby thinks the game is active but the battle sequence failed to generate. Running the initiation steps synchronously inside a single Postgres transaction ensures a 100% guarantee of consistency.

---

## 4. How the Current Architecture Accommodates Domain Events

We have structured the codebase specifically to allow an easy migration to events:
1. **Hook Points**: The state machine service methods (e.g., `StartBattle` and `ToggleReady`) are isolated in their own transaction blocks. The publish call can be added at the very end of these methods immediately after transaction commit:
   ```go
   err = s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
       // state changes...
       return nil
   })
   if err == nil {
       s.publisher.Publish(ctx, RoomReadyEvent{RoomID: id})
   }
   ```
2. **Interface Abstraction**: By using Go interfaces for module communication, we can wrap our services in decorators that publish events without modifying any of the core domain logic.
3. **Outbox Pattern Preparation**: All state changes are written to relational tables in PostgreSQL. When migrating to an event bus, we can easily add an `outbox_events` table in the same transactions, guaranteeing that state updates and event publishing remain atomic.
