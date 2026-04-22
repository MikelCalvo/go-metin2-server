# World Runtime — Next 10 Slices After Leave Hardening

> **For Hermes:** use test-driven-development. Keep slices tiny, keep docs/tests/code in the same slice, and push one commit at a time.

**Goal:** take `go-metin2-server` from partial teardown hardening into reconnect-safe runtime ownership, the first real AOI policy beyond whole-map visibility, and the first non-player entity scaffolding without opening combat, quests, or inventory yet.

**Architecture:** finish the remaining M2 runtime seams before any M3 character-state work. The next work should (1) freeze and harden disconnect/reconnect semantics across `player`, `entity`, `map`, and `session` ownership, (2) replace the current whole-map-only visibility assumption with an explicit opt-in AOI policy boundary owned by `internal/worldruntime`, and (3) introduce static non-player entity identity/map-presence scaffolding without changing public wire behavior yet.

**Tech stack:** Go 1.26, current `internal/minimal`, `internal/player`, `internal/worldruntime`, `internal/worldentry`, `internal/warp`, bootstrap file-backed persistence in `internal/accountstore` and `internal/loginticket`, protocol docs under `spec/protocol/`, and repo-wide validation with `go test ./...` + `go vet ./...`.

**Current starting point:**
- latest runtime slice integrated on `main`: `69f44e0 fix: harden shared-world leave cleanup`
- latest repository head: `d19a092 test: fix pipeline regressions`
- M2 already owns: live selected-player runtime, first entity registry, player directory, map index, session directory, transfer rebootstrap burst, and idempotent partial-teardown leave cleanup
- the biggest remaining M2 gaps are now: reconnect/rejoin semantics across all runtime indexes, real AOI/radius policy support, topology-owned social scope queries, and the first non-player entity scaffolding

---

## Ordering and scope rules

1. Finish reconnect/AOI/non-player runtime ownership before inventory, equipment, combat, mobs, NPC AI, or quests.
2. Keep every slice public and self-contained: docs first where behavior is being frozen, then failing tests, then the minimum green implementation.
3. Keep `internal/minimal` working while new runtime seams continue moving into `internal/worldruntime`.
4. Keep persistence explicit; live runtime mutation must never silently replace persisted account state.
5. Avoid multi-channel networking, DB-backed persistence, scripting, or full content systems in this 10-slice window.
6. Treat non-player work in this window as identity + map presence scaffolding only, not visible combat/content behavior yet.

---

## Slice 1: Freeze reconnect and teardown runtime contract

**Objective:** document exactly what disconnect, close, transfer-then-close, and reconnect are supposed to do now that `player`, `entity`, `map`, and `session` ownership are split.

**Files:**
- Read: `README.md`
- Read: `spec/protocol/entity-runtime-bootstrap.md`
- Read: `spec/protocol/transfer-rebootstrap-burst.md`
- Create: `spec/protocol/runtime-reconnect-cleanup.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Steps:**
1. Define the owned cleanup order for close/disconnect:
   - transport hook removal
   - entity/player/map index cleanup
   - idempotent repeated close behavior
2. Define what reconnect means in bootstrap terms:
   - persisted snapshot remains the source of truth for a fresh session
   - live runtime state is rebuilt, not resumed from stale in-memory pointers
   - reconnect after transfer must not leave stale map/session ownership behind
3. Record the current non-goals explicitly:
   - no final reconnect UX contract beyond runtime correctness
   - no inter-channel recovery
   - no auth/session resumption token design
4. Register the new doc in `spec/protocol/README.md` and align README language for M2.
5. Commit.

**Verification:**
- referenced docs exist and do not contradict current transfer/visibility docs
- README milestone language still matches current code reality

---

## Slice 2: Add failing tests for reconnect-safe cleanup across all runtime indexes

**Objective:** prove the repo still lacks full reconnect/rejoin guarantees even after partial `Leave(...)` hardening.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/worldruntime/player_directory_test.go`
- Modify: `internal/worldruntime/map_index_test.go`
- Modify: `internal/worldruntime/session_directory_test.go`

