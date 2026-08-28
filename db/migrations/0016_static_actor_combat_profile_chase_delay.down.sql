-- go-metin2 migration: 0016 static_actor_combat_profile_chase_delay down
ALTER TABLE static_actor_combat_profiles DROP COLUMN chase_delay_ms;
