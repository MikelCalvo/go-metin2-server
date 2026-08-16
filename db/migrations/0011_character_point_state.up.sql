-- go-metin2 migration: 0011 character_point_state up
CREATE TABLE character_points (
    character_id BIGINT NOT NULL,
    point_index INTEGER NOT NULL,
    value BIGINT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (character_id, point_index),
    FOREIGN KEY (character_id) REFERENCES characters(id),
    CHECK (character_id > 0),
    CHECK (point_index >= 0 AND point_index < 255),
    CHECK (value >= -2147483648 AND value <= 2147483647)
);

CREATE INDEX character_points_character_index
    ON character_points (character_id);
