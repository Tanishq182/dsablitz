# DSAblitz Interview Preparation: System Design Q&A

This document compiles a comprehensive system design Q&A bank for **DSAblitz**, structured to resemble MAANG-level system design and technical architectural interviews.

---

## SECTION 1 — Architecture Basics

### Q: What is DSAblitz?
* **Strong Interview Answer**: DSAblitz is a real-time, 1v1 competitive coding platform where developers compete to solve DSA-themed conceptual questions (MCQs, complexity prediction, and code ordering) within a fixed duration (5 or 10 minutes). Unlike traditional coding platforms (like LeetCode), it focuses on rapid-fire theory and logic verification via WebSockets rather than running code in sandboxes.
* **Tradeoffs**:
  * *Pros*: Faster match times, sub-second grading, extremely low server cost (no heavy compilation/Docker runner sandboxes).
  * *Cons*: Cannot test compilation correctness, syntactic style, or full-scale coding execution.
* **Follow-up Questions**: *"How do you verify if the questions remain challenging if there's no code compilation?"*
* **Senior-level Discussion**: By focusing on algorithm conceptual blocks (e.g., ordering Dijkstra relaxation statements, calculating recurrence tree leaf nodes), we test cognitive understanding of code patterns. This represents a different pedagogical angle: checking if a programmer can read and dry-run code mentally, which is a major signal in speed-oriented technical settings.

---

### Q: Why build a modular monolith instead of microservices?
* **Strong Interview Answer**: We chose a modular monolith to maximize velocity and reduce operational complexity for the MVP. It allows us to enforce strict domain boundaries (e.g., separating `auth`, `users`, `rooms`, `battle`, and `questions` into distinct packages with clear interface boundaries) while running on a single application server with one database.
* **Tradeoffs**:
  * *Pros*: Zero network overhead between domains, simplified transactions, single deployment pipeline.
  * *Cons*: Shared resource boundaries (CPU/Memory/DB connections). A bug or memory leak in the Battle module can crash the entire server.
* **Follow-up Questions**: *"If the Battle module experiences high CPU load due to WebSocket handling, how do you protect the Auth module?"*
* **Senior-level Discussion**: To prepare for a future microservice split, we strictly forbid cross-module database imports. Modules communicate exclusively via Go interface contracts. If the Battle module needs to validate a question, it calls `questionsService.ValidateAnswer` through an interface. When scale warrants, we can easily split the Battle module into a separate deployment using gRPC to bridge the interfaces, without modifying the underlying business logic.

---

### Q: Why Go (Golang) for the backend?
* **Strong Interview Answer**: Go is designed for highly concurrent, high-throughput network services. It compiles to a single binary with a minimal memory footprint, has built-in primitives for concurrency (goroutines and channels), and features an efficient garbage collector that minimizes latency spikes.
* **Tradeoffs**:
  * *Pros*: High execution speed, small memory footprint, simple syntax making it easier to read and maintain.
  * *Cons*: Lack of advanced OOP patterns, verbose error handling (`if err != nil`), and less extensive ORM ecosystems compared to Node/Java.
* **Follow-up Questions**: *"How does Go's garbage collector affect real-time WebSocket latency?"*
* **Senior-level Discussion**: Go's GC operates concurrently with application threads, aiming for sub-millisecond pause times. By avoiding heap allocations (using stack allocation when possible, returning values instead of pointers, and pooling objects with `sync.Pool`), we minimize garbage collection sweeps. This is critical for real-time multiplayer applications where GC pauses would cause noticeable packet drops.

---

### Q: Why Gin as the HTTP framework?
* **Strong Interview Answer**: Gin is a lightweight, high-performance HTTP web framework. It uses a custom Radix-tree-based router (via `httprouter`), which performs zero memory allocation during routing, making it one of the fastest Go routers.
* **Tradeoffs**:
  * *Pros*: Very fast routing, simple middleware integration, built-in JSON rendering.
  * *Cons*: Lacks built-in dependency injection or structural patterns; requires the developer to structure the architecture manually.
