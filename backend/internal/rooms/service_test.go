package rooms

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Mock BattleCoordinator
type mockBattleCoordinator struct {
	startedBattles map[uuid.UUID][]BattlePlayer
	mockBattleID   uuid.UUID
	err            error
}

func (m *mockBattleCoordinator) StartBattle(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, players []BattlePlayer, seed int64) (uuid.UUID, error) {
	if m.err != nil {
		return uuid.Nil, m.err
	}
	m.startedBattles[roomID] = players
	return m.mockBattleID, nil
}

// Mock RoomRepository
type mockRoomRepository struct {
	rooms          map[uuid.UUID]Room
	roomsByCode    map[string]Room
	roomPlayers    map[uuid.UUID]map[uuid.UUID]RoomPlayer // roomID -> userID -> RoomPlayer
	ratings        map[uuid.UUID]int
	activeBattleID uuid.UUID
}

func newMockRoomRepository() *mockRoomRepository {
	return &mockRoomRepository{
		rooms:       make(map[uuid.UUID]Room),
		roomsByCode: make(map[string]Room),
		roomPlayers: make(map[uuid.UUID]map[uuid.UUID]RoomPlayer),
		ratings:     make(map[uuid.UUID]int),
	}
}

func (m *mockRoomRepository) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

func (m *mockRoomRepository) InsertRoom(ctx context.Context, tx pgx.Tx, room Room) error {
	m.rooms[room.ID] = room
	m.roomsByCode[room.Code] = room
	return nil
}

func (m *mockRoomRepository) GetRoomByCode(ctx context.Context, code string) (Room, error) {
	r, ok := m.roomsByCode[code]
	if !ok {
		return Room{}, ErrNotFound
	}
	return r, nil
}

func (m *mockRoomRepository) GetRoomByCodeForUpdate(ctx context.Context, tx pgx.Tx, code string) (Room, error) {
	return m.GetRoomByCode(ctx, code)
}

func (m *mockRoomRepository) GetRoomForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Room, error) {
	r, ok := m.rooms[id]
	if !ok {
		return Room{}, ErrNotFound
	}
	return r, nil
}

func (m *mockRoomRepository) UpdateRoomStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status RoomStatus) error {
	r, ok := m.rooms[id]
	if !ok {
		return ErrNotFound
	}
	r.Status = status
	m.rooms[id] = r
	m.roomsByCode[r.Code] = r
	return nil
}

func (m *mockRoomRepository) InsertRoomPlayer(ctx context.Context, tx pgx.Tx, p RoomPlayer) error {
	if _, ok := m.roomPlayers[p.RoomID]; !ok {
		m.roomPlayers[p.RoomID] = make(map[uuid.UUID]RoomPlayer)
	}
	m.roomPlayers[p.RoomID][p.UserID] = p
	return nil
}

func (m *mockRoomRepository) GetRoomPlayer(ctx context.Context, roomID, userID uuid.UUID) (RoomPlayer, error) {
	players, ok := m.roomPlayers[roomID]
	if !ok {
		return RoomPlayer{}, ErrNotFound
	}
	p, ok := players[userID]
	if !ok {
		return RoomPlayer{}, ErrNotFound
	}
	return p, nil
}

func (m *mockRoomRepository) GetActivePlayersForUpdate(ctx context.Context, tx pgx.Tx, roomID uuid.UUID) ([]RoomPlayer, error) {
	playersMap, ok := m.roomPlayers[roomID]
	if !ok {
		return nil, nil
	}
	var list []RoomPlayer
	for _, p := range playersMap {
		if p.Status == PlayerJoined || p.Status == PlayerReady {
			list = append(list, p)
		}
	}
	return list, nil
}

func (m *mockRoomRepository) UpdatePlayerStatus(ctx context.Context, tx pgx.Tx, roomID, userID uuid.UUID, status RoomPlayerStatus) error {
	players, ok := m.roomPlayers[roomID]
	if !ok {
		return ErrNotFound
	}
	p, ok := players[userID]
	if !ok {
		return ErrNotFound
	}
	p.Status = status
	players[userID] = p
	return nil
}

func (m *mockRoomRepository) DeleteRoomPlayer(ctx context.Context, tx pgx.Tx, roomID, userID uuid.UUID) error {
	players, ok := m.roomPlayers[roomID]
	if !ok {
		return nil
	}
	delete(players, userID)
	return nil
}

func (m *mockRoomRepository) MarkPlayerLeft(ctx context.Context, tx pgx.Tx, roomID, userID uuid.UUID) error {
	players, ok := m.roomPlayers[roomID]
	if !ok {
		return ErrNotFound
	}
	p, ok := players[userID]
	if !ok {
		return ErrNotFound
	}
	p.Status = PlayerLeft
	now := time.Now()
	p.LeftAt = &now
	players[userID] = p
	return nil
}

