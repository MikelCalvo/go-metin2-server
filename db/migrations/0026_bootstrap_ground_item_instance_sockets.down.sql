-- go-metin2 migration: 0026 bootstrap_ground_item_instance_sockets down
ALTER TABLE bootstrap_ground_items DROP COLUMN socket2;
ALTER TABLE bootstrap_ground_items DROP COLUMN socket1;
ALTER TABLE bootstrap_ground_items DROP COLUMN socket0;
ALTER TABLE bootstrap_ground_items DROP COLUMN has_sockets;
