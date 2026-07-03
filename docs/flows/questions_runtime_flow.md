# Runtime Flow: Question Lookup and Validation

This document tracks the step-by-step execution path when a player submits an answer to a question in **DSAblitz**.

```
[Battle Service] 
       │ (1) ValidateAnswer(battleID, userID, questionID, rawAnswer)
       ▼
[Questions Service]
       │ (2) GetQuestionByID(ctx, questionID)
       ├─────────────────────────────────────────┐
       ▼ (Cache Hit - 99.9% of requests)         ▼ (Cache Miss - fallback)
[In-Memory Cache (sync.RWMutex)]           [Questions Repository]
       │                                         │
       │                                         ▼ (Query SQL row)
       │                                   [PostgreSQL DB]
       │                                         │
       ├─────────────────────────────────────────┘
       ▼
[Validation Library (validation.go)]
       │ (3) Evaluates SubmissionAnswer DTO against correct_answer key
       ▼
[Battle Service] (Receives validation boolean)
       │ (4) Adjusts player index & attempts counts; computes score changes
       ▼ (Executes writes inside SQL Tx)
[Battle Repository]
       │ (5) INSERT INTO submissions / UPDATE battle_players FOR UPDATE
       ▼
[PostgreSQL DB] (Tx Commit)
```

---

## 1. Domain Ownership Boundaries

### **A. Questions Module (Stateless Context)**
* **Responsibilities**: Metadata loading, client-safe sanitization, and value comparison.
* **Domain Invariants**: Enforces core entity validations (e.g. valid question types, difficulty ranges 1-5, valid UUID structures) inside `Question.Validate()`.
* **Database Reads**: Zero active queries under load. Lookups are routed to the Questions Service cache.

### **B. Battle Module (Stateful Engine)**
* **Responsibilities**: Progression index incrementing, attempts counter verification, scoring, and transaction coordination.
* **Concurrency Protection**: Locks the player progression row using pessimistic serialization:
  ```sql
  SELECT current_question_index, current_question_attempts 
  FROM battle_players 
  WHERE battle_id = $1 AND user_id = $2 
  FOR UPDATE;
  ```
* **Database Writes**: Writes submissions records and increments player pointers inside a single database transaction.
