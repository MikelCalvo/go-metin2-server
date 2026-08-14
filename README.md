# go-metin2-server

Clean-room Metin2 server emulator in Go, targeting TMP4-era client compatibility.

This project is a public rewrite built around project-owned protocol notes, small verified slices, and a gradual path from a stable boot flow to a real shared-world game server. Legacy trees and captures may be used only as external behavior oracles; this repository must not copy legacy source code.

## Current status

`go-metin2-server` is **pre-alpha**. It is not a playable legacy-compatible server yet, but it is well past the packet-experiment stage.

The current `main` branch owns:

- real `authd` and `gamed` daemon entrypoints,
- secure legacy handshake, auth/login, character selection, loading, and game-entry flows,
- a shared in-process world runtime with player visibility, movement, chat, transfer, reconnect, and static/non-player actor seams,
- broad bootstrap inventory, equipment, quickslot, item-use, ground-item, shop, exchange/refine/storage fail-closed, and reward slices,
- authored static actors, interactions, merchant catalogs, content bundles, spawn groups, and stationary practice-mob profiles,
- first combat/death/respawn/restart/reward behavior around practice mobs,
- loopback-only debug/operator endpoints for runtime inspection, content/state validation, quest-state validation, backup/restore preflights, and local QA,
- a migration CLI for catalog summaries, offline ledger-snapshot plans, and explicit CLI-only apply runs,
- GitHub Actions CI for formatting, tests, vet, binary builds, and Docker image builds.

Latest repository scan for this refresh:

- Go version: `1.26`
- Go packages: `38`
- Go files: `133`
- Go test files: `78`
- Markdown docs after this refresh: `130`
- protocol docs under `spec/protocol`: `76`
- current refreshed baseline: `3abdb852 feat: add item exchange fail-closed guard`

Legend used below:

- `[x]` implemented enough for the current milestone
- `[~]` partial / bootstrap / intentionally narrow
- `[ ]` not started or not compatibility-grade yet

## Milestone ladder

- `[x]` **M0 — Protocol-owned boot path**
  - Frame handling, phases, secure handshake, auth/login, selection, loading, game entry, and early bootstrap packets are owned by Go code, docs, and tests.

- `[~]` **M1 — Shared-world pre-alpha**
  - Multiple connected sessions can exist in the same in-process world, see each other, move, chat, transfer through bootstrap seams, reconnect, and rebuild visibility. This is still a single-process bootstrap runtime, not a production channel/shard architecture.

- `[~]` **M2 — Character, inventory, equipment, and economy bootstrap**
  - Inventory/equipment replay, item move/split/merge/use/drop/pickup, quickslots, merchant buy/sell, gold mutation, a first exchange open/cancel shell, refine fail-closed path, authored item-template guards, and persistence validation exist. Accepted trade finalization, storage, item sockets/bonuses, full restrictions, refine success, ownership timers, and DB-backed item persistence remain future work.

- `[~]` **M3 — Content and NPC authoring seam**
  - Static actors, interaction definitions, `info`/`talk`/`warp`/`shop_preview`, merchant catalogs, content bundle import/export, portable combat profiles, reward descriptors, and spawn groups can drive current bootstrap content. This is useful content infrastructure, not a quest scripting system yet.

- `[~]` **M4 — PvE practice loop**
  - Practice mobs can be targeted, attacked, killed, respawned, and can grant deterministic EXP/gold/fixed-drop descriptors through narrow owned contracts. Retaliation/player-death/restart seams, the spawn-position leash classifier, a pure capped return-step planner, a loopback-only one-step return trigger, a pending-frame return-step executor, and local pending return-step inspection exist, with authored spawn home preserved separately from current placement for spawn-backed actors. Real autonomous mob AI movement, chase/return packet choreography, attack formulas, skills, projectile/ranged combat, loot tables, PvP/duels, and full death/revive/corpse choreography remain future work.

- `[~]` **M5 — Operations and developer workflow**
  - The repo has a Makefile, Dockerfile, CI, pprof/debug mux, health endpoint, local-only runtime/config/player/map/visibility/content/persistence endpoints, backup/restore preflights, crash-temp cleanup primitives, a validated migration catalog with dry-run planning, a migration CLI with explicit apply support, QA docs, and clean-room workflow docs. Release/versioning policy, production DB engine/driver selection, production deployment, metrics/logging policy, and production-safe admin tooling are still pending.

- `[ ]` **M6 — Legacy parity / production server**
  - The project does not claim full legacy parity. The next target is a narrow playable vertical; broad parity and production operations come later.

