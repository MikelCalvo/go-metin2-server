-- go-metin2 migration: 0015 character_safebox_money up
ALTER TABLE character_safebox_passwords
    ADD COLUMN money INTEGER NOT NULL DEFAULT 0
    CHECK (money >= 0 AND money <= 2147483647);
