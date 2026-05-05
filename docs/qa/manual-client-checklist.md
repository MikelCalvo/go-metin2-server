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
  - password: `hunter2`

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
- [ ] If the authored QA merchant catalog exposes an affordable test item, attempt one buy from the open window
- [ ] If the bought item is stackable and the character already carries the same `vnum`, confirm the count can increase on that existing stack instead of always creating a new slot
- [ ] If the QA setup allows it, fill the carried inventory, leave two compatible carried stacks nearly full, buy a stackable merchant entry whose count exactly matches their combined remaining room, and confirm both existing stacks fill without needing any fresh slot
- [ ] If the QA setup allows it, leave one compatible carried stack nearly full, buy a stackable merchant entry whose count overflows that stack, and confirm the existing stack fills first while the remainder lands in a fresh carried slot
- [ ] If the QA setup allows it, leave several compatible carried stacks nearly full plus one free carried slot, buy a stackable merchant entry whose count exceeds the combined remaining room in those existing stacks, and confirm the existing stacks fill first in carried-slot order while only the final remainder lands in the fresh slot
- [ ] If the QA setup allows it, force one insufficient-gold merchant buy and confirm the client receives the current placeholder info feedback (`Not enough gold.`)
- [ ] If the QA setup allows it, force one no-placement merchant buy and confirm the client receives the current placeholder info feedback (`Inventory full.`)
- [ ] Re-interact immediately once to confirm repeated spam is suppressed or remains stable within the current cooldown contract

Expected result:
- `info` and `talk` still return deterministic self-only text
- merchant interaction opens a stable bootstrap `GC::SHOP START` window
- a bootstrap `SHOP BUY` request can debit gold and grant the authored item without disconnecting the client
- when the authored item is stackable and a compatible carried stack already exists, the buy can refresh that same slot with the increased count
- when several compatible carried stacks together can absorb the full authored count, the buy can fill those existing stacks in carried-slot order without needing a fresh slot
- when several compatible carried stacks together cannot absorb the full authored count but one free carried slot exists, the buy can fill those existing stacks first and place only the final remainder into one fresh carried slot
- insufficient-gold and no-placement merchant failures preserve state and currently surface one self-only placeholder info chat instead of silently failing
- repeated interaction does not disconnect the client

Important note:
- this smoke step validates only the current bootstrap open / buy / close merchant slice
- final merchant failure choreography, sell flow, stock semantics, and richer NPC UI are still ahead

#### 5.4.2 Warp interaction

- [ ] Approach a visible authored QA warp NPC
- [ ] Interact once
- [ ] Confirm any authored informational text appears first if configured
- [ ] Confirm the client re-enters the world at the authored destination and remains connected

Expected result:
- the warp actor relocates the character through the current transfer/rebootstrap flow
- the client remains stable after the warp
- no merchant window, quest window, or inventory mutation appears

### 5.5 Bootstrap equip / unequip appearance refresh

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

#### 5.5.1 Template-backed equip point refresh

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
- already-visible peers still only receive the projected appearance refresh; no peer-visible point stream is frozen by this slice

### 5.6 Template-backed consumable item use

- [ ] Seed or confirm one carried consumable whose item template has a `use_effect` payload (current bootstrap QA seed: `27001`)
- [ ] Use `/use_item <slot>` on that carried consumable
- [ ] Confirm one self-only `PLAYER_POINT_CHANGE` arrives before the item-slot refresh
- [ ] Confirm the consumed slot decrements by exactly one stack item or clears entirely if it was the last item
- [ ] Confirm one self-only `CHAT_TYPE_INFO` placeholder effect arrives using the template-authored message
- [ ] Reconnect and confirm the consumed stack and updated point value persisted

Expected result:
- `/use_item <slot>` resolves through item-template metadata rather than a runtime-only hardcoded consumable switch
- the current seeded bootstrap template still yields `type = 1`, `amount = 50`, `value = updated Points[1]`, and `consume:27001:+50`
- the response burst stays self-only and ordered as `PLAYER_POINT_CHANGE` then `ITEM_SET`/`ITEM_DEL` then `CHAT_TYPE_INFO`
- the selected-character snapshot persists atomically through the current save/rollback boundary

### 5.7 Training dummy repeated-hit smoke

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
- [ ] Equip or unequip a supported `body`, `weapon`, or `head` item on client A
- [ ] Disconnect client A while client B stays in-world
- [ ] Reconnect client A through a fresh login/select/enter-game flow
- [ ] Confirm client B sees A re-enter with the latest visible body/weapon/head appearance in the normal peer-entry burst

Expected result:
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
- [ ] Confirm the stale socket may still receive only its self-local merchant success burst (`ITEM_SET`/`CHAT_TYPE_INFO` in the current slice)
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
- [ ] On the stale old socket, keep the merchant gate active and issue `SHOP BUY` (or `/shop_buy <slot>` in the local harness) so only the stale socket sees the local success burst
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
- [ ] Confirm the harness returns one `ITEM_SET` per changed carried slot in carried-slot order plus the current merchant success info delivery
- [ ] Confirm persisted `gold` and inventory match the same final state already frozen for the packet `SHOP BUY` path

