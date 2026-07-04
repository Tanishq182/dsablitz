# Authentication API Reference

This document provides technical documentation for the authentication API endpoints in the DSAblitz monolith.

---

## 1. Overview & Base Path

All authentication endpoints are prefixed with the base path: `/api/v1/auth`

---

## 2. Endpoint Catalog

| Method | Path | Authentication | Description |
| :--- | :--- | :--- | :--- |
| `POST` | `/signup` | None | Register a new user account. |
| `POST` | `/login` | None | Authenticate credentials and establish session cookies. |
| `POST` | `/refresh` | Refresh Cookie | Rotate JWT access and refresh tokens. |
| `POST` | `/logout` | Refresh Cookie | Revoke the active refresh token session. |
| `GET` | `/me` | Access Token | Retrieve profile details of the authenticated user. |

---

## 3. Detailed Endpoint Specifications

### 3.1 `POST /signup`
Registers a new user, automatically initializes player statistics, and returns authentication cookies.

*   **Request Schema** ([dto.go:L5-L10](file:///home/tanishq/dsablitz/backend/internal/auth/dto.go#L5-L10)):
    ```json
    {
      "email": "player@dsablitz.com",
      "password": "securepassword123",
      "handle": "dsa_master",
      "display_name": "DSA Master"
    }
    ```
*   **Validation Rules**:
    *   `email`: Required, valid email format.
    *   `password`: Required, minimum 8 characters, maximum 128 characters.
    *   `handle`: Required, minimum 3 characters, maximum 32 characters.
    *   `display_name`: Required, minimum 1 character, maximum 80 characters.
*   **Response Schema** (Status `201 Created`):
    ```json
    {
      "user": {
        "id": "76495df2-70b9-4a94-8742-1e9deab7b2b7",
        "email": "player@dsablitz.com",
        "handle": "dsa_master",
        "display_name": "DSA Master",
        "status": "active",
        "created_at": "2026-07-04T19:59:00Z",
        "updated_at": "2026-07-04T19:59:00Z"
      }
    }
    ```
*   **Response Headers**:
    Sets HTTP-Only cookies: `access_token` and `refresh_token`.

---

### 3.2 `POST /login`
Authenticates credentials and establishes session cookies.

*   **Request Schema** ([dto.go:L12-L15](file:///home/tanishq/dsablitz/backend/internal/auth/dto.go#L12-L15)):
    ```json
    {
      "email": "player@dsablitz.com",
      "password": "securepassword123"
    }
    ```
*   **Validation Rules**:
    *   `email`: Required, valid email format.
    *   `password`: Required.
*   **Response Schema** (Status `200 OK`):
    ```json
    {
      "user": {
        "id": "76495df2-70b9-4a94-8742-1e9deab7b2b7",
        "email": "player@dsablitz.com",
        "handle": "dsa_master",
        "display_name": "DSA Master",
        "status": "active",
        "last_login_at": "2026-07-04T20:00:00Z",
        "created_at": "2026-07-04T19:59:00Z",
        "updated_at": "2026-07-04T20:00:00Z"
      }
    }
    ```
*   **Response Headers**:
    Sets HTTP-Only cookies: `access_token` and `refresh_token`.

---

### 3.3 `POST /refresh`
Rotates the active session's access and refresh tokens.

*   **Request Credentials**:
    Requires the `refresh_token` cookie.
*   **Response Schema** (Status `200 OK`):
    ```json
    {
      "user": {
        "id": "76495df2-70b9-4a94-8742-1e9deab7b2b7",
        "email": "player@dsablitz.com",
        "handle": "dsa_master",
        "display_name": "DSA Master",
        "status": "active",
        "created_at": "2026-07-04T19:59:00Z",
        "updated_at": "2026-07-04T20:00:00Z"
      }
    }
    ```
*   **Response Headers**:
    Overwrites `access_token` and `refresh_token` cookies with new rotated values.

---

### 3.4 `POST /logout`
Revokes the current active session.

*   **Request Credentials**:
    Requires the `refresh_token` cookie.
*   **Response Schema** (Status `244 No Content` / `204 No Content`):
    No body content. Clears auth cookies.

---

### 3.5 `GET /me`
Retrieves profile details of the currently logged-in user.

*   **Request Credentials**:
    Requires the `access_token` cookie (verified via Gin JWT middleware).
*   **Response Schema** (Status `200 OK`):
    ```json
    {
      "user": {
        "id": "76495df2-70b9-4a94-8742-1e9deab7b2b7",
        "email": "player@dsablitz.com",
        "handle": "dsa_master",
        "display_name": "DSA Master",
        "status": "active",
        "created_at": "2026-07-04T19:59:00Z",
        "updated_at": "2026-07-04T20:00:00Z"
      }
    }
    ```

---

## 4. Error Responses

Standard structure for error payloads:
```json
{
  "error": "error message details"
}
```

| HTTP Status | Triggering Condition | Example Error String |
| :--- | :--- | :--- |
| `400 Bad Request` | Missing binding payload / malformed JSON | `"invalid request body"` |
| `401 Unauthorized` | Invalid password or email, missing cookies, or expired JWT | `"invalid credentials"` or `"unauthorized"` |
| `403 Forbidden` | Authenticated user is disabled by administrator | `"user status is disabled"` |
| `409 Conflict` | Target email or handle already registered in database | `"email already taken"` or `"handle already taken"` |
| `500 Internal Error` | Database connection failure or unhandled exception | `"internal server error"` |

---

## 5. Transaction Boundaries

- **`Signup`**: Starts a transaction to insert the new user profile row into `users` and initialize their statistics in `user_stats`. If stats initialization fails, the user insert is rolled back to prevent orphaned accounts.
- **`Refresh` (Rotation)**: Starts a transaction to fetch the active session, set `revoked_at` to the current time, and insert a new rotated session row in `auth_sessions`. If either step fails, the operation is rolled back.

---

## 6. Idempotency Considerations

- **Logout**: Multiple logout calls are safe. If the session has already been revoked or the cookie is missing, the API returns success and clears the cookies.
- **Signup**: Repeated identical signup attempts fail with `409 Conflict` on unique email/handle checks, preventing duplicate accounts.

---

## 7. Production Considerations

- **Secure Cookies**: In production environments, the `Secure` attribute is enabled on cookies, forcing browsers to only transmit them over HTTPS.
- **SameSite Policy**: Cookies use `SameSite=Lax` to prevent Cross-Site Request Forgery (CSRF) while allowing authentications on cross-site navigations.
- **Token Expiry**:
  - `access_token`: Expires in 15 minutes.
  - `refresh_token`: Expires in 7 days (scoped strictly to `/api/v1/auth`).

---

## 8. Planned Work (V2)

- **Rate Limiting**: Add IP-based rate limiting on `/login` and `/signup` endpoints to prevent brute-force attacks.
- **OAuth Integration Handlers**: Implement callback routes (`/api/v1/auth/callback/google` and `/api/v1/auth/callback/github`) to link external accounts.

---

## 9. Code References

- **HTTP Handlers**: [auth/handler.go](file:///home/tanishq/dsablitz/backend/internal/auth/handler.go)
- **Token Operations**: [auth/token.go](file:///home/tanishq/dsablitz/backend/internal/auth/token.go)
- **Routes Wiring**: [auth/routes.go](file:///home/tanishq/dsablitz/backend/internal/auth/routes.go)

---

## 10. Related Documents

- **Database Schema**: [schema.md](file:///home/tanishq/dsablitz/docs/database/schema.md)
- **Authentication Sequence**: [authentication_sequence.md](file:///home/tanishq/dsablitz/docs/diagrams/authentication_sequence.md)
- **Login Flow Deep Dive**: [login_flow.md](file:///home/tanishq/dsablitz/docs/flows/login_flow.md)
