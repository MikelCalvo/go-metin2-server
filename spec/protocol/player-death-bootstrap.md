# Player Death Bootstrap

This document freezes the first owned zero-HP death signal for the selected player in `go-metin2-server`.

It sits on top of:
- `player-point-change-bootstrap.md`
- `combat-normal-attack-bootstrap.md`
- `content-spawn-groups-bootstrap.md`

Those documents already freeze:
- the selected player's current self-only `GC PLAYER_POINT_CHANGE` carrier
- the first hostile content-loaded practice-mob retaliation loop, including immediate hit-triggered point loss and the delayed server-origin cadence
- fail-closed owner-side `TARGET` / `ATTACK` rejection once that retaliation floor has already reached `0` HP
- fail-closed owner-side `MOVE` / `SYNC_POSITION` rejection at that same retaliation floor before relocation or transfer-trigger rebootstrap can run
- fail-closed owner-side static-actor `INTERACT` rejection at that same retaliation floor before talk/info, merchant preview, or warp-side effects can run
- fail-closed owner-side merchant-buy attempts at that same retaliation floor before inventory / gold mutation can run through packet `SHOP BUY` or the local `/shop_buy` harness path
- fail-closed owner-side slash item-use attempts at that same retaliation floor before inventory consumption or point restoration can run through the local `/use_item` harness path
- fail-closed owner-side slash carried-inventory move attempts at that same retaliation floor before runtime or persisted slot mutation can run through the local `/inventory_move` harness path
- fail-closed owner-side slash equipment mutation attempts at that same retaliation floor before carried/equipped item movement, appearance refresh, or template-backed point mutation can run through the local `/equip_item` and `/unequip_item` harness paths

What this document adds is the next narrower question:

**What is the smallest honest zero-HP owner death signal the repo can own now without pretending that full player death, peer-visible corpse state, or respawn gameplay already exist?**

## Scope

This contract currently applies only to:
- one selected live player character already engaged with a content-loaded `spawn_groups` practice mob using `combat_profile = training_dummy`
- one immediate self-only retaliation tick appended to an accepted owner hit
- one delayed self-only server-origin retaliation beat flushed through the pending server-frame path
- the edge where either of those retaliation point-loss beats drives the engaged owner's live bootstrap HP from a positive value to `0`
- one self-only `GC DEAD(owner_vid)` signal paired with the existing self-only combat-target clear companion

This contract does **not** yet claim:
- peer-visible player death fanout
- corpse state, knockdown animations, or corpse interaction
- player respawn, revive menus, town return, or map transfer on death
- broader self-only `CHAT_TYPE_INFO` or full input gating after death beyond the now-owned combat `TARGET` / `ATTACK`, relocation `MOVE` / `SYNC_POSITION`, static-actor `INTERACT`, merchant-buy rejection, slash item-use rejection, slash inventory-move rejection, slash equipment-mutation rejection, and peer-facing `CHAT` / `WHISPER` rejection at `0` HP
- PvP death semantics or non-combat causes of player death

## Current implementation status