func (m *mockRoomRepository) MarkAllPlayersLeft(ctx context.Context, tx pgx.Tx, roomID uuid.UUID) error {
	players, ok := m.roomPlayers[roomID]
	if !ok {
		return nil
	}
	now := time.Now()
	for uID, p := range players {
		if p.Status == PlayerJoined || p.Status == PlayerReady {
			p.Status = PlayerLeft
			p.LeftAt = &now
			players[uID] = p
		}
	}
	return nil
}

func (m *mockRoomRepository) GetPlayerRating(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (int, error) {
	rating, ok := m.ratings[userID]
	if !ok {
		return 1000, nil
	}
	return rating, nil
}

func (m *mockRoomRepository) GetActiveBattleID(ctx context.Context, tx pgx.Tx, roomID uuid.UUID) (uuid.UUID, error) {
	if m.activeBattleID == uuid.Nil {
		return uuid.Nil, ErrNotFound
	}
	return m.activeBattleID, nil
}

func TestRoomService_CreateRoom(t *testing.T) {
	repo := newMockRoomRepository()
	bc := &mockBattleCoordinator{}
	service := NewService(repo, bc)

	hostID := uuid.New()
	room, err := service.CreateRoom(context.Background(), hostID, 120)
	if err != nil {
		t.Fatalf("unexpected error creating room: %v", err)
	}

	if room.HostUserID != hostID {
		t.Errorf("expected host ID %s, got %s", hostID, room.HostUserID)
	}
	if room.Status != StatusWaiting {
		t.Errorf("expected room status waiting, got %s", room.Status)
	}
	if len(room.Code) != 6 {
		t.Errorf("expected 6-char room code, got %s", room.Code)
	}

	// Verify host is seated at seat 1
	player, err := repo.GetRoomPlayer(context.Background(), room.ID, hostID)
	if err != nil {
		t.Fatalf("failed to retrieve host room player: %v", err)
	}
	if player.SeatNumber != 1 || player.Status != PlayerJoined {
		t.Errorf("host player initialized incorrectly: %+v", player)
	}
}

func TestRoomService_JoinRoom(t *testing.T) {
	repo := newMockRoomRepository()
	bc := &mockBattleCoordinator{}
	service := NewService(repo, bc)

	hostID := uuid.New()
	guestID := uuid.New()

	room, err := service.CreateRoom(context.Background(), hostID, 120)
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	joinedRoom, err := service.JoinRoom(context.Background(), guestID, room.Code)
	if err != nil {
		t.Fatalf("failed to join room: %v", err)
	}

	activePlayers, _ := repo.GetActivePlayersForUpdate(context.Background(), nil, joinedRoom.ID)
	if len(activePlayers) != 2 {
		t.Errorf("expected 2 active players, got %d", len(activePlayers))
	}

	guestPlayer, err := repo.GetRoomPlayer(context.Background(), room.ID, guestID)
	if err != nil {
		t.Fatalf("failed to find guest player: %v", err)
	}
	if guestPlayer.SeatNumber != 2 || guestPlayer.Status != PlayerJoined {
		t.Errorf("guest player seated incorrectly: %+v", guestPlayer)
	}
}

func TestRoomService_JoinRoom_Idempotency(t *testing.T) {
	repo := newMockRoomRepository()
	bc := &mockBattleCoordinator{}
	service := NewService(repo, bc)

	hostID := uuid.New()
	guestID := uuid.New()

	room, _ := service.CreateRoom(context.Background(), hostID, 120)
	_, _ = service.JoinRoom(context.Background(), guestID, room.Code)

	// Join again
	joinedRoom, err := service.JoinRoom(context.Background(), guestID, room.Code)
	if err != nil {
		t.Errorf("expected second join to succeed (idempotent), got error: %v", err)
	}

	activePlayers, _ := repo.GetActivePlayersForUpdate(context.Background(), nil, joinedRoom.ID)
	if len(activePlayers) != 2 {
		t.Errorf("expected player count to remain 2, got %d", len(activePlayers))
	}
}

func TestRoomService_ToggleReady(t *testing.T) {
	repo := newMockRoomRepository()
	bc := &mockBattleCoordinator{}
	service := NewService(repo, bc)

	hostID := uuid.New()
	guestID := uuid.New()

	room, _ := service.CreateRoom(context.Background(), hostID, 120)
	_, _ = service.JoinRoom(context.Background(), guestID, room.Code)

	// Host readies up
	room, err := service.ToggleReady(context.Background(), hostID, room.Code, true)
	if err != nil {
		t.Fatalf("unexpected ready error: %v", err)
	}
	if room.Status != StatusWaiting {
		t.Errorf("expected room to stay waiting when only 1 is ready, got %s", room.Status)
	}

	// Guest readies up
	room, err = service.ToggleReady(context.Background(), guestID, room.Code, true)
	if err != nil {
		t.Fatalf("unexpected ready error: %v", err)
	}
	if room.Status != StatusReady {
		t.Errorf("expected room to transition to ready when both are ready, got %s", room.Status)
	}

	// Guest unreadies
	room, err = service.ToggleReady(context.Background(), guestID, room.Code, false)
	if err != nil {
		t.Fatalf("unexpected unready error: %v", err)
	}
	if room.Status != StatusWaiting {
		t.Errorf("expected room to transition back to waiting when player unreadies, got %s", room.Status)
	}
}

func TestRoomService_LeaveRoom_Host(t *testing.T) {
	repo := newMockRoomRepository()
	bc := &mockBattleCoordinator{}
	service := NewService(repo, bc)

	hostID := uuid.New()
	guestID := uuid.New()

	room, _ := service.CreateRoom(context.Background(), hostID, 120)
	_, _ = service.JoinRoom(context.Background(), guestID, room.Code)

	// Host leaves
	err := service.LeaveRoom(context.Background(), hostID, room.Code)
	if err != nil {
		t.Fatalf("unexpected leave error: %v", err)
	}

	room, _ = repo.GetRoomByCode(context.Background(), room.Code)
	if room.Status != StatusClosed {
		t.Errorf("expected room status to be closed, got %s", room.Status)
	}

	// Verify all players marked left
	hp, _ := repo.GetRoomPlayer(context.Background(), room.ID, hostID)
	gp, _ := repo.GetRoomPlayer(context.Background(), room.ID, guestID)

	if hp.Status != PlayerLeft || gp.Status != PlayerLeft {
		t.Errorf("expected players to be marked left, got host status %s, guest status %s", hp.Status, gp.Status)
	}
}

func TestRoomService_StartBattle(t *testing.T) {
	repo := newMockRoomRepository()
	mockBattleID := uuid.New()
	bc := &mockBattleCoordinator{
		startedBattles: make(map[uuid.UUID][]BattlePlayer),
		mockBattleID:   mockBattleID,
	}
	service := NewService(repo, bc)

	hostID := uuid.New()
	guestID := uuid.New()

	room, _ := service.CreateRoom(context.Background(), hostID, 120)
	_, _ = service.JoinRoom(context.Background(), guestID, room.Code)

	// Ready both players
	_, _ = service.ToggleReady(context.Background(), hostID, room.Code, true)
	room, _ = service.ToggleReady(context.Background(), guestID, room.Code, true)

	// Host starts battle
	bID, err := service.StartBattle(context.Background(), hostID, room.Code)
	if err != nil {
		t.Fatalf("unexpected start battle error: %v", err)
	}

	if bID != mockBattleID {
		t.Errorf("expected battle ID %s, got %s", mockBattleID, bID)
	}

	room, _ = repo.GetRoomByCode(context.Background(), room.Code)
	if room.Status != StatusInBattle {
		t.Errorf("expected room status to be in_battle, got %s", room.Status)
	}

	// Verify idempotency
	repo.activeBattleID = mockBattleID // simulate DB having an active battle for this room
	bID2, err := service.StartBattle(context.Background(), hostID, room.Code)
	if err != nil {
		t.Fatalf("unexpected start battle error on retry: %v", err)
	}
	if bID2 != mockBattleID {
		t.Errorf("expected same battle ID %s on retry, got %s", mockBattleID, bID2)
	}
}

func TestRoomService_LeaveRoom_ActiveBattle(t *testing.T) {
	repo := newMockRoomRepository()
	bc := &mockBattleCoordinator{}
	service := NewService(repo, bc)

	hostID := uuid.New()
	guestID := uuid.New()

	room, _ := service.CreateRoom(context.Background(), hostID, 120)
	_, _ = service.JoinRoom(context.Background(), guestID, room.Code)

	// Simulate active battle by setting room status to StatusInBattle
	_ = repo.UpdateRoomStatus(context.Background(), nil, room.ID, StatusInBattle)

	// Host tries to leave, should fail
	err := service.LeaveRoom(context.Background(), hostID, room.Code)
	if err == nil {
		t.Error("expected error when host tries to leave room during active battle, got nil")
	}
	if err.Error() != "cannot leave room while a battle is active" {
		t.Errorf("expected 'cannot leave room while a battle is active', got: %v", err)
	}
}
