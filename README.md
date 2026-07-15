# go-metin2-server

Clean-room Metin2 server emulator in Go, targeting TMP4-era client compatibility.

This repository is a public rewrite built around owned protocol documentation, small verified slices, and a gradual path from a stable boot flow to a real shared-world game server. It intentionally avoids copying legacy source code: legacy trees and captures are treated only as external behavior oracles.

## Status at a glance

`go-metin2-server` is **pre-alpha**. It is not a playable legacy-compatible server yet, but it is also no longer just a packet experiment. The repository currently has:

- real `authd` and `gamed` daemon entrypoints,
- a secure legacy handshake and login/select/game boot path,
- a shared in-process world runtime,
- protocol codecs and fixtures for the owned packet families,
- real-client-oriented integration tests around movement, visibility, chat, items, shops, combat, death, restart, and respawn slices,
- local operator/debug endpoints for runtime inspection and controlled bootstrap actions.

Current repository shape from the latest scan:

- Go version: `1.26`
- Go packages: 38
- Go files: 122
- Go test files: 67
- Markdown docs: 115
- protocol docs under `spec/protocol`: 68
- CI: GitHub Actions runs `gofmt`, `go test ./...`, `go vet ./...`, daemon builds, and Docker runtime/debug builds

Legend used below:

- `[x]` implemented enough for the current milestone
- `[~]` partial / bootstrap / intentionally narrow
- `[ ]` not started or not compatibility-grade yet

## Milestone ladder

- `[x]` **M0 — Protocol-owned boot path**
  - Frame parsing, session phases, secure legacy handshake, auth/login, character selection, loading, enter-game, initial character/point bootstrap, and basic control packets are owned by Go code, protocol docs, and tests.

- `[~]` **M1 — Shared-world pre-alpha**
  - Multiple players can exist in the same in-process world, see each other, move, sync position, talk locally, receive notices, route whispers by exact name, transfer through bootstrap map seams, reconnect/cleanup, and rebuild visibility. It is still a single-process bootstrap runtime, not a production channel/shard architecture.

- `[~]` **M2 — World/entity runtime foundation**
  - The repo has topology, map indexing, AOI/radius-style visibility, player/session directories, entity registries, non-player directories, static actors, spawn groups, runtime scopes, and operator snapshots. The next work is depth: richer lifecycle, better spawn policy, stronger transfer/reconnect edges, and long-running production behavior.

- `[~]` **M3 — Character, inventory, and item systems**
  - Inventory/equipment bootstrap, carried item movement, counted split/merge, quickslot edits, consumable use, `ITEM_USE_TO_ITEM` stack merging, item dropping, ground visibility, pickup, merchant buy/sell, gold mutation, and item/quickslot persistence slices exist. Full legacy item semantics are still not done: sockets, attributes, refine, anti-flag breadth, storage, trade, ownership timers, and compatibility-grade DB persistence remain future work.

- `[~]` **M4 — NPCs, shops, static actors, and authored content**
  - Static actors can be authored, inspected, imported/exported, and connected to `info`, `talk`, `warp`, and `shop_preview` definitions. Shops have structured catalogs and first buy/sell behavior. Spawn groups can materialize stationary practice mobs with bootstrap combat profiles and reward descriptors, including portable `combat_profiles` snapshots that canonicalization can register before spawn validation. This is a useful content seam, not a complete quest/NPC/content scripting system.

- `[~]` **M5 — Combat, mobs, death, restart, and rewards**
  - Target selection, normal attack ingress, cadence gates, runtime HP, dead-state rejection, delayed respawn, aggro-lite engagement ownership, retaliation, player death floor, restart-here/restart-town bootstrap recovery, deterministic EXP/gold rewards, and fixed drop-vnum reward seams exist for practice mobs. Real combat formulas, skills, PvP, mob AI, chase/leash/return, loot tables, and full revive choreography are not compatibility-grade yet.

- `[~]` **M6 — Operations and developer workflow**
  - The project has a Makefile, Dockerfile, CI, pprof/debug mux, health endpoint, local-only runtime-config/player/visibility/map/content endpoints, and development/testing/debugging docs. The runtime-config endpoint exposes the active bootstrap visibility/AOI policy (`whole_map` vs `radius`) so local QA can inspect daemon state without reading environment variables. It still needs release/versioning policy, production deployment docs, migrations, backups, admin tooling, and multi-channel ops maturity.

- `[ ]` **M7 — Legacy parity / production server**
  - Not started as a claim. The current goal is to keep landing small verified compatibility slices until the server can support a narrow playable vertical, then broaden toward legacy-grade systems.

## Subsystem status

### Foundation and workflow

Status: `[x]` strong for a pre-alpha repo.

Already present:

- Go module with daemon entrypoints in `cmd/authd` and `cmd/gamed`.
- Clean `internal/*` package boundaries for protocol, session flow, stores, world runtime, minimal integrated runtime, and ops.
- `Makefile` targets for format, test, build, and Docker image builds.
- GitHub Actions CI for formatting, tests, vet, daemon builds, and Docker builds.
- Development, workflow, testing, QA, debugging/profiling, and clean-room policy docs.

Still missing:

- release/versioning policy,
- production deployment guide outside the current lab environment,
- issue/contribution taxonomy,
- migration/backup/recovery workflow.

### Protocol and boot path

Status: `[x]` owned for the current milestone, `[~]` incomplete for full legacy coverage.

Already present:

- frame envelope and stream handling,
- session phase model,
- control handshake, phase, ping/pong, and key exchange,
- auth/login/select/loading/game entry choreography,
- character delete/select/bootstrap updates,
- movement, sync, chat, whisper, notice, item, quickslot, interaction, shop, combat, and world packet families needed by current slices,
- packet docs in `spec/protocol/` with a maintained index.

Still missing:

- many packet families outside the current verticals,
- deeper evidence for uncertain client behaviors,
- skill, quest, party/guild, messenger, trade/storage, player-shop, GM/admin, and broader world-event ownership.

### Auth, login, and selection

Status: `[x]` bootstrap-compatible.

Already present:

- `authd` and `gamed` sockets,
- secure legacy handshake coverage,
- login ticket flow,
- account/character snapshot loading,
- selection and enter-game transitions,
- character delete in selection,
- tolerated client-version path during loading.

Still missing:

- real account database integration,
- production authentication policy,
- account/session security hardening beyond the current clean-room bootstrap,
- multi-channel selection/dispatch semantics.

### Shared world, visibility, maps, and transfer

Status: `[~]` real in-process runtime, not production world architecture.

Already present:

- connected session registry,
- player directory and map index,
- topology model,
- AOI/radius-style visibility boundaries,
- visibility rebuild helpers,
- local chat/move/sync peer fanout,
- map relocation and transfer bootstrap paths,
- reconnect/quit/logout cleanup,
- player list, map, visibility, transfer, and relocate operator views/actions.

Still missing:

- production channel/shard ownership,
- long-running resource/concurrency policy,
- richer sector behavior,
- robust multi-map content lifecycle,
- world-state persistence and crash recovery.

### Character, inventory, equipment, and quickslots

Status: `[~]` broad bootstrap coverage with many legacy details still pending.

Already present:

- carried inventory/equipment bootstrap replay,
- item set/delete/update refreshes, including selected-character `ITEM_SET` projection of the currently owned authored anti-flag metadata (`anti_get`, transfer/job/sex/empire guards, and stack guard) while leaving unowned bits zero,
- item move, swap, split, and merge cases,
- stack compatibility checks and max-stack guards for current slices,
- locked source/target and duplicate occupancy rejection paths,
- template-backed equip/unequip point-effect application and removal guards,
- consumable item use,
- `ITEM_USE_TO_ITEM` stack merge behavior,
- quickslot add/delete/swap persistence, including type-scoped retarget cleanup when an item/skill/command tuple is rebound to a new bar position,
- quickslot cleanup when item mutations remove or fully merge a source slot,
- basic persisted account/character snapshots.

Still missing:

- full item-type behavior,
- sockets, attributes, refine, metin stones, bonus changers, books/scroll families,
- complete anti-flag/class/sex/level/equipment restrictions,
- storage/safebox/mall,
- player trade/exchange and player shops,
- compatibility-grade database persistence.

### Ground items, pickup, gold, and merchant economy

Status: `[~]` useful first vertical, not a real economy yet.

Already present:

- carried item drop and counted drop,
- temporary ground handles,
- ground-item visibility to peers in scope,
- pickup into inventory with stack-merge behavior,
- first owner-delivery/notice shape for pickups,
- merchant preview/catalog/open/close/buy/sell slices,
- gold mutation for current merchant and reward cases.

Still missing:

- durable ground ownership timers,
- party ownership rules based on real party state,
- drop permission transitions,
- complete merchant edge cases and shop variants,
- NPC-driven service breadth,
- real economy balancing and persistence.

### Static actors, NPCs, shops, and content authoring

Status: `[~]` authored bootstrap content seam.

Already present:

- static actor store,
- interaction definition store,
- `info`, `talk`, `warp`, and `shop_preview` interaction kinds,
- structured merchant catalogs,
- content bundle import/export,
- loopback-only local endpoints for static actors, interactions, visibility, and content bundles,
- example bootstrap NPC service bundle.

Still missing:

- quest runtime,
- scripted triggers/results,
- richer NPC service kinds,
- live content reload policy,
- content validation tooling beyond current store/bundle checks,
- compatibility-grade regen/drop table ingestion.

### Non-player entities, mobs, combat, death, and rewards

Status: `[~]` first PvE loop exists around practice mobs.

Already present:

- non-player entity directory,
- static/non-player combat profiles,
- target selection and normal attack packet ingress,
- selected-target snapshot/version checks,
- HP mutation and HP percent refreshes,
- dead-state rejection and target clear,
- delayed respawn rebuild path,
- engagement ownership to prevent noisy multi-owner combat loops,
- retaliation ticks against the engaged player,
- player death floor with denial gates for several live actions,
- restart-here and restart-town slash-command recovery seams,
- deterministic EXP/gold/fixed-drop reward descriptors for accepted non-player deaths,
- extensive TCP-level regression tests around watcher/owner respawn, retarget, cleanup, and reward cases.

Still missing:

- real damage formulas,
- attack animations/types beyond the first normal path,
- skill combat,
- PvP and duel policy,
- mob AI: aggro radius, chase, leash, return, patrol, target switching,
- broad loot/drop tables,
- full death/revive/corpse/menu evidence.

### Social systems and chat

Status: `[~]` chat works, social systems are bootstrap-only.

Already present:

- local talking chat fanout,
- exact-name whisper routing,
- shout/party/guild bootstrap fanout,
- server notices and info messages,
- some dead-player denial behavior for selected paths.

Still missing:

- party membership state,
- party invite/leave/kick/roles,
- party EXP/drop sharing,
- guild roster/ranks/wars/notices,
- friend/messenger/block systems,
- moderation and permission model.

### Persistence and data model

Status: `[~]` enough for bootstrap slices, not legacy-grade.

Already present:

- file-backed account/login-ticket snapshots,
- persisted selected character state used by current boot flow,
- persisted position for selected slices,
- persisted inventory/equipment/quickslots/gold for current item and merchant paths,
- item/static/interaction stores and bundle import/export.

Still missing:

- real database-backed schema,
- migrations,
- domain repositories for gameplay systems,
- crash recovery policy,
- backup/restore,
- persistent party/guild/quest/world state.

