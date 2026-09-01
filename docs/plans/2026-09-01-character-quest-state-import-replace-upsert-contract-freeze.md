# Character quest-state SQL import replace/upsert contract freeze — 2026-09-01

## Objective

Freeze the next fail-closed **scoped replace** policy for quarantined tip-`0004`
(`character_quest_state` / `character_quest_flags`) before opening RED, so
operators can re-backfill a retained quest-flag export without hitting
insert-only primary-key conflicts.

This freeze does **not** invent a stock production driver, live DB quest
repository, catalog tip `0030`, quest UI / mall surfaces, or silent row-merge
semantics.

## Why docs-first

Track E tip-`0003` scoped replace is already GREEN
([character item-state import scoped replace GREEN](2026-08-31-character-item-state-import-scoped-replace-green.md)).
Tip-`0015` safebox scoped replace is already frozen
([character safebox-state import replace/upsert contract freeze](2026-09-01-character-safebox-state-import-replace-upsert-contract-freeze.md));
its GREEN remains a follow-on (currently sitting on `lane/items`, not yet on
`main`). Catalog tip remains `0029_bootstrap_ground_item_instance_attributes`.

`queststate.ImportCharacterQuestState` is still insert-only and fails closed on
a second import of the same primary keys (explicitly “does not invent upsert /
merge policy”). That is safe for first-time backfill of durable quest flags, but
it blocks the honest operator re-backfill path after a retained tip-`0004`
export is corrected or a lab DB is rebuilt to tip and re-seeded.

Quest flags sit on the playable PvE vertical (kill-quest credit, quest-gated
NPC / merchant / warehouse / teleporter routes). Owning the re-backfill contract
next keeps that durable state operable without waiting on production-engine
selection.

Opening RED without freezing:

- default vs opt-in confirmation,
- replace scope (which table / which character ids),
- empty-export wipe vs no-op rules,
- how declared character scope interacts with wipe-to-empty,
- transaction / fail-closed semantics,
- CLI flag shape,

would invent policy mid-implementation. Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default remains insert-only

1. `ImportCharacterQuestState(...)` without an explicit replace option keeps
   today's insert-only behavior: duplicate primary keys fail closed and roll
   the transaction back.
2. `metin2-migrate import-export` without an explicit replace confirmation keeps
   today's insert-only path for every kind, including `character-quest-state`.
3. No silent upgrade of insert-only into replace.

### B. Opt-in scoped replace for tip-`0004` only (this freeze)

1. A new programmatic option (name TBD in GREEN; treat as
   `ImportCharacterQuestStateOptions{Replace: true}` mirroring tip-`0003`)
   performs **scoped replace** for the character ids present in the quarantined
   export summary.
2. Scope is exactly the tip-`0004` table already owned by
   `ImportCharacterQuestState`:
   - `character_quest_flags`
3. For each `character_id` listed in the canonicalized export summary /
   declared `CharacterIDs`:
   - delete existing rows for that character in `character_quest_flags`,
   - insert the canonicalized export rows for that character,
   - all inside **one** transaction after the existing quarantine + schema
     preflight (ledger entry for version `4` / `character_quest_state`).
4. Characters **not** listed in the export are left untouched (no global truncate).
5. Parent `accounts` / `characters` rows from tip-`0002` remain prerequisites;
   this replace path does not invent roster upsert.
6. Export tip identity stays `0004_character_quest_state`; do **not** retip
   export / quarantine / import-result identity.
7. The export's `character` name field remains operator aid only and is not
   written to SQL (same as insert-only).

### C. Declared character scope + empty / partial export semantics

1. Tip-`0004` exports gain an optional `character_ids` field (same merge rules as
   tip-`0003`): declared ids merge with flag row-derived ids so a listed
   character with zero flag rows can wipe-to-empty.
2. An export with an empty `character_ids` list and empty `flags` is a no-op
   mutation after quarantine/schema preflight (commit allowed; result counts
   stay zero).
3. Malformed / non-quarantinable exports continue to fail before any DELETE /
   INSERT.
4. Replace of a listed character replaces that character's entire
   `character_quest_flags` set for the export; there is no per-quest_ref
   half-replace mode in this freeze.

### D. CLI confirmation shape (tip-`0004` added beside owned replace kinds)

1. Replace stays off unless the operator passes an explicit confirmation flag
   in addition to `--i-confirm-sql-import`. GREEN should reuse the existing
   `--i-confirm-scoped-replace` flag and widen it to also accept
   `--kind character-quest-state` (still reject every other kind that has not
   frozen+GREEN'd its own replace path).
2. Successful stdout remains metadata-only `CharacterQuestStateImportResult`
   JSON (no DSN, no SQL text, no flag payloads). GREEN should add
   `replaced: true` (omitempty) mirroring tip-`0003`.
3. Print-only `import-export-drill` does **not** auto-enable replace; any later
   drill printer change is a separate slice.

### E. Explicit non-goals

- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- live DB quest repository replacing FileStore rematerialize
- catalog tip `0030` / retip of tip-`0004` / `0003` / `0015` / `0010` export
  identities
- upsert / replace for roster (`0002`), points (`0011`), safebox (`0015`),
  ground (`0010`), templates (`0009`), tickets (`0007`), static actors (`0013`),
  or myshop prices (`0023`) in this freeze (tip-`0003` replace stays owned;
  tip-`0015` replace stays owned by its own freeze/GREEN)
- silent merge / per-row `ON CONFLICT DO UPDATE` without scoped DELETE
- online mutation while selected-character sessions are live (CLI remains an
  offline operator tool; no daemon mutation route)
- quest UI / remote admin / secrets in git / README churn / gameplay changes
- another crash/restart rematerialize twin (quest-flag FileStore rematerialize
  is already GREEN)

## Proof shape (for later RED → GREEN)

1. SQLite harness: first insert-only import succeeds; second insert-only import
   of the same export fails closed; opt-in replace of the same export succeeds
   and leaves exactly the canonical flag rows.
2. Scoped wipe: character A in export, character B only in DB beforehand →
   replace updates A and leaves B untouched.
3. Empty wipe: listed character with zero flag rows via declared
   `character_ids` → that character's `character_quest_flags` rows become empty
   inside the transaction.
4. CLI: insert-only confirmation alone cannot replace; `--i-confirm-scoped-replace`
   is accepted for `character-quest-state` (and still for already-owned replace
   kinds); other unfrozen kinds still reject it.
5. Negatives: missing schema tip `4`, bad quarantine, nil executor, and
   FK-missing parent characters still fail closed before commit.

## Likely files to change (later GREEN, not this freeze)

- `internal/queststate/export.go` / `export_quarantine.go` (optional
  `character_ids` merge, mirroring tip-`0003`)
- `internal/queststate/quest_state_import.go` (+ unit / sqlite harness tests)
- `internal/migratecli` import-export flag wiring (+ tests)
- `docs/development.md` / `docs/workflow/migration-apply-runbook.md` (GREEN
  wording once the kind is accepted)
- Track E / migration-contract next-slice pointers (flip freeze → Done on GREEN)

## Status

Docs/spec freeze landed first; tip-`0004` scoped replace GREEN is now owned by
[character quest-state import scoped replace GREEN](2026-09-01-character-quest-state-import-scoped-replace-green.md).
Insert-only remains the default without the replace option / CLI confirmation.
