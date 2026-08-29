-- go-metin2 migration: 0022 item_template_refine_fail_result_vnum down
ALTER TABLE item_template_refine_infos DROP COLUMN fail_result_vnum;
