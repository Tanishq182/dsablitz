package rooms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RoomStatus string

const (
	StatusWaiting  RoomStatus = "waiting"
	StatusReady    RoomStatus = "ready"
	StatusInBattle RoomStatus = "in_battle"
	StatusClosed   RoomStatus = "closed"
	StatusExpired  RoomStatus = "expired"
)

// Battle statuses queried by Rooms package to avoid hardcoded strings
const (
	BattleStatusCreated   = "created"
	BattleStatusCountdown = "countdown"
	BattleStatusActive    = "active"
)

type RoomPlayerStatus string

const (
	PlayerJoined RoomPlayerStatus = "joined"
	PlayerReady  RoomPlayerStatus = "ready"
	PlayerLeft   RoomPlayerStatus = "left"
	PlayerKicked RoomPlayerStatus = "kicked"
)

// Room represents the stateful database lobby.
type Room struct {
	ID              uuid.UUID  `db:"id"`
	Code            string     `db:"code"`
	HostUserID      uuid.UUID  `db:"host_user_id"`
	Status          RoomStatus `db:"status"`
	MaxPlayers      int16      `db:"max_players"`
	DurationSeconds int        `db:"duration_seconds"`
	ExpiresAt       *time.Time `db:"expires_at"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

// Validate checks core structural invariants for a Room.
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
		return fmt.Errorf("unsupported room status: %s", r.Status)
	}
	return nil
}

// RoomPlayer represents a participant inside a room.
type RoomPlayer struct {
	ID         uuid.UUID        `db:"id"`
	RoomID     uuid.UUID        `db:"room_id"`
	UserID     uuid.UUID        `db:"user_id"`
	SeatNumber int16            `db:"seat_number"`
	Status     RoomPlayerStatus `db:"status"`
	JoinedAt   time.Time        `db:"joined_at"`
	LeftAt     *time.Time       `db:"left_at"`
	CreatedAt  time.Time        `db:"created_at"`
	UpdatedAt  time.Time        `db:"updated_at"`
}

// Validate checks core structural invariants for a RoomPlayer.
func (rp *RoomPlayer) Validate() error {
	if rp.ID == uuid.Nil {
		return errors.New("room player record ID cannot be nil")
	}
	if rp.RoomID == uuid.Nil {
		return errors.New("room ID cannot be nil")
	}
	if rp.UserID == uuid.Nil {
		return errors.New("user ID cannot be nil")
	}
	if rp.SeatNumber != 1 && rp.SeatNumber != 2 {
		return fmt.Errorf("seat number must be 1 or 2, got %d", rp.SeatNumber)
	}
	switch rp.Status {
	case PlayerJoined, PlayerReady, PlayerLeft, PlayerKicked:
		// Valid status
	default:
		return fmt.Errorf("unsupported player status: %s", rp.Status)
	}
	return nil
}

// BattlePlayer represents participant details passed to BattleCoordinator.
type BattlePlayer struct {
	UserID       uuid.UUID
	SeatNumber   int16
	RatingBefore int
}

// BattleCoordinator defines the interface for communicating with the gameplay engine.
type BattleCoordinator interface {
	StartBattle(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, players []BattlePlayer, seed int64) (uuid.UUID, error)
}
