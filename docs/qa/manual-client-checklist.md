# Manual Client QA Checklist

This checklist is the reusable manual QA gate for real-client validation against the current Go server.

Use it to:
- validate milestone progress with a real client, not only automated tests
- keep a stable regression checklist across slices
- record what the client can really do today without mixing in legacy-server expectations

This checklist is intentionally conservative.
It focuses on the current bootstrap scope and avoids treating clearly out-of-scope systems as regressions.

## How to use this document

For each manual run, create a short run note and fill the checklist against the exact build under test.

Suggested run-note template:

```md
## Manual QA Run
- Date/time:
- Tester:
- Server commit/build:
- Client build/hash:
- Target IP:
- Target auth port:
- Target game port:
- Legacy server also running: yes/no
- Result: PASS / PASS WITH ISSUES / FAIL
- Blocking issues:
- Non-blocking issues:
- Logs captured:
- Next action:
```

## Current bootstrap assumptions

Adapt these to the deployment under test:

- auth port: use the configured `authd` legacy port
- game port: use the configured `gamed` legacy port
- if the default minimal runtime is used, the current stub credentials are:
  - login: `mkmk`
  - password: `[REDACTED]`

Important:
- if both the legacy server and the Go server are running, confirm the client is really pointing at the Go server before interpreting results
- if the channel does not appear online, stop and debug publication/firewall/target config first

---

## 0. Test run header

Fill this before starting:

- [ ] Date/time recorded
- [ ] Tester recorded
- [ ] Client build/hash recorded
- [ ] Server commit/build recorded
- [ ] Target IP recorded
- [ ] Target auth port recorded
- [ ] Target game port recorded
- [ ] It is clear whether the legacy server is also running
- [ ] A run note exists for this session

---

## 1. Preflight — safe, non-destructive

### 1.1 Service health

- [ ] `authd` is running
- [ ] `gamed` is running
- [ ] Both expected listen ports are open
- [ ] Recent logs show no fresh fatal startup failure

Expected result:
- the server is stably up before opening the client

### 1.2 Target sanity

- [ ] The client is pointing to the Go auth endpoint, not the legacy auth endpoint
- [ ] The advertised/public IP is reachable from the client machine
- [ ] There is no ambiguity about which server the client is hitting

Expected result:
- a failed client path can be interpreted as a server issue, not a targeting mistake

### 1.3 Channel visibility smoke test

- [ ] Open the client and reach the server/channel list
- [ ] Confirm the target channel appears online/normal

Expected result:
- at least one bootstrap channel is visible as online/normal

If this fails, stop the rest of the checklist and record:
- target client config
- current server publication/firewall state
- recent `authd` and `gamed` logs

---

## 2. Single-client login and selection

### 2.1 Bad credentials path

- [ ] Attempt login with a known bad password

Expected result:
- login is rejected cleanly
- the client does not hang or crash
- the server remains alive

### 2.2 Valid credentials path

- [ ] Login with the configured valid QA credentials

Expected result:
- login succeeds
- the client reaches the character selection surface
- there is no disconnect between auth and selection

### 2.3 Empty-account / empire-selection path

Run this only if the QA account is empty.

- [ ] Confirm empire selection appears when expected
- [ ] Choose an empire once
- [ ] Verify the session remains usable after empire selection

Expected result:
- empire selection is accepted
- the client returns to a valid selection/create state

### 2.4 Character list rendering

- [ ] Existing characters appear on the selection screen
- [ ] Character names render correctly
- [ ] Character slots do not show obvious corruption

Expected result:
- the selection surface is usable enough for continued testing

---

## 3. Character creation / deletion

Use dedicated QA names to avoid confusion.
A prefix like `QA_` is recommended.

### 3.1 Create character

- [ ] Create a new character in an empty slot
- [ ] Use a dedicated QA name
- [ ] Verify the new character appears in the selection screen immediately

Expected result:
- create succeeds cleanly
- the new character is visible without restarting the client

### 3.2 Invalid / duplicate create guard

- [ ] Attempt one clearly invalid or duplicate create case

Expected result:
- the client receives a clean failure path
- the session remains usable afterward
- no forced disconnect occurs

### 3.3 Delete character

Run this only on a disposable QA character.

- [ ] Delete the disposable QA character
- [ ] Confirm the slot updates correctly in the selection screen

Expected result:
- delete succeeds cleanly
- the deleted character disappears from the selection surface
- no selection-state desync occurs

---

## 4. World entry

### 4.1 Select character

- [ ] Select a valid character

Expected result:
- the client leaves selection cleanly
- the loading phase is stable

### 4.2 Enter game

- [ ] Complete the enter-game flow
- [ ] Wait until the character appears in-world

Expected result:
- the character spawns in-world
- there is no immediate disconnect
- there is no client crash
- there is no server crash

### 4.3 Stability after entry

- [ ] Stay idle for 15 seconds after spawn
- [ ] Perform only minor input such as camera rotation

Expected result:
- the session remains stable
- there is no delayed kick immediately after entry

---

## 4.5 Bootstrap item use / inventory smoke

Run this only with a disposable QA character and known seeded item-template data.

### 4.5.1 Consume a carried item (`ITEM_USE`)

- [ ] Put a known template-backed consumable in one carried inventory cell
- [ ] If packet logging is available, confirm the initial `ITEM_SET` for template-authored fixtures carries the owned item `flags` bits for `stackable` / `sell_count_per_gold` / `rare` / `unique` / `confirm_when_use` / `quest_use` / `quest_use_multiple` / `applicable`, carries the owned `anti_flags` bits for `anti_get`, transfer/job/sex/empire guards, projects authored socket/attribute display arrays and the template `highlight` byte, and leaves unowned bits zero
- [ ] Bind that carried cell to an item quickslot and also keep an unrelated skill/command quickslot that uses the same byte slot value if the client setup allows it
- [ ] Use the item once from inventory or the quickslot

Expected result:
- the client receives a `PLAYER_POINT_CHANGE` from the template-authored `use_effect`
- if more than one item remains in the stack, the carried cell refreshes with the decremented count, preserves authored socket/attribute display arrays in the `ITEM_UPDATE`, and both item and non-item quickslots for that still-occupied cell remain unchanged
- if the consumed stack reaches zero, the carried cell disappears and every item quickslot referencing that cell is cleared in deterministic quickslot-position order; unrelated skill/command quickslots remain
- locked carried stacks fail closed: no point change, item refresh, quickslot change, or placeholder chat is visible
- if a corrupt/disposable fixture has duplicate live items in the same carried cell, `ITEM_USE` fails closed with no point change, item refresh, quickslot change, placeholder chat, or persisted-state mutation
- templates marked `anti_stack`, `anti_get`, `anti_drop`, `anti_give`, or `anti_sell` also fail closed for direct consumable use: no point change, item refresh, quickslot change, or placeholder chat is visible
- templates with authored job, sex, empire, or `min_level` restrictions for the selected character fail closed the same way
- selected characters at the bootstrap zero-HP floor cannot consume carried items; the request fails closed before stack, quickslot, point, placeholder-chat, or persisted-state mutation
- a carried stack whose live count already exceeds its loaded template-authored `max_count` fails closed before stack, quickslot, point, placeholder-chat, or persisted-state mutation
- a consumable whose resolved template `max_count` cannot fit the current one-byte item refresh count range fails closed before stack, quickslot, point, placeholder-chat, or persisted-state mutation
- a consumable whose template-authored point delta would overflow the bootstrap signed 32-bit point value fails closed before stack, quickslot, point, placeholder-chat, or persisted-state mutation
- the placeholder `CHAT_TYPE_INFO` message uses the template-authored `use_effect.message`

### 4.5.2 Drag stack onto stack (`ITEM_USE_TO_ITEM`)

- [ ] Put two compatible stackable carried items with the same `vnum` and different item instance IDs into separate inventory cells
- [ ] Drag one stack onto the other stack

Expected result:
- compatible stacks consolidate up to the template-authored `max_count`
- authored stack `max_count` values above the current bootstrap client count range (`255`) are rejected at item-template load time, not accepted as runtime use-to-item behavior
- the consumed source cell disappears only on a full merge
- if the target has only partial room, both source and target counts refresh, and item/non-item quickslots bound to either still-occupied cell remain unchanged
- all item quickslots for a removed source cell are cleared in deterministic quickslot-position order on full merge, target item quickslots remain stable on full merge even when both source and target cells were quickslotted before the drag, and unrelated skill/command quickslots remain
- restricted or invalid states (`anti_stack`, transfer anti-flags, missing/non-stackable/malformed/mismatched templates, source/target `vnum` mismatches, locked source/target stacks, selected-character job/sex/empire/min-level restrictions, duplicate source/target item instance IDs, duplicate live occupancy of the source or target carried cell, already-full targets, source/target counts already above template `max_count`, or selected characters at the bootstrap zero-HP floor) fail closed with no visible mutation; for `anti_stack`, both carried stacks and item quickslots should remain unchanged
- a `min_level` restriction above the selected character's level or a selected character at the bootstrap zero-HP floor leaves both carried stacks and any source-cell item quickslot unchanged even when the source and target are otherwise compatible

### 4.5.3 Retarget a quickslot tuple (`QUICKSLOT_ADD`)

- [ ] Bind a carried inventory item cell to an item quickslot
- [ ] Bind the same carried item cell to a different item quickslot position
- [ ] If the client setup allows it, repeat the retarget with one skill binding and one command binding
- [ ] If the client setup allows it, keep an unrelated quickslot of a different type whose byte slot value matches the retargeted tuple

Expected result:
- the older same-type quickslot tuple is cleared before the new binding is added
- the new item/skill/command quickslot binding persists after reconnect
- unrelated quickslots of a different type with the same byte slot value remain unchanged
- if stale/reclaimed-socket QA tooling is available, a reclaimed old socket may still see its own quickslot refresh frames, but reconnecting / inspecting the fresh authoritative session shows no persisted quickslot change from that stale socket

### 4.5.4 Delete an occupied quickslot (`QUICKSLOT_DEL` / `QUICKSLOT_ADD` type none)

- [ ] Delete a quickslot position that currently contains an item, skill, or command binding
- [ ] If the client path emits `QUICKSLOT_ADD` with `slot.type = 0` for clearing, clear an occupied quickslot through that path too
- [ ] Delete a different quickslot bar position that is currently empty

Expected result:
- deleting the occupied position clears that binding and persists after reconnect
- a type-none `QUICKSLOT_ADD` clear returns the same visible delete behavior: the occupied binding is cleared, no new none binding remains, and reconnect shows the binding gone
- deleting the empty position fails closed: no quickslot refresh frame is visible, existing quickslot bindings remain, and reconnect shows no persisted change

### 4.5.5 Swap quickslots (`QUICKSLOT_SWAP`)

- [ ] Swap two occupied quickslot positions
- [ ] Swap one occupied quickslot position with an empty quickslot position
- [ ] Attempt to swap two empty quickslot positions

Expected result:
- occupied-to-occupied swaps exchange the bindings and persist after reconnect
- occupied-to-empty swaps move the binding to the empty target position and persist after reconnect
- empty-to-empty swaps fail closed: no quickslot refresh frame is visible, existing quickslot bindings remain, and reconnect shows no persisted change

### 4.5.6 Drop and pick up a carried item or gold (`ITEM_DROP` / `ITEM_PICKUP`)

- [ ] Drop a known template-backed carried item stack in a safe visible location
- [ ] Pick up the same temporary ground handle while still in range
- [ ] Drop a small amount of gold/elk through the client gold-drop path and, if QA tooling can vary the packed item position, repeat with a non-carried position while the gold amount is non-zero
- [ ] If possible in the QA fixture, repeat with a deliberately missing, malformed, mismatched, or ground-count-over-template-`max_count` authored item-template/state fixture for that `vnum`

Expected result:
- valid pickup removes the ground actor, refreshes the carried inventory slot or compatible stack according to the authored stack metadata, preserves existing item/non-item quickslots for a compatible merge target cell, shows the normal pickup notice, and does not produce a second/duplicate delayed ground-delete for the collector after the direct pickup response
- a non-zero gold/elk field follows the gold-drop path regardless of the packed item position: gold decreases, the carried inventory remains unchanged, and a gold ground marker appears; gold pickup restores gold only when the positive point-change total still fits the current bootstrap signed 32-bit carrier, otherwise it fails closed without removing the marker so it can be retried after the recipient state is valid
- loaded drop template metadata whose carried stack already exceeds the authored `max_count`, is transfer-guarded with `anti_get` / `anti_drop` / `anti_give` / `anti_sell`, is rejected by selected-character job/sex/empire/min-level restrictions, or is attempted while the selected character is at the bootstrap zero-HP floor fails closed: no ground actor, no carried-slot deletion/update, and no quickslot mutation is visible
- if a corrupt/disposable fixture reaches the shared-world ground-handle seam with stale equipment-slot metadata on an otherwise unequipped ground snapshot, registration fails closed and no temporary ground actor becomes available
- if a corrupt/disposable fixture has duplicate live items in the same carried cell, `ITEM_DROP` / `ITEM_DROP2` fails closed with no ground actor, no carried-slot deletion/update, no quickslot mutation, and no persisted-state mutation
- missing, malformed, mismatched, or ground-count-over-template-`max_count` authored pickup template metadata fails closed: no item pickup notice, no inventory mutation, and the ground handle remains available for a later valid retry
- fallback/no-template pickup fixtures whose ground stack count exceeds the current one-byte item refresh range (`255`) fail closed before item pickup notice, inventory mutation, or ground-handle removal
- loaded pickup template metadata marked `anti_get` / `anti_give` or restricted by the selected character's job/sex/min-level metadata also fails closed with the bootstrap inventory-full info message and leaves the ground handle available for a later valid retry
- selected characters at the bootstrap zero-HP floor cannot pick up visible ground items; `ITEM_PICKUP` fails closed with no item pickup notice, inventory/gold mutation, or ground-handle removal
- if a corrupt/disposable fixture already has the same non-zero item instance ID in carried inventory or equipment as the temporary ground item being picked up, pickup fails closed with the bootstrap inventory-full info message, no inventory mutation, and the ground handle remains available for a later valid retry

### 4.5.7 Drag inventory stack onto inventory stack (`ITEM_MOVE`)

- [ ] Put two carried stacks with the same `vnum` into separate inventory cells
- [ ] Confirm their loaded item template is stackable and not `anti_stack`, `anti_drop`, `anti_give`, or `anti_sell`, then drag one stack onto the other through normal inventory movement
- [ ] Repeat with an otherwise matching template that is marked `anti_stack`, `anti_drop`, `anti_give`, or `anti_sell`

