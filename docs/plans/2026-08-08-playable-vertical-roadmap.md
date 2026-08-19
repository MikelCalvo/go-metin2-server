# Playable PvE Vertical Roadmap — 2026-08-08

This roadmap replaces the May 2026 broad parity map as the current planning anchor. The project should still move toward legacy parity, but the next useful milestone is narrower: make the server feel like a small, inspectable, recoverable PvE game loop.

This is not permission for a large rewrite. Keep landing small, verified slices and keep `main` green.

## Guiding rules

1. Keep `main` green and linear.
2. Keep one cohesive commit per slice.
3. Keep docs/specs/tests aligned with behavior.
4. Treat legacy source and captures only as external behavior oracles.
5. Prefer client-visible behavior over generic refactors.
6. Prefer runtime behavior that can be inspected by local ops endpoints while the system is still bootstrap-shaped.
7. Do not claim full legacy parity when a slice is only a practice-mob, file-store, or local-debug seam.

## Milestone goal

A narrow playable vertical means a player can:

1. connect through `authd` and `gamed`,
2. enter a map,
3. see authored NPC/static content and content-loaded mobs,
4. move and keep visibility stable,
5. target and fight mobs,
6. kill mobs and receive deterministic rewards,
7. pick up, use, equip, trade/reject, or sell basic items through owned paths,
8. reconnect/restart without corrupting state,
9. let an operator inspect and recover bootstrap stores through local-only tools.

Exit criteria for this milestone:

- A manual client QA pass can exercise login → map → mob kill → reward → pickup/use/equip/shop → reconnect/restart without relying primarily on slash/debug paths.
- Automated tests cover the same contract at protocol, runtime, and persistence boundaries.
- Unsupported behaviors fail closed and are documented.
- Local operator endpoints can validate the relevant state before and after the run.

## Track A — Mob AI and spawn lifecycle

**Objective:** turn stationary practice spawns into the first believable PvE mob lifecycle.

Primary lane:

- `lane/world` / `go-metin2-mob-spawn-lifecycle-worker`

Likely areas:

- `internal/worldruntime`
- `internal/minimal`
- `internal/contentbundle`
- `internal/staticstore`
- `spec/protocol/non-player-entity-bootstrap.md`
- `spec/protocol/content-spawn-groups-bootstrap.md`
- `spec/protocol/combat-normal-attack-bootstrap.md`
- `docs/qa/manual-client-checklist.md`

Next slices:

1. Finish integrating map-local static actor snapshot coverage already pending on `lane/world`.
2. Make one content-loaded practice mob lifecycle fully explicit: spawn → target → hit → death → respawn → fresh reselection.
3. Add aggro-lite reset/cleanup boundaries for disconnect, transfer, death, and respawn; `/restart_here` now also has focused coverage for due return-step preflight while a zero-HP owner skipped lifecycle frames.
4. Add first independent mob reaction timing that is not only piggybacked on player hits: proximity aggro-radius acquisition now has the pure helper plus pending-frame live consumer, and that same engagement now also arms the owned delayed self-only server-origin retaliation cadence without requiring an accepted hit or inventing selected-target ownership.
5. Extend the first chase/leash/return planning seam beyond pure classification: tested pure return-step and chase-step planners, the pending-frame chase executor (including proximity-armed due chase without a hit), read-only pending chase inspection, retained-viewer chase `MOVE` replication, and retained-viewer same-map return-step / return-home `MOVE` replication now exist; cross-map return-home and respawn rebuild remain on delete/readd.
6. Harden multi-map and reconnect behavior so mobs do not duplicate, leak, or resurrect incorrectly: the content-loaded anti-leak matrix is now frozen (still-dead EnterGame/reconnect trailing `DEAD`, one-ref/one-actor, map-scoped import visibility, cross-map return-home delete/readd without dual-map occupancy, Leave/transfer ownership cleanup); the next honest RED is content-loaded still-dead EnterGame bootstrap coverage mirroring the existing training-dummy proof.

Exit criteria:

- content-loaded mobs round-trip through authored data and runtime state,
- a dead mob stays dead until its server-owned respawn boundary,
- respawn uses the normal visibility rebuild path,
- transfer/reconnect cannot leave stale target or engagement ownership behind,
- docs keep calling this bootstrap/practice AI until movement/pathing is real.

