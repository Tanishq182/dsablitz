# ADR 002: Shared Question Stream with Asynchronous Progression

## Status
Accepted

## Context
In **DSAblitz**, players compete in real-time 1v1 battles. A key design goal is ensuring absolute fairness in matchmaking while rewarding speed and accuracy. 

Earlier designs considered using fixed-N question sets (e.g., first player to complete 5 questions wins) or sending completely randomized questions to each player. However:
1. Randomized questions introduce unfairness, as one player may receive significantly easier questions.
2. Fixed-N question sets limit game length, cap the maximum potential score, and do not fully leverage a time-bound competitive environment where players can solve as many questions as possible.

## Decision
We will implement a **Shared Question Stream with Asynchronous Progression**:

1. **Match Durations**: 
   - Battles will have a fixed duration of either **5 minutes** or **10 minutes**.
2. **Unlimited Questions**: 
   - There is no fixed limit to the number of questions in a match. Players can answer as many questions as they can before the timer expires.
3. **Shared Global Stream**: 
   - At battle initialization, a single ordered stream of question IDs is generated for the room based on the chosen difficulty/tags and a deterministic random seed.
   - Both players share this exact same stream; they will encounter the identical questions in the identical order.
4. **Asynchronous Progression**:
   - Players solve questions at their own pace. Their progress is tracked independently via a player-specific index pointer.
   - For example, Player A may be on question 8 (index 7) while Player B is on question 5 (index 4).
   - Upon submitting a valid answer (correct or incorrect), the player's pointer is incremented, and they receive the next question in the sequence.
5. **State Storage & Persistence**:
   - PostgreSQL is the source of truth for battle metadata, the deterministic question sequence, submissions, and player progression/pointers.
   - Redis is reserved strictly for ephemeral real-time state (like matchmaking, WebSocket presence/connections, and active timers), avoiding premature integration into the Questions module.

## Consequences
- **Fairness**: Absolute competitive integrity is maintained since both players face the exact same questions in the exact same sequence.
- **Speed Differentiation**: High-performing, fast players are directly rewarded as they can answer more questions within the 5/10 minute limit.
- **Data Persistence**: Storing progression pointers in PostgreSQL ensures battle history, stats, and submissions are fully durable, consistent, and auditable.
- **Room State Complexity**: The system must store and maintain separate progression state pointers per player in PostgreSQL rather than a single room-wide index.
- **Stream Generation Sizing**: The stream must be pre-populated with enough question IDs to ensure players do not run out of questions during a 10-minute sprint (e.g., seeding 100+ questions per battle room).
