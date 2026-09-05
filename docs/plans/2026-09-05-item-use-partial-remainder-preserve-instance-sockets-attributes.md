# ITEM_USE / `/use_item` partial remainder presence preserve — 2026-09-05

## Objective

Freeze the next Track C honesty seam after partial-drop source remainder
presence preserve landed: partial-stack `ITEM_USE` / `/use_item` /
`UseItem` must keep the consumed cell's **presence-aware**
sockets/attributes on the count-only remainder (including explicit zero;
omit→omit with template encode fallback), and the remainder must be an
**independent** clone so later writes cannot alias the pre-use live
inventory pointer.

Today `UseItem` copies the live cell by value, decrements `Count` in place,
and writes that same pointer-bearing struct back into `liveInventory`.
Encode already prefers `EffectiveSockets` / `EffectiveAttributes`, but
spec/QA still say partial-use `ITEM_UPDATE` preserves **template-authored**
display arrays, and focused remainder proofs still omit independence.
Partial drop / merchant sell remainders are GREEN; use remainder still
aliases.

## Why docs-first

This is priority-queue #1 (use persistence / item-state consistency) on
the ordinary PvE consumable path. Opening RED without freezing:

- that partial use is count-only identity-preserving on the same cell,
- that instance presence (incl. explicit zero) wins over template on the
  remainder `ITEM_UPDATE`,
- that omitted presence stays omitted (later encode may use template
  fallback),
- that the remainder must not share sockets/attributes pointers with the
  pre-use live inventory,

would leave the stale template-only wording as the contract. Keep refine
catalysts, mall, and party ownership notices deferred. Do not reopen drop
remainder, sell remainder, or `ITEM_USE_TO_ITEM` destination-wins merge
contracts.

## Contract to freeze (before RED)

1. **Partial use remainder**: when accepted `UseItem` (or packet `ITEM_USE`
   / slash `/use_item`) consumes fewer than the live stack count, the
   remaining `ItemInstance` must:
   - keep the source item identity and slot;
   - decrement only `Count`;
   - carry an independent clone of the source's presence-aware sockets and
     attributes:
     - if the source `HasSockets()`, clone those sockets onto the remainder
       (including explicit `{0,0,0}`) so later writes cannot alias the
       pre-use live inventory pointer;
     - if the source `HasAttributes()`, clone those attributes onto the
       remainder (including explicit all-zero / type-zero);
     - if the source omits sockets and/or attributes, the remainder likewise
       omits them (later encode keeps template fallback).
2. **Last-stack use**: when consume count equals the live stack count, the
   cell is removed (`ITEM_DEL`) as already owned; no remainder presence
   contract applies.
3. **Wire honesty**: partial-use-burst `ITEM_UPDATE` must project
   presence-aware instance sockets/attributes via ordinary
   `EffectiveSockets` / `EffectiveAttributes` (instance presence including
   explicit zero wins over template; omitted keeps template fallback).
4. **Persistence**: the selected-character account snapshot after successful
   partial use must round-trip independent presence-aware fields on the
   remainder (including explicit zero) without copying template display
   metadata onto instance presence.
5. **Non-goals**: inventing new use reject/effect policy, `ITEM_USE_TO_ITEM`
   merge reopen, refine catalysts / mall / party ownership notices, or
   changing last-stack `ITEM_DEL` / locked / anti-use / over-count rejects
   already owned.

## Proof shape (for later RED → GREEN)

1. Unit: seed a multi-count carried consumable with authoritative instance
   presence (active / explicit-zero / omitted) → partial `UseItem` →
   remainder identity/slot preserved; presence is an independent clone;
   mutating the remainder leaves the pre-use live inventory pointer
   unchanged; omitted stays omitted.
2. Session: packet `ITEM_USE` of a presence-bearing stack whose instance
   sockets/attrs differ from the loaded template → use-burst `ITEM_UPDATE`
   + account snapshot carry instance presence (not template); omitted stays
   omitted / template-fallback encode.
3. Negatives: last-stack remove, locked / anti-use / over-count / point
   overflow rejects stay non-mutating as already owned.

## Likely files to change (later GREEN, not this freeze)

- `internal/player/runtime.go` (`UseItem` partial remainder branch)
- `internal/player/use_preserve_instance_presence_test.go` (new)
- `internal/minimal/item_use_preserve_instance_presence_test.go` (new)
- `spec/protocol/item-use-bootstrap.md` (wording already corrected here)
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/player -run 'UseItem.*PreservesInstance' -count=1
go test ./internal/minimal -run 'ItemUse.*PreservesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: partial-stack `ITEM_USE` / `/use_item` remainder keeps
identity/slot and an independent presence-aware sockets/attributes clone
(including explicit zero; omit→omit) through `ITEM_UPDATE` + account snapshot
(`TestRuntimeUseItemPreservesInstancePresenceIndependently`,
`TestGameRuntimeItemUsePartialPreservesInstanceSocketsAndAttributes`).
Last-stack remove / reject paths stay already owned. Refine catalysts / mall /
party ownership remain deferred.
