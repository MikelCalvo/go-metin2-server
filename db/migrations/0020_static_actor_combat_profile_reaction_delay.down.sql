-- go-metin2 migration: 0020 static_actor_combat_profile_reaction_delay down
ALTER TABLE static_actor_combat_profiles DROP COLUMN reaction_delay_ms;
