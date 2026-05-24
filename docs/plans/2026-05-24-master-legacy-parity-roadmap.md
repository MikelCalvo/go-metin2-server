# Master Legacy-Parity Roadmap — 2026-05-24

This roadmap is intentionally broader than the per-slice plans in `docs/plans/`. It defines the major work tracks needed to move `go-metin2-server` from its current pre-alpha bootstrap state toward a legacy-compatible Metin2 server.

The project should continue to land small, independently verified slices. This document is a planning map, not permission to batch large rewrites.

## Guiding rules

1. Keep `main` green.
2. Keep one commit per slice.
3. Keep docs/specs/tests aligned with behavior.
4. Treat legacy source and captures only as external behavior oracles.
5. Prefer client-visible compatibility over generic refactors.
6. Parallelize by subsystem lanes, but serialize integration through `main`.

## Track A — Items, inventory, and equipment

Objective: make the item layer feel legacy-compatible enough to support real gameplay loops.

Likely areas:

- `internal/player`
- `internal/inventory`
- `internal/proto/item`
- `internal/proto/quickslot`
- `internal/minimal` item runtime tests
- `spec/protocol/item-*`
- `docs/qa/manual-client-checklist.md`

Next priorities:

1. Finish nearby `ITEM_USE_TO_ITEM` edge cases:
   - partial merges,
   - full target rejection,
   - locked source/target rejection,
   - non-stackable rejection,
   - incompatible target behavior.
2. Grow item-use families from evidence:
   - socket/metin interactions,
   - enchant/change bonus style interactions,
   - scrolls/books/consumables.
3. Harden item restrictions:
   - anti-drop,
   - anti-sell,
   - anti-give,
   - class/sex/level restrictions,
   - equipment slot validity.
4. Extend ground item ownership:
   - timers,
   - party ownership,
   - permission transitions,
   - pickup denial feedback where owned.
5. Prepare storage/trade boundaries without implementing them prematurely.

Exit criteria:

- item mutation paths are deterministic, tested, persisted, and visible to the real client,
- common carried/equipped/ground/merchant paths no longer rely on slash/debug seams,
- unsupported legacy item uses fail closed and are documented.

Anti-goals:

- do not implement broad trade/storage/player-shop systems in this track until the carried item semantics are stable,
- do not guess item-use packet behavior without evidence.

## Track B — Combat, mobs, rewards, and death policy

Objective: turn the practice-mob loop into a real PvE gameplay loop.

Likely areas:

- `internal/proto/combat`
- `internal/worldruntime`
- `internal/minimal`
- `internal/player`
- `spec/protocol/combat-*`
- `spec/protocol/player-death-*`
- `docs/qa/manual-client-checklist.md`

Next priorities:

1. Harden target/attack/death/restart regressions around the current practice mob.
2. Add reward seams after mob death:
   - EXP placeholder,
   - gold/yang placeholder,
   - deterministic item drop placeholder.
3. Replace fixed dummy damage with a first authored combat profile:
   - attack value,
   - defense value,
   - HP/max HP,
   - level/rank where needed.
4. Add first mob AI slices:
   - aggro radius,
   - chase/return,
   - attack cadence,
   - leash,
   - target release.
5. Grow player death policy:
   - revive/restart evidence,
   - live/dead recipient rules,
   - persistence split for HP/position,
   - broader peer replay.

Exit criteria:

- a player can kill a spawned mob and receive basic rewards,
- the mob can respawn and re-enter visibility correctly,
- death/restart behavior is deterministic and does not corrupt persisted state.

Anti-goals:

- do not jump to full skill/PvP formulas before the PvE baseline is stable,
- do not persist runtime-only HP loss unless a dedicated persistence-policy slice owns it.

## Track C — World runtime, AOI, maps, spawns, and transfer

Objective: make the world layer robust enough to host real content and multiple concurrent players.

Likely areas:

- `internal/worldruntime`
- `internal/minimal`
- `internal/warp`
- `internal/contentbundle`
- `internal/staticstore`
- `spec/protocol/*visibility*`
- `spec/protocol/*transfer*`
- `docs/debugging-and-profiling.md`

Next priorities:

1. Harden AOI and map-index behavior across movement, sync, transfer, and reconnect.
2. Extend static/non-player actor lifecycle:
   - update in place,
   - relocate,
   - remove,
   - replay dead/alive state,
   - preserve identity where intended.
