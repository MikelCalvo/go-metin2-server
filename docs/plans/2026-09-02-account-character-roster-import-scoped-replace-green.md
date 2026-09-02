# Account/character roster SQL import scoped replace GREEN — 2026-09-02

## Objective

Implement the opt-in tip-`0002` (`account_character_roster` / `accounts` +
`characters`) **scoped replace** path frozen in
[account/character roster import replace/upsert contract freeze](2026-09-02-account-character-roster-import-replace-upsert-contract-freeze.md)
so operators can re-backfill a retained select-screen roster export without
insert-only primary-key / unique-index conflicts.

## Contract shipped

1. Default `ImportAccountCharacterRoster(...)` remains insert-only.
2. `ImportAccountCharacterRoster(..., ImportAccountCharacterRosterOptions{Replace: true})`
   deletes tip-`0002` `characters` then `accounts` for each quarantined
   `account_id`, then inserts the canonicalized export rows inside one
   transaction.
3. Optional export `account_ids` merges with account-row-derived ids so a listed
   account with zero account/character rows can wipe-to-empty; empty
   `account_ids` plus empty `accounts` / `characters` is a no-op mutation after
   schema/quarantine preflight.
4. Quarantine continues to reject zero/negative ids, empty logins/names,
   duplicate account ids / normalized logins, duplicate character ids /
   normalized names, duplicate `(account_id, slot)`, character `account_id`
   references missing from the same export's account rows, `level < 1`,
   `map_index <= 0`, and gold outside signed BIGINT.
5. Accounts not listed remain untouched (no global truncate).
6. Child tip domains are not cascade-deleted; FK dependents fail closed and
   roll the transaction back.
7. CLI: `metin2-migrate import-export ... --i-confirm-sql-import
   --i-confirm-scoped-replace` accepts tip-`0002` `account-character-roster`
   beside tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023`. Other
   kinds reject the replace confirmation as usage.
8. Successful replace stdout includes metadata-only
   `AccountCharacterRosterImportResult` with `replaced: true`.
9. Still no stock production driver, no daemon mutation route, no catalog tip
   `0030`, and no cascade account-tree replace.

## Proof

- `go test ./internal/accountstore -run 'Roster|ImportAccountCharacterRoster|QuarantineAccountCharacterRoster|ValidateAccountCharacterRoster'`
- `go test ./internal/migratecli -run 'ImportExport.*ScopedReplace|ImportExportRejectsUsage'`
- `go test -tags=sqlite_harness ./internal/accountstore -run 'SQLiteHarnessRosterImport'`

## Status

GREEN on `lane/persistence`. Follow-on tip-`0010` ground-item-state scoped-replace freeze is owned by
[bootstrap ground-item-state import replace/upsert contract freeze](2026-09-02-bootstrap-ground-item-state-import-replace-upsert-contract-freeze.md);
production-engine selection remains deferred.
