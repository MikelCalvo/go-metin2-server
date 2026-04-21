# World Runtime and Character State — Next 25 Slices

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: take `go-metin2-server` from the first owned player-runtime and entity-registry boundaries into a reusable M2 world runtime, then open M3 with the first owned inventory/equipment character-state slices.

Architecture: finish world-runtime ownership before starting combat, NPCs, or quests. The next work should separate directories and indexes inside `internal/worldruntime`, close the remaining self-session warp/rebootstrap gap, add the first real AOI policy beyond whole-map visibility, and only then begin inventory/equipment state on top of the live `internal/player` runtime.

Tech stack: Go 1.26, current `internal/minimal`, `internal/player`, `internal/worldentry`, `internal/warp`, `internal/worldruntime`, bootstrap file-backed persistence in `internal/accountstore` and `internal/loginticket`, protocol docs under `spec/protocol/`, and full-suite validation with `go test ./...` + `go vet ./...`.

Current starting point:
- latest integrated slice: `21f8aa5 refactor: route shared-world through entity registry`
- M0 is complete and the original 10-slice follow-up plan is closed
- M1 is still partial because transfer self-session choreography, AOI, and topology ownership are still bootstrap
- M2 has started with `internal/player` and the first `internal/worldruntime` entity registry, but session/world/entity ownership is still mixed inside `internal/minimal/shared_world.go`
- M3 has not started yet; inventory/equipment/item-use remain untouched

---

## Ordering and scope rules

1. Finish the remaining M1/M2 world-runtime work before starting combat, mobs, NPCs, or quests.
2. Keep each slice as one public unit: docs/spec, tests, code, README status updates when the global story changes.
3. Prefer spec/docs first, then focused failing tests, then the smallest green implementation.
4. Keep `internal/minimal` working while new runtime boundaries are extracted around it.
5. Keep persistence writes explicit; do not let live runtime mutation silently stand in for persistence.
6. Do not open multi-channel networking or DB-backed persistence in this plan; keep the next 25 slices honest and repo-sized.

---

## Task 11: Freeze the post-bootstrap world-runtime ownership contract

Objective: document the next owned architecture boundary after Tasks 1-10 so later extractions stop being implicit.

