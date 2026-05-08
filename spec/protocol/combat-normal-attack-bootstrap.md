# Combat Normal Attack Bootstrap

This document freezes the first owned attack-intent and clear-target contract for `go-metin2-server`.

It sits on top of:
- `combat-training-dummy-bootstrap.md`
- `non-player-entity-bootstrap.md`
- `shared-world-peer-visibility.md`
- `runtime-reconnect-cleanup.md`

Those documents already freeze:
- one visible `training_dummy` target class addressed by client-visible `VID`
- the first self-only `GC TARGET` acknowledgement for accepted target selection
- the current visibility/range/runtime ownership rules that decide whether a dummy can stay targetable at all
- the existing reconnect/reclaim cleanup style that later combat slices must reuse instead of inventing separate ownership semantics

What this document adds is the next narrower question:

**What is the smallest honest attack-intent step the project can own next without pretending that full damage, death, aggro, or mob AI already exist?**

The first owned death / respawn follow-up now lives in:
- `non-player-death-respawn-bootstrap.md`

The first owned owner-side zero-HP retaliation follow-up now lives in:
- `player-death-bootstrap.md`

## Scope

This contract currently applies only to:
- one connected `GAME` session with a selected live character
- one active selected combat target already accepted through the existing `TARGET` selection path
- one currently visible in-range non-player actor still marked as `training_dummy`
- one immediate attack-intent request against that already selected target
- one tiny target-refresh surface that can still describe `current target`, `updated hp percent`, or `no active target`

This contract does **not** yet claim:
- the full gameplay meaning of every non-zero `attack_type` value beyond the first narrow bootstrap ownership boundary
- combo chains, animation timing, attack speed, or projectile choreography
- richer attack-result packets, hit effects, floating damage numbers, or skill systems
- combat against player targets
- aggro, retaliation, patrol, or movement AI
- loot, EXP, death rewards, corpse state, or quest hooks
- final persistence rules for non-player combat state

## Why freeze attack intent before full combat

The repository already has a real target-selection slice:
- `TARGET` can now select one visible in-range `training_dummy`
- that selection already reuses shared-world visibility, map, and ownership rules
- the runtime already knows how to reject invisible, out-of-range, stale, or non-targetable candidates fail-closed

What is still missing is the next concrete step after target selection.

Without a written attack-intent contract, later slices would risk:
- inventing ad-hoc attack ingress straight inside `internal/minimal`
- coupling HP updates to a guessed packet layout too early
- introducing a separate clear-target packet family before proving the smaller reuse path is insufficient

So this document freezes the smallest next ownership boundary first.

## First owned attack-intent family

The first owned combat request is now frozen exactly as:
- name: `ATTACK`
- direction: client -> server
- phase: `GAME`
- header: `0x0401`
- payload length: `7`
- status: documented and codec-owned in `internal/proto/combat`

The exact payload layout is:
1. `uint8 attack_type`
2. `uint32 target_vid` (little-endian)
3. `uint8 crc_proc_piece`
4. `uint8 crc_file_piece`

What is frozen now about those fields:
- the first live dummy attack path accepts only `attack_type = 0` (`normal attack`) in this slice
- `target_vid` is the wire-visible target identity the client places in the request
- `crc_proc_piece` and `crc_file_piece` are currently owned as exact trailing raw bytes in the codec, but their higher-level validation role remains intentionally narrow in the clean-room runtime for now

This exact codec ownership matters because the next flow slices no longer need to guess the attack header or open-code a one-off byte layout inside `internal/minimal`.

## Active-target prerequisite

The first owned attack-intent path is intentionally target-relative rather than free-form.

Even though the exact wire request carries a `target_vid`, the bootstrap runtime contract still treats that field as **subordinate to the currently selected combat target** rather than as permission to attack an arbitrary visible actor.

An `ATTACK` request is only eligible when all of the following are true:
- the session is already in `GAME`
- the session still owns a selected live character
- that live character currently holds one active combat target from the existing `TARGET` selection contract
- the request uses `attack_type = 0` for the first normal-attack bootstrap path
- the request `target_vid` exactly matches the session's currently selected combat target
- that selected target still resolves to a visible same-map `training_dummy`
- that selected target still passes the current bootstrap combat band

This keeps the first attack slice aligned with the already-owned `TARGET` path instead of creating a second competing target-identity model.

## First clear-target representation

