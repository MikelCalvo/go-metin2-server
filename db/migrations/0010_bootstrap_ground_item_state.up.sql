-- go-metin2 migration: 0010 bootstrap_ground_item_state up
CREATE TABLE bootstrap_ground_items (
    vid BIGINT PRIMARY KEY,
    vnum BIGINT NOT NULL,
    item_count INTEGER,
    gold_amount BIGINT,
    owner_login TEXT NOT NULL,
    owner_character_id BIGINT NOT NULL,
    owner_vid BIGINT NOT NULL,
    owner_name TEXT NOT NULL,
    map_index BIGINT NOT NULL,
    x INTEGER NOT NULL,
    y INTEGER NOT NULL,
    z INTEGER NOT NULL DEFAULT 0,
    pickup_range INTEGER NOT NULL DEFAULT 300,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_character_id) REFERENCES characters(id),
    CHECK (vid > 0 AND vid <= 4294967295),
    CHECK (vnum > 0 AND vnum <= 4294967295),
    CHECK (owner_login <> ''),
    CHECK (owner_character_id > 0),
    CHECK (owner_vid > 0 AND owner_vid <= 4294967295),
    CHECK (owner_name <> '' AND length(owner_name) <= 25),
    CHECK (map_index > 0),
    CHECK (pickup_range > 0),
    CHECK (
        (item_count IS NOT NULL AND item_count > 0 AND item_count <= 255 AND gold_amount IS NULL)
        OR (item_count IS NULL AND gold_amount IS NOT NULL AND gold_amount > 0 AND gold_amount <= 2147483647 AND vnum = 1)
    )
);

CREATE INDEX bootstrap_ground_items_map_index_index
    ON bootstrap_ground_items (map_index, x, y);

CREATE INDEX bootstrap_ground_items_owner_identity_index
    ON bootstrap_ground_items (owner_login, owner_character_id, owner_vid);
