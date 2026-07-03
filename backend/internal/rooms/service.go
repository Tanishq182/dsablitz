package rooms

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type roomRepository interface {
	WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
	InsertRoom(ctx context.Context, tx pgx.Tx, room Room) error
	GetRoomByCode(ctx context.Context, code string) (Room, error)
	GetRoomByCodeForUpdate(ctx context.Context, tx pgx.Tx, code string) (Room, error)
	GetRoomForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Room, error)
	UpdateRoomStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status RoomStatus) error
	InsertRoomPlayer(ctx context.Context, tx pgx.Tx, p RoomPlayer) error
	GetRoomPlayer(ctx context.Context, roomID, userID uuid.UUID) (RoomPlayer, error)
	GetActivePlayersForUpdate(ctx context.Context, tx pgx.Tx, roomID uuid.UUID) ([]RoomPlayer, error)
	UpdatePlayerStatus(ctx context.Context, tx pgx.Tx, roomID, userID uuid.UUID, status RoomPlayerStatus) error
	DeleteRoomPlayer(ctx context.Context, tx pgx.Tx, roomID, userID uuid.UUID) error
	MarkPlayerLeft(ctx context.Context, tx pgx.Tx, roomID, userID uuid.UUID) error
	MarkAllPlayersLeft(ctx context.Context, tx pgx.Tx, roomID uuid.UUID) error
	GetPlayerRating(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (int, error)
	GetActiveBattleID(ctx context.Context, tx pgx.Tx, roomID uuid.UUID) (uuid.UUID, error)
}

type Service struct {
	repo             roomRepository
	battleCoordinator BattleCoordinator
}

func NewService(repo roomRepository, bc BattleCoordinator) *Service {
	return &Service{
		repo:             repo,
		battleCoordinator: bc,
	}
}

// CreateRoom creates a new room, generates a unique code, and joins the host.
func (s *Service) CreateRoom(ctx context.Context, hostUserID uuid.UUID, durationSeconds int) (Room, error) {
	if durationSeconds != 120 && durationSeconds != 300 {
		return Room{}, errors.New("duration must be 120 or 300 seconds")
	}

	var room Room
	var hostPlayer RoomPlayer
	var err error
	success := false

	for i := 0; i < 3; i++ {
		code := generateCode()
		err = s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
			_, getErr := s.repo.GetRoomByCodeForUpdate(ctx, tx, code)
			if getErr == nil {
				return errors.New("room code collision")
			}
			if !errors.Is(getErr, ErrNotFound) {
				return getErr
			}

			expiresAt := time.Now().Add(10 * time.Minute)
			room = Room{
				ID:              uuid.New(),
				Code:            code,
				HostUserID:      hostUserID,
				Status:          StatusWaiting,
				MaxPlayers:      2,
				DurationSeconds: durationSeconds,
				ExpiresAt:       &expiresAt,
			}

			if err = room.Validate(); err != nil {
				return fmt.Errorf("room validation: %w", err)
			}

			if err = s.repo.InsertRoom(ctx, tx, room); err != nil {
				return err
			}

			// Insert host player
			hostPlayer = RoomPlayer{
				ID:         uuid.New(),
				RoomID:     room.ID,
				UserID:     hostUserID,
				SeatNumber: 1,
				Status:     PlayerJoined,
			}

			if err = hostPlayer.Validate(); err != nil {
				return fmt.Errorf("host player validation: %w", err)
			}

			if err = s.repo.InsertRoomPlayer(ctx, tx, hostPlayer); err != nil {
				return err
			}

			return nil
		})

		if err == nil {
			success = true
			break
		}
		if err.Error() != "room code collision" {
			return Room{}, err
		}
	}

	if !success {
		return Room{}, errors.New("failed to generate unique room code")
	}

	return room, nil
}

