-- go-metin2 migration: 0020 static_actor_combat_profile_reaction_delay up
ALTER TABLE static_actor_combat_profiles
    ADD COLUMN reaction_delay_ms BIGINT NOT NULL DEFAULT 0
    CHECK (reaction_delay_ms = 0 OR (reaction_delay_ms >= 250 AND reaction_delay_ms <= 60000));
