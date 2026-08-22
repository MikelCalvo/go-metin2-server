-- go-metin2 migration: 0013 static_actor_combat_profile_state up
CREATE TABLE static_actor_combat_profiles (
    profile TEXT PRIMARY KEY,
    max_hp INTEGER NOT NULL,
    damage_per_normal_attack INTEGER NOT NULL,
    attack_value INTEGER NOT NULL,
    defense_value INTEGER NOT NULL,
    level INTEGER NOT NULL,
    rank INTEGER NOT NULL,
    respawn_delay_ms BIGINT NOT NULL,
    aggro_radius INTEGER NOT NULL DEFAULT 0,
    leash_radius INTEGER NOT NULL DEFAULT 0,
    retaliation_point_delta INTEGER NOT NULL DEFAULT 0,
    death_reward_experience BIGINT NOT NULL DEFAULT 0,
    death_reward_gold BIGINT NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (profile <> '' AND profile GLOB '[a-z]*' AND profile NOT GLOB '*[^a-z0-9_]*'),
    CHECK (profile NOT IN ('practice_mob', 'training_dummy')),
    CHECK (max_hp > 0 AND max_hp <= 255),
    CHECK (damage_per_normal_attack > 0 AND damage_per_normal_attack <= 255),
    CHECK (damage_per_normal_attack <= max_hp),
    CHECK (attack_value > 0 AND attack_value <= 65535),
    CHECK (defense_value >= 0 AND defense_value <= 65535),
    CHECK (level >= 0 AND level <= 65535),
    CHECK (rank >= 0 AND rank <= 255),
    CHECK (respawn_delay_ms > 0),
    CHECK (aggro_radius >= 0),
    CHECK (leash_radius >= 0),
    CHECK (retaliation_point_delta <= 0),
    CHECK (death_reward_experience >= 0 AND death_reward_experience <= 2147483647),
    CHECK (death_reward_gold >= 0 AND death_reward_gold <= 2147483647),
    CHECK (
        attack_value <= defense_value
        OR (attack_value - defense_value) <= max_hp
    ),
    CHECK (
        damage_per_normal_attack = CASE
            WHEN attack_value > defense_value THEN attack_value - defense_value
            ELSE 1
        END
    )
);

CREATE TABLE static_actor_combat_profile_death_reward_drops (
    profile TEXT NOT NULL,
    position INTEGER NOT NULL,
    item_vnum BIGINT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (profile, position),
    FOREIGN KEY (profile) REFERENCES static_actor_combat_profiles(profile),
    CHECK (profile <> ''),
    CHECK (position >= 0 AND position < 255),
    CHECK (item_vnum > 0 AND item_vnum <= 4294967295)
);

CREATE UNIQUE INDEX static_actor_combat_profile_death_reward_drops_profile_item_vnum_index
    ON static_actor_combat_profile_death_reward_drops (profile, item_vnum);

CREATE INDEX static_actor_combat_profile_death_reward_drops_profile_index
    ON static_actor_combat_profile_death_reward_drops (profile, position);
