-- go-metin2 migration: 0023 character_myshop_unit_prices up
CREATE TABLE character_myshop_unit_prices (
    character_id BIGINT NOT NULL,
    vnum BIGINT NOT NULL,
    unit_price BIGINT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (character_id, vnum),
    FOREIGN KEY (character_id) REFERENCES characters(id),
    CHECK (character_id > 0),
    CHECK (vnum > 0 AND vnum <= 4294967295),
    CHECK (unit_price >= 0 AND unit_price <= 4294967295)
);

CREATE INDEX character_myshop_unit_prices_character_index
    ON character_myshop_unit_prices (character_id);
