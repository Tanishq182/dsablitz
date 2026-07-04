# HR - SDE1 Production Level

This document contains HR and team-fit interview notes for SDE1 level candidates, focusing on feature ownership, quality assurance, and prioritization when balancing feature requests with technical debt.

---

## Q&A Set 1: End-to-End Feature Ownership (Auth Session Tracking)

### 1. Interviewer Intent
The interviewer wants to assess the candidate's autonomy, execution capability, and commitment to quality when owning a feature end-to-end—from database migrations to API routing.

### 2. Strong Answer
Taking complete ownership of a feature means ensuring its security, performance, and reliability at every layer of the stack.

For example, when implementing the **Auth Session Tracking** feature, I owned the entire pipeline:
1. **Database Schema**: Created the `auth_sessions` table migration, defining columns for refresh token hashes, IP addresses, and session expiration.
2. **Repository Layer**: Wrote query methods to insert sessions, retrieve sessions, and rotate refresh tokens atomically inside database transactions.
3. **Domain Service**: Implemented the token manager, which generates access tokens and handles refresh token rotations securely.
4. **API Routing**: Configured the HTTP handlers and routes in Gin, securing them with middleware.

To ensure quality, I wrote unit tests validating access token expiration and refresh token rotation, verifying that the session was revoked if reused. This end-to-end ownership ensured a secure and reliable auth implementation.

### 3. Common Mistakes
* Implementing only the API handler or repository layer and leaving the other components (like migrations or unit tests) to other team members.
* Skipping unit tests or relying solely on manual testing for critical paths like authentication.
* Failing to document configuration details (like secret keys or token TTLs).

### 4. Follow-up Questions
* **How did you secure refresh tokens stored in the database?**
  * *Answer*: We store SHA-256 hashes of the refresh tokens rather than raw values, protecting active user sessions from database compromise.

### 5. How DSAblitz demonstrates this concept
The Auth module contains the complete flow, from migrations to token verification services.

### 6. Relevant code references
* Auth session migrations: [000002_create_auth_sessions.up.sql:L1-L15](file:///home/tanishq/dsablitz/backend/migrations/000002_create_auth_sessions.up.sql#L1-L15)
* Token verification: [token.go:L88-L134](file:///home/tanishq/dsablitz/backend/internal/auth/token.go#L88-L134)
* Token manager tests: [token_test.go:L1-L100](file:///home/tanishq/dsablitz/backend/internal/auth/token_test.go#L1-L100)

### 7. Related documentation
* [Authentication API Reference](file:///home/tanishq/dsablitz/docs/api/auth.md)
* [Database Schema Reference](file:///home/tanishq/dsablitz/docs/database/schema.md)

---

## Q&A Set 2: Managing Time & Prioritizing Refactors

### 1. Interviewer Intent
The interviewer wants to evaluate the candidate's prioritization framework, pragmatism, and how they balance business feature deadlines with critical system refactoring.

### 2. Strong Answer
Balancing business feature deadlines with technical refactoring requires a clear risk-impact framework. I prioritize tasks based on whether they block users or impact system stability.

For example, when we encountered room code collisions in PostgreSQL that aborted transactions and blocked users, I had to prioritize this refactor over implementing new social features. Although social features were on the roadmap, a bug that crashes matchmaking lobbies directly impacts user retention.

I presented the tradeoffs to the product manager:
* Implementing the refactor takes 1 day and resolves lobby crashes.
* Delaying it risk-contaminates the database connection pool under high load.

We agreed to prioritize the refactor. I moved the code generation retry logic outside of the transaction block. This resolved the issue, and we delivered the next features on a stable foundation.

### 3. Common Mistakes
* Refactoring code without aligning with product managers or explaining the business impact of the technical debt.
* Ignoring database stability bugs to hit feature deadlines, leading to outages in production.
* Making massive, unapproved refactors that delay roadmap features.

### 4. Follow-up Questions
* **How do you track technical debt to ensure it isn't forgotten?**
  * *Answer*: We list technical debt items (like asynchronous seeder parsing or rating persistence) directly in our project documentation and review them during planning.

### 5. How DSAblitz demonstrates this concept
The project roadmap and technical debt items are tracked in `PROJECT_CONTEXT.md` to guide prioritization.

### 6. Relevant code references
* Phase 5 technical debt registry: [PROJECT_CONTEXT.md:L82-L87](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L82-L87)
* Resolved audit implementation: [PROJECT_CONTEXT.md:L89-L96](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md#L89-L96)

### 7. Related documentation
* [Modular Monolith Design ADR](file:///home/tanishq/dsablitz/docs/adr/0001_modular_monolith_design.md)
* [Migration Strategy & Operations](file:///home/tanishq/dsablitz/docs/database/migrations.md)

---

## Key Takeaways
1. **Feature ownership** requires delivering database migrations, service logic, API routes, and unit tests as a single unit.
2. **Refresh tokens** must be stored as SHA-256 hashes to secure active user sessions against database leaks.
3. **System stability refactors** should be prioritized over new features if they block users or impact database connections.

---

## Interview Questions
* **Why are unit tests critical for authentication token managers?**
  * *Answer*: They verify that expired tokens are rejected, signatures are validated, and claims are parsed correctly, preventing authentication bypass bugs.
* **How do you communicate the value of technical refactors to non-technical stakeholders?**
  * *Answer*: I explain the risk in terms of system downtime, user crashes, and future development slowdowns.

---

## Common Mistakes
* **Incomplete ownership**: Leaving database migrations or test coverage out of feature pull requests.
* **Ignoring roadmap constraints**: Refactoring code without aligning with the team, causing roadmap delays.

---

## Related Documents
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
* [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)

---

## Lessons Learned
* **Prioritizing stability**: Addressing transaction boundary issues early resolved connection failures and provided a reliable foundation for gameplay features.
