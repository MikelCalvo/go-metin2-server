# Roadmap

## Direction

Build a clean-room Go implementation that reaches useful legacy-client parity through observable, tested player journeys—not copied legacy code or feature-count claims.

## Immediate target: a complete small PvE loop

A real client can reliably log in, select a character, enter a shared map, encounter and defeat supported mobs, receive and use loot, interact with an NPC, and retain the resulting progress across reconnect or restart. Automated coverage and a recorded real-client run must support that claim.

## Deliberately deferred parity

The following are not implied by the current milestone and need separately scoped, evidence-backed work:

- full quest scripting, client quest UI, and event systems;
- skills, classes, combat formulas, and PvP balance beyond the supported loop;
- complete party, guild, social, trade, and player-shop behaviour;
- complete maps, NPC/content catalogues, dungeons, and game events;
- production persistence, security, operations, scaling, and release tooling;
- broad client-version, platform, localization, and load compatibility.

For implementation history and detailed sequencing, see the [playable PvE roadmap](plans/2026-08-08-playable-vertical-roadmap.md) and [master parity roadmap](plans/2026-05-24-master-legacy-parity-roadmap.md).

Progress is demonstrated by focused tickets, tests, and real-client QA—not an automatic stream of micro-slices.
