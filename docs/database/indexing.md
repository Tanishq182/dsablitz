# Indexing & Query Patterns

This document details the indexing strategy of the PostgreSQL database, detailing every index currently defined in migrations and how they map to specific query patterns in the repository code.

---

## 1. Purpose

Indexes accelerate read queries by avoiding expensive sequential scans (O(N) table scans) in favor of fast index lookups (O(log N) tree navigations). In real-time 1v1 matchmaking and gameplay, sub-50ms query response times are crucial, making optimized index selection critical.

---

## 2. Design Rationale

### Why this design?
- **Pessimistic Key Indexes**: Primary keys (UUIDs) and foreign keys are explicitly indexed. PostgreSQL automatically indexes primary keys and unique constraints, but foreign keys must be indexed manually to prevent table locks and slow joins when deleting parent rows.
- **Partial Indexes**: Partial indexes (e.g., `WHERE status IN ('joined', 'ready')` or `WHERE revoked_at IS NULL`) are used to exclude irrelevant historical data. This reduces the index storage footprint and ensures index pages fit entirely in RAM, improving cache efficiency.
- **Composite (Compound) Indexes**: Multi-column indexes (like `submissions(battle_id, user_id, submitted_at)`) are designed to match queries that filter on multiple fields, or query patterns that filter on one field and sort by another (e.g., sorting leaderboards by rating or querying submission history by timestamp).
- **Specialized GIN Indexes**: The static `questions` table uses GIN (Generalized Inverted Index) indexing on array columns (specifically `tags`) to support fast intersection searches (`&&`), avoiding slow text parsing operations.

### Alternatives Considered

#### Why append-only submissions?
- *Rejected Alternative*: Updating a single scorecard row in `battle_players` and overwriting attempt records instead of logging them in `submissions`.
- *Rationale for Rejection*: Logging attempts in an append-only `submissions` table creates an audit log of player activity. This log is crucial for anti-cheat analysis, latency tracking, and resolving scoring disputes. To optimize lookup speeds on this append-only data, we use composite indexes (`idx_submissions_battle_user_submitted_at`) to retrieve recent user submissions quickly.
- *Tradeoffs*: Append-only writes consume more disk space over time. This requires table partitioning or archiving completed matches to cold storage.

---

## 3. Current Indexes Catalog

The following is a comprehensive list of all **34** indexes currently present in the PostgreSQL database migrations. No planned or non-existent indexes are documented.

### 3.1 Auth & Users Module

#### Table: `users`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`users_email_key`**: Implicit unique index on `email` (for login lookups).
*   **`users_handle_key`**: Implicit unique index on `handle` (for profile lookups).
*   **`idx_users_status_created_at`** (Compound B-Tree):
    *   *Definition*: `ON users(status, created_at)`
    *   *Why it exists*: Accelerates admin lookups and matchmaking queries that filter active users sorted by creation date.
    *   *Query Pattern*: Used when listing profiles or active players.

#### Table: `oauth_accounts`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`oauth_accounts_provider_account_unique`**: Implicit unique index on `(provider, provider_account_id)` (for social sign-in lookup).
*   **`oauth_accounts_user_provider_unique`**: Implicit unique index on `(user_id, provider)` (prevents linking multiple social accounts of the same provider to one user).
*   **`idx_oauth_accounts_user_id`** (B-Tree):
    *   *Definition*: `ON oauth_accounts(user_id)`
    *   *Why it exists*: Speeds up user deletion cascading and profile queries checking linked providers.
    *   *Query Pattern*: Joining users with OAuth accounts.

