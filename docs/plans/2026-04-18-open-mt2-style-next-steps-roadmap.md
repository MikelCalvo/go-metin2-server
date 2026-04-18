# Open-mt2-Style Next Steps Roadmap

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: move `go-metin2-server` from a protocol-owned bootstrap runtime into a credible shared-world pre-alpha and then into the first real gameplay systems, following the current repository scan rather than a speculative rewrite fantasy.

Architecture: keep the protocol-first, slice-first discipline already established in the repo, but shift the center of gravity from packet ownership to world ownership. The next work should replace bootstrap shortcuts in a deliberate order: topology and relocation, world/entity runtime, character systems, combat, content runtime, and finally compatibility-grade persistence/ops.

Tech stack: Go 1.26, existing `cmd/authd`, `cmd/gamed`, `internal/*`, `spec/protocol/*`, `docs/plans/*`, and GitHub Actions CI.

---

## Planning principles

1. Keep the current clean-room and protocol-first discipline.
2. Prefer tiny vertical slices with hard exit criteria.
3. Do not start inventory/combat/quests before the world/runtime layer can support them.
4. Keep public repo maturity moving in parallel with technical maturity.
5. Keep bootstrap behavior explicit in docs until each area is replaced by a real system.

## Phase 1 — Shared-world pre-alpha

Objective: replace the remaining bootstrap world shortcuts with explicit world topology and relocation behavior.

### Track 1.1 — World topology ownership

Files likely to change:
- Create: `internal/worldruntime/topology.go`
- Create: `internal/worldruntime/topology_test.go`
- Create: `spec/protocol/world-topology-bootstrap.md`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `README.md`

Tasks:
1. Freeze the first project-owned topology model:
   - channel id
   - map index
   - one local process as current ownership boundary
2. Add failing tests for topology-aware runtime decisions.
3. Introduce a small runtime topology object instead of scattering scope checks in `internal/minimal`.
4. Route existing visibility/chat decisions through that object.
5. Document the resulting contract.

Exit criteria:
- topology decisions are explicit in code and docs
- same-map/same-channel assumptions stop being ad-hoc helper logic

### Track 1.2 — Warp and map transfer

Files likely to change:
- Create: `internal/warp/flow.go`
- Create: `internal/warp/flow_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `spec/protocol/packet-matrix.md`
- Create: `spec/protocol/map-transfer-bootstrap.md`

Tasks:
1. Confirm the first compatible warp/relocation packet path to own next.
2. Add failing tests for player relocation across maps.
3. Implement the minimal transfer path.
4. Persist the new location correctly.
5. Ensure old-map peers lose visibility and new-map peers gain visibility.

Exit criteria:
- a player can relocate from one bootstrap map to another
- visibility is rebuilt correctly after relocation

### Track 1.3 — Visibility rebuild and AOI baseline

Files likely to change:
- Create: `internal/worldruntime/visibility.go`
- Create: `internal/worldruntime/visibility_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Create: `spec/protocol/visibility-rebuild.md`

Tasks:
1. Move visibility decisions into a dedicated helper/module.
2. Add tests for enter, leave, relocate, and reconnect visibility bursts.
3. Add the first AOI/culling abstraction boundary, even if the first implementation still uses a simple whole-map scope.
4. Keep the implementation simple while preventing future rewrites of every caller.

Exit criteria:
- visibility is not a registry side effect only
- callers can ask the visibility layer what changed when a player moves/warps

## Phase 2 — Entity/world runtime foundation

Objective: stop modeling the runtime as mostly session closures and start modeling a reusable world runtime.

### Track 2.1 — Player runtime model

Files likely to change:
- Create: `internal/player/runtime.go`
- Create: `internal/player/runtime_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/worldentry/flow.go`

Tasks:
1. Define a player-runtime struct separate from ticket/account snapshots.
2. Split persisted snapshot state from live session/world state.
3. Make session flows attach to live player runtime objects.
4. Keep the external behavior stable while reducing bootstrap coupling.

Exit criteria:
- live player state and persisted character snapshots are no longer the same conceptual object

### Track 2.2 — World/entity registry

Files likely to change:
- Create: `internal/worldruntime/entity.go`
- Create: `internal/worldruntime/entity_registry.go`
- Create: `internal/worldruntime/entity_registry_test.go`
- Modify: `internal/minimal/shared_world.go`

Tasks:
1. Introduce a generic entity registry abstraction.
2. Move player-visibility registration through entity identity instead of direct session bookkeeping.
3. Define the minimal APIs needed for future NPC/mob insertion.

Exit criteria:
- the world runtime can hold non-player entities without another conceptual rewrite

## Phase 3 — Character systems

Objective: make player characters meaningful beyond location and chat.

### Track 3.1 — Inventory model

Files likely to change:
- Create: `internal/inventory/model.go`
- Create: `internal/inventory/model_test.go`
- Create: `spec/protocol/inventory-bootstrap.md`
- Modify: `internal/player/runtime.go`
- Modify: persistence packages as needed

Tasks:
1. Define inventory slot model and constraints.
2. Add failing tests for storing/loading and mutating inventory state.
3. Introduce the first minimal persistence boundary for inventory.
4. Expose only the smallest compatibility slice needed next.

Exit criteria:
- player runtime owns an inventory model
- inventory survives across fresh sessions

### Track 3.2 — Equipment model

Files likely to change:
- Create: `internal/equipment/model.go`
- Create: `internal/equipment/model_test.go`
- Modify: `internal/inventory/*`
- Modify: `internal/player/runtime.go`
- Create: `spec/protocol/equipment-bootstrap.md`

