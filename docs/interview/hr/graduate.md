# HR - Graduate / Intern Level

This document contains HR and cultural-fit interview notes for Graduate/Intern level candidates, focusing on career goal alignment, teamwork, and resolving minor peer conflicts.

---

## Q&A Set 1: Career Goal Alignment with DSAblitz

### 1. Interviewer Intent
The interviewer wants to evaluate the candidate's motivation, their interest in real-time gaming architectures, and how well their long-term career goals align with our team's technical stack.

### 2. Strong Answer
I want to join DSAblitz because I am passionate about building high-performance, concurrent backend systems. As a graduate developer, my goal is to transition from writing basic academic code to mastering production-level patterns like concurrency control, transactional databases, and low-latency API design.

DSAblitz's backend tech stack (Go, Gin, PostgreSQL, Redis, and WebSockets) offers the perfect environment for this growth. The modular monolith architecture is clean and accessible, yet handles real-world concurrency challenges (such as pessimistic locking and memory caching). Working here will allow me to contribute to a live gaming platform while learning from experienced engineers.

### 3. Common Mistakes
* Giving generic answers like "I just want a software job" without mentioning DSAblitz's specific domain or technical stack.
* Overemphasizing frontend or unrelated technologies if the role is focused on high-performance backend systems.
* Focusing only on what the company can provide to the candidate, rather than how the candidate's goals align with the team's needs.

### 4. Follow-up Questions
* **Which module of DSAblitz interests you the most and why?**
  * *Answer*: The Battle module, because it handles real-time player progression, monotonic index validations, and pessimistic locking, which are critical for low-latency backend systems.

### 5. How DSAblitz demonstrates this concept
The health check route and standard Gin configuration demonstrate a clean, simple web entry point suitable for graduate onboarding.

### 6. Relevant code references
* Health check handler: [routes.go:L38-L38](file:///home/tanishq/dsablitz/backend/internal/server/routes.go#L38-L38)
* Health check json response: [routes.go:L76-L81](file:///home/tanishq/dsablitz/backend/internal/server/routes.go#L76-L81)

### 7. Related documentation
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
* [Request Lifecycle Reference](file:///home/tanishq/dsablitz/docs/architecture/request_lifecycle.md)

---

## Q&A Set 2: Constructive Peer Collaboration & Conflict Resolution

### 1. Interviewer Intent
The interviewer wants to assess the candidate's interpersonal skills, communication style, emotional maturity, and ability to handle technical disagreements constructively with peers.

### 2. Strong Answer
When working on collaborative coding tasks, technical disagreements are common. In these situations, my approach is to focus on facts and project context rather than personal preferences.

For example, if a peer suggests using raw, hardcoded strings for player states (like `"joined"` or `"ready"`) because it is faster to write, I would explain the risks: hardcoded strings can lead to typo bugs that are difficult to debug in database queries.

Instead of arguing, I would point to our codebase's design guidelines and propose using type-safe domain constants:
```go
type RoomPlayerStatus string
const (
    PlayerJoined RoomPlayerStatus = "joined"
    PlayerReady  RoomPlayerStatus = "ready"
)
```
This keeps the codebase type-safe and prevents runtime bugs. By focusing on code quality and stability, we can reach a consensus quickly.

### 3. Common Mistakes
* Escalating minor technical disagreements to managers immediately without attempting to talk to the peer first.
* Agreeing to poor code patterns just to avoid conflict, which compromises system quality.
* Getting personal or defensive when a peer critiques your implementation.

### 4. Follow-up Questions
* **What would you do if you and your peer still couldn't agree after discussing the options?**
  * *Answer*: I would document both options, outline the tradeoffs, and ask a senior engineer or mentor to guide us to a decision.

### 5. How DSAblitz demonstrates this concept
Type-safe status constants are implemented in `rooms/models.go` to prevent raw string comparison bugs.

### 6. Relevant code references
* Type-safe Room status constants: [models.go:L13-L21](file:///home/tanishq/dsablitz/backend/internal/rooms/models.go#L13-L21)
* Type-safe RoomPlayer status constants: [models.go:L30-L37](file:///home/tanishq/dsablitz/backend/internal/rooms/models.go#L30-L37)

### 7. Related documentation
* [Modular Monolith Design ADR](file:///home/tanishq/dsablitz/docs/adr/0001_modular_monolith_design.md)
* [Package Structure Reference](file:///home/tanishq/dsablitz/docs/architecture/package_structure.md)

---

## Key Takeaways
1. **Career goals** should align with the technical challenges of the team (e.g. concurrency, low-latency, and caching).
2. **Technical conflicts** are best resolved by focusing on facts, code safety, and codebase design guidelines.
3. **Type-safe constants** prevent runtime bugs caused by hardcoded strings in database queries.

---

## Interview Questions
* **How do you stay updated on backend engineering trends and best practices?**
  * *Answer*: I read technical blogs, review open-source projects, and study engineering documentation from high-scale systems.
* **Why are type-safe enums preferred over raw strings in Go databases?**
  * *Answer*: They prevent typos, enforce compile-time checks, and restrict database columns to supported values.

---

## Common Mistakes
* **Lack of preparation**: Failing to research the technical stack and architecture of the team before the interview.
* **Passive agreement**: Accepting unsafe code patterns to avoid discussions, which compromises system quality.

---

## Related Documents
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
* [Database Schema Reference](file:///home/tanishq/dsablitz/docs/database/schema.md)

---

## Lessons Learned
* **Type-safety guidelines**: Defining explicit type-safe enums in the domain models package reduced runtime query bugs and made state transitions self-documenting.
