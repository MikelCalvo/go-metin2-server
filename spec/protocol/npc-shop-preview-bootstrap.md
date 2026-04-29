# NPC Shop Preview Bootstrap

This document freezes the browse-only merchant-style NPC contract for `go-metin2-server`.

Its authored payload is now paired with the structured catalog contract in:
- `npc-shop-catalog-bootstrap.md`

It sits on top of:
- `npc-service-interactions-bootstrap.md`
- `static-actor-interaction-request.md`
- `static-actor-interaction-authoring.md`

## Scope

This contract currently applies only to:
- a bootstrap static actor that is already visible to a connected `GAME` session
- the existing `INTERACT (0x0501)` request targeting that actor by visible `VID`
- a deterministic authored interaction definition with:
  - `kind = "shop_preview"`
  - stable `ref`
  - the structured merchant payload frozen in `npc-shop-catalog-bootstrap.md`
- a self-only browse-only response with no inventory or currency mutation

## Authored definition shape

`shop_preview` is no longer meant to stay conceptually tied to arbitrary merchant text.

The contract-bearing authored payload is now the structured merchant catalog frozen in:
- `npc-shop-catalog-bootstrap.md`

That follow-up doc owns:
- the top-level `title + catalog[]` shape
- stable per-entry `slot`
- stable per-entry `item_vnum`
- deterministic preview rendering from template-backed merchant entries

Transition note:
- the current implementation still serves the old text-backed `shop_preview` payload until the next slice widens store/bundle/runtime wiring
- this slice freezes the long-term preview contract first so the next RED/GREEN implementation can stay small

## Runtime behavior

When a player interacts with a visible static actor whose metadata resolves to a valid `shop_preview` definition:
- the runtime keeps the existing visibility and distance checks
- the runtime keeps the existing per-session per-target `1s` cooldown
- the player receives exactly one self-only `GC_CHAT` delivery
- that delivery uses:
  - `CHAT_TYPE_INFO`
  - `VID = 0`
  - `Empire = 0`
  - `Message = deterministic structured merchant preview render`
- no peer-visible frames are emitted
- no transfer, inventory mutation, purchase, sell-back, or persistent merchant state is created

The exact preview-string shape is now frozen in `npc-shop-catalog-bootstrap.md`.

## Operator authoring and QA visibility

Current loopback authoring surface:
- `GET /local/interactions`
- `POST /local/interactions`
- `PATCH /local/interactions/{kind}/{ref}`
- `PUT /local/interactions/{kind}/{ref}`
- `DELETE /local/interactions/{kind}/{ref}`

Current loopback QA/debugging surface:
- `GET /local/interaction-visibility`

For `shop_preview`, interaction-visibility returns the actor together with the compact resolved preview string instead of an unsupported-kind marker.

At the time this contract is frozen, the loopback authoring and bundle surfaces still expose the existing text-backed payload; the next slice is expected to adopt the structured merchant catalog shape there.

## Explicit non-goals

This slice does **not** yet freeze:
- real shop buy/sell packets
- inventory or gold checks
- item grant/removal
- price tables or stock depletion
- merchant dialog windows
- option selection state
- quest acceptance or script execution

## Success definition

After this slice, the repository should be able to say:
- `shop_preview` remains a valid browse-only authored interaction kind
- the player-facing browse response is still frozen as self-only and deterministic
- the structured merchant payload that will replace raw merchant text is now frozen in project-owned docs
- visible static actors still resolve `shop_preview` through the existing `INTERACT` ingress
- the project still does not pretend that a real shop transaction system exists