Files:
- Read: `README.md`
- Read: `spec/protocol/world-topology-bootstrap.md`
- Read: `spec/protocol/visibility-rebuild.md`
- Read: `spec/protocol/map-transfer-bootstrap.md`
- Create: `spec/protocol/entity-runtime-bootstrap.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

Steps:
1. Define the owned runtime concepts explicitly: live player runtime, entity registry, player directory, map occupancy index, session directory, and AOI policy.
2. Record what is still bootstrap: one process, one local channel, player-only entities, and temporary self-session transfer choreography.
3. State clear non-goals: no combat runtime, no NPC/mob actors, no DB schema, no real shard/channel routing yet.
4. Register the new spec in the protocol index and align the README milestone language.
5. Commit.

Verification:
- docs match the current codebase after Task 10
- all referenced docs and paths exist

---

## Task 12: Add failing tests for indexed player lookup by entity ID, VID, and name

Objective: prove the current entity registry is still too thin for world-runtime callers.

Files:
- Modify: `internal/worldruntime/entity_registry_test.go`
- Optionally create: `internal/worldruntime/player_directory_test.go`

Steps:
1. Add a failing test for exact lookup by entity ID.
2. Add a failing test for exact lookup by `VID`.
3. Add a failing test for exact lookup by character name.
4. Add a failing test that proves removal clears all indexes together.
5. Run the focused tests and confirm RED for missing APIs.
6. Commit only after the tests are red.

Verification:
- `go test ./internal/worldruntime -run 'TestEntityRegistry|TestPlayerDirectory' -count=1`

---

## Task 13: Introduce a dedicated player directory in `internal/worldruntime`

Objective: stop making every world caller rebuild name/VID lookup rules out of raw entity slices.

Files:
- Create: `internal/worldruntime/player_directory.go`
- Create: `internal/worldruntime/player_directory_test.go`
- Modify: `internal/worldruntime/entity_registry.go`
- Modify: `internal/worldruntime/entity_registry_test.go`
- Modify: `README.md`

Steps:
1. Add a small player-directory type that indexes player entities by entity ID, `VID`, and exact name.
2. Keep the first implementation player-only and in-memory.
3. Make registration/removal/update keep all indexes in sync.
4. Keep the public API narrow: register, update, remove, and exact lookup helpers.
5. Re-run the RED tests until green.
6. Commit.

Verification:
- `go test ./internal/worldruntime -run 'TestEntityRegistry|TestPlayerDirectory' -count=1`

---

## Task 14: Route whisper and runtime snapshot lookups through the player directory

Objective: make exact-name and runtime-snapshot lookups consume the new owned directory instead of scanning session maps ad hoc.

Files:
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `README.md`

Steps:
1. Replace exact-name whisper lookup with player-directory access.
2. Replace any connected-player snapshot helpers that still depend on open-coded session scans where directory lookup is a better source.
3. Keep external behavior unchanged.
4. Add regression tests for whisper delivery and runtime snapshot output.
5. Commit.

Verification:
- `go test ./internal/minimal -run 'Test.*Whisper|Test.*Connected|Test.*Visibility' -count=1`
- `go test ./...`

---

## Task 15: Add failing tests for a world-runtime map membership index

Objective: force map occupancy to become an owned runtime primitive instead of a recomputed side effect.

Files:
- Create: `internal/worldruntime/map_index_test.go`
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Add a failing test for inserting a player into an effective map bucket.
2. Add a failing test for relocation moving a player between map buckets.
3. Add a failing test for removal clearing the player from occupancy.
4. Add a failing test for stable sorted snapshots per map.
5. Run focused tests and confirm RED for missing map-index primitives.
6. Commit only after the tests are red.

Verification:
- `go test ./internal/worldruntime ./internal/minimal -run 'TestMapIndex|Test.*MapOccupancy' -count=1`

---

## Task 16: Introduce `internal/worldruntime/map_index.go`

Objective: own effective-map membership in a reusable runtime helper.

Files:
- Create: `internal/worldruntime/map_index.go`
- Create: `internal/worldruntime/map_index_test.go`
- Modify: `internal/worldruntime/entity_registry.go`
- Modify: `README.md`

Steps:
1. Add a map-index type that tracks player entity IDs by effective `MapIndex`.
2. Keep it topology-aware by using effective-map semantics instead of raw `MapIndex` assumptions.
3. Support register, move, remove, and per-map snapshot helpers.
4. Keep the API read-mostly and deterministic for tests.
5. Re-run the RED tests until green.
6. Commit.

Verification:
- `go test ./internal/worldruntime -run 'TestMapIndex|TestPlayerDirectory|TestEntityRegistry' -count=1`

---

## Task 17: Route visibility, occupancy, and relocate preview through the map index

Objective: stop rebuilding map occupancy by scanning all connected players every time.

Files:
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/visibility-rebuild.md`
- Modify: `README.md`

Steps:
1. Make occupancy snapshots consume the map index.
2. Make relocate preview and transfer preview use map-index snapshots as their before/after occupancy source.
3. Keep visible-peer behavior unchanged while replacing the occupancy source of truth.
4. Add regression tests that prove preview and occupancy stay correct across join/leave/transfer.
5. Commit.

Verification:
- `go test ./internal/minimal -run 'Test.*MapOccupancy|Test.*Transfer|Test.*Preview' -count=1`
- `go test ./...`

---

## Task 18: Extract a session directory boundary for queued frame sinks and relocators

Objective: separate live world actors from session-bound transport hooks.

