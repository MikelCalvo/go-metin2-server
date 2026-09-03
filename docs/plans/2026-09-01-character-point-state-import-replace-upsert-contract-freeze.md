# Character point-state SQL import replace/upsert contract freeze — 2026-09-01

## Objective

Freeze the next fail-closed **scoped replace** policy for quarantined tip-`0011`
(`character_point_state` / `character_points`) before opening RED, so operators
can re-backfill a retained selected-character point-vector export without
hitting insert-only primary-key conflicts.

This freeze does **not** invent a stock production driver, live DB point
repository, catalog tip `0030`, combat UI surfaces, or silent row-merge
semantics.

## Why docs-first

Track E tip-`0003` / tip-`0015` / tip-`0004` scoped replace paths are already
GREEN on `main`:

- [character item-state import scoped replace GREEN](2026-08-31-character-item-state-import-scoped-replace-green.md)
- [character safebox-state import scoped replace GREEN](2026-09-01-character-safebox-state-import-scoped-replace-green.md)
- [character quest-state import scoped replace GREEN](2026-09-01-character-quest-state-import-scoped-replace-green.md)

Catalog tip remains `0029_bootstrap_ground_item_instance_attributes`.

`accountstore.ImportCharacterPointState` is still insert-only and fails closed
on a second import of the same `(character_id, point_index)` primary keys
(explicitly “does not invent upsert / merge policy”). That is safe for
first-time backfill of the fixed-width selected-character point vector, but it
blocks the honest operator re-backfill path after a retained tip-`0011` export
is corrected or a lab DB is rebuilt to tip and re-seeded.

Point state sits on the playable PvE vertical (HP / SP / gold-adjacent point
indexes, death-floor signed values, item/equipment/combat mutations that already
rematerialize across daemon restart). Owning the re-backfill contract next keeps
that durable state operable without waiting on production-engine selection.

Opening RED without freezing:

- default vs opt-in confirmation,
- replace scope (which table / which character ids),
- how fixed-width `0..254` vectors interact with wipe-to-empty,
- declared character scope vs row-derived ids,
- transaction / fail-closed semantics,
- CLI flag shape,

would invent policy mid-implementation. Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default remains insert-only

1. `ImportCharacterPointState(...)` without an explicit replace option keeps
   today's insert-only behavior: duplicate primary keys fail closed and roll
   the transaction back.
2. `metin2-migrate import-export` without an explicit replace confirmation keeps
   today's insert-only path for every kind, including `character-point-state`.
3. No silent upgrade of insert-only into replace.

### B. Opt-in scoped replace for tip-`0011` only (this freeze)

1. A new programmatic option (name TBD in GREEN; treat as
   `ImportCharacterPointStateOptions{Replace: true}` mirroring tip-`0003` /
   tip-`0004` / tip-`0015`) performs **scoped replace** for the character ids
   present in the quarantined export summary.
2. Scope is exactly the tip-`0011` table already owned by
   `ImportCharacterPointState`:
   - `character_points`
3. For each `character_id` listed in the canonicalized export summary /
   declared `CharacterIDs`:
   - delete existing rows for that character in `character_points`,
   - insert the canonicalized export rows for that character,
   - all inside **one** transaction after the existing quarantine + schema
     preflight (ledger entry for version `11` / `character_point_state`).
4. Characters **not** listed in the export are left untouched (no global truncate).
5. Parent `accounts` / `characters` rows from tip-`0002` remain prerequisites;
   this replace path does not invent roster upsert.
6. Export tip identity stays `0011_character_point_state`; do **not** retip
   export / quarantine / import-result identity.

### C. Declared character scope + fixed-width / empty semantics

1. Tip-`0011` exports gain an optional `character_ids` field (same merge rules as
   tip-`0003` / tip-`0004` / tip-`0015`): declared ids merge with point
   row-derived ids so a listed character with zero point rows can wipe-to-empty.
2. When a character contributes any `points` rows, quarantine continues to
   require the complete fixed-width `0..254` vector (255 rows). Sparse /
   duplicate / out-of-range vectors stay fail-closed.
