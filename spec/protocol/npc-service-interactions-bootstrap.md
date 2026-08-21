# NPC Service Interactions Bootstrap

This document freezes the next owned NPC gameplay contract for `go-metin2-server`.

It sits on top of:
- `static-actor-interaction-bootstrap.md`
- `static-actor-interaction-request.md`
- `transfer-rebootstrap-burst.md`
- `non-player-entity-bootstrap.md`

Those documents already freeze:
- bootstrap static actors as the first non-player runtime seam
- `INTERACT (0x0501)` as the first client-originated interaction request
- self-only `info` / `talk` authored responses
- the current gameplay-triggered transfer / rebootstrap contract

What this document adds is the next narrower question:

**What is the first honest NPC gameplay vertical the project can own now, before branching quests, dialog-window UI, or final shop choreography exist?**

## Scope

This contract applies only to:
- bootstrap static actors already visible to a connected `GAME` session
- the existing `INTERACT (0x0501)` request targeting that actor by visible `VID`
- service-style NPC actions that can complete in one request with no branching dialog state
- self-facing or transfer-triggered outcomes that reuse already-owned packet/runtime contracts
- deterministic authored interaction definitions loaded and validated by `gamed`

This document now freezes the contract and also records the two landed service-style verticals:
- `warp` is now implemented on top of the existing `INTERACT` ingress and the existing transfer / rebootstrap runtime
- `shop_preview` now opens the bootstrap merchant window and buy-only merchant flow on top of the same ingress and the same structured merchant catalog seam
- authored content bundles may carry process-local combat profile snapshots only when the same bundle references them from a static actor or spawn group

## Why service-style NPCs first

The current repository already owns enough runtime to support a narrow but real NPC gameplay loop:
- visible static actors exist in the live world
- players can already target those actors through `INTERACT`
- authored interaction definitions already exist and are persisted deterministically
- gameplay-triggered transfer / rebootstrap already exists

At the same time, several larger systems are still intentionally missing:
- richer merchant-window acknowledgement choreography
- branching quest scripts, rewards, and client quest UI
- broader client-owned dialog-window or option-selection contracts beyond the current merchant window family

The first standalone quest-state primitive, loopback transition harness, and a narrow static-actor `quest_flag` trigger are now documented in `quest-state-bootstrap.md`. They are deliberately separate from this service-style NPC execution path: `warp` and `shop_preview` continue to focus on one-request service outcomes rather than branching quest/dialog state.

Because of those constraints, the next honest NPC gameplay vertical here remains **service-style interaction**, not branching dialogs, quest trees, or broader merchant/dialog semantics first.

## First owned service-style families

The next owned NPC gameplay families are:

### 1. `warp`
A visible static actor can act as a teleporter-style NPC.

Frozen target behavior:
- the player sends the existing `INTERACT (0x0501)` request
- the runtime resolves a deterministic authored `warp` definition behind that actor
- that authored store-level definition is now expected to carry `map_index`, `x`, `y`, and optional informational text
- the same definition may optionally carry a selected-character quest gate (`quest_ref` + `quest_flag` + optional `quest_from`) that must match before transfer; gated warps never mutate quest state (`quest_to` stays `0`)
- the runtime may deliver one small self-facing informational message if the authored definition carries text
- the runtime then reuses the existing gameplay transfer / self-session rebootstrap contract
- no dialog state, option selection, or persistent conversation session is created

Current owned warp failure semantics:
- if the resolved warp definition is malformed inside live runtime state, the player receives one self-only `CHAT_TYPE_INFO` message: `Warp destination is invalid.`
- if the runtime cannot apply the transfer after resolution, the player receives one self-only `CHAT_TYPE_INFO` message: `Warp unavailable right now.`
- if an optional quest gate is present and the selected character's current flag value does not match `quest_from`, the player receives one self-only `CHAT_TYPE_INFO` message: `Quest requirements are not met.` and no transfer occurs

