# Deep Dive: Questions Cache Design

This document details the architectural decisions and distributed system tradeoffs regarding question bank caching in **DSAblitz**.

---

## 1. Why the Cache Exists

In a real-time multiplayer coding match, players submit answers rapidly (often multiple times per minute). 
* **The Constraint**: Each submission requires checking the user's answer against the database's `correct_answer` field.
* **The Problem**: Querying PostgreSQL (`SELECT correct_answer FROM questions WHERE id = $1`) on every single user submission causes heavy I/O lock contention, limits concurrency, and quickly exhausts the database connection pool.
* **The Solution**: Since the question bank is static (updates only happen via administrative seeder execution), caching the static details in application memory reduces database reads to **zero** during active matches.

---

## 2. Alternative Designs Considered

### **Option A: Query Database Directly**
* **Pros**: Simple, zero stale cache issues.
* **Cons**: Relational database becomes the bottleneck under multiplayer load.

### **Option B: Distributed Cache (Redis)**
* **Pros**: Easily scales out across multiple instances, consistent data updates.
* **Cons**: Introduces network hops (latency) and network failure risks for static configuration data.

### **Option C: Global In-Memory Cache (Chosen)**
* **Pros**: Sub-microsecond local RAM lookup, no network latency, extremely low footprint (50 questions $\approx$ 50 KB).
* **Cons**: State inconsistency across multiple nodes when updates are made (requires cache invalidation patterns).

---

## 3. Cache Invalidation & Distributed Systems

### **Local Invalidation (Single Instance)**
In a single-node deployment, invalidating the cache upon admin edits is trivial:
```go
func (s *Service) UpdateQuestion(ctx context.Context, q Question) error {
    if err := s.repo.InsertOrUpdateQuestion(ctx, q); err != nil {
        return err
    }
    s.mu.Lock()
    s.cache[q.ID] = q
    s.mu.Unlock()
    return nil
}
```

### **Horizontal Scaling Cluster (Multi-Instance)**
When multiple instances run behind a load balancer:
1. Admin edits a question on **Node A**. Node A writes to PostgreSQL and updates its local memory cache.
2. **Node B** and **Node C** continue serving the stale version of the question from their local memories.
3. **Solution (Redis Pub/Sub)**:
   * Node A publishes an event to Redis: `PUBLISH questions:invalidation <question_id>`.
   * Node B and Node C subscribe to the `questions:invalidation` channel.
   * Upon receiving the event, they load the latest data from PostgreSQL and update their memory maps.

```
                  ┌──────────────┐
                  │  Admin API   │
                  └──────┬───────┘
                         │ (Write to DB & Publish Event)
                         ▼
        ┌──────────────────────────────────┐
        │              Node A              │
        └────────┬─────────────────▲───────┘
                 │ (1) SQL Write   │ (2) Redis PUBLISH
                 ▼                 │
        ┌────────────────┐ ┌───────┴────────┐
        │  PostgreSQL    │ │ Redis Pub/Sub  │◄────────┐
        └───────▲────────┘ └────────┬───────┘         │
                │                   │ (3) Event Broadcast
                │                   ▼                 │
                │          ┌────────────────┐         │
                └──────────┤     Node B     ├─────────┘
                  SQL Read └────────────────┘
```

---

## 4. Interview Cheat Sheet

* **Why not Redis for the static questions cache?**
  * Network I/O is slower than memory I/O. Accessing a Go memory map takes ~50ns, while checking Redis takes ~1ms (a 20,000x difference). For static configurations, local RAM is optimal.
* **What if `LoadCache()` fails on startup?**
  * The service utilizes a **Lazy Read-Through Cache** mechanism. If a cache miss occurs, the system queries PostgreSQL, populates the map, and returns the result, preventing startup glitches from bringing down the system.
