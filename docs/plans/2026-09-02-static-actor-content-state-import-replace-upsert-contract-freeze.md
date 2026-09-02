# Static-actor content-state SQL import replace/upsert contract freeze — 2026-09-02

## Objective

Freeze the next fail-closed **scoped replace** policy for quarantined tip-`0013`
(`static_actor_combat_profile_state` / interaction definitions + static actors +
portable combat profiles, including additive `0016` chase-delay / `0017`
return-delay / `0018` homeward-delay / `0019` max-step / `0020` reaction-delay
columns on `static_actor_combat_profiles`) before opening RED, so operators can
re-backfill a retained authored static-actor / interaction / combat-profile
export without hitting insert-only primary-key or unique-index conflicts.

This freeze does **not** invent a stock production driver, live DB content
repository, catalog tip `0030`, DB-backed live rematerialize, remote admin,
cascade rewrite of unscoped FK dependents, or silent row-merge semantics.

## Why docs-first

Track E character-domain, roster, pending-ground, and item-template scoped
replace paths are already GREEN on `main`:

- [character item-state import scoped replace GREEN](2026-08-31-character-item-state-import-scoped-replace-green.md)
- [character safebox-state import scoped replace GREEN](2026-09-01-character-safebox-state-import-scoped-replace-green.md)
- [character quest-state import scoped replace GREEN](2026-09-01-character-quest-state-import-scoped-replace-green.md)
- [character point-state import scoped replace GREEN](2026-09-01-character-point-state-import-scoped-replace-green.md)
- [character myshop unit-prices import scoped replace GREEN](2026-09-01-character-myshop-unit-prices-import-scoped-replace-green.md)
- [account/character roster import scoped replace GREEN](2026-09-02-account-character-roster-import-scoped-replace-green.md)
- [bootstrap ground-item-state import scoped replace GREEN](2026-09-02-bootstrap-ground-item-state-import-scoped-replace-green.md)
- [item-template-state import scoped replace GREEN](2026-09-02-item-template-state-import-scoped-replace-green.md)

Catalog tip remains `0029_bootstrap_ground_item_instance_attributes`.

`staticstore.ImportStaticActorContentState` is still insert-only and fails closed
on a second import of the same tip-`0013` primary keys / unique indexes
(explicitly “does not invent upsert / merge policy”). That is safe for
first-time backfill of authored NPC / shop / quest-flag / combat-profile
content, but it blocks the honest operator re-backfill path after a retained
tip-`0013` export is corrected or a lab DB is rebuilt to tip and re-seeded.

Authored static actors, interaction definitions, and portable combat profiles
sit under the playable map → NPC / merchant / quest-flag / mob-kill → reward
vertical already owned by FileStore rematerialize. Owning the re-backfill
contract next keeps that durable projection operable without waiting on
production-engine selection or the remaining ticket (`0007`) replace freeze.

Opening RED without freezing:

- default vs opt-in confirmation,
- replace scope across the three tip-`0013` identity namespaces,
- FK-safe delete order (no `ON DELETE CASCADE` on shipped child FKs),
- declared scope lists vs row-derived ids,
- wipe-to-empty / empty no-op semantics,
- fail-closed policy when unscoped actors still reference a scoped interaction,
- transaction / CLI flag shape,

would invent policy mid-implementation. Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default remains insert-only

1. `ImportStaticActorContentState(...)` without an explicit replace option keeps
   today's insert-only behavior: duplicate primary keys / unique-index
   collisions fail closed and roll the transaction back.
2. `metin2-migrate import-export` without an explicit replace confirmation keeps
   today's insert-only path for every kind, including
   `static-actor-content-state`.
3. No silent upgrade of insert-only into replace.

### B. Opt-in scoped replace for tip-`0013` only (this freeze)

1. A new programmatic option (name TBD in GREEN; treat as
   `ImportStaticActorContentStateOptions{Replace: true}` mirroring tip-`0002` /
   tip-`0003` / tip-`0004` / tip-`0009` / tip-`0010` / tip-`0011` / tip-`0015` /
   tip-`0023`) performs **scoped replace** for the tip-`0013` identities present
   in the quarantined export summary / declared scope lists.
2. Scope is exactly the tip-`0013` parent + child tables already owned by
   `ImportStaticActorContentState`:
   - `interaction_definitions`
   - `interaction_merchant_catalog_entries`
   - `interaction_quest_flag_reward_items`
   - `interaction_quest_flag_consume_items`
   - `static_actors`
   - `static_actor_reward_drops`
   - `static_actor_combat_profiles` (including additive `0016` /
     `0017` / `0018` / `0019` / `0020` columns already required by the insert
     schema gate)
   - `static_actor_combat_profile_death_reward_drops`
