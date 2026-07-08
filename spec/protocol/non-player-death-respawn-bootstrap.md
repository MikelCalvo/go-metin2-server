# Non-Player Death / Respawn Bootstrap

This document freezes the first owned death / respawn contract for bootstrap non-player combatants in `go-metin2-server`.

It sits on top of:
- `combat-training-dummy-bootstrap.md`
- `combat-normal-attack-bootstrap.md`
- `non-player-entity-bootstrap.md`
- `shared-world-peer-visibility.md`

Those documents already freeze:
- one visible `training_dummy` target class addressed by client-visible `VID`
- the first owned `ATTACK` request and deterministic runtime-owned dummy HP loop
- the current self-only `GC TARGET(target_vid, hp_percent)` success carrier plus `GC TARGET(0, 0)` clear companion
- the current visibility, ownership, and stale-session rules that decide whether a dummy can be targeted or attacked at all

What this document adds is the next narrower question:

**What is the smallest honest client/server contract for zero-HP death, dead-state target rejection, and respawn reset without mixing in the separate bootstrap reward, corpse gameplay, or full mob-system contracts?**

## Scope

This contract currently applies only to:
- one runtime-owned visible non-player combatant currently authored as `training_dummy`
- one accepted bootstrap normal-attack loop that can drive that dummy from live HP to `0`
- currently visible sessions that still share the dummy in their visible world
- self-only target clear for sessions whose active combat target currently binds that dummy
- one runtime-owned dead interval during which the dummy is no longer targetable or attackable
- one server-driven respawn reset that restores the dummy as a new live combat snapshot at bootstrap HP

This contract does **not** yet claim:
- the deterministic bootstrap EXP/gold/drop reward contract, which is documented separately in `non-player-reward-bootstrap.md`
- quest credit, party ownership rolls, randomized loot tables, level-up choreography, or broader reward distribution
- corpse interaction, corpse timers, revive menus, or corpse-specific UI
- hostile retaliation, aggro, patrol, pathing, or spawn-group AI beyond the later authored seam frozen in `content-spawn-groups-bootstrap.md`
- player death / respawn semantics
- persistence of dummy HP, dead state, or respawn timers across reconnect or process restart
- skill-based or animation-rich death choreography beyond the narrow packet families frozen here

## Current implementation status

The repository now implements this full bootstrap contract for the authored/runtime-marked `training_dummy`:
- zero-HP death is live
- visible sessions receive `GC DEAD(vid)` on the death edge
- sessions that still had that dummy selected receive the existing self-only `GC TARGET(0, 0)` clear companion in the same transition window
- post-death `TARGET` / `ATTACK` requests fail closed while the dummy remains dead
- the first server-driven dead timer is now live as one fixed `2s` bootstrap delay for built-in bootstrap profiles
- registered bootstrap combat profiles use their registered `respawn_delay` in the same pending server-frame path, and the respawn rebuild restores the actor to that profile's registered full HP
- if a live session is shown that same still-dead dummy again before the timer expires through any later add-style visibility presentation — fresh bootstrap, visibility re-entry, or a retained delete-plus-rebootstrap refresh — it first receives the ordinary `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` burst and then one `GC DEAD(vid)` replay so the actor does not silently look alive again
- once that timer expires, currently visible sessions receive the respawn rebuild burst: `CHARACTER_DEL` + `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE`
- if a still-connected visible player had already reached the current retaliation-owned `0`-HP floor, that zero-HP recipient is skipped from later dummy `GC DEAD(vid)` fanout and from that later respawn rebuild burst while other live viewers still receive the ordinary lifecycle frames
- the same runtime-owned dead interval is now surfaced through loopback runtime/operator snapshots too: static-actor entries returned by `/local/static-actors`, `/local/static-actors/{entity_id}`, `/local/visibility`, `/local/maps`, `/local/relocate-preview`, and `/local/transfer` carry `dead: true` until the dummy respawns
- the rebuilt dummy returns at bootstrap HP as a fresh live combat snapshot that requires fresh target acquisition before later attacks succeed again
- if the dying combatant was an engaged content-loaded practice mob with delayed owner-side retaliation armed, the death edge cancels that pending delayed beat before the respawn delay elapses, and the respawn rebuild does not restart retaliation until a fresh post-respawn target / accepted hit establishes a new engagement

