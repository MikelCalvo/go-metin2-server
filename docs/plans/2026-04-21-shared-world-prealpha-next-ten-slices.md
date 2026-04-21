# Shared-world pre-alpha next ten slices implementation plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: take `go-metin2-server` from the newly extracted bootstrap topology slice into a cleaner shared-world pre-alpha by closing the first owned transfer/warp boundary, extracting visibility into explicit runtime primitives, and then separating live player runtime from persisted bootstrap snapshots.

Architecture: finish the remaining M1 shared-world work before jumping into inventory/combat. First close the transfer path on top of `internal/worldruntime/topology.go`, then move visibility into a dedicated runtime helper with an AOI boundary, then introduce a live player runtime and a generic entity registry so the project can enter M2 without another conceptual rewrite.

Tech stack: Go 1.26, current `internal/minimal`, `internal/worldruntime`, bootstrap transfer ops on `gamed`, protocol docs under `spec/protocol/`, plans under `docs/plans/`, and existing full-suite validation with `go test ./...` + `go vet ./...`.

Current starting point:
- latest integrated slice: `7e5159c refactor: extract bootstrap world topology`
- topology decisions now live in `internal/worldruntime/topology.go`
- M1 is still open because warp/transfer, visibility ownership, and runtime identity boundaries are not fully frozen yet

---

## Ordering and scope rules

1. Do not start inventory, equipment, combat, NPC, or quest work from this plan.
2. Keep each task as one public slice with docs + tests + code together.
3. Prefer spec/docs first, then focused failing tests, then minimal implementation.
4. Keep `internal/minimal` working while new runtime boundaries are introduced.
5. Only claim progress on M1/M2 when backed by repo-owned docs and automated tests.

---

## Task 1: Freeze the first self-facing transfer reply contract

Objective: stop treating map transfer as a server-only mutation and document the first honest client-visible contract for the moved player.

Files:
- Read: `spec/protocol/bootstrap-map-transfer-contract.md`
- Read: `spec/protocol/exact-position-bootstrap-transfer-trigger.md`
- Read: `spec/protocol/map-relocation-visibility-rebuild.md`
- Read: `spec/protocol/client-game-entry-sequence.md`
- Create: `spec/protocol/map-transfer-bootstrap.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `README.md`

Steps:
1. Define the first project-owned self-session contract for a successful bootstrap transfer.
2. Decide explicitly whether the current “no immediate self ack, only visibility deltas” behavior remains the owned temporary contract or whether one minimal self-facing transfer reply should be frozen next.
3. Document exact non-goals: no inter-channel migration, no final loading-screen choreography, no reconnect semantics.
4. Update the packet matrix so transfer-related behavior stops living only in narrative docs.
5. Commit.

Verification:
- docs match current repo reality and current transfer implementation boundaries
- all referenced files and packet names exist after the change

---

## Task 2: Add failing tests for end-to-end bootstrap transfer between maps

Objective: prove the current transfer path from gameplay trigger to visible-world rebuild in one focused regression slice.

Files:
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory_test.go`
- Optionally create: `internal/warp/flow_test.go`

Steps:
1. Add a failing test for a selected player transferring from one effective `MapIndex` to another and receiving the currently owned self-session result.
2. Add a failing test that proves old-map peers lose visibility and destination-map peers gain visibility.
3. Add a failing test that proves future movement/chat scope follows the destination map after transfer.
4. Run the focused package tests and confirm failure for the expected reason.
5. Commit only after the tests are in place and red.

Verification:
- `go test ./internal/minimal -run Transfer -count=1`

---

## Task 3: Introduce a minimal `internal/warp` flow boundary

Objective: stop invoking transfer behavior as an ad-hoc branch inside gameplay handlers and give warp/transfer its own package boundary.

