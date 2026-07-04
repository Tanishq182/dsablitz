# Battle Sequence Diagram

This document presents a sequence diagram showing the request lifecycle for initializing battles, retrieving questions, submitting answers, and completing matches.

---

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    autonumber
    actor Client as Player Client
    participant Service as Battle Service
    participant Repo as Battle Repository
    participant QService as Questions Service
    participant DB as PostgreSQL

    %% Start Battle Tx
    Note over Client, DB: Start Battle Flow (Coordinated from Rooms)
    Client->>Service: StartBattleTx(ctx, tx, roomID, players, seed, durationSeconds)
    Note over Service: Reuses active transaction context (tx pgx.Tx)
    Service->>QService: GetActiveQuestionsByFilters(ctx, 0, nil)
    Note over QService, Service: Reads from startup in-memory cache
    QService-->>Service: Active Questions List
    Note over Service: Generate deterministic shuffled sequence of question IDs using seed
    Service->>Repo: InsertBattle(tx, battle)
    Repo->>DB: INSERT INTO battles (id, room_id, status = 'active', battle_seed ...)
    Service->>Repo: InsertBattlePlayers(tx, players)
    Repo->>DB: INSERT INTO battle_players (id, battle_id, user_id, current_question_index = 0 ...)
    Service->>Repo: InsertBattleSequence(tx, battleID, sequence)
    Repo->>DB: INSERT INTO battle_question_sequence (battle_id, sequence_index, question_id)
    Service-->>Client: Battle ID

    %% Get Next Question
    Note over Client, DB: Get Next Question Flow
    Client->>Service: GetNextQuestion(ctx, battleID, userID)
    Service->>Repo: GetPlayerQuestionState(ctx, battleID, userID)
    Repo->>DB: SELECT b.status, bp.current_question_index, q.question_id FROM battle_players ... JOIN battles ... LEFT JOIN battle_question_sequence ...
    DB-->>Repo: Player state & mapped question ID
    Repo-->>Service: PlayerQuestionState Entity
    Note over Service: Verify battle is active and timer has not expired
    Service->>QService: GetSanitizedQuestion(ctx, questionID)
    QService-->>Service: SanitizedQuestionResponse
    Service-->>Client: Sanitized Question JSON (no answers revealed)

    %% Submit Answer
    Note over Client, DB: Submit Answer Flow
    Client->>Service: SubmitAnswer(ctx, battleID, userID, submissionIndex, answer, responseTimeMs)
    Service->>Repo: WithTransaction(ctx)
    Repo->>DB: BEGIN TRANSACTION
    Service->>Repo: GetBattlePlayerForUpdate(tx, battleID, userID)
    Repo->>DB: SELECT * FROM battle_players WHERE battle_id = $1 AND user_id = $2 FOR UPDATE
    DB-->>Repo: Player Progression Row Locked
    Service->>Repo: GetBattleTx(tx, battleID)
    Repo->>DB: SELECT * FROM battles WHERE id = $1
    DB-->>Repo: Battle Row
    Note over Service: Verify battle is active and timer has not expired
    Note over Service: Verify submissionIndex matches CorrectCount + IncorrectCount + 1
    Service->>Repo: GetSubmissionsForQuestion(tx, battleID, userID, qID)
    Repo->>DB: SELECT raw_answer FROM submissions WHERE ...
    DB-->>Repo: Previous submissions for question
    Note over Service: Verify answer is not a duplicate submission
    Service->>QService: ValidateAnswer(ctx, qID, answer)
    QService-->>Service: isCorrect
    Note over Service: Apply Option C Progression Policy (update player score & indices)
    Service->>Repo: InsertSubmission(tx, submissionParams)
    Repo->>DB: INSERT INTO submissions (id, battle_id, user_id, question_id, raw_answer, is_correct, score_awarded ...)
    Service->>Repo: UpdateBattlePlayer(tx, player)
    Repo->>DB: UPDATE battle_players SET score, correct_count, current_question_index ...
    Service->>Repo: Commit
    Repo->>DB: COMMIT TRANSACTION
    Service-->>Client: SubmissionResult JSON

    %% Complete Battle
    Note over Client, DB: Battle Completion Flow
    Client->>Service: CompleteBattle(ctx, battleID)
    Service->>Repo: WithTransaction(ctx)
    Repo->>DB: BEGIN TRANSACTION
    Service->>Repo: GetBattleTx(tx, battleID)
    Repo->>DB: SELECT * FROM battles WHERE id = $1
    Note over Service: If status is already finished, commit as no-op (idempotency)
    Service->>Repo: GetBattlePlayersTx(tx, battleID)
    Repo->>DB: SELECT * FROM battle_players WHERE battle_id = $1 ORDER BY user_id ASC FOR UPDATE
    DB-->>Repo: Match scorecards locked in sorted order
    Note over Service: Compare player scores and assign results (win/loss/draw)
    Service->>Repo: UpdateBattlePlayerResult(tx, battleID, player1, result)
    Repo->>DB: UPDATE battle_players SET result = $3 WHERE ...
    Service->>Repo: UpdateBattlePlayerResult(tx, battleID, player2, result)
    Repo->>DB: UPDATE battle_players SET result = $3 WHERE ...
    Service->>Repo: UpdateRoomStatusDirect(tx, roomID, 'ready')
    Repo->>DB: UPDATE rooms SET status = 'ready' WHERE id = roomID
    Service->>Repo: CompleteBattleWithResultTx(tx, battleID, winnerUserID, endedAt)
    Repo->>DB: UPDATE battles SET status = 'finished', winner_user_id = $3, ended_at = $4 ...
    Service->>Repo: Commit
    Repo->>DB: COMMIT TRANSACTION
    Service-->>Client: Success Status
```

---

## 2. Step-by-Step Trace

1.  **Start Battle Coordination**: The Rooms module starts a transaction and calls the Battle service, passing the transaction context (`tx`). The Battle service queries questions from the in-memory cache, shuffles them deterministically, and inserts the battle sequence.
2.  **Get Next Question**: Fetches progress pointers and sequence mappings in a single query using joins, reducing network roundtrips.
3.  **Submit Answer**: Starts a transaction, acquires a row lock on the player's scorecard, checks the monotonic index to prevent duplicates, validates correctness, applies the attempt policy, logs the submission, updates player counters, and commits.
4.  **Complete Battle**: Locks match participant rows sorted by ID to prevent deadlocks, evaluates scores, updates player results, resets the lobby status to `ready`, sets the battle status to `finished`, and commits.