Current owned warp operator-summary semantics:
- `GET /local/content-bundle/summary` and dry-run `POST /local/content-bundle/summary` now report deterministic `warp_destinations` entries for every authored `warp` definition
- each destination summary entry carries `kind`, `ref`, optional `text`, `map_index`, `x`, `y`, and any authored quest-gate fields
- `GET /local/content-bundle/warp-destinations/{kind}/{ref}` now returns one exact `warp` destination row for loopback local QA without fetching the full bundle summary or triggering a transfer
- the same summary now reports deterministic `warp_routes` entries for every interactable static actor that resolves to a `warp` definition
- each route summary entry carries `actor_name`, source `map_index`/`x`/`y`, `ref`, optional `text`, target `map_index`/`x`/`y`, and any authored quest-gate fields
- `GET /local/content-bundle/warp-routes/{actor_name}` now returns every exact route row for one authored teleporter actor name, so duplicated placements remain inspectable without fetching the full bundle summary or triggering a transfer
- `GET /local/content-bundle/maps/{map_index}/warp-routes` now returns every route row whose source actor is on one authored map, so local QA can audit all teleporters on a map without filtering the full summary or knowing actor names first
- `GET /local/content-bundle/maps/{map_index}/static-actors` and `/interactable-static-actors` now return map-local authored static-actor rows and clickable/service-preview rows, so operators can audit one map's NPC content before narrowing to route-specific projections
- the same summary `maps[]` audit reports `warp_actor_count` for each authored map, counting visible static actors on that map that resolve to a `warp` definition
- `POST /local/content-bundle/import-preview` compares a candidate replacement bundle against the live exported bundle and returns no-mutation `current` / `candidate` summaries plus count/amount `deltas`, including exact portable static-actor `added` / `removed` rows, exact interactable static-actor `added` / `removed` / `changed` rows with compact resolved previews, per-interaction-kind reference deltas, per-definition `added` / `removed` / `changed` deltas with compact current/candidate previews, exact `warp_destinations` `added` / `removed` / `changed` deltas keyed by interaction `kind` + `ref`, exact `warp_routes` `added` / `removed` / `changed` deltas keyed by actor/source/ref route, exact `spawn_groups` `added` / `removed` / `changed` deltas keyed by authored spawn-group `ref`, exact portable `combat_profiles` `added` / `removed` / `changed` deltas keyed by profile name, reward EXP/gold totals when authored spawn rewards would change, and grouped `reward_drops` `added` / `removed` / `changed` deltas keyed by item vnum
- `POST /local/content-bundle/import-preview/interaction-kinds/{kind}` returns one exact interaction-kind delta for a candidate replacement bundle, so local QA can inspect whether a family such as `info`, `talk`, `quest_flag`, `shop_preview`, or `warp` changes total, referenced, or unreferenced counts without fetching and filtering the broad preview
- `POST /local/content-bundle/import-preview/interactable-static-actors/{name}` returns exact clickable actor preview deltas for one authored actor name, so local QA can inspect whether a visible NPC's resolved interaction preview would be added, removed, or changed without filtering the broad preview
- `POST /local/content-bundle/import-preview/warp-destinations/{kind}/{ref}` returns one exact authored warp-destination delta for a candidate replacement bundle, so local QA can inspect one teleporter destination impact without fetching and filtering the broad preview
- `POST /local/content-bundle/import-preview/warp-routes/{actor_name}` returns every exact warp-route delta for one authored teleporter actor name, so local QA can inspect one teleporter placement impact without fetching and filtering the broad preview
- `POST /local/content-bundle/import-preview/maps/{map_index}` returns one exact authored map delta for a candidate replacement bundle, including map-local shop-route and warp-route deltas when merchant/teleporter placement details change, so local QA can inspect one map's static-actor, spawn, reward, merchant, and teleporter impact without fetching and filtering the broad preview
- `POST /local/content-bundle/import-preview/reward-drops/{item_vnum}` returns one exact grouped reward-drop delta for a candidate replacement bundle, so local QA can inspect one reward item impact without fetching and filtering the broad preview
- this makes both teleporter destinations and exact actor-to-destination routes inspectable without fetching the full authored bundle or applying a candidate import, and it makes replacement impact across exact authored maps, static actors, authored definitions, warp destinations/routes, combat-profile snapshots, spawn-group rows, and interaction families inspectable before committing a candidate import

Current owned interaction cooldown semantics:
- a fixed `1s` runtime cooldown now applies per live session and per target static-actor `VID`
- the cooldown currently applies across all owned interaction kinds, including `info`, `talk`, `quest_flag`, `shop_preview`, `warp`, and `open_safebox`
- repeated `INTERACT` requests for the same target while that cooldown is active are consumed as a deliberate no-op with no outgoing frames
- different players do not share a cooldown bucket with each other, and a fresh reconnect starts with a fresh cooldown state

This is now the first implemented **real NPC gameplay loop** because it reuses already-owned transfer behavior instead of requiring speculative new subsystems.

### 2. `shop_preview`
A visible static actor can act as a merchant-style NPC anchored to the structured merchant catalog seam.

