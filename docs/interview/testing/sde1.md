# Testing Strategies - SDE1 Level Interview Prep

This guide focuses on service-layer integration testing, mock repository design, testing complex state machines (such as question progression rules), and defining clear interface boundaries.

---

## Q&A Sets

### Q1: How do you design an in-memory mock repository to test complex service-layer logic without requiring a physical database connection?

#### Interviewer Intent
Assess the candidate's understanding of interface segregation, in-memory state tracking, and how to write fast, isolated integration tests for core business services.

#### Strong Answer
To test the business logic of our services (like start battle, submit answers, and complete match) without the latency and setup complexity of a database, we implement a **Mock Repository** that matches the repository interface.

Instead of calling database connections, this mock uses local, thread-safe (or single-threaded test) data structures like maps, structs, and slices to simulate table rows:
1. **Define the Interface**: The service layer defines the exact database operations it requires. For example, `BattleRepository` specifies `InsertBattle`, `GetBattlePlayer`, `UpdateBattlePlayer`, etc.
2. **Implement in Test**: In the test file, we create a struct `mockBattleRepository` that holds in-memory maps corresponding to database tables:
   ```go
   type mockBattleRepository struct {
       battle     Battle
       players    map[uuid.UUID]BattlePlayer
       sequence   []uuid.UUID
       submissions map[string][]SubmissionAnswer
   }
   ```
3. **Simulate DB Queries**: Mock methods mimic database actions. `InsertBattlePlayers` iterates over input slices and stores them in the in-memory `players` map. `GetBattlePlayer` retrieves from the map and returns a cloned object (or value type) to mimic isolation, returning a standard `ErrNotFound` if the key is missing.

This enables tests to run in microseconds, guarantees that tests have no cross-contamination, and allows asserting that service mutations are correctly applied to the repository state.

#### Common Mistakes
* Spawning a real PostgreSQL Docker container (e.g., testcontainers) for unit tests, making the test suite slow (seconds instead of microseconds) and dependent on external container engines.
* Hardcoding static mock return values (e.g., "always return ID 1") instead of writing a stateful mock that accurately updates and retrieves values dynamically.
* Modifying production repository structures just to make mocking easier, rather than using clean interface abstractions.

#### Follow-up Questions
* How do you ensure your mock repository behaves exactly like the real PostgreSQL repository (e.g., constraint checks)?
* How would you test concurrent access to the mock repository? (By introducing sync primitives like `sync.Mutex` or `sync.RWMutex` inside the mock).
* Should repository interfaces be defined in the repository package or the service package? (Service package, enforcing **Dependency Inversion** so the service defines what it needs).

#### How DSAblitz demonstrates this concept
In `backend/internal/battle/service_test.go`, the `mockBattleRepository` struct implements all `BattleRepository` methods, tracking players, sequences, and submissions using map and slice properties in memory.

