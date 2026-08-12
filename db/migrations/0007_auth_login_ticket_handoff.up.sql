-- go-metin2 migration: 0007 auth_login_ticket_handoff up
CREATE TABLE auth_login_tickets (
    login_key BIGINT NOT NULL,
    issued_at TEXT NOT NULL,
    login TEXT NOT NULL,
    login_normalized TEXT NOT NULL,
    empire INTEGER NOT NULL DEFAULT 0,
    consumed_at TEXT,
    characters_snapshot_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (login_key, issued_at),
    CHECK (login_key > 0 AND login_key <= 4294967295),
    CHECK (issued_at <> ''),
    CHECK (login <> ''),
    CHECK (login_normalized <> ''),
    CHECK (empire >= 0),
    CHECK (consumed_at IS NULL OR consumed_at <> ''),
    CHECK (consumed_at IS NULL OR consumed_at >= issued_at),
    CHECK (characters_snapshot_json <> '')
);

CREATE UNIQUE INDEX auth_login_tickets_active_login_key_unique
    ON auth_login_tickets (login_key)
    WHERE consumed_at IS NULL;

CREATE INDEX auth_login_tickets_active_login_normalized_index
    ON auth_login_tickets (login_normalized)
    WHERE consumed_at IS NULL;

CREATE INDEX auth_login_tickets_issued_at_index
    ON auth_login_tickets (issued_at);
