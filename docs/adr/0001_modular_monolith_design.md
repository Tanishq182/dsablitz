# ADR 0001: Modular Monolith Architecture Design

This document answers the primary question: **Why did we choose a modular monolith instead of a microservices or flat monolithic architecture, and what are its systemic tradeoffs?**

---

## Status
**Approved** (Phase 1 Decision)

## Context
For a real-time multiplayer application like DSAblitz, we require high-frequency low-latency updates (under 100ms) for answer verification and room actions. 
* **Microservices**: Introduce network serialization/deserialization overhead, require message brokers for cross-service events, and introduce distributed transaction complexities (e.g. Saga pattern) across room states and active battles.
* **Flat Monolith**: Risks code entanglement ("spaghetti code"), making it easy to create circular relationships between lobby management and gameplay mechanics, resulting in high maintenance debt.

---

## Decision
We adopted a **Modular Monolith** architecture:
* Every package under `backend/internal/` operates as a logically isolated module.
* Cross-module interaction occurs strictly through Go interfaces and Dependency Injection (DI) adapters.
* All modules share a single transactional connection pool (`pgxpool.Pool`), but queries must only join tables owned by the respective module.

---

## Alternatives Considered & Rejected

### Why not Microservices?
* **Rejected**: The network roundtrips between a separate "Lobby service" and a "Gameplay service" would add unnecessary milliseconds of latency. Additionally, managing database consistencies across separate databases would require complex distributed lock systems (like Redis DLM / Redlock) which increase failure points.

### Why not Flat Monolith?
* **Rejected**: While easy to build initially, flat monolithic structures quickly lead to tight coupling. The Go compiler would not prevent circular imports since developers would put everything in a few flat directories, leading to unmaintainable code.

---

## Architectural Tradeoffs

### Pros
* **High Performance**: In-memory interface calls take nanoseconds, avoiding RPC serialization latencies.
* **Operational Simplicity**: Compiles into a single binary executable, minimizing container and Kubernetes cluster complexity.
* **Strict Decoupling**: Decoupled package boundaries prevent dependency circles at compile time.

### Cons
* **Scaling Limitations**: Scaling requires replicating the entire application binary rather than scaling individual modules.
* **Database Connection Bottleneck**: All modules query the same database pool, requiring strict tuning of connection limits.

### Limitations
* The unified database connection acts as a single point of failure (SPOF) and a throughput bottleneck under high concurrent matches.

### Future Improvements
* Transitioning read-only traffic to Postgres read-replicas.
* Moving high-write submission telemetry logs to a timeseries database or decoupled logging pool.

---

## Key Takeaways
1. The modular monolith combines the deployment simplicity of a monolith with the logical decoupling of microservices.
2. We prioritize raw network performance and simple deployment overheads over individual service scaling features.

## Common Interview Questions
* **How would you migrate this Modular Monolith to Microservices if the need arose?**
  * *Answer*: Because our modules are strictly separated, share no tables, and communicate only via interfaces (e.g., [BattleCoordinator](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go#L18)), we can migrate a package into a separate service by replacing the in-memory interface implementation with a gRPC client wrapper.
* **What database patterns prevent modular monologue schema coupling?**
  * *Answer*: Schema isolation. Even though all modules share the same connection pool, they execute queries that target only their module-owned tables. We strictly prohibit cross-module SQL joins.

## Related Documents
* For structural layering context, see [overall_architecture.md](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md).
* For package relationships, see [dependency_graph.md](file:///home/tanishq/dsablitz/docs/architecture/dependency_graph.md).
