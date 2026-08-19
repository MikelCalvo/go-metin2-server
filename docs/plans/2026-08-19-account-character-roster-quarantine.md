# Account/Character Roster Export Quarantine — 2026-08-19

## Objective

Add a fail-closed import/quarantine preflight for the existing `0002_account_character_roster` migration-shaped export so operators can verify a retained roster JSON artifact before any future DB backfill or repository work mutates stores.

## Contract frozen by this slice

1. `accountstore.ValidateAccountCharacterRosterExport(...)` accepts only:
   - `migration_version == 2`
   - `migration_name == "account_character_roster"`
   - non-nil `accounts` and `characters` slices (empty is valid)
   - account rows with `id > 0`, non-empty trimmed `login` / `login_normalized`, `login_normalized == lower(login)`, no NUL text, unique `id`, and unique `login_normalized` (`empire` remains a `uint8`, so non-negativity is type-enforced)
   - character rows with `id > 0`, `account_id` present in the export accounts, `slot` in `0..3`, non-empty trimmed `name` / `name_normalized`, `name_normalized == lower(name)`, no NUL in name/guild text, `level >= 1`, `map_index > 0`, `gold <= signed BIGINT max`, unique `id`, unique `(account_id, slot)`, and unique `name_normalized`
2. Successful validation returns a metadata-only quarantine summary:
   - `account_count`
   - `character_count`
   - deterministic sorted `account_ids`
   - deterministic sorted `character_ids`
3. `accountstore.QuarantineAccountCharacterRosterExport(...)` validates, then returns the same summary plus a canonicalized export ordered by:
   - accounts: ascending `login_normalized`, then `login`, then `id`
   - characters: ascending `account_id`, then `slot`, then `id`
4. Loopback-only `POST /local/account-store/exports/account-character-roster/quarantine` on `gamed`:
   - accepts the export JSON body
   - rejects non-loopback callers with `403`
   - rejects wrong methods with `405`
   - rejects oversized / malformed / invalid UTF-8 bodies with `400` / `413`
   - returns `409` when the payload fails quarantine validation
   - never opens a database, never writes account snapshots, never emits SQL

## What this is not yet

- DB INSERT / backfill execution
- account-store mutation or restore from roster rows
- a repository seam
- quarantine for quest-state / login-ticket / item-template / static-actor exports
- remote admin auth

## TDD and validation

Focused coverage:

- `go test ./internal/accountstore -run 'AccountCharacterRoster' -count=1`
- `go test ./internal/ops -run 'AccountCharacterRosterQuarantine' -count=1`

## Follow-up options

1. Extend the same quarantine pattern to quest-state / login-ticket / item-template / static-actor exports.
2. Add CLI-only quarantine inspection beside `metin2-migrate`.
3. Extract a repository seam only after quarantine preflight proves the boundary.
