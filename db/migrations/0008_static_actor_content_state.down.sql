-- go-metin2 migration: 0008 static_actor_content_state down
DROP TABLE static_actor_reward_drops;
DROP INDEX static_actors_interaction_ref_index;
DROP INDEX static_actors_map_index_index;
DROP INDEX static_actors_spawn_group_ref_unique;
DROP TABLE static_actors;
DROP TABLE interaction_merchant_catalog_entries;
DROP TABLE interaction_definitions;
