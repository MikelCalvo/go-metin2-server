# Post-bootstrap Merchant Next 25 Slices Implementation Plan

> **For Hermes:** use `test-driven-development`. Keep slices tiny, green on `main`, and ship docs + focused RED tests + code + README alignment together whenever behavior changes.

**Goal:** take `go-metin2-server` from the current bootstrap merchant-buy harness (`INTERACT` + local `/shop_buy <catalog_slot>`) into the first protocol-owned, client-visible item progression loop, then use that richer M3 footing to leave the first honest combat vertical prepared as a strict RED.

**Architecture:** split the next work into five bands:
1. replace the temporary local merchant-buy chat harness with an owned shop packet family,
2. make merchant item grants honest for stackable and non-stackable items,
3. make bought/equipped state visible through first appearance refreshes,
4. replace hardcoded item/use behavior with the first template-driven derived-stat seams,
5. freeze and prepare the first combat target/attack vertical without overreaching into full damage systems yet.

Keep `internal/worldruntime` as the owner of topology-aware queries and non-player lookup, keep `internal/minimal` as the packet/session orchestration layer, keep `internal/player` owning selected-session mutable character state, keep file-backed stores deterministic, and avoid opening sell-back, quests, mobs, or DB persistence before the buy/equip/stat foundation is honest.

**Tech stack:** Go 1.26, current `internal/minimal`, `internal/game`, `internal/player`, `internal/worldruntime`, `internal/inventory`, `internal/itemstore`, `internal/interactionstore`, `internal/contentbundle`, `internal/proto/*`, file-backed snapshot stores in `internal/accountstore` / `internal/loginticket` / `internal/staticstore`, protocol docs under `spec/protocol/`, plans under `docs/plans/`, and client QA notes under `docs/qa/manual-client-checklist.md`.

---

## Current starting point
- Current `main` head when this plan is written: `a4b62bf feat: add bootstrap merchant buy path`
- Recently landed slices:
  - `15586aa feat: add item template store seam`
  - `261fb43 docs: freeze merchant catalog contract`
  - `21fb8df feat: render structured merchant previews`
  - `33cdeb1 docs: freeze merchant buy gate`
  - `a4b62bf feat: add bootstrap merchant buy path`
- The repo now already owns:
  - self bootstrap inventory/equipment state,
  - bootstrap carried-item move/equip/unequip/use seams,
  - structured `shop_preview`,
  - a file-backed item-template catalog,
  - the first buy-only merchant execution path gated behind `INTERACT` + local `/shop_buy <catalog_slot>`.
- README milestone truth is still roughly:
  - M1 `[~]`
  - M2 `[~]`
  - M3 `[~]`
  - M4 combat `[ ]`
  - M5 content runtime `[ ]`
  - M6 persistence/ops `[ ]`
- Important repo reality right now:
  - merchant buying is real enough for runtime mutation tests, but not yet protocol-owned from the client packet family,
  - stack merge and richer buy semantics are still bootstrap-level,
  - peer-visible appearance refresh for equipment is still missing,
  - derived stats are still largely hardcoded or absent,
  - combat should not go green until shop/item/equip/stat seams are honest enough to support it.

---

## Ordering principles for this 25-slice window
1. Convert the current merchant buy harness into an owned packet path before opening more gameplay systems on top of it.
2. Keep buy-only scope explicit:
   - no sell-back,
   - no shopping basket,
   - no storage/safebox,
   - no stock depletion or dynamic repricing.
3. Make inventory/item semantics more honest from the bottom up:
   - template contract,
   - merge/max-count rules,
   - runtime mutation,
   - packet refresh,
   - QA/docs.
4. Do not start a green combat loop until these are true:
   - bought items can arrive through an owned protocol path,
   - equip state can affect visible appearance and self-facing points,
   - the first target path is frozen in docs/tests.
5. Reuse existing repo seams whenever possible:
   - `player.Runtime` for selected-session mutation,
   - `worldruntime.Scopes` and non-player directories for visibility/lookup,
   - `internal/proto/item` and future `internal/proto/shop` for wire ownership,
   - loopback/local QA surfaces before broader operator/runtime expansion.
6. End this window the same way the previous one ended best: with the next major vertical left as a deliberate RED, not as vague future prose.

---

## Band A — replace the bootstrap local merchant-buy harness with an owned shop packet family

### Slice 1: Freeze the first owned merchant open/close/buy packet contract

