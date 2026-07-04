# Gin Web Framework - SDE1 Level

This document provides production-level interview preparation material on the Gin web framework, focusing on context lifetimes, safety rules for asynchronous execution, and standard practices for error collection and response generation.

---

## Q&A Sets

### Q1: Why is it unsafe to pass a raw `*gin.Context` to a background goroutine, and how do we safely handle background operations in Gin?

#### Interviewer Intent
The interviewer wants to test your understanding of:
- The lifecycle of request-scoped objects (specifically `gin.Context`).
- Concurrency and memory recycling inside web frameworks.
- Safe asynchronous execution patterns in Go server applications.

#### Strong Answer
In Gin, the `*gin.Context` object is **not thread-safe** and has a lifetime tied directly to the HTTP request-response cycle. To optimize memory allocations and minimize garbage collection overhead under high load, Gin uses a **sync.Pool** to recycle `gin.Context` instances.
- Once an HTTP request handler returns, Gin resets the context data and returns the `*gin.Context` instance to the pool to be reused by another incoming request.
- If you spin up a background goroutine and pass the raw `*gin.Context` to it, the background goroutine will be reading and writing to a recycled context that may already be reassigned to a completely different incoming HTTP request. This leads to severe data races, security leaks (e.g. sharing user IDs across requests), and runtime panics.

To perform background work safely:
1. **Never pass the raw `*gin.Context` to a goroutine**.
2. If you need read-only metadata (like request keys) in the background, call **`ctx.Copy()`**. This returns a read-only, shallow copy of the context metadata (including keys) that is safe to pass because it is not recycled by the pool.
3. If you need to perform database operations or network calls in the background, extract the standard library context using **`ctx.Request.Context()`** (or create a new context with a timeout) and pass that, rather than the `gin.Context`.

```go
// BAD: Data race and panic potential
go func() {
    userID, _ := ctx.Get("auth.user_id") // CRITICAL BUG: ctx is recycled!
    log.Println("User left:", userID)
}()

// GOOD: Safe shallow copy
ctxCopy := ctx.Copy()
go func() {
    userID, _ := ctxCopy.Get("auth.user_id") // Safe read-only copy
    log.Println("User left:", userID)
}()
```

#### Common Mistakes
- **Writing responses from background goroutines**: Attempting to call `ctx.JSON` or `ctx.Status` inside a background goroutine after the main handler has returned. The HTTP connection is already closed, resulting in writes to a closed socket or memory corruption.
- **Leaking transaction contexts**: Passing a transaction handle (`pgx.Tx`) stored in the context to a background goroutine. The transaction will be committed or rolled back by the main request thread, leaving the background thread with an aborted connection.
- **Forgetting context cancellation**: Background routines should monitor context cancellation (`ctx.Done()`) to avoid orphaned work.

#### Follow-up Questions
1. How does `sync.Pool` improve memory performance in Go?
2. What values are copied when you call `ctx.Copy()`? (It copies the context keys map, but not the request or writer streams).

#### How DSAblitz demonstrates this concept
In DSAblitz, asynchronous tasks like game state cleanup (e.g. `ExpireRooms`) are executed outside the request lifecycle. Web handlers (like `LeaveRoom`) complete synchronously and commit database transactions before returning, avoiding asynchronous execution races on active request contexts.

#### Relevant code references
- `[routes.go:L123-L143](file:///home/tanishq/dsablitz/backend/internal/rooms/routes.go#L123-L143)`: The `LeaveRoom` handler, executing all database actions synchronously on the request context `ctx.Request.Context()`.
- `[routes.go:L171-L181](file:///home/tanishq/dsablitz/backend/internal/rooms/routes.go#L171-L181)`: Resolving context keys synchronously within the handler thread before routing execution.

