-- go-metin2 migration: 0012 static_actor_pve_interaction_state up
CREATE TABLE interaction_definitions_mig12 (
    kind TEXT NOT NULL,
    ref TEXT NOT NULL,
    text TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    map_index BIGINT,
    x INTEGER,
    y INTEGER,
    size INTEGER NOT NULL DEFAULT 0,
    quest_ref TEXT NOT NULL DEFAULT '',
    quest_flag TEXT NOT NULL DEFAULT '',
    quest_from BIGINT NOT NULL DEFAULT 0,
    quest_to BIGINT NOT NULL DEFAULT 0,
    reward_experience BIGINT NOT NULL DEFAULT 0,
    reward_gold BIGINT NOT NULL DEFAULT 0,
    consume_gold BIGINT NOT NULL DEFAULT 0,
    consume_experience BIGINT NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (kind, ref),
    CHECK (kind IN ('info', 'talk', 'warp', 'shop_preview', 'open_safebox', 'quest_flag')),
    CHECK (ref <> ''),
    CHECK (size >= 0 AND size <= 3),
    CHECK (quest_from >= 0 AND quest_from <= 4294967295),
    CHECK (quest_to >= 0 AND quest_to <= 4294967295),
    CHECK (reward_experience >= 0 AND reward_experience <= 2147483647),
    CHECK (reward_gold >= 0 AND reward_gold <= 2147483647),
    CHECK (consume_gold >= 0 AND consume_gold <= 2147483647),
    CHECK (consume_experience >= 0 AND consume_experience <= 2147483647),
    CHECK (
        (
            kind IN ('info', 'talk')
            AND text <> ''
            AND title = ''
            AND map_index IS NULL
            AND x IS NULL
            AND y IS NULL
            AND size = 0
            AND reward_experience = 0
            AND reward_gold = 0
            AND consume_gold = 0
            AND consume_experience = 0
            AND (
                (quest_ref = '' AND quest_flag = '' AND quest_from = 0 AND quest_to = 0)
                OR (quest_ref <> '' AND quest_flag <> '' AND quest_to = 0)
            )
        )
        OR (
            kind = 'shop_preview'
            AND title <> ''
            AND text = ''
            AND map_index IS NULL
            AND x IS NULL
            AND y IS NULL
            AND size = 0
            AND reward_experience = 0
            AND reward_gold = 0
            AND consume_gold = 0
            AND consume_experience = 0
            AND (
                (quest_ref = '' AND quest_flag = '' AND quest_from = 0 AND quest_to = 0)
                OR (quest_ref <> '' AND quest_flag <> '' AND quest_to = 0)
            )
        )
        OR (
            kind = 'warp'
            AND text <> ''
            AND title = ''
            AND map_index > 0
            AND x IS NOT NULL
            AND x <> 0
            AND y IS NOT NULL
            AND y <> 0
            AND size = 0
            AND reward_experience = 0
            AND reward_gold = 0
            AND consume_gold = 0
            AND consume_experience = 0
            AND (
                (quest_ref = '' AND quest_flag = '' AND quest_from = 0 AND quest_to = 0)
                OR (quest_ref <> '' AND quest_flag <> '' AND quest_to = 0)
            )
        )
        OR (
            kind = 'open_safebox'
            AND text <> ''
            AND title = ''
            AND map_index IS NULL
            AND x IS NULL
            AND y IS NULL
            AND size >= 0
            AND size <= 3
            AND reward_experience = 0
            AND reward_gold = 0
            AND consume_gold = 0
            AND consume_experience = 0
            AND (
                (quest_ref = '' AND quest_flag = '' AND quest_from = 0 AND quest_to = 0)
                OR (quest_ref <> '' AND quest_flag <> '' AND quest_to = 0)
            )
        )
        OR (
            kind = 'quest_flag'
            AND text <> ''
            AND title = ''
            AND map_index IS NULL
            AND x IS NULL
            AND y IS NULL
            AND size = 0
            AND quest_ref <> ''
            AND quest_flag <> ''
            AND quest_from <> quest_to
        )
    )
);

INSERT INTO interaction_definitions_mig12 (
    kind, ref, text, title, map_index, x, y, created_at, updated_at
)
SELECT kind, ref, text, title, map_index, x, y, created_at, updated_at
FROM interaction_definitions;

