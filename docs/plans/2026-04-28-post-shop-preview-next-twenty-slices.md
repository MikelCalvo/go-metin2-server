# Post-shop-preview Next 20 Slices Implementation Plan

> **For Hermes:** use `test-driven-development`. Keep slices tiny, green on `main`, and ship docs + focused RED tests + code + README alignment together whenever behavior changes.

**Goal:** take `go-metin2-server` from the current bootstrap NPC service preview milestone (`info` / `talk` / `warp` / `shop_preview`) into the first real M3 character-state foundation, while still closing the highest-value remaining M2 runtime gaps first.

**Architecture:** split the next work into four bands:
1. finish the remaining M2 ownership cleanups that directly reduce risk before adding new mutable gameplay state,
2. open M3 with inventory/equipment/runtime persistence and the first client-visible self bootstrap for that state,
3. use that new state to unlock the first honest item/equipment behavior,
4. convert `shop_preview` from raw text into structured merchant data and freeze the real transactional gate that follows.

Keep `internal/worldruntime` owning query/diff/snapshot composition, keep `internal/minimal` as the packet/session orchestration layer, keep persistence explicit through file-backed snapshot stores, and avoid speculative UI-heavy protocol design until the client-visible packet family is frozen in docs and tests.

**Tech stack:** Go 1.26, `internal/minimal`, `internal/worldruntime`, `internal/player`, `internal/accountstore`, `internal/loginticket`, existing loopback ops mux under `internal/ops`, current authored-content seams under `internal/interactionstore` and `internal/contentbundle`, new M3 state under new `internal/inventory` / `internal/itemstore` / `internal/proto/item` packages, protocol docs under `spec/protocol/`, plans under `docs/plans/`, and QA docs under `docs/qa/`.

---

## Current starting point
- Current `main` head when this plan is written: `19aeedc feat: refresh npc interaction visibility previews`
- Recently landed slices:
  - `2a14d3f feat: add warp interaction definitions to store`
  - `c6ed2dc feat: widen warp interaction authoring surfaces`
  - `b17de7e feat: add warp npc interactions`
  - `82027de feat: harden warp interaction deny paths`
  - `904e92c feat: add static actor interaction cooldown`
  - `56895ce feat: add shop preview npc interactions`
  - `19aeedc feat: refresh npc interaction visibility previews`
- README still says:
  - M2 is only `[~]` and still needs broader reconnect hardening / richer AOI / fuller non-player systems
  - inventory / equipment / item use are all `[ ]`
  - combat / respawn / quests are all `[ ]`
- Important repo reality right now:
  - the current NPC vertical is honest because it stays one-click and self-facing or transfer-backed
  - real shop buy/sell is still blocked by missing owned item, inventory, equipment, and currency/runtime state
  - there are still no inventory or equipment protocol docs, no item runtime package, and no item persistence layer in the repo

---

## Ordering principles for this 20-slice window
1. Close the highest-risk M2 ownership leaks **before** adding new mutable item/equipment state.
2. Open M3 from the bottom up:
   - docs/protocol contract
   - value objects
   - persistence
   - live runtime
   - packet builders
   - ops/QA
   - first mutations
3. Reuse current seams where they already exist:
   - `player.Runtime` for live selected-session state
   - `worldruntime.Scopes` for topology/AOI-aware query ownership
   - loopback-only ops surfaces for QA before broader gameplay systems exist
4. Do not jump into real shop transactions until both of these are true:
   - item/inventory/currency state is owned and persisted
   - the client-side shop transaction packet family is frozen in docs/tests
5. Keep this plan honest about non-goals:
   - no DB migration story
   - no multi-channel world ownership
   - no full quest runtime
   - no mob AI / spawn groups / pathing
   - no combat beyond a later docs-first handoff slice

---

## Band A — finish the highest-value remaining M2 risk before new mutable gameplay state

### Slice 1: Route join/leave/transfer visibility diffs fully through `internal/worldruntime/scopes.go`

**Objective:** stop keeping the last visibility-diff composition logic split between `internal/minimal/shared_world.go` and `internal/worldruntime`.

