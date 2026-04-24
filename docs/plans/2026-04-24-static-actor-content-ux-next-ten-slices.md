# Static Actor Content UX Next Ten Slices Implementation Plan

> **For Hermes:** Use test-driven-development. Keep slices tiny, public, and green on `main`.

**Goal:** finish the first believable static-actor content loop so already-seeded actors behave consistently across enter, move, sync, transfer, live operator edits, restart, and the next interaction-facing seams.

**Architecture:** work in three stages: (1) close the remaining player-visible gaps in the current static-actor lifecycle, especially transfer and live authoring while players are online; (2) make seeded static content durable enough to survive daemon restarts and easier to debug; and (3) prepare the first interaction-capable content seam without prematurely committing to full NPC/shop/quest behavior. Keep `internal/worldruntime` owning visibility queries/diffs and content snapshots, while `internal/minimal` remains the packet/session orchestration layer.

**Tech Stack:** Go 1.26, `internal/minimal`, `internal/worldruntime`, `internal/ops`, a new file-backed static-actor store package if needed, protocol docs under `spec/protocol/`, plans under `docs/plans/`, validation with `gofmt`, focused `go test`, `go test ./...`, and `go vet ./...`.

---

## Current starting point
- Current `main` head when this plan is written: `891d9ce feat: rebuild static actor visibility across aoi moves`
- Already owned in repo:
  - static actors can be seeded, listed, updated, and removed through loopback-only operator endpoints
  - entering players now receive visible static actors in the `ENTERGAME` burst
  - `MOVE` and `SYNC_POSITION` now rebuild self-facing static-actor visibility under configured radius AOI
  - runtime map occupancy already includes static actors and static-only maps
- Biggest remaining gaps after that work:
  - gameplay-triggered transfer/rebootstrap still does not obviously freeze static-actor destination content in the returned self burst
  - operator create/update/delete while players are already online still does not complete the live content loop in the most user-visible way
  - static actor content is still bootstrap-runtime data, not yet clearly durable across restarts
  - there is still no first owned interaction-facing seam for visible static actors

---

## Ordering principles for this 10-slice window
1. Prefer slices that improve **what the player sees** before slices that only improve admin ergonomics.
2. Keep each slice docs + tests + code in one commit when behavior changes.
3. Keep `internal/worldruntime` owning visibility queries/diffs and `internal/minimal` owning packet emission.
4. Reuse the currently-owned visibility family (`CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, `CHARACTER_UPDATE`, `CHARACTER_DEL`) before inventing new wire contracts.
5. Do not jump into full NPC/quest/shop/AI behavior until the static-actor lifecycle is coherent and durable.

---

## Task 1: Append visible static actors to gameplay-triggered transfer rebootstrap

**Objective:** when an exact-position transfer trigger fires on `MOVE` / `SYNC_POSITION`, the mover should immediately see the destination static actors that share visible world, not only peer players.

**Why now:** this is the largest remaining player-visible gap after enter/bootstrap and AOI move/sync rebuilds were closed.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/transfer-rebootstrap-burst.md`
- Modify: `spec/protocol/non-player-entity-bootstrap.md`
- Modify: `README.md`

**Slice notes:**
- keep the current self transfer burst ordering explicit:
  1. self rebootstrap frames
  2. peer player deltas
  3. destination visible static-actor bursts
- do not broaden this slice into live operator fanout

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*Transfer.*StaticActor|Test.*ExactPositionTransferTrigger' -count=1
```

---

## Task 2: Freeze explicit static-actor diffs in relocate-preview and transfer results

**Objective:** make operator preview/commit results explicitly report which static actors are added/removed, instead of forcing callers to infer that from before/after snapshots.

**Why now:** once transfer rebootstrap includes static actors, the operator preview/transfer surface should expose the same content movement more directly.

**Files:**
- Modify: `internal/worldruntime/scopes.go`
- Modify: `internal/worldruntime/scopes_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/bootstrap-map-transfer-contract.md`
- Modify: `README.md`

**Slice notes:**
- keep `before_map_occupancy` and `after_map_occupancy`
- add explicit arrays such as `added_static_actors` / `removed_static_actors` only if they materially simplify tooling
- do not change character-count map deltas

**Verification:**
```bash
go test ./internal/worldruntime ./internal/minimal -run 'Test.*RelocationPreview|Test.*Transfer.*StaticActor' -count=1
```

---

## Task 3: Fan out newly seeded static actors to already-visible online players

**Objective:** `POST /local/static-actors` should immediately make the new actor appear for players who already share visible world with it.

**Why now:** this is the first live content-authoring slice with immediate in-game payoff while players remain online.

**Files:**
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/ops/pprofmux_test.go` if endpoint behavior needs visible regression coverage
- Modify: `spec/protocol/non-player-entity-bootstrap.md`
- Modify: `README.md`

**Slice notes:**
- reuse the existing actor bootstrap burst
- only enqueue to players who currently share visible world with the new actor
- keep the endpoint contract unchanged

**Verification:**
```bash
go test ./internal/minimal ./internal/ops -run 'Test.*StaticActor.*Register|Test.*StaticActor.*Visible' -count=1
```

---

## Task 4: Fan out static-actor deletes to already-visible online players

**Objective:** `DELETE /local/static-actors/{entity_id}` should immediately remove the actor from players who are currently seeing it.

**Why now:** live creation without live teardown leaves visible ghost content and makes authoring unsafe.

**Files:**
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/non-player-entity-bootstrap.md`
- Modify: `README.md`

**Slice notes:**
- emit one `CHARACTER_DEL` to affected players
- keep removal tolerant if no players currently see the actor
- do not change operator HTTP semantics

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*Remove.*StaticActor.*Visible|Test.*StaticActor.*Delete' -count=1
```

---

## Task 5: Refresh already-visible static actors on same-map/same-AOI updates

**Objective:** `PATCH` / `PUT` updates that change a visible actor's name, race, or in-range position should refresh that actor for players who are already seeing it.

**Why now:** once live create/delete works, the next obvious content-authoring gap is editing a visible actor without forcing players to relog or move away and back.

**Files:**
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/scopes.go` only if a small helper is needed
- Modify: `spec/protocol/non-player-entity-bootstrap.md`
- Modify: `README.md`

**Slice notes:**
- prefer the smallest honest wire rule: delete + re-add burst is acceptable if a pure update packet is not yet owned for this actor family
- keep scope to updates that remain in the same visible world set

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*Update.*StaticActor.*Visible' -count=1
```

---

## Task 6: Rebuild online player visibility correctly when static actors relocate across map/AOI boundaries

**Objective:** `PATCH` / `PUT` updates that move an actor across map or AOI boundaries should delete it for sessions leaving visibility and add it for sessions entering visibility.

**Why now:** this completes the live edit loop for operator-driven actor relocation while players are online.

**Files:**
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/scopes.go`
- Modify: `internal/worldruntime/scopes_test.go`
- Modify: `spec/protocol/non-player-entity-bootstrap.md`
- Modify: `README.md`

**Slice notes:**
- this is the operator-edit analogue of the already-owned self-facing move/sync rebuild
- affected sessions should see only the deltas that match their own visibility change

**Verification:**
```bash
go test ./internal/worldruntime ./internal/minimal -run 'Test.*StaticActor.*Relocate|Test.*StaticActor.*AOI' -count=1
```

---

## Task 7: Add a file-backed static-actor snapshot store

**Objective:** bootstrap static actors should survive daemon restarts instead of existing only in in-memory runtime state.

**Why now:** once live content authoring works, losing all actors on restart becomes the next biggest practical gap.

**Files:**
- Create: `internal/staticstore/store.go`
- Create: `internal/staticstore/file_store.go`
- Create: `internal/staticstore/file_store_test.go`
- Modify: `README.md`
- Modify: `spec/protocol/non-player-entity-bootstrap.md`

**Slice notes:**
- keep the first persistence shape narrow: full snapshot replace is fine
- do not redesign player/account persistence in this slice
- choose a deterministic on-disk JSON format first; avoid over-generalizing to DB/schema work

**Verification:**
```bash
go test ./internal/staticstore -count=1
```

---

## Task 8: Load persisted static actors on boot and persist POST/PATCH/DELETE mutations

**Objective:** `gamed` should restore saved static actors at startup and keep the snapshot updated after every operator mutation.

**Why now:** the store is only useful once it is wired into runtime construction and mutation flows.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `cmd/gamed/main.go` if explicit wiring is needed
- Modify: `README.md`

**Slice notes:**
- load at runtime bootstrap before sessions join
- persist on successful create/update/delete only
- fail closed on malformed persisted data, with tests that keep behavior explicit

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*StaticActor.*Persist|Test.*StaticActor.*Boot' -count=1
```

---

## Task 9: Expose per-player static-actor visibility through runtime introspection

**Objective:** make content QA operable by showing which static actors each connected player currently sees under the active topology/AOI policy.

**Why now:** after live create/update/delete and persistence, content debugging becomes the next bottleneck.

**Files:**
- Modify: `internal/worldruntime/scopes.go`
- Modify: `internal/worldruntime/scopes_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/ops/pprofmux.go` or add a dedicated handler if needed
- Modify: `internal/ops/pprofmux_test.go`
- Modify: `README.md`

**Slice notes:**
- either extend `/local/visibility` with visible static actors or add a dedicated `/local/static-visibility` endpoint
- keep the JSON deterministic and loopback-only
- do not couple this slice to live packet behavior changes

**Verification:**
```bash
go test ./internal/worldruntime ./internal/ops -run 'Test.*Static.*Visibility' -count=1
```

---

## Task 10: Add interaction-ready metadata to static actors and freeze the first interaction contract

**Objective:** prepare the next real content vertical by letting static actors carry a minimal interaction kind/reference, then document the first narrow interaction behavior to implement next.

**Why now:** after visibility, lifecycle, and persistence are coherent, the next sensible frontier is the first actual content interaction — but the metadata seam should land before packet behavior.

**Files:**
- Modify: `internal/worldruntime/entity.go`
- Modify: `internal/worldruntime/non_player_directory.go`
- Modify: `internal/worldruntime/entity_registry.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/ops/pprofmux.go`
- Modify: `internal/ops/pprofmux_test.go`
- Create: `spec/protocol/static-actor-interaction-bootstrap.md`
- Modify: `README.md`

**Slice notes:**
- keep the first metadata seam tiny, e.g.:
  - `interaction_kind`
  - `interaction_ref`
- do not implement full NPC/shop logic here
- this slice should end by freezing the next vertical, likely an info-only or talk-like interaction first

**Verification:**
```bash
go test ./internal/worldruntime ./internal/minimal ./internal/ops -run 'Test.*StaticActor.*Interaction' -count=1
```

---

## Recommended execution grouping
- **Tasks 1-2**: close the remaining transfer/preview gaps in the current static-actor visibility contract
- **Tasks 3-6**: complete the live online content-authoring loop
- **Tasks 7-9**: make static content durable and debuggable
- **Task 10**: prepare the first interaction-bearing content seam

---

## Why this order makes the most sense now
1. It maximizes **player-visible payoff first**: transfer, live create, live delete, live move/update.
2. It keeps using the **already-owned wire family** instead of inventing speculative packet types too early.
3. It makes static actors **durable enough to be real content**, not just session-local scaffolding.
4. It delays risky NPC/shop/quest behavior until the current visible-world contract is stable and easier to observe.

---

## Anti-goals for this 10-slice window
Do **not** do these here:
- inventory, equipment, consumables, or character formulas
- combat, damage, targeting, death, or respawn loops
- mob AI, aggro, spawn groups, or quest runtime
- multi-channel world ownership or inter-process entity handoff
- database-backed persistence redesign
- broad shop/NPC systems before the first interaction contract is frozen

## Ready-to-start next slice
Start with **Task 1: Append visible static actors to gameplay-triggered transfer rebootstrap**.
It is the highest-return remaining gap because the current runtime already owns static-actor enter/bootstrap and AOI move/sync rebuild, but transfer is still the most obvious content UX discontinuity.