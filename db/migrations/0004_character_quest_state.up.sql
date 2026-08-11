-- go-metin2 migration: 0004 character_quest_state up
CREATE TABLE character_quest_flags (
    character_id BIGINT NOT NULL,
    quest_ref TEXT NOT NULL,
    flag_name TEXT NOT NULL,
    value BIGINT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (character_id, quest_ref, flag_name),
    FOREIGN KEY (character_id) REFERENCES characters(id),
    CHECK (character_id > 0),
    CHECK (quest_ref <> ''),
    CHECK (flag_name <> ''),
    CHECK (value > 0)
);

CREATE INDEX character_quest_flags_quest_ref_index
    ON character_quest_flags (quest_ref);