* **Follow-up Questions**: *"How does Gin handle concurrent requests, and is it thread-safe?"*
* **Senior-level Discussion**: Gin wraps Go's standard `net/http` server, which spawns a separate goroutine for every incoming TCP connection. Gin's context object (`*gin.Context`) is not thread-safe and must not be shared across goroutines without a deep copy. We manage concurrency by copying parameters out of the Gin context before handing them off to background goroutines.

---

### Q: Why PostgreSQL as the primary database?
* **Strong Interview Answer**: PostgreSQL is a robust, ACID-compliant relational database. It offers advanced indexing (GIN, GiST, B-tree), supports JSONB columns for semi-structured data (like question options), and provides strong transactional guarantees necessary for Elo ratings, score updates, and match history.
* **Tradeoffs**:
  * *Pros*: Exceptional reliability, rich query capability, strong constraints, native JSONB support.
  * *Cons*: Harder to scale horizontally compared to NoSQL databases; connections are process-based, requiring a pool manager.
* **Follow-up Questions**: *"Why not use DynamoDB or MongoDB for battle history and questions?"*
* **Senior-level Discussion**: Relational constraints are critical for our domain model. For example, updating a player's Elo rating, recording their submission, and incrementing their win-loss record must occur within a single atomic database transaction. If we used a NoSQL database, we would have to implement complex, slower application-level transactional wrappers.

---

### Q: Why Redis?
* **Strong Interview Answer**: Redis is an in-memory, single-threaded data structure store. In DSAblitz, Redis is strictly reserved for ephemeral, high-frequency, real-time operations: active matchmaking queues, WebSocket connection presence indicators, live match countdown timers, and caching static question sets.
* **Tradeoffs**:
  * *Pros*: Sub-millisecond response times, low CPU usage, atomic data operations.
  * *Cons*: Data is lost on crash unless persistent logging (AOF/RDB) is configured, which adds disk write overhead.
* **Follow-up Questions**: *"If Redis crashes, what happens to ongoing battles?"*
* **Senior-level Discussion**: Because PostgreSQL is the source of truth, a Redis crash does not corrupt persistent player data. Ongoing battles continue because their state is persisted in Postgres; only ephemeral indicators (like lobby presence or matchmaking queues) would need to be rebuilt. We use Redis as a transient accelerator, never as the ultimate system registry.

---

### Q: Why hybrid authentication (JWT + HTTP-Only Cookies)?
* **Strong Interview Answer**: We use a hybrid auth model: JWT access tokens for stateless API authorization, and HTTP-only, secure, SameSite cookies to store refresh tokens. Access tokens are kept short-lived (e.g., 15 minutes) and are stored in client memory. Refresh tokens are kept long-lived (e.g., 7 days) and are protected against Cross-Site Scripting (XSS) by the browser's cookie policies.
* **Tradeoffs**:
  * *Pros*: Secure against XSS token theft, low server-side check overhead for access tokens, seamless session renewals.
  * *Cons*: Requires maintaining a refresh token revocation list (blacklist) on the backend for force logout, adding minor state overhead.
* **Follow-up Questions**: *"How do you prevent Cross-Site Request Forgery (CSRF) if using cookies?"*
* **Senior-level Discussion**: Access tokens are sent as standard Authorization headers, which browser requests do not append automatically, making them immune to CSRF. Refresh tokens are sent via cookies, but they are protected by setting `SameSite=Strict` and `Secure` attributes, ensuring the browser never attaches the cookie to third-party or cross-origin requests.

---

## SECTION 2 — Database Design

### Q: Why choose a normalized schema?
* **Strong Interview Answer**: A normalized schema prevents data redundancy and ensures database write consistency. For example, by separating `questions`, `question_stats`, `users`, `user_stats`, `battles`, and `submissions` into normalized tables, we ensure that an update to a question's tags does not require locking or updating battle history rows.
* **Tradeoffs**:
  * *Pros*: Minimal storage footprint, guaranteed write consistency, clean table indexing.
  * *Cons*: Complex queries requiring multiple SQL `JOIN` statements, which can degrade read performance under heavy load if not properly indexed.
