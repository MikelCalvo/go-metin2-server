# NPC Shop Preview Bootstrap

This document freezes the structured merchant preview / identity contract for `go-metin2-server`.

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
- the structured merchant identity and preview surface that now also feeds the current bootstrap merchant-window open / buy / close flow

## Authored definition shape

`shop_preview` is no longer meant to stay conceptually tied to arbitrary merchant text.

The contract-bearing authored payload is now the structured merchant catalog frozen in:
- `npc-shop-catalog-bootstrap.md`

That follow-up doc owns:
- the top-level `title + catalog[]` shape
- stable per-entry `slot`
- stable per-entry `item_vnum`
- deterministic preview rendering from template-backed merchant entries

The structured merchant catalog is now live across interaction CRUD, bundle import/export, interaction-visibility, and merchant-window runtime wiring.

## Runtime behavior

When a player interacts with a visible static actor whose metadata resolves to a valid `shop_preview` definition:
- the runtime keeps the existing visibility and distance checks
- the runtime keeps the existing per-session per-target `1s` cooldown
- the live session currently receives exactly one self-only `GC::SHOP START` merchant-window open response built from that structured catalog
- the same definition still owns the deterministic compact preview render frozen in `npc-shop-catalog-bootstrap.md` for QA/debug and lower-level resolution surfaces
- no peer-visible frames are emitted
- no transfer is triggered by `shop_preview`
- later merchant state mutation still remains limited to the separately frozen bootstrap buy-only merchant path

The exact preview-string shape is now frozen in `npc-shop-catalog-bootstrap.md`, and the current merchant-window open / buy / close behavior is frozen separately in `npc-shop-open-close-bootstrap.md` and `npc-shop-transaction-bootstrap.md`.

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

The loopback authoring and bundle surfaces now also expose that same structured merchant catalog payload directly.

## Explicit non-goals

This slice does **not** yet freeze:
- sell-back
- merchant stock depletion or refresh
- merchant dialog windows
- option selection state
- quest acceptance or script execution

## Success definition

After this slice, the repository should be able to say:
- `shop_preview` remains a valid structured merchant authored interaction kind
- the player-facing merchant identity and compact preview render are still frozen as deterministic outputs of the structured catalog
- the structured merchant payload has replaced raw merchant text across the current authoring, bundle, and merchant-window runtime surfaces
- visible static actors still resolve `shop_preview` through the existing `INTERACT` ingress
- the project now also owns the first bootstrap merchant-window open / buy / close flow on top of that same ingress, while still avoiding broader sell/stock/window semantics
