# Architectural Review: Phase 5 (Questions & Battle MVP)

This document provides a comprehensive post-implementation architectural review of Phase 5.

---

## 1. Strengths
* **Absolute Stateless Separation**: The Questions module is fully isolated. It has zero dependencies on game sessions or scoring rules, facilitating cache scalability.
* **Database I/O Relief**: The in-memory cache in the Questions module completely eliminates PostgreSQL reads for question fetching and submission checks during matches.
* **Strong Type Safety**: Standardized on type-safe `uuid.UUID` models across Go structs and pgx database queries instead of unsafe string conversions.
* **Pessimistic Serialization**: Row-locking (`FOR UPDATE` on `battle_players`) guarantees atomic, race-free submission evaluations and blocks double-scoring attempts.

---

## 2. Weaknesses & Technical Debt
* **Lack of Cache Coherency Broker**: The local Questions cache only reloads at startup. If V2 Admin APIs are added, we must implement a Redis Pub/Sub invalidation broker.
* **Synchronous Seeder Parsing**: The JSON seeder parses and validates the entire catalog file in-memory. For massive question banks, this must be refactored into a chunked streaming reader.
* **Mock Coupling in Tests**: Mock structures in unit tests are coupled to repository structures rather than a fully segregated client mock framework.

---

## 3. Future Scalability Risks
* **In-Memory Cache Size**: Storing questions locally in a Go map scale-out. If the question bank grows to $10,000+$ questions, RAM consumption per instance will grow (though it remains highly manageable at $<50$ MB).
* **Pessimistic Lock Connection Starvation**: Under high concurrency (e.g. 100k active matches), keeping database transactions open while validating submissions holds PostgreSQL connections longer. If validation logic incurs latency (e.g. if the Questions module calls external services), it could starve the pgxpool connection pool.

---

## 4. Rejected Alternatives
* **Redis Caching for Questions**: Rejected in favor of Go RAM maps. Local map access takes ~50ns vs. Redis's ~1ms network hop.
* **Room-Seed Question Streams**: Rejected room-based sequences. A room can host multiple battles; using room seeds would regenerate identical streams.
* **UNIQUE(title) Database Constraint**: Rejected because separate questions can share identical titles (e.g., "Selection Sort Complexity").

---

## 5. Key Engineering Learnings
* **Defensive Domain Invariants**: Separating structural constraints (UUID, non-empty titles) into the entity `Validate()` method while moving business rules (option checks) to the seeder layer keeps models clean and reuseable.
* **Pessimistic DB Locks**: Direct row locking in PostgreSQL remains the most robust, transaction-safe approach to eliminate duplicate submissions at the database level.
