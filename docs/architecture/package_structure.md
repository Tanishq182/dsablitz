# Go Package & Directory Structure

This document answers the primary question: **How are directories laid out in the DSAblitz Go backend, why was this packaging structure chosen, and what are its tradeoffs?**

---

## 1. Directory Structure

The Go backend code matches the Standard Go Project Layout, separated into `cmd/` for CLI binary main entries and `internal/` for private module implementations:

```
backend/
├── cmd/
│   └── dsablitz/
│       └── main.go                  # System entrypoint (initializes DB, loads cache & runs server)
├── internal/
│   ├── auth/                        # JWT authentication mechanics and auth middleware
│   ├── battle/                      # Gameplay state engine, Option C rules, and submissions
│   ├── platform/
│   │   ├── cache/                   # Memory cache utilities
│   │   └── database/                # Database pool connection setups
│   ├── questions/                   # Stateless question loader, imports, and validation
│   ├── rooms/                       # Matchmaking rooms, capacity counters, and ready lobbies
│   ├── server/                      # Server startup, Gin routes, and adapter mappings
│   └── users/                       # User data storage
```

---

## 2. Package Packaging Rules

To maintain high legibility and code organization, each domain module inside `internal/` follows a strict structure:

* **`models.go`**: Declares package models, domain entities, database tags, and domain-level error sentinels (e.g. [models.go](file:///home/tanishq/dsablitz/backend/internal/battle/models.go)).
* **`repository.go`**: Handles SQL operations on PostgreSQL. No business logic is permitted here; it only executes database operations and maps rows to models (e.g. [repository.go](file:///home/tanishq/dsablitz/backend/internal/battle/repository.go)).
* **`service.go`**: Defines the module business service. Manages transactional execution boundaries, evaluates domain rules, and coordinates with external package adapters (e.g. [service.go](file:///home/tanishq/dsablitz/backend/internal/battle/service.go)).
* **`routes.go`**: Connects REST endpoints to Gin framework controllers and configures path middleware validation checks.

---

## 3. Prohibited Coding Conventions

1. **Global Package State**: Global variables (except configuration structures and immutable caches) are strictly banned. Everything must be passed via struct constructors (`NewService`, `NewRepository`).
2. **Raw Database Calls in Services**: Service files must never call `db.Exec` or query PostgreSQL directly. They must operate through repositories to preserve interface decoupling.
3. **No Direct Inter-Module Dependency**: Core domains must call other packages only through adapters or dependency interfaces.

---

## 4. Alternatives Considered & Rejected

### Why not flat structure (all Go files in root)?
* **Rejected**: A flat directory makes it very easy to introduce circular dependencies as the system grows. Separating packages allows the Go compiler to enforce compile-time acyclic graphs.

### Why not clean architecture (separate `domain/`, `usecases/`, `infrastructure/` directories)?
* **Rejected**: Clean Architecture introduces too many folders and file-to-file mappings (mappers, interfaces, entities, usecases) which increases code clutter for an MVP monolith. Package-by-feature (e.g. putting `models.go`, `repository.go`, and `service.go` in the same directory) is highly readable and cohesive.

---

## 5. Architectural Tradeoffs

### Pros
* **High Cohesion**: All code related to `battle` (SQL, models, services) is co-located in the same package directory, simplifying navigation.
* **Encapsulation**: Lowercase struct fields and helper methods are only visible within the package boundary, preventing accidental external mutations.

### Cons
* **Boilerplate**: Injecting dependencies through constructors (`NewService`) increases boilerplate code in `server/routes.go`.
* **Testing overhead**: Testing package-private structs requires writing test suites within the same package or exporting mock models.

### Limitations
* Domain models inside `models.go` contain database tags, coupling the domain representation slightly with the relational storage engine.

---

## Key Takeaways
1. Code is packaged by feature rather than layer.
2. The `internal/` folder prevents external applications from importing private module functions.

## Common Interview Questions
* **Why do you separate Go files into package-by-feature folders?**
  * *Answer*: To maximize cohesion (keeping SQL, services, and models close to each other) while allowing the Go compiler to enforce acyclic imports.
* **Why is `main.go` located in `cmd/dsablitz/` instead of the project root?**
  * *Answer*: It separates compile entry configurations (like CLI arguments, database initialization, and routing configurations) from the actual domain library code under `internal/`.

## Related Documents
* For structural blueprint layering, see [overall_architecture.md](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md).
* For package import hierarchies, see [dependency_graph.md](file:///home/tanishq/dsablitz/docs/architecture/dependency_graph.md).