**Objective:** document the smallest client-visible shop packet family the repo will own next, including open, buy request, success/failure refresh expectations, and close.

**Why now:** the runtime already knows how to preview and even execute a buy through a local seam, so the highest-value next step is freezing the real wire contract instead of deepening another debug-only path.

**Files:**
- Create: `spec/protocol/npc-shop-open-close-bootstrap.md`
- Modify: `spec/protocol/npc-shop-transaction-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Verification:**
- docs explicitly name the first owned merchant packet family and the buy-only non-goals
- README milestone text no longer implies that merchant transactions are still entirely missing

---

### Slice 2: Add failing codec tests for the merchant packet family

**Objective:** create the first exact RED around parsing/building the selected shop packet frames before runtime wiring begins.

**Why now:** the protocol surface should be owned in tests before `internal/minimal` starts dispatching new packet types.

**Files:**
- Create: `internal/proto/shop/shop_test.go`
- Optionally create: `internal/proto/shop/testdata/*`
- Modify: `spec/protocol/npc-shop-open-close-bootstrap.md`

**Verification:**
```bash
go test ./internal/proto/shop -count=1
```
Expected: RED for missing package/APIs, not for ambiguous fixture shape.

---

### Slice 3: Implement `internal/proto/shop` open/buy/close builders and parsers

**Objective:** own the merchant packet family in a dedicated protocol package parallel to existing `chat`, `item`, `move`, and `interact` packages.

**Why now:** runtime ingress should depend on a small wire package instead of open-coded frame handling inside `internal/minimal`.

**Files:**
- Create: `internal/proto/shop/shop.go`
- Modify: `internal/proto/shop/shop_test.go`
- Modify: `spec/protocol/npc-shop-open-close-bootstrap.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/proto/shop -count=1
```

---

### Slice 4: Add failing session-flow tests for interaction-triggered merchant open and client-driven close

**Objective:** prove end to end that a visible merchant interaction can now open a client-visible shop session and that the session can be closed cleanly.

**Why now:** before buy requests are rerouted, the runtime should own the simpler open/close choreography with explicit merchant context.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/game/flow_test.go`
- Modify: `spec/protocol/npc-shop-open-close-bootstrap.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/game -run 'Test.*(ShopOpen|ShopClose|MerchantOpen)' -count=1
```
Expected: RED for missing runtime behavior.

---

### Slice 5: Implement client-visible merchant open/close in the minimal runtime

**Objective:** on accepted merchant interaction, enqueue the owned shop-open frames and track one active merchant-shop context per session until explicit close or teardown.

**Why now:** it replaces preview-only chat as the primary user-facing merchant surface without yet broadening buy semantics.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/game/flow.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/game -run 'Test.*(ShopOpen|ShopClose|MerchantOpen)' -count=1
```
```bash
go test ./...
```

---

### Slice 6: Route buy requests through the owned shop packet path and demote `/shop_buy` to a local debug harness

**Objective:** make the real shop buy request the default path while keeping `/shop_buy <catalog_slot>` only as an explicitly local/bootstrap seam for QA and recovery.

**Why now:** once open/close is owned, leaving buy execution on a chat slash command would keep the most important merchant mutation outside the actual protocol surface.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/game/flow.go`
- Modify: `spec/protocol/npc-shop-transaction-bootstrap.md`
- Modify: `docs/qa/manual-client-checklist.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/game ./internal/proto/shop -run 'Test.*(ShopBuy|Merchant|ShopOpen)' -count=1
```
```bash
go test ./...
```

---

## Band B — make merchant item grants honest for stackable and non-stackable items

### Slice 7: Freeze the first carried-item stacking and merchant-grant merge contract

**Objective:** document when a merchant buy should merge into an existing carried stack, when it must claim a new slot, and when it must fail closed.

**Why now:** buy requests now arrive through a real packet path, so slot/stack rules must stop being implicit runtime behavior.

**Files:**
- Create: `spec/protocol/item-stack-bootstrap.md`
- Modify: `spec/protocol/npc-shop-transaction-bootstrap.md`
- Modify: `spec/protocol/item-use-bootstrap.md`
- Modify: `README.md`

**Verification:**
- docs define merge vs new-slot behavior using item-template metadata (`stackable`, `max_count`)
- failure cases are explicit: no free slot, invalid count, over-max, and incompatible target merge

---

### Slice 8: Add failing tests for stackable merchant buys merging into carried inventory

**Objective:** prove that stackable merchant purchases should prefer an existing compatible carried stack before allocating a fresh slot.

**Why now:** this is the smallest behavior gap between "runtime can add an item" and "merchant buying behaves like a real inventory system".

**Files:**
- Modify: `internal/player/runtime_inventory_test.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/itemstore/file_store_test.go`
- Modify: `spec/protocol/item-stack-bootstrap.md`

**Verification:**
```bash
go test ./internal/player ./internal/minimal ./internal/itemstore -run 'Test.*(Stack|MerchantBuy|Inventory)' -count=1
```
Expected: RED for missing merge semantics.

---

### Slice 9: Implement template-driven stack merge for merchant grants

**Objective:** teach `player.Runtime` to merge eligible merchant grants into an existing carried stack up to `max_count` before claiming a new slot.

**Why now:** runtime mutation rules should live below session orchestration and be reusable for later loot/reward/item-grant paths.

**Files:**
- Modify: `internal/player/runtime.go`
- Modify: `internal/player/runtime_inventory_test.go`
- Modify: `internal/inventory/model.go`
- Modify: `internal/itemstore/store.go`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/player ./internal/inventory ./internal/itemstore -run 'Test.*(Stack|Merge|MerchantGrant)' -count=1
```

---

### Slice 10: Add failing tests for partial-merge and full-inventory deny paths

**Objective:** cover the harder buy cases where an existing stack can absorb only part of the grant or where no valid new slot remains after merge attempts.

**Why now:** once the happy-path merge exists, denial behavior becomes the next correctness risk for gold and item consistency.

**Files:**
- Modify: `internal/player/runtime_inventory_test.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/item-stack-bootstrap.md`
- Modify: `spec/protocol/npc-shop-transaction-bootstrap.md`

**Verification:**
```bash
go test ./internal/player ./internal/minimal -run 'Test.*(PartialMerge|InventoryFull|MerchantBuy)' -count=1
```
Expected: RED for unfinished edge-case handling.

---

### Slice 11: Implement fail-closed merchant grant validation and deterministic user feedback

**Objective:** make invalid or impossible merchant buys reject cleanly with deterministic state preservation and one stable self-facing explanation.

**Why now:** the repo should not grow more item paths before buy failures are as explicit and testable as buy successes.

**Files:**
- Modify: `internal/player/runtime.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/proto/chat/chat.go` only if one stable self-facing info message helper is needed
- Modify: `docs/qa/manual-client-checklist.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/player ./internal/minimal -run 'Test.*(MerchantBuy|InventoryFull|Insufficient|Invalid).*' -count=1
```
```bash
go test ./...
```

---

## Band C — make bought and equipped state visible through first appearance refreshes

### Slice 12: Freeze the first equipment-driven appearance refresh contract

**Objective:** document the smallest peer-visible appearance fields the repo will now own when bought items become equipable and visible.

**Why now:** M3 is no longer only self-facing; once merchants can grant real equipment, the next honest improvement is making at least one equipped path visible to the player and peers.

**Files:**
- Create: `spec/protocol/equipment-appearance-bootstrap.md`
- Modify: `spec/protocol/inventory-equipment-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Verification:**
- docs name the first owned visible appearance fields and explicitly defer full costume/refine/class detail

---

### Slice 13: Add failing tests for equip/unequip self and peer appearance refresh

**Objective:** prove that equipping or unequipping the chosen first visible slot emits deterministic self and peer refresh behavior.

**Why now:** the contract should be test-led before runtime code starts mutating visible character state.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/player/runtime_inventory_test.go`
- Modify: `spec/protocol/equipment-appearance-bootstrap.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/player -run 'Test.*(Equip|Unequip|Appearance|CharacterUpdate)' -count=1
```
Expected: RED for missing refresh behavior.

---

### Slice 14: Implement minimal appearance projection from equipped slots

**Objective:** project the first owned equipment appearance data from live runtime state into bootstrap character refresh frames.

**Why now:** later combat and content loops will be easier to reason about if visible state already depends on the same owned equipment model as self-facing inventory.

**Files:**
- Modify: `internal/player/runtime.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/proto/world/world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/player ./internal/minimal ./internal/proto/world -run 'Test.*(Appearance|Equip|CharacterUpdate)' -count=1
```
```bash
go test ./...
```

---

### Slice 15: Add failing end-to-end tests for buy-then-equip over the owned shop packet path

**Objective:** connect the newly owned merchant protocol path to the newly visible equipment refresh path in one narrow end-to-end RED.

**Why now:** it proves the current bands are joining into a real gameplay loop instead of isolated subsystems.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `docs/qa/manual-client-checklist.md`
- Modify: `spec/protocol/npc-shop-transaction-bootstrap.md`

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*(ShopBuy.*Equip|BuyThenEquip|Merchant.*Appearance)' -count=1
```
Expected: RED for the missing cross-slice loop.

---

### Slice 16: Implement the first end-to-end bought-item equip loop and QA checklist coverage

**Objective:** allow one real bought equipment item to be purchased, equipped, and reflected in visible runtime refreshes without client-only handwaving.

**Why now:** this is the first compact M3 loop that a real player can observe from merchant to equipment state.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/player/runtime.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `docs/qa/manual-client-checklist.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/player -run 'Test.*(BuyThenEquip|Merchant|Appearance|Equip)' -count=1
```
```bash
go test ./...
```
```bash
go vet ./...
```

---

## Band D — replace hardcoded item behavior with the first template-driven derived-stat seams

### Slice 17: Freeze the first derived-stat contract from base snapshot, equipment, and item effects

**Objective:** define the smallest owned derived-stat model needed next: what comes from persisted/base character state, what can be modified by equipment, and what item-use effects refresh immediately.

**Why now:** the repo already has item use, equipment, and merchant buy paths, but they still lack one shared stat contract that later combat can trust.

**Files:**
- Create: `spec/protocol/player-derived-stats-bootstrap.md`
- Modify: `spec/protocol/item-use-bootstrap.md`
- Modify: `spec/protocol/inventory-equipment-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `README.md`

**Verification:**
- docs clearly separate base state, derived refresh, and deferred combat/stat complexity

---

### Slice 18: Add failing tests for template-driven consumable effect resolution

**Objective:** prove that the first consumable effect should come from item-template metadata instead of a hardcoded `vnum` switch hidden in runtime code.

**Why now:** if merchant/catalog/item-use behavior is going to expand, hardcoded effect logic will become the next major drift source.

**Files:**
- Modify: `internal/itemstore/store.go`
- Modify: `internal/itemstore/file_store_test.go`
- Modify: `internal/player/runtime_test.go`
- Modify: `spec/protocol/player-derived-stats-bootstrap.md`

**Verification:**
```bash
go test ./internal/itemstore ./internal/player -run 'Test.*(Consumable|Effect|ItemTemplate|Derived)' -count=1
```
Expected: RED for missing metadata/runtime resolution.

---

### Slice 19: Implement item-template effect metadata and runtime effect resolution

**Objective:** extend the deterministic item-template store with the smallest effect metadata needed for the first consumable/stat-driven paths.

**Why now:** it turns item use and later merchant grants into template-owned behavior instead of one-off runtime special cases.

**Files:**
- Modify: `internal/itemstore/store.go`
- Modify: `internal/itemstore/file_store.go`
- Modify: `internal/itemstore/file_store_test.go`
- Modify: `internal/player/runtime.go`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/itemstore ./internal/player -run 'Test.*(ItemTemplate|Effect|Consumable|Runtime)' -count=1
```

---

### Slice 20: Add failing tests for equipment-derived point changes and recompute ordering

**Objective:** prove that equipping or unequipping the chosen first stat-bearing item recomputes derived points in a deterministic order and emits the right refreshes.

**Why now:** appearance-only equipment is not enough for the later combat slice; at least one real stat dependency should exist first.

**Files:**
- Modify: `internal/player/runtime_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/player-derived-stats-bootstrap.md`

**Verification:**
```bash
go test ./internal/player ./internal/minimal -run 'Test.*(Derived|Equip|PointChange|Recompute)' -count=1
```
Expected: RED for missing derived-stat recompute behavior.

---

### Slice 21: Implement minimal derived-stat recompute with self-facing point refresh on equip and item use

**Objective:** make the first owned derived-stat loop real by recomputing a narrow stat set and refreshing points after equip/unequip/use.

**Why now:** it closes the M3 story far more honestly and gives the next combat plan a real state boundary instead of placeholders.

**Files:**
- Modify: `internal/player/runtime.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/proto/world/world.go`
- Modify: `internal/player/runtime_test.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/player ./internal/minimal ./internal/proto/world -run 'Test.*(Derived|PointChange|Equip|UseItem)' -count=1
```
```bash
go test ./...
```
```bash
go vet ./...
```

---

## Band E — freeze and prepare the first combat vertical from the richer M3/M5 footing

### Slice 22: Freeze the first combat target-selection contract around a visible training dummy actor

**Objective:** document the smallest non-player combat target path the repo can own next without opening full mob AI or spawn systems.

**Why now:** after the above slices, the project finally has enough owned character/item state to tee up combat honestly, but it still needs a deliberately tiny target surface.

**Files:**
- Create: `spec/protocol/combat-training-dummy-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`
- Modify: `docs/qa/manual-client-checklist.md`

**Verification:**
- docs explicitly freeze a visible training-dummy target path and explicitly defer real mobs, aggro, damage formulas, death, and respawn

---

### Slice 23: Add failing tests for target lookup, ownership, and range gating against the first combat dummy

**Objective:** prove that attack intent should fail closed unless the selected dummy target is visible, targetable, and in the allowed range band.

**Why now:** ownership and range validation should be in place before any attack request path goes green.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/non_player_directory_test.go`
- Modify: `internal/game/flow_test.go`
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime ./internal/game -run 'Test.*(Target|AttackIntent|CombatDummy|Range)' -count=1
```
Expected: RED for missing target path behavior.

---

### Slice 24: Introduce the minimal targetable non-player runtime seam for combat dummies

**Objective:** add the smallest runtime metadata and lookup path needed for a visible non-player actor to be considered targetable by the next attack slice.

**Why now:** it keeps the first combat target path reusing current static/non-player ownership instead of inventing mobs wholesale.

**Files:**
- Modify: `internal/worldruntime/entity.go`
- Modify: `internal/worldruntime/non_player_directory.go`
- Modify: `internal/worldruntime/non_player_directory_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/worldruntime ./internal/minimal -run 'Test.*(NonPlayer|Targetable|CombatDummy)' -count=1
```
```bash
go test ./...
```

---

### Slice 25: Add failing tests for the first attack request path and leave the next GREEN obvious

**Objective:** finish this 25-slice window with a strict RED for one minimal combat attack request against the training dummy, including expected deny paths and the intended success-frame shape.

**Why now:** it is the cleanest handoff into the next milestone window: M3 is much more honest, content/runtime seams are richer, and the first combat green can stay tiny.

**Files:**
- Modify: `internal/game/flow_test.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/game ./internal/minimal -run 'Test.*(Attack|CombatDummy|Target|Deny)' -count=1
```
Expected: RED for missing attack execution behavior, not for malformed harness setup.

---

## Recommended execution grouping
- **Slices 1-6**: own the merchant packet family and make client-visible merchant open/buy/close the default path
- **Slices 7-11**: make merchant grants inventory-correct for stackable and denial edge cases
- **Slices 12-16**: connect bought items to visible equipment appearance and real end-to-end equip loops
- **Slices 17-21**: replace hardcoded item logic with template-driven derived-stat seams and point refreshes
- **Slices 22-25**: freeze the training-dummy combat target path and leave the first attack green prepared as a RED

---

## Why this order makes the most sense now
1. It pays down the current biggest honesty gap first: merchant transactions exist, but the primary buy path is still bootstrap-local instead of protocol-owned.
2. It keeps inventory correctness ahead of feature breadth: merge/max-count/deny rules come before any wider content or reward systems.
3. It gives players something actually visible from the new M3 work: bought and equipped items can start affecting appearance, not just hidden runtime state.
4. It turns item behavior from ad hoc runtime assumptions into template-driven logic that later merchants, loot, and combat can reuse.
5. It finally opens combat the disciplined way: docs first, target path first, RED first, with no pressure to fake a whole mob system in one leap.

---

## Anti-goals for this 25-slice window
Do **not** jump to these before the above is done:
- sell-back, `SELL2`, shopping basket, or safebox/storage
- quest/script runtime or branching dialog trees beyond current NPC seams
- full mob AI, spawn groups, or pathing
- broad combat formula design, death, respawn, or skill systems
- DB-backed persistence redesign
- multi-channel topology or inter-process world ownership
- heavy UI archaeology without a tight runtime payoff
- large private branches that bypass slice-by-slice public validation

---

## Ready-to-start next slice
Start with **Slice 1: Freeze the first owned merchant open/close/buy packet contract**.

It is the smallest high-signal next step after `a4b62bf`: the runtime already proved a buy-only path can work, so now the project should own the real packet family before expanding item, appearance, stat, or combat behavior.