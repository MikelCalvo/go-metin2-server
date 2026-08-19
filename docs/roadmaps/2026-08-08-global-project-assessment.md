# Global Project Assessment — 2026-08-08

This assessment captures the project-wide state of `go-metin2-server` after the repository moved beyond the first boot, shared-world, item, combat, and content bootstrap waves.

The top-level README is intentionally shorter and more public-facing. This document keeps the maintainer-level view: what is real today, what is still bootstrap, and where the next work should concentrate.

## Executive summary

`go-metin2-server` is still **pre-alpha**, but it has crossed the most important early threshold: the project is no longer mainly about proving that a TMP4-era client can speak to a clean-room Go process.

The repository now has:

- real `authd` and `gamed` daemon entrypoints,
- secure legacy handshake and login/select/game-entry ownership,
- a shared in-process world runtime,
- player visibility, movement, chat, transfer, reconnect, static/non-player actor, and runtime inspection seams,
- broad bootstrap item, equipment, quickslot, ground-item, shop, exchange/refine fail-closed, and persistence validation slices,
- authored static actors, interactions, merchant catalogs, content bundles, spawn groups, reward descriptors, and combat profiles,
- a first practice-mob PvE loop with target, attack, HP, death, respawn, retaliation, player death floor, restart seams, and deterministic rewards,
- strong protocol documentation and a large automated test surface.

The major challenge has shifted:

> The next challenge is no longer owning the protocol. It is turning the owned bootstrap slices into a coherent playable game loop.

## Repository snapshot

Scan state used for this refresh:

- baseline commit: `3abdb852 feat: add item exchange fail-closed guard`
- Go version: `1.26`
- Go packages: `38`
- Go files: `133`
- Go test files: `78`
- Markdown docs after this refresh: `130`
- protocol docs under `spec/protocol`: `76`
- CI: `.github/workflows/ci.yml` runs `gofmt`, `go test ./...`, `go vet ./...`, daemon builds, and Docker runtime/debug builds

Two lane-only commits existed at assessment time and were **not** included in the `main` baseline because integration validation had timed out in `internal/minimal`:

- `lane/items`: exchange anti-give/display-slot guard follow-up
- `lane/world`: map-local static actor snapshot endpoint follow-up

The README is written against the clean `main` baseline while the roadmap accounts for those areas as near-term work.

## Maturity by track

### Foundation and workflow

Status: strong for a pre-alpha public repository.

Already strong:

- daemon entrypoints are real and tested,
- packages are split around protocol, flow, stores, runtime, ops, and content,
- clean-room policy is explicit,
- docs and tests are first-class artifacts,
- CI exists and is public,
- lane-based autonomous work is now split by subsystem with a single integration gate.

Still weak:

- release/versioning policy is missing,
- deployment docs are lab-oriented rather than production-oriented,
- migration and recovery runbooks are still emerging,
- public contribution/issue taxonomy is not defined.

### Protocol and boot path

Status: owned for the current milestone; incomplete for full legacy parity.

Already strong:

- frame envelope,
- session phases,
- secure legacy handshake,
- auth/login/select/loading/game-entry choreography,
- current movement/chat/item/shop/combat/world/content packet families,
- packet matrix and per-boundary protocol docs.

Still weak:

- many legacy packet families are not owned yet,
- skill, quest, social, trade/storage, player-shop, GM/admin, and many world-event packets remain future work,
- uncertain behavior still needs real-client captures or project-owned fixtures before being claimed.

### Shared world and actor runtime

Status: real in-process runtime, not production world architecture.

Already real:

- sessions share an in-process world,
- visibility, movement, sync, local chat, map scopes, transfers, reconnect, and cleanup exist,
- static and non-player actors can be represented and inspected,
- topology and map-index helpers exist,
- visibility and occupancy state can be queried through local ops endpoints.

Still bootstrap:

- no production channel/shard ownership,
- no long-running resource/concurrency policy,
- no durable world-state persistence,
- no real mobile mob lifecycle,
- multi-map content lifecycle is still narrow and test-driven around current seams.

### Items, inventory, equipment, and economy

Status: broad bootstrap coverage, not legacy-grade.

Already real enough for slices:

- item inventory/equipment replay and mutations,
- item move/swap/split/merge,
- item use and drag-to-item merge,
- quickslot add/delete/swap and synchronization around item mutations,
- carried drop/pickup and ground visibility,
- merchant catalog/open/buy/sell paths,
- authored item-template guards and display metadata,
- file-backed persistence validation for account/item/quickslot state,
- fail-closed ownership for unsupported `ITEM_GIVE`, `EXCHANGE`, and `REFINE` behavior.

Still missing:

- accepted two-party trade/exchange,
- storage/safebox/mall,
- player shops,
- real refine success semantics,
- sockets/metins/bonuses/books/scrolls,
- complete restrictions and anti-flags,
- durable economy and DB-backed item persistence.

### Content, NPCs, shops, and quests

Status: useful authored content seam, not a complete content system.

Already present:

- static actor and interaction stores,
- `info`, `talk`, `quest_flag`, `warp`, and `shop_preview` definitions,
- merchant catalogs and first buy/sell behavior,
- first standalone quest-flag persistence plus one static-actor `quest_flag` trigger seam,
- content bundle import/export and preview deltas,
- portable combat profiles, reward descriptors, and spawn groups,
- local inspection endpoints and a deterministic bootstrap NPC service bundle.

Still missing:

- client-visible quest UI/runtime,
- branching quest scripts and rewards,
- regen/drop table ingestion,
- richer NPC service kinds,
- live reload/update policy.

### Combat, mobs, death, restart, and rewards

Status: first practice-mob PvE loop exists; real combat and AI do not.

Already present:

- target selection,
- normal attack ingress,
- cadence gates,
- runtime HP and HP percent refresh,
- death, target clear, dead-state rejection, delayed respawn rebuild,
- engagement ownership,
- retaliation ticks and player death floor,
- restart-here/restart-town bootstrap recovery,
- deterministic EXP/gold/fixed-drop reward descriptors,
- authored combat-profile formula seam (`attack_value` / `defense_value` -> `damage_per_normal_attack`) for practice mobs,
- authoring-only fixed `drop_tables` expansion into spawn-group reward descriptors,
- persisted bootstrap `0`-HP floor across reconnect / `/phase_select` / fresh `ENTERGAME`,
- codec-owned but non-emitted presentation packet families for future combat surfaces.

Still missing:

- compatibility-grade formulas beyond the current bootstrap combat-profile defaults,
- attack types and animations beyond the current normal path,
- accepted skill/ranged/projectile runtime behavior,
- PvP/duel policy,
- broader mob AI beyond the owned proximity aggro, chase-step, leash, and return seams (patrol, target switching, independent attack cadence packets),
- weighted/random loot tables and pickup mutation beyond fixed descriptors,
- full death/revive/corpse/menu behavior.

### Social systems

Status: bootstrap-only.

Already present:

- local chat,
- exact-name whisper,
- shout/party/guild bootstrap fanout,
- notices and info messages.

Still missing:

- party membership,
- party invite/leave/kick/roles,
- EXP/drop sharing,
- guild roster/ranks/wars/notices,
- messenger/friends/block systems,
- moderation/permission model.

### Persistence and operations

Status: enough for bootstrap slices and local QA; not production-ready.

Already useful:

- file-backed account/login-ticket snapshots,
- file-backed static actor, interaction, and item-template snapshots,
- strict validation and crash-temp reporting/cleanup,
- manifest-backed backup/restore preflights for key stores,
- loopback-only local ops endpoints,
- runtime config inspection and pprof/debug surfaces.

Still missing:

- DB schema beyond the initial `schema_migrations` ledger,
- runtime migration application/backfill tooling,
- domain repositories,
- production backup/restore policy,
- metrics/logging policy,
- release/deploy workflow,
- authenticated admin tooling beyond loopback debug primitives.

## Strategic assessment

The project has strong engineering discipline: small slices, test-first contracts, protocol docs, and clean-room boundaries. That is the right posture for a server rewrite where behavior can easily drift into guesswork.

The main risk is now focus, not foundation. Adding more isolated packet/codecs/endpoints will not by itself make the server feel playable. The project should prioritize one narrow player-facing loop:

1. log in,
2. enter a map,
3. see NPCs/mobs,
4. move with stable visibility,
5. fight spawned mobs,
6. receive deterministic rewards,
7. pick up/use/equip/trade basic items,
8. persist enough state to survive reconnect/restart,
9. inspect/recover the runtime locally when something fails.

That loop is deliberately smaller than legacy parity, but it is large enough to validate the server as a game rather than a protocol/runtime scaffold.

## Lane orientation after this assessment

The current autonomous lane direction should match the project phase:

- `items`: keep focused on item/economy/trade/storage/refine edges.
- `combat`: keep focused on combat formulas, skills, reward policy, and PvP/duel semantics once evidence exists.
- `content`: keep focused on NPC/content/quest state, not just more small endpoints.
- `world`: retarget to mob AI and spawn lifecycle rather than generic world/AOI work.
- `persistence`: retarget to DB/migrations/production ops rather than more bootstrap snapshot hardening.
- integrator/dirty guard/CI watchdog: keep preserving a green `main` while lanes work independently.

## Recommended next milestone

The next milestone should be a **playable PvE vertical**, not broad parity:

- content-loaded practice mobs can be targeted, attacked, killed, respawned, and inspected,
- mobs have the first real AI/lifecycle rules beyond stationary retaliation,
- rewards produce item/gold/EXP behavior that flows through inventory/economy and persistence seams,
- reconnect/transfer/restart do not duplicate, lose, or resurrect invalid actor/item state,
- local operator endpoints can validate and recover the relevant stores before a DB migration exists.

Only after this vertical is stable should the project widen into full social systems, richer quests, full storage/trade, PvP/duels, and production deployment claims.