Frozen target behavior:
- the player sends the existing `INTERACT (0x0501)` request
- the runtime resolves a deterministic authored `shop_preview` definition behind that actor
- the live session receives the current bootstrap merchant-window open response (`GC::SHOP START`) built from that structured catalog
- later `SHOP BUY` / `SHOP END` requests reuse the same active merchant context and the same authored catalog identity frozen by the merchant docs
- the same catalog still owns a deterministic compact preview render for QA/debug and lower-level resolution surfaces

Current owned shop operator-summary semantics:
- `GET /local/content-bundle/summary` and dry-run `POST /local/content-bundle/summary` now report deterministic `shop_catalogs` entries for every authored `shop_preview` definition
- template-backed shop/item/reward summary rows include the currently owned transfer/merchant guard flags (`anti_get`, `anti_drop`, `anti_give`, `anti_sell`, `anti_stack`) and selected-character guard metadata (`anti_male`, `anti_female`, `anti_warrior`, `anti_assassin`, `anti_sura`, `anti_shaman`, `anti_empire_a`, `anti_empire_b`, `anti_empire_c`, `min_level`) beside the owned buy/drop/give/pickup/sell rejection messages so deny metadata is inspectable without opening the full bundle
- `GET /local/content-bundle/reward-drops/{item_vnum}` returns one exact aggregate reward-drop row with `source_count` and resolved item-template metadata, so local QA can inspect one authored reward item without fetching the full bundle summary or expanding every spawn group
- the same summary now reports deterministic `shop_routes` entries for every interactable static actor that resolves to a `shop_preview` definition
- each route summary entry carries `actor_name`, source `map_index`/`x`/`y`, `ref`, merchant `title`, and catalog `entry_count`
- `GET /local/content-bundle/shop-routes/{actor_name}` now returns every exact route row for one authored merchant actor name, so duplicated placements remain inspectable without fetching the full bundle summary or opening the merchant in-game
- `GET /local/content-bundle/maps/{map_index}/shop-routes` now returns every route row whose source actor is on one authored map, so local QA can audit all merchants on a map without filtering the full summary or knowing actor names first
- `GET /local/content-bundle/maps/{map_index}/static-actors` and `/interactable-static-actors` now return the broader authored actor rows and resolved clickable previews for one map, so merchant QA can verify surrounding NPC placement before narrowing to `shop_preview` routes
- per-map `maps[]` entries include `shop_preview_actor_count` and `shop_catalog_entry_count`
- `POST /local/content-bundle/import-preview` exposes the same current/candidate summary comparison and count deltas for merchant catalog entries and shop routes before a candidate bundle is applied
- `POST /local/content-bundle/import-preview/shop-catalogs/{kind}/{ref}` returns one exact structured shop-catalog delta for a candidate replacement bundle, so local QA can inspect one merchant catalog impact without fetching and filtering the broad preview
- `POST /local/content-bundle/import-preview/shop-routes/{actor_name}` returns every exact shop-route delta for one authored merchant actor name, so local QA can inspect one merchant placement impact without fetching and filtering the broad preview
- this makes exact actor-to-catalog merchant placement inspectable without fetching the full authored bundle or applying a candidate import

Current owned self-only interaction operator-summary semantics:
- per-map `maps[]` entries now also include `info_actor_count`, `talk_actor_count`, and `quest_flag_actor_count`
- these counts audit authored `info` / `talk` / `quest_flag` static actors separately from service-style `shop_preview` and `warp` actors without requiring operators to expand the full `interactable_static_actors` array
- `GET /local/content-bundle/interaction-definitions/{kind}/{ref}` now returns one compact authored definition preview row with `kind`, `ref`, `preview`, and `referenced`, so operators can inspect a single `info` / `talk` / `quest_flag` / service definition summary without fetching the full bundle summary or full bundle payload
- `GET /local/content-bundle/item-templates/{vnum}` now returns one exact summarized item-template row for local QA, including the guard/rejection metadata already exposed in content-bundle summaries, without fetching the full bundle summary or opening a merchant in-game

This remains intentionally narrow even now that the first buy-only merchant path exists: sell-back, stock depletion, and richer merchant-window choreography still remain separate later work.

### 3. `open_safebox`
A visible static actor can act as a warehouse-style NPC that opens the already-owned bootstrap safebox presentation.

Frozen target behavior:
- the player sends the existing `INTERACT (0x0501)` request
- the runtime resolves a deterministic authored `open_safebox` definition behind that actor
- that authored store-level definition may carry optional informational `text`, optional bootstrap page `size` in `1..3` (omitted / `0` defaults to `1`), and the same optional non-mutating selected-character quest gate used by `warp` / `shop_preview`
- when the interaction applies, the runtime may deliver one self-facing informational chat message if authored text is present
- the runtime then reuses the existing `/open_safebox` presentation seam: set the same-socket busy flag, emit `GC::SAFEBOX_SIZE`, and re-emit any remembered same-session `SAFEBOX_SET` rows
- no password load, durable safebox persistence, mall, money, or `SAFEBOX_ITEM_MOVE` is introduced by this kind

