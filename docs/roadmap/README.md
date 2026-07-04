# DSAblitz Product & Feature Roadmap

This document outlines the current state of implementation for the DSAblitz platform, separating completed features from planned future enhancements.

---

## Completed Features (MVP Scope)

The following capabilities are fully implemented, tested, and integrated into the backend:

### 1. Stateless Questions Engine
* **In-Memory Cache**: Question lookup from a local startup cache ([service.go](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L18)) protected by `sync.RWMutex`.
* **CSV Import Pipeline**: Automatic parsing of static question lists during startup initialization ([service.go](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L35)).
* **Multi-Format Validation**: Stateless verification algorithms supporting MCQ, predictive coding complexity, numerical results, and block-ordering ([validation.go](file:///home/tanishq/dsablitz/backend/internal/questions/validation.go#L10)).

### 2. Real-Time Rooms & Lobbies
* **Lobby State Machine**: Thread-safe room transition stages (`waiting`, `ready`, `in_battle`, `closed`) orchestrated by pessimistic lock boundaries ([service.go](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L25)).
* **Capacity Safeguards**: Enforcement of room size rules, ensuring a maximum of 2 active players per lobby before triggering battle states.
* **Idempotency Protections**: Verification of join/leave events to block duplicate state mutations ([service.go](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L110)).

### 3. Gameplay Battle Engine
* **Deterministic Sequences**: Seed-based question streams generated and shuffled via a custom PRNG state machine ([service.go](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L316)).
* **Option C Progression**: The gameplay state machine enforcing a maximum of 2 attempts per question. Correct answers advance index and score; double incorrect answers trigger automatic question skips ([service.go](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L251)).
* **Monotonic Submission Counter**: Rejection of duplicate or out-of-order answers based on transaction-verified submission counts ([service.go](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L228)).
* **Configurable Scoring**: Injection of `ScoreCalculator` wrappers allowing basic or advanced scoring logic ([service.go](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L28)).

### 4. Background Expiration & Timers (Phase 7B)
* **Active Expiration Handlers**: A centralized background cleanup runner executing periodic checks to expire idle lobbies ([rooms/service.go](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L427)) and automatically finalize active battles when their duration timer expires ([battle/service.go](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L418)).

---

## Planned Work (V2)

The following components are scheduled for future development phases and are **not** currently implemented in the codebase:

### 1. Elaborations of Battle Lifecycle
* **Websocket State Push**: Real-time push notifications of question and timer transitions.

### 2. Rating & Statistics (Phase 8)
* **Elo Calculations**: Dynamic adjustment of user matchmaking ratings (MMR) based on win/loss outcomes at battle completion.
* **User Stats Service**: Compilation of total matches played, overall accuracy, streaks, and platform placement leaderboards.

### 3. Matchmaking Queue (Phase 9)
* **Lobby Pools**: Centralized waiting pool matching players with comparable Elo ratings.
* **Matchmaking Loop**: A background worker polling the pool and creating ready rooms.

---

## Related Documents
* For system-wide structure, see [overall_architecture.md](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md).
* For terms and concepts definition, see [glossary](file:///home/tanishq/dsablitz/docs/glossary/README.md).
