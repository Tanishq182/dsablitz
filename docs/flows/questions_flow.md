# Questions Module Execution Flows

This document details the critical execution paths for the Questions module.

## 1. Question Ingestion Flow (File-Based Seeding)

During application startup, local JSON/YAML files containing predefined questions are parsed and loaded into the database. Admin HTTP CRUD APIs are deferred to V2.

```mermaid
sequenceDiagram
    actor DevOps/CLI
    participant Seeder as Seeder / CLI Tool
    participant Service as QuestionsService
    participant Repo as QuestionsRepository
    participant DB as PostgreSQL

    DevOps/CLI->>Seeder: Execute seeding command / Startup script
    Note over Seeder: Load & Parse JSON/YAML files<br/>(e.g., seeds/questions.yaml)
    loop For each Question in File
        Seeder->>Service: IngestQuestion(ctx, QuestionDetail)
        Service->>Repo: InsertQuestion(ctx, DBParams)
        Repo->>DB: INSERT INTO questions ...
        DB-->>Repo: Return inserted Question row & ID
        Repo->>DB: INSERT INTO question_stats (question_id) VALUES (id)
        DB-->>Repo: OK
        Repo-->>Service: Return Question
        Service-->>Seeder: Return OK
    end
```

## 2. Shared Question Stream Generation (Battle Init)

When a match starts, the Battle Module generates a shared, deterministic question stream (e.g., unlimited stream capacity or a very large pool) based on the room's difficulty, tags, and seed. Both players share this identical stream of question IDs.

```mermaid
sequenceDiagram
    participant Battle as BattleService
    participant QService as QuestionsService
    participant QRepo as QuestionsRepository
    participant DB as PostgreSQL

    Note over Battle: Room Init (5 or 10 min match duration)
    Battle->>QService: GetSharedQuestionStream(ctx, difficulty, tags, roomSeed)
    QService->>QRepo: FindActiveQuestionIDs(ctx, difficulty, tags)
    QRepo->>DB: SELECT id FROM questions WHERE is_active=true AND difficulty=$1
    DB-->>QRepo: Slice of matching Question IDs
    QRepo-->>QService: Return list of IDs
    Note over QService: Shuffle / order question IDs<br/>deterministically using roomSeed
    QService-->>Battle: Return ordered List of Question IDs
    Note over Battle: Store question stream in PostgreSQL
```

## 3. Question Retrieval & Sanitization Flow (Asynchronous Battle Progression)

Players progress through the shared question stream at different speeds. When Player A requests their current question (e.g., Q8) or Player B requests theirs (e.g., Q5), the Battle Service retrieves the question ID from the shared stream, gets the sanitized question, and returns it to the player.

```mermaid
sequenceDiagram
    actor PlayerA as Player A (on Q8)
    actor PlayerB as Player B (on Q5)
    participant Battle as BattleService (WebSocket/HTTP)
    participant QService as QuestionsService
    participant QRepo as QuestionsRepository
    participant DB as PostgreSQL

    par Player A requests Q8
        PlayerA->>Battle: Request Question at index 7 (Q8)
        Note over Battle: Look up question ID at index 7<br/>from Room stream in PostgreSQL
        Battle->>QService: GetSanitizedQuestion(ctx, qid_8)
        QService->>QRepo: FindQuestionByID(ctx, qid_8)
        QRepo->>DB: SELECT * FROM questions WHERE id = $1
        DB-->>QRepo: Question Row (contains correct_answer)
        QRepo-->>QService: Return Question Entity
        QService-->>Battle: Return SanitizedQuestionResponse (No answer/explanation)
        Battle-->>PlayerA: Send Q8 Sanitized JSON
    and Player B requests Q5
        PlayerB->>Battle: Request Question at index 4 (Q5)
        Note over Battle: Look up question ID at index 4<br/>from Room stream in PostgreSQL
        Battle->>QService: GetSanitizedQuestion(ctx, qid_5)
        QService->>QRepo: FindQuestionByID(ctx, qid_5)
        QRepo->>DB: SELECT * FROM questions WHERE id = $1
        DB-->>QRepo: Question Row (contains correct_answer)
        QRepo-->>QService: Return Question Entity
        QService-->>Battle: Return SanitizedQuestionResponse (No answer/explanation)
        Battle-->>PlayerB: Send Q5 Sanitized JSON
    end
```
