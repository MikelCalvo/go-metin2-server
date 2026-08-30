-- go-metin2 migration: 0026 bootstrap_ground_item_instance_sockets up
ALTER TABLE bootstrap_ground_items
    ADD COLUMN has_sockets INTEGER NOT NULL DEFAULT 0
    CHECK (has_sockets IN (0, 1));

ALTER TABLE bootstrap_ground_items
    ADD COLUMN socket0 INTEGER NOT NULL DEFAULT 0
    CHECK (socket0 >= -2147483648 AND socket0 <= 2147483647);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN socket1 INTEGER NOT NULL DEFAULT 0
    CHECK (socket1 >= -2147483648 AND socket1 <= 2147483647);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN socket2 INTEGER NOT NULL DEFAULT 0
    CHECK (
        socket2 >= -2147483648 AND socket2 <= 2147483647
        AND (has_sockets = 1 OR (socket0 = 0 AND socket1 = 0 AND socket2 = 0))
    );
