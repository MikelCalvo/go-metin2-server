-- go-metin2 migration: 0018 static_actor_combat_profile_homeward_delay up
ALTER TABLE static_actor_combat_profiles
    ADD COLUMN homeward_delay_ms BIGINT NOT NULL DEFAULT 0
    CHECK (homeward_delay_ms = 0 OR (homeward_delay_ms >= 250 AND homeward_delay_ms <= 60000));
