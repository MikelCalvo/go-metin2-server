-- go-metin2 migration: 0003 character_item_state up
CREATE TABLE character_inventory_items (
    id BIGINT PRIMARY KEY,
    character_id BIGINT NOT NULL,
    slot INTEGER NOT NULL,
    vnum BIGINT NOT NULL,
    count INTEGER NOT NULL,
    locked INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (character_id) REFERENCES characters(id),
    CHECK (id > 0),
    CHECK (slot >= 0 AND slot < 90),
    CHECK (vnum > 0),
    CHECK (count > 0),
    CHECK (locked IN (0, 1))
);

CREATE UNIQUE INDEX character_inventory_items_character_slot_unique
    ON character_inventory_items (character_id, slot);

CREATE TABLE character_equipment_items (
    id BIGINT PRIMARY KEY,
    character_id BIGINT NOT NULL,
    equip_slot TEXT NOT NULL,
    vnum BIGINT NOT NULL,
    count INTEGER NOT NULL,
    locked INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (character_id) REFERENCES characters(id),
    CHECK (id > 0),
    CHECK (equip_slot IN ('body', 'weapon', 'head', 'hair', 'shield', 'wrist', 'shoes', 'neck', 'ear', 'unique1', 'unique2', 'arrow')),
    CHECK (vnum > 0),
    CHECK (count > 0),
    CHECK (locked IN (0, 1))
);

CREATE UNIQUE INDEX character_equipment_items_character_slot_unique
    ON character_equipment_items (character_id, equip_slot);

CREATE TABLE character_quickslots (
    character_id BIGINT NOT NULL,
    position INTEGER NOT NULL,
    type INTEGER NOT NULL,
    slot INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (character_id) REFERENCES characters(id),
    CHECK (position >= 0 AND position < 36),
    CHECK (
        (type = 0 AND slot = 0) OR
        (type = 1 AND slot >= 0 AND slot < 90) OR
        (type = 2 AND slot >= 0 AND slot < 200) OR
        (type = 3 AND slot >= 0 AND slot < 60)
    )
);

CREATE UNIQUE INDEX character_quickslots_character_position_unique
    ON character_quickslots (character_id, position);
