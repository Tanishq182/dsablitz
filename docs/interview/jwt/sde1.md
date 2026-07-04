# JWT Security - SDE1 Level Interview Prep

This guide focuses on the database structures, constraints, transactional session rotation, and programmatic error mapping used to implement JWT refresh systems in production.

---

## Q&A Sets

### Q1: Why should refresh tokens be hashed (e.g., using SHA-256) before storing them in the database? How does our database schema structure support active session verification?

#### Interviewer Intent
Evaluate the candidate's understanding of data-at-rest security principles (protecting credentials in case of a DB leak) and efficient schema design (partial indexing, database constraints).

#### Strong Answer
Refresh tokens are long-lived credentials that grant administrative access to user sessions. If we store them in plaintext, a database leak (either via SQL injection, backup exposure, or internal compromise) would compromise all active user accounts immediately. To prevent this, we treat refresh tokens similarly to passwords: we generate cryptographically secure, high-entropy tokens via `crypto/rand`, and store only their one-way hash (SHA-256) in the database.

When the client presents a refresh token, we compute its SHA-256 hash and search for a matching record. To make this verification highly performant, we structure our database schema with:
1. **Unique Constraint**: The `refresh_token_hash` column is marked `UNIQUE` to prevent any duplicate session rows.
2. **Partial Indexing**: We define a partial index:
   ```sql
   CREATE INDEX idx_auth_sessions_refresh_token_hash_active
       ON auth_sessions(refresh_token_hash)
       WHERE revoked_at IS NULL;
   ```
   Since most sessions are eventually revoked or expired, the database only indexes active sessions. This reduces index size, keeps the index hot in memory, and accelerates session validation queries.
3. **Integrity Constraint**: A check constraint ensures that `expires_at > created_at`, preventing malformed session durations.

#### Common Mistakes
* Storing refresh tokens in plaintext.
* Using slow hashing algorithms like `bcrypt` or `Argon2` for API token lookups. (Bcrypt is designed to be slow to prevent brute-force attacks on low-entropy human passwords, but since refresh tokens have high entropy (256 bits of randomness), SHA-256 is secure and provides sub-millisecond lookups).
* Forgetting to index columns used in WHERE clauses (like `expires_at` or `user_id` when checking user sessions).

#### Follow-up Questions
* Why don't we store the SHA-256 hash of the Access Token in the database?
* How does `ON DELETE CASCADE` on `user_id` help with user management and data compliance (e.g., GDPR)?
* What is the purpose of the partial index filter `WHERE revoked_at IS NULL` compared to a composite index?

#### How DSAblitz demonstrates this concept
DSAblitz generates high-entropy base64 refresh tokens and hashes them using SHA-256. The database table `auth_sessions` implements the unique constraint, length constraint, and partial indexing.

