# Bootstrap ground-item-state SQL import replace/upsert contract freeze — 2026-09-02

## Objective

Freeze the next fail-closed **scoped replace** policy for quarantined tip-`0010`
(`bootstrap_ground_item_state` / `bootstrap_ground_items`, including additive
`0026` instance sockets + `0029` instance attributes) before opening RED, so
operators can re-backfill a retained pending ground-handle export without
hitting insert-only primary-key conflicts on `vid`.

This freeze does **not** invent a stock production driver, live DB ground
repository, catalog tip `0030`, DB-backed live rematerialize, remote admin, or
silent row-merge semantics.

## Why docs-first

Track E character-domain and roster scoped replace paths are already GREEN on
`main`:

- [character item-state import scoped replace GREEN](2026-08-31-character-item-state-import-scoped-replace-green.md)
- [character safebox-state import scoped replace GREEN](2026-09-01-character-safebox-state-import-scoped-replace-green.md)
- [character quest-state import scoped replace GREEN](2026-09-01-character-quest-state-import-scoped-replace-green.md)
- [character point-state import scoped replace GREEN](2026-09-01-character-point-state-import-scoped-replace-green.md)
- [character myshop unit-prices import scoped replace GREEN](2026-09-01-character-myshop-unit-prices-import-scoped-replace-green.md)
- [account/character roster import scoped replace GREEN](2026-09-02-account-character-roster-import-scoped-replace-green.md)

Catalog tip remains `0029_bootstrap_ground_item_instance_attributes`.

`worldruntime.ImportBootstrapGroundItemState` is still insert-only and fails
closed on a second import of the same `vid` primary keys (explicitly “does not
invent upsert / merge policy”). That is safe for first-time backfill of pending
ground handles, but it blocks the honest operator re-backfill path after a
retained tip-`0010` export is corrected or a lab DB is rebuilt to tip and
re-seeded.

Pending ground handles sit on the playable PvE kill → reward/drop → reconnect
vertical (FileStore rematerialize is already owned; SQL import is already owned
insert-only). Owning the re-backfill contract next keeps that durable projection
operable without waiting on production-engine selection.

Opening RED without freezing:

- default vs opt-in confirmation,
- replace scope (which table / which visible ground VIDs),
- declared VID scope vs row-derived VIDs,
- wipe-to-empty / empty no-op semantics,
- transaction / fail-closed semantics,
- CLI flag shape,

would invent policy mid-implementation. Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default remains insert-only

1. `ImportBootstrapGroundItemState(...)` without an explicit replace option keeps
   today's insert-only behavior: duplicate primary keys fail closed and roll the
   transaction back.
2. `metin2-migrate import-export` without an explicit replace confirmation keeps
   today's insert-only path for every kind, including
   `bootstrap-ground-item-state`.
3. No silent upgrade of insert-only into replace.

### B. Opt-in scoped replace for tip-`0010` only (this freeze)

1. A new programmatic option (name TBD in GREEN; treat as
   `ImportBootstrapGroundItemStateOptions{Replace: true}` mirroring tip-`0002` /
   tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023`) performs
   **scoped replace** for the visible ground VIDs present in the quarantined
   export summary.
2. Scope is exactly the tip-`0010` table already owned by
   `ImportBootstrapGroundItemState`:
   - `bootstrap_ground_items` (including additive `0026` socket columns and
     additive `0029` attribute columns already required by the insert schema
     gate)
3. For each `vid` listed in the canonicalized export summary / declared `VIDs`:
   - delete the existing `bootstrap_ground_items` row for that `vid` (if any),
   - insert the canonicalized export row for that `vid` when present,
   - all inside **one** transaction after the existing quarantine + schema
     preflight (ledger entries for version `10` / `bootstrap_ground_item_state`
     plus additive `26` / `bootstrap_ground_item_instance_sockets` plus additive
     `29` / `bootstrap_ground_item_instance_attributes`).
4. VIDs **not** listed in the export are left untouched (no global truncate).
5. Parent `characters` rows from tip-`0002` remain prerequisites for inserted
   owner identity; this replace path does not invent roster upsert.
6. Export tip identity stays `0010_bootstrap_ground_item_state`; do **not** retip
   export / quarantine / import-result identity (additive sockets/attributes keep
   tip-`0010` identity).

### C. Declared VID scope + empty semantics

1. Tip-`0010` exports gain an optional `vids` field (same merge idea as
   tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023` `character_ids`
   and tip-`0002` `account_ids`): declared ids merge with ground-row-derived VIDs
   so a listed VID with zero ground rows can wipe-to-empty.
