-- go-metin2 migration: 0017 static_actor_combat_profile_return_delay up
ALTER TABLE static_actor_combat_profiles
    ADD COLUMN return_delay_ms BIGINT NOT NULL DEFAULT 0
    CHECK (return_delay_ms = 0 OR (return_delay_ms >= 250 AND return_delay_ms <= 60000));