#### Relevant code references
* [service.go:L50-L70](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L50-L70) - `BattleRepository` interface definition inside the service layer.
* [service_test.go:L59-L81](file:///home/tanishq/dsablitz/backend/internal/battle/service_test.go#L59-L81) - `mockBattleRepository` struct declaration.
* [service_test.go:L115-L138](file:///home/tanishq/dsablitz/backend/internal/battle/service_test.go#L115-L138) - In-memory implementation of player lookup methods.

#### Related documentation
* [diagrams/repository_flow.md](file:///home/tanishq/dsablitz/docs/diagrams/repository_flow.md)

---

### Q2: How do you verify and test complex state machine rules (like game progression or attempt limits) using mock services?

#### Interviewer Intent
Verify the candidate's ability to translate complex business specifications (like game progression rules) into robust test scenarios, validating state transitions, attempts, and edge cases.

#### Strong Answer
Testing complex business state transitions (like DSAblitz's "Option C" progression policy) requires a structured, multi-step test case that asserts state properties at each transition boundary.

For example, the Option C gameplay rule states: *Players get a maximum of 2 attempts per question. Answering correctly advances to the next question. Failing twice skips the question with 0 points.*

To test this:
1. **Arrange**: Set up the `mockBattleRepository` with a battle in the `active` status, initialize a player at `current_question_index = 0` and `attempts = 0`, and load a sequence of mock questions. Mock the validation service to control when answer validation returns `true` or `false`.
2. **Act & Assert Step 1 (Incorrect Attempt 1)**: Mock the answer validator to return `false`. Call `SubmitAnswer`. Assert that:
   * The result returned is `IsCorrect = false`.
   * The player's `CurrentQuestionIndex` remains `0`.
   * The player's `CurrentQuestionAttempts` increments to `1`.
3. **Act & Assert Step 2 (Incorrect Attempt 2 - Trigger Skip)**: Submit another incorrect answer. Assert that:
   * The result returned is `IsCorrect = false`.
   * The player's `CurrentQuestionIndex` advances to `1` (skipped).
   * The player's `CurrentQuestionAttempts` resets to `0`.
   * The player's `Score` remains `0`.
4. **Act & Assert Step 3 (Correct Attempt on Next Question)**: Mock the validator to return `true`. Submit the answer. Assert that:
   * The result returned is `IsCorrect = true`.
   * The player's `CurrentQuestionIndex` advances to `2`.
   * The player's `Score` increases to `1`.

This step-by-step state verification ensures that attempts are counted, reset rules are respected, and index increments occur exactly as specified.

#### Common Mistakes
* Testing only the "correct answer" path and neglecting incorrect paths, skips, or out-of-order index boundaries.
* Storing mutable state in the test suite itself instead of relying on the service-under-test modifying the injected mock.
* Asserting only output values without validating that intermediate states (e.g. database rows) were updated.

#### Follow-up Questions
* What is the duplicate submission index validation, and how do we test it?
* How does the `ScoreCalculator` interface enable testing different scoring policies (like speed-based or streak-based scoring) independently from the state machine?
* What happens if a user submits an answer to a question they have already passed or skipped?

#### How DSAblitz demonstrates this concept
DSAblitz verifies the Option C progression rules inside `TestBattleService_SubmitAnswer_OptionCPolicy` in `service_test.go`. The test walks a mock player through failure, skip, and success transitions, checking index, attempts, and score values at each step.

#### Relevant code references
* [service.go:L259-L292](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L259-L292) - Option C progression logic inside `SubmitAnswer`.
* [service_test.go:L277-L358](file:///home/tanishq/dsablitz/backend/internal/battle/service_test.go#L277-L358) - Full integration test for Option C progression, attempts count, and skip mechanics.

#### Related documentation
* [deep-dives/submission_lifecycle.md](file:///home/tanishq/dsablitz/docs/deep-dives/submission_lifecycle.md)
* [flows/submission_flow.md](file:///home/tanishq/dsablitz/docs/flows/submission_flow.md)

---

## Key Takeaways
* **Interface Segregation**: Defining repository interfaces in the service package decouples the business logic from data storage, making mocking natural and simple.
* **Stateful Mocking**: Mocks should store internal state using maps or slices rather than returning static dummy values, allowing multi-step state verification.
* **Step-by-Step State Assertions**: Tests for game state machines must assert intermediate counters (attempts, indices, scores) after every trigger to verify transition accuracy.

## Interview Questions
1. Why is it beneficial to define database repository interfaces in the business/service layer instead of the database layer?
2. How would you test that a player cannot submit duplicate answers to the same question in a battle?
3. How do in-memory mocks improve CI/CD test execution times compared to container-based databases?

## Common Mistakes
* Relying on `interface{}` to bypass type checks in mocks, reducing compile-time safety.
* Not asserting that transaction functions were actually executed by the mock repository.
* Over-complicating mocks by re-implementing database query features (like sorting or joins) inside the Go mock code.

## Related Documents
* [deep-dives/submission_lifecycle.md](file:///home/tanishq/dsablitz/docs/deep-dives/submission_lifecycle.md)
* [flows/submission_flow.md](file:///home/tanishq/dsablitz/docs/flows/submission_flow.md)
* [architecture/module_boundaries.md](file:///home/tanishq/dsablitz/docs/architecture/module_boundaries.md)

## Lessons Learned
* Writing robust unit tests requires designing clean module boundaries and interface signatures from day one.
* Integration testing should focus on validating domain invariants and state progression rules under both correct and erroneous inputs.
