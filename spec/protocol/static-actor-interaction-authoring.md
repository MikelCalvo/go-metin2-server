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
- `GET /local/interactions/{kind}/{ref}`
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
- create/update bodies must be valid UTF-8 before JSON decoding; malformed raw bytes are rejected before runtime mutation callbacks can see lossy replacement-character strings
- exact lookup is read-only and loopback-only; it returns the authored definition JSON for one `kind + ref`, returns `404` when absent, and rejects blank or decoded slash-containing identities as path-ambiguous `400` requests
- updates are full-identity upserts, not partial nested edits
- update body `kind + ref` must match the path exactly
- delete fails closed while any bootstrap static actor still references that definition
- the backing catalog remains deterministic and file-backed under `internal/interactionstore`
- the file-backed interaction-definition loader rejects JSON `null` document roots, `null` `definitions` collections, unknown top-level JSON fields, and trailing JSON instead of accepting a lossy or partial object silently
- the static-actor store accepts interaction metadata only for currently owned definition kinds (`info`, `talk`, `warp`, `shop_preview`); future content kinds must be added to the interaction definition catalog before static actors can reference them durably

## Interaction-focused QA visibility

The first owned QA/debugging surface is:
- `GET /local/interaction-visibility`
- `GET /local/interaction-visibility/{name}`

The collection endpoint returns, per connected bootstrap player:
- the player snapshot
- the currently visible interactable static actors only
- each actor's `interaction_kind`
- each actor's `interaction_ref`
- each actor's current runtime `dead` flag when the target actor is at the bootstrap combat `0`-HP floor
- a compact resolved preview when the referenced definition currently resolves to a currently previewable kind (`info`, `talk`, `shop_preview`, `warp`); compact previews trim surrounding whitespace and cap by Unicode rune boundaries so operator JSON never receives truncated invalid UTF-8
- a fail-closed `resolution_failure` marker when it does not

The exact-name endpoint returns the same snapshot shape for one connected bootstrap player and fails closed without leaking actor details when the subject does not resolve.
It mirrors `/local/visibility/{name}` for interaction QA: URL-escaped names are accepted, blank or slash-containing path values are rejected, and missing/disconnected characters return `404`.

This is intended for live QA/debugging without packet captures. It now preserves both sides of the runtime state needed for interaction triage: the connected-player `dead` flag and the visible interactable actor `dead` flag.

## Deterministic authored-content bundle

The first owned bundle surface is:
- `GET /local/content-bundle`
- `POST /local/content-bundle`
- `GET /local/content-bundle/summary`
- `POST /local/content-bundle/summary`
- `GET /local/content-bundle/interactable-static-actors/{name}`
- `POST /local/content-bundle/validate`

Current rules:
- export returns one pretty-printed deterministic canonical JSON artifact; `/local/content-bundle`, `/local/content-bundle/validate`, and successful `/local/content-bundle` imports now share the same byte-stable encoder so checked-in example bundles can be pasted into the local endpoints without hidden formatting drift
- that artifact contains:
  - `static_actors`
  - `spawn_groups` when authored spawn-backed actors are present
  - `combat_profiles` when a referenced non-default combat profile must travel with spawn content
  - `item_templates` when the runtime has an authored item-template snapshot loaded
  - `interaction_definitions`