Expected result:
- stackable, non-`anti_stack` items merge only up to the template-authored `max_count`
- if the target has only partial room, both source and target counts refresh, source and target item quickslots remain stable, and non-item quickslots with the same byte slot remain unchanged
- an exact counted or zero-count full-stack merge removes the source cell, refreshes the destination count, deletes every source item quickslot in deterministic quickslot-position order, leaves target item quickslots stable, and leaves unrelated skill/command quickslots unchanged
- `anti_stack` and non-stackable same-`vnum` full-stack drag requests do not merge; they use the full-stack carried swap path instead, refreshing both cells and retargeting item quickslots for the moved source identity while preserving unrelated skill/command quickslots
- `anti_drop`, `anti_give`, and `anti_sell` templates, same-`vnum` merge attempts with missing source-template metadata in an explicitly authored item-template snapshot, duplicate source/target cell occupancy fixtures, and corrupt fixtures where source and target cells carry the same item instance ID fail closed: no item counts change, no source cell disappears, no quickslot change is persisted, and no item refresh frames are visible

### 4.5.8 Equip a carried item (`ITEM_MOVE` to equipment cell)

- [ ] Put a known template-backed equipment item in a carried inventory cell
- [ ] Confirm the template's authored `equip_slot` matches the destination equipment cell, has no selected-character job/sex anti-flag, and is not guarded with `anti_stack`, `anti_drop`, `anti_give`, or `anti_sell`
- [ ] Drag the carried item into its matching equipment cell
- [ ] Repeat with the same item shape but a selected-character job/sex anti-flag that should reject the character
- [ ] Repeat with authored metadata whose `equip_slot` or `vnum` does not match the carried item/destination cell
- [ ] Repeat with otherwise matching equipment metadata guarded by one of `anti_stack`, `anti_drop`, `anti_give`, or `anti_sell`

Expected result:
- allowed equipment moves from carried inventory to the authored equipment cell, emits the self-only item refresh burst, deletes item quickslots bound to the cleared carried source cell, leaves unrelated skill/command quickslots with the same byte slot value unchanged, and applies the template-authored `equip_effect` point change only after the matching item is actually equipped in that authored cell
- selected-character anti-flagged, mismatched-`vnum`, mismatched-slot, or transfer-guarded equipment fails closed: no item refresh, no quickslot change, no point change, no carried/equipment mutation, and no persistence change
- corrupt/disposable fixtures that try to apply an equipment point effect without a matching valid equipped item in the authored equipment cell fail closed with no point mutation
- equipment whose template-authored `equip_effect` point delta would overflow the bootstrap signed 32-bit point value also fails closed before item, quickslot, point, or persistence mutation

### 4.5.9 Unequip a template-backed equipment item (`ITEM_MOVE` from equipment cell)

- [ ] Start with a known template-backed equipped item whose authored `equip_slot` matches the worn cell
- [ ] Confirm the item template has the current narrow `equip_effect` point metadata
- [ ] Drag the worn item back into an empty carried inventory cell
- [ ] Repeat with a corrupt/disposable fixture where the removal metadata does not match the just-removed item `vnum`

Expected result:
- allowed unequip emits the self-only equipment clear, carried-cell set, template-authored negative `PLAYER_POINT_CHANGE`, and appearance update
- the point-effect removal is backed by the just-removed item instance, so it still subtracts the authored delta after the item has moved out of the equipment slice
- mismatched or malformed removal metadata fails closed with no point change and no committed inventory/equipment/persistence mutation

### 4.5.10 Merchant buy/sell template restrictions (`SHOP BUY` / `SHOP SELL2`)

- [ ] Open a known bootstrap merchant window with a disposable QA character
- [ ] Attempt to buy a catalog item whose authored template requires a higher `min_level` than the selected character has
- [ ] Attempt to sell a carried item whose authored template requires a higher `min_level` than the selected character has
- [ ] Attempt to sell carried items whose authored templates are marked `anti_get`, `anti_drop`, `anti_give`, or `anti_sell`

Expected result:
- restricted packet paths fail with the current merchant invalid-position companion and no inventory, item quickslot, gold, or persisted account mutation is visible
- adjacent allowed merchant buy/sell cases still use the template-authored price/sell-credit behavior

---

## 5. Single-client movement

### 5.1 Basic movement

- [ ] Walk a short distance
- [ ] Walk again in a different direction
- [ ] Stop moving and wait 5 seconds

Expected result:
- movement works
- the client remains connected
- there is no severe rubber-band that blocks testing

### 5.2 Repeat movement after idle

- [ ] Wait 15 seconds
- [ ] Move again

Expected result:
- movement still works after idle
- there is no silent session death

### 5.3 Reconnect persistence smoke test

- [ ] Exit the client cleanly
- [ ] Reopen the client
- [ ] Login and re-enter with the same character

Expected result:
- the character still exists
- login, selection, and enter-game still work after reconnect

### 5.4 Bootstrap NPC interaction smoke

Run this only when the target build has authored QA NPC content loaded nearby.

If the lab currently has no such content, either:
- import/adapt `docs/examples/bootstrap-npc-service-bundle.json` through `/local/content-bundle`, or
- record this subsection as **N/A** instead of treating the absence of authored NPCs as a gameplay regression.

#### 5.4.1 Talk / info / merchant interactions

- [ ] Approach a visible authored QA NPC with `info`, `talk`, or merchant `shop_preview`
- [ ] For `info` / `talk`, interact once and wait for the self-only response
- [ ] For a merchant actor, interact once and confirm a merchant window opens instead of only a chat preview
- [ ] If the authored QA merchant catalog exposes an affordable test item, attempt one packet `SHOP BUY` from the open window and confirm the success path returns self-only inventory refreshes without an extra merchant-family `GC::SHOP OK` or the older placeholder info chat; newly occupied slots should use `ITEM_SET`, while merges into already-known carried stacks should use `ITEM_UPDATE`
- [ ] If the QA setup allows it, sell one carried item stack from the open merchant window and confirm the success path returns a carried-slot refresh (`ITEM_DEL` for whole-stack removal or `ITEM_UPDATE` for partial-stack decrement) followed by `PLAYER_POINT_CHANGE(POINT_GOLD)`, with no extra bare merchant-family `GC::SHOP OK`
- [ ] If a corrupt/disposable fixture has duplicate live items in the same carried cell, confirm merchant sell-back from that cell fails closed with no gold or inventory mutation
- [ ] If the bought item is stackable and the character already carries the same `vnum`, confirm the count can increase on that existing stack instead of always creating a new slot
- [ ] If the QA setup allows it, fill the carried inventory, leave two compatible carried stacks nearly full, buy a stackable merchant entry whose count exactly matches their combined remaining room, and confirm both existing stacks fill without needing any fresh slot
- [ ] If the QA setup allows it, leave one compatible carried stack nearly full, buy a stackable merchant entry whose count overflows that stack, and confirm the existing stack fills first while the remainder lands in a fresh carried slot
- [ ] If the QA setup allows it, leave several compatible carried stacks nearly full plus one free carried slot, buy a stackable merchant entry whose count exceeds the combined remaining room in those existing stacks, and confirm the existing stacks fill first in carried-slot order while only the final remainder lands in the fresh slot
- [ ] If the QA setup allows it, force one insufficient-gold merchant buy from the open merchant window and confirm the client now follows the merchant-family insufficient-money error path instead of the older placeholder info chat
- [ ] If the QA setup allows it, force one no-placement merchant buy from the open merchant window and confirm the client now follows the merchant-family inventory-full error path instead of the older placeholder info chat
- [ ] If the QA setup allows it, keep a merchant window open, send one packet `SHOP BUY` for an authored slot that does not exist in that bound catalog snapshot and confirm the client receives one merchant-family invalid-position response without any gold or inventory mutation
- [ ] If the QA setup allows it, use the loopback static-actor or interaction-definition update endpoints to invalidate the currently open merchant actor/catalog and confirm the next packet `SHOP BUY` auto-closes that stale merchant window with one merchant-family `GC::SHOP END` without changing gold or inventory
- [ ] If the QA setup allows it, keep a merchant window open, send one position-only `MOVE` far enough that the bound merchant actor leaves the current interaction/visibility gate, and confirm the client still first receives the normal self `MOVE_ACK` and then one queued merchant-family `GC::SHOP END`; then confirm a later `SHOP END` or `SHOP BUY` fails closed until the merchant is opened again
- [ ] If the QA setup allows it, reopen the merchant, send one position-only `SYNC_POSITION` that moves the owner out of that same interaction/visibility gate, and confirm the client still first receives the normal self `SYNC_POSITION_ACK` and then one queued merchant-family `GC::SHOP END`; then confirm a later `SHOP END` or `SHOP BUY` fails closed until the merchant is opened again
- [ ] If the QA setup allows it, keep a merchant window open, trigger one successful warp or exact-position transfer, and confirm the client first receives one merchant-family `GC::SHOP END` before the normal self transfer rebootstrap burst; then confirm a later `SHOP END` or `SHOP BUY` on the destination side fails closed until the merchant is opened again
- [ ] If the QA setup allows it, keep a merchant window open, let a content-loaded practice mob's delayed retaliation beat drop the selected character to `0` HP, and confirm the client receives `PLAYER_POINT_CHANGE(value=0)`, `DEAD(owner_vid)`, `TARGET(0, 0)`, then one merchant-family `GC::SHOP END`; then confirm later `SHOP BUY` or `SHOP END` attempts fail closed until broader revive/reopen semantics are owned
- [ ] If the QA setup allows it, keep a merchant window open, send `/phase_select`, and confirm the client first receives one merchant-family `GC::SHOP END` before the select-phase transition frame; then confirm the next selected character starts without any stale merchant context until the merchant is opened again
- [ ] If the QA setup allows it, keep a merchant window open, send `/quit`, and confirm the client first receives one merchant-family `GC::SHOP END` before the existing self command-chat `quit` delivery; then confirm the session has no usable stale merchant context while it waits for disconnect
- [ ] If the QA setup allows it, keep a merchant window open, send `/logout`, and confirm the client first receives one merchant-family `GC::SHOP END` before the close-phase transition frame; then confirm the socket leaves the shared world without any stale merchant context surviving
- [ ] Re-interact immediately once to confirm repeated spam is suppressed or remains stable within the current cooldown contract

Expected result:
- `info` and `talk` still return deterministic self-only text
- merchant interaction opens a stable bootstrap `GC::SHOP START` window
- a bootstrap `SHOP BUY` request can debit gold and grant the authored item without disconnecting the client, and successful packet buys now return self-only inventory refreshes (`ITEM_SET` for newly occupied slots, `ITEM_UPDATE` for existing-stack count refreshes) without an extra merchant-family `GC::SHOP OK`
- a bootstrap `SHOP SELL` / `SELL2` request can credit gold and remove or decrement the authored carried item without disconnecting the client, and successful packet sells now return only the carried-slot refresh plus `PLAYER_POINT_CHANGE(POINT_GOLD)` without an extra merchant-family `GC::SHOP OK`
- merchant sell-back fails closed if the live carried inventory contains duplicate authoritative entries for the same cell, preserving gold and inventory rather than deleting an arbitrary duplicate
- when the authored item is stackable and a compatible carried stack already exists, the buy can refresh that same slot with the increased count
- when several compatible carried stacks together can absorb the full authored count, the buy can fill those existing stacks in carried-slot order without needing a fresh slot
- when several compatible carried stacks together cannot absorb the full authored count but one free carried slot exists, the buy can fill those existing stacks first and place only the final remainder into one fresh carried slot
- insufficient-gold, no-placement, and unknown-slot merchant failures preserve state and now surface the merchant-family error path from the open window instead of silently failing or falling back to the older placeholder info chat on packet `SHOP BUY`
- if a still-open merchant window becomes stale because the live actor or authored `shop_preview` definition changed underneath it, the next packet `SHOP BUY` auto-closes that stale merchant window with self-only `GC::SHOP END`, clears the active merchant context, and still does not mutate gold or inventory
- if a position-only `MOVE` or `SYNC_POSITION` leaves the bound merchant actor outside the current interaction/visibility gate while that merchant window is still open, the client still keeps the normal self movement acknowledgement first and then sees one queued self-only `GC::SHOP END`, with the active merchant context already cleared before any later `SHOP END` or `SHOP BUY`
- if a successful warp or exact-position transfer begins while that merchant window is still open, the client now sees one self-only `GC::SHOP END` before the normal transfer rebootstrap burst, and the destination-side merchant context stays cleared until the player opens a fresh merchant window again
- if a content-loaded practice mob's delayed retaliation beat reaches the selected character's `0`-HP floor while that merchant window is still open, the client now sees the existing floor sequence followed by one self-only `GC::SHOP END`, and later `SHOP BUY` / `SHOP END` attempts fail closed until broader revive or merchant-reopen semantics are owned
- if same-socket `/phase_select` begins while that merchant window is still open, the client now sees one self-only `GC::SHOP END` before the select-phase transition frame, and the next selected character starts without any stale merchant context until the merchant is opened again
- if same-socket `/quit` or `/logout` begins while that merchant window is still open, the client now sees one self-only `GC::SHOP END` before the existing command/close-phase teardown frame, and the socket keeps no usable stale merchant context afterward
- repeated interaction does not disconnect the client

Important note:
- this smoke step validates only the current bootstrap open / buy / sell / close merchant slice
- broader merchant update choreography, stock semantics, and richer NPC UI are still ahead
- local fallback QA through `/shop_buy <slot>` now mirrors the same merchant-family `GC::SHOP NOT_ENOUGH_MONEY` / `GC::SHOP INVENTORY_FULL` / `GC::SHOP INVALID_POS` failure surfaces as the owned packet path for those same authoritative results instead of keeping a silent unknown-slot branch; its local debug success surface may still append the older bare `GC::SHOP OK` after item refreshes until that harness is tightened separately

#### 5.4.2 Warp interaction

- [ ] Approach a visible authored QA warp NPC
- [ ] Interact once
- [ ] Confirm any authored informational text appears first if configured
- [ ] Confirm the client re-enters the world at the authored destination and remains connected

Expected result:
- the warp actor relocates the character through the current transfer/rebootstrap flow
- the client remains stable after the warp
- no merchant window, quest window, or inventory mutation appears

### 5.5 Bootstrap item drop / pickup smoke

