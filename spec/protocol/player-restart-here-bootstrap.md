# Player Restart-Here Bootstrap

This document freezes the first connected-player recovery seam after the retaliation-owned zero-HP floor in `player-death-bootstrap.md`.

It sits on top of:
- `game-slash-command-bootstrap.md`
- `player-death-bootstrap.md`
- `transfer-rebootstrap-burst.md`

Those documents already freeze:
- the existing slash-command ingress while a session is already in `GAME`
- the retaliation-owned player-death floor at `0` HP, including self `DEAD(owner_vid)` + `TARGET(0, 0)` and the current post-floor denial gates
- the reusable selected-character self bootstrap burst (`CHARACTER_ADD` -> `CHAR_ADDITIONAL_INFO` -> `CHARACTER_UPDATE` -> `PLAYER_POINT_CHANGE`)

## Question

**What is the smallest honest same-socket recovery path the repo can own after a retaliation-driven player death without pretending that real revive menus, town return, or full corpse gameplay already exist?**

## Scope

This contract is intentionally narrow.

It applies only to:
- one selected live player session that is already in `GAME`
- the same retaliation-owned zero-HP floor already frozen in `player-death-bootstrap.md`
- one slash-command harness ingress: `/restart_here`
- one self-only in-place recovery burst on the same game socket
- one peer-visible delete-plus-rebootstrap refresh for the revived owner

It does **not** yet claim:
- a separate non-chat client-originated revive packet; the follow-up note in `player-restart-request-bootstrap.md` keeps that ingress question explicitly unproven/capture-gated for now
- restart-at-town or map-transfer recovery
- corpse timers, corpse interaction, or knockdown choreography
- persistence of retaliation-owned HP loss across `/restart_here`
- broader player-death persistence policy

## Acceptance rule

`/restart_here` is accepted only when all of these are true:
- the session still owns a live shared-world player entry
- the selected live player runtime is already at the retaliation-owned `0`-HP floor
- the session is still in `GAME`

Otherwise it fails closed.

The nearest explicitly deferred neighbors from this slice are now tracked separately:
- `/restart_town` is frozen in `player-restart-town-bootstrap.md`
- any later non-chat restart ingress remains an open capture-backed follow-up in `player-restart-request-bootstrap.md`

## Owner-side result

When accepted, `/restart_here` now:
1. keeps the session in `GAME`
2. rebuilds the selected player's live runtime from the persisted account snapshot
3. preserves the current in-world position for this first slice
4. returns the ordinary selected-character bootstrap burst on the same socket:
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`
   - `PLAYER_POINT_CHANGE`
5. keep the already-owned post-death rule that a fresh `TARGET` is required before later `ATTACK`

For this bootstrap slice, the self recovery rebuild is intentionally asymmetric with the engaged practice mob:
- the player rebuilds from persisted player state
- a still-live practice mob keeps its current runtime-owned HP and engagement reset rules

## Peer-visible result

When accepted, currently visible live peers now receive one queued refresh for that revived owner in this order:
1. `CHARACTER_DEL(owner_vid)`
2. `CHARACTER_ADD(owner_vid, ...)`
3. `CHAR_ADDITIONAL_INFO(owner_vid, ...)`
4. `CHARACTER_UPDATE(owner_vid, ...)`

This keeps the first alive-again surface honest:
- no new dedicated revive packet is invented yet
- the repo reuses already-owned visibility packet families
- peers learn about the alive-again transition through teardown + re-entry instead of a speculative corpse-revive opcode

Recipients that are themselves already at the current zero-HP floor remain out of audience as already frozen in `player-death-bootstrap.md`.

## Persistence rule

For this first slice, `/restart_here` does not invent new persistence semantics.

The narrow bootstrap rule is:
- retaliation-owned HP loss is still runtime-only
- `/restart_here` rebuilds from the persisted account snapshot for points/inventory/equipment
- therefore `/restart_here` implicitly clears the runtime-only retaliation loss instead of persisting it

## Post-restart combat rule

After accepted `/restart_here`:
- later owner-side `TARGET` may succeed again under the ordinary live combat rules
- later owner-side `ATTACK` still requires a fresh accepted `TARGET`
- the previously engaged practice mob, if still alive, remains at its current runtime-owned HP rather than resetting because of the owner's recovery

## Why this is the next honest slice

The repo already owns:
- retaliation-driven death at `0` HP
- fail-closed action denial at that floor
- same-socket `/phase_select` escape back to selection
- an existing self bootstrap burst and existing peer visibility rebuild packet families

So the smallest connected recovery seam is not a brand-new packet family.
It is reusing those already-owned bootstrap/rebuild surfaces for one same-socket in-place `/restart_here` recovery while explicitly deferring town return to the separate `/restart_town` slice in `player-restart-town-bootstrap.md`.