## Subsystem status

### Foundation and workflow

Status: `[x]` strong for a pre-alpha repo.

Already present:

- Go module with daemon entrypoints in `cmd/authd` and `cmd/gamed`.
- Clear `internal/*` package boundaries for protocol, flows, stores, runtime, and ops.
- Makefile and CI for format/test/vet/build/image validation.
- Clean-room policy, testing strategy, workflow, development, debugging, and manual QA docs.
- Lane-based development model with integration through a green `main` branch.

Still missing:

- release/versioning policy,
- production deployment guide,
- contribution/issue taxonomy,
- public release artifacts and migration/runbook maturity.

### Protocol and boot path

Status: `[x]` for the current milestone, `[~]` for full legacy coverage.

Already present:

- frame envelope and session phase model,
- control handshake, phase, ping/pong, and key exchange,
- auth/login/select/loading/game-entry choreography,
- character delete/select/bootstrap updates,
- packet families used by current movement, chat, item, shop, interaction, combat, world, restart, and content slices,
- maintained protocol index and packet matrix.

Still missing:

- many packet families outside the current verticals,
- stronger evidence for uncertain real-client behaviors,
- skill, quest, party/guild, messenger, trade/storage, player-shop, GM/admin, and broader world-event ownership.

### Shared world, visibility, maps, and actors

Status: `[~]` real in-process runtime, not production world architecture.

Already present:

- connected session registry, player directory, map index, topology, and visibility scopes,
- whole-map and radius-style visibility policy support,
- movement/sync/local-chat peer fanout,
- transfer/rebootstrap, reconnect, quit/logout cleanup, and visibility rebuild helpers,
- static/non-player actor directories, runtime snapshots, spawn-group read models, map occupancy and local QA endpoints,
- fail-closed identity validation and stale-index repair/suppression paths for several player/static actor edges.

Still missing:

- production channel/shard ownership,
- richer sectors and long-running resource policy,
- robust multi-map content lifecycle,
- world-state persistence and crash recovery,
- real mob movement/AI lifecycle beyond stationary practice seams.

### Character, inventory, equipment, items, and economy

Status: `[~]` broad bootstrap coverage with many legacy details pending.

Already present:

- inventory/equipment bootstrap replay and self-only item refreshes,
- item move/swap/split/merge, consumable use with optional template-authored self-only `SPECIAL_EFFECT`, drag-to-item stack merge, drop/pickup, merchant buy/sell, gold mutation, and quickslot persistence,
- authored item-template metadata for selected display/guard behavior, including template-backed refine-dialog preview metadata that now stays fail-closed when selected-character restrictions or transfer guards disallow the carried item, plus content-bundle summary projection of refine guard metadata (`refineable` / `refine_reject_message`) before import,
- fail-closed validation for malformed templates, snapshots, quickslots, item windows, duplicate instances, and persistence edge cases,
- client packet ownership for `ITEM_GIVE`, `EXCHANGE`, `REFINE`, and first safebox/mall storage requests plus codec ownership for safebox/mall responses and server refine-information frames, including visible-target-gated `ITEM_GIVE` anti-give feedback, the first visible-peer exchange open/cancel/busy-target shell plus active-shell display-only exchange item-add/item-del/gold-add/accept frames with duplicate display-slot/source-item suppression, active-shell anti-give reject text, accept-marker reset on later display changes, `/quit` / `/logout` / `/phase_select` exchange-window teardown, successful carried-item-use / carried item-move / slash inventory-move / carried-equipment move / slash equipment-mutation / drag-to-item stack-consolidation / drop / merchant buy/sell exchange teardown, and self-only template-backed `REFINE_INFORMATION_NEW` previews only after current item restrictions pass while accepted trade finalization, refine result semantics, and storage mutations remain intentionally narrow or fail-closed.

Still missing:

- accepted two-party trade/exchange finalization,
- storage/safebox/mall and player shops,
- item sockets/metins/bonuses/books/scrolls,
- complete anti-flag/class/sex/level/equipment restrictions,
- accepted refine result semantics,
- durable ground ownership timers and party ownership rules,
- compatibility-grade DB-backed item/economy persistence.

### Content, NPCs, shops, and quests

Status: `[~]` authored content seam, not a full content system.

Already present:

- static actor store and interaction definition store,
- `info`, `talk`, `warp`, and `shop_preview` interaction kinds,
- merchant catalogs and first shop open/buy/sell behavior,
- first standalone deterministic quest-flag store/transition primitive with loopback validation, focused readback, and crash-temp cleanup preflights,
- content bundle import/export with preview deltas for static actors, spawn groups, combat profiles, reward drops, NPC routes, warp destinations, focused portable quest-state overview/flag readers, and exact quest-flag import-preview deltas,
- loopback-only authoring/inspection endpoints and a deterministic bootstrap NPC service bundle.

Still missing:

- client-visible quest runtime,
- scripted triggers/results,
- richer NPC service kinds,
- live reload/update policy,
- compatibility-grade regen/drop table ingestion,
- content tooling beyond the current validation and bundle checks.

### Combat, mobs, death, restart, and rewards

Status: `[~]` first PvE loop exists around practice mobs.

Already present:

- target selection and normal attack ingress,
- cadence gates, runtime HP, HP percent refreshes, dead-state rejection, target clear, and delayed respawn rebuild,
- aggro-lite engagement ownership and retaliation ticks,
- player death floor and restart-here/restart-town bootstrap recovery seams,
- deterministic EXP/gold/fixed-drop reward descriptors for accepted non-player deaths,
- loopback spawn-leash tooling for materialized spawn groups: read-only exact inspection (`/local/spawn-groups/{entity_id}/leash?radius=...`) and map-local inspection (`/local/maps/{map_index}/spawn-group-leashes?radius=...`), a controlled one-step return trigger (`POST /local/spawn-groups/{entity_id}/return-step?max_step=...`) that applies one capped planned step for `return_required` live spawn-backed actors through the existing persistence and visibility rebuild path while resetting selected combat-target/engagement ownership when the actor actually moves, read-only pending return-step inspection (`/local/spawn-group-return-steps`, `/local/spawn-group-return-steps/{entity_id}`, `/local/maps/{map_index}/spawn-group-return-steps`), a first pending-frame server-owned return-step executor for out-of-leash spawn-backed actors — including live persisted actors restored already outside leash at startup and manual/operator steps that still leave the actor return-required — using the same stepped visibility/target-reset path, re-arming only while the actor stays live and return-required, suppressing return-step scheduling for dead return-required actors that still own respawn timers, retrying transient static-snapshot persistence failures without fabricating movement or visibility frames, and clearing stale pending return-step deadlines when exact-home return or actor removal commits, a controlled exact-home trigger (`POST /local/spawn-groups/{entity_id}/return-home`) that moves a live spawn-backed actor back to preserved authored home from either `within_radius` drift or `return_required` displacement while resetting selected combat-target/engagement ownership, and a first fail-closed combat gate with an explicit `target_return_required` runtime reason for spawn actors outside their current owned leash,
- codec-owned presentation families for fly effects, PvP/duel, stun, character position / change-speed, and target markers, plus a first presentation-only `CHARACTER_POSITION(position=0|4)` self/peer stance echo,
- tests around watcher/owner respawn, retarget, cleanup, reward, leash-classification, return-step planning, and authored-home respawn-return cases.

Still missing:

- real damage formulas and attack types,
- independent mob AI: aggro radius, chase, autonomous return packet choreography, patrol, target switching, and broader live use of the current leash classifier,
- accepted `USE_SKILL`, ranged/projectile, PvP, and duel runtime policy,
- broad loot/drop tables,
- full death/revive/corpse/menu choreography.

### Social systems and chat

Status: `[~]` chat works; social systems are bootstrap-only.

Already present:

- local talking chat fanout,
- exact-name whisper routing,
- shout/party/guild bootstrap fanout,
- notices and info messages,
- selected dead-player denial behavior for several paths.

Still missing:

- party membership/invite/leave/kick/roles,
- party EXP/drop sharing,
- guild roster/ranks/wars/notices,
- friends/messenger/block systems,
- moderation and permission model.

### Persistence and production operations

Status: `[~]` file-backed bootstrap persistence and useful local ops; not legacy-grade.

Already present:

- file-backed account snapshots and login tickets,
- persisted selected character, position, inventory, equipment, quickslots, gold, item-template, quest-state, static actor, and interaction slices needed by current behavior,
- strict snapshot/template validation, crash-temp reporting/cleanup, manifest-backed backup/restore preflights for several stores, and a first migration catalog with schema ledger, account/character roster, character item-state, character quest-state, item-template, and auth login-ticket handoff migrations plus read-only ledger dry-run planning and a programmatic up/down migration apply primitive,
- loopback-only local endpoints for validation, backup/restore, runtime inspection, and controlled debug actions,
- a `metin2-migrate` CLI that prints catalog summaries and plans from offline ledger snapshots, plus an explicit CLI-only `apply` command that requires an operator-supplied driver/DSN/snapshot/target and stays outside daemon ops endpoints.

