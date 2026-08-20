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
3. ~~Add aggro-lite reset/cleanup boundaries for disconnect, transfer, death, and respawn.~~ Done for bootstrap scope: leave/logout/close, phase-select, transfer/rebootstrap, owner death floor, actor death/respawn, return-home/return-step, operator update, and EnterGame reclaim (including chase-deadline prune) release engagement / selected-target / pending retaliation ownership; `/restart_here` also has focused coverage for due return-step and due chase-step preflight while a zero-HP owner skipped lifecycle frames, and `/restart_town` now mirrors due destination respawn / return-step / chase-step preflights (return-step via outside-leash displace; chase-step with a live destination engager).
4. ~~Add first independent mob reaction timing that is not only piggybacked on player hits.~~ Done for bootstrap scope: proximity aggro-radius acquisition has the pure helper plus pending-frame live consumer, arms delayed self-only server-origin retaliation and chase without inventing selected-target ownership, releases on leave-radius walk-away, and keeps the same still-inside candidate suppressed until leave `DefaultSpawnAggroRadius` + re-enter after in-radius release as well as after death/respawn seed.
5. ~~Extend the first chase/leash/return planning seam beyond pure classification: tested pure return-step and chase-step planners, the pending-frame chase executor (including proximity-armed due chase without a hit), read-only pending chase inspection, retained-viewer chase `MOVE` replication, retained-viewer same-map return-step / return-home `MOVE` replication, and retained-viewer same-map live spawn-backed operator/runtime position `MOVE` now exist; presentation/name/race refreshes, cross-map return-home, and respawn rebuild remain on delete/readd.~~
6. ~~Harden multi-map and reconnect behavior so mobs do not duplicate, leak, or resurrect incorrectly.~~ Done for bootstrap scope: content-loaded still-dead EnterGame/reconnect now mirrors the `training_dummy` trailing-`DEAD` proof, due-respawn EnterGame preflight stays owned, one-ref/one-actor lookup fails closed on duplicates, import visibility stays map/AOI scoped, cross-map return-home delete/readd dual-map occupancy coverage is owned, Leave/transfer ownership cleanup remains owned, and non-identical still-dead content-bundle replacement remaps pending dead/respawn state by authored `spawn_group_ref` instead of resurrecting early. Same-map live spawn-backed operator/runtime position updates now reuse retained-viewer `MOVE` (presentation/name/race stay on delete/readd). Daemon-restart still-dead timer persistence is now owned too: spawn-backed static-actor snapshots carry optional `combat_current_hp=0` plus absolute `respawn_ready_at`, process restart rematerializes mid-dead practice mobs as still-dead / non-targetable through the remaining deadline, and successful respawn clears those persistence fields. Cross-map return MOVE / warp choreography remains deferred behind the `spawn-leash-bootstrap.md` packet freeze (delete/readd + dual-map occupancy stay owned today); live damaged-HP restart durability remains out of scope.
7. ~~Next: profile-authored optional `aggro_radius` on portable `combat_profiles`.~~ Done for bootstrap scope: optional authored `aggro_radius` round-trips through `combat_profiles`, resolves through `EffectiveStaticActorSpawnAggroRadius` / `...ForActor`, and live proximity acquire / leave-radius release / suppress seeding honor the effective radius (default `200`, fail-closed above leash `400`).

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
2. ~~Define one authored combat-profile formula seam for practice mobs without claiming full legacy math.~~ Done for bootstrap scope: portable `combat_profiles` already drive `max(1, attack_value - defense_value)`, and `docs/examples/bootstrap-combat-profile-formula-bundle.json` is the first playable QA fixture (`qa_formula_practice_mob` / `practice.qa_formula_mob`). Full legacy math remains out of scope.
3. ~~Extend reward descriptors from fixed examples toward table-driven EXP/gold/drop data.~~ Done for bootstrap scope: authoring-only `drop_tables` + `reward_drop_table_ref` expand into fixed spawn-group EXP/gold/drop-vnum descriptors before import (`docs/examples/bootstrap-drop-table-authoring-bundle.json`); weighted/random loot tables and pickup mutation remain out of scope / items-lane.
4. Harden player-death floor and restart behavior around mob retaliation and reconnect: **done for bootstrap scope** — when immediate/delayed practice-mob retaliation reaches the `0`-HP floor, the selected-character account snapshot now persists that bootstrap HP point as `0`, so reconnect / `/phase_select` / fresh `ENTERGAME` replay dead bootstrap (`PLAYER_POINT_CHANGE` at `0` + `GC DEAD(owner_vid)`). Accepted `/restart_here` / `/restart_town` restore race create MaxHP into that persisted snapshot as part of recovery. Partial (above-floor) retaliation loss stays runtime-only. Broader corpse / revive menus stay out of scope.
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
2. ~~Implement the contract-frozen confirm-after-preview refine success path for `probability = 100` only (preview opens same-socket dialog; matching confirm consumes gold/materials, replaces source `vnum`, persists, emits self-only refreshes; `type = 255` cancels; busy windows / lower probability stay fail-closed).~~ Done: preview remembers a same-socket dialog; matching `probability = 100` confirm consumes gold/materials, replaces source `vnum` in-place, persists, and emits self-only material refreshes + result `ITEM_SET` + gold `PLAYER_POINT_CHANGE`; `type = 255` cancels; busy merchant/exchange/safebox confirm attempts and lower probability stay fail-closed.
3. ~~Extend remaining item-template restriction/feedback edges only where a client-visible gap remains after the owned class/sex/level/anti-flag/equip-slot guards.~~ Done for the direct `ITEM_USE` / `/use_item` seam and for active-shell `EXCHANGE ITEM_ADD`: authored `use_reject_message` freezes self-only info-chat feedback for transfer guards / selected-character restrictions / confirm-quest-applicable guards on direct use, and authored `give_reject_message` now freezes the same transfer/selected-character feedback set for exchange display add; omitted text stays silent/no-frame. Broader remaining gaps (for example restart-restored ownership/despawn timers) stay deferred.
4. Keep partner-side open player-shop / cube busy-window rejection text deferred until those presentation seams exist. Open refine-dialog exchange `START` busy rejects are now owned beside merchant/safebox.
5. ~~Land the frozen bootstrap safebox-open presentation seam (`/open_safebox` / `/close_safebox` + `SAFEBOX_SIZE`) and wire exchange START requester/partner busy rejects to that open flag; keep password/load/placement/money deferred.~~ Done for bootstrap scope: `/open_safebox [1..3]` emits self-only `SAFEBOX_SIZE`, `/close_safebox` clears the open flag, and exchange START reuses the merchant busy-window chat strings for requester/partner open-safebox rejects without inventing storage mutation. Open refine-dialog presentation now uses the same requester/partner busy chat strings for exchange `START`.
6. ~~Finish integrating pending exchange anti-give/display-slot guards / mutual-accept finalization / ownership timers~~ Done for bootstrap scope on `lane/items`; do not reopen those as if they were still pending.
7. ~~Freeze the first accepted refine success seam in docs/spec before opening RED.~~ Done: `item-refine-bootstrap.md` now owns confirm-after-preview (`probability = 100`, `type = 255` cancel, busy-window fail-closed confirm).
8. ~~Own the bootstrap in-memory ground-item destroy deadline (`300` seconds from registration) so unclaimed public item/gold handles emit `GC::ITEM_GROUND_DEL` instead of littering forever.~~ Done for bootstrap scope; restart-restored despawn timers remain deferred.

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
4. Expand NPC service kinds only when client-visible behavior is owned.
5. ~~Add content validation/canonicalization that catches bad bundles before runtime mutation.~~ Done for the owned bootstrap path: authoring-only `drop_tables` accept kill-quest-only rows, and gated service / kill-quest require refs must have an in-bundle writer (`quest_flag` or kill-quest credit). Completely empty tables and orphan gates still fail closed.
6. Keep deterministic example bundles updated for manual QA; kill-quest-only `drop_tables` authoring now has `docs/examples/bootstrap-kill-quest-only-drop-table-authoring-bundle.json` beside the combat+kill-quest fixtures.

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
2. ~~Extract narrow repository seams only where tests prove reduced coupling.~~ Done for the first account character-state seam (`0002` roster / `0003` item-state / `0011` point-state), the matching quest-state seam (`0004`), the auth login-ticket handoff seam (`0007`), the item-template seam (`0009`), and the static-content seam (`0008`): named `AccountCharacterStateExporter` / `CharacterQuestStateExporter` / `AuthLoginTicketHandoffExporter` / `ItemTemplateStateExporter` / `StaticActorContentStateExporter` plus hermetic `MemoryStore`s (including paired `interactionstore.MemoryStore`); ground-handle seams and SQL-backed repositories remain follow-on.
3. ~~Write backup/restore runbooks backed by local validation or preflight tests.~~ Done for the six manifested JSON stores plus the read-only drill printer; ground-item durability remains deferred.
4. Harden crash/restart recovery for item/gold/character state used by the PvE loop.
5. Document release/deploy/versioning policy; the first release-identity stamp and loopback `/local/build-info` surface now exist, while deployment topology remains follow-on work.
6. Add production-safe observability conventions before remote admin surfaces.

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
