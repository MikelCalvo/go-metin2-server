# Static Actor Interaction Authoring

This document freezes the first loopback-only authoring and promotion surface for bootstrap static actors plus their minimal interaction definitions.

It sits on top of:
- `static-actor-interaction-bootstrap.md`
- `static-actor-interaction-request.md`
- `non-player-entity-bootstrap.md`

## Scope

This contract currently applies only to:
- loopback-only operator HTTP endpoints on `gamed`
- deterministic authoring of minimal `info`, `talk`, and `warp` definitions plus the frozen contract boundary for the next structured `shop_preview` merchant catalog
- deterministic export/import of bootstrap static actors together with their interaction definitions

It does **not** yet claim:
- public/admin-authenticated remote authoring
- merge semantics across environments
- partial import semantics
- quest/script payloads
- real merchant transactions, branching dialogs, or richer authored UI state

## Interaction-definition authoring

The first owned catalog surface is:
- `GET /local/interactions`
- `POST /local/interactions`
- `PATCH /local/interactions/{kind}/{ref}`
- `PUT /local/interactions/{kind}/{ref}`
- `DELETE /local/interactions/{kind}/{ref}`

Current rules:
- bodies always use JSON `kind` and `ref`
- `ref` is a canonical path-safe interaction key in the form `<namespace>:<name>`; both segments start with a lowercase ASCII letter and then contain only lowercase ASCII letters, digits, or `_`
- refs without a namespace, refs containing `/`, whitespace, dots, hyphens, uppercase letters, blank segments, or extra `:` separators are rejected before persistence/import
- `info` / `talk` currently use authored `text`
- `shop_preview` currently uses authored `title + catalog[]`
- `warp` currently uses authored `map_index`, `x`, `y`, with optional `text`
- updates are full-identity upserts, not partial nested edits
- update body `kind + ref` must match the path exactly
- delete fails closed while any bootstrap static actor still references that definition
- the backing catalog remains deterministic and file-backed under `internal/interactionstore`
- the file-backed loader rejects unknown top-level JSON fields and trailing JSON instead of accepting a partial object silently
- the static-actor store accepts interaction metadata only for currently owned definition kinds (`info`, `talk`, `warp`, `shop_preview`); future content kinds must be added to the interaction definition catalog before static actors can reference them durably

## Interaction-focused QA visibility

The first owned QA/debugging surface is:
- `GET /local/interaction-visibility`

It returns, per connected bootstrap player:
- the player snapshot
- the currently visible interactable static actors only
- each actor's `interaction_kind`
- each actor's `interaction_ref`
- a compact resolved preview when the referenced definition currently resolves to a currently previewable kind (`info`, `talk`, `shop_preview`, `warp`)
- a fail-closed `resolution_failure` marker when it does not

This is intended for live QA/debugging without packet captures.

## Deterministic authored-content bundle

The first owned bundle surface is:
- `GET /local/content-bundle`
- `POST /local/content-bundle`
- `GET /local/content-bundle/summary`
- `POST /local/content-bundle/summary`
- `POST /local/content-bundle/validate`

Current rules:
- export returns one deterministic JSON artifact containing:
  - `static_actors`
  - `spawn_groups` when authored spawn-backed actors are present
  - `combat_profiles` when a referenced non-default combat profile must travel with spawn content
  - `item_templates` when the runtime has an authored item-template snapshot loaded
  - `interaction_definitions`
- exported interaction definitions preserve the current per-kind payload fields, including the structured `shop_preview` `title + catalog[]` merchant contract frozen in `npc-shop-catalog-bootstrap.md`
- exported item templates preserve the owned item-template fields needed by merchant previews/buys and item bootstrap behavior, sorted deterministically by `vnum`
- when a bundle includes `item_templates`, every `shop_preview` catalog entry must reference one of those bundled templates; this keeps portable merchant bundles self-contained instead of relying on an implicit default item catalog
- when a bundle carries fixed item-shaped reward drops through `spawn_groups` or bundled custom `combat_profiles`, every `reward_drop_vnums` entry must also reference one bundled `item_templates` entry; reward-drop bundles without matching item templates are rejected before import
- the deterministic example bundle at `docs/examples/bootstrap-npc-service-bundle.json` is intentionally self-contained for merchant QA: its `item_templates` section carries every item referenced by the `shop_preview` catalog
- exported static actors are **portable authored content**, not runtime entities, so the bundle omits runtime-only `entity_id`
- import is full-replace for the authored bootstrap content currently loaded by `gamed`
- import validates that every referenced interaction definition exists before mutating runtime state
- import rejects non-canonical interaction refs before mutating runtime state, using the same `<namespace>:<name>` rule as the interaction-definition store and static-actor store
- import also rejects duplicate portable static-actor rows after canonical trimming, so a bundle cannot silently materialize the same authored actor twice
- import also rejects malformed per-kind definition payloads, including invalid `warp` destinations, invalid item templates, and invalid `shop_preview` catalogs
- import persists bundled `item_templates` to the file-backed item-template store and updates the live runtime template index before exposing the imported content
- import updates the live bootstrap runtime so the resulting static-actor, item-template, and interaction-definition content becomes the current authored state, not only the on-disk store contents
- `GET /local/content-bundle/summary` is a read-only operator view over the same canonical export path; it returns deterministic counts by content family, including static actors, interactable static actors, structured shop catalog entries, and authored warp destinations, per-kind referenced/unreferenced interaction counts, exact referenced/unreferenced interaction definition identities, compact per-definition previews for every authored interaction definition, exact portable static-actor identities (`name`, `map_index`, `x`, `y`, `race_num`, optional `combat_profile`, optional `interaction_kind`, optional `interaction_ref`) for both plain and interactable actors, exact interactable static-actor identities (`name`, `map_index`, `x`, `y`, `race_num`, `interaction_kind`, `interaction_ref`) with compact resolved previews, exact warp destination identities (`kind`, `ref`, optional `text`, `map_index`, `x`, `y`), exact spawn-group identities (`ref`, `name`, `map_index`, `x`, `y`, `race_num`, `combat_profile`, and reward descriptor), exact portable combat-profile snapshots, exact item-template identities (`vnum`, `name`, `stackable`, `max_count`, optional `shop_buy_price`), and per-map authored static-actor / interactable static-actor / spawn-group occupancy without returning the full bundle payload
- `POST /local/content-bundle/summary` is a loopback-only dry-run summary for an operator-supplied bundle; it uses the same strict decode and canonicalization rules as import/validate, returns only the compact deterministic summary, includes the same exact portable static actors, referenced/unreferenced interaction definition identities, compact per-definition previews, exact interactable static actors, warp destinations, exact spawn-group placement/template identities, portable combat-profile snapshots, and item-template identities, and does not call the live runtime exporter or mutate authored content

## Success definition

After this slice, the repository should be able to say:
- minimal `info`, `talk`, and `warp` definitions plus the structured `shop_preview` merchant catalog are authorable through loopback HTTP today
- visible interactables can still be inspected live with compact resolved previews for the currently previewable kinds and fail-closed markers otherwise
- bootstrap static actors, item templates, and their interaction definitions can be exported/imported as one deterministic authored-content bundle, with the structured merchant export/import shape already wired through that bundle surface
- local operators can inspect a compact deterministic content-bundle summary, including interaction-definition previews, exact warp destinations, spawn-group identities, and item-template identities, for either the live exported bundle or a candidate bundle before deciding whether to fetch or import the full bundle payload
