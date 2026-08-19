# Auth Login-Ticket Handoff Export Quarantine — 2026-08-19

## Objective

Add a fail-closed import/quarantine preflight for the existing `0007_auth_login_ticket_handoff` migration-shaped export so operators can verify a retained login-ticket JSON artifact before any future DB backfill or repository work mutates stores.

## Contract frozen by this slice

1. `loginticket.ValidateAuthLoginTicketHandoffExport(...)` accepts only:
   - `migration_version == 7`
   - `migration_name == "auth_login_ticket_handoff"`
   - non-nil `tickets` slice (empty is valid)
   - ticket rows with `login_key > 0`, non-zero `issued_at`, non-empty trimmed `login` / `login_normalized`, `login_normalized == lower(login)`, no NUL in login text, unique `(login_key, issued_at)`, unique active `login_key` when `consumed_at` is nil, optional `consumed_at` that is non-zero and `>= issued_at`, and non-empty valid UTF-8 `characters_snapshot_json` that decodes as a JSON array (not `null`) of bootstrap-valid character snapshots
2. Successful validation returns a metadata-only quarantine summary:
   - `ticket_count`
   - `active_ticket_count`
   - deterministic sorted `login_keys`
3. `loginticket.QuarantineAuthLoginTicketHandoffExport(...)` validates, then returns the same summary plus a canonicalized export ordered by:
   - ascending `login_normalized`, then `login`, then `login_key`
4. Loopback-only `POST /local/login-tickets/exports/auth-login-ticket-handoff/quarantine` on `gamed`:
   - accepts the export JSON body
   - rejects non-loopback callers with `403`
   - rejects wrong methods with `405`
   - rejects oversized / malformed / invalid UTF-8 bodies with `400` / `413`
   - returns `409` when the payload fails quarantine validation
   - never opens a database, never writes login-ticket snapshots, never emits SQL, never consumes tickets

## What this is not yet

- DB INSERT / backfill execution
- login-ticket mutation or restore from export rows
- a repository seam
- quarantine for item-template / static-content / ground-item exports
- remote admin auth

## TDD and validation

Focused coverage:

- `go test ./internal/loginticket -run 'AuthLoginTicketHandoff' -count=1`
- `go test ./internal/ops -run 'AuthLoginTicketHandoffQuarantine' -count=1`

## Follow-up options

1. Extend the same quarantine pattern to item-template / static-content / ground-item exports.
2. Add CLI-only quarantine inspection beside `metin2-migrate`.
3. Extract a repository seam only after quarantine preflight proves the boundary.
