# ADR 0002: Rating Engine Abstraction and Module Decoupling

This document answers the primary question: **Why did we abstract rating calculations from the Battle and Users modules, how is it designed, and how does it support future Glicko-2 migration without code modifications?**

---

## Status
**Approved** (Phase 8A Decision)

## Context
In competitive DSAblitz gameplay, players' ratings are adjusted upon match completion.
* **Tight Coupling Risk**: If the Battle module directly calculates Elo rating changes and updates the user tables, it creates tight coupling. The Battle module would require direct dependency on ratings math, the `user_stats` table schema, and database transaction structures.
* **Glicko-2 Swapping**: We plan to replace the Elo system with Glicko-2 in V2. Glicko-2 is mathematically complex and requires additional tracking parameters (Rating Deviation $RD$ and Volatility $\sigma$). If ratings calculations are embedded inside Battle, swapping engines requires rewriting Battle progression, risking logic bugs.

---

## Decision
We introduced a highly decoupled **Ratings Module** (`backend/internal/ratings`) following strict Clean Architecture rules:
1. **Stateless pure RatingEngine**: We defined `RatingEngine` as a pure, stateless calculation interface. It has no DB access, no repository, no context, and operates purely on domain objects (`PlayerRating`, `MatchOutcome`) returning `RatingUpdate` changes.
2. **Domain Objects instead of Primitives**: We avoided primitive obsession by introducing typed structures:
   * `Rating`: Wraps integer ratings to enforce domain semantic boundary.
   * `PlayerRating`: Encapsulates user ID and `Rating` value.
   * `MatchResult`: Bundles pre-match ratings, user IDs, and outcome.
   * `RatingUpdate`: Bundles rating before, after, and delta.
3. **Ratings Repository Boundary**: The `ratings.Repository` ONLY owns the `rating_history` table (append-only log). It contains no queries or updates to the `user_stats` table (which belongs to the Users module).
4. **RatingCoordinator Decoupling**: The Battle service interacts exclusively with the `RatingCoordinator` interface. It passes a `MatchResult` inside a transaction handle. The coordinator calculates rating updates, logs them to `rating_history`, and returns the `RatingUpdate` objects to the Battle service. Battle service then passes the updates to the Users module's `StatsCoordinator` to update `user_stats`.

---

## Alternatives Considered & Rejected

### Why not calculate ratings inside the Battle module?
* **Rejected**: It violates the Single Responsibility Principle (SRP). The Battle module's sole concern is evaluating user submissions and tracking gameplay loops. Mixing math-heavy Elo or Glicko equations there dilutes gameplay logic and clutters unit tests.

### Why not let Ratings module directly update user stats ratings?
* **Rejected**: Directly writing to `user_stats` from the Ratings repository violates database table ownership boundaries. Tables must be isolated. Ratings module should not be coupled to the `user_stats` schema or lock management.

---

## Architectural Tradeoffs

### Pros
* **Extensibility**: Replacing Elo with Glicko-2 requires implementing a new `RatingEngine` interface. No changes are required in the Battle module or Users module.
* **Excellent Testability**: Pure mathematical engines require no database seeds, contexts, or mocks to reach 100% code coverage.
* **Zero Coupling**: Clean separate repository boundaries prevent table joins across modules.

### Cons
* **Orchestration Overhead**: The Battle module must perform coordinating calls to the Ratings module (to log history and get updates) and Users module (to persist rating changes). However, this overhead is minimal and clean.

---

## Related Documents
* See [overall_architecture.md](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md).
* See [ratings_stats_leaderboards_architecture.md](file:///home/tanishq/.gemini/antigravity-cli/brain/9147f1d4-f1a9-4885-bdb4-558216e9cf8d/ratings_stats_leaderboards_architecture.md).
