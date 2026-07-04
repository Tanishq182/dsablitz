# DSAblitz Interview Prep: Cross-Framework Comparisons (SDE1 Level)

This document provides comparisons between different technology stacks and design decisions for production-level engineers (SDE1), focusing on HTTP routing, caching, and authentication patterns in **DSAblitz**.

---

## Q&A 1: Gin vs. Chi / Echo (Go HTTP Routers & Middleware Chains)

### Interviewer Intent
The interviewer wants to understand if you know how HTTP routers differ in Go (Radix tree vs. regular expressions), how middleware chains execute, and the design implications of framework-specific contexts (like `*gin.Context`) vs. standard `net/http` context propagation.

### Strong Answer
Go has several popular web routing frameworks. We compared Gin (used in DSAblitz) with Chi and Echo:

```
Gin Router:
Route Pattern: "/api/v1/rooms/:code" 
                 └── Radix Tree Trie Lookup (Fast, O(K) where K is route length, zero allocations)

Chi Router:
Route Pattern: Compliant with net/http. Standard context injection. Slightly more runtime overhead.
```

- **Routing Algorithm**: Gin utilizes a custom Radix-tree based trie router (derived from `httprouter`). It is extremely fast and optimizes memory allocations by avoiding string parsing or regular expressions during path matching. Echo also uses a Radix tree. Chi is slightly slower as it prioritizes middleware flexibility and standard `http.Handler` interfaces over raw path-matching speeds.
- **Context Wrapper**: Gin introduces `*gin.Context`, wrapping request and response writers, parameters, and key-value state. While convenient, a common pitfall is passing `*gin.Context` into database repositories or services. If a request terminates, Gin recycles the context struct, which can cause panics or race conditions in asynchronous routines. In DSAblitz, we convert client inputs inside the controller and pass standard Go `context.Context` down to our repositories and services.
- **Middleware Chain**: Gin implements middleware by maintaining a slice of handler functions (`gin.HandlerFunc`) and executing them sequentially using `ctx.Next()`. Chi and Echo use standard nested middleware decorators, which can lead to deeper call stacks.

DSAblitz chooses Gin because of its high throughput, built-in parameter binding, and lightweight JSON rendering helper functions.

### Common Mistakes
- **Leaking `*gin.Context`**: Passing `*gin.Context` into async goroutines or repository queries. If the HTTP request completes, Gin cleans up and pools the context object for subsequent requests. Any asynchronous read on it will fetch corrupted data.
- **Route Shadowing**: Assuming wildcards (like `/rooms/:code`) do not conflict with static routes (like `/rooms/expire`). Radix tree routers have strict rules about overlapping wildcards and static routes.

### Follow-up Questions
1. *How would you implement custom authentication middleware in Chi vs. Gin?*
2. *How does Gin handle panics in HTTP handlers, and how does it prevent the server from crashing?*

