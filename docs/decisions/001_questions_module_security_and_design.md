# ADR 001: Questions Module Design and Security Constraints

## Status
Accepted

## Context
In **DSAblitz**, players compete in real-time 1v1 battles by answering rapid-fire questions (MCQ, Complexity prediction, etc.). 
If players can access the `correct_answer` or `explanation` fields of a question through public APIs, they could easily write client-side scripts to fetch the answers and cheat during a battle.

DSAblitz is battle-first, not question-browser-first. We do not require a public question browser or general user CRUD interface. 

Additionally, for the MVP, we do not need complex web-based administration panels or Admin CRUD APIs to ingest or manage questions; these are deferred to V2.

## Decision
1. **Payload Separation**:
   - Introduce DTOs for representing questions:
     - `QuestionDetail`: Contains all fields, including `correct_answer` and `explanation` (used internally by ingestion and the battle evaluation service).
     - `SanitizedQuestionResponse`: Contains only fields necessary to display the question to players (ID, title, prompt, options, difficulty, question_type, time_limit_sec, tags). Omits `correct_answer` and `explanation`.
2. **Access Control & Battle-First Flow**:
   - General public endpoints for listing or browsing questions (e.g., `GET /api/v1/questions`) will not be exposed to general users. Questions are only served dynamically in the context of an active battle.
   - To check answers, the client will submit answers to the Battle module endpoint (e.g., via WebSocket or Battle submission HTTP POST), and the backend will do the validation against the database record, never exposing the answer to the client beforehand.
3. **File-Based Ingestion**:
   - For MVP, question ingestion is JSON/YAML file-based. Seed files will be loaded into the database during migrations or startup/seeding scripts.
   - Admin CRUD APIs for managing questions are deferred to V2.

## Consequences
- **Security**: Strong protection against API-scraping of answers during live matches.
- **Simplicity**: No need to implement and secure admin CRUD REST endpoints for questions in the MVP.
- **Inter-module Dependency**: The Battle module must reference the Questions repository/service internally to check answers, rather than requiring the client to pass the answer or retrieve it.

