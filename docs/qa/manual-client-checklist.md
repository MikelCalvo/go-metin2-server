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
- [ ] If QA depends on authored item templates, `POST /local/item-templates/validate` returns the expected template summary and the committed snapshot has a present JSON `templates` array rather than a missing/null collection; item-template names must not contain embedded NUL bytes; if validation reports only disposable `.item-templates-*.json` crash-temp residue, clean it with `POST /local/item-templates/crash-temps/cleanup` before the run
- [ ] If QA imports authored NPC/content bundles, `POST /local/content-bundle/validate` accepts the exact candidate fixture before import; invalid UTF-8, JSON `null` collection fields, dangling refs, unsupported future interaction kinds such as unfrozen quest/dialog metadata, missing merchant/reward item templates, or other validation failures should be fixed in the bundle instead of imported into the live runtime
- [ ] Quest-state work remains a small server-side persistence/content-trigger primitive, not a quest UI; use `POST /local/quest-state/validate` to preflight the configured snapshot, `GET /local/quest-state` for the whole committed store overview, `GET /local/content-bundle/quest-state`, `GET /local/content-bundle/quest-state/characters/{character}`, `GET /local/content-bundle/quest-state/quests/{quest_ref}`, or `GET /local/content-bundle/quest-state/flags/{character}/{quest_ref}/{flag}` to inspect live exported bundle summaries, `POST /local/content-bundle/import-preview/quest-state` for a compact current-vs-candidate quest-state-only import preview, `POST /local/content-bundle/import-preview/quest-state/characters/{character}`, `POST /local/content-bundle/import-preview/quest-state/quests/{quest_ref}`, or `POST /local/content-bundle/import-preview/quest-state/flags/{character}/{quest_ref}/{flag}` to inspect candidate bundle quest-state deltas without mutating runtime state, `POST /local/quest-state/transition-preview` to dry-run operator-authored compare-and-set flag changes without writing the snapshot, `POST /local/quest-state/transition` only as the loopback mutating compare-and-set harness for those same flags, and `POST /local/quest-state/crash-temps/cleanup` only for disposable interrupted `.quest-state-*.json` temp writes; no manual client quest window, rewards, or branching script behavior should be expected yet
- [ ] Before destructive authored static-actor / spawn-content restore experiments, create a loopback-only manifested backup with `POST /local/static-actors/backup` and dry-run it with `POST /local/static-actors/backup/validate`; invalid UTF-8 backup manifests or checksum/size/coverage drift should fail validation instead of being restored; if testing restore, drain connected game sessions first and restore only into an empty active static-actor store with `POST /local/static-actors/restore`, then confirm `/local/persistence/status` reports the restored `backup_manifest` and that live NPC/spawn actors match the restored snapshot
- [ ] Before destructive interaction-definition migration/import experiments, create a loopback-only manifested backup with `POST /local/interaction-store/backup` and dry-run it with `POST /local/interaction-store/backup/validate`; invalid UTF-8 backup manifests or checksum/size/coverage drift should fail validation instead of being restored; if testing restore, drain connected game sessions first and restore only into an empty active interaction store with `POST /local/interaction-store/restore`
- [ ] Before destructive item-template migration/import experiments, create a loopback-only manifested backup with `POST /local/item-templates/backup` and dry-run it with `POST /local/item-templates/backup/validate`; invalid UTF-8 backup manifests or checksum/size/coverage drift should fail validation instead of being restored; if testing restore, drain connected game sessions first and restore only into an empty active item-template store with `POST /local/item-templates/restore`. For item-template export/backfill QA, `GET /local/item-templates/exports/item-template-state` should report the current `item_template_refine_info` boundary and include deterministic `refine_infos` / `refine_materials` rows for authored refine-preview templates without implying accepted refine-result execution.

Expected result:
- the server is stably up before opening the client
- authored item-template/content-bundle state is either clean or has only intentionally cleaned crash-temp residue before item/economy/NPC checks

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
- [ ] If packet logging is available, confirm the initial `ITEM_SET` for template-authored fixtures carries the owned item `flags` bits for `refineable` / `save` / `stackable` / `sell_count_per_gold` / `slow_query` / `rare` / `unique` / `make_count` / `irremovable` / `confirm_when_use` / `quest_use` / `quest_use_multiple` / `log` / `applicable`, carries the owned `anti_flags` bits for `anti_get`, transfer/job/sex/empire guards, storage/shop metadata bits (`anti_save`, `anti_pk_drop`, `anti_myshop`, `anti_safebox`), projects authored socket/attribute display arrays and the template `highlight` byte, and leaves unowned bits zero
- [ ] Bind that carried cell to an item quickslot and also keep an unrelated skill/command quickslot that uses the same byte slot value if the client setup allows it
- [ ] Use the item once from inventory or the quickslot

Expected result:
- packet-originated `ITEM_USE` first receives a self-only server `ITEM_USE` echo (`0x0512`) for the consumed cell and item `vnum`; the slash `/use_item <slot>` harness does not emit this packet-only echo
- the client receives a `PLAYER_POINT_CHANGE` from the template-authored `use_effect`
- when the consumed template authors non-zero `use_effect.special_effect_type`, the client also receives one self-only `SPECIAL_EFFECT` for the selected character after the point/item refresh and before the placeholder info chat; omitted or zero metadata preserves the older no-special-effect burst
- template-authored negative `use_effect.point_delta` consumables are allowed in this bootstrap path: the self-only `PLAYER_POINT_CHANGE.amount` should be negative, `value` should be the decreased signed point value, and the normal item refresh plus placeholder info message still follow
- template-authored `use_effect.consume_count` values above `1` consume exactly that many stack units on success; if the live stack has fewer units than the authored count, the request fails closed with no point change, item refresh, quickslot change, placeholder chat, or persisted-state mutation
- if more than one item remains in the stack, the carried cell refreshes with the decremented count, preserves authored socket/attribute display arrays in the `ITEM_UPDATE`, and both item and non-item quickslots for that still-occupied cell remain unchanged
- if the consumed stack reaches zero, the carried cell disappears and every item quickslot referencing that cell is cleared in deterministic quickslot-position order; unrelated skill/command quickslots remain
- locked carried stacks fail closed: no point change, item refresh, quickslot change, or placeholder chat is visible
- if a corrupt/disposable fixture has duplicate live items in the same carried cell, `ITEM_USE` fails closed with no point change, item refresh, quickslot change, placeholder chat, or persisted-state mutation
- templates marked `anti_stack`, `anti_get`, `anti_drop`, `anti_give`, or `anti_sell` also fail closed for direct consumable use: no point change, item refresh, quickslot change, or successful-use placeholder chat is visible; if the same template authors `use_reject_message`, exactly one self-only info-chat rejection with that text is visible instead of the older silent rejection
- templates marked `confirm_when_use` are usable through ordinary direct consumable use after the client-local confirm dialog: accepted use shows the same point change, item refresh, quickslot sync, and optional special-effect / info chat already owned by non-confirm consumables; `confirm_when_use` itself does not invent a server ack packet. Transfer / selected-character / authored `use_reject_message` guards still fail closed before mutation; `quest_use` / `quest_use_multiple` / `applicable` remain fail-closed
- templates marked `quest_use`, `quest_use_multiple`, or `applicable` also fail closed for direct consumable use until those item-family flows are owned: no point change, item refresh, quickslot change, or successful-use placeholder chat is visible; if the same template authors `use_reject_message`, exactly one self-only info-chat rejection with that text is visible instead of the older silent rejection
- if an authored `use_reject_message` rejection is triggered while a merchant window is open, the client should first receive a self-only `GC::SHOP END` and later merchant `SHOP END` / `SHOP BUY` attempts on that stale window should fail closed until the merchant is opened again; if the same rejection is triggered while a bootstrap exchange shell is open, the requester should first receive self `GC::EXCHANGE END`, the paired peer should receive one queued `GC::EXCHANGE END`, then the requester should receive the rejection chat, with no item, point, quickslot, gold, exchange display, or persisted-state mutation
- templates with authored job, sex, empire, or `min_level` restrictions for the selected character fail closed the same way: silent/no-frame when `use_reject_message` is omitted, otherwise exactly one self-only info-chat rejection with that authored text
- selected characters at the bootstrap zero-HP floor cannot consume carried items; the request fails closed before stack, quickslot, point, placeholder-chat, or persisted-state mutation
- a carried stack whose live count already exceeds its loaded template-authored `max_count` fails closed before stack, quickslot, point, placeholder-chat, or persisted-state mutation
- a consumable whose resolved template `max_count` cannot fit the current one-byte item refresh count range fails closed before stack, quickslot, point, placeholder-chat, or persisted-state mutation
- a consumable whose template-authored point delta would overflow the bootstrap signed 32-bit point value fails closed before stack, quickslot, point, placeholder-chat, or persisted-state mutation
- a consumable whose template-authored negative point delta would underflow the bootstrap signed 32-bit point value fails closed before stack, quickslot, point, placeholder-chat, or persisted-state mutation
- if operator/test fixtures can force account persistence failure during an otherwise-valid `ITEM_USE`, the consume fails closed: no point change, item refresh, quickslot change, or placeholder chat is visible, and reconnect shows the pre-use inventory/points/quickslots unchanged
- the placeholder `CHAT_TYPE_INFO` message uses template-authored `use_effect.info_message` when non-empty, otherwise it falls back to `use_effect.message` for older templates and the built-in bootstrap fallback; authored snapshots with embedded NUL bytes in either field should fail item-template validation/runtime startup rather than reaching the client as truncated chat text

### 4.5.2 Drag stack onto stack (`ITEM_USE_TO_ITEM`)

- [ ] Put two compatible stackable carried items with the same `vnum` and different item instance IDs into separate inventory cells
- [ ] Drag one stack onto the other stack
- [ ] If an exchange shell is open with a visible peer, repeat the drag while one of those stacks is displayed in the exchange window

Expected result:
- compatible stacks consolidate up to the template-authored `max_count`
- authored stack `max_count` values above the current bootstrap client count range (`255`) are rejected at item-template load time, not accepted as runtime use-to-item behavior
- the consumed source cell disappears only on a full merge
- if the target has only partial room, both source and target counts refresh, and item/non-item quickslots bound to either still-occupied cell remain unchanged
- all item quickslots for a removed source cell are cleared in deterministic quickslot-position order on full merge, target item quickslots remain stable on full merge even when both source and target cells were quickslotted before the drag, and unrelated skill/command quickslots remain
- if the stack consolidation succeeds while an exchange shell is open, the requester receives one self-only `GC::EXCHANGE END` before the item/quickslot merge refresh frames, the paired peer receives one queued `GC::EXCHANGE END`, and no exchange finalization/result frames appear
- restricted or invalid states (`anti_stack`, transfer anti-flags, missing/non-stackable/malformed/mismatched templates, source/target `vnum` mismatches, locked source/target stacks, selected-character job/sex/empire/min-level restrictions, duplicate source/target item instance IDs, duplicate live occupancy of the source or target carried cell, already-full targets, source/target counts already above template `max_count`, or selected characters at the bootstrap zero-HP floor) fail closed with no visible mutation; for `anti_stack`, both carried stacks and item quickslots should remain unchanged
- a `min_level` restriction above the selected character's level or a selected character at the bootstrap zero-HP floor leaves both carried stacks and any source-cell item quickslot unchanged even when the source and target are otherwise compatible
- if operator/test fixtures can force account persistence failure during an otherwise-valid full or partial `ITEM_USE_TO_ITEM` stack consolidation, the merge fails closed: no item refresh, quickslot change, or exchange teardown is visible, and reconnect shows the pre-merge inventory/quickslots unchanged

### 4.5.3 Retarget a quickslot tuple (`QUICKSLOT_ADD`)

- [ ] Bind a carried inventory item cell to an item quickslot
- [ ] Bind the same carried item cell to a different item quickslot position
- [ ] If the client setup allows it, repeat the retarget with one skill binding and one command binding
- [ ] If the client setup allows it, keep an unrelated quickslot of a different type whose byte slot value matches the retargeted tuple

Expected result:
- the older same-type quickslot tuple is cleared before the new binding is added
- if a bootstrap exchange shell is open on the same socket, an accepted quickslot add closes it first: the requester receives `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, then the requester sees the quickslot delete/add refresh frames, with no exchange finalization/result frames
- the new item/skill/command quickslot binding persists after reconnect
- if operator/test fixtures can force account persistence failure during an otherwise-valid `QUICKSLOT_ADD`, the binding fails closed: no quickslot refresh frame is visible, existing quickslots remain unchanged, and reconnect shows no persisted quickslot mutation
- binding an item quickslot to a locked, malformed, authored-missing-template, mismatched-template, or over-template-max carried item fails closed: no `QUICKSLOT_ADD` is visible, existing quickslots remain unchanged, and reconnect shows no persisted quickslot mutation. Missing-file/empty-store fallback template boot still permits older ad-hoc item `vnum` smoke fixtures under the live-item validation path.
- account/login-ticket snapshots that somehow contain the same non-item skill/command `{type, slot}` tuple at two different bar positions are now invalid and fail closed on save/load instead of replaying duplicate skill/command bindings; duplicate item-cell quickslot fixtures remain loadable for current item-removal cleanup tests
- unrelated quickslots of a different type with the same byte slot value remain unchanged
- if stale/reclaimed-socket QA tooling is available, a reclaimed old socket may still see its own quickslot refresh frames, but reconnecting / inspecting the fresh authoritative session shows no persisted quickslot change from that stale socket

### 4.5.4 Delete an occupied quickslot (`QUICKSLOT_DEL` / `QUICKSLOT_ADD` type none)

- [ ] Delete a quickslot position that currently contains an item, skill, or command binding
- [ ] If the client path emits `QUICKSLOT_ADD` with `slot.type = 0` for clearing, clear an occupied quickslot through that path too
- [ ] Delete a different quickslot bar position that is currently empty

Expected result:
- deleting the occupied position clears that binding and persists after reconnect
- if a bootstrap exchange shell is open on the same socket, an accepted quickslot delete closes it first: the requester receives `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, then the requester sees `GC::QUICKSLOT_DEL`, with no exchange finalization/result frames
- a type-none `QUICKSLOT_ADD` clear returns the same visible delete behavior: the occupied binding is cleared, no new none binding remains, and reconnect shows the binding gone
- if packet tooling can emit a malformed type-none `QUICKSLOT_ADD` with non-zero `slot.pos`, it fails closed: no quickslot refresh frame is visible, the occupied binding remains, and reconnect shows no persisted mutation
- deleting the empty position fails closed: no quickslot refresh frame is visible, existing quickslot bindings remain, and reconnect shows no persisted change

### 4.5.5 Swap quickslots (`QUICKSLOT_SWAP`)

- [ ] Swap two occupied quickslot positions
- [ ] Swap one occupied quickslot position with an empty quickslot position
- [ ] Attempt to swap two empty quickslot positions