CREATE TABLE interaction_merchant_catalog_entries_mig12 (
    definition_kind TEXT NOT NULL,
    definition_ref TEXT NOT NULL,
    slot INTEGER NOT NULL,
    item_vnum BIGINT NOT NULL,
    price BIGINT NOT NULL,
    count INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (definition_kind, definition_ref, slot),
    FOREIGN KEY (definition_kind, definition_ref) REFERENCES interaction_definitions_mig12(kind, ref),
    CHECK (definition_kind = 'shop_preview'),
    CHECK (slot >= 0 AND slot < 40),
    CHECK (item_vnum > 0 AND item_vnum <= 4294967295),
    CHECK (price > 0 AND price <= 4294967295),
    CHECK (count > 0 AND count <= 255)
);

INSERT INTO interaction_merchant_catalog_entries_mig12 (
    definition_kind, definition_ref, slot, item_vnum, price, count, created_at, updated_at
)
SELECT definition_kind, definition_ref, slot, item_vnum, price, count, created_at, updated_at
FROM interaction_merchant_catalog_entries;

CREATE TABLE static_actors_mig12 (
    entity_id BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    map_index BIGINT NOT NULL,
    x INTEGER NOT NULL,
    y INTEGER NOT NULL,
    race_num BIGINT NOT NULL,
    spawn_home_map_index BIGINT,
    spawn_home_x INTEGER,
    spawn_home_y INTEGER,
    combat_profile TEXT NOT NULL DEFAULT '',
    interaction_kind TEXT,
    interaction_ref TEXT,
    spawn_group_ref TEXT,
    reward_experience BIGINT NOT NULL DEFAULT 0,
    reward_gold BIGINT NOT NULL DEFAULT 0,
    reward_quest_ref TEXT NOT NULL DEFAULT '',
    reward_quest_flag TEXT NOT NULL DEFAULT '',
    reward_quest_from BIGINT NOT NULL DEFAULT 0,
    reward_quest_to BIGINT NOT NULL DEFAULT 0,
    reward_quest_text TEXT NOT NULL DEFAULT '',
    require_quest_ref TEXT NOT NULL DEFAULT '',
    require_quest_flag TEXT NOT NULL DEFAULT '',
    require_quest_from BIGINT NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (interaction_kind, interaction_ref) REFERENCES interaction_definitions_mig12(kind, ref),
    CHECK (entity_id > 0),
    CHECK (name <> ''),
    CHECK (map_index > 0),
    CHECK (race_num > 0 AND race_num <= 65535),
    CHECK ((spawn_home_map_index IS NULL AND spawn_home_x IS NULL AND spawn_home_y IS NULL) OR (spawn_home_map_index > 0 AND spawn_home_x IS NOT NULL AND spawn_home_y IS NOT NULL)),
    CHECK ((interaction_kind IS NULL AND interaction_ref IS NULL) OR (interaction_kind IN ('info', 'talk', 'warp', 'shop_preview', 'open_safebox', 'quest_flag') AND interaction_ref IS NOT NULL AND interaction_ref <> '')),
    CHECK (spawn_group_ref IS NULL OR spawn_group_ref <> ''),
    CHECK (spawn_group_ref IS NULL OR (combat_profile <> '' AND interaction_kind IS NULL AND interaction_ref IS NULL)),
    CHECK ((spawn_group_ref IS NOT NULL) OR (reward_experience = 0 AND reward_gold = 0 AND reward_quest_ref = '' AND reward_quest_flag = '' AND reward_quest_from = 0 AND reward_quest_to = 0 AND reward_quest_text = '' AND require_quest_ref = '' AND require_quest_flag = '' AND require_quest_from = 0)),
    CHECK (reward_experience >= 0),
    CHECK (reward_gold >= 0),
    CHECK (reward_quest_from >= 0 AND reward_quest_from <= 4294967295),
    CHECK (reward_quest_to >= 0 AND reward_quest_to <= 4294967295),
    CHECK (require_quest_from >= 0 AND require_quest_from <= 4294967295),
    CHECK (
        (
            reward_quest_ref = ''
            AND reward_quest_flag = ''
            AND reward_quest_from = 0
            AND reward_quest_to = 0
            AND reward_quest_text = ''
            AND require_quest_ref = ''
            AND require_quest_flag = ''
            AND require_quest_from = 0
        )
        OR (
            spawn_group_ref IS NOT NULL
            AND reward_quest_ref <> ''
            AND reward_quest_flag <> ''
            AND reward_quest_from <> reward_quest_to
            AND (
                (require_quest_ref = '' AND require_quest_flag = '' AND require_quest_from = 0)
                OR (require_quest_ref <> '' AND require_quest_flag <> '')
            )
        )
    )
);

