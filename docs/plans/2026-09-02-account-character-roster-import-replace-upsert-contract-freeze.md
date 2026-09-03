# Account/character roster SQL import replace/upsert contract freeze — 2026-09-02

## Objective

Freeze the next fail-closed **scoped replace** policy for quarantined tip-`0002`
(`account_character_roster` / `accounts` + `characters`) before opening RED, so
operators can re-backfill a retained select-screen roster export without hitting
insert-only primary-key / unique-index conflicts.

This freeze does **not** invent a stock production driver, live DB login/select
repository, catalog tip `0030`, cascade-delete of child tip domains, silent
row-merge semantics, or remote admin.

## Why docs-first

Track E character-child scoped replace paths are already GREEN on `main`:

- [character item-state import scoped replace GREEN](2026-08-31-character-item-state-import-scoped-replace-green.md)
- [character safebox-state import scoped replace GREEN](2026-09-01-character-safebox-state-import-scoped-replace-green.md)
- [character quest-state import scoped replace GREEN](2026-09-01-character-quest-state-import-scoped-replace-green.md)
- [character point-state import scoped replace GREEN](2026-09-01-character-point-state-import-scoped-replace-green.md)
- [character myshop unit-prices import scoped replace GREEN](2026-09-01-character-myshop-unit-prices-import-scoped-replace-green.md)

Catalog tip remains `0029_bootstrap_ground_item_instance_attributes`.

`accountstore.ImportAccountCharacterRoster` is still insert-only and fails
closed on a second import of the same account/character primary keys or unique
normalized-login / normalized-name / `(account_id, slot)` collisions (explicitly
“does not invent upsert / merge policy”). That is safe for first-time backfill
of the select-screen roster, but it blocks the honest operator re-backfill path
after a retained tip-`0002` export is corrected or a lab DB is rebuilt to tip
and re-seeded.

Roster rows are the FK parent for every character-child import already owned by
Track E (`0003` / `0004` / `0011` / `0015` / `0023`, plus ground-item owner
identity). Owning the re-backfill contract next keeps the playable login →
select → map vertical operable without waiting on production-engine selection.

Opening RED without freezing:

- default vs opt-in confirmation,
- replace scope (which tables / which account ids),
- account-scoped character wipe vs character-id-only rewrite,
- declared account scope vs row-derived ids,
- FK-safe delete order and fail-closed child-domain policy,
- transaction / empty / wipe-to-empty semantics,
- CLI flag shape,

would invent policy mid-implementation. Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default remains insert-only

1. `ImportAccountCharacterRoster(...)` without an explicit replace option keeps
   today's insert-only behavior: duplicate primary keys / unique-index
   collisions fail closed and roll the transaction back.
2. `metin2-migrate import-export` without an explicit replace confirmation keeps
   today's insert-only path for every kind, including
   `account-character-roster`.
3. No silent upgrade of insert-only into replace.

### B. Opt-in scoped replace for tip-`0002` only (this freeze)

1. A new programmatic option (name TBD in GREEN; treat as
   `ImportAccountCharacterRosterOptions{Replace: true}` mirroring tip-`0003` /
   tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023`) performs **scoped replace**
   for the account ids present in the quarantined export summary.
2. Scope is exactly the tip-`0002` tables already owned by
   `ImportAccountCharacterRoster`:
   - `accounts`
   - `characters`
3. For each `account_id` listed in the canonicalized export summary / declared
   `AccountIDs`:
   - delete existing `characters` rows for that account,
   - delete the existing `accounts` row for that account,
   - insert the canonicalized export account row (when present),
   - insert the canonicalized export character rows for that account,
   - all inside **one** transaction after the existing quarantine + schema
     preflight (ledger entry for version `2` / `account_character_roster`).
4. Accounts **not** listed in the export are left untouched (no global truncate).
5. Export tip identity stays `0002_account_character_roster`; do **not** retip
   export / quarantine / import-result identity.
6. GREEN must delete `characters` before `accounts` for a listed account so the
   tip-`0002` FK (`characters.account_id → accounts.id`) cannot fail closed mid
   replace.

### C. Declared account scope + empty / wipe semantics

1. Tip-`0002` exports gain an optional `account_ids` field (same merge idea as
   tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023` `character_ids`):
   declared ids merge with account-row-derived ids so a listed account with zero
   account/character rows can wipe-to-empty.
2. Quarantine continues to reject zero/negative ids, empty logins/names,
   duplicate account ids / normalized logins, duplicate character ids /
   normalized names, duplicate `(account_id, slot)`, character `account_id`
   references missing from the same export's account rows, `level < 1`,
   `map_index <= 0`, and gold outside signed BIGINT. Those fail closed before
   any DELETE / INSERT.
3. A listed account with zero account rows and zero character rows is allowed
   only as an explicit wipe-to-empty scope entry (via declared `account_ids`).
4. An export with an empty `account_ids` list, empty `accounts`, and empty
   `characters` is a no-op mutation after quarantine/schema preflight (commit
   allowed; result counts stay zero).