Expected result:
- occupied-to-occupied swaps exchange the bindings and persist after reconnect
- occupied-to-empty swaps move the binding to the empty target position and persist after reconnect
- if a bootstrap exchange shell is open on the same socket, an accepted quickslot swap closes it first: the requester receives `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, then the requester sees `GC::QUICKSLOT_SWAP`, with no exchange finalization/result frames
- empty-to-empty swaps fail closed: no quickslot refresh frame is visible, existing quickslot bindings remain, and reconnect shows no persisted change

### 4.5.6 Drop and pick up a carried item or gold (`ITEM_DROP` / `ITEM_PICKUP`)

- [ ] Drop a known template-backed carried item stack in a safe visible location
- [ ] Repeat with a disposable item template marked with a transfer guard such as `anti_drop` and a non-empty `drop_reject_message`
- [ ] If packet logging is available, repeat as a counted `ITEM_DROP2` partial-stack drop and confirm the source-slot `ITEM_UPDATE` preserves the template-authored socket/attribute display arrays while updating only the remaining count
- [ ] If packet logging is available, confirm each `GC::ITEM_GROUND_ADD` payload decodes in client-facing order as `x/y/z` first, then `vid`, then `vnum`; the item should appear at the dropper's current coordinates with the expected template id rather than at obviously bogus coordinates caused by interpreting `vid` as `x`
- [ ] Pick up the same temporary ground handle while still in range
- [ ] Drop a small amount of gold/elk through the client gold-drop path and, if QA tooling can vary the packed item position, repeat with a non-carried position while the gold amount is non-zero
- [ ] If QA can load a disposable template for the bootstrap gold marker (`vnum = 1`) with `pickup_range`, test both a long-range accepted reclaim and a short-range out-of-reach rejection
- [ ] If QA can load a disposable template for the bootstrap gold marker (`vnum = 1`) with `anti_give` plus a `pickup_reject_message`, have a visible peer attempt to pick up the owner's still-owned gold marker before the owner reclaims it
- [ ] If possible in the QA fixture, repeat with a deliberately missing, malformed, mismatched, or ground-count-over-template-`max_count` authored item-template/state fixture for that `vnum`
- [ ] If possible in the QA fixture, repeat pickup with a transfer-guarded or selected-character-restricted template that authors a non-empty `pickup_reject_message`; also verify authored item-template validation rejects a `pickup_reject_message` on a template with no owned pickup rejection guard before gameplay testing starts
- [ ] While a bootstrap exchange shell is open with a visible peer, pick up one pending ground handle from the same socket; if QA can alter the template before pickup, repeat with a guarded pickup template that emits `pickup_reject_message`

Expected result:
- valid pickup removes the ground actor, refreshes the carried inventory slot or compatible stack according to the authored stack metadata, preserves template-authored socket/attribute display arrays in compatible-stack `ITEM_UPDATE` refreshes, preserves existing item/non-item quickslots for a compatible merge target cell, shows the normal pickup notice, closes any active same-socket exchange shell first with self/peer `GC::EXCHANGE END`, and does not produce a second/duplicate delayed ground-delete for the collector after the direct pickup response
- `GC::ITEM_GROUND_ADD` uses the frozen TMP4-compatible wire order `x/y/z/vid/vnum`, so ground actors render at the server-selected world position with the correct item identity
- a non-zero gold/elk field follows the gold-drop path regardless of the packed item position: gold decreases, the carried inventory remains unchanged, and a gold ground marker appears; gold pickup restores gold only when the positive point-change total still fits the current bootstrap signed 32-bit carrier and the collector is within the marker's reach, otherwise it fails closed without removing the marker so it can be retried after the recipient state/range becomes valid; when the authored `vnum = 1` gold marker template carries non-zero `pickup_range`, that authored distance replaces the default 300-unit reach for the marker; when the authored `vnum = 1` gold marker template is `anti_give`, visible-peer pickup returns the template-authored pickup rejection text, queues no owner frames, mutates neither owner nor collector gold, and leaves the marker available for owner retry
- loaded drop template metadata whose carried stack already exceeds the authored `max_count`, is transfer-guarded with `anti_get` / `anti_drop` / `anti_give` / `anti_sell` / `anti_stack`, is rejected by selected-character job/sex/empire/min-level restrictions, or is attempted while the selected character is at the bootstrap zero-HP floor fails closed: no ground actor, no carried-slot deletion/update, and no quickslot mutation is visible; transfer-guard rejections show the template-authored `drop_reject_message` as a self-only info chat when present and otherwise use the deterministic fallback text `You cannot drop this item.` Selected-character restriction rejections show the authored `drop_reject_message` when present and otherwise remain silent/no-frame.
- if stale/reclaimed-socket QA tooling is available, an old reclaimed socket may see its own self-local carried-slot deletion/update and source quickslot delete frames for a valid item drop, but it must not create a ground actor, ownership label, delayed peer ground visibility, or persisted account mutation; the fresh authoritative session should still be able to drop the original item afterward.
- if a corrupt/disposable fixture reaches the shared-world ground-handle seam with stale equipment-slot metadata on an otherwise unequipped ground snapshot, registration fails closed and no temporary ground actor becomes available
- if a corrupt/disposable fixture has duplicate live items in the same carried cell, `ITEM_DROP` / `ITEM_DROP2` fails closed with no ground actor, no carried-slot deletion/update, no quickslot mutation, and no persisted-state mutation
- if operator/test fixtures can force account persistence failure during an otherwise-valid drop, the drop fails closed with no ground actor, no carried-slot deletion/update, no quickslot mutation, and reconnect shows the pre-drop inventory/quickslots unchanged
- if operator/test fixtures can force account persistence failure during an otherwise-valid pickup after a successful drop, the pickup fails closed with no inventory/gold refresh and the temporary ground handle remains available for a later valid retry
- missing, malformed, mismatched, or ground-count-over-template-`max_count` authored pickup template metadata fails closed: no item pickup notice, no inventory mutation, and the ground handle remains available for a later valid retry; a valid authored equipment template is accepted into carried inventory and must not auto-equip
- fallback/no-template pickup fixtures whose ground stack count exceeds the current one-byte item refresh range (`255`) fail closed before item pickup notice, inventory mutation, or ground-handle removal
- loaded pickup template metadata marked `anti_get` / `anti_give` / `anti_stack` or restricted by the selected character's job/sex/empire/min-level metadata also fails closed, emits guarded template-authored `pickup_reject_message` as self-only info chat when present and otherwise the bootstrap inventory-full info message, closes any active same-socket exchange shell before that info chat, and leaves the ground handle available for a later valid retry; `pickup_reject_message` authored without an owned pickup guard is an invalid template snapshot rather than a runtime pickup policy
- selected characters at the bootstrap zero-HP floor cannot pick up visible ground items; `ITEM_PICKUP` fails closed with no item pickup notice, inventory/gold mutation, or ground-handle removal
- if a corrupt/disposable fixture already has the same non-zero item instance ID in carried inventory or equipment as the temporary ground item being picked up, pickup fails closed with the bootstrap inventory-full info message, no inventory mutation, and the ground handle remains available for a later valid retry

### 4.5.7 Drag inventory stack onto inventory stack (`ITEM_MOVE`)

- [ ] Put two carried stacks with the same `vnum` into separate inventory cells
- [ ] Confirm their loaded item template is stackable and not `anti_stack`, `anti_drop`, `anti_give`, or `anti_sell`, then drag one stack onto the other through normal inventory movement
- [ ] Repeat with an otherwise matching template that is marked `anti_stack`, `anti_drop`, `anti_give`, or `anti_sell`

Expected result:
- stackable, non-`anti_stack` items merge only up to the template-authored `max_count`
- if the target has only partial room, both source and target counts refresh, those `ITEM_UPDATE` frames preserve template-authored socket/attribute display arrays while changing only count, source and target item quickslots remain stable, and non-item quickslots with the same byte slot remain unchanged
- an exact counted or zero-count full-stack merge removes the source cell, refreshes the destination count with an `ITEM_UPDATE` that preserves template-authored socket/attribute display arrays, deletes every source item quickslot in deterministic quickslot-position order, leaves target item quickslots stable, and leaves unrelated skill/command quickslots unchanged
- `anti_stack` and non-stackable same-`vnum` full-stack drag requests do not merge; when both occupied cells still fit authored template count bounds, they use the full-stack carried swap path instead, refreshing both cells and retargeting item quickslots for the moved source identity while preserving unrelated skill/command quickslots
- incompatible occupied-destination full-stack swaps require both source and target templates when an authored item-template snapshot is loaded; successful swaps refresh each cell with that item's authored flags/anti-flags/highlight/socket/attribute metadata, delete stale destination item quickslots, retarget source item quickslots to the destination cell, and persist the swapped carried cells
- `anti_drop`, `anti_give`, and `anti_sell` templates, same-`vnum` merge attempts with missing source-template metadata in an explicitly authored item-template snapshot, occupied-source/target cells whose live counts already exceed authored `max_count`, incompatible swaps with missing source or target template metadata in an explicitly authored item-template snapshot, duplicate source/target cell occupancy fixtures, and corrupt fixtures where source and target cells carry the same item instance ID fail closed: no item counts change, no source cell disappears, no quickslot change is persisted, and no item refresh frames are visible

### 4.5.8 Equip a carried item (`ITEM_MOVE` to equipment cell)

- [ ] Put a known template-backed equipment item in a carried inventory cell
- [ ] Confirm the template's authored `equip_slot` matches the destination equipment cell, has no selected-character job/sex/empire/`min_level` restriction, and is not guarded with `anti_stack`, `anti_drop`, `anti_give`, or `anti_sell`; include one `hair` item when validating appearance projection
- [ ] Confirm authored equipment templates used for QA are non-stackable (`stackable = false`, `max_count = 1`); a fixture that combines `stackable = true` with `equip_slot` should fail item-template validation/runtime boot before gameplay testing starts
- [ ] Drag the carried item into its matching equipment cell
- [ ] With that cell already occupied by another matching wearable, drag a second carried wearable onto the same equipment cell (and separately try `/equip_item` if the slash harness is available)
- [ ] Repeat with the same item shape but a selected-character job/sex anti-flag that should reject the character
- [ ] Repeat with an empire anti-flag (`anti_empire_a` / `anti_empire_b` / `anti_empire_c`) that matches the selected character's empire, and separately with a `min_level` higher than the selected character's level; include one of those fixtures with non-empty `equip_reject_message`
- [ ] Repeat with authored metadata whose `equip_slot` or `vnum` does not match the carried item/destination cell
- [ ] Repeat with otherwise matching equipment metadata guarded by one of `anti_stack`, `anti_drop`, `anti_give`, or `anti_sell`
- [ ] While an exchange shell is open with a visible peer, repeat a guarded equip attempt that returns template-authored `equip_reject_message`, and separately repeat an occupied-wear equip reject

Expected result:
- allowed equipment moves from carried inventory to the authored equipment cell, emits the self-only item refresh burst, deletes item quickslots bound to the cleared carried source cell, leaves unrelated skill/command quickslots with the same byte slot value unchanged, applies any template-authored `equip_effect` point change only after the matching item is actually equipped in that authored cell, and refreshes visible `CHARACTER_UPDATE.parts` for projected `body` / `weapon` / `head` / `hair` equipment
- if the wearable template authors non-zero `appearance_vnum`, `CHARACTER_UPDATE.parts` and peer-visible appearance should use that authored visible id while the equipment `ITEM_SET.vnum`, persisted equipment item, and later inventory/equipment inspection still show the original item `vnum`
- equipment templates may now author negative `equip_effect.point_delta` penalties; on equip the visible `PLAYER_POINT_CHANGE.amount` should be the negative authored value, and on unequip the inverse positive amount should restore the point value
- equipping onto an already-occupied matching wear cell with a compatible empty source cell swaps the worn item onto that source cell and the carried wearable onto the wear cell; expect self-only `ITEM_SET(source)` + `ITEM_SET(equipment)` + any invert-then-apply `PLAYER_POINT_CHANGE` frames + `CHARACTER_UPDATE`, with inventory/equipment/points persisted across reconnect; locked / non-swappable occupied destinations still fail closed with exactly one self-only `CHAT_TYPE_INFO` `You are already wearing equipment.` and no mutation; `irremovable` previous worn templates keep the owned unequip reject chat
- selected-character job/sex/empire/`min_level` restricted, mismatched-`vnum`, mismatched-slot, or transfer-guarded equipment fails closed: no item refresh, no quickslot change, no point change, no carried/equipment mutation, and no persistence change; when the rejected template authors `equip_reject_message`, exactly one self-only info-chat rejection with that text is visible instead of the older silent rejection
- if the guarded equip rejection or non-swappable occupied-wear reject runs while an exchange shell is open, the requester first receives one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, then the requester receives the self-only `CHAT_TYPE_INFO` rejection; no exchange result/finalization frame or inventory/equipment/point/quickslot/persistence mutation appears; successful occupied-wear swap while an exchange shell is open closes with the same self/peer `GC::EXCHANGE END` ordering before the swap refresh/appearance burst
- disposable corrupted persistence fixtures where an `inventory` entry is already marked equipped, or an `equipment` entry is not marked equipped, should be rejected by account/login-ticket validation before the character can enter the normal inventory/equipment runtime path
- corrupt/disposable fixtures that try to apply an equipment point effect without a matching valid equipped item in the authored equipment cell fail closed with no point mutation
- equipment whose template-authored `equip_effect` point delta would overflow the bootstrap signed 32-bit point value also fails closed before item, quickslot, point, or persistence mutation
- corrupt/disposable equipment fixtures whose live carried source count exceeds the authored template `max_count` fail closed before item refresh, quickslot cleanup, point, appearance, or persisted-state mutation
- if operator/test fixtures can force account persistence failure during an otherwise-valid equip, the equip fails closed: no item refresh, quickslot change, point change, appearance update, or persisted inventory/equipment mutation is visible

### 4.5.9 Unequip a template-backed equipment item (`ITEM_MOVE` from equipment cell)

- [ ] Start with a known template-backed equipped item whose authored `equip_slot` matches the worn cell
- [ ] Confirm the item template has the current narrow `equip_effect` point metadata
- [ ] Drag the worn item back into an empty carried inventory cell
- [ ] Repeat with otherwise matching equipment metadata marked `irremovable`
- [ ] Repeat with a corrupt/disposable fixture where the removal metadata does not match the just-removed item `vnum`
- [ ] While an exchange shell is open with a visible peer, repeat an `irremovable` unequip rejection

Expected result:
- allowed unequip emits the self-only equipment clear, carried-cell set, any template-authored inverse `PLAYER_POINT_CHANGE` when the item has `equip_effect`, and appearance update; unequipping projected `hair` equipment restores `parts[3]` to the character's base `HairPart`
- the point-effect removal is backed by the just-removed item instance, so it still subtracts the authored delta after the item has moved out of the equipment slice
- `irremovable` removal metadata fails closed with one self-only `CHAT_TYPE_INFO` rejection; when the template authors `unequip_reject_message`, that text is shown, otherwise the deterministic fallback is `You cannot remove this item.`; no carried/equipment, point, appearance, quickslot, or persisted-state mutation is committed
- if the `irremovable` unequip rejection runs while an exchange shell is open, the requester first receives one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, then the requester receives the unequip rejection chat; no exchange result/finalization frame or carried/equipment/point/quickslot/persistence mutation appears
- mismatched or malformed removal metadata fails closed with no point change and no committed inventory/equipment/persistence mutation
- corrupt/disposable equipped-source fixtures whose live count exceeds the authored template `max_count` fail closed the same way: no item refresh, no point change, no appearance update, and no committed inventory/equipment/persistence mutation
- corrupt/disposable duplicate equipped-slot fixtures also fail closed for both fallback and template-backed unequip: no ambiguous first-match item is moved, no item refresh is emitted, and no carried/equipment or persisted-state mutation is committed

### 4.5.10 Merchant buy/sell template restrictions (`SHOP BUY` / `SHOP SELL2`)

- [ ] Open a known bootstrap merchant window with a disposable QA character
- [ ] Attempt to buy a catalog item whose authored template requires a higher `min_level` than the selected character has; if possible, author a non-empty `buy_reject_message` on that restricted template too
- [ ] Attempt to sell a carried item whose authored template requires a higher `min_level` than the selected character has; if possible, author a non-empty `sell_reject_message` on that restricted template too
- [ ] Repeat with an empire anti-flag (`anti_empire_a` / `anti_empire_b` / `anti_empire_c`) that matches the selected character's empire for both packet `SHOP BUY` and `SHOP SELL2`; include one of those fixtures with non-empty `buy_reject_message` / `sell_reject_message`
- [ ] If authoring or importing disposable interaction fixtures during the run, include one negative dry-run where an `info` / `talk` / `warp` text field or a `shop_preview` title contains an embedded JSON `\u0000` byte and confirm `/local/interactions` or `/local/content-bundle/validate` rejects it before the content can be persisted or loaded
- [ ] If authoring a disposable `quest_flag` turn-in through loopback HTTP, `POST` / `PUT` `/local/interactions` with the owned reward/consume fields (`reward_gold`, `reward_experience`, `reward_items` or scalar reward shorthand, `consume_gold`, `consume_experience`, `consume_items`) and confirm `GET /local/interactions/{kind}/{ref}` returns those fields instead of silently dropping them; also confirm a non-`quest_flag` body that carries `reward_gold` is rejected with `400`
- [ ] Attempt to sell carried items whose authored templates are marked `anti_get`, `anti_drop`, `anti_give`, `anti_sell`, or `anti_stack`; include one `anti_sell` fixture with a non-empty `sell_reject_message` and one non-`anti_sell` transfer-guard fixture (for example `anti_stack`) with a non-empty `sell_reject_message`
- [ ] If packet logging is available, sell part of a carried stack whose authored template has non-zero display sockets/attributes
- [ ] If QA data allows it, sell a carried stack whose authored template has non-zero `shop_sell_price` and confirm the visible gold credit is exactly `shop_sell_price * sold_count`

Expected result:
- restricted packet paths fail with the current merchant invalid-position companion and no inventory, item quickslot, gold, or persisted account mutation is visible; `anti_sell` additionally shows the authored `sell_reject_message` as self-only info chat when present, otherwise the deterministic merchant-refusal fallback text; `anti_get`, `anti_drop`, `anti_give`, and `anti_stack` sell-back guards show authored `sell_reject_message` when present and otherwise stay on the bare invalid-position companion; selected-character job/sex/empire/`min_level` buy/sell restrictions show authored `buy_reject_message` / `sell_reject_message` when present and otherwise stay on the bare invalid-position companion
- adjacent allowed merchant buy/sell cases still use the template-authored price/sell-credit behavior; non-zero `shop_sell_price` is the current explicit per-unit sell credit, appears in content-bundle summary item/catalog/reward rows when authored, and omitted/zero values preserve the older derived sell-credit fallback
- authored interaction text/title fields reject embedded NUL bytes at operator decode / content-bundle validation / runtime startup, so they cannot reach client chat, merchant-window titles, or compact preview strings as truncated content
- carried sell-back stacks whose live count already exceeds the resolved template-authored `max_count` fail with the same invalid-position companion before inventory, item quickslot, gold, or persisted account mutation
- partial-stack `SHOP SELL2` success refreshes the remaining stack with `ITEM_UPDATE`, preserves the authored display socket/attribute arrays while changing only the count, credits gold, and keeps item quickslots for the still-occupied cell unchanged
- if operator/test fixtures can force account persistence failure during an otherwise-valid `SHOP BUY`, the buy fails closed: no inventory refresh or gold debit is visible, and reconnect shows the pre-buy inventory/gold unchanged

### 4.5.11 Unsupported item give guard (`ITEM_GIVE`)

Run this only if packet tooling or the client build can emit an item-give attempt.

- [ ] Attempt to give a normal carried item stack to a visible actor or player using the `ITEM_GIVE` path
- [ ] Repeat with a carried item whose loaded template authors `anti_give = true` and non-empty `give_reject_message` while targeting a currently visible connected player
- [ ] While a merchant window is open on the same socket, repeat that authored `anti_give` item-give rejection with a visible connected player target
- [ ] While an exchange shell is open with a visible peer, repeat that authored `anti_give` item-give rejection from one participant
- [ ] Repeat with a non-inventory source cell, zero/unknown/invisible target VID, or zero/oversized count if packet tooling can construct it

Expected result:
- ordinary `ITEM_GIVE` attempts are parsed by the game socket but remain unsupported in the shipped bootstrap runtime: no response frames are visible, no carried inventory/equipment/quickslot state changes, no ground actor appears, no peer receives item-transfer frames, and reconnect/operator inspection shows the selected-character snapshot unchanged
- an `anti_give` carried item with `give_reject_message` returns exactly one self-only `CHAT_TYPE_INFO` message using that authored text only when `target_vid` names a currently visible connected player and the requested count is non-zero and does not exceed the live carried stack; the visible target receives no queued transfer/rejection frames, and inventory, equipment, quickslots, peers, ground handles, and persistence remain unchanged
- if the authored `ITEM_GIVE` rejection runs while a merchant window is open, the requester first sees one self-only `GC::SHOP END`, then the item-give rejection chat; merchant context is cleared, later `SHOP END` / `SHOP BUY` attempts fail closed until the merchant is reopened, and carried inventory/equipment/quickslots/gold/ground handles/persistence remain unchanged
- if the authored `ITEM_GIVE` rejection runs while an exchange shell is open, the requester first sees one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, then the requester receives the self-only item-give rejection chat; exchange display/accept state is cleared, but carried inventory/equipment/quickslots/gold/ground handles/persistence remain unchanged and no exchange finalization/result frame appears
- zero-target, unknown/invisible-target, zero-count, or oversized-count `ITEM_GIVE` attempts remain no-frame/no-mutation rejections even when the carried item authors `anti_give` plus `give_reject_message`
- item-template validation rejects `give_reject_message` if it contains embedded NUL bytes or is authored without one owned exchange-display / give rejection guard (`anti_stack`, `anti_get`, `anti_drop`, `anti_give`, `anti_sell`, job/sex/empire anti flags, or `min_level`); the `ITEM_GIVE` runtime feedback path still requires `anti_give` specifically, so malformed give-rejection text should fail before gameplay testing starts
- this is a fail-closed guard, not a completed transfer/exchange feature

### 4.5.12 Unsupported refine guard (`REFINE`)

Run this only if packet tooling or the client build can emit a refine attempt.

- [ ] Attempt one `REFINE` request for a disposable carried item slot / refine type
- [ ] Repeat with a carried item whose loaded template is not `refineable` and authors a non-empty `refine_reject_message`
- [ ] Repeat with a carried item whose loaded template is `refineable` and authors valid `refine_info` (`result_vnum`, `cost`, `probability`, and up to five materials)
- [ ] If packet tooling allows it, repeat with a different raw refine `type` value and confirm that the preview frame echoes that type
- [ ] While a bootstrap merchant window is still open on the same socket, repeat one template-authored refine rejection and one template-authored refine-information preview
- [ ] While an exchange shell is still open with a visible peer, repeat one template-authored refine rejection and one template-authored refine-information preview from the same participant
- [ ] After a successful `probability = 100` refine preview, confirm the dialog with a matching `REFINE` (`same pos` / `same type`) that has enough gold and materials, including at least one material cell bound to an item quickslot that will be fully consumed and one partially decremented material cell that also has an item quickslot, then confirm one trailing self-only `CHAT_TYPE_COMMAND` `RefineSuceeded <type>` (historical spelling) after the gold point-change, then reconnect and inspect the resulting source `vnum`, gold, material stacks, and quickslots
- [ ] After a successful `probability = 0` refine preview, confirm the dialog with a matching `REFINE` that has enough gold and materials, including one material cell and the source cell each bound to an item quickslot, then confirm source destroy + trailing self-only `CHAT_TYPE_COMMAND` `RefineFailed <type>` after the gold point-change, then reconnect and inspect gold, material stacks, missing source cell, and cleared quickslots
- [ ] After a successful preview for authored `probability` in `1..99`, confirm with a matching `REFINE` that has enough gold and materials; production draws one `crypto/rand` roll so manual QA cannot force an exact outcome — observe either the owned success burst (`RefineSuceeded <type>`, result `ITEM_SET`) or the owned destroy burst (`RefineFailed <type>`, source `ITEM_DEL` + source quickslot sync). Automated tests inject fixed rolls via `QueueRefineConfirmRollForTest`.
- [ ] After a successful refine preview, send `REFINE` with `type = 255` and confirm the dialog cancels with no mutation
- [ ] After a successful refine preview, attempt confirm while a merchant window or `/open_safebox` presentation is open and confirm the attempt stays fail-closed with no refine mutation; then close that busy window and confirm the still-open dialog can succeed. A new exchange shell cannot be opened while the refine dialog is already open under the owned exchange busy-window policy, so treat exchange-during-confirm as a defensive server guard rather than a client QA sequence.

Expected result:
- ordinary `REFINE` attempts are parsed by the game socket but remain unsupported in the shipped bootstrap runtime unless one of the authored feedback paths or the contract-frozen confirm-after-preview seams applies: no response frames are visible for unsupported cases, no carried inventory/equipment/quickslot/point state changes, no ground actor appears, no peer receives item-result frames, and reconnect/operator inspection shows the selected-character snapshot unchanged
- a non-refineable carried item with template-authored `refine_reject_message` returns exactly one self-only `CHAT_TYPE_INFO` message with that authored text, while still leaving inventory, equipment, quickslots, points, peers, ground handles, and persistence unchanged
- a refineable carried item with template-authored `refine_info` returns exactly one self-only `REFINE_INFORMATION_NEW` frame with the request `type` / `pos`, source item `vnum`, authored result `vnum`, cost, probability, and material rows only when selected-character class/sex/empire/level restrictions allow the item and transfer guards (`anti_stack`, `anti_get`, `anti_drop`, `anti_give`, `anti_sell`) are unset; this opens the same-socket refine-dialog presentation used by the accepted confirm paths and still leaves inventory/gold unchanged until that confirm succeeds
- if either template-authored refine feedback path runs while a merchant window is open, the client first sees one self-only `GC::SHOP END`, then the self-only refine rejection chat or refine-information frame; the merchant context is cleared, later `SHOP END` / `SHOP BUY` attempts fail closed until the merchant is reopened, and carried inventory/equipment/quickslots/gold/ground handles/persistence remain unchanged
- if either template-authored refine feedback path runs while an exchange shell is open, the requester first sees one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, then the requester receives the self-only refine rejection chat or refine-information frame; exchange display/accept state is cleared, but carried inventory/equipment/quickslots/gold/ground handles/persistence remain unchanged and no exchange finalization/result frame appears
- after a successful preview for `probability = 100`, a matching confirm may consume gold/materials, replace the source carried `vnum` with `result_vnum` in the same cell, persist, and emit self-only material refreshes + material-removal `GC::QUICKSLOT_DEL` for fully consumed material cells + result `ITEM_SET` + gold `PLAYER_POINT_CHANGE` + self-only `CHAT_TYPE_COMMAND` `RefineSuceeded <type>`; item quickslots for partially decremented material cells remain; skill/command quickslots that share the same byte payload remain; `type = 255` cancels the open dialog with no mutation; busy merchant or safebox presentations opened after preview stay fail-closed for confirm without auto-closing into mutation, and closing that busy window leaves the remembered dialog available for a later matching confirm; insufficient gold/materials and stale source identity also stay fail-closed for confirm
- after a successful preview for `probability = 0`, a matching confirm may consume gold/materials, destroy the source carried item, persist, and emit self-only material refreshes + material-removal `GC::QUICKSLOT_DEL` for fully consumed material cells + source `ITEM_DEL` + source-removal `GC::QUICKSLOT_DEL` + gold `PLAYER_POINT_CHANGE` + self-only `CHAT_TYPE_COMMAND` `RefineFailed <type>`
- after a successful preview for `probability` in `1..99`, a matching confirm draws one production `crypto/rand` roll (`1..100`) and routes to the owned success burst when `roll <= probability` or the owned destroy burst (including destroy-source quickslot sync) when `roll > probability`; manual QA cannot force an exact roll and should accept either owned outcome; tests inject rolls via `QueueRefineConfirmRollForTest`
- guarded refineable templates that fail those restrictions emit no refine-preview frame and still leave inventory, equipment, quickslots, points, peers, ground handles, and persistence unchanged
- item-template validation rejects `refine_reject_message` if it contains embedded NUL bytes or is authored on a template that also sets `refineable = true`, so contradictory refine feedback should fail before gameplay testing starts
- item-template validation rejects `refine_info` without `refineable`, with a zero result `vnum`, negative cost, probability outside `0..100`, more than five materials, or material rows with zero `vnum` / non-positive count
- the owned `REFINE_INFORMATION` / `REFINE_INFORMATION_NEW` payload has `type`, `pos`, `src_vnum`, `result_vnum`, `material_count`, `cost`, `prob`, and exactly five fixed material rows; `material_count > 5` is malformed for the codec boundary
- this is still not a completed refine system: catalysts, keep-grade/downgrade failure variants, and broader window choreography remain deferred beyond the confirm-after-preview success seam, the deterministic `probability = 0` destroy + `RefineFailed` companion, and the `1..99` injected-roll confirm seam

### 4.5.12b Unsupported storage/safebox/mall guard

Run this only if packet tooling or the client build can emit safebox/mall item-transfer attempts.

- [ ] Attempt one `SAFEBOX_CHECKIN` request for a disposable carried item slot
- [ ] Repeat `SAFEBOX_CHECKIN` with a carried item whose loaded template authors `anti_safebox` and non-empty `safebox_reject_message`
- [ ] Attempt one `SAFEBOX_CHECKOUT` request into a disposable carried destination slot
- [ ] After `/open_safebox` with at least one remembered open-presentation safebox item, attempt one whole-stack `SAFEBOX_ITEM_MOVE` into an empty in-range safebox cell and one compatible same-`vnum` merge when tooling allows it
- [ ] After `/open_safebox` with a multi-count remembered open-presentation safebox stack, attempt one partial-count `SAFEBOX_ITEM_MOVE` into an empty in-range safebox cell and one compatible partial merge when tooling allows it
- [ ] Attempt one closed-presentation / out-of-range / oversize-count `SAFEBOX_ITEM_MOVE` and confirm it stays fail-closed
- [ ] Attempt one `MALL_CHECKOUT` request into a disposable carried destination slot
- [ ] If packet tooling allows it, repeat one malformed payload-size case
- [ ] If slash harness tooling is available, run `/open_safebox` and confirm one self-only `GC::SAFEBOX_SIZE` plus one self-only `GC::SAFEBOX_MONEY_CHANGE` (default `0` when no durable warehouse gold) appear without inventory/carried-gold mutation
- [ ] If authored QA NPC content is loaded, interact with the visible `Warehouse` `open_safebox` actor and confirm optional self-only info chat plus `CHAT_TYPE_COMMAND` `ShowMeSafeboxPassword` without `SAFEBOX_SIZE` / `SAFEBOX_MONEY_CHANGE` / busy exchange state; then `/safebox_password 000000` (or the durable custom password) opens with `SAFEBOX_SIZE` (+ rematerialized `SAFEBOX_SET` when present) plus `SAFEBOX_MONEY_CHANGE` and becomes busy for exchange `START`; wrong password yields header-only `SAFEBOX_WRONG_PASSWORD` and leaves presentation closed; with the quest gate unmet, expect `Quest requirements are not met.` and no password prompt
- [ ] After a successful warehouse open + `/close_safebox`, immediately re-interact and `/safebox_password` again and confirm self-only info chat `You cannot open the warehouse again so soon after closing it.` with no open frames; wait at least 10 seconds, then confirm the same password opens normally; lab `/open_safebox` during that cooldown still opens immediately
- [ ] After warehouse `ShowMeSafeboxPassword`, walk so `ApproxDistance` from the challenge position exceeds `1000`, then `/safebox_password` and confirm self-only info chat `You are too far from the warehouse to open it.` with pending preserved; walk back inside the gate and confirm the same password opens
- [ ] After a successful warehouse password open, walk so `ApproxDistance` from the open-anchor exceeds `1000` and confirm queued self-only `CloseSafebox` closes the presentation, later `/close_safebox` is silent, and immediate re-challenge + `/safebox_password` hits the 10-second reopen cooldown; pending-only challenge walk-away must leave the password prompt intact without emitting `CloseSafebox`
- [ ] If slash harness tooling is available, run `/safebox_change_password 000000 vault2` (or another valid pair) without an open warehouse window and confirm self-only info chat `The warehouse password has been changed.`; later warehouse open must reject the old password and accept the new one; wrong old / missing / over-6-char passwords return `You have entered the wrong password.` with no durable mutation and no ordinary talking-chat fallthrough
- [ ] If slash harness tooling is available, after `/open_safebox` run `/safebox_money_save <amount>` with enough carried gold and confirm self-only gold `PLAYER_POINT_CHANGE` plus `SAFEBOX_MONEY_CHANGE` for the new warehouse total; then `/safebox_money_withdraw <amount>` restores carried gold the same way; while an exchange shell or merchant window is also open, a successful save/withdraw must first emit self/peer `GC::EXCHANGE END` and/or self-only `GC::SHOP END` (SHOP before exchange when both are active) before those money refresh frames; closed presentation / insufficient gold / malformed amounts emit no frames, leave any open merchant/exchange shell untouched, and do not fall through as ordinary talking chat; reopen after reconnect/restart rematerializes the same warehouse money via open-burst `SAFEBOX_MONEY_CHANGE`, and a second character on the same account must not see the first character's warehouse money
- [ ] If a bootstrap merchant window is already open, interact with that same `Warehouse` and confirm one self-only `GC::SHOP END` precedes the warehouse chat / `ShowMeSafeboxPassword`, merchant context is cleared, and later `SHOP END` / `SHOP BUY` fail closed until the merchant is opened again
- [ ] If slash harness tooling is available, run `/safebox_password` without a pending warehouse challenge and confirm no frames / no ordinary chat fallthrough; after `/open_safebox` already opened the presentation, `/safebox_password` returns info-chat `The warehouse is already open.`
- [ ] If slash harness tooling is available, run `/open_safebox 4` (or another out-of-range size) and confirm no `SAFEBOX_SIZE`, no ordinary chat fallthrough, and no safebox busy state for later exchange `START`
- [ ] If slash harness tooling is available, run `/close_safebox` (or the client `/safebox_close` companion) after that open and confirm one self-only `CHAT_TYPE_COMMAND` `CloseSafebox` hides the window and later exchange `START` is no longer blocked by the safebox busy guard
- [ ] After a successful check-in, reconnect or restart `gamed`, reopen `/open_safebox`, and confirm the remembered `SAFEBOX_SET` rematerializes for the same character; a second character on the same account must not see those rows

Expected result:
- ordinary unsupported storage-facing packets (`MALL_CHECKOUT`) are parsed by the game socket but remain fail-closed in the shipped bootstrap runtime: no storage-transfer response frames (`SAFEBOX_WRONG_PASSWORD`, `MALL_OPEN`, `MALL_SET`, or `MALL_DEL`) and no unexpected money/open frames are visible for those unsupported transfer paths, no carried inventory/equipment/quickslot/point/gold state changes, no ground actor appears, no exchange/merchant peer frames are queued, and reconnect/operator inspection shows the selected-character inventory snapshot unchanged
- after `/open_safebox`, a non-`anti_safebox` `SAFEBOX_CHECKIN` into an empty in-range `safe_pos` removes the carried item, syncs source item quickslots, stores the item in the open-presentation table and durable same-account safebox FileStore, persists the inventory/quickslot account snapshot together with that safebox write, and emits self-only inventory removal plus `GC::SAFEBOX_SET`; `/close_safebox` / `/safebox_close` clear the busy presentation flag with self-only `CHAT_TYPE_COMMAND` `CloseSafebox`, and a later `/open_safebox` (including after reconnect / process restart for the same login + character id) re-emits `SAFEBOX_SIZE` plus rematerialized `SAFEBOX_SET` rows; if a merchant window is open and the check-in would otherwise succeed, the client first sees self `GC::SHOP END`; if that carried source cell is currently displayed in an open exchange shell, check-in instead fails closed with no frames while the shell stays open/cancellable
- after `/open_safebox` with a remembered open-presentation safebox item, a `SAFEBOX_CHECKOUT` into an accepting carried destination removes that safebox cell from presentation + durable store, places/merges the whole stack into inventory while preserving item identity on fresh-cell placement, persists the inventory/quickslot account snapshot together with that safebox write, and emits self-only `GC::SAFEBOX_DEL` plus inventory `GC::ITEM_SET` / `GC::ITEM_UPDATE`; incompatible / locked / over-max / empty / closed / out-of-range attempts stay fail-closed with no frames; if an exchange shell is open and the check-out would otherwise succeed, the client first sees self/peer `GC::EXCHANGE END`; if a merchant window is open and the check-out would otherwise succeed, the client first sees self `GC::SHOP END`; if the destination carried cell is currently displayed in that open exchange shell, check-out stays fail-closed with no frames and does not close the shell
- after `/open_safebox` with a remembered open-presentation safebox item, a whole-stack `SAFEBOX_ITEM_MOVE` into an empty in-range safebox cell relocates that item while preserving identity and emits self-only `GC::SAFEBOX_DEL` + `GC::SAFEBOX_SET`; a compatible same-`vnum` destination under template `max_count` merges the whole source count the same way; a partial-count move into an empty cell decrements the source and places a new-identity split stack with dual self-only `GC::SAFEBOX_SET`, and a compatible partial merge refreshes both cells the same way; the TMP4 client may pack both move positions with the inventory window type and safebox-slot cell bytes, and that wire is accepted the same way as explicit safebox-window tooling; carried inventory/equipment/quickslots/gold/account inventory persistence stay unchanged while the durable safebox FileStore updates; closed presentation, oversize counts, out-of-range/same-cell/unsupported windows, and incompatible destinations stay fail-closed with no frames; if an exchange shell is open and the move would otherwise succeed, the client first sees self/peer `GC::EXCHANGE END`; if a merchant window is open and the move would otherwise succeed, the client first sees self `GC::SHOP END`
- an `anti_safebox` carried item with template-authored `safebox_reject_message` returns exactly one self-only `CHAT_TYPE_INFO` message with that authored text on `SAFEBOX_CHECKIN`, while still leaving inventory, equipment, quickslots, points, gold, ground handles, and persistence unchanged; if a bootstrap merchant window is open on that same socket, the client first sees self `GC::SHOP END` and later `SHOP END` / `SHOP BUY` attempts fail closed until the merchant is reopened; if a bootstrap exchange shell is open on that same socket, the client first sees self `GC::EXCHANGE END`, the visible peer receives queued `GC::EXCHANGE END`, and no exchange item/gold display is finalized or mutated
- the client may see one self-only `GC::SAFEBOX_SIZE` plus one self-only `GC::SAFEBOX_MONEY_CHANGE` presentation frame from `/open_safebox` and the same-socket character becomes busy for exchange `START` under the owned requester/partner busy-window chat strings; `/close_safebox` / `/safebox_close` clear that busy presentation with self-only `CHAT_TYPE_COMMAND` `CloseSafebox` without deleting durable safebox contents; already-closed close attempts emit no frames; out-of-range `/open_safebox 4` (and other sizes outside `1..3`) emit no frames, do not fall through as ordinary talking chat, and do not set the busy presentation flag; practice-mob floor, exact-position transfer / warp rebootstrap, and `/phase_select` / `/quit` / `/logout` likewise emit self-only `CloseSafebox` when the presentation was open so the client hides the stale window without mutating inventory/gold or durable safebox rows
- `/safebox_change_password <old> <new>` persists a durable password for the selected login + character id with self-only `The warehouse password has been changed.` on success, or `You have entered the wrong password.` on malformed / mismatched old password, without opening or closing the warehouse presentation and without inventing a `SAFEBOX_CHANGE_PASSWORD` packet
- `/safebox_money_save` / `/safebox_money_withdraw` while the warehouse presentation is open deposit/withdraw durable warehouse gold against carried gold with self-only gold `PLAYER_POINT_CHANGE` + `SAFEBOX_MONEY_CHANGE` on success; when a merchant window and/or exchange shell is also open, success prepends self-only `GC::SHOP END` and/or self/peer `GC::EXCHANGE END` (SHOP before exchange when both are active) before those money refresh frames; closed / pending-only / insufficient / overflow / malformed amounts stay silent fail-closed-consume and leave merchant/exchange shells untouched; TMP4 CG `SAFEBOX_MONEY` request header stays deferred
- item-template validation rejects `safebox_reject_message` if it contains embedded NUL bytes or is authored without `anti_safebox`, so contradictory storage feedback should fail before gameplay testing starts
- malformed storage payload sizes fail at the codec/dispatcher boundary rather than mutating runtime state
- this is still not a completed safebox or mall feature: mall / client `SAFEBOX_CHANGE_PASSWORD` packets / TMP4 CG `SAFEBOX_MONEY` request header remain deferred; warehouse password challenge + durable optional password + `/safebox_change_password` + durable warehouse money (`SAFEBOX_MONEY_CHANGE` + save/withdraw slashes) + same-account cell rematerialize + `/safebox_password` 10-second post-close reopen cooldown + `ApproxDistance > 1000` open-anchor distance gate + already-open MOVE / SyncPosition walk-away `CloseSafebox` auto-close are owned for check-in/out/move

### 4.5.13 Unsupported item exchange guard (`EXCHANGE`)

Run this only if packet tooling or the client build can emit an exchange/trade attempt.

- [ ] Attempt one `EXCHANGE START` request against a visible connected player, then close it with `EXCHANGE CANCEL`
- [ ] Attempt one `EXCHANGE START` request against a visible connected player that is already paired in an exchange shell
- [ ] Attempt one `EXCHANGE START` request against a player that is not visible or not connected
- [ ] Attempt one `EXCHANGE START` request against a same-map visible player that is outside the owned exchange-distance gate (`ApproxDistance >= 1000`)
- [ ] Open a valid exchange shell, then walk one participant far enough that `ApproxDistance >= 1000` while the shell is still open
- [ ] Open a merchant window first, then attempt `EXCHANGE START` against a visible connected player while that merchant window is still open
- [ ] Have a visible connected peer open a merchant window first, then attempt `EXCHANGE START` against that peer while their merchant window is still open
- [ ] Open the safebox presentation first with `/open_safebox`, then attempt `EXCHANGE START` against a visible connected player while that safebox presentation is still open
- [ ] Have a visible connected peer open their safebox presentation with `/open_safebox` first, then attempt `EXCHANGE START` against that peer while their safebox presentation is still open
- [ ] Open a refine-dialog preview first for a refineable carried item, then attempt `EXCHANGE START` against a visible connected player while that refine dialog is still open
- [ ] Have a visible connected peer open a refine-dialog preview first, then attempt `EXCHANGE START` against that peer while their refine dialog is still open
- [ ] Attempt one `EXCHANGE ITEM_ADD` request for a disposable carried item slot
- [ ] When the carried item is allowed by its template, inspect both exchange windows and packet logs for the display-only `GC::EXCHANGE ITEM_ADD` frames
- [ ] Attempt one active-shell `EXCHANGE ELK_ADD` / gold-add request for an amount the requester currently has
- [ ] Attempt one active-shell `EXCHANGE ELK_ADD` / gold-add request above the requester's current live gold
- [ ] Repeat active-shell `EXCHANGE ITEM_ADD` with carried items whose loaded templates author non-empty `give_reject_message` plus one owned exchange-display rejection guard (`anti_give`, and separately `anti_stack` / `anti_get` / `anti_drop` / `anti_sell`, plus one selected-character job/sex/empire/`min_level` fixture)
- [ ] Repeat the same guarded `EXCHANGE ITEM_ADD` before any exchange shell is open
- [ ] If packet tooling allows it, repeat that guarded `EXCHANGE ITEM_ADD` with display slot `12` or higher
- [ ] If packet tooling allows it, add one valid item to display slot `7` and then try to add a second item to the same display slot
- [ ] If packet tooling allows it, add one valid item to display slot `7` and then try to add that same carried item cell to a different display slot
- [ ] If packet tooling allows it, send `EXCHANGE ITEM_DEL` for the occupied display slot, then try another valid `ITEM_ADD` into that same display slot and try the previously displayed carried item cell again
- [ ] If packet tooling allows it, have both exchange sides display at least one carried item and/or in-budget gold, send `ACCEPT` from both sides, and confirm the second accept finalizes the currently displayed trade: both inventories/gold update, source quickslots for transferred whole stacks clear with self/queued `GC::QUICKSLOT_DEL` after inventory refresh frames and before gold/success-chat/`END`, both account snapshots persist, each side receives one self-facing info-chat `The trade with <partner_name> has been successful.` after its refresh frames and before `GC::EXCHANGE END`, and both sides receive `GC::EXCHANGE END` after the accept markers
- [ ] If packet tooling allows it, have both exchange sides send `ACCEPT`, then send a valid `ITEM_ADD` or in-budget `ELK_ADD` display change from one side before the second accept finalizes
- [ ] If packet tooling allows it, display an in-budget `ELK_ADD`, reduce that same character's live gold through a normal owned currency mutation such as gold drop, then send `ACCEPT` while the remembered display amount is now above live gold
- [ ] If packet tooling allows it, display an in-budget `ELK_ADD`, have that same side send `ACCEPT`, reduce that already-accepted side's live gold through a normal owned currency mutation such as gold drop, then have the paired side send the second `ACCEPT`
- [ ] If packet tooling allows it, display one carried item, alter that live carried item through a controlled/operator fixture so its item instance id, `vnum`, count, slot, lock state, carried-slot uniqueness, or currently loaded item-template eligibility no longer matches the exchange display, then send `ACCEPT`
- [ ] If packet tooling allows it, display one carried item, have that same side send `ACCEPT`, alter that already-accepted side's displayed live item or currently loaded item-template eligibility through a controlled/operator fixture, then have the paired side send the second `ACCEPT`
- [ ] If packet tooling allows it, have one side display a carried item, have that side send `ACCEPT`, arrange the paired receiver so the displayed item cannot fit into carried inventory under the loaded template (full inventory with no compatible unlocked stack room, including the case where only locked compatible stacks remain), then have the receiver send the second `ACCEPT`
- [ ] If packet tooling/operator fixtures allow it, have one side display a carried item whose item instance id also appears in the paired receiver's carried inventory or equipment, have the displayed side send `ACCEPT`, then have the receiver send the second `ACCEPT`
- [ ] If packet tooling/operator fixtures allow it, have one side display a stackable carried item, have that side send `ACCEPT`, arrange the paired receiver so an existing compatible carried stack for that same `vnum` already exceeds the loaded template's `max_count`, then have the receiver send the second `ACCEPT`
- [ ] If packet tooling allows it, have one side display in-budget `ELK_ADD`, have that side send `ACCEPT`, arrange the paired receiver so incoming displayed gold would overflow the current gold point-change carrier, then have the receiver send the second `ACCEPT`
- [ ] While an exchange shell is open with a displayed item, open `/open_safebox` on the requester, then attempt `EXCHANGE ACCEPT` and confirm it fails closed with no accept/finalize frames while the shell stays cancellable and inventory/gold unchanged
- [ ] While an exchange shell is open with a displayed item, open a merchant window on the requester, then attempt `EXCHANGE ACCEPT` and confirm it fails closed with no accept/finalize frames while the shell stays cancellable and inventory/gold unchanged
- [ ] While an exchange shell is open with a displayed item, open `/open_safebox` on the paired partner before any accept marker, then attempt requester `EXCHANGE ACCEPT` and confirm it fails closed with no accept/finalize frames while the shell stays cancellable
- [ ] While an exchange shell is open with a displayed item, open a merchant window on the paired partner before any accept marker, then attempt requester `EXCHANGE ACCEPT` and confirm it fails closed with no accept/finalize frames while the shell stays cancellable
- [ ] While an exchange shell is open, have the first side `ACCEPT`, then open `/open_safebox` on the already-accepted partner before the second `ACCEPT`, and confirm the second accept fails closed with no finalize frames while the shell stays cancellable
- [ ] While an exchange shell is open, have the first side `ACCEPT`, then open a merchant window on the already-accepted partner before the second `ACCEPT`, and confirm the second accept fails closed with no finalize frames while the shell stays cancellable
- [ ] Note for QA tooling: mid-shell refine preview currently closes the exchange shell before feedback, so requester/partner refine ACCEPT busy rejects are covered by the shared-world `AcceptExchange` unit proof rather than a live mid-shell refine ACCEPT packet path
- [ ] Note for QA tooling: commit-time busy-window / display / precondition drift after a second-accept finalize plan is built is covered by the shared-world `CommitExchangeFinalize` unit proof (`SharedWorldCommitExchangeFinalizeRejectsBusyWindowOpenedAfterAcceptPlan`) rather than a live mid-commit packet race; busy-window drift returns the same self-only START/ACCEPT busy info-chat strings to the commit requester; Check/Space/gold-overflow display/precondition drift emits the dual-sided info-chat strings named in the exchange bootstrap contract; other non-busy receiver precondition drift stays silent/no-frame
- [ ] If operator/test fixtures can force account persistence failure on either the first accepter or the second accepter during mutual-accept finalize, confirm the trade fails closed with no finalize frames, both inventories/gold/quickslots unchanged, and the shell still cancellable
- [ ] While an exchange shell is still open, send same-socket `/quit`, `/logout`, or `/phase_select` from one participant
- [ ] While an exchange shell is still open, trigger one successful exact-position transfer / warp (including a same-map destination that would still satisfy the walk-away distance gate) from one participant
- [ ] While an exchange shell is still open and showing one carried item, use that carried item with `ITEM_USE` or the slash `/use_item <slot>` harness from the same participant and confirm the request fails closed with no frames while the shell stays open
- [ ] While an exchange shell is still open and showing one carried stack, drag that stack onto a compatible carried stack with `ITEM_USE_TO_ITEM` from the same participant and confirm the request fails closed with no frames while the shell stays open
- [ ] While an exchange shell is still open and showing one carried item, drop that carried item with the normal `ITEM_DROP` / carried-item drop path from the same participant and confirm the request fails closed with no frames while the shell stays open
- [ ] While an exchange shell is still open and a pending ground handle is visible to the same participant, pick up that handle; if QA can alter the template before pickup, repeat with a guarded pickup template that emits `pickup_reject_message`
- [ ] While an exchange shell is still open and showing one carried item, attempt to drop a different non-displayed template-guarded carried item whose loaded template authors `drop_reject_message`
- [ ] While an exchange shell is still open and showing one carried item, equip that displayed item or attempt merchant `SHOP SELL` / `SELL2` of that displayed cell and confirm both fail closed with no frames while the shell stays open
- [ ] While an exchange shell is still open and showing one carried item, open `/open_safebox` and attempt `SAFEBOX_CHECKIN` of that displayed cell; also reopen safebox after seeding a remembered same-session safebox item and attempt `SAFEBOX_CHECKOUT` into that displayed destination cell; both must fail closed with no frames while the shell stays cancellable
- [ ] While an exchange shell is still open, equip one non-displayed carried item and unequip one worn item into a non-displayed carried cell with same-socket equipment movement or the `/equip_item` / `/unequip_item` harness
- [ ] While an exchange shell is still open, complete one same-socket merchant `SHOP BUY` and one merchant `SHOP SELL` / `SELL2` of a non-displayed carried cell from a previously opened merchant window
- [ ] If packet tooling allows it, repeat with `ELK_ADD` and `ACCEPT` subheaders
- [ ] If packet tooling allows it, repeat with a malformed payload size

Expected result:
- visible connected-player `EXCHANGE START` emits one self `GC::EXCHANGE START` naming the peer `VID` and queues one peer `GC::EXCHANGE START` naming the requester `VID`; `EXCHANGE CANCEL` emits one self `GC::EXCHANGE END` and queues one peer `GC::EXCHANGE END`
- `EXCHANGE START` against a visible connected peer that is already paired returns one self-only `GC::EXCHANGE ALREADY` to the requester, queues no frames to the existing pair, and leaves the existing exchange shell intact
- non-visible, disconnected, requester-already-paired, out-of-range (`ApproxDistance >= 1000`), or zero-HP exchange starts fail closed with no exchange frames
- walking either paired participant outside the owned exchange-distance gate closes the shell with self/peer `GC::EXCHANGE END` and leaves inventory/equipment/quickslots/gold/persistence unchanged
- `EXCHANGE START` while a same-socket merchant window is already open returns one self-only `CHAT_TYPE_INFO` with `You cannot trade while another trade window is open.`, leaves the merchant window open, creates no exchange pairing/display state, and leaves inventory/equipment/quickslots/gold/persistence unchanged
- `EXCHANGE START` against a visible connected peer that currently has an open bootstrap merchant window returns one self-only `CHAT_TYPE_INFO` with `That player cannot trade right now.`, queues no peer frames, creates no exchange pairing/display state, leaves the partner merchant window open, and leaves inventory/equipment/quickslots/gold/persistence unchanged
- `EXCHANGE START` while a same-socket safebox presentation is already open returns the same self-only `CHAT_TYPE_INFO` with `You cannot trade while another trade window is open.`, leaves the safebox presentation open, creates no exchange pairing/display state, and leaves inventory/equipment/quickslots/gold/persistence unchanged
- `EXCHANGE START` against a visible connected peer that currently has an open bootstrap safebox presentation returns the same self-only `CHAT_TYPE_INFO` with `That player cannot trade right now.`, queues no peer frames, creates no exchange pairing/display state, leaves the partner safebox presentation open, and leaves inventory/equipment/quickslots/gold/persistence unchanged
- `EXCHANGE START` while a same-socket refine-dialog presentation is already open returns the same self-only `CHAT_TYPE_INFO` with `You cannot trade while another trade window is open.`, leaves the refine dialog open, creates no exchange pairing/display state, and leaves inventory/equipment/quickslots/gold/persistence unchanged
- `EXCHANGE START` against a visible connected peer that currently has an open refine-dialog presentation returns the same self-only `CHAT_TYPE_INFO` with `That player cannot trade right now.`, queues no peer frames, creates no exchange pairing/display state, leaves the partner refine dialog open, and leaves inventory/equipment/quickslots/gold/persistence unchanged
- `EXCHANGE START` while the requester already holds gold at or above the owned signed point-change / bootstrap gold carrier max (`1<<31-1`) returns one self-only `CHAT_TYPE_INFO` with `You have more than 2 Billion Yang. You cannot trade.`, creates no exchange pairing/display state, leaves any open merchant/safebox/refine presentation untouched, and leaves inventory/equipment/quickslots/gold/persistence unchanged; this gate is evaluated after the owned busy-window and distance/`ALREADY` rejects
- `EXCHANGE START` against a visible connected peer that already holds gold at or above that same carrier max returns one self-only `CHAT_TYPE_INFO` with `The player has more than 2 Billion Yang. You cannot trade with him.`, queues no peer frames, creates no exchange pairing/display state, and leaves inventory/equipment/quickslots/gold/persistence unchanged; when both sides are over the cap, the requester-side string wins
- `EXCHANGE ACCEPT` while the requester already holds gold at or above the owned signed point-change / bootstrap gold carrier max (`1<<31-1`) fails closed with one self-only info-chat `You have more than 2 Billion Yang. You cannot trade.`, no accept/finalize frames, leaves the exchange shell cancellable, and leaves inventory/equipment/quickslots/gold/persistence unchanged; this gate is evaluated after the owned busy-window rejects and before Check/Space finalization preconditions
- `EXCHANGE ACCEPT` while only the paired partner already holds gold at or above that same carrier max fails closed with one self-only info-chat `The player has more than 2 Billion Yang. You cannot trade with him.`, with the same no-accept / still-cancellable contract; when both sides are over the cap, the requester-side string wins
- if either paired side's live gold drifts to or above `1<<31-1` after a second-accept finalize plan is built but before shared-world commit applies, mutual-accept finalize fails closed with one self-only gold-carrier-cap info-chat to the commit requester (`You have more than 2 Billion Yang. You cannot trade.` when that requester is over the cap, otherwise `The player has more than 2 Billion Yang. You cannot trade with him.`), no finalize/accept/`END` frames, and leaves the shell cancellable
- `EXCHANGE ACCEPT` while the requester already has an open same-socket merchant window, `/open_safebox` presentation, or refine-dialog presentation fails closed with one self-only info-chat `You cannot trade while another trade window is open.`, no accept/finalize frames, leaves that busy presentation open, leaves the exchange shell cancellable, and leaves inventory/equipment/quickslots/gold/persistence unchanged
- `EXCHANGE ACCEPT` (including second accept / mutual-accept finalize) while the paired partner currently has an open bootstrap merchant window, `/open_safebox` presentation, or refine-dialog presentation fails closed with one self-only info-chat `That player cannot trade right now.`, no accept/finalize frames, leaves the partner busy presentation open, leaves the exchange shell cancellable, and leaves inventory/equipment/quickslots/gold/persistence unchanged
- if either paired side opens a merchant / safebox / refine busy presentation after a second-accept finalize plan is built but before shared-world commit applies, mutual-accept finalize fails closed with one self-only busy info-chat to the commit requester (`You cannot trade while another trade window is open.` when that requester is busy, otherwise `That player cannot trade right now.`), no finalize/accept/`END` frames, rolls back any already-written account/live snapshots from that finalize attempt, and leaves the shell cancellable; live displayed item/gold Check drift at that same commit gate emits dual-sided info-chat (`Not enough Yang or the item is not in place.` / `The other player does not have enough Yang or their item is not in place.`) with the same rollback / still-cancellable contract; inventory-capacity drift emits dual-sided Space info-chat (`There isn't enough space in your inventory.` / `The other person has no space left in their inventory.`); gold-overflow finalization precondition drift emits dual-sided info-chat (`You cannot carry any more gold.` / `The other person cannot carry any more gold.`); other receiver finalization precondition drift (item-id collision, over-template-max, locked-compatible-stack capacity, selected-character restrictions) emits dual-sided `CHAT_TYPE_INFO` `Unknown error`
- the current exchange shell is only a window/open-close/display proof: no carried inventory/equipment/quickslot/gold state changes, no ground actor appears, and reconnect/operator inspection shows selected-character snapshots unchanged
- allowed active-shell `EXCHANGE ITEM_ADD` emits one self `GC::EXCHANGE ITEM_ADD` with `is_me = 1` and one queued peer `GC::EXCHANGE ITEM_ADD` with `is_me = 0`; both frames carry the carried item `vnum`, exchange display slot, count, and loaded template-authored `sockets` / `attributes`, but the item remains in carried inventory and is not removed or persisted as traded. While that carried item identity remains displayed, same-socket mutations that would change it (`ITEM_USE`, carried `ITEM_MOVE` / equip/unequip, `ITEM_USE_TO_ITEM`, `ITEM_DROP` / `ITEM_DROP2`, merchant sell of that cell, `SAFEBOX_CHECKIN` of that displayed source cell, and `SAFEBOX_CHECKOUT` into that displayed destination cell) fail closed with no frames and leave the exchange shell open; `ITEM_DEL`, cancel/close, death/walk-away/lifecycle teardown, and successful mutual-accept finalize clear that display lock
- duplicate display-slot `EXCHANGE ITEM_ADD` attempts from the same side fail closed with no additional frames and no mutation
- duplicate source-item `EXCHANGE ITEM_ADD` attempts from the same side also fail closed while that carried item identity is already shown in another display slot, so one live carried item cannot appear twice in the current display-only shell
- active-shell `EXCHANGE ITEM_DEL` for an occupied display slot emits one self `GC::EXCHANGE ITEM_DEL` with `is_me = 1` and one queued peer `GC::EXCHANGE ITEM_DEL` with `is_me = 0`; both carry the cleared display slot in `arg1`, clear the exchange-window display entry, allow that display slot and the previously displayed carried item identity to be reused by a later display-only `ITEM_ADD`, and still leave carried inventory/equipment/quickslots/gold/ground handles/persistence unchanged; if either side had displayed `ACCEPT`, `GC::EXCHANGE ACCEPT(arg1 = 0)` reset markers arrive before the item-del clear frame for each receiver
- active-shell `EXCHANGE ELK_ADD` / gold-add for an amount the requester currently has emits one self `GC::EXCHANGE GOLD_ADD` with `is_me = 1` and one queued peer `GC::EXCHANGE GOLD_ADD` with `is_me = 0`; both carry the displayed amount in `arg1`, but live/persisted gold is unchanged
- active-shell gold-add above the requester's current live gold emits one self `GC::EXCHANGE LESS_GOLD`, queues no peer frame, and does not mutate live/persisted gold
- active-shell `EXCHANGE ACCEPT` emits one self `GC::EXCHANGE ACCEPT` with `is_me = 1` and one queued peer `GC::EXCHANGE ACCEPT` with `is_me = 0`; both carry `arg1 = 1`, leave item/gold display state unchanged, leave the shell cancellable, and do not transfer items/gold or persist trade state only while requester-side remembered displayed carried items still match live item id/`vnum`/count/slot state, currently loaded template metadata still permits those displayed items, requester remembered gold remains within live gold, any already-accepted partner display state still matches that partner's current shared-world character state and current item-template metadata, and second-accept receiver item-id collision, over-template-max compatible-stack, locked-compatible-stack capacity, inventory-capacity, gold-overflow, and either-side open merchant/safebox/refine busy-window preconditions pass (inventory-capacity failures emit dual-sided Space info-chat; gold-overflow failures emit dual-sided `You cannot carry any more gold.` / `The other person cannot carry any more gold.`); a later accepted display-changing `ITEM_ADD`, `ITEM_DEL`, or in-budget `ELK_ADD` clears previously shown accept markers with `GC::EXCHANGE ACCEPT(arg1 = 0)` before showing the new display state, again with no item/gold mutation; if a requester-side previously displayed gold amount is now above the requester's current live gold, `ACCEPT` instead emits one self-only `GC::EXCHANGE LESS_GOLD`, queues no peer accept marker, and still performs no trade mutation; if a requester-side previously displayed carried item is now missing, changed, locked, equipped, duplicate-owned, duplicate-occupied, or no longer allowed by current template metadata, `ACCEPT` emits no frames, queues no peer accept marker, keeps the shell cancellable, and still performs no trade mutation; if an already-accepted partner's displayed gold or carried item drifts before the second accept, the second accept emits dual-sided Check info-chat (`Not enough Yang or the item is not in place.` to the drifted side; `The other player does not have enough Yang or their item is not in place.` to the paired partner), keeps the shell cancellable, and still performs no trade mutation; if receiver inventory-capacity fails before that second accept, the second accept emits dual-sided Space info-chat (`There isn't enough space in your inventory.` to the full receiver; `The other person has no space left in their inventory.` to the paired partner) under the same no-mutation / still-cancellable contract; if receiver item-id collision, over-template-max compatible-stack, locked-compatible-stack capacity, or either-side open merchant/safebox/refine busy-window preconditions fail before that second accept, busy failures emit the already-owned self-only busy info-chat while item-id collision / over-template-max / locked-compatible-stack / selected-character restriction failures emit dual-sided `CHAT_TYPE_INFO` `Unknown error`, queue no peer accept marker, keep the shell cancellable, and still perform no trade mutation
- when the second accept passes those checks, mutual-accept finalize transfers the displayed items/gold, clears source item quickslots for transferred whole stacks with self/queued `GC::QUICKSLOT_DEL` after inventory refresh frames and before gold/success-chat/`END`, persists both accounts, emits dual-sided self-facing `CHAT_TYPE_INFO` `The trade with <partner_name> has been successful.` after that side's refresh frames and before `GC::EXCHANGE END`, and closes the shell with self/peer `GC::EXCHANGE END`
- same-socket `/quit`, `/logout`, and `/phase_select` close an open bootstrap exchange shell before completing their lifecycle behavior: the command sender sees one self-only `GC::EXCHANGE END` before the command-delivery or phase-transition frame, the paired peer receives one queued `GC::EXCHANGE END`, and carried inventory/equipment/quickslots/gold/ground handles/persistence remain unchanged
- same-socket exact-position transfer / warp rebootstrap closes an open bootstrap exchange shell before the transfer burst, including same-map in-range destinations that would still satisfy the walk-away distance gate: the transferring player sees one self-only `GC::EXCHANGE END` before the normal self transfer bootstrap / origin visibility frames, the paired peer receives one queued `GC::EXCHANGE END`, the exchange display/accept state is cleared without trade mutation, later `ACCEPT` / mutual-accept finalize attempts fail closed with no frames, and when an open merchant window is also closed by that same transfer the already-owned merchant `GC::SHOP END` precedes the exchange `END`
- same-socket successful carried-item use closes an open bootstrap exchange shell before the item-use response burst only when that use targets a non-displayed carried item: the requester sees one self-only `GC::EXCHANGE END` before the packet `ITEM_USE` echo or slash item-use point/item/info frames, the paired peer receives one queued `GC::EXCHANGE END`, the exchange display/accept state is cleared, and the item-use mutation follows the normal template-backed item-use persistence contract without any exchange finalization/result frames. Using an already-displayed exchange item instead fails closed with no frames while leaving the shell open and leaving live/persisted inventory/quickslot/points/gold unchanged
- same-socket successful `ITEM_USE_TO_ITEM` stack consolidation closes an open bootstrap exchange shell before the stack-merge response burst only when both source and target are non-displayed: the requester sees one self-only `GC::EXCHANGE END` before the item/quickslot refresh frames, the paired peer receives one queued `GC::EXCHANGE END`, the exchange display/accept state is cleared, and the stack mutation follows the normal template-backed item-use persistence contract without any exchange finalization/result frames. If either cell is currently displayed in the exchange shell, the request fails closed with no frames and the shell stays open
- same-socket successful carried-item drop closes an open bootstrap exchange shell before the drop response burst only when that drop targets a non-displayed carried item: the requester sees one self-only `GC::EXCHANGE END` before the item/quickslot/ground/ownership frames, the paired peer receives one queued `GC::EXCHANGE END` before the visible ground add/ownership frames, the exchange display/accept state is cleared, and the drop mutation follows the normal item-drop persistence contract without any exchange finalization/result frames. Dropping an already-displayed exchange item instead fails closed with no frames while leaving the shell open
- same-socket template-backed carried-drop rejection feedback also closes an open bootstrap exchange shell before the self-only info chat: the requester sees one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, then the requester sees the authored or fallback `CHAT_TYPE_INFO` rejection while carried inventory, quickslots, points, gold, ground handles, and persistence remain unchanged
- same-socket successful `ITEM_PICKUP` closes an open bootstrap exchange shell before the pickup response burst: the requester sees one self-only `GC::EXCHANGE END` before `ITEM_GROUND_DEL`, item refresh, point-change when gold is picked up, and `ITEM_GET` frames, the paired peer receives one queued `GC::EXCHANGE END` before any queued visible-ground delete, the exchange display/accept state is cleared, and the pickup mutation follows the normal item-drop/pickup persistence contract without any exchange finalization/result frames
- same-socket template-backed pickup rejection feedback or the inventory-full fallback also closes an open bootstrap exchange shell before the self-only info chat: the requester sees one self-only `GC::EXCHANGE END`, the paired peer receives one queued `GC::EXCHANGE END`, then the requester sees the authored or fallback `CHAT_TYPE_INFO` rejection while carried inventory, quickslots, points, gold, ground handles, and persistence remain unchanged
- same-socket successful carried-to-equipment or equipment-to-carried movement closes an open bootstrap exchange shell before the equipment mutation response burst only when the carried cell involved is not currently displayed: the requester sees one self-only `GC::EXCHANGE END` before item/equipment/appearance/quickslot refresh frames, the paired peer receives one queued `GC::EXCHANGE END` before any queued appearance update, the exchange display/accept state is cleared, and the equipment mutation follows the normal template-backed equipment persistence contract without any exchange finalization/result frames. Equipping a displayed exchange item, or unequipping into a displayed exchange cell, instead fails closed with no frames while leaving the shell open
- same-socket successful merchant `SHOP BUY`, and merchant `SHOP SELL` / `SELL2` of a non-displayed carried cell, close an open bootstrap exchange shell before the merchant transaction refresh burst: the requester sees one self-only `GC::EXCHANGE END` before item/gold refresh frames, the paired peer receives one queued `GC::EXCHANGE END`, the exchange display/accept state is cleared, and the merchant mutation follows the normal shop persistence contract without any exchange finalization/result frames. Selling an already-displayed exchange item instead fails closed with no frames while leaving the shell open
- finalization/result semantics beyond the first mutual-accept mutation + shell close remain unsupported in the shipped bootstrap runtime; apart from the current guard feedback, display-only item-add/item-del/gold-add/first-side accept paths, and the owned mutual-accept finalize path, unsupported exchange requests emit no exchange result frames and do not mutate state
- if either account persistence write fails during mutual-accept finalize, the trade fails closed with no finalize/result frames, both live and persisted inventory/equipment/quickslot/gold snapshots stay unchanged, and the exchange shell remains cancellable
- a carried item with `give_reject_message` plus one owned exchange-display rejection guard (`anti_stack`, `anti_get`, `anti_drop`, `anti_give`, `anti_sell`, job/sex/empire anti flags, or `min_level`) returns exactly one self-only `CHAT_TYPE_INFO` message for active-shell `EXCHANGE ITEM_ADD` using that authored text only when the requested display slot is in the current `0..11` exchange item range, while still leaving inventory, equipment, quickslots, gold, peers, ground handles, and persistence unchanged; it does not emit `GC::EXCHANGE ITEM_ADD`; the same guarded item-add attempted before an exchange shell is open remains a silent no-frame/no-mutation rejection, and omitted `give_reject_message` keeps the older silent display-suppression path
- out-of-range `EXCHANGE ITEM_ADD` display slots stay no-frame/no-mutation even for those guarded templates
- malformed `EXCHANGE` payload sizes fail at the codec/dispatcher boundary rather than mutating runtime state
- this is an exchange-window shell plus the first owned mutual-accept finalize path, not a completed exchange, trade, safebox, or player-shop feature
- `CG::MYSHOP` (`0x0802`) encode/decode plus GAME dispatch are owned in this bootstrap: the default remains deny-no-response, and the first host-only accepted private-shop open presentation is now live (sign/stock/busy gates + same-socket open/busy flag + one `GC::SHOP_SIGN` with host VID + sign, no inventory/gold mutation on open); host-only empty-sign `GC::SHOP_SIGN` clear/close is also live on `/phase_select` / `/quit` / `/logout`, practice-mob floor, transfer/warp, and lab `/close_myshop`; visible peers now receive the same live/empty `SHOP_SIGN` around-broadcast on open/close, and newly visible peers rematerialize one live `SHOP_SIGN` for an already-open host; guest browse open is now live via peer `CG::ON_CLICK` on that host VID → one guest-only `GC::SHOP START` stock table (busy merchant/safebox/refine/exchange rejects reuse the owned exchange busy info-chat strings; guest own open MYSHOP / closed host stay silent); guest leave is now live via `CG::SHOP END` → one guest-only `GC::SHOP END` (host empty-sign / guest lifecycle also clear browse with one guest END); guest private-shop buy is now live (`docs/plans/2026-08-24-myshop-guest-buy-mutation-contract-freeze.md`): browsing `CG::SHOP BUY` transfers live host stock/gold with guest `UPDATE_ITEM(vnum=0)` and fail-closed distance/sold-out/inventory-full/insufficient-gold; multi-guest sold-slot fan-out to other browsing guests is also owned (`docs/plans/2026-08-24-myshop-guest-buy-multi-guest-update-item-fanout.md`); guest sell-into-PC-shop and cube busy rejects stay deferred; partner open-private-shop exchange START/ACCEPT/commit busy rejects are now owned beside merchant/safebox/refine; open MYSHOP also locks host item use/move/drop/pickup/give/safebox/refine mutations fail-closed until empty-sign close, so keep treating tax/empire multipliers / guest sell / cube busy as out of scope for playable PvE checks

### 4.5.14 Guest browse an open private shop (`ON_CLICK` → `SHOP START` / `SHOP END`)

- [ ] Have player A open an accepted host-only `MYSHOP` with at least one listed carried stock row and a visible non-empty sign
- [ ] From player B (visible same-map peer), click player A / send `CG::ON_CLICK` with player A's VID
- [ ] From player B, send `CG::SHOP END` / close the private-shop window
- [ ] Repeat browse, then close player A's shop with `/close_myshop` while player B is still browsing
- [ ] Repeat while player B has `/open_safebox`, a merchant window, a refine dialog, or an exchange shell open
- [ ] Repeat while player B also has their own accepted `MYSHOP` open
- [ ] Close player A's shop with `/close_myshop`, then repeat the click from player B

Expected result:
- successful guest browse emits exactly one guest-only `GC::SHOP START` whose `OwnerVID` is player A's VID and whose display table fills the listed `display_pos` with remembered `vnum` / `count` / `price` plus template sockets/attributes when the live carried cell still matches; inventory/gold stay unchanged on both sides and no extra `SHOP_SIGN` is re-emitted by the browse
- guest `CG::SHOP END` while browsing emits exactly one guest-only `GC::SHOP END` and clears browse; a second END is silent
- host `/close_myshop` while a guest is browsing queues one guest `GC::SHOP END` beside the owned empty-sign path; later guest END stays silent
- guest open merchant/safebox/refine/exchange returns one self-only busy info-chat `You cannot trade while another trade window is open.` with no START
- guest own open MYSHOP, closed host, or unknown VID stay silent/no-frame
- guest private-shop buy is live; see 4.5.15 for the buy mutation checks

### 4.5.15 Guest buy from an open private shop (`SHOP BUY` while browsing)

Contract freeze: `docs/plans/2026-08-24-myshop-guest-buy-mutation-contract-freeze.md`.

- [ ] Have player A open `MYSHOP` with one listed carried row and known price, have player B browse, then send `CG::SHOP BUY` for that `display_pos`
- [ ] Confirm guest gold debit + host gold credit + guest item grant + host stack removal + guest `UPDATE_ITEM(vnum=0)` for that slot
- [ ] Confirm second buy of the same slot fails sold-out / invalid with no further mutation
- [ ] With a third visible peer C also browsing player A's shop, have player B buy once and confirm player C receives one `UPDATE_ITEM(vnum=0)` for that sold slot with no inventory/gold change on C, then confirm C's buy of the same slot fails sold-out / invalid
- [ ] Confirm distance `> 2000` yields `You are too far away from the shop to buy something.` with no shop error frame
- [ ] Confirm insufficient gold / full inventory use bare `NOT_ENOUGH_MONEY` / `INVENTORY_FULL`
- [ ] Confirm guest `SHOP SELL` / `SELL2` while browsing a private shop stay silent/no-frame

Expected result:
- one successful guest buy transfers live host stock and gold without bare `GC::SHOP OK`, keeps browse open until leave/host close, and clears the sold display slot for remaining guests
- a still-browsing second guest sees the sold-slot `UPDATE_ITEM(vnum=0)` fan-out without inventory/gold mutation (`docs/plans/2026-08-24-myshop-guest-buy-multi-guest-update-item-fanout.md`)
- tax/empire multipliers, guest sell-into-PC-shop, and cube busy rejects stay deferred

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

### 5.1.1 Stance presentation smoke

- [ ] If the client UI exposes sit/stand controls, toggle ground-sit and then stand while another live character can see the actor
- [ ] If packet tooling is available, send client `CHARACTER_POSITION(position=4)` and then `CHARACTER_POSITION(position=0)` from a live `GAME` session, and try one unsupported/battle position byte as a negative check

Expected result:
- accepted `position=4` and `position=0` requests return `GC::CHARACTER_POSITION(selected_vid, position)` to the sender and are visible to the nearby live peer
- `position=3` chair-sit requests are accepted as conservative sit requests and publish the same `position=4` ground-sit presentation until real chair placement is owned
- repeating the already active stand/sit state is a no-op: no duplicate self or peer `CHARACTER_POSITION` frame is visible
- the selected combat target, practice-mob HP, normal-attack cadence, retaliation timers, inventory, points, and persistence are unchanged by the stance presentation
- unsupported/battle position bytes fail closed with no visible frames or side effects

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
- import/adapt `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` when validating the authoring-form regen/drop + quest loop (validate/import expands it into the same owned runtime content, including the gated `Warehouse` `open_safebox` actor; the checked-in expanded twin is `docs/examples/bootstrap-pve-vertical-canonical-bundle.json`), or
- record this subsection as **N/A** instead of treating the absence of authored NPCs as a gameplay regression.

Optional operator preflight before importing a candidate bundle:
- [ ] If using `docs/examples/bootstrap-npc-service-bundle.json`, confirm the candidate summary reports one quest-state flag for `QuestHero` (`quest:first_steps.step = 1`) before import plus quest-flag interaction previews, `quest_flag_trigger_count`, and `quest_flag_route_count` for `quest:first_steps`, `quest:first_steps_kill_turnin`, and `quest:first_steps_reset`; also confirm the `practice.qa_reward_mob` spawn-group summary carries kill-quest credit fields for `quest:first_steps.killed_qa_mob` and that `QuestHunter` resolves to the kill-quest turn-in definition. The seed is portable server-side QA data, while the `quest_flag` actors are owned self-only content triggers that can advance or clear a selected character flag without claiming client quest UI
- [ ] `POST /local/content-bundle/summary` with the candidate JSON and confirm the compact counts, distinct quest-state flag/character/quest counts when `quest_state` rows are present, per-kind referenced/unreferenced interaction breakdown, exact referenced/unreferenced interaction identities, compact interaction-definition previews, exact quest-flag trigger identities inspectable through `/local/content-bundle/quest-flag-triggers/{kind}/{ref}`, exact quest-flag route identities inspectable through `/local/content-bundle/quest-flag-routes/{actor_name}`, exact spawn-group identities, exact portable combat-profile snapshots inspectable through `/local/content-bundle/combat-profiles/{profile}`, exact item-template identities including non-zero `shop_sell_price`, exact aggregate `reward_drops` rows inspectable through `/local/content-bundle/reward-drops/{item_vnum}`, selected-character guard metadata such as `anti_warrior` / `anti_empire_a` / `min_level`, direct-use `use_effect` metadata, equipment guard/effect metadata such as `equip_slot` / `appearance_vnum` / `irremovable` / `equip_effect` plus authored `equip_reject_message` / `unequip_reject_message`, refine metadata such as `refineable` / `refine_reject_message`, authored `buy_reject_message` / `sell_reject_message` values, and per-map occupancy look plausible before applying it
- [ ] For map-local reward audits, inspect `/local/content-bundle/maps/{map_index}/reward-drops` before import QA; it should return `reward_drops[]` rows whose `source_count` reflects only spawn groups on that map, and an empty array for a known authored map without item-shaped rewards
- [ ] For duplicated or similarly named authored actors, inspect `/local/content-bundle/static-actors/{name}` before import QA; it should return every exact portable `static_actors[]` row for that name, including plain non-interactable placements, without requiring the full bundle summary
- [ ] `POST /local/content-bundle/import-preview` with the same candidate JSON and confirm the no-mutation `current` / `candidate` summaries plus top-level and per-map deltas identify the maps, exact static-actor/spawn-group rows, map-local quest-flag/shop-route/warp-route/open-safebox-route rows, grouped reward-drop changes by item vnum, interaction-kind count/reference changes, interaction-definition preview changes, combat-profile changes, shop-route changes, warp-destination/route changes, open-safebox-route changes, quest-state flag/character/quest-count changes, and service actor/spawn/reward counts that would change; when only one authored map, interaction kind, interaction definition, interactable actor preview, merchant route, teleporter route, warehouse route, spawn group, combat profile, or reward item matters, use `POST /local/content-bundle/import-preview/maps/{map_index}`, `POST /local/content-bundle/import-preview/interaction-kinds/{kind}`, `POST /local/content-bundle/import-preview/interaction-definitions/{kind}/{ref}`, `POST /local/content-bundle/import-preview/interactable-static-actors/{name}`, `POST /local/content-bundle/import-preview/quest-flag-triggers/{kind}/{ref}`, `POST /local/content-bundle/import-preview/quest-flag-routes/{actor_name}`, `POST /local/content-bundle/import-preview/shop-routes/{actor_name}`, `POST /local/content-bundle/import-preview/warp-routes/{actor_name}`, `POST /local/content-bundle/import-preview/open-safebox-routes/{actor_name}`, `POST /local/content-bundle/import-preview/spawn-groups/{ref}`, `POST /local/content-bundle/import-preview/combat-profiles/{profile}`, or `POST /local/content-bundle/import-preview/reward-drops/{item_vnum}` to inspect that exact delta without filtering the broad preview
- [ ] `POST /local/content-bundle/validate` with the same JSON and confirm the pretty-printed canonical bundle response matches the intended authored content; for `docs/examples/bootstrap-npc-service-bundle.json`, the response should be byte-for-byte identical to the checked-in fixture. If testing `docs/examples/bootstrap-pve-vertical-authoring-bundle.json`, confirm validation expands authoring-only `regen_spawns` / `drop_tables` into the checked-in byte-canonical twin `docs/examples/bootstrap-pve-vertical-canonical-bundle.json` (stripped authoring collections, denser pack members, formula profile, gated Warehouse). If testing `docs/examples/bootstrap-drop-table-authoring-bundle.json`, confirm validation expands authoring-only `drop_tables` / `reward_drop_table_ref` into canonical `spawn_groups[].reward_experience`, `spawn_groups[].reward_gold`, sorted `spawn_groups[].reward_drop_vnums`, and the table-authored kill-quest credit fields including the optional require gate (`reward_quest_ref`, `reward_quest_flag`, `reward_quest_to`, `reward_quest_text`, `require_quest_ref`, `require_quest_flag`, `require_quest_from`), and returns no top-level `drop_tables`. If testing `docs/examples/bootstrap-kill-quest-only-drop-table-authoring-bundle.json`, confirm validation expands the gated kill-quest-only table (empty EXP/gold/drop channels, no item templates) into one canonical `spawn_groups[]` row carrying only the kill-quest credit + require-gate fields, strips `drop_tables` / `reward_drop_table_ref`, and retains the minimal `quest:first_steps.met_guide` `quest_flag` writer. If testing `docs/examples/bootstrap-kill-quest-only-regen-authoring-bundle.json`, confirm validation expands one-count `regen_spawns` plus the gated kill-quest-only table (empty EXP/gold/drop channels, no item templates) into one canonical `spawn_groups[]` row carrying only the kill-quest credit + require-gate fields, strips `regen_spawns` / `drop_tables` / `reward_drop_table_ref`, and retains the same `met_guide` writer. If testing `docs/examples/bootstrap-regen-authoring-bundle.json`, confirm validation expands `regen_spawns` with `count = 1` plus the fixed reward table into one canonical `spawn_groups[]` row carrying the table-authored kill-quest credit fields including the optional require gate (`reward_quest_ref`, `reward_quest_flag`, `reward_quest_to`, `reward_quest_text`, `require_quest_ref`, `require_quest_flag`, `require_quest_from`), and strips `regen_spawns`, `drop_tables`, and `reward_drop_table_ref`. If testing `docs/examples/bootstrap-multi-count-regen-authoring-bundle.json`, confirm validation expands `count = 2` + `pack_spacing = 100` into two independent members (`practice.qa_multi_regen_mob.m01` at the authored origin and `.m02` at `x+100`) that share the table-authored rewards/kill-quest gate, then strips `regen_spawns`, `drop_tables`, `reward_drop_table_ref`, and `pack_spacing`. If testing `docs/examples/bootstrap-pve-vertical-authoring-bundle.json`, confirm validation expands the composed `regen_spawns` + gated `drop_tables` + quest NPC loop into three canonical `spawn_groups[]` rows: gated kill-quest `practice.qa_pve_vertical_mob` plus denser pack members `practice.qa_pve_vertical_pack.m01` / `.m02` (`count = 2`, `pack_spacing = 100`), all bound to formula profile `qa_pve_vertical_practice_mob` (`max_hp = 20`, derived `damage_per_normal_attack = 5`) plus the portable `combat_profiles[]` row and the same quest-flag/service definitions, while stripping authoring-only `regen_spawns` / `drop_tables` / `reward_drop_table_ref` / `pack_spacing`. If testing `docs/examples/bootstrap-combat-profile-formula-bundle.json`, confirm validation expands the formula-first `qa_formula_practice_mob` profile into derived `damage_per_normal_attack = 5` and default `level = 1`, copies the profile-default death reward onto `practice.qa_formula_mob`, and keeps the portable `combat_profiles[]` row. If the candidate includes duplicate reward drop vnums in `spawn_groups`, `regen_spawns`, `drop_tables`, or bundled `combat_profiles[].death_reward.drop_vnums`, validation should fail before import. For deterministic negative dry-runs without improvising JSON, `POST /local/content-bundle/validate` with `docs/examples/bootstrap-invalid-regen-count-bundle.json` (`count = 2` without `pack_spacing`), `docs/examples/bootstrap-invalid-regen-over-max-count-bundle.json` (`count = 9`), `docs/examples/bootstrap-invalid-regen-one-count-pack-spacing-bundle.json` (`count = 1` with `pack_spacing = 100`), or `docs/examples/bootstrap-invalid-orphan-quest-gate-bundle.json` (gated kill-quest-only table missing the `met_guide` writer), or `docs/examples/bootstrap-invalid-orphan-service-quest-gate-bundle.json` (gated `shop_preview` missing the `met_guide` writer), or `docs/examples/bootstrap-invalid-empty-drop-table-bundle.json` (completely empty `drop_tables` row with no combat channels and no kill-quest credit), or `docs/examples/bootstrap-invalid-conflicting-kill-quest-credit-bundle.json` (spawn group already authors kill-quest credit and also expands a kill-quest-bearing `drop_tables` row) and confirm each returns `400` / fail closed before import. Owned multi-count regen authoring follows `docs/plans/2026-08-23-multi-count-regen-pack-placement-contract-freeze.md`: live runtime still sees only independent one-actor `spawn_groups` after expansion.
- [ ] If the candidate includes a formula-first custom `combat_profiles[]` row that authors `attack_value` / `defense_value` while omitting `damage_per_normal_attack` or `level`, confirm validation/import/export summaries return the derived `damage_per_normal_attack = max(1, attack_value - defense_value)` plus the default bootstrap `level = 1`, so the portable bundle stays self-describing. The repository fixture `docs/examples/bootstrap-combat-profile-formula-bundle.json` is the preferred smoke for that path (`attack_value = 9`, `defense_value = 4`, derived damage `5`, `max_hp = 20`).
- [ ] After importing a custom-profile `spawn_groups` practice mob, restart `gamed` against the same static-actor snapshot and confirm `GET /local/content-bundle/combat-profiles/{profile}` plus `GET /local/spawn-groups/by-ref/{spawn_group_ref}` still expose the same custom HP/damage/formula defaults before any fresh operator profile registration; this proves the static-actor store carried the canonical `combat_profiles[]` row across daemon restart.

#### 5.4.1 Talk / info / quest flag / merchant interactions

- [ ] Approach a visible authored QA NPC with `info`, `talk`, `quest_flag`, merchant `shop_preview`, or warehouse `open_safebox`
- [ ] For `info` / `talk`, interact once and wait for the self-only response
- [ ] For a `quest_flag` actor, interact once and confirm the self-only info acknowledgement appears; then inspect `GET /local/quest-state/characters/{character}` or `GET /local/content-bundle/quest-state/characters/{character}` and confirm the authored flag advanced for the selected character
- [ ] Before re-interacting, inspect `GET /local/interaction-visibility/{character}` and confirm that the same visible `quest_flag` actor now previews `Quest requirements are not met.` without mutating quest-state, because the authored `quest_from` value no longer matches
- [ ] Re-interact with the same `quest_flag` actor after the interaction cooldown expires and the authored `quest_from` value no longer matches; confirm the client sees the deterministic self-only info text `Quest requirements are not met.` while the persisted quest-state snapshot stays unchanged
- [ ] If using `docs/examples/bootstrap-npc-service-bundle.json`, confirm the QA teleporter is quest-gated on `quest:first_steps.met_guide = 1`: before meeting `QuestGuide`, interacting with `Teleporter` yields `Quest requirements are not met.` and no transfer; after the guide advances the flag, the same teleporter transfers normally without changing quest-state
- [ ] If using that same QA bundle, confirm the QA merchant is also quest-gated on `quest:first_steps.met_guide = 1`: before meeting `QuestGuide`, interacting with `Merchant` yields `Quest requirements are not met.` and no merchant window opens; after the guide advances the flag, the same merchant opens normally without changing quest-state
- [ ] If using that same QA bundle, keep the QA merchant window open after `QuestGuide` unlocks it, then clear `met_guide` through `QuestResetGuide` or `POST /local/quest-state/transition`; confirm the next packet `SHOP BUY` / `SHOP SELL` / `SHOP SELL2` or `/shop_buy` auto-closes that stale window with one merchant-family `GC::SHOP END` and leaves gold/inventory unchanged until the merchant is opened again
- [ ] If using that same QA bundle, confirm the QA warehouse is also quest-gated on `quest:first_steps.met_guide = 1`: before meeting `QuestGuide`, interacting with `Warehouse` yields `Quest requirements are not met.` and no password prompt / safebox presentation opens; after the guide advances the flag, the same warehouse prompts `ShowMeSafeboxPassword` and opens after `/safebox_password 000000` without changing quest-state. Operators can also inspect `GET /local/content-bundle/open-safebox-routes/Warehouse` or `GET /local/content-bundle/maps/1/open-safebox-routes` for the authored size/gate placement without opening safebox in-game
- [ ] If using that same QA bundle, confirm `VillageGuide` (`talk`) and `VillageSignpost` (`info`) are also quest-gated on `quest:first_steps.met_guide = 1`: before meeting `QuestGuide`, interacting with either yields `Quest requirements are not met.` and no authored chat text; after the guide advances the flag, both return their authored self-only chat deliveries without changing quest-state
- [ ] For a merchant actor, interact once and confirm a merchant window opens instead of only a chat preview
- [ ] If packet logging is available and the QA merchant item template authors display sockets/attributes, confirm the non-empty `GC::SHOP START` catalog entries carry those socket/attribute values instead of zeroed display metadata; authored merchant entries with `price` above `uint32` or `count` above `uint8` should be rejected by content validation/import before a merchant window can open
- [ ] If the authored QA merchant catalog exposes an affordable test item, attempt one packet `SHOP BUY` from the open window and confirm the success path returns self-only inventory refreshes without an extra merchant-family `GC::SHOP OK` or the older placeholder info chat; newly occupied slots should use `ITEM_SET`, while merges into already-known carried stacks should use `ITEM_UPDATE`
- [ ] If the QA setup allows it, sell one carried item stack from the open merchant window and confirm the success path returns a carried-slot refresh (`ITEM_DEL` for whole-stack removal or `ITEM_UPDATE` for partial-stack decrement) followed by `PLAYER_POINT_CHANGE(POINT_GOLD)`, with no extra bare merchant-family `GC::SHOP OK`
- [ ] If a corrupt/disposable fixture has duplicate live items in the same carried cell, confirm merchant sell-back from that cell fails closed with no gold or inventory mutation
- [ ] If the bought item is stackable and the character already carries the same `vnum`, confirm the count can increase on that existing stack instead of always creating a new slot
- [ ] If the QA setup allows it, fill the carried inventory, leave two compatible carried stacks nearly full, buy a stackable merchant entry whose count exactly matches their combined remaining room, and confirm both existing stacks fill without needing any fresh slot
- [ ] If the QA setup allows it, leave one compatible carried stack nearly full, buy a stackable merchant entry whose count overflows that stack, and confirm the existing stack fills first while the remainder lands in a fresh carried slot
- [ ] If the QA setup allows it, leave several compatible carried stacks nearly full plus one free carried slot, buy a stackable merchant entry whose count exceeds the combined remaining room in those existing stacks, and confirm the existing stacks fill first in carried-slot order while only the final remainder lands in the fresh slot
- [ ] If the QA setup allows it, force one insufficient-gold merchant buy from the open merchant window and confirm the client now follows the merchant-family insufficient-money error path instead of the older placeholder info chat
- [ ] If the QA setup allows it, force one no-placement merchant buy from the open merchant window and confirm the client now follows the merchant-family inventory-full error path instead of the older placeholder info chat
- [ ] If the QA setup allows it, expose one merchant catalog item whose template is marked `anti_get` and authors `buy_reject_message`, then attempt packet `SHOP BUY` from the open merchant window
- [ ] If the QA setup allows it, keep a merchant window open, send one packet `SHOP BUY` for an authored slot that does not exist in that bound catalog snapshot and confirm the client receives one merchant-family invalid-position response without any gold or inventory mutation
- [ ] If the QA setup allows it, use the loopback static-actor or interaction-definition update endpoints to invalidate the currently open merchant actor/catalog and confirm the next packet `SHOP BUY` auto-closes that stale merchant window with one merchant-family `GC::SHOP END` without changing gold or inventory
- [ ] If the QA setup allows it, keep a quest-gated merchant window open, clear the required selected-character quest flag through a loopback quest-state transition or a `quest_flag` reset actor, and confirm the next packet `SHOP BUY` / `SHOP SELL` / `SHOP SELL2` or `/shop_buy` auto-closes that stale merchant window with one merchant-family `GC::SHOP END` without changing gold or inventory
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
- `quest_flag` applies exactly one selected-character compare-and-set transition, persists it to the quest-state store, and returns one self-only info acknowledgement on success; repeated same-actor attempts that no longer match the authored `quest_from` preserve the snapshot and return one self-only `Quest requirements are not met.` info chat without inventing quest UI
- merchant interaction opens a stable bootstrap `GC::SHOP START` window whose non-empty catalog entries are backed by the resolved item template's authored socket/attribute display metadata when that metadata exists; authored merchant catalogs whose `price` / `count` cannot fit the current `GC::SHOP START` `uint32` / `uint8` carriers are rejected at validation/import time instead of wrapping in the visible window
- interacting with a non-merchant static actor or triggering a player-visible interaction failure while that merchant window is still open should first close the stale merchant shell with one self-only `GC::SHOP END`, then return the interaction's own self response; later `SHOP END` / `SHOP BUY` attempts against the closed stale merchant window should fail closed until the player opens a merchant again
- a bootstrap `SHOP BUY` request can debit gold and grant the authored item without disconnecting the client, and successful packet buys now return self-only inventory refreshes (`ITEM_SET` for newly occupied slots, `ITEM_UPDATE` for existing-stack count refreshes) without an extra merchant-family `GC::SHOP OK`
- a bootstrap `SHOP SELL` / `SELL2` request can credit gold and remove or decrement the authored carried item without disconnecting the client, and successful packet sells now return only the carried-slot refresh plus `PLAYER_POINT_CHANGE(POINT_GOLD)` without an extra merchant-family `GC::SHOP OK`
- merchant sell-back fails closed if the live carried inventory contains duplicate authoritative entries for the same cell, preserving gold and inventory rather than deleting an arbitrary duplicate
- when the authored item is stackable and a compatible carried stack already exists, the buy can refresh that same slot with the increased count
- when several compatible carried stacks together can absorb the full authored count, the buy can fill those existing stacks in carried-slot order without needing a fresh slot
- when several compatible carried stacks together cannot absorb the full authored count but one free carried slot exists, the buy can fill those existing stacks first and place only the final remainder into one fresh carried slot
- insufficient-gold, no-placement, and unknown-slot merchant failures preserve state and now surface the merchant-family error path from the open window instead of silently failing or falling back to the older placeholder info chat on packet `SHOP BUY`
- `anti_get` merchant-buy failures preserve state, first surface `GC::SHOP INVALID_POS`, and then show the template-authored `buy_reject_message` as self-only info chat when present, otherwise the deterministic merchant-sale refusal fallback text
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
- local fallback QA through `/shop_buy <slot>` now mirrors the same merchant-family `GC::SHOP NOT_ENOUGH_MONEY` / `GC::SHOP INVENTORY_FULL` / `GC::SHOP INVALID_POS` failure surfaces as the owned packet path for those same authoritative results instead of keeping a silent unknown-slot branch; its success surface also matches packet `SHOP BUY` now: item refreshes only, with no extra bare `GC::SHOP OK`

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
- [ ] On client B, attempt to pick up client A's still-owned ground item during the first ~30 seconds after the drop and confirm the attempt fails closed: no inventory change, no ground delete, and no pickup notice for either client
- [ ] Wait until the exclusive ownership window elapses (~30 seconds) and confirm both clients receive a blank ownership label for the same ground handle
- [ ] On client B, pick up the now-public ground item after the blank ownership release
- [ ] Optionally repeat the drop and leave the public handle unclaimed until the bootstrap destroy deadline (~300 seconds from drop); confirm both clients receive one `GC::ITEM_GROUND_DEL` and the ground actor disappears without inventory/gold mutation

Expected result:
- while exclusive ownership is still active, client B's pickup attempt produces no frames and leaves the ground handle pending for client A
- after the exclusive ownership timer elapses, visible clients receive one blank `GC::ITEM_OWNERSHIP` and client B can reclaim the item as ordinary public pickup
- client B then receives a ground delete plus a normal/self pickup notice (`arg = 0`, empty `from_name`), and client A receives only the peer ground delete
- the item is added to client B's owned account/runtime rather than being delivered back to client A
- if the dropped item's loaded template becomes `anti_give` or job/sex/empire/`min_level`-restricted for client B after public release, client B sees template-authored `pickup_reject_message` when present and otherwise the bootstrap inventory-full info rejection, neither inventory mutates, no owner notice is queued, and the ground handle remains available for a later valid retry
- `anti_drop` / `anti_give` / `anti_sell` template-flagged items fail closed when dropped through the normal client inventory path, show template-authored `drop_reject_message` when present and otherwise the bootstrap "You cannot drop this item." info rejection, and leave carried inventory plus quickslots unchanged; selected-character restricted drops show authored `drop_reject_message` when present and otherwise stay silent/no-frame
- if debug/fixture tooling forces a deterministic ground-`VID` collision with an already-pending bootstrap handle, the colliding drop fails closed with no item refresh, no peer-visible ground add, and no live or persisted carried-inventory mutation
- exclusive ownership timers, public ownership release, the bootstrap `300`-second destroy deadline, FileStore rematerialize with absolute timers, process-local exclusive `OwnerID` rebind on matching owner rejoin, and graceful Leave / stale-reclaim FileStore deletion of owned ground handles are now owned for the bootstrap path; real party membership remains deferred

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
- when the equipped template authors non-zero `appearance_vnum`, that refresh shows the authored appearance id only in the visible `parts` slot; item identity remains unchanged
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
- if the selected character is restricted by the wearable template's job/sex/empire/`min_level` anti flags, or if the wearable template is temporarily authored with transfer/pickup-style anti flags such as `anti_get`, packet and slash equip fail closed with no item-slot, point, or appearance mutation
- already-visible peers still only receive the projected appearance refresh; no peer-visible point stream is frozen by this slice

### 5.7 Template-backed consumable item use

- [ ] Seed or confirm one carried consumable whose item template has a `use_effect` payload (current bootstrap QA seed: `27001`)
- [ ] Use the carried consumable through the current client item-use path or a carried-slot `ITEM_USE` packet (the older `/use_item <slot>` harness still remains valid)
- [ ] Confirm one self-only `PLAYER_POINT_CHANGE` arrives before the item-slot refresh
- [ ] Confirm the consumed slot decrements by exactly one stack item or clears entirely if it was the last item
- [ ] Confirm one self-only `CHAT_TYPE_INFO` placeholder effect arrives using the template-authored message
- [ ] Reconnect and confirm the consumed stack and updated point value persisted
- [ ] If QA data allows it, repeat with the selected character restricted by the consumable template's authored job/sex/empire/`min_level` anti flags; confirm no item, point, quickslot, or info-chat mutation is visible

Expected result:
- the current carried-slot client item-use path resolves through item-template metadata rather than a runtime-only hardcoded consumable switch
- the current seeded bootstrap template still yields `type = 1`, `amount = 50`, `value = updated Points[1]`, and `consume:27001:+50`
- the response burst stays self-only and ordered as `PLAYER_POINT_CHANGE` then `ITEM_SET`/`ITEM_DEL` then `CHAT_TYPE_INFO`
- selected-character job/sex/empire/`min_level` anti-flag templates fail closed before the consumable point/effect path runs
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
- invalid, malformed carried-item, anti-sell, equipped, runtime-locked, explicit zero-count `SELL2`, zero-HP owner, template sell-credit carrier overflow, or resulting-gold carrier overflow attempts fail closed and leave both live and persisted inventory/gold unchanged; the anti-sell branch also shows template-authored `sell_reject_message` when present, otherwise the deterministic merchant-refusal fallback text
- after practice-mob retaliation reaches the player's current zero-HP floor, both whole-stack `SELL` and partial-stack `SELL2` attempts emit no sell success frames and do not delete carried-item quickslots
- richer `GC::SHOP UPDATE_ITEM` / `UPDATE_PRICE` merchant-window choreography remains out of scope for this bootstrap sell-back smoke

### 5.10 Training dummy repeated-hit smoke

Run this only when the target build has a visible authored `training_dummy` nearby.

- [ ] Approach the dummy until it is clearly within the current bootstrap target/attack band
- [ ] Select the dummy once and confirm the client shows it as the active target
- [ ] If loopback ops access is available, query `GET /local/combat-target/{character_name}` or `GET /local/combat-targets` after selection and confirm the selected `target_vid`, `snapshot_version`, `hp_percent`, `target_current_hp`, `target_max_hp`, `normal_attack_damage`, `target_attack_value`, and `target_defense_value` match the visible dummy/combat profile
- [ ] For authored `spawn_groups` practice mobs, query `GET /local/spawn-groups/{entity_id}/leash?radius=400` and, when checking several mobs on one map, `GET /local/maps/{map_index}/spawn-group-leashes?radius=400`; confirm the read-only classifiers report the authored spawn point as `home`, the current materialized actor position as `current`, and stationary mobs as `status = at_home` with no `return_target`; if a local runtime/static-actor update fixture has moved the materialized mob away from its authored position, confirm `home` stays anchored to the authored spawn while `current` reflects the moved position and `status` becomes `within_radius` or `return_required`; these inspections must not change HP, target ownership, return-step schedules, respawn timers, or visible frames
- [ ] If that fixture reports `status = return_required`, send `TARGET` for that visible mob and confirm it fails closed with no target acknowledgement; when inspecting runtime attempt logs/tests, the owned reason is `target_return_required`. This is a temporary stationary-mob gate until a later return/chase slice moves the actor back inside leash
- [ ] If loopback ops access is available for the same displaced live mob, first call `POST /local/spawn-groups/{entity_id}/return-step?max_step=100` when it reports `return_required` and confirm the response includes `step.next`, moves the materialized actor one capped step toward authored home, persists that stepped position, releases any current engagement/selected-target ownership for that actor, sends one self-only `TARGET(0, 0)` clear to selected owners after the ordinary visibility refresh, and leaves stale attacks/delayed retaliation silent until a fresh `TARGET` / accepted hit reengages; if the actor is merely `within_radius`, the same return-step call should no-op without target clears or queued frames. For a server-owned smoke without invoking the operator step, wait through the owned return-step delay and confirm the same retained/removed/added visibility refresh plus selected-target reset appears from the pending server-frame path (retained viewers now receive one server-driven `MOVE` replication for same-map return-step / return-home instead of delete/readd), with a new one-second return step armed only while the actor remains `return_required`; if the static-actor snapshot store is temporarily unavailable during a due server-owned step, confirm no visibility frames are emitted and the next due retry still moves the unchanged return-required actor once persistence recovers; if the QA fixture restarts `gamed` with a persisted live spawn-backed actor already outside leash, confirm the first post-start flush arms and applies that same return-step path instead of leaving the actor permanently `return_required`; if a due step brings the actor back inside leash radius, confirm no later no-op return-step frames arrive just to correct exact home. If a manual/operator return-step moves the actor but leaves it still `return_required`, wait until the old pre-manual deadline and confirm no immediate extra refresh fires, then wait until one second after the manual step and confirm exactly the next capped return-step refresh arrives. Repeat or use `POST /local/spawn-groups/{entity_id}/return-home` to restore exact home, then confirm the response reports `status = at_home`, the actor's `current` position equals its preserved `home`, old-position-only viewers receive `CHARACTER_DEL`, home-position viewers receive the normal add/info/update burst, retained same-visible-set viewers receive delete-plus-readd at authored home, any previously pending automatic return-step deadline no longer fires, and a fresh `TARGET` works again while HP/death/reward profile state is otherwise unchanged; return-home applies both to `return_required` recovery and to exact-home restoration from a `within_radius` drift. If the mob was already selected/engaged when a successful return-home trigger runs, confirm the owning client also receives one self-only `TARGET(0, 0)` clear and must reselect before attacking again; if the actor was already exactly at authored home, confirm the trigger still clears stale target/engagement state without requiring a no-op static snapshot write; if the trigger ran while the actor was already at home after an accepted hit, confirm any pre-trigger delayed retaliation beat does not arrive and only a fresh post-reselect/post-hit engagement arms a new delayed beat
- [ ] If loopback ops access is available while a server-owned return step is armed, query `GET /local/spawn-group-return-steps`, `GET /local/spawn-group-return-steps/{entity_id}`, and `GET /local/maps/{map_index}/spawn-group-return-steps` and confirm the pending row exposes the same `entity_id`, current `actor`, planned `step.next`, `ready_at`, and `remaining_ms` countdown; map-local lookup should include only pending return steps for that actor's current effective map and should return an empty array for known maps with no due rows
- [ ] In a separate timing run, leave a live spawn-backed actor outside leash until its server-owned return-step deadline is already due, then enter with a fresh nearby client and confirm the initial static-actor visibility reflects the stepped server-owned position instead of stale displaced visibility followed by a duplicate queued return-step rebuild
- [ ] In a separate timing run, keep an engaged practice mob's pending chase-step deadline already due, then enter with a fresh nearby client and confirm the initial static-actor visibility reflects the stepped chase position instead of stale pre-chase visibility followed by a duplicate queued chase-step rebuild
- [ ] In a separate timing run, keep an engaged practice mob's pending chase-step deadline already due on the destination map, then trigger a MOVE / SYNC_POSITION transfer rebootstrap onto that map and confirm destination static-actor visibility reflects the stepped chase position instead of stale pre-chase visibility followed by a duplicate queued chase-step rebuild
- [ ] Confirm the pending-frame chase executor can now apply one retained engaged chase step after an accepted hit or proximity acquisition: wait through the owned `5s` chase delay while still engaged/in-leash and confirm the retained owner receives one server-driven `MOVE` replication for the actor VID at the planned next coordinates without clearing the selected combat target; if the actor becomes `return_required`, confirm chase loses to the return-step path. Proximity acquisition alone may arm chase and, after the same `5s` delay, apply that same retained-viewer `MOVE` chase step, and may also arm the delayed server-origin retaliation cadence, without inventing selected-target ownership or an immediate retaliation piggyback. Remove/add visibility cases across a chase step still keep `CHARACTER_DEL` / add-info-update. Return-step / return-home same-map retained viewers now also receive one server-driven `MOVE` while remove/add stay on delete/bootstrap and return recovery still clears engagement / selected targets; cross-map return-home remains on delete/readd. A same-map live spawn-backed operator/runtime position-only update fixture should likewise emit one retained-viewer `MOVE` (not delete/readd) while still releasing engagement / selected-target ownership; presentation/name/race refreshes stay on delete/readd. If loopback ops access is available while that chase deadline is armed, query `GET /local/spawn-group-chase-steps`, `GET /local/spawn-group-chase-steps/{entity_id}`, and `GET /local/maps/{map_index}/spawn-group-chase-steps` and confirm the pending row exposes the same `entity_id`, current `actor`, planned `step.next`, `ready_at`, and `remaining_ms` countdown without mutating position or engagement; map-local lookup should include only pending chase steps for that actor's current effective map and should return an empty array for known maps with no due rows.
- [ ] If a loopback fixture displaces a live spawn-backed actor onto a foreign map (`UpdateStaticActor` cross-map), confirm `GET /local/spawn-group-return-steps/{entity_id}` arms immediately, waiting through the owned `1s` return-step delay snaps the actor back to authored home via delete/readd (foreign-map `CHARACTER_DEL`, home-map add/info/update, no invented cross-map `MOVE`), leaves exactly one occupancy on the home map / none on the foreign map, clears the pending return-step row, and persists authored home; this is the automatic pending-frame twin of operator `return-home` dual-map anti-leak, not a cross-map MOVE/warp packet yet
- [ ] If a same-map live spawn-backed operator/runtime position update fixture leaves the mob `within_radius` (not `return_required`), confirm `GET /local/spawn-group-homeward-steps` / exact / map-local expose a pending homeward row after the update, return-step stays unarmed, and waiting through the owned `1s` homeward delay restores toward authored home with retained-viewer `MOVE` without arming return-step
- [ ] If a content-bundle import is available while a within_radius homeward deadline is armed: reimport the same canonical bundle and confirm the pending homeward due time is preserved while an unrelated stale entity-id schedule is pruned; replace the spawn with a different authored ref and confirm the old homeward deadline cannot fire afterward; force a failed replacement rollback and confirm the original within_radius actor plus its pre-import homeward due time are restored and still fire one retained-viewer homeward `MOVE`
- [ ] If loopback ops access is available while a server-owned homeward step is armed (hit -> chase displace -> leave aggro -> TARGET(0) / engagement release, or the operator/runtime within_radius update path above), query `GET /local/spawn-group-homeward-steps`, `GET /local/spawn-group-homeward-steps/{entity_id}`, and `GET /local/maps/{map_index}/spawn-group-homeward-steps` and confirm the pending row exposes the same `entity_id`, current within_radius `actor`, planned homeward `step.next`, `ready_at`, and `remaining_ms` countdown without mutating position or engagement; map-local lookup should include only pending homeward steps for that actor's current effective map and should return an empty array for known maps with no due rows; force death or re-engage and confirm the pending homeward row is omitted
- [ ] In a separate death-floor timing run, hit -> wait through delayed retaliation -> wait through chase displace so the mob is `within_radius`, then let the next delayed retaliation reach the owner `0`-HP floor; confirm `GET /local/spawn-group-homeward-steps/{entity_id}` still arms after that floor release, the dead owner receives no later homeward `MOVE`, and a living retained watcher receives one due homeward `MOVE` back to authored home
- [ ] In a separate timing run, leave a within_radius practice mob's pending homeward-step deadline already due after chase→release, then confirm EnterGame / MOVE-transfer / `/restart_here` bootstrap visibility shows the stepped homeward position instead of stale within_radius coords followed by a duplicate queued homeward rebuild
- [ ] Walk a living client inside the default spawn aggro radius (`200`) of an unengaged authored practice mob without attacking or selecting it, then with a second living visible client send a fresh `TARGET` and confirm it fails closed while the first client still owns that hit-independent engagement; confirm the first client receives no immediate HP `POINT_CHANGE`, then after the owned `1s` delayed retaliation delay receives one self-only HP `POINT_CHANGE` plus one self plain `DAMAGE_INFO(owner_vid, abs(delta))` while still having no selected combat target; with that same second visible live client, confirm the peer also receives the matching owner retaliation `DAMAGE_INFO` without the owner's delayed `POINT_CHANGE`
- [ ] In a separate proximity-only death-floor run (owner near `1` HP, still no selected combat target and no accepted hit), wait through the owned `1s` delayed beat until the owner reaches `0` HP and confirm self `PLAYER_POINT_CHANGE(value=0)` → `DEAD(owner_vid)` → `TARGET(0, 0)` even though no prior selection existed, the visible peer receives one queued `DEAD(owner_vid)`, later owner `TARGET` / `ATTACK` fail closed, no further delayed owner `DAMAGE_INFO` arrives, and the second living client can freshly `TARGET` the still-live mob after that floor release
- [ ] From that same proximity-only engagement (still no selected combat target), walk the engaged owner outside the default aggro radius before the next delayed beat and confirm no later self HP `POINT_CHANGE` arrives for that stale cadence, a second living visible client can now freshly `TARGET` the same still-live mob, and the owner receives no invented self `TARGET(0, 0)` clear merely because proximity engagement released
- [ ] From a proximity-only engagement that is released while the owner is still inside the default aggro radius (for example client `TARGET(0)`), confirm the same still-inside owner does **not** instantly reacquire that mob on the next pending-frame flush / delayed retaliation window; only after walking outside radius `200` and re-entering should proximity acquisition lock again
- [ ] After killing and waiting through the owned practice-mob respawn delay while remaining inside aggro radius, confirm the rebuilt live mob does **not** instantly re-lock the nearby owner; leave radius `200` and re-enter before expecting a fresh proximity engagement
- [ ] In a separate proximity-only death-floor → `/restart_here` run (owner near `1` HP, no selected combat target), wait through the owned `1s` delayed beat to owner `0` HP, then issue same-socket `/restart_here` while still inside aggro radius `200`; confirm the recovered owner does **not** instantly reacquire the still-live mob on the next pending-frame flush / delayed retaliation window, and only after leaving radius `200` and re-entering should proximity acquisition lock again
- [ ] Optional registry/debug guard for the same contract: if a harness floors the owner's shared-world HP snapshot to `0` before subject engagement release, confirm the releasing owner is still suppress-marked so a later live-HP restore while still inside radius does not instantly reacquire without leave/re-enter
- [ ] Repeat that proximity-only death-floor suppress expectation after same-socket `/phase_select` → fresh `SELECT`/`ENTERGAME` → `/restart_here` while still inside radius `200`, and again after abrupt disconnect/reconnect → `/restart_here`; confirm the recovered owner still stays suppressed until an explicit leave/re-enter of radius `200`
- [ ] Walk a living client outside that default aggro radius but still inside leash/visibility and confirm no engagement is established until a hit or a later move into radius
- [ ] Perform one accepted normal attack
- [ ] Confirm the selected target remains stable and the dummy HP display moves down from full by one deterministic bootstrap step
- [ ] Confirm a standalone bootstrap `training_dummy` hit shows a damage-number / hit-effect companion after the HP refresh; this is the first self plain `DAMAGE_INFO` runtime emission for standalone combat-profile actors
- [ ] With a second living visible client watching the same standalone bootstrap dummy, confirm that watcher also sees the same plain hit-effect companion for the hit, without receiving the attacker's self-only target HP refresh
- [ ] Clear the selected target in the client UI, then try to attack the old dummy without reselecting it if the packet harness/client path allows it
- [ ] Confirm the old-target attack fails closed with no HP refresh or damage-info frame until a fresh non-zero `TARGET` selection succeeds again
- [ ] If a packet harness or bow/ranged client path can emit `SHOOT(0x0403)`, send one unsupported shot while the dummy is still selected
- [ ] Confirm that unsupported shot does not disconnect the session, emits no visible combat frames, queues no peer frames, and does not change the selected dummy's HP; the next ordinary accepted normal `ATTACK` should still move the dummy from its previous HP to the next expected value
- [ ] If packet tooling can emit client `FLY_TARGETING(0x0404)` / `ADD_FLY_TARGETING(0x0405)`, send one request while the dummy is still selected and confirm it stays silent/no-frame, queues no peer frames, does not disconnect, does not change HP/cadence, and does not trigger server `FLY_TARGETING(0x0411)`, `ADD_FLY_TARGETING(0x0412)`, or `CREATE_FLY(0x0413)` frames; those server fly-effect families are codec-owned only until a later projectile/skill slice emits them
- [ ] If packet logging is available during this PvE-only dummy smoke, confirm the server does not emit `PVP(0x0414)` or `DUEL_START(0x0415)` frames; those PVP/duel presentation families are codec-owned only until a later PvP/duel runtime slice emits them
- [ ] In that same packet log, confirm ordinary dummy targeting and hits do not emit marker packets `TARGET_CREATE_NEW(0x0A13)`, `TARGET_UPDATE(0x0A11)`, or `TARGET_DELETE(0x0A12)`; those marker families are codec-owned only until a later quest/minimap target-marker runtime slice emits them and are deliberately separate from selected-target `TARGET(0x0A10)` HP refreshes
- [ ] If packet tooling or a client click path can emit `ON_CLICK(0x0A02)` while the dummy is still selected, send one request and confirm it stays silent/no-frame, queues no peer frames, does not disconnect, and does not change the selected dummy's HP or interaction/shop state
- [ ] If packet tooling or a client UI path can emit `CHARACTER_POSITION(0x0A60)` while the dummy is still selected and the player is alive, send accepted stand / sit requests and confirm they only emit the owned stance presentation (`GC CHARACTER_POSITION` to self and currently visible live peers when the normalized stance changes, duplicate stand/sit as no-op) without changing the selected dummy's HP or the next normal-attack cadence; unsupported battle-position bytes should still stay silent/no-frame, queue no peer frames, and not disconnect
- [ ] If loopback ops access is available, query the same combat-target endpoint again and confirm `hp_percent` reflects the damaged runtime-owned dummy instead of resetting to `100`
- [ ] Optional loopback guard: if a debug fixture moves the selected subject outside the combat band while the dummy remains visible, moves the selected spawn-group dummy to `return_required`, or leaves the mob aggro-owned by another live session, confirm `/local/combat-target/{character_name}` returns `404` and `/local/combat-targets` omits that stale row without changing gameplay target state; runtime attempt logs/tests should distinguish the aggro-owned case as `target_engaged` rather than the generic non-targetable failure
- [ ] If QA tooling can trigger exact-position transfer or a warp interaction while an authored practice mob is currently selected/engaged, confirm the transfer/rebootstrap response carries one self-only `TARGET(0, 0)` clear, no delayed retaliation fires for the abandoned target after the transfer, and a still-visible live peer can freshly target the same damaged mob without waiting for death/respawn
- [ ] Perform at least one more accepted normal attack
- [ ] Confirm the selected target HP display steps down again instead of bouncing back to full on every hit
- [ ] If practical, re-select the same still-visible dummy and confirm the current HP display stays at the already-mutated runtime value instead of silently resetting because of the re-selection itself
- [ ] Confirm the character's own inventory, equipment, and visible player stats do not unexpectedly change because of dummy hits alone

Expected result:
- repeated accepted hits against the same selected dummy decrement HP in deterministic bootstrap-sized steps
- the client-visible feedback is still narrow: the attacker receives selected-target refresh plus the standalone `training_dummy` hit-effect companion, and visible live peers receive only the matching hit-effect companion rather than the attacker's target HP refresh
- the known client `SHOOT` combat-family packet is only a safe decode-and-fail-closed guard in this slice; ranged/projectile hit resolution remains out of scope
- the known client `ON_CLICK` target/UI packet is only a safe decode-and-fail-closed guard in this slice; NPC/shop/quest click gameplay remains on later evidence-backed slices and the currently owned authored-service path stays separate
- the known client `CHARACTER_POSITION` target/UI packet is only a narrow presentation-only stance ingress in this slice; accepted stand/sit/chair requests may emit the owned `GC CHARACTER_POSITION` presentation while live, but battle-mode gameplay, persistent stance, and speed/change-speed rendering remain out of scope
- `PVP` and `DUEL_START` are currently server-presentation codecs only; no bootstrap PvE combat, death, restart, or reward smoke should produce those packets
- `TARGET_CREATE_NEW`, `TARGET_UPDATE`, and `TARGET_DELETE` are currently marker-presentation codecs only; ordinary selected-combat-target selection and HP refreshes should stay on `TARGET(0x0A10)` until a later marker runtime slice owns emission
- dummy hits do not spend items, grant items, mutate equipment, or alter saved player progression/state by themselves
- optional loopback combat-target snapshots are read-only debugging aids; they must reflect the same selected-target runtime state that the client sees, not introduce another authoritative combat path

Important note:
- the current contract says dummy HP is shared-world runtime state only
- do **not** treat the absence of account-style persistence for dummy HP as a regression in this slice
- reconnect/transfer/reset behavior for dummy HP should be recorded if observed, but it is still a later contract than this repeated-hit smoke step

### 5.10.1 Authored formula combat-profile practice-mob smoke

Run this when the target build can import `docs/examples/bootstrap-combat-profile-formula-bundle.json`. That fixture owns one formula-first custom profile (`qa_formula_practice_mob`: `max_hp = 20`, `attack_value = 9`, `defense_value = 4`, `aggro_radius = 150`, `leash_radius = 350`) bound to `practice.qa_formula_mob`, with profile-default EXP/gold/drop reward metadata.

- [ ] `POST /local/content-bundle/validate` the fixture and confirm the response derives `damage_per_normal_attack = 5`, defaults `level = 1`, preserves authored `aggro_radius = 150` / `leash_radius = 350`, and expands the spawn-group reward descriptor from the profile death reward
- [ ] Import the fixture, then confirm `GET /local/content-bundle/combat-profiles/qa_formula_practice_mob` and `GET /local/content-bundle/spawn-groups/practice.qa_formula_mob` expose the same formula/reward/radius defaults
- [ ] Query `GET /local/spawn-groups/{entity_id}/leash` without an explicit `radius` override and confirm the classifier uses the authored effective leash (`350`) rather than bootstrap `400`; an explicit `?radius=400` override may still be used for inspection
- [ ] Walk a living client to a distance clearly outside authored aggro `150` but inside bootstrap default aggro `200` (for example ~175 units from spawn home) and confirm proximity acquisition does **not** lock; then walk inside authored aggro `150` and confirm proximity acquisition does lock
- [ ] Approach and select `QAFormulaMob`; confirm the first self-only `GC TARGET` ack reports full HP (`100`)
- [ ] Land one accepted normal hit and confirm the target refresh steps by formula damage (`20 -> 15`, visible as `75` percent) plus one self plain `DAMAGE_INFO(..., damage = 5)` companion
- [ ] Continue accepted hits on the owned cadence until death and confirm it takes exactly four formula hits (`20 / 5`) rather than the built-in one-damage practice-mob loop
- [ ] Confirm the killing hit still uses death + clear before the profile-default reward frames

Expected result:
- the playable QA loop uses authored profile formula damage / max HP and authored acquire/leash radii, not the built-in `practice_mob` one-point / `200`/`400` defaults
- validation/import keep the portable formula profile self-describing without requiring a prior local `POST /local/static-actor-combat-profiles` registration
- the composed PvE vertical authoring fixture (`docs/examples/bootstrap-pve-vertical-authoring-bundle.json` / `qa_pve_vertical_practice_mob`) now also owns the same first-hit formula `DAMAGE_INFO(damage = 5)` companion in the automated vertical gameplay proof beside its four-hit kill count, plus the same authored `aggro_radius = 150` / `leash_radius = 350`
- this still does **not** claim full legacy combat math or wider attack types

### 5.11 Practice-mob reward smoke

Run this when the target build has authored QA `spawn_groups` practice-mob content loaded with a non-zero bootstrap death-reward descriptor. The repository example bundle at `docs/examples/bootstrap-npc-service-bundle.json` now includes `practice.qa_reward_mob` with EXP, gold, one fixed drop-vnum reward descriptor, and kill-quest credit that advances `quest:first_steps.killed_qa_mob` for the selected killer after the accepted death edge only when `quest:first_steps.met_guide = 1`. Separate authoring-only fixtures at `docs/examples/bootstrap-drop-table-authoring-bundle.json`, `docs/examples/bootstrap-kill-quest-only-drop-table-authoring-bundle.json`, `docs/examples/bootstrap-kill-quest-only-regen-authoring-bundle.json`, and `docs/examples/bootstrap-regen-authoring-bundle.json` can be used to validate fixed `drop_tables` expansion (including kill-quest-only tables with empty combat channels), and one-count `regen_spawns` expansion for EXP, gold, drop-vnum, and gated kill-quest credit channels before importing the canonicalized spawn-group reward descriptor. The composed authoring fixture `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` combines that regen/drop authoring form with a formula-first portable combat profile (`qa_pve_vertical_practice_mob`, including authored `aggro_radius = 150` / `leash_radius = 350`), one denser multi-count practice pack beside the gated kill-quest mob, and the `QuestGuide` unlock / gated services / `QuestHunter` turn-in loop so one validate/import path covers the full owned PvE vertical including authored damage/HP, first-hit formula `DAMAGE_INFO`, non-default acquire/leash radii, and denser independent pack members without pack AI. The formula-profile fixture in 5.10.1 also carries profile-default reward metadata and may be reused when validating reward frames after a formula kill. The narrower kill-quest-only fixture at `docs/examples/bootstrap-kill-quest-credit-bundle.json` remains available when QA wants that credit seam without the broader NPC service content.

- [ ] Approach and select the visible practice mob
- [ ] Land accepted normal attacks until the mob reaches the owned zero-HP death edge
- [ ] Confirm the killing hit still shows the death + target-clear choreography before any reward feedback
- [ ] If the QA mob grants EXP, confirm one self-only `PLAYER_POINT_CHANGE(POINT_EXP)` arrives after death/clear and that reconnect keeps the updated EXP value
- [ ] If the QA mob grants gold, confirm one self-only `PLAYER_POINT_CHANGE(POINT_GOLD)` arrives after death/clear and that reconnect keeps the updated gold value
- [ ] If the QA mob drops items, confirm one self-visible `GROUND_ADD` + `OWNERSHIP` pair appears per configured drop, at the killer's current position
- [ ] If the QA mob authors kill-quest credit fields, confirm one self-only info chat with the authored `reward_quest_text` arrives after death/clear (and after any independent EXP/gold/drop frames), then inspect `GET /local/quest-state/characters/{character}` and confirm the selected killer's authored flag advanced; a second kill while the current value no longer matches must stay silent for quest chat and leave the persisted quest-state snapshot unchanged. If the mob also authors a require gate (`require_quest_ref` / `require_quest_flag`), killing without that selected-character prerequisite must stay silent for quest chat and leave quest-state unchanged, while killing after the prerequisite matches still applies credit normally
- [ ] If using `docs/examples/bootstrap-npc-service-bundle.json`, pick up the practice-mob drop (`27001`) after kill credit advances `killed_qa_mob`, then interact with `QuestHunter`; confirm one self-only info chat acknowledging the turn-in, one self-only `PLAYER_POINT_CHANGE(POINT_GOLD)` debit for authored `consume_gold = 25`, one self-only `PLAYER_POINT_CHANGE(POINT_EXP)` debit for authored `consume_experience = 10`, one self-only `PLAYER_POINT_CHANGE(POINT_GOLD)` for authored `reward_gold = 100`, one self-only `PLAYER_POINT_CHANGE(POINT_EXP)` for authored `reward_experience = 50`, carried-inventory DEL/UPDATE frames for authored `consume_items` (`27001` x1), carried-inventory SET/UPDATE for authored `reward_items` (`11200` x1), and reconnect/operator inspection showing the cleared `killed_qa_mob` flag plus the net gold/experience/inventory result. Interacting without enough gold for `consume_gold` must return `You do not have enough gold.`; without enough experience for `consume_experience` must return `You do not have enough experience.`; without the required carried potion for `consume_items` must return `You do not have the required items.`; none of those fee failures may mutate gold/inventory/quest state
- [ ] If QA can temporarily re-author one `reward_items` template as `anti_get`, or with a selected-character job/sex/empire/`min_level` restriction that rejects the turn-in character, confirm the same `QuestHunter` interaction fails closed with exactly one self-only info chat (`buy_reject_message` when authored, otherwise `You cannot receive this quest reward.`), no point/item frames, leaves quest-state/`killed_qa_mob`, gold, experience, and inventory unchanged, and reconnect/operator inspection matches the pre-turn-in snapshot
- [ ] If QA can temporarily fill carried inventory so the second authored `reward_items` entry cannot place, confirm `QuestHunter` returns exactly one self-only info chat `You have too many items.`, no point/item frames, and leaves quest-state/gold/experience/inventory unchanged
- [ ] If QA can temporarily seed the turn-in character near the bootstrap gold or experience carrier cap so authored `reward_gold` / `reward_experience` would overflow `1<<31-1`, confirm `QuestHunter` returns exactly one self-only info chat (`You cannot carry any more gold.` or `You cannot gain any more experience.`), no point/item frames, and leaves quest-state/gold/experience/inventory unchanged
- [ ] If the mob was authored through `drop_tables`, confirm the actual imported/runtime `spawn_groups` row carries direct `reward_experience`, `reward_gold`, `reward_drop_vnums`, and any table-authored kill-quest credit fields; there should be no gameplay-visible table lookup or random roll at kill time
- [ ] Pick up one reward drop and confirm the normal bootstrap pickup path removes the ground item, adds it to carried inventory, persists it, and rejects a replayed pickup
- [ ] In a separate daemon-restart run, kill a rewarded practice mob, restart `gamed` while the exclusive ownership window is still open, then enter with the killer and a nearby peer: confirm the rematerialized reward stays exclusive for the killer (peer pickup fails closed), the killer can reclaim it through ordinary pickup, and FileStore / ops ground snapshots clear after that pickup
- [ ] With a second living visible client watching, wait until the exclusive ownership window elapses (~30 seconds) after a kill reward drop and confirm both clients receive a blank ownership label; then have the watcher pick up the now-public reward drop as ordinary collector pickup
- [ ] If the reward drop's authored item template sets `pickup_range`, move outside the default 300-unit reach but inside that authored reach and confirm pickup succeeds; repeat with a shorter authored reach and confirm the visible-but-out-of-range pickup fails closed while the ground handle remains pending
- [ ] With a second living visible client watching, disconnect the reward-drop owner before pickup and confirm the watcher sees deterministic ground-delete cleanup for the owner's still-owned reward drops
- [ ] If a second watcher is already at the bootstrap `0`-HP floor, confirm that dead watcher receives neither the owner leave delete nor the owned-ground delete noise
- [ ] If using a debug/fixture harness that deliberately pre-seeds one colliding reward-drop `VID`, confirm the colliding drop is omitted while the accepted death edge, scalar rewards, and any non-colliding drop entries still succeed

Expected result:
- reward frames are ordered after `DEAD` and `TARGET(0, 0)`
- scalar EXP/gold rewards persist to the selected character before their point-change frame is emitted
- item drops are runtime ground items first; they do not mutate inventory until an explicit pickup succeeds
- kill-quest credit, when authored, applies one selected-killer compare-and-set quest-flag transition after the accepted death edge and returns one self-only info chat only when that transition applies
- `quest_flag` turn-in `reward_items` that resolve to `anti_get` or selected-character-restricted templates fail closed before the quest transition: one self-only info chat (`buy_reject_message` when authored, otherwise `You cannot receive this quest reward.`), no point/item frames, and no quest-state/gold/experience/inventory mutation
- inventory-full `quest_flag` reward placement fails closed the same way with one self-only info chat `You have too many items.` and no quest-state/gold/experience/inventory mutation
- `quest_flag` turn-in `reward_gold` / `reward_experience` carrier overflow fails closed the same way with one self-only info chat (`You cannot carry any more gold.` / `You cannot gain any more experience.`) and no quest-state/gold/experience/inventory mutation
- reward drop pickup uses the same template-authored reach override as ordinary dropped handles when loaded item-template metadata is present
- reward drops reuse the same bootstrap exclusive-ownership timer / blank public ownership release / destroy deadline as ordinary player drops, including FileStore rematerialize with absolute timers across `gamed` process restart; exclusive non-owner pickup fails closed until release, then ordinary collector pickup succeeds
- invalid or unsupported reward descriptors preserve the accepted death/clear edge while omitting reward mutation and reward frames
- a live ground-drop `VID` collision suppresses only that colliding drop; it does not roll back death/clear, scalar rewards, or independent non-colliding drops

Important note:
- default `training_dummy` / `practice_mob` content remains rewardless unless the QA setup deliberately overrides or authors a non-zero descriptor
- level progression, party distribution, broader quest scripting, and corpse gameplay are still out of scope for this bootstrap smoke; kill-reward exclusive ownership / public release / FileStore rematerialize already reuse the owned player-drop ground path
- kill-quest credit is a separate spawn-group content seam from the EXP/gold/drop descriptor; the combined NPC service QA fixture now authors both, `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` proves the same loop from authoring-form regen/drop input, `docs/examples/bootstrap-kill-quest-credit-bundle.json` remains the narrow direct credit-only example, and `docs/examples/bootstrap-kill-quest-only-drop-table-authoring-bundle.json` is the matching kill-quest-only `drop_tables` authoring form, and `docs/examples/bootstrap-kill-quest-only-regen-authoring-bundle.json` is the matching regen authoring form of that kill-quest-only table (see `quest-state-bootstrap.md`)

### 5.12 Practice-mob retaliation death and restart-here smoke

Run this when the target build has authored QA `spawn_groups` practice-mob content loaded with the current bootstrap retaliation behavior.
If the QA bundle uses a custom registered or bundled combat profile instead of the built-in `training_dummy` / `practice_mob` profiles, expect the same immediate and delayed self-only `PLAYER_POINT_CHANGE` retaliation cadence while the engaged owner remains live.
If that custom profile authors a negative `retaliation_point_delta`, expect both the immediate hit-triggered tick and the delayed server-origin cadence to use that authored amount, clamped only at the `0`-HP floor.

- [ ] Approach and select the visible practice mob
- [ ] Land accepted normal attacks and wait through delayed retaliation beats until the player reaches the owned zero-HP floor
- [ ] Confirm the owner receives the final `PLAYER_POINT_CHANGE` to `0`, then `DEAD(owner_vid)`, then `TARGET(0, 0)`
- [ ] If loopback ops access is available, query `GET /local/combat-target/{character_name}` or `GET /local/combat-targets` after the floor and confirm the dead owner's stale selected target is absent from the read-only snapshot output
- [ ] If a merchant window is open when the immediate or delayed retaliation beat reaches `0` HP, confirm one self-only `GC::SHOP END` follows the death/clear sequence and later `SHOP END` / `SHOP BUY` attempts fail closed until a fresh merchant interaction opens a new window
- [ ] If an accepted host-only `MYSHOP` is open when the immediate or delayed retaliation beat reaches `0` HP, confirm one empty-sign `GC::SHOP_SIGN` follows the death/clear sequence (after any merchant `SHOP END`, before any safebox `CloseSafebox` or exchange close), currently visible peers receive the same empty-sign around-broadcast, later `/close_myshop` stays silent, and inventory/gold stay unchanged
- [ ] If a second living client was browsing that open MYSHOP when the host reached `0` HP, confirm the guest receives one queued `GC::SHOP END` beside the empty-sign around-broadcast and later guest `SHOP END` / `ON_CLICK` stay silent
- [ ] If the dying client itself was browsing another peer's open MYSHOP when retaliation reached `0` HP, confirm one self-only `GC::SHOP END` follows death/clear, browse clears without inventory/gold mutation, and later guest `SHOP END` / `ON_CLICK` stay silent
- [ ] If a bootstrap exchange window is open with a living visible peer when the immediate or delayed retaliation beat reaches `0` HP, confirm the dead owner receives one self-only `GC::EXCHANGE END` after the death/clear sequence, the paired peer receives one queued `GC::EXCHANGE END`, and later stale `EXCHANGE CANCEL` / display requests fail closed
- [ ] If `/open_safebox` was already open when the retaliation beat reaches `0` HP, confirm one self-only `CHAT_TYPE_COMMAND` `CloseSafebox` follows the death/clear sequence (after any merchant `SHOP END` / MYSHOP empty-sign, before any exchange close), then after `/restart_here` a fresh `EXCHANGE START` against a living visible peer succeeds instead of returning the open-safebox busy reject
- [ ] If a refine-dialog preview was already open when the retaliation beat reaches `0` HP, confirm the death/clear sequence stays free of extra refine frames, then after `/restart_here` a fresh `EXCHANGE START` against a living visible peer succeeds instead of returning the open-refine busy reject
- [ ] Try a fresh target or attack while still at `0` HP
- [ ] Confirm the attempt fails closed with no new combat-visible frames
- [ ] Try one carried-inventory `ITEM_MOVE` drag while still at `0` HP
- [ ] Confirm the move fails closed: no item cells change and no item refresh frames are visible
- [ ] If storage/safebox packet tooling is available, try `SAFEBOX_CHECKIN`, `SAFEBOX_CHECKOUT`, `SAFEBOX_ITEM_MOVE`, and `MALL_CHECKOUT` while still at `0` HP; also try `SAFEBOX_CHECKIN` with an `anti_safebox` item that would show template-authored info-chat feedback while alive
- [ ] Confirm post-floor storage attempts stay silent and non-mutating: no safebox/mall response frames, no `anti_safebox` info-chat feedback, no carried inventory/equipment/quickslot/point/gold changes, no ground handles, and no persistence update
- [ ] Issue whitespace-padded restart-looking chat such as `/restart_here ` or `/ restart_town` on the same socket
- [ ] Confirm the owner remains dead and no recovery/peer frames are emitted
- [ ] Issue exact `/restart_here` on the same socket
- [ ] Confirm the character rebuilds in place with the ordinary self bootstrap burst and restored persisted HP
- [ ] Confirm the same recovery also refreshes currently visible practice-mob / static-actor presence with delete-plus-readd catch-up frames after the self burst, so any lifecycle the dead owner skipped is resynchronized before combat resumes
- [ ] Confirm a stale attack still fails until the practice mob is selected again
- [ ] If loopback ops access is available, confirm the combat-target snapshot remains absent after `/restart_here` until the practice mob is freshly selected again
- [ ] With a second living visible client, confirm that a practice mob left alive by the owner's zero-HP floor can be freshly targeted by that second client without waiting for mob death / respawn or owner disconnect
- [ ] In a separate live-owner run, land one accepted hit on a practice mob, clear the selected target before the delayed retaliation timer expires, and confirm no delayed retaliation beat arrives for that cleared target
- [ ] In that same clear-target run, wait through the owned `5s` chase delay and confirm no delayed chase `MOVE` arrives for the abandoned engagement after client `TARGET(0)`
- [ ] In that same clear-target run, confirm a second living visible client can freshly target the still-live practice mob after the owner clears it
- [ ] Re-select the still-live practice mob and confirm its HP remains at the current runtime-owned value instead of resetting because of `/restart_here`
- [ ] In a separate timing run, let a second living client kill the practice mob while the owner is still at `0` HP, wait until the owned respawn delay is already due, then issue `/restart_here` and confirm the recovery catch-up shows the live rebuilt mob (no stale `DEAD` replay and no duplicate queued respawn afterward)
- [ ] In another timing run, while the owner is still at `0` HP, displace a still-live authored spawn-backed practice mob outside leash so it arms a server-owned return-step, wait until that return-step deadline is already due without flushing the dead owner's skipped lifecycle frames, then issue `/restart_here` and confirm the recovery catch-up shows the stepped post-return-step position (not the pre-step displaced coords), no duplicate queued return-step rebuild follows, and the mob remains non-targetable while it still classifies `return_required`
- [ ] In a separate timing run with a second living engager still holding the practice mob, keep that engaged chase-step deadline already due, then issue `/restart_here` from a floored nearby client and confirm the recovery catch-up shows the stepped chase position instead of stale pre-chase coords followed by a duplicate queued chase-step rebuild; the live engager should observe the retained chase `MOVE`
- [ ] In another timing run with a second living engager that chase-displaces then releases engagement (`TARGET(0)` after leaving aggro) so a within_radius homeward deadline is already due, issue `/restart_here` from a floored nearby client and confirm the recovery catch-up shows the stepped homeward/home position instead of stale within_radius coords followed by a duplicate queued homeward rebuild; the living engager should observe the retained homeward `MOVE`
- [ ] Optional fixture/debug guard: if the selected character's persisted account snapshot is deliberately seeded at `0` HP, issue `/restart_here` and confirm it recovers with race create MaxHP persisted and a live self bootstrap burst

### 5.12.1 Practice-mob pending-retaliation cleanup on mob death

Run this when the target build has authored QA `spawn_groups` practice-mob content loaded and enough selected-player HP to kill the mob before player death.

- [ ] Select the visible practice mob and land one accepted hit to arm the delayed server-origin retaliation cadence
- [ ] Continue accepted hits, respecting the owned normal-attack cadence, until the practice mob reaches the zero-HP death edge
- [ ] Confirm the killing hit shows only the mob `DEAD(target_vid)` plus `TARGET(0, 0)` clear from the combat lifecycle, not an extra owner-side retaliation point-change
- [ ] Wait less than the owned respawn delay and confirm no stale delayed retaliation beat arrives after mob death
- [ ] Wait until the owned respawn delay expires and confirm the ordinary mob rebuild burst (`CHARACTER_DEL` + add/info/update)
- [ ] In a separate run, disconnect or leave the killer after mob death, wait until the respawn delay has already expired, then enter with a fresh nearby client and confirm its initial static-actor bootstrap shows the mob live at full HP with add/info/update only: no stale `DEAD(target_vid)` replay and no redundant queued respawn rebuild after entry
- [ ] In a separate still-dead timing run, kill an authored `spawn_groups` practice mob, disconnect or leave before the owned respawn delay expires, then enter with a fresh nearby client and confirm its initial static-actor bootstrap shows add/info/update followed by one trailing `GC DEAD(target_vid)` replay: the still-dead mob must stay non-targetable, must not look silently alive, and must not flush respawn early
- [ ] In a separate daemon-restart still-dead run, kill an authored `spawn_groups` practice mob, restart `gamed` before the owned respawn delay expires, then enter with a fresh nearby client and confirm the rematerialized mob stays still-dead: add/info/update plus one trailing `GC DEAD(target_vid)`, fail-closed `TARGET` / `ATTACK`, and no early live full-HP resurrection; once the absolute deadline is due, confirm the ordinary respawn rebuild restores live full HP and clears any still-dead persistence fields from the static-actor snapshot
- [ ] If the materialized spawn-backed mob had been moved away from its authored spawn position by a local/runtime update before death, confirm the respawn rebuild returns it to the authored home rather than to the displaced current position; old-position-only viewers should see only the delete while authored-home viewers see the add/info/update burst
- [ ] If `/local/static-actor-respawns/{entity_id}` is available, confirm the due respawn row disappears after a successful rebuild; if the mob had a stale pending `/local/spawn-group-return-steps/{entity_id}` row before death, confirm that return-step row is also gone after the authored-home respawn; if a static-actor store failure is intentionally injected in a test harness, confirm the actor stays dead/displaced and the due row remains visible for retry instead of queuing a partial rebuild
- [ ] Confirm no stale delayed retaliation beat arrives immediately after that respawn rebuild unless the mob is freshly reselected and hit again
- [ ] If a debug harness can preserve a stale selected target across the respawn boundary, confirm the selected client receives one `TARGET(0, 0)` clear before the respawn rebuild and that later attacks still fail closed until a fresh `TARGET` succeeds

Expected result:
- owner-side retaliation death uses `PLAYER_POINT_CHANGE(value=0)` -> `DEAD(owner_vid)` -> `TARGET(0, 0)`
- `/restart_here` is accepted only after the zero-HP floor and keeps the session in `GAME`
- player HP is rebuilt from race create MaxHP into the persisted snapshot on accepted restart, while a still-live practice mob keeps its runtime-owned HP and requires fresh target acquisition; a zero-HP owner does not keep that mob orphan-locked against fresh targeting from another living visible client
- `/restart_here` also preflights due mob respawn / return-step / chase-step / homeward-step timers and refreshes currently visible static actors for the recovered owner, so skipped zero-HP lifecycle frames do not leave stale local mob visuals
- if persisted player HP is already `0`, `/restart_here` recovers that dead snapshot by writing race create MaxHP and emitting the ordinary live recovery burst
- post-floor `ITEM_MOVE` is silent and non-mutating until a restart/recovery seam is used
- open bootstrap exchange windows are closed with `GC::EXCHANGE END` on the player-death edge and do not remain usable after the owner reaches the zero-HP floor
- mob death cancels pending delayed retaliation, respawn clears any stale selected-target / aggro-lite ownership that survived to the rebuild boundary, and respawn does not resurrect stale retaliation work without fresh target acquisition

### 5.13 Practice-mob retaliation restart-town smoke

Run this when the QA character can safely exercise the bootstrap town-return recovery after the retaliation-owned zero-HP floor.

- [ ] Drive the selected player to `0` HP through the current practice-mob retaliation loop
- [ ] Issue `/restart_town` on the same socket
- [ ] Confirm the character stays in `GAME` and receives the ordinary self bootstrap burst at the owned empire town-return position; when disposable QA characters are available for empires 1, 2, and 3, smoke each owned table row rather than only the empire-2 path
- [ ] If the town-return crosses maps away from a visible practice mob, confirm the same socket also receives the ordinary source-map `CHARACTER_DEL` teardown for that mob after the self bootstrap burst
- [ ] Confirm later movement/interaction works from the town-return position after recovery
- [ ] Reconnect and confirm the town-return position plus recovered race create MaxHP persisted
- [ ] In a separate timing run with a living destination engager still holding an authored practice mob near the owned empire town-return point, keep that engaged chase-step deadline already due, then issue `/restart_town` from a floored source-map client and confirm destination visibility in the recovery burst shows the stepped chase position instead of stale pre-chase coords followed by a duplicate queued chase-step rebuild
- [ ] In another timing run, while a destination-map authored spawn-backed practice mob is displaced outside leash so a server-owned return-step is already due, issue `/restart_town` from a floored source-map client and confirm destination visibility in the recovery burst shows the stepped post-return-step position instead of the pre-step displaced coords, with no duplicate queued return-step rebuild afterward; the stepped mob must remain non-targetable while it still classifies `return_required`
- [ ] In another timing run with a living destination engager that chase-displaces then releases engagement (`TARGET(0)` after leaving aggro) so a within_radius homeward deadline is already due near the owned empire town-return point, issue `/restart_town` from a floored source-map client and confirm destination visibility in the recovery burst shows the stepped homeward/home position instead of stale within_radius coords followed by a duplicate queued homeward rebuild; the living engager should observe the retained homeward `MOVE`
- [ ] Optional fixture/debug guard: if the selected character's persisted account snapshot is deliberately seeded at `0` HP, issue `/restart_town` and confirm it recovers with race create MaxHP plus the town-return position persisted

Expected result:
- `/restart_town` is accepted only after the zero-HP floor
- the selected player rebuilds with race create MaxHP and moves to the currently owned empire create-position fallback
- if persisted player HP is already `0`, `/restart_town` recovers that dead snapshot by writing race create MaxHP and the town-return position
- `/restart_town` also preflights due destination-map respawn / return-step / chase-step / homeward-step timers before encoding destination static-actor visibility, matching the EnterGame / transfer / `/restart_here` lifecycle preflight contract
- source-map non-player visibility is torn down through existing delete frames when the town restart leaves that map
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
- [ ] Confirm the stale socket may still receive only its self-local merchant success burst (`ITEM_SET` / `ITEM_UPDATE` refreshes without `GC::SHOP OK`) on either packet `SHOP BUY` or the local `/shop_buy` harness
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
- [ ] On the stale old socket, keep the merchant gate active and issue `SHOP BUY` (or `/shop_buy <slot>` in the local harness) so only the stale socket sees the local success burst (`ITEM_SET` / `ITEM_UPDATE` refreshes without `GC::SHOP OK`)
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
- [ ] Confirm the harness returns one inventory refresh per changed carried slot in carried-slot order (`ITEM_SET` for fresh slots, `ITEM_UPDATE` for existing stack fills) and no extra merchant success companion
- [ ] If the QA template authors visible sockets/attributes for that stackable item, confirm each existing-stack `ITEM_UPDATE` preserves those display arrays instead of zeroing them while only the count changes
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
- the response stays self-only and uses the existing `ITEM_DEL` / `ITEM_SET` / `ITEM_UPDATE` refresh family; quickslot sync retargets source item quickslots only for full-stack moves/swaps where the source item lands in the destination cell, deletes stale destination-cell item quickslots first, preserves source item quickslots for partial splits/merges, and deletes source item quickslots when the source stack is fully consumed by a compatible merge; the bootstrap `/inventory_move` compatibility seam follows the same authored-template incompatible-swap guard boundary and quickslot retarget/delete rule for accepted full-stack carried-cell moves
- non-carried windows and out-of-range cells fail closed without mutation

### 6.20 `ITEM_USE_TO_ITEM` stack consolidation smoke (packet-harness optional)

- [ ] Enter `GAME` with a QA character that has two compatible carried stacks for a template-backed stackable item such as `27001`
- [ ] Send one real client `ITEM_USE_TO_ITEM` request from the source carried slot onto the target carried slot
- [ ] Confirm the selected session receives `ITEM_DEL(source)` then count-only `ITEM_UPDATE(target)` when the source fits completely into the target; if the QA template authors display sockets/attributes for that stackable item, confirm the target update preserves those arrays while changing only the count
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
- [ ] If a runtime-locked carried or equipped fixture is present, inspect `/local/inventory/{name}` or `/local/equipment/{name}` and confirm that only the locked rows include `locked: true`

Expected result:
- packet `SELL` / `SELL2` mutate the selected character's carried inventory and gold while the merchant context is active
- the current bootstrap sell price uses the loaded item template's ordinary shop-buy price and count-per-gold flag through the owned legacy count/price branch, then applies the shared `/5` and `3%` tax floors; anti-sell and runtime-locked item guards fail closed without mutation, while richer sell UI choreography remains later slices
- loopback inventory/equipment snapshots expose runtime lock state for QA/debugging while unlocked rows keep the compact existing shape
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
- if a dead combat actor is explicitly updated to a different combat profile through an operator/debug seam, the stale death timer is cancelled and the updated actor becomes a fresh live snapshot at the new profile's full HP instead of replaying the old profile's pending respawn rebuild later
- outright runtime removal of a still-selected dummy tears down both the visible actor and the selected combat-target ownership immediately: the client sees the ordinary `CHARACTER_DEL` plus one self-only `GC TARGET(0, 0)` companion before later stale attacks stay denied
- a dead (`0` HP) dummy is no longer eligible for accepted bootstrap target selection or attack refreshes
- these rejections stay silent in the current slice: no peer fanout, no compensating chat spam, and no accidental HP mutation

### 6.23 Training-dummy zero-HP death clears selected targets (packet-harness optional)

- [ ] Prepare two visible sessions if possible: one attacker and one watcher that can also select the same visible `training_dummy`
- [ ] On both sessions, select the same dummy and confirm the normal self-only `GC TARGET(target_vid, 100)` ack before any attacks
- [ ] From the attacker, issue successive normal `ATTACK` requests until the dummy reaches its final accepted hit from `1` to `0`
- [ ] Confirm non-lethal standalone `training_dummy` / runtime-registered practice-profile hits still use the normal self-only `GC TARGET(target_vid, hp_percent)` refresh path (`90`, `80`, ... , `10`) and append one self-only plain `GC DAMAGE_INFO(target_vid, damage)` hit-effect companion after the target refresh
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

- [ ] Import or preload one authored `spawn_groups` entry that materializes a visible stationary practice mob using either owned built-in practice profile (`combat_profile = training_dummy` or `combat_profile = practice_mob`)
- [ ] Confirm the mob appears at the authored position with the authored display name and can be targeted in the same way as the earlier bootstrap dummy slices
- [ ] With two visible clients, let client one land the first accepted hit and verify client two's fresh `TARGET` attempt on the already-engaged mob fails closed while client one still owns that live engagement
- [ ] On the owning client, confirm each accepted live content-loaded hit now returns the usual target-refresh, one immediate self-only HP `POINT_CHANGE` decrement, one self plain `DAMAGE_INFO(target_vid, damage)` mob hit-effect companion, and then one self plain `DAMAGE_INFO(owner_vid, abs(delta))` retaliation companion while the mob remains alive; with a second visible live client, confirm that peer receives the matching queued mob `DAMAGE_INFO` plus the matching owner retaliation `DAMAGE_INFO` without also receiving the owner's self-only target refresh or retaliation point-change
- [ ] With three visible clients, let client one die to practice-mob retaliation at `0` HP (self `DEAD` + clear-target), then have client two reacquire the still-live mob and land one accepted non-lethal hit; confirm client three receives the queued mob + owner retaliation `DAMAGE_INFO` companions while dead client one receives none, and after the owned `1s` delayed beat confirm the same dead-recipient skip for the delayed owner `DAMAGE_INFO`
- [ ] If you can control timing precisely, send one repeated normal `ATTACK` against that same live selected mob before the owned `250ms` cadence window expires and confirm it fails closed with no target refresh, no extra immediate retaliation tick, and no delayed-cadence reset
- [ ] Wait at least the owned `250ms` cadence window and confirm the next same-target normal `ATTACK` is accepted again
- [ ] If two visible practice targets are available, land one accepted hit on the first target, immediately select the second target, and confirm an immediate normal `ATTACK` against that second target still fails closed until the same `250ms` cadence window expires
- [ ] In that same two-target setup, have a second visible client try a fresh `TARGET` against the first still-live mob after the owner retargets to the second mob; confirm the attempt still fails closed until the owner sends an explicit `TARGET(0)` or another owned release boundary occurs, then confirm a fresh second-client `TARGET` succeeds at the first mob's current runtime-owned HP after that explicit clear
- [ ] After the first accepted live owner hit, stop sending `ATTACK` for at least the owned `1s` retaliation delay and confirm one queued self-only HP `POINT_CHANGE` follow-up beat plus one self plain `DAMAGE_INFO(owner_vid, abs(delta))` arrives without a second client attack; with a second visible live client, confirm that peer also receives the matching owner retaliation `DAMAGE_INFO` without the owner's delayed point-change
- [ ] Wait one more owned `1s` delay without another accepted hit and confirm a second queued self-only HP `POINT_CHANGE` follow-up beat plus another owner self plain `DAMAGE_INFO` arrives while the mob stays alive and engaged; confirm the second visible live client again receives the matching owner retaliation `DAMAGE_INFO`
- [ ] If the practice mob uses a custom registered or bundled combat profile with a negative `retaliation_point_delta`, confirm the immediate owner-side `PLAYER_POINT_CHANGE` and the delayed server-origin follow-up beats use that authored negative amount rather than the default `-1`, clamping only the final floor transition to the remaining positive HP
- [ ] If you can control timing precisely, land a later accepted owner hit while one autonomous delayed beat is already pending and confirm the current slice still yields only one queued delayed follow-up beat on the original timer rather than accelerating or resetting that cadence window
- [ ] If you can control timing precisely, also try a rapid second accepted hit before the first delayed beat fires and confirm the current slice still yields only one queued delayed follow-up beat for that first pending window
- [ ] While the practice mob is damaged but still alive, re-import the exact same canonical authored content bundle and confirm no replacement `CHARACTER_DEL` / add-info-update burst is queued, the current target remains selected, and the next accepted hit continues from the damaged HP percentage instead of resetting to full
- [ ] Lower the owning character's HP near `0` if you can, then confirm the immediate or delayed retaliation tick clamps at `0` HP instead of going negative and that no further delayed follow-up beat arrives once that floor is reached
- [ ] After retaliation has already driven the owning character to `0` HP, use `/phase_select` or a full disconnect/reconnect and confirm the fresh bootstrap rebuilds the owner's points at `0` plus one self-only `DEAD(owner_vid)` from the persisted death floor instead of silently restoring the pre-retaliation live value
- [ ] If a dedicated QA account/fixture is already persisted at `0` HP or below before `ENTERGAME`, confirm the selected-character bootstrap sends the ordinary self burst through `PLAYER_POINT_CHANGE(..., amount = 0, value = 0)` and then one self-only `DEAD(owner_vid)` before any trailing peer/static/ground visibility frames; `/restart_here` and `/restart_town` should recover that dead snapshot by writing race create MaxHP
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then send a same-map `MOVE`, `SYNC_POSITION`, or transfer-triggering move/rebootstrap and confirm the live session still shows the reduced runtime-only retaliation points while a later reconnect still rebuilds from the older persisted point value instead of leaking that partial retaliation loss through position-only saves
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then send one successful `/use_item <slot>` or carried-slot `ITEM_USE` and confirm the item consumption plus its authored use-effect point delta persist while a later reconnect still rebuilds from the older persisted point value plus that owned use-item delta instead of leaking the runtime-only partial retaliation loss through the point-bearing use-item save. If the consumed stack reaches zero and an item quickslot points at that carried slot, confirm the client receives `ITEM_DEL` followed by `QUICKSLOT_DEL` for item quickslots only before the self `INFO` effect message, and that skill/command quickslots using the same byte value remain intact.
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then send one successful `/equip_item <slot> <equip_slot>` and confirm the carried->equipped item mutation plus its authored equip-effect point delta persist while a later reconnect still rebuilds from the older persisted point value plus that owned equip delta instead of leaking the runtime-only partial retaliation loss through the point-bearing equip save
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then send one successful `/unequip_item <equip_slot> <slot>` and confirm the equipped->carried item mutation plus the authored equip-effect removal persist while a later reconnect still rebuilds from the older persisted point value minus that owned equip delta instead of leaking the runtime-only partial retaliation loss through the point-bearing unequip save
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then send one successful `/inventory_move <from_slot> <to_slot>` and confirm the carried-slot mutation persists while a later reconnect still rebuilds the older persisted point value instead of leaking that runtime-only partial retaliation loss through the non-point-bearing inventory save
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then complete one successful merchant preview + `/shop_buy 0` purchase and confirm the bought item plus gold debit persist while a later reconnect still rebuilds the older persisted point value instead of leaking that runtime-only partial retaliation loss through the non-point-bearing merchant-buy save
- [ ] Before retaliation reaches `0` HP, land one accepted hit, then complete one successful merchant preview + `SHOP SELL` or `SHOP SELL2` sell-back and confirm the carried-slot / quickslot / gold result persists while a later reconnect still rebuilds the older persisted point value instead of leaking that runtime-only partial retaliation loss through the non-point-bearing merchant-sell save
- [ ] After that `/phase_select` recovery on the same socket, if the same practice mob stayed alive, send a fresh `TARGET` and confirm the ack still shows the mob's current runtime-owned HP instead of silently resetting it to full, then send one normal `ATTACK` and confirm the usual target-refresh plus immediate self-only retaliation resumes
- [ ] After retaliation has already driven the owning character to `0` HP, send same-socket `/restart_here` and confirm the owner stays in `GAME`, receives the ordinary self bootstrap burst (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE` -> `PLAYER_POINT_CHANGE`) rebuilt with race create MaxHP persisted, and that a visible live peer sees one queued delete-plus-rebootstrap refresh for that owner (`CHARACTER_DEL` -> `CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE`)
- [ ] Immediately after `/restart_here`, try one same-target normal `ATTACK` without reselecting and confirm it still fails closed on the live socket with no response frame; then send a fresh `TARGET` and confirm the same still-live practice mob resumes from its current runtime-owned HP instead of resetting because of the owner's recovery
- [ ] While the owner is still alive, try same-socket `/restart_here` and confirm it fails closed with no self bootstrap burst and no peer-facing refresh
- [ ] After retaliation has already driven the owning character to `0` HP, send same-socket `/restart_town` and confirm the owner stays in `GAME`, receives the ordinary self transfer rebootstrap burst rebuilt with race create MaxHP at the owned empire town-return coordinates, and no longer keeps the old practice mob selected
- [ ] With one visible live peer still on the source map and one visible live peer already on the destination town map, confirm `/restart_town` queues one `CHARACTER_DEL` to the source peer, one queued owner re-entry burst (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE`) to the destination peer, and one self-facing `CHARACTER_DEL` for the source practice mob after the owner's self rebootstrap burst
- [ ] Immediately after `/restart_town`, try one same-target normal `ATTACK` without reselecting and confirm it still fails closed on the live socket with no response frame
- [ ] Immediately after `/restart_town`, try a fresh `TARGET` from the town-restarted owner against the old source-map practice mob and confirm it fails closed while ordinary visibility keeps that mob outside the owner's new town position
- [ ] From the still-live source-map peer, send a fresh `TARGET` against that same still-live practice mob and confirm the ack shows the current runtime-owned HP instead of full HP, proving the owner's town recovery did not reset the source-map mob; if the owner later returns to visibility, a fresh owner-side `TARGET` should follow the same current-HP rule
- [ ] After that accepted `/restart_town`, let one fresh live peer enter the destination town visibility later and confirm it sees the recovered owner through the ordinary peer-entry burst only (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE`) with no replayed `GC DEAD(owner_vid)` from the earlier pre-restart death window
- [ ] When that immediate or delayed retaliation floor reaches `0` HP, confirm the client receives one self-only `PLAYER_POINT_CHANGE(HP, amount = final clamped negative retaliation delta, value = 0)`, then one self-only `DEAD(owner_vid)`, then one self-only `TARGET(0, 0)` clear instead of keeping the stale engaged practice mob selected
- [ ] After that retaliation floor reaches `0` HP, wait at least one more owned `1s` retaliation delay and confirm no further delayed `PLAYER_POINT_CHANGE` arrives for the stale engagement
- [ ] After retaliation has already driven the owning character to `0` HP, send a fresh combat `TARGET` against the same or another visible practice mob and confirm it fails closed with no self-only target acknowledgement
- [ ] After retaliation has already driven the owning character to `0` HP, send another same-target normal `ATTACK`; confirm it fails closed with no target refresh, no extra point-loss, and no re-armed delayed follow-up beat
- [ ] If a packet harness or class-skill client action can send `USE_SKILL`, send one against the selected practice mob and confirm the current bootstrap guard fails closed with no self frames, no queued peer frames, no target HP mutation, and no normal-attack cadence/retaliation change; the next normal `ATTACK` should still behave as the first accepted hit after the last owned cadence boundary
- [ ] If a packet harness, bow action, or class-skill client action can send `SHOOT`, send one ranged-shot packet while the practice mob remains selected; confirm the current bootstrap guard fails closed with no self frames, no queued peer frames, no target HP mutation, and no normal-attack cadence change
- [ ] If a packet harness, bow action, or class-skill client action can send `FLY_TARGETING` / `ADD_FLY_TARGETING`, send one projectile-targeting packet with the current target `VID` and one additional target/position packet while the practice mob remains selected; confirm both current bootstrap guards fail closed with no self frames, no queued peer frames, no target HP mutation, and no normal-attack cadence/retaliation change
- [ ] If a packet harness or client click path can send `ON_CLICK`, send one click packet for the selected practice mob and one for a visible static actor/NPC while combat remains active; confirm the current bootstrap guard fails closed with no self frames, no queued peer frames, no selected-target rewrite, no target HP mutation, no normal-attack cadence/retaliation change, and no interaction/shop/quest side effect
- [ ] If a packet harness or battle-position client path can send `CHARACTER_POSITION` while the practice mob remains selected and the owner is still alive, confirm accepted stand/sit/chair requests remain presentation-only (`GC CHARACTER_POSITION` to self and currently visible live peers only when the normalized stance changes) with no selected-target rewrite, no target HP mutation, and no normal-attack cadence/retaliation change; unsupported battle-position bytes should still fail closed with no frames
- [ ] After retaliation has already driven the owning character to `0` HP, send one accepted-position `CHARACTER_POSITION` stance request and confirm it now fails closed with no self `GC CHARACTER_POSITION`, no queued peer stance frame, no selected-target rewrite, no target HP mutation, and no normal-attack cadence/retaliation change
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
- [ ] After one accepted owner-side hit starts the delayed `1s` retaliation beat, wait only half the delay, land another accepted normal hit after the `250ms` attack cadence expires, and confirm there is still no delayed retaliation before the original due time; at the original `1s` due time, confirm exactly one delayed `PLAYER_POINT_CHANGE` arrives, proving accepted hits while one delayed beat is already pending do not stack, accelerate, or reset the timer
- [ ] Replace the selected practice-mob target before the next owned `1s` delay expires and confirm the abandoned target's queued delayed follow-up beat is cancelled, then have a second visible player try to `TARGET` that abandoned still-live mob and confirm it still fails closed while the old owner remains connected and engaged; after the owner sends explicit `TARGET(0)` or reaches another owned release boundary, confirm the second player can `TARGET` that same mob at its current runtime-owned HP instead of full HP
- [ ] Move or sync far enough to force a self `TARGET(0, 0)` clear before the next owned `1s` delay expires and confirm that same queued delayed follow-up beat also fails closed after target invalidation, then have a second visible player `TARGET` the abandoned still-live mob and confirm the ack now succeeds at its current runtime-owned HP
- [ ] After the first accepted owner hit but before the next owned `1s` delay expires, cross a transfer trigger / rebootstrap seam and confirm the owner gets the normal self transfer burst with no late delayed retaliation beat, while a still-visible source-map peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP
- [ ] After the first accepted owner hit, query loopback `/local/combat-targets` or `/local/combat-target/{name}` if available and confirm the selected target snapshot includes the engaged owner's `engaged_by_entity_id` / `engaged_by`, the current runtime-owned `hp_percent`, the owned `retaliation_point_delta` with `retaliation_server_origin: true`, and the armed delayed beat as `retaliation_pending`, `retaliation_ready_at`, and `retaliation_remaining_ms`; this is a read-only debug check and should not produce any gameplay packet or mutate combat state
- [ ] After the first accepted owner hit but before the next owned `1s` delay expires, issue `/logout` on the owning session and confirm it transitions toward close with no later queued retaliation beat, any visible peer sees the owner disappear cleanly, and that peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP
- [ ] Repeat the same pending-cadence teardown with `/quit` and confirm the owner still receives self `CHAT_TYPE_COMMAND quit` while staying in `GAME`, but any visible peer still sees the owner disappear cleanly, the queued retaliation beat is cancelled, and that peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP without waiting for disconnect completion
- [ ] Repeat the same pending-cadence teardown with `/phase_select` and confirm the owner transitions back to character select, any visible peer still sees the owner disappear cleanly, the queued retaliation beat is cancelled, and that peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP while a later fresh bootstrap still requires the owner to reselect the mob
- [ ] Repeat the same pending-cadence teardown with an abrupt socket close or client disconnect and confirm any visible peer still sees the owner disappear cleanly, the queued retaliation beat is cancelled, and that peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP without waiting for a later reconnect
- [ ] In a harness that can reclaim the same selected character through a second `ENTERGAME` while a pending delayed practice-mob retaliation beat is armed, confirm reclaim clears that pending beat and releases aggro-lite engagement so a still-visible peer can immediately `TARGET` the same still-live practice mob at its current runtime-owned HP, with no late delayed retaliation frames on the stale or replacement session
- [ ] If the content-loaded mob has a bootstrap EXP-only death reward configured, drive one full kill and confirm the killing client receives `GC DEAD(mob_vid)` -> `GC TARGET(0, 0)` -> one self-only `PLAYER_POINT_CHANGE(POINT_EXP = 3)` with the configured EXP amount and new EXP total
- [ ] If the content-loaded mob has a bootstrap gold-only death reward configured, drive one full kill and confirm the killing client receives `GC DEAD(mob_vid)` -> `GC TARGET(0, 0)` -> one self-only `PLAYER_POINT_CHANGE(POINT_GOLD = 11)` with the configured gold amount and new gold total
- [ ] Reconnect after a successful EXP-only or gold-only reward kill and confirm the selected character snapshot reflects the persisted reward total
- [ ] Repeat with a drop-only reward descriptor in a debug harness if available and confirm the kill returns `GC DEAD(mob_vid)` -> `GC TARGET(0, 0)` -> one self-only `ITEM_GROUND_ADD` at the killer's current coordinates plus one self-only `ITEM_OWNERSHIP` for the killer per configured drop vnum, with no inventory/account persistence mutation from the drop reward alone
- [ ] In that same drop-only reward harness, mark one configured reward-drop item template as currently restricted for the killer (`anti_get`, `anti_drop`, `anti_give`, `anti_sell`, `anti_stack`, equipment-shaped, or an owned job/sex/empire/min-level guard) while leaving another configured drop valid; confirm the killing hit still emits the owned death/clear frames and only the valid drop's `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` pair, with no live ground handle for the restricted vnum
- [ ] In a debug harness with an invalid reward owner display name/login shape (including blank, padded, embedded-whitespace, embedded-NUL, invalid UTF-8, or owner-name metadata longer than the fixed 25-byte ownership field), confirm the killing hit still emits death/clear plus any scalar reward feedback, but emits no `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` frames and leaves no live reward ground handle
- [ ] When loading a self-contained authored content bundle that includes top-level `item_templates`, confirm every configured `reward_drop_vnums` entry in `spawn_groups` is backed by one of those item templates; a bundle with a dangling drop vnum should be rejected by the loopback content-bundle import before any runtime actor replacement occurs
- [ ] If a retaliation-driven owner is already at `0` HP, try an operator/debug ground-gold drop registration for that owner and confirm it fails closed with no `ITEM_GROUND_ADD`, no `ITEM_OWNERSHIP`, and no live ground item occupying the map
- [ ] Drive one full target -> hit -> zero-HP death -> timed respawn -> fresh reselect cycle against that content-loaded mob
- [ ] Re-export or otherwise inspect authored content and confirm the actor still round-trips as `spawn_groups`, not as an interaction-backed `static_actor`

