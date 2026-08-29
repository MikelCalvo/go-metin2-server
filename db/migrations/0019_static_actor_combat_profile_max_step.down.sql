-- go-metin2 migration: 0019 static_actor_combat_profile_max_step down
ALTER TABLE static_actor_combat_profiles DROP COLUMN max_step;
