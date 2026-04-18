# Global Project Assessment — 2026-04-18

This document captures the project-wide status of `go-metin2-server` after a full repository scan on 2026-04-18.

It is intentionally broader than the slice-level protocol notes in `spec/protocol/` and broader than the execution plans in `docs/plans/`.
The goal is to answer a different question:

- where is the project globally,
- which parts are already real,
- which parts are still bootstrap shortcuts,
- and what should happen next if the repository is meant to mature into an open-mt2-style public server project.

## Executive summary

The project is no longer at the "blank repo" or "packet experiment" stage.
It already owns a serious amount of protocol, runtime, and documentation work:

- split `authd` and `gamed` daemons,
- protocol-owned packet codecs,
- handshake/login/select/create/delete/enter-game path,
- a minimal visible-world bootstrap,
- shared-player visibility,
- movement and sync fanout,
- the first social/system channels,
- bootstrap persistence for account and character snapshots,
- public-facing docs and slice plans,
- container packaging and public CI baseline.

That means the project has effectively completed its foundation and boot-path phases.

The next global challenge is no longer "own the protocol".
The next challenge is "own the world runtime".

In practical terms:

- M0 (protocol-owned boot path) is effectively complete.
- The repository is entering M1 (shared-world pre-alpha).
- The highest-value next work is world topology, warp/map migration, visibility rebuilding, and a reusable world/entity runtime.
- Inventory, equipment, combat, NPCs, mobs, and quest scripting should come later, after the world/runtime layer is no longer mostly bootstrap glue.

## Repository snapshot at scan time

Approximate scan snapshot:

- 54 Go source files
- 28 Go test files
- 24 Go packages
- ~8.8k lines of Go code
- ~2.7k lines of Markdown documentation
- 2 public daemon entrypoints: `authd`, `gamed`

Public repo shape:

- `cmd/` — shipped binaries
- `internal/proto/` — owned wire contracts
- `internal/*flow` — handshake/auth/login/worldentry/game flow composition
- `internal/minimal/` — current bootstrap runtime
- `internal/accountstore` / `internal/loginticket` — bootstrap persistence
- `internal/service` / `internal/ops` — runtime listeners, ops HTTP server, server-push hooks
- `spec/protocol/` — owned protocol documentation
- `docs/plans/` — slice-by-slice implementation plans
- `.github/workflows/` — public CI baseline

## Current maturity by track

### 1. Foundation and project hygiene

Status: strong

Already present:

- Go 1.26 baseline
- split `authd` / `gamed`
- multistage Docker build
- Makefile for common local tasks
- pprof/ops HTTP server
- public CI workflow baseline
- clean-room policy and workflow docs

Assessment:

This is already beyond "toy emulator repo" quality.
The project has a maintainable public structure.

### 2. Protocol ownership and compatibility scaffolding

Status: strong

Already present:

- frame envelope contract
- session phases
- control packets
- auth/login packet families
- selection/world-entry packet families
- move/sync/chat packet families
- protocol docs for each implemented slice
- tests across codec, flow, runtime, and socket layers

Assessment:

The repository clearly owns its protocol surface.
This is one of its strongest areas.

### 3. Boot path and first in-world loop

Status: effectively complete for the current milestone

Already present:

- handshake
- auth/login
- character list
- empire selection
- create character
- delete character
- select character
- enter game
- self bootstrap
- basic movement
- sync-position reconciliation

Assessment:

The original "can we boot into the game and do something real" milestone is no longer speculative.
It is implemented.

### 4. Shared-world runtime

Status: early but real

Already present:

- peer visibility on enter
- peer removal on disconnect
- queued `MOVE` replication
- queued `SYNC_POSITION` replication
- `MapIndex`-based same-map visibility boundary
- same-map + same-empire local talking fanout
- same-empire shout fanout
- local-only operator-triggered server notice broadcast

Still missing:

- real channel topology
- world ownership beyond one bootstrap shared registry
- warp/map transfer
- visibility rebuild on transfer
- AOI/range/sector culling
- non-player entities in the runtime

Assessment:

This is the main frontier.
The repo has entered this phase, but it is still closer to "bootstrap shared-world" than to a reusable world simulation layer.

### 5. Social systems

Status: bootstrap-only

Already present:

- local talking chat
- whisper
- bootstrap party chat
- bootstrap guild chat
- bootstrap shout
- `INFO`
- server-originated `NOTICE`
- local-only notice ops endpoint

Still missing:

- real party membership/state
- real guild lifecycle/roster state
- permissions/admin model beyond loopback-only ops
- richer social persistence

Assessment:

Useful for compatibility slices, not yet real game-server social systems.

### 6. Persistence and data model

Status: bootstrap-only

Already present:

- login ticket persistence
- bootstrap account snapshots
- persisted selection-surface changes
- persisted coordinates
- persisted bootstrap `MapIndex`

