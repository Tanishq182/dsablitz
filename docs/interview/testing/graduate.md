# Testing Strategies - Graduate Level Interview Prep

This guide covers basic unit testing, test structures in Go, and interface mocking (such as clock or time providers) to make tests deterministic.

---

## Q&A Sets

### Q1: What is unit testing, and how does the Go standard library facilitate writing unit tests? Explain how to organize tests and write basic assertions.

#### Interviewer Intent
Assess the candidate's core knowledge of testing principles, familiarity with Go's built-in `testing` package, and ability to structure basic assertions.

#### Strong Answer
Unit testing is the practice of verifying that individual, isolated components of code (such as a single function or a struct method) behave correctly under various conditions, independent of external systems like databases or web services.

In Go, the standard library provides the `testing` package, and we write tests directly alongside production code:
1. **File Naming**: Test files are named with a `_test.go` suffix (e.g., `password_test.go` tests `password.go`).
2. **Function Signature**: Test functions must begin with `Test` followed by an uppercase letter and accept a single parameter: `t *testing.T` (e.g., `func TestHashPasswordAndVerifyPassword(t *testing.T)`).
3. **Assertions**: Go does not have built-in assertion keywords (like `assert` or `expect`). Instead, we write standard `if` statements and call methods on the `testing.T` struct:
   * `t.Errorf(...)`: Logs a failure message but allows the test function to continue running. Useful for non-fatal issues or iterating through table-driven test cases.
   * `t.Fatalf(...)`: Logs a failure and halts execution of the current test function immediately. Essential when a setup step fails, making further checks pointless.
4. **Execution**: We run all tests in a package by executing `go test ./...` in the terminal.

#### Common Mistakes
* Placing test files in a separate root directory (e.g., `tests/`), which breaks Go's package visibility conventions.
* Writing unit tests that make actual network calls or read local configuration files, making tests slow and flaky.
* Failing to test both positive (success paths) and negative (error paths) scenarios.

#### Follow-up Questions
* What is the difference between package-internal tests (e.g., `package auth`) and package-external tests (e.g., `package auth_test`)?
* How does the Go test runner handle parallel execution of tests? (Using `t.Parallel()`).
* What is the `-race` flag in `go test`, and why is it crucial for concurrent Go code?

#### How DSAblitz demonstrates this concept
DSAblitz uses Go's standard `testing` package to verify core cryptographic utility functions like password hashing and verification. The test verifies both a positive matching scenario and a negative mismatch scenario.

#### Relevant code references
* [password_test.go:L1-L27](file:///home/tanishq/dsablitz/backend/internal/auth/password_test.go#L1-L27) - Unit test verifying password hashing and verification logic.

#### Related documentation
* [checklists/documentation.md](file:///home/tanishq/dsablitz/docs/checklists/documentation.md)

---

### Q2: Why is mocking time or clocks necessary in unit testing, and how do we design an interface to make time-based logic deterministic in tests?

#### Interviewer Intent
Evaluate the candidate's understanding of test determinism, dependency inversion, and how to mock non-deterministic global states (like the current system time).

#### Strong Answer
Using the real system clock (`time.Now()`) in unit tests makes tests non-deterministic (flaky). Time-dependent tests (e.g., checking if a session token has expired, or calculating speed-based bonuses in a game) can fail randomly depending on execution speed, system resource utilization, or scheduling delays.

To make time-based code testable and deterministic, we apply the **Dependency Inversion Principle**:
1. We define a `Clock` interface that exposes a method returning the current time:
   ```go
   type Clock interface {
       Now() time.Time
   }
   ```
2. In production, we inject a concrete `RealClock` implementation that delegates to `time.Now()`:
   ```go
   type RealClock struct{}
   func (RealClock) Now() time.Time { return time.Now() }
   ```
3. In unit tests, we inject a stateful `mockClock` implementation. This mock holds a fixed timestamp in memory that we can manually advance or control:
   ```go
   type mockClock struct {
       now time.Time
   }
   func (m *mockClock) Now() time.Time { return m.now }
   ```

Using this pattern, we can precisely simulate time expiration (e.g., setting the clock to 1 second before expiration, verifying success, then advancing the clock by 2 seconds, and verifying failure) without running actual `time.Sleep` commands, keeping the test suite fast and reliable.

#### Common Mistakes
* Using `time.Sleep` in tests to wait for timers to fire, which slows down the build pipeline.
* Relying on `time.Now()` directly inside core business logic instead of passing a Clock dependency.
* Assuming that tests running on local machines will take exactly the same duration on CI environments.

#### Follow-up Questions
* What are the performance implications of replacing `time.Sleep` with a mocked clock?
* How does dynamic score calculation benefit from a mocked clock?
* Can we use the same clock mocking technique to test timers and timeouts in WebSockets?

#### How DSAblitz demonstrates this concept
DSAblitz declares a `Clock` interface in `backend/internal/battle/service.go`. The test file `backend/internal/battle/service_test.go` defines a `mockClock` that allows tests to manually control the current timestamp, enabling deterministic validation of expired battle states and submission timers.

#### Relevant code references
* [service.go:L16-L26](file:///home/tanishq/dsablitz/backend/internal/battle/service.go#L16-L26) - `Clock` interface and `RealClock` production implementation.
* [service_test.go:L15-L23](file:///home/tanishq/dsablitz/backend/internal/battle/service_test.go#L15-L23) - `mockClock` test implementation.
* [service_test.go:L415-L454](file:///home/tanishq/dsablitz/backend/internal/battle/service_test.go#L415-L454) - Test case simulating battle expiration by manually advancing `mockClock` time.

#### Related documentation
* [flows/battle_lifecycle.md](file:///home/tanishq/dsablitz/docs/flows/battle_lifecycle.md)
* [diagrams/battle_sequence.md](file:///home/tanishq/dsablitz/docs/diagrams/battle_sequence.md)

---

## Key Takeaways
* **Standard Library First**: Go's native `testing` package is the standard tool for writing test suites; assertions are constructed via standard control flow (`if` statements).
* **Table-Driven Tests**: Group multiple test parameters and expectations into slices to run subtests cleanly.
* **Deterministic Time**: Injecting a custom `Clock` interface allows tests to simulate token expiration and battle timing without using slow, non-deterministic `time.Sleep`.

## Interview Questions
1. How do you construct a basic unit test in Go, and how do `t.Errorf` and `t.Fatalf` differ?
2. What are the benefits of mocking time inside an API session validator or a game loop?
3. How do you run tests in Go, and how do you ensure that package-internal code is accessible to your tests?

## Common Mistakes
* Writing tests that depend on external resources, making the test suite flaky.
* Injecting static configurations that cause side effects across test cases.
* Using `time.Sleep` in tests, which slows down execution and introduces flakiness.

## Related Documents
* [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)
* [checklists/documentation.md](file:///home/tanishq/dsablitz/docs/checklists/documentation.md)

## Lessons Learned
* Always keep unit tests fast and side-effect free.
* Dependency injection via simple interfaces is the most effective way to test complex backend states like timers, clocks, and databases.