Anti-goals:

- no full pathfinding rewrite,
- no broad formula work in the world lane,
- no distributed channel/shard architecture before single-process semantics are stable.

## Track B — Combat, death, rewards, and formulas

**Objective:** make the PvE loop resolve damage, death, and rewards through owned contracts.

Primary lane:

- `lane/combat` / `go-metin2-combat-worker`

Likely areas:

- `internal/proto/combat`
- `internal/proto/world`
- `internal/worldruntime`
- `internal/minimal`
- `internal/player`
- `spec/protocol/combat-*`
- `spec/protocol/non-player-death-respawn-bootstrap.md`
- `spec/protocol/non-player-reward-bootstrap.md`
- `spec/protocol/player-death-bootstrap.md`

Next slices:

1. Keep target-marker, fly-effect, stun, PvP/duel packet families codec-only until runtime policy is proven.
2. ~~Define one authored combat-profile formula seam for practice mobs without claiming full legacy math.~~ Done for bootstrap scope: portable `combat_profiles` already drive `max(1, attack_value - defense_value)`, and `docs/examples/bootstrap-combat-profile-formula-bundle.json` is the first playable QA fixture (`qa_formula_practice_mob` / `practice.qa_formula_mob`). Full legacy math remains out of scope.
3. Extend reward descriptors from fixed examples toward table-driven EXP/gold/drop data.
4. Harden player-death floor and restart behavior around mob retaliation and reconnect.
5. Add first accepted skill/ranged/projectile runtime only after packet evidence and tests are owned.
6. Defer PvP/duel runtime policy until the PvE baseline is stable.

Exit criteria:

- combat outcome is deterministic, tested, and visible to the client,
- rewards flow into item/gold/EXP systems through existing mutation/persistence seams,
- player death/restart does not corrupt inventory, quickslots, target state, or persistence,
- unsupported combat packet families fail closed instead of producing fake behavior.

Anti-goals:

- no full skill system before normal PvE combat is stable,
- no PvP policy guessing,
- no fake loot/PnL-style evidence; rewards must be backed by tests and docs.

## Track C — Items, economy, trade, and storage foundations

**Objective:** make item/economy behavior useful enough to support the PvE loop.

Primary lane:

- `lane/items` / `go-metin2-items-worker`

Likely areas:

- `internal/player`
- `internal/inventory`
- `internal/itemstore`
- `internal/proto/item`
- `internal/proto/quickslot`
- `internal/minimal`
- `spec/protocol/item-*`
- `spec/protocol/quickslot-bootstrap.md`
- `docs/qa/manual-client-checklist.md`

Next slices:

1. Finish integrating pending exchange anti-give/display-slot guards on `lane/items`.
2. Turn `EXCHANGE` from fail-closed packet ownership into a staged two-party trade plan with RED tests before any mutation.
3. Add ownership timers and pickup permission transitions for player/mob drops.
4. Extend item-template restrictions: class, sex, level, anti-flags, equipment slot policy, and edge-case feedback.
5. Add storage/safebox planning and the smallest fail-closed packet/runtime seam.
6. Continue refine as fail-closed until material/cost/result semantics are frozen; then add a tiny accepted refine success path.

Exit criteria:

- PvE rewards can become carried or ground items safely,
- common item uses are visible and persistent,
- trade/storage/refine either work in a narrow tested way or fail closed with documented feedback,
- item mutations update quickslots, peers, and persistence consistently.

Anti-goals:

- no broad player-shop or mall work before carried/ground/trade semantics are stable,
- no guessed item-use families without evidence,
- no item/economy behavior that bypasses template validation.

## Track D — Content, NPC services, quests, regen, and drops

**Objective:** make authored content drive the vertical instead of hardcoded examples.

Primary lane:

- `lane/content` / `go-metin2-content-worker`

Likely areas:

- `internal/contentbundle`
- `internal/staticstore`
- `internal/interactionstore`
- `internal/minimal`
- future quest/content packages
- `spec/protocol/static-actor-*`
- `spec/protocol/npc-*`
- `spec/protocol/content-spawn-groups-bootstrap.md`
- `docs/examples/*`

