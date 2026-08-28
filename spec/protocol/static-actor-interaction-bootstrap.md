# Static Actor Interaction Bootstrap

This document freezes the first interaction-ready metadata seam for bootstrap static actors.

It sits on top of:
- `non-player-entity-bootstrap.md`
- `visible-world-bootstrap.md`
- `character-update-bootstrap.md`
- `npc-service-interactions-bootstrap.md`

The goal is narrow:
- let bootstrap static actors carry minimal interaction metadata in runtime state
- expose and persist that metadata through the existing local operator surfaces
- freeze the owned interaction families carried by that metadata without claiming richer NPC gameplay is already complete

## Scope

This contract currently applies only to:
- bootstrap static actors owned by `internal/worldruntime`
- loopback-only operator create/update/read surfaces on `gamed`
- file-backed static-actor snapshots restored on boot
- runtime introspection snapshots that already surface static actors

It does **not** yet claim live client-visible interaction packet choreography.

## Metadata fields

A bootstrap static actor may now carry two optional fields:
- `interaction_kind`
- `interaction_ref`

These fields are intentionally tiny:
- `interaction_kind` identifies the interaction family
- `interaction_ref` is a stable authoring lookup key owned by later slices

Current `interaction_ref` values are intentionally path-safe and canonical:
- exactly one namespace separator: `<namespace>:<name>`
- each segment must be lower-snake-ish: starts with `[a-z]`, then `[a-z0-9_]*`
- `/`, whitespace, dots, hyphens, uppercase letters, blank segments, and extra `:` separators are rejected

This keeps interaction identities safe for the current JSON stores and `/local/interactions/{kind}/{ref}` operator routes before broader quest/content naming exists.

## Validity rule

The first owned validation rule is:
- both fields empty = no interaction
- both fields non-empty = interaction metadata present
- exactly one field present = invalid
- when present, `interaction_kind` must be one of the currently owned interaction-definition kinds: `info`, `talk`, `quest_flag`, `warp`, or `shop_preview`
- when present, `interaction_ref` must satisfy the canonical `<namespace>:<name>` rule above
- low-level runtime entities in `internal/worldruntime` must already carry canonical, unpadded metadata; whitespace-padded `interaction_kind` or `interaction_ref` values are rejected fail-closed at that boundary rather than normalized into a different live actor

This rule applies consistently in:
- runtime registration/update validation after the local operator layer has parsed/canonicalized request values
- local operator request decoding
- file-backed static-actor snapshot validation
- interaction-definition store validation
- content-bundle canonicalization/import validation

## Current owned behavior

At this stage, the repository owns metadata plus the first narrow interaction-ready behavior:
- static actors can preserve `interaction_kind` / `interaction_ref` in runtime state
- runtime static-actor registration/update now rejects unsupported interaction kinds before they can reach visibility, interaction, or persistence paths
- runtime static-actor registration/update and file-backed restore now also reject static actors whose `entity_id` cannot fit the current `uint32` client-visible static-actor `VID`, because that same `VID` is the interaction target identity
- `/local/static-actors` create/update responses can surface that metadata
- runtime snapshot/introspection surfaces can report that metadata
- file-backed static-actor snapshots can persist and restore that metadata across boot
- a deterministic file-backed interaction-definition store can now persist minimal `info` / `talk` definitions by stable `kind + ref`
- `gamed` now loads that interaction-definition catalog at boot when present
- loopback-only `GET`/`POST /local/interactions` plus `PATCH`/`PUT`/`DELETE /local/interactions/{kind}/{ref}` now author that catalog without hand-editing the backing JSON file
- delete now fails closed while a bootstrap static actor still references the targeted definition
- persisted static actors with interaction refs now fail closed at boot if those refs do not resolve in the loaded interaction-definition catalog
- runtime static-actor create/update with interaction metadata now also fail closed when the referenced definition does not exist in the loaded interaction-definition catalog
- visible static actors whose metadata resolves to `interaction_kind = "info"` now answer with a self-only informational chat-backed delivery
- visible static actors whose metadata resolves to `interaction_kind = "talk"` now answer with a self-only speaker-prefixed multi-line chat-backed delivery
- visible static actors whose metadata resolves to `interaction_kind = "shop_preview"` now carry the structured merchant catalog authoring seam that powers the current bootstrap merchant window open / buy / close flow
- authored interaction definition text/title fields reject embedded NUL bytes at every owned authoring/load boundary before they can reach client-visible chat, merchant titles, or compact operator previews. The current fail-closed fields are `info.text`, `talk.text`, optional `warp.text`, and `shop_preview.title`.

## Owned interaction families

The first owned interaction families stay intentionally narrow:
- self-only `info` / `talk`
- quest-state `quest_flag`
- service-style `warp`
- merchant-style `shop_preview`

