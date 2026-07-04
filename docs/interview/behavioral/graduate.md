# Behavioral - Graduate / Intern Level

This document contains behavioral interview notes and scenarios for Graduate/Intern level candidates, focusing on rapid learning, adaptability, and collaborating on code review feedback.

---

## Scenario 1: Learning a New Technology Under Pressure

### 1. Interviewer Intent
The interviewer wants to assess the candidate's learning methods, self-direction, resourcefulness, and adaptability when faced with unfamiliar frameworks or programming languages (like adopting Go or Gin).

### 2. Strong Answer
During the setup of the DSAblitz project, I had to implement HTTP API endpoints using the **Gin Web Framework** in Go, a framework I had not used before.

Instead of trying to read the entire documentation cover-to-cover or copying random snippets from the internet, I adopted a structured approach:
1. **Understand the Request Lifecycle**: I first learned how Gin structures HTTP middleware, routing groups, and request contexts.
2. **Review Codebase Patterns**: I studied the existing authentication middleware to understand how handlers authenticate users and pass context data.
3. **Build a Prototype**: I built a small, isolated endpoint first, verified its behavior, and then implemented the production routes for the matchmaking lobbies.

This structured learning allowed me to build the API endpoints quickly while adhering to the project's architecture.

### 3. Common Mistakes
* Giving abstract answers without concrete examples (e.g. saying "I read documentation and got it working" without naming the framework or the specific components).
* Admitting to copy-pasting code from StackOverflow or AI tools without understanding the underlying architectural patterns.
* Claiming to master a technology instantly, which lacks humility and realism.

### 4. Follow-up Questions
* **How did you verify that the new routes were working correctly?**
  * *Answer*: I wrote integration tests for the HTTP handlers using Go's `net/http/httptest` package, simulating client requests and verifying status codes.

### 5. How DSAblitz demonstrates this concept
The Gin router structure is set up in `server.go` and routes are registered across modules, demonstrating clean modular setup.

### 6. Relevant code references
* Gin router setup: [server.go:L19-L36](file:///home/tanishq/dsablitz/backend/internal/server/server.go#L19-L36)
* Routing registration: [routes.go:L37-L74](file:///home/tanishq/dsablitz/backend/internal/server/routes.go#L37-L74)

### 7. Related documentation
* [Request Lifecycle Reference](file:///home/tanishq/dsablitz/docs/architecture/request_lifecycle.md)
* [API Routes - Rooms](file:///home/tanishq/dsablitz/docs/api/rooms.md)

---

## Scenario 2: Incorporating Database Code Review Feedback

### 1. Interviewer Intent
The interviewer wants to assess if the candidate is open to constructive feedback, handles critiques professionally, and collaborates with peers to improve database schema designs.

### 2. Strong Answer
When designing the `room_players` table to track participants in matchmaking lobbies, I initially added unique constraints on `(room_id, user_id)` and `(room_id, seat_number)` to prevent players from double-joining or taking the same seat.

During code review, a senior engineer pointed out that these table-level unique constraints had a major flaw: when a player leaves a room, we update their status to `left` rather than deleting the row (to preserve logs). However, the unique constraint would block that player from joining a new room or re-joining the same room, as their old row still existed.

I welcomed the feedback and worked with the reviewer to find a solution. We replaced the table-level constraints with **PostgreSQL Partial Unique Indexes**:
```sql
CREATE UNIQUE INDEX idx_room_players_room_user_active 
ON room_players(room_id, user_id) 
WHERE status IN ('joined', 'ready');
```
This solved the bug while preserving the historical audit trail.

### 3. Common Mistakes
* Getting defensive during code reviews or taking technical critiques personally.
* Agreeing to feedback without understanding the technical rationale behind the change.
* Bypassing database-level constraints and writing validation checks solely in application code, leaving the database vulnerable to race conditions.

### 4. Follow-up Questions
* **Why did you decide to preserve player rows with a 'left' status instead of just deleting them?**
  * *Answer*: Deleting rows destroys the audit trail. Keeping player rows with a `left` status allows us to track presence, debug connection drop issues, and run analytics.

### 5. How DSAblitz demonstrates this concept
The database schema migration file contains the partial indexes implemented after the code review.

### 6. Relevant code references
* Room players partial indexes migration: [000004_fix_room_players_constraints.up.sql:L6-L12](file:///home/tanishq/dsablitz/backend/migrations/000004_fix_room_players_constraints.up.sql#L6-L12)
* Verification of player status enum: [models.go:L78-L112](file:///home/tanishq/dsablitz/backend/internal/rooms/models.go#L78-L112)

### 7. Related documentation
* [Database Schema Reference](file:///home/tanishq/dsablitz/docs/database/schema.md)
* [Migration Strategy & Operations](file:///home/tanishq/dsablitz/docs/database/migrations.md)

---

## Key Takeaways
1. **Adopting new frameworks** requires understanding the request lifecycle and routing patterns before writing code.
2. **Constructive code review** helps identify edge cases (like constraints blocking soft-deleted rows) that are easy to miss.
3. **Partial database indexes** enforce unique constraints on active states while preserving historic logs.

---

## Interview Questions
* **How do you handle disagreements with team members during a design discussion?**
  * *Answer*: I focus on facts, document the alternatives, and refer to project constraints (such as simplicity, safety, or scalability) to guide the decision.
* **Why is it important to write down Down migrations alongside Up migrations?**
  * *Answer*: Down migrations allow rolling back schema changes during local testing or pipeline failures, ensuring local environments match the codebase state.

---

## Common Mistakes
* **Defensiveness**: Rejecting feedback because "it works on my machine" without evaluating edge cases.
* **Ignoring constraints**: Omitting database-level constraints to speed up development, which leads to corrupted data.

---

## Related Documents
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
* [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)

---

## Lessons Learned
* **Soft-delete constraints**: Relational constraints must account for soft deletes. Applying a standard unique constraint on a soft-deleted table will block future inserts. Partial unique indexes resolve this issue.