The first owned clear-target companion is now frozen as a **reuse of the existing server -> client `TARGET` family**, not as a separate dedicated clear packet.

The working contract is:
- server -> client `TARGET` with `target_vid = 0`
- server -> client `TARGET` with `hp_percent = 0`
- combined meaning: **no active combat target remains bound to the session**

Why this reuse is the current preferred contract:
- the repository already owns `GC TARGET` as the smallest self-only target-state surface
- the same packet family can already describe `current target + hp percent`
- reusing it for `no target` avoids inventing a second clear-only family before tests prove a richer path is needed

So the first owned target-state surface is now intentionally tiny but expressive enough for three states:
1. `TARGET(target_vid > 0, hp_percent = 100)` — selected live dummy with fresh full bootstrap HP on first owned selection
2. `TARGET(target_vid > 0, hp_percent = updated)` — same selected dummy after accepted bootstrap attack-driven HP changes
3. `TARGET(0, 0)` — selected target cleared or no longer valid

## Relationship to later HP / death work

This document freezes the first deterministic bootstrap HP mutation and the preferred visible HP-refresh carrier.
The first owned death / respawn wire contract is now documented separately in `non-player-death-respawn-bootstrap.md`, but the live implementation still has not landed that zero-HP transition yet.

The current owned bootstrap combat state is intentionally tiny:
- visible `training_dummy` combat state is runtime-owned and starts at `10` HP
- each accepted bootstrap normal attack decrements the dummy by `1` HP
- the visible refresh stays on server `TARGET(target_vid, hp_percent)` using the current runtime HP converted to percent in `10`-point steps (`100`, `90`, `80`, ...)
- until the death / respawn implementation slice lands, the currently shipped loop may still stop at the pre-death floor instead of claiming live zero-HP behavior too early

What this still freezes about the **visible state carrier** for later slices:
- accepted later attack-driven HP refreshes should continue preferring server `TARGET` with the same selected `target_vid` plus the updated `hp_percent`
- target loss, invalidation, death cleanup, reconnect cleanup, transfer cleanup, or reclaim cleanup should prefer the zero-target `TARGET(0, 0)` companion before introducing a new clear-target family
- when subject movement or sync updates make the selected dummy leave current visibility or the bootstrap combat band, the runtime should proactively clear the active target with one self-only `TARGET(0, 0)` companion
- when the later zero-HP death transition lands, it should keep death-triggered target clear on the same `TARGET(0, 0)` surface while `GC DEAD(vid)` and respawn rebuild stay owned by `non-player-death-respawn-bootstrap.md`

If future captures or tests prove this carrier insufficient, the repository may add a richer combat packet family later.
But the next slices should begin from this smaller contract first.

## Repeated-hit loop and runtime-only HP ownership

The current bootstrap repeated-hit rule is now frozen as narrowly as possible:
- a visible `training_dummy` starts the live combat loop at authored/bootstrap max HP
- each accepted normal attack against the still-selected dummy decrements current live HP exactly once by the current bootstrap step
- the server reuses self-only `GC TARGET(target_vid, hp_percent)` after each accepted hit so the same selected target surface shows the updated percentage
- re-selecting that same still-visible dummy during the same live world runtime should return the current runtime-owned `hp_percent`, not silently recreate full HP on every request

The ownership rule is equally important:
- dummy HP belongs to shared-world runtime state, not to account or character persistence
- accepted dummy hits must not write inventory, equipment, player points, or any other character save payload as a side effect of combat alone
- this document does **not** yet freeze whether a reconnect, transfer, or future world rebuild should preserve or recreate dummy HP; it only freezes that the current bootstrap loop is runtime-owned and non-persistent

## First target-relative normal-attack cadence window

The next owned timing rule is still intentionally tiny:
- the bootstrap runtime now owns one fixed `250ms` cadence window for repeated normal `ATTACK` attempts against the same active selected target snapshot
- the first accepted normal hit on that live selected snapshot starts the server-owned window
- another same-target normal `ATTACK` that arrives before the `250ms` window expires fails closed with no combat-visible frames, no extra HP mutation, no extra immediate retaliation, and no extra delayed retaliation scheduling side effect
- once the `250ms` window expires, the next same-target normal `ATTACK` can be accepted again if the rest of the current target/visibility/range/dead-state checks still pass
- the window is measured from server-owned runtime time (`runtime.now` in tests, wall-clock time otherwise), not from client animation or any client-supplied timestamp
- clearing or replacing the active selected target resets this first owned cadence window; a broader session-wide attack-speed policy across target swaps still remains out of scope for now

