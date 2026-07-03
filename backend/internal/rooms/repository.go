package rooms

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("record not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// WithTransaction runs a function inside a pgx transaction.
func (r *Repository) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// InsertRoom inserts a room record.
func (r *Repository) InsertRoom(ctx context.Context, tx pgx.Tx, room Room) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO rooms (id, code, host_user_id, status, max_players, duration_seconds, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`, room.ID, room.Code, room.HostUserID, room.Status, room.MaxPlayers, room.DurationSeconds, room.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert room: %w", err)
	}
	return nil
}

// GetRoomByCode retrieves a room by its code.
func (r *Repository) GetRoomByCode(ctx context.Context, code string) (Room, error) {
	var room Room
	err := r.db.QueryRow(ctx, `
		SELECT id, code, host_user_id, status, max_players, duration_seconds, expires_at, created_at, updated_at
		FROM rooms
		WHERE code = $1
	`, code).Scan(&room.ID, &room.Code, &room.HostUserID, &room.Status, &room.MaxPlayers, &room.DurationSeconds, &room.ExpiresAt, &room.CreatedAt, &room.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Room{}, ErrNotFound
		}
		return Room{}, fmt.Errorf("get room by code: %w", err)
	}
	return room, nil
}

// GetRoomByCodeForUpdate locks and retrieves a room by its code.
func (r *Repository) GetRoomByCodeForUpdate(ctx context.Context, tx pgx.Tx, code string) (Room, error) {
	var room Room
	err := tx.QueryRow(ctx, `
		SELECT id, code, host_user_id, status, max_players, duration_seconds, expires_at, created_at, updated_at
		FROM rooms
		WHERE code = $1
		FOR UPDATE
	`, code).Scan(&room.ID, &room.Code, &room.HostUserID, &room.Status, &room.MaxPlayers, &room.DurationSeconds, &room.ExpiresAt, &room.CreatedAt, &room.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Room{}, ErrNotFound
		}
		return Room{}, fmt.Errorf("get room by code for update: %w", err)
	}
	return room, nil
}

// GetRoomForUpdate locks and retrieves a room by its ID.
func (r *Repository) GetRoomForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Room, error) {
	var room Room
	err := tx.QueryRow(ctx, `
		SELECT id, code, host_user_id, status, max_players, duration_seconds, expires_at, created_at, updated_at
		FROM rooms
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&room.ID, &room.Code, &room.HostUserID, &room.Status, &room.MaxPlayers, &room.DurationSeconds, &room.ExpiresAt, &room.CreatedAt, &room.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Room{}, ErrNotFound
		}
		return Room{}, fmt.Errorf("get room for update: %w", err)
	}
	return room, nil
}

// UpdateRoomStatus updates the room's status.
func (r *Repository) UpdateRoomStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status RoomStatus) error {
	_, err := tx.Exec(ctx, `
		UPDATE rooms
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`, id, status)
	if err != nil {
		return fmt.Errorf("update room status: %w", err)
	}
	return nil
}