3. Tip-`0013` has **three** identity namespaces. Replace scope is the union of:
   - `entity_id` values for `static_actors` / `static_actor_reward_drops`
   - `(kind, ref)` keys for `interaction_definitions` and their merchant /
     quest-flag child tables
   - `profile` names for `static_actor_combat_profiles` /
     `static_actor_combat_profile_death_reward_drops`
4. For each identity listed in the canonicalized export summary / declared
   scope fields:
   - delete existing tip-`0013` child rows for that identity first (because the
     shipped schema declares `FOREIGN KEY (... ) REFERENCES ...` **without**
     `ON DELETE CASCADE`), then delete the parent row,
   - insert the canonicalized export parent/child rows for that identity when
     present,
   - all inside **one** transaction after the existing quarantine + schema
     preflight (ledger entries for version `13` /
     `static_actor_combat_profile_state` plus additive `16` /
     `static_actor_combat_profile_chase_delay`, `17` /
     `static_actor_combat_profile_return_delay`, `18` /
     `static_actor_combat_profile_homeward_delay`, `19` /
     `static_actor_combat_profile_max_step`, and `20` /
     `static_actor_combat_profile_reaction_delay`).
5. Recommended GREEN delete order (FK-safe, children before parents):
   1. `static_actor_reward_drops` for scoped `entity_id`s
   2. `static_actors` for scoped `entity_id`s
   3. `interaction_quest_flag_reward_items` /
      `interaction_quest_flag_consume_items` /
      `interaction_merchant_catalog_entries` for scoped `(kind, ref)` keys
   4. `interaction_definitions` for scoped `(kind, ref)` keys
   5. `static_actor_combat_profile_death_reward_drops` for scoped `profile`s
   6. `static_actor_combat_profiles` for scoped `profile`s
6. Recommended GREEN insert order stays the already-owned insert-only order:
   interaction definitions → merchant catalog → quest-flag reward/consume items
   → static actors → actor reward drops → combat profiles → profile death-reward
   drops.
7. Identities **not** listed in the export/declared scope are left untouched (no
   global truncate).
8. Export tip identity stays `0013_static_actor_combat_profile_state`; do **not**
   retip export / quarantine / import-result identity (additive `0016`–`0020`
   keep tip-`0013` identity).
9. `static_actors.combat_profile` remains a plain TEXT column with **no** FK to
   `static_actor_combat_profiles`. Profile replace therefore does not invent an
   actor↔profile FK cascade; actor rows outside entity scope stay untouched even
   when a profile they name is replaced.

### C. Declared multi-namespace scope + empty / wipe semantics

