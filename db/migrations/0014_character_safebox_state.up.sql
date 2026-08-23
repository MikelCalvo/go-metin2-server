-- go-metin2 migration: 0014 character_safebox_state up
CREATE TABLE character_safebox_passwords (
    character_id BIGINT PRIMARY KEY,
    login TEXT NOT NULL,
    password TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (character_id) REFERENCES characters(id),
    CHECK (character_id > 0),
    CHECK (login <> ''),
    CHECK (length(password) <= 6),
    CHECK (password = '' OR password GLOB '[A-Za-z0-9]*')
);

CREATE INDEX character_safebox_passwords_login_index
    ON character_safebox_passwords (login);

CREATE TABLE character_safebox_items (
    id BIGINT PRIMARY KEY,
    character_id BIGINT NOT NULL,
    login TEXT NOT NULL,
    cell INTEGER NOT NULL,
    vnum BIGINT NOT NULL,
    count INTEGER NOT NULL,
    locked INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (character_id) REFERENCES characters(id),
    CHECK (id > 0),
    CHECK (character_id > 0),
    CHECK (login <> ''),
    CHECK (cell >= 0 AND cell < 15),
    CHECK (vnum > 0),
    CHECK (count > 0),
    CHECK (locked IN (0, 1))
);

CREATE UNIQUE INDEX character_safebox_items_character_cell_unique
    ON character_safebox_items (character_id, cell);

CREATE INDEX character_safebox_items_character_index
    ON character_safebox_items (character_id);

CREATE INDEX character_safebox_items_login_index
    ON character_safebox_items (login);