Files:
- Create: `internal/worldruntime/session_directory.go`
- Create: `internal/worldruntime/session_directory_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `README.md`

Steps:
1. Add a small directory that maps entity/session IDs to pending frame queues and relocate callbacks.
2. Keep it transport-only: frame sinks and relocators, not character state.
3. Add tests for register, lookup, replace, and remove behavior.
4. Keep the rest of the world runtime untouched in this slice.
5. Commit.

Verification:
- `go test ./internal/worldruntime -run 'TestSessionDirectory' -count=1`

---

## Task 19: Route shared-world join/leave/transfer through the session directory

Objective: stop storing transport hooks directly inside `sharedWorldRegistry` entries.

Files:
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `README.md`

Steps:
1. Replace direct `sharedWorldSession` transport bookkeeping with session-directory lookups.
2. Keep entity/player/map state in worldruntime-owned structures and transport hooks in the session directory.
3. Re-run transfer, whisper, and disconnect tests.
4. Keep external wire behavior unchanged.
5. Commit.

Verification:
- `go test ./internal/minimal -run 'Test.*Transfer|Test.*Whisper|Test.*Leave|Test.*Close' -count=1`
- `go test ./...`

---

## Task 20: Freeze the first owned self-session transfer rebootstrap contract

Objective: replace the current deliberately temporary self-session transfer description with the first explicit transfer rebootstrap sequence the project will own.

Files:
- Read: `spec/protocol/map-transfer-bootstrap.md`
- Read: `spec/protocol/loading-to-game-bootstrap-burst.md`
- Read: `spec/protocol/client-game-entry-sequence.md`
- Create: `spec/protocol/transfer-rebootstrap-burst.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `README.md`

Steps:
1. Decide the first project-owned self-session transfer burst after a successful map transfer.
2. Define exactly which existing bootstrap frames are reused and which are still deferred.
3. Record non-goals clearly: no reconnect, no channel migration, no loading-screen perfection yet.
4. Update packet-matrix language so the transfer self-session contract is no longer narrative-only.
5. Commit.

Verification:
- docs are internally consistent with current boot-path and transfer docs
- referenced packet names and docs all exist

---

## Task 21: Add failing tests for the transfer rebootstrap self-session burst

Objective: prove the new transfer contract end to end before implementation.

Files:
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/warp/flow_test.go`
- Optionally create: `internal/worldentry/bootstrap_test.go`

Steps:
1. Add a failing test for a transfer-triggered self-session burst matching the new contract.
2. Add a failing test proving old-map peers still receive delete/update effects while the moved player receives the rebootstrap burst.
3. Add a failing test that proves movement/chat after transfer use the new live map immediately.
4. Run focused tests and confirm RED for the expected missing behavior.
5. Commit only after the tests are red.

Verification:
- `go test ./internal/minimal ./internal/warp ./internal/worldentry -run 'Test.*Transfer' -count=1`

---

## Task 22: Implement transfer rebootstrap through `internal/warp` and a shared world-entry bootstrap builder

Objective: stop duplicating self-bootstrap frame composition between `ENTERGAME` and transfer.

Files:
- Create: `internal/worldentry/bootstrap.go`
- Create: `internal/worldentry/bootstrap_test.go`
- Modify: `internal/worldentry/flow.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/warp/flow.go`
- Modify: `README.md`

Steps:
1. Extract shared self-bootstrap frame building into `internal/worldentry/bootstrap.go`.
2. Make the transfer flow reuse that builder for the moved player.
3. Keep persistence-before-commit guarantees intact.
4. Re-run the RED tests until green.
5. Commit.

Verification:
- `go test ./internal/worldentry ./internal/warp ./internal/minimal -run 'Test.*Bootstrap|Test.*Transfer' -count=1`
- `go test ./...`
- `go vet ./...`

---

## Task 23: Add failing tests for reconnect/rejoin teardown across player, entity, and session directories

Objective: harden the new ownership boundaries before they become deeper dependencies.

Files:
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/worldruntime/session_directory_test.go`
- Modify: `internal/worldruntime/player_directory_test.go`