The currently implemented bootstrap interaction families remain conservative:
- the actor must already be visible to the player
- the actor must also be within the current bootstrap interaction range on the same effective map; a nearby coordinate on a different map is rejected as out of range
- the actor must not be in the runtime-owned dead interval; dead interactable actors remain visible/introspectable but resolve `INTERACT` as the fail-closed `target_dead` path until respawn
- the runtime resolves `interaction_kind` + `interaction_ref`
- `info` and `talk` remain self-facing chat-backed responses
- `quest_flag` runs one compare-and-set transition against the selected character's persisted quest-state flags, including authored clear transitions with `quest_to = 0`, returns one self-facing info-chat acknowledgement when the transition applies, and now returns a deterministic self-facing requirement-mismatch info chat when the persisted current value does not match the authored `quest_from` value
- `warp` reuses the existing self-session transfer / rebootstrap path instead of inventing a separate dialog or warp packet family
- `shop_preview` reuses the structured merchant catalog plus the current bootstrap merchant window open / buy / close contract instead of inventing a second merchant-definition seam
- no broader quest scripting, barter, reward, or combat side effects are required; the standalone quest-state primitive and loopback transition harness remain documented in `quest-state-bootstrap.md`

The current out-of-range failure is intentionally bootstrap-scoped. It is owned only as a fail-closed guard around already-visible static actors, not as the final NPC interaction distance policy.

Current owned meanings:
- `interaction_kind = "info"`
  - return a simple self-facing informational response carrying the authored text
- `interaction_kind = "talk"`
  - return a simple self-facing talk/dialog-style response carrying a deterministic speaker-prefixed multi-line payload
- `interaction_kind = "quest_flag"`
  - resolve a content-authored quest-state trigger using the existing `INTERACT` ingress, apply exactly one selected-character compare-and-set flag transition, including clear-on-zero, return one self-only `CHAT_TYPE_INFO` acknowledgement on success, and return self-only `Quest requirements are not met.` info chat on `current_value_mismatch` without mutating the quest-state snapshot
- `interaction_kind = "warp"`
  - resolve a teleporter-style service interaction using the existing `INTERACT` ingress and the existing transfer / rebootstrap runtime rather than a dedicated dialog or warp packet family
- `interaction_kind = "shop_preview"`
  - resolve a merchant-style interaction using the same structured catalog authoring seam now frozen by the merchant preview / open-close / transaction docs

## Explicit non-goals

This slice does not yet freeze:
- click packet handling
- NPC dialog trees
- sell-back, stock depletion, or richer merchant-window choreography beyond the current bootstrap open / buy / close seam
- quest mission UI, branching quest scripts, rewards, or multi-step dialog runtime
- actor targeting/combat semantics
- animation/emote/state-machine behavior
- persistent merchant stock state

## Success definition

After this slice, the repository should be able to say:
- bootstrap static actors can carry `interaction_kind` / `interaction_ref`
- that metadata survives create/update/list/persist/boot paths
- invalid partial metadata is rejected consistently
- a deterministic file-backed interaction-definition store now exists for minimal `info` / `talk` / `quest_flag` / `shop_preview` content plus the first `warp` destination payload keyed by `kind + ref`
- interaction definition validation rejects embedded NUL bytes in the owned client-visible text/title fields, so local operator writes, content-bundle validation/import, and runtime startup cannot persist or load truncated authored strings
- `gamed` now loads that catalog before boot-restoring persisted static actors and before accepting new interaction metadata on static-actor create/update paths
- loopback-only CRUD endpoints now author that catalog while preserving stable `kind + ref` identity on update and rejecting deletes for referenced definitions
- static actors that point at missing interaction definitions are now rejected fail closed at boot and on runtime create/update
- content-bundle canonicalization likewise rejects dangling interaction refs before import; the checked-in negative fixture `docs/examples/bootstrap-invalid-dangling-interaction-ref-bundle.json` is the preferred `/local/content-bundle/validate` dry-run for that reject without improvising JSON
- content-bundle canonicalization likewise rejects unsupported future static-actor interaction kinds such as unfrozen `quest` metadata before import; the checked-in negative fixture `docs/examples/bootstrap-invalid-unsupported-interaction-kind-bundle.json` is the preferred `/local/content-bundle/validate` dry-run for that reject without improvising JSON
- content-bundle canonicalization likewise rejects duplicate portable static-actor rows after canonical trimming before import; the checked-in negative fixture `docs/examples/bootstrap-invalid-duplicate-static-actor-bundle.json` is the preferred `/local/content-bundle/validate` dry-run for that reject without improvising JSON
- visible actors can now answer the interacting player with self-only `info` / `talk`, can apply one persisted selected-character `quest_flag`, can reuse the same metadata seam for the current merchant-window `shop_preview` flow, and can still power QA/debug preview rendering without redesigning the actor model first
- the same metadata seam now also powers the current service-style NPC `warp` interaction family
