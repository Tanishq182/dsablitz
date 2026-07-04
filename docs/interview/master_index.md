# DSAblitz Interview Preparation Index

Welcome to the DSAblitz technical interview preparation repository. This directory contains comprehensive documentation, Q&A guides, architectural deep-dives, and production failure scenarios designed to prepare candidates for engineering interviews at various seniority levels.

---

## 📂 Interview Guide Directory Structure

The interview guides are organized by technical domains and behavioral categories under `docs/interview/`:

```text
docs/interview/
├── master_index.md                 # This index file
├── project/                        # Project Architecture (Monolith boundaries, Option C policy)
│   ├── graduate.md | sde1.md | senior.md
├── go/                             # Go Programming Language (RWMutex, Memory Model, Cache Bouncing)
│   ├── graduate.md | sde1.md | senior.md
├── gin/                            # Gin Framework (Middleware chains, Context recycling, Radix Trees)
│   ├── graduate.md | sde1.md | senior.md
├── postgres/                       # PostgreSQL Database (Keys, Check constraints, GIN indices, Locking)
│   ├── graduate.md | sde1.md | senior.md
├── pgx/                            # Pgx Database Driver (Pool stats, Transaction propagation, Starvation)
│   ├── graduate.md | sde1.md | senior.md
├── jwt/                            # JWT Security (Cookie paths, Token rotations, RTR reuse detection)
│   ├── graduate.md | sde1.md | senior.md
├── testing/                        # Testing Strategies (Table-driven, Integration mocks, Transaction mocks)
│   ├── graduate.md | sde1.md | senior.md
├── concurrency/                    # Concurrency Controls (Row locking, Lock hierarchies, Deadlock prevention)
│   ├── graduate.md | sde1.md | senior.md
├── system-design/                  # System Design (Caching, Monotonic indexes, Scale-out strategies)
│   ├── graduate.md | sde1.md | senior.md
├── behavioral/                     # Behavioral Scenarios (Technical debt, conflicts, decision iterations)
│   ├── graduate.md | sde1.md | senior.md
├── framework/                      # Frameworks & Patterns (Router setup, Dependency Injection, Service Layer)
│   ├── graduate.md | sde1.md | senior.md
├── cross-framework/                # Cross-Framework Comparisons (Gin vs Chi, Go vs Spring, JWT vs Sessions)
│   ├── graduate.md | sde1.md | senior.md
├── hr/                             # HR Expectations (Culture fit, team leadership, mentorship)
│   ├── graduate.md | sde1.md | senior.md
└── mock-interviews/                # Realistic Mock Sessions (Graduate: 45m, SDE1: 1h, Senior: 90m)
    └── graduate.md | sde1.md | senior.md
```

---

## 🎯 Role Mapping & Candidate Expectations

We categorize our interview questions into three distinct levels of engineering expertise:

### 🎓 1. Graduate / Internship Level (`**/graduate.md`)
*   **Focus**: Foundational concepts, basic programming paradigms, API endpoints, relational schemas, simple unit tests, and structural models.
*   **Key Topics**:
    *   Basic Go syntax, slices, maps, and thread safety.
    *   Gin route mappings, request body bindings, and status codes.
    *   PostgreSQL primary/foreign key definitions and basic indices.
    *   Client session signup/login flow and secure cookie flags.
    *   Clean Separation of Concerns across packages.

### 🛠️ 2. SDE-1 / Production Engineer Level (`**/sde1.md`)
*   **Focus**: Intermediate system design, unit/integration testing mocks, database performance, transaction scopes, and boundary enforcement.
*   **Key Topics**:
    *   Trie-based Gin routing lookups and custom authentication middleware.
    *   Seed-based deterministic sequence generation for fair matchmaking.
    *   Go RWMutex cache concurrency mechanics and memory visibility.
    *   Centralized error handling and transaction propagation using `pgx.Tx` handles.
    *   Stateless unit validation and mock repository adapters.

### 🧠 3. Senior SDE / Tech Lead Level (`**/senior.md`)
*   **Focus**: High-concurrency architectures, connection pool starvation, deadlock analysis, cache coherence, distributed systems, and production debugging.
*   **Key Topics**:
    *   Lock-free caches, cache line bouncing, and atomic swaps.
    *   Global lock hierarchies and deterministic row locking (`ORDER BY id ASC FOR UPDATE`).
    *   PostgreSQL transaction poisoning abort behaviors and retry patterns outside transactions.
    *   Distributed cache invalidation via Redis Pub/Sub brokers.
    *   Zero-downtime database migrations (backwards compatibility, lock avoidances).

---

## 📚 Related Documentation

To gain a complete understanding of the DSAblitz codebase before diving into the interview notes, please review these key references:

*   **Overall Project Intent**: [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)
*   **Database Schema & Strategy**:
    *   [schema.md](file:///home/tanishq/dsablitz/docs/database/schema.md)
    *   [indexing.md](file:///home/tanishq/dsablitz/docs/database/indexing.md)
    *   [transactions.md](file:///home/tanishq/dsablitz/docs/database/transactions.md)
*   **API & Flow Specifications**:
    *   [auth.md](file:///home/tanishq/dsablitz/docs/api/auth.md)
    *   [rooms.md](file:///home/tanishq/dsablitz/docs/api/rooms.md)
    *   [battle.md](file:///home/tanishq/dsablitz/docs/api/battle.md)
    *   [login_flow.md](file:///home/tanishq/dsablitz/docs/flows/login_flow.md)
    *   [submission_flow.md](file:///home/tanishq/dsablitz/docs/flows/submission_flow.md)
