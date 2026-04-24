# Runtime AOI and Static Actor Next Ten Slices Implementation Plan

> **For Hermes:** Use test-driven-development. Keep slices tiny, public, and green on `main`.

**Goal:** turn the existing AOI boundary into a real runtime capability and then use that stronger M2 foundation to make static non-player actors more useful before opening any combat, inventory, or mob/NPC behavior.

**Architecture:** finish the highest-return M2 work in three stages: (1) wire AOI selection from config into the actual bootstrap runtime, (2) deepen runtime/operator ownership around AOI-aware snapshots and movable static actors, and (3) freeze then implement the first client-visible static-actor bootstrap contract. Keep `internal/worldruntime` as the owner of topology, AOI policy, map presence, and snapshot composition, while `internal/minimal` remains the session/packet orchestration layer.

**Tech Stack:** Go 1.26, `internal/config`, `internal/minimal`, `internal/worldruntime`, `internal/ops`, protocol docs under `spec/protocol/`, plans under `docs/plans/`, validation with `gofmt`, focused `go test`, `go test ./...`, and `go vet ./...`.

---

## Current starting point
- Current `main` head before this plan starts: `e05b7fe fix: tolerate partial teardown in static map index removal`
- Already owned in repo:
  - `VisibilityPolicy` boundary plus `WholeMapVisibilityPolicy` and opt-in `RadiusVisibilityPolicy`
  - topology helpers `WithRadiusVisibilityPolicy(...)` / `WithWholeMapVisibilityPolicy()`
  - `internal/worldruntime/scopes.go` owning runtime-facing visibility and snapshot composition
  - static actor seed/list/remove runtime support and loopback-only ops endpoints
- Biggest remaining gap behind the current code shape:
  - AOI exists mostly as an internal/testing seam, not as a real runtime configuration chosen by `gamed`
  - static actors can be created and removed, but cannot be updated or moved in-place
  - the repo still has no frozen or implemented client-visible static-actor bootstrap contract

---

## Ordering and scope rules
1. Finish AOI/runtime ownership and static-actor usefulness before inventory, equipment, combat, mobs, shops, or quests.
2. Keep every slice docs + tests + code in the same commit when behavior changes.
3. Follow strict TDD for code slices: write the focused failing test first, run it and observe RED, then implement the minimum code.
4. Keep `internal/worldruntime` owning policy/query/snapshot rules and `internal/minimal` owning packet/session orchestration.
5. Do not over-design generalized content systems in this window; prioritize the smallest seams with immediate payoff.

---

## Task 1: Wire AOI selection from service config into the bootstrap runtime

**Objective:** make the current AOI policy boundary real by letting `gamed` choose whole-map vs radius visibility through `config.Service`, with runtime validation and topology wiring.

**Files:**
- Modify: `internal/config/service.go`
- Modify: `internal/config/service_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `README.md`
- Modify: `spec/protocol/visibility-rebuild.md`

**Step 1: Write failing tests**
- Add config tests for service/global overrides of:
  - `VISIBILITY_MODE`
  - `VISIBILITY_RADIUS`
  - `VISIBILITY_SECTOR_SIZE`
- Add runtime tests proving `newGameRuntimeWithAccountStore(...)`:
  - selects `RadiusVisibilityPolicy` when config requests it
  - rejects invalid AOI config (unknown mode, zero/non-positive radius, zero/non-positive sector size)

**Step 2: Run RED tests**
Run:
```bash
go test ./internal/config ./internal/minimal -run 'TestLoadService|TestNewGameSessionFactory' -count=1
```
Expected: FAIL because config/runtime do not yet carry AOI settings.

**Step 3: Write minimal implementation**
- Extend `config.Service` with AOI-related fields.
- Load service-specific and global AOI overrides.
- Build `BootstrapTopology` from config inside `internal/minimal/factory.go`.
- Keep whole-map as the default when no AOI mode is configured.

**Step 4: Run GREEN tests**
Run the same focused command until green.

**Step 5: Commit**
```bash
git add internal/config/service.go internal/config/service_test.go internal/minimal/factory.go internal/minimal/factory_test.go README.md spec/protocol/visibility-rebuild.md

