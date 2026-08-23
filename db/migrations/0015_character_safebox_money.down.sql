-- go-metin2 migration: 0015 character_safebox_money down
ALTER TABLE character_safebox_passwords DROP COLUMN money;