Run this when two QA clients can enter the same visible bootstrap world.

- [ ] On client A, drop one ordinary carried item stack using the normal client inventory drop path
- [ ] If QA tooling can force the newer counted-drop path with count `0`, confirm it behaves as a whole-stack drop: the carried cell disappears, the ground item appears, item quickslots for that cell clear, and unrelated skill/command quickslots remain
- [ ] Confirm client A sees a ground item plus ownership label
- [ ] Confirm visible client B sees the same ground item plus ownership label
- [ ] If QA data allows it, attempt to drop a locked or malformed/guarded carried test item and confirm the inventory, quickslots, and visible ground handles remain unchanged
- [ ] If QA data allows it, drop and reclaim an `anti_stack` authored item while client A already carries another compatible stack; confirm pickup restores the dropped stack into a fresh carried slot instead of merging it into the existing stack
- [ ] On client B, pick up client A's still-owned ground item

Expected result:
- client B receives a ground delete plus a party-shaped pickup notice naming client A
- client A receives the ground delete plus a party-shaped pickup notice naming client B
- the item is delivered back to client A's owned account/runtime rather than being added to client B
- client A can immediately use another normal item action against the delivered/updated carried slot, proving the live owner runtime was refreshed and not only the account file
- if the dropped item's loaded template becomes `anti_give` or job/sex-restricted for client A before client B picks it up, client B sees the bootstrap inventory-full info rejection, neither inventory mutates, no owner notice is queued, and the ground handle remains available for a later valid retry
- `anti_drop` / `anti_give` / `anti_sell` template-flagged items fail closed when dropped through the normal client inventory path, show the bootstrap "You cannot drop this item." info rejection, and leave carried inventory plus quickslots unchanged
- this remains a bootstrap party approximation; real party membership, ownership timers, and public ownership release are still not owned

### 5.6 Bootstrap equip / unequip appearance refresh

Run this only when the QA character has one wearable `body`, `weapon`, or `head` item plus at least one free carried slot.

- [ ] Use the current QA slash seam to equip a supported wearable item
- [ ] Confirm the item leaves the carried inventory and appears in the expected equipment cell
- [ ] Confirm the selected character's visible body/weapon/head appearance refreshes immediately without reconnecting
- [ ] Use the current QA slash seam to unequip that same item back into a carried slot
- [ ] Confirm the item returns to carried inventory and the selected character's visible body/weapon/head appearance reverts immediately

Expected result:
- successful equip/unequip still returns self-only item-slot frames in the current slice
- successful equip/unequip now appends one visible-character refresh after the item-slot frames
- the client remains connected, inventory/equipment state stays consistent, and already-visible stable peers can refresh the same appearance without reconnecting

Important note:
- broader visibility-changing appearance fanout beyond the currently frozen late-join, reconnect-driven, transfer-driven, duplicate-live retry-`ENTERGAME`, and radius-AOI move-into-range branches is still out of scope for this slice

#### 5.6.1 Template-backed equip point refresh

- [ ] Seed or confirm one wearable item whose template carries `equip_effect` metadata (current bootstrap QA seed: `12200`, weapon)
- [ ] Record the current selected-character point value used by the seeded template (`Points[1]` in the current bootstrap slice)
- [ ] Use `/equip_item <from> weapon` on that item
- [ ] Confirm one self-only `PLAYER_POINT_CHANGE` arrives after the item-slot frames and before the self-only `CHARACTER_UPDATE`
- [ ] Confirm the point refresh uses the template-authored delta (`+10` for the current seeded practice blade) and the updated value persists after reconnect
- [ ] Use `/unequip_item weapon <to>` on that same item
- [ ] Confirm one self-only `PLAYER_POINT_CHANGE` again arrives after the item-slot frames and before the self-only `CHARACTER_UPDATE`
- [ ] Confirm the unequip point refresh uses the inverse template-authored delta (`-10` for the current seeded practice blade) and restores the previous selected-character point value after reconnect

Expected result:
- equip/unequip point refresh is driven by item-template `equip_effect` metadata instead of a runtime-only hardcoded item switch
- the current seeded practice blade still resolves to `vnum = 12200`, `type = 1`, and `amount = +/-10` on equip/unequip
- the response burst stays self-only and ordered as `ITEM_DEL` + `ITEM_SET` + optional `PLAYER_POINT_CHANGE` + `CHARACTER_UPDATE`
- if a point-bearing wearable is forced through the wrong slash seam slot, the item mutation can still stay appearance-only in the current bootstrap slice but the template-backed `PLAYER_POINT_CHANGE` must not fire
- if the selected character is restricted by the wearable template's job/sex anti flags, or if the wearable template is temporarily authored with transfer/pickup-style anti flags such as `anti_get`, packet and slash equip fail closed with no item-slot, point, or appearance mutation
- already-visible peers still only receive the projected appearance refresh; no peer-visible point stream is frozen by this slice

### 5.7 Template-backed consumable item use

- [ ] Seed or confirm one carried consumable whose item template has a `use_effect` payload (current bootstrap QA seed: `27001`)
- [ ] Use the carried consumable through the current client item-use path or a carried-slot `ITEM_USE` packet (the older `/use_item <slot>` harness still remains valid)
- [ ] Confirm one self-only `PLAYER_POINT_CHANGE` arrives before the item-slot refresh
- [ ] Confirm the consumed slot decrements by exactly one stack item or clears entirely if it was the last item
- [ ] Confirm one self-only `CHAT_TYPE_INFO` placeholder effect arrives using the template-authored message
- [ ] Reconnect and confirm the consumed stack and updated point value persisted
- [ ] If QA data allows it, repeat with the selected character restricted by the consumable template's authored job/sex anti flags; confirm no item, point, quickslot, or info-chat mutation is visible

Expected result:
- the current carried-slot client item-use path resolves through item-template metadata rather than a runtime-only hardcoded consumable switch
- the current seeded bootstrap template still yields `type = 1`, `amount = 50`, `value = updated Points[1]`, and `consume:27001:+50`
- the response burst stays self-only and ordered as `PLAYER_POINT_CHANGE` then `ITEM_SET`/`ITEM_DEL` then `CHAT_TYPE_INFO`
- selected-character job/sex anti-flag templates fail closed before the consumable point/effect path runs
- the selected-character snapshot persists atomically through the current save/rollback boundary

### 5.7.1 Drag-to-item stack consolidation

Run this when the selected QA character has two carried stacks with the same `vnum` and a stackable template.

- [ ] Drag one carried stack onto another compatible carried stack
- [ ] Confirm compatible stacks consolidate without triggering the normal consumable point/effect path
- [ ] Confirm a full-source merge removes any item quickslot that referenced the removed source cell, while skill/command quickslots with the same slot byte stay unchanged
- [ ] Relog after a full-source merge and confirm the merged inventory plus item-quickslot cleanup persisted
- [ ] Confirm a partial merge refreshes both changed counts and keeps the source item quickslot
- [ ] If QA data allows it, repeat with template metadata using each of `anti_stack`, `anti_drop`, `anti_give`, and `anti_sell`; confirm every request fails closed with no item/quickslot mutation

Expected result:
- the current `ITEM_USE_TO_ITEM` path only owns stack-on-stack consolidation for carried inventory positions
- non-stackable templates, locked items, incompatible stacks, full/over-max stacks, zero-count stacks, missing/invalid templates, and anti-transfer templates fail closed with no fallback consumable effect

### 5.7.2 Quickslot bootstrap replay

Run this only when the selected QA character has persisted quickslots in its bootstrap account snapshot.

- [ ] Enter the world with that character
- [ ] Confirm the client receives/restores the expected quickslot bar entries after world entry
- [ ] Reconnect and enter again with the same character
- [ ] Confirm the same quickslot entries are replayed without manual reconfiguration

Expected result:
- persisted selected-character quickslots are replayed as self-only `QUICKSLOT_ADD` bootstrap frames after the selected-character presence/state burst
- quickslot entries are stable across auth/login-ticket handoff and reconnect
- client-authored quickslot add/delete/swap edits return the matching self-only quickslot refresh frame, persist to the selected-character snapshot, and survive reconnect
- a same-position quickslot swap is treated as a no-op rejection: no quickslot refresh frame is emitted and no persisted quickslot mutation occurs
- item-type quickslot add requests that point at an empty carried inventory cell fail closed with no frame and no persisted quickslot mutation
- skill quickslot add requests outside slots `0..199` and command quickslot add requests outside slots `0..59` fail closed with no frame and no persisted quickslot mutation
- automatic item-mutation quickslot synchronization is now owned for the current bootstrap paths: item moves retarget/delete item quickslots, while last-stack item consume or full-source drag-to-item consolidation deletes item quickslots that referenced the removed carried cell and leaves skill/command quickslots unchanged

### 5.7.3 Template-backed item anti-flag display

Run this only when the QA character can enter with one carried or equipped item whose template carries currently owned anti-flag metadata (`anti_drop`, `anti_give`, `anti_sell`, `anti_stack`, job flags, or sex flags).

- [ ] Enter `GAME` and observe the selected-character item bootstrap
- [ ] Confirm the affected `ITEM_SET` frame carries the matching `anti_flags` bits instead of `0`
- [ ] Mutate the item through an owned full-slot `ITEM_MOVE`, equip, unequip, merchant buy, or accepted pickup path that emits `ITEM_SET`
- [ ] Confirm the refreshed `ITEM_SET` still carries anti-flags from the item template

Expected result:
- client-visible occupied-slot `ITEM_SET` frames are backed by authored template anti-flag metadata for the currently owned flag subset
- unowned anti-flag bits remain zero until the matching template metadata/runtime behavior is owned

### 5.7.4 Drag-to-item carried-stack merge

Run this only when the QA character has two compatible carried stacks for the same stackable template (current bootstrap seed: `27001`).

- [ ] Send one `ITEM_USE_TO_ITEM` / drag-to-item request from a source carried stack into a compatible target carried stack
- [ ] Confirm the source stack shrinks or clears and the target stack grows by the moved count
- [ ] If the source stack clears and an item quickslot references that source slot, confirm the quickslot is deleted instead of being retargeted onto the destination stack
- [ ] Confirm no normal consumable `PLAYER_POINT_CHANGE` or `CHAT_TYPE_INFO` effect placeholder fires from this drag-to-item path
- [ ] If the QA setup can temporarily author an otherwise valid stackable template with `max_count > 255`, repeat the drag-to-item request and confirm it fails closed without item refresh frames or inventory mutation because the current owned item refresh packets expose count as one byte
- [ ] Repeat against an incompatible target stack
- [ ] Confirm incompatible, empty-source, empty-target, and same-cell requests fail closed: no item refresh frames, no point/effect placeholder, no quickslot changes, and no inventory mutation
- [ ] Reconnect and confirm the accepted merge persisted while the rejected request did not

Expected result:
- `ITEM_USE_TO_ITEM` currently owns only carried same-`vnum` stack-on-stack consolidation
- the path reuses the existing self-only carried inventory refresh family and selected-character persistence boundary
- broader drag-to-item behavior such as sockets, enchanting, refines, quest items, or equipment effects remains out of scope

### 5.8 Counted carried-slot `ITEM_MOVE` stack bounds

Run this only when the QA character has two compatible carried stacks for the same stackable template (current bootstrap seed: `27001`) and the destination stack can be brought near that template's `max_count`.

- [ ] Send one counted carried-slot `ITEM_MOVE` from a compatible source stack into a destination stack where `destination_count + count == template.max_count`
- [ ] Confirm the move succeeds with source/destination count refreshes, decrements the source stack, grows the destination stack, and persists after reconnect
- [ ] Repeat with a count that would make `destination_count + count > template.max_count`
- [ ] Confirm the move fails closed: no item refresh frames, no source decrement, no destination growth, and no persisted inventory change

Expected result:
- packet-originated compatible partial merges respect item-template `max_count`, not only the packet count or storage integer bounds
- failure preserves live and persisted carried-slot state atomically

### 5.9 Merchant sell-back gold refresh

Run this only when the target build has a visible authored `shop_preview` merchant and a disposable carried item stack with a sellable item template.

- [ ] Open the merchant window through the visible shop actor
- [ ] Sell one whole carried stack through the client merchant `SELL` path
- [ ] Confirm the carried slot clears and the selected character's displayed gold increases without requiring reconnect
- [ ] Repeat with a multi-count carried stack through the `SELL2` path for a partial count
- [ ] Confirm the already-known carried slot refreshes through the lighter `ITEM_UPDATE` count path, remains with the reduced count, and the selected character's displayed gold increases immediately
- [ ] Reconnect and confirm the updated carried inventory and gold persisted

Expected result:
- accepted merchant sell-back responses are ordered as whole-stack `ITEM_DEL` or partial-stack `ITEM_UPDATE`, then self-only `PLAYER_POINT_CHANGE(POINT_GOLD)`, with no extra bare `GC::SHOP OK`
- invalid, anti-sell, equipped, runtime-locked, explicit zero-count `SELL2`, or zero-HP owner attempts fail closed and leave both live and persisted inventory/gold unchanged
- after practice-mob retaliation reaches the player's current zero-HP floor, both whole-stack `SELL` and partial-stack `SELL2` attempts emit no sell success frames and do not delete carried-item quickslots
- richer `GC::SHOP UPDATE_ITEM` / `UPDATE_PRICE` merchant-window choreography remains out of scope for this bootstrap sell-back smoke

### 5.10 Training dummy repeated-hit smoke

Run this only when the target build has a visible authored `training_dummy` nearby.

- [ ] Approach the dummy until it is clearly within the current bootstrap target/attack band
- [ ] Select the dummy once and confirm the client shows it as the active target
- [ ] Perform one accepted normal attack
- [ ] Confirm the selected target remains stable and the dummy HP display moves down from full by one deterministic bootstrap step
- [ ] Perform at least one more accepted normal attack
- [ ] Confirm the selected target HP display steps down again instead of bouncing back to full on every hit
- [ ] If practical, re-select the same still-visible dummy and confirm the current HP display stays at the already-mutated runtime value instead of silently resetting because of the re-selection itself
- [ ] Confirm the character's own inventory, equipment, and visible player stats do not unexpectedly change because of dummy hits alone

Expected result:
- repeated accepted hits against the same selected dummy decrement HP in deterministic bootstrap-sized steps
- the client-visible feedback is still the narrow self-only selected-target refresh surface, not a broader peer/combat fanout contract yet
- dummy hits do not spend items, grant items, mutate equipment, or alter saved player progression/state by themselves

