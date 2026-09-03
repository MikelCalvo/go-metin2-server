# Auth-login-ticket-handoff SQL import scoped replace GREEN — 2026-09-03

## Objective

Implement the opt-in tip-`0007` (`auth_login_ticket_handoff` / `auth_login_tickets`)
**scoped replace** path frozen in
[auth-login-ticket-handoff import replace/upsert contract freeze](2026-09-02-auth-login-ticket-handoff-import-replace-upsert-contract-freeze.md)
so operators can re-backfill a retained authd→gamed login-ticket handoff export
without insert-only primary-key or active-login-key unique-index conflicts.

## Contract shipped

1. Default `ImportAuthLoginTicketHandoff(...)` remains insert-only.
2. `ImportAuthLoginTicketHandoff(..., ImportAuthLoginTicketHandoffOptions{Replace: true})`
   deletes existing `auth_login_tickets` rows for each quarantined `login_key`,
   then inserts the canonicalized export rows inside one transaction.
3. Optional export `login_keys` merges with ticket-row-derived keys so a listed
   login key with zero ticket rows can wipe-to-empty; empty `login_keys` plus
   empty tickets is a no-op mutation after schema/quarantine preflight.
4. Login keys not listed remain untouched (no global truncate).
5. Replace of a listed login key replaces that key's entire tip-`0007` row set
   (every `(login_key, issued_at)` history row present in the export, including
   optional consumed history). Missing history for a listed key is removed by
   the scoped DELETE.
6. CLI: `metin2-migrate import-export ... --i-confirm-sql-import
   --i-confirm-scoped-replace` accepts tip-`0007` `auth-login-ticket-handoff`
   beside tip-`0002` / tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` /
   tip-`0023` / tip-`0010` / tip-`0009` / tip-`0013`.
7. Successful replace stdout includes metadata-only
   `AuthLoginTicketHandoffImportResult` with `replaced: true`.
8. Still no stock production driver, no daemon mutation route, no catalog tip
   `0030`, and no live DB ticket repository replacing FileStore issue/load/consume.

## Proof

- `go test ./internal/loginticket -run 'ImportAuthLoginTicketHandoff|QuarantineAuthLoginTicketHandoffExportMerges|QuarantineAuthLoginTicketHandoffExportRejectsInvalid'`
- `go test ./internal/migratecli -run 'ImportExport.*ScopedReplace|ImportExportRejectsUsage'`
- `go test -tags=sqlite_harness ./internal/loginticket -run 'SQLiteHarnessAuthLoginTicketHandoffImport'`

## Status

GREEN on `lane/persistence`. Current tip vocabulary scoped-replace paths are now
complete for the landed export kinds; production-engine selection remains
deferred.
