# Domain Invariants vs. Room Business Rules

This document outlines the architectural separation between **Domain Invariants** (structural validations on models) and **Business Rules** (stateful workflows enforced by services) within the Rooms module of **DSAblitz**.

---

## 1. Domain Invariants

Domain Invariants are rules that define the structural integrity of an entity. An entity should never be allowed to exist in memory in an invalid state, regardless of the application context, state history, or user identity.

These checks are **stateless**, **self-contained**, and are defined directly in the entity model's `Validate()` method.

### **Room Invariants**
Validated in `Room.Validate()`:
* **UUID Validity**: Room ID must be a non-nil, valid UUID.
* **Room Code Format**: Room code must be alphanumeric, uppercase, and have a character length between 4 and 16.
* **Status Enum**: Status must be one of the defined values: `waiting`, `ready`, `in_battle`, `closed`, `expired`.
* **Host Validity**: Host User ID must be a non-nil, valid UUID.
* **Max Players**: The room capacity (`max_players`) must be exactly `2` (since the MVP only supports 1v1).
* **Duration Seconds**: The battle duration must be either `120` seconds (2 minutes) or `300` seconds (5 minutes).

### **RoomPlayer Invariants**
Validated in `RoomPlayer.Validate()`:
* **UUID Validity**: Player ID, Room ID, and User ID must all be non-nil, valid UUIDs.
* **Seat Number**: Seat number must be either `1` or `2`.
* **Status Enum**: Player status must be one of: `joined`, `ready`, `left`, `kicked`.

---

## 2. Room Business Rules

Business Rules are stateful constraints that govern how entities transition from one valid state to another based on operations, sequences, or user roles. These rules require contextual knowledge (such as the database state, transaction logs, or current authenticated user) and are enforced within the **Service Layer** under database transactions.

### **Examples of Room Business Rules**
* **Lobby Entry Restriction**: A player cannot join a room if the room is in `in_battle`, `closed`, or `expired` status.
* **Readiness Sequence Constraint**: A player cannot mark themselves as `ready` unless they are currently in `joined` status in that room.
* **Lobby Capacity Enforcment**: A player cannot join a room if there are already 2 active players (status `joined` or `ready`).
* **Game Launch Authority**: Only the host of the room (the player with `user_id == room.host_user_id`) is authorized to call the start-battle endpoint.
* **Battle Readiness Requirement**: A battle cannot be started unless the room status is `ready` (meaning both players are active and have toggled their status to `ready`).
* **Active Match Isolation**: While a room is `in_battle`, players cannot leave the lobby, and new players cannot join. (Disconnections do not remove players; they must submit abandonment actions to exit).

---

## 3. DSAblitz Architectural Mapping

Below is a summary of how we partition these rules in Go:

### **In the Entity Model (`internal/rooms/models.go`)**
```go
func (r *Room) Validate() error {
	if r.ID == uuid.Nil {
		return errors.New("room ID cannot be nil")
	}
	if len(r.Code) < 4 || len(r.Code) > 16 {
		return errors.New("room code must be between 4 and 16 characters")
	}
	if r.HostUserID == uuid.Nil {
		return errors.New("host user ID cannot be nil")
	}
	if r.MaxPlayers != 2 {
		return errors.New("max players must be exactly 2")
	}
	if r.DurationSeconds != 120 && r.DurationSeconds != 300 {
		return errors.New("duration must be 120 or 300 seconds")
	}
	switch r.Status {
	case StatusWaiting, StatusReady, StatusInBattle, StatusClosed, StatusExpired:
		// Valid status
	default:
		return fmt.Errorf("invalid room status: %s", r.Status)
	}
	return nil
}
```

### **In the Service Layer (`internal/rooms/service.go`)**
```go
func (s *Service) StartBattle(ctx context.Context, hostUserID uuid.UUID, roomCode string) (uuid.UUID, error) {
	// ... inside transaction ...
	room, err := s.repo.GetRoomByCodeForUpdate(ctx, tx, roomCode)
	
	// Enforce Business Rule: Only host can start battle
	if room.HostUserID != hostUserID {
		return uuid.Nil, errors.New("only the host can start the battle")
	}
	
	// Enforce Business Rule: Room must be ready
	if room.Status != StatusReady {
		return uuid.Nil, errors.New("cannot start battle; room is not in ready state")
	}
	
	// ... proceed with battle creation ...
}
```

---

## 4. Key Takeaways

1. **Entities are Guardians of Structure**: They make sure that no corrupt data can be loaded from the database or constructed in memory.
2. **Services are Guardians of Process**: They verify sequence, authorization, and state machine validity, utilizing repository-level transactional boundaries.
