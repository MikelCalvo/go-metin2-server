-- go-metin2 migration: 0006 item_template_safebox_reject_message up
ALTER TABLE item_templates ADD COLUMN safebox_reject_message TEXT NOT NULL DEFAULT '' CHECK (safebox_reject_message = '' OR anti_safebox = 1);
