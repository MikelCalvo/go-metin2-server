# Protocol notes

The repository targets TMP4-era client compatibility, but the protocol contract is documented here in project-owned language.

## Protocol documents

- `session-phases.md` — working session model and allowed transitions
- `frame-layout.md` — stream envelope and control-packet framing assumptions
- `control-handshake.md` — control-plane packet layouts for phase, ping, pong, and key exchange
- `client-subsystems.md` — observed behavioral differences among the main stream, auth connector, and state checker
- `auth-login.md` — minimal auth-server credential exchange and login-key issuance
- `client-auth-and-selection-flow.md` — client-side auth-to-select choreography and selection-surface gotchas
- `login-selection.md` — minimal login-by-key and selection-surface packet layouts
- `select-world-entry.md` — minimal selection, loading, and enter-game packet choreography
- `client-game-entry-sequence.md` — client-side loading, enter-game, self bootstrap, and early movement expectations
- `loading-to-game-bootstrap-burst.md` — exact server-owned burst emitted when `ENTERGAME` moves the session from `LOADING` to `GAME`
- `character-delete-selection.md` — deterministic character deletion in `SELECT`
- `client-version-loading.md` — tolerant `CLIENT_VERSION` metadata path during `LOADING`
- `game-ping-pong.md` — minimal control-plane `PING`/`PONG` behavior once the session is in `GAME`
- `shared-world-peer-visibility.md` — minimal peer enter/remove visibility across concurrent `gamed` sessions
- `move-peer-fanout.md` — minimal queued `MOVE` replication to already-visible peers
- `sync-position-peer-fanout.md` — minimal queued `SYNC_POSITION` replication to already-visible peers
- `local-chat-peer-fanout.md` — minimal local talking chat fanout to already-visible peers
- `whisper-name-routing.md` — minimal exact-name whisper routing among connected `GAME` sessions
- `party-chat-bootstrap.md` — minimal bootstrap `CHAT_TYPE_PARTY` fanout across connected `GAME` sessions
- `guild-chat-bootstrap.md` — minimal bootstrap `CHAT_TYPE_GUILD` fanout across connected `GAME` sessions
- `shout-chat-bootstrap.md` — minimal bootstrap `CHAT_TYPE_SHOUT` fanout across connected `GAME` sessions
- `info-notice-bootstrap.md` — minimal bootstrap system-chat handling for `CHAT_TYPE_INFO` and `CHAT_TYPE_NOTICE`
- `server-notice-broadcast.md` — first programmatic server-originated `CHAT_TYPE_NOTICE` broadcast contract
- `game-slash-command-bootstrap.md` — first owned `/quit`, `/logout`, `/phase_select`, `/restart_here`, and `/restart_town` slash-command behavior while already in `GAME`
- `chat-scope-first-hardening.md` — first non-global scoping pass for talking/guild/shout using currently available runtime data
- `map-index-world-scope-hardening.md` — first map-index-backed visible-world scoping pass for visibility, movement, sync, and local talking chat
- `world-topology-bootstrap.md` — first explicit local-channel and map-scope topology model for the bootstrap runtime
- `visibility-rebuild.md` — first dedicated visibility helper owned by `internal/worldruntime`
- `entity-runtime-bootstrap.md` — first explicit ownership contract for live player runtime, entity identity, and the next M2 world-runtime directories/indexes
- `map-relocation-visibility-rebuild.md` — first server-side visible-world rebuild primitive for relocating a connected player between bootstrap maps
- `bootstrap-map-transfer-contract.md` — first minimal structured preview/commit contract for bootstrap map transfer on top of the relocation primitive
- `transfer-rebootstrap-burst.md` — first owned self-session rebootstrap burst for bootstrap map transfer on the game socket
- `runtime-reconnect-cleanup.md` — first owned close/disconnect/reconnect cleanup contract for bootstrap shared-world runtime ownership
- `non-player-entity-bootstrap.md` — first owned non-player runtime contract for identity and map presence before packet/content behavior exists
- `static-actor-interaction-bootstrap.md` — first interaction-ready metadata seam for bootstrap static actors, now powering self-only `info` / `talk`, `warp`, and merchant-facing `shop_preview`
- `static-actor-interaction-request.md` — first owned client-originated `GAME`-phase interaction request for a visible bootstrap static actor target by `VID`
- `npc-service-interactions-bootstrap.md` — first frozen service-style NPC gameplay contract built on top of bootstrap static actors and the existing `INTERACT` ingress
- `combat-training-dummy-bootstrap.md` — first frozen combat-preparation target contract around one visible `training_dummy` actor
- `combat-normal-attack-bootstrap.md` — first owned `ATTACK` wire contract, selected-dummy normal-attack gate, deterministic bootstrap HP refresh loop, safe `USE_SKILL`/`SHOOT`/`FLY_TARGETING`/`ON_CLICK`/`CHARACTER_POSITION` ingress guards, and clear-target companion on top of the visible `training_dummy` target slice
- `combat-damage-info-bootstrap.md` — first owned server `DAMAGE_INFO` codec contract plus standalone hit-effect fanout and owner self-only spawn-backed practice-mob hit-effect emission
- `combat-fly-effect-bootstrap.md` — first owned server `FLY_TARGETING`, `ADD_FLY_TARGETING`, and `CREATE_FLY` packet shapes, currently codec/documentation only and deliberately not emitted by the bootstrap runtime yet
- `combat-pvp-duel-bootstrap.md` — first owned server `PVP` and `DUEL_START` packet shapes, currently codec/documentation only and deliberately not emitted by the shipped runtime yet
- `combat-target-marker-bootstrap.md` — first owned server `TARGET_CREATE_NEW`, `TARGET_UPDATE`, and `TARGET_DELETE` marker packet shapes, currently codec/documentation only and deliberately separate from selected-combat-target HP refreshes
- `non-player-death-respawn-bootstrap.md` — first owned zero-HP death, target clear, dead-state rejection, and respawn-reset contract for bootstrap non-player combatants
- `non-player-reward-bootstrap.md` — first non-player death reward seam, default rewardless descriptors, deterministic EXP/gold persistence, and fixed drop-vnum ground-item rewards
- `player-death-bootstrap.md` — first owned retaliation-driven player-death edge for engaged owners, including self `DEAD` + clear, one visibility-gated peer `DEAD(owner_vid)` fanout, merchant-window close on that floor, the first recipient-side exact-name whisper denial for still-connected zero-HP owners, and the first post-floor `MOVE` / `SYNC_POSITION`, static-actor `INTERACT`, merchant-buy, client/slash item-use, packet item-move/give/refine, slash item-inventory/equipment, peer-facing `CHAT` / `WHISPER`, and self-only `CHAT_TYPE_INFO` denial seams
- `player-stun-bootstrap.md` — first owned server `STUN` packet shape, currently codec/documentation only and deliberately not emitted by the bootstrap combat/death runtime yet
- `character-position-change-speed-bootstrap.md` — first owned server `CHARACTER_POSITION` and `CHANGE_SPEED` packet shapes, including presentation-only self/peer `CHARACTER_POSITION(position=0|4)` emission for accepted client stance requests while `CHANGE_SPEED` remains codec/documentation only
- `player-restart-here-bootstrap.md` — same-socket `/restart_here` recovery contract for a retaliation-owned zero-HP player, reusing self bootstrap and peer visibility rebuild packet families for in-place recovery
- `player-restart-town-bootstrap.md` — same-socket `/restart_town` recovery contract for a retaliation-owned zero-HP player, reusing the existing transfer rebootstrap burst plus legacy empire town-return targets without claiming full revive menus yet
- `player-restart-request-bootstrap.md` — restart-ingress follow-up note: current public evidence supports `/restart_here` and `/restart_town` as the owned restart ingress for this compatibility track, and any separate dedicated restart packet stays unowned unless later captures or owned fixtures prove one
- `content-spawn-groups-bootstrap.md` — first authored `spawn_groups` content contract for loading stationary attackable non-player spawns through `combat_profile` + map placement
- `spawn-leash-bootstrap.md` — first pure runtime classification seam for authored spawn home vs current position, capped return-step planning/execution, and read-only pending return-step inspection before broader mob chase/leash/return movement is implemented
- `npc-shop-preview-bootstrap.md` — structured merchant preview/identity contract built on top of the existing `INTERACT` ingress and deterministic interaction-definition authoring
- `npc-shop-catalog-bootstrap.md` — structured merchant catalog contract behind `shop_preview`, with stable catalog slots and template-backed entries
- `npc-shop-open-close-bootstrap.md` — first client-visible merchant window contract, keeping merchant open anchored to `INTERACT` while freezing the current bootstrap `SHOP START` / `BUY` / `END` behavior
- `npc-shop-transaction-bootstrap.md` — first buy-only merchant transaction gate built on structured `shop_preview` catalogs while keeping full shop-window choreography capture-gated
- `quest-state-bootstrap.md` — first deterministic quest-flag persistence and compare-and-set transition seam, not yet wired to client packets, NPC dialogs, or rewards
- `static-actor-interaction-authoring.md` — first loopback-only authoring, QA visibility, and deterministic bundle surface for bootstrap static actors plus interaction definitions
- `exact-position-bootstrap-transfer-trigger.md` — first gameplay-side exact-position trigger that can invoke the bootstrap transfer commit path from `MOVE` / `SYNC_POSITION`
- `character-update-bootstrap.md` — first self-only state refresh after entering `GAME`
- `player-point-change-bootstrap.md` — first self-only point refresh after entering `GAME`
- `inventory-equipment-bootstrap.md` — first owned self-only inventory/equipment bootstrap surface and item-family boundary for M3 character state
- `item-template-store-bootstrap.md` — current authored item-template snapshot boundary, validation rules, missing-file fallback, and fail-closed unknown-field hardening
- `item-use-bootstrap.md` — first owned self-only consumable item-use bootstrap contract, now covering both the slash harness and the first tiny client-originated `ITEM_USE` ingress for carried inventory, plus the first carried-stack `ITEM_USE_TO_ITEM` drag-to-item consolidation runtime path
- `item-use-special-effect-bootstrap.md` — first template-authored self-only `SPECIAL_EFFECT` companion for accepted direct carried consumable use
- `item-move-bootstrap.md` — first owned client-originated carried-inventory `ITEM_MOVE` contract for GAME-phase moves, counted splits/merges, zero-count and exact-count incompatible occupied-destination swaps, and self-only slot refreshes
- `item-drop-pickup-bootstrap.md` — frozen ground-item drop/pickup packet codecs and `GAME` dispatch seams, now with carried-slot drop mutation, shared temporary ground handles, visible-world pickup, and the first party-shaped owner-delivery pickup notices while durable ownership timers and real party membership remain pending
- `item-give-bootstrap.md` — first owned `ITEM_GIVE` client codec and `GAME` dispatch seam, intentionally fail-closed in the shipped runtime until player/NPC transfer semantics are owned, with visible-target-gated template-authored `anti_give` feedback and active exchange-shell teardown before that feedback
- `item-exchange-bootstrap.md` — first owned `EXCHANGE` client codec, shared server response codec, `GAME` dispatch seam, visible-peer `START` / `CANCEL` exchange-window shell, display-only `GC::EXCHANGE ITEM_ADD` frames with template-authored sockets/attributes, display-only `GC::EXCHANGE ITEM_DEL` slot clears, display-only gold/accept markers with accept reset on later display changes, same-socket `/quit` / `/logout` / `/phase_select` plus successful carried-item-use / carried item-move / slash inventory-move / drag-to-item stack-consolidation / drop exchange teardown, and template-authored `anti_give` / `give_reject_message` self-only `ITEM_ADD` guard while finalization remains deferred
- `item-refine-bootstrap.md` — first owned `REFINE` client codec and `GAME` dispatch seam, intentionally fail-closed in the shipped runtime until refine result semantics are owned
- `item-storage-guard-bootstrap.md` — first owned safebox/mall client request codecs (`SAFEBOX_CHECKIN`, `SAFEBOX_CHECKOUT`, `SAFEBOX_ITEM_MOVE`, `MALL_CHECKOUT`), first server response codecs (`SAFEBOX_SET` / `DEL` / status, `MALL_OPEN` / `SET` / `DEL`), `GAME` dispatch seam, and template-authored `anti_safebox` rejection feedback with active merchant/exchange presentation teardown, while accepted storage semantics remain intentionally fail-closed
- `quickslot-bootstrap.md` — first owned quickslot packet codecs, selected-character bootstrap replay, and accepted self-only client quickslot edit persistence (`QUICKSLOT_ADD`, `QUICKSLOT_DEL`, `QUICKSLOT_SWAP`) before automatic item-mutation quickslot synchronization
- `sync-position-bootstrap.md` — first self-only sync-position reconciliation after entering `GAME`
- `boot-path.md` — first milestone from connect to basic movement
- `packet-matrix.md` — working inventory for the first protocol slice

## What belongs here

- boot-path packet inventory
- session phase matrix
- frame layout notes
- capture-derived golden fixtures
- compatibility assumptions and rejected assumptions

Reference implementations may be studied externally, but this repository must only store original documentation and code.
