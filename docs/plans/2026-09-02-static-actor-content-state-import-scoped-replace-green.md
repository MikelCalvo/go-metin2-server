# Static-actor content-state SQL import scoped replace GREEN — 2026-09-02

## Objective

Implement the opt-in tip-`0013` (`static_actor_combat_profile_state` /
interaction definitions + static actors + portable combat profiles, including
additive `0016` chase-delay / `0017` return-delay / `0018` homeward-delay /
`0019` max-step / `0020` reaction-delay columns on `static_actor_combat_profiles`)
**scoped replace** path frozen in
[static-actor content-state import replace/upsert contract freeze](2026-09-02-static-actor-content-state-import-replace-upsert-contract-freeze.md)
so operators can re-backfill a retained authored static-actor / interaction /
combat-profile export without insert-only primary-key conflicts across the three
tip-`0013` identity namespaces.

## Contract shipped

1. Default `ImportStaticActorContentState(...)` remains insert-only.
2. `ImportStaticActorContentState(..., ImportStaticActorContentStateOptions{Replace: true})`
   deletes tip-`0013` parent+child rows for each quarantined identity (children
   first because the shipped schema has no `ON DELETE CASCADE`), then inserts
   the canonicalized export rows inside one transaction.
3. Replace scope is the union of:
   - `entity_ids` (optional declared) merged with static-actor-row-derived ids
   - `interaction_definition_keys` (optional declared `{kind, ref}`) merged with
     definition-row-derived keys
   - `combat_profile_names` (optional declared) merged with combat-profile-row-
     derived names
4. A listed identity with zero parent rows is allowed as an explicit wipe-to-
   empty scope entry; empty declared scopes plus empty tip-`0013` row arrays are
   a no-op mutation after schema/quarantine preflight.
5. Identities not listed remain untouched (no global truncate).
6. Quarantine continues to reject zero/duplicate declared scopes, orphan
   merchant/quest-flag/reward-drop/death-drop children, non-contiguous drop
   positions, migration-invalid bounds, and reconstructed snapshots that fail
   authored bootstrap validation (including additive combat delay / max-step /
   reaction-delay consistency). Quarantine summary `interaction_kinds` stays
   kinds-only metadata and is **not** a wipe/replace key.
7. Unscoped `static_actors` that still reference a scoped interaction definition
   fail closed via FK and roll the transaction back. Actor `combat_profile`
   remains plain TEXT with no FK, so profile wipe does not cascade into unscoped
   actors.
8. CLI: `metin2-migrate import-export ... --i-confirm-sql-import
   --i-confirm-scoped-replace` accepts tip-`0013` `static-actor-content-state`
   beside tip-`0002` / tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` /
   tip-`0023` / tip-`0010` / tip-`0009`. Other kinds reject the replace
   confirmation as usage.
9. Successful replace stdout includes metadata-only
   `StaticActorContentStateImportResult` with `replaced: true`.
10. Still no stock production driver, no daemon mutation route, no catalog tip
    `0030`, and no DB-backed live static-actor rematerialize.

## Proof

- `go test ./internal/staticstore -run 'QuarantineStaticActorContentStateExport|ImportStaticActorContentStateRejects|ValidateStaticActorContentStateExport'`
- `go test ./internal/migratecli -run 'ImportExport.*ScopedReplace|ImportExportRejectsUsage'`
- `go test -tags=sqlite_harness ./internal/staticstore -run 'SQLiteHarnessStaticActorContentStateImport'`

## Status

GREEN on `lane/persistence`. Tip-`0007` auth-login-ticket-handoff scoped replace
GREEN is owned by
[auth-login-ticket-handoff import scoped replace GREEN](2026-09-03-auth-login-ticket-handoff-import-scoped-replace-green.md).
Production-engine selection remains deferred.
