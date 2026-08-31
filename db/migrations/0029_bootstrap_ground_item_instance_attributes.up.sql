-- go-metin2 migration: 0029 bootstrap_ground_item_instance_attributes up
ALTER TABLE bootstrap_ground_items
    ADD COLUMN has_attributes INTEGER NOT NULL DEFAULT 0
    CHECK (has_attributes IN (0, 1));

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr0_type INTEGER NOT NULL DEFAULT 0
    CHECK (attr0_type >= 0 AND attr0_type <= 255);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr0_value INTEGER NOT NULL DEFAULT 0
    CHECK (attr0_value >= -32768 AND attr0_value <= 32767);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr1_type INTEGER NOT NULL DEFAULT 0
    CHECK (attr1_type >= 0 AND attr1_type <= 255);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr1_value INTEGER NOT NULL DEFAULT 0
    CHECK (attr1_value >= -32768 AND attr1_value <= 32767);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr2_type INTEGER NOT NULL DEFAULT 0
    CHECK (attr2_type >= 0 AND attr2_type <= 255);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr2_value INTEGER NOT NULL DEFAULT 0
    CHECK (attr2_value >= -32768 AND attr2_value <= 32767);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr3_type INTEGER NOT NULL DEFAULT 0
    CHECK (attr3_type >= 0 AND attr3_type <= 255);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr3_value INTEGER NOT NULL DEFAULT 0
    CHECK (attr3_value >= -32768 AND attr3_value <= 32767);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr4_type INTEGER NOT NULL DEFAULT 0
    CHECK (attr4_type >= 0 AND attr4_type <= 255);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr4_value INTEGER NOT NULL DEFAULT 0
    CHECK (attr4_value >= -32768 AND attr4_value <= 32767);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr5_type INTEGER NOT NULL DEFAULT 0
    CHECK (attr5_type >= 0 AND attr5_type <= 255);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr5_value INTEGER NOT NULL DEFAULT 0
    CHECK (attr5_value >= -32768 AND attr5_value <= 32767);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr6_type INTEGER NOT NULL DEFAULT 0
    CHECK (attr6_type >= 0 AND attr6_type <= 255);

ALTER TABLE bootstrap_ground_items
    ADD COLUMN attr6_value INTEGER NOT NULL DEFAULT 0
    CHECK (
        attr6_value >= -32768 AND attr6_value <= 32767
        AND (
            has_attributes = 1
            OR (
                attr0_type = 0 AND attr0_value = 0
                AND attr1_type = 0 AND attr1_value = 0
                AND attr2_type = 0 AND attr2_value = 0
                AND attr3_type = 0 AND attr3_value = 0
                AND attr4_type = 0 AND attr4_value = 0
                AND attr5_type = 0 AND attr5_value = 0
                AND attr6_type = 0 AND attr6_value = 0
            )
        )
    );