git commit -m "feat: wire runtime aoi policy from config"
```

---

## Task 2: Freeze integrated AOI behavior at the bootstrap runtime edge

**Objective:** prove the actual runtime now obeys the configured AOI policy for visibility, movement fanout, sync fanout, and local chat.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `README.md`
- Modify: `spec/protocol/visibility-rebuild.md`
- Modify: `spec/protocol/chat-scope-first-hardening.md`

**Steps:**
1. Add RED tests showing that under radius AOI:
   - `ENTERGAME` only bootstraps nearby peers
   - `MOVE` and `SYNC_POSITION` only fan out to nearby peers
   - local talking still requires same empire *and* current AOI visibility
2. Keep whole-map behavior unchanged under default config.
3. Implement only the missing runtime wiring or regression fixes required for those tests.
4. Rerun focused tests, then full suite, then vet.
5. Commit.

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*Visibility|Test.*Move|Test.*Sync|Test.*Chat' -count=1
```

---

## Task 3: Expose active AOI/runtime policy through loopback introspection

**Objective:** make AOI debugging operable without reading code or guessing what `gamed` booted with.

**Files:**
- Modify: `internal/ops/pprofmux.go`
- Modify: `internal/ops/pprofmux_test.go`
- Modify: `cmd/gamed/main.go`
- Modify: `internal/minimal/factory.go`
- Modify: `README.md`

**Steps:**
1. Add a tiny runtime snapshot or dedicated endpoint reporting:
   - visibility mode
   - radius/sector size when applicable
   - local channel ID if already available there
2. Add RED ops tests for loopback-only access and JSON shape.
3. Implement the minimal surface.
4. Verify focused ops tests, then full suite, then vet.
5. Commit.

**Verification:**
```bash
go test ./internal/ops ./cmd/gamed -run 'Test.*AOI|Test.*RuntimeConfig' -count=1
```

---

## Task 4: Refresh the M2 roadmap/docs to match post-hardening reality

**Objective:** stop the repo roadmap from lagging the actual runtime after reconnect hardening, AOI wiring, and static-actor scaffolding.

**Files:**
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/entity-runtime-bootstrap.md`
- Create: `docs/plans/2026-04-24-runtime-foundation-after-aoi-wiring.md`

**Steps:**
1. Write a docs-first checkpoint summarizing what M2 now owns.
2. Make AOI config, reconnect hardening, and static-actor runtime status explicit.
3. Keep non-goals equally explicit.
4. Commit.

**Verification:**
- docs no longer contradict current code paths or current operator/runtime surfaces

---

## Task 5: Add in-place static-actor update/move support inside `internal/worldruntime`

**Objective:** let static actors become useful content scaffolding by supporting position/class/name updates without delete-and-recreate.

**Files:**
- Modify: `internal/worldruntime/non_player_directory.go`
- Modify: `internal/worldruntime/non_player_directory_test.go`
- Modify: `internal/worldruntime/map_index.go`
- Modify: `internal/worldruntime/map_index_test.go`
- Modify: `internal/worldruntime/entity_registry.go`
- Modify: `internal/worldruntime/entity_registry_test.go`

**Steps:**
1. Add RED tests for updating a static actor while preserving identity.
2. Cover both same-map and cross-map updates.
3. Ensure map-index presence moves cleanly and remains partial-teardown tolerant.
4. Implement only the minimal update surface.
5. Commit.

**Verification:**
```bash
go test ./internal/worldruntime -run 'Test.*Static.*Update|TestMapIndex|TestEntityRegistry' -count=1
```

---

## Task 6: Expose loopback-only PATCH/PUT static-actor editing on `gamed`

**Objective:** make the new static-actor update/move support usable from operator tooling.

**Files:**
- Modify: `internal/ops/pprofmux.go`
- Modify: `internal/ops/pprofmux_test.go`
- Modify: `cmd/gamed/main.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `README.md`

**Steps:**
1. Add RED tests for strict request parsing and loopback-only enforcement.
2. Support update-in-place by entity ID.
3. Keep GET/POST/DELETE behavior unchanged.
4. Return 400/404 precisely for malformed or missing actors.
5. Commit.

