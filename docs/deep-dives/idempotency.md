# Deep Dive: Idempotency in Distributed Systems

This document details strategies for handling duplicate requests, network retries, and transaction consistency inside real-time competitive gaming systems like **DSAblitz**.

---

## 1. The Challenge of Duplication

In distributed network systems, failures are inevitable. A client submits an answer to a question, but the connection drops before the client receives the server response.
* **The Retry Storm**: The client app automatically retries the submission.
* **The Double-Score Risk**: Without idempotency, the server might process the request twice, incrementing the player's score and question index twice for a single correct solution.

---

## 2. Standard Idempotency Strategies

### **A. Unique Request Keys (Idempotency Keys)**
* **Mechanism**: The client attaches a unique, UUIDv4 transaction key (e.g. `X-Idempotency-Key`) to the header of every submission request.
* **Server Logic**: The server attempts to store the key in Redis with a short TTL (e.g., 5 minutes) using `SETNX`. If the key already exists, the server returns the cached response of the previous execution.
* **Pros**: Standard, works for generic API endpoints.
* **Cons**: Adds a Redis dependency to the validation hot-path.

### **B. Deterministic State Progression (Chosen for DSAblitz)**
* **Mechanism**: Instead of abstract keys, use the player's current progression index inside the game state as a natural lock.
* **Server Logic**: The client submits the answer along with the index of the question they are attempting (e.g., `SubmissionAnswer{QuestionIndex: 3}`).
* **Repository Validation**: Inside the locked database transaction, the server reads the player's current progression index. If `db_index != client_submitted_index`, the request is rejected as a duplicate or stale retry.
* **Result**: Atomic, zero-dependency safety. Stale retries are filtered out at the SQL boundary.

---

## 3. Implementation in Future Phases

### **WebSocket Frame Sequence Validation**
For WebSocket real-time matches:
1. Every client submission frame includes a sequential ID (`frame_seq_id`).
2. The Room Controller tracks the latest processed `frame_seq_id` for each connection in Redis presence.
3. If an incoming WebSocket frame arrives with a sequence ID less than or equal to the recorded value, it is discarded immediately.

### **Database Dedup Constraints**
Our submissions audit table enforces a unique key constraint on the active game loop tuple:
```sql
ALTER TABLE submissions 
ADD CONSTRAINT unique_player_question_submission 
UNIQUE (battle_id, user_id, question_id);
```
*If a network retry attempts to insert a duplicate submission for a question the player has already answered, the database transaction fails and rolls back, ensuring absolute integrity.*