Next slices:

1. Keep content summaries useful, but stop prioritizing endpoint-only work unless it unblocks QA or migration.
2. Add first quest-state seam: quest flags, NPC dialog state, and one simple persisted trigger/result contract.
3. Add regen/drop table ingestion in a deliberately small authored format.
4. Expand NPC service kinds only when client-visible behavior is owned.
5. Add content validation/canonicalization that catches bad bundles before runtime mutation.
6. Keep deterministic example bundles updated for manual QA.

Exit criteria:

- the PvE vertical can be authored with bundle data,
- one quest-style state can persist and resume,
- spawn/regen/drop definitions are validated before runtime use,
- unsupported service kinds fail early and clearly.

Anti-goals:

- no full quest-script compatibility in one pass,
- no live reload that can half-commit invalid content,
- no content definitions that bypass store validation.

## Track E — Persistence, DB, migrations, and production ops

**Objective:** move from bootstrap file snapshots toward durable, operable service foundations.

Primary lane:

- `lane/persistence` / `go-metin2-db-ops-worker`

Likely areas:

- `db/migrations`
- `internal/accountstore`
- `internal/loginticket`
- `internal/player`
- `internal/itemstore`
- `internal/config`
- `internal/ops`
- `docs/development.md`
- `docs/workflow.md`
- future deployment/runbook docs

Next slices:

1. Extend the new `db/migrations` catalog beyond the initial `schema_migrations` ledger only when a narrow repository/backfill contract is ready; the first point-state export quarantine/preflight now exists for `0011_character_point_state`.
2. Extract narrow repository seams only where tests prove reduced coupling.
3. Write backup/restore runbooks backed by local validation or preflight tests.
4. Harden crash/restart recovery for item/gold/character state used by the PvE loop.
5. Document release/deploy/versioning policy; the first release-identity stamp and loopback `/local/build-info` surface now exist, while deployment topology remains follow-on work.
6. Add production-safe observability conventions before remote admin surfaces.

Exit criteria:

- migrations are repeatable,
- operators can validate and back up/restore key state,
- core gameplay state has a clear path from file-backed bootstrap to DB-backed storage,
- sensitive ops remain loopback/local-only unless an explicit auth design exists.

Anti-goals:

- no big-bang DB migration,
- no secret-bearing production config in git,
- no remote admin API without authentication and threat model.

## Track F — Social systems after the PvE loop

**Objective:** replace bootstrap chat fanout with real social state after the PvE loop has enough gameplay to share.

Likely future areas:

- `internal/proto/chat`
- future party/guild/messenger packages
- `internal/minimal`
- `internal/worldruntime`
- `spec/protocol/party-*`
- `spec/protocol/guild-*`

Next slices after PvE stabilizes:

1. Party membership state.
2. Invite/accept/leave/kick where packet evidence exists.
3. Party chat scoping based on membership rather than bootstrap fanout.
4. Party EXP/drop sharing after reward ownership is stable.
5. Guild roster/rank state.
6. Friend/messenger/block systems.

Exit criteria:

- party/guild chat is membership-scoped,
- gameplay systems can query social state safely,
- membership persistence is either implemented or explicitly scoped.

Anti-goals:

- no social effects before membership exists,
- no claims of real party/guild support while fanout remains bootstrap-shaped.

## Integration and validation model

Autonomous lanes should continue to produce small commits on lane branches. The integrator should be the only job that pushes `origin/main`.

Recommended lane mapping:

- `lane/items` — Track C
- `lane/combat` — Track B
- `lane/world` — Track A
- `lane/content` — Track D
- `lane/persistence` — Track E
- `main` — integration only

Validation expectations:

- focused tests for the touched slice,
- `gofmt` clean,
- `git diff --check`,
- `go test ./...` when the integration environment can complete it reliably,
- `go vet ./...`,
- daemon builds before pushing code changes to `main`.

If the full suite times out in filesystem-heavy `internal/minimal` file-store tests, treat that as an infrastructure/timeout problem to fix deliberately. Do not push code changes that have not passed the agreed integration gate; for documentation-only changes, at minimum validate links, formatting, and repository reality.