**Why now:** this is a low-risk ownership cleanup that reduces drift before inventory/equipment state adds more mutation paths.

**Files:**
- Modify: `internal/worldruntime/scopes.go`
- Modify: `internal/worldruntime/scopes_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/visibility-rebuild.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/worldruntime ./internal/minimal -run 'Test.*(Scopes|VisibilityDiff|Join|Leave|Transfer)' -count=1
```

---

### Slice 2: Route relocation preview and transfer structured result composition through `Scopes`

**Objective:** make before/after visibility plus static-actor occupancy/result composition an owned runtime query surface instead of half-runtime / half-minimal glue.

**Why now:** current relocation preview already matters for operator QA and transfer correctness; consolidating it now will make later character-state previews easier.

**Files:**
- Modify: `internal/worldruntime/scopes.go`
- Modify: `internal/worldruntime/scopes_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/bootstrap-map-transfer-contract.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/worldruntime ./internal/minimal -run 'Test.*(RelocationPreview|Transfer).*' -count=1
```

---

### Slice 3: Reject duplicate-live re-entry even when only the session hook or last-known snapshot survives

**Objective:** close the reconnect corner case where `EntityRegistry` state is gone but the transport/session side still makes the old owner live enough to conflict.

**Why now:** if new M3 state is about to become persistent and mutable, stale duplicate ownership becomes more dangerous than it is today.

**Files:**
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/runtime-reconnect-cleanup.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*(Duplicate|Reclaim|Reconnect|SessionDirectory)' -count=1
```

---

### Slice 4: Stop stale reclaimed sockets from mutating peers or persisted state after ownership loss

**Objective:** once another session has reclaimed the character, the stale socket must stop affecting shared-world fanout **and** must stop overwriting persisted position/chat-visible state.

**Why now:** the next band adds inventory/equipment/currency mutations; stale reclaimed sockets must not be able to race those writes.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/service/secure_legacy_test.go` only if the retryable-socket contract needs wider regression coverage
- Modify: `spec/protocol/runtime-reconnect-cleanup.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/service -run 'Test.*(Reclaim|Stale|Reconnect|Whisper|Move|Sync)' -count=1
```

---

## Band B — open M3 with owned inventory/equipment state

### Slice 5: Freeze the first `inventory-equipment-bootstrap` contract in docs and packet matrix

**Objective:** define the smallest owned M3 surface before code starts inventing item/equipment semantics ad hoc.

**Why now:** the repo currently has no inventory/equipment protocol docs at all, so every implementation choice would otherwise be speculative.

**Files:**
- Create: `spec/protocol/inventory-equipment-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`
- Modify: `docs/testing-strategy.md` if the new packet family needs a golden-test note

**Scope to freeze:**
- minimal self bootstrap only
- inventory slots
- equipment slots
- non-goals for this first stage:
  - no storage mall / safebox
  - no drag-to-ground
  - no trade
  - no crafting
  - no sell-back yet

**Verification:**
- docs no longer treat inventory/equipment as undefined repo territory
- packet-matrix rows exist for the first owned family names/status

---

### Slice 6: Introduce the first owned inventory value objects

**Objective:** create a small package that owns slot identity, item instance identity, stack count, and equipment slot naming before persistence/runtime wiring.

**Why now:** this keeps item semantics out of `loginticket.Character` and `player.Runtime` field soup.

**Files:**
- Create: `internal/inventory/model.go`
- Create: `internal/inventory/model_test.go`
- Create: `internal/inventory/slots.go`
- Create: `internal/inventory/slots_test.go`
- Modify: `README.md`

**Suggested first types:**
```go
type SlotIndex uint16

type EquipmentSlot uint8

type ItemInstance struct {
    ID        uint64
    Vnum      uint32
    Count     uint16
    Slot      SlotIndex
    Equipped  bool
    EquipSlot EquipmentSlot
}
```

**Verification:**
```bash
go test ./internal/inventory -count=1
```

---

### Slice 7: Extend persisted character snapshots with inventory/equipment/currency fields, keeping JSON backwards-compatible

**Objective:** make M3 state durable in `accountstore` / `loginticket` before any runtime mutation is allowed.