Files:
- Create: `internal/warp/flow.go`
- Create: `internal/warp/flow_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `README.md`

Steps:
1. Create a small `internal/warp` package that accepts the selected-character snapshot plus destination and returns the currently owned transfer result.
2. Route the exact-position transfer trigger through that package instead of inlining transfer logic inside gameplay handlers.
3. Keep the external behavior unchanged; this slice is about ownership and boundaries, not new functionality.
4. Re-run the transfer-focused tests until green.
5. Commit.

Verification:
- `go test ./internal/warp ./internal/minimal -count=1`

---

## Task 4: Harden persistence-before-commit transfer semantics

Objective: make transfer persistence guarantees explicit and regression-tested instead of incidental.

Files:
- Modify: `internal/minimal/factory.go`
- Modify: `internal/accountstore/*` as needed
- Modify: `internal/minimal/factory_test.go`
- Modify: `spec/protocol/bootstrap-map-transfer-contract.md`

Steps:
1. Add failing tests for transfer commit rejection when persistence cannot be applied.
2. Prove that failed persistence does not partially mutate runtime visibility or map occupancy.
3. Keep the success path explicit: persist destination snapshot first, then commit runtime transfer.
4. Update the transfer contract doc so failure/success guarantees are precise.
5. Commit.

Verification:
- `go test ./internal/minimal ./internal/accountstore -count=1`

---

## Task 5: Extract visibility ownership into `internal/worldruntime/visibility.go`

Objective: stop keeping visibility math as a side effect of the shared-world registry and move it into a dedicated runtime helper.

Files:
- Create: `internal/worldruntime/visibility.go`
- Create: `internal/worldruntime/visibility_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `spec/protocol/README.md`
- Create: `spec/protocol/visibility-rebuild.md`

Steps:
1. Move visible-peer computation into `internal/worldruntime/visibility.go`.
2. Keep the first implementation simple: same local channel + same effective map, backed by the topology object.
3. Cover enter, leave, relocate, and reconnect scenarios with tests.
4. Update docs so visibility ownership is explicit and no longer implied by registry internals.
5. Commit.

Verification:
- `go test ./internal/worldruntime ./internal/minimal -count=1`

---

## Task 6: Replace inline visibility side effects with explicit visibility diffs

Objective: make callers ask for “what changed” instead of open-coding adds/deletes/bursts in multiple places.

Files:
- Modify: `internal/worldruntime/visibility.go`
- Modify: `internal/worldruntime/visibility_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Introduce a visibility-diff result type that can describe removed peers, added peers, and self-facing visibility deltas.
2. Refactor shared-world join/leave/transfer paths to consume that diff instead of rebuilding frame decisions inline.
3. Keep packet emission behavior unchanged while simplifying ownership.
4. Add tests that prove the new helper returns the right diff for enter/leave/relocate paths.
5. Commit.

Verification:
- `go test ./internal/worldruntime ./internal/minimal -run Visibility -count=1`

---

## Task 7: Add the first AOI boundary with a whole-map implementation

Objective: introduce an abstraction boundary for future range/sector culling without changing current same-map visibility behavior yet.

Files:
- Modify: `internal/worldruntime/visibility.go`
- Modify: `internal/worldruntime/visibility_test.go`
- Modify: `internal/worldruntime/topology.go`
- Modify: `README.md`
- Modify: `spec/protocol/visibility-rebuild.md`

Steps:
1. Add a minimal AOI/visibility-policy interface or helper boundary.
2. Implement the first policy as the current whole-map rule so behavior stays stable.
3. Ensure callers no longer assume visibility is always computed directly from map equality.
4. Document clearly that AOI exists as an abstraction only; real range culling is still deferred.
5. Commit.

Verification:
- `go test ./internal/worldruntime ./internal/minimal -count=1`

---

## Task 8: Create a live player runtime model separate from persisted snapshots

Objective: stop treating `loginticket.Character` snapshots as the same conceptual object as the live in-world player.

Files:
- Create: `internal/player/runtime.go`
- Create: `internal/player/runtime_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/worldentry/flow.go`
- Modify: `README.md`

Steps:
1. Define a player-runtime struct containing live session/world state separate from persisted bootstrap character data.
2. Keep the minimal fields only: identity, effective world position, selected session linkage, and access to the persisted snapshot.
3. Add tests that prove live runtime state can change without redefining the persistence contract itself.
4. Keep external wire behavior stable while reducing conceptual coupling.
5. Commit.

Verification:
- `go test ./internal/player ./internal/minimal ./internal/worldentry -count=1`

---

## Task 9: Attach gameplay/session flows to player runtime objects

Objective: make the runtime operate on live players rather than directly mutating snapshot structs everywhere.

Files:
- Modify: `internal/minimal/factory.go`
- Modify: `internal/worldentry/flow.go`
- Modify: `internal/game/*` as needed
- Modify: `internal/player/runtime.go`
- Modify: related tests under `internal/minimal` and `internal/worldentry`

Steps:
1. Route selection and enter-game so they attach the session to a live player-runtime object.
2. Make movement, sync, chat, and transfer paths use player-runtime state as their live source of truth.
3. Keep persistence updates explicit and narrow instead of letting runtime writes silently stand in for persistence.
4. Re-run existing M0/M1 tests to prove no boot-path regression.
5. Commit.

Verification:
- `go test ./internal/minimal ./internal/worldentry ./internal/game -count=1`
- `go test ./...`

---

## Task 10: Introduce the first generic entity registry in `internal/worldruntime`

Objective: prepare the repo for non-player world actors without another rewrite of player visibility/ownership later.

Files:
- Create: `internal/worldruntime/entity.go`
- Create: `internal/worldruntime/entity_registry.go`
- Create: `internal/worldruntime/entity_registry_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/player/runtime.go`
- Modify: `README.md`

Steps:
1. Add a generic entity identity model that can hold players first and NPCs/mobs later.
2. Move player registration in the shared-world runtime through the entity registry instead of direct session bookkeeping.
3. Keep behavior player-only in this slice; the point is the abstraction boundary, not NPC functionality yet.
4. Add tests proving the registry can register, remove, and look up player entities safely.
5. Commit.

Verification:
- `go test ./internal/worldruntime ./internal/minimal ./internal/player -count=1`

---

## Recommended execution grouping

- **Slices 1-4**: close the remaining M1 transfer/warp boundary
- **Slices 5-7**: move visibility ownership into reusable world-runtime primitives
- **Slices 8-10**: begin M2 by separating live player/entity runtime from bootstrap snapshots

---

## Global validation rule for every slice in this plan

Before marking any slice complete:
- run the narrow package tests for the touched area first
- run `go test ./...`
- run `go vet ./...`
- keep docs and code in the same commit
- keep the tree clean before starting the next slice

## Ready-to-start next slice

If work starts immediately, begin with **Task 1: Freeze the first self-facing transfer reply contract**.
That is the smallest next slice that keeps the current roadmap honest and unlocks the rest of the warp/visibility work without prematurely jumping into inventory or combat.