5. Malformed / non-quarantinable exports continue to fail before any DELETE /
   INSERT.
6. Replace of a listed account replaces that account's entire tip-`0002` roster
   set for the export (`accounts` row + all `characters` slots for that
   account). There is no per-slot half-replace / silent merge mode in this
   freeze.
7. Characters belonging to a listed account but absent from the export are
   removed by the scoped character DELETE; this freeze does **not** invent a
   keep-missing-slots merge mode.

### D. FK-safe child-domain policy (explicit fail-closed)

1. Tip-`0002` replace deletes only `characters` / `accounts` for the listed
   account scope. It does **not** cascade-delete tip-`0003` inventory /
   equipment / quickslots, tip-`0004` quest flags, tip-`0011` points,
   tip-`0015` safebox rows, tip-`0023` myshop unit prices, tip-`0010` ground
   owner rows, tip-`0007` tickets, tip-`0009` templates, or tip-`0013` static
   actors.
2. If a listed account's existing character ids still have child-domain FK
   dependents, GREEN must fail closed and roll the transaction back rather than
   inventing cascade purge or orphan-repair policy in this slice.
3. Operators who need child-domain rewrite remain on the already-GREEN
   character-scoped replace paths (`0003` / `0004` / `0011` / `0015` / `0023`)
   before or after a roster replace, as a separate confirmed import.
4. This freeze does not invent a multi-kind transactional “replace account tree”
   operator command.

### E. CLI confirmation shape (tip-`0002` added beside owned replace kinds)

1. Replace stays off unless the operator passes an explicit confirmation flag
   in addition to `--i-confirm-sql-import`. GREEN should reuse the existing
   `--i-confirm-scoped-replace` flag and widen it to also accept
   `--kind account-character-roster` (still reject every other kind that has
   not frozen+GREEN'd its own replace path).
2. Successful stdout remains metadata-only
   `AccountCharacterRosterImportResult` JSON (no DSN, no SQL text, no roster
   payloads). GREEN should add `replaced: true` (omitempty) mirroring tip-`0003`
   / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023`.
3. Print-only `import-export-drill` does **not** auto-enable replace by default; opt-in `--i-confirm-print-scoped-replace` is owned by
   [import-export-drill opt-in scoped-replace printer](2026-09-03-import-export-drill-opt-in-scoped-replace.md).

### F. Explicit non-goals

- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- live DB account/character repository replacing FileStore login/select
- catalog tip `0030` / retip of tip-`0002` / `0003` / `0004` / `0011` / `0015` /
  `0023` / `0010` export identities
- upsert / replace for ground (`0010`), templates (`0009`), tickets (`0007`), or
  static actors (`0013`) in this freeze (tip-`0003` / tip-`0004` / tip-`0011` /
  tip-`0015` / tip-`0023` replace stay owned)
- cascade-delete or rewrite of character-child tip domains from the roster
  replace path
- silent merge / per-row `ON CONFLICT DO UPDATE` without scoped DELETE
- online mutation while selected-character sessions are live (CLI remains an
  offline operator tool; no daemon mutation route)
- password / hash columns, remote admin, secrets in git, README churn, gameplay
  changes
- another crash/restart rematerialize twin (FileStore account/character
  rematerialize is already owned)

## Proof shape (for later RED → GREEN)

1. SQLite harness: first insert-only import succeeds; second insert-only import
   of the same export fails closed; opt-in replace of the same export succeeds
   and leaves exactly the canonical account/character rows.
2. Scoped wipe: account A in export, account B only in DB beforehand → replace
   updates A and leaves B untouched.
3. Empty wipe: listed account with zero account/character rows via declared
   `account_ids` → that account's tip-`0002` rows become empty inside the
   transaction.
4. FK fail-closed: listed account whose characters still have tip-`0003` (or
   other child) dependents fails closed and rolls back without deleting sibling
   accounts.
5. CLI: insert-only confirmation alone cannot replace;
   `--i-confirm-scoped-replace` is accepted for `account-character-roster`
   (and still for already-owned replace kinds); other unfrozen kinds still
   reject it.
6. Negatives: missing schema tip `2`, bad quarantine (duplicate login /
   duplicate slot), nil executor, and unique-name collisions against untouched
   accounts still fail closed before commit.

## Likely files to change (later GREEN, not this freeze)

- `internal/accountstore/roster_export.go` / `roster_quarantine.go` (optional
  `account_ids` merge + wipe-to-empty exception)
- `internal/accountstore/roster_import.go` (+ unit / sqlite harness tests)
- `internal/migratecli` import-export flag wiring (+ tests)
- `docs/development.md` / `docs/workflow/migration-apply-runbook.md` (GREEN
  wording once the kind is accepted)
- Track E / migration-contract next-slice pointers (flip freeze → Done on GREEN)

## Status

Docs/spec freeze landed. Tip-`0002` scoped replace GREEN is owned by
[account/character roster import scoped replace GREEN](2026-09-02-account-character-roster-import-scoped-replace-green.md).
Insert-only remains the default without the replace option / CLI confirmation.
Production-engine selection remains deferred.
