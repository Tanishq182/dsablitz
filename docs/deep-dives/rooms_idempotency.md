# Room Idempotency Specifications

This document defines the idempotent behaviors for Rooms module operations in **DSAblitz**. These rules ensure that retries, network glitches, or user double-clicks do not cause data corruption or return unexpected 500 error pages.

---

## 1. Idempotency Matrix

| API Endpoint | Request Scenario | Resulting State | Response Behavior |
| :--- | :--- | :--- | :--- |
| `JoinRoom` | Player joins room they are already active in | No state change | Returns HTTP 200 with current room state |
| `JoinRoom` | Player joins room that is full (not in it) | No state change | Returns HTTP 400 ("Room full") |
| `LeaveRoom` | Player leaves room they are not in / already left | No state change | Returns HTTP 200 (Success, no-op) |
| `ToggleReady` | Player toggles ready status to same value | No state change | Returns HTTP 200 (Success, no-op) |
| `StartBattle` | Host calls start battle while room is `in_battle` | No state change | Returns HTTP 200 with the active `battle_id` |

---

## 2. Detailed Behavior Specs

### **2.1. JoinRoom Idempotency**
* **Context**: A client sends a join request, but the network drops before receiving the response. The client retries.
* **Backend Logic**:
  - The transaction locks the room.
  - The service checks if the requesting `user_id` already exists in `room_players` with an active status (`joined` or `ready`).
  - If the user is already active, the backend bypasses the seat assignment and insertion steps, returning the existing room details with an HTTP 200 OK.

### **2.2. LeaveRoom Idempotency**
* **Context**: A client disconnects and triggers a leave request. The retry or a subsequent cleanup call is sent.
* **Backend Logic**:
  - The transaction locks the room.
  - The service checks if the user is in the room.
  - If the user is not found, or their status is already `left`, the service immediately commits and returns HTTP 200. It does not throw "player not found" errors to the client.

### **2.3. ToggleReady Idempotency**
* **Context**: A user double-clicks the "Ready" button.
* **Backend Logic**:
  - The transaction locks the room.
  - The service fetches the player's status.
  - If the target status matches the current status (e.g., target is `ready` and current is already `ready`), the service skips writing to the DB and returns HTTP 200.

### **2.4. StartBattle Idempotency**
* **Context**: The host clicks "Start Match" twice in rapid succession.
* **Backend Logic**:
  - The transaction locks the room.
  - If the room status is already `in_battle`, the service queries the `battles` table to find the active battle associated with the room:
    ```sql
    SELECT id FROM battles WHERE room_id = $1 AND status != 'finished' AND status != 'aborted';
    ```
  - If an active battle exists, the service skips battle generation and returns that battle's ID with HTTP 200 OK.
