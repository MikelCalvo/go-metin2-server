-- go-metin2 migration: 0021 item_template_refine_keep_on_fail down
ALTER TABLE item_template_refine_infos DROP COLUMN keep_on_fail;