**Test shapes to add first:**
```go
func TestSharedWorldRegistryCloseRemovesPlayerFromAllRuntimeIndexes(t *testing.T)
func TestGameRuntimeReconnectRebuildsCleanRuntimeState(t *testing.T)
func TestGameRuntimeTransferThenCloseLeavesNoStaleSessionHooks(t *testing.T)
```

**Steps:**
1. Add a RED test proving a connected player removed via close leaves no stale session-directory entry.
2. Add a RED test proving map occupancy drops back to the correct counts after close.
3. Add a RED test proving reconnecting the same login/character creates exactly one live runtime entry and one map occupant.
4. Add a RED test proving transfer followed by close does not leave stale relocate callbacks or fanout sinks behind.
5. Run the focused tests and confirm RED for the expected ownership gap.
6. Commit only after the tests are red.

**Verification:**
- `go test ./internal/worldruntime ./internal/minimal -run 'Test.*Reconnect|Test.*Close|Test.*Leave|Test.*Transfer' -count=1`

---

## Slice 3: Implement reconnect-safe cleanup and rejoin semantics

**Objective:** make disconnect/reconnect flows remove and rebuild runtime ownership exactly once across player/entity/map/session boundaries.

**Files:**
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/worldruntime/entity_registry.go`
- Modify: `internal/worldruntime/player_directory.go`
- Modify: `internal/worldruntime/map_index.go`
- Modify: `internal/worldruntime/session_directory.go`
- Modify: `README.md`

**Implementation sketch:**
```go
// exact names can differ; keep the boundary small
func (r *sharedWorldRegistry) Leave(id uint64)
func (r *sharedWorldRegistry) Join(character loginticket.Character, sink FrameSink, relocate Relocator) (uint64, []loginticket.Character)
```

**Steps:**
1. Ensure close/disconnect removes the same player from every runtime index exactly once.
2. Keep repeated close/leave idempotent.
3. Ensure reconnect rebuilds runtime state from persisted snapshot + fresh session hooks, not from stale in-memory ownership.
4. Ensure transfer-then-close tears down destination ownership correctly.
5. Rerun the RED tests until green.
6. Commit.

**Verification:**
- `go test ./internal/worldruntime ./internal/minimal -run 'Test.*Reconnect|Test.*Close|Test.*Leave|Test.*Transfer' -count=1`
- `go test ./...`
- `go vet ./...`

---

## Slice 4: Introduce a reusable live-position value object in `internal/worldruntime`

**Objective:** stop spreading raw `(MapIndex, X, Y)` tuples across player/entity/AOI code before geometry grows.

**Files:**
- Create: `internal/worldruntime/position.go`
- Create: `internal/worldruntime/position_test.go`
- Modify: `internal/player/runtime.go`
- Modify: `internal/worldruntime/entity.go`
- Modify: `internal/worldruntime/map_index.go`

**API sketch:**
```go
type Position struct {
    MapIndex uint32
    X        int32
    Y        int32
}

