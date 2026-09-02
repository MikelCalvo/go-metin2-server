# Item-template-state SQL import replace/upsert contract freeze — 2026-09-02

## Objective

Freeze the next fail-closed **scoped replace** policy for quarantined tip-`0009`
(`item_template_refine_info` / `item_templates` plus child socket / attribute /
use-effect / equip-effect / refine-info / refine-material tables, including
additive `0021` `keep_on_fail` + `0022` `fail_result_vnum`) before opening RED,
so operators can re-backfill a retained authored item-template export without
hitting insert-only primary-key conflicts on `vnum`.

This freeze does **not** invent a stock production driver, live DB template
repository, catalog tip `0030`, DB-backed live template loading, remote admin, or
silent row-merge semantics.

## Why docs-first

Track E character-domain, roster, and pending-ground scoped replace paths are
already GREEN on `main`:

- [character item-state import scoped replace GREEN](2026-08-31-character-item-state-import-scoped-replace-green.md)
- [character safebox-state import scoped replace GREEN](2026-09-01-character-safebox-state-import-scoped-replace-green.md)
- [character quest-state import scoped replace GREEN](2026-09-01-character-quest-state-import-scoped-replace-green.md)
- [character point-state import scoped replace GREEN](2026-09-01-character-point-state-import-scoped-replace-green.md)
- [character myshop unit-prices import scoped replace GREEN](2026-09-01-character-myshop-unit-prices-import-scoped-replace-green.md)
- [account/character roster import scoped replace GREEN](2026-09-02-account-character-roster-import-scoped-replace-green.md)
- [bootstrap ground-item-state import scoped replace GREEN](2026-09-02-bootstrap-ground-item-state-import-scoped-replace-green.md)

Catalog tip remains `0029_bootstrap_ground_item_instance_attributes`.

`itemstore.ImportItemTemplateState` is still insert-only and fails closed on a
second import of the same `vnum` primary keys (explicitly “does not invent upsert
/ merge policy”). That is safe for first-time backfill of authored templates, but
it blocks the honest operator re-backfill path after a retained tip-`0009` export
is corrected or a lab DB is rebuilt to tip and re-seeded.

Authored item templates sit under every PvE inventory / equip / refine / merchant
/ safebox / ground-drop path already owned by the vertical. Owning the re-backfill
contract next keeps that durable projection operable without waiting on
production-engine selection.

Opening RED without freezing:

- default vs opt-in confirmation,
- replace scope (which tables / which template vnums),
- child-row delete order (FK without `ON DELETE CASCADE`),
- declared vnum scope vs row-derived vnums,
- wipe-to-empty / empty no-op semantics,
- transaction / fail-closed semantics,
- CLI flag shape,

would invent policy mid-implementation. Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default remains insert-only

1. `ImportItemTemplateState(...)` without an explicit replace option keeps
   today's insert-only behavior: duplicate primary keys fail closed and roll the
   transaction back.
2. `metin2-migrate import-export` without an explicit replace confirmation keeps
   today's insert-only path for every kind, including `item-template-state`.
3. No silent upgrade of insert-only into replace.

### B. Opt-in scoped replace for tip-`0009` only (this freeze)

1. A new programmatic option (name TBD in GREEN; treat as
   `ImportItemTemplateStateOptions{Replace: true}` mirroring tip-`0002` /
   tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023` / tip-`0010`)
   performs **scoped replace** for the template vnums present in the quarantined
   export summary.
2. Scope is exactly the tip-`0009` parent + child tables already owned by
   `ImportItemTemplateState`:
   - `item_templates` (including additive `0006` `safebox_reject_message`)
   - `item_template_sockets`
   - `item_template_attributes`
   - `item_template_use_effects`
   - `item_template_equip_effects`
   - `item_template_refine_infos` (including additive `0021` `keep_on_fail` and
     additive `0022` `fail_result_vnum`)
   - `item_template_refine_materials`
3. For each `vnum` listed in the canonicalized export summary / declared
   `Vnums`:
   - delete existing tip-`0009` child rows for that `vnum` first (because the
     shipped schema declares `FOREIGN KEY (... ) REFERENCES ...` **without**
     `ON DELETE CASCADE`):
     - `item_template_refine_materials` for that `vnum`
     - `item_template_refine_infos` for that `vnum`
     - `item_template_sockets` / `item_template_attributes` /
       `item_template_use_effects` / `item_template_equip_effects` for that
       `vnum`
     - then `item_templates` for that `vnum`
   - insert the canonicalized export parent row (when present) then its child
     rows,
   - all inside **one** transaction after the existing quarantine + schema
     preflight (ledger entries for version `9` / `item_template_refine_info`
     plus additive `21` / `item_template_refine_keep_on_fail` plus additive `22`
     / `item_template_refine_fail_result_vnum`).
4. Vnums **not** listed in the export are left untouched (no global truncate).
5. This replace path does not invent live template-index rematerialize, content
   bundle import, or FileStore mutation.
6. Export tip identity stays `0009_item_template_refine_info`; do **not** retip
   export / quarantine / import-result identity (additive `0021` / `0022` keep
   tip-`0009` identity).

### C. Declared vnum scope + empty semantics

1. Tip-`0009` exports gain an optional `vnums` field (same merge idea as
   tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023` `character_ids`,
   tip-`0002` `account_ids`, and tip-`0010` `vids`): declared ids merge with
   template-row-derived vnums so a listed vnum with zero template rows can
   wipe-to-empty.