1. Tip-`0013` exports gain optional declared scope fields that merge with
   row-derived identities (same merge idea as tip-`0002` `account_ids`, tip-`0003`
   / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023` `character_ids`, tip-`0009`
   `vnums`, and tip-`0010` `vids`):
   - `entity_ids` (`[]uint64`) — merge with static-actor-row-derived entity ids
   - `interaction_definition_keys` (structured `{kind, ref}` rows, or an
     equivalent GREEN encoding that preserves both halves of the PK) — merge
     with definition-row-derived `(kind, ref)` keys
   - `combat_profiles` (`[]string`) — merge with combat-profile-row-derived
     profile names
2. Quarantine summary `interaction_kinds` stays kinds-only metadata and is
   **not** a wipe/replace scope key. Declared wipe/replace of definitions must
   use full `(kind, ref)` keys.
3. Quarantine continues to reject duplicate entity ids / definition keys /
   profile names, orphan merchant/quest-flag/reward-drop/death-drop children,
   non-contiguous drop positions, migration-invalid bounds, and reconstructed
   snapshots that fail authored bootstrap validation (including additive combat
   delay / max-step / reaction-delay consistency). Those fail closed before any
   DELETE / INSERT.
4. A listed identity with zero parent rows for that namespace is allowed only as
   an explicit wipe-to-empty scope entry (via the matching declared scope
   field).
5. An export with empty declared scope lists and empty tip-`0013` row arrays is
   a no-op mutation after quarantine/schema preflight (commit allowed; result
   counts stay zero).
6. Malformed / non-quarantinable exports continue to fail before any DELETE /
   INSERT.
7. Replace of a listed identity replaces that identity's entire tip-`0013`
   parent+child set for its namespace. There is no per-column / per-child
   half-replace / silent merge mode in this freeze.

### D. FK-safe unscoped-dependent policy (explicit fail-closed)

1. Tip-`0013` replace deletes only scoped tip-`0013` rows. It does **not**
   cascade-delete or rewrite tip-`0002` / `0003` / `0004` / `0007` / `0009` /
   `0010` / `0011` / `0015` / `0023` domains.
2. If a scoped interaction definition is still referenced by an **unscoped**
   `static_actors.interaction_kind` / `interaction_ref` pair that survives the
   entity-scoped DELETE, GREEN must fail closed and roll the transaction back
   rather than inventing cascade purge, nulling, or orphan-repair policy.
3. Operators who need both the referencing actor and the definition rewritten
   must include both identities in the same replace export / declared scope.
4. This freeze does not invent a multi-kind transactional “replace world content
   tree” operator command spanning tickets, templates, or character domains.

### E. CLI confirmation shape (tip-`0013` added beside owned replace kinds)

1. Replace stays off unless the operator passes an explicit confirmation flag
   in addition to `--i-confirm-sql-import`. GREEN should reuse the existing
   `--i-confirm-scoped-replace` flag and widen it to also accept
   `--kind static-actor-content-state` (still reject every other kind that has
   not frozen+GREEN'd its own replace path — after this freeze that still
   excludes `auth-login-ticket-handoff` until its own freeze+GREEN).
2. Successful stdout remains metadata-only
   `StaticActorContentStateImportResult` JSON (no DSN, no SQL text, no content
   payloads). GREEN should add `replaced: true` (omitempty) mirroring tip-`0002`
   / tip-`0003` / tip-`0004` / tip-`0009` / tip-`0010` / tip-`0011` / tip-`0015` /
   tip-`0023`.
3. Print-only `import-export-drill` does **not** auto-enable replace; any later
   drill printer change is a separate slice.

### F. Explicit non-goals

- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- live DB static-actor / interaction / combat-profile repository replacing
  FileStore / MemoryStore authored indexes
- catalog tip `0030` / retip of tip-`0013` / `0002` / `0003` / `0004` / `0009` /
  `0010` / `0011` / `0015` / `0023` export identities
- upsert / replace for tickets (`0007`) in this freeze (tip-`0002` / tip-`0003` /
  tip-`0004` / tip-`0009` / tip-`0010` / tip-`0011` / tip-`0015` / tip-`0023`
  replace stay owned; tip-`0013` replace is frozen here)
- inventing an actor↔combat-profile SQL FK or cascade from profile wipe into
  unscoped actors
- silent merge / per-row `ON CONFLICT DO UPDATE` without scoped DELETE
- online mutation of live shared-world static actors / interaction indexes /
  content bundles (CLI remains an offline operator tool; no daemon mutation
  route; FileStore / MemoryStore / live runtime stay untouched)
- remote admin, secrets in git, README churn, gameplay changes
- another crash/restart rematerialize twin (authored static-actor /
  interaction FileStore reload is already owned)

## Proof shape (for later RED → GREEN)

1. SQLite harness: first insert-only import succeeds; second insert-only import
   of the same export fails closed; opt-in replace of the same export succeeds
   and leaves exactly the canonical tip-`0013` parent+child rows (including
   additive chase/return/homeward/max-step/reaction delay columns).
2. Scoped wipe across namespaces: entity A + definition K + profile P in export,
   sibling entity B / definition L / profile Q only in DB beforehand → replace
   updates A/K/P and leaves B/L/Q untouched.
3. Empty wipe: listed entity / definition key / profile with zero parent rows via
   declared scope → that identity's tip-`0013` parent+child rows become absent
   inside the transaction.
4. FK fail-closed: scoped definition still referenced by an unscoped actor fails
   closed and rolls back without deleting sibling unscoped content.
5. CLI: insert-only confirmation alone cannot replace;
   `--i-confirm-scoped-replace` is accepted for `static-actor-content-state`
   (and still for already-owned replace kinds); `auth-login-ticket-handoff`
   and any other unfrozen kind still reject it.
6. Negatives: missing schema tip `13` / additive `16`–`20`, bad quarantine
   (duplicate entity / orphan child / invalid drop positions), nil executor, and
   FK-ordering mistakes still fail closed before commit.

## Likely files to change (later GREEN, not this freeze)

- `internal/staticstore/migration_export.go` /
  `migration_export_quarantine.go` (optional `entity_ids` /
  `interaction_definition_keys` / `combat_profiles` merge + wipe-to-empty
  exceptions)
- `internal/staticstore/static_actor_content_state_import.go` (+ unit / sqlite
  harness tests)
- `internal/migratecli` import-export flag wiring (+ tests)
- `docs/development.md` / `docs/workflow/migration-apply-runbook.md` (GREEN
  wording once the kind is accepted)
- Track E / migration-contract next-slice pointers (flip freeze → Done on GREEN)

## Status

Docs/spec freeze landed. Tip-`0013` scoped replace GREEN is the next
implementation slice after this freeze. Insert-only remains the default without
the replace option / CLI confirmation. Upsert / replace for
`auth-login-ticket-handoff` (`0007`) stays deferred behind its own freeze.
Production-engine selection remains deferred.