func NewPosition(mapIndex uint32, x, y int32) (Position, bool)
func (p Position) SameMap(other Position) bool
```

**Steps:**
1. Add the smallest value object needed for AOI work: map identity + coordinates.
2. Add tests for zero/invalid map handling, equality, and same-map comparisons.
3. Thread the new type through the live player runtime and any runtime entity helpers that currently carry loose tuples.
4. Keep the first API tiny; do not over-design pathfinding or vector math.
5. Commit.

**Verification:**
- `go test ./internal/worldruntime ./internal/player -run 'TestPosition|TestRuntime' -count=1`

---

## Slice 5: Add failing tests for radius and sector AOI helpers

**Objective:** freeze the first real AOI behavior in tests before changing visibility ownership.

**Files:**
- Create: `internal/worldruntime/aoi_test.go`
- Modify: `internal/worldruntime/visibility_test.go`
- Modify: `internal/worldruntime/topology_test.go`

**Test shapes to add first:**
```go
func TestRadiusVisibilityPolicyAllowsPeersInsideRadius(t *testing.T)
func TestRadiusVisibilityPolicyRejectsPeersOutsideRadius(t *testing.T)
func TestSectorKeyIsStableForCoordinates(t *testing.T)
```

**Steps:**
1. Add a RED test for same-map players inside a small radius being visible.
2. Add a RED test for same-map players outside that radius not being visible.
3. Add a RED test for deterministic sector-key calculation from position.
4. Keep the current whole-map policy untouched in this slice.
5. Run focused tests and confirm RED.
6. Commit only after the tests are red.

**Verification:**
- `go test ./internal/worldruntime -run 'TestAOI|TestVisibility|TestBootstrapTopology' -count=1`

---

## Slice 6: Implement the first bootstrap AOI helper and opt-in radius policy

**Objective:** add a real AOI implementation beside `WholeMapVisibilityPolicy` without forcing every caller to use it yet.

**Files:**
- Create: `internal/worldruntime/aoi.go`
- Create: `internal/worldruntime/aoi_test.go`
- Modify: `internal/worldruntime/visibility.go`
- Modify: `internal/worldruntime/topology.go`
- Modify: `README.md`

**API sketch:**
```go
type SectorKey struct {
    MapIndex uint32
    SX       int32
    SY       int32
}

type RadiusVisibilityPolicy struct {
    Radius     int32
    SectorSize int32
}

func (p RadiusVisibilityPolicy) CanSee(topology BootstrapTopology, subject, peer loginticket.Character) bool
```

**Steps:**
1. Implement deterministic sector/radius helpers using the new `Position` type.
2. Add a radius-based visibility policy that still respects topology map identity first.
3. Keep `WholeMapVisibilityPolicy` as the default bootstrap behavior.
4. Do not change caller-visible wire behavior in this slice.
5. Rerun the RED tests until green.
6. Commit.

**Verification:**
- `go test ./internal/worldruntime -run 'TestAOI|TestVisibility' -count=1`

---

## Slice 7: Let bootstrap topology carry AOI configuration explicitly

**Objective:** move AOI selection from ad hoc tests into a first owned topology-level config surface.

**Files:**
- Modify: `internal/worldruntime/topology.go`
- Modify: `internal/worldruntime/topology_test.go`
- Modify: `spec/protocol/visibility-rebuild.md`
- Modify: `README.md`

**Steps:**
1. Add a tiny topology configuration surface for choosing whole-map vs radius AOI policy.
2. Keep defaults backward-compatible.
3. Document clearly that AOI remains bootstrap, local-process, and player-only.
4. Add tests proving topology returns the expected policy in both default and opt-in cases.
5. Commit.

**Verification:**
- `go test ./internal/worldruntime -run 'TestBootstrapTopology|TestAOI|TestVisibility' -count=1`

---

## Slice 8: Route visibility diffs and local social scope queries through policy-aware worldruntime helpers

**Objective:** stop scattering visibility and local social targeting rules across `internal/minimal/shared_world.go`.

**Files:**
- Create: `internal/worldruntime/scopes.go`
- Create: `internal/worldruntime/scopes_test.go`
- Modify: `internal/worldruntime/visibility.go`
- Modify: `internal/worldruntime/visibility_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/chat-scope-first-hardening.md`
- Modify: `README.md`

**API sketch:**
```go
type Scopes struct {
    Topology BootstrapTopology
    Entities *EntityRegistry
}