#### Related documentation
- [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
- [Project Context](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)

---

### Q2: How can we implement centralized error handling in Gin using `ctx.Error()` instead of writing immediate error responses in every handler?

#### Interviewer Intent
The interviewer wants to see:
- Your ability to write clean, maintainable web handlers.
- Understanding of the separation of concerns between business logic errors and HTTP responses.
- Knowledge of Gin's error collection array (`ctx.Errors`).

#### Strong Answer
In a standard Gin controller, writing error responses immediately (e.g., calling `ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})` followed by a `return`) leads to code duplication, inconsistent response formats, and lack of central observability (such as logging or metric tracking).

A cleaner, production-grade pattern uses **centralized error handling**:
1. Handlers do not write error responses directly. Instead, they attach the error to the context using **`ctx.Error(err)`** and return.
2. We register a custom **Error Handling Middleware** at the root router level.
3. This middleware runs downstream using `ctx.Next()`. Once the main handler finishes, the middleware inspects **`ctx.Errors`** (which is a slice of `*gin.Error` gathered during the request).
4. The middleware formats the error, maps custom application errors (e.g., validation, authorization, database failures) to appropriate HTTP status codes, logs the details, and returns a unified JSON payload to the client.

```go
// Centralized Error Handling Middleware Example
func ErrorHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next() // Execute downstream handlers

        // Check if errors were registered
        if len(c.Errors) > 0 {
            lastErr := c.Errors.Last().Err
            
            // Map custom errors to HTTP status codes
            status := http.StatusInternalServerError
            if errors.Is(lastErr, auth.ErrUnauthorized) {
                status = http.StatusUnauthorized
            } else if errors.Is(lastErr, rooms.ErrNotFound) {
                status = http.StatusNotFound
            }
            
            c.JSON(status, gin.H{"error": lastErr.Error()})
        }
    }
}
```

#### Common Mistakes
- **Forgetting to return after calling `ctx.Error(err)`**: Some developers attach the error to the context but forget to return from the handler, causing the handler code to continue executing and possibly panicking.
- **Double writing the response**: Registering an error via `ctx.Error(err)` *and* writing a JSON body in the handler. This results in the custom error middleware attempting to write to the response header after it has already been sent to the client (triggering a `http: superfluous response.WriteHeader` warning).
- **Exposing internal database errors**: Not filtering error types in the middleware, which leads to raw database errors (like SQL constraint violations) being returned directly to the client.

#### Follow-up Questions
1. How do you attach additional metadata (like log levels or status codes) to a `gin.Error`? (By using `err.SetType()` or `err.SetMeta()`).
2. Why is it important to execute `ctx.Next()` *before* handling the error in the middleware? (Because errors are gathered *during* the execution of downstream handlers; the middleware must wait for them to finish).

#### How DSAblitz demonstrates this concept
In DSAblitz, controllers use a helper function `writeError` to return standard JSON error schemas. Database errors like `ErrNotFound` are checked using standard library `errors.Is` to map database errors to client-safe HTTP statuses.

#### Relevant code references
- `[routes.go:L183-L185](file:///home/tanishq/dsablitz/backend/internal/rooms/routes.go#L183-L185)`: The `writeError` helper providing consistent JSON error formatting.
- `[routes.go:L75-L82](file:///home/tanishq/dsablitz/backend/internal/rooms/routes.go#L75-L82)`: Mapping repository errors (like `ErrNotFound`) to `http.StatusNotFound` vs `http.StatusBadRequest`.

#### Related documentation
- [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)
- [Project Context](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)

---

## Key Takeaways
- **`gin.Context` is recycled**: Passing it raw to a background goroutine leads to concurrency bugs and data races. Use `ctx.Copy()` for metadata or extract the underlying context via `ctx.Request.Context()`.
- **Centralized error handling** decouples HTTP translation from controller logic. Collect errors using `ctx.Error()` and parse them in a custom root-level middleware.
- Always check and map internal database errors to client-safe error messages.

## Interview Questions
1. Under what circumstances will a `*gin.Context` be garbage collected vs returned to a pool?
2. Explain how to write a Gin middleware that logs uncaught panic errors and returns a `500 Internal Server Error` to the client.

## Common Mistakes
- Attempting to read or write to `*gin.Context` inside asynchronous workers.
- Writing raw internal SQL error strings directly in HTTP response payloads.

## Related Documents
- [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)
- [Overall Architecture](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md)

## Lessons Learned
- Synchronizing request lifecycles and performing all DB mutations within the main handler thread avoids database connection leaks and race conditions.
