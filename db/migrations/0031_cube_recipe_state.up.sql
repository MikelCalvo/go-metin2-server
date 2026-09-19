-- go-metin2 migration: 0031 cube_recipe_state up
CREATE TABLE cube_recipe_npcs (
    npc_vnum BIGINT PRIMARY KEY,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (npc_vnum > 0 AND npc_vnum <= 4294967295)
);

CREATE TABLE cube_recipes (
    npc_vnum BIGINT NOT NULL,
    position INTEGER NOT NULL,
    reward_vnum BIGINT NOT NULL,
    reward_count INTEGER NOT NULL,
    gold BIGINT NOT NULL DEFAULT 0,
    percent INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (npc_vnum, position),
    FOREIGN KEY (npc_vnum) REFERENCES cube_recipe_npcs(npc_vnum),
    CHECK (npc_vnum > 0 AND npc_vnum <= 4294967295),
    CHECK (position >= 0),
    CHECK (reward_vnum > 0 AND reward_vnum <= 4294967295),
    CHECK (reward_count > 0 AND reward_count <= 65535),
    CHECK (gold >= 0 AND gold <= 9223372036854775807),
    CHECK (percent >= 0 AND percent <= 100)
);

CREATE INDEX cube_recipes_npc_vnum_index
    ON cube_recipes (npc_vnum);

CREATE TABLE cube_recipe_materials (
    npc_vnum BIGINT NOT NULL,
    recipe_position INTEGER NOT NULL,
    material_position INTEGER NOT NULL,
    item_vnum BIGINT NOT NULL,
    count INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (npc_vnum, recipe_position, material_position),
    FOREIGN KEY (npc_vnum, recipe_position) REFERENCES cube_recipes(npc_vnum, position),
    CHECK (npc_vnum > 0 AND npc_vnum <= 4294967295),
    CHECK (recipe_position >= 0),
    CHECK (material_position >= 0),
    CHECK (item_vnum > 0 AND item_vnum <= 4294967295),
    CHECK (count > 0 AND count <= 65535)
);

CREATE INDEX cube_recipe_materials_recipe_index
    ON cube_recipe_materials (npc_vnum, recipe_position);

CREATE TABLE cube_recipe_material_options (
    npc_vnum BIGINT NOT NULL,
    recipe_position INTEGER NOT NULL,
    option_index INTEGER NOT NULL,
    material_position INTEGER NOT NULL,
    item_vnum BIGINT NOT NULL,
    count INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (npc_vnum, recipe_position, option_index, material_position),
    FOREIGN KEY (npc_vnum, recipe_position) REFERENCES cube_recipes(npc_vnum, position),
    CHECK (npc_vnum > 0 AND npc_vnum <= 4294967295),
    CHECK (recipe_position >= 0),
    CHECK (option_index >= 0),
    CHECK (material_position >= 0),
    CHECK (item_vnum > 0 AND item_vnum <= 4294967295),
    CHECK (count > 0 AND count <= 65535)
);

CREATE INDEX cube_recipe_material_options_recipe_index
    ON cube_recipe_material_options (npc_vnum, recipe_position);
