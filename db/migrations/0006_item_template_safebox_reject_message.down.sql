-- go-metin2 migration: 0006 item_template_safebox_reject_message down
ALTER TABLE item_templates DROP COLUMN safebox_reject_message;
