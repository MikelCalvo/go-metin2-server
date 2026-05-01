# Reconnect/Reclaim Appearance Next Slices Plan

> **For Hermes:** use `test-driven-development`. Keep slices tiny, keep `main` green, and land tests + code + docs together.

**Goal:** finish the smallest remaining reconnect/reclaim appearance-correctness gaps before switching back to the pending merchant hybrid remainder.

**Architecture:** stay on the existing `internal/minimal` shared-world/session seams. Treat the replacement live owner as authoritative after reclaim, and force stale sockets to stay self-local/non-authoritative for any later appearance-driving item mutation.

**Tech Stack:** Go 1.26, `internal/minimal`, `internal/player`, `internal/inventory`, account-backed snapshots in `internal/accountstore`, protocol docs in `spec/protocol/`, manual QA in `docs/qa/manual-client-checklist.md`.

---

## Why this branch next

Recent slices already froze appearance correctness for:
- stable already-visible peers
- late join
- radius AOI move-into-range
- transfer-driven visibility rebuild
- reconnect-driven visibility rebuild
- duplicate-live retry `ENTERGAME`

The remaining high-value gap on this path is **what happens after live ownership is reclaimed**. The runtime already hardens `MOVE`, `SYNC_POSITION`, and `WHISPER` for stale sockets, but appearance-driving item mutations are not yet frozen the same way.

---

## Planned next slices

### Slice 13 — stale reclaimed equip/unequip stays non-authoritative
**Objective:** after a replacement session has reclaimed the live character, a stale old socket may at most observe self-local equip/unequip frames; it must not persist account state, must not queue peer appearance refreshes, and must not overwrite the replacement owner's live runtime snapshots.

**Likely files:**
- `internal/minimal/shared_world_test.go`
- `internal/minimal/factory.go`
- `spec/protocol/runtime-reconnect-cleanup.md`
- `spec/protocol/equipment-appearance-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `README.md`

### Slice 14 — stale reclaimed `/use_item` stays non-authoritative
**Objective:** extend the same stale-socket rule from equip/unequip to the first remaining selected-item mutation entrypoint by freezing `/use_item <slot>` as self-local-only after reclaim.

**Likely files:**
- `internal/minimal/shared_world_test.go`
- `internal/minimal/factory.go`
- `spec/protocol/runtime-reconnect-cleanup.md`
- `spec/protocol/item-use-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `README.md`

### Slice 15 — stale reclaimed merchant buy stays non-authoritative
**Objective:** apply the same stale-socket rule to the active merchant buy path so stale post-reclaim `SHOP BUY` / `/shop_buy` grants cannot persist or replace the authoritative live owner's loopback state.

**Likely files:**
- `internal/minimal/shared_world_test.go`
- `internal/minimal/factory.go`
- `spec/protocol/runtime-reconnect-cleanup.md`
- `spec/protocol/npc-shop-transaction-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `README.md`

### Slice 16 — retry/reconnect after stale attempted mutation still rebuilds from authoritative state
**Objective:** prove that once a stale socket attempted a non-authoritative item mutation, later retry/reconnect bursts still rebuild from the authoritative persisted/live owner state rather than the stale socket's local divergence.

**Likely files:**
- `internal/minimal/shared_world_test.go`
- `spec/protocol/runtime-reconnect-cleanup.md`
- `spec/protocol/equipment-appearance-bootstrap.md`
- `docs/qa/manual-client-checklist.md`

### Slice 17 — return to merchant hybrid multi-stack + fresh-slot remainder
**Objective:** resume the paused merchant line and open the explicit RED/green slice for the still-pending `multi-stack existing + fresh-slot remainder` placement case.

**Likely files:**
- `internal/player/runtime_inventory_test.go`
- `internal/minimal/shared_world_test.go`
- `internal/player/runtime.go`
- `spec/protocol/item-stack-bootstrap.md`
- `spec/protocol/npc-shop-transaction-bootstrap.md`
- `README.md`

---

## Immediate execution order

1. Return to the paused merchant `multi-stack existing + fresh-slot remainder` case with a new narrow RED test.
2. Keep the slice purely on deterministic placement semantics; do not reopen reconnect/reclaim or broader merchant UI choreography.
3. Update `item-stack-bootstrap.md`, `npc-shop-transaction-bootstrap.md`, and QA notes in the same slice.
4. Run focused tests, then `go test ./...`, then `go vet ./...`.
5. Review, commit, and push before opening any broader merchant/runtime work again.