Expected result:
- the first attackable content-loaded mob now comes from the authored `spawn_groups` seam instead of ad hoc runtime-only bootstrap registration
- its runtime combat loop still reuses the owned built-in practice-profile semantics for HP, death, timed respawn, retaliation, and the first fixed same-target `250ms` normal-attack cadence gate, while current spawn-backed `DAMAGE_INFO` hit-effect emission now covers the owner and currently visible live peers for both mob hits and non-floor owner retaliation without forwarding the owner's self-only target refresh or retaliation point-change
- after the first accepted hit, the mob now owns one tiny aggro-lite gate: fresh third-party `TARGET` attempts fail closed while the engaged owner still lives, but that same still-live mob becomes targetable again if retaliation kills the current owner before mob death / respawn resets it
- while alive, each accepted owner-side hit also applies one deterministic immediate self-only HP decrement back to that engaged session, and the first accepted live hit now starts a delayed self-only follow-up cadence that keeps firing one beat at a time after each owned `1s` server timer while the same engagement remains live
- bootstrap death rewards now cover EXP-only and gold-only persisted point/currency rewards plus a first single-drop-vnum ground-item reward that is visible to the killer without mutating inventory/account persistence by itself
- that owner-side retaliation point-loss now clamps at `0` HP too; once the floor is reached the current slice emits self-only `DEAD(owner_vid)` plus self-only `TARGET(0, 0)`, tears down any already-open merchant preview with one self-only `GC::SHOP END` after that same floor transition, persists the selected-character bootstrap HP point as `0`, and later same-owner combat `TARGET` / `ATTACK` attempts fail closed too, without yet claiming broader corpse gameplay
- that retaliation point-loss stays runtime-only while the owner remains above the floor, so a fresh `/phase_select` or reconnect bootstrap still rebuilds the owner's points from persisted state rather than carrying partial retaliation loss across sessions; once the floor is reached, reconnect/`ENTERGAME` rebuilds from persisted `0` HP plus self `DEAD`, while later successful `/use_item`, `/equip_item`, and `/unequip_item` saves still persist their own authored use/equip point delta plus the associated item/equipment mutation and a still-live practice mob keeps its current runtime-owned HP and still requires a fresh post-recovery `TARGET`
- same-target normal `ATTACK` attempts inside the owned `250ms` cadence window fail closed without refreshing target HP, without appending another immediate retaliation tick, and without resetting delayed retaliation timing
- while one delayed follow-up beat is already pending, extra accepted hits do not stack, accelerate, or reset the current cadence timer; the already-pending delayed beat still fires once at its original due time
- if that engagement loses target intent through an explicit release boundary — client `TARGET(0)`, movement / sync forcing a self `TARGET(0, 0)` clear, transfer/rebootstrap, owner disappearance, owner zero-HP stale cleanup, actor update/removal, or mob death/respawn — the pending delayed follow-up beat should fail closed and the abandoned still-live mob should become targetable again at its current runtime-owned HP instead of staying orphan-locked behind the old owner; merely replacing the selected target with another non-zero target does not release the already-engaged mob to third-party target selection
- a successful transfer / rebootstrap during that pending window also counts as an immediate cadence-reset boundary: the owner gets the normal self transfer burst with no late delayed retaliation beat, and a still-visible source-map peer can immediately retarget the same still-live mob at its current runtime-owned HP
- a same-socket `/quit`, `/logout`, or `/phase_select` also counts as an immediate owner-disappearance boundary for that cadence in the current slice, and abrupt session close does too: the owner leaves shared-world visibility right away, the pending delayed follow-up beat is cancelled, and the still-live practice mob becomes targetable again at its current runtime-owned HP without waiting for disconnect, close, or later fresh bootstrap completion
- EnterGame reclaim of a stale same-character owner mid-pending delayed retaliation also counts as that same ownership-loss boundary: reclaim clears the shared-world pending timer, releases aggro-lite engagement, and lets a still-visible peer retarget the damaged mob without inheriting a late delayed beat on the stale or replacement session
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
- [ ] item use beyond the currently owned bootstrap special-effect / transfer ITEM_USE guards
- [ ] richer merchant-window choreography beyond the current bootstrap open / buy / sell / close slice (for example stock depletion, cash shops, or extra `UPDATE_ITEM` / `UPDATE_PRICE` polish)
- [ ] broader mob/skill combat beyond the current `training_dummy` / content-loaded `practice_mob` target -> hit -> death -> timed-respawn loop
- [ ] client quest packets, quest acceptance/completion UI, branching dialog trees, or script-VM quest runtime beyond the owned `quest_flag` / kill-quest credit compare-and-set seam
- [ ] broader player death / respawn systems beyond the current retaliation-owned `DEAD`, `/restart_here`, and `/restart_town` bootstrap seams
- [ ] random/weighted loot tables, party/contribution reward splits, level-up/stat recalculation choreography, corpse gameplay, or public-loot expiry beyond the current deterministic bootstrap EXP/gold/drop-vnum reward descriptor seam
- [ ] multi-channel real behavior
- [ ] polished client-facing warp/loading choreography

