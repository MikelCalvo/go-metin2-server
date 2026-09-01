# Character safebox-state SQL import replace/upsert contract freeze — 2026-09-01

## Objective

Freeze the second fail-closed **scoped replace** policy for quarantined tip-`0015`
(`character_safebox_money`, including additive `0025` safebox cell sockets +
`0028` safebox cell attributes) before opening RED, so operators can re-backfill
a retained warehouse export without hitting insert-only primary-key conflicts.

This freeze does **not** invent a stock production driver, live DB safebox
repository, catalog tip `0030`, mall / SAFEBOX password-change surfaces, or
silent row-merge semantics.

## Why docs-first

Track E tip-`0003` scoped replace is already GREEN
([character item-state import scoped replace GREEN](2026-08-31-character-item-state-import-scoped-replace-green.md)).
Catalog tip remains `0029_bootstrap_ground_item_instance_attributes`.

`safeboxstore.ImportCharacterSafeboxState` is still insert-only and fails closed
on a second import of the same primary keys (explicitly “does not invent upsert /
merge policy”). That is safe for first-time backfill of durable passwords /
warehouse money / cells, but it blocks the honest operator re-backfill path after
a retained tip-`0015` export is corrected or a lab DB is rebuilt to tip and
re-seeded.

Opening RED without freezing:

- default vs opt-in confirmation,
- replace scope (which tables / which character ids),
- empty-export wipe vs no-op rules,
- how password/money rows interact with wipe-to-empty,
- transaction / fail-closed semantics,
- CLI flag shape,

would invent policy mid-implementation. Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default remains insert-only

1. `ImportCharacterSafeboxState(...)` without an explicit replace option keeps
   today's insert-only behavior: duplicate primary keys fail closed and roll
   the transaction back.
2. `metin2-migrate import-export` without an explicit replace confirmation keeps
   today's insert-only path for every kind, including `character-safebox-state`.
3. No silent upgrade of insert-only into replace.

### B. Opt-in scoped replace for tip-`0015` only (this freeze)

1. A new programmatic option (name TBD in GREEN; treat as
   `ImportCharacterSafeboxStateOptions{Replace: true}` mirroring tip-`0003`)
   performs **scoped replace** for the character ids present in the quarantined
   export summary.
2. Scope is exactly the tip-`0015` tables already owned by
   `ImportCharacterSafeboxState`:
   - `character_safebox_passwords` (login / password / money)
   - `character_safebox_items` (cells, including additive `0025` sockets and
     additive `0028` attributes)
3. For each `character_id` listed in the canonicalized export summary /
   declared `CharacterIDs`:
   - delete existing rows for that character in the two tables above,
   - insert the canonicalized export rows for that character,
   - all inside **one** transaction after the existing quarantine + schema
     preflight (`0015` + additive `0025` + additive `0028`).
4. Characters **not** listed in the export are left untouched (no global truncate).
5. Parent `accounts` / `characters` rows from tip-`0002` remain prerequisites;
   this replace path does not invent roster upsert.
6. Additive socket/attribute columns stay on tip-`0015` identity; do **not**
   retip export / quarantine / import-result identity to `25` or `28`.

### C. Declared character scope + empty / partial export semantics

1. Tip-`0015` exports gain an optional `character_ids` field (same merge rules as
   tip-`0003`): declared ids merge with password/item row-derived ids so a listed
   character with zero password and zero item rows can wipe-to-empty.
2. Because today's quarantine requires every item row to reference a password
   row, an ordinary live export always emits a password row per character. For
   wipe-to-empty, GREEN must accept a declared `character_ids` entry that has
   neither password nor item rows after quarantine merge (password+item wipe).
3. An export with an empty `character_ids` list and empty passwords/items is a
   no-op mutation after quarantine/schema preflight (commit allowed; result
   counts stay zero).
4. Malformed / non-quarantinable exports continue to fail before any DELETE /
   INSERT.
5. Replace of a listed character that includes a password row replaces that
   character's password/money and cells together; there is no password-only or
   cells-only half-replace mode in this freeze.

### D. CLI confirmation shape (tip-`0015` added beside tip-`0003`)

1. Replace stays off unless the operator passes an explicit confirmation flag
   in addition to `--i-confirm-sql-import`. GREEN should reuse the existing
   `--i-confirm-scoped-replace` flag and widen it to also accept
   `--kind character-safebox-state` (still reject every other kind).
2. Successful stdout remains metadata-only `CharacterSafeboxStateImportResult`
   JSON (no DSN, no SQL text, no password/item payloads). GREEN should add
   `replaced: true` (omitempty) mirroring tip-`0003`.
3. Print-only `import-export-drill` does **not** auto-enable replace; any later
   drill printer change is a separate slice.

### E. Explicit non-goals

- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- live DB safebox repository replacing FileStore rematerialize
- catalog tip `0030` / retip of tip-`0015` / `0003` / `0010` export identities
- upsert / replace for roster (`0002`), quest (`0004`), points (`0011`),
  ground (`0010`), templates (`0009`), tickets (`0007`), static actors (`0013`),
  or myshop prices (`0023`) in this freeze (tip-`0003` replace stays owned)
- silent merge / per-row `ON CONFLICT DO UPDATE` without scoped DELETE
- online mutation while selected-character sessions are live (CLI remains an
  offline operator tool; no daemon mutation route)
- mall / SAFEBOX_CHANGE_PASSWORD / remote admin / secrets in git / README churn /
  gameplay changes
- another crash/restart rematerialize twin (safebox FileStore rematerialize is
  already GREEN)

## Proof shape (for later RED → GREEN)

1. SQLite harness: first insert-only import succeeds; second insert-only import
   of the same export fails closed; opt-in replace of the same export succeeds
   and leaves exactly the canonical password + item rows (including
   `has_sockets` / `has_attributes` presence, explicit-zero sockets/attrs).
2. Scoped wipe: character A in export, character B only in DB beforehand →
   replace updates A and leaves B untouched.
3. Empty wipe: listed character with zero password/item rows via declared
   `character_ids` → that character's two tables become empty inside the
   transaction.
4. CLI: insert-only confirmation alone cannot replace; `--i-confirm-scoped-replace`
   is accepted for `character-safebox-state` (and still for
   `character-item-state`); other kinds still reject it.
5. Negatives: missing schema tip `15`/`25`/`28`, bad quarantine, nil executor,
   and FK-missing parent characters still fail closed before commit.

## Likely files to change (later GREEN, not this freeze)

- `internal/safeboxstore/export.go` / `export_quarantine.go` (optional
  `character_ids` merge, mirroring tip-`0003`)
- `internal/safeboxstore/safebox_state_import.go` (+ unit / sqlite harness tests)
- `internal/migratecli` import-export flag wiring (+ tests)
- `docs/development.md` / `docs/workflow/migration-apply-runbook.md` (GREEN
  wording once the kind is accepted)
- Track E / migration-contract next-slice pointers (flip freeze → Done on GREEN)

## Status

Docs/spec freeze landed first; tip-`0015` scoped replace GREEN is owned by
[character safebox-state import scoped replace GREEN](2026-09-01-character-safebox-state-import-scoped-replace-green.md).
Insert-only remains the default without the replace option / CLI confirmation.
