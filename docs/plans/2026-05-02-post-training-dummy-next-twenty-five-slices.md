# Post-training-dummy Next 25 Slices Implementation Plan

> **For Hermes:** use `test-driven-development`. Keep slices tiny, keep `main` green, and land docs + focused RED tests + code + QA notes together. One commit per slice.

**Goal:** take `go-metin2-server` from the current self-only `training_dummy` target-selection slice into a real first combat loop (target -> attack -> HP change -> death -> respawn), then use that finished M4 footing to open the first honest M5 content-runtime RED around a hostile practice mob.

**Architecture:** split the next work into five bands:
1. freeze and own the first real attack ingress on top of the existing `TARGET` contract,
2. add mutable runtime combat state for authored non-player actors,
3. harden combat ownership/visibility/reconnect behavior,
4. complete the first end-to-end dummy death/respawn loop,
5. generalize dummy-only combat into the first content-runtime mob/spawn footing without overreaching into full AI, loot, quests, or production persistence.

Keep `internal/worldruntime` as the owner of topology-aware entity lookup plus non-player combat state, keep `internal/minimal` as the session/packet orchestration layer, keep `internal/player` focused on selected-character mutable state, keep authored content deterministic and file-backed, and avoid opening skills, drops, quest rewards, or mob pathing before the dummy loop is honest.

**Tech stack:** Go 1.26, current `internal/minimal`, `internal/game`, `internal/worldruntime`, `internal/player`, `internal/staticstore`, `internal/contentbundle`, `internal/proto/*`, file-backed stores in `internal/accountstore` / `internal/loginticket`, protocol docs under `spec/protocol/`, plans under `docs/plans/`, and live-client QA notes under `docs/qa/manual-client-checklist.md`.

---

## Current starting point
- Current `main` head when this plan is written: `f5aff4a docs: polish public README`
- Most relevant recently landed slices:
  - `f279913 feat: keep reclaimed item use local`
  - `00a850c feat: keep reclaimed merchant buys local`
  - `9b1c8f4 test: freeze reconnect after stale item use`
  - `bfe1ab8 test: freeze reconnect after stale merchant buy`
  - `f2ea1b8 test: freeze /shop_buy placement parity`
  - `274b7ca feat: resolve item use through template metadata`
  - `db51433 feat: refresh equip points from template metadata`
  - `f750188 test: freeze equip-effect slot matching`
  - `538b95a docs: freeze training dummy target contract`
  - `534cc9c feat: validate training dummy combat targets`
  - `88ee5fa feat: add training dummy target packets`
- The repo now already owns:
  - secure legacy handshake/login/select/enter-game,
  - shared-world player visibility/movement/chat,
  - bootstrap static actors plus interaction-backed merchant/warp seams,
  - first inventory/equipment/item-use/template-driven point slices,
  - first self-only `TARGET` acknowledgement for a visible in-range `training_dummy`.
- Important repo reality right now:
  - there is still **no owned attack ingress** after target selection,
  - `StaticEntity` only carries authored `CombatKind`; it does not yet own mutable HP/dead/respawn runtime state,
  - stale-socket hardening already exists for movement/whisper/item/merchant paths and should become the model for combat,
  - manual QA still treats mobs/combat/death as out of scope,
  - README truth is now roughly:
    - M1 `[~]`
    - M2 `[~]`
    - M3 `[~]`
    - M4 `[~]` (target-selection only)
    - M5 `[ ]`
    - M6 `[ ]`

---

## Ordering principles for this 25-slice window
1. **Docs before code for every new packet or choreography claim.** Do not guess the attack/death/respawn wire shape in code first.
2. **Finish one honest dummy loop before generalizing to mobs.** `training_dummy` remains the narrow testbed until target, hit, HP, death, and respawn are all real.
3. **Runtime combat state belongs to the world, not persistence.** HP/dead/respawn should live in runtime-owned non-player state, not leak into account persistence or fake player point state.
4. **Reuse the reclaim/reconnect hardening pattern.** Combat should inherit the same live-owner vs stale-socket rules already frozen for movement, item use, and merchant buys.
5. **Prefer tiny compatibility seams over heroic simulation.** If the client can accept a smaller clear/death/respawn contract, land that first instead of opening animation systems, mob AI, or loot tables.
6. **End the window with the next major vertical left as an explicit RED.** The best finish is a concrete hostile-mob retaliation test, not vague prose about “future AI”.

