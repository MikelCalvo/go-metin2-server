# NPC Shop Catalog Bootstrap

This document freezes the first structured merchant-catalog contract that sits behind `shop_preview` in `go-metin2-server`.

It sits on top of:
- `npc-shop-preview-bootstrap.md`
- `inventory-equipment-bootstrap.md`
- `static-actor-interaction-authoring.md`

The goal is intentionally narrow:
- stop treating merchant previews as arbitrary authored text
- give `shop_preview` a deterministic catalog payload that refers to owned item templates by stable ID
- keep the current merchant identity, preview render, and bootstrap open/buy flow anchored to one deterministic catalog source of truth
- keep the owned surface small enough that later merchant-window acknowledgement or sell-flow slices do not need to reopen the authored catalog contract

## Scope

This contract currently applies only to:
- a bootstrap static actor that is already visible to a connected `GAME` session
- the existing `INTERACT (0x0501)` request targeting that actor by visible `VID`
- one authored `shop_preview` definition identified by stable `kind + ref`
- a structured merchant catalog whose entries refer to owned item templates through stable `vnum`
- a deterministic self-only preview render with no inventory, gold, or persistence mutation

This contract is now implemented across the structured interaction-definition store, loopback authoring/bundle surfaces, interaction-visibility previews, merchant-window open responses, and the first buy-only merchant transaction gate.

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

The structured merchant catalog still owns a deterministic **preview render** in this slice.
That render now lives beside the already-landed merchant-window open / buy flow rather than replacing it.

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
- the compact preview string shown by `GET /local/interaction-visibility`
- lower-level runtime resolution / QA-debug surfaces that still need a deterministic merchant summary without opening the live merchant window

## Runtime behavior contract

With the structured merchant catalog wired, runtime behavior now stays narrow in two coordinated surfaces:
- the runtime keeps the existing visibility and distance checks
- the runtime keeps the existing per-session per-target `1s` cooldown
- live session handling of `INTERACT` now opens the current bootstrap merchant window through `GC::SHOP START`, built from that structured catalog
- the same deterministic preview render remains available for QA/debug and lower-level resolution surfaces without opening the live merchant window
- later `SHOP BUY` / `SHOP END` handling continues to reuse the same active merchant context and the same authored catalog identity
- no peer-visible frames are emitted
- no merchant stock depletion or persistent merchant stock state is created

## Current implemented scope

The structured merchant catalog now already drives:
- loopback interaction CRUD payloads
- content-bundle export/import
- interaction-visibility previews
- the merchant-window `GC::SHOP START` open response
- the first buy-only `SHOP BUY` transaction gate

The next merchant slices are now about richer compatibility-facing `GC::SHOP` acknowledgement choreography and broader merchant semantics, not about replacing text-backed payloads anymore.

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
- `shop_preview` no longer has to stay conceptually tied to arbitrary merchant text
- the first structured merchant catalog shape is frozen in project-owned docs and already wired through the current runtime surfaces
- merchant entries now refer to owned item templates by stable `vnum`
- the preview string format remains deterministic for QA/debug even though live session handling now opens the merchant window instead of sending only chat preview text
- the project still does not pretend that sell-back, stock semantics, or final merchant-window choreography already exist
