DROP TRIGGER IF EXISTS set_question_stats_updated_at ON question_stats;
DROP TRIGGER IF EXISTS set_user_stats_updated_at ON user_stats;
DROP TRIGGER IF EXISTS set_questions_updated_at ON questions;
DROP TRIGGER IF EXISTS set_battle_players_updated_at ON battle_players;
DROP TRIGGER IF EXISTS set_battles_updated_at ON battles;
DROP TRIGGER IF EXISTS set_room_players_updated_at ON room_players;
DROP TRIGGER IF EXISTS set_rooms_updated_at ON rooms;
DROP TRIGGER IF EXISTS set_friendships_updated_at ON friendships;
DROP TRIGGER IF EXISTS set_oauth_accounts_updated_at ON oauth_accounts;
DROP TRIGGER IF EXISTS set_users_updated_at ON users;

DROP TABLE IF EXISTS rating_history;
DROP TABLE IF EXISTS question_stats;
DROP TABLE IF EXISTS user_stats;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS battle_players;
DROP TABLE IF EXISTS battles;
DROP TABLE IF EXISTS room_players;
DROP TABLE IF EXISTS rooms;
DROP TABLE IF EXISTS friendships;
DROP TABLE IF EXISTS oauth_accounts;
DROP TABLE IF EXISTS users;

DROP FUNCTION IF EXISTS set_updated_at();
DROP EXTENSION IF EXISTS citext;
DROP EXTENSION IF EXISTS pgcrypto;
