# DSAblitz Interview Preparation: Project Stories

This document compiles core engineering stories from the development of **DSAblitz** formatted as STAR narratives (Situation, Task, Action, Result) for engineering and system design interviews.

---

## 1. Module Boundary Leakage (Domain Boundaries)

### Interview Question
> *"Tell me about a time you had to refactor a system's architecture because of poor boundary definitions or tight coupling."*

### STAR Narrative
* **Situation**: During the initial scoping of the Questions module for DSAblitz, we needed a way to serve questions, evaluate answers, update player scores, and record submissions. To move quickly, we placed the player's progression state (`current_question_index`, `attempts`), submission persistence, and scoring logic directly within the Questions module.
* **Task**: As we prepared to scale, we realized this design violated clean architectural boundaries. The Questions module (which should be a read-heavy, static catalog of DSA questions) was suddenly responsible for high-write player game loops and business-heavy scoring rules. This tightly coupled the Questions module to the Battle gameplay engine, making independent scaling and code maintenance difficult.
* **Action**: I decoupled the concerns by redefining domain ownership:
  1. The **Questions Module** was restricted to read-only question metadata retrieval and stateless answer validation (`ValidateAnswer(questionID, rawAnswer)`).
  2. The **Battle Module** took complete ownership of match-related writes: sequence storage, player pointers, score increments, and submission persistence.
  3. I designed a clean interface boundaries where the Battle Service queries the Questions Service for validation results and sanitized question payloads, keeping the database transaction boundary isolated to the Battle Module.
* **Result**: The refactored monolith has clean separation. If we need to transition to microservices in V2, the Questions module can be sliced off as a read-heavy microservice with 100% read-caching, while the Battle engine handles the high-intensity write workloads independently.

### Technical Deep Dive
```
[Questions Module] <--- Stateless Call --- [Battle Module (Active Transaction)]
  - FindQuestionByID                          - Acquire Lock: SELECT FOR UPDATE
  - ValidateAnswer(id, ans)                   - Evaluate progression & scoring
                                              - Insert to submissions
                                              - Commit
```
The Questions module uses `pgxpool.Pool` to fetch question metadata. The Battle module starts a transaction, uses a pessimistic lock on the `battle_players` table, executes stateless validation via Questions, updates progression, records the submission, and commits—all within the Battle module boundary.

### Key Engineering Lesson
**Domain ownership > developer convenience.** Writing code within a single module is faster initially, but treating a modular monolith's boundaries as if they were microservice network boundaries forces you to write decoupled, clean interfaces that stand the test of scale.

### 30-Second Verbal Pitch
> "While building a real-time DSA battle app, we initially let the Questions module handle player progression and scoring. I quickly realized this created a major boundary leak: a static question database shouldn't handle highly dynamic game-state writes. I refactored the boundaries, restricting the Questions module to read-only metadata and stateless validation, and shifted progression and scoring into the Battle module. This decoupled the system, allowing us to cache the static question bank aggressively while isolating live write transactions to the Battle database engine."

---

## 2. Race Condition in Answer Submission (Concurrency)

### Interview Question
> *"Describe a challenging concurrency issue you encountered and how you resolved it."*

### STAR Narrative
* **Situation**: In a rapid-fire coding blitz match, players submit answers in sub-second intervals. Under load testing, we detected a race condition: if a player double-clicked a submit button or sent two rapid submissions, the server processed both concurrently.
* **Task**: Since the progression read and write operations were not atomic, both concurrent requests read the same progress index (e.g., Question 3) and processed it. This led to duplicate points being awarded for the same question, out-of-order pointer jumps, and database index inconsistencies.
* **Action**: I implemented a database-level concurrency control strategy. Rather than using distributed locks in Redis (which would introduce an extra network hop and memory overhead for the MVP), I leveraged PostgreSQL's transactional locking. Inside the evaluation transaction, the player's progression state is read using `SELECT ... FOR UPDATE` on the `battle_players` table, blocking any concurrent requests for that specific player in that battle until the transaction commits or aborts.
* **Result**: We eliminated duplicate scoring and state corruption. Concurrent submissions from the same user are now serialized at the database connection layer, ensuring consistent, atomic, and safe progressions.

### Technical Deep Dive
By utilizing `FOR UPDATE`, Postgres locks the returned rows as if they were updated:
```sql
BEGIN;
SELECT current_question_index, current_question_attempts
FROM battle_players
WHERE battle_id = $1 AND user_id = $2
FOR UPDATE; -- Blocks concurrent executions for this specific player
```
This forces the second request to wait until the first transaction completes. Once the lock is released, the second transaction reads the updated index (e.g., Question 4) and immediately rejects the late request as a stale submission.