- exported interaction definitions preserve the current per-kind payload fields, including the structured `shop_preview` `title + catalog[]` merchant contract frozen in `npc-shop-catalog-bootstrap.md`
- exported item templates preserve the owned item-template fields needed by merchant previews/buys, item bootstrap behavior, authored spawn rewards, and bundled combat-profile reward defaults, including `buy_reject_message`, `sell_reject_message`, and `pickup_range` when authored, sorted deterministically by `vnum`
- when a bundle includes `item_templates`, every `shop_preview` catalog entry must reference one of those bundled templates; this keeps portable merchant bundles self-contained instead of relying on an implicit default item catalog
- when a bundle carries fixed item-shaped reward drops through `spawn_groups` or bundled custom `combat_profiles`, every `reward_drop_vnums` entry must also reference one bundled `item_templates` entry; reward-drop bundles without matching item templates are rejected before import, and export keeps templates referenced only after registered combat-profile reward defaults are expanded so exported bundles remain self-contained and immediately re-importable
- the deterministic example bundle at `docs/examples/bootstrap-npc-service-bundle.json` is intentionally self-contained for merchant QA: its `item_templates` section carries every item referenced by the `shop_preview` catalog
- exported static actors are **portable authored content**, not runtime entities, so the bundle omits runtime-only `entity_id`
- authored static actor and spawn-group names are trimmed, must remain non-empty after trimming, and reject embedded NUL bytes or invalid UTF-8 before persistence, import, or loopback static-actor mutation callbacks
- import is full-replace for the authored bootstrap content currently loaded by `gamed`
- import validates that every referenced interaction definition exists before mutating runtime state
- import rejects non-canonical interaction refs before mutating runtime state, using the same `<namespace>:<name>` rule as the interaction-definition store and static-actor store
- import also rejects static-actor interaction metadata whose `interaction_kind` is not one of the currently owned kinds (`info`, `talk`, `warp`, `shop_preview`), even when the referenced key is otherwise canonical; this keeps future seams such as quest/dialog kinds closed until their store/runtime contracts are actually frozen
- import also rejects duplicate portable static-actor rows after canonical trimming, so a bundle cannot silently materialize the same authored actor twice
- import also rejects malformed per-kind definition payloads, including invalid `warp` destinations, invalid item templates, invalid `shop_preview` catalogs, non-canonical portable combat-profile identities, and portable combat-profile snapshots that conflict with already-registered process-local profile defaults
- import persists bundled `item_templates` to the file-backed item-template store and updates the live runtime template index before exposing the imported content
- import updates the live bootstrap runtime so the resulting static-actor, item-template, and interaction-definition content becomes the current authored state, not only the on-disk store contents
- `GET /local/content-bundle/summary` is a read-only operator view over the same canonical export path; it returns deterministic counts by content family, including static actors, interactable static actors, structured shop catalog entries, and authored warp destinations, per-kind referenced/unreferenced interaction counts, exact referenced/unreferenced interaction definition identities, compact Unicode-safe per-definition previews for every authored interaction definition, exact portable static-actor identities (`name`, `map_index`, `x`, `y`, `race_num`, optional `combat_profile`, optional `interaction_kind`, optional `interaction_ref`) for both plain and interactable actors, exact interactable static-actor identities (`name`, `map_index`, `x`, `y`, `race_num`, `interaction_kind`, `interaction_ref`) with compact resolved previews, exact shop route identities (`actor_name`, source `map_index`/`x`/`y`, `ref`, `title`, `entry_count`) for interactable static actors that resolve to `shop_preview`, exact warp destination identities (`kind`, `ref`, optional `text`, `map_index`, `x`, `y`), exact actor-to-warp route identities (`actor_name`, source `map_index`/`x`/`y`, `ref`, optional `text`, target `map_index`/`x`/`y`) for interactable static actors that resolve to `warp`, exact spawn-group identities (`ref`, `name`, `map_index`, `x`, `y`, `race_num`, `combat_profile`, and reward descriptor), aggregate reward totals (`reward_experience_total`, `reward_gold_total`, `reward_drop_item_count`) plus grouped `reward_drops`, exact portable combat-profile snapshots, exact item-template identities (`vnum`, `name`, `stackable`, `max_count`, optional `shop_buy_price`, optional `shop_sell_price`, optional transfer/merchant guard flags (`anti_get`, `anti_drop`, `anti_give`, `anti_sell`, `anti_stack`), optional selected-character guard metadata (`anti_male`, `anti_female`, job/empire guards, `min_level`), optional equipment metadata (`equip_slot`, `appearance_vnum`, `irremovable`), optional rejection messages (`buy_reject_message`, `drop_reject_message`, `give_reject_message`, `pickup_reject_message`, `sell_reject_message`, `equip_reject_message`, `unequip_reject_message`), optional `pickup_range`), and per-map authored static-actor / interactable static-actor / spawn-group occupancy with per-map `info_actor_count`, `talk_actor_count`, `shop_preview_actor_count`, `shop_catalog_entry_count`, `warp_actor_count`, spawn reward totals, and drop item counts without returning the full bundle payload
- `POST /local/content-bundle/summary` is a loopback-only dry-run summary for an operator-supplied bundle; it uses the same strict decode, invalid UTF-8 rejection, JSON `null` root rejection, and canonicalization rules as import/validate, returns only the compact deterministic summary, includes the same exact portable static actors, referenced/unreferenced interaction definition identities, compact Unicode-safe per-definition previews, exact interactable static actors, shop route identities, warp destinations, actor-to-warp route identities, exact spawn-group placement/template identities, aggregate reward totals and grouped reward drops, per-map interaction counts (`info_actor_count`, `talk_actor_count`, `shop_preview_actor_count`, `shop_catalog_entry_count`, `warp_actor_count`), per-map spawn reward totals and drop item counts, portable combat-profile snapshots, and item-template identities, and does not call the live runtime exporter or mutate authored content
- `GET /local/content-bundle/maps/{map_index}` is a loopback-only exact-map summary reader over the live exported content-bundle summary; it returns one deterministic `maps[]` row for a non-zero authored `map_index`, returns `404` when no authored content summary row exists for that map, and exists for local QA/introspection rather than gameplay protocol or content mutation
- `GET /local/content-bundle/interactable-static-actors/{name}` is a loopback-only exact-name reader over the live exported content-bundle summary; it accepts a URL-escaped authored static-actor name, rejects blank or slash-containing names as path-ambiguous `400` requests, returns every deterministic `interactable_static_actors[]` row with that exact name so duplicated authored placements remain inspectable, returns `404` when no matching interactable actor exists, and exists for local QA/introspection rather than gameplay protocol or content mutation
- `GET /local/content-bundle/shop-catalogs/{kind}/{ref}` and `GET /local/content-bundle/warp-destinations/{kind}/{ref}` are loopback-only exact service-summary readers over the same live exported content-bundle summary; they accept only `shop_preview` catalog identities and `warp` destination identities respectively, return one deterministic `shop_catalogs[]` or `warp_destinations[]` row, return `404` when no matching authored service definition exists, and exist for local QA/introspection rather than gameplay protocol or content mutation
- `POST /local/content-bundle/import-preview` uses the same strict decode, invalid UTF-8 rejection, JSON `null` root rejection, and canonicalization rules as import/validate, then returns the same `current` and `candidate` summaries plus top-level count/amount deltas, a deterministic `deltas.static_actors` array for exact portable static-actor rows that are added or removed, a deterministic `deltas.interaction_kinds` array for only interaction kinds whose total/referenced/unreferenced counts change, a deterministic `deltas.interaction_definitions` array for authored definitions that are `added`, `removed`, or `changed`, a deterministic `deltas.combat_profiles` array for portable custom combat-profile snapshots that are `added`, `removed`, or `changed`, deterministic `deltas.shop_routes` and `deltas.warp_routes` arrays for compact NPC service routes that are `added`, `removed`, or `changed`, a deterministic `deltas.warp_destinations` array for authored warp destinations that are `added`, `removed`, or `changed`, a deterministic `deltas.reward_drops` array keyed by item `vnum` for grouped reward-drop sources that are `added`, `removed`, or `changed`, and a deterministic `deltas.maps` array for only map indexes whose tracked authored counts or reward totals change; each static-actor delta carries `change` plus the canonical `current` and/or `candidate` portable row (`name`, `map_index`, `x`, `y`, `race_num`, optional `combat_profile`, optional `interaction_kind`, optional `interaction_ref`), each interaction-definition delta carries `kind`, `ref`, `change`, and compact current/candidate previews when present, each combat-profile delta carries `profile`, `change`, and the canonical current/candidate profile snapshot including HP, damage/formula, presentation, respawn delay, and death-reward defaults when present, each shop/warp route delta is keyed by actor name, source `map_index`/`x`/`y`, and interaction `ref` and includes compact current/candidate route records when present, each interaction kind delta carries total, referenced, and unreferenced count deltas, each map delta groups count/amount changes plus per-map static-actor/spawn-group deltas, and invalid candidate bundles return `400` before the preview callback runs
- content-bundle import, validate, dry-run summary, import-preview, direct checked-in fixture decoding, and the project-owned bundle JSON decoder reject invalid UTF-8 bodies before Go's JSON decoder can replace malformed bytes, unknown top-level or per-collection fields, explicit JSON `null` values for top-level collection fields (`static_actors`, `spawn_groups`, `combat_profiles`, `item_templates`, `interaction_definitions`), and static actors that name unsupported future interaction kinds; operators should omit empty optional collections or send `[]`, and canonical export emits `[]` for the required contract collections instead of `null`

## Success definition

After this slice, the repository should be able to say:
- minimal `info`, `talk`, and `warp` definitions plus the structured `shop_preview` merchant catalog are authorable and exactly readable through loopback HTTP today
- visible interactables can still be inspected live with compact resolved previews for the currently previewable kinds and fail-closed markers otherwise
- bootstrap static actors, item templates, and their interaction definitions can be exported/imported as one deterministic authored-content bundle, with the structured merchant export/import shape already wired through that bundle surface
- local operators can inspect a compact deterministic content-bundle summary, including interaction-definition previews, exact warp destinations, spawn-group identities, and item-template identities, for either the live exported bundle or a candidate bundle before deciding whether to fetch or import the full bundle payload