// JoinRoom allows a guest player to join a room by its code.
func (s *Service) JoinRoom(ctx context.Context, userID uuid.UUID, code string) (Room, error) {
	var room Room

	err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// 1. Lock room
		var err error
		room, err = s.repo.GetRoomByCodeForUpdate(ctx, tx, code)
		if err != nil {
			return err
		}

		// Idempotency: check if the user is already in the room
		activePlayers, err := s.repo.GetActivePlayersForUpdate(ctx, tx, room.ID)
		if err != nil {
			return err
		}

		for _, p := range activePlayers {
			if p.UserID == userID {
				return nil // User is already in the room, return room state (idempotency)
			}
		}

		// Business Rule: cannot join a room that is not waiting
		if room.Status != StatusWaiting {
			return fmt.Errorf("cannot join: room is in status %s", room.Status)
		}

		// Business Rule: cannot join if lobby is full
		if len(activePlayers) >= 2 {
			return errors.New("cannot join: room is full")
		}

		// Seating logic: determine seat number. Host is seat 1, guest gets seat 2
		seatNumber := int16(2)
		if len(activePlayers) == 0 {
			// Edge case: if host left, first joining player gets seat 1
			seatNumber = 1
		} else if activePlayers[0].SeatNumber == 2 {
			seatNumber = 1
		}

		guestPlayer := RoomPlayer{
			ID:         uuid.New(),
			RoomID:     room.ID,
			UserID:     userID,
			SeatNumber: seatNumber,
			Status:     PlayerJoined,
		}

		if err = guestPlayer.Validate(); err != nil {
			return fmt.Errorf("guest player validation: %w", err)
		}

		if err = s.repo.InsertRoomPlayer(ctx, tx, guestPlayer); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return Room{}, err
	}

	// Fetch fresh room state after transaction committed
	return s.repo.GetRoomByCode(ctx, code)
}

// ToggleReady allows a player to update their readiness status inside the room.
func (s *Service) ToggleReady(ctx context.Context, userID uuid.UUID, code string, ready bool) (Room, error) {
	var room Room

	err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		var err error
		room, err = s.repo.GetRoomByCodeForUpdate(ctx, tx, code)
		if err != nil {
			return err
		}

		// Business Rule: ready toggle only allowed in waiting or ready status
		if room.Status != StatusWaiting && room.Status != StatusReady {
			return fmt.Errorf("cannot toggle ready: room is in status %s", room.Status)
		}

		activePlayers, err := s.repo.GetActivePlayersForUpdate(ctx, tx, room.ID)
		if err != nil {
			return err
		}

		// Find the target player
		var targetPlayer *RoomPlayer
		for i := range activePlayers {
			if activePlayers[i].UserID == userID {
				targetPlayer = &activePlayers[i]
				break
			}
		}

		if targetPlayer == nil {
			return errors.New("player is not an active participant in this room")
		}

		// Idempotency check
		targetStatus := PlayerJoined
		if ready {
			targetStatus = PlayerReady
		}

		if targetPlayer.Status == targetStatus {
			return nil // No-op (idempotency)
		}

		// Update database
		err = s.repo.UpdatePlayerStatus(ctx, tx, room.ID, userID, targetStatus)
		if err != nil {
			return err
		}

		// Re-evaluate Room Status based on all active players
		readyCount := 0
		for _, p := range activePlayers {
			status := p.Status
			if p.UserID == userID {
				status = targetStatus
			}
			if status == PlayerReady {
				readyCount++
			}
		}

		newRoomStatus := StatusWaiting
		if len(activePlayers) == 2 && readyCount == 2 {
			newRoomStatus = StatusReady
		}

		if room.Status != newRoomStatus {
			err = s.repo.UpdateRoomStatus(ctx, tx, room.ID, newRoomStatus)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return Room{}, err
	}

	return s.repo.GetRoomByCode(ctx, code)
}

