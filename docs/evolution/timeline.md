# Systems Architecture Evolution Timeline

This document tracks how the architecture of DSAblitz evolved phase-by-phase, detailing why decisions were made, the problems encountered, design mistakes resolved, and lessons learned.

---

## Phase 1: Authentication & User Management

### Why Introduced
To establish user identity context, secure gameplay endpoints, and prevent anonymous connection exploits.

### Alternatives Considered & Rejected
* **Why not OAuth2/OIDC Providers (e.g. Auth0, Keycloak)?**
  * *Reasoning*: Added external service runtime dependencies. For an MVP focused on developer coding battles, custom JWT management keeps setup self-contained and minimizes database queries.
* **Why not Statefully Sessions (Redis-backed)?**
  * *Reasoning*: Requires managing a separate cache service (Redis) and checking session keys on every single websocket event, introducing database/network hops.

### Design Mistakes & Iterations
* **Mistake**: Initially, JWT keys were hardcoded in the codebase configurations.
* **Refactor**: Moved key parameters to dynamic environmental variable loading, using a structured token manager abstraction inside [manager.go](file:///home/tanishq/dsablitz/backend/internal/auth/manager.go#L16).

---

## Phase 2: Stateless Questions Module

### Why Introduced
To manage the DSA coding questions stream and perform stateless validations of user submissions.

### Alternatives Considered & Rejected
* **Why not Dynamic Database Querying per Lookup?**
  * *Reasoning*: High query volume from multiple concurrent players would saturate the connection pool. We cached the question bank at startup.
* **Why not Redis Cache?**
  * *Reasoning*: A local in-memory map cache is faster (nanoseconds latency) and avoids Redis network serialization overhead.

### Design Mistakes & Iterations
* **Mistake**: Initial design queried database sequence mappings directly inside the stateless `questions` module, which created a dependency on gameplay parameters.
* **Refactor**: Extracted sequence mappings and player indices completely to the `battle` module, making `questions` 100% stateless and decoupled. See [service.go](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L18).

---

## Phase 3: Matchmaking Rooms & Lobbies

### Why Introduced
To group players into competitive matchmaking lobbies before creating battles.

### Alternatives Considered & Rejected
* **Why not In-Memory Lobby Map?**
  * *Reasoning*: Disallows horizontal scaling and results in data loss if the server restarts. We persisted lobby states in PostgreSQL.
* **Why not WebSockets for Lobby Operations?**
  * *Reasoning*: Keep simple CRUD actions (create, join, leave) on REST HTTP endpoints, reserving WebSockets purely for real-time battle loops.

### Design Mistakes & Iterations
* **Mistake**: The `rooms` module directly imported the `battle` package to initialize games, creating a circular dependency.
* **Refactor**: Applied Dependency Inversion. The `rooms` module declared a `BattleCoordinator` interface, implemented by `battle` and injected at startup in [routes.go](file:///home/tanishq/dsablitz/backend/internal/server/routes.go#L61).

---

## Phase 7A: Battle Gameplay Engine

### Why Introduced
To coordinate active match lifecycles, enforce progression logic (Option C), and log submissions.

### Alternatives Considered & Rejected
* **Why not Client-Specified Question ID?**
  * *Reasoning*: Subject to cheating. A client could submit answers to questions they haven't unlocked yet. The server derives the question ID from the player's progression pointer.
* **Why not 300ms Cooldown Rate-Limiting?**
  * *Reasoning*: Clock drifts and network latency make time-based blocks non-deterministic. We replaced it with a monotonic submission index parameter.

### Design Mistakes & Iterations
* **Mistake**: Hardcoded basic +1 scoring directly in the submission loop.
* **Refactor**: Introduced `ScoreCalculator` interface to decouple scoring logic, allowing V2 extensions without modifying the database transaction block. See [service.go](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L28).

---

## Key Takeaways
1. **Decouple Early**: Interface adapters prevent circular dependencies.
2. **Pessimistic Serialization**: Row locks prevent double-join or double-submit concurrency issues.
3. **Database Simplicity**: Keep state machines normalized in the database rather than inventing complex state synchronizations.

## Common Interview Questions
* **How did you prevent circular dependencies?**
  * *Answer*: Through Dependency Inversion. We defined a `BattleCoordinator` interface in the `rooms` package which is implemented by the `battle` package adapter and wired at server startup.

## Related Documents
* For architecture design details, see [overall_architecture.md](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md).
* For database schema design, see [schema.md](file:///home/tanishq/dsablitz/docs/database/schema.md).
