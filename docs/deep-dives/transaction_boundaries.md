# Transaction Boundaries & Service Ownership

In the DSAblitz architecture, database transactions are owned and managed strictly within the Service layer (specifically in the Battle Service). The Repository layer acts as a stateless data access gateway and is completely decoupled from transaction policy decisions.

---

## Architectural Rationale for Service Ownership

### 1. Separation of Concerns
* **Repository**: Its sole responsibility is converting domain actions into SQL queries and scanning results back into domain models. It has no context of business processes, multi-table coordination, or external API validation rules.
* **Service**: Orchestrates business logic across multiple modules (e.g. Questions validation, Battle state progression, Room state synchronization). Business operations often involve multiple distinct repository queries and external validation steps that must fail or succeed as a single unit.

### 2. Preventing Connection Pools Starvation & Deadlocks
If repository methods were to initialize, manage, and commit their own internal transactions:
* Orchestrator services would have no way to span a single transaction across multiple repository methods.
* Attempting to run nested transactions would lock connections in the pool, leading to starvation under load.
* Controlling lock ordering (such as locking a `battle_player` before updating its corresponding room) would be impossible to coordinate from the repository level, leading to deadlocks.

### 3. Decoupling Business Rules from SQL Drivers
The Service layer owns the transactional context (`ctx`) and transaction handle (`tx pgx.Tx`), passing it down to repository functions. This allows the Service to decide:
* When to execute reads or writes.
* When to rollback due to business logic validation failures (e.g. game expiration or duplicate submission).
* How to handle transactional retries.

---

## Transaction Boundary Design Pattern

All database mutators in the repository must take `tx pgx.Tx` as a parameter and execute their queries against it:

```go
// Repository method: No transaction ownership, executes on provided tx
func (r *Repository) UpdateBattlePlayer(ctx context.Context, tx pgx.Tx, p BattlePlayer) error {
	_, err := tx.Exec(ctx, `UPDATE battle_players ...`, ...)
	return err
}
```

The Service layer uses `WithTransaction` to manage the boundary:

```go
// Service orchestrates the boundary
err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
    // 1. Pessimistically lock player row
    player, err := s.repo.GetBattlePlayerForUpdate(ctx, tx, battleID, userID)
    
    // 2. Load battle details inside transaction snapshot
    battle, err := s.repo.GetBattleTx(ctx, tx, battleID)
    
    // 3. Process business logic (errors here trigger automatic Rollback)
    if battle.Status != StatusActive {
        return ErrBattleFinished
    }
    
    // 4. Perform writes on tx
    err = s.repo.UpdateBattlePlayer(ctx, tx, player)
    
    return nil // Triggers Commit
})
```

---

## State Mutators & Idempotency Hook

When a battle completes, the completion function executes on the service-managed transaction:

```go
// CompleteBattle starts the transaction boundary in the Service
func (s *Service) CompleteBattle(ctx context.Context, battleID uuid.UUID) error {
	return s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Acquire battle row lock to make completion idempotent and run exactly once
		b, err := s.repo.GetBattleTx(ctx, tx, battleID)
		if err != nil {
			return err
		}
		if b.Status == StatusCompleted {
			return nil // Already completed, exit idempotently
		}
		return s.repo.CompleteBattle(ctx, tx, battleID)
	})
}
```
This guarantees that:
* The transaction is created and committed/rolled back entirely by the service.
* Any failure during the database execution results in an immediate rollback of the entire sequence.
