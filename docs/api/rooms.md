# Matchmaking & Rooms API Reference

This document provides technical documentation for the Matchmaking and Rooms API endpoints in the DSAblitz monolith.

---

## 1. Overview & Base Path

All matchmaking and lobby endpoints are prefixed with the base path: `/api/v1/rooms`

All endpoints in this catalog require authentication via the `access_token` cookie (verified by the Gin JWT middleware).

---

## 2. Endpoint Catalog

| Method | Path | Description |
| :--- | :--- | :--- |
| `POST` | `/` | Create a new matchmaking lobby and register the host. |
| `POST` | `/:code/join` | Join an existing lobby as a guest player using the room code. |
| `POST` | `/:code/ready` | Toggle the ready status of a participant in the lobby. |
| `POST` | `/:code/leave` | Leave the lobby (triggers host closure or reset to waiting). |
| `POST` | `/:code/start-battle` | Launch the battle sequence (Host only, requires both players ready). |

---

## 3. Endpoint Specifications

### 3.1 `POST /`
Creates a new matchmaking room, generates a unique 6-character code, and registers the caller as host.

*   **Request Schema** ([routes.go:L35-L37](file:///home/tanishq/dsablitz/backend/internal/rooms/routes.go#L35-L37)):
    ```json
    {
      "duration_seconds": 300
    }
    ```
*   **Validation Rules**:
    *   `duration_seconds`: Required. Must be exactly `120` (2 minutes) or `300` (5 minutes).
*   **Response Schema** (Status `201 Created`):
    ```json
    {
      "id": "c138dfb7-0b16-43d9-a477-96a86c67efbc",
      "code": "KM39AG",
      "host_user_id": "76495df2-70b9-4a94-8742-1e9deab7b2b7",
      "status": "waiting",
      "max_players": 2,
      "duration_seconds": 300,
      "expires_at": "2026-07-04T20:10:00Z",
      "created_at": "2026-07-04T20:00:00Z",
      "updated_at": "2026-07-04T20:00:00Z"
    }
    ```

---

### 3.2 `POST /:code/join`
Joins an existing room as a guest. The caller is placed in seat 2.

*   **Request Credentials**:
    Authenticated user context. No request body required.
*   **Response Schema** (Status `200 OK`):
    ```json
    {
      "id": "c138dfb7-0b16-43d9-a477-96a86c67efbc",
      "code": "KM39AG",
      "host_user_id": "76495df2-70b9-4a94-8742-1e9deab7b2b7",
      "status": "waiting",
      "max_players": 2,
      "duration_seconds": 300,
      "expires_at": "2026-07-04T20:10:00Z",
      "created_at": "2026-07-04T20:00:00Z",
      "updated_at": "2026-07-04T20:00:00Z"
    }
    ```

---

### 3.3 `POST /:code/ready`
Toggles a participant's readiness status. If both players in a full room are ready, the room status transitions to `ready`.

*   **Request Schema** ([routes.go:L87-L89](file:///home/tanishq/dsablitz/backend/internal/rooms/routes.go#L87-L89)):
    ```json
    {
      "ready": true
    }
    ```
*   **Validation Rules**:
    *   `ready`: Required boolean.
*   **Response Schema** (Status `200 OK`):
    ```json
    {
      "id": "c138dfb7-0b16-43d9-a477-96a86c67efbc",
      "code": "KM39AG",
      "host_user_id": "76495df2-70b9-4a94-8742-1e9deab7b2b7",
      "status": "ready",
      "max_players": 2,
      "duration_seconds": 300,
      "expires_at": "2026-07-04T20:10:00Z",
      "created_at": "2026-07-04T20:00:00Z",
      "updated_at": "2026-07-04T20:00:00Z"
    }
    ```

---

### 3.4 `POST /:code/leave`
Removes a player from the lobby. If the host leaves, the room is closed, and all active players are removed.

*   **Response Schema** (Status `200 OK`):
    ```json
    {
      "status": "success"
    }
    ```

---

### 3.5 `POST /:code/start-battle`
Launches the battle sequence. Triggers state updates in the Rooms module and initializes the battle record via the Battle coordinator.

*   **Response Schema** (Status `200 OK`):
    ```json
    {
      "battle_id": "87506ab8-10c9-4ef2-ba22-87ba652309f1"
    }
    ```

---

## 4. Error Responses

Lobby API endpoints return errors in the following format:
```json
{
  "error": "error message details"
}
```

| HTTP Status | Triggering Condition | Example Error String |
| :--- | :--- | :--- |
| `400 Bad Request` | Duration not supported or body validation failed | `"duration must be 120 or 300 seconds"` |
| `401 Unauthorized` | Missing or invalid auth token cookie | `"unauthorized"` |
| `404 Not Found` | Room code does not match any active lobby | `"room not found"` |
| `409 Conflict` | Host tries to start battle but room is not in `ready` state | `"cannot start battle: room is in status waiting"` |

---

## 5. Transaction Boundaries

- **`CreateRoom`**: Runs inside a transaction. Generates a random 6-character code and inserts the room and host player row. If the generated code already exists, the transaction rolls back, and the service retries up to 3 times before returning an error.
- **`JoinRoom`**: Runs inside a transaction. Locks the target room (`FOR UPDATE`) and active players to prevent concurrent registrations from exceeding the 2-player capacity.
- **`ToggleReady`**: Runs inside a transaction. Locks the room and its active players, updates the caller's readiness, and transitions the room status if all players are ready.
- **`StartBattle`**: Runs inside a single transaction. Locks the room and active players, updates the room status to `in_battle`, and calls the Battle coordinator to create the battle metadata, player scorecards, and question sequence.

---

## 6. Idempotency Considerations

- **JoinRoom**: Re-joining an active room returns the current room state without creating a new player record.
- **ToggleReady**: Toggling to the same ready state is a no-op, returning the current room state.
- **StartBattle**: If the room is already in the `in_battle` state, the endpoint retrieves the active battle ID and returns it, preventing duplicate battles.

---

## 7. Production Considerations

- **Lobby Expiration**: Lobbies have a 10-minute expiry time (`expires_at`). A cron task (`ExpireRooms`) periodically finds expired lobbies and transitions their status to `expired`, freeing up resources.
- **Seat Allocation**: Seats are assigned sequentially (host gets seat 1, guest gets seat 2). If a guest leaves, their seat is freed, allowing other players to join.

---

## 8. Planned Work (V2)

- **WebSocket Presence Synchronization**: Transition lobby updates from REST polling to real-time WebSocket events.
- **Custom Question Filters**: Allow hosts to select specific tags (e.g., `graphs`, `dynamic-programming`) during lobby creation.

---

## 9. Code References

- **HTTP Handlers**: [rooms/routes.go](file:///home/tanishq/dsablitz/backend/internal/rooms/routes.go)
- **Room Lifecycle Service**: [rooms/service.go](file:///home/tanishq/dsablitz/backend/internal/rooms/service.go)
- **Database Repository**: [rooms/repository.go](file:///home/tanishq/dsablitz/backend/internal/rooms/repository.go)

---

## 10. Related Documents

- **Database Transactions**: [transactions.md](file:///home/tanishq/dsablitz/docs/database/transactions.md)
- **Room Creation Flow**: [room_creation_flow.md](file:///home/tanishq/dsablitz/docs/flows/room_creation_flow.md)
- **Room Joining Flow**: [room_joining_flow.md](file:///home/tanishq/dsablitz/docs/flows/room_joining_flow.md)
