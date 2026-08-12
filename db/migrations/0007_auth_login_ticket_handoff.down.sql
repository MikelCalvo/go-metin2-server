-- go-metin2 migration: 0007 auth_login_ticket_handoff down
DROP INDEX auth_login_tickets_issued_at_index;
DROP INDEX auth_login_tickets_active_login_normalized_index;
DROP INDEX auth_login_tickets_active_login_key_unique;
DROP TABLE auth_login_tickets;
