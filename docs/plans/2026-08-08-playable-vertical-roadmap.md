# Playable PvE Vertical Roadmap — 2026-08-08

This roadmap replaces the May 2026 broad parity map as the current planning anchor. The project should still move toward legacy parity, but the next useful milestone is narrower: make the server feel like a small, inspectable, recoverable PvE game loop.

This is not permission for a large rewrite. Keep landing small, verified slices and keep `main` green.

## Guiding rules

1. Keep `main` green and linear.
2. Keep one cohesive commit per slice.
3. Keep docs/specs/tests aligned with behavior.
4. Treat legacy source and captures only as external behavior oracles.
5. Prefer client-visible behavior over generic refactors.
6. Prefer runtime behavior that can be inspected by local ops endpoints while the system is still bootstrap-shaped.
7. Do not claim full legacy parity when a slice is only a practice-mob, file-store, or local-debug seam.

## Milestone goal

A narrow playable vertical means a player can:

1. connect through `authd` and `gamed`,
2. enter a map,
3. see authored NPC/static content and content-loaded mobs,
4. move and keep visibility stable,
5. target and fight mobs,
6. kill mobs and receive deterministic rewards,
7. pick up, use, equip, trade/reject, or sell basic items through owned paths,
8. reconnect/restart without corrupting state,
9. let an operator inspect and recover bootstrap stores through local-only tools.

Exit criteria for this milestone:

- A manual client QA pass can exercise login → map → mob kill → reward → pickup/use/equip/shop → reconnect/restart without relying primarily on slash/debug paths.
- Automated tests cover the same contract at protocol, runtime, and persistence boundaries.
- Unsupported behaviors fail closed and are documented.
- Local operator endpoints can validate the relevant state before and after the run.

## Track A — Mob AI and spawn lifecycle

**Objective:** turn stationary practice spawns into the first believable PvE mob lifecycle.

Primary lane:

- `lane/world` / `go-metin2-mob-spawn-lifecycle-worker`

Likely areas:

- `internal/worldruntime`
- `internal/minimal`
- `internal/contentbundle`
- `internal/staticstore`
- `spec/protocol/non-player-entity-bootstrap.md`
- `spec/protocol/content-spawn-groups-bootstrap.md`
- `spec/protocol/combat-normal-attack-bootstrap.md`
- `docs/qa/manual-client-checklist.md`

Next slices:

1. ~~Finish integrating map-local static actor snapshot coverage already pending on `lane/world`.~~ Done: `StaticActorsForMap` + loopback `GET /local/maps/{map_index}/static-actors` are owned with focused runtime and ops coverage.
2. ~~Make one content-loaded practice mob lifecycle fully explicit: spawn → target → hit → death → respawn → fresh reselection.~~ Done for bootstrap scope: `TestGameSessionFlowContentSpawnGroupPracticeMobRespawnsAfterServerDrivenDelayAndRequiresFreshReselect` owns the content-loaded kill → server-driven respawn → fresh reselection contract.
3. ~~Add aggro-lite reset/cleanup boundaries for disconnect, transfer, death, and respawn.~~ Done for bootstrap scope: leave/logout/close, phase-select, transfer/rebootstrap, owner death floor, actor death/respawn, return-home/return-step, operator update, and EnterGame reclaim (including chase-deadline prune) release engagement / selected-target / pending retaliation ownership; `/restart_here` also has focused coverage for due return-step and due chase-step preflight while a zero-HP owner skipped lifecycle frames, and `/restart_town` now mirrors due destination respawn / return-step / chase-step / homeward-step preflights (return-step via outside-leash displace; chase-step with a live destination engager; homeward-step after chase→release on the destination map).
4. ~~Add first independent mob reaction timing that is not only piggybacked on player hits.~~ Done for bootstrap scope: proximity aggro-radius acquisition has the pure helper plus pending-frame live consumer, arms delayed self-only server-origin retaliation and chase without inventing selected-target ownership, releases on leave-radius walk-away, and keeps the same still-inside candidate suppressed until leave `DefaultSpawnAggroRadius` + re-enter after in-radius release as well as after death/respawn seed.
5. ~~Extend the first chase/leash/return planning seam beyond pure classification: tested pure return-step and chase-step planners, the pending-frame chase executor (including proximity-armed due chase without a hit), read-only pending chase inspection, retained-viewer chase `MOVE` replication, retained-viewer same-map return-step / return-home `MOVE` replication, and retained-viewer same-map live spawn-backed operator/runtime position `MOVE` now exist; presentation/name/race refreshes, cross-map return-home, and respawn rebuild remain on delete/readd.~~
6. ~~Harden multi-map and reconnect behavior so mobs do not duplicate, leak, or resurrect incorrectly.~~ Done for bootstrap scope: content-loaded still-dead EnterGame/reconnect now mirrors the `training_dummy` trailing-`DEAD` proof, due-respawn EnterGame preflight stays owned, one-ref/one-actor lookup fails closed on duplicates, import visibility stays map/AOI scoped, cross-map return-home delete/readd dual-map occupancy coverage is owned, automatic pending-frame cross-map return-step after `UpdateStaticActor` displace now mirrors that same dual-map anti-leak proof (due flush snaps to authored home via delete/readd with no invented MOVE), Leave/transfer ownership cleanup remains owned, and non-identical still-dead content-bundle replacement remaps pending dead/respawn state by authored `spawn_group_ref` instead of resurrecting early. Same-map live spawn-backed operator/runtime position updates now reuse retained-viewer `MOVE` (presentation/name/race stay on delete/readd). Daemon-restart still-dead timer persistence is now owned too: spawn-backed static-actor snapshots carry optional `combat_current_hp=0` plus absolute `respawn_ready_at`, process restart rematerializes mid-dead practice mobs as still-dead / non-targetable through the remaining deadline, and successful respawn clears those persistence fields. Daemon-restart live damaged HP persistence is now owned beside that seam: accepted non-lethal spawn-group hits persist `combat_current_hp` in `1..max_hp-1` with `respawn_ready_at` omitted, process restart rematerializes the damaged HP / `hp_percent`, and engagement / selected-target / chase / return ownership stay fail-closed across restart. Live damaged HP remapping across non-identical content-bundle replacement is now owned beside still-dead replacement remapping (engagement stays fail-closed). Cross-map return MOVE / warp choreography remains deferred behind the `spawn-leash-bootstrap.md` packet freeze (delete/readd + dual-map occupancy stay owned today).
7. ~~Next: profile-authored optional `aggro_radius` on portable `combat_profiles`.~~ Done for bootstrap scope: optional authored `aggro_radius` round-trips through `combat_profiles`, resolves through `EffectiveStaticActorSpawnAggroRadius` / `...ForActor`, and live proximity acquire / leave-radius release / suppress seeding honor the effective radius (default `200`, fail-closed above the profile's effective leash).
8. ~~Next: profile-authored optional `leash_radius` on portable `combat_profiles`.~~ Done for bootstrap scope: optional authored `leash_radius` round-trips through `combat_profiles`, resolves through `EffectiveStaticActorSpawnLeashRadius` / `...ForActor`, and live leash classification / return-step / chase clamp / `target_return_required` gating honor the effective radius (default `400`, fail-closed below the profile's effective aggro). Operator leash GET endpoints keep an explicit `radius` override and default omitted lookups through the actor effective leash. Cross-map return MOVE / warp choreography remains deferred behind the packet freeze.
9. ~~Next: unengaged `within_radius` homeward-step after chase/engagement release.~~ Done for bootstrap scope: pure `PlanStaticActorSpawnLeashHomewardStep`, pending-frame `1s` / `max_step=100` homeward executor with same-map retained-viewer `MOVE`, engagement-release arming, chase mutual exclusion, EnterGame/transfer/restart preflight flush after `return_required` return-steps / before chase, focused EnterGame / MOVE-transfer / `/restart_here` / `/restart_town` due-homeward preflight coverage now owned alongside chase/return, daemon-restart rematerialization of live unengaged `within_radius` spawn-backed actors now arms pending homeward the same way `return_required` arms return-step, and read-only pending homeward inspection (`/local/spawn-group-homeward-steps`, exact, map-local). Operator `return-step` still no-ops `within_radius`; exact-home remains `return-home`. Operator/runtime `UpdateStaticActor` that leaves an unengaged spawn-backed actor `within_radius` now re-arms pending homeward through the shared eligibility sync. Cross-map return MOVE stays deferred.
10. ~~Next: operator/runtime same-map position `UpdateStaticActor` that leaves a live unengaged spawn-backed actor classified `within_radius` should re-arm pending homeward (mirroring how `return_required` re-arms return-step) instead of only clearing the deadline.~~ Done for bootstrap scope: `UpdateStaticActor` now calls `syncSpawnGroupHomewardStepScheduleForEntity` after return-step sync / chase clear; focused coverage proves arming plus due homeward flush restoring `at_home` without arming return-step.
11. ~~Next: automatic pending-frame cross-map return-step dual-map anti-leak after `UpdateStaticActor` displace.~~ Done for bootstrap scope: `TestGameRuntimeFlushServerFramesAppliesDueCrossMapSpawnGroupReturnStepLeavesNoDualMapOccupancy` owns arm → due flush → authored-home delete/readd with no invented MOVE, one-ref/one-actor, empty foreign-map occupancy, cleared pending schedule, and persisted authored home beside the already-owned operator return-home dual-map proof.
12. ~~Next: content-bundle import prune/restore for pending homeward-step schedules (mirror return/chase).~~ Done for bootstrap scope: `ImportContentBundle` no-op / successful replacement / failed-rollback paths now prune or restore `spawnHomewardStepDueAt` beside return/chase; focused coverage owns stale prune, removed-actor clear, and failed-import due-at restore + retained-viewer homeward `MOVE`.
13. ~~Next: keep within-radius homeward armed across owner death-floor engagement release after chase displace.~~ Done for bootstrap scope: immediate and delayed death-floor paths no longer clear the homeward schedule that `clearActiveCombatTarget` just armed; `TestGameRuntimeOwnerDeathFloorArmsHomewardAfterChaseDisplaceWithinRadius` proves the still-connected dead owner is skipped for retained homeward `MOVE` while a living watcher receives the due step back to authored home.
14. ~~Next: proximity-armed owner death-floor release must seed leave/re-enter suppress through same-socket `/restart_here` while still inside aggro radius.~~ Done for bootstrap scope: `TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorRestartHere` proves the recovered still-inside owner stays suppressed through later pending-frame flushes until an explicit leave/re-enter of `DefaultSpawnAggroRadius`, matching `TARGET(0)` and mob death/respawn suppress seeding. Next honest Track A follow-on: the same suppress must survive `/phase_select` Leave → fresh Join (new entity ID) → `/restart_here` while still inside radius; cross-map return MOVE / warp choreography remains deferred behind the `spawn-leash-bootstrap.md` freeze (no speculative RED).

Exit criteria:

- content-loaded mobs round-trip through authored data and runtime state,
- a dead mob stays dead until its server-owned respawn boundary,
- respawn uses the normal visibility rebuild path,
- transfer/reconnect cannot leave stale target or engagement ownership behind,
- docs keep calling this bootstrap/practice AI until movement/pathing is real.

Anti-goals:

- no full pathfinding rewrite,
- no broad formula work in the world lane,
- no distributed channel/shard architecture before single-process semantics are stable.

## Track B — Combat, death, rewards, and formulas

**Objective:** make the PvE loop resolve damage, death, and rewards through owned contracts.

Primary lane:

- `lane/combat` / `go-metin2-combat-worker`

Likely areas:

- `internal/proto/combat`
- `internal/proto/world`
- `internal/worldruntime`
- `internal/minimal`
- `internal/player`
- `spec/protocol/combat-*`
- `spec/protocol/non-player-death-respawn-bootstrap.md`
- `spec/protocol/non-player-reward-bootstrap.md`
- `spec/protocol/player-death-bootstrap.md`

Next slices:

1. Keep target-marker, fly-effect, stun, PvP/duel packet families codec-only until runtime policy is proven.
2. ~~Define one authored combat-profile formula seam for practice mobs without claiming full legacy math.~~ Done for bootstrap scope: portable `combat_profiles` already drive `max(1, attack_value - defense_value)`, and `docs/examples/bootstrap-combat-profile-formula-bundle.json` is the first playable QA fixture (`qa_formula_practice_mob` / `practice.qa_formula_mob`). The composed PvE vertical authoring fixture now also owns packet-level first-hit formula `DAMAGE_INFO(damage = 5)` coverage beside hit-count kill proofs. Full legacy math remains out of scope.
3. ~~Extend reward descriptors from fixed examples toward table-driven EXP/gold/drop data.~~ Done for bootstrap scope: authoring-only `drop_tables` + `reward_drop_table_ref` expand into fixed spawn-group EXP/gold/drop-vnum descriptors before import (`docs/examples/bootstrap-drop-table-authoring-bundle.json`); weighted/random loot tables and pickup mutation remain out of scope / items-lane.
4. Harden player-death floor and restart behavior around mob retaliation and reconnect: **done for bootstrap scope** — when immediate/delayed practice-mob retaliation reaches the `0`-HP floor, the selected-character account snapshot now persists that bootstrap HP point as `0`, so reconnect / `/phase_select` / fresh `ENTERGAME` replay dead bootstrap (`PLAYER_POINT_CHANGE` at `0` + `GC DEAD(owner_vid)`). Accepted `/restart_here` / `/restart_town` restore race create MaxHP into that persisted snapshot as part of recovery. Partial (above-floor) retaliation loss stays runtime-only. Proximity-armed delayed beats that never invented selected-target ownership now also prove the same floor ordering (`PLAYER_POINT_CHANGE(value=0)` → `DEAD` → `TARGET(0, 0)`), peer `DEAD` fanout, persistence, engagement release, and post-floor fail-closed gates via `TestGameRuntimeProximityAggroDelayedRetaliationReachesOwnerDeathFloorWithoutHitOrTarget`. Same-socket `/restart_here` while still inside aggro radius after that proximity-armed floor keeps leave/re-enter proximity suppress via `TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorRestartHere`. Broader corpse / revive menus stay out of scope.
5. Add first accepted skill/ranged/projectile runtime only after packet evidence and tests are owned.
6. Defer PvP/duel runtime policy until the PvE baseline is stable.

Exit criteria:

- combat outcome is deterministic, tested, and visible to the client,
- rewards flow into item/gold/EXP systems through existing mutation/persistence seams,
- player death/restart does not corrupt inventory, quickslots, target state, or persistence,
- unsupported combat packet families fail closed instead of producing fake behavior.

Anti-goals:

- no full skill system before normal PvE combat is stable,
- no PvP policy guessing,
- no fake loot/PnL-style evidence; rewards must be backed by tests and docs.

## Track C — Items, economy, trade, and storage foundations

**Objective:** make item/economy behavior useful enough to support the PvE loop.

Primary lane:

- `lane/items` / `go-metin2-items-worker`

Likely areas:

- `internal/player`
- `internal/inventory`
- `internal/itemstore`
- `internal/proto/item`
- `internal/proto/quickslot`
- `internal/minimal`
- `spec/protocol/item-*`
- `spec/protocol/quickslot-bootstrap.md`
- `docs/qa/manual-client-checklist.md`

Next slices:

1. ~~Harden the bootstrap `/open_safebox` fail-closed edge for out-of-range size (`/open_safebox 4`) so it emits no frames, does not open the busy flag, and does not fall through as ordinary chat.~~ Done: recognized out-of-range / invalid `/open_safebox` attempts are consumed fail-closed with no `SAFEBOX_SIZE`, no talking-chat fallthrough, and no same-socket open/busy mutation.
2. ~~Implement the contract-frozen confirm-after-preview refine success path for `probability = 100` only (preview opens same-socket dialog; matching confirm consumes gold/materials, replaces source `vnum`, persists, emits self-only refreshes; `type = 255` cancels; busy windows / lower probability stay fail-closed).~~ Done: preview remembers a same-socket dialog; matching `probability = 100` confirm consumes gold/materials, replaces source `vnum` in-place, persists, and emits self-only material refreshes + material-removal `GC::QUICKSLOT_DEL` for fully consumed material cells + result `ITEM_SET` + gold `PLAYER_POINT_CHANGE` + `CHAT_TYPE_COMMAND` `RefineSuceeded <type>`; `type = 255` cancels; busy merchant/exchange/safebox confirm attempts stay fail-closed. ~~Also own deterministic `probability = 0` destroy + `RefineFailed <type>`.~~ Done: matching `probability = 0` confirm consumes gold/materials, destroys the source cell, syncs material/source quickslots, persists, and emits self-only material refreshes + source `ITEM_DEL` + gold `PLAYER_POINT_CHANGE` + `CHAT_TYPE_COMMAND` `RefineFailed <type>`. ~~Next: freeze then implement injected-roll confirm for `probability` in `1..99` (`docs/plans/2026-08-22-refine-confirm-probability-1-99-injected-roll.md`); keep-grade/catalyst variants stay deferred.~~ Done: matching `probability` in `1..99` confirm draws one roll via `takeRefineConfirmRoll()` (`crypto/rand` production; `QueueRefineConfirmRollForTest` for tests), routes `roll <= probability` to the owned success burst / `RefineSuceeded` and `roll > probability` to the owned destroy burst / `RefineFailed` (including destroy-source quickslot sync), and fails closed for out-of-range rolls; keep-grade/catalyst variants stay deferred. Next: restart-restored ground-item ownership/despawn timers remain deferred until a later items-lane slice owns them; partner player-shop / cube busy rejects stay deferred until those presentation seams exist.
3. ~~Extend remaining item-template restriction/feedback edges only where a client-visible gap remains after the owned class/sex/level/anti-flag/equip-slot guards.~~ Done for the direct `ITEM_USE` / `/use_item` seam and for active-shell `EXCHANGE ITEM_ADD`: authored `use_reject_message` freezes self-only info-chat feedback for transfer guards / selected-character restrictions / quest-applicable guards on direct use, and authored `give_reject_message` now freezes the same transfer/selected-character feedback set for exchange display add; omitted text stays silent/no-frame. Broader remaining gaps (for example restart-restored ownership/despawn timers) stay deferred. ~~Next: reclassify `confirm_when_use` as client-local confirm + ordinary `ITEM_USE` (`docs/plans/2026-08-22-item-use-confirm-when-use-ordinary-path.md`); keep `quest_use` / `applicable` fail-closed.~~ Done: `confirm_when_use` follows the ordinary consumable success path and is no longer a `use_reject_message` store guard; `quest_use` / `quest_use_multiple` / `applicable` stay fail-closed.
4. Keep partner-side open player-shop / cube busy-window rejection text deferred until those presentation seams exist. Open refine-dialog exchange `START` busy rejects are now owned beside merchant/safebox; exchange `ACCEPT` / second-accept mutual-accept finalize now also fail closed when either paired side has an open merchant/safebox/refine busy presentation and emit the same self-only requester/partner busy info-chat strings already owned by `START`; exact-position transfer / warp rebootstrap now tears down an open exchange shell with self/peer `GC::EXCHANGE END` before the transfer burst (including same-map in-range destinations); mutual-accept `CommitExchangeFinalize` now revalidates busy windows plus displayed item/gold / receiver finalization preconditions at commit time so post-plan drift cannot sneak past the accept-time gate; and commit-time busy-window drift now returns those same self-only START/ACCEPT busy info-chat strings to the commit requester; second-accept / commit-time Check-shaped displayed item/gold drift and inventory-capacity failures now emit dual-sided info-chat (`Not enough Yang or the item is not in place.` / partner wording; `There isn't enough space in your inventory.` / partner wording) while other non-busy receiver precondition failures stay silent/no-frame (`docs/plans/2026-08-22-exchange-finalize-check-space-reject-chat.md`). ~~Next: freeze then implement dual-sided gold-overflow finalize reject chat (`docs/plans/2026-08-22-exchange-finalize-gold-overflow-reject-chat.md`); id-collision / restriction rejects stay silent.~~ Done: second-accept / commit-time receiver gold-overflow emits dual-sided `You cannot carry any more gold.` / `The other person cannot carry any more gold.`; id-collision / restriction rejects stay silent. ~~Next: freeze then implement dual-sided mutual-accept success chat (`docs/plans/2026-08-22-exchange-finalize-success-chat.md`); id-collision / restriction rejects stay silent.~~ Done: mutual-accept finalize emits dual-sided `The trade with <partner_name> has been successful.` before shell `END`; id-collision / restriction rejects stay silent. ~~Next: freeze then implement `EXCHANGE START` gold-carrier-cap reject chat (`docs/plans/2026-08-22-exchange-start-gold-carrier-cap-reject-chat.md`); id-collision / restriction rejects stay silent.~~ ~~Next: freeze then implement the first accepted in-memory `SAFEBOX_CHECKIN` while `/open_safebox` is already open (`docs/plans/2026-08-21-safebox-checkin-in-memory-mutation.md`); password/load/checkout/money/durable persistence stay deferred.~~ Done for bootstrap scope: accepted open-presentation `SAFEBOX_CHECKIN` removes a carried stack, syncs source item quickslots, stores same-session in-memory safebox contents, emits `ITEM_DEL` + `SAFEBOX_SET`, re-emits remembered rows on reopen, and closes an active exchange shell on success. ~~Next: freeze then implement the first accepted in-memory `SAFEBOX_CHECKOUT` while `/open_safebox` is already open (`docs/plans/2026-08-21-safebox-checkout-in-memory-mutation.md`); password/load/move/money/durable persistence stay deferred.~~ Done for bootstrap scope: accepted open-presentation `SAFEBOX_CHECKOUT` removes a remembered same-session safebox cell, places/merges into carried inventory with `SAFEBOX_DEL` + `ITEM_SET` / `ITEM_UPDATE`, persists inventory, and closes an active exchange shell on success. ~~Next: freeze then implement the first accepted same-session `SAFEBOX_ITEM_MOVE` while `/open_safebox` is already open (`docs/plans/2026-08-21-safebox-item-move-in-memory-mutation.md`); password/load/money/durable persistence stay deferred.~~ ~~Done for bootstrap scope: accepted open-presentation whole-stack `SAFEBOX_ITEM_MOVE` relocates into an empty safebox cell or merges into a compatible same-`vnum` cell under template `max_count`, emits self-only `SAFEBOX_DEL` + `SAFEBOX_SET` without inventory/quickslot/gold/account mutation, and closes an active exchange shell on success; partial splits / password/load/money/durable persistence stay deferred.~~ ~~Next: freeze then implement merchant-window auto-close on accepted check-in/out/move success (`docs/plans/2026-08-21-safebox-accepted-mutation-merchant-auto-close.md`).~~ Done for bootstrap scope: accepted open-presentation `SAFEBOX_CHECKIN` / `SAFEBOX_CHECKOUT` / `SAFEBOX_ITEM_MOVE` now emit self-only `GC::SHOP END` before refresh frames when a merchant window is open (SHOP before exchange when both shells are active). ~~Next: freeze then implement partial-count `SAFEBOX_ITEM_MOVE` split / compatible partial merge (`docs/plans/2026-08-21-safebox-item-move-partial-split.md`).~~ Done for bootstrap scope: partial-count empty-destination split allocates a new safebox item identity and emits dual `SAFEBOX_SET`; compatible partial merge refreshes both cells the same way; money / password / durable persistence stay deferred. ~~Next: emit legacy `CHAT_TYPE_COMMAND` `CloseSafebox` on `/close_safebox` / `/safebox_close` (`docs/plans/2026-08-21-safebox-close-command-chat.md`).~~ Done for bootstrap scope: open presentation close returns self-only `CloseSafebox` command chat so TMP4 clients hide the window; already-closed close stays silent/consume; password/load/money/durable persistence stay deferred. Done for bootstrap scope: practice-mob floor, exact-position transfer/warp rebootstrap, and `/phase_select` / `/quit` / `/logout` now also emit self-only `CloseSafebox` when the presentation was open (`docs/plans/2026-08-21-safebox-lifecycle-close-command-chat.md`). Partner-side open player-shop / cube busy-window exchange rejects stay deferred until those presentation seams exist. ~~Next: accept the TMP4 inventory-window `SAFEBOX_ITEM_MOVE` wire (`docs/plans/2026-08-21-safebox-item-move-tmp4-inventory-window-wire.md`).~~ Done for bootstrap scope: open-presentation `SAFEBOX_ITEM_MOVE` now accepts TMP4 `WindowInventory` packed positions (and mixed inventory/safebox tooling) as same-session safebox slot indices beside explicit `WindowSafebox`; mall/equipment/other windows stay fail-closed; money / password / durable persistence stay deferred. ~~Next: freeze docs so `quickslot-bootstrap.md` / packet-matrix own the already-shipped open-presentation `SAFEBOX_CHECKIN` source-cell `QUICKSLOT_DEL` path (`docs/plans/2026-08-21-safebox-checkin-quickslot-sync-docs.md`).~~ Done for bootstrap scope: docs now name accepted check-in quickslot removal sync beside use/sell/drop/refine/exchange; check-out and safebox item-move stay non-quickslot-mutating for carried bindings; mall/timeout/destruction/belt sync and password/money/durable persistence stay deferred. ~~Next: emit legacy `CHAT_TYPE_COMMAND` `RefineSuceeded <type>` after accepted `probability = 100` refine confirm (`docs/plans/2026-08-21-refine-confirm-success-command-chat.md`).~~ Done for bootstrap scope: success confirm burst ends with self-only `RefineSuceeded <type>` after gold `PLAYER_POINT_CHANGE`; `RefineFailed` / lower-probability destroy stay deferred.
5. ~~Land the frozen bootstrap safebox-open presentation seam (`/open_safebox` / `/close_safebox` + `SAFEBOX_SIZE`) and wire exchange START requester/partner busy rejects to that open flag; keep password/load/placement/money deferred.~~ Done for bootstrap scope: `/open_safebox [1..3]` emits self-only `SAFEBOX_SIZE`, `/close_safebox` / `/safebox_close` clear the open flag with self-only `CHAT_TYPE_COMMAND` `CloseSafebox`, and exchange START reuses the merchant busy-window chat strings for requester/partner open-safebox rejects without inventing storage mutation. Open refine-dialog presentation now uses the same requester/partner busy chat strings for exchange `START`.
6. ~~Finish integrating pending exchange anti-give/display-slot guards / mutual-accept finalization / ownership timers~~ Done for bootstrap scope on `lane/items`; do not reopen those as if they were still pending.
7. ~~Freeze the first accepted refine success seam in docs/spec before opening RED.~~ Done: `item-refine-bootstrap.md` now owns confirm-after-preview (`probability = 100`, `type = 255` cancel, busy-window fail-closed confirm).
8. ~~Own the bootstrap in-memory ground-item destroy deadline (`300` seconds from registration) so unclaimed public item/gold handles emit `GC::ITEM_GROUND_DEL` instead of littering forever.~~ Done for bootstrap scope, including kill-reward drops that reuse the same exclusive-ownership timer / blank public `ITEM_OWNERSHIP` release / ordinary collector pickup path as player drops; pending handles now also rematerialize across `gamed` process restart from the dedicated ground-item FileStore with absolute ownership/despawn timers.

Exit criteria:

- PvE rewards can become carried or ground items safely,
- common item uses are visible and persistent,
- trade/storage/refine either work in a narrow tested way or fail closed with documented feedback,
- item mutations update quickslots, peers, and persistence consistently.

Anti-goals:

- no broad player-shop or mall work before carried/ground/trade semantics are stable,
- no guessed item-use families without evidence,
- no item/economy behavior that bypasses template validation.

## Track D — Content, NPC services, quests, regen, and drops

**Objective:** make authored content drive the vertical instead of hardcoded examples.

Primary lane:

- `lane/content` / `go-metin2-content-worker`

Likely areas:

- `internal/contentbundle`
- `internal/staticstore`
- `internal/interactionstore`
- `internal/minimal`
- future quest/content packages
- `spec/protocol/static-actor-*`
- `spec/protocol/npc-*`
- `spec/protocol/content-spawn-groups-bootstrap.md`
- `docs/examples/*`

Next slices:

1. Keep content summaries useful, but stop prioritizing endpoint-only work unless it unblocks QA or migration.
2. ~~Add first quest-state seam: quest flags, NPC dialog state, and one simple persisted trigger/result contract.~~ Done for bootstrap scope: `quest_flag` interact CAS, optional service gates, kill-quest credit + require gates, and turn-in NPC close the persisted flag loop.
3. ~~Add regen/drop table ingestion in a deliberately small authored format.~~ Done for bootstrap scope: one-count `regen_spawns` + fixed `drop_tables` expand before import; `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` now composes that authoring form with the quest NPC loop.
4. Expand NPC service kinds only when client-visible behavior is owned; `open_safebox` is now the third owned service family beside `warp` / `shop_preview`, and gated warehouse mismatch previews are covered beside the other service gate previews.
5. ~~Add content validation/canonicalization that catches bad bundles before runtime mutation.~~ Done for the owned bootstrap path: authoring-only `drop_tables` accept kill-quest-only rows, and gated service / kill-quest require refs must have an in-bundle writer (`quest_flag` or kill-quest credit). Completely empty tables and orphan gates still fail closed.
6. Keep deterministic example bundles updated for manual QA; kill-quest-only `drop_tables` authoring now has `docs/examples/bootstrap-kill-quest-only-drop-table-authoring-bundle.json` plus the regen twin `docs/examples/bootstrap-kill-quest-only-regen-authoring-bundle.json` beside the combat+kill-quest fixtures. `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` now also includes the gated `Warehouse` `open_safebox` actor and formula-first portable combat profile `qa_pve_vertical_practice_mob` so one authoring-form import covers warehouse smoke beside merchant / warp / quest turn-in plus authored damage/HP. Kill-quest / PvE / focused `quest_flag` turn-in gameplay proofs now also run on hermetic static/interaction/item/quest MemoryStores rather than disposable content FileStores. Quest-gated and `quest_flag` interaction-visibility preview suites now use the same MemoryStore injection path instead of `QuestStateStorePath` FileStores. Content-bundle runtime export/import/summary suites now use the same MemoryStore injection path as well.

Exit criteria:

- ~~the PvE vertical can be authored with bundle data,~~ done for bootstrap scope via the composed authoring fixture plus the byte-canonical NPC service fixture,
- ~~one quest-style state can persist and resume,~~ done for bootstrap scope (`met_guide` / `killed_qa_mob` survive reconnect and gate services / kill credit),
- ~~spawn/regen/drop definitions are validated before runtime use,~~ done for the owned one-count / fixed-table authoring path,
- unsupported service kinds fail early and clearly.

Anti-goals:

- no full quest-script compatibility in one pass,
- no live reload that can half-commit invalid content,
- no content definitions that bypass store validation.

## Track E — Persistence, DB, migrations, and production ops

**Objective:** move from bootstrap file snapshots toward durable, operable service foundations.

Primary lane:

- `lane/persistence` / `go-metin2-db-ops-worker`

Likely areas:

- `db/migrations`
- `internal/accountstore`
- `internal/loginticket`
- `internal/player`
- `internal/itemstore`
- `internal/config`
- `internal/ops`
- `docs/development.md`
- `docs/workflow.md`
- future deployment/runbook docs

Next slices:

1. Extend the new `db/migrations` catalog beyond the initial `schema_migrations` ledger only when a narrow repository/backfill contract is ready; export quarantine/preflight now exists for `0002`/`0003`/`0004`/`0007`/`0008`/`0009`/`0010`/`0011` migration-shaped exports, and offline `metin2-migrate quarantine-export` / `backup-restore-drill` / `ledger-snapshot-status` close the runbook gap beside the loopback surfaces.
2. ~~Extract narrow repository seams only where tests prove reduced coupling.~~ Done for the first account character-state seam (`0002` roster / `0003` item-state / `0011` point-state), the matching quest-state seam (`0004`), the auth login-ticket handoff seam (`0007`), the item-template seam (`0009`), the static-content seam (`0008`), and the bootstrap ground-handle seam (`0010`): named `AccountCharacterStateExporter` / `CharacterQuestStateExporter` / `AuthLoginTicketHandoffExporter` / `ItemTemplateStateExporter` / `StaticActorContentStateExporter` / `BootstrapGroundItemStateExporter` plus hermetic `MemoryStore`s (including paired `interactionstore.MemoryStore` and `worldruntime.MemoryGroundItemStore` / `SnapshotGroundItemExporter`); SQL-backed repositories remain follow-on.
3. ~~Write backup/restore runbooks backed by local validation or preflight tests.~~ Done for the six manifested JSON stores plus the read-only drill printer; pending ground-item FileStore status/runtime-config reporting is owned, while ground-handle BackupTo/RestoreFrom drill coverage remains follow-on.
4. ~~Harden crash/restart recovery for item/gold/character state used by the PvE loop.~~ Done for committed FileStore rematerialization of character gold, inventory, and quest flags after a `quest_flag` reward turn-in (`TestGameRuntimeQuestFlagRewardStateRematerializesAcrossDaemonRestart`), equipment and quickslots after live equip/bind (`TestGameRuntimeEquipmentAndQuickslotsRematerializeAcrossDaemonRestart`), map/x/y and character point-state after live item-use + transfer (`TestGameRuntimePositionAndPointsRematerializeAcrossDaemonRestart`), and pending bootstrap ground item/gold handles with absolute ownership/despawn timers (`TestGameRuntimePendingGroundItemAndGoldRematerializeAcrossDaemonRestart`); SQL import/backfill from quarantined `0010` exports remains deferred.
5. ~~Document release/deploy/versioning policy;~~ Done for release identity (`internal/buildinfo`, loopback `/local/build-info`, `metin2-migrate version`) plus the first lab host layout and artifact-retention trees in `docs/workflow/lab-deployment-topology.md`; multi-host / orchestrated deploy automation remains follow-on work. Lab leftover-lock recovery is now owned beside that topology in `docs/workflow/lab-stale-lock-recovery.md` with an advisory `manual_clear_candidate` bit on `apply-lock-status` (still no auto-delete).
6. ~~Add production-safe observability conventions before remote admin surfaces.~~ Done for daemon JSON logging via `internal/observability.NewServiceLogger` and `docs/workflow/production-observability.md` (identity attrs + sensitive-key redaction) plus metadata-only loopback `/local/*` access logs via `observability.WrapOpsAccessLog` (no bodies/query strings; `/healthz` and `/debug/pprof/*` stay quiet); metrics/tracing/remote log shipping remain follow-on work.

Exit criteria:

- migrations are repeatable,
- operators can validate and back up/restore key state,
- core gameplay state has a clear path from file-backed bootstrap to DB-backed storage,
- sensitive ops remain loopback/local-only unless an explicit auth design exists.

Anti-goals:

- no big-bang DB migration,
- no secret-bearing production config in git,
- no remote admin API without authentication and threat model.

## Track F — Social systems after the PvE loop

**Objective:** replace bootstrap chat fanout with real social state after the PvE loop has enough gameplay to share.

Likely future areas:

- `internal/proto/chat`
- future party/guild/messenger packages
- `internal/minimal`
- `internal/worldruntime`
- `spec/protocol/party-*`
- `spec/protocol/guild-*`

Next slices after PvE stabilizes:

1. Party membership state.
2. Invite/accept/leave/kick where packet evidence exists.
3. Party chat scoping based on membership rather than bootstrap fanout.
4. Party EXP/drop sharing after reward ownership is stable.
5. Guild roster/rank state.
6. Friend/messenger/block systems.

Exit criteria:

- party/guild chat is membership-scoped,
- gameplay systems can query social state safely,
- membership persistence is either implemented or explicitly scoped.

Anti-goals:

- no social effects before membership exists,
- no claims of real party/guild support while fanout remains bootstrap-shaped.

## Integration and validation model

Autonomous lanes should continue to produce small commits on lane branches. The integrator should be the only job that pushes `origin/main`.

Recommended lane mapping:

- `lane/items` — Track C
- `lane/combat` — Track B
- `lane/world` — Track A
- `lane/content` — Track D
- `lane/persistence` — Track E
- `main` — integration only

Validation expectations:

- focused tests for the touched slice,
- `gofmt` clean,
- `git diff --check`,
- `go test ./...` when the integration environment can complete it reliably,
- `go vet ./...`,
- daemon builds before pushing code changes to `main`.

If the full suite times out in filesystem-heavy `internal/minimal` file-store tests, treat that as an infrastructure/timeout problem to fix deliberately. Do not push code changes that have not passed the agreed integration gate; for documentation-only changes, at minimum validate links, formatting, and repository reality.
