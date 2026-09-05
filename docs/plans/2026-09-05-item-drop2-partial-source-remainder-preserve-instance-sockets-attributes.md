# ITEM_DROP2 partial-drop source remainder presence preserve — 2026-09-05

## Objective

Freeze the next Track C honesty seam after merchant partial-sell remainder
presence preserve landed: counted partial `ITEM_DROP` / `ITEM_DROP2` source-slot
`ITEM_UPDATE` must project **presence-aware** sockets/attributes
(`EffectiveSockets` / `EffectiveAttributes`) on the count-only carried remainder
(including explicit zero; omit→omit with template encode fallback), matching the
just-landed merchant sell remainder contract and the already-owned partial
`ITEM_DROP2` fresh ground identity + independent clone path.

Today protocol/QA still say the source-slot `ITEM_UPDATE` preserves
**template-authored** display sockets/attributes. Runtime
`dropInventoryItem` already decrements the cloned inventory cell (so remainder
presence stays independent of later ground writes), and encode already prefers
instance presence. Without an explicit freeze, a later RED could invent
template-copy onto the remainder or reopen the already-owned fresh ground-ID
contract.

## Why docs-first

This is priority-queue #1 (drop/pickup item-state consistency) on the ordinary
PvE drop path. Opening RED without freezing:

- that the carried remainder is count-only identity-preserving,
- that instance presence (incl. explicit zero) wins over template on the
  source-slot `ITEM_UPDATE`,
- that omitted presence stays omitted,
- that the already-owned partial ground fresh-ID + independent clone stays in
  force beside this remainder wording correction,

would leave the stale template-only wording as the contract. Keep refine
catalysts, mall, and party ownership notices deferred. Do not reopen whole-stack
ground preserve, sell remainder, or pickup destination-wins merge contracts.

## Contract to freeze (before RED)

1. **Partial drop source remainder**: when accepted counted `ITEM_DROP` /
   `ITEM_DROP2` drops `1..source_count-1` from a carried stack, the remaining
   carried `ItemInstance` must:
   - keep the source item identity and slot;
   - decrement only `Count`;
   - keep presence-aware sockets/attributes as before the drop (including
     explicit zero; omit→omit), without copying template display metadata onto
     instance presence.
2. **Wire honesty**: source-slot `ITEM_UPDATE` must project presence-aware
   instance sockets/attributes via ordinary `EffectiveSockets` /
   `EffectiveAttributes` (instance presence including explicit zero wins over
   template; omitted keeps template fallback).
3. **Ground twin already owned**: the registered partial ground handle continues
   to mint a fresh identity plus an independent presence clone
   (`docs/plans/2026-09-03-item-drop2-partial-fresh-ground-instance-id-and-clone.md`).
4. **Persistence**: the selected-character account snapshot after successful
   partial drop must round-trip remainder presence (including explicit zero)
   without template-copy onto instance fields.
5. **Non-goals**: inventing new drop reject/ownership policy, reopen whole-stack
   ground preserve, reopen pickup destination-wins merge, refine catalysts /
   mall / party ownership notices, or gold `DROP2`.

## Proof shape (for later RED → GREEN)

1. Unit: seed a multi-count carried stack with authoritative instance presence
   (active / explicit-zero / omitted) → partial drop → remainder identity/slot
   preserved; presence unchanged and independent of the ground clone.
2. Session: partial `ITEM_DROP2` whose instance sockets/attrs differ from the
   loaded template → source-slot `ITEM_UPDATE` + account snapshot carry instance
   presence (not template); omitted stays omitted / template-fallback encode;
   ground handle stays fresh-ID + independent clone as already owned.
3. Negatives: whole-stack remove, anti-drop / locked / over-count rejects stay
   non-mutating as already owned.

## Likely files to change (later GREEN, not this freeze)

- focused proofs under `internal/player` / `internal/minimal`
- `spec/protocol/item-drop-pickup-bootstrap.md` (wording already corrected here)
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/player -run 'Drop.*PreservesInstance|PartialDrop.*Presence' -count=1
go test ./internal/minimal -run 'ItemDrop2Partial.*PreservesInstance|PartialDrop.*Presence' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: counted partial `ITEM_DROP` / `ITEM_DROP2` source remainder
keeps identity/slot and an independent presence-aware sockets/attributes clone
(including explicit zero; omit→omit) through source-slot `ITEM_UPDATE` + account
snapshot (`TestRuntimeDropInventoryItemPreservesInstancePresenceIndependently`,
`TestGameRuntimeItemDrop2PartialPreservesInstanceSocketsAndAttributes`). Fresh
ground identity + independent clone stays already owned. Refine catalysts / mall
/ party ownership remain deferred.