Steps:
1. Add a failing test for disconnect removing the player from all runtime indexes.
2. Add a failing test for reconnecting the same login/character after disconnect and getting clean runtime state.
3. Add a failing test for transfer followed by close, ensuring no stale session sink remains.
4. Run focused tests and confirm RED for missing cleanup guarantees.
5. Commit only after the tests are red.

Verification:
- `go test ./internal/worldruntime ./internal/minimal -run 'Test.*Reconnect|Test.*Leave|Test.*Close' -count=1`

---

## Task 24: Implement reconnect-safe cleanup and rejoin semantics

Objective: make the new directories safe under disconnect/reconnect flows.

Files:
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/worldruntime/player_directory.go`
- Modify: `internal/worldruntime/session_directory.go`
- Modify: `internal/worldruntime/map_index.go`
- Modify: `README.md`

Steps:
1. Ensure close/disconnect removes the player from all live indexes exactly once.
2. Ensure a reconnect or new session rebuilds runtime state from persisted snapshot + live selection cleanly.
3. Keep duplicate cleanup idempotent.
4. Re-run reconnect and transfer tests until green.
5. Commit.

Verification:
- `go test ./internal/worldruntime ./internal/minimal -run 'Test.*Reconnect|Test.*Leave|Test.*Close|Test.*Transfer' -count=1`
- `go test ./...`

---

## Task 25: Introduce a reusable live-position value object in `internal/worldruntime`

Objective: prepare AOI work without spreading raw `MapIndex`/`X`/`Y` tuples everywhere.

Files:
- Create: `internal/worldruntime/position.go`
- Create: `internal/worldruntime/position_test.go`
- Modify: `internal/player/runtime.go`
- Modify: `internal/worldruntime/entity.go`
- Modify: `README.md`

Steps:
1. Add a small position value object containing effective map identity and coordinates.
2. Add helpers for equality and any simple normalization needed next.
3. Keep the first API narrow and avoid geometry overreach.
4. Add tests that prove the new type is safe for nil/zero/default cases where needed.
5. Commit.

Verification:
- `go test ./internal/worldruntime ./internal/player -run 'TestPosition|TestRuntime' -count=1`

---

## Task 26: Add failing tests for sector/radius AOI helpers

Objective: make AOI behavior explicit before changing current whole-map visibility.

Files:
- Create: `internal/worldruntime/aoi_test.go`
- Modify: `internal/worldruntime/visibility_test.go`

Steps:
1. Add a failing test for same-map players inside a small radius being visible.
2. Add a failing test for same-map players outside the radius not being visible.
3. Add a failing test for stable sector-key calculation from coordinates.
4. Keep the current whole-map policy untouched for now.
5. Run the focused tests and confirm RED.
6. Commit only after the tests are red.

Verification:
- `go test ./internal/worldruntime -run 'TestAOI|TestVisibility' -count=1`

---

## Task 27: Implement a bootstrap sector/radius AOI policy beside the current whole-map policy

Objective: add the first real AOI implementation without forcing it on every caller yet.

Files:
- Create: `internal/worldruntime/aoi.go`
- Create: `internal/worldruntime/aoi_test.go`
- Modify: `internal/worldruntime/topology.go`
- Modify: `README.md`

Steps:
1. Add a first AOI policy implementation based on same effective map plus a small radius/sector boundary.
2. Keep `WholeMapVisibilityPolicy` as the default.
3. Make the new AOI policy opt-in and fully testable in isolation.
4. Re-run the RED tests until green.
5. Commit.

Verification:
- `go test ./internal/worldruntime -run 'TestAOI|TestVisibility' -count=1`

---

## Task 28: Let bootstrap topology carry AOI policy configuration explicitly

Objective: move AOI selection from ad hoc tests into a first owned topology-level configuration surface.

Files:
- Modify: `internal/worldruntime/topology.go`
- Modify: `internal/worldruntime/topology_test.go`
- Modify: `spec/protocol/visibility-rebuild.md`
- Modify: `README.md`

Steps:
1. Add a small topology configuration surface for selecting whole-map vs AOI policy.
2. Keep defaults backward-compatible.
3. Document clearly that AOI is still bootstrap and local-process only.
4. Add tests that prove topology configuration selects the expected policy.
5. Commit.

Verification:
- `go test ./internal/worldruntime -run 'TestBootstrapTopology|TestAOI|TestVisibility' -count=1`

---

## Task 29: Route visibility diffs through the pluggable AOI policy

Objective: make callers consume AOI-aware visibility decisions without changing their public responsibilities.

Files:
- Modify: `internal/worldruntime/visibility.go`
- Modify: `internal/worldruntime/visibility_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `README.md`

