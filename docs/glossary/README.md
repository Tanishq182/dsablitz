# DSAblitz Terminology Glossary

This glossary defines the domain concepts, states, and engineering terms used throughout the DSAblitz codebase.

---

## Domain Terms & Definitions

### Battle
* **Definition**: A competitive match session between registered players operating on a shared, deterministic stream of questions.
* **Core Table**: `battles` ([models.go](file:///home/tanishq/dsablitz/backend/internal/battle/models.go#L53))
* **Key Fields**: `started_at`, `ended_at`, `status`, `winner_user_id`.

### Room
* **Definition**: A game lobby that holds players before matchmaking completes and starts a Battle.
* **Core Table**: `rooms` ([models.go](file:///home/tanishq/dsablitz/backend/internal/rooms/models.go#L33))
* **States**: `waiting`, `ready`, `in_battle`, `closed`.

### Seat
* **Definition**: An allocated slot within a room lobby or battle mapping to a unique `user_id`. 
* **Key Fields**: `seat_number` (e.g., 1 or 2).

### Option C attempts policy
* **Definition**: The specific player progression rule for questions in a battle. A player has a maximum of two attempts to answer a question. 
* **Logic**:
  * Correct on 1st or 2nd attempt: Points awarded $\to$ Move to next question index.
  * Incorrect on 1st attempt: 0 points $\to$ Stay on question.
  * Incorrect on 2nd attempt: 0 points $\to$ Automatically skip to next question index.
* **Reference**: [SubmitAnswer](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L251)

### Monotonic Submission Index
* **Definition**: A client-supplied integer index that increments by exactly 1 for every attempt/submission made by the player in the active battle.
* **Purpose**: Prevents duplicate answer submissions caused by concurrent retries or network double-clicks by validating the client count against the server-computed count ($CorrectCount + IncorrectCount + 1$).
* **Reference**: [SubmitAnswer](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L225)

---

## Technical & State Terms

### In-Memory Cache
* **Definition**: A read-only questions map cache populated from static CSV data during server startup.
* **Implementation**: Thread-safe wrapper (`sync.RWMutex`) to minimize database reads during high-frequency validation checks.
* **Reference**: [Cache](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L18)

### Row-Level Lock
* **Definition**: A pessimistic database lock (`SELECT ... FOR UPDATE`) used to serialize concurrent edits on a single player or room progression row.
* **Purpose**: Prevents race conditions during state transitions and answer evaluations.
* **Reference**: [GetBattlePlayerForUpdate](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L125)

---

## Related Documents
* For feature milestones, see [roadmap](file:///home/tanishq/dsablitz/docs/roadmap/README.md).
* For structural details, see [overall_architecture.md](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md).