2. Quarantine continues to reject zero `vid`, duplicate `vid`, invalid owner /
   map / pickup bounds, exclusive item-count vs gold-amount shape violations,
   gold `vnum != 1`, presence-aware socket/attribute inconsistencies
   (`has_sockets=false` requires zero sockets; `has_attributes=false` requires
   zero attributes; gold-shaped rows stay socket-less and attribute-less). Those
   fail closed before any DELETE / INSERT.
3. A listed VID with zero ground rows is allowed only as an explicit wipe-to-empty
   scope entry (via declared `vids`).
4. An export with an empty `vids` list and empty `ground_items` is a no-op
   mutation after quarantine/schema preflight (commit allowed; result counts stay
   zero).
5. Malformed / non-quarantinable exports continue to fail before any DELETE /
   INSERT.
6. Replace of a listed VID replaces that VID's entire tip-`0010` row for the
   export (including additive socket/attribute columns). There is no per-column
   half-replace / silent merge mode in this freeze.

### D. CLI confirmation shape (tip-`0010` added beside owned replace kinds)

1. Replace stays off unless the operator passes an explicit confirmation flag
   in addition to `--i-confirm-sql-import`. GREEN should reuse the existing
   `--i-confirm-scoped-replace` flag and widen it to also accept
   `--kind bootstrap-ground-item-state` (still reject every other kind that has
   not frozen+GREEN'd its own replace path).
2. Successful stdout remains metadata-only
   `BootstrapGroundItemStateImportResult` JSON (no DSN, no SQL text, no ground
   payloads). GREEN should add `replaced: true` (omitempty) mirroring tip-`0002`
   / tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023`.
3. Print-only `import-export-drill` does **not** auto-enable replace; any later
   drill printer change is a separate slice.

### E. Explicit non-goals

- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- live DB ground repository replacing FileStore rematerialize
- catalog tip `0030` / retip of tip-`0010` / `0002` / `0003` / `0004` / `0011` /
  `0015` / `0023` export identities
- upsert / replace for templates (`0009`), tickets (`0007`), or static actors
  (`0013` / tip-`0008` content) in this freeze (tip-`0002` / tip-`0003` /
  tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023` replace stay owned)
- owner-character-scoped cascade wipe of unrelated VIDs not listed in the export
- silent merge / per-row `ON CONFLICT DO UPDATE` without scoped DELETE
- online mutation of live shared-world handles (CLI remains an offline operator
  tool; no daemon mutation route; FileStore / MemoryStore / live AOI stay
  untouched)
- remote admin, secrets in git, README churn, gameplay changes
- another crash/restart rematerialize twin (FileStore pending-ground rematerialize
  is already GREEN)

## Proof shape (for later RED → GREEN)

1. SQLite harness: first insert-only import succeeds; second insert-only import
   of the same export fails closed; opt-in replace of the same export succeeds
   and leaves exactly the canonical ground rows (item-shaped + gold-shaped,
   including presence-aware sockets/attributes).
2. Scoped wipe: VID A in export, VID B only in DB beforehand → replace updates A
   and leaves B untouched.
3. Empty wipe: listed VID with zero ground rows via declared `vids` → that VID's
   `bootstrap_ground_items` row becomes absent inside the transaction.
4. CLI: insert-only confirmation alone cannot replace;
   `--i-confirm-scoped-replace` is accepted for `bootstrap-ground-item-state`
   (and still for already-owned replace kinds); other unfrozen kinds still
   reject it.
5. Negatives: missing schema tip `10` / additive `26` / additive `29`, bad
   quarantine (duplicate vid / both item+gold / gold sockets), nil executor, and
   FK-missing parent characters still fail closed before commit.

## Likely files to change (later GREEN, not this freeze)

- `internal/worldruntime/migration_export.go` /
  `migration_export_quarantine.go` (optional `vids` merge + wipe-to-empty
  exception)
- `internal/worldruntime/ground_item_state_import.go` (+ unit / sqlite harness
  tests)
- `internal/migratecli` import-export flag wiring (+ tests)
- `docs/development.md` / `docs/workflow/migration-apply-runbook.md` (GREEN
  wording once the kind is accepted)
- Track E / migration-contract next-slice pointers (flip freeze → Done on GREEN)

## Status

Docs/spec freeze landed. Tip-`0010` scoped replace GREEN remains follow-on.
Insert-only remains the default without the replace option / CLI confirmation.
Production-engine selection remains deferred.
