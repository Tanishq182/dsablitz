# Authentication Sequence Diagram

This document presents a sequence diagram showing the request lifecycle for user signup, login, and token refresh.

---

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant Router as Gin Router
    participant Handler as Auth Handler
    participant Service as Auth Service
    participant Repo as Auth Repository
    participant DB as PostgreSQL

    %% Signup Flow
    Note over Client, DB: User Registration (Signup) Flow
    Client->>Router: POST /api/v1/auth/signup (Body: email, password, handle, display_name)
    Router->>Handler: Validate Auth Headers & Bind JSON DTO
    Handler->>Service: Signup(ctx, request, clientInfo)
    
    Service->>Repo: CreateUser(ctx, params)
    
    Repo->>DB: BEGIN TRANSACTION
    Repo->>DB: INSERT INTO users (email, password_hash, handle, display_name)
    alt Email or Handle Conflict
        DB-->>Repo: Unique Violation Error (23505)
        Repo->>DB: ROLLBACK TRANSACTION
        Repo-->>Service: ErrEmailTaken or ErrHandleTaken
        Service-->>Handler: Error
        Handler-->>Client: HTTP 409 Conflict
    else Insert User Success
        DB-->>Repo: User Entity Created
        Repo->>DB: INSERT INTO user_stats (user_id) VALUES (id)
        Repo->>DB: COMMIT TRANSACTION
        Repo-->>Service: User Entity
    end
    
    Note over Service: Generate Access JWT & Refresh UUID
    Service->>Repo: CreateSession(ctx, sessionParams)
    Repo->>DB: INSERT INTO auth_sessions (user_id, refresh_token_hash ...)
    DB-->>Repo: Session Entity
    
    Service-->>Handler: AccessToken, RefreshToken, User DTO
    Note over Handler: Set access_token & refresh_token HTTP-Only Cookies
    Handler-->>Client: HTTP 201 Created (User DTO JSON)

    %% Login Flow
    Note over Client, DB: User Authentication (Login) Flow
    Client->>Router: POST /api/v1/auth/login (Body: email, password)
    Router->>Handler: Validate Auth Headers & Bind JSON DTO
    Handler->>Service: Login(ctx, request, clientInfo)
    
    Service->>Repo: FindUserByEmail(ctx, email)
    Repo->>DB: SELECT * FROM users WHERE email = $1
    DB-->>Repo: User Row Details
    
    Note over Service: 1. Validate user status is 'active'<br/>2. Verify password matches password_hash
    
    Note over Service: Generate Access JWT & Refresh UUID
    Service->>Repo: CreateSession(ctx, sessionParams)
    Repo->>DB: INSERT INTO auth_sessions (user_id, refresh_token_hash ...)
    DB-->>Repo: Session Entity
    
    Service->>Repo: UpdateLastLogin(ctx, userID)
    Repo->>DB: UPDATE users SET last_login_at = NOW() WHERE id = $1
    
    Service-->>Handler: AccessToken, RefreshToken, User DTO
    Note over Handler: Set access_token & refresh_token HTTP-Only Cookies
    Handler-->>Client: HTTP 200 OK (User DTO JSON)

    %% Refresh Flow
    Note over Client, DB: Token Rotation (Refresh) Flow
    Client->>Router: POST /api/v1/auth/refresh (Cookie: refresh_token)
    Router->>Handler: Extract refresh_token Cookie
    Handler->>Service: Refresh(ctx, refreshToken, clientInfo)
    
    Note over Service: Compute SHA-256 hash of refresh_token
    Service->>Repo: FindActiveSessionByHash(ctx, tokenHash)
    Repo->>DB: SELECT * FROM auth_sessions WHERE refresh_token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()
    DB-->>Repo: Session Row
    
    Service->>Repo: RotateSession(ctx, oldHash, newParams)
    Repo->>DB: BEGIN TRANSACTION
    Repo->>DB: UPDATE auth_sessions SET revoked_at = NOW() WHERE refresh_token_hash = oldHash
    Repo->>DB: INSERT INTO auth_sessions (user_id, refresh_token_hash ...)
    Repo->>DB: COMMIT TRANSACTION
    Repo-->>Service: New Session Entity
    
    Service-->>Handler: AccessToken, RefreshToken, User DTO
    Note over Handler: Overwrite access_token & refresh_token HTTP-Only Cookies
    Handler-->>Client: HTTP 200 OK (User DTO JSON)
```

---

## 2. Step-by-Step Trace

### 2.1 Signup Step-by-Step:
1.  **POST `/signup`**: Client sends signup details. Handlers bind to the JSON DTO and validate formats.
2.  **`CreateUser` Transaction**: Opens a transaction, inserts the user row, and checks for unique constraint violations (returns `409 Conflict` on duplicates). It then initializes the user's statistics in `user_stats` and commits.
3.  **Session Generation**: Generates access and refresh tokens, saves the refresh token hash in `auth_sessions`, sets cookies, and returns `201 Created`.

### 2.2 Login Step-by-Step:
1.  **POST `/login`**: Client sends login details.
2.  **Lookup & Compare**: Queries the user by email, checks their status is `active`, and verifies the password hash.
3.  **Establish Session**: Generates tokens, logs the refresh token hash in `auth_sessions`, updates the last login timestamp, sets cookies, and returns `200 OK`.

### 2.3 Refresh Step-by-Step:
1.  **POST `/refresh`**: Client sends refresh token cookie.
2.  **Verify Active Session**: Hashes the refresh token and queries the database for an active, unrevoked, unexpired session matching the hash.
3.  **Atomic Rotation**: Starts a transaction, revokes the old session, inserts a new rotated session row, commits, sets new cookies, and returns `200 OK`.