### Key Engineering Lesson
**Lock as close to the source of truth as possible.** While application-level mutexes or Redis locks are common, utilizing relational database locks (`FOR UPDATE`) provides ACID guarantees with minimal architectural overhead when the database is your system's primary source of truth.

### 30-Second Verbal Pitch
> "During load testing of our 1v1 battle engine, we found that rapid sub-second submissions caused a race condition where a player could double-submit the same answer and get double points. I fixed this by implementing row-level pessimistic locking in PostgreSQL. By executing a `SELECT ... FOR UPDATE` on the player's progress row within an atomic transaction, we serialized concurrent evaluation requests. This guaranteed that the second request would block until the first completed, reading the updated index and immediately rejecting the duplicate submission."

---

## 3. Battle Question Sequence Schema Debate (Database Design)

### Interview Question
> *"How do you evaluate database normalization tradeoffs when designing schemas for high-throughput systems?"*

### STAR Narrative
* **Situation**: When designing the database schema to store the 200-question stream for each battle, we debated two approaches: Option A (denormalized `UUID[]` array directly in the `battles` table) and Option B (normalized `battle_question_sequence` junction table with columns `battle_id`, `sequence_index`, and `question_id`).
* **Task**: I needed to choose the architecture that would scale to hundreds of thousands of active matches while supporting analytics, replayability, and dynamic question serving.
* **Action**: I compared both options deeply. While Option A offered faster read performance (zero joins) and lower database size growth, Option B offered structural normalization, standard referential integrity, and superior query analytics (e.g., joining submissions against sequence indices to analyze question difficulty).
* **Result**: I recommended and implemented **Option B**. To offset the write overhead of inserting 200 rows per battle, we batch-inserted the sequence rows using a single transaction and connection pool pipeline, keeping connection times under 10ms.
* **Tradeoffs**: Option B increases table size faster (1 million battles = 200 million sequence rows), but with proper indexing on `(battle_id, sequence_index)` and PostgreSQL partitioning, the analytical and structural integrity benefits vastly outweighed the storage costs.

### Technical Deep Dive
* **Option A (`UUID[]`)**:
  - Pros: Single write on battle start, fast read (`SELECT question_ids FROM battles`).
  - Cons: Violates 1NF; difficult to query which battles used a specific question without expensive array searches; lack of referential constraints (a question could be deleted while referenced in an array).
* **Option B (`battle_question_sequence` table)**:
  - Pros: Clean relational design; constraints prevent deleting active questions; trivial queries for question analytics.
  - Cons: Requires inserting $N$ rows at match creation. Optimized via batch inserting:
    ```go
    batch := &pgx.Batch{}
    for idx, qID := range sequence {
        batch.Queue("INSERT INTO battle_question_sequence ...", battleID, idx, qID)
    }
    br := tx.SendBatch(ctx, batch)
    ```

### Key Engineering Lesson
**Optimize write pipelines rather than compromising schema integrity.** Normalization is often prematurely abandoned for denormalization under the guise of performance. Utilizing batching, bulk queries, and transaction connection pools allows you to scale clean relational schemas without taking performance hits.

### 30-Second Verbal Pitch
> "To store the 200-question stream per battle, we debated between storing a denormalized UUID array inside the battles table versus creating a normalized sequence junction table. I pushed for the normalized junction table because it enforced relational integrity and allowed rich query analytics, like identifying which question index caused the most drop-offs. To overcome the write overhead of inserting 200 rows at the start of every battle, we implemented a bulk batch-insert pipeline in Go, achieving sub-10ms insertion times while maintaining a fully normalized schema."

---

## 4. Deterministic Randomness Challenge (Algorithms)

### Interview Question
> *"Can you give an example of an algorithmic bug you solved that involved random seed generation or determinism?"*

### STAR Narrative
* **Situation**: For 1v1 fairness, both players in a battle must solve the same sequence of questions in the same order, but the sequence must vary between matches. We achieved this by generating a sequence of 200 question IDs using a deterministic `battle_seed` at match creation.
* **Task**: Our active question bank had roughly 50 questions. When generating a stream of 200 questions, the generator shuffled the pool and appended it. However, the initial algorithm had a critical bug: it repeated the identical shuffled cycle of 50 questions four times. A player reaching Question 51 instantly knew the order of the remaining questions.
* **Action**: I refactored the stream generator. Instead of a static modulo loop, I implemented a stateful reshuffle. The generator shuffles the base pool, appends it to the stream, and then shuffles the pool *again* using the **same** active Pseudo-Random Number Generator (PRNG) instance before appending the next chunk.
* **Result**: Because the PRNG progresses its internal state with every shuffle call, each of the 4 cycles is shuffled differently. The stream remains 100% deterministic for a given `battle_seed` (both players get the exact same sequence), but the pattern does not repeat.

