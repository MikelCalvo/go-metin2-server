-- go-metin2 migration: 0030 bootstrap_ground_item_ownership_timer up
ALTER TABLE bootstrap_ground_items
    ADD COLUMN ownership_exclusive INTEGER NOT NULL DEFAULT 0
    CHECK (ownership_exclusive IN (0, 1));

ALTER TABLE bootstrap_ground_items
    ADD COLUMN ownership_expires_at TEXT
    CHECK (ownership_expires_at IS NULL OR ownership_expires_at <> '');

ALTER TABLE bootstrap_ground_items
    ADD COLUMN despawn_at TEXT
    CHECK (
        (despawn_at IS NULL OR despawn_at <> '')
        AND (
            (ownership_exclusive = 0 AND ownership_expires_at IS NULL)
            OR (
                ownership_exclusive = 1
                AND ownership_expires_at IS NOT NULL
                AND despawn_at IS NOT NULL
                AND ownership_expires_at <= despawn_at
            )
        )
    );
