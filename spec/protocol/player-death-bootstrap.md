# Player Death Bootstrap

This document freezes the first owned zero-HP death signal for the selected player in `go-metin2-server`.

It sits on top of:
- `player-point-change-bootstrap.md`
- `combat-normal-attack-bootstrap.md`
- `content-spawn-groups-bootstrap.md`

Those documents already freeze:
- the selected player's current self-only `GC PLAYER_POINT_CHANGE` carrier
- the first hostile content-loaded practice-mob retaliation loop, including immediate hit-triggered point loss and the delayed server-origin cadence
- that same retaliation point-loss is currently live-runtime only for the engaged selected session, so it does not yet persist across fresh `/phase_select` re-entry or reconnect
- fail-closed owner-side `TARGET` / `ATTACK` rejection once that retaliation floor has already reached `0` HP
- fail-closed owner-side `MOVE` / `SYNC_POSITION` rejection at that same retaliation floor before relocation or transfer-trigger rebootstrap can run
- fail-closed owner-side static-actor `INTERACT` rejection at that same retaliation floor before talk/info, merchant preview, or warp-side effects can run
- fail-closed owner-side merchant-buy attempts at that same retaliation floor before inventory / gold mutation can run through packet `SHOP BUY` or the local `/shop_buy` harness path
- fail-closed owner-side merchant-sell attempts at that same retaliation floor before inventory / gold / quickslot mutation can run through packet `SHOP SELL` / `SHOP SELL2`
- fail-closed owner-side client/slash item-use attempts at that same retaliation floor before inventory consumption, carried-item stacking, or point restoration can run through the local `/use_item` harness path, carried-slot `ITEM_USE`, or `ITEM_USE_TO_ITEM`
- fail-closed owner-side item/gold drop attempts at that same retaliation floor before ground-item registration, currency debit, or inventory persistence can run through `ITEM_DROP` / `ITEM_DROP2`
- fail-closed owner-side carried-inventory move attempts at that same retaliation floor before runtime or persisted slot mutation can run through packet `ITEM_MOVE` or the local `/inventory_move` harness path
- fail-closed owner-side slash equipment mutation attempts at that same retaliation floor before carried/equipped item movement, appearance refresh, or template-backed point mutation can run through the local `/equip_item` and `/unequip_item` harness paths
- fail-closed owner-side quickslot add/delete/swap attempts at that same retaliation floor before quickslot mutation can run through packet `QUICKSLOT_ADD` / `QUICKSLOT_DEL` / `QUICKSLOT_SWAP`

What this document adds is the next narrower question:

**What is the smallest honest zero-HP owner death signal the repo can own now without pretending that full player death, peer-visible corpse state, or respawn gameplay already exist?**

## Scope

