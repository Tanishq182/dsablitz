# Go Concurrency & RWMutex Cache - SDE1 Level

This document provides production engineering interview preparation material on Go concurrency, reader-writer locks, lock starvation, and the Go memory model.

---

## Q&A Sets

### Q1: How does `sync.RWMutex` prevent writer starvation in Go when there is a high frequency of incoming concurrent readers?

#### Interviewer Intent
The interviewer is looking for deep knowledge of standard library concurrency implementations:
- Understanding of the writer starvation problem (where a continuous stream of readers prevents a writer from ever acquiring the lock).
- How Go's scheduler and `sync.RWMutex` coordinate queue ordering and blocking state.
- Contrast between naïve reader-writer locks and Go's starvation-preventing implementation.

#### Strong Answer
In a naïve reader-writer lock implementation, if readers continuously acquire the lock, a writer might wait indefinitely because the active reader count never drops to zero (this is called **writer starvation**).

Go's `sync.RWMutex` resolves writer starvation using a two-state transition mechanism:
1. When a writer calls `Lock()`, it subtracts a large constant (`rwmutexMaxReaders = 1 << 30`) from the active reader count. This makes the reader count negative, signaling to any incoming readers that a write operation is pending.
2. Any new reader calling `RLock()` checks this count. If it is negative, the reader increments a pending readers count and blocks, sleeping on a runtime semaphore (`readerSem`).
3. Active readers already holding the lock continue executing. The last active reader releasing the lock (`RUnlock()`) detects the pending writer and releases the writer semaphore (`writerSem`), waking up the blocked writer.
4. Once the writer completes and calls `Unlock()`, it restores the reader count by adding `rwmutexMaxReaders` back, and wakes up all the blocked readers waiting on `readerSem`.

This ensures that once a writer requests the lock, all subsequent readers are queued behind the writer, bounding the latency of write operations.

```
Time -------------------------------------------------------------->
[Reader 1 (Active)] ===== RUnlock() ====> Wakes up Writer
[Reader 2 (Active)] ================ RUnlock() ====>
                     [Writer Calls Lock()] (Blocks new readers)
                     [Reader 3 (Blocks)] .................... [Resumes]
                                          [Writer executes] ===> Unlock()
```

#### Common Mistakes
- **Assuming Go uses a first-in-first-out (FIFO) queue for all locks**: While Go tries to maintain fairness, it is not strictly FIFO. Waking up goroutines uses runtime semaphores, which depend on the Go runtime scheduler.
- **Thinking RWMutex is always faster than Mutex**: `sync.RWMutex` actually has more internal accounting overhead (atomic operations on reader counts, tracking reader semaphores). Under low-concurrency or write-heavy workloads, `sync.Mutex` is faster.
- **Trying to upgrade a lock**: Go does not support lock upgrading (holding a read lock and then calling `Lock()` to make it a write lock). Doing so results in an immediate deadlock.

#### Follow-up Questions
1. What is the value of `rwmutexMaxReaders`? (It is $1 \ll 30$ in the Go standard library).
2. How does the Go runtime suspend goroutines waiting for a lock? (It uses semaphores via `runtime_Semacquire` and `runtime_Semrelease`).

#### How DSAblitz demonstrates this concept
In DSAblitz, the questions service in-memory cache is read-heavy. During startup or admin reload calls, the cache must be updated. By utilizing `sync.RWMutex`, incoming reader requests (e.g. from active battles retrieving questions) will temporarily block behind a cache reload operation, guaranteeing that stale or partially written state is never read by players.

#### Relevant code references
- `[service.go:L38-L45](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L38-L45)`: `LoadCache` calling `s.mu.Lock()` to reload the cache, blocking new readers.
- `[service.go:L50-L53](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L50-L53)`: `GetQuestionByID` calling `s.mu.RLock()` which will block if a write is pending.

#### Related documentation
- [Cache Design](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)
- [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)

---

### Q2: Why is it dangerous to read or write a standard package variable/pointer across goroutines without sync primitives, even if you do not care about getting slightly stale data?

