# NPC Shop Preview Bootstrap

This document freezes the first browse-only merchant-style NPC contract for `go-metin2-server`.

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
  - non-blank `text`
- a self-only browse-only response with no inventory or currency mutation

## Authored definition shape

`shop_preview` currently uses the minimal text-backed shape:

```json
{
  "kind": "shop_preview",
  "ref": "npc:merchant",
  "text": "Browse wares."
}
```

Current validation rules:
- `kind` must equal `shop_preview`
- `ref` must be non-blank
- `text` must be non-blank after trimming
- `map_index`, `x`, and `y` must stay zero for this kind
- duplicate `(kind, ref)` definitions are rejected

## Runtime behavior

When a player interacts with a visible static actor whose metadata resolves to a valid `shop_preview` definition:
- the runtime keeps the existing visibility and distance checks
- the runtime keeps the existing per-session per-target `1s` cooldown
- the player receives exactly one self-only `GC_CHAT` delivery
- that delivery uses:
  - `CHAT_TYPE_INFO`
  - `VID = 0`
  - `Empire = 0`
  - `Message = definition.Text`
- no peer-visible frames are emitted
- no transfer, inventory mutation, purchase, sell-back, or persistent merchant state is created

## Operator authoring and QA visibility

Current loopback authoring surface:
- `GET /local/interactions`
- `POST /local/interactions`
- `PATCH /local/interactions/{kind}/{ref}`
- `PUT /local/interactions/{kind}/{ref}`
- `DELETE /local/interactions/{kind}/{ref}`

Current loopback QA/debugging surface:
- `GET /local/interaction-visibility`

For `shop_preview`, interaction-visibility now returns the actor together with the compact resolved preview text instead of an unsupported-kind marker.

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
- `shop_preview` is a valid authored interaction kind in `internal/interactionstore`
- loopback interaction-definition CRUD can create and update `shop_preview` definitions
- visible static actors can resolve `shop_preview` through the existing `INTERACT` ingress
- the player receives a deterministic self-only preview message
- the project still does not pretend that a real shop transaction system exists