INSERT INTO static_actors_mig12 (
    entity_id, name, map_index, x, y, race_num,
    spawn_home_map_index, spawn_home_x, spawn_home_y,
    combat_profile, interaction_kind, interaction_ref, spawn_group_ref,
    reward_experience, reward_gold, created_at, updated_at
)
SELECT
    entity_id, name, map_index, x, y, race_num,
    spawn_home_map_index, spawn_home_x, spawn_home_y,
    combat_profile, interaction_kind, interaction_ref, spawn_group_ref,
    reward_experience, reward_gold, created_at, updated_at
FROM static_actors;

CREATE TABLE static_actor_reward_drops_mig12 (
    entity_id BIGINT NOT NULL,
    position INTEGER NOT NULL,
    item_vnum BIGINT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (entity_id, position),
    FOREIGN KEY (entity_id) REFERENCES static_actors_mig12(entity_id),
    CHECK (position >= 0 AND position < 255),
    CHECK (item_vnum > 0 AND item_vnum <= 4294967295)
);

INSERT INTO static_actor_reward_drops_mig12 (
    entity_id, position, item_vnum, created_at, updated_at
)
SELECT entity_id, position, item_vnum, created_at, updated_at
FROM static_actor_reward_drops;

DROP TABLE static_actor_reward_drops;
DROP INDEX static_actors_interaction_ref_index;
DROP INDEX static_actors_map_index_index;
DROP INDEX static_actors_spawn_group_ref_unique;
DROP TABLE static_actors;
DROP TABLE interaction_merchant_catalog_entries;
DROP TABLE interaction_definitions;

ALTER TABLE interaction_definitions_mig12 RENAME TO interaction_definitions;
ALTER TABLE interaction_merchant_catalog_entries_mig12 RENAME TO interaction_merchant_catalog_entries;
ALTER TABLE static_actors_mig12 RENAME TO static_actors;
ALTER TABLE static_actor_reward_drops_mig12 RENAME TO static_actor_reward_drops;

CREATE UNIQUE INDEX static_actors_spawn_group_ref_unique
    ON static_actors (spawn_group_ref)
    WHERE spawn_group_ref IS NOT NULL;

CREATE INDEX static_actors_map_index_index
    ON static_actors (map_index);

CREATE INDEX static_actors_interaction_ref_index
    ON static_actors (interaction_kind, interaction_ref)
    WHERE interaction_kind IS NOT NULL AND interaction_ref IS NOT NULL;

CREATE TABLE interaction_quest_flag_reward_items (
    definition_kind TEXT NOT NULL,
    definition_ref TEXT NOT NULL,
    position INTEGER NOT NULL,
    item_vnum BIGINT NOT NULL,
    count INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (definition_kind, definition_ref, position),
    FOREIGN KEY (definition_kind, definition_ref) REFERENCES interaction_definitions(kind, ref),
    CHECK (definition_kind = 'quest_flag'),
    CHECK (position >= 0 AND position < 8),
    CHECK (item_vnum > 0 AND item_vnum <= 4294967295),
    CHECK (count > 0 AND count <= 255)
);

CREATE TABLE interaction_quest_flag_consume_items (
    definition_kind TEXT NOT NULL,
    definition_ref TEXT NOT NULL,
    position INTEGER NOT NULL,
    item_vnum BIGINT NOT NULL,
    count INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (definition_kind, definition_ref, position),
    FOREIGN KEY (definition_kind, definition_ref) REFERENCES interaction_definitions(kind, ref),
    CHECK (definition_kind = 'quest_flag'),
    CHECK (position >= 0 AND position < 8),
    CHECK (item_vnum > 0 AND item_vnum <= 4294967295),
    CHECK (count > 0 AND count <= 255)
);
