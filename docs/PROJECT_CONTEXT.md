# DSAblitz

DSAblitz is a real-time 1v1 DSA battle platform.

> [!IMPORTANT]
> **Living Documentation Policy**: Documentation is a living artifact and must evolve alongside the code. Every completed phase must update architecture, roadmap, ADRs, interview notes, and deep dives before the phase is considered complete.

## Gameplay
- Players compete in 1v1 battles
- Match duration: 5 min or 10 min
- Continuous rapid-fire questions (shared stream, asynchronous progression)
- Faster solving = more questions attempted
- Score depends on:
  - correctness
  - speed
  - streak bonus

## Question Types
- MCQ
- Complexity prediction
- Numeric answer
- Algorithm ordering

## Tech Stack
* **Frontend**: React, Tailwind, Zustand
* **Backend**: Go, Gin, PostgreSQL, Redis, WebSockets

## Architecture
Modular Monolith.

### Modules:
* **auth**: handles credentials and JWT authentication.
* **users**: manages ratings (Elo/Glicko) and profiles.
* **rooms**: handles WebSocket connections, presence, matchmaking lobby state.
* **battle**: owns match progression indices, sequence mapping, and points calculations.
* **questions**: stateless reference data catalog.

---

## Locked Decisions (DO NOT CHANGE)

### **Backend Framework**
* Gin (mandatory)

### **MVP Gameplay Rules**
* **1v1 matches only**: No bracket/group lobbies.
* **Quiz-based format**: No sandboxed docker compilation environments (strictly multiple-choice, ordering, numeric, and complexity answers).
* **Option C progression policy**: Maximum 2 attempts per question. Correct on first/second attempt advances to next question. Failing twice skips the question with 0 points scored.
* **Deterministic Sequences**: Sequence generation is seed-based, deterministically shuffled using a stateful PRNG copy of the question bank.

### **In-Memory Caching**
* Questions module reads are offloaded from PostgreSQL using a local, thread-safe in-memory map cache (`sync.RWMutex`), loaded at application startup.

### **Concurrency & Locking**
* Player progression updates are secured using pessimistic row locking (`SELECT ... FOR UPDATE` on `battle_players`) within atomic transactions to prevent duplicate submissions or score tampering.
* **Monotonic Submission Index**: Duplicate and out-of-order submissions are prevented by verifying a client-supplied monotonic submission index (`submissionIndex`) against the player's server-computed expected index ($\text{correct\_count} + \text{incorrect\_count} + 1$) inside the locked transaction.
* **Configurable Scoring**: The engine delegates score calculations to an injected `ScoreCalculator` interface, allowing V2 to support difficulty-based scoring, streak bonuses, and penalties without altering the core submission engine.
* **Optimized Question Retrieval**: Retrieving a player's current question joins `battle_players`, `battles`, and `battle_question_sequence` in a single database query to minimize network roundtrips.
* **Idempotent Battle Completion**: Battle completion uses a service-managed transaction that locks the battle row and updates the status exactly once, computing the winner and releasing locked resources safely.

### **Global Lock Ordering Rule**
* Any transaction spanning multiple tables must acquire row locks in this exact hierarchy to prevent deadlocks: `rooms` -> `room_players` -> `battles` -> `battle_players` -> `battle_question_sequence`.

### **Dependency Inversion Rule**
* The Rooms module must never import the concrete Battle module. All cross-module gameplay actions (starting/aborting battles) must go through the Rooms-owned `BattleCoordinator` interface, wired at startup.

### **Transaction Boundary Rule**
* Room state transitions and battle initialization must run inside a single atomic database transaction to prevent inconsistent states (e.g. rooms marked `in_battle` without an associated battle record).
* **Shared Transaction Context**: Interfaces for cross-module mutations must accept the parent transaction handle (`pgx.Tx`) to execute on the same database connection. This prevents nested transactions, connection starvation, and connection-pool deadlocks.

### **Deterministic Row Locking Rule**
* Any transaction locking multiple rows `FOR UPDATE` concurrently (such as cleanups or batch updates) must order the target rows deterministically (e.g. `ORDER BY id ASC`) before locking to prevent deadlock cycles.

### **Postgres Abort & Retry Boundary Rule**
* Because PostgreSQL aborts transactions permanently upon any query error, retry loops (such as room code collision handling) must execute outside the transaction block so each retry starts a clean, fresh transaction.

### **Domain Event Future Direction**
* The architecture is designed to support asynchronous domain events (`RoomReady`, `BattleStarted`, `BattleFinished`) in the future. The MVP uses direct synchronous service calls for transaction consistency, but service hooks are positioned to transition to an event bus or outbox model later.

---

## Phase 5 Technical Debt
* **Distributed Cache Invalidation**: Currently, the Questions cache is read-only and loaded at startup. In V2, when Admin CRUD APIs are added, we must implement a **Redis Pub/Sub invalidation hook** to coordinate cache reloading across multiple nodes.
* **Synchronous Seeder Parsing**: The JSON seeder parses catalog files entirely in memory. For massive question banks, this must be refactored into a chunked file streamer.
* **Rating Persistence**: The battle completions hook does not update user Elo ratings yet (waiting for Users module database integration).

---

## Phase 5 Step 2 Audit Implementation
* **Transaction boundaries resolved**: Replaced nested transactions by propagating the parent transaction context through `BattleCoordinator` into `StartBattleTx`.
* **Domain constants aligned**: Replaced all hardcoded battle status strings with standard type-safe domain constants in models and repositories.
* **Deterministic row locking**: Fixed `ExpireRooms` by enforcing sorting `ORDER BY id ASC` before `FOR UPDATE`.
* **Postgres retry boundaries**: Moved room code generation retry logic outside of the transaction block.
* **Presence guard**: Prevented `LeaveRoom` during active battles.

---

## Roadmap
- Phase 1 Scaffold ✅
- Phase 2 Infra ✅
- Phase 3 Schema ✅
- Phase 4A Auth Design ✅
- Phase 4B Auth Implementation ✅
- Phase 4C Auth Hardening ⏳ Deferred
- Phase 5 Questions & Battle MVP ✅ (Audit items fully implemented)
- Phase 6 Rooms Lifecycle ⏳ Deferred
- Phase 7 Battle Engine 🎯 Next Phase
- Phase 8 Friends/Social