**Why now:** persistence must exist before live mutation helpers, or reconnect semantics become hand-wavy immediately.

**Files:**
- Modify: `internal/loginticket/store.go`
- Modify: `internal/accountstore/store.go`
- Modify: `internal/accountstore/store_test.go`
- Modify: `internal/loginticket/store_test.go`
- Modify: `README.md`
- Modify: `spec/protocol/inventory-equipment-bootstrap.md`

**Notes:**
- preserve old snapshots that lack the new fields
- keep zero/default inventory as empty, not malformed
- make the first currency field explicit instead of hiding it behind undocumented point indices

**Verification:**
```bash
go test ./internal/accountstore ./internal/loginticket -count=1
```

---

### Slice 8: Attach inventory/equipment/currency state to `player.Runtime`

**Objective:** keep selected-session live state for M3 in the same owned runtime boundary already used for position/state today.

**Why now:** the project already separates persisted snapshot from live runtime; inventory/equipment should follow the same pattern from the first slice.

**Files:**
- Modify: `internal/player/runtime.go`
- Create: `internal/player/runtime_inventory_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `README.md`

**Notes:**
- add nil-safe runtime helpers
- make persisted snapshot updates explicit, not silent
- keep runtime mutation APIs narrow: move item, equip, unequip, set currency, apply persisted snapshot

**Verification:**
```bash
go test ./internal/player ./internal/minimal -run 'Test.*(Runtime|Inventory|Equipment)' -count=1
```

---

### Slice 9: Create the first owned item/inventory protocol codec package with golden tests

**Objective:** own the wire format for the first self inventory/equipment packet family before enter-game wiring.

**Why now:** this repo is protocol-first; packet builders should be frozen before they are threaded through `worldentry`.

**Files:**
- Create: `internal/proto/item/item.go`
- Create: `internal/proto/item/item_test.go`
- Optionally create: `testdata/packets/item/`
- Modify: `docs/testing-strategy.md`
- Modify: `spec/protocol/inventory-equipment-bootstrap.md`

**Verification:**
```bash
go test ./internal/proto/item -count=1
```

---

### Slice 10: Add loopback-only inventory/equipment introspection endpoints

**Objective:** make the new M3 state observable and debuggable before client-visible mutations are attempted.

**Why now:** the repo already benefited from `/local/visibility`, `/local/interaction-visibility`, and `/local/content-bundle`; inventory should get the same operator footing immediately.

**Files:**
- Modify: `internal/ops/pprofmux.go`
- Modify: `internal/ops/pprofmux_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `README.md`

**Suggested endpoints:**
- `GET /local/inventory/{name}`
- `GET /local/equipment/{name}`
- optionally `GET /local/currency/{name}` if kept separate

**Verification:**
```bash
go test ./internal/ops ./internal/minimal -run 'Test.*(Inventory|Equipment|Currency).*Loopback' -count=1
```

---

### Slice 11: Emit self inventory/equipment bootstrap on `ENTERGAME`

**Objective:** make the first M3 state client-visible in the same docs-first, self-bootstrap-first style the repo used for character and point state.

**Why now:** after docs, value objects, persistence, runtime, and codecs exist, the next honest payoff is seeing owned item/equipment state on world entry.

**Files:**
- Modify: `internal/worldentry/flow.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/boot/flow_socket_test.go`
- Modify: `spec/protocol/inventory-equipment-bootstrap.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/worldentry ./internal/minimal ./internal/boot -run 'Test.*(EnterGame|Bootstrap|Inventory|Equipment)' -count=1
```

---

### Slice 12: Implement the first inventory mutation: slot move/swap with persistence writeback

**Objective:** prove the first real M3 mutation loop end-to-end without introducing item use, equipment bonuses, or shop transactions yet.

**Why now:** moving/swapping items is the smallest honest mutation that exercises runtime + persistence + packet response together.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/player/runtime.go`
- Modify: `internal/inventory/model.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `spec/protocol/inventory-equipment-bootstrap.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/player ./internal/inventory -run 'Test.*(InventoryMove|InventorySwap|Persistence)' -count=1
```

---

## Band C — use M3 to unlock the first honest item/equipment behavior