3. Grow spawn groups:
   - multiple actors,
   - respawn policies,
   - map-specific placement,
   - content validation.
4. Improve operator/runtime snapshots:
   - map occupancy,
   - runtime config,
   - spawn state,
   - selected target state.
5. Prepare multi-channel ownership without disrupting the single-channel bootstrap.

Exit criteria:

- visibility transitions are predictable across maps/AOI/reconnect/death/respawn,
- static and non-player actors can be inspected and updated safely,
- content-loaded spawns no longer behave like one-off bootstrap fixtures.

Anti-goals:

- do not rewrite the world runtime wholesale,
- do not introduce distributed/multi-process ownership before the single-process semantics are stable.

## Track D — NPCs, shops, content, and quests

Objective: move content interactions from narrow NPC/shop slices toward a real content runtime.

Likely areas:

- `internal/interactionstore`
- `internal/staticstore`
- `internal/contentbundle`
- `internal/proto/interact`
- `internal/proto/shop`
- `internal/minimal`
- `spec/protocol/npc-*`
- future quest packages

Next priorities:

1. Expand NPC services with evidence-backed interaction results.
2. Harden merchant open/close/buy/sell edge cases.
3. Add richer shop definitions:
   - multi-tab shops,
   - stock/soldout semantics,
   - price updates,
   - invalid/stale contexts.
4. Introduce first quest runtime seam:
   - quest flags,
   - NPC dialog state,
   - simple trigger/result contract,
   - persistence model.
5. Connect mob kill/item/level events into quest hooks only after the base runtime exists.

Exit criteria:

- content definitions can drive useful NPC/shop behavior without code changes,
- the first quest-style interaction can persist and resume state,
- unsupported service kinds fail early and clearly.

Anti-goals:

- do not attempt full quest-script compatibility in one pass,
- do not let content definitions bypass validation.

## Track E — Social systems

Objective: replace bootstrap chat fanout with real party/guild/messenger systems.

Likely areas:

- `internal/proto/chat`
- `internal/minimal`
- `internal/worldruntime/scopes.go`
- future party/guild/messenger packages
- `spec/protocol/party-*`
- `spec/protocol/guild-*`

Next priorities:

1. Introduce explicit party membership state.
2. Add party invite/accept/leave/kick where packet evidence exists.
3. Connect party membership to chat, pickup, EXP, and drop sharing.
4. Introduce guild roster/rank state.
5. Add friend/messenger/block systems after core party/guild paths are stable.

Exit criteria:

- party/guild chat no longer means bootstrap-global fanout,
- membership is persisted or explicitly scoped,
- gameplay systems can query social membership safely.

Anti-goals:

- do not over-claim real party/guild behavior while fanout remains bootstrap-shaped,
- do not add social effects before membership state exists.

## Track F — Persistence, DB, operations, and release

Objective: move from bootstrap file snapshots to compatibility-grade service operation.

Likely areas:

- stores/repositories under `internal/*store`
- future DB/migration packages
- `internal/ops`
- `internal/config`
- `docs/development.md`
- `docs/workflow.md`
- deployment docs

Next priorities:

1. Define DB schema boundaries for accounts, characters, items, quests, guilds, parties, and world state.
2. Add migrations and repository interfaces without forcing every existing slice to migrate at once.
3. Add backup/restore and crash-recovery policy.
4. Expand operator/admin endpoints cautiously.
5. Define release/deploy workflow and public CI gates.

Exit criteria:

- the server can persist core gameplay state in a DB-backed store,
- migrations are repeatable,
- operators can inspect and recover the service safely,
- public releases have a documented validation path.

Anti-goals:

- do not prematurely convert every file-backed snapshot before gameplay schemas are stable,
- do not expose sensitive operator actions beyond loopback/trusted boundaries without explicit design.

## Parallel development model

The recommended acceleration model is lane-based:

- `lane/items` — Track A
- `lane/combat` — Track B
- `lane/world` — Track C
- optional future `lane/content` or `lane/archaeology` — Tracks D/E/protocol evidence
- `main` — integration only

Worker lanes should produce small commits on their branches. The integrator should merge or cherry-pick one lane at a time into `main`, run full validation, and push only green mainline progress.

This keeps throughput high without letting concurrent agents race on `main`.