* **Follow-up Questions**: *"How do you prevent join overhead in the active match path?"*
* **Senior-level Discussion**: The active gameplay path does not join tables. The Battle module queries only the sequence and progress tables using index lookups. We only run heavy joins (e.g., joining users, battles, and submissions) during post-match screens or analytical calculations, which are not time-critical.

---

### Q: Why create a separate `battle_question_sequence` table?
* **Strong Interview Answer**: The `battle_question_sequence` table (Option B) normalizes the relationship between battles and questions. It defines a composite primary key `(battle_id, sequence_index)` containing foreign keys to both tables. This keeps our schema clean, allows database-enforced referential integrity, and makes it easy to run analytical queries.
* **Tradeoffs**:
  * *Pros*: Clean relational design, simple index lookups, prevents deleting active questions, easy query joining.
  * *Cons*: Requires inserting 200 rows per battle.
* **Follow-up Questions**: *"What is the index strategy for the sequence table?"*
* **Senior-level Discussion**: In PostgreSQL, a `PRIMARY KEY (battle_id, sequence_index)` automatically creates a unique index on those columns. This makes queries like `SELECT question_id FROM battle_question_sequence WHERE battle_id = $1 AND sequence_index = $2` run in $O(\log N)$ time, completely avoiding sequential scans.

---

### Q: Why use migrations instead of ORM auto-schema synchronization?
* **Strong Interview Answer**: Go frameworks like GORM support auto-migration, but this creates unpredictability in production schemas. Explicit migrations (e.g., using `golang-migrate`) allow us to write optimized SQL, define precise indices, set custom check constraints, and easily roll back changes in deployment pipelines.
* **Tradeoffs**:
  * *Pros*: Deterministic schema evolution, database-agnostic control, safe migrations.
  * *Cons*: Requires manual SQL management, adding minor overhead to dev workflow.
* **Follow-up Questions**: *"How do you handle migrations in a zero-downtime deployment?"*
* **Senior-level Discussion**: We write migrations to be backward-compatible (expanding schemas only). For example, if we need to modify a column, we add the new column first, run the application in dual-write mode, run a backfill migration, and finally deprecate the old column in a subsequent release.

---

### Q: Why UUIDs for primary keys?
* **Strong Interview Answer**: UUIDs (specifically UUIDv4) provide globally unique identifiers. This allows us to generate IDs in-app before saving to the database, preventing ID enumeration attacks (where malicious users crawl resources by incrementing sequential integer IDs) and facilitating database sharding.
* **Tradeoffs**:
  * *Pros*: Hard to guess, safe for distributed databases, prevents serial ID scraping.
  * *Cons*: Takes 16 bytes of space (vs 4 bytes for integers), which makes indexes larger and can lead to index fragmentation under random insertions.
* **Follow-up Questions**: *"How do you prevent index fragmentation with random UUIDs?"*
* **Senior-level Discussion**: While UUIDv4 is random and can fragment indexes under extreme loads, PostgreSQL handles index page splits efficiently. For future scaling, we can adopt UUIDv7, which embeds a timestamp prefix. This makes the UUID sequential (lexicographically sortable), maintaining B-tree index insertion efficiency while keeping the uniqueness and security of standard UUIDs.

---

### Q: Why separate static `questions` metadata from dynamic `question_stats`?
* **Strong Interview Answer**: The `questions` table is read-heavy (users retrieve prompt data), while `question_stats` is write-heavy (updated on every submission/battle to track correct attempts). By isolating them, we prevent write lock contention on the `questions` table, allowing PostgreSQL to cache the static question rows in memory efficiently.
* **Tradeoffs**:
  * *Pros*: Reduces table locking, improves read cache hit rates on metadata.
  * *Cons*: Requires a separate join to show question accuracy stats.
