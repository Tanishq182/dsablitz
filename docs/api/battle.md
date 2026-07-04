# Battle Engine API Reference

This document provides technical documentation for the Battle Engine's interface. It separates the current external HTTP routes, the current internal Go service layer interfaces, and the planned V2 REST endpoints.

---

## 1. Current HTTP API Surface (Current MVP)

> [!IMPORTANT]
> **No Public HTTP Routes**: The Battle module currently has **zero** public HTTP REST or WebSocket endpoints exposed to clients in the delivery layer ([battle/routes.go](file:///home/tanishq/dsablitz/backend/internal/battle/routes.go)). 

Any external client attempt to request battle endpoints directly will result in a `404 Not Found` from the Gin gateway, as no routes are registered.

---

## 2. Current Internal Go Service API

Matchmaking and room handlers interact with the Battle Engine programmatically at the service layer. The concrete service is defined in [battle/service.go](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L72).

### 2.1 `StartBattle`
Initializes a battle sequence, scorecards, and question sequences within a transaction context.
*   **Go Signature** ([service.go:L89](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L89)):
    ```go
    func (s *Service) StartBattle(ctx context.Context, roomID uuid.UUID, players []BattlePlayer, seed int64, durationSeconds int) (uuid.UUID, error)
    ```
*   **Go Parameters**:
    *   `roomID`: Mapped origin room lobby.
    *   `players`: List of participants (seat number, Elo ratings).
    *   `seed`: Seed for deterministic question shuffling.
    *   `durationSeconds`: Battle duration (120 or 300 seconds).

### 2.2 `GetNextQuestion`
Resolves a player's progression pointer and returns their current question.
*   **Go Signature** ([service.go:L157](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L157)):
    ```go
    func (s *Service) GetNextQuestion(ctx context.Context, battleID, userID uuid.UUID) (questions.SanitizedQuestionResponse, error)
    ```
*   **Go Return DTO** ([questions/models.go:L28-L37](file:///home/tanishq/dsablitz/backend/internal/questions/models.go#L28-L37)):
    ```go
    type SanitizedQuestionResponse struct {
        ID           uuid.UUID `json:"id"`
        QuestionType string    `json:"question_type"`
        Difficulty   int16     `json:"difficulty"`
        Title        string    `json:"title"`
        Prompt       string    `json:"prompt"`
        Options      []string  `json:"options,omitempty"`
        TimeLimitSec int       `json:"time_limit_sec"`
        Tags         []string  `json:"tags"`
    }
    ```

### 2.3 `SubmitAnswer`
Evaluates an answer submission, updates scorecards, and logs the attempt.
*   **Go Signature** ([service.go:L184](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L184)):
    ```go
    func (s *Service) SubmitAnswer(ctx context.Context, battleID, userID uuid.UUID, submissionIndex int, answer questions.SubmissionAnswer, responseTimeMs int) (SubmissionResult, error)
    ```
*   **Go Return DTO** ([models.go:L113-L119](file:///home/tanishq/dsablitz/backend/internal/battle/models.go#L113-L119)):
    ```go
    type SubmissionResult struct {
        IsCorrect            bool      `json:"is_correct"`
        AttemptsMade         int       `json:"attempts_made"`
        CurrentQuestionIndex int       `json:"current_question_index"`
        Score                int       `json:"score"`
        QuestionID           uuid.UUID `json:"question_id"`
    }
    ```

### 2.4 `CompleteBattle`
Concludes a match, determines the outcome, and updates player results.
*   **Go Signature** ([service.go:L303](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L303)):
    ```go
    func (s *Service) CompleteBattle(ctx context.Context, battleID uuid.UUID) error
    ```

---

## 3. Planned REST API (V2 / Phase 7C)

These endpoints are planned for Phase 7C to expose the internal Go service methods as public REST APIs.

### 3.1 `GET /api/v1/battle/:id/question`
Exposes `GetNextQuestion` via HTTP.
*   **Headers**: Requires `access_token` cookie.
*   **Response Schema** (Status `200 OK`):
    ```json
    {
      "id": "278c5c7d-8153-43ef-ad99-231a7bc89d2d",
      "question_type": "mcq",
      "difficulty": 3,
      "title": "Reverse Linked List",
      "prompt": "Write a function to reverse a singly linked list.",
      "options": ["O(1) space", "O(N) space", "O(log N) space"],
      "time_limit_sec": 60,
      "tags": ["linked-list", "pointers"]
    }
    ```

### 3.2 `POST /api/v1/battle/:id/submit`
Exposes `SubmitAnswer` via HTTP.
*   **Headers**: Requires `access_token` cookie.
*   **Request Schema**:
    ```json
    {
      "submission_index": 1,
      "answer": {
        "text_answer": "O(1) space"
      },
      "response_time_ms": 1450
    }
    ```
*   **Response Schema** (Status `200 OK`):
    ```json
    {
      "is_correct": true,
      "attempts_made": 1,
      "current_question_index": 1,
      "score": 1,
      "question_id": "90d1f4ba-d4f1-4dfb-90cb-ec65d4b5a2bf"
    }
    ```

---

## 4. Error Invariants

The internal service methods throw typed Go errors when invariants are violated, which will map to HTTP status codes in V2:

| Go Error Constant | Triggering Condition | Planned V2 HTTP Code |
| :--- | :--- | :--- |
| `ErrBattleFinished` | Submitting or querying a finished battle | `409 Conflict` |
| `ErrBattleExpired` | Action performed after match duration elapsed | `410 Gone` |
| `ErrDuplicateSubmission` | Resubmitting an answer or index mismatch | `409 Conflict` |
| `ErrInvalidSubmission` | Missing answer fields or index mismatch | `400 Bad Request` |
| `ErrQuestionExhausted` | Player completes all sequence questions | `204 No Content` |

---

## 5. Transaction Boundaries

- **`StartBattle`**: Starts a transaction (when called outside of the Rooms context). Initializes the battle, player scorecards, and question sequence in a single block.
- **`SubmitAnswer`**: Runs inside a transaction. Locks the player scorecard row (`GetBattlePlayerForUpdate`), validates the submission index, checks for duplicate attempts, updates progress, logs the submission, and updates the scorecard.
- **`CompleteBattle`**: Runs inside a transaction. Locks all participants sorted by ID (`ORDER BY user_id ASC FOR UPDATE`) to prevent deadlocks, evaluates scores, updates player results, resets the lobby status, and completes the battle.

---

## 6. Idempotency Considerations

- **SubmitAnswer**: The monotonic submission index checks prevent duplicate requests from being processed twice.
- **CompleteBattle**: If the battle status is already `finished`, the method returns immediately without repeating rating calculations or updates.

---

## 7. Production Considerations

- **In-Memory Question Cache**: Reading question details during matching and submission validation bypasses database I/O, utilizing the in-memory cache loaded at startup.
- **Pessimistic lock timeouts**: Real-time submissions run inside row locks, meaning we must monitor and limit transaction lock durations to prevent connection pool exhaustion.

---

## 8. Code References

- **Battle Service Layer**: [battle/service.go](file:///home/tanishq/dsablitz/backend/internal/battle/service.go)
- **Scorecard locking SQL**: [battle/repository.go:L119-L128](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go#L119-L128)
- **Monotonic Check**: [battle/service.go:L227-L233](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L227-L233)

---

## 9. Related Documents

- **Database Transactions**: [transactions.md](file:///home/tanishq/dsablitz/docs/database/transactions.md)
- **Submission Flow Deep Dive**: [submission_flow.md](file:///home/tanishq/dsablitz/docs/flows/submission_flow.md)
- **Battle Sequence Diagram**: [battle_sequence.md](file:///home/tanishq/dsablitz/docs/diagrams/battle_sequence.md)
