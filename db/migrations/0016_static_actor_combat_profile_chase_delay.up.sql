-- go-metin2 migration: 0016 static_actor_combat_profile_chase_delay up
ALTER TABLE static_actor_combat_profiles
    ADD COLUMN chase_delay_ms BIGINT NOT NULL DEFAULT 0
    CHECK (chase_delay_ms = 0 OR (chase_delay_ms > 1000 AND chase_delay_ms <= 60000));