### Technical Deep Dive
```go
// Flawed modulo loop (predictable cycles):
for i := 0; i < 200; i++ {
    sequence[i] = shuffled[i % len(shuffled)] // repeats every N questions
}

// Stateful Reshuffle (non-repeating cycles):
r := rand.New(rand.NewSource(battleSeed))
sequence := make([]string, 0, 200)
for len(sequence) < 200 {
    poolCopy := make([]Question, len(activeQuestions))
    copy(poolCopy, activeQuestions)
    r.Shuffle(len(poolCopy), func(i, j int) {
        poolCopy[i], poolCopy[j] = poolCopy[j], poolCopy[i]
    })
    for _, q := range poolCopy {
        if len(sequence) < 200 {
            sequence = append(sequence, q.ID)
        }
    }
}
```

### Key Engineering Lesson
**Deterministic randomness requires strict state lifecycle management.** Shuffling with a seed is only deterministic if the operations applied to the RNG are identical and stateful. Re-initializing or using simple modulos defeats the unpredictable nature of PRNG sequences.

### 30-Second Verbal Pitch
> "To keep our 1v1 matches fair, we generate a 200-question stream using a deterministic battle seed. Since our question bank was smaller than 200, our initial generator looped the shuffled list, resulting in a predictable repeating cycle of questions. I solved this by implementing a stateful reshuffling pipeline. By using a single, stateful RNG instance to continuously shuffle and append the question pool to the stream, we ensured the sequence is completely deterministic and identical for both players, but completely unpredictable from one cycle to the next."

---

## 5. Anti-Cheat Architecture (Security)

### Interview Question
> *"How did you design your APIs to prevent client-side cheating and reverse-engineering of answers?"*

### STAR Narrative
* **Situation**: DSAblitz is a fast-paced competitive blitz platform. If correct answers or explanations are exposed in the JSON response payload when fetching questions, a player could easily inspect network traffic, extract the answers, and write a bot to cheat.
* **Task**: I had to design a secure, zero-trust API that serves all question metadata necessary to render the UI, without leaking the answer or explanation.
* **Action**: I implemented a **Payload Sanitization** pattern and **Backend-only Validation**:
  1. I created a client-safe DTO (`SanitizedQuestionResponse`) that excludes the `correct_answer` and `explanation` fields.
  2. The database entity (`Question`) contains the answers, but the controller never serializes this to the client.
  3. When a player submits an answer, they send the raw input to the Battle API. The backend retrieves the actual question from PostgreSQL, validates it, updates player points, and returns only the boolean result.
* **Result**: Cheat-proof API design. Clients only see options and prompts; the answers never travel over the network prior to submission evaluation.

### Technical Deep Dive
* **Static Question Entity**:
  ```go
  type Question struct {
      ID            string   `db:"id"`
      Prompt        string   `db:"prompt"`
      Options       []string `db:"options"`
      CorrectAnswer string   `db:"correct_answer"` // Confined to backend
      Explanation   string   `db:"explanation"`    // Confined to backend
  }
  ```
* **Sanitized Client DTO**:
  ```go
  type SanitizedQuestionResponse struct {
      ID           string   `json:"id"`
      Prompt       string   `json:"prompt"`
      Options      []string `json:"options"` // Lacks answer/explanation
  }
  ```

### Key Engineering Lesson
**Never trust the client, and never send data you don't want exposed.** Client-side validation or filtering is a security vulnerability. True security requires server-side validation using completely sanitized data models.

### 30-Second Verbal Pitch
> "To prevent cheating on our competitive coding platform, I implemented a zero-trust API architecture. Instead of exposing standard question objects, the backend utilizes a Sanitized DTO that completely strips the correct answer and explanation before sending it over the network. When a user submits their answer, the validation is performed entirely on the backend using database records. This guarantees that a player cannot scrape the answers from network logs or browser state, maintaining match integrity."

---

## 6. Seeder Idempotency Bug (Data Migrations)