### Slice 13: Implement equip/unequip flow with self refresh packets

**Objective:** open the first owned equipment behavior on top of the new inventory runtime without yet claiming combat formulas.

**Why now:** equipping is the natural second M3 mutation after slot movement and is a prerequisite for later derived stats.

**Files:**
- Modify: `internal/player/runtime.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/proto/item/item.go`
- Modify: `spec/protocol/inventory-equipment-bootstrap.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/player ./internal/minimal ./internal/proto/item -run 'Test.*(Equip|Unequip|Equipment)' -count=1
```

---

### Slice 14: Freeze the first `item-use-bootstrap` contract

**Objective:** define the smallest owned item-use vertical before behavior starts: consumable-only, self-only, no quest hooks.

**Why now:** the repo now has owned inventory state but still no honest item-use contract.

**Files:**
- Create: `spec/protocol/item-use-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Scope to freeze:**
- one consumable path only
- inventory decrement
- self-facing effect and point/state change
- explicit non-goals:
  - no quest item scripting
  - no timed buffs yet
  - no equipment enchanting
  - no drag-to-world use semantics

**Verification:**
- docs are explicit enough to drive RED tests for one consumable path

---

### Slice 15: Implement the first consumable item use vertical

**Objective:** deliver the first fully owned M3 item-use loop: consume one item, mutate self state, persist, and emit deterministic self updates.

**Why now:** this closes the first end-to-end character-state loop after inventory and equip/unequip.

**Files:**
- Modify: `internal/player/runtime.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/proto/item/item.go`
- Modify: `spec/protocol/item-use-bootstrap.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/player ./internal/minimal ./internal/proto/item -run 'Test.*(ItemUse|Consumable|PointChange)' -count=1
```

---

### Slice 16: Add the first owned item-template seam for future merchant catalogs

**Objective:** stop treating items as only opaque runtime instances by introducing a deterministic item-template catalog that later merchant data can reference.

**Why now:** the next NPC step should not hardcode merchant payloads directly inside `shop_preview` text or raw item instances.

**Files:**
- Create: `internal/itemstore/store.go`
- Create: `internal/itemstore/file_store.go`
- Create: `internal/itemstore/file_store_test.go`
- Modify: `README.md`
- Modify: `spec/protocol/inventory-equipment-bootstrap.md`

**Suggested first schema:**
- `vnum`
- `name`
- `stackable`
- `max_count`
- optional `equip_slot`

**Verification:**
```bash
go test ./internal/itemstore -count=1
```

---

## Band D — turn `shop_preview` into structured merchant content and freeze the transactional gate

### Slice 17: Freeze a structured merchant catalog contract behind `shop_preview`

**Objective:** evolve `shop_preview` from raw text into deterministic authored catalog data while keeping the current client-visible behavior as a preview only.

**Why now:** with item templates and inventory state in place, the current merchant preview should stop being pure arbitrary text.

**Files:**
- Create: `spec/protocol/npc-shop-catalog-bootstrap.md`
- Modify: `spec/protocol/npc-shop-preview-bootstrap.md`
- Modify: `spec/protocol/static-actor-interaction-authoring.md`
- Modify: `README.md`

**Scope to freeze:**
- catalog entries refer to item templates by stable ID
- preview rendering stays self-only and deterministic
- still **no** buy/sell packet family in this slice

**Verification:**
- docs are explicit enough to drive store/bundle RED tests next

---

### Slice 18: Extend authored-content surfaces to support structured merchant catalogs

**Objective:** let `interactionstore`, content bundles, example artifacts, and interaction visibility render merchant previews from structured catalog entries instead of raw ad hoc text.

**Why now:** this deepens the current `shop_preview` slice without requiring speculative shop UI transactions yet.

**Files:**
- Modify: `internal/interactionstore/store.go`
- Modify: `internal/interactionstore/file_store_test.go`
- Modify: `internal/contentbundle/bundle.go`
- Modify: `internal/contentbundle/bundle_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/interaction_visibility_test.go`
- Modify: `internal/ops/interaction_visibility_test.go`
- Modify: `docs/examples/bootstrap-npc-service-bundle.json`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/interactionstore ./internal/contentbundle ./internal/minimal ./internal/ops -run 'Test.*(ShopPreview|Catalog|InteractionVisibility|Bundle)' -count=1
```