#### Interviewer Intent
The interviewer wants to test your understanding of:
- The Go Memory Model and its rules for visibility.
- Compiler optimizations (register allocation, instruction reordering).
- Processor-level cache coherence and memory fences.
- CPU/Memory hardware behavior.

#### Strong Answer
Many developers believe that accessing a shared variable without locks is safe if they only care about "eventual consistency" or are willing to tolerate slightly stale data. This is a dangerous assumption in Go due to three main factors:

1. **Undefined Behavior & Data Races**: The Go memory model specifies that if a write to a memory location is concurrent with another read or write of the same location, the behavior is a **data race** and is undefined. This can result in reading garbage memory or partial/torn writes.
2. **Compiler Optimizations (Register Allocation)**: The compiler may optimize your code by storing a variable in a CPU register rather than querying main memory/cache on every iteration. For example, a loop checking `while !stop { ... }` where `stop` is modified in another goroutine can run forever because the reading goroutine cached the value in a register. Without synchronization primitives (or channels/atomics), the compiler has no indication that the variable is shared and is free to optimize away memory checks.
3. **Instruction Reordering & Cache Incoherency**: Both the Go compiler and the hardware CPU can reorder memory read/write instructions to maximize pipeline efficiency. If a struct pointer is updated concurrently, a goroutine may read the new pointer address *before* the CPU cache has propagated the actual fields of the struct, leading to a nil pointer dereference or corrupted reads.

Synchronization primitives like `sync.RWMutex` establish **happens-before** relationships. A `sync.RWMutex.Unlock()` happens-before a subsequent `sync.RWMutex.RLock()` finishes, guaranteeing that all memory modifications made prior to the unlock are visible to the next reader.

#### Common Mistakes
- **Assuming standard types (int, bool, float) are atomic**: In Go, no type is guaranteed to be atomic. On 32-bit architectures, writing a 64-bit value requires two operations. Without lock protection, a reader can read half of the old value and half of the new value (torn reads).
- **Thinking `volatile` exists in Go**: Go does not have a `volatile` keyword. Only synchronization primitives establish visibility.
- **Relying on sleep**: Inserting `time.Sleep(...)` does not establish a happens-before relationship and does not guarantee memory visibility.

#### Follow-up Questions
1. What is a "happens-before" relationship in the Go memory model?
2. What tool does Go provide to find these bugs? (The Go race detector via `go test -race` or `go run -race`).

#### How DSAblitz demonstrates this concept
In DSAblitz, players request questions concurrently. The question cache contains nested pointers (such as slices of options or tags). Reading these slices while a cache refresh is occurring without locking would result in memory corruption or data races, which is why the cache read operations are bound by `s.mu.RLock()` and `s.mu.RUnlock()`.

#### Relevant code references
- `[service.go:L50-L55](file:///home/tanishq/dsablitz/backend/internal/questions/service.go#L50-L55)`: `GetQuestionByID` reading the cache map and returning the Question struct safely wrapped in RLock.

#### Related documentation
- [Cache Design](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)
- [Database Indexing](file:///home/tanishq/dsablitz/docs/database/indexing.md)

---

## Key Takeaways
- `sync.RWMutex` actively prevents writer starvation by making the active reader count negative, routing new readers to block on a semaphore until the writer completes.
- Go compiler and CPU architectures are allowed to reorder memory writes and cache values in registers. Synchronization primitives are necessary to establish a memory barrier.
- Never write lock-free code using standard Go types; always use `sync` packages or `sync/atomic`.

## Interview Questions
1. How does Go's memory model define a data race?
2. How does the compiler optimize loops that access shared variables without synchronization?

## Common Mistakes
- Relying on raw pointers or primitive types for inter-goroutine signaling without synchronization.
- Assuming reader-writer locks are always superior to standard mutual exclusion locks.

## Related Documents
- [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md)
- [Cache Design Deep Dive](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)

## Lessons Learned
- Incorporate the Go race detector (`-race`) in CI pipelines to automatically catch synchronization issues before deployment.
