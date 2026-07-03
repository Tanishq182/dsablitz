-- Remove table-level unique constraints that block seats/users for left/kicked status
ALTER TABLE room_players DROP CONSTRAINT IF EXISTS room_players_room_user_unique;
ALTER TABLE room_players DROP CONSTRAINT IF EXISTS room_players_room_seat_unique;

-- Create partial unique indexes that apply only to active players (joined or ready)
CREATE UNIQUE INDEX idx_room_players_room_user_active 
    ON room_players(room_id, user_id) 
    WHERE status IN ('joined', 'ready');

CREATE UNIQUE INDEX idx_room_players_room_seat_active 
    ON room_players(room_id, seat_number) 
    WHERE status IN ('joined', 'ready');

-- Create partial unique index on battles to prevent multiple active battles in the same room
CREATE UNIQUE INDEX idx_battles_active_room_unique 
    ON battles(room_id) 
    WHERE status IN ('created', 'countdown', 'active');