The repository now implements this narrow bootstrap contract:
- if an immediate retaliation tick reaches the engaged owner's live HP floor at `0`, the accepted attack frames now append one self-only `GC DEAD(owner_vid)` before the existing self-only `GC TARGET(0, 0)` clear
- if a delayed server-origin retaliation beat reaches that same `0`-HP floor, the queued pending server frames now append the same self-only `GC DEAD(owner_vid)` before the same self-only clear-target companion
- the current slice stays self-only: no watcher/peer `GC DEAD(owner_vid)` fanout is claimed yet
- once this floor is reached, the existing delayed retaliation cadence stops and later owner-side combat `TARGET` / `ATTACK` attempts still fail closed as already frozen elsewhere
- once this floor is reached, later owner-side `MOVE` / `SYNC_POSITION` attempts also fail closed with no self ack, no shared-world relocation update, and no transfer-trigger rebootstrap burst
- once this floor is reached, later owner-side static-actor `INTERACT` attempts also fail closed with no self chat/info delivery, no merchant preview open, and no warp transfer / rebootstrap burst
- once this floor is reached, later owner-side merchant-buy attempts also fail closed with no item-set success burst and no inventory / gold mutation, even if a merchant preview had already been opened earlier in that session
- once this floor is reached, later owner-side slash item-use attempts also fail closed with no point-change / item-set success burst and no runtime or persisted inventory / point mutation
- once this floor is reached, later owner-side slash `/inventory_move` attempts also fail closed with no item-set success burst and no runtime or persisted carried-slot mutation
- once this floor is reached, later owner-side slash `/equip_item` and `/unequip_item` attempts also fail closed with no item-delete / item-set / point-change / character-update success burst and no runtime or persisted inventory / equipment / point mutation
- once this floor is reached, later owner-side peer-facing `CHAT` requests with `type = TALKING`, `PARTY`, `GUILD`, or `SHOUT` plus later owner-side `WHISPER` requests also fail closed before sender echo, queued peer delivery, or exact-name lookup can run
- the earlier slash-command seams stay separate here: `/quit`, `/logout`, and `/phase_select` keep their current independent behavior, while the already-owned `/shop_buy`, `/use_item`, `/inventory_move`, `/equip_item`, and `/unequip_item` denial paths keep their existing post-floor rules

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

## First owned owner-side zero-HP death signal

The first owned player-death signal is now frozen as:
- packet family: server -> client `DEAD`
- header: `0x0217`
- payload: little-endian `uint32 vid`
- audience: self-only, limited to the engaged owner session whose retaliation beat reached `0` HP

The current bootstrap meaning is intentionally narrow:
- the selected player's live bootstrap HP has just reached `0`
- this is the owner's own death edge inside the currently owned practice-mob retaliation loop
- the repo still does **not** yet claim peer-visible player death, corpse gameplay, or respawn choreography from this packet alone

## Ordered frame rule at the floor

When an immediate or delayed retaliation beat reaches the owner's live HP floor at `0`, the current bootstrap frame order is now:
1. self-only `GC PLAYER_POINT_CHANGE` carrying the final `value = 0`
2. self-only `GC DEAD(owner_vid)`
3. self-only `GC TARGET(0, 0)`

That ordering applies in both current bootstrap owners:
- immediate retaliation piggybacked on an accepted live owner `ATTACK`
- delayed retaliation flushed later through the pending server-frame path

The target-clear companion remains important even after `GC DEAD(owner_vid)` because the current slice still wants the stale engaged practice-mob target removed deterministically on the same edge.

## Why self-only only

The smallest honest slice stops at self-only death visibility.

The project does **not** yet own enough player-death runtime to claim all of these together:
- peer-visible player death
- player corpse lifetime
- player respawn or revive
- broader chat / interaction / merchant / input policy beyond the now-owned combat and relocation rejection seams at `0` HP

So this first owner-side death signal stays narrow:
- the engaged owner learns about the zero-HP edge immediately
- the stale target is cleared immediately
- peers still do not receive a new player-death fanout yet
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
- the repo still does **not** yet claim broader chat, whisper, or revive policy at `0` HP

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
- the repo still does **not** yet claim broader chat, whisper, or revive policy at `0` HP

## First owned post-floor slash item-use denial

The current bootstrap player-death contract now also owns one narrow post-floor item-use rule for that same selected live owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later slash `/use_item <slot>` attempts fail closed
- the denial happens before inventory consumption, point restoration, or persistence mutation can run
- the denial stays intentionally quiet in this slice: no synthetic failure info chat, no fallback revive packet, and no broader consumable UI contract change are claimed yet

This keeps the next post-floor expansion small and honest too:
- after merchant-buy denial, slash item use is the next dangerous already-open gameplay context because it can still mutate runtime and persisted inventory or restore points even if the owner has already died in the current bootstrap retaliation loop
- the repo still does **not** yet claim broader chat, whisper, non-slash item packet, or revive policy at `0` HP

