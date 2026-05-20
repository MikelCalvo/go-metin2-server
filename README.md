# go-metin2-server

Clean-room Metin2 server emulator in Go, targeting TMP4-era client compatibility.

This repository is a public rewrite built around owned protocol docs, small vertical slices, and a gradual path from a stable boot flow to a real shared-world runtime.

## Status

`go-metin2-server` is still *pre-alpha*, but it already has a real playable bootstrap surface.

Current snapshot:
- [x] Secure legacy handshake, login, character selection, and enter-game flow
- [x] First shared-world bootstrap: peer visibility, movement replication, local chat, whisper, party/guild/shout fanout, and server notices
- [x] First content/runtime seams: static actors, interactions, shop open/buy/sell bootstrap, and warp bootstrap
- [~] First character systems: inventory, equipment, consumable use, and appearance refreshes
- [~] First combat loop: target selection, owned `ATTACK` ingress, a fixed same-target `250ms` normal-attack cadence gate, authored `training_dummy` combat-profile HP refreshes, visible zero-HP death/clear, and a timed respawn rebuild are live
- [~] First content-loaded combatant: `spawn_groups` can now materialize one stationary practice mob through an authored `combat_profile`; its first post-hit aggro-lite gate blocks fresh third-party `TARGET` attempts while the engaged owner still lives, and that same first authoritative hit now also invalidates any other session's stale preselected shared-world target ownership for that mob, queues one self-only `GC TARGET(0, 0)` stale-selection clear to those affected third parties, and prevents later third-party `ATTACK` from bypassing the gate, while retaliation-driven owner death still releases that same still-live mob again without waiting for mob death / respawn or owner disconnect. Accepted owner-side hits also carry a deterministic immediate self-only HP retaliation tick, and the same engagement now owns both a fixed same-target `250ms` normal-attack cadence gate and a one-beat-at-a-time delayed self-only server-origin follow-up cadence that clamps retaliation point-loss at `0` HP, keeps that retaliation loss runtime-only for now so fresh `/phase_select` re-entry or reconnect still rebuild from persisted point state while a still-live practice mob keeps its current runtime-owned HP, and later position-only `MOVE` / `SYNC_POSITION` / transfer rebootstrap saves plus successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves plus non-point-bearing slash `/inventory_move` / merchant-buy saves still persist their authored use/equip-effect point delta, consumed or carried/equipped item state, coordinates, carried-slot state, or purchased item/gold state without leaking that retaliation loss into account state, cancels any pending delayed follow-up beat and releases that same still-live mob again on successful transfer / rebootstrap too, on same-socket `/quit`, `/logout`, `/phase_select`, or on abrupt session close, also releases that abandoned still-live mob again when movement / sync clears target intent or a fresh `TARGET` retargets another visible practice mob, emits one self-only `DEAD(owner_vid)` plus target clear at that floor, queues one visible-peer `DEAD(owner_vid)` for currently visible sessions there, fail-closes later owner combat `TARGET` / `ATTACK`, owner `MOVE` / `SYNC_POSITION`, owner static-actor `INTERACT`, owner merchant-buy attempts, owner slash `/use_item` and carried-slot `ITEM_USE` attempts, owner slash `/inventory_move` attempts, owner slash `/equip_item` / `/unequip_item` attempts, owner peer-facing `CHAT` / `WHISPER` attempts, and owner self-only `CHAT_TYPE_INFO` attempts there, while broader mob/AI and player-death systems stay out of scope
- [ ] Still missing: broader combat systems, richer mobs/AI, rewards/loot, compatibility-grade persistence, and production deployment

The first authored `combat_profile` seam now survives shared-world registration, static-actor snapshot persistence, and content-bundle import/export, so the `training_dummy` loop is no longer wired only through bootstrap-only runtime metadata.