### How DSAblitz demonstrates this concept
DSAblitz uses Gin for routing, request validation, and cookie-based JWT authentication middleware.
- **JWT Middleware**: Implemented using a custom Gin handler in [middleware.go:L11-L28](file:///home/tanishq/dsablitz/backend/internal/auth/middleware.go#L11-L28).
- **Separation of Context**: Handlers parse parameters and only pass standard `context.Context` to the service, e.g., in [handler.go:L1-L100](file:///home/tanishq/dsablitz/backend/internal/auth/handler.go) (where `ctx.Request.Context()` is passed to services).

### Related Documentation
- [Request Lifecycle](file:///home/tanishq/dsablitz/docs/architecture/request_lifecycle.md)
- [Module Interactions](file:///home/tanishq/dsablitz/docs/architecture/module_interactions.md)

---

## Q&A 2: Stateless JWT vs. Stateful Session Authentication

### Interviewer Intent
The interviewer wants to evaluate your understanding of authentication security, DB load, tokens, cookies, and scalability trade-offs in real-time WebSockets and API architectures.

### Strong Answer
For DSAblitz, authentication must secure both REST endpoints and long-lived WebSocket connections. We compared stateless JWT tokens (our choice) against stateful sessions (e.g. stored in Redis or DB):

| Metric / Tradeoff | Stateless JWT (DSAblitz Choice) | Stateful Sessions (Database / Redis) |
| :--- | :--- | :--- |
| **Database/Cache I/O** | Zero (verified locally via HMAC signatures) | High (requires check against store on every request) |
| **Token Size** | Medium/Large (contains encoded headers, claims, signature) | Small (random session ID UUID) |
| **Revocation** | Difficult (requires blocklisting or short lifetimes) | Instant (delete session ID from store) |
| **Scalability** | Horizontal (any node can verify without sharing state) | Vertical/Horizontal (requires distributed session store like Redis) |

DSAblitz utilizes a custom manual JWT implementation in the `auth` module:
1. **Cookie-Based**: The JWT is transmitted in a secure, `HttpOnly` cookie to prevent Cross-Site Scripting (XSS) scraping.
2. **Stateless Signature Validation**: Every gameplay request verifies the token's validity using the server's local `JWT_SECRET`. This prevents database query overhead during rapid-fire gameplay.
3. **Decoupled Identity**: Once verified, the user ID is injected into the context, allowing subsequent modules to remain agnostic of the authentication framework.

### Common Mistakes
- **Storing Sensitive Data in JWT Claims**: Forgetting that JWT claims are Base64Url-encoded and can be read by anyone. DSAblitz only stores the user's UUID in the `sub` claim.
- **Using a weak secret key**: Using a short secret string. DSAblitz requires a `JWT_SECRET` of at least 32 characters to prevent offline brute-force signature forgery.
- **Checking the DB anyway**: Implementing JWTs but querying the database on every request to see if the user exists, defeating the entire performance benefit of stateless JWT validation.

### Follow-up Questions
1. *How would you implement immediate session revocation in a stateless JWT architecture if a user's credentials are compromised?*
2. *Why is `subtleCompare` (constant-time comparison) used when validating signatures in `VerifyAccessToken`?*

### How DSAblitz demonstrates this concept
DSAblitz manually builds, signs, and parses JWT claims without relying on heavy third-party libraries, ensuring high performance.
- **Token Generation & Validation**: Handled in [token.go:L56-L134](file:///home/tanishq/dsablitz/backend/internal/auth/token.go#L56-L134) using HMAC-SHA256 signatures and constant-time string comparisons.
- **Cookie Authentication**: Configured to read the cookie directly in [middleware.go:L13-L27](file:///home/tanishq/dsablitz/backend/internal/auth/middleware.go#L13-L27).

### Related Documentation
- [Authentication Flow](file:///home/tanishq/dsablitz/docs/flows/login_flow.md)
- [Auth API Docs](file:///home/tanishq/dsablitz/docs/api/auth.md)

---

## Q&A 3: In-Memory Caching (sync.RWMutex) vs. Redis for Read-Heavy Catalogs

### Interviewer Intent
The interviewer is looking for a deep understanding of cache hierarchies, concurrency primitives in Go (`sync.RWMutex`), network overhead tradeoffs, and cache coherency strategies.

### Strong Answer
In-memory maps and Redis are both caching mechanisms, but they serve different performance envelopes and scaling limits:

- **Latency**: Local in-memory caches retrieve data in nanoseconds (no network hop). Redis caches require network TCP roundtrips, taking 1-5 milliseconds depending on deployment topology.
- **Concurrency**: A local map requires Go synchronization primitives (`sync.Mutex` or `sync.RWMutex`) to prevent race conditions during concurrent reads and writes. Redis handles concurrency internally via single-threaded execution of commands or Redis-level locking.
- **Consistency**: In-memory caching on multiple nodes leads to split-brain consistency bugs if cache updates are not coordinated (Node A updates local cache, Node B still serves stale memory). Redis provides a centralized, single source of cache truth.

```
In-Memory Cache (sync.RWMutex):
[Request] ---> [Go App RAM Map] (Nanoseconds, zero I/O)

Redis Cache:
[Request] ---> [Go App] --(TCP/IP Connection)--> [Redis Server] (1-5ms)
```

In DSAblitz, we use **In-Memory Caching** for the Questions module. Since the question bank is a static catalog loaded at application startup, we load all active questions into a thread-safe map guarded by a `sync.RWMutex`.
- Reads use `RLock()` (read-lock), allowing unlimited concurrent readers to retrieve questions simultaneously without blocking each other.
- Writes (like initial loading or cache invalidation) acquire a `Lock()` (write-lock), blocking all readers to prevent data race corruption.

This approach provides maximum performance and eliminates PostgreSQL database load during competitive matches.

### Common Mistakes
- **Neglecting to Unlock**: Forgetting to call `defer s.mu.RUnlock()` or `defer s.mu.Unlock()`, which locks the service indefinitely and causes all subsequent requests to freeze.
- **Concurrent Map Write Panic**: Modifying a map concurrently without locking. Go runtimes will instantly panic with `fatal error: concurrent map writes` and crash the server.

### Follow-up Questions
1. *When should you use `sync.RWMutex` instead of a standard `sync.Mutex`?*
2. *What is cache stampede, and how does falling back to PostgreSQL inside `GetQuestionByID` protect against it?*

### How DSAblitz demonstrates this concept
The Questions service implements this local caching pattern.
- **Cache Loading & Thread-Safety**: Handled in [service.go:L31-L57](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L31-L57) using `sync.RWMutex` and map allocations.
- **No-I/O Validation**: Evaluates answer correctness entirely in-memory using cached data in [service.go:L69-L75](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L69-L75).

### Related Documentation
- [Cache Design](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)
- [Questions Flow](file:///home/tanishq/dsablitz/docs/flows/questions_flow.md)

---

## Key Takeaways
- **Trie-Based Routing**: Modern Go web frameworks like Gin use Radix-tree routing for fast lookup, but requiring caution when mixing wildcard parameters and static routes.
- **Stateless Tokens**: Verification of JWTs is entirely mathematical, eliminating DB lookups but making immediate token revocation a challenge.
- **Read-Write Locking**: Go's `sync.RWMutex` is ideal for read-heavy, write-rare local caches, guaranteeing high throughput with strict concurrency safety.

## Interview Questions
1. *What are the architectural tradeoffs of routing contexts like `*gin.Context` vs. standard library context?*
2. *Explain the differences in latency and memory overhead between a local in-memory cache and a remote Redis cache.*
3. *How does cookie-based JWT authentication protect against CSRF attacks compared to local storage?*

## Common Mistakes
1. **Map Concurrency Violation**: Modifying or reading standard Go maps concurrently without a lock, leading to system crashes.
2. **Context Garbage Collection**: Storing requests reference contexts across Go routines.
3. **Session Bloat**: Storing extensive user claims inside the JWT token, bloating HTTP header sizes on every API request.

## Related Documents
- [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
- [Cache Design](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)

## Lessons Learned
- Isolate framework dependencies to the entry point (router handler layer) and use pure standard Go abstractions inside services.
- Never write to memory structures concurrently without explicit locks, even in test code.