## First owned post-floor peer-facing chat / whisper denial

The current bootstrap player-death contract now also owns one narrow peer-facing communication rule for that same selected live owner session:
- once immediate or delayed practice-mob retaliation has already driven the owner's live bootstrap HP to `0`, later owner-side `CHAT` requests with `type = CHAT_TYPE_TALKING`, `CHAT_TYPE_PARTY`, `CHAT_TYPE_GUILD`, or `CHAT_TYPE_SHOUT` fail closed
- once that same floor has been reached, later owner-side `WHISPER` requests fail closed too
- those denials happen before sender echo, queued peer fanout, or exact-name target lookup can run
- the denial stays intentionally quiet in this slice: no self `GC_CHAT` echo, no queued peer chat delivery, no queued target whisper delivery, and no synthetic `WHISPER_TYPE_NOT_EXIST` fallback
- existing slash-command seams stay separate: `/quit`, `/logout`, `/phase_select`, and the already-owned `/shop_buy` / `/use_item` paths keep their current independent behavior instead of being widened by this follow-up implicitly
- broader client-origin `CHAT_TYPE_INFO`, revive, mute/block, or general full action-lock policy still remain out of scope until a later slice writes those contracts down explicitly

Why this is the current owned boundary:
- after combat, relocation, interaction, merchant-buy, and slash item-use denial were already owned at the same `0`-HP floor, peer-facing chat and whisper were the next dangerous already-open player-origin surfaces because they could still fan out to other live sessions from a dead owner
- keeping slash commands separate preserves the smallest honest rule instead of widening the whole chat seam into a blanket post-floor command lock

## Explicit non-goals

This slice does **not** yet freeze:
- peer-visible `GC DEAD(owner_vid)` fanout
- a player respawn timer or revive request packet
- self-bootstrap or transfer choreography after death
- broader self-only `CHAT_TYPE_INFO` or full action-lock semantics at `0` HP beyond the now-owned combat, relocation, static-actor interaction, merchant-buy, slash item-use, and peer-facing chat / whisper rejection seams above
- death penalties, EXP loss, inventory drops, or corpse recovery

## Success definition

After this document lands, the repository should be able to say:
- owner-side retaliation-driven `0` HP is no longer only an implicit point floor; the repo now owns one visible self-only zero-HP death signal for that edge
- the current bootstrap player-death packet is `GC DEAD(owner_vid)` with header `0x0217`
- that self-only owner death signal is emitted on both immediate and delayed retaliation beats when they drive the engaged owner to `0` HP
- the current ordered owner-side floor transition is `GC PLAYER_POINT_CHANGE(value=0)` -> `GC DEAD(owner_vid)` -> `GC TARGET(0, 0)`
- once that same floor is reached, later owner-side `MOVE` / `SYNC_POSITION` attempts also fail closed before self ack, shared-world relocation mutation, or transfer-trigger rebootstrap work can run
- once that same floor is reached, later owner-side static-actor `INTERACT` attempts also fail closed before talk/info delivery, merchant preview open, or warp transfer / rebootstrap work can run
- once that same floor is reached, later owner-side merchant-buy attempts also fail closed before runtime/persisted inventory or gold mutation can run through packet `SHOP BUY` or the local `/shop_buy` harness path
- once that same floor is reached, later owner-side slash `/use_item` attempts also fail closed before runtime/persisted inventory consumption or point restoration can run
- once that same floor is reached, later owner-side peer-facing `CHAT` requests with types `TALKING`, `PARTY`, `GUILD`, and `SHOUT` plus later owner-side `WHISPER` requests also fail closed before sender echo, peer delivery, or exact-name lookup can run
- peer-visible player death, corpse state, respawn, broader self-only `CHAT_TYPE_INFO`, and general post-death gameplay remain deliberately out of scope