The first authored `spawn_groups` seam is now live too: content-bundle import/export can round-trip one stationary practice mob whose runtime death/respawn loop is owned by the existing `training_dummy` combat profile, whose first accepted hit establishes a tiny aggro-lite gate against fresh third-party `TARGET` attempts while the engaged owner still lives, now also invalidates any other session's stale preselected shared-world target ownership for that same mob, queues one self-only `GC TARGET(0, 0)` stale-selection clear to those affected third parties, and prevents later third-party `ATTACK` from bypassing that gate, while retaliation-driven owner death still releases that same still-live mob again without waiting for mob death / respawn or owner disconnect, whose accepted owner-side live hits now also append a deterministic immediate self-only HP retaliation tick, and whose same live selected-target engagement now owns both a fixed same-target `250ms` normal-attack cadence gate and a one-beat-at-a-time delayed self-only server-origin follow-up cadence that clamps retaliation point-loss at `0` HP, keeps that retaliation loss runtime-only for now so fresh `/phase_select` re-entry or reconnect still rebuild from persisted point state while a still-live practice mob keeps its current runtime-owned HP, and later position-only `MOVE` / `SYNC_POSITION` / transfer rebootstrap saves plus successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves plus non-point-bearing slash `/inventory_move` / merchant-buy saves still persist their authored use/equip-effect point delta, consumed or carried/equipped item state, coordinates, carried-slot state, or purchased item/gold state without leaking that retaliation loss into account state, cancels any pending delayed follow-up beat and releases that same still-live mob again on successful transfer / rebootstrap too, on same-socket `/quit`, `/logout`, `/phase_select`, or on abrupt session close, also releases that abandoned still-live mob again when movement / sync clears target intent or a fresh `TARGET` retargets another visible practice mob, emits one self-only `DEAD(owner_vid)` plus target clear at that floor, queues one visible-peer `DEAD(owner_vid)` for currently visible sessions there, and fail-closes later owner combat `TARGET` / `ATTACK`, owner `MOVE` / `SYNC_POSITION`, owner static-actor `INTERACT`, owner merchant-buy attempts, owner slash `/use_item` and carried-slot `ITEM_USE` attempts, owner slash `/inventory_move` attempts, owner slash `/equip_item` / `/unequip_item` attempts, owner peer-facing `CHAT` / `WHISPER` attempts, and owner self-only `CHAT_TYPE_INFO` attempts there while that engagement remains live.

If that same owner already had a merchant preview window open when retaliation reaches `0` HP, the owned floor transition now also tears that window down with one self-only `GC::SHOP END` after the existing self `DEAD` + target-clear transition.

That same retaliation-driven `0`-HP floor now also owns one same-socket `/restart_here` recovery seam: while the session stays in `GAME`, the dead owner can rebuild in place from the persisted account snapshot, visible live peers see one delete-plus-rebootstrap refresh for that owner, the old selected practice-mob target still stays cleared until a fresh `TARGET`, and the still-live practice mob keeps its current runtime-owned HP instead of resetting because of the owner's recovery.

That same zero-HP floor now also owns one same-socket `/restart_town` recovery seam: while the session stays in `GAME`, the dead owner can rebuild from the persisted account snapshot at the owned legacy empire town-return position, persist that town-return coordinate through the existing transfer ordering, reuse the ordinary self transfer rebootstrap burst plus transfer visibility deltas on the same socket, and still leave the old selected practice-mob target cleared until a fresh `TARGET`.

Those two recovery seams remain slash-command-backed today. Current public evidence now supports keeping `/restart_here` and `/restart_town` as the owned restart ingress for this compatibility track; a separate dedicated restart packet stays unowned unless later captures or owned fixtures prove one with exact bytes.

That same retaliation-driven `0`-HP floor now also makes the still-connected owner temporarily unavailable as a whisper recipient: peer-originated exact-name whispers fail closed with no queued target delivery and no synthetic `WHISPER_TYPE_NOT_EXIST` fallback until broader recipient-side player-death policy is owned.

The same floor now also removes that still-connected owner from later peer-originated `CHAT_TYPE_TALKING`, `CHAT_TYPE_PARTY`, `CHAT_TYPE_GUILD`, and `CHAT_TYPE_SHOUT` recipient fanout: the live sender still keeps the ordinary self echo, but the zero-HP owner receives no queued peer-chat delivery.

That same retaliation-driven `0`-HP floor now also removes that still-connected owner from later server-originated `CHAT_TYPE_NOTICE` broadcasts: other connected live sessions still receive the queued notice, but the zero-HP owner is skipped silently until broader player-death recipient policy is owned.

The same zero-HP recipient rule now also applies to later visibility-gated peer-death fanout: if another visible player reaches that same retaliation-owned floor later, the already-dead still-connected owner receives no queued peer `GC DEAD(other_vid)` frame.