## Failure semantics

The first owned attack-intent path must stay fail-closed.

An `ATTACK` request must fail closed when any of these are true:
- wrong phase
- malformed codec payload
- no selected live character exists
- no active combat target is currently bound to the session
- the request uses a non-normal bootstrap `attack_type`
- the request `target_vid` does not match the session's active combat target
- the selected target is no longer visible
- the selected target is no longer a `training_dummy`
- the selected target is no longer within the current bootstrap combat band
- the selected target no longer matches the current runtime snapshot bound to the session's accepted target selection
- the selected target is now at `0` HP / dead under runtime-owned dummy state
- the engaged owner's current bootstrap HP is already `0` after the current practice-mob retaliation slice reached the floor
- the session already lost authoritative live ownership because another session reclaimed the same character

The current visible failure expectations are intentionally narrow:
- malformed or wrong-phase requests may still stop at codec/flow rejection without a visible combat packet
- plain denied attack attempts do not yet require chat spam, peer fanout, or richer combat-result frames
- when runtime state already held a previously selected combat target and subject movement/sync makes that target invisible or out of the current combat band, the preferred first visible reset companion is one self-only `TARGET(0, 0)` plus local active-target cleanup

## Ownership and lifecycle rule

The first owned attack-intent contract must inherit the existing shared-world ownership model:
- attack authority belongs to the current live selected-character session
- stale reclaimed sockets must not authoritatively damage runtime-owned dummy HP, clear or replace the live owner's selected combat target, or queue combat-visible refresh frames to the replacement owner
- accepted target ownership now binds both the current dummy `target_vid` and the current runtime snapshot behind that `VID`; later attacks fail closed if the dummy was replaced before the session reselects it
- transfer rebootstrap, same-socket fresh bootstrap re-entry, and reconnect now clear session-local active combat target ownership before later attacks can proceed again
- non-player HP/dead state belongs to runtime world ownership, not to character persistence

Only some lifecycle edges are owned so far.
This document now freezes movement/sync invalidation plus fresh bootstrap/rebootstrap cleanup, while later combat slices still need to align the remaining lifecycle edges with the same runtime ownership model instead of creating a second combat-only ownership model.

## First sustained delayed server-origin retaliation cadence for engaged content practice mobs

The first owned delayed server-origin retaliation cadence is still narrow, but it is now autonomous once engagement has started:
- it currently applies only to content-loaded practice mobs imported from `spawn_groups` with `combat_profile = training_dummy`
- the first accepted owner-side normal hit that leaves that engaged mob alive arms one additional self-only `GC POINT_CHANGE` HP decrement after a fixed `1s` delay
- once that delayed beat fires while the same engaged mob still owns the same live owner, it automatically arms the next delayed beat after the same fixed `1s` delay even if the player sends no later `ATTACK`
- each queued beat is server-origin only: it arrives through the pending server-frame path instead of piggybacking only on a fresh client attack frame
- it reuses the same bootstrap player-point carrier and `-1` HP decrement already used by the immediate owner-side retaliation piggyback
- that owner-side retaliation point-loss now clamps at the current bootstrap HP floor instead of driving the owner's visible HP negative; once the owner's live HP reaches `0`, later immediate or delayed retaliation point-loss beats fail closed until broader player-death semantics are owned separately
- those immediate and delayed owner-side retaliation point-loss beats currently mutate only the engaged selected-session live runtime; they do **not** write the persisted account snapshot, so a fresh `/phase_select` re-entry or reconnect still rebuilds from the pre-retaliation persisted point value until broader player-death persistence or respawn semantics are owned
- when an immediate or delayed retaliation beat reaches that owner-side `0`-HP floor, the current slice now emits one self-only `GC DEAD(owner_vid)` before also clearing the live combat target with one self-only `GC TARGET(0, 0)` instead of leaving the stale engagement selected while broader player-death choreography stays out of scope
- once that engaged owner's live HP is already `0` because an earlier immediate or delayed retaliation beat reached the current bootstrap floor, later combat `TARGET` and normal `ATTACK` attempts from that same owner against engaged practice mobs also fail closed instead of continuing the combat loop while broader player-death semantics remain out of scope
- once that same owner-side floor has already been reached, later owner-side `MOVE` / `SYNC_POSITION` attempts also fail closed before live-position mutation, peer relocation fanout, or transfer-trigger rebootstrap work can run
- once that same owner-side floor has already been reached, later owner-side slash `/inventory_move` attempts also fail closed before carried-slot mutation can run
- once that same owner-side floor has already been reached, later owner-side slash `/equip_item` and `/unequip_item` attempts also fail closed before carried/equipped item movement, self appearance refresh, or template-backed point mutation can run
- the runtime still keeps at most one pending delayed beat at a time for that engaged owner/target pair; if another accepted hit lands while a delayed beat is already pending, the current slice does not stack, accelerate, or reset the already-owned cadence timer yet
- the cadence fails closed and stops if the engaged owner loses live shared-world ownership, clears or replaces the selected target, or the engaged actor dies / rebuilds before the next delay expires
- same-socket `/quit` and `/logout` now both count as immediate live-ownership loss boundaries for that cadence: each command removes the owner from shared-world visibility, cancels any pending delayed beat, and releases the current practice-mob engagement right away, while `/quit` still remains in `GAME` just long enough to return its self `CHAT_TYPE_COMMAND quit` delivery
- this is still a tiny deterministic cadence, not broader AI: it remains owner-only, fixed-delay, and bound to the current engaged live target instead of widening into movement, chase, or mob packet families yet