Steps:
1. Make enter/leave/relocate visibility diffs depend on the configured policy only.
2. Keep the shared-world caller logic stable while changing the policy source of truth.
3. Add tests that compare whole-map and AOI policy behavior across enter and relocate cases.
4. Commit.

Verification:
- `go test ./internal/worldruntime ./internal/minimal -run 'Test.*Visibility|Test.*Relocate' -count=1`
- `go test ./...`

---

## Task 30: Extract topology-aware social scope queries into `internal/worldruntime`

Objective: stop letting social routing rules live as scattered conditions inside shared-world fanout code.

Files:
- Create: `internal/worldruntime/scopes.go`
- Create: `internal/worldruntime/scopes_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `spec/protocol/chat-scope-first-hardening.md`
- Modify: `README.md`

Steps:
1. Add reusable query helpers for talking, shout, guild, and exact-name whisper targets.
2. Make those helpers consume topology + directory/index state rather than raw session scans.
3. Keep external chat behavior unchanged unless AOI policy explicitly changes talking visibility.
4. Add regression tests for each chat family.
5. Commit.

Verification:
- `go test ./internal/worldruntime ./internal/minimal -run 'Test.*Chat|Test.*Whisper|Test.*Guild|Test.*Shout' -count=1`
- `go test ./...`

---

## Task 31: Freeze the first inventory bootstrap contract

Objective: define the smallest project-owned inventory surface before adding storage/runtime code.

Files:
- Create: `spec/protocol/inventory-bootstrap.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `README.md`

Steps:
1. Decide the smallest inventory slice to own next: slot model, self bootstrap, and one rearrangement path.
2. Record what is intentionally out of scope: item templates, drops, shops, loot, stacking rules beyond the first minimal rule set.
3. Define the first self-facing inventory bootstrap frames the server will own.
4. Register the new doc and align README milestone language.
5. Commit.

Verification:
- the inventory contract is precise enough to drive failing tests
- packet-matrix language names every newly owned packet/state transition

---

## Task 32: Add a persisted inventory model to bootstrap character/account snapshots

Objective: give inventory a durable home before adding live runtime behavior.

Files:
- Create: `internal/inventory/model.go`
- Create: `internal/inventory/model_test.go`
- Modify: `internal/loginticket/store.go`
- Modify: `internal/accountstore/store.go`
- Modify: `internal/accountstore/store_test.go`
- Modify: `README.md`

Steps:
1. Add a minimal inventory slot/item model in `internal/inventory`.
2. Extend bootstrap character/account persistence with inventory state.
3. Keep the first persistence contract tiny and explicit.
4. Add tests for load/save round-trips and default empty inventory behavior.
5. Commit.

Verification:
- `go test ./internal/inventory ./internal/accountstore ./internal/loginticket -count=1`

---

## Task 33: Attach a live inventory runtime to `player.Runtime` and bootstrap it on enter-game

Objective: make inventory a real live character subsystem instead of a passive persisted blob.

