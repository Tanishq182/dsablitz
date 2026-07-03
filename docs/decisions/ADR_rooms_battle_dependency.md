# Architectural Decision Record (ADR): Rooms-Battle Dependency Inversion

## Status
Approved

## Context
The Rooms module acts as the orchestrator of matchmaking lobbies. When a lobby becomes `ready`, the room host can start the battle. This action requires initializing a battle record, generating a question sequence, and preparing player progression pointers. 

If the Rooms module directly imports and depends on `battle.Service` (the concrete implementation of the Battle module), it introduces several architectural issues:
1. **Tight Coupling**: Any changes to `battle.Service`'s structure or dependency tree directly affect the Rooms module.
2. **Circular Thinking**: If the Battle module later needs to query Room details, we risk circular package dependencies in Go.
3. **Harder Testing**: To unit test the Room lobby state machine, we would have to instantiate or mock the entire concrete `battle.Service` and its dependencies (like the database pool and question bank service).
4. **Separation of Concerns**: In the future, if the system is split into microservices, the Battle service will run independently. Directly depending on local Go structs makes this future migration expensive.

## Decision
We will apply the **Dependency Inversion Principle (DIP)**. The Rooms module will define and own a high-level abstraction (interface) for launching battles, rather than depending on a concrete implementation:

```go
package rooms

import (
	"context"
	"github.com/google/uuid"
)

// BattlePlayer represents a participant detail passed during battle initialization.
type BattlePlayer struct {
	UserID       uuid.UUID
	SeatNumber   int16
	RatingBefore int
}

// BattleStarter defines the dependency interface owned by the Rooms module
// to initialize battles, decoupling it from the concrete battle module.
type BattleStarter interface {
	StartBattle(ctx context.Context, roomID uuid.UUID, players []BattlePlayer, seed int64) (uuid.UUID, error)
}
```

During dependency wiring (in [routes.go](file:///home/tanishq/dsablitz/backend/internal/server/routes.go)), the concrete `battle.Service` (which already implements a compatible method signature) will be injected as the `BattleStarter` into the `rooms.Service` constructor.

## Alternatives Considered

### Alternative 1: Concrete `battle.Service` Dependency
* **Description**: `rooms.Service` takes a direct reference to `*battle.Service`.
* **Why Rejected**: Creates tight coupling and makes unit testing the room lifecycle dependent on the entire question-seeding and battle-repository ecosystem.

### Alternative 2: Event-Driven Battle Initiation (Pub/Sub)
* **Description**: Rooms module publishes a `LobbyReadyEvent` or `StartBattleRequested` event over a message channel (e.g. Redis Pub/Sub, Go channels). The Battle module listens to this event, creates the battle, and publishes a `BattleStartedEvent` back to the Rooms module.
* **Why Rejected**: While highly decoupled, it introduces significant asynchronous complexity (handling out-of-order messages, intermediate network retries, complex distributed transactions). For our MVP modular monolith, a synchronous interface call is cleaner and easier to reason about.

## Trade-offs

### Pros
* **Modular Decoupling**: Rooms and Battle packages compile independently. Changes to the Battle repository or internal gameplay engine do not require changes to Rooms.
* **Trivial Testing**: We can write a simple, stateless mock implementation of `BattleStarter` in our unit tests to verify room transitions without needing database connections or question banks.
* **Microservice Readiness**: If the Battle module is extracted to a separate service in the future, the Rooms module's code remains unchanged; we only replace the local interface implementation with a gRPC/HTTP client wrapper.

### Cons
* **Minor Boilerplate**: Requires mapping data structures (e.g. converting `rooms.RoomPlayer` details to `rooms.BattlePlayer` before passing them to the interface).

## Interview Discussion

* **Question**: *Why did you define the interface in the Rooms package instead of the Battle package?*
* **Answer**: In clean architecture, the client package (Rooms) defines the abstraction it *needs* to consume, rather than the supplier package (Battle) defining what it *provides*. This is the core of Dependency Inversion: ownership of the abstraction belongs to the module using it, preventing the compiler and design flow from being driven by low-level details.