* **Follow-up Questions**: *"How do you update stats without blocking other users?"*
* **Senior-level Discussion**: We run updates to `question_stats` asynchronously or in batches. When a player finishes a match, we can push stats updates to a background worker queue rather than updating the database inside the live match transaction.

---

## SECTION 3 — Scalability

### Q: How would you scale the system to 100k concurrent active battles?
* **Strong Interview Answer**: Scaling to 100k concurrent battles requires scaling the connection, application, and database layers:
  1. **WebSocket Gateway Layer**: Use a fleet of stateless Go instances coordinated via a Redis Pub/Sub backplane.
  2. **Database Read Offloading**: Cache the pre-generated sequences and question metadata.
  3. **Pessimistic Lock Isolation**: Run connection pools efficiently, keeping transactions short.
* **Tradeoffs**:
  * *Pros*: Cost-effective, robust, horizontally scalable.
  * *Cons*: Increased infrastructure complexity (requires load balancers, container orchestration, and Redis clustering).
* **Follow-up Questions**: *"How would you handle horizontal sharding of the database?"*
* **Senior-level Discussion**: Since battles are completely independent, we can partition the database using `battle_id` as the shard key. Active battles can be routed to specific shards. A shard coordinator router intercepts connection queries and directs them to the correct shard database, keeping cross-shard operations to zero.

---

### Q: How do we scale WebSocket connections horizontally?
* **Strong Interview Answer**: WebSocket connections are stateful and bound to a single server. To scale horizontally, we place a load balancer (like NGINX or AWS ALB) using round-robin routing in front of our Go servers. To allow servers to communicate (e.g., if Player A is connected to Server 1 and Player B to Server 2), we connect the Go instances via a **Redis Pub/Sub** message broker.
* **Tradeoffs**:
  * *Pros*: Horizontal scaling, clean separation of connection state, low message broker overhead.
  * *Cons*: Introduces Redis Pub/Sub as a single point of failure; requires handling connection drop/reconnection logic.
* **Follow-up Questions**: *"How do you handle backpressure on WebSockets?"*
* **Senior-level Discussion**: Go channels act as internal buffers for WebSocket writes. If a client's connection degrades, the server channel will fill up. We enforce write timeouts. If a client's channel fills past a threshold, we terminate the connection to prevent memory bloat on the server.

---

## SECTION 4 — Security

### Q: How do you prevent answer scraping and reverse-engineering?
* **Strong Interview Answer**: We implement **Payload Sanitization**. The public APIs and WebSocket payloads only serve `SanitizedQuestionResponse` containing prompts and options. The `correct_answer` and `explanation` columns remain strictly inside PostgreSQL. Validation is performed server-side inside the Battle module using database records.
* **Tradeoffs**:
  * *Pros*: 100% cheat-proof network payload.
  * *Cons*: Slightly increases database lookups since the server must fetch the answer key on every submission.
* **Follow-up Questions**: *"What if the user writes a script to scrape questions by playing many matches?"*
* **Senior-level Discussion**: We monitor and limit access via rate limiters on matchmaking. If a user starts and abandons battles repeatedly to scrape questions, our system flags their account. In V2, we can also dynamically generate math expressions or variable names in prompts to ensure questions are not easily cataloged.

---

### Q: Why use Argon2 instead of bcrypt for password hashing?
* **Strong Interview Answer**: Argon2 (specifically Argon2id) was selected as the winner of the Password Hashing Competition. Unlike bcrypt (which is only CPU-hard), Argon2 is **memory-hard**. This prevents GPU-based brute-force cracking attacks, where attackers use parallel GPU cores to compute millions of hashes per second.
* **Tradeoffs**:
  * *Pros*: State-of-the-art resistance to hardware cracking.
  * *Cons*: High memory usage during hash calculations, requiring careful tuning of CPU/Memory parameters to prevent Denial of Service (DoS) attacks on the registration/login endpoints.
* **Follow-up Questions**: *"How do you prevent attackers from DOS-ing your server with concurrent login requests?"*
* **Senior-level Discussion**: We protect the login endpoint using rate limiters (Token Bucket algorithm) tracked by IP and account identifier in Redis. This prevents attackers from exhausting system CPU/Memory resources with concurrent Argon2 calculations.