## Explicit unknowns still left for the next RED

The next flow/gameplay slices still need to prove or freeze:
- whether the runtime should validate or currently only preserve the two trailing raw CRC bytes
- whether later attack-speed ownership should stay target-relative or widen into a broader session-wide/global policy across target swaps
- the exact bootstrap respawn delay constant and scheduler shape for the first server-driven dummy respawn reset
- whether later hostile retaliation should widen beyond the current fixed-delay owner-only cadence into broader AI or richer mob-origin packet surfaces

Those unknowns are deliberate.
The codec now owns the exact wire shape, but the gameplay contract is still intentionally narrower than full combat semantics.

## Explicit non-goals

This slice does **not** yet freeze:
- the final gameplay meaning of every `attack_type` value
- final damage formulas beyond the current bootstrap `1` HP decrement
- broader session-wide attack-speed rules beyond the first fixed same-target `250ms` cadence window
- miss/crit/block results
- the server-driven respawn timer, delete/re-add reset burst, and full post-death rebuild that the separate death / respawn doc already freezes for the next slice
- broader hostile retaliation beyond the current owner-side self-only point-loss surfaces: one immediate piggyback on accepted practice-mob hits plus one sustained fixed-delay delayed server-origin follow-up cadence at a time
- broader player-death / respawn semantics or broader non-combat gameplay gating for zero-HP owners after that floor is reached beyond the self-only `GC DEAD(owner_vid)` signal frozen in `player-death-bootstrap.md`
- player-vs-player attack semantics
- skills, buffs, debuffs, or status effects
- any loot, rewards, corpse gameplay, aggro movement, or independent mob AI


## Success definition

