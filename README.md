# DSAblitz

DSAblitz is a real-time competitive coding battle platform where users challenge friends in 1v1 DSA battles, race to solve algorithmic problems, and compete through ratings, streaks, and leaderboards.

## Vision
Make DSA practice addictive through:
- competition
- real-time battles
- rankings
- streaks
- friend rivalry

Inspired by competitive gaming systems and coding interview preparation.

---

## Tech Stack

### Backend
- Go
- Gin
- PostgreSQL
- Redis
- Docker
- WebSockets (planned)

### Frontend
- React (planned)

### Architecture
- Modular Monolith
- Event-driven components (future)
- Production-grade backend design

---

## Current Progress

### Phase 1 — Backend Scaffold
- Gin server
- Modular architecture
- Route registration
- Middleware
- Health endpoint

### Phase 2 — Infrastructure
- Environment configuration
- PostgreSQL connection pooling
- Redis client setup
- Graceful shutdown
- Docker setup

### Phase 3 — Database Schema
Implemented production schema for:
- users
- friendships
- rooms
- battles
- submissions
- ratings
- stats

### Phase 4 — Authentication
Implemented:
- Signup
- Login
- Logout
- Refresh tokens
- JWT middleware
- Argon2 password hashing
- Session-based refresh token storage
- Secure HttpOnly cookies

---

## Roadmap

- [x] Backend scaffold
- [x] Infrastructure
- [x] Schema design
- [x] Authentication
- [ ] Users module
- [ ] Friends system
- [ ] Room lifecycle
- [ ] Matchmaking
- [ ] Battle engine
- [ ] WebSockets
- [ ] Leaderboards
- [ ] Deployment

---

## System Design Highlights
- Stateless access authentication using JWT
- Stateful refresh sessions
- Production-ready PostgreSQL schema
- Room and battle lifecycle separation
- Scalable battle architecture for real-time gameplay