Important note:
- the current contract says dummy HP is shared-world runtime state only
- do **not** treat the absence of account-style persistence for dummy HP as a regression in this slice
- reconnect/transfer/reset behavior for dummy HP should be recorded if observed, but it is still a later contract than this repeated-hit smoke step

### 5.11 Practice-mob reward smoke

Run this when the target build has authored QA `spawn_groups` practice-mob content loaded with a non-zero bootstrap death-reward descriptor. The repository example bundle at `docs/examples/bootstrap-npc-service-bundle.json` now includes `practice.qa_reward_mob` with EXP, gold, and one fixed drop-vnum reward descriptor for this smoke.

- [ ] Approach and select the visible practice mob
- [ ] Land accepted normal attacks until the mob reaches the owned zero-HP death edge
- [ ] Confirm the killing hit still shows the death + target-clear choreography before any reward feedback
- [ ] If the QA mob grants EXP, confirm one self-only `PLAYER_POINT_CHANGE(POINT_EXP)` arrives after death/clear and that reconnect keeps the updated EXP value
- [ ] If the QA mob grants gold, confirm one self-only `PLAYER_POINT_CHANGE(POINT_GOLD)` arrives after death/clear and that reconnect keeps the updated gold value
- [ ] If the QA mob drops items, confirm one self-visible `GROUND_ADD` + `OWNERSHIP` pair appears per configured drop, at the killer's current position
- [ ] Pick up one reward drop and confirm the normal bootstrap pickup path removes the ground item, adds it to carried inventory, persists it, and rejects a replayed pickup
- [ ] With a second living visible client watching, disconnect the reward-drop owner before pickup and confirm the watcher sees deterministic ground-delete cleanup for the owner's still-owned reward drops
- [ ] If a second watcher is already at the bootstrap `0`-HP floor, confirm that dead watcher receives neither the owner leave delete nor the owned-ground delete noise
- [ ] If using a debug/fixture harness that deliberately pre-seeds one colliding reward-drop `VID`, confirm the colliding drop is omitted while the accepted death edge, scalar rewards, and any non-colliding drop entries still succeed

Expected result:
- reward frames are ordered after `DEAD` and `TARGET(0, 0)`
- scalar EXP/gold rewards persist to the selected character before their point-change frame is emitted
- item drops are runtime ground items first; they do not mutate inventory until an explicit pickup succeeds
- invalid or unsupported reward descriptors preserve the accepted death/clear edge while omitting reward mutation and reward frames
- a live ground-drop `VID` collision suppresses only that colliding drop; it does not roll back death/clear, scalar rewards, or independent non-colliding drops

Important note:
- default `training_dummy` / `practice_mob` content remains rewardless unless the QA setup deliberately overrides or authors a non-zero descriptor
- level progression, party distribution, loot ownership expiry, quest credit, and corpse gameplay are still out of scope for this bootstrap smoke

### 5.12 Practice-mob retaliation death and restart-here smoke

Run this when the target build has authored QA `spawn_groups` practice-mob content loaded with the current bootstrap retaliation behavior.

- [ ] Approach and select the visible practice mob
- [ ] Land accepted normal attacks and wait through delayed retaliation beats until the player reaches the owned zero-HP floor
- [ ] Confirm the owner receives the final `PLAYER_POINT_CHANGE` to `0`, then `DEAD(owner_vid)`, then `TARGET(0, 0)`
- [ ] If a merchant window is open when the immediate or delayed retaliation beat reaches `0` HP, confirm one self-only `GC::SHOP END` follows the death/clear sequence and later `SHOP END` / `SHOP BUY` attempts fail closed until a fresh merchant interaction opens a new window
- [ ] Try a fresh target or attack while still at `0` HP
- [ ] Confirm the attempt fails closed with no new combat-visible frames
- [ ] Try one carried-inventory `ITEM_MOVE` drag while still at `0` HP
- [ ] Confirm the move fails closed: no item cells change and no item refresh frames are visible
- [ ] Issue `/restart_here` on the same socket
- [ ] Confirm the character rebuilds in place with the ordinary self bootstrap burst and restored persisted HP
- [ ] Confirm a stale attack still fails until the practice mob is selected again
- [ ] Re-select the still-live practice mob and confirm its HP remains at the current runtime-owned value instead of resetting because of `/restart_here`

### 5.12.1 Practice-mob pending-retaliation cleanup on mob death

Run this when the target build has authored QA `spawn_groups` practice-mob content loaded and enough selected-player HP to kill the mob before player death.

- [ ] Select the visible practice mob and land one accepted hit to arm the delayed server-origin retaliation cadence
- [ ] Continue accepted hits, respecting the owned normal-attack cadence, until the practice mob reaches the zero-HP death edge
- [ ] Confirm the killing hit shows only the mob `DEAD(target_vid)` plus `TARGET(0, 0)` clear from the combat lifecycle, not an extra owner-side retaliation point-change
- [ ] Wait less than the owned respawn delay and confirm no stale delayed retaliation beat arrives after mob death
- [ ] Wait until the owned respawn delay expires and confirm the ordinary mob rebuild burst (`CHARACTER_DEL` + add/info/update)
- [ ] Confirm no stale delayed retaliation beat arrives immediately after that respawn rebuild unless the mob is freshly reselected and hit again

Expected result:
- owner-side retaliation death uses `PLAYER_POINT_CHANGE(value=0)` -> `DEAD(owner_vid)` -> `TARGET(0, 0)`
- `/restart_here` is accepted only after the zero-HP floor and keeps the session in `GAME`
- player HP is rebuilt from persisted state, while a still-live practice mob keeps its runtime-owned HP and requires fresh target acquisition
- post-floor `ITEM_MOVE` is silent and non-mutating until a restart/recovery seam is used
- mob death cancels pending delayed retaliation and respawn does not resurrect stale retaliation work without fresh target acquisition

### 5.13 Practice-mob retaliation restart-town smoke

Run this when the QA character can safely exercise the bootstrap town-return recovery after the retaliation-owned zero-HP floor.

- [ ] Drive the selected player to `0` HP through the current practice-mob retaliation loop
- [ ] Issue `/restart_town` on the same socket
- [ ] Confirm the character stays in `GAME` and receives the ordinary self bootstrap burst at the owned empire town-return position
- [ ] Confirm later movement/interaction works from the town-return position after recovery
- [ ] Reconnect and confirm the town-return position persisted, while the retaliation HP loss itself did not persist

Expected result:
- `/restart_town` is accepted only after the zero-HP floor
- the selected player rebuilds from persisted state and moves to the currently owned empire create-position fallback
- the recovery does not invent a separate revive packet or claim final map-specific death-return rules

---

## 6. Two-client shared-world checks

Run this only when two real clients are available.
Prefer two disposable QA characters.

### 6.1 Dual login

- [ ] Connect client A
- [ ] Connect client B
- [ ] Enter the world on both

Expected result:
- both sessions stay connected
- one client does not kick the other during entry

### 6.2 Peer visibility

- [ ] Put both characters in the same bootstrap map
- [ ] Confirm A can see B
- [ ] Confirm B can see A

Expected result:
- mutual visibility works
- appearance/disappearance is sane enough for the current bootstrap scope

### 6.3 Peer movement replication

- [ ] Move character A while watching from B
- [ ] Move character B while watching from A

Expected result:
- movement replicates between visible peers
- there is no obvious one-way visibility bug

### 6.4 Local talking chat

- [ ] Send a normal local chat message from A
- [ ] Confirm B receives it
- [ ] Send one from B
- [ ] Confirm A receives it

Expected result:
- local chat works between visible peers in the same bootstrap scope

### 6.5 Whisper by exact name

- [ ] Whisper from A to B by exact character name
- [ ] Confirm B receives it
- [ ] Whisper to a non-existing name

Expected result:
- exact-name whisper delivery works
- an unknown target returns a clean not-exist behavior to the sender

### 6.6 Disconnect cleanup

- [ ] Close client B cleanly while A stays in-world

Expected result:
- A does not crash
- B disappears from A cleanly within the current bootstrap behavior

### 6.7 Peer equip / unequip appearance refresh

- [ ] Put both characters in the same bootstrap map and keep them mutually visible
- [ ] Equip a supported `body`, `weapon`, or `head` item on client A
- [ ] Confirm client B sees A's visible body/weapon/head appearance refresh immediately
- [ ] Unequip the same item on client A
- [ ] Confirm client B sees A's appearance revert immediately

Expected result:
- the mutating client still gets only the normal self item-slot frames plus its self refresh
- already-visible stable peers now also receive one visible-character refresh carrying the same projected appearance
- no reconnect, duplicate peer insert, or forced visibility reset is required

### 6.8 Late join after peer appearance mutation

- [ ] Connect client A first and enter the world alone
- [ ] Equip or unequip a supported `body`, `weapon`, or `head` item on client A
- [ ] Connect client B afterward and enter the same bootstrap map
- [ ] Confirm client B immediately sees A with the latest visible body/weapon/head appearance in the normal peer burst

Expected result:
- no extra reconnect or manual refresh is needed on client A
- client B sees the same projected appearance that already-visible peers would see
- the peer bootstrap burst stays the normal `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` sequence

### 6.9 Radius-AOI move-into-range after peer appearance mutation

- [ ] Start `gamed` with radius AOI enabled for QA
- [ ] Put client A and client B on the same effective map but outside the configured visible radius
- [ ] Equip or unequip a supported `body`, `weapon`, or `head` item on client A while B stays out of range
- [ ] Move client B into A's visible range
- [ ] Confirm client B sees A with the latest visible body/weapon/head appearance in the normal peer-entry burst

Expected result:
- client A still mutates appearance through the normal equip/unequip path while B remains out of range
- once B crosses into range, the move-driven peer-entry burst carries A's latest projected appearance in `CHAR_ADDITIONAL_INFO` and `CHARACTER_UPDATE`
- no reconnect or manual refresh is needed after the move-driven visibility rebuild

### 6.10 Transfer-driven peer appearance after runtime mutation

- [ ] Put client A and client B on different effective bootstrap maps
- [ ] Equip or unequip a supported `body`, `weapon`, or `head` item on client A while they remain on separate maps
- [ ] Trigger a supported transfer/warp path that makes client A newly visible to client B
- [ ] Confirm client B sees A with the latest visible body/weapon/head appearance in the normal peer-entry burst after the transfer

Expected result:
- client A keeps the latest projected appearance through the transfer
- once the transfer makes A newly visible to B, the destination peer-entry burst carries that latest projected appearance in `CHAR_ADDITIONAL_INFO` and `CHARACTER_UPDATE`
- no extra reconnect or manual refresh is needed after the transfer-driven visibility rebuild

### 6.11 Reconnect-driven peer appearance after runtime mutation

- [ ] Put client A and client B in the same bootstrap visibility scope and keep them mutually visible
- [ ] Equip or unequip a supported `body`, `weapon`, or `head` item on client A; when testing packet `ITEM_MOVE` equip, confirm the source item's loaded template authors the requested `equip_slot` because mismatched template metadata now fails closed without item/point mutation
- [ ] Disconnect client A while client B stays in-world
- [ ] Reconnect client A through a fresh login/select/enter-game flow
- [ ] Confirm client B sees A re-enter with the latest visible body/weapon/head appearance in the normal peer-entry burst

Expected result:
- valid template-backed equips/unequips mutate self inventory/equipment and visible appearance; mismatched template-backed equip attempts emit no frames and leave live/persisted state unchanged
- client B first sees A disappear cleanly on disconnect
- the reconnect peer-entry burst carries A's latest projected appearance in `CHAR_ADDITIONAL_INFO` and `CHARACTER_UPDATE`
- no stale duplicate actor or manual refresh is needed after the reconnect

### 6.12 Duplicate-live retry `ENTERGAME` appearance reuse (debug-harness optional)

- [ ] Using a debug harness or controlled same-character duplicate-session setup, leave a second session for the same character waiting in `LOADING` after rejected `ENTERGAME`
- [ ] While the original live owner stays visible to another client, equip or unequip a supported `body`, `weapon`, or `head` item on that live owner
- [ ] Close the original live owner
- [ ] Retry `ENTERGAME` on the waiting duplicate session
- [ ] Confirm the watcher sees the retried owner re-enter with the latest visible body/weapon/head appearance in the normal peer-entry burst

Expected result:
- the waiting session does not reuse stale pre-rejection appearance cached before the runtime mutation
- the retried peer-entry burst carries the latest projected appearance in `CHAR_ADDITIONAL_INFO` and `CHARACTER_UPDATE`
- no stale duplicate actor or manual refresh is needed after the retry

### 6.13 Reclaimed stale equip / unequip isolation (debug-harness optional)

- [ ] Using a debug harness or controlled same-character duplicate-session setup, let a replacement session reclaim live ownership while the old socket remains open but stale
- [ ] On the stale old socket, run `/equip_item` or `/unequip_item` for a supported `body`, `weapon`, or `head` item
- [ ] Confirm the stale socket may still receive only its self-local item/appearance refresh frames
- [ ] Confirm the authoritative live replacement session and any visible watcher do **not** change appearance because of that stale mutation
- [ ] Confirm loopback-only `/local/inventory/{name}` and `/local/equipment/{name}` still report the replacement live owner's authoritative state, not the stale socket's local divergence

Expected result:
- stale post-reclaim equip/unequip remains non-authoritative
- no persisted carried/equipped state changes because of the stale socket
- no queued peer appearance refresh is emitted from the stale socket
- exact-name loopback inventory/equipment snapshots remain owned by the replacement live session

### 6.14 Reclaimed stale item-use isolation (debug-harness optional)

- [ ] Using a debug harness or controlled same-character duplicate-session setup, let a replacement session reclaim live ownership while the old socket remains open but stale
- [ ] On the stale old socket, run `/use_item <slot>` against a supported carried template-backed consumable stack (current QA seed: `27001`)
- [ ] Confirm the stale socket may still receive only its self-local point/item/info refresh frames
- [ ] Confirm the authoritative live replacement session and any visible watcher do **not** change because of that stale mutation
- [ ] Confirm loopback-only `/local/inventory/{name}` still reports the replacement live owner's authoritative carried state, not the stale socket's locally decremented stack

Expected result:
- stale post-reclaim item use remains non-authoritative
- no persisted points/inventory change because of the stale socket
- no peer-facing packet fanout is emitted from the stale socket
- exact-name loopback inventory snapshots remain owned by the replacement live session

### 6.15 Reclaimed stale merchant-buy isolation (debug-harness optional)

- [ ] Using a debug harness or controlled same-character duplicate-session setup, let a replacement session reclaim live ownership while the old socket remains open but stale
- [ ] On the stale old socket, keep a merchant window/context open and send one real `SHOP BUY` for slot `0` (or the local `/shop_buy 0` harness where appropriate)
- [ ] Confirm the stale socket may still receive only its self-local merchant success burst (`ITEM_SET` / `ITEM_UPDATE` refreshes without `GC::SHOP OK` on packet `SHOP BUY`, or those refreshes plus the debug-harness companion on `/shop_buy` where that local harness is used)
- [ ] Confirm the authoritative live replacement session and any visible watcher do **not** gain gold/items or otherwise change because of that stale mutation
- [ ] Confirm loopback-only `/local/inventory/{name}` (and currency introspection if available) still report the replacement live owner's authoritative state, not the stale socket's local divergence

Expected result:
- stale post-reclaim merchant buy remains non-authoritative
- no persisted gold/inventory change because of the stale socket
- no peer-facing packet fanout is emitted from the stale socket
- exact-name loopback inventory/currency snapshots remain owned by the replacement live session

### 6.16 Reconnect after stale item-use close rebuilds authoritative state (debug-harness optional)

- [ ] Using a debug harness or controlled same-character duplicate-session setup, let a replacement session reclaim live ownership while the old socket remains open but stale
- [ ] On the stale old socket, run `/use_item <slot>` against a supported carried template-backed consumable stack (current QA seed: `27001`) and observe the self-local divergence
- [ ] Close the authoritative replacement session first, then close the stale old socket
- [ ] Reconnect fresh on the same character
- [ ] Confirm the new bootstrap/reconnect frames and loopback state show the authoritative persisted `points`/inventory values from before the stale local-only mutation, not the stale socket's decremented stack or boosted points

Expected result:
- stale local-only item-use divergence dies with the stale socket
- reconnect rebuilds from authoritative persisted state
- no stale point/inventory divergence leaks into the new session bootstrap

### 6.17 Reconnect after stale merchant-buy close rebuilds authoritative state (debug-harness optional)

- [ ] Using a debug harness or controlled same-character duplicate-session setup, let a replacement session reclaim live ownership while the old socket remains open but stale
- [ ] On the stale old socket, keep the merchant gate active and issue `SHOP BUY` (or `/shop_buy <slot>` in the local harness) so only the stale socket sees the local success burst (`ITEM_SET` / `ITEM_UPDATE` refreshes without `GC::SHOP OK` on packet `SHOP BUY`, or those refreshes plus the debug-harness companion where that local harness is used)
- [ ] Close the authoritative replacement session first, then close the stale old socket
- [ ] Reconnect fresh on the same character
- [ ] Confirm the new bootstrap/reconnect state keeps the authoritative persisted `gold` and empty/unchanged carried inventory from before the stale local-only buy, not the stale socket's local grant

Expected result:
- stale local-only merchant-buy divergence dies with the stale socket
- reconnect rebuilds from authoritative persisted gold/inventory state
- no stale granted item leaks into the new session bootstrap

### 6.18 `/shop_buy` complex merchant-placement parity (debug-harness optional)

- [ ] Using the local merchant debug harness, prepare a buyer with several compatible partial `27001` carried stacks plus at least one free carried slot
- [ ] Open the merchant context and run `/shop_buy <slot>` for an authored entry whose `count` requires filling those compatible carried stacks and placing the final remainder into the lowest free carried slot
- [ ] Confirm the harness returns one inventory refresh per changed carried slot in carried-slot order (`ITEM_SET` for fresh slots, `ITEM_UPDATE` for existing stack fills) plus the current merchant success companion
- [ ] Repeat with the merchant entry's template temporarily authored with `anti_get`
- [ ] Confirm persisted `gold` and inventory match the same final state already frozen for the packet `SHOP BUY` path

Expected result:
- the local `/shop_buy` harness reuses the same deterministic carried-placement semantics as packet `SHOP BUY`
- compatible existing stacks fill first in slot order, then the remainder lands in the lowest free carried slot
- no harness-only placement drift appears in persisted or live runtime state
- `anti_get` merchant templates reject before gold, inventory, quickslot, or persisted-state mutation

### 6.19 Packet carried inventory move/swap/split/merge smoke (packet-harness optional)

- [ ] Enter `GAME` with a QA character that has one known carried item stack in slot `A` and an empty carried slot `B`
- [ ] Send one real client `ITEM_MOVE` request from `A` to `B` (`source TItemPos`, `destination TItemPos`, `count = 0`) to exercise full-stack drag/drop semantics
- [ ] Confirm the selected session receives `ITEM_DEL(A)` followed by `ITEM_SET(B)`
- [ ] Confirm loopback inventory snapshots or reconnect state show the item persisted in slot `B`
- [ ] Send one real client `ITEM_MOVE` request that attempts to equip a carried item whose explicitly loaded item-template snapshot omits that source `vnum`; confirm the request fails closed with no item frames, no point change, and no persisted inventory/equipment mutation
- [ ] Repeat with an incompatible destination occupied by another carried item if the QA setup has two disposable carried items, and confirm the runtime swaps the two carried items; if an item quickslot points at the source slot, confirm it is retargeted to the destination slot and any stale destination item quickslot is deleted
- [ ] Reset to two compatible carried stacks, then send `ITEM_MOVE` from `A` into occupied compatible stack slot `C` with `count = 0`
- [ ] Confirm the selected session receives self-only count refreshes: `ITEM_UPDATE(A)` if a source remainder survives or `ITEM_DEL(A)` if the source is fully consumed, followed by `ITEM_UPDATE(C)` capped at the authored template `max_count`
- [ ] Reset to a stack count greater than one, then send `ITEM_MOVE` from `A` to empty slot `B` with a partial count lower than the current stack count
- [ ] Confirm the selected session receives self-only refreshes for both slots: source stack remains in `A` with the reduced count and the split stack appears in `B`
- [ ] Reset to two compatible carried stacks, then send a partial-count `ITEM_MOVE` from `A` into occupied compatible stack slot `C`
- [ ] Confirm the selected session receives self-only `ITEM_SET` refreshes for both slots: source stack remains in `A` with the reduced count and destination stack `C` grows by the moved count
- [ ] Repeat the same partial-count request with an incompatible occupied destination and confirm it fails closed without changing live or persisted inventory

Expected result:
- packet `ITEM_MOVE` reuses the same authoritative full-stack empty-destination move semantics as `/inventory_move`
- empty-destination partial splits plus compatible occupied-destination partial, exact, and zero-count merges are accepted and persisted; full-stack incompatible occupied destinations swap and persist instead of failing closed
- the response stays self-only and uses the existing `ITEM_DEL` / `ITEM_SET` / `ITEM_UPDATE` refresh family; quickslot sync retargets source item quickslots only for full-stack moves/swaps where the source item lands in the destination cell, preserves source item quickslots for partial splits/merges, and deletes source item quickslots when the source stack is fully consumed by a compatible merge
- non-carried windows and out-of-range cells fail closed without mutation

### 6.20 `ITEM_USE_TO_ITEM` stack consolidation smoke (packet-harness optional)

- [ ] Enter `GAME` with a QA character that has two compatible carried stacks for a template-backed stackable item such as `27001`
- [ ] Send one real client `ITEM_USE_TO_ITEM` request from the source carried slot onto the target carried slot
- [ ] Confirm the selected session receives `ITEM_DEL(source)` then `ITEM_SET(target)` when the source fits completely into the target
- [ ] If one or more item quickslots point at the removed source slot, confirm each receives `QUICKSLOT_DEL` after the item refresh frames; skill/command quickslots with the same byte slot value must stay unchanged and persist across reconnect
- [ ] Repeat with a target stack that has only partial room under the authored `max_count`
- [ ] Confirm the selected session receives count-only refreshes for both carried cells and the source item quickslot remains
- [ ] Repeat with incompatible `vnum`, missing/invalid template metadata, `anti_stack`, non-stackable, locked, empty, same-cell, already-full, and over-template-max setups where available
- [ ] For the same-cell case specifically, send `ITEM_USE_TO_ITEM` with identical source and target carried cells and confirm no item, quickslot, point, or persisted-state change occurs

Expected result:
- accepted drag-to-item consolidation is self-only, persists the merged inventory, and never runs the normal consumable `use_effect`
- rejection cases fail closed with no frames, no inventory mutation, no quickslot mutation, and no point/effect fallback
- templates with `max_count > 255` reject because the current bootstrap `ITEM_SET` / `ITEM_UPDATE` count field is one byte

### 6.21 Visible-peer item drop / pickup smoke (packet-harness optional)

- [ ] Put client A and client B in the same visible bootstrap scope with client A carrying one disposable stack
- [ ] Send one real client `ITEM_DROP` or `ITEM_DROP2` request from client A for that carried slot
- [ ] Confirm client A receives its carried-slot mutation refresh followed by `GC::ITEM_GROUND_ADD` and `GC::ITEM_OWNERSHIP` naming client A's character
- [ ] Confirm visible client B receives one queued/rendered ground-item add plus the matching ownership label for the same visible ground handle
- [ ] Send one real client `ITEM_PICKUP` request from client B for that handle
- [ ] Confirm client B receives `GC::ITEM_GROUND_DEL` followed by deterministic carried inventory refreshes: `GC::ITEM_SET` for a restored/new carried slot, `GC::ITEM_UPDATE` for compatible stack merges, or multiple `GC::ITEM_UPDATE` frames plus a `GC::ITEM_SET` when a stackable pickup fills partial stacks and places a remainder
- [ ] Confirm client A sees the queued ground delete and no longer owns the dropped item in persisted inventory
- [ ] If the dropped item's loaded template can be marked `anti_give` after it is on the ground in a debug harness, repeat client B's owned-item pickup and confirm client B receives only the inventory-full info rejection, client A receives no owner-delivery frames, neither inventory mutates, and client A can still reclaim the pending handle
- [ ] Repeat with client A dropping bootstrap gold and client B picking up the owned gold marker; confirm client B receives ground delete plus delivered-to-party-member `ITEM_GET`, while client A receives the peer-visible delete, a positive `POINT_CHANGE(POINT_GOLD)`, and from-party-member `ITEM_GET`; confirm client B's gold total is unchanged while client A's persisted gold is restored
- [ ] Attempt a replayed pickup for the same handle and confirm it fails closed without extra item grants

Expected result:
- accepted drops publish one temporary bootstrap ground handle plus the current ownership label to currently visible peers
- `anti_drop` / `anti_give` template-flagged carried items reject `ITEM_DROP` / `ITEM_DROP2` before inventory, quickslots, or temporary ground handles mutate
- if a debug harness can force the dropping character to zero HP before the shared-world ground-handle registration seam, no new visible ground handle should be published
- `anti_stack` / `anti_sell` template-flagged carried items, and stackable templates with `max_count > 255` that cannot be represented by the current bootstrap `ITEM_SET` / `ITEM_UPDATE` count fields, reject `ITEM_USE_TO_ITEM` stack consolidation before inventory, quickslots, points, or persisted state mutate
- full `ITEM_USE_TO_ITEM` stack merges delete every item quickslot that pointed at the removed source slot while leaving skill/command quickslots with the same byte slot value unchanged
- visible peers can collect the temporary handle when compatible carried stack capacity and/or a carried destination slot can accept the entire picked count
- owner-owned visible gold markers restore the owner's gold with party-shaped pickup notices when a visible peer collects them
- `anti_give` or recipient-restricted owner-owned item pickup by a visible peer rejects before owner/collector inventory mutation while leaving the pending handle available for owner reclaim
- the recipient mutation persists before the temporary handle is removed
- ground-item delete fanout reaches other visible sessions after successful pickup
- replayed, unknown, invisible, no-merge-capacity, or no-free-slot pickup attempts fail closed
- reconnecting does not restore the temporary bootstrap ground handle as a durable world entity

### 6.22 Radius-AOI ground item visibility rebuild smoke (packet-harness optional)

- [ ] Start `gamed` with radius AOI enabled for QA
- [ ] Put client A carrying one disposable stack inside radius of the future drop point, and keep client B initially outside that radius
- [ ] Have client A drop the carried stack and confirm client A receives the carried-slot mutation plus `GC::ITEM_GROUND_ADD` and `GC::ITEM_OWNERSHIP`
- [ ] Confirm client B does not receive the ground add/ownership pair while still outside radius
- [ ] Move client B into radius with position-only `MOVE` or `SYNC_POSITION`
- [ ] Confirm client B receives the ordinary queued visibility-entry frames first and then one queued `GC::ITEM_GROUND_ADD` plus `GC::ITEM_OWNERSHIP` for the still-pending handle
- [ ] Move client B back outside radius and confirm it receives the ordinary visibility-exit cleanup first and then one queued `GC::ITEM_GROUND_DEL` for that handle

Expected result:
- pending bootstrap ground handles rebuild for sessions that cross into their visible world after the original drop
- pending bootstrap ground handles are torn down for sessions that cross back out before pickup/despawn policy exists
- the rebuild/teardown is self-facing to the moving/syncing session and does not make the handle durable across reconnects

### 6.23 Packet merchant sell-back smoke (packet-harness optional)

- [ ] Open a structured merchant `shop_preview` window while the QA character has at least one carried inventory stack
- [ ] Send one real client `SHOP SELL` request for a carried slot containing a stack
- [ ] Confirm whole-stack sell removes the carried slot and answers with `ITEM_DEL` followed by `PLAYER_POINT_CHANGE(POINT_GOLD)` with no extra bare `GC::SHOP OK`
- [ ] Repeat with `SHOP SELL2` and a count lower than the stack size
- [ ] Confirm partial sell refreshes the carried slot with `ITEM_UPDATE`, then answers with `PLAYER_POINT_CHANGE(POINT_GOLD)` with no extra bare `GC::SHOP OK`
- [ ] Repeat `SHOP SELL2` with a count larger than the current stack and confirm it returns the merchant invalid-position path without changing gold, carried inventory, or persisted account state
- [ ] If the QA setup can mark a carried item as runtime-locked, attempt one `SHOP SELL` or `SHOP SELL2` for that slot from an open merchant window
- [ ] Confirm the locked sell attempt returns the merchant invalid-position path and does not change gold, carried inventory, or persisted account state
- [ ] Confirm loopback inventory/currency snapshots or reconnect state show the credited bootstrap gold and updated carried inventory

Expected result:
- packet `SELL` / `SELL2` mutate the selected character's carried inventory and gold while the merchant context is active
- the current bootstrap sell price uses the loaded item template's ordinary shop-buy price and count-per-gold flag through the owned legacy count/price branch, then applies the shared `/5` and `3%` tax floors; anti-sell and runtime-locked item guards fail closed without mutation, while richer sell UI choreography remains later slices
- no peer-facing packet fanout is emitted from sell-back alone

### 6.20 Training-dummy combat target selection (packet-harness optional)

- [ ] Seed or confirm one visible authored/runtime-marked `training_dummy` actor exists near the QA character
- [ ] Using the first live client path or a packet harness that can emit `TARGET`, send one target-selection request while the character stands within the current bootstrap `300`-unit band
- [ ] Confirm the selected session receives exactly one self-only `GC TARGET` acknowledgement carrying the dummy's `target_vid` and the current bootstrap `hp_percent = 100`
- [ ] Repeat once from outside the current `300`-unit target-selection band
- [ ] Repeat once against a visible non-player actor that is *not* authored/runtime-marked as `training_dummy`

Expected result:
- accepted in-range visible `training_dummy` selection returns exactly one self-only `GC TARGET` ack
- the ack stays tiny in the current slice: `target_vid` plus `hp_percent`, with no attack, damage, aggro, or death choreography implied
- out-of-range, invisible, or visible non-targetable actors fail closed without self-only chat spam, peer fanout, persistence writes, or a compensating clear-target packet
- if the QA client does not yet expose a visible HUD reaction for `GC TARGET`, treat the packet-level acceptance as the source of truth for this slice rather than blocking on richer UI choreography

### Combat ownership smoke bundle

Treat sections 6.20 through 6.23 as one ownership-focused smoke bundle when debugging bootstrap combat state.
Together they cover:
- target clear on bootstrap/reset seams
- stale reclaim non-authoritative behavior
- dead or replaced dummy snapshot rejection
- visible zero-HP death plus selected-target clear behavior

### 6.20 Training-dummy target clears across transfer / re-enter / reconnect (packet-harness optional)

- [ ] Select one visible authored/runtime-marked `training_dummy` and confirm the current session receives the normal self-only `GC TARGET(target_vid, 100)` ack
- [ ] Cross one owned transfer/rebootstrap seam (for example a QA warp/transfer trigger), then return to the original dummy so it is visible and in range again
- [ ] Without sending a fresh `TARGET`, issue one normal `ATTACK` toward that same dummy `VID`
- [ ] Repeat the same expectation after same-socket `/phase_select` → fresh `SELECT`/`ENTERGAME`, or after a full disconnect/reconnect if that is the easier QA path
- [ ] Finally send a fresh `TARGET` again and confirm the next normal `ATTACK` resumes the expected self-only dummy HP refresh path

Expected result:
- fresh bootstrap/rebootstrap boundaries clear the active dummy target context instead of carrying stale linkage forward
- post-transfer, post-`/phase_select` re-entry, and post-reconnect attacks fail closed until the client reacquires target intent with a new accepted `TARGET`
- once reselected, the same dummy immediately resumes the current self-only `GC TARGET(target_vid, hp_percent)` attack-refresh behavior

### 6.21 Stale reclaimed combat attempts stay non-authoritative (debug-harness optional)

- [ ] Using a debug harness or controlled same-character duplicate-session setup, let a replacement session reclaim live ownership while the old socket remains open but stale
- [ ] On the authoritative replacement session, select one visible `training_dummy` and keep it ready as the current live combat target
- [ ] On the stale old socket, try one `TARGET` and one normal `ATTACK` against the same or another visible dummy `VID`
- [ ] Confirm the stale socket receives no combat-visible success refresh and the authoritative replacement session receives no queued combat frames from those stale requests
- [ ] On the authoritative replacement session, issue one normal `ATTACK` against its currently selected dummy without reselecting again

Expected result:
- stale post-reclaim `TARGET` / `ATTACK` attempts fail closed and stay non-authoritative
- runtime-owned dummy HP does not change because of the stale socket
- the replacement live owner's selected dummy target remains intact and its next authoritative attack still produces the normal self-only `GC TARGET(target_vid, hp_percent)` refresh

### 6.22 Replaced or dead training-dummy targets fail closed (debug-harness optional)

- [ ] Select one visible `training_dummy` and confirm the normal self-only `GC TARGET(target_vid, 100)` ack
- [ ] Using a debug harness/admin seam, replace that same dummy's runtime snapshot in place (for example by moving/updating the actor while keeping it visible and in range) without sending a fresh `TARGET`
- [ ] Confirm the session first receives the ordinary actor refresh / visibility-transition frames from that update and then one self-only `GC TARGET(0, 0)` clear
- [ ] With a second still-live visible session that had not yet targeted the engaged practice mob, send a fresh `TARGET` after the update and confirm it succeeds again instead of staying aggro-gated
- [ ] Immediately send one normal `ATTACK` against the still-visible dummy `VID`
- [ ] Re-select the dummy and confirm the next normal `ATTACK` works again with the usual self-only `GC TARGET(target_vid, hp_percent)` refresh
- [ ] Remove that same still-selected dummy outright through an operator/debug seam and confirm the session first receives the ordinary actor `CHARACTER_DEL` and then one self-only `GC TARGET(0, 0)` clear before any later stale `ATTACK` fails closed
- [ ] Repeat with a harness-injected dead state (`current HP = 0`) and confirm both a fresh `TARGET` and a later `ATTACK` against the old selected dummy fail closed

Expected result:
- accepted combat ownership is bound to the selected dummy snapshot, not only the visible `VID`
- if that snapshot is replaced before reselection, the runtime now also tears down the stale selected combat-target ownership immediately: after the ordinary actor refresh / visibility-transition frames, the client receives one self-only `GC TARGET(0, 0)`, the old practice-mob engagement is released, and stale `ATTACK` intent fails closed until the client reacquires target ownership with a new accepted `TARGET`
- outright runtime removal of a still-selected dummy tears down both the visible actor and the selected combat-target ownership immediately: the client sees the ordinary `CHARACTER_DEL` plus one self-only `GC TARGET(0, 0)` companion before later stale attacks stay denied
- a dead (`0` HP) dummy is no longer eligible for accepted bootstrap target selection or attack refreshes
- these rejections stay silent in the current slice: no peer fanout, no compensating chat spam, and no accidental HP mutation

### 6.23 Training-dummy zero-HP death clears selected targets (packet-harness optional)

- [ ] Prepare two visible sessions if possible: one attacker and one watcher that can also select the same visible `training_dummy`
- [ ] On both sessions, select the same dummy and confirm the normal self-only `GC TARGET(target_vid, 100)` ack before any attacks
- [ ] From the attacker, issue successive normal `ATTACK` requests until the dummy reaches its final accepted hit from `1` to `0`
- [ ] Confirm non-lethal hits still use the normal self-only `GC TARGET(target_vid, hp_percent)` refresh path (`90`, `80`, ... , `10`)
- [ ] Confirm the final zero-HP hit makes the attacker receive `GC DEAD(vid)` plus one self-only `GC TARGET(0, 0)` clear instead of a final `GC TARGET(..., 0)` refresh
- [ ] If a second visible selected session is present, confirm it also receives `GC DEAD(vid)` and its own self-only `GC TARGET(0, 0)` clear during that same death window
- [ ] Without waiting for any future respawn slice, try one fresh `TARGET` and one `ATTACK` against that same dummy `VID`

Expected result:
- the zero-HP edge is now visibly owned: `GC DEAD(vid)` is emitted to visible sessions when the dummy dies
- any session that still had that dummy selected receives the existing self-only clear-target companion immediately on death
- the bootstrap combat loop does not send a synthetic `GC TARGET(..., 0)` refresh at death; it switches surfaces from HP refresh to death + clear
- fresh `TARGET` and `ATTACK` attempts fail closed while the dummy remains dead
- the timed respawn/reset path is validated separately in 6.24; this step only proves death, clear, and dead-state rejection before the respawn window expires

### 6.24 Training-dummy timed respawn rebuild requires fresh reselection (packet-harness optional)

- [ ] Starting from the zero-HP death state in 6.23, keep the dead dummy visible to at least one session and, if possible, to a second watcher that had it selected before death
- [ ] Before the owned `2s` dead timer expires, bring in one fresh live session (or move/re-enter one back into visibility) and confirm it receives the ordinary dummy `CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE` burst immediately followed by one `GC DEAD(vid)` replay instead of a silently live-looking mob
- [ ] If you have operator static-actor edit access, refresh or retarget that still-dead visible dummy without letting the respawn timer expire and confirm any retained delete-plus-rebootstrap refresh likewise ends with one trailing `GC DEAD(vid)`
- [ ] Confirm that no respawn rebuild packets arrive before the first owned `2s` dead timer expires
- [ ] Once the timer expires, confirm each currently visible session receives the respawn rebuild burst in this order: `CHARACTER_DEL(vid)` -> `CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE`
- [ ] Confirm the rebuilt actor returns at the authored/bootstrap position and uses the same visible `VID`
- [ ] Without sending a fresh `TARGET`, issue one normal `ATTACK` from the previous attacker and, if applicable, from the previous watcher that still had the dead dummy selected before respawn
- [ ] Then send a fresh `TARGET` and confirm the next accepted `GC TARGET(target_vid, 100)` and first post-respawn `ATTACK` resume the normal self-only HP loop from full bootstrap HP

Expected result:
- the first respawn is purely server-driven and waits for the owned fixed `2s` dead interval
- late or refreshed visibility before respawn replays dead state explicitly: any later add-style actor presentation gets the ordinary actor add/info/update burst plus one trailing `GC DEAD(vid)`
- respawn reuses normal visibility teardown + rebuild packet families instead of inventing a dedicated revive packet
- the rebuilt dummy is a fresh live combat snapshot even if the visible `VID` is reused
- stale pre-death target ownership does not survive the respawn boundary; post-respawn attacks fail closed until the session reselects target intent with a new accepted `TARGET`
- once reselected, the dummy immediately resumes the current bootstrap HP refresh path from `100` -> `90` on the next accepted normal hit

### 6.25 Content-loaded `spawn_groups` practice mob smoke