**Verification:**
```bash
go test ./internal/ops ./internal/minimal -run 'Test.*StaticActor' -count=1
```

---

## Task 7: Freeze static-actor visibility/occupancy diffs in relocate-preview and transfer results

**Objective:** make runtime/operator previews explicit about what static actors are present before and after moves, instead of only showing counts/snapshots indirectly.

**Files:**
- Modify: `internal/worldruntime/scopes.go`
- Modify: `internal/worldruntime/scopes_test.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `README.md`
- Modify: `spec/protocol/bootstrap-map-transfer-contract.md`

**Steps:**
1. Add RED tests for structured preview/result behavior preserving static-only maps and static-actor snapshots.
2. Decide whether explicit static-actor diff arrays are needed or whether before/after snapshots are sufficient.
3. Keep character-count map deltas unchanged.
4. Commit.

**Verification:**
```bash
go test ./internal/worldruntime ./internal/minimal -run 'Test.*RelocationPreview|Test.*Transfer' -count=1
```

---

## Task 8: Freeze the first client-visible static-actor bootstrap contract

**Objective:** document the smallest honest wire-visible contract for static actors before changing packet behavior.

**Files:**
- Create: `spec/protocol/static-actor-bootstrap-visibility.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `README.md`

**Steps:**
1. Define exactly what the first visible static actor means:
   - when the client sees it
   - which packet family is reused
   - what remains unsupported
2. State explicit non-goals:
   - no AI
   - no combat
   - no shops
   - no dynamic pathing/spawn groups
3. Register the doc and align README language.
4. Commit.

**Verification:**
- docs are precise enough to drive the next RED tests without guessing packet order or scope

---

## Task 9: Emit static actors during `ENTERGAME` / self bootstrap under owned visibility rules

**Objective:** deliver the first visible payoff from the non-player runtime by bootstrapping visible static actors to the joining client.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/scopes.go`
- Modify: `internal/worldruntime/scopes_test.go`

**Steps:**
1. Add RED tests for static actors appearing in the joining client bootstrap burst.
2. Keep AOI/topology ownership in `worldruntime`.
3. Reuse existing packet builders where honest and possible; do not claim a richer lifecycle than exists.
4. Commit.

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*EnterGame|Test.*StaticActor' -count=1
```

---

## Task 10: Rebuild static-actor visibility correctly on relocate, transfer, reconnect, and AOI changes

**Objective:** finish the first client-visible static-actor loop so runtime ownership remains consistent after motion and reconnect.

**Files:**
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/scopes.go`
- Modify: `internal/worldruntime/scopes_test.go`
- Modify: `spec/protocol/static-actor-bootstrap-visibility.md`
- Modify: `README.md`

**Steps:**
1. Add RED tests for add/remove visibility across:
   - relocate preview/apply
   - gameplay-triggered transfer
   - reconnect into the same map
   - AOI boundary crossing if supported in this window
2. Implement only the required visibility rebuild.
3. Keep behavior deterministic and docs exact.
4. Commit.

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*StaticActor|Test.*Relocate|Test.*Transfer|Test.*Reconnect' -count=1
```

---

## Recommended execution grouping
- **Tasks 1-3**: make AOI real and operable
- **Tasks 4-7**: refresh docs and turn static actors into useful runtime/operator data
- **Tasks 8-10**: freeze and implement the first visible static-actor client contract

---

## Global validation rule for every slice in this plan
Before marking any slice complete:
- run the focused RED test first and observe the expected failure
- run `gofmt -w` on every touched Go file
- rerun the focused tests until green
- run `go test ./...`
- run `go vet ./...`
- keep the tree clean before starting the next slice
- push after each completed slice

## Anti-goals for this 10-slice window
Do **not** do these here:
- inventory/equipment/item-use work
- combat, targeting, damage, death, respawn
- mob AI, NPC AI, shops, quests, spawn groups
- DB persistence redesign
- inter-channel or inter-process world ownership
- generalized content authoring systems beyond the specific static-actor seams above

## Ready-to-start next slice
Begin with **Task 1: Wire AOI selection from service config into the bootstrap runtime**.
It has the best immediate return because the repo already owns the AOI abstractions, but the runtime still boots with the default whole-map policy regardless of configuration.