### Operations, debug, and local tooling

Status: `[~]` good lab/debug surface, not production ops.

Already present:

- pprof/debug mux,
- `/healthz`,
- local runtime config endpoint,
- local inventory/equipment/currency snapshots,
- local static actor and interaction authoring endpoints,
- local content bundle import/export,
- local notice, relocate, transfer, players, visibility, and maps endpoints,
- Docker runtime and debug image targets.

Still missing:

- authentication/authorization for production admin surfaces,
- release packaging,
- deployment guide,
- metrics/logging policy,
- backup/restore and migration runbooks,
- admin/GM tooling beyond local debug endpoints.

## What is in the repo

- `cmd/authd` / `cmd/gamed` — daemon entrypoints.
- `internal/proto/*` — owned packet codecs, fixtures, and wire contracts.
- `internal/auth`, `internal/authboot`, `internal/boot`, `internal/handshake`, `internal/login`, `internal/worldentry`, `internal/game` — connection/session/auth/select/game flow.
- `internal/service` — legacy TCP service runtime and secure session wiring.
- `internal/config` — environment-driven daemon configuration.
- `internal/worldruntime` — topology, maps, AOI/visibility, entities, sessions, combat-oriented static actor state, and runtime scopes.
- `internal/minimal` — current integrated game runtime used by tests and daemons.
- `internal/player`, `internal/inventory`, `internal/itemstore` — current character, inventory, item template, equipment, quickslot, and currency behavior.
- `internal/accountstore`, `internal/loginticket` — bootstrap persistence stores.
- `internal/staticstore`, `internal/interactionstore`, `internal/contentbundle` — authored content, static actors, interactions, merchant previews, and bundle import/export.
- `internal/ops` — local debug/pprof/operator HTTP mux.
- `db/migrations` — placeholder for future database migration work.
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

Default legacy listener addresses are documented in [docs/development.md](docs/development.md). The important defaults are:

- `authd`: `:11002`
- `gamed`: `:13000`
- pprof/debug: `:6061` for `authd`, `:6060` for `gamed`

Useful docs:

- [Development guide](docs/development.md)
- [Debugging and profiling](docs/debugging-and-profiling.md)
- [Manual client QA checklist](docs/qa/manual-client-checklist.md)
- [Testing strategy](docs/testing-strategy.md)
- [Workflow](docs/workflow.md)
- [Clean-room policy](docs/clean-room-policy.md)
- [Protocol index](spec/protocol/README.md)
- [Current project assessment](docs/roadmaps/2026-05-24-global-project-assessment.md)
- [Master roadmap](docs/plans/2026-05-24-master-legacy-parity-roadmap.md)

## Roadmap focus

The next challenge is no longer proving that the target client can talk to a clean-room Go server. The next challenge is turning the owned slices into a coherent game loop.

Near-term priorities:

1. **PvE vertical depth** — move practice mobs toward real spawned mobs with authored combat profiles, AI, chase/leash/return, broader rewards, and stable death/restart behavior.
2. **Item and economy parity** — finish item-use families, anti-flags, ownership timers, pickup rules, shop variants, and persistence edges.
3. **World runtime hardening** — make AOI, transfer, reconnect, respawn, static/non-player lifecycle, and visibility replay more robust under multiple clients.
4. **Content and quest seams** — grow NPC services, content validation, spawn/regen/drop data, and the first quest-style state machine.
5. **Real social systems** — replace bootstrap party/guild fanout with membership, permissions, persistence, and gameplay effects.
6. **Persistence and production ops** — introduce DB migrations, backup/restore, release/deploy workflow, observability, and safe admin tooling.

## Clean-room rule

This repository must only contain code, documentation, fixtures, and tests produced for this project.

Do not copy legacy Metin2 server/client source into this repository. Use legacy behavior only as an external oracle for independently written specs, tests, and Go implementations.
