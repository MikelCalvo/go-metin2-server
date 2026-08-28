-- go-metin2 migration: 0018 static_actor_combat_profile_homeward_delay down
ALTER TABLE static_actor_combat_profiles DROP COLUMN homeward_delay_ms;