This contract currently applies only to:
- one selected live player character already engaged with a content-loaded `spawn_groups` practice mob using `combat_profile = training_dummy`
- one immediate self-only retaliation tick appended to an accepted owner hit
- one delayed self-only server-origin retaliation beat flushed through the pending server-frame path
- the edge where either of those retaliation point-loss beats drives the engaged owner's live bootstrap HP from a positive value to `0`
- one self-only `GC DEAD(owner_vid)` signal paired with the existing self-only combat-target clear companion
- the first visible-peer `GC DEAD(owner_vid)` fanout to sessions that can currently see that owner through the shared-world visibility rules
- one silent recipient-side whisper-delivery gate for later peer-originated exact-name whispers aimed at that same still-connected zero-HP owner
- recipient-side queued chat-recipient gates for later peer-originated `CHAT_TYPE_TALKING`, `CHAT_TYPE_PARTY`, `CHAT_TYPE_GUILD`, and `CHAT_TYPE_SHOUT` fanout aimed at that same still-connected zero-HP owner through the current bootstrap chat-routing paths
- one recipient-side server-notice gate for later server-originated `CHAT_TYPE_NOTICE` broadcasts aimed at that same still-connected zero-HP owner through the current bootstrap global notice path
- one recipient-side peer-visibility gate for later fresh visible peer joins whose queued `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst would otherwise be delivered to that same still-connected zero-HP owner through the current shared-world join path
- one recipient-side same-visible-set peer-update gate for later queued peer `CHARACTER_UPDATE` appearance refreshes from live equip / unequip mutations aimed at that same still-connected zero-HP owner through the current shared-world stable visibility-transition path
- one recipient-side same-visible-set peer-movement gate for later queued peer `MOVE_ACK` / `SYNC_POSITION_ACK` replication from live peers that remain visible to that same still-connected zero-HP owner through the current shared-world stable visibility-transition path
- one live-recipient dead-owner replay rule for later shared-world join / visibility-entry paths that newly present an already-dead owner to other live sessions
- one recipient-side peer-visibility teardown gate for later visible peer `CHARACTER_DEL` frames from peer leave, stale-ownership reclaim cleanup, relocate-away transfer, or AOI `MOVE` / `SYNC_POSITION` out-of-range teardown aimed at that same still-connected zero-HP owner through the current shared-world leave / reclaim / relocate / AOI visibility-transition paths
- one recipient-side non-player combat-lifecycle gate for later visible practice-mob `GC DEAD(...)` fanout plus its later timed respawn rebuild burst aimed at that same still-connected zero-HP owner
- one recipient-side non-player visibility gate for later live static-actor register / update / remove delivery aimed at that same still-connected zero-HP owner through the current shared-world static-actor visibility paths
- one recipient-side generic visible-session gate for later shared-world fanouts routed through the reusable `EnqueueToVisibleSessions(...)` helper and aimed at that same still-connected zero-HP owner
- one recipient-side destination ground-item and ground-gold visibility gate for later transfer/rebootstrap paths whose queued `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` burst would otherwise be delivered to that same still-connected zero-HP owner after relocating into a map/AOI with existing ground entries
- one shared-world dead-subject gate for direct static-actor `INTERACT`, combat `TARGET`, and selected combat `ATTACK` attempt seams so alternate runtime callers cannot bypass the zero-HP owner floor only through session-local guards
- one loopback runtime/operator player-snapshot dead-state rule across `/local/players`, `/local/visibility`, `/local/interaction-visibility`, `/local/maps`, `/local/relocate-preview`, and `/local/transfer` while that same zero-HP owner remains connected

This contract does **not** yet claim:
- corpse state, knockdown animations, or corpse interaction
- broader player respawn, revive menus, or compatibility-grade death return rules beyond the currently owned same-socket `/restart_here` and `/restart_town` bootstrap recovery seams
- broader full input gating after death beyond the now-owned combat `TARGET` / `ATTACK`, relocation `MOVE` / `SYNC_POSITION`, static-actor `INTERACT`, merchant-buy / merchant-sell rejection, client/slash item-use / use-to-item rejection, item/gold drop rejection, slash inventory-move rejection, slash equipment-mutation rejection, quickslot mutation rejection, peer-facing `CHAT` / `WHISPER` rejection, self-only `CHAT_TYPE_INFO` rejection, and recipient-side server-origin `CHAT_TYPE_NOTICE` skip at `0` HP
- PvP death semantics or non-combat causes of player death

## Current implementation status

The repository now implements this narrow bootstrap contract:
- if an immediate retaliation tick reaches the engaged owner's live HP floor at `0`, the accepted attack frames now append one self-only `GC DEAD(owner_vid)` before the existing self-only `GC TARGET(0, 0)` clear
- if a delayed server-origin retaliation beat reaches that same `0`-HP floor, the queued pending server frames now append the same self-only `GC DEAD(owner_vid)` before the same self-only clear-target companion
- when either of those retaliation beats reaches that same `0`-HP floor, currently visible peer sessions now also receive one queued `GC DEAD(owner_vid)` using the existing shared-world visibility rules
- that same queued peer-visible death fanout now skips recipients whose own live bootstrap HP is already at the current `0`-HP floor, so a still-connected dead owner does not keep receiving later peer-death `GC DEAD(...)` frames from other sessions
- those immediate and delayed retaliation point-loss beats stay runtime-only for the selected live session: they do **not** write the persisted account snapshot, so fresh `/phase_select` re-entry, reconnect, and the owned same-socket `/restart_here` and `/restart_town` recovery seams all still rebuild from the pre-retaliation point value until broader player-death persistence or respawn semantics are owned
- once this floor is reached, the existing delayed retaliation cadence stops and later owner-side combat `TARGET` / `ATTACK` attempts still fail closed as already frozen elsewhere
- once this floor is reached, later owner-side `MOVE` / `SYNC_POSITION` attempts also fail closed with no self ack, no shared-world relocation update, and no transfer-trigger rebootstrap burst
- once this floor is reached, later owner-side static-actor `INTERACT` attempts also fail closed with no self chat/info delivery, no merchant preview open, and no warp transfer / rebootstrap burst
- once this floor is reached, later owner-side merchant-buy attempts also fail closed with no item-set success burst and no inventory / gold mutation, even if a merchant preview had already been opened earlier in that session
- once this floor is reached, later owner-side packet merchant-sell attempts also fail closed with no item-delete / item-update / quickslot-delete / point-change success burst and no runtime or persisted inventory / gold / quickslot mutation, including both whole-stack `SHOP SELL` and partial-stack `SHOP SELL2`
- once this floor is reached, later owner-side client/slash item-use attempts also fail closed with no point-change / item-set success burst and no runtime or persisted inventory / point mutation
- once this floor is reached, later owner-side `ITEM_USE_TO_ITEM` attempts also fail closed with no item-set / item-del / quickslot success burst and no runtime or persisted carried-item stacking mutation
- once this floor is reached, later owner-side `ITEM_DROP` / `ITEM_DROP2` attempts also fail closed with no point-change, ground-add, ownership, quickslot, or item-set/delete success burst and no runtime or persisted inventory / gold mutation
- once this floor is reached, later owner-side carried-inventory move attempts also fail closed with no item-set success burst and no runtime or persisted carried-slot mutation; this applies to both packet `ITEM_MOVE` and the local `/inventory_move` harness path
- once this floor is reached, later owner-side slash `/equip_item` and `/unequip_item` attempts also fail closed with no item-delete / item-set / point-change / character-update success burst and no runtime or persisted inventory / equipment / point mutation
- once this floor is reached, later owner-side packet quickslot add/delete/swap attempts also fail closed with no quickslot refresh frames and no runtime or persisted quickslot or inventory mutation
- once this floor is reached, later owner-side peer-facing `CHAT` requests with `type = TALKING`, `PARTY`, `GUILD`, or `SHOUT` plus later owner-side `WHISPER` requests also fail closed before sender echo, queued peer delivery, or exact-name target lookup can run
- once this floor is reached, later peer-originated `WHISPER` requests targeting that same exact owner name also fail closed before queued target delivery or a synthetic `WHISPER_TYPE_NOT_EXIST` fallback can run
- once this floor is reached, later peer-originated local `CHAT` requests with `type = TALKING` from still-visible sessions continue to return the live sender's ordinary self echo, but queued peer delivery skips that zero-HP owner recipient entirely
- once this floor is reached, later peer-originated `CHAT` requests with `type = PARTY`, `GUILD`, or `SHOUT` also continue to return the live sender's ordinary self echo, but queued peer delivery skips that same zero-HP owner recipient under the current bootstrap party/global guild/empire shout routing rules
- once this floor is reached, later server-originated `CHAT_TYPE_NOTICE` broadcasts still queue normally for other connected live sessions, but queued notice delivery skips that same still-connected zero-HP owner recipient entirely under the current bootstrap global notice path
- once this floor is reached, later fresh visible peer joins still queue their normal `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst for other live recipients, but that same queued peer-entry burst skips the zero-HP owner recipient entirely under the current shared-world join path
- once this floor is reached, later live equip / unequip mutations still queue their ordinary queued peer `CHARACTER_UPDATE` appearance refresh for other live recipients that stay visible across that mutation, but that same queued same-visible-set peer-update path skips the zero-HP owner recipient entirely under the current shared-world stable visibility-transition helper
- once this floor is reached, later same-visible-set peer `MOVE_ACK` / `SYNC_POSITION_ACK` replication still queues normally for other live viewers while the mover or syncer keeps the ordinary self acknowledgement, but that same queued stable peer-movement path skips the zero-HP owner recipient entirely under the current shared-world stable visibility-transition helper
- once this floor is reached, if that already-dead owner is later presented to another live session through a shared-world join or visibility-entry helper, the live recipient now receives the ordinary peer-entry burst plus one trailing `GC DEAD(owner_vid)` replay instead of silently treating the already-dead owner as live
- once this floor is reached, later fresh `ENTERGAME` bootstrap that newly brings another live session into visibility of that same still-connected dead owner also appends one queued `GC DEAD(owner_vid)` for the newcomer right after the ordinary peer-entry burst for that owner, so the newcomer does not silently treat an already-dead visible player as live
- once this floor is reached, later `MOVE`, `SYNC_POSITION`, or transfer-driven visibility-entry rebuilds that newly pair another live session with that same still-connected dead owner also append one queued `GC DEAD(owner_vid)` for the newly paired live peer right after the ordinary peer-entry burst for that owner, so those later visibility rebuilds do not silently re-present an already-dead visible player as live either
- once this floor is reached, later peer leave, stale-ownership reclaim cleanup, AOI `MOVE` / `SYNC_POSITION` out-of-range teardown, or relocate-away transfer teardown that would ordinarily queue a visible peer `CHARACTER_DEL` to that same still-connected dead owner also skips that zero-HP owner recipient entirely while the leaving, reclaimed, moving, syncing, or transferred live peer still receives its ordinary live-session cleanup or replacement re-entry behavior
- once this floor is reached, later visibility-gated queued peer `GC DEAD(other_vid)` fanout from another visible player's retaliation-owned death edge also skips that same still-connected zero-HP owner recipient entirely
- once this floor is reached, later visible practice-mob `GC DEAD(mob_vid)` fanout plus that mob's later timed respawn rebuild burst (`CHARACTER_DEL` + `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE`) also skip that same still-connected zero-HP owner recipient entirely while other live viewers still receive the ordinary non-player lifecycle frames
- once this floor is reached, later live static-actor register / update / remove visibility delivery also keeps queuing the ordinary non-player add / refresh / delete frames for other live viewers, but those same queued static-actor visibility frames skip that still-connected zero-HP owner recipient entirely
- once this floor is reached, later shared-world fanouts that still route through the reusable `EnqueueToVisibleSessions(...)` helper also keep queuing their ordinary frames for other live visible sessions, but that same generic visible-session delivery path now skips the still-connected zero-HP owner recipient entirely
- once this floor is reached, later transfer/rebootstrap paths that move that same zero-HP owner into visibility of existing ground items or ground gold also skip the destination `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` burst for that owner, while the ground entry remains live and available to living visible sessions
- once this floor is reached, direct shared-world static-actor `INTERACT`, combat `TARGET`, and selected combat `ATTACK` attempt seams also fail closed with `subject_dead` before target resolution can run, so alternate runtime callers cannot bypass the zero-HP owner floor by skipping the current session-layer guards
- once this floor is reached, the same loopback runtime/operator player snapshots now surface `dead: true` for that still-connected zero-HP owner whether it appears as the main `character` / `target`, as a visible peer, or inside map-occupancy / connected-player arrays
- once this floor is reached, later owner-side self-only `CHAT` requests with `type = INFO` also fail closed before local `GC_CHAT` delivery can run
- the earlier slash-command seams stay separate here: `/quit`, `/logout`, and `/phase_select` keep their current independent behavior, with regression coverage now freezing that exact `/quit` and `/logout` still work after the immediate-retaliation death edge; the owned same-socket `/restart_here` and `/restart_town` recovery seams are frozen separately in `player-restart-here-bootstrap.md` and `player-restart-town-bootstrap.md`, and the already-owned `/shop_buy`, `/use_item`, `ITEM_USE`, `/inventory_move`, `/equip_item`, and `/unequip_item` denial paths keep their existing post-floor rules

