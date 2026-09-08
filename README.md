<div align="center">

# go-metin2-server

**Rebuilding the Metin2 server experience in Go — one playable milestone at a time.**

[![CI](https://github.com/MikelCalvo/go-metin2-server/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/MikelCalvo/go-metin2-server/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![Stage](https://img.shields.io/badge/stage-pre--alpha-orange)](#the-road-to-a-playable-server)
[![Approach](https://img.shields.io/badge/approach-clean--room-6366f1)](docs/clean-room-policy.md)

[Progress](#the-road-to-a-playable-server) · [What's working](#whats-working-today) · [How we build](docs/agent-workflow.md) · [Get involved](#follow-or-contribute)

</div>

---

## A familiar world, a new foundation

An independent, clean-room Metin2 server emulator written in Go, targeting **TMP4-era client compatibility**. The goal is to rebuild the server step by step, with readable code, documented behavior, and tests behind each new piece of gameplay.

**The next destination:** a small, coherent PvE experience — enter the world, fight mobs, collect rewards, use your gear, visit NPCs, and come back without losing progress.

> **Pre-alpha, not a ready-to-host game server.** Parts of that loop already exist in code and automated tests, but full client compatibility, broad content, and production readiness are still ahead. This repository does not distribute a game client or proprietary game assets.

## The road to a playable server

```text
  ✅ Connect       🟡 Build the loop      ◇ Grow the world      ◇ Go live
  ────────────────●───────────────────────────────────────────────────→
  Login & entry    WE ARE HERE            Richer gameplay       Production
```

| Milestone | Status | What it means |
| :--- | :--- | :--- |
| **01 · Open the gates** | ✅ Foundation in place | Secure connection, login, character selection, and world entry. |
| **02 · Share the world** | 🟡 Working, still growing | Players can see each other, move, chat, and travel through supported map flows. |
| **03 · Complete the PvE loop** | 🟡 Current focus | Bring mobs, combat, loot, equipment, NPC services, and recovery together. |
| **04 · Expand the adventure** | ◇ Ahead | Richer quests, skills, social systems, and broader gameplay compatibility. |
| **05 · Make it ready to host** | ◇ Ahead | Production data storage, operations, releases, and wider compatibility testing. |

*Milestones overlap: persistence and tooling are being developed alongside gameplay. These are direction markers, not release dates or completion percentages.*

## What's working today

These are **limited pre-alpha implementations**, not claims of complete legacy parity.

| | In the current `main` branch |
| :--- | :--- |
| 🌍 **Shared world** | Player visibility, movement, chat, map transfers, and reconnect handling. |
| ⚔️ **First PvE encounters** | Authored practice mobs, basic aggro/chase/return behavior, attacks, death, respawn, and rewards. |
| 🎒 **Items & economy** | Inventory, equipment, potions, loot pickup, NPC buying/selling, and early trade, storage, refining, and player-shop paths. |
| 🏘️ **NPCs & content** | Importable content bundles, dialogue, travel, shops, warehouse services, crafting, and early quest-state behavior. |
| 💾 **Recovery & tooling** | File-backed progress, tested restart scenarios, backup/restore tools, and SQL migration/import tooling. |

**Still ahead:** a full quest scripting system, skills and broader combat rules, complete party/guild systems, production database-backed gameplay, and large-scale server operation.

### What we're working toward next

- **Make the loop feel connected:** fewer isolated features, more complete player journeys.
- **Keep progress safe:** item integrity, rewards, death/restart, and reconnect recovery.
- **Make it provable in the real client:** repeatable manual QA alongside automated tests.

For implementation detail, see the [living PvE roadmap](docs/plans/2026-08-08-playable-vertical-roadmap.md) and [manual client checklist](docs/qa/manual-client-checklist.md).

## Built in the open, with AI agents

We use **Hermes Agent** to coordinate five development lanes: world, combat, items, content, and persistence. Each works in its own Git worktree; a separate integrator checks and brings completed changes into `main`.

**Agents write code and tests. Maintainers set direction and validate the player experience.** Automated integration is not a claim that every commit has had human review, and passing tests is not the same as a finished game.

→ **[Read how our agent workflow works](docs/agent-workflow.md)** — roles, checks, automation, and human oversight.

## Follow or contribute

- ⭐ **Star the repository** to bookmark the project; use **Watch** for GitHub notifications.
- 🧭 **Follow progress:** [milestones above](#the-road-to-a-playable-server), [development history](https://github.com/MikelCalvo/go-metin2-server/commits/main/), and [CI runs](https://github.com/MikelCalvo/go-metin2-server/actions).
- 🐛 **Share useful findings:** reproducible client behavior, focused bug reports, and clear expected vs. actual results help more than broad parity requests. [Open an issue](https://github.com/MikelCalvo/go-metin2-server/issues).
- 🛠️ **Contribute a small improvement:** start with the [development guide](docs/development.md), [workflow](docs/workflow.md), and [clean-room policy](docs/clean-room-policy.md). Keep changes focused and include tests where applicable.

<details>
<summary><strong>Developer corner · setup, tests, and technical references</strong></summary>

Requires **Go 1.26**. Start with the [development guide](docs/development.md) for configuration and local daemon setup; this is not a turnkey server installation.

```bash
git clone https://github.com/MikelCalvo/go-metin2-server.git
cd go-metin2-server
make test
go vet ./...
```

- [Development & configuration](docs/development.md)
- [Testing strategy](docs/testing-strategy.md) · [Manual client QA](docs/qa/manual-client-checklist.md)
- [Protocol reference](spec/protocol/README.md)
- [Debugging & profiling](docs/debugging-and-profiling.md)
- [Lab deployment](docs/workflow/lab-deployment-topology.md) · [Release/versioning notes](docs/workflow/release-versioning.md)

Operator/debug endpoints must remain loopback-only; do not expose them publicly.

</details>

---

**Clean-room commitment:** project-owned code, specs, fixtures, and tests only. Legacy behavior may inform independent implementations; legacy server/client source must not be copied into this repository. See the [full policy](docs/clean-room-policy.md).
