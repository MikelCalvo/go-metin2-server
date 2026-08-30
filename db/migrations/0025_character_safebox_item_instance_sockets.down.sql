-- go-metin2 migration: 0025 character_safebox_item_instance_sockets down
ALTER TABLE character_safebox_items DROP COLUMN socket2;
ALTER TABLE character_safebox_items DROP COLUMN socket1;
ALTER TABLE character_safebox_items DROP COLUMN socket0;
ALTER TABLE character_safebox_items DROP COLUMN has_sockets;