## Why freeze death / respawn separately

The repository now already owns enough combat state to make this boundary explicit:
- target selection is real
- normal attacks are real
- runtime dummy HP is real
- stale/replaced/dead-target hardening is already part of the owned combat model

What is still missing is the transition when runtime HP reaches zero.

Without a written contract first, later slices would risk:
- inventing zero-HP behavior ad hoc in `internal/minimal`
- mixing death clear, corpse visibility, and respawn rebuild into one untestable blob
- accidentally reusing unrelated target-marker packet families that the legacy client reserves for minimap/atlas markers instead of combat targeting

So this document freezes the smallest next boundary before implementation.

## First owned death transition

The first owned bootstrap death transition is now frozen as:
- trigger: an accepted bootstrap normal attack commits the dummy's runtime HP from `1` to `0`
- visible death signal: server -> client `DEAD`
- header: `0x0217`
- payload: little-endian `uint32 vid`
- audience: every currently visible session that still has that dummy actor instantiated in its current visible world

The legacy-compatible meaning of this packet in the current bootstrap contract is intentionally narrow:
- the referenced visible actor is now dead
- that actor must no longer accept new bootstrap `TARGET` or `ATTACK` progress
- the repo does not yet claim loot, corpse interaction, or post-death scripted behavior

## First owned target-clear rule on death

Death also clears session-local combat targeting.

The first owned visible clear companion remains:
- server -> client `TARGET`
- header: `0x0A10`
- payload: `target_vid = 0`, `hp_percent = 0`
- meaning: no active combat target remains bound to that session

When a dummy dies:
- any session whose active combat target currently binds that dummy should receive one self-only `GC TARGET(0, 0)`
- that clear should happen as part of the same death transition window rather than waiting for a later reconnect, movement, or reselection path
- later attacks from that session must fail closed until a fresh post-respawn `TARGET` succeeds again

This keeps death aligned with the already-owned combat target surface.

## Why not `TARGET_DELETE` / `TARGET_UPDATE`

Legacy headers expose `GC TARGET_CREATE_NEW`, `GC TARGET_UPDATE`, and `GC TARGET_DELETE` families.
In the observed client code, those families currently drive atlas/minimap target markers rather than the combat target board.

So the first owned combat death / respawn contract explicitly does **not** reuse them.
Instead it keeps:
- combat target clear on `GC TARGET(0, 0)`
- combatant death on `GC DEAD(vid)`
- combatant respawn on normal actor visibility teardown + rebuild packets

## Dead-state runtime rule

A dummy whose runtime HP reached `0` is now in the owned dead state.

That dead state freezes these rules:
- `TARGET` must fail closed for that dummy while it remains dead
- `ATTACK` must fail closed for that dummy while it remains dead
- a stale pre-death selected target does not bypass the dead gate
- dead state remains runtime-owned only; it is not character/account persistence
- the dead dummy may remain visible to nearby sessions as a dead actor after `GC DEAD(vid)`
- any later add-style visibility presentation before respawn should replay that same dead state with one trailing `GC DEAD(vid)` after the ordinary actor add/info/update burst
- local runtime/operator snapshot surfaces should preserve that same dead interval explicitly with `dead: true` on the visible static-actor entries they return during preview, transfer, visibility, and static-actor inspection

What is intentionally **not** frozen here:
- corpse interaction affordances
- corpse decay timers as user-facing gameplay
- reward distribution beyond the separate bootstrap `non-player-reward-bootstrap.md` EXP/gold/drop descriptor seam
- pickup ownership expiry, public loot release, or kill-credit fanout

## Respawn trigger rule

The first owned respawn trigger is server-driven, not client-driven.

What is frozen now:
- a dummy respawns only because the server-owned dead timer/cooldown expires
- the timer starts when the authoritative zero-HP death transition commits
- the client does not request respawn through `TARGET`, `ATTACK`, `INTERACT`, movement, reconnect, or any corpse action
- the first built-in bootstrap profiles use one deterministic fixed-delay rule, not per-player custom timing
- the built-in bootstrap delay constant is `2s`
- registered bootstrap combat profiles use the `respawn_delay` accepted by `RegisterStaticActorCombatProfile(...)`; the pending respawn still uses the same runtime-owned shared-world state and `FlushServerFrames()` server-push seam between legacy-client reads

