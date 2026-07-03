-- Drop the partial unique indexes
DROP INDEX IF EXISTS idx_battles_active_room_unique;
DROP INDEX IF EXISTS idx_room_players_room_seat_active;
DROP INDEX IF EXISTS idx_room_players_room_user_active;

-- Re-create the original table-level unique constraints
ALTER TABLE room_players 
    ADD CONSTRAINT room_players_room_user_unique UNIQUE (room_id, user_id),
    ADD CONSTRAINT room_players_room_seat_unique UNIQUE (room_id, seat_number);