Still missing:

- DB-backed account/character/item stores and production migration CLI/ops execution tooling,
- domain repository boundaries for gameplay systems,
- production backup/restore policy,
- crash recovery beyond current file-store primitives,
- metrics/logging policy,
- authenticated/admin-safe tooling beyond loopback debug surfaces.

## Repository layout

- `cmd/authd` / `cmd/gamed` — daemon entrypoints.
- `cmd/metin2-migrate` — migration CLI for catalog summaries, offline ledger-snapshot plans, and explicit CLI-only apply runs.
- `internal/proto/*` — owned packet codecs, fixtures, and wire contracts.
- `internal/auth`, `internal/authboot`, `internal/boot`, `internal/handshake`, `internal/login`, `internal/worldentry`, `internal/game` — connection/session/auth/select/game flow.
- `internal/service` — legacy TCP service runtime and secure session wiring.
- `internal/config` — environment-driven daemon configuration.
- `internal/worldruntime` — topology, maps, AOI/visibility, entities, sessions, combat-oriented actor state, and runtime scopes.
- `internal/minimal` — integrated bootstrap game runtime used by tests and daemons.
- `internal/player`, `internal/inventory`, `internal/itemstore` — character, inventory, item template, equipment, quickslot, and currency behavior.
- `internal/accountstore`, `internal/loginticket` — bootstrap persistence stores.
- `internal/staticstore`, `internal/interactionstore`, `internal/contentbundle` — authored content, static actors, interactions, merchant previews, and bundle import/export.
- `internal/ops` — local debug/pprof/operator HTTP mux.
- `db/migrations` — validated project-owned SQL migration catalog skeleton, first schema ledger/domain migrations including item templates, read-only dry-run planner, metadata-only CLI preflight, and programmatic up/down apply primitive.
- `docs/` — engineering notes, QA, roadmaps, workflow, development, and clean-room docs.
- `spec/protocol/` — owned protocol contracts and packet inventory.

## Development

Run the main checks:

```bash
make test
go vet ./...
git diff --check
```

Run the daemons locally:

```bash
go run ./cmd/authd
go run ./cmd/gamed
```

Important default listener addresses:

- `authd` legacy TCP: `:11002`
- `gamed` legacy TCP: `:13000`
- `authd` ops/pprof/local endpoints: `127.0.0.1:6061`
- `gamed` ops/pprof/local endpoints: `127.0.0.1:6060`

The ops listeners carry `/local/*` operator endpoints and must stay loopback-only. Use SSH tunneling or another explicit local transport for remote access.

Useful docs:

- [Development guide](docs/development.md)
- [Debugging and profiling](docs/debugging-and-profiling.md)
- [Manual client QA checklist](docs/qa/manual-client-checklist.md)
- [Testing strategy](docs/testing-strategy.md)
- [Workflow](docs/workflow.md)
- [Clean-room policy](docs/clean-room-policy.md)
- [Protocol index](spec/protocol/README.md)
- [Current project assessment](docs/roadmaps/2026-08-08-global-project-assessment.md)
- [Current roadmap](docs/plans/2026-08-08-playable-vertical-roadmap.md)

## Current roadmap focus

The next challenge is no longer proving that the client can talk to a clean-room Go server. The next challenge is turning the owned bootstrap slices into a coherent playable loop.

Near-term priorities:

1. **Playable PvE vertical** — content-loaded mobs with lifecycle, targetability, death/respawn, basic AI, rewards, reconnect/restart safety, and stable visibility.
2. **Items and economy** — finish trade/exchange boundaries, ownership timers, item restrictions, storage/refine foundations, and item/economy persistence edges.
3. **Content and quests** — move beyond static interactions into quest state, richer NPC services, regen/drop tables, and validated content workflows.
4. **DB and production ops** — introduce migration contracts, repository seams, backup/restore runbooks, crash recovery, release/deploy docs, and production-safe observability.
5. **Social systems** — replace bootstrap party/guild fanout with membership, permissions, persistence, and gameplay effects after the PvE loop is stable.

## Clean-room rule

This repository must only contain code, documentation, fixtures, and tests produced for this project.

Do not copy legacy Metin2 server/client source into this repository. Use legacy behavior only as an external oracle for independently written specs, tests, and Go implementations.
