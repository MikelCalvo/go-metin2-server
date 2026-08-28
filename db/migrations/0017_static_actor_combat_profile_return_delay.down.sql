-- go-metin2 migration: 0017 static_actor_combat_profile_return_delay down
ALTER TABLE static_actor_combat_profiles DROP COLUMN return_delay_ms;