For future content-loaded attackable actors, the authored identity that tells the runtime what to recreate and where to recreate it is now documented separately in `content-spawn-groups-bootstrap.md`.

## First owned respawn client refresh path

The first owned respawn reset does **not** claim a dedicated revive packet.

Instead, respawn is frozen as a visibility rebuild using families the repo already owns:
1. server -> client `CHARACTER_DEL(vid)` for the dead visible dummy actor
2. server -> client `CHARACTER_ADD`
3. server -> client `CHAR_ADDITIONAL_INFO`
4. server -> client `CHARACTER_UPDATE`

The meaning of that sequence in this bootstrap contract is:
- remove the dead visible actor instance
- recreate the dummy as a fresh live visible actor at its bootstrap/authored position
- restore full bootstrap HP for the new live combat snapshot
- require fresh target acquisition; respawn does not auto-bind a combat target or auto-send a success `GC TARGET(target_vid, 100)` on its own

The implementation may reuse the same visible `VID` after the delete/re-add cycle, but even if it does, the respawned dummy is a **new live combat snapshot**.
Any pre-death target binding is gone and later attacks must reselect normally.

## Visibility and audience rule

The first death / respawn contract should respect the current visible-world rules already owned elsewhere:
- `GC DEAD(vid)` goes only to sessions that can currently see that dummy
- `GC TARGET(0, 0)` goes only to sessions whose active combat target is that dummy
- if a session is shown an already-dead dummy again before respawn through fresh bootstrap, later visibility re-entry, or a retained delete-plus-rebootstrap refresh, the ordinary actor add/info/update burst still goes only to sessions that should currently see that dummy and is immediately followed by one `GC DEAD(vid)` replay for that same audience
- respawn `CHARACTER_DEL` / add-burst packets go only to sessions that should currently see the dummy after the respawn reset
- the current bootstrap recipient-side player-death rule also applies: if a still-connected visible player already sits at the retaliation-owned `0`-HP floor, later dummy `GC DEAD(vid)` fanout and later respawn rebuild frames skip that zero-HP recipient silently until broader player-death recipient policy is owned

This document does not open any global broadcast rule for combat lifecycle.

## Explicit non-goals

This slice does **not** yet freeze:
- damage numbers or `DAMAGE_INFO`
- reward behavior beyond the narrow deterministic EXP/gold/drop descriptor contract in `non-player-reward-bootstrap.md`
- randomized loot, party distribution, quest credit, level-up choreography, public loot release, or broader reward fanout
- corpse interaction or corpse pickup
- player resurrection
- hostile AI wake-up on respawn
- spawn packs or multi-actor regeneration groups
- persistence across daemon restart

## Success definition

After this document lands, the repository should be able to say:
- zero-HP dummy behavior is no longer implied; the first death / respawn boundary is written down
- the first owned visible death signal for a bootstrap non-player combatant is `GC DEAD(vid)` with header `0x0217`
- death-triggered target clear stays on the already-owned self-only `GC TARGET(0, 0)` surface
- a dead dummy is explicitly non-targetable and non-attackable until respawn
- late or refreshed viewers no longer have to infer dead state: any later add-style visibility presentation before respawn now replays one trailing `GC DEAD(vid)` after the ordinary actor add/info/update burst
- respawn is server-driven and timer-based, not client-requested
- the first owned respawn reset reuses visible actor teardown + rebuild (`CHARACTER_DEL` + normal add/info/update burst) instead of inventing a dedicated revive packet
- the respawned dummy is a new live combat snapshot that requires fresh target acquisition even if the visible `VID` is reused
- later dummy death / respawn lifecycle fanout now also respects the current bootstrap player-death recipient gate, so an already-dead still-connected owner does not keep receiving those later non-player lifecycle frames
- deterministic EXP/gold/drop rewards are kept in the separate bootstrap reward contract, while randomized loot, party distribution, quest credit, corpse gameplay, and AI remain deliberately out of scope
