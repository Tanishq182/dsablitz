# Architectural Review: Phase 8A (Post-Refactoring ratings module)

This document provides a post-implementation architectural review of the completed Phase 8A ratings module.

---

## 1. Strengths
* **Strict Interface Separation**: The Ratings module only depends on Go interfaces for its internal math and coordinate abstractions. All battle and users domain parameters are isolated.
* **No Domain Spill**: Ratings has zero dependencies on `user_stats` or `users` packages. The Ratings repository only handles database logs for the `rating_history` table.
* **Domain Model Safety**: Raw primitives have been replaced with dedicated domain types (`Rating`, `PlayerRating`, `MatchResult`, `RatingUpdate`), eliminating primitive obsession and clarifying boundaries.
* **Pure Stateless Engine**: The `EloEngine` operates as a pure stateless math calculator taking domain inputs and returning delta outputs, with 100% test coverage and zero database or context dependencies.
* **Single Entrypoint outcome**: Standardized match resolution outcomes on a single enum (`Player1Win`, `Draw`, `Player2Win`) which makes swapping engines (e.g. to Glicko-2) trivial.
* **Pessimistic Serialization Control**: Lock ordering is managed at the orchestration level by calling User statistics updates first in `user_id ASC` order, followed by Ratings logging.

---

## 2. Weaknesses & Technical Debt
* **Transaction Span**: While the Rating coordinator is pure orchestration and has no SQL directly, it executes multiple database insertions. The parent transaction must be committed promptly by the caller (Battle Service) to release locked resources.

---

## 3. Future Scalability Risks
* **Weekly/Monthly Aggregations**: Summing deltas directly over `rating_history` in SQL will scale linearly. Once the match volume grows, we must execute the planned migration to Redis Sorted Sets (ZSET) to offload Postgres reads.

---

## 4. Rejected Alternatives
* **Updating user ratings directly in the Ratings Repository**: Rejected. Modifying `user_stats` from inside ratings creates circular dependencies. Shifting statistics and rating updates to a Users module `StatsCoordinator` keeps table boundaries clean.

---

## 5. Key Engineering Learnings
* **Clean Coordinators**: Orchestrating post-match updates via independent interfaces (`RatingCoordinator`, `StatsCoordinator`, `HistoryCoordinator`) provides maximum decoupling while keeping database mutations wrapped in a single transaction.