Still missing:

- DB-backed persistence
- migrations used by live systems
- domain repositories for gameplay systems
- compatibility-grade persistence strategy

Assessment:

Enough to unblock current vertical slices.
Not enough yet for a serious open-mt2-style server backend.

### 7. Gameplay systems

Status: not started in any meaningful sense

Not yet present as real subsystems:

- inventory
- equipment
- item use
- derived stat systems
- combat
- targeting
- damage/death/respawn
- NPCs
- shops
- mobs and AI
- spawns
- quest runtime

Assessment:

This is where the future project size still lives.
Most of the expensive work is still ahead.

### 8. OSS maturity and public project ergonomics

Status: promising but incomplete

Already present:

- slice documentation
- public roadmap language in README
- clean-room and workflow docs
- CI baseline
- container build path

Still missing:

- richer milestone board / issue taxonomy
- release process
- compatibility fixture library growth
- deployment and operations guides for more than the lab setup

Assessment:

The repo is already strong enough to be followed publicly.
It is not yet mature enough to be easy for new contributors to pick up without guidance.

## Where the project is today

The repository is best described as:

- past the protocol-bootstrap phase,
- finishing the boot-path phase,
- entering the world-runtime phase.

If described as milestone progress:

- M0 — Protocol-owned boot path: complete enough to move on
- M1 — Shared-world pre-alpha: in progress
- M2+ — Real systems/server architecture: not started yet

That is the most important global conclusion from the scan.

## Recommended route from here

The recommended order is:

1. world topology and runtime first
2. reusable entity/world layer second
3. character systems third
4. combat vertical slice fourth
5. NPC/mob/content runtime after that
6. compatibility-grade persistence and operations hardening in parallel where needed
7. quest runtime late, not early

## Why this route makes sense

The project already has enough packet ownership to stop prioritizing new protocol slices for their own sake.

What it lacks now is the layer that makes future systems believable:

- map/channel ownership
- relocation/warp semantics
- visibility rebuilds
- entity registration and lifetime
- reusable world runtime boundaries

If inventory, equipment, combat, or NPC work starts before that layer exists, the repo risks growing around bootstrap shortcuts instead of a durable server model.

## Recommended milestone ladder

### M1 — Shared-world pre-alpha

Primary goal:

Replace the remaining "everyone shares one bootstrap bubble" assumptions with real world topology.

Exit criteria:

- map/channel topology is explicit
- warp/map transfer exists
- visibility is rebuilt after transfer
- same-map visibility is no longer a thin registry shortcut only
- local chat, visibility, and movement semantics are stable under relocation

### M2 — Entity/world runtime foundation

Primary goal:

Introduce reusable runtime ownership for maps, entities, and player attachment.

Exit criteria:

- a real world/entity runtime exists outside bootstrap helper closures
- player sessions bind to reusable world entities
- non-player entities can be introduced without rewriting the runtime model

### M3 — Character systems

Primary goal:

Make the player state meaningful beyond movement/chat.

Exit criteria:

- inventory exists
- equipment exists
- item use exists
- character-state persistence boundaries are defined

### M4 — Combat vertical slice

Primary goal:

Prove the first complete gameplay loop.

Exit criteria:

- target selection works
- one attack path works
- HP/damage/death/respawn exist
- the slice is testable and documented

### M5 — Content runtime

Primary goal:

Add the first real world actors beyond player characters.

Exit criteria:

- NPC placeholders
- mobs/spawns
- shops or another first content loop
- enough content runtime to support the next non-trivial slices

### M6 — Compatibility-grade persistence and operations

Primary goal:

Move from bootstrap persistence and lab-only ops toward a durable public server project.

Exit criteria:

- DB-backed persistence strategy is implemented
- migration story exists
- admin/ops tooling is broader than local-only notice posting
- CI/release/deploy guidance are strong enough for outside use

## What should not be prioritized yet

The scan strongly suggests avoiding these as immediate priorities:

- a full quest runtime before world/runtime systems exist
- broad inventory/equipment work before entity/world boundaries are cleaner
- deep admin/auth systems before operator needs go beyond local-only actions
- large protocol expansion just to increase packet count without unlocking runtime value

## Practical next-step recommendation

If only one big track should be prioritized next, it should be:

- world topology + warp/map transfer + visibility rebuild

If two tracks can move in parallel, they should be:

- world topology/runtime
- public project hygiene (CI, roadmap, documentation, contributor-facing clarity)

## Final assessment

The project is in a good place.

Not because it is already a real game server,
but because it has already solved the part that usually kills public emulator rewrites early:

- no architecture,
- no owned protocol contract,
- no tests,
- no documentation,
- no coherent incremental path.

Those pieces are in place here.

The hard part that remains is the large one:
turning a protocol-owned bootstrap runtime into a world-owned game server.

That is the right problem to have at this stage.
