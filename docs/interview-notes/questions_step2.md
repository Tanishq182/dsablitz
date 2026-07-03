# Interview Preparation: Questions Module Design

These notes prepare you for placements/interviews, highlighting system tradeoffs, design decisions, and lessons learned while implementing the stateless Questions module in **DSAblitz**.

---

## 1. Interview Questions & Cheat Sheet

### **Q: Why is the Questions module stateless while the Battle module is stateful?**
* **A**: The questions catalog represents static, read-only configuration data. If we mixed player progression pointers (active indices, points, and score counts) into the Questions module, it would violate modular boundaries. Keeping the Questions module stateless allows us to cache it globally and query it without concern for game session Lifecycles. The Battle module manages mutable, concurrent game state transactions.

### **Q: Why did you implement a local Go memory cache instead of Redis?**
* **A**: Question metadata is static. Network roundtrips to Redis take ~1-2 milliseconds, whereas Go memory map dereferences execute in ~50 nanoseconds (a 20,000x improvement). By keeping the static questions pool (50-500 questions, taking less than 100 KB of RAM) in local RAM, we reduce gameplay read queries on PostgreSQL to zero.

### **Q: How do you prevent race conditions when two players submit answers concurrently?**
* **A**: We serialize submission processing using row-level pessimistic locking (`SELECT ... FOR UPDATE` on the `battle_players` progression row). This guarantees that concurrent validation requests queue up, preventing double-point exploits or out-of-order writes.

---

## 2. Technical Decisions & Tradeoffs

### **Domain Invariants vs. Business Rules**
* **Decisions**: Structural constraints (UUIDs, prompt non-emptiness, and difficulty ranges) reside inside `Question.Validate()`. Business rules (MCQ option counts, correct answer matching, duplicate checks) are handled in the `seeder.go` ingestion pipeline and validation.go.
* **Tradeoff**: Restricting the entity model logic makes it simpler to write test cases and keeps database scanners clean, while isolating changing product rules.

---

## 3. STAR Stories from this Phase

### **Story: Eliminating Database Reads in Active Gameplay**
* **Situation**: The initial design queried PostgreSQL for the correct answer on every player submission.
* **Task**: Redesign answer validation to scale to thousands of concurrent submissions without exhausting the PostgreSQL connection pool.
* **Action**: Designed a thread-safe global questions cache in Go RAM using a `sync.RWMutex` map loaded on startup, with a lazy read-through query fallback if a key is missed.
* **Result**: Validation queries were reduced to sub-microsecond in-memory lookups, scaling active submission throughput while keeping PostgreSQL connection usage to zero.
