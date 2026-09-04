# Safebox CHECKIN presence-aware instance sockets/attributes preserve — 2026-09-04

## Objective

Freeze the next Track C honesty seam after whole-stack `ITEM_DROP` /
`ITEM_DROP2` ground presence preserve landed: open-presentation
`SAFEBOX_CHECKIN` must store an **independent** presence-aware
sockets/attributes clone of the removed carried stack into the remembered
safebox cell / durable FileStore (including explicit zero; omit→omit), matching
the already-owned checkout free-cell preserve, durable rematerialize suites, and
`WithInventorySlot` / drop clone helpers.

Today production already projects instance `EffectiveSockets` /
`EffectiveAttributes` onto check-in `SAFEBOX_SET` and durable rematerialize
proves values survive restart, but `SafeboxCheckinItem` still returns the live
struct with shared presence pointers and focused proofs still only cover
identity/count (or rematerialize values without independence). Without an
explicit freeze + proof, a later RED could keep pointer-sharing into the
presentation/durable cell while docs claim presence round-trips honestly.

## Why docs-first

This is priority-queue #1 (item-state consistency) on the ordinary warehouse
check-in path — the inbound twin of the already-owned checkout free-cell
preserve. Opening RED without freezing:

- that check-in stores an independent clone rather than sharing the pre-checkin
  seed pointers,
- how omitted vs explicit-zero presence behaves after check-in,
- that wire `SAFEBOX_SET` / durable reopen stay presence-aware,

would invent policy mid-implementation. Keep refine catalysts, mall, and party
ownership notices deferred. Do not reopen checkout free-cell preserve or
destination-wins merge contracts.

## Contract to freeze (before RED)

1. **Whole-stack check-in presence clone**: when accepted `SAFEBOX_CHECKIN` /
   `SafeboxCheckinItem` removes a carried stack into an empty in-range safebox
   cell, the stored / returned `ItemInstance` must:
   - keep the source item identity and count;
   - carry an independent clone of the source's presence-aware sockets and
     attributes:
     - if the source `HasSockets()`, clone those sockets onto the safebox cell
       (including explicit `{0,0,0}`) so later inventory/seed writes cannot
       alias the remembered/durable cell;
     - if the source `HasAttributes()`, clone those attributes onto the safebox
       cell (including explicit all-zero / type-zero);
     - if the source omits sockets and/or attributes, the safebox cell likewise
       omits them (later encode / rematerialize keeps template fallback).
2. **Source of truth**: the pre-checkin carried stack already validated for
   check-in; inventory removal stays as already owned.
3. **Wire honesty**: check-in-burst `SAFEBOX_SET` must project presence-aware
   instance sockets/attributes via ordinary `EffectiveSockets` /
   `EffectiveAttributes`.
4. **Persistence**: the durable same-account safebox FileStore cell (when
   durable sync is enabled) and same-session reopen `SAFEBOX_SET` must
   round-trip independent presence-aware fields (including explicit zero).
5. **Non-goals**: checkout free-cell preserve (already owned), compatible merge
   (already destination-wins), refine catalysts / mall / party ownership
   notices, inventing merge-on-stack rules, or changing exclusive-ownership /
   despawn timers.

## Proof shape (RED → GREEN)

1. Unit: seed a carried stack with authoritative instance presence (active /
   explicit-zero / omitted) → `SafeboxCheckinItem` → returned item keeps
   identity/count; presence is an independent clone; mutating the returned
   clone leaves the seed pointer unchanged; omitted stays omitted.
2. Session: seed a presence-bearing stack → `/open_safebox` → `SAFEBOX_CHECKIN`
   → check-in `SAFEBOX_SET` + reopen `SAFEBOX_SET` carry preserved presence;
   durable FileStore round-trip when sync is enabled.
3. Negatives: anti-safebox / closed / occupied / out-of-range rejects stay
   non-mutating as already owned.

## Likely files to change (GREEN)

- `internal/player/runtime.go` (`SafeboxCheckinItem` clone on return)
- `internal/player/runtime_test.go` or focused checkin preserve twin
- `internal/minimal/item_storage_runtime_test.go` (session twin)
- `spec/protocol/item-storage-guard-bootstrap.md`
- `docs/qa/manual-client-checklist.md` (narrow honesty note)
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation

```bash
go test ./internal/player -run 'SafeboxCheckinItemPreservesInstance' -count=1
go test ./internal/minimal -run 'SafeboxCheckinPreservesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: accepted `SAFEBOX_CHECKIN` / `SafeboxCheckinItem` stores
an independent presence-aware sockets/attributes clone (including explicit
zero; omit→omit) onto the remembered safebox cell through check-in
`SAFEBOX_SET` and same-session reopen
(`TestRuntimeSafeboxCheckinItemPreservesInstancePresenceIndependently`,
`TestGameRuntimeSafeboxCheckinPreservesInstanceSocketsAndAttributes`). Durable
rematerialize suites remain owned. Checkout free-cell preserve and
destination-wins merge stay as already proven. Refine catalysts / mall / party
ownership remain deferred.