---

## SECTION 5 — Battle Engine

### Q: Why use a shared question stream with asynchronous progression?
* **Strong Interview Answer**: It ensures complete competitive fairness. If both players received different questions, one could get significantly easier prompts, introducing luck into the match. By sharing the same stream but allowing players to progress asynchronously, we reward speed and accuracy. A faster player can solve 10 questions while a slower player solves 5, but both face the identical sequence.
* **Tradeoffs**:
  * *Pros*: 100% fair matchmaking, rewards speed.
  * *Cons*: Requires generating a large sequence buffer (200 questions) and tracking progress independently.
* **Follow-up Questions**: *"How do you handle latency differences between players?"*
* **Senior-level Discussion**: We track response time on the backend. When a question is served, the server records a `served_at` timestamp. When the player submits, we compute the delta (`submitted_at - served_at`) to ensure network lag does not unfairly penalize a player.

---

### Q: Explain the 2-attempt wrong answer policy (Option C).
* **Strong Interview Answer**: To balance speed with accuracy, we implement a limited-attempts skip policy:
  * A player gets a maximum of 2 attempts per question.
  * A correct answer on the 1st or 2nd attempt awards 1 point and advances the player.
  * An incorrect answer on the 1st attempt keeps the player on the question (1 attempt remaining).
  * An incorrect answer on the 2nd attempt auto-skips the question, awards 0 points, and advances the player.
* **Tradeoffs**:
  * *Pros*: Prevents guessing loops while stopping players from getting stuck indefinitely on a single hard question.
  * *Cons*: Minor state complexity (must track attempts per question).
* **Follow-up Questions**: *"Why not allow unlimited attempts with a time penalty?"*
* **Senior-level Discussion**: Unlimited attempts lead to brute-forcing (especially on 4-option MCQs). Auto-skipping after 2 attempts forces players to read prompts carefully, while ensuring the match tempo remains fast and engaging.

---

## SECTION 6 — Future System Design

### Q: How would you design a Spectator Mode?
* **Strong Interview Answer**: To implement spectator mode, we decouple the match event pipeline from the direct player connection. When a battle progresses, the server publishes state update events to a Redis Pub/Sub channel (e.g., `battle:{id}:events`). Spectators join the room's event feed as read-only listeners, receiving state changes (scores, current question index) in real-time without interfering with the active player channels.
* **Tradeoffs**:
  * *Pros*: Zero performance impact on active players, clean event-driven design.
  * *Cons*: Increases Redis Pub/Sub message volume under high viewership.
* **Follow-up Questions**: *"How do you prevent spectators from sharing answers with players?"*
* **Senior-level Discussion**: We enforce a stream delay (e.g., 5 seconds) on spectator feeds. This is implemented by routing spectator events through a delay queue, ensuring that by the time a spectator sees a question, the player has already answered it or timed out.

---

### Q: How would you design a Global Leaderboard?
* **Strong Interview Answer**: A real-time global leaderboard can be implemented using **Redis Sorted Sets (ZSET)**. The set key is `leaderboard`, the score is the player's Elo rating, and the member is the `user_id`. Redis updates and retrieves ranks in $O(\log N)$ time, allowing us to serve top-100 lists and individual player ranks instantly.
* **Tradeoffs**:
  * *Pros*: Extremely fast, low CPU usage, native ranking algorithms.
  * *Cons*: Leaderboard data is kept in memory. (We mitigate this by backing up ratings in PostgreSQL and rebuild the ZSET if Redis crashes).
* **Follow-up Questions**: *"What if the player base grows to 10 million users?"*
* **Senior-level Discussion**: With 10 million users, a single Redis ZSET remains highly performant, but we can optimize it by partitioning (sharding) the leaderboard by region or division, or updating the global leaderboard asynchronously (e.g., every 5 minutes) using database batch queries rather than real-time Redis writes on every match resolution.
