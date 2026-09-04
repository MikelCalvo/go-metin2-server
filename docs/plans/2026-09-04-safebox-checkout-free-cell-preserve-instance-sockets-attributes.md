# Safebox CHECKOUT free-cell preserve instance sockets/attributes — 2026-09-04

## Objective

Freeze the next Track C honesty seam after carried `ITEM_MOVE` partial-split
independent clone landed: open-presentation whole-stack `SAFEBOX_CHECKOUT` into
an empty carried cell must preserve the safebox source's presence-aware
sockets/attributes onto that free-cell placement (independent clone), matching
the already-owned pickup free-cell preserve, exchange finalize free-cell
preserve, and the shipped `SafeboxCheckoutItem` → `WithInventorySlot` →
`CloneSockets` / `CloneAttributes` path.

Today production already clones through `WithInventorySlot`, and protocol/QA
language already claims fresh-cell checkout preserves presence, but focused unit
and session proofs still only cover identity/count (or destination-wins merge).
Without an explicit freeze + proof, a later RED could drop free-cell checkout
back to bare `{ID,Vnum,Count}` placement while docs still claim preserve.

## Why docs-first

This is priority-queue #1 (item-state consistency) on the ordinary warehouse
checkout path. Opening RED without freezing:

- that only empty-destination whole-stack checkout is in scope (not compatible
  merge, already destination-wins),
- that destination must clone rather than share presence pointers,
- how omitted vs explicit-zero presence behaves after checkout,

would invent policy mid-implementation. Keep refine catalysts, mall, and party
ownership notices deferred. Do not reopen destination-wins compatible-merge
contracts already owned for safebox checkout merge.

## Contract to freeze (before RED)

1. **Whole-stack empty-destination checkout**: when `SAFEBOX_CHECKOUT` /
   `SafeboxCheckoutItem` places a remembered safebox stack into an empty carried
   cell, the placed `ItemInstance` must carry an independent clone of the
   safebox source's presence-aware sockets and attributes:
   - if the source `HasSockets()`, clone those sockets onto the destination
     (including explicit `{0,0,0}`) so later safebox-source / carried writes
     cannot alias;
   - if the source `HasAttributes()`, clone those attributes onto the
     destination (including explicit all-zero / type-zero);
   - if the source omits sockets and/or attributes, the destination likewise
     omits them (later encode keeps template fallback).
2. **Source of truth**: the remembered open-presentation / durable safebox cell
   already validated for checkout; destination keeps that item identity as
   already owned.
3. **Wire honesty**: checkout-burst `ITEM_SET` must project presence-aware
   instance sockets/attributes via ordinary `EffectiveSockets` /
   `EffectiveAttributes`.
4. **Persistence**: the selected-character account snapshot after successful
   checkout must round-trip independent presence-aware fields on the placed
   carried cell (including explicit zero).
5. **Non-goals**: compatible merge (already destination-wins count-only),
   refine catalysts / mall / party ownership notices, inventing merge-on-stack
   rules, or changing check-in rematerialize proofs already owned by durable
   safebox FileStore suites.

## Proof shape (RED → GREEN)

1. Unit: seed a safebox source stack with authoritative instance presence
   (active / explicit-zero / omitted) → `SafeboxCheckoutItem` into an empty
   carried cell → destination identity preserved; destination presence is an
   independent clone; mutating destination leaves the seed source pointer
   unchanged; omitted stays omitted.
2. Session: `/open_safebox` → `SAFEBOX_CHECKIN` of a presence-bearing stack →
   `SAFEBOX_CHECKOUT` into an empty cell → `ITEM_SET` + account snapshot carry
   preserved/cloned presence; destination identity matches the checked-in item.
3. Negatives: compatible merge stays count-only destination-wins as already
   owned; closed / empty / over-max rejects stay non-mutating.

## Likely files to change (GREEN)

- `internal/player/compatible_stack_merge_destination_wins_test.go` (or a focused
  checkout preserve twin beside it)
- `internal/minimal/item_storage_runtime_test.go` (or focused checkout preserve
  session twin)
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- `docs/qa/manual-client-checklist.md` (narrow honesty note only if needed)
- `spec/protocol/item-storage-guard-bootstrap.md` already names the contract;
  tighten only if wording still under-claims independence

## Validation

```bash
go test ./internal/player -run 'SafeboxCheckoutItemFreeCellPreserves' -count=1
go test ./internal/minimal -run 'SafeboxCheckoutFreeCellPreservesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: whole-stack empty-destination `SAFEBOX_CHECKOUT` /
`SafeboxCheckoutItem` preserves presence-aware instance sockets/attributes
(including explicit zero; omit→omit with template encode fallback) onto an
independent free-cell clone through checkout-burst `ITEM_SET` and the account
snapshot
(`TestRuntimeSafeboxCheckoutItemFreeCellPreservesSourceInstancePresence`,
`TestGameRuntimeSafeboxCheckoutFreeCellPreservesInstanceSocketsAndAttributes`).
Compatible merge stays destination-wins count-only. Refine catalysts / mall /
party ownership remain deferred.
