# Item-template-state SQL import scoped replace GREEN — 2026-09-02

## Objective

Implement the opt-in tip-`0009` (`item_template_refine_info` / `item_templates`
plus child socket / attribute / use-effect / equip-effect / refine-info /
refine-material tables, including additive `0021` `keep_on_fail` + `0022`
`fail_result_vnum`) **scoped replace** path frozen in
[item-template-state import replace/upsert contract freeze](2026-09-02-item-template-state-import-replace-upsert-contract-freeze.md)
so operators can re-backfill a retained authored item-template export without
insert-only primary-key conflicts on `vnum`.

## Contract shipped

1. Default `ImportItemTemplateState(...)` remains insert-only.
2. `ImportItemTemplateState(..., ImportItemTemplateStateOptions{Replace: true})`
   deletes tip-`0009` parent+child rows for each quarantined `vnum` (children
   first because the shipped schema has no `ON DELETE CASCADE`), then inserts
   the canonicalized export rows inside one transaction.
3. Optional export `vnums` merges with template-row-derived vnums so a listed
   vnum with zero template rows can wipe-to-empty; empty `vnums` plus empty
   `templates` / child arrays is a no-op mutation after schema/quarantine
   preflight.
4. Quarantine continues to reject zero/duplicate template `vnum`, orphan child
   rows, migration-invalid positions/bounds, non-contiguous refine materials,
   and reconstructed templates that fail authored bootstrap validation
   (including safebox-reject / `keep_on_fail` / `fail_result_vnum` consistency).
5. Vnums not listed remain untouched (no global truncate).
6. CLI: `metin2-migrate import-export ... --i-confirm-sql-import
   --i-confirm-scoped-replace` accepts tip-`0009` `item-template-state` beside
   tip-`0002` / tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023` /
   tip-`0010`. Other kinds reject the replace confirmation as usage.
7. Successful replace stdout includes metadata-only
   `ItemTemplateStateImportResult` with `replaced: true`.
8. Still no stock production driver, no daemon mutation route, no catalog tip
   `0030`, and no DB-backed live template rematerialize.

## Proof

- `go test ./internal/itemstore -run 'QuarantineItemTemplateStateExport|ImportItemTemplateStateRejects'`
- `go test ./internal/migratecli -run 'ImportExport.*ScopedReplace|ImportExportRejectsUsage'`
- `go test -tags=sqlite_harness ./internal/itemstore -run 'SQLiteHarnessItemTemplateStateImport'`

## Status

GREEN on `lane/persistence`. Follow-on upsert / replace freezes for
`auth-login-ticket-handoff` (`0007`) and `static-actor-content-state` (`0013`)
remain deferred; production-engine selection remains deferred.
