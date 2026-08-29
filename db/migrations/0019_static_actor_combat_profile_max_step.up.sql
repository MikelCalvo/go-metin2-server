-- go-metin2 migration: 0019 static_actor_combat_profile_max_step up
ALTER TABLE static_actor_combat_profiles
    ADD COLUMN max_step INTEGER NOT NULL DEFAULT 0
    CHECK (max_step = 0 OR (max_step >= 1 AND max_step <= 1000));
