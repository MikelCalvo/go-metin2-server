# Global Project Assessment — 2026-05-24

This document captures the project-wide state of `go-metin2-server` after the repository had moved beyond the first login/world bootstrap and into item, combat, world-runtime, and content slices.

The top-level README is intentionally short. This file keeps the more detailed maintainer assessment that used to make the README hard to scan.

## Executive summary

`go-metin2-server` is a clean-room Go rewrite of a TMP4-era Metin2 server. It is still pre-alpha, but it has crossed several important thresholds:

- the secure legacy handshake and login path are owned,
- the select/loading/game transition is exercised,
- real daemons exist for auth and game sockets,
- a shared-world runtime exists,
- players can see, move, chat, transfer, and reconnect through owned seams,
- inventory/equipment/item/merchant/ground-item slices are now behavior-bearing,
- static actors, NPC interactions, shops, and practice mobs exist as authored content seams,
- the first combat/death/restart loop is live,
- protocol docs, tests, and slice plans are first-class project artifacts.

The project is no longer blocked on proving that a clean-room server can talk to the target client. The major remaining challenge is breadth and depth: converting bootstrap-compatible verticals into full legacy-grade systems.

## Repository snapshot

Approximate scan state at this assessment:

- 119 Go files
- 64 Go test files
- 38 Go packages
- 116 Markdown files
- 66 protocol docs under `spec/protocol`
- 36 implementation plans under `docs/plans`
- `main` was clean and green when this assessment was prepared

The current public structure is healthy:

- `cmd/authd` and `cmd/gamed` are the daemon entrypoints.
- `internal/proto/*` owns packet codecs and wire contracts.
- `internal/worldruntime` owns topology, maps, AOI/visibility, entity directories, and runtime scopes.
- `internal/player` and `internal/inventory` own early character/item mutation semantics.
- `internal/minimal` composes the current integrated runtime used by daemons and tests.
- `spec/protocol` and `docs/plans` preserve clean-room contracts and next-step history.

## Maturity by track

### Foundation and workflow

Status: strong.

Already present:

- Go project layout with public daemon entrypoints.
- Makefile and standard Go test flow.
- Clean-room policy.
- Development, workflow, testing, QA, and debugging docs.
- Automated tests across protocol, flow, runtime, stores, and ops packages.

Still needed:

- release/versioning policy,
- production deployment documentation beyond the lab setup,
- contributor-facing issue taxonomy,
- broader CI/release ergonomics.

### Protocol and boot path

Status: strong for the current milestone.

Already present:

- frame envelope,
- session phases,
- control/handshake packets,
- auth/login/select packets,
- world-entry packets,
- movement/sync/chat packets,
- item/shop/combat packet families for current slices,
- packet matrix and per-slice protocol docs.

Still needed:

- broader packet-family coverage,
- more real-client captures/fixtures for uncertain families,
- dedicated evidence for restart ingress if the target client proves one outside the current slash-command path,
- skill, quest, party/guild, trade/storage, and GM/admin packet ownership.

### Shared-world runtime

Status: real but not production-complete.

Already present:

- connected session registry,
- player directory and visibility scopes,
- map index/topology seams,
- AOI-style visibility boundaries,
- movement and sync replication,
- transfer/rebootstrap paths,
- reconnect cleanup and stale ownership handling,
- static actor visibility and runtime snapshots.

Still needed:

- channel ownership beyond bootstrap assumptions,
- deeper sector/runtime policy,
- richer non-player lifecycle,
- production concurrency/resource policy,
- world persistence and recovery.

### Character, inventory, and item systems

Status: broad bootstrap coverage, many legacy details still missing.

Already present:

- carried inventory and equipment snapshots,
- item set/delete/update refreshes,
- item move/split/merge semantics for current cases,
- consumable use,
- drag-to-item stack merge seam,
- quickslot persistence and refreshes,
- carried drops and counted drops,
- ground item visibility, ownership labels, pickup, and stack-merge pickup,
- merchant buy/sell integration with inventory/gold.

Still needed:

- complete item restrictions,
- full anti-flag behavior,
- item sockets/attributes/refine semantics,
- metin/enchant/socket use cases,
- storage/safebox/mall,
- trading/exchange,
- ownership timers and permission transitions,
- compatibility-grade DB persistence.

### Combat, mobs, and death/restart

Status: first vertical slice exists.

Already present:

- target selection,
- normal attack ingress,
- same-target cadence gating,
- runtime HP for practice mobs,
- visible mob death and respawn,
- aggro-lite ownership gate,
- immediate and delayed retaliation ticks,
- bootstrap combat-profile defaults with attack/defense formula metadata,
- deterministic authored EXP/gold/fixed-drop reward descriptors for accepted non-player deaths,
- retaliation-owned player death floor,
- restart-here and restart-town bootstrap seams,
- dead-player recipient and visibility gating for several owned paths.

Still needed:

- compatibility-grade damage formulas beyond the current bootstrap combat-profile defaults,
- attack types and animations beyond the first normal attack path,
- broader EXP/gold/drop reward policy beyond the current deterministic descriptor seam,
- mob AI, aggro, leash, chase, and return behavior,
- skill combat,
- PvP and duel policy,
- broader revive/corpse/death choreography.

### Content runtime

Status: useful seams, not full content system.

Already present:

- static actors,
- authored interaction definitions,
- shop catalogs,
- content bundle import/export,
- spawn groups that can materialize stationary practice mobs,
- operator/runtime views for current content state.

Still needed:

- richer NPC services,
- quest runtime,
- scripted triggers,
- mob regen tables,
- drop tables,
- content validation tooling,
- live content reload/update semantics.

### Social systems

Status: bootstrap-only.

Already present:

- talking chat fanout,
- whisper routing,
- party/guild/shout bootstrap fanout,
- notices and info messages.

Still needed:

- real party membership,
- party roles, bonuses, EXP/drop sharing,
- guild creation, roster, ranks, wars, notices,
- friend/messenger/block systems,
- permissions and moderation controls.

### Persistence and operations

Status: enough for bootstrap, not legacy-grade.

Already present:

- file-backed account/login-ticket snapshots,
- persisted character position and selected bootstrap state,
- persisted inventory/equipment/quickslot slices,
- loopback-only operator/debug endpoints,
- pprof/debugging docs.

Still needed:

- DB-backed schema and migrations,
- domain repositories for gameplay systems,
- backup/restore,
- crash recovery,
- live ops/admin tooling,
- multi-channel deployment and release workflows.

## Strategic assessment

The repository is strong in discipline and architecture: small slices, docs-first contracts, TDD, clean-room boundaries, and high test coverage relative to the project stage.

The risk is not foundation quality; the risk is scope. A full Metin2 legacy-compatible server is a large product. The fastest path is to keep the current slice discipline while parallelizing work by subsystem lanes:

- items/inventory/equipment,
- combat/mobs/rewards,
- world/runtime/spawns/visibility,
- docs/evidence/archaeology when needed.

The top priority should be preserving a green, shippable `main` while allowing independent lanes to explore and implement narrow slices in worktrees.

## Recommended next milestone

Aim for a playable vertical before claiming broad parity:

1. A player can log in, move, see peers, and interact with basic content.
2. A player can fight real spawned mobs with deterministic damage.
3. Mobs can die, respawn, and grant basic rewards.
4. Items/gold can drop, be picked up, equipped, consumed, and persisted.
5. NPC shops and basic services are usable without debug commands.
6. The runtime survives reconnects/transfers/death/restart across those systems.

That milestone is much smaller than full 1:1 parity but large enough to validate the server as an actual game loop rather than a packet/runtime scaffold.