func (s Scopes) LocalTalkTargets(subject loginticket.Character) []loginticket.Character
func (s Scopes) ShoutTargets(subject loginticket.Character) []loginticket.Character
func (s Scopes) GuildTargets(subject loginticket.Character) []loginticket.Character
func (s Scopes) PlayerByExactName(name string) (PlayerEntity, bool)
```

**Steps:**
1. Make visibility-diff queries depend only on the configured policy surface.
2. Extract local talking, guild, shout, and exact-name lookup queries into `internal/worldruntime` helpers.
3. Keep external whisper/guild/shout behavior stable unless the configured AOI policy explicitly changes local talking visibility.
4. Add regression tests for visible-world diffs plus each chat family.
5. Commit.

**Verification:**
- `go test ./internal/worldruntime ./internal/minimal -run 'Test.*Visibility|Test.*Chat|Test.*Whisper|Test.*Guild|Test.*Shout|Test.*Relocate' -count=1`
- `go test ./...`

---

## Slice 9: Freeze the first non-player entity bootstrap contract

**Objective:** define the smallest owned non-player runtime surface before any code starts registering non-player actors.

**Files:**
- Create: `spec/protocol/non-player-entity-bootstrap.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Steps:**
1. Define the first owned non-player scope as runtime identity + static map presence only.
2. State clearly what is *not* in scope yet:
   - combat
   - pathing/AI
   - drop tables
   - shops
   - spawn groups
   - client-visible packet fanout
3. Define the minimum fields the runtime will own next for a non-player actor:
   - entity ID / kind
   - map position
   - display/class identifier
   - optional name/template identifier
4. Register the new protocol doc and align README M2 language.
5. Commit.

**Verification:**
- the new doc is precise enough to drive tests for static non-player registration next
- README and protocol index mention non-player scaffolding honestly as a bootstrap seam, not as gameplay support

---

## Slice 10: Introduce static non-player entity registration and map-presence scaffolding

**Objective:** make `internal/worldruntime` capable of owning non-player entity identity and map presence without changing current wire behavior yet.

**Files:**
- Create: `internal/worldruntime/non_player_directory.go`
- Create: `internal/worldruntime/non_player_directory_test.go`
- Modify: `internal/worldruntime/entity.go`
- Modify: `internal/worldruntime/entity_registry.go`
- Modify: `internal/worldruntime/entity_registry_test.go`
- Modify: `internal/worldruntime/map_index.go`
- Modify: `internal/worldruntime/map_index_test.go`
- Modify: `README.md`

**API sketch:**
```go
const EntityKindStaticActor EntityKind = "static_actor"

type StaticEntity struct {
    Entity   Entity
    Position Position
    RaceNum  uint32
    Name     string
}

func (r *EntityRegistry) RegisterStaticActor(actor StaticEntity) (uint64, bool)
func (r *EntityRegistry) StaticActor(id uint64) (StaticEntity, bool)
```

**Steps:**
1. Extend runtime identity so non-player actors can exist beside players.
2. Add a static-actor directory/register surface in `internal/worldruntime` only.
3. Reuse the map index for non-player map presence where that does not complicate player callers.
4. Keep `internal/minimal` wire behavior unchanged in this slice; no visible spawn packets yet.
5. Add tests for register, lookup, remove, and deterministic per-map snapshots.
6. Commit.

**Verification:**
- `go test ./internal/worldruntime -run 'TestEntityRegistry|TestMapIndex|TestNonPlayer' -count=1`
- `go test ./...`
- `go vet ./...`

---

## Recommended execution grouping

- **Slices 1-3**: freeze and harden reconnect/teardown correctness
- **Slices 4-8**: introduce real AOI ownership and route visibility/social scopes through it
- **Slices 9-10**: open the first non-player entity-runtime seam without touching gameplay yet

---

## Global validation rule for every slice in this plan

Before marking any slice complete:
- run the narrow package tests for the touched area first
- run `go test ./...`
- run `go vet ./...`
- keep docs and code in the same commit when behavior/ownership changed
- keep the tree clean before starting the next slice
- push after each completed slice so public CI validates it

## Anti-goals for this 10-slice window

Do **not** do these inside this plan:
- combat loops, targeting, damage, death, or respawn
- NPC AI, mob AI, spawn groups, shops, or quest runtime
- inventory/equipment/item-use packets or persistence
- DB-backed persistence or migrations
- real multi-channel/shard routing
- client-visible non-player spawn/update packet choreography

## Ready-to-start next slice

If work starts immediately, begin with **Slice 1: Freeze reconnect and teardown runtime contract**.
That is the safest next step after `69f44e0`, because it makes the remaining disconnect/reconnect work explicit before we keep extracting AOI and non-player runtime seams on top of it.
