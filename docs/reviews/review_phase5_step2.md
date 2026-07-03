# Architecture Review: Phase 5 Step 2 (Questions Module MVP)

This document provides a post-implementation architecture review of the **Questions Module MVP** (Phase 5, Step 2) design for **DSAblitz**.

---

## 1. Approved Decisions

1. **Stateless Service Boundaries**: The Questions module operates purely as a read-heavy, stateless metadata and validation provider. It owns zero player state, match progression index pointer logic, or scoring code.
2. **Type-Safe UUID Mapping**: Shifted all domain entity and database reference fields from `string` to type-safe `github.com/google/uuid.UUID`. The pgx driver natively maps PostgreSQL's UUID fields to this type.
3. **Idempotent Upsert Seeding**: Primary key collision handling (`ON CONFLICT (id) DO UPDATE`) was implemented inside the seeder, avoiding non-standard database uniqueness constraints on properties like `title`.
4. **Strongly Typed Validation**: Introduced the `SubmissionAnswer` DTO containing typed text, float, and slice pointers, eliminating generic interface type assertions in validation functions.
5. **Zero-DB Read Validation Cache**: Offloaded active gameplay read transactions from PostgreSQL by introducing a thread-safe global in-memory lookup cache (`sync.RWMutex`) inside the Questions Service.
6. **Separation of Invariants**: Kept `Question.Validate()` restricted strictly to structural domain invariants (UUIDs, prompt non-emptiness, and limits), shifting product policies (option checks, correct answer existence) to the seeder and validation logic layers.

---

## 2. Concurrency & Performance

### **Active Gameplay Database Read Elimination**
* **The Problem**: Running `FindQuestionByID` on every answer submission would cause database lock contention and connection starvation at scale.
* **The Solution**: The Questions Service loads the static question catalog into memory (`map[uuid.UUID]Question`) at startup.
* **Result**: Validation and DTO queries execute as sub-microsecond memory lookups, reducing gameplay database read operations to zero.

---

## 3. Boundary Mapping & Domain Invariants

* **Questions Module Boundary**: Confined to question metadata storage, cached retrieval, DTO sanitization, and stateless answer checks.
* **Battle Module Boundary**: Owns match sequence tables, progress pointer rows, transaction-safe optimistic/pessimistic locking, submissions, and scoring.
* **Domain Invariants (`models.go`)**: Validates that difficulty is between 1 and 5, and time limits are between 10 and 120 seconds.
* **Business Rules (`seeder.go`)**: Validates that MCQ options contain exactly 4 values and that the answer key exists within the options list.
