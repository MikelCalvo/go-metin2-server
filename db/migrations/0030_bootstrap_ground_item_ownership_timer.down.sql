-- go-metin2 migration: 0030 bootstrap_ground_item_ownership_timer down
ALTER TABLE bootstrap_ground_items DROP COLUMN despawn_at;
ALTER TABLE bootstrap_ground_items DROP COLUMN ownership_expires_at;
ALTER TABLE bootstrap_ground_items DROP COLUMN ownership_exclusive;