2. Quarantine continues to reject zero/duplicate template `vnum`, child rows that
   reference missing templates, migration-invalid positions/bounds, contiguous
   refine-material positions from `0`, and reconstructed templates that fail the
   authored bootstrap validation rules (including safebox-reject /
   `keep_on_fail` / `fail_result_vnum` consistency). Those fail closed before any
   DELETE / INSERT.
3. A listed vnum with zero template rows is allowed only as an explicit
   wipe-to-empty scope entry (via declared `vnums`).
4. An export with an empty `vnums` list and empty `templates` (and therefore empty
   child arrays) is a no-op mutation after quarantine/schema preflight (commit
   allowed; result counts stay zero).
5. Malformed / non-quarantinable exports continue to fail before any DELETE /
   INSERT.
6. Replace of a listed vnum replaces that vnum's entire tip-`0009` parent+child
   set for the export (including additive refine columns). There is no
   per-column / per-child half-replace / silent merge mode in this freeze.

### D. CLI confirmation shape (tip-`0009` added beside owned replace kinds)

1. Replace stays off unless the operator passes an explicit confirmation flag
   in addition to `--i-confirm-sql-import`. GREEN should reuse the existing
   `--i-confirm-scoped-replace` flag and widen it to also accept
   `--kind item-template-state` (still reject every other kind that has not
   frozen+GREEN'd its own replace path — today that still excludes
   `auth-login-ticket-handoff`; tip-`0013` `static-actor-content-state` now has its own freeze).
2. Successful stdout remains metadata-only `ItemTemplateStateImportResult` JSON
   (no DSN, no SQL text, no template payloads). GREEN should add
   `replaced: true` (omitempty) mirroring tip-`0002` / tip-`0003` / tip-`0004` /
   tip-`0011` / tip-`0015` / tip-`0023` / tip-`0010`.
3. Print-only `import-export-drill` does **not** auto-enable replace; any later
   drill printer change is a separate slice.

### E. Explicit non-goals

- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- live DB template repository replacing FileStore / MemoryStore authored indexes
- catalog tip `0030` / retip of tip-`0009` / `0002` / `0003` / `0004` / `0011` /
  `0015` / `0023` / `0010` export identities
- upsert / replace for tickets (`0007`) or static actors (`0013` / tip-`0008`
  content) in this freeze (tip-`0002` / tip-`0003` / tip-`0004` / tip-`0011` /
  tip-`0015` / tip-`0023` / tip-`0010` replace stay owned)
- silent merge / per-row `ON CONFLICT DO UPDATE` without scoped DELETE
- online mutation of live template indexes / content bundles (CLI remains an
  offline operator tool; no daemon mutation route; FileStore / MemoryStore /
  live item use stay untouched)
- remote admin, secrets in git, README churn, gameplay changes
- another crash/restart rematerialize twin (authored item-template FileStore
  reload is already owned)

## Proof shape (for later RED → GREEN)

1. SQLite harness: first insert-only import succeeds; second insert-only import
   of the same export fails closed; opt-in replace of the same export succeeds
   and leaves exactly the canonical parent+child rows (including presence of
   optional `keep_on_fail` / `fail_result_vnum`).
2. Scoped wipe: vnum A in export, vnum B only in DB beforehand → replace updates
   A and leaves B untouched.
3. Empty wipe: listed vnum with zero template rows via declared `vnums` → that
   vnum's tip-`0009` parent+child rows become absent inside the transaction.
4. CLI: insert-only confirmation alone cannot replace;
   `--i-confirm-scoped-replace` is accepted for `item-template-state` (and still
   for already-owned replace kinds); other unfrozen kinds still reject it.
5. Negatives: missing schema tip `9` / additive `21` / additive `22`, bad
   quarantine (duplicate vnum / orphan child / invalid refine materials), nil
   executor, and FK-ordering mistakes still fail closed before commit.

## Likely files to change (later GREEN, not this freeze)

- `internal/itemstore/migration_export.go` /
  `migration_export_quarantine.go` (optional `vnums` merge + wipe-to-empty
  exception)
- `internal/itemstore/item_template_state_import.go` (+ unit / sqlite harness
  tests)
- `internal/migratecli` import-export flag wiring (+ tests)
- `docs/development.md` / `docs/workflow/migration-apply-runbook.md` (GREEN
  wording once the kind is accepted)
- Track E / migration-contract next-slice pointers (flip freeze → Done on GREEN)

## Status

Docs/spec freeze landed. Tip-`0009` scoped replace GREEN is owned by
[item-template-state import scoped replace GREEN](2026-09-02-item-template-state-import-scoped-replace-green.md).
Insert-only remains the default without the replace option / CLI confirmation.
Production-engine selection remains deferred. Tip-`0013` static-actor
content-state scoped replace freeze is owned by
[static-actor content-state import replace/upsert contract freeze](2026-09-02-static-actor-content-state-import-replace-upsert-contract-freeze.md);
upsert / replace for `auth-login-ticket-handoff` (`0007`) is frozen in
[auth-login-ticket-handoff import replace/upsert contract freeze](2026-09-02-auth-login-ticket-handoff-import-replace-upsert-contract-freeze.md);
RED → GREEN remains follow-on.