That same retaliation-driven `0`-HP floor now also removes the still-connected dead owner from later peer-entry visibility delivery: other live recipients still receive the newcomer’s ordinary `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` burst on fresh joins, live movers/syncing peers still receive their ordinary origin-side peer-entry burst when crossing into visibility later, and transferred live peers still receive their ordinary origin-side peer-entry burst when relocated into visibility later, but the zero-HP owner receives no queued peer-entry frames from those current join / `MOVE` / `SYNC_POSITION` / transfer visibility-entry paths.

When a fresh `ENTERGAME` bootstrap later brings a newcomer into visibility of that same still-connected dead owner, the newcomer now also receives one trailing `GC DEAD(owner_vid)` right after the ordinary peer-entry burst for the already-dead owner so that late visibility bootstrap does not silently present that owner as live.

The same dead-state replay now also applies to later `MOVE`, `SYNC_POSITION`, and transfer visibility-entry rebuilds: whichever live peer is newly paired with that already-dead owner still gets the ordinary peer-entry burst first, then one trailing `GC DEAD(owner_vid)` so neither later AOI re-entry nor operator/runtime transfer relocation silently re-presents that owner as live.

If that already-dead owner itself is later relocated through the current loopback `/local/transfer` path into another live peer’s visible world or into visibility of another static actor, live peers still get the ordinary dead-owner replay described above, but the dead owner now skips both queued destination peer-entry bursts and destination static-actor bootstrap bursts and keeps only any old-world cleanup frames still needed locally.

That same zero-HP recipient rule now also applies to later same-visible-set peer movement replication: live movers and syncers still keep their ordinary self `MOVE_ACK` / `SYNC_POSITION_ACK`, other live viewers still get the ordinary queued peer `MOVE_ACK` / `SYNC_POSITION_ACK`, but the still-connected dead owner is skipped from those later queued stable peer movement frames entirely.

That same queued peer-death rule also stays in force after the first owner floor: if another still-visible live player later reaches that same retaliation-owned `0`-HP floor, other live viewers still receive the ordinary queued `GC DEAD(other_vid)` fanout while the already-dead connected owner is skipped from that later peer-visible death frame entirely.

That same zero-HP recipient rule now also applies to later peer-visibility teardown on disconnect, stale-ownership reclaim cleanup, relocate-away transfer, and AOI move/sync exit: the leaving, reclaimed, moving, syncing, or transferred live peer still gets its ordinary cleanup or replacement re-entry behavior, but the still-connected dead owner is skipped from those later queued peer `CHARACTER_DEL` frames entirely.

That same retaliation-driven `0`-HP recipient rule now also applies to later visible practice-mob death / respawn lifecycle fanout: other live viewers still receive the ordinary `GC DEAD(mob_vid)` plus timed respawn rebuild burst, but an already-dead still-connected owner is skipped from those later non-player lifecycle frames entirely.

If a client is shown a still-dead practice mob again before its fixed respawn delay expires — via fresh bootstrap, later visibility re-entry, or a retained delete-plus-rebootstrap refresh — it now receives the ordinary actor add/info/update burst immediately followed by one replayed `GC DEAD(mob_vid)` so that the mob does not resurrect visually.

The local runtime/operator snapshots that describe those same visible practice mobs now also expose `dead: true` during that owned dead interval, so relocate-preview, transfer, visibility, map-occupancy, and static-actor inspection no longer have to infer dead state only from packet replay.

That same loopback snapshot family now also exposes `dead: true` on still-connected zero-HP player entries during the current retaliation-owned death interval, whether that player appears as the main `character` / `target`, as a visible peer, or inside `/local/players`, `/local/visibility`, `/local/interaction-visibility`, and `/local/maps` character arrays.

That same zero-HP recipient rule now also applies to later live static-actor visibility delivery: later static-actor register / update / remove fanout still reaches other live viewers through the ordinary add / refresh / delete paths, but the still-connected dead owner is skipped from those queued visibility frames entirely.

If operator/runtime mutation removes a currently selected live practice mob outright, the affected selected session now also receives the ordinary actor `CHARACTER_DEL` plus one self-only `GC TARGET(0, 0)` so stale combat-target ownership does not survive runtime removal.