---

### Slice 19: Freeze the first real buy-only merchant transaction contract

**Objective:** document the exact gate for moving from read-only `shop_preview` into a real purchase flow without pretending the current repo already owns the client-side shop UI semantics.

**Why now:** once structured catalogs exist, the next step should be a docs-first contract, not a speculative implementation jump.

**Files:**
- Create: `spec/protocol/npc-shop-transaction-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`
- Modify: `docs/qa/manual-client-checklist.md`

**This slice should freeze:**
- required client packet family names/headers if already known from captures
- what remains unknown and must be captured before GREEN implementation
- buy-only first, no sell-back, no storage, no drag-drop shopping basket
- required runtime dependencies now satisfied by earlier slices:
  - inventory
  - equipment state
  - currency
  - item templates
  - merchant catalog

**Verification:**
- docs make the implementation gate explicit instead of ambiguous

---

### Slice 20: Add failing tests for the first buy-only merchant transaction path and leave the next GREEN implementation obvious

**Objective:** end this 20-slice window with the next real NPC economy step prepared as a strict RED, so the following implementation pass can stay tiny and confident.

**Why now:** by this point the repo should already own nearly every prerequisite except the exact transaction execution path.

**Files:**
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/interactionstore/file_store_test.go`
- Modify: `internal/contentbundle/bundle_test.go`
- Modify: `spec/protocol/npc-shop-transaction-bootstrap.md`

**RED test shapes to add:**
```go
func TestGameSessionFlowShopBuyInteractionDebitsCurrencyAndAddsItem(t *testing.T)
func TestGameSessionFlowShopBuyInteractionRejectsInsufficientCurrency(t *testing.T)
func TestGameSessionFlowShopBuyInteractionRejectsNoFreeSlot(t *testing.T)
```

**Verification:**
```bash
go test ./internal/minimal ./internal/interactionstore ./internal/contentbundle -run 'Test.*(ShopBuy|Merchant|Currency|Inventory)' -count=1
```
Expected: RED for missing execution behavior, not for malformed test setup.

---

## Recommended execution grouping
- **Slices 1-4**: finish the highest-value remaining M2 ownership/race-condition cleanup
- **Slices 5-12**: open M3 character-state foundation and first client-visible self bootstrap
- **Slices 13-16**: land the first honest item/equipment behavior on top of M3
- **Slices 17-20**: turn `shop_preview` into structured merchant content and freeze the real transaction gate cleanly

---

## Why this order makes the most sense now
1. It respects the current repo truth: M2 is not fully done, but the highest-risk remaining gaps are now **small ownership hardening slices**, not a whole new runtime rewrite.
2. It unlocks the first real next milestone that many later systems depend on: **owned item / inventory / equipment state**.
3. It keeps the current NPC work honest: `shop_preview` becomes more useful **only after** item templates and inventory exist, instead of pretending real buy/sell can land from text alone.
4. It avoids inventing quest/combat/shop UI protocols prematurely; instead it freezes them docs-first at the moment the repo finally has the right state foundation.
5. It leaves the next immediate follow-up after this 20-slice window in a strong place: the first GREEN merchant transaction slice can be implemented from a prepared RED and a much richer owned runtime.

---

## Anti-goals for this 20-slice window
Do **not** jump to these before the above is done:
- real sell-back / storage / safebox
- quest flags, script VM, or branching NPC dialog windows
- mob AI, spawn groups, or pathing
- full combat classes/skills beyond the later docs-first handoff
- DB-backed persistence redesign
- multi-channel or inter-process world ownership
- broad live-ops surfaces for item mutation beyond what is needed for safe local QA

## Ready-to-start next slice
Start with **Slice 1: Route join/leave/transfer visibility diffs fully through `internal/worldruntime/scopes.go`**.

It is the smallest high-signal cleanup left in M2 and lowers the risk of every later M3 mutation slice without delaying the inventory/equipment frontier for long.