Expected result:
- the local `/shop_buy` harness reuses the same deterministic carried-placement semantics as packet `SHOP BUY`
- compatible existing stacks fill first in slot order, then the remainder lands in the lowest free carried slot
- no harness-only placement drift appears in persisted or live runtime state

### 6.19 Training-dummy combat target selection (packet-harness optional)

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
- [ ] Immediately send one normal `ATTACK` against the still-visible dummy `VID`
- [ ] Re-select the dummy and confirm the next normal `ATTACK` works again with the usual self-only `GC TARGET(target_vid, hp_percent)` refresh
- [ ] Repeat with a harness-injected dead state (`current HP = 0`) and confirm both a fresh `TARGET` and a later `ATTACK` against the old selected dummy fail closed

Expected result:
- accepted combat ownership is bound to the selected dummy snapshot, not only the visible `VID`
- if that snapshot is replaced before reselection, stale `ATTACK` intent fails closed until the client reacquires target ownership with a new accepted `TARGET`
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
- [ ] Confirm that no respawn rebuild packets arrive before the first owned `2s` dead timer expires
- [ ] Once the timer expires, confirm each currently visible session receives the respawn rebuild burst in this order: `CHARACTER_DEL(vid)` -> `CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE`
- [ ] Confirm the rebuilt actor returns at the authored/bootstrap position and uses the same visible `VID`
- [ ] Without sending a fresh `TARGET`, issue one normal `ATTACK` from the previous attacker and, if applicable, from the previous watcher that still had the dead dummy selected before respawn
- [ ] Then send a fresh `TARGET` and confirm the next accepted `GC TARGET(target_vid, 100)` and first post-respawn `ATTACK` resume the normal self-only HP loop from full bootstrap HP

Expected result:
- the first respawn is purely server-driven and waits for the owned fixed `2s` dead interval
- respawn reuses normal visibility teardown + rebuild packet families instead of inventing a dedicated revive packet
- the rebuilt dummy is a fresh live combat snapshot even if the visible `VID` is reused
- stale pre-death target ownership does not survive the respawn boundary; post-respawn attacks fail closed until the session reselects target intent with a new accepted `TARGET`
- once reselected, the dummy immediately resumes the current bootstrap HP refresh path from `100` -> `90` on the next accepted normal hit

### 6.25 Content-loaded `spawn_groups` practice mob smoke

- [ ] Import or preload one authored `spawn_groups` entry that materializes a visible stationary practice mob using `combat_profile = training_dummy`
- [ ] Confirm the mob appears at the authored position with the authored display name and can be targeted in the same way as the earlier bootstrap dummy slices
- [ ] With two visible clients, let client one land the first accepted hit and verify client two's fresh `TARGET` attempt on the already-engaged mob fails closed until the existing death / respawn reset boundary
- [ ] On the owning client, confirm each accepted live hit now returns both the usual target-refresh and one immediate self-only HP `POINT_CHANGE` decrement while the mob remains alive
- [ ] After the first accepted live owner hit, stop sending `ATTACK` for at least the owned `1s` retaliation delay and confirm one queued self-only HP `POINT_CHANGE` follow-up beat arrives without a second client attack
- [ ] Wait one more owned `1s` delay without another accepted hit and confirm a second queued self-only HP `POINT_CHANGE` follow-up beat arrives while the mob stays alive and engaged
- [ ] If you can control timing precisely, land a later accepted owner hit while one autonomous delayed beat is already pending and confirm the current slice still yields only one queued delayed follow-up beat on the original timer rather than accelerating or resetting that cadence window
- [ ] If you can control timing precisely, also try a rapid second accepted hit before the first delayed beat fires and confirm the current slice still yields only one queued delayed follow-up beat for that first pending window
- [ ] Drive one full target -> hit -> zero-HP death -> timed respawn -> fresh reselect cycle against that content-loaded mob
- [ ] Re-export or otherwise inspect authored content and confirm the actor still round-trips as `spawn_groups`, not as an interaction-backed `static_actor`

Expected result:
- the first attackable content-loaded mob now comes from the authored `spawn_groups` seam instead of ad hoc runtime-only bootstrap registration
- its runtime combat loop still reuses the owned `training_dummy` profile semantics for HP, death, and timed respawn
- after the first accepted hit, the mob now owns one tiny aggro-lite gate: fresh third-party `TARGET` attempts fail closed until death / respawn resets the current engagement
- while alive, each accepted owner-side hit also applies one deterministic immediate self-only HP decrement back to that engaged session, and the first accepted live hit now starts a delayed self-only follow-up cadence that keeps firing one beat at a time after each owned `1s` server timer while the same engagement remains live
- while one delayed follow-up beat is already pending, extra accepted hits should not stack, accelerate, or reset the current cadence timer yet
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
- [ ] broader mob/skill combat beyond the current `training_dummy` target -> hit -> death -> timed-respawn loop
- [ ] quest acceptance, progression, or rewards
- [ ] player death / respawn, loot, or EXP reward systems
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
- [ ] no crash or forced disconnect occurs during the run
