DROP TABLE IF EXISTS battle_question_sequence;

ALTER TABLE battles
DROP COLUMN IF EXISTS battle_seed;

ALTER TABLE battle_players
DROP COLUMN IF EXISTS current_question_index,
DROP COLUMN IF EXISTS current_question_attempts;