After this document lands, the repository should be able to say:
- the next combat ingress is no longer vague; `ATTACK` is frozen exactly as client -> server header `0x0401`
- the project now owns the first clean-room `ATTACK` codec layout: `attack_type`, `target_vid`, `crc_proc_piece`, `crc_file_piece`
- the first live dummy attack path accepts only `attack_type = 0` and keeps gameplay target-relative by requiring the request `target_vid` to match the active selected combat target
- the first accepted bootstrap attack now mutates runtime-owned `training_dummy` HP deterministically from `10` downward in `1`-HP steps while reusing self-only `GC TARGET(target_vid, hp_percent)` as its visible success refresh
- accepted reselection of the same damaged dummy reuses the same current runtime `hp_percent` instead of resetting the visible target state back to `100`
- subject movement/sync that makes the selected dummy leave current visibility or the bootstrap combat band now proactively emits one self-only `GC TARGET(0, 0)` and clears the session-local active target
- transfer rebootstrap, same-socket fresh bootstrap re-entry, and reconnect now clear the session-local active target too, but the first owned contract keeps those lifecycle resets silent instead of claiming a visible clear-target packet
- duplicate-live reclaim now inherits the same shared-world hardening model as movement, whisper, item use, and merchant seams: stale `TARGET` / `ATTACK` packets fail closed and cannot mutate runtime dummy HP or the replacement owner's target state
- accepted target ownership now also carries the current runtime snapshot behind the selected dummy `VID`, so later `ATTACK` requests fail closed if that dummy is replaced before the session reselects it
- the zero-HP transition is now live: the final accepted hit drives the dummy from `1` to `0`, emits `GC DEAD(vid)` to visible sessions, and clears any selected session's combat target on the existing self-only `GC TARGET(0, 0)` surface
- a dead dummy is no longer targetable or attackable through the current bootstrap `TARGET` / `ATTACK` loop until the separate respawn-reset slice lands
- the first owned clear-target representation is now `GC TARGET(0, 0)`
- later HP refreshes stay on the same `GC TARGET(target_vid, hp_percent)` carrier until the zero-HP death edge, after which the repo switches to `GC DEAD(vid)` + target clear rather than inventing richer combat-result packets early
- the first death / respawn wire contract is now frozen separately in `non-player-death-respawn-bootstrap.md`, and this attack slice now implements the death side of that contract while leaving server-driven respawn reset for the next runtime slice
- content-loaded `spawn_groups` practice mobs now own the first aggro-lite post-hit target gate too: once the first authoritative hit is accepted, fresh third-party `TARGET` attempts fail closed until the existing death / respawn reset boundary, unless that engaged owner's retaliation-driven `0`-HP death clears the current engagement first, without claiming broader mob hostility yet
- repeated same-target normal `ATTACK` attempts are now also rate-owned in one narrow bootstrap shape: after an accepted hit, the same live selected target snapshot rejects further normal attacks for `250ms`, then accepts again once that fixed server-owned window expires
- that same first hostility seam is now slightly richer but still deterministic: while the engaged content-loaded practice mob stays alive, each accepted owner-side normal hit still appends one immediate self-only `GC POINT_CHANGE` HP decrement to the attack success frames, and the first accepted live hit now starts a delayed self-only `GC POINT_CHANGE` follow-up cadence that keeps firing every `1s` while the same engagement remains live
- those immediate and delayed owner-side retaliation point-loss beats stay runtime-only for the engaged selected session today: they do **not** write the persisted account snapshot, and later position-only persistence helpers (`MOVE`, `SYNC_POSITION`, or transfer rebootstrap saves) now keep saving coordinates without overwriting that pre-retaliation point value, so a fresh `/phase_select` re-entry or reconnect still rebuilds from the pre-retaliation point value until broader player-death persistence or respawn semantics are owned
- those owner-side retaliation point-loss beats now stop at the bootstrap HP floor too: neither the immediate hit-triggered tick nor the delayed server-origin cadence can drive the owner's visible HP below `0`, and once `0` is reached the current slice simply stops further point-loss without yet claiming broader player-death choreography
- when either the immediate retaliation tick or a delayed follow-up beat reaches that owner-side `0`-HP floor, the current slice now emits one self-only `GC DEAD(owner_vid)` and one self-only `GC TARGET(0, 0)` clear so the stale engaged mob is no longer kept as the active combat target
- when that same owner-side `0`-HP floor is reached while a content-loaded practice mob still remains alive, the dead owner also stops holding that mob's aggro-lite engagement gate: a different visible live session may reacquire the same still-live mob with a fresh `TARGET` without waiting for the mob to die / respawn or for the dead owner to disconnect first
- once that retaliation floor has already reached `0`, later same-owner combat `TARGET` and normal `ATTACK` attempts against still-engaged content practice mobs now fail closed too, so the current hostility seam no longer lets a zero-HP owner keep reacquiring or advancing dummy combat state while broader player-death semantics are still pending
- accepted hits while one delayed follow-up beat is already pending still do not stack, accelerate, or reset the current cadence timer yet; the runtime keeps only one queued delayed beat outstanding at a time
- same-target normal `ATTACK` attempts denied inside that `250ms` cadence window stay fully silent: they do not refresh target HP, do not append immediate retaliation, and do not create or reset delayed retaliation work
- if that engaged owner loses live shared-world ownership, clears or replaces target intent, or the engaged actor dies / rebuilds before a pending delay expires, the queued follow-up beat fails closed and the current cadence stops instead of claiming broader AI cleanup
- same-socket `/quit` and `/logout` now both count as immediate owner-disappearance boundaries for that queued delayed cadence: either command removes the owner from shared-world visibility, cancels any pending delayed beat, and releases the same still-live practice mob right away, although `/quit` still remains in `GAME` long enough to return its self `CHAT_TYPE_COMMAND quit` delivery
