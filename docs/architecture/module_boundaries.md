# Architectural Specification: Module Boundaries

This document defines the strict ownership, dependencies, database partitions, and transactional rules for **DSAblitz**.

---

## 1. Module Ownership

```
┌────────────────────────────────────────────────────────┐
│                        Monolith                        │
│                                                        │
│  ┌───────────┐      ┌───────────┐      ┌────────────┐  │
│  │   Auth    │      │   Rooms   │      │   Battle   │  │
│  └─────┬─────┘      └─────┬─────┘      └─────┬──────┘  │
│        │                  │                  │         │
│        ▼                  ▼                  ▼         │
│  ┌───────────┐      ┌───────────┐      ┌────────────┐  │
│  │   Users   │◄─────┤   Users   │      │ Questions  │  │
│  └───────────┘      └───────────┘      └────────────┘  │
└────────────────────────────────────────────────────────┘
```

* **Auth**: Manages user authentication, password hashing, credentials verification, and JWT generation.
* **Users**: Stores user profiles, historical stats, ratings (Glicko/Elo), and metadata.
* **Questions**: Stateless catalog containing question bank metadata and stateless validation functions.
* **Battle**: Core stateful match engine that tracks live player progression, sequence generation, submissions, and scoring.
* **Rooms**: Orchestrates game lobbies, WebSocket connections, matchmaking queues, and player presence.

---

## 2. Database Table Ownership

Table access is partitioned by module ownership. Direct SQL queries across partitions are prohibited. Instead, modules must request data via service interfaces.

| Table | Owning Module | Allowed Read-Write Access | Allowed Read-Only Access |
| :--- | :--- | :--- | :--- |
| `users` | Users | Users Module | Auth, Rooms, Battle |
| `auth_credentials` | Auth | Auth Module | None |
| `questions` | Questions | Questions Module (Seeder) | Battle Module |
| `question_stats` | Questions | Questions Module | None |
| `battles` | Battle | Battle Module | Rooms Module |
| `battle_players` | Battle | Battle Module | Rooms Module |
| `battle_question_sequence` | Battle | Battle Module | None |
| `submissions` | Battle | Battle Module | None |

---

## 3. Dependency Rules

To prevent circular dependencies and compilation errors in Go:
1. **Questions Module**: Completely independent. Must never import `Battle`, `Rooms`, `Auth`, or `Users`.
2. **Users Module**: Independent. Must never import other business modules.
3. **Battle Module**: May import `Questions` (for question lookups and validates) and `Users` (for rating updates). Must never import `Rooms`.
4. **Rooms Module**: May import `Battle` (to trigger battle creation and retrieve state) and `Users`.
5. **Auth Module**: May import `Users`.

---

## 4. Transaction Boundaries

Database transactions must be atomic and confined within a single service layer execution context.

* **Battle Module**: Starts and commits read-write transactions for Match Initializations and Answer Submissions.
* **Questions Module**: Must never start write transactions during runtime (only reads from read-only Go memory cache).
* **Rooms Module**: Must never start write transactions affecting Battle tables.

---

## 5. State Ownership

* **Battle Progression** (`current_question_index`): **Battle Module**.
* **Submissions**: **Battle Module**.
* **Score**: **Battle Module**.
* **Timers**: **Rooms Module** (driven ephemerally in Redis/WebSockets).
* **Question Validation**: **Questions Module** (stateless comparison).
* **Question Catalog**: **Questions Module** (static reference).
* **Room Lifecycle**: **Rooms Module** (WebSocket connection and lobby state).

---

## 6. Infrastructure Ownership

* **Postgres**: Co-owned but table partitions are strictly respected.
* **Redis**: Co-owned; primarily driven by the **Rooms Module** (presence, matchmaking queue, locks) and ephemeral cache utilities.
* **WebSockets**: Owned by the **Rooms Module**.
* **JWT**: Owned by the **Auth Module**.