---

## Band A — own the first real attack ingress above the current `TARGET` slice

### Slice 1: Freeze the first owned attack / clear-target contract

**Objective:** document the smallest next combat wire contract the repo will own after `TARGET`, including the first player attack ingress, the accepted response shape, and how target clear is represented (for example, zero-target reuse of an existing packet if that is what captures support).

**Why now:** the repo already owns target selection, so the next ambiguity to remove is what concrete client/server packet family advances that state into a real hit attempt.

**Files:**
- Create: `spec/protocol/combat-normal-attack-bootstrap.md`
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Verification:**
- docs explicitly define the next attack-family scope and non-goals
- clear-target behavior is no longer left implicit
- README milestone wording stays truthful: still no damage/death yet

---

### Slice 2: Add failing codec tests for the first attack packet family

**Objective:** create the RED around parsing/building the chosen attack/clear-target packet surface before any session flow starts depending on it.

**Why now:** `internal/minimal` should not open-code another combat family without a small owned protocol package contract.

**Files:**
- Modify: `internal/proto/combat/combat_test.go`
- Optionally create: `internal/proto/combat/testdata/*`
- Modify: `spec/protocol/combat-normal-attack-bootstrap.md`

**Verification:**
```bash
go test ./internal/proto/combat -count=1
```
Expected: RED for missing builders/parsers or missing APIs, not ambiguous fixture shape.

---

### Slice 3: Implement the first attack / clear-target codec helpers

**Objective:** extend `internal/proto/combat` so the new attack request and its current server-side acknowledgement/clear helpers are owned in one place alongside the existing `TARGET` frames.

**Why now:** runtime ingress should depend on a focused protocol package instead of manual frame decoding in the gameplay flow.

**Files:**
- Modify: `internal/proto/combat/combat.go`
- Modify: `internal/proto/combat/combat_test.go`
- Modify: `spec/protocol/combat-normal-attack-bootstrap.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/proto/combat -count=1
```

---

### Slice 4: Add failing session-flow tests for accepted dummy attacks

**Objective:** write end-to-end RED tests proving that only a live `GAME`-phase session with an active selected visible in-range dummy target may advance from target selection into an accepted attack attempt.

