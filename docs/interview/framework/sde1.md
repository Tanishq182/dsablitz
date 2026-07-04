# Framework & Patterns - SDE1 Production Level

This document contains framework and design pattern interview notes for SDE1 level candidates, focusing on password hashing cryptography (Argon2id) and the Repository/Adapter pattern.

---

## Q&A Set 1: Argon2id Password Hashing

### 1. Interviewer Intent
The interviewer wants to assess the candidate's understanding of secure password storage, modern cryptographic algorithms, GPU/ASIC brute-force resistance, and how parameter configurations affect server CPU/memory performance.

### 2. Strong Answer
Passwords must never be stored in plain text. Traditional hashing algorithms like MD5 or SHA-256 are extremely fast and vulnerable to high-speed brute-force attacks using GPUs or custom ASICs.

We resolve this by using **Argon2id** (configured via `golang.org/x/crypto/argon2`), the industry standard for password hashing. Argon2id is a memory-hard and time-hard algorithm, meaning it requires a configurable amount of RAM and CPU iterations to compute a hash, preventing GPU/ASIC parallel acceleration.

Our implementation uses the following parameters:
* **Memory**: 64 MB (`argonMemory`) to resist memory-cost acceleration.
* **Iterations**: 3 passes (`argonIterations`) to control execution time.
* **Parallelism**: 2 threads (`argonParallelism`) to leverage multi-core CPUs.
* **Salt Length**: 16 random bytes (`argonSaltLength`) to prevent rainbow table attacks.

Verification uses constant-time comparison via Go's `subtle.ConstantTimeCompare` to prevent timing attacks, which deduce password hashes by measuring small differences in string comparison speeds.

### 3. Common Mistakes
* Using standard comparison operators (like `==`) to verify password hashes, exposing the system to timing attacks.
* Choosing legacy algorithms like Bcrypt or PBKDF2 without analyzing the security advantages of Argon2's memory-hardness.
* Setting Argon2 memory or execution parameters too high, which can slow down login APIs and expose the server to Denial of Service (DoS) CPU exhaustion attacks.

### 4. Follow-up Questions
* **What is the difference between Argon2d, Argon2i, and Argon2id?**
  * *Answer*: Argon2d is optimized for GPU resistance but vulnerable to side-channel timing attacks. Argon2i resists side-channel attacks but is vulnerable to GPU acceleration. Argon2id combines both approaches, providing resistance to both timing attacks and hardware acceleration.

### 5. How DSAblitz demonstrates this concept
The Auth module implements Argon2id password hashing and verification with secure default parameters.

### 6. Relevant code references
* Argon2 parameters configuration: [password.go:L15-L21](file:///home/tanishq/dsablitz/backend/internal/auth/password.go#L15-L21)
* HashPassword generation: [password.go:L23-L42](file:///home/tanishq/dsablitz/backend/internal/auth/password.go#L23-L42)
* VerifyPassword constant-time comparison: [password.go:L44-L52](file:///home/tanishq/dsablitz/backend/internal/auth/password.go#L44-L52)

### 7. Related documentation
* [Authentication API Reference](file:///home/tanishq/dsablitz/docs/api/auth.md)
* [Database Schema Reference](file:///home/tanishq/dsablitz/docs/database/schema.md)

---

## Q&A Set 2: Repository & Adapter Pattern for Domain Decoupling

### 1. Interviewer Intent
The interviewer wants to evaluate the candidate's understanding of Clean Architecture principles, specifically how to decouple business logic (domain services) from data persistence layers (SQL engines, database drivers) using interfaces and adapters.

### 2. Strong Answer
To keep business logic clean and maintainable, domain services should remain unaware of database-specific execution details (like SQL strings, pgx connection pools, or transactions). We achieve this decoupling using the **Repository and Adapter Pattern**.

The service layer defines a data access contract as an interface (e.g. `QuestionsRepository` inside `questions` package). The persistence layer implements this interface in a concrete structure containing the database driver context.

```
┌─────────────────────────────────┐
│        questions.Service        │ (Domain Logic)
└────────────────┬────────────────┘
                 │ (Depends on Interface)
                 ▼
┌─────────────────────────────────┐
│  questions.QuestionsRepository  │ (Interface Contract)
└────────────────┬────────────────┘
                 │ (Implemented by)
                 ▼
┌─────────────────────────────────┐
│      questions.Repository       │ (Concrete SQL Implementation)
└─────────────────────────────────┘
```

This decoupling provides major benefits:
1. **Mockability**: During unit testing, we inject mock implementations of the repository interface, allowing us to test the service's business logic without spinning up a real database.
2. **Persistence Independence**: If we decide to swap our PostgreSQL database for another data store, we only rewrite the repository adapter package, leaving the core domain service untouched.

### 3. Common Mistakes
* Injecting concrete repository structs directly into domain services, preventing mock injection for testing.
* Mixing database-specific details (like transaction interfaces or raw SQL queries) with service business rules.
* Allowing domain models to contain SQL driver tags or database schema-specific structures.

### 4. Follow-up Questions
* **How does the repository pattern handle database transaction boundaries?**
  * *Answer*: The repository defines a transaction wrapper (like `WithTransaction`), and interface methods accept the transaction handle (`pgx.Tx`), allowing the service to run multiple writes within a single atomic block.

### 5. How DSAblitz demonstrates this concept
The Questions module defines the `QuestionsRepository` interface inside the service file and implements it in a separate repository file.

### 6. Relevant code references
* Repository interface definition: [service.go:L11-L16](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L11-L16)
* Repository SQL adapter implementation: [repository.go:L20-L72](file:///home/tanishq/dsablitz/backend/internal/questions/repository.go#L20-L72)

### 7. Related documentation
* [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
* [Package Structure Reference](file:///home/tanishq/dsablitz/docs/architecture/package_structure.md)

---

## Key Takeaways
1. **Argon2id** is the industry standard for password hashing, using memory-hard parameters to resist GPU/ASIC brute-force attacks.
2. **Constant-time string comparison** (`subtle.ConstantTimeCompare`) prevents timing attacks from leaking password hashes.
3. **Repository interfaces** decouple domain services from SQL details, enabling mock testing and database flexibility.

---

## Interview Questions
* **What are timing attacks, and how do constant-time comparisons prevent them?**
  * *Answer*: Timing attacks measure how long a string comparison takes to fail, allowing attackers to guess hashes character-by-character. Constant-time comparisons check all bytes of a string regardless of match status, keeping response times uniform.
* **Why should SQL query errors be mapped to domain errors in the repository layer?**
  * *Answer*: Mapping database-specific errors (like `pgx.ErrNoRows`) to domain errors (like `ErrNotFound`) prevents SQL leakage, keeping the service layer decoupled.

---

## Common Mistakes
* **Timing attack exposure**: Verifying password hashes using standard comparison operators (like `==`), exposing the system to timing leaks.
* **Database driver leakage**: Importing driver packages or passing database-specific connections directly into domain services.

---

## Related Documents
* [Authentication API Reference](file:///home/tanishq/dsablitz/docs/api/auth.md)
* [Database Transactions](file:///home/tanishq/dsablitz/docs/database/transactions.md)

---

## Lessons Learned
* **Decoupled validation**: Implementing the repository interface allowed us to test the Questions service using mocks, resolving testing blockages and improving test coverage.
