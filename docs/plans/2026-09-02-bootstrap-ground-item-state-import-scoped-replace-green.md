# Bootstrap ground-item-state SQL import scoped replace GREEN — 2026-09-02

## Objective

Implement the opt-in tip-`0010` (`bootstrap_ground_item_state` /
`bootstrap_ground_items`, including additive `0026` sockets + `0029` attributes)
**scoped replace** path frozen in
[bootstrap ground-item-state import replace/upsert contract freeze](2026-09-02-bootstrap-ground-item-state-import-replace-upsert-contract-freeze.md)
so operators can re-backfill a retained pending ground-handle export without
insert-only primary-key conflicts on `vid`.

## Contract shipped

1. Default `ImportBootstrapGroundItemState(...)` remains insert-only.
2. `ImportBootstrapGroundItemState(..., ImportBootstrapGroundItemStateOptions{Replace: true})`
   deletes tip-`0010` `bootstrap_ground_items` for each quarantined `vid`, then
   inserts the canonicalized export rows inside one transaction.
3. Optional export `vids` merges with ground-row-derived VIDs so a listed VID
   with zero ground rows can wipe-to-empty; empty `vids` plus empty
   `ground_items` is a no-op mutation after schema/quarantine preflight.
4. Quarantine continues to reject zero/duplicate VIDs, exclusive item/gold shape
   violations, and presence-aware socket/attribute inconsistencies.
5. VIDs not listed remain untouched (no global truncate).
6. CLI: `metin2-migrate import-export ... --i-confirm-sql-import
   --i-confirm-scoped-replace` accepts tip-`0010` `bootstrap-ground-item-state`
   beside tip-`0002` / tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` /
   tip-`0023`. Other kinds reject the replace confirmation as usage.
7. Successful replace stdout includes metadata-only
   `BootstrapGroundItemStateImportResult` with `replaced: true`.
8. Still no stock production driver, no daemon mutation route, no catalog tip
   `0030`, and no DB-backed live ground rematerialize.

## Proof

- `go test ./internal/worldruntime -run 'QuarantineBootstrapGroundItemStateExport|ImportBootstrapGroundItemStateRejects'`
- `go test ./internal/migratecli -run 'ImportExport.*ScopedReplace|ImportExportRejectsUsage'`
- `go test -tags=sqlite_harness ./internal/worldruntime -run 'SQLiteHarnessGroundItemStateImport'`

## Status

GREEN on `lane/persistence`. Tip-`0009` item-template-state scoped replace GREEN
is owned by
[item-template-state import scoped replace GREEN](2026-09-02-item-template-state-import-scoped-replace-green.md).
Tip-`0013` static-actor content-state scoped replace freeze is owned by
[static-actor content-state import replace/upsert contract freeze](2026-09-02-static-actor-content-state-import-replace-upsert-contract-freeze.md).
Tip-`0013` scoped replace GREEN is Done in
[static-actor content-state import scoped replace GREEN](2026-09-02-static-actor-content-state-import-scoped-replace-green.md).
Tip-`0007` auth-login-ticket-handoff scoped replace GREEN is owned by
[auth-login-ticket-handoff import scoped replace GREEN](2026-09-03-auth-login-ticket-handoff-import-scoped-replace-green.md);
production-engine selection remains deferred.