## First owned same-socket `/phase_select` recovery boundary

The repo now also owns one narrow recovery seam for that same retaliation-driven `0`-HP floor:
- after the owner has already received the current self-only `GC DEAD(owner_vid)` plus self-only `GC TARGET(0, 0)` clear, `/phase_select` may still transition that same socket back to character select
- if the player then re-selects the same persisted character and sends a fresh `ENTERGAME` on that same socket, the self bootstrap rebuilds from the persisted account snapshot rather than carrying the runtime-only retaliation loss forward
- that rebuild is intentionally asymmetric with the engaged practice mob: if the mob stayed alive, its HP remains runtime-owned at the last live value instead of resetting just because the owner used `/phase_select`
- the owner still has to send a fresh `TARGET` after re-entry before `ATTACK` can resume; the earlier engaged-target/session intent does not survive the phase transition

This keeps the first recovery boundary honest:
- the selected live player points remain session/runtime-owned during the retaliation loop until a later slice owns broader player-death persistence or revive policy
- the practice-mob HP and engagement loop remain shared-world/runtime-owned until the mob's own death/respawn reset seam runs
- `/phase_select` is therefore only a bootstrap re-entry boundary, not a full corpse / revive / respawn system

## First owned same-socket `/restart_here` recovery boundary

The repo now also owns one narrower connected recovery seam on that same retaliation-driven `0`-HP floor:
- after the owner has already received the current self-only `GC DEAD(owner_vid)` plus self-only `GC TARGET(0, 0)` clear, same-socket `/restart_here` may rebuild that same selected session in place while keeping the socket in `GAME`
- the owner receives the existing selected-character bootstrap burst on the same socket (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE` -> `PLAYER_POINT_CHANGE`) rebuilt from the persisted account snapshot while keeping the current in-world position for this first slice
- currently visible live peers receive one queued alive-again refresh for that owner as `CHARACTER_DEL` -> `CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE`
- the owner still has to send a fresh `TARGET` after `/restart_here` before `ATTACK` can resume; the earlier selected practice-mob target does not survive that recovery seam
- the already-live practice mob stays asymmetric here too: if it survived, it keeps its current runtime-owned HP and current engagement-reset rules instead of resetting because the owner used `/restart_here`
- `/restart_here` stays narrow and honest in this slice: it fails closed while the owner is still alive, it keeps reusing existing bootstrap / visibility packet families instead of inventing a dedicated revive opcode, and the separate town-return follow-up is now frozen in `player-restart-town-bootstrap.md`

## Why freeze this separately

The repository already owned enough of the owner-side retaliation loop to make `0` HP observable:
- retaliation point loss is real
- it can now reach `0`
- the current slice already clears the stale engaged target and stops later combat progress at that floor

But without a written death-signal contract, the runtime would still leave the player's `0`-HP edge as only a point mutation plus target clear.

This document freezes one smaller visible step first:
- keep using the already-owned `GC PLAYER_POINT_CHANGE` carrier for the numeric HP transition
- add one self-only `GC DEAD(owner_vid)` to mark the zero-HP edge
- keep broader player death / respawn semantics out of scope until later slices can own them honestly

## First owned zero-HP death signal family

The first owned player-death signal is now frozen as:
- packet family: server -> client `DEAD`
- header: `0x0217`
- payload: little-endian `uint32 vid`
- audience:
  - self-only for the engaged owner session whose retaliation beat reached `0` HP
  - visibility-gated queued peers for the same owner `vid` when that same retaliation beat reaches `0` HP

The current bootstrap meaning is intentionally narrow:
- the selected player's live bootstrap HP has just reached `0`
- this is the owner's own death edge inside the currently owned practice-mob retaliation loop
- the repo still does **not** yet claim corpse gameplay, respawn choreography, or broader player-death lifecycle ownership from this packet family alone

## Ordered frame rule at the floor

When an immediate or delayed retaliation beat reaches the owner's live HP floor at `0`, the current bootstrap frame order is now:
1. self-only `GC PLAYER_POINT_CHANGE` carrying the final `value = 0`
2. self-only `GC DEAD(owner_vid)`
3. self-only `GC TARGET(0, 0)`

That ordering applies in both current bootstrap owners:
- immediate retaliation piggybacked on an accepted live owner `ATTACK`
- delayed retaliation flushed later through the pending server-frame path

The target-clear companion remains important even after `GC DEAD(owner_vid)` because the current slice still wants the stale engaged practice-mob target removed deterministically on the same edge.

## Why this stays narrow

The current slice widens visibility only as far as the same retaliation-owned death edge needs.

The project does **not** yet own enough player-death runtime to claim all of these together:
- player corpse lifetime
- player respawn or revive
- broader chat / interaction / merchant / input policy beyond the now-owned combat and relocation rejection seams at `0` HP

So this first death signal family stays narrow:
- the engaged owner learns about the zero-HP edge immediately
- the stale target is cleared immediately
- currently visible live peers receive only one queued `GC DEAD(owner_vid)` and no broader teardown/rebuild choreography
- already-dead connected recipients are skipped from that queued peer-visible death fanout instead of learning about later peer deaths through extra `GC DEAD(...)` frames
- later slices may widen audience and lifecycle only after those contracts are written down explicitly

## Relationship to the existing retaliation floor gate

This document does not replace the already-owned retaliation-floor rules.

Those rules still stand:
- retaliation point loss clamps at `0`
- the delayed cadence stops when that floor is reached
- later same-owner combat `TARGET` / `ATTACK` attempts fail closed once the owner is already at `0`
- later same-owner `MOVE` / `SYNC_POSITION` attempts also fail closed once the owner is already at `0`

What this document adds is only the visible death-edge packet companion for that already-owned `0`-HP transition.

## First owned post-floor relocation denial

The current bootstrap player-death contract now also owns one narrow relocation rule for that same selected live owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later owner-side `MOVE` requests fail closed
- once that same floor has been reached, later owner-side `SYNC_POSITION` requests also fail closed
- those denials happen before the runtime applies live-position mutation, shared-world relocation fanout, or transfer-trigger rebootstrap work
- the denial stays intentionally quiet in this slice: no self `MOVE_ACK`, no self `SYNC_POSITION_ACK`, no compensating chat/info packet, and no peer-facing relocation packet companion

This keeps the first post-floor expansion small and honest:
- the repo already owns `TARGET` / `ATTACK` denial at the same floor
- `MOVE` and `SYNC_POSITION` are the next most dangerous owner-authoritative packets because they can still mutate live world position or trigger transfer rebootstrap even after the owner has already died in the current bootstrap retaliation loop
- broader chat and full action-lock policy still remain out of scope until later slices freeze them explicitly

## First owned post-floor static-actor interaction denial

The current bootstrap player-death contract now also owns one narrow authored-interaction rule for that same selected live owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later owner-side static-actor `INTERACT` requests fail closed
- those denials happen before the runtime resolves visible static-actor metadata, opens a merchant preview, appends a self chat/info delivery, or applies warp transfer / rebootstrap work
- the denial stays intentionally quiet in this slice: no self chat/info fallback, no merchant-close companion, no transfer failure packet, and no peer-facing interaction packet family

This keeps the next post-floor expansion small and honest too:
- `INTERACT` is the next dangerous client-origin packet after relocation because it can still mutate runtime position through warp or open follow-up merchant context even after the owner has already died in the current bootstrap retaliation loop
- the repo still does **not** yet claim revive policy at `0` HP or a broader general post-death action-lock contract

## First owned post-floor merchant-buy denial

The current bootstrap player-death contract now also owns one narrow post-preview merchant rule for that same selected live owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later merchant-buy attempts fail closed even if the session had already opened a merchant preview earlier
- this denial applies to both currently owned merchant-buy ingress paths:
  - packet `SHOP BUY`
  - the local `/shop_buy <slot>` harness routed through the chat command seam
- the denial happens before inventory placement, gold debit, or persistence mutation can run
- the denial stays intentionally quiet in this slice: no synthetic merchant-close packet, no failure info chat, and no broader merchant UI contract change are claimed yet

This keeps the next post-floor expansion small and honest:
- after static-actor `INTERACT` denial, merchant buy is the next dangerous already-open gameplay context because it can still mutate runtime and persisted inventory / currency even if the owner has already died in the current bootstrap retaliation loop
- the repo still does **not** yet claim revive policy at `0` HP or a broader general post-death action-lock contract

## First owned merchant-window close at the retaliation floor

The current bootstrap player-death contract now also owns one narrow merchant teardown rule for that same selected live owner session:
- if that session already opened a merchant preview before immediate or delayed practice-mob retaliation reached `0` HP, the same owned floor transition now appends one self-only `GC::SHOP END`
- that close companion arrives only after the already-owned owner-side floor ordering completes first: `GC PLAYER_POINT_CHANGE(value=0)` -> `GC DEAD(owner_vid)` -> `GC TARGET(0, 0)`
- the same floor transition clears the active merchant context immediately, so a later client `SHOP BUY` or `SHOP END` request on that same dead owner session now fails closed until broader revive / reopen semantics are owned separately
- the slice is covered on the delayed server-origin retaliation path with a content-loaded practice mob and an authored merchant, not only the immediate hit-piggyback path
- the slice stays self-only: no peer-facing merchant packet companion is added, and no broader merchant reset choreography beyond that one close frame is claimed

Why this is the current owned boundary:
- once post-floor merchant buys already failed closed, an already-open merchant window became the next smallest stale UI/runtime context still surviving the same retaliation-owned death edge
- reusing the existing `GC::SHOP END` close companion keeps the teardown honest without inventing a second death-specific merchant packet family

## First owned post-floor client/slash item-use, item-drop, and item-move denial

The current bootstrap player-death contract now also owns narrow post-floor item-use, item-drop, and item-move rules for that same selected live owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later slash `/use_item <slot>` attempts fail closed
- once that same floor has been reached, later carried-slot `ITEM_USE` and `ITEM_USE_TO_ITEM` attempts also fail closed
- the item-use denial happens before inventory consumption, carried-item stacking, point restoration, or persistence mutation can run
- once that same floor has been reached, later `ITEM_DROP` / `ITEM_DROP2` attempts also fail closed for both inventory items and gold
- the item-drop denial happens before inventory mutation, gold debit, ground-item registration, ownership delivery, quickslot sync, or persistence mutation can run
- once that same floor has been reached, later packet `ITEM_MOVE` attempts also fail closed for carried-inventory moves before runtime slot mutation, persistence, item-set frames, or quickslot sync can run
- the denial stays intentionally quiet in this slice: no synthetic failure info chat, no fallback revive packet, and no broader consumable/drop/move UI contract change are claimed yet

This keeps the next post-floor expansion small and honest too:
- after merchant-buy denial, client/slash item use, item drop, and packet item move were the next dangerous already-open gameplay contexts because they could still mutate runtime and persisted inventory/gold, restore points, stack or move carried items, or register visible ground items even if the owner had already died in the current bootstrap retaliation loop
- the repo still does **not** yet claim revive policy or a broader general post-death action-lock contract at `0` HP

## First owned post-floor peer-facing chat / whisper denial

The current bootstrap player-death contract now also owns one narrow peer-facing communication rule for that same selected live owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later owner-side `CHAT` requests with `type = CHAT_TYPE_TALKING`, `CHAT_TYPE_PARTY`, `CHAT_TYPE_GUILD`, or `CHAT_TYPE_SHOUT` fail closed
- once that same floor has been reached, later owner-side `WHISPER` requests fail closed too
- those denials happen before sender echo, queued peer fanout, or exact-name target lookup can run
- the denial stays intentionally quiet in this slice: no self `GC_CHAT` echo, no queued peer chat delivery, no queued target whisper delivery, and no synthetic `WHISPER_TYPE_NOT_EXIST` fallback
- existing slash-command seams stay separate: `/quit`, `/logout`, `/phase_select`, and the already-owned `/shop_buy` / `/use_item` / `ITEM_USE` paths keep their current independent behavior instead of being widened by this follow-up implicitly
- broader revive, mute/block, or general full action-lock policy still remain out of scope until a later slice writes those contracts down explicitly

Why this is the current owned boundary:
- after combat, relocation, interaction, merchant-buy, and client/slash item-use denial were already owned at the same `0`-HP floor, peer-facing chat and whisper were the next dangerous already-open player-origin surfaces because they could still fan out to other live sessions from a dead owner
- keeping slash commands separate preserves the smallest honest rule instead of widening the whole chat seam into a blanket post-floor command lock

## First owned post-floor self-only info-chat denial

The current bootstrap player-death contract now also owns one narrow self-only communication rule for that same selected live owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later owner-side `CHAT` requests with `type = CHAT_TYPE_INFO` fail closed
- that denial happens before self `GC_CHAT` info delivery can run
- the denial stays intentionally quiet in this slice: no self info echo, no fallback death/revive companion, and no peer-facing packet side effect are claimed
- existing slash-command seams still stay separate: `/quit`, `/logout`, and `/phase_select` keep their current independent behavior instead of being widened by this follow-up implicitly

Why this is the current owned boundary:
- after peer-facing chat / whisper denial, self-only `CHAT_TYPE_INFO` is the next remaining already-open chat ingress that could still create visible post-floor client output from a dead owner
- keeping slash commands separate still preserves the smallest honest rule instead of widening the entire chat seam into a blanket post-floor command lock

## First owned peer-visible player death fanout

The current bootstrap player-death contract now also owns one narrow visible-peer death rule for that same selected live owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, currently visible peer sessions receive one queued `GC DEAD(owner_vid)`
- that fanout is visibility-gated through the existing shared-world rules: only sessions that can currently see the owner receive it
- recipients whose own live bootstrap HP is already at that same `0`-HP floor are skipped from that queued fanout even if they still remain connected and visible in shared world
- the owner-side transition stays unchanged and still uses the existing self-only `GC DEAD(owner_vid)` plus self-only `GC TARGET(0, 0)` clear ordering
- this slice does **not** yet add a peer-facing target clear companion because the current owned combat-target model still belongs to bootstrap non-player targets, not player-vs-player selection
- this slice also does **not** yet delete, respawn, or otherwise rebuild the dead player actor for peers

Why this is the current owned boundary:
- once the owner-side death signal and the first post-floor input denials were already owned, peer-visible `GC DEAD(owner_vid)` became the next smallest missing visible consequence for the same retaliation-owned death edge
- reusing the current shared-world visibility fanout keeps the slice honest without pretending corpse state, respawn, or broader player-death choreography already exists

## First owned post-floor peer-entry visibility recipient skips

The current bootstrap player-death contract now also owns one narrow visibility-recipient rule for that same still-connected zero-HP owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later fresh visible peer joins no longer queue their ordinary `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst to that zero-HP owner recipient
- other connected live recipients still receive the ordinary peer-entry burst for that newcomer
- when stale shared-world ownership cleanup later forces that same visible peer to delete and re-enter through the current join path, live watchers still receive the stale `CHARACTER_DEL`, then the ordinary peer-entry burst again, and—if the reclaimed peer is already at that same `0`-HP floor—one trailing `GC DEAD(peer_vid)` right after the re-entry burst instead of silently reviving the peer
- once that same owner is already sitting at that zero-HP floor, later movement- or `SYNC_POSITION`-driven peer visibility re-entry bursts also skip that zero-HP owner recipient when another player crosses into visibility
- the moving or syncing live peer still receives the ordinary origin-side peer-entry burst for the already-dead owner under the current bootstrap visibility model
- once that same owner is already sitting at that zero-HP floor, later transfer-driven peer visibility re-entry bursts also skip that zero-HP owner recipient when another live player is relocated into visibility
- whichever live peer is newly paired with that already-dead owner through the current transfer path — either because a transferred live peer enters the dead owner's visible world or because the already-dead owner itself is later relocated into another live peer's visible world — still receives the ordinary origin-side peer-entry burst for that owner followed immediately by one queued `GC DEAD(owner_vid)`
- if that already-dead owner itself is later relocated through the current loopback/operator transfer path, that same dead owner session now also skips the queued destination peer-entry burst for any newly paired live peer and the queued destination static-actor add / info / update burst for any newly visible destination actor; only any source-side visibility cleanup already needed for the old visible world stays queued locally
- this slice stays recipient-only plus dead-state replay for newly paired live peers: it does not yet claim that fresh joiners, movers, syncers, or transferred peers stop seeing the dead owner, and it does not yet widen beyond the current shared-world join / move / `SYNC_POSITION` / transfer seams

Why this is the current owned boundary:
- after whisper/chat/notice/peer-`DEAD` recipient skips were already owned, fresh peer-entry bursts on later joins became the next smallest remaining queued recipient surface that could still deliver ordinary world-visibility output to a still-connected zero-HP owner
- once that join seam was owned, the next honest widening stayed on the same packet family and reused the existing AOI visibility rebuild path for `MOVE` / `SYNC_POSITION` and the already-owned transfer visibility rebuild instead of jumping straight to broader corpse-state teardown or full post-death observer rules
- keeping the rule scoped to the existing join plus movement/sync/transfer visibility-entry seams preserves a tiny honest contract without claiming full corpse-state visibility teardown or a broader post-death observer model yet

## First owned post-floor peer-visibility teardown recipient skips

The current bootstrap player-death contract now also owns one narrow peer-visibility teardown rule for that same still-connected zero-HP owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later visible peer leave on the current shared-world leave path no longer queues the ordinary `CHARACTER_DEL` teardown to that zero-HP owner recipient
- once that same owner is already sitting at that zero-HP floor, later relocate-away transfer teardown on the current shared-world transfer path also skips that zero-HP owner recipient for the ordinary peer `CHARACTER_DEL`
- once that same owner is already sitting at that zero-HP floor, later AOI `MOVE` / `SYNC_POSITION` out-of-range teardown on the shared-world visibility-transition path also skips that zero-HP owner recipient for the ordinary peer `CHARACTER_DEL`
- the leaving, moving, syncing, or transferred live peer still receives its ordinary origin-side visibility cleanup; this slice only narrows delivery to the already-dead recipient

Why this is the current owned boundary:
- after chat/notice recipient skips plus peer-entry recipient skips were already owned, later queued peer `CHARACTER_DEL` teardown on disconnect/transfer/AOI-exit became the next smallest remaining player-visibility surface that could still deliver ordinary world teardown to a still-connected zero-HP owner
- reusing the existing leave / transfer / visibility-transition teardown loops keeps the widening honest without pretending a broader corpse-state observer model is already frozen

## First owned post-floor static-actor visibility recipient skips

The current bootstrap player-death contract now also owns one narrow non-player visibility-recipient rule for that same still-connected zero-HP owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later live static-actor registration no longer queues the ordinary `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst to that zero-HP owner recipient
- other connected live recipients still receive the ordinary static-actor registration burst when they share visible world with that actor
- once that same owner is already sitting at that zero-HP floor, later in-place static-actor updates also skip that zero-HP owner recipient for both same-visible-set refresh delivery (`CHARACTER_DEL` + `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE`) and add/remove visibility deltas across map or AOI changes
- once that same owner is already sitting at that zero-HP floor, later static-actor removals also skip that zero-HP owner recipient for the ordinary `CHARACTER_DEL`
- if that same owner is later relocated through the current loopback/operator transfer path, that same dead owner session also skips the queued destination static-actor `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst for newly visible destination actors while still keeping any source-side static-actor cleanup already needed for the old visible world
- this slice stays recipient-only: it does not yet claim broader dead-owner observer rules for every later non-player packet family, and it does not widen past the current shared-world static-actor register / update / remove visibility seams plus that same operator transfer destination rebuild seam

Why this is the current owned boundary:
- after peer chat/notice/peer-entry recipient skips plus later practice-mob death/respawn lifecycle recipient skips were already owned, live static-actor visibility delivery became the next smallest remaining queued non-player surface that could still deliver ordinary world-presence output to a still-connected zero-HP owner
- reusing the existing static-actor register / update / remove fanout paths keeps the widening honest without pretending corpse teardown, revive policy, or a broader dead-observer visibility model already exist

## Explicit non-goals

This slice does **not** yet freeze:
- a player respawn timer or revive request packet
- broader self-bootstrap or transfer choreography after death beyond the currently owned persisted `/phase_select` re-entry / reconnect rebuild semantics plus dead-owner destination peer/static-actor burst suppression on the current loopback/operator transfer path
- broader self-only chat/command surfaces or full action-lock semantics at `0` HP beyond the now-owned combat, relocation, static-actor interaction, merchant-buy, client/slash item-use, packet item-move, slash inventory/equipment mutation, peer-facing chat / whisper, and self-only `CHAT_TYPE_INFO` rejection seams above
- broader recipient-side communication or world-visibility policy beyond the now-owned exact-name whisper denial, queued `CHAT_TYPE_TALKING` / `PARTY` / `GUILD` / `SHOUT` recipient skips, queued peer-entry visibility recipient skips on join plus movement/sync/transfer visibility rebuilds, queued peer-teardown `CHARACTER_DEL` recipient skips on leave plus relocate-away transfer plus AOI move/sync out-of-range teardown, queued static-actor visibility recipient skips on register/update/remove, queued practice-mob death/respawn lifecycle recipient skips, queued destination ground-item visibility skips on transfer/rebootstrap, and server-origin `CHAT_TYPE_NOTICE` recipient skip for connected zero-HP owners
- death penalties, EXP loss, inventory drops, or corpse recovery

## Success definition

After this document lands, the repository should be able to say:
- owner-side retaliation-driven `0` HP is no longer only an implicit point floor; the repo now owns one visible zero-HP death signal family for that edge
- the current bootstrap player-death packet is `GC DEAD(owner_vid)` with header `0x0217`
- that owner death signal is emitted on both immediate and delayed retaliation beats when they drive the engaged owner to `0` HP
- those immediate and delayed retaliation point-loss beats stay runtime-only for the selected live session today, so fresh `/phase_select` re-entry or reconnect still rebuild from the pre-retaliation persisted point value until broader player-death persistence / respawn semantics are owned
- the current ordered owner-side floor transition is `GC PLAYER_POINT_CHANGE(value=0)` -> `GC DEAD(owner_vid)` -> `GC TARGET(0, 0)`
- once that same floor is reached, later owner-side `MOVE` / `SYNC_POSITION` attempts also fail closed before self ack, shared-world relocation mutation, or transfer-trigger rebootstrap work can run
- once that same floor is reached, later owner-side static-actor `INTERACT` attempts also fail closed before talk/info delivery, merchant preview open, or warp transfer / rebootstrap work can run
- once that same floor is reached, later owner-side merchant-buy attempts also fail closed before runtime/persisted inventory or gold mutation can run through packet `SHOP BUY` or the local `/shop_buy` harness path
- if that same floor is reached while the owner already had a merchant preview open, the same floor transition now also appends one self-only `GC::SHOP END` after self `GC DEAD(owner_vid)` plus self `GC TARGET(0, 0)` and clears the active merchant context so later client `SHOP END` fails closed too
- once that same floor is reached, later owner-side slash `/use_item` and carried-slot `ITEM_USE` attempts also fail closed before runtime/persisted inventory consumption or point restoration can run
- once that same floor is reached, later owner-side packet `ITEM_MOVE` attempts also fail closed before runtime/persisted carried-inventory slot mutation or item-set success frames can run
- once that same floor is reached, later owner-side peer-facing `CHAT` requests with types `TALKING`, `PARTY`, `GUILD`, and `SHOUT` plus later owner-side `WHISPER` requests also fail closed before sender echo, peer delivery, or exact-name lookup can run
- once that same floor is reached, later peer-originated `WHISPER` requests aimed at that same exact connected owner name also fail closed before queued target delivery or a synthetic `WHISPER_TYPE_NOT_EXIST` fallback can run
- once retaliation has already driven the owning character to `0` HP, later peer-originated `CHAT` requests with types `TALKING`, `PARTY`, `GUILD`, and `SHOUT` continue to return the live sender's ordinary self echo, but queued peer delivery skips that same zero-HP owner recipient under the current bootstrap routing rules
- once retaliation has already driven the owning character to `0` HP, later fresh visible peer joins also keep queuing their ordinary `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst for other live recipients, but that same queued peer-entry burst skips the still-connected zero-HP owner recipient under the current shared-world join path
- once retaliation has already driven the owning character to `0` HP, later movement- or `SYNC_POSITION`-driven peer visibility re-entry bursts also keep queuing their ordinary `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst for the live mover/syncing origin, but that same queued peer-entry burst skips the still-connected zero-HP owner recipient under the current shared-world AOI rebuild path
- once retaliation has already driven the owning character to `0` HP, later transfer-driven peer visibility re-entry bursts also keep queuing their ordinary `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst for the live transferred origin, but that same queued peer-entry burst skips the still-connected zero-HP owner recipient under the current shared-world transfer rebuild path
- once retaliation has already driven the owning character to `0` HP, later peer leave, relocate-away transfer teardown, or AOI `MOVE` / `SYNC_POSITION` out-of-range teardown also keeps the ordinary origin-side visibility cleanup for the live leaving/moving/syncing/transferred peer, but the same queued peer `CHARACTER_DEL` teardown skips the still-connected zero-HP owner recipient entirely
- once retaliation has already driven the owning character to `0` HP, later visible practice-mob `GC DEAD(mob_vid)` fanout and that mob's later timed respawn rebuild burst also keep queuing their ordinary non-player death / respawn lifecycle frames for other live viewers, but those same queued frames skip the still-connected zero-HP owner recipient entirely
- once retaliation has already driven the owning character to `0` HP, later live static-actor register / update / remove visibility delivery also keeps queuing the ordinary non-player add / refresh / delete frames for other live viewers, but those same queued static-actor visibility frames skip the still-connected zero-HP owner recipient entirely
- once retaliation has already driven the owning character to `0` HP, later transfer/rebootstrap paths that move that same zero-HP owner into visibility of existing ground items also skip destination `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` frames for that dead recipient while preserving the live ground item for living visible sessions
- once retaliation has already driven the owning character to `0` HP, later owner-side self-only `CHAT` requests with type `INFO` also fail closed before self info delivery can run
- if that same floor is reached while the dead owner still held the aggro-lite gate for a live content-loaded practice mob, that same floor transition now also releases the mob's engagement so another visible live session may reacquire it with a fresh `TARGET` without waiting for owner disconnect or mob death / respawn
- once either retaliation beat reaches that same floor, currently visible live peer sessions also receive one queued `GC DEAD(owner_vid)` while already-dead connected recipients are skipped and corpse state, respawn, and broader player-death choreography remain deliberately out of scope
- peer-visible player death beyond that one visible `GC DEAD(owner_vid)` fanout, corpse state, respawn, and broader general post-death gameplay remain deliberately out of scope
