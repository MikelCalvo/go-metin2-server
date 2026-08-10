-- go-metin2 migration: 0002 account_character_roster up
CREATE TABLE accounts (
    id BIGINT PRIMARY KEY,
    login TEXT NOT NULL,
    login_normalized TEXT NOT NULL,
    empire INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (login <> ''),
    CHECK (login_normalized <> ''),
    CHECK (empire >= 0)
);

CREATE UNIQUE INDEX accounts_login_normalized_unique
    ON accounts (login_normalized);

CREATE TABLE characters (
    id BIGINT PRIMARY KEY,
    account_id BIGINT NOT NULL,
    slot INTEGER NOT NULL,
    name TEXT NOT NULL,
    name_normalized TEXT NOT NULL,
    job INTEGER NOT NULL,
    race_num INTEGER NOT NULL,
    level INTEGER NOT NULL,
    play_minutes BIGINT NOT NULL DEFAULT 0,
    st INTEGER NOT NULL DEFAULT 0,
    ht INTEGER NOT NULL DEFAULT 0,
    dx INTEGER NOT NULL DEFAULT 0,
    iq INTEGER NOT NULL DEFAULT 0,
    main_part INTEGER NOT NULL DEFAULT 0,
    change_name INTEGER NOT NULL DEFAULT 0,
    hair_part INTEGER NOT NULL DEFAULT 0,
    x INTEGER NOT NULL DEFAULT 0,
    y INTEGER NOT NULL DEFAULT 0,
    z INTEGER NOT NULL DEFAULT 0,
    map_index INTEGER NOT NULL DEFAULT 1,
    empire INTEGER NOT NULL DEFAULT 0,
    skill_group INTEGER NOT NULL DEFAULT 0,
    guild_id BIGINT NOT NULL DEFAULT 0,
    guild_name TEXT NOT NULL DEFAULT '',
    gold BIGINT NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    CHECK (slot >= 0 AND slot < 4),
    CHECK (name <> ''),
    CHECK (name_normalized <> ''),
    CHECK (level >= 1),
    CHECK (map_index > 0),
    CHECK (gold >= 0)
);

CREATE UNIQUE INDEX characters_account_slot_unique
    ON characters (account_id, slot);

CREATE UNIQUE INDEX characters_name_normalized_unique
    ON characters (name_normalized);
