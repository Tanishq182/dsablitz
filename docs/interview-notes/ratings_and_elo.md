# Interview Notes: Ratings & Elo Algorithm

This document outlines key technical discussions, possible interviewer questions, tradeoffs, scaling strategies, and design decisions regarding the Ratings and Elo system in **DSAblitz**.

---

## 1. Possible Interviewer Questions & Answers

### Q: Why does the Rating Engine abstraction exist?
**A:** The `RatingEngine` abstraction decouples the core rating computation rules from database calls, transaction context, and other system modules (like Battle and Users). This isolation serves two primary purposes:
1. **Stateless Logic**: The engine is a pure mathematical calculator taking `PlayerRating` and `MatchOutcome` domain objects and returning `RatingUpdate` objects. This allows testing calculations (upsets, draws, rating floor) offline with zero database or mock overhead.
2. **Pluggable Architecture**: We can swap rating algorithms (e.g. from Elo to Glicko-2) by implementing the `RatingEngine` interface with a new calculator, leaving the rest of the coordinator service and database persistence code completely untouched.

### Q: Why not calculate Elo inside the Battle module?
**A:** Calculating ratings inside Battle violates the Single Responsibility Principle (SRP).
* **Battle Module Role**: Responsible strictly for match progression loops, evaluating question submissions, and checking timeouts.
* **Separation of Concerns**: Calculating Elo changes requires math formulas, while saving the history requires updating the `rating_history` table. Placing this logic in Battle would tightly couple it to the ratings mathematical model and schema.
* **Test Isolation**: Keeping them separate allows us to unit test the Battle state machine and the ratings logic independently.

### Q: What is the migration path to Glicko-2?
**A:** Since Glicko-2 is planned for V2:
1. **Historical Replays**: Our schema records every match result in an append-only `rating_history` table. If we migrate, we can run a migration script that replays historical matches in chronological order to bootstrap Glicko-2 values for all users.
2. **Table Schema Extension**: We will add `rating_deviation` ($RD$) and `volatility` ($\sigma$) columns to the database (managed by the Users or a dedicated ratings metadata store).
3. **Engine Implementation**: We implement `Glicko2Engine` satisfying the `RatingEngine` interface.
4. **Zero Impact on Battle**: Since Battle interacts with ratings only through the `RatingCoordinator` interface, we swap the engine implementation at startup in dependency injection without altering a single line of Battle code.

### Q: Why did you decouple repository tables across Ratings and Users?
**A:** In modular monoliths, modules should only execute queries on tables they own.
* **Ratings Module**: Owns the `rating_history` table. Its repository has no database methods accessing the `user_stats` table (which belongs to the Users module).
* **Data Decoupling**: Rating calculations only require pre-match ratings. The Battle service retrieves pre-match ratings from `battle_players` and passes them to `RatingCoordinator` inside a `MatchResult` struct. This keeps the Ratings module completely independent of the Users module database tables.

---

## 2. Tradeoffs & Alternative Designs

### The Coordinator Pattern Tradeoffs
* **How it works**: The Battle module determines the match winner/loser/draw, then invokes independent coordinator interfaces (`RatingCoordinator`, `StatsCoordinator`, `HistoryCoordinator`) to execute post-match tasks inside a shared transaction.
* **Pros**:
  * **Strict Modularity**: Each module (Ratings, Users, History) remains isolated, owning its database tables and business logic.
  * **Transactional Consistency**: All updates run inside the same database transaction propagated from the Battle service. If any write fails, the entire match completion rolls back.
* **Cons**:
  * **Orchestration Complexity**: The Battle module must hold references to multiple coordinator interfaces and manage their calling sequence. However, this is a clean architectural tradeoff that yields high maintainability and prevents circular imports at compile time.
