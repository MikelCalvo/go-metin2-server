-- go-metin2 migration: 0008 static_actor_content_state up
CREATE TABLE interaction_definitions (
    kind TEXT NOT NULL,
    ref TEXT NOT NULL,
    text TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    map_index BIGINT,
    x INTEGER,
    y INTEGER,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (kind, ref),
    CHECK (kind IN ('info', 'talk', 'warp', 'shop_preview')),
    CHECK (ref <> ''),
    CHECK (
        (kind IN ('info', 'talk') AND text <> '' AND title = '' AND map_index IS NULL AND x IS NULL AND y IS NULL)
        OR (kind = 'shop_preview' AND title <> '' AND text = '' AND map_index IS NULL AND x IS NULL AND y IS NULL)
        OR (kind = 'warp' AND text <> '' AND title = '' AND map_index > 0 AND x IS NOT NULL AND x <> 0 AND y IS NOT NULL AND y <> 0)
    )
);

CREATE TABLE interaction_merchant_catalog_entries (
    definition_kind TEXT NOT NULL,
    definition_ref TEXT NOT NULL,
    slot INTEGER NOT NULL,
    item_vnum BIGINT NOT NULL,
    price BIGINT NOT NULL,
    count INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (definition_kind, definition_ref, slot),
    FOREIGN KEY (definition_kind, definition_ref) REFERENCES interaction_definitions(kind, ref),
    CHECK (definition_kind = 'shop_preview'),
    CHECK (slot >= 0 AND slot < 40),
    CHECK (item_vnum > 0 AND item_vnum <= 4294967295),
    CHECK (price > 0 AND price <= 4294967295),
    CHECK (count > 0 AND count <= 255)
);

CREATE TABLE static_actors (
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
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (interaction_kind, interaction_ref) REFERENCES interaction_definitions(kind, ref),
    CHECK (entity_id > 0),
    CHECK (name <> ''),
    CHECK (map_index > 0),
    CHECK (race_num > 0 AND race_num <= 65535),
    CHECK ((spawn_home_map_index IS NULL AND spawn_home_x IS NULL AND spawn_home_y IS NULL) OR (spawn_home_map_index > 0 AND spawn_home_x IS NOT NULL AND spawn_home_y IS NOT NULL)),
    CHECK ((interaction_kind IS NULL AND interaction_ref IS NULL) OR (interaction_kind IN ('info', 'talk', 'warp', 'shop_preview') AND interaction_ref IS NOT NULL AND interaction_ref <> '')),
    CHECK (spawn_group_ref IS NULL OR spawn_group_ref <> ''),
    CHECK (spawn_group_ref IS NULL OR (combat_profile <> '' AND interaction_kind IS NULL AND interaction_ref IS NULL)),
    CHECK ((spawn_group_ref IS NOT NULL) OR (reward_experience = 0 AND reward_gold = 0)),
    CHECK (reward_experience >= 0),
    CHECK (reward_gold >= 0)
);

CREATE UNIQUE INDEX static_actors_spawn_group_ref_unique
    ON static_actors (spawn_group_ref)
    WHERE spawn_group_ref IS NOT NULL;

CREATE INDEX static_actors_map_index_index
    ON static_actors (map_index);

CREATE INDEX static_actors_interaction_ref_index
    ON static_actors (interaction_kind, interaction_ref)
    WHERE interaction_kind IS NOT NULL AND interaction_ref IS NOT NULL;

CREATE TABLE static_actor_reward_drops (
    entity_id BIGINT NOT NULL,
    position INTEGER NOT NULL,
    item_vnum BIGINT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (entity_id, position),
    FOREIGN KEY (entity_id) REFERENCES static_actors(entity_id),
    CHECK (position >= 0 AND position < 255),
    CHECK (item_vnum > 0 AND item_vnum <= 4294967295)
);