The first consumable item-use vertical is now no longer slash-only: the bootstrap runtime also owns one tiny carried-slot `ITEM_USE` client packet ingress that reuses the same template-backed self-only point/item/info response path as `/use_item <slot>`. The merchant surface now also has a first live `SELL` / `SELL2` sell-back path: it can remove or decrement carried inventory stacks, derive ordinary and count-per-gold sell value from item-template `shop_buy_price` / `sell_count_per_gold` with the owned legacy `/5` plus 3% tax floors, credit live gold, persist the selected-character snapshot, and now return the self-only `ITEM_DEL` / `ITEM_SET` mutation followed by `PLAYER_POINT_CHANGE(POINT_GOLD)` and bare `GC::SHOP OK` success companion while anti-sell and runtime-locked item guards fail closed without mutation and richer sell UI choreography remains future work; the server `SHOP` family also now owns codec-level `UPDATE_ITEM` and `UPDATE_PRICE` refresh shapes plus a one-page `SHOP_HOST_ITEM_MAX = 40` structured catalog cap for later stock/player-shop slices without emitting those refreshes from the current bootstrap NPC buy/sell paths. The packet `SHOP BUY` path is now a little closer to the live merchant family too: successful packet buys return self-only `ITEM_SET` refreshes for changed carried slots without an extra bare `GC::SHOP OK`, insufficient-gold, no-valid-placement, and unknown-slot packet buys return bare self-only `GC::SHOP NOT_ENOUGH_MONEY` / `GC::SHOP INVENTORY_FULL` / `GC::SHOP INVALID_POS`, a previously opened merchant window whose live actor or authored `shop_preview` definition changed underneath that session now auto-closes on the next packet `SHOP BUY` with one self-only `GC::SHOP END`, a position-only `MOVE` or `SYNC_POSITION` that leaves that bound merchant actor outside the current interaction/visibility gate now queues one self-only `GC::SHOP END` after the normal self movement acknowledgement, a successful warp or exact-position transfer while that merchant window is still open now prepends one self-only `GC::SHOP END` before the self transfer rebootstrap burst, a content-loaded practice mob's delayed retaliation beat that drops the selected character to the `0`-HP floor while that merchant window is still open now appends one self-only `GC::SHOP END` after the owned point/dead/target-clear sequence, same-socket `/phase_select` while that merchant window is still open now prepends one self-only `GC::SHOP END` before the select-phase transition frame, same-socket `/quit` or `/logout` while that merchant window is still open now prepends one self-only `GC::SHOP END` before the command or close-phase teardown frame, and the temporary local `/shop_buy <slot>` debug harness now reuses that same merchant-family success / insufficient-gold / no-valid-placement / unknown-slot surface instead of keeping a second placeholder or silent unknown-slot path. The client `SHOP SELL` / `SELL2` byte shapes are owned as codecs and runtime ingress.

The README stays intentionally high-level. If you want the deeper technical view, start here:
- [Project assessment](docs/roadmaps/2026-04-18-global-project-assessment.md)
- [Protocol index](spec/protocol/README.md)
- [Detailed plans / slice roadmaps](docs/plans/)

## Milestone ladder