**Why now:** this freezes routing/validation behavior before runtime state mutation lands.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/game/flow_test.go`
- Modify: `spec/protocol/combat-normal-attack-bootstrap.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/game -run 'Test.*(Combat|Attack|Target)' -count=1
```
Expected: RED for missing flow/runtime behavior.

---

### Slice 5: Implement minimal attack dispatch with explicit fail-closed behavior

**Objective:** route the first attack ingress through the existing shared-world seam, accept only the selected visible in-range dummy target, and fail closed for malformed, stale, cross-map, or non-targetable attempts.

**Why now:** this turns `TARGET` from a dead-end acknowledgement into the entrypoint for a real gameplay loop without yet claiming damage or death.

**Files:**
- Modify: `internal/game/flow.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/game ./internal/proto/combat -run 'Test.*(Combat|Attack|Target)' -count=1
```
```bash
go test ./...
```

---

## Band B — add mutable runtime combat state for authored non-player actors

### Slice 6: Introduce runtime-owned non-player combat state

**Objective:** add a narrow runtime structure for mutable non-player combat state (`max_hp`, `current_hp`, `dead`, optional respawn metadata) instead of overloading `StaticEntity` authored metadata.

**Why now:** repeated hits cannot be honest while HP still exists only as an implied `100%` in the target ack.

**Files:**
- Create or modify: `internal/worldruntime/*`
- Modify: `internal/worldruntime/entity.go`
- Modify: `internal/worldruntime/entity_registry.go`
- Modify: `internal/worldruntime/non_player_directory.go`
- Modify: `internal/worldruntime/*_test.go`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/worldruntime -run 'Test.*(NonPlayer|Combat|EntityRegistry)' -count=1
```

---

### Slice 7: Seed dummy combat defaults from authored content

**Objective:** make `training_dummy` runtime combat state come from deterministic authored/bootstrap defaults rather than hardcoded ad-hoc values scattered through flow tests.

**Why now:** the project needs one content-owned place to say what a dummy is before repeated-hit and respawn behavior start depending on it.

**Files:**
- Modify: `internal/staticstore/*`
- Modify: `internal/contentbundle/*`
- Modify: `internal/minimal/content_bundle_runtime_test.go`
- Modify: `internal/minimal/interaction_definitions_runtime_test.go`
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*(TrainingDummy|ContentBundle|StaticActor)' -count=1
```

---

### Slice 8: Add failing tests for the first HP-changing hit

**Objective:** open a RED proving that an accepted attack on a live dummy decrements runtime HP exactly once and returns the updated server-owned combat refresh shape.

**Why now:** this is the exact point where the combat vertical stops being ceremonial and starts mutating real state.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/*_test.go`
- Modify: `spec/protocol/combat-normal-attack-bootstrap.md`
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*(TrainingDummy|CombatHP|Attack)' -count=1
```
Expected: RED for missing HP mutation and refresh behavior.

---

### Slice 9: Implement deterministic dummy damage + HP-percent refresh

**Objective:** on an accepted hit, decrement the dummy's runtime HP, clamp correctly, compute the new `hp_percent`, and return the smallest owned client refresh that keeps the dummy loop honest.

**Why now:** this is the minimum believable “combat happened” step and unlocks later death/respawn work.

**Files:**
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/worldruntime/*`
- Modify: `internal/proto/combat/combat.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime ./internal/proto/combat -run 'Test.*(TrainingDummy|CombatHP|Attack)' -count=1
```
```bash
go test ./...
```

---

### Slice 10: Freeze the repeated-hit loop and no-persistence rule

**Objective:** document and QA-freeze the first repeated-hit behavior for dummies, explicitly stating that dummy HP is runtime-owned only and must not touch account persistence.

**Why now:** before reconnect/reclaim and death semantics broaden the surface, the repo should be explicit that this first combat state is world-local runtime state.

**Files:**
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`
- Modify: `spec/protocol/combat-normal-attack-bootstrap.md`
- Modify: `docs/qa/manual-client-checklist.md`
- Modify: `docs/development.md`
- Modify: `README.md`

**Verification:**
- QA checklist contains a first repeated-hit smoke section
- docs explicitly say dummy HP is runtime-only, not persisted character state

---

## Band C — harden combat ownership, visibility, and reconnect edges

### Slice 11: Clear target when visibility or combat range is lost

**Objective:** if the selected dummy leaves visibility or the player leaves the accepted combat band, clear the active combat target deterministically instead of allowing stale attacks to continue.

**Why now:** once attacks mutate HP, stale target retention becomes a correctness bug instead of a harmless bootstrap omission.

**Files:**
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/interaction_visibility_test.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/scopes.go`
- Modify: `spec/protocol/combat-normal-attack-bootstrap.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*(CombatTargetClear|Visibility|Range)' -count=1
```

---

### Slice 12: Clear combat target across transfer, rebootstrap, and reconnect

**Objective:** freeze and implement what happens to selected combat targets when a character transfers, re-enters, or reconnects so no stale dummy linkage survives across a new bootstrap.

**Why now:** the repo already treats reconnect/rebootstrap as first-class runtime seams, so combat must align with those existing lifecycle rules.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `spec/protocol/runtime-reconnect-cleanup.md`
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`
- Modify: `docs/qa/manual-client-checklist.md`

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*(Reconnect|Transfer|CombatTarget)' -count=1
```

---

### Slice 13: Keep stale reclaimed combat attempts non-authoritative

**Objective:** extend the existing reclaim hardening model so a stale post-reclaim socket cannot authoritatively damage a dummy, clear/replace the live owner's target, or mutate runtime combat state.

**Why now:** movement, whisper, item-use, and merchant seams already freeze this ownership rule; combat should not reopen the stale-socket hole.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/worldruntime/*`
- Modify: `spec/protocol/runtime-reconnect-cleanup.md`
- Modify: `spec/protocol/combat-normal-attack-bootstrap.md`
- Modify: `docs/qa/manual-client-checklist.md`

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*(Reclaim|Stale|Combat)' -count=1
```
```bash
go test ./...
```

---

### Slice 14: Reject attacks against dead or replaced target snapshots

**Objective:** ensure a hit attempt fails closed if the selected dummy is already dead, was replaced by a new runtime instance, or no longer matches the current world snapshot behind the selected `vid`.

**Why now:** runtime HP/death state introduces race-like correctness edges even in a single-process bootstrap world.

**Files:**
- Modify: `internal/worldruntime/*`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/combat-normal-attack-bootstrap.md`
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*(DeadTarget|ReplacedTarget|Combat)' -count=1
```

---

### Slice 15: Freeze combat ownership QA and operator-facing troubleshooting notes

**Objective:** update QA and troubleshooting docs so combat ownership bugs (range clear, reconnect clear, stale reclaim, dead-target reject) are easy to reproduce and diagnose.

**Why now:** this closes the “invisible rules” gap before death/respawn broadens the state machine again.

**Files:**
- Modify: `docs/qa/manual-client-checklist.md`
- Modify: `docs/debugging-and-profiling.md`
- Modify: `spec/protocol/runtime-reconnect-cleanup.md`
- Modify: `README.md`

**Verification:**
- QA contains explicit combat-ownership checks
- operator docs mention where to inspect combat-world state if local debugging endpoints exist

---

## Band D — complete the first end-to-end dummy death / respawn loop

### Slice 16: Freeze the first death / respawn contract for non-player combatants

**Objective:** document the smallest owned server/client contract for dummy death, target clear, non-attackable dead state, and respawn reset, without claiming loot, EXP, corpse interaction, or fancy animation systems.

**Why now:** the repo needs a written boundary before zero-HP behavior is implemented and asserted in tests.

**Files:**
- Create: `spec/protocol/non-player-death-respawn-bootstrap.md`
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`
- Modify: `spec/protocol/combat-normal-attack-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Verification:**
- docs explicitly define dead-state non-goals and respawn trigger rules
- the chosen death/respawn client refresh path is written down instead of implied

---

### Slice 17: Add failing tests for zero-HP death and post-death rejection

**Objective:** write RED tests proving that repeated hits can drive a dummy to zero HP, flip it into a dead state, clear combat targeting as needed, and reject further hits until respawn.

**Why now:** this is the cleanest TDD seam for the first full combat loop.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/*_test.go`
- Modify: `spec/protocol/non-player-death-respawn-bootstrap.md`
- Modify: `docs/qa/manual-client-checklist.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*(DummyDeath|DummyRespawn|Combat)' -count=1
```
Expected: RED for missing death/respawn behavior.

---

### Slice 18: Implement dummy death transition + no-further-damage semantics

**Objective:** when HP reaches zero, mark the dummy dead, suppress extra damage, clear/refresh targeting as documented, and keep the runtime state internally consistent.

**Why now:** this completes the “death” half of the first honest combat loop.

**Files:**
- Modify: `internal/worldruntime/*`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/proto/combat/combat.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime ./internal/proto/combat -run 'Test.*(DummyDeath|Combat)' -count=1
```
```bash
go test ./...
```

---

### Slice 19: Implement deterministic dummy respawn reset

**Objective:** add the smallest owned respawn mechanism so a dead dummy eventually returns with reset HP and can be targeted/attacked again as a new live combatant.

**Why now:** without respawn, the M4 vertical stops at death and still does not prove a reusable world-runtime loop.

**Files:**
- Modify: `internal/worldruntime/*`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/non-player-death-respawn-bootstrap.md`
- Modify: `docs/development.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*(DummyRespawn|Combat)' -count=1
```

---

### Slice 20: Update README and QA when M4 becomes a real loop

**Objective:** once target -> hit -> HP -> death -> respawn is true, refresh README and QA language so the public repo status clearly says the first end-to-end dummy combat loop exists.

**Why now:** the public-facing milestone should move the moment the loop becomes real, not several slices later.

**Files:**
- Modify: `README.md`
- Modify: `docs/qa/manual-client-checklist.md`
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`
- Modify: `spec/protocol/non-player-death-respawn-bootstrap.md`

**Verification:**
- README M4 text no longer says “target selection only”
- QA exit criteria mention the dummy combat loop explicitly

---

## Band E — generalize the dummy loop into first content-runtime mob footing

### Slice 21: Generalize `CombatKind` into authored combat profiles

**Objective:** replace the single-purpose `training_dummy` toggle with a narrow authored combat-profile concept that can still represent the dummy but can also describe the first practice mob.

**Why now:** once dummy combat is real, the next bottleneck is that combat metadata is too coarse to grow into content runtime.

**Files:**
- Modify: `internal/worldruntime/entity.go`
- Modify: `internal/staticstore/*`
- Modify: `internal/contentbundle/*`
- Modify: `internal/worldruntime/*_test.go`
- Modify: `spec/protocol/combat-training-dummy-bootstrap.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/worldruntime ./internal/minimal -run 'Test.*(CombatProfile|TrainingDummy|Content)' -count=1
```

---

### Slice 22: Freeze the first spawn-group / combat-profile content contract

**Objective:** document the smallest authored content shape for spawning attackable non-player combatants, including profile selection, map placement, and respawn ownership.

**Why now:** the first mob should arrive through an explicit content contract, not another hardcoded test-only registration path.

**Files:**
- Create: `spec/protocol/content-spawn-groups-bootstrap.md`
- Modify: `spec/protocol/non-player-death-respawn-bootstrap.md`
- Modify: `spec/protocol/README.md`
- Modify: `docs/development.md`
- Modify: `README.md`

**Verification:**
- docs define spawn-group scope and explicit non-goals: no wandering AI, no loot tables, no quest hooks yet

---

### Slice 23: Add failing tests for content-loaded attackable non-player spawns

**Objective:** open the RED proving that authored content can load one attackable non-player runtime instance using the generalized combat profile and respawn ownership rules.

**Why now:** this is the narrow bridge from a hardcoded dummy path into a real M5 content-runtime surface.

**Files:**
- Modify: `internal/minimal/content_bundle_runtime_test.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/*_test.go`
- Modify: `spec/protocol/content-spawn-groups-bootstrap.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*(SpawnGroup|CombatProfile|PracticeMob)' -count=1
```
Expected: RED for missing generalized content/runtime behavior.

---

### Slice 24: Implement the first hostile stationary practice mob

**Objective:** land one authored non-player combatant that uses the generalized combat-profile + spawn-group path, can be attacked like the dummy, and completes the same death/respawn loop while still remaining stationary and simple.

**Why now:** this is the smallest honest step from “combat demo object” to “combat-capable game content”.

**Files:**
- Modify: `internal/contentbundle/*`
- Modify: `internal/staticstore/*`
- Modify: `internal/worldruntime/*`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/content_bundle_runtime_test.go`
- Modify: `docs/qa/manual-client-checklist.md`
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*(PracticeMob|SpawnGroup|Combat)' -count=1
```
```bash
go test ./...
```

---

### Slice 25: Leave the next RED for hostile retaliation / aggro-lite

**Objective:** finish this window by writing the failing test and protocol notes for the first hostile post-hit reaction, narrowed to one tiny aggro-lite rule: after the first accepted hit on a content-loaded `spawn_group` practice mob, fresh third-party `TARGET` attempts must fail closed until the existing death/respawn reset boundary, without implementing it yet.

**Why now:** this keeps the repo's momentum pattern healthy: end the window with a concrete next vertical already framed by docs and a real RED.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/*_test.go`
- Modify: `spec/protocol/combat-normal-attack-bootstrap.md`
- Modify: `spec/protocol/content-spawn-groups-bootstrap.md`
- Modify: `docs/plans/2026-05-02-post-training-dummy-next-twenty-five-slices.md`

**Verification:**
```bash
go test ./internal/minimal ./internal/worldruntime -run 'Test.*(MobRetaliation|Aggro|PracticeMob)' -count=1
```
Expected: RED that clearly states the next missing behavior.

---

## Recommended execution order inside the window
1. Finish Band A completely before introducing mutable HP.
2. Finish Band B before spending time on reconnect/reclaim combat edges.
3. Use Band C to make the dummy loop safe under the repo's existing shared-world lifecycle rules.
4. Treat Band D as the line where M4 becomes publicly claimable as a real gameplay loop.
5. Use Band E only after the dummy loop is honest, and stop again once the first hostile-retaliation RED exists.

---

## Explicit non-goals for these 25 slices
- player-vs-player combat
- skill/spell systems
- combo timing or animation fidelity
- loot drops or EXP rewards
- quest triggers from kills
- mob movement/pathing AI
- party combat semantics
- DB-backed world persistence for non-player HP/death state
- production anti-cheat or combat-rate limiting beyond the minimal compatibility checks needed by the slice

---

## Expected repo state after this window
If the full window lands cleanly, the repo should be able to truthfully say:
- M4 has a real first combat loop, not just target selection
- non-player combat state is runtime-owned and test-covered
- reconnect/reclaim rules also hold for combat mutations
- one authored hostile practice mob exists on top of the same runtime/content seams
- the next concrete RED is hostile retaliation / aggro-lite, not a vague “combat someday” placeholder