Important note:
- the project has operator-side transfer primitives and ongoing runtime transfer work
- for current QA, validate only the existing bootstrap warp/rebootstrap path; polished final warp/loading choreography is still not a general pass/fail gate
- do **not** treat the owned bootstrap quest loop as out of scope: when QA content is loaded, `quest_flag` unlock/turn-in, gated `info` / `talk` / `warp` / `shop_preview` / `open_safebox`, kill-quest credit, and turn-in gold/experience/item consume+grant feedback are expected pass/fail checks (see §5.4 / §5.11)
- do **not** treat bootstrap merchant sell-back or `quest_flag` inventory/currency mutation as out of scope when those seams are under test

---

## 10. Exit criteria for a healthy current build

A current build is a good candidate when all of these pass:

- [ ] channel visible online
- [ ] valid login works
- [ ] selection screen is usable
- [ ] create/select/enter-game work
- [ ] single-client movement works
- [ ] reconnect works
- [ ] when authored QA NPC content is loaded, supported NPC smoke checks (`info` / `talk`, `quest_flag`, `shop_preview`, `warp`, `open_safebox`) pass without disconnecting the client
- [ ] when authored QA quest content is loaded, the owned guide unlock -> gated services -> kill-quest credit -> turn-in loop matches §5.4 / §5.11
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