// LeaveRoom processes a player leaving the lobby, handling host close rules.
func (s *Service) LeaveRoom(ctx context.Context, userID uuid.UUID, code string) error {
	return s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		room, err := s.repo.GetRoomByCodeForUpdate(ctx, tx, code)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil // Idempotency: Room doesn't exist
			}
			return err
		}

		if room.Status == StatusInBattle {
			return errors.New("cannot leave room while a battle is active")
		}

		activePlayers, err := s.repo.GetActivePlayersForUpdate(ctx, tx, room.ID)
		if err != nil {
			return err
		}

		var targetPlayer *RoomPlayer
		for i := range activePlayers {
			if activePlayers[i].UserID == userID {
				targetPlayer = &activePlayers[i]
				break
			}
		}

		if targetPlayer == nil {
			return nil // Idempotency: player already left
		}

		// Business Rule: If Host leaves, close the room and kick all players
		if room.HostUserID == userID {
			err = s.repo.UpdateRoomStatus(ctx, tx, room.ID, StatusClosed)
			if err != nil {
				return err
			}
			err = s.repo.MarkAllPlayersLeft(ctx, tx, room.ID)
			if err != nil {
				return err
			}
		} else {
			// Guest leaves: mark guest left
			err = s.repo.MarkPlayerLeft(ctx, tx, room.ID, userID)
			if err != nil {
				return err
			}
			// If room was ready, reset to waiting
			if room.Status == StatusReady {
				err = s.repo.UpdateRoomStatus(ctx, tx, room.ID, StatusWaiting)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// StartBattle launches a battle, changing room status to 'in_battle'.
func (s *Service) StartBattle(ctx context.Context, hostUserID uuid.UUID, code string) (uuid.UUID, error) {
	var battleID uuid.UUID

	err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		room, err := s.repo.GetRoomByCodeForUpdate(ctx, tx, code)
		if err != nil {
			return err
		}

		// Idempotency: if room is already in_battle, find the active battle and return it
		if room.Status == StatusInBattle {
			bID, err := s.repo.GetActiveBattleID(ctx, tx, room.ID)
			if err == nil {
				battleID = bID
				return nil // Idempotency success
			}
			if !errors.Is(err, ErrNotFound) {
				return fmt.Errorf("check active battle: %w", err)
			}
			// If no active battle row is found, proceed to create it
		}

		// Business Rule: only host can start the battle
		if room.HostUserID != hostUserID {
			return errors.New("only the host can start the battle")
		}

		// Business Rule: room must be ready to start the battle
		if room.Status != StatusReady {
			return fmt.Errorf("cannot start battle: room is in status %s (must be ready)", room.Status)
		}

		activePlayers, err := s.repo.GetActivePlayersForUpdate(ctx, tx, room.ID)
		if err != nil {
			return err
		}

		if len(activePlayers) != 2 {
			return errors.New("cannot start battle: room does not have exactly 2 active players")
		}

		// Load Elos for both players from user_stats, default to 1000 if not found
		battlePlayers := make([]BattlePlayer, len(activePlayers))
		for i, p := range activePlayers {
			rating, err := s.repo.GetPlayerRating(ctx, tx, p.UserID)
			if err != nil {
				return fmt.Errorf("load player rating: %w", err)
			}

			battlePlayers[i] = BattlePlayer{
				UserID:       p.UserID,
				SeatNumber:   p.SeatNumber,
				RatingBefore: rating,
			}
		}

		// Update room status
		err = s.repo.UpdateRoomStatus(ctx, tx, room.ID, StatusInBattle)
		if err != nil {
			return err
		}

		// Generate random seed
		seed, err := cryptoRandSeed()
		if err != nil {
			return fmt.Errorf("generate seed: %w", err)
		}

		// Call BattleCoordinator to start battle.
		// Note: The battleCoordinator is called. If battleCoordinator starts its own transaction,
		// we should be careful. To ensure complete atomic rollback, we call StartBattle.
		// If StartBattle fails, this transaction rolls back and room status reverts to StatusReady.
		bID, err := s.battleCoordinator.StartBattle(ctx, tx, room.ID, battlePlayers, seed)
		if err != nil {
			return fmt.Errorf("start battle coordinator: %w", err)
		}
		battleID = bID

		return nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	return battleID, nil
}

// ExpireRooms runs periodic cleanup of expired lobbies.
func (s *Service) ExpireRooms(ctx context.Context) (int, error) {
	var count int
	err := s.repo.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Select expired rooms ordered deterministically (e.g. ORDER BY id ASC) before FOR UPDATE
		rows, err := tx.Query(ctx, `
			SELECT id FROM rooms
			WHERE status IN ('waiting', 'ready') AND expires_at < NOW()
			ORDER BY id ASC
			FOR UPDATE
		`)
		if err != nil {
			return err
		}
		defer rows.Close()

		var ids []uuid.UUID
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}

		for _, id := range ids {
			if err := s.repo.UpdateRoomStatus(ctx, tx, id, StatusExpired); err != nil {
				return err
			}
			if err := s.repo.MarkAllPlayersLeft(ctx, tx, id); err != nil {
				return err
			}
			count++
		}

		return nil
	})
	return count, err
}

func generateCode() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 6)
	for i := range code {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		code[i] = chars[num.Int64()]
	}
	return string(code)
}

func cryptoRandSeed() (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}
