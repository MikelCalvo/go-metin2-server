# Character item-state SQL import replace/upsert contract freeze — 2026-08-31

## Objective

Freeze the first fail-closed **scoped replace** policy for quarantined tip-`0003`
(`character_item_state`, including additive `0024` sockets + `0027` attributes)
before opening RED, so operators can re-backfill a retained export without
hitting insert-only primary-key conflicts.

This freeze does **not** invent a stock production driver, live DB inventory
repository, catalog tip `0030`, or silent row-merge semantics.

## Why docs-first

Track E tip chain through post-carried rematerialize tip-sync is Done
([ops docs tip sync after carried rematerialize](2026-08-31-ops-docs-post-carried-rematerialize-tip-sync.md)).
Catalog tip remains `0029_bootstrap_ground_item_instance_attributes`.

Every landed `Import*` seam is still insert-only and fails closed on a second
import of the same primary keys (`accountstore.ImportCharacterItemState`
explicitly documents “does not invent upsert / merge policy”). That is safe for
first-time backfill, but it blocks the honest operator re-backfill path after a
retained export is corrected or a lab DB is rebuilt to tip and re-seeded.

Opening RED without freezing:

- default vs opt-in confirmation,
- replace scope (which tables / which character ids),
- empty-export wipe vs no-op rules,
- transaction / fail-closed semantics,
- CLI flag shape,

would invent policy mid-implementation. Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default remains insert-only

1. `ImportCharacterItemState(...)` without an explicit replace option keeps
   today's insert-only behavior: duplicate primary keys fail closed and roll
   the transaction back.
2. `metin2-migrate import-export` without an explicit replace confirmation keeps
   today's insert-only path for every kind, including `character-item-state`.
3. No silent upgrade of insert-only into replace.

### B. Opt-in scoped replace for tip-`0003` only (this freeze)

1. A new programmatic option (name TBD in GREEN; treat as
   `ImportCharacterItemStateReplace` / `ReplaceCharacterItemState` / option flag
   on the existing primitive) performs **scoped replace** for the character ids
   present in the quarantined export.
2. Scope is exactly the tip-`0003` child tables already owned by
   `ImportCharacterItemState`:
   - `character_inventory_items`
   - `character_equipment_items`
   - `character_quickslots`
3. For each `character_id` listed in the canonicalized export summary /
   `CharacterIDs`:
   - delete existing rows for that character in the three tables above,
   - insert the canonicalized export rows for that character,
   - all inside **one** transaction after the existing quarantine + schema
     preflight (`0003` + additive `0024` + additive `0027`).
4. Characters **not** listed in the export are left untouched (no global truncate).
5. Parent `accounts` / `characters` rows from tip-`0002` remain prerequisites;
   this replace path does not invent roster upsert.
6. Additive socket/attribute columns stay on tip-`0003` identity; do **not**
   retip export / quarantine / import-result identity to `24` or `27`.

### C. Empty / partial export semantics

1. An export that quarantines successfully with zero inventory / equipment /
   quickslot rows for a listed character still replaces that character's scoped
   tables with empty sets (wipe-to-empty is intentional and must be covered by
   tests).
2. An export with an empty `CharacterIDs` list is a no-op mutation after
   quarantine/schema preflight (commit allowed; result counts stay zero).
3. Malformed / non-quarantinable exports continue to fail before any DELETE /
   INSERT.

### D. CLI confirmation shape (tip-`0003` only in first GREEN)

1. Replace stays off unless the operator passes an explicit confirmation flag
   in addition to `--i-confirm-sql-import` (exact flag name TBD in GREEN;
   freeze requires a distinct confirmation, not reuse of insert-only alone).
2. First GREEN may limit CLI replace to `--kind character-item-state` only.
   Other kinds stay insert-only until their own freezes.
3. Successful stdout remains metadata-only `CharacterItemStateImportResult`
   JSON (no DSN, no SQL text, no item payloads). Optionally add a boolean
   `replaced: true` field in GREEN if tests need it; absence of that field in
   this freeze is fine.
4. Print-only `import-export-drill` does **not** auto-enable replace by default; opt-in `--i-confirm-print-scoped-replace` is owned by
   [import-export-drill opt-in scoped-replace printer](2026-09-03-import-export-drill-opt-in-scoped-replace.md).

### E. Explicit non-goals

- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- live DB inventory / safebox / ground repositories replacing FileStore
- catalog tip `0030` / retip of tip-`0003` / `0010` / `0015` export identities
- upsert / replace for roster (`0002`), quest (`0004`), points (`0011`),
  safebox (`0015`), ground (`0010`), templates (`0009`), tickets (`0007`),
  static actors (`0013`), or myshop prices (`0023`) in this freeze
- silent merge / per-row `ON CONFLICT DO UPDATE` without scoped DELETE
- online mutation while selected-character sessions are live (CLI remains an
  offline operator tool; no daemon mutation route)
- remote admin, secrets in git, README churn, gameplay changes
- another crash/restart rematerialize twin (carried / safebox / ground
  sockets+attributes already GREEN)

## Docs / QA lag folded into this freeze

QA checklist still claims “tip-`0015` additive safebox cell socket SQL remain
deferred” even though tip-`0015`+`0025` (+`0028` attributes) SQL companions and
seeded tip syncs are owned. This freeze commit corrects that lag beside the
replace-contract prose so operators do not chase a finished schema gate.

## Proof shape (for later RED → GREEN)

1. SQLite harness: first insert-only import succeeds; second insert-only import
   of the same export fails closed; opt-in replace of the same export succeeds
   and leaves exactly the canonical rows (including `has_sockets` /
   `has_attributes` presence, explicit-zero sockets/attrs).
2. Scoped wipe: character A in export, character B only in DB beforehand →
   replace updates A and leaves B untouched.
3. Empty wipe: listed character with zero child rows → that character's three
   tables become empty inside the transaction.
4. CLI: insert-only confirmation alone cannot replace; distinct replace
   confirmation is required for `character-item-state`.
5. Negatives: missing schema tip `3`/`24`/`27`, bad quarantine, nil executor,
   and FK-missing parent characters still fail closed before commit.

## Likely files to change (later GREEN, not this freeze)

- `internal/accountstore/item_state_import.go` (+ unit / sqlite harness tests)
- `internal/migratecli` import-export flag wiring (+ tests)
- `docs/development.md` / `docs/workflow/migration-apply-runbook.md` (GREEN
  wording once the flag exists)
- Track E / migration-contract next-slice pointers (flip freeze → Done on GREEN)

## Status

Docs/spec freeze landed first; tip-`0003` scoped replace GREEN is now owned by
[character item-state import scoped replace GREEN](2026-08-31-character-item-state-import-scoped-replace-green.md).
Insert-only remains the default without the replace option / CLI confirmation.