// InsertRoomPlayer inserts a player presence record.
func (r *Repository) InsertRoomPlayer(ctx context.Context, tx pgx.Tx, p RoomPlayer) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO room_players (id, room_id, user_id, seat_number, status, joined_at, left_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), $6, NOW(), NOW())
	`, p.ID, p.RoomID, p.UserID, p.SeatNumber, p.Status, p.LeftAt)
	if err != nil {
		return fmt.Errorf("insert room player: %w", err)
	}
	return nil
}

// GetRoomPlayer retrieves a player presence record.
func (r *Repository) GetRoomPlayer(ctx context.Context, roomID, userID uuid.UUID) (RoomPlayer, error) {
	var p RoomPlayer
	err := r.db.QueryRow(ctx, `
		SELECT id, room_id, user_id, seat_number, status, joined_at, left_at, created_at, updated_at
		FROM room_players
		WHERE room_id = $1 AND user_id = $2
	`, roomID, userID).Scan(&p.ID, &p.RoomID, &p.UserID, &p.SeatNumber, &p.Status, &p.JoinedAt, &p.LeftAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RoomPlayer{}, ErrNotFound
		}
		return RoomPlayer{}, fmt.Errorf("get room player: %w", err)
	}
	return p, nil
}

// GetActivePlayersForUpdate locks and retrieves active room players (joined or ready) in the room.
func (r *Repository) GetActivePlayersForUpdate(ctx context.Context, tx pgx.Tx, roomID uuid.UUID) ([]RoomPlayer, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, room_id, user_id, seat_number, status, joined_at, left_at, created_at, updated_at
		FROM room_players
		WHERE room_id = $1 AND status IN ('joined', 'ready')
		ORDER BY seat_number ASC
		FOR UPDATE
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("query active players: %w", err)
	}
	defer rows.Close()

	var players []RoomPlayer
	for rows.Next() {
		var p RoomPlayer
		err := rows.Scan(&p.ID, &p.RoomID, &p.UserID, &p.SeatNumber, &p.Status, &p.JoinedAt, &p.LeftAt, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan room player: %w", err)
		}
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return players, nil
}

// UpdatePlayerStatus updates a participant's ready/presence status.
func (r *Repository) UpdatePlayerStatus(ctx context.Context, tx pgx.Tx, roomID, userID uuid.UUID, status RoomPlayerStatus) error {
	_, err := tx.Exec(ctx, `
		UPDATE room_players
		SET status = $3, updated_at = NOW()
		WHERE room_id = $1 AND user_id = $2
	`, roomID, userID, status)
	if err != nil {
		return fmt.Errorf("update player status: %w", err)
	}
	return nil
}

// DeleteRoomPlayer deletes a player presence record completely.
func (r *Repository) DeleteRoomPlayer(ctx context.Context, tx pgx.Tx, roomID, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM room_players
		WHERE room_id = $1 AND user_id = $2
	`, roomID, userID)
	if err != nil {
		return fmt.Errorf("delete room player: %w", err)
	}
	return nil
}

// MarkPlayerLeft sets player status to 'left' and sets left_at timestamp.
func (r *Repository) MarkPlayerLeft(ctx context.Context, tx pgx.Tx, roomID, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE room_players
		SET status = 'left', left_at = NOW(), updated_at = NOW()
		WHERE room_id = $1 AND user_id = $2
	`, roomID, userID)
	if err != nil {
		return fmt.Errorf("mark player left: %w", err)
	}
	return nil
}

// MarkAllPlayersLeft sets all active players' status to 'left' and sets left_at timestamp.
func (r *Repository) MarkAllPlayersLeft(ctx context.Context, tx pgx.Tx, roomID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE room_players
		SET status = 'left', left_at = NOW(), updated_at = NOW()
		WHERE room_id = $1 AND status IN ('joined', 'ready')
	`, roomID)
	if err != nil {
		return fmt.Errorf("mark all players left: %w", err)
	}
	return nil
}// GetPlayerRating retrieves the rating for a user from user_stats, defaulting to 1000 if not found.
func (r *Repository) GetPlayerRating(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (int, error) {
	var rating int
	err := tx.QueryRow(ctx, `
		SELECT rating FROM user_stats WHERE user_id = $1
	`, userID).Scan(&rating)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 1000, nil
		}
		return 0, fmt.Errorf("query player rating: %w", err)
	}
	return rating, nil
}

// GetActiveBattleID checks if there is an active battle associated with the room.
func (r *Repository) GetActiveBattleID(ctx context.Context, tx pgx.Tx, roomID uuid.UUID) (uuid.UUID, error) {
	var bID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM battles 
		WHERE room_id = $1 AND status IN ($2, $3, $4)
		LIMIT 1
	`, roomID, BattleStatusCreated, BattleStatusCountdown, BattleStatusActive).Scan(&bID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, fmt.Errorf("query active battle ID: %w", err)
	}
	return bID, nil
}
