# Item-Template-State Export Quarantine — 2026-08-19

## Objective

Add a fail-closed import/quarantine preflight for the existing `0009_item_template_refine_info` migration-shaped export so operators can verify a retained item-template JSON artifact before any future DB backfill or repository work mutates stores.

## Contract frozen by this slice

1. `itemstore.ValidateItemTemplateStateExport(...)` accepts only:
   - `migration_version == 9`
   - `migration_name == "item_template_refine_info"`
   - non-nil `templates`, `sockets`, `attributes`, `use_effects`, `equip_effects`, `refine_infos`, and `refine_materials` slices (empty is valid)
   - template rows that reconstruct into a bootstrap-valid authored template (`vnum > 0`, non-empty UTF-8 name without NUL, `max_count` / pricing / equip-slot / reject-text / refine / effect bounds)
   - unique template `vnum` values
   - child rows that reference an exported template `vnum`, keep migration-valid positions/bounds, and do not collide on primary keys (`(vnum, position)` for sockets/attributes/refine materials; unique `vnum` for use/equip/refine-info rows)
   - refine-info / refine-material rows only for refineable templates, with material positions contiguous from `0`
2. Successful validation returns a metadata-only quarantine summary:
   - `template_count`
   - `socket_count`
   - `attribute_count`
   - `use_effect_count`
   - `equip_effect_count`
   - `refine_info_count`
   - `refine_material_count`
   - deterministic sorted `vnums`
3. `itemstore.QuarantineItemTemplateStateExport(...)` validates, then returns the same summary plus a canonicalized export ordered by:
   - templates: ascending `vnum`
   - sockets / attributes / refine materials: ascending `vnum`, then `position`
   - use effects / equip effects / refine infos: ascending `vnum`
4. Loopback-only `POST /local/item-templates/exports/item-template-state/quarantine` on `gamed`:
   - accepts the export JSON body
   - rejects non-loopback callers with `403`
   - rejects wrong methods with `405`
   - rejects oversized / malformed / invalid UTF-8 bodies with `400` / `413`
   - returns `409` when the payload fails quarantine validation
   - never opens a database, never writes item-template snapshots, never emits SQL

## What this is not yet

- DB INSERT / backfill execution
- item-template mutation or restore from export rows
- a repository seam
- quarantine for static-actor / ground-item exports
- remote admin auth

## TDD and validation

Focused coverage:

- `go test ./internal/itemstore -run 'ItemTemplateState' -count=1`
- `go test ./internal/ops -run 'ItemTemplateStateQuarantine' -count=1`

## Follow-up options

1. Extend the same quarantine pattern to static-actor content-state / bootstrap ground-item exports.
2. Add CLI-only quarantine inspection beside `metin2-migrate`.
3. ~~Extract a repository seam only after quarantine preflight proves the boundary.~~ Done: `ItemTemplateStateExporter` + hermetic `MemoryStore` now land beside the quarantine preflight.