| Milestone | Status | Focus |
| --- | --- | --- |
| M0 — Protocol-owned boot path | [x] | Handshake, auth/login, selection, enter-game, and first movement loop are stable. |
| M1 — Shared-world pre-alpha | [~] | Players can already see each other, move, chat, and receive notices inside the current bootstrap world rules. |
| M2 — Entity/world runtime foundation | [~] | Entities, maps, sessions, transfers, and static actors are moving out of bootstrap-only shortcuts into owned runtime systems. |
| M3 — Character systems | [~] | Inventory, equipment, item use, appearance, and merchant-driven item state are becoming first-class runtime systems. |
| M4 — Combat vertical slice | [~] | The repo now owns the first end-to-end `training_dummy` loop: target selection, `ATTACK` ingress, a fixed same-target `250ms` cadence gate, deterministic authored combat-profile HP refreshes, visible zero-HP death/clear, a timed respawn rebuild, proactive selected-target clears when operator/runtime replacement or removal invalidates the currently selected dummy snapshot, and practice-mob update resets that old life's aggro-lite ownership as part of the same reset boundary. |
| M5 — Content runtime | [~] | NPCs, mobs, spawn groups, shops, and the first quest/script runtime become available; `spawn_groups` + `combat_profile` can already load one stationary practice mob, whose first accepted hit now owns a tiny aggro-lite target gate while the engaged owner still lives, proactively clears stale preselected third-party targets there with one self-only `GC TARGET(0, 0)`, whose retaliation-driven owner death now releases that same still-live mob again, an immediate self-only retaliation tick on accepted live hits, the same-target `250ms` attack cadence gate, and one sustained delayed self-only server-origin follow-up cadence at a time with retaliation point-loss clamped at `0` HP, that retaliation loss staying runtime-only for now so fresh `/phase_select` re-entry or reconnect still rebuild from persisted point state while the same live practice mob keeps its current runtime-owned HP, with later position-only `MOVE` / `SYNC_POSITION` / transfer rebootstrap saves plus successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves plus non-point-bearing slash `/inventory_move` / merchant-buy saves still persisting their authored use/equip-effect point delta, consumed or carried/equipped item state, coordinates, carried-slot state, or purchased item/gold state without leaking that retaliation loss into account state, successful transfer / rebootstrap plus same-socket `/quit`, `/logout`, `/phase_select`, or abrupt session close also cancelling any pending delayed follow-up beat and releasing that same still-live mob again right away, with movement / sync target clear or a fresh `TARGET` retarget to another visible practice mob also releasing that abandoned still-live mob immediately, one self-only `DEAD(owner_vid)` plus stale-target clear there, one visibility-gated peer `DEAD(owner_vid)` fanout there, an already-open merchant window closing on the same retaliation floor with self-only `SHOP END`, and later owner combat `TARGET` / `ATTACK`, owner `MOVE` / `SYNC_POSITION`, owner static-actor `INTERACT`, owner merchant-buy attempts, owner slash `/use_item` and carried-slot `ITEM_USE`, `/inventory_move`, `/equip_item`, and `/unequip_item` attempts, owner peer-facing `CHAT` / `WHISPER` attempts, and owner self-only `CHAT_TYPE_INFO` attempts fail-closed there while broader authored content runtime is still pending. |
| M6 — Compatibility-grade persistence and operations | [ ] | DB-backed persistence, richer observability/admin tooling, and a real deploy/release story land. |

## What’s in the repo

- `cmd/authd` / `cmd/gamed` — daemon entrypoints
- `internal/boot`, `internal/handshake`, `internal/login` — connection and boot-path flow
- `internal/worldruntime` — topology, visibility, maps, entities, and session routing
- `internal/player`, `internal/inventory`, `internal/itemstore` — early character and item systems
- `internal/staticstore`, `internal/interactionstore` — static actors, authored combat-profile metadata, and interaction content
- `internal/proto/*` — packet codecs and wire-level slices
- `docs/` — engineering notes, testing, QA, workflow, and roadmaps
- `spec/protocol/` — owned protocol docs and packet inventory

## Documentation

- [Development guide](docs/development.md) — local commands, Docker, runtime addresses, and config knobs
- [Debugging and profiling](docs/debugging-and-profiling.md) — pprof, local operator endpoints, and examples
- [Manual client QA checklist](docs/qa/manual-client-checklist.md) — smoke-test reference for a real client, including the current combat ownership/death/respawn bundle
- [Protocol document index](spec/protocol/README.md) — packet docs and wire-level notes
- [Project assessment](docs/roadmaps/2026-04-18-global-project-assessment.md) — deeper state-of-project write-up
- [Plans directory](docs/plans/) — implementation roadmaps and next slices
- [Testing strategy](docs/testing-strategy.md)
- [Workflow](docs/workflow.md)
- [Clean-room policy](docs/clean-room-policy.md)

## Development

Run the main checks:

```bash
make test
make build
```

Run the daemons locally:

```bash
go run ./cmd/authd
go run ./cmd/gamed
```

For network defaults, advertised/public host settings, local operator endpoints, and profiling examples, see:
- [docs/development.md](docs/development.md)
- [docs/debugging-and-profiling.md](docs/debugging-and-profiling.md)

## Clean-room rule

This repository must only contain code, documentation and fixtures produced for this project.
Do not copy legacy Metin2 server/client source into this repository.