#### Table: `auth_sessions`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`auth_sessions_refresh_token_hash_key`**: Implicit unique index on `refresh_token_hash` B-Tree.
*   **`idx_auth_sessions_user_id`** (B-Tree):
    *   *Definition*: `ON auth_sessions(user_id)`
    *   *Why it exists*: Quick lookup of all active sessions for a specific user to support single-session validation or multi-device logouts.
    *   *Query Pattern*: User session queries in [FindActiveSessionByHash](file:///home/tanishq/dsablitz/backend/internal/auth/repository.go#L146).
*   **`idx_auth_sessions_refresh_token_hash_active`** (Partial B-Tree):
    *   *Definition*: `ON auth_sessions(refresh_token_hash) WHERE revoked_at IS NULL`
    *   *Why it exists*: Speeds up active refresh token lookups by excluding revoked sessions.
    *   *Query Pattern*: Token rotation and session verification in [FindActiveSessionByHash](file:///home/tanishq/dsablitz/backend/internal/auth/repository.go#L146).
*   **`idx_auth_sessions_expires_at`** (B-Tree):
    *   *Definition*: `ON auth_sessions(expires_at)`
    *   *Why it exists*: Accelerates periodic cleanup tasks that remove expired sessions.
    *   *Query Pattern*: Session cleaning operations.

> ### 💬 Interview Discussion: Composite Indexes & Index-Only Scans
> - **Interviewer Intent**: Evaluate understanding of composite B-Tree structures and prefix matching rules.
> - **Strong Answer**: Order composite index columns from most selective to least selective. When querying, ensure filters match the index column prefix (left-to-right). For example, `(battle_id, user_id, submitted_at)` accelerates queries filtering by both `battle_id` and `user_id`, but cannot be used for queries filtering on `submitted_at` alone.
> - **Common Mistakes**: Creating multiple single-column indexes on every field instead of a single compound index, forcing the database engine to perform expensive index merges.
> - **Follow-up Questions**: What is an index-only scan, and how does it improve query performance? (Answer: An index-only scan retrieves all requested columns directly from the index tree, avoiding the need to read data blocks from the main table).
> - **How DSAblitz demonstrates this**: Composite indexing on submissions is defined in [000001_create_core_schema.up.sql:L271-L274](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L271-L274).

---

### 3.2 Users Module Stats & Social

#### Table: `friendships`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`idx_friendships_unique_pair`** (Expression/Unique B-Tree):
    *   *Definition*: `ON friendships (LEAST(requester_id, addressee_id), GREATEST(requester_id, addressee_id))`
    *   *Why it exists*: Enforces relationship uniqueness between two users, preventing duplicate records regardless of who sent the request.
    *   *Query Pattern*: Relationship creation verification.
*   **`idx_friendships_requester_status`** (Compound B-Tree):
    *   *Definition*: `ON friendships(requester_id, status)`
    *   *Why it exists*: Speeds up queries fetching a user's outgoing pending or blocked friendships.
    *   *Query Pattern*: Friends list pagination queries.
*   **`idx_friendships_addressee_status`** (Compound B-Tree):
    *   *Definition*: `ON friendships(addressee_id, status)`
    *   *Why it exists*: Speeds up queries fetching a user's incoming pending requests.
    *   *Query Pattern*: Incoming friend request notifications.

#### Table: `user_stats`
*   **Primary Key**: Implicit index on `user_id` B-Tree.
*   **`idx_user_stats_rating`** (Compound Sort B-Tree):
    *   *Definition*: `ON user_stats(rating DESC, user_id)`
    *   *Why it exists*: Enables fast sorting of leaderboards by rating.
    *   *Query Pattern*: Leaderboard queries.

#### Table: `rating_history`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`idx_rating_history_user_created_at`** (Compound Sort B-Tree):
    *   *Definition*: `ON rating_history(user_id, created_at DESC)`
    *   *Why it exists*: Quickly retrieves a user's rating changes in reverse chronological order for graphing.
    *   *Query Pattern*: User rating history charts.
*   **`idx_rating_history_battle_id`** (Partial B-Tree):
    *   *Definition*: `ON rating_history(battle_id) WHERE battle_id IS NOT NULL`
    *   *Why it exists*: Speeds up lookup of rating changes associated with a specific match.
    *   *Query Pattern*: Post-match rating adjustments.

---

### 3.3 Rooms Module

#### Table: `rooms`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`rooms_code_key`**: Implicit unique index on `code` B-Tree (for lobby entries).
*   **`idx_rooms_host_status`** (Compound B-Tree):
    *   *Definition*: `ON rooms(host_user_id, status)`
    *   *Why it exists*: Checks if a host user is already running an active matchmaking session.
    *   *Query Pattern*: Preventing multiple hostings.
*   **`idx_rooms_status_created_at`** (Compound B-Tree):
    *   *Definition*: `ON rooms(status, created_at)`
    *   *Why it exists*: Speeds up lobby searches and cleans up idle rooms.
    *   *Query Pattern*: Matching/cleanup queries.
*   **`idx_rooms_expires_at`** (Partial B-Tree):
    *   *Definition*: `ON rooms(expires_at) WHERE expires_at IS NOT NULL`
    *   *Why it exists*: Optimizes the periodic cleanup of expired rooms.
    *   *Query Pattern*: Used in [ExpireRooms](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L427).

#### Table: `room_players`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`idx_room_players_room_id`** (B-Tree):
    *   *Definition*: `ON room_players(room_id)`
    *   *Why it exists*: Retrieves player presences when loading room details.
    *   *Query Pattern*: Loading participant lists in [GetActivePlayersForUpdate](file:///home/tanishq/dsablitz/backend/internal/rooms/repository.go#L149).
*   **`idx_room_players_user_id`** (B-Tree):
    *   *Definition*: `ON room_players(user_id)`
    *   *Why it exists*: Checks if a user is currently in a lobby.
    *   *Query Pattern*: Room presence checks.
*   **`idx_room_players_status`** (B-Tree):
    *   *Definition*: `ON room_players(status)`
    *   *Why it exists*: Filters players by status (e.g. joined or ready) within a lobby.
    *   *Query Pattern*: Presence evaluation queries.
*   **`idx_room_players_room_user_active`** (Partial Unique B-Tree):
    *   *Definition*: `ON room_players(room_id, user_id) WHERE status IN ('joined', 'ready')`
    *   *Why it exists*: Prevents a user from occupying multiple slots in the same room. Unlike table-level unique constraints, this allows users to have historical rows with status `'left'` or `'kicked'` in the same room.
    *   *Query Pattern*: Player joins.
*   **`idx_room_players_room_seat_active`** (Partial Unique B-Tree):
    *   *Definition*: `ON room_players(room_id, seat_number) WHERE status IN ('joined', 'ready')`
    *   *Why it exists*: Prevents seat conflicts (e.g., seat 1 or seat 2) in active lobbies.
    *   *Query Pattern*: Matchmaking seat assignment checks.

> ### 💬 Interview Discussion: Partial Unique Indexes for Soft-Deleted States
> - **Interviewer Intent**: Evaluate capability to enforce unique constraints on tables containing historical soft-deleted rows.
> - **Strong Answer**: Traditional unique constraints block insertion attempts if a row containing the target values exists, even if it is soft-deleted. To resolve this, use partial unique indexes (e.g. `idx_room_players_room_user_active ON room_players(room_id, user_id) WHERE status IN ('joined', 'ready')`). This enforces uniqueness on active rows while allowing multiple historical rows with status `left` or `kicked`.
> - **Common Mistakes**: Using standard unique constraints on soft-deleted tables, which prevents users from re-joining rooms.
> - **Follow-up Questions**: How does database size affect B-Tree index searches? (Answer: B-Trees scale logarithmically; searches require very few read operations, but large indexes must fit in memory to avoid slow disk read bottlenecks).
> - **How DSAblitz demonstrates this**: Partial unique indexes are defined in [000004_fix_room_players_constraints.up.sql](file:///home/tanishq/dsablitz/backend/migrations/000004_fix_room_players_constraints.up.sql).

---

### 3.4 Battle Module

#### Table: `battles`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`idx_battles_room_id`** (Partial B-Tree):
    *   *Definition*: `ON battles(room_id) WHERE room_id IS NOT NULL`
    *   *Why it exists*: Resolves the battle associated with a room lobby.
    *   *Query Pattern*: Joining rooms and battles.
*   **`idx_battles_status_created_at`** (Compound B-Tree):
    *   *Definition*: `ON battles(status, created_at)`
    *   *Why it exists*: Accelerates cleanup of stale matches or active battle lookups.
    *   *Query Pattern*: Battle manager queries.
*   **`idx_battles_winner_user_id`** (Partial B-Tree):
    *   *Definition*: `ON battles(winner_user_id) WHERE winner_user_id IS NOT NULL`
    *   *Why it exists*: Speeds up winner history queries.
    *   *Query Pattern*: Winner retrieval.
*   **`idx_battles_active_room_unique`** (Partial Unique B-Tree):
    *   *Definition*: `ON battles(room_id) WHERE status IN ('created', 'countdown', 'active')`
    *   *Why it exists*: Prevents a room from starting multiple concurrent battles.
    *   *Query Pattern*: Room battle launches.

#### Table: `battle_players`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`battle_players_battle_user_unique`**: Implicit unique index on `(battle_id, user_id)` (prevents a user from joining the same battle twice).
*   **`battle_players_battle_seat_unique`**: Implicit unique index on `(battle_id, seat_number)` (prevents seat assignment conflicts in a battle).
*   **`idx_battle_players_battle_id`** (B-Tree):
    *   *Definition*: `ON battle_players(battle_id)`
    *   *Why it exists*: Retrieves match participant scorecards.
    *   *Query Pattern*: Matches details retrievals.
*   **`idx_battle_players_user_id`** (B-Tree):
    *   *Definition*: `ON battle_players(user_id)`
    *   *Why it exists*: Compiles a player's battle history.
    *   *Query Pattern*: Match history lists.
*   **`idx_battle_players_result`** (Partial B-Tree):
    *   *Definition*: `ON battle_players(result) WHERE result IS NOT NULL`
    *   *Why it exists*: Filters matches by outcome (e.g. wins or losses).
    *   *Query Pattern*: Win-loss ratio calculations.

#### Table: `battle_question_sequence`
*   **Primary Key**: Implicit unique index on `(battle_id, sequence_index)` B-Tree.
*   *Why it exists*: Guarantees sequence order uniqueness and speeds up question retrievals by index.
*   *Query Pattern*: Used in [GetQuestionIDAtSequenceIndex](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L225).

#### Table: `submissions`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`idx_submissions_battle_user_submitted_at`** (Compound B-Tree):
    *   *Definition*: `ON submissions(battle_id, user_id, submitted_at)`
    *   *Why it exists*: Optimizes progression queries that retrieve a user's submissions in a match sorted by time.
    *   *Query Pattern*: Used in [GetLastSubmission](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L207).
*   **`idx_submissions_battle_question`** (Compound B-Tree):
    *   *Definition*: `ON submissions(battle_id, question_id)`
    *   *Why it exists*: Validates prior submissions for a specific question in a match to prevent double scoring.
    *   *Query Pattern*: Used in [GetSubmissionsForQuestion](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L180).
*   **`idx_submissions_user_submitted_at`** (Compound B-Tree):
    *   *Definition*: `ON submissions(user_id, submitted_at)`
    *   *Why it exists*: Compiles a user's submission history over time.
    *   *Query Pattern*: User performance stats.
*   **`idx_submissions_question_id`** (B-Tree):
    *   *Definition*: `ON submissions(question_id)`
    *   *Why it exists*: Aggregates response statistics for a specific question.
    *   *Query Pattern*: Question analytics.

---

### 3.5 Questions Module

#### Table: `questions`
*   **Primary Key**: Implicit index on `id` B-Tree.
*   **`idx_questions_active_difficulty_type`** (Compound B-Tree):
    *   *Definition*: `ON questions(is_active, difficulty, question_type)`
    *   *Why it exists*: Speeds up question filtering by difficulty and type during session initialization.
    *   *Query Pattern*: Used in [FindActiveQuestionsByFilters](file:///home/tanishq/dsablitz/backend/internal/questions/repository.go#L35).
*   **`idx_questions_created_by`** (Partial B-Tree):
    *   *Definition*: `ON questions(created_by) WHERE created_by IS NOT NULL`
    *   *Why it exists*: Lists custom questions created by users.
    *   *Query Pattern*: Custom question list views.
*   **`idx_questions_tags_gin`** (GIN Array Index):
    *   *Definition*: `ON questions USING GIN(tags)`
    *   *Why it exists*: Optimizes tag array searches, supporting fast tag intersection queries (`tags && $1`).
    *   *Query Pattern*: Filtering questions by topics.

#### Table: `question_stats`
*   **Primary Key**: Implicit index on `question_id` B-Tree.
*   **`idx_question_stats_times_answered`** (Compound Sort B-Tree):
    *   *Definition*: `ON question_stats(times_answered DESC, question_id)`
    *   *Why it exists*: Ranks questions by popularity or usage.
    *   *Query Pattern*: Question popularity reports.

> ### 💬 Interview Discussion: GIN Array Indexes
> - **Interviewer Intent**: Assess capacity to design database indexes for non-scalar columns (such as arrays or JSON payloads).
> - **Strong Answer**: Standard B-Tree indexes evaluate entire values, making them useless for searching elements inside array columns. PostgreSQL GIN (Generalized Inverted Index) indexes split arrays into individual elements, mapping each element back to its parent row. This accelerates tag intersection queries (e.g. `tags && ARRAY['graphs']`) to O(log N) instead of triggering a sequential scan.
> - **Common Mistakes**: Relying on standard B-Tree indexes for array columns, which forces the database engine to run full-table scans.
> - **Follow-up Questions**: What is the write overhead of GIN indexes compared to B-Trees? (Answer: GIN indexes are slower to update because inserting a single row requires updating multiple entries in the index tree).
> - **How DSAblitz demonstrates this**: The tags GIN index is defined in [000001_create_core_schema.up.sql:L269](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L269).

---

## 4. Production Considerations

- **What changes in production?**
  In production, table sizes can cause indexes to grow larger than the available RAM, forcing PostgreSQL to read from disk and slowing down queries. We must monitor index cache hit ratios to ensure high-traffic indexes remain cached in memory.
- **What monitoring is required?**
  - Track index cache hit ratios (using `pg_statio_user_indexes` views).
  - Identify unused indexes (which consume memory and slow down writes) using `pg_stat_user_indexes`.
  - Monitor index bloat on high-frequency tables like `submissions`.
- **What will fail first?**
  As the `submissions` table grows, standard index updates will slow down writes. Index fragmentation on UUID columns will also degrade search performance.
- **How would we evolve this design?**
  Partition the `submissions` table by range on `submitted_at`, and rebuild or reindex fragmented index partitions concurrently during low-traffic windows.

---

## 5. Planned Work (V2)

- **Covered index for Leaderboard rating**: Implement a covered index `idx_user_stats_ranking_covered` on `user_stats(rating DESC, user_id) INCLUDE (display_name, battles_won)` to support leaderboard queries using index-only scans, avoiding main table lookups.

---

## 6. Exact Code References

- **Tag Array GIN Index**: Defined in [000001_create_core_schema.up.sql:L269](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L269).
- **Expression LEAST/GREATEST Index**: Configured in [000001_create_core_schema.up.sql:L241](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L241).
- **Lobby Seat Constraints Index**: Configured in [000004_fix_room_players_constraints.up.sql:L6-L12](file:///home/tanishq/dsablitz/backend/migrations/000004_fix_room_players_constraints.up.sql#L6-L12).
- **Submissions Composite Index**: Defined in [000001_create_core_schema.up.sql:L271-L274](file:///home/tanishq/dsablitz/backend/migrations/000001_create_core_schema.up.sql#L271-L274).

---

## Key Takeaways

1. **Foreign key columns** must be explicitly indexed to prevent table-level locks on parent updates.
2. **Partial indexes** minimize memory footprints by excluding inactive rows.
3. **Compound index column ordering** must match query filter and sort clauses.

---

## Interview Questions

- **Why is the composite index `idx_submissions_battle_user_submitted_at` structured as `(battle_id, user_id, submitted_at)`? Why not place `submitted_at` first?**
  * *Answer*: Ordering column prefix matches are essential. Queries search for submissions by filtering on a specific `battle_id` and `user_id`, then sorting by `submitted_at`. Placing `submitted_at` first would prevent the index from being used for queries that filter on `battle_id` and `user_id` without specifying a timestamp, forcing a full table scan.

---

## Common Mistakes

- **Creating redundant duplicate indexes**: Adding single-column indexes on `(battle_id)` and `(user_id)` in addition to the composite index `(battle_id, user_id, submitted_at)`. PostgreSQL can use the composite index prefix to resolve queries filtering on `battle_id` alone, making the single-column `battle_id` index redundant.

---

## Related Documents

- **Database Schema Reference**: [schema.md](file:///home/tanishq/dsablitz/docs/database/schema.md)
- **Database Transactions**: [transactions.md](file:///home/tanishq/dsablitz/docs/database/transactions.md)

---

## Lessons Learned

- **Refactoring constraints into partial unique indexes**: Initially, we used standard database-wide constraints on `room_players` to prevent users from taking duplicate seats. This blocked players from re-joining lobbies they had left. In migration 4, we replaced the unique constraints with partial unique indexes that only apply to active status codes (`joined` and `ready`), resolving the issue while retaining left/kicked player history.