3. A listed character with zero point rows is allowed only as an explicit
   wipe-to-empty scope entry (via declared `character_ids`); GREEN must not
   invent a sparse partial-vector replace mode.
4. An export with an empty `character_ids` list and empty `points` is a no-op
   mutation after quarantine/schema preflight (commit allowed; result counts
   stay zero).
5. Malformed / non-quarantinable exports continue to fail before any DELETE /
   INSERT.
6. Replace of a listed character replaces that character's entire
   `character_points` set for the export; there is no per-`point_index`
   half-replace mode in this freeze.

### D. CLI confirmation shape (tip-`0011` added beside owned replace kinds)

1. Replace stays off unless the operator passes an explicit confirmation flag
   in addition to `--i-confirm-sql-import`. GREEN should reuse the existing
   `--i-confirm-scoped-replace` flag and widen it to also accept
   `--kind character-point-state` (still reject every other kind that has not
   frozen+GREEN'd its own replace path).
2. Successful stdout remains metadata-only `CharacterPointStateImportResult`
   JSON (no DSN, no SQL text, no point payloads). GREEN should add
   `replaced: true` (omitempty) mirroring tip-`0003` / tip-`0004` / tip-`0015`.
3. Print-only `import-export-drill` does **not** auto-enable replace by default; opt-in `--i-confirm-print-scoped-replace` is owned by
   [import-export-drill opt-in scoped-replace printer](2026-09-03-import-export-drill-opt-in-scoped-replace.md).

### E. Explicit non-goals

- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- live DB point repository replacing FileStore rematerialize
- catalog tip `0030` / retip of tip-`0011` / `0003` / `0004` / `0015` / `0010`
  export identities
- upsert / replace for roster (`0002`), ground (`0010`), templates (`0009`),
  tickets (`0007`), static actors (`0013`), or myshop prices (`0023`) in this
  freeze (tip-`0003` / tip-`0004` / tip-`0015` replace stay owned)
- silent merge / per-row `ON CONFLICT DO UPDATE` without scoped DELETE
- online mutation while selected-character sessions are live (CLI remains an
  offline operator tool; no daemon mutation route)
- combat UI / remote admin / secrets in git / README churn / gameplay changes
- another crash/restart rematerialize twin (position/points FileStore
  rematerialize is already GREEN)

## Proof shape (for later RED → GREEN)

1. SQLite harness: first insert-only import succeeds; second insert-only import
   of the same export fails closed; opt-in replace of the same export succeeds
   and leaves exactly the canonical 255-row vectors.
2. Scoped wipe: character A in export, character B only in DB beforehand →
   replace updates A and leaves B untouched.
3. Empty wipe: listed character with zero point rows via declared
   `character_ids` → that character's `character_points` rows become empty
   inside the transaction.
4. CLI: insert-only confirmation alone cannot replace; `--i-confirm-scoped-replace`
   is accepted for `character-point-state` (and still for already-owned replace
   kinds); other unfrozen kinds still reject it.
5. Negatives: missing schema tip `11`, bad quarantine (sparse vector), nil
   executor, and FK-missing parent characters still fail closed before commit.

## Likely files to change (later GREEN, not this freeze)

- `internal/accountstore/point_state_export.go` / `point_state_quarantine.go`
  (optional `character_ids` merge + wipe-to-empty exception)
- `internal/accountstore/point_state_import.go` (+ unit / sqlite harness tests)
- `internal/migratecli` import-export flag wiring (+ tests)
- `docs/development.md` / `docs/workflow/migration-apply-runbook.md` (GREEN
  wording once the kind is accepted)
- Track E / migration-contract next-slice pointers (flip freeze → Done on GREEN)

## Status

Docs/spec freeze landed first; tip-`0011` scoped replace GREEN is now owned by
[character point-state import scoped replace GREEN](2026-09-01-character-point-state-import-scoped-replace-green.md).
Insert-only remains the default without the replace option / CLI confirmation.
This freeze also folds the missing Track E / migration-contract Done marker for
already-landed tip-`0015` scoped replace GREEN so operator next-slice pointers
stop skipping that ownership.
