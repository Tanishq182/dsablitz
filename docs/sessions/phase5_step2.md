# Session Notes: Phase 5 Step 2 (Questions Module MVP)

## What Was Implemented
* Created the stateless Questions module containing the Go models, repository pgx scanners, service logic, DTO mappings, and JSON seeder.
* Established PostgreSQL migrations for the shared sequence tables and player progress pointers.
* Wrote 50 seed questions with stable UUIDs, integer numerical answers, and clear ordering steps.
* Implemented answer validation utilizing type-safe `SubmissionAnswer` models and epsilon tolerance rules.
* Offloaded database read traffic during active matches by introducing a thread-safe `sync.RWMutex` cache in Questions Service.

## Files Changed/Created
1. **Migrations**: [backend/migrations/000003_add_battle_sequence_and_progression.up.sql](file:///home/tanishq/dsablitz/backend/migrations/000003_add_battle_sequence_and_progression.up.sql) / `.down.sql`
2. **Seeds**: [backend/internal/questions/seeds/questions.json](file:///home/tanishq/dsablitz/backend/internal/questions/seeds/questions.json)
3. **Core Code**:
   - [backend/internal/questions/models.go](file:///home/tanishq/dsablitz/backend/internal/questions/models.go)
   - [backend/internal/questions/validation.go](file:///home/tanishq/dsablitz/backend/internal/questions/validation.go)
   - [backend/internal/questions/repository.go](file:///home/tanishq/dsablitz/backend/internal/questions/repository.go)
   - [backend/internal/questions/service.go](file:///home/tanishq/dsablitz/backend/internal/questions/service.go)
   - [backend/internal/questions/seeder.go](file:///home/tanishq/dsablitz/backend/internal/questions/seeder.go)
   - [backend/internal/questions/routes.go](file:///home/tanishq/dsablitz/backend/internal/questions/routes.go)
4. **Tests**:
   - [backend/internal/questions/validation_test.go](file:///home/tanishq/dsablitz/backend/internal/questions/validation_test.go)
   - [backend/internal/questions/service_test.go](file:///home/tanishq/dsablitz/backend/internal/questions/service_test.go)
5. **Documentation**:
   - [docs/flows/questions_runtime_flow.md](file:///home/tanishq/dsablitz/docs/flows/questions_runtime_flow.md)
   - [docs/deep-dives/cache_design.md](file:///home/tanishq/dsablitz/docs/deep-dives/cache_design.md)
   - [docs/interview-notes/questions_step2.md](file:///home/tanishq/dsablitz/docs/interview-notes/questions_step2.md)

## Architectural Decisions & Refactors
* **Module Boundary Separation**: Shifted submissions, progress pointers, scoring, and sequence generation out of the Questions module. The Questions module is 100% stateless.
* **Domain Invariant Isolation**: Restrained `Question.Validate()` to structural invariants (UUID, ranges, types), moving ingestion policies (MCQ option counts, content matching) into `seeder.go` and `validation.go`.
* **Zero DB-Read Caching**: Built a global read-only in-memory questions cache to eliminate database lookups during submissions.

## Technical Debt & Future Improvements
* **Redis Pub/Sub Invalidation**: Currently, the cache is initialized only at startup. In V2, once Admin CRUD APIs are added, we must implement a Redis Pub/Sub invalidation topic to synchronize cache state across multiple horizontally scaled backend nodes.
* **Dedicated Cache Refactoring**: Currently, the cache map and mutex reside directly inside `QuestionsService`. In the next phase, we can extract this into a dedicated `QuestionsCache` helper class to satisfy the Single Responsibility Principle.
