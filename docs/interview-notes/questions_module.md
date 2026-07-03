# Interview Notes: Questions Module

This document outlines key technical discussions, possible interviewer questions, tradeoffs, scaling strategies, and alternative designs for the Questions module.

## 1. Possible Interviewer Questions & Answers

### Q: Why do we separate the `questions` and `question_stats` into two tables?
**A:** Separating static metadata (prompt, options, tags) from high-frequency dynamic metrics (times served, times answered, times correct) prevents write lock contention on the main `questions` table. Writes to `question_stats` occur dynamically during active matches when submissions are evaluated. Keeping `questions` read-heavy allows database engines to cache these rows efficiently.

### Q: How do we prevent cheating if the client is fetching questions?
**A:** We use **Payload Sanitization** and **Internal Validation**. We do not expose public query APIs (like `GET /api/v1/questions`) to players. Questions are served dynamically through active Battle endpoints (e.g., WebSockets). The backend strips `correct_answer` and `explanation` from the payload, sending only `SanitizedQuestionResponse` containing options and prompt details. The verification of the answer is performed entirely server-side.

### Q: Why use a shared global ordered stream of questions instead of letting players solve different questions or using fixed-N sets?
**A:**
1. **Fairness**: Both players face the exact same questions in the exact same sequence. No player gets luckier with "easier" questions.
2. **Speed Reward**: By using a time-bound format (5 or 10 minutes) with *unlimited* questions, players progress asynchronously. A faster player can solve 10 questions while a slower player solves 5, giving them a higher potential score.
3. **No Ceiling**: Unlike fixed-N sets (e.g., "first to solve 5"), the match runs for the full duration, allowing for high-intensity, continuous gameplay until the timer expires.

---

## 2. Tradeoffs & Alternative Designs

### File-Based Ingestion (JSON/YAML) vs. Admin CRUD APIs for MVP
* **Decision**: For the MVP, we use local JSON/YAML file-based seeding loaded at database startup or CLI command. Admin CRUD APIs are deferred to V2.
* **Tradeoff**: File-based ingestion is extremely simple and fast to implement, requiring zero HTTP routing, JWT admin checks, or admin UI screens. However, updating questions requires a code redeployment or running a migration script. Since the MVP has a static set of vetted DSA questions, this is a highly efficient tradeoff.

---

## 3. Scaling Discussions
### Question Stream Generation & Scaling
* **Decision**: For the MVP, PostgreSQL is the source of truth for battle metadata, the deterministic question sequence, submissions, and player progression.
* **Challenge**: When thousands of matches start simultaneously, generating dynamic streams from database queries (e.g. `ORDER BY RANDOM()`) will degrade DB performance.
* **Scaling Strategy**:
  1. Store the generated question stream (list of question IDs) in a structured/array column in the `battles` table in PostgreSQL.
  2. Cache the active question catalog (ID lists grouped by difficulty) in Redis (e.g., `questions:difficulty:3`) to avoid querying the `questions` table repeatedly when starting battles.
  3. The Battle engine shuffles and generates the sequence in-memory using a deterministic seed, then saves the generated array to PostgreSQL once.
  4. Subsequent player progress fetches retrieve the pre-generated stream array from PostgreSQL (or an read-through cache if needed), while player pointers are kept in a relational state.

---

## 4. Feature Extension: "How would you implement X?"

### Scenario A: How would you handle a player running out of questions in a match?
* **Implementation**: We can either:
  - Generate a very large stream initially (e.g., 100 questions, which is highly unlikely to be exhausted in 5-10 minutes) and persist it in PostgreSQL.
  - Loop the stream deterministically if a player reaches the end.
  - Dynamically append new questions to the stream in PostgreSQL using the same deterministic seed rules.

### Scenario B: How do you enforce the 5 or 10-minute match duration?
* **Implementation**: The room lifecycle is managed by the Battle Module. When a battle starts, an expiration timestamp is set in PostgreSQL (`end_time = start_time + duration`). The WebSocket server checks this timestamp against the current server time and rejects any submission received after `end_time`. A Redis-based timer or active scheduler triggers final room resolution and score calculation exactly at `end_time`.


