# DSAblitz

DSAblitz is a real-time 1v1 DSA battle platform.

## Gameplay
- Players compete in 1v1 battles
- Match duration: 2 min or 5 min
- Continuous rapid-fire questions
- Faster solving = more questions attempted
- Score depends on:
  - correctness
  - speed
  - streak bonus

## Question Types
- MCQ
- Complexity prediction
- Pattern recognition
- Numeric answer
- Algorithm ordering

## Tech Stack

Frontend:
- React
- Tailwind
- Zustand

Backend:
- Go
- Gin
- PostgreSQL
- Redis
- WebSockets

## Architecture
Modular Monolith

Modules:
- auth
- users
- rooms
- battle
- questions
## Locked Decisions (DO NOT CHANGE)

Backend Framework:
- Gin (mandatory)

Architecture:
- Modular Monolith

MVP Constraints:
- 1v1 only
- Quiz based
- No coding judge
- No Docker sandbox

Important:
- Extend existing scaffold incrementally
- Never rewrite architecture unless explicitly requested