#### Relevant code references
* [token.go:L136-L149](file:///home/tanishq/dsablitz/backend/internal/auth/token.go#L136-L149) - `GenerateRefreshToken` and `HashRefreshToken` using SHA-256.
* [repository.go:L146-L162](file:///home/tanishq/dsablitz/backend/internal/auth/repository.go#L146-L162) - Finding active session by hash.
* [000002_create_auth_sessions.up.sql:L1-L20](file:///home/tanishq/dsablitz/backend/migrations/000002_create_auth_sessions.up.sql#L1-L20) - Database structure and partial index.

#### Related documentation
* [database/schema.md](file:///home/tanishq/dsablitz/docs/database/schema.md)
* [database/indexing.md](file:///home/tanishq/dsablitz/docs/database/indexing.md)

---

### Q2: How does the server handle token rotation during the refresh flow, and how are errors like unique constraint violations or database integrity issues handled programmatically?

#### Interviewer Intent
Check the candidate's implementation of session rotation logic, concurrency management (ensuring atomicity of revocation/creation), and domain-specific error mapping.

#### Strong Answer
To rotate a token, the application must perform two steps atomically:
1. **Revoke the old session**: Mark the current session as revoked by setting `revoked_at = NOW()`.
2. **Issue the new session**: Insert a new session record containing the new token's hash.

We wrap these operations inside an explicit PostgreSQL transaction (`tx`). This prevents partial state issues: if either step fails (e.g., the old session is already revoked, or inserting the new token fails), the database rolls back the entire transaction.

To ensure errors are cleanly handled and do not leak internal database implementation details:
* The repository executes a transaction and inspects PostgreSQL errors.
* We inspect PG error code `23505` (unique violation) using the constraint name to return domain errors like `ErrEmailTaken` or `ErrHandleTaken`.
* If a session rotation fails because the rows affected during update is not exactly 1 (meaning the token was already rotated or revoked), we immediately abort and return a domain-level `ErrUnauthorized` error.
* The HTTP handler catches these domain errors and maps them to HTTP status codes (e.g., `401 Unauthorized` for `ErrInvalidToken`/`ErrUnauthorized`, `409 Conflict` for `ErrEmailTaken`).

#### Common Mistakes
* Performing session revocation and creation in separate non-transactional database connections, leaving room for race conditions.
* Allowing multiple active refresh tokens to exist simultaneously without revoking the previous one, violating token rotation policy.
* Returning raw database error strings to the client, exposing schema details.

#### Follow-up Questions
* If a network error occurs after the database transaction is committed but before the client receives the response, how can the client recover?
* How does the system handle concurrent duplicate requests to `/refresh`?
* What is the purpose of checking `RowsAffected` after the UPDATE query in session rotation?

#### How DSAblitz demonstrates this concept
DSAblitz coordinates session rotation within a single database transaction in `Service.Refresh`. The repository handles the update-and-insert logic, checking row counts and checking unique constraints.

#### Relevant code references
* [service.go:L80-L125](file:///home/tanishq/dsablitz/backend/internal/auth/service.go#L80-L125) - Transactional refresh method.
* [repository.go:L164-L199](file:///home/tanishq/dsablitz/backend/internal/auth/repository.go#L164-L199) - `RotateSession` method executing in a transaction block.
* [repository.go:L249-L252](file:///home/tanishq/dsablitz/backend/internal/auth/repository.go#L249-L252) - Hashing unique violation mapping helper.
* [handler.go:L164-L175](file:///home/tanishq/dsablitz/backend/internal/auth/handler.go#L164-L175) - Handler level error translation block.

#### Related documentation
* [database/transactions.md](file:///home/tanishq/dsablitz/docs/database/transactions.md)
* [api/auth.md](file:///home/tanishq/dsablitz/docs/api/auth.md)

---

## Key Takeaways
* **SHA-256 Hashing**: Prevents session hijacking in case of database leaks, while keeping verification checks highly performant.
* **Partial Indexing**: Speeds up queries by only indexing active sessions (`WHERE revoked_at IS NULL`).
* **Transactional Rotation**: Atomic `RotateSession` prevents orphan tokens and double-use windows.
* **Clean Error Mapping**: Isolates database constraints from the API presentation layer to prevent schema leakage.

## Interview Questions
1. Why is a standard SHA-256 hash safe for refresh tokens, but unsafe for user passwords?
2. How does the partial index on `refresh_token_hash` optimize database memory usage?
3. What mechanism prevents multiple active refresh tokens from being generated from the same parent session?

## Common Mistakes
* Logging raw refresh tokens or their database hashes in debugging messages.
* Trusting the client-supplied user ID instead of validating the user ID directly associated with the active session token hash in the database.
* Neglecting database constraint checks inside Go service unit tests.

## Related Documents
* [database/transactions.md](file:///home/tanishq/dsablitz/docs/database/transactions.md)
* [database/schema.md](file:///home/tanishq/dsablitz/docs/database/schema.md)
* [flows/login_flow.md](file:///home/tanishq/dsablitz/docs/flows/login_flow.md)

## Lessons Learned
* Database integrity starts with constraints. Defining indices and triggers properly minimizes application logic bloat.
* Always enforce transactions for compound state changes (e.g. invalidate old, create new) to guarantee consistency in the face of sudden server restarts or concurrent requests.
