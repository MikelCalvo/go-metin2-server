# Player Restart-Town Bootstrap

This document freezes the next connected-player recovery seam after the retaliation-owned zero-HP floor in `player-death-bootstrap.md` and the first same-socket in-place recovery in `player-restart-here-bootstrap.md`.

It sits on top of:
- `game-slash-command-bootstrap.md`
- `player-death-bootstrap.md`
- `player-restart-here-bootstrap.md`
- `transfer-rebootstrap-burst.md`

Those documents already freeze:
- the existing slash-command ingress while a session is already in `GAME`
- the retaliation-owned player-death floor at `0` HP, including self `DEAD(owner_vid)` + `TARGET(0, 0)` and the current post-floor denial gates
- the first same-socket `/restart_here` recovery seam that rebuilds in place from persisted player state
- the reusable self transfer rebootstrap burst plus trailing visibility deltas on the same game socket

## Question

**What is the smallest honest same-socket town-return recovery path the repo can own after a retaliation-driven player death without pretending that real revive menus, corpse timers, or broader respawn systems already exist?**

## Scope

This contract is intentionally narrow.

It applies only to:
- one selected live player session that is already in `GAME`
- the same retaliation-owned zero-HP floor already frozen in `player-death-bootstrap.md`
- one slash-command harness ingress: `/restart_town`
- one owned town-return target chosen from the existing legacy empire create-position table
- one self rebootstrap burst on the same game socket
- one transfer-style visibility rebuild at the old and new positions

It does **not** yet claim:
- a separate non-chat client-originated revive packet; the follow-up note in `player-restart-request-bootstrap.md` keeps that ingress question explicitly unproven/capture-gated for now
- arbitrary respawn points, corpse timers, or revive menus
- broader player-death persistence policy
- persistence of retaliation-owned HP loss across the restart
- map- or dungeon-specific death return rules beyond the existing legacy empire create-position mapping

## Acceptance rule

`/restart_town` is accepted only when all of these are true:
- the session still owns a live shared-world player entry
- the selected live player runtime is already at the retaliation-owned `0`-HP floor
- the session is still in `GAME`

Otherwise it fails closed.

The nearest explicitly deferred neighbor stays out of scope:
- any separate non-chat client respawn / revive ingress remains unsupported for now and is tracked as an open capture-backed follow-up in `player-restart-request-bootstrap.md`

## Town target rule

For this first slice, `/restart_town` reuses the already-owned legacy empire create-position table as the town-return target:
- empire `1` -> `map_index 1`, `x 459800`, `y 953900`
- empire `2` -> `map_index 21`, `x 52070`, `y 166600`
- empire `3` -> `map_index 41`, `x 957300`, `y 255200`

The current bootstrap fallback rule is also explicit now:
- if the persisted selected character snapshot has `empire = 0`, `/restart_town` falls back to the current session-ticket empire
- if that fallback empire is also `0` or otherwise outside the owned table, the current legacy create-position helper falls back to the empire-`1` coordinates above

Runtime regression coverage now exercises all three owned empire table rows plus the `0`/unknown fallback path through the full same-socket `/restart_town` recovery seam, not only through the standalone helper.

This keeps the slice honest:
- the repo already owns those positions as deterministic create positions
- the repo does **not** yet claim that these are final compatibility-grade death respawn points for every map or content case

## Owner-side result

When accepted, `/restart_town`:
1. keeps the session in `GAME`
2. rebuilds the selected player's live runtime from the persisted account snapshot
3. updates the selected player's persisted and live position to the owned town-return target for that empire
4. returns the ordinary selected-character bootstrap burst on the same socket:
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`
   - `PLAYER_POINT_CHANGE`
5. appends the already-owned transfer-origin visibility deltas after that self burst:
   - source visible peers leave through `CHARACTER_DEL`
   - destination visible peers enter through the ordinary add/info/update burst
   - source and destination static-actor visibility still reuse the ordinary transfer/static-actor delta families already frozen elsewhere
6. keeps the already-owned post-death rule that a fresh `TARGET` is required before later `ATTACK`

For this bootstrap slice, the recovery stays intentionally asymmetric with the engaged practice mob:
- the player rebuilds from persisted player state and moves to the owned town-return target
- a still-live practice mob keeps its current runtime-owned HP and its current engagement-reset rules instead of resetting because the owner used `/restart_town`
- source-map live sessions that still see that practice mob after the restarting owner leaves can reselect it and observe the current runtime-owned HP percentage instead of a full-HP reset

## Peer-visible result

When accepted, currently visible live peers continue to learn about the moved/revived owner only through already-owned visibility packet families:
- old visible peers receive `CHARACTER_DEL(owner_vid)`
- newly visible destination peers receive `CHARACTER_ADD(owner_vid, ...)`, `CHAR_ADDITIONAL_INFO(owner_vid, ...)`, and `CHARACTER_UPDATE(owner_vid, ...)`
- already-dead connected recipients remain out of audience as already frozen in `player-death-bootstrap.md`
- later peers that enter visibility after the accepted town restart treat that owner as live again; they do **not** receive a replayed `DEAD(owner_vid)` from the earlier pre-restart death window
- later peers that enter the old/source visible world after the accepted town restart do not see either a stale owner add or a replayed `DEAD(owner_vid)`; they only receive ordinary source-map actors/peers that remain there

No dedicated revive packet is invented yet.

## Persistence rule

For this first slice, `/restart_town` still keeps retaliation-owned HP loss runtime-only.

The narrow bootstrap rule is:
- retaliation-owned HP loss is still runtime-only
- `/restart_town` rebuilds points/inventory/equipment from the persisted account snapshot
- `/restart_town` does persist the owned town-return position before runtime commit, reusing the existing bootstrap transfer ordering
- therefore `/restart_town` clears the runtime-only retaliation loss while still saving the owned town-return coordinates

## Post-restart combat rule

After accepted `/restart_town`:
- later owner-side `TARGET` may succeed again under the ordinary live combat rules
- later owner-side `ATTACK` still requires a fresh accepted `TARGET`
- the previously engaged practice mob, if still alive, remains at its current runtime-owned HP rather than resetting because of the owner's recovery

## Why this is the next honest slice

The repo already owns:
- retaliation-driven death at `0` HP
- same-socket `/restart_here` recovery in place
- the existing legacy empire create-position mapping
- the existing self transfer rebootstrap burst and visibility rebuild families

So the next smallest connected recovery seam is not a new packet family.
It is reusing those already-owned transfer/bootstrap surfaces for one same-socket `/restart_town` recovery while explicitly deferring broader revive menus and full death/respawn gameplay.