- [ ] Import or preload one authored `spawn_groups` entry that materializes a visible stationary practice mob using `combat_profile = training_dummy`
- [ ] Confirm the mob appears at the authored position with the authored display name and can be targeted in the same way as the earlier bootstrap dummy slices
- [ ] With two visible clients, let client one land the first accepted hit and verify client two's fresh `TARGET` attempt on the already-engaged mob fails closed while client one still owns that live engagement
- [ ] On the owning client, confirm each accepted live hit now returns both the usual target-refresh and one immediate self-only HP `POINT_CHANGE` decrement while the mob remains alive
- [ ] If you can control timing precisely, send one repeated normal `ATTACK` against that same live selected mob before the owned `250ms` cadence window expires and confirm it fails closed with no target refresh, no extra immediate retaliation tick, and no delayed-cadence reset
- [ ] Wait at least the owned `250ms` cadence window and confirm the next same-target normal `ATTACK` is accepted again
- [ ] After the first accepted live owner hit, stop sending `ATTACK` for at least the owned `1s` retaliation delay and confirm one queued self-only HP `POINT_CHANGE` follow-up beat arrives without a second client attack
- [ ] Wait one more owned `1s` delay without another accepted hit and confirm a second queued self-only HP `POINT_CHANGE` follow-up beat arrives while the mob stays alive and engaged
- [ ] If you can control timing precisely, land a later accepted owner hit while one autonomous delayed beat is already pending and confirm the current slice still yields only one queued delayed follow-up beat on the original timer rather than accelerating or resetting that cadence window
- [ ] If you can control timing precisely, also try a rapid second accepted hit before the first delayed beat fires and confirm the current slice still yields only one queued delayed follow-up beat for that first pending window
- [ ] Lower the owning character's HP near `0` if you can, then confirm the immediate or delayed retaliation tick clamps at `0` HP instead of going negative and that no further delayed follow-up beat arrives once that floor is reached
- [ ] After retaliation has already driven the owning character to `0` HP, use `/phase_select` or a full disconnect/reconnect and confirm the fresh bootstrap rebuilds the owner's points from the pre-retaliation persisted value instead of carrying the runtime-only retaliation loss across sessions
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then send a same-map `MOVE`, `SYNC_POSITION`, or transfer-triggering move/rebootstrap and confirm the live session still shows the reduced runtime-only retaliation points while a later reconnect still rebuilds from the older persisted point value instead of leaking that retaliation loss through position-only saves
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then send one successful `/use_item <slot>` or carried-slot `ITEM_USE` and confirm the item consumption plus its authored use-effect point delta persist while a later reconnect still rebuilds from the older persisted point value plus that owned use-item delta instead of leaking the runtime-only retaliation loss through the point-bearing use-item save. If the consumed stack reaches zero and an item quickslot points at that carried slot, confirm the client receives `ITEM_DEL` followed by `QUICKSLOT_DEL` for item quickslots only before the self `INFO` effect message, and that skill/command quickslots using the same byte value remain intact.
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then send one successful `/equip_item <slot> <equip_slot>` and confirm the carried->equipped item mutation plus its authored equip-effect point delta persist while a later reconnect still rebuilds from the older persisted point value plus that owned equip delta instead of leaking the runtime-only retaliation loss through the point-bearing equip save
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then send one successful `/unequip_item <equip_slot> <slot>` and confirm the equipped->carried item mutation plus the authored equip-effect removal persist while a later reconnect still rebuilds from the older persisted point value minus that owned equip delta instead of leaking the runtime-only retaliation loss through the point-bearing unequip save
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then send one successful `/inventory_move <from_slot> <to_slot>` and confirm the carried-slot mutation persists while a later reconnect still rebuilds the older persisted point value instead of leaking that runtime-only retaliation loss through the non-point-bearing inventory save
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then complete one successful merchant preview + `/shop_buy 0` purchase and confirm the bought item plus gold debit persist while a later reconnect still rebuilds the older persisted point value instead of leaking that runtime-only retaliation loss through the non-point-bearing merchant-buy save
- [ ] After that `/phase_select` recovery on the same socket, if the same practice mob stayed alive, send a fresh `TARGET` and confirm the ack still shows the mob's current runtime-owned HP instead of silently resetting it to full, then send one normal `ATTACK` and confirm the usual target-refresh plus immediate self-only retaliation resumes
- [ ] After retaliation has already driven the owning character to `0` HP, send same-socket `/restart_here` and confirm the owner stays in `GAME`, receives the ordinary self bootstrap burst (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE` -> `PLAYER_POINT_CHANGE`) rebuilt from the persisted point value, and that a visible live peer sees one queued delete-plus-rebootstrap refresh for that owner (`CHARACTER_DEL` -> `CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE`)
- [ ] Immediately after `/restart_here`, try one same-target normal `ATTACK` without reselecting and confirm it still fails closed; then send a fresh `TARGET` and confirm the same still-live practice mob resumes from its current runtime-owned HP instead of resetting because of the owner's recovery
- [ ] While the owner is still alive, try same-socket `/restart_here` and confirm it fails closed with no self bootstrap burst and no peer-facing refresh
- [ ] After retaliation has already driven the owning character to `0` HP, send same-socket `/restart_town` and confirm the owner stays in `GAME`, receives the ordinary self transfer rebootstrap burst rebuilt from the persisted point value at the owned empire town-return coordinates, and no longer keeps the old practice mob selected
- [ ] With one visible live peer still on the source map and one visible live peer already on the destination town map, confirm `/restart_town` queues one `CHARACTER_DEL` to the source peer, one queued owner re-entry burst (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE`) to the destination peer, and one self-facing `CHARACTER_DEL` for the source practice mob after the owner's self rebootstrap burst
- [ ] Immediately after `/restart_town`, try one same-target normal `ATTACK` without reselecting and confirm it still fails closed; if the same practice mob later remains alive and becomes visible again, send a fresh `TARGET` and confirm the mob resumes from its current runtime-owned HP instead of resetting because of the owner's recovery
- [ ] After that accepted `/restart_town`, let one fresh live peer enter the destination town visibility later and confirm it sees the recovered owner through the ordinary peer-entry burst only (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE`) with no replayed `GC DEAD(owner_vid)` from the earlier pre-restart death window
- [ ] When that immediate or delayed retaliation floor reaches `0` HP, confirm the client first receives one self-only `DEAD(owner_vid)` and then one self-only `TARGET(0, 0)` clear instead of keeping the stale engaged practice mob selected
- [ ] After retaliation has already driven the owning character to `0` HP, send a fresh combat `TARGET` against the same or another visible practice mob and confirm it fails closed with no self-only target acknowledgement
- [ ] After retaliation has already driven the owning character to `0` HP, send another same-target normal `ATTACK`; confirm it fails closed with no target refresh, no extra point-loss, and no re-armed delayed follow-up beat
- [ ] After retaliation has already driven the owning character to `0` HP, send a `MOVE` toward a different visible coordinate (or a known transfer-trigger coordinate if one is configured) and confirm it fails closed with no self `MOVE_ACK`, no peer movement replication, and no transfer / rebootstrap burst
- [ ] After retaliation has already driven the owning character to `0` HP, send a `SYNC_POSITION` update for that same character and confirm it fails closed with no self `SYNC_POSITION_ACK` and no peer synchronization replication
- [ ] After retaliation has already driven the owning character to `0` HP, try one visible authored static-actor `INTERACT` request (`info`, `talk`, `shop_preview`, or `warp`) and confirm it fails closed with no self chat/info delivery, no merchant preview open, and no transfer / rebootstrap burst
- [ ] Open a merchant preview before retaliation reaches the owner's `0` HP floor, then after that floor is reached confirm the owner first receives one self-only `GC::SHOP END` after the owned self `DEAD` + self `TARGET(0, 0)` transition, and confirm a later client `SHOP END` request fails closed because that merchant context was already cleared
- [ ] Open a merchant preview before retaliation reaches the owner's `0` HP floor, then after that floor is reached send `SHOP BUY` or `/shop_buy 0` and confirm the buy fails closed with no `GC ITEM_SET`, no merchant success/failure chat, and no inventory / gold mutation in loopback runtime snapshots
- [ ] Carry a consumable item before retaliation reaches the owner's `0` HP floor, then after that floor is reached send `/use_item <slot>` and one carried-slot `ITEM_USE`; confirm both fail closed with no `GC PLAYER_POINT_CHANGE`, no `GC ITEM_SET`, no info chat, and no inventory / point mutation in loopback runtime snapshots
- [ ] Carry one droppable item before retaliation reaches the owner's `0` HP floor, then while still alive send one carried-slot `ITEM_DROP` and confirm the client receives a self-only carried-slot delete plus one `ITEM_GROUND_ADD` at the character's current coordinates; repeat with `ITEM_DROP2` on a stack and confirm the carried-slot count decrements with `ITEM_UPDATE` plus one self-only ground-add. With a visible live peer, pick up that owned ground item from the peer and confirm party-shaped pickup notices: the peer receives ground delete plus delivered-to-party-member `ITEM_GET`, while the owner receives the peer-visible delete, recipient inventory refresh frames using compatible stack-before-fresh-slot placement, and from-party-member `ITEM_GET`. After the owner reaches `0` HP, retry `ITEM_DROP` / `ITEM_DROP2` and confirm both fail closed with no inventory mutation and no ground-add.
- [ ] Carry one slash-equipable item and/or wear one slash-unequipable item before retaliation reaches the owner's `0` HP floor, then after that floor is reached send `/equip_item <slot> <equip_slot>` and `/unequip_item <equip_slot> <slot>` and confirm both fail closed with no `GC ITEM_DEL`, no `GC ITEM_SET`, no `GC PLAYER_POINT_CHANGE`, no `GC CHARACTER_UPDATE`, and no inventory / equipment / point mutation in loopback runtime snapshots
- [ ] Carry one movable inventory item before retaliation reaches the owner's `0` HP floor, then after that floor is reached send `/inventory_move <from_slot> <to_slot>` and confirm it fails closed with no `GC ITEM_SET` and no runtime or persisted carried-slot mutation in loopback runtime snapshots
- [ ] After retaliation has already driven the owning character to `0` HP, send one peer-facing `CHAT` with each owned type (`TALKING`, `PARTY`, `GUILD`, `SHOUT`) and confirm every request fails closed with no self `GC_CHAT` echo and no queued peer delivery
- [ ] If you have a packet harness or test-client path that can still send client-originated `CHAT_TYPE_INFO`, try one after retaliation has already driven the owning character to `0` HP and confirm it fails closed with no self `GC_CHAT` info delivery
- [ ] With a second visible player online, drive the owning character to `0` HP through either the immediate retaliation tick or the delayed follow-up beat and confirm that peer receives exactly one queued `GC DEAD(owner_vid)` while the owner still receives the existing self-only `GC DEAD(owner_vid)` plus self-only `GC TARGET(0, 0)` clear
- [ ] While that same practice mob still remains alive after the owner's retaliation-driven `0`-HP death, have the second visible player send a fresh `TARGET` against it and confirm the ack now succeeds at the mob's current runtime-owned HP instead of staying orphan-locked behind the dead owner
- [ ] In that same two-player setup, have the still-live peer change visible appearance with `/equip_item` or `/unequip_item` and confirm that peer still receives the ordinary self item/point/update result while the still-connected dead owner receives no queued peer `CHARACTER_UPDATE` appearance refresh
- [ ] With a third live visible peer available in the same world, drive the owning character to `0` HP and then move one still-live peer with `MOVE` while both live peers remain visible to each other; confirm the mover still receives the ordinary self `MOVE_ACK`, the third live peer still receives the ordinary queued peer `MOVE_ACK`, and the still-connected dead owner receives no queued peer `MOVE_ACK`
- [ ] Repeat the same same-visible-set expectation with `SYNC_POSITION` and confirm the syncing live peer still receives the ordinary self `SYNC_POSITION_ACK`, the third live peer still receives the ordinary queued peer `SYNC_POSITION_ACK`, and the still-connected dead owner again receives no queued peer sync replication
- [ ] After retaliation has already driven the owning character to `0` HP, send one `WHISPER` to a live visible peer and one to a missing exact-name target; confirm both fail closed with no queued target delivery and no self `WHISPER_TYPE_NOT_EXIST` fallback
- [ ] With a second visible player online, drive the owning character to `0` HP and then send one exact-name `WHISPER` from that second player to the dead owner's still-connected character name; confirm it fails closed with no queued target `GC_WHISPER` delivery and no self `WHISPER_TYPE_NOT_EXIST` fallback for the sender
- [ ] With that same second visible player online, drive the owning character to `0` HP and then send one local `CHAT_TYPE_TALKING`; confirm the sender still receives the ordinary self `GC_CHAT` echo while the dead owner receives no queued peer chat delivery
- [ ] With that same second visible player online, drive the owning character to `0` HP and then send one `CHAT_TYPE_PARTY`, one `CHAT_TYPE_GUILD`, and one `CHAT_TYPE_SHOUT`; confirm each sender still receives the ordinary self `GC_CHAT` echo while the dead owner receives no queued peer chat delivery
- [ ] With that same second visible player online, drive the owning character to `0` HP and then trigger one loopback/runtime server notice (`/local/notice` or equivalent); confirm the live peer still receives the queued `CHAT_TYPE_NOTICE` broadcast while the dead owner receives no queued notice delivery
- [ ] With a third visible player or fresh reconnect available after the owning character reached `0` HP, let that later peer join the same visible world and confirm currently live recipients still receive the ordinary queued `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst while the still-connected dead owner receives no queued peer-entry frames from that later join
- [ ] In that same fresh-join setup, confirm the newcomer also receives one trailing `GC DEAD(owner_vid)` for the already-dead visible owner right after the ordinary peer-entry burst for that owner instead of silently presenting the owner as live
- [ ] If you can boot with radius AOI or another controlled visibility gate, keep one live peer initially outside visibility after the owner reaches `0` HP, then move that peer into range with `MOVE` and confirm the mover still receives the ordinary queued origin-side `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst for the dead owner, followed immediately by one queued `GC DEAD(owner_vid)`, while the zero-HP owner receives no queued peer-entry burst for that mover
- [ ] Repeat the same visibility re-entry expectation with `SYNC_POSITION` crossing into range and confirm the syncing live peer still receives the ordinary queued origin-side peer-entry burst for the dead owner, followed immediately by one queued `GC DEAD(owner_vid)`, while the zero-HP owner again receives no queued peer-entry burst for that syncing peer
- [ ] Using `/local/transfer`, another controlled runtime relocate flow, or the exact-position transfer trigger if available, move a live peer into the dead owner’s visible world after that owner reached `0` HP and confirm the transferred peer still receives the ordinary queued origin-side peer-entry burst for the dead owner, followed immediately by one queued `GC DEAD(owner_vid)`, while the zero-HP owner receives no queued peer-entry burst for that transferred peer
- [ ] Using `/local/transfer` or another controlled runtime relocate flow, move that already-dead owner itself into another live peer’s visible world after the same `0`-HP transition and confirm the newly paired live peer still receives the ordinary queued peer-entry burst for the relocated dead owner, followed immediately by one queued `GC DEAD(owner_vid)`, rather than silently treating the transferred owner as live, while the dead owner itself receives no queued destination peer-entry burst for that newly paired live peer
- [ ] In that same dead-owner relocate setup, move the zero-HP owner into visibility of a destination practice mob or another visible static actor and confirm the dead owner still receives no queued destination `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst for that actor; only any old-world cleanup frames should remain queued locally
- [ ] If loopback runtime/operator snapshots are available, inspect `/local/players` or `/local/visibility` after that same `0`-HP transition and confirm the still-connected owner is marked `dead: true`; if you use `/local/relocate-preview` or `/local/transfer`, confirm the same `dead: true` flag is preserved when that owner appears as `character`, `target`, or a visible peer in the structured result
- [ ] With that same dead owner still connected, close or disconnect one other currently visible live peer and confirm the departing peer closes normally while the zero-HP owner receives no queued peer `CHARACTER_DEL` teardown for that later leave
- [ ] Using `/local/transfer` or another controlled runtime relocate flow, move one other currently visible live peer out of the dead owner’s visible world after that owner reached `0` HP and confirm the transferred peer still receives its ordinary origin-side cleanup while the zero-HP owner receives no queued peer `CHARACTER_DEL` teardown for that later relocate-away transfer
- [ ] If you can boot with radius AOI or another controlled visibility gate, keep one other live peer initially visible after the owner reaches `0` HP, then move that peer out of range with `MOVE` and confirm the mover still receives its ordinary origin-side cleanup while the zero-HP owner receives no queued peer `CHARACTER_DEL` teardown for that later AOI move-out
- [ ] Repeat the same teardown expectation with `SYNC_POSITION` crossing out of range and confirm the syncing peer still receives its ordinary origin-side cleanup while the zero-HP owner again receives no queued peer `CHARACTER_DEL` teardown for that later AOI sync-out
- [ ] With that same second visible player online, first drive that second peer to `0` HP through its own practice-mob retaliation floor, then later drive the original owner to `0` HP too and confirm the already-dead still-connected peer receives no queued `GC DEAD(owner_vid)` frame from the later peer-visible death fanout
- [ ] After that owner has already reached `0` HP but while the same visible practice mob is still alive, let another live visible player kill that mob and confirm the killer still gets the ordinary self `GC DEAD(mob_vid)` + target-clear transition while the already-dead owner receives no queued later visible practice-mob `GC DEAD(mob_vid)` fanout
- [ ] In that same setup, wait for the owned `2s` dummy respawn delay and confirm live viewers still receive the ordinary `CHARACTER_DEL` + `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` respawn rebuild burst while the already-dead owner receives none of those later practice-mob respawn frames
- [ ] With that same dead owner still connected and at least one other live viewer still sharing visible world, register one new visible static actor through the loopback operator path and confirm the live viewer receives the ordinary `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` burst while the already-dead owner receives no queued static-actor registration frames
- [ ] Update that same static actor in place while it remains visible and confirm the live viewer receives the ordinary static-actor refresh (`CHARACTER_DEL` + `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE`) while the already-dead owner receives no queued static-actor refresh frames
- [ ] Remove that same static actor and confirm the live viewer receives the ordinary `CHARACTER_DEL` while the already-dead owner receives no queued static-actor delete frame
- [ ] With two visible live players and one visible content-loaded practice mob, let both players `TARGET` the same mob before either attacks, then land the first accepted hit from only one player and confirm the other player now receives one self-only `GC TARGET(0, 0)` clear-target companion, a stale preselected `ATTACK` still fails closed afterward, and a fresh `TARGET` retry on that same still-live mob also stays blocked until the owned release boundary
- [ ] Replace the selected practice-mob target before the next owned `1s` delay expires and confirm the queued delayed follow-up beat fails closed, then have a second visible player `TARGET` the abandoned still-live mob and confirm the ack now succeeds at its current runtime-owned HP instead of staying orphan-locked behind the old owner
- [ ] Move or sync far enough to force a self `TARGET(0, 0)` clear before the next owned `1s` delay expires and confirm that same queued delayed follow-up beat also fails closed after target invalidation, then have a second visible player `TARGET` the abandoned still-live mob and confirm the ack now succeeds at its current runtime-owned HP
- [ ] After the first accepted owner hit but before the next owned `1s` delay expires, cross a transfer trigger / rebootstrap seam and confirm the owner gets the normal self transfer burst with no late delayed retaliation beat, while a still-visible source-map peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP
- [ ] After the first accepted owner hit but before the next owned `1s` delay expires, issue `/logout` on the owning session and confirm it transitions toward close with no later queued retaliation beat, any visible peer sees the owner disappear cleanly, and that peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP
- [ ] Repeat the same pending-cadence teardown with `/quit` and confirm the owner still receives self `CHAT_TYPE_COMMAND quit` while staying in `GAME`, but any visible peer still sees the owner disappear cleanly, the queued retaliation beat is cancelled, and that peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP without waiting for disconnect completion
- [ ] Repeat the same pending-cadence teardown with `/phase_select` and confirm the owner transitions back to character select, any visible peer still sees the owner disappear cleanly, the queued retaliation beat is cancelled, and that peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP while a later fresh bootstrap still requires the owner to reselect the mob
- [ ] Repeat the same pending-cadence teardown with an abrupt socket close or client disconnect and confirm any visible peer still sees the owner disappear cleanly, the queued retaliation beat is cancelled, and that peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP without waiting for a later reconnect
- [ ] If the content-loaded mob has a bootstrap EXP-only death reward configured, drive one full kill and confirm the killing client receives `GC DEAD(mob_vid)` -> `GC TARGET(0, 0)` -> one self-only `PLAYER_POINT_CHANGE(POINT_EXP = 3)` with the configured EXP amount and new EXP total
- [ ] If the content-loaded mob has a bootstrap gold-only death reward configured, drive one full kill and confirm the killing client receives `GC DEAD(mob_vid)` -> `GC TARGET(0, 0)` -> one self-only `PLAYER_POINT_CHANGE(POINT_GOLD = 11)` with the configured gold amount and new gold total
- [ ] Reconnect after a successful EXP-only or gold-only reward kill and confirm the selected character snapshot reflects the persisted reward total
- [ ] Repeat with a drop-only reward descriptor in a debug harness if available and confirm the kill returns `GC DEAD(mob_vid)` -> `GC TARGET(0, 0)` -> one self-only `ITEM_GROUND_ADD` at the killer's current coordinates plus one self-only `ITEM_OWNERSHIP` for the killer per configured drop vnum, with no inventory/account persistence mutation from the drop reward alone
- [ ] When loading a self-contained authored content bundle that includes top-level `item_templates`, confirm every configured `reward_drop_vnums` entry in `spawn_groups` is backed by one of those item templates; a bundle with a dangling drop vnum should be rejected by the loopback content-bundle import before any runtime actor replacement occurs
- [ ] If a retaliation-driven owner is already at `0` HP, try an operator/debug ground-gold drop registration for that owner and confirm it fails closed with no `ITEM_GROUND_ADD`, no `ITEM_OWNERSHIP`, and no live ground item occupying the map
- [ ] Drive one full target -> hit -> zero-HP death -> timed respawn -> fresh reselect cycle against that content-loaded mob
- [ ] Re-export or otherwise inspect authored content and confirm the actor still round-trips as `spawn_groups`, not as an interaction-backed `static_actor`

Expected result:
- the first attackable content-loaded mob now comes from the authored `spawn_groups` seam instead of ad hoc runtime-only bootstrap registration
- its runtime combat loop still reuses the owned `training_dummy` profile semantics for HP, death, timed respawn, and the first fixed same-target `250ms` normal-attack cadence gate
- after the first accepted hit, the mob now owns one tiny aggro-lite gate: fresh third-party `TARGET` attempts fail closed while the engaged owner still lives, but that same still-live mob becomes targetable again if retaliation kills the current owner before mob death / respawn resets it
- while alive, each accepted owner-side hit also applies one deterministic immediate self-only HP decrement back to that engaged session, and the first accepted live hit now starts a delayed self-only follow-up cadence that keeps firing one beat at a time after each owned `1s` server timer while the same engagement remains live
- bootstrap death rewards now cover EXP-only and gold-only persisted point/currency rewards plus a first single-drop-vnum ground-item reward that is visible to the killer without mutating inventory/account persistence by itself
- that owner-side retaliation point-loss now clamps at `0` HP too; once the floor is reached the current slice emits self-only `DEAD(owner_vid)` plus self-only `TARGET(0, 0)`, tears down any already-open merchant preview with one self-only `GC::SHOP END` after that same floor transition, and later same-owner combat `TARGET` / `ATTACK` attempts fail closed too, without yet claiming broader player-death behavior
- that retaliation point-loss is currently runtime-only for the engaged selected session, so a fresh `/phase_select` or reconnect bootstrap still rebuilds the owner's points from persisted state rather than carrying the just-finished retaliation loss across sessions, while later successful `/use_item`, `/equip_item`, and `/unequip_item` saves still persist their own authored use/equip point delta plus the associated item/equipment mutation and a still-live practice mob keeps its current runtime-owned HP and still requires a fresh post-recovery `TARGET`
- same-target normal `ATTACK` attempts inside the owned `250ms` cadence window fail closed without refreshing target HP, without appending another immediate retaliation tick, and without resetting delayed retaliation timing
- while one delayed follow-up beat is already pending, extra accepted hits should not stack, accelerate, or reset the current cadence timer yet
- if that engagement loses target intent first — either by replacing the selected practice-mob target or by movement / sync forcing a self `TARGET(0, 0)` clear — the pending delayed follow-up beat should fail closed, and the abandoned still-live mob should become targetable again at its current runtime-owned HP instead of staying orphan-locked behind the old owner
- a successful transfer / rebootstrap during that pending window also counts as an immediate cadence-reset boundary: the owner gets the normal self transfer burst with no late delayed retaliation beat, and a still-visible source-map peer can immediately retarget the same still-live mob at its current runtime-owned HP
- a same-socket `/quit`, `/logout`, or `/phase_select` also counts as an immediate owner-disappearance boundary for that cadence in the current slice, and abrupt session close does too: the owner leaves shared-world visibility right away, the pending delayed follow-up beat is cancelled, and the still-live practice mob becomes targetable again at its current runtime-owned HP without waiting for disconnect, close, or later fresh bootstrap completion
- once retaliation has already driven the owning character to `0` HP, later owner-side combat `TARGET` / `ATTACK`, owner `MOVE` / `SYNC_POSITION`, owner static-actor `INTERACT`, owner merchant-buy, owner slash `/use_item`, owner slash `/inventory_move`, owner slash `/equip_item` / `/unequip_item`, owner peer-facing `CHAT` / `WHISPER`, and owner self-only `CHAT_TYPE_INFO` attempts fail closed too, while `/quit`, `/logout`, and `/phase_select` keep their current separate command behavior
- once retaliation has already driven the owning character to `0` HP, later peer-originated exact-name `WHISPER` requests aimed at that same still-connected owner also fail closed with no queued target delivery and no synthetic `WHISPER_TYPE_NOT_EXIST` fallback yet
- once retaliation has already driven the owning character to `0` HP, later peer-originated local `CHAT_TYPE_TALKING` still returns the live sender's ordinary self `GC_CHAT` echo, but the zero-HP owner is skipped from queued peer delivery
- once retaliation has already driven the owning character to `0` HP, later server-originated `CHAT_TYPE_NOTICE` broadcasts still reach other connected live sessions, but queued notice delivery skips that same still-connected zero-HP owner entirely under the current bootstrap global notice path
- once retaliation has already driven the owning character to `0` HP, later fresh visible peer joins still queue their ordinary peer-entry burst for other live recipients, but the same queued `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` delivery skips that same still-connected dead owner under the current shared-world join path
- once retaliation has already driven the owning character to `0` HP, later live equip / unequip mutations still queue their ordinary queued peer `CHARACTER_UPDATE` appearance refresh for other live recipients that stay visible across that mutation, but that same queued same-visible-set peer-update delivery skips that same still-connected dead owner under the current shared-world stable visibility-transition path
- once retaliation has already driven the owning character to `0` HP, later movement- or `SYNC_POSITION`-driven peer visibility re-entry bursts still queue their ordinary peer-entry burst for the live moving/syncing origin, but the same queued `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` delivery skips that same still-connected dead owner under the current shared-world AOI rebuild path
- once retaliation has already driven the owning character to `0` HP, later transfer-driven peer visibility re-entry bursts still queue their ordinary peer-entry burst for the live transferred origin, but the same queued `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` delivery skips that same still-connected dead owner under the current shared-world transfer rebuild path
- once retaliation has already driven the owning character to `0` HP, loopback runtime/operator player snapshots now also surface that same still-connected owner explicitly with `dead: true`, including `/local/players`, `/local/visibility`, and structured `/local/relocate-preview` / `/local/transfer` player entries
- once retaliation has already driven the owning character to `0` HP, later stale-ownership reclaim cleanup for another visible peer also skips that same still-connected dead owner: the live replacement session still completes its reclaim/re-entry flow, but the dead owner receives no queued stale-peer `CHARACTER_DEL` or compensating re-entry burst during that cleanup
- when either the immediate retaliation tick or the delayed follow-up beat reaches that same owner floor, currently visible live peers also receive one queued `GC DEAD(owner_vid)` while already-dead connected recipients are skipped and broader corpse/respawn choreography still stays out of scope
- once retaliation has already driven the owning character to `0` HP, later visible practice-mob `GC DEAD(mob_vid)` fanout plus that mob's later timed respawn rebuild burst still reach other live viewers normally, but those same queued non-player lifecycle frames skip the still-connected dead owner entirely
- authored respawn ownership is anchored to the spawn-group `ref`, while live entity IDs and death/respawn timing remain runtime-owned
- import/export stays deterministic: the practice mob keeps round-tripping through `spawn_groups` + `combat_profile` without pretending broader mob AI already exists

---
## 7. Optional bootstrap chat-scope checks

These checks are useful but secondary.
Do not block a general smoke pass on them unless the milestone specifically targets chat behavior.

### 7.1 Party chat bootstrap

- [ ] Send a party-type message if the current client path allows it

Expected result:
- the current bootstrap fanout behaves consistently with implementation and does not destabilize the session

### 7.2 Shout bootstrap

- [ ] Send a shout if the current client path allows it

Expected result:
- the current shout bootstrap behavior works without disconnecting the client

Note:
- current party/guild/shout behavior is still bootstrap-only and not backed by full gameplay systems

---

## 8. Regression watchlist

Record any of these immediately if seen:

- [ ] Channel list missing or wrong online state
- [ ] Login succeeds but selection screen fails
- [ ] Character create/delete desyncs the selection screen
- [ ] Enter-game disconnect
- [ ] Spawn succeeds but the first movement disconnects the session
- [ ] Two clients cannot coexist in-world
- [ ] Players do not see each other on the same bootstrap map
- [ ] Peer movement does not replicate
- [ ] Local chat crashes or disconnects a client
- [ ] Whisper exact-name routing is broken
- [ ] Reconnect loses the QA character unexpectedly
- [ ] Server logs show panic, fatal errors, or a restart loop

When a regression appears, record:
- exact checklist step number
- character names used
- whether the legacy server was also running
- recent `authd` log lines
- recent `gamed` log lines

---

## 9. Do NOT treat these as failures yet

These are currently out of scope for the present server state unless the milestone explicitly says otherwise:

- [ ] inventory UX completeness
- [ ] full equipment UX/stat semantics beyond the current bootstrap equip/unequip + shared-world appearance refresh slice
- [ ] item use
- [ ] full merchant UI semantics beyond the current bootstrap open / buy / close slice, or any sell flow
- [ ] inventory or currency mutation from non-merchant NPC interactions
- [ ] broader mob/skill combat beyond the current `training_dummy` / content-loaded `practice_mob` target -> hit -> death -> timed-respawn loop
- [ ] quest acceptance, progression, or rewards
- [ ] broader player death / respawn systems beyond the current retaliation-owned `DEAD`, `/restart_here`, and `/restart_town` bootstrap seams
- [ ] random loot tables, party/contribution reward splits, level-up/stat recalculation choreography, corpse gameplay, or public-loot expiry beyond the current deterministic bootstrap EXP/gold/drop-vnum reward descriptor seam
- [ ] multi-channel real behavior
- [ ] polished client-facing warp/loading choreography

Important note:
- the project has operator-side transfer primitives and ongoing runtime transfer work
- for current QA, validate only the existing bootstrap warp/rebootstrap path; polished final warp/loading choreography is still not a general pass/fail gate

---

## 10. Exit criteria for a healthy current build

A current build is a good candidate when all of these pass:

- [ ] channel visible online
- [ ] valid login works
- [ ] selection screen is usable
- [ ] create/select/enter-game work
- [ ] single-client movement works
- [ ] reconnect works
- [ ] when authored QA NPC content is loaded, supported NPC smoke checks (`info` / `talk`, `shop_preview`, `warp`) pass without disconnecting the client
- [ ] with two clients: peer visibility works
- [ ] with two clients: peer movement works
- [ ] with two clients: local chat and whisper work
- [ ] with two clients: peer equip/unequip appearance refresh works
- [ ] with two clients: late-join peer appearance after runtime equip/unequip works
- [ ] with two clients + radius AOI: move-into-range peer appearance after runtime equip/unequip works
- [ ] with two clients + transfer path: transfer-driven peer appearance after runtime equip/unequip works
- [ ] with two clients + reconnect: reconnect-driven peer appearance after runtime equip/unequip works
- [ ] when authored/runtime-marked training dummies are available: the target -> hit -> death -> timed-respawn loop works and requires fresh reselection after respawn
- [ ] when authored reward descriptors are loaded on content practice mobs: deterministic EXP/gold point-change feedback, owned ground-drop feedback, persisted scalar rewards, and non-persistent drop rewards match the bootstrap reward checklist
- [ ] no crash or forced disconnect occurs during the run
