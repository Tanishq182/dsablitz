-- Alter battles table to add battle seed (no default value)
ALTER TABLE battles
ADD COLUMN battle_seed BIGINT NOT NULL;

-- Create battle question sequence table
CREATE TABLE battle_question_sequence (
    battle_id UUID NOT NULL REFERENCES battles(id) ON DELETE CASCADE,
    sequence_index INTEGER NOT NULL,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE RESTRICT,
    PRIMARY KEY (battle_id, sequence_index)
);

-- Alter battle_players table to track player progression pointers
ALTER TABLE battle_players
ADD COLUMN current_question_index INTEGER NOT NULL DEFAULT 0,
ADD COLUMN current_question_attempts INTEGER NOT NULL DEFAULT 0;
