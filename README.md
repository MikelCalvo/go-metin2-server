# go-metin2-server

Clean-room Metin2 server emulator in Go, targeting TMP4-era client compatibility.

This repository is a public rewrite built around owned protocol documentation, small verified slices, and a gradual path from a stable boot flow to a real shared-world game server. It intentionally avoids copying legacy source code: legacy trees and captures are treated only as behavior oracles.

## Current state

`go-metin2-server` is still **pre-alpha**, but it is no longer only a packet experiment. The project already owns a stable login-to-game bootstrap, a first shared-world runtime, and several client-visible gameplay verticals.

Legend:
- `[x]` implemented enough for the current milestone
- `[~]` partial / bootstrap / intentionally narrow
- `[ ]` not started or not compatibility-grade yet

## Milestone ladder

- `[x]` **M0 — Protocol-owned boot path**
  - Secure legacy handshake, auth/login, character selection, enter-game, and initial in-world bootstrap are covered by owned docs and tests.

- `[~]` **M1 — Shared-world pre-alpha**
  - Players can enter the same world, see each other, move, sync position, chat, receive notices, transfer through the current bootstrap seams, and rebuild visibility across the owned runtime paths.

- `[~]` **M2 — World/entity runtime foundation**
  - The repo has owned topology, map indexing, AOI-style visibility, session directories, static actors, spawn groups, and operator/debug snapshots, but it is still evolving toward a full production world simulation.

- `[~]` **M3 — Character systems**
  - Inventory, equipment, item movement, item use, quickslots, merchant buy/sell, carried drops, ground visibility, pickup, and early appearance refreshes exist as narrow compatibility slices. Full legacy item semantics are still ahead.

- `[~]` **M4 — Combat vertical slice**
  - Target selection, normal attack ingress, cadence gating, runtime-owned HP, practice mob death/respawn, retaliation-owned player death, and restart bootstrap seams exist. Real combat formulas, skills, rewards, PvP, and broader death policy remain future work.

- `[~]` **M5 — Content runtime**
  - Static actors, authored interactions, basic NPC/shop services, warp-style interactions, and stationary practice mobs loaded from spawn groups are available. Rich NPC services, mob AI, loot, quest runtime, and content scripting are not yet compatibility-grade.

- `[ ]` **M6 — Persistence, operations, and production readiness**
  - Current persistence is sufficient for bootstrap slices, but DB-backed storage, migrations, production deployment, richer GM/admin tooling, and release operations remain major roadmap items.

## Roadmap focus

The next challenge is no longer only owning packet formats; it is turning the existing slices into a real game-server runtime.

Near-term priorities:

1. **Items and inventory parity** — finish the nearby item-use, stack, lock, anti-flag, ground ownership, and pickup edge cases.
2. **Combat and rewards** — move from practice-mob combat toward real damage, EXP, gold, drops, mob lifecycle, and player recovery rules.
3. **World runtime depth** — harden AOI, map transfer, reconnect, spawn lifecycle, static/non-player entity updates, and visibility replay.
4. **Content systems** — grow NPC services, shop variants, authored spawn groups, and the first quest/script seams.
5. **Social systems** — replace bootstrap party/guild/chat fanout with real membership, persistence, permissions, and gameplay effects.
6. **Persistence and ops** — introduce compatibility-grade storage, migrations, backup/restore, observability, release, and admin workflows.

For the detailed maintainer view, see:
- [Current project assessment](docs/roadmaps/2026-05-24-global-project-assessment.md)
- [Master roadmap](docs/plans/2026-05-24-master-legacy-parity-roadmap.md)
- [Protocol index](spec/protocol/README.md)
- [Detailed slice plans](docs/plans/)

## What is in the repo

- `cmd/authd` / `cmd/gamed` — daemon entrypoints
- `internal/proto/*` — owned packet codecs and wire contracts
- `internal/boot`, `internal/handshake`, `internal/login`, `internal/worldentry` — connection and boot-path flow
- `internal/worldruntime` — topology, maps, AOI/visibility, entities, and session routing
- `internal/player`, `internal/inventory`, `internal/itemstore` — early character and item systems
- `internal/staticstore`, `internal/interactionstore`, `internal/contentbundle` — authored content and bootstrap runtime data
- `internal/minimal` — current integrated game runtime used by tests and daemons
- `docs/` — engineering notes, QA, roadmaps, workflow, and development docs
- `spec/protocol/` — owned protocol contracts and packet inventory

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

Useful docs:
- [Development guide](docs/development.md)
- [Debugging and profiling](docs/debugging-and-profiling.md)
- [Manual client QA checklist](docs/qa/manual-client-checklist.md)
- [Testing strategy](docs/testing-strategy.md)
- [Workflow](docs/workflow.md)
- [Clean-room policy](docs/clean-room-policy.md)

## Clean-room rule

This repository must only contain code, documentation, fixtures, and tests produced for this project.

Do not copy legacy Metin2 server/client source into this repository. Use legacy behavior only as an external oracle for independently written specs, tests, and Go implementations.
