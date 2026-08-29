-- go-metin2 migration: 0021 item_template_refine_keep_on_fail up
ALTER TABLE item_template_refine_infos
    ADD COLUMN keep_on_fail INTEGER NOT NULL DEFAULT 0
    CHECK (
        keep_on_fail = 0
        OR (keep_on_fail = 1 AND probability >= 1 AND probability <= 99)
    );