Current owned `open_safebox` failure semantics:
- if an optional quest gate is present and the selected character's current flag value does not match `quest_from`, the player receives one self-only `CHAT_TYPE_INFO` message: `Quest requirements are not met.` and no safebox presentation opens
- invalid authored sizes and foreign fields fail closed at store / content-bundle validation before runtime mutation

This remains intentionally narrow: check-in / check-out already exist once the presentation is open, but durable storage and password load stay deferred.

## Content-bundle combat profile guardrail

Content bundles can carry authored combat profile snapshots so imported spawn groups can materialize process-local practice-mob variants before the static actors are restored.
That import path is intentionally fail-closed:

- every non-built-in `combat_profiles[].profile` entry must be referenced by at least one authored static actor or spawn group in the same bundle
- duplicate profile snapshots are rejected
- snapshots that conflict with an already-registered local profile are rejected
- any item-shaped death reward in either `spawn_groups[].reward_drop_vnums` or `combat_profiles[].death_reward.drop_vnums` must be backed by a matching bundled `item_templates` entry; reward-drop bundles cannot depend on an implicit ambient item catalog
- duplicate reward drop vnums in either spawn groups or bundled combat-profile defaults are rejected before canonical import; malformed profile-default drop lists must not be silently deduplicated into valid content
- failed imports must not register the profile, materialize actors, persist content, or leak queued visibility frames

This keeps `/local/content-bundle` from becoming a side-channel for unreferenced process-local combat profile mutation.

## Routing rule

These next service-style NPC interactions continue to use the current ingress contract:
- request packet: `INTERACT`
- direction: client -> server
- header: `0x0501`
- phase: `GAME`
- target identity: visible static-actor `VID`

No new client-originated packet family is frozen in this stage.

## Response rule

The current owned response families stay intentionally conservative:
- `info` and `talk` remain self-only chat-backed authored responses; they may optionally carry the same non-mutating selected-character quest gate as `warp` / `shop_preview` / `open_safebox`, returning `Quest requirements are not met.` instead of the authored text when the gate mismatches
- `warp` now reuses the already-owned transfer / rebootstrap contract rather than inventing a separate NPC warp packet; if authored `text` is present, the interacting player first receives one self-only informational chat delivery and then the transfer rebootstrap frames
- `shop_preview` now reuses the current bootstrap merchant window open / buy / close contract, while preserving the deterministic preview render for QA/debug and lower-level resolution surfaces
- `open_safebox` now reuses the current bootstrap safebox open presentation (`SAFEBOX_SIZE` + remembered `SAFEBOX_SET` rows) rather than inventing a separate warehouse packet family; if authored `text` is present, the interacting player first receives one self-only informational chat delivery and then the safebox open frames

## Ordered implementation status

The originally planned sequence is now landed in this order:
1. interaction failure reasons became player-visible instead of silently fail-closed
2. an explicit interaction distance gate landed, separate from mere visibility ownership
3. authored `warp` definitions were added and now execute through the existing transfer path
4. the same ingress and authoring seam then widened into the first bootstrap merchant window open / buy / close flow

That order kept the first real NPC gameplay payoff small and honest before merchant-window work widened further.

## Explicit non-goals

This stage still does **not** freeze:
- client dialog-window packets outside the currently owned merchant window family
- branching NPC dialogs or option trees
- quest acceptance, progression, rewards, or script execution
- sell-back or richer merchant stock/update semantics
- combat, buffs, healing, aggro, or AI behavior
- persistent NPC conversation state
- click-to-move choreography beyond the current direct `INTERACT` request

## Success definition

After the currently landed and later follow-up slices, the repository should be able to say:
- bootstrap static actors already support self-only `info` / `talk` plus merchant-style `shop_preview`
- the current owned service-style NPC gameplay families are `warp`, merchant `shop_preview`, and warehouse `open_safebox`
- `warp` is the first real NPC gameplay action and already reuses the existing transfer / rebootstrap runtime through `INTERACT`
- `shop_preview` now already resolves through `INTERACT` into the bootstrap merchant window open / buy / close flow built on the same structured catalog seam
- `open_safebox` now already resolves through `INTERACT` into the bootstrap safebox open presentation used by same-session check-in / check-out
- the project still avoids speculative dialog-window, quest-script, sell-back, and durable storage semantics until the underlying systems exist