Files:
- Create: `internal/inventory/runtime.go`
- Create: `internal/inventory/runtime_test.go`
- Create: `internal/proto/item/inventory.go`
- Create: `internal/proto/item/inventory_test.go`
- Modify: `internal/player/runtime.go`
- Modify: `internal/worldentry/bootstrap.go`
- Modify: `internal/minimal/factory.go`
- Modify: `README.md`

Steps:
1. Add a live inventory runtime linked from `player.Runtime`.
2. Keep persistence explicit: runtime state should load from and write back to the persisted snapshot deliberately.
3. Emit the first self-only inventory bootstrap burst on `ENTERGAME`.
4. Add tests for runtime separation and self bootstrap encoding.
5. Commit.

Verification:
- `go test ./internal/inventory ./internal/player ./internal/proto/item ./internal/worldentry ./internal/minimal -count=1`
- `go test ./...`

---

## Task 34: Freeze the first equipment bootstrap contract

Objective: decide the smallest owned equip/unequip compatibility slice before implementing item-state mutations.

Files:
- Create: `spec/protocol/equipment-bootstrap.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `README.md`

Steps:
1. Define the first owned equipment surface: equipment slots, self refresh, and one equip/unequip request path.
2. Record non-goals clearly: item bonuses, full appearance matrix, refinement, sockets, and combat coupling stay deferred.
3. Align the equipment contract with the inventory bootstrap doc.
4. Commit.

Verification:
- docs are specific enough to drive equip/unequip RED tests next
- inventory and equipment docs do not contradict each other

---

## Task 35: Add the first minimal equip/unequip flow with self refresh

Objective: open M3 for real by turning persisted item state into a live character-state mutation path.

Files:
- Create: `internal/equipment/model.go`
- Create: `internal/equipment/model_test.go`
- Create: `internal/equipment/flow.go`
- Create: `internal/equipment/flow_test.go`
- Create: `internal/proto/item/equipment.go`
- Create: `internal/proto/item/equipment_test.go`
- Modify: `internal/inventory/runtime.go`
- Modify: `internal/player/runtime.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/worldentry/bootstrap.go`
- Modify: `README.md`

Steps:
1. Add a minimal equipment model and slot rules.
2. Add failing tests for equip and unequip against the first supported slot set.
3. Implement only the narrowest self-facing flow and refresh packets needed by the contract.
4. Keep persistence explicit and covered by tests.
5. Commit.

Verification:
- `go test ./internal/equipment ./internal/inventory ./internal/player ./internal/proto/item ./internal/minimal -count=1`
- `go test ./...`
- `go vet ./...`

---

## Recommended execution grouping

- **Slices 11-19**: finish the missing world-runtime directory/index ownership boundaries
- **Slices 20-24**: close the self-session transfer rebootstrap gap and harden disconnect/reconnect semantics
- **Slices 25-30**: add the first real AOI boundary and move social scope queries onto owned runtime helpers
- **Slices 31-35**: open M3 with inventory + equipment bootstrap state

---

## Global validation rule for every slice in this plan

Before marking any slice complete:
- run the narrow package tests for the touched area first
- run `go test ./...`
- run `go vet ./...`
- keep docs and code in the same commit
- keep the tree clean before starting the next slice
- push after each completed slice so public CI validates it

## Anti-goals for this 25-slice window

Do not do these inside this plan:
- combat loops, damage, death, or respawn
- NPCs, mobs, spawns, or quest runtime
- DB-backed persistence or migration tooling
- real multi-channel/shard routing
- broad protocol archaeology without a concrete runtime payoff

## Ready-to-start next slice

If work starts immediately, begin with **Task 11: Freeze the post-bootstrap world-runtime ownership contract**.
That is the smallest next slice that keeps M2 honest, documents the new architecture boundaries after the first 10 slices, and unlocks the directory/index extractions without jumping early into inventory or combat.