Tasks:
1. Define equipped-slot model.
2. Add failing tests for equip/unequip rules.
3. Connect equipment to visible player state where necessary.
4. Keep initial equipment effects minimal and explicit.

Exit criteria:
- a player can equip/unequip at least one minimal item path

### Track 3.3 — Item use baseline

Files likely to change:
- Create: `internal/itemuse/flow.go`
- Create: `internal/itemuse/flow_test.go`
- Modify: `internal/inventory/*`
- Modify: protocol docs as required

Tasks:
1. Choose one first item-use slice.
2. Add failing tests for request, state mutation, and persistence.
3. Implement only that slice.

Exit criteria:
- one real item-use loop exists end to end

## Phase 4 — Combat vertical slice

Objective: prove a first real gameplay loop.

### Track 4.1 — Targeting and attack request

Files likely to change:
- Create: `internal/combat/targeting.go`
- Create: `internal/combat/targeting_test.go`
- Create: `internal/combat/attack_flow.go`
- Create: `internal/combat/attack_flow_test.go`
- Create: `spec/protocol/combat-first-slice.md`

Tasks:
1. Define minimal target ownership rules.
2. Add failing tests for target selection.
3. Add failing tests for one attack request path.
4. Implement the narrowest possible vertical slice.

### Track 4.2 — HP, damage, death, respawn

Files likely to change:
- Create: `internal/combat/stats.go`
- Create: `internal/combat/damage.go`
- Create: `internal/combat/death.go`
- Create: tests for each
- Modify: `internal/player/runtime.go`

Tasks:
1. Add HP to live character runtime.
2. Add damage application.
3. Add death state and respawn baseline.
4. Persist only the state that actually needs persistence.

Exit criteria for Phase 4:
- at least one minimal combat loop exists against a testable target path

## Phase 5 — NPCs, mobs, and content runtime

Objective: move from player-only runtime to first real world content.

### Track 5.1 — NPC placeholders

Files likely to change:
- Create: `internal/npc/model.go`
- Create: `internal/npc/model_test.go`
- Modify: `internal/worldruntime/entity_registry.go`
- Create: `spec/protocol/npc-visibility-bootstrap.md`

Tasks:
1. Add non-player entity insertion.
2. Make NPCs visible in the shared world.
3. Keep behavior read-only at first.

### Track 5.2 — Mob placeholders and spawn groups

Files likely to change:
- Create: `internal/mob/model.go`
- Create: `internal/mob/spawn.go`
- Create: tests
- Modify: world runtime packages

Tasks:
1. Define mob runtime identity.
2. Define spawn-group bootstrap format.
3. Add visibility and lifecycle tests.

### Track 5.3 — Shops or another first NPC interaction loop

Files likely to change:
- Create new interaction package(s)
- Add protocol docs and tests

Exit criteria for Phase 5:
- the runtime contains persistent non-player world actors and at least one interaction loop

## Phase 6 — Social systems beyond bootstrap

Objective: replace bootstrap social shortcuts with real systems.

### Tracks
- real party membership/state
- real guild roster/lifecycle state
- operator/admin permissions beyond loopback-only actions
- shout scope aligned with actual topology

Files likely to change:
- Create: `internal/party/*`
- Create: `internal/guild/*`
- Modify: `internal/minimal/*` or their replacements
- Update social protocol docs

Exit criteria:
- party/guild are no longer "all connected sessions" style bootstrap shortcuts

## Phase 7 — Compatibility-grade persistence and public ops

Objective: move from bootstrap snapshots to a durable backend and public-project operational maturity.

### Track 7.1 — Persistence architecture

Files likely to change:
- Create: `internal/persistence/*`
- Create: DB-backed repository implementations
- Create: migrations under `db/migrations/`
- Create: persistence docs

Tasks:
1. Decide the first durable schema boundaries.
2. Move from flat bootstrap snapshots toward explicit repositories.
3. Add migration tooling and tests.

### Track 7.2 — Observability and operator ergonomics

Files likely to change:
- Modify: `internal/ops/*`
- Create metrics/logging/admin docs
- Update README and development docs

Tasks:
1. Add richer logs/metrics.
2. Add safer operator surfaces where needed.
3. Improve contributor and deployment ergonomics.

Exit criteria for Phase 7:
- persistence and ops are strong enough that outside users can run and debug the server without lab-specific tribal knowledge

## Recommended slice order inside the next milestone window

Immediate next slices should be tackled in this order:

1. world topology contract
2. warp/map transfer slice
3. visibility rebuild slice
4. player runtime separation from persisted snapshot state
5. entity registry baseline
6. only then start inventory/equipment or combat

## Validation discipline for every future slice

For each slice:

1. update the spec first
2. add the failing test first
3. run the test and confirm the correct red state
4. implement the minimum green change
5. run focused tests
6. run `go test ./...`
7. run `go vet ./...`
8. update docs/README when global status changes
9. commit a single coherent unit
10. push so CI validates it publicly

## Anti-goals for the next phase

Do not do these early:

- a giant "world rewrite" branch
- inventory/equipment/combat all at once
- quest runtime before NPC/mob/world layers exist
- broad protocol expansion without runtime payoff
- hidden private fixes that bypass project-owned docs/tests

## Success definition for the next major checkpoint

The next major checkpoint should look like this:

- players can relocate between bootstrap maps
- visibility rebuilds correctly after relocation
- the world runtime has clearer ownership boundaries
- the README global matrix reflects progress without slice-by-slice archaeology
- the public repo feels like a guided project, not just a stream of packet slices
