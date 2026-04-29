# NPC Shop Catalog Bootstrap

This document freezes the first structured merchant-catalog contract that sits behind `shop_preview` in `go-metin2-server`.

It sits on top of:
- `npc-shop-preview-bootstrap.md`
- `inventory-equipment-bootstrap.md`
- `static-actor-interaction-authoring.md`

The goal is intentionally narrow:
- stop treating merchant previews as arbitrary authored text
- give `shop_preview` a deterministic catalog payload that refers to owned item templates by stable ID
- keep the current player-facing behavior browse-only and self-only
- make the next implementation slice small enough to wire authored-content stores, preview rendering, and QA surfaces without reopening the contract

## Scope

This contract currently applies only to:
- a bootstrap static actor that is already visible to a connected `GAME` session
- the existing `INTERACT (0x0501)` request targeting that actor by visible `VID`
- one authored `shop_preview` definition identified by stable `kind + ref`
- a structured merchant catalog whose entries refer to owned item templates through stable `vnum`
- a deterministic self-only preview render with no inventory, gold, or persistence mutation

This slice does **not** yet implement the catalog in runtime code.
It freezes the contract first so the next slice can add RED tests and wiring safely.

## Structured authored definition shape

The first structured `shop_preview` shape is:

```json
{
  "kind": "shop_preview",
  "ref": "npc:village_merchant",
  "title": "Village Merchant",
  "catalog": [
    {
      "slot": 0,
      "item_vnum": 27001,
      "price": 50,
      "count": 1
    },
    {
      "slot": 1,
      "item_vnum": 11200,
      "price": 500,
      "count": 1
    }
  ]
}
```

Top-level fields:
- `kind` — must equal `shop_preview`
- `ref` — stable non-blank authored identity
- `title` — non-blank merchant preview title
- `catalog` — non-empty list of merchant entries

Catalog entry fields:
- `slot` — stable zero-based catalog position used for deterministic preview ordering and later buy-addressing
- `item_vnum` — stable reference into the deterministic file-backed `internal/itemstore` template catalog
- `price` — non-zero gold price for the entry
- `count` — non-zero quantity previewed/sold by one future buy action

## Validation rules

The structured merchant contract must validate all of the following:
- `kind` must equal `shop_preview`
- `ref` must be non-blank after trimming
- `title` must be non-blank after trimming
- `catalog` must contain at least one entry
- legacy location fields `map_index`, `x`, and `y` remain zero for this kind
- freeform `text` is no longer the contract-bearing merchant payload once the structured shape is wired
- each `slot` must be unique inside the catalog
- slots must form a dense zero-based sequence after sorting (`0..n-1`) so later transaction addressing stays deterministic
- each `item_vnum` must be non-zero and must resolve to a valid template in the loaded `internal/itemstore` catalog
- each `price` must be greater than zero
- each `count` must be greater than zero
- if the referenced template is non-stackable, `count` must equal `1`
- if the referenced template is stackable, `count` must not exceed that template's `max_count`

## Deterministic preview rendering

The structured merchant catalog still remains **preview only** in this slice.
It does not open a real merchant UI or a buy/sell packet family.

When a visible static actor resolves to a valid structured `shop_preview` catalog, the deterministic preview string is built like this:
- sort entries by `slot` ascending
- render each entry as:
  - `[<slot>] <template.name> x<count> @ <price>g`
- join rendered entries with `; `
- prefix the joined entry list with `<title>: `

Example rendered preview:

```text
Village Merchant: [0] Small Red Potion x1 @ 50g; [1] Wooden Sword x1 @ 500g
```

This exact compact render is the frozen preview string for:
- the one self-only `GC_CHAT` browse response after `INTERACT`
- the compact preview string shown by `GET /local/interaction-visibility`

## Runtime behavior contract

Once the structured merchant catalog is wired, interaction behavior stays narrow:
- the runtime keeps the existing visibility and distance checks
- the runtime keeps the existing per-session per-target `1s` cooldown
- the player receives exactly one self-only `GC_CHAT` delivery
- that delivery uses:
  - `CHAT_TYPE_INFO`
  - `VID = 0`
  - `Empire = 0`
  - `Message = deterministic structured merchant preview render`
- no peer-visible frames are emitted
- no gold is debited
- no inventory slots are checked or mutated
- no items are granted
- no merchant stock state is created or persisted

## Transition note for the next slice

At the time this contract is frozen:
- the current runtime still serves `shop_preview` from the existing text-backed interaction definition shape
- loopback interaction CRUD and content-bundle export/import still only know the current text-backed `shop_preview` payload
- the next implementation slice is expected to widen those authored-content surfaces from raw text to the structured `title + catalog[]` merchant shape defined here

This transition is intentional.
The contract is now frozen before the store/bundle/runtime wiring changes.

## Explicit non-goals

This slice does **not** yet freeze:
- buy or sell request packets
- sell-back
- inventory free-slot checks
- gold balance checks or debit flows
- merchant stock depletion or refresh
- page switching or multi-tab shop windows
- drag/drop shopping semantics
- quest-scripted merchant branching
- storage, safebox, or mall integration

## Success definition

After this slice, the repository should be able to say:
- `shop_preview` no longer has to stay conceptually tied to arbitrary merchant text forever
- the first structured merchant catalog shape is frozen in project-owned docs
- merchant entries now refer to owned item templates by stable `vnum`
- the future preview string format is deterministic enough to drive RED tests in the next slice
- the project still does not pretend that real merchant transactions already exist
