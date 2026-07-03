# Master Architecture Interview Guide: Why This Architecture?

This document outlines the core architectural decisions of **DSAblitz**, detailing the reasoning, alternative designs, tradeoffs, and interview-ready responses for each technical choice.

---

## 1. Why Modular Monolith? (Why not Microservices?)

### **Short Answer**
A modular monolith offers simple deployment, zero network latency between components, and direct transactional consistency, while enforcing strict logical boundaries that keep the codebase ready for future microservices extraction.

### **Deep Explanation**
Microservices introduce overhead: network latency, serialisation costs, service discovery, distribution of transaction boundaries (requiring Saga or 2PC patterns), and deployment complexity. For an early-stage 1v1 competitive coding platform, the speed of feature delivery and database consistency is paramount. A modular monolith allows us to use standard Go package boundaries and clean interfaces to decouple modules (like `rooms`, `battle`, `auth`) while keeping them in a single repository sharing a single database.

### **Tradeoffs**
* **Pros**: Single deployment artifact, direct in-memory method invocation, compile-time type safety across modules, single database connection pool.
* **Cons**: All modules must share the same Go version and deploy together. A crash in one module (e.g. out of memory in Questions) takes down the entire server.

### **Why it fits DSAblitz**
At MVP stage, the team is small and the user base is growing. Operating Kubernetes clusters or multiple CI/CD pipelines for microservices would consume unnecessary engineering resources.

### **How to Answer in an Interview**
> *"We chose a modular monolith to maximize velocity and runtime reliability while avoiding microservice tax early on. By enforcing strict package boundaries in Go and passing interfaces instead of concrete types, we ensure our modules compile independently. If scale demands it later, we can extract modules like `battle` or `questions` into their own microservices with minimal code changes."*

---

## 2. Why PostgreSQL? (Why not NoSQL?)

### **Short Answer**
PostgreSQL provides ACID transactions, robust row-level locking (`SELECT FOR UPDATE`), and relational integrity required for managing matches, billing, Elo ratings, and lobby states.

### **Deep Explanation**
NoSQL databases (like DynamoDB or MongoDB) are designed for horizontal scale but lack native multi-row ACID guarantees and pessimistic locking at scale. DSAblitz gameplay depends on transactional integrity (e.g., ensuring scoring counters increment accurately, preventing duplicate join seat allocations, and updating ratings). PostgreSQL allows us to lock parent and child rows explicitly inside single SQL transactions.

### **Tradeoffs**
* **Pros**: Rigid schema enforcement, relational checks, row locking, GIN indexes for tag searches.
* **Cons**: Harder to scale horizontally compared to key-value stores.

### **How to Answer in an Interview**
> *"We selected PostgreSQL because the core of a competitive platform is data consistency. Operations like checking lobby capacity, assigning seats, evaluating answers, and updating Elo ratings must be atomic. PostgreSQL's row-level pessimistic locking (`FOR UPDATE`) allows us to handle high-concurrency race conditions gracefully without risking dirty writes or duplicate entries."*

---

## 3. Why Redis?

### **Short Answer**
Redis is used strictly as an ephemeral, high-throughput caching and real-time state coordinator (e.g., WebSocket session tracking and matchmaking queues) to offload transient read-heavy loads from PostgreSQL.

### **Deep Explanation**
Lobby search, presence monitoring, and matchmaking happen at a very high frequency. Querying PostgreSQL repeatedly for "who is online" or "which rooms are open" would exhaust database connection pools. Redis, being an in-memory key-value store, handles hundreds of thousands of operations per second with sub-millisecond latency. However, Redis is not the source of truth for match history, user credentials, or ratings; if Redis restarts, the platform can rebuild transient state.

### **Tradeoffs**
* **Pros**: Extreme read/write throughput, built-in publish/subscribe, TTL-based key expiry.
* **Cons**: RAM is expensive, and data is ephemeral (loss of persistency during crashes must be handled by fallback mechanisms).

### **How to Answer in an Interview**
> *"We reserve Redis strictly for ephemeral, high-frequency, real-time concerns. This includes matching queues and WebSocket state tracking. All durable data, such as completed matches, submissions, and ratings, is stored in PostgreSQL. If Redis crashes, no user data is lost, and the transient session state can be rebuilt."*

---

## 4. Why UUIDs instead of Serial IDs?

### **Short Answer**
UUIDs prevent resource enumeration attacks, simplify client routing, and allow IDs to be safely generated in memory without roundtrips to the database.

### **Deep Explanation**
Auto-incrementing integer IDs (1, 2, 3...) expose system volumes (e.g. `/api/v1/battles/482` makes it obvious there have been 482 battles, and allows attackers to iterate IDs). UUIDs (v4) are cryptographically random 128-bit values.

### **Tradeoffs**
* **Pros**: Global uniqueness, non-enumerable, can be generated by Go code before inserting.
* **Cons**: Occupy 16 bytes (compared to 4 or 8 bytes for integers), which makes database indexes larger and slightly slower.

### **How to Answer in an Interview**
> *"We use UUID v4 for all primary keys to prevent resource enumeration and scraping attacks. Furthermore, generating UUIDs directly in the Go application layer allows us to initialize full object graphs—like a Battle alongside its BattlePlayers—before executing a single database insert statement."*

---

## 5. Why Battle-First Architecture (Stateless Questions Module)?

### **Short Answer**
The Questions module acts as a read-only catalog, while the Battle module owns the stateful progression indexes and answer submissions. This prevents gameplay queries from locking or overloading the static question catalog.

### **Deep Explanation**
Separating the static metadata from high-frequency player writes ensures that the Questions module can be cached in-memory at startup (`sync.Map`). When a user submits an answer, the write is sent to `battle_players` and `submissions`. The Questions module only provides stateless verification logic (`ValidateAnswer`).

### **How to Answer in an Interview**
> *"The Questions module is entirely stateless and cached in memory. Gameplay state—like current question indexes, scoring counters, and submission history—belongs strictly to the Battle module. This prevents high-frequency write contention on the questions table and allows the question pool to scale independently."*

---

## 6. Why Room Orchestrates instead of Battle?

### **Short Answer**
Lobbies (Rooms) have different lifecycles than active matches (Battles). A lobby can exist before a match starts, survive multiple matches (rematches), and handles physical user presence, whereas a Battle only represents a single running match.

### **Deep Explanation**
By separating Rooms (presence) and Battles (gameplay), we support rematching inside the same room code. If the Battle module orchestrated the lobby, starting a second match would require creating a new room code or rewriting the user presence records.

---

## 7. Why Dependency Inversion?

### **Short Answer**
It prevents compile-time circular dependencies in Go, simplifies testing via mock adapters, and prepares modules to be split into separate microservices if needed.

### **Deep Explanation**
Instead of the Rooms module importing the concrete `battle` package, it defines a `BattleCoordinator` interface. The `battle` package implements it, and the dependency is injected at server startup.

---

## 8. Why Deterministic Question Sequences?

### **Short Answer**
It ensures complete fairness in 1v1 matches (both players get the exact same questions in the same order) while allowing players to progress at their own speed asynchronously without central queue lockouts.

---

## 9. Why not CQRS or Event Sourcing?

### **Short Answer**
They add massive architectural complexity (read/write model synchronization, event replay logic, eventual consistency lags) that is completely unjustified for a 1v1 MVP.

### **Deep Explanation**
Event sourcing is powerful for complex auditing systems but makes simple relational operations (like checking room capacity or verifying logins) extremely complex. The MVP uses a simple relational schema with pessimistic locking to guarantee immediate consistency and rapid feature delivery.
