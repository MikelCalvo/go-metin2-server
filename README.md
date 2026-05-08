# go-metin2-server

Clean-room Metin2 server emulator in Go, targeting TMP4-era client compatibility.

This repository is a public rewrite built around owned protocol docs, small vertical slices, and a gradual path from a stable boot flow to a real shared-world runtime.

## Status

`go-metin2-server` is still *pre-alpha*, but it already has a real playable bootstrap surface.

Current snapshot:
- [x] Secure legacy handshake, login, character selection, and enter-game flow
- [x] First shared-world bootstrap: peer visibility, movement replication, local chat, whisper, party/guild/shout fanout, and server notices
- [x] First content/runtime seams: static actors, interactions, shop open/buy bootstrap, and warp bootstrap
- [~] First character systems: inventory, equipment, consumable use, and appearance refreshes
- [~] First combat loop: target selection, owned `ATTACK` ingress, a fixed same-target `250ms` normal-attack cadence gate, authored `training_dummy` combat-profile HP refreshes, visible zero-HP death/clear, and a timed respawn rebuild are live
- [~] First content-loaded combatant: `spawn_groups` can now materialize one stationary practice mob through an authored `combat_profile`; its first post-hit aggro-lite gate blocks fresh third-party `TARGET` attempts while the engaged owner still lives, but retaliation-driven owner death now releases that same still-live mob again without waiting for mob death / respawn or owner disconnect. Accepted owner-side hits also carry a deterministic immediate self-only HP retaliation tick, and the same engagement now owns both a fixed same-target `250ms` normal-attack cadence gate and a one-beat-at-a-time delayed self-only server-origin follow-up cadence that clamps retaliation point-loss at `0` HP, keeps that retaliation loss runtime-only for now so fresh `/phase_select` re-entry or reconnect still rebuild from persisted point state while a still-live practice mob keeps its current runtime-owned HP, cancels any pending delayed follow-up beat and releases that same still-live mob again on same-socket `/quit` or `/logout`, emits one self-only `DEAD(owner_vid)` plus target clear at that floor, queues one visible-peer `DEAD(owner_vid)` for currently visible sessions there, fail-closes later owner combat `TARGET` / `ATTACK`, owner `MOVE` / `SYNC_POSITION`, owner static-actor `INTERACT`, owner merchant-buy attempts, owner slash `/use_item` attempts, owner slash `/inventory_move` attempts, owner slash `/equip_item` / `/unequip_item` attempts, owner peer-facing `CHAT` / `WHISPER` attempts, and owner self-only `CHAT_TYPE_INFO` attempts there, while broader mob/AI and player-death systems stay out of scope
- [ ] Still missing: broader combat systems, richer mobs/AI, rewards/loot, compatibility-grade persistence, and production deployment

The first authored `combat_profile` seam now survives shared-world registration, static-actor snapshot persistence, and content-bundle import/export, so the `training_dummy` loop is no longer wired only through bootstrap-only runtime metadata.

The first authored `spawn_groups` seam is now live too: content-bundle import/export can round-trip one stationary practice mob whose runtime death/respawn loop is owned by the existing `training_dummy` combat profile, whose first accepted hit establishes a tiny aggro-lite gate against fresh third-party `TARGET` attempts while the engaged owner still lives, whose retaliation-driven owner death now releases that same still-live mob again without waiting for mob death / respawn or owner disconnect, whose accepted owner-side live hits now also append a deterministic immediate self-only HP retaliation tick, and whose same live selected-target engagement now owns both a fixed same-target `250ms` normal-attack cadence gate and a one-beat-at-a-time delayed self-only server-origin follow-up cadence that clamps retaliation point-loss at `0` HP, keeps that retaliation loss runtime-only for now so fresh `/phase_select` re-entry or reconnect still rebuild from persisted point state while a still-live practice mob keeps its current runtime-owned HP, cancels any pending delayed follow-up beat and releases that same still-live mob again on same-socket `/quit` or `/logout`, emits one self-only `DEAD(owner_vid)` plus target clear at that floor, queues one visible-peer `DEAD(owner_vid)` for currently visible sessions there, and fail-closes later owner combat `TARGET` / `ATTACK`, owner `MOVE` / `SYNC_POSITION`, owner static-actor `INTERACT`, owner merchant-buy attempts, owner slash `/use_item` attempts, owner slash `/inventory_move` attempts, owner slash `/equip_item` / `/unequip_item` attempts, owner peer-facing `CHAT` / `WHISPER` attempts, and owner self-only `CHAT_TYPE_INFO` attempts there while that engagement remains live.

If that same owner already had a merchant preview window open when retaliation reaches `0` HP, the owned floor transition now also tears that window down with one self-only `GC::SHOP END` after the existing self `DEAD` + target-clear transition.

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
| M4 — Combat vertical slice | [~] | The repo now owns the first end-to-end `training_dummy` loop: target selection, `ATTACK` ingress, a fixed same-target `250ms` cadence gate, deterministic authored combat-profile HP refreshes, visible zero-HP death/clear, and a timed respawn rebuild. |
| M5 — Content runtime | [~] | NPCs, mobs, spawn groups, shops, and the first quest/script runtime become available; `spawn_groups` + `combat_profile` can already load one stationary practice mob, whose first accepted hit now owns a tiny aggro-lite target gate while the engaged owner still lives, whose retaliation-driven owner death now releases that same still-live mob again, an immediate self-only retaliation tick on accepted live hits, the same-target `250ms` attack cadence gate, and one sustained delayed self-only server-origin follow-up cadence at a time with retaliation point-loss clamped at `0` HP, that retaliation loss staying runtime-only for now so fresh `/phase_select` re-entry or reconnect still rebuild from persisted point state while the same live practice mob keeps its current runtime-owned HP, same-socket `/quit` or `/logout` also cancelling any pending delayed follow-up beat and releasing that same still-live mob again right away, one self-only `DEAD(owner_vid)` plus stale-target clear there, one visibility-gated peer `DEAD(owner_vid)` fanout there, an already-open merchant window closing on the same retaliation floor with self-only `SHOP END`, and later owner combat `TARGET` / `ATTACK`, owner `MOVE` / `SYNC_POSITION`, owner static-actor `INTERACT`, owner merchant-buy attempts, owner slash `/use_item`, `/inventory_move`, `/equip_item`, and `/unequip_item` attempts, owner peer-facing `CHAT` / `WHISPER` attempts, and owner self-only `CHAT_TYPE_INFO` attempts fail-closed there while broader authored content runtime is still pending. |
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