### Interview Question
> *"Tell me about a time a minor assumption in database seeding or migrations broke environment reliability."*

### STAR Narrative
* **Situation**: To import questions into the database, we built a JSON file seeder. To prevent duplicate records during re-deployments, we initially implemented an upsert using `ON CONFLICT (title) DO UPDATE`.
* **Task**: During testing, we realized this seeding strategy was flawed: question titles are not guaranteed to be unique. For instance, two different questions might be titled "Binary Search Complexity" but feature completely different code prompts.
* **Action**: Adding a unique constraint to `title` would break the database model. I refactored the seeding pipeline:
  1. Every question in the seed JSON file was assigned a predefined, stable UUID string as its `"id"`.
  2. I updated the seeder's database query to resolve conflicts on the primary key `id` (`ON CONFLICT (id) DO UPDATE`).
  3. I removed the redundant unique constraint from the schema migration files.
* **Result**: We achieved 100% idempotent seeding without compromising database constraints. Seed files can be updated and run repeatedly across local, staging, and production environments.

### Technical Deep Dive
* **Upsert implementation**:
  ```sql
  INSERT INTO questions (id, question_type, difficulty, title, prompt, options, correct_answer, explanation, time_limit_sec, tags, source, is_active)
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, true)
  ON CONFLICT (id) DO UPDATE -- Confirms conflict against PK UUID
  SET question_type = EXCLUDED.question_type,
      difficulty = EXCLUDED.difficulty,
      title = EXCLUDED.title,
      prompt = EXCLUDED.prompt,
      options = EXCLUDED.options,
      correct_answer = EXCLUDED.correct_answer,
      explanation = EXCLUDED.explanation,
      time_limit_sec = EXCLUDED.time_limit_sec,
      tags = EXCLUDED.tags,
      source = EXCLUDED.source,
      updated_at = NOW();
  ```

### Key Engineering Lesson
**Seed data must have stable, deterministic identity keys.** Relying on descriptive attributes (like names or titles) for uniqueness is a common trap that limits schema flexibility and introduces data-corruption risks.

### 30-Second Verbal Pitch
> "When building our question database seeder, we initially upserted records by matching on question titles. However, we quickly realized that titles aren't unique—multiple questions can share a title while having different prompts. If we added a unique constraint on titles, it would break our schema design. I solved this by assigning stable, hardcoded UUIDs to each question in our seed file and shifting the conflict resolution to the primary key ID. This achieved idempotent, clean database seeding without compromising schema constraints."

---

## 7. AI-Assisted Development Challenge (Engineering Process)

### Interview Question
> *"How do you use AI tools in your development workflow, and what are the limitations you've encountered?"*

### STAR Narrative
* **Situation**: During the initial phases of building the Questions module, we used an AI coding assistant to quickly generate boilerplate models, repositories, and services. The AI produced clean Go code at high speed.
* **Task**: While the individual files compiled and worked in isolation, we soon realized the AI had made major structural design errors. It had merged progression tracking, scoring logic, and question databases into a single monolithic Questions module, violating basic domain boundaries.
* **Action**: I paused the code generation and refactored the design. I separated the database transactions, created clean interfaces for the Questions module, shifted progress states to the Battle module, and established strict row-locking patterns to prevent race conditions.
* **Result**: The refactoring corrected the architectural flaws. I learned that AI is an excellent code completion assistant but a poor architect. 
* **Key Takeaway**: High-velocity code generation without deep code review and structural oversight leads to technical debt. The primary developer must serve as the primary architect.

### Technical Deep Dive
* **AI Output**: A coupled system where `questions.Service` directly queried `battle_players` and updated score increments, blending read-heavy static tables with transaction-heavy battle player states.
* **Human Refactoring**: Introduced `QuestionsRepository` and `BattleRepository` interfaces. Isolated PostgreSQL `FOR UPDATE` transaction locks to `battle.Service` and exposed `ValidateAnswer` as a stateless utility.

### Key Engineering Lesson
**Velocity is not productivity.** AI can generate hundreds of lines of code in seconds, but without architectural judgment and code review, it can lead to architectural rot and coupled modules.

### 30-Second Verbal Pitch
> "We used AI tools to generate the scaffolding for our database models and services, which accelerated initial setup. However, the generated code had architectural flaws, coupling progression writes with static question reads. I stepped back, drew clear domain boundaries, and refactored the code to isolate database transactions and row-locking. This taught me that while AI is a powerful code-completion accelerator, the developer must remain the active architect, focusing heavily on boundary isolation and concurrency limits."
