# NPC Gameplay — Next Ten Vertical Slices Implementation Plan

> **For Hermes:** use `test-driven-development`. Keep slices tiny, public, and green on `main`.

**Goal:** turn the current static-actor `INTERACT` seam into the first real NPC gameplay loop by adding honest service-style NPC interactions that the current runtime can already support end-to-end, starting with player-facing failure feedback, range-gated interaction, one-click warp NPCs, and a read-only shop preview path.

**Architecture:** build directly on the existing owned seams:
- visible static actors in `internal/worldruntime`
- `INTERACT (0x0501)` ingress in `GAME`
- validated visible-target interaction attempts in `internal/minimal/shared_world`
- authored `interaction_kind + interaction_ref` content behind bootstrap static actors
- deterministic file-backed interaction definitions plus authored-content bundles
- existing transfer/rebootstrap behavior for gameplay-triggered relocation

Keep `internal/worldruntime` owning visibility and target lookup, keep `internal/interactionstore` as the deterministic authored-content boundary, and keep `internal/minimal` owning runtime resolution, session-scope rules, and self-facing or transfer-triggered execution behavior.

**Tech Stack:** Go 1.26, `internal/interactionstore`, `internal/contentbundle`, `internal/minimal`, `internal/worldruntime`, `internal/ops`, protocol docs under `spec/protocol/`, plans under `docs/plans/`, QA docs under `docs/qa/`, validation with `gofmt`, focused `go test`, `go test ./...`, and `go vet ./...`.

---

## Current starting point
- Current `main` head before this plan starts: `0f42214`
- Already owned in repo:
  - static actors are runtime-owned, visible, AOI-aware, persisted, authorable, and client-visible
  - `INTERACT (0x0501)` exists as a deterministic `GAME`-phase packet targeting visible static actors by `vid`
  - `internal/worldruntime` can resolve visible static actors by that `vid`
  - `internal/minimal/shared_world` already validates interaction attempts and distinguishes fail-closed reasons
  - authored `info` and `talk` interactions already resolve to self-only chat-backed deliveries
  - content bundles already export/import bootstrap static actors plus interaction definitions
  - gameplay-triggered transfer/rebootstrap already exists and is reusable
- Important constraints that should drive the next slices:
  - inventory, equipment, and item-use are still `[ ] Not started`, so real buy/sell is not an honest immediate target
  - quest/script runtime is still `[ ] Not started`, so branching mission logic or persistent quest rewards should stay out of scope here
  - there is still no frozen client-side NPC dialog-window or option-selection packet contract in this repository
  - the next NPC loop should therefore reuse already-owned runtime and packet seams instead of inventing speculative protocol

---

## Ordering and scope rules
1. Prioritize the first **real NPC gameplay payoff** over broad content infrastructure.
2. Reuse the existing `INTERACT` ingress and current transfer runtime before inventing any new packet family.
3. Make failure behavior player-visible and deterministic before adding richer success behavior.
4. Add only interaction kinds that the current project can support honestly end-to-end.
5. Keep shops as **read-only preview only** until inventory/currency/item mutation exists.
6. Keep quests, branching dialog trees, and script VM work out of this 10-slice window.
7. Every behavior-changing slice should land docs + focused RED tests + code + README status alignment in the same commit.

---

## Task 1: Freeze the bootstrap NPC gameplay contract around service-style interactions

**Objective:** document the first honest NPC gameplay families the repo will own next, so later implementation is driven by a precise scope instead of by ad hoc handler growth.

**Files:**
- Create: `spec/protocol/npc-service-interactions-bootstrap.md`
- Modify: `spec/protocol/static-actor-interaction-request.md`
- Modify: `spec/protocol/static-actor-interaction-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Steps:**
1. Freeze the first post-`info`/`talk` interaction families as service-style NPC behavior rather than full dialog trees.
2. Define the first two target families clearly:
   - `warp` / teleporter-style interaction
   - `shop_preview` / browse-only catalog preview
3. State that both still begin with the existing `INTERACT (0x0501)` packet and a visible static-actor target by `vid`.
4. Make the temporary contract explicit:
   - no new packet header yet
   - no inventory mutation yet
   - no quest state yet
   - no branching dialog UI yet
5. Register the new spec and align README milestone/status language with that scope.

**Verification:**
- docs no longer describe NPC gameplay as completely absent once this plan starts executing
- the new spec is internally consistent with existing interaction, static-actor, and transfer docs

---

## Task 2: Make interaction failure outcomes self-visible instead of silently fail-closed

**Objective:** stop `INTERACT` from failing as an invisible no-op when the runtime already knows why the request was rejected.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/static-actor-interaction-request.md`
- Modify: `README.md`

**Steps:**
1. Add RED tests for current failure reasons resolving to deterministic self-only `GC_CHAT` feedback.
2. Cover at least:
   - `subject_not_found`
   - `target_not_visible`
   - `target_has_no_interaction`
   - `interaction_definition_not_found`
   - `unsupported_interaction_kind`
3. Keep the wording minimal and bootstrap-honest; these are debugging-friendly player messages, not final localized UX.
4. Implement only the minimal mapping from known failure reasons to self-facing `CHAT_TYPE_INFO` deliveries.
5. Update the protocol doc so failure semantics are no longer described as pure silent fail-close for every path.

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*Interaction.*(Failure|Reject|Missing|Unsupported)' -count=1
```

---

## Task 3: Add an explicit range gate for `INTERACT`, separate from visibility ownership

**Objective:** avoid whole-map-visible or broad-radius players interacting with distant actors just because visibility currently permits target lookup.

**Files:**
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/worldruntime/scopes.go`
- Modify: `internal/worldruntime/scopes_test.go`
- Modify: `spec/protocol/static-actor-interaction-request.md`
- Modify: `README.md`

**Steps:**
1. Add RED tests proving that a target may be visible but still rejected for interaction when it is outside the current interaction radius.
2. Introduce one small runtime/helper boundary for distance validation; do not duplicate geometry rules across callers.
3. Keep the first rule simple and global for bootstrap runtime:
   - one configured or fixed max interaction distance
   - same behavior for `info`, `talk`, `warp`, and later `shop_preview`
4. Return a deterministic self-visible deny message using the new Task 2 failure-delivery pattern.
5. Update the spec so interaction scope means **visible and near enough**, not merely visible.

**Verification:**
```bash
go test ./internal/worldruntime ./internal/minimal -run 'Test.*Interaction.*(Range|Distance|Visible)' -count=1
```

---

## Task 4: Extend `internal/interactionstore` for authored warp definitions

**Objective:** support the first real NPC action using a deterministic, file-backed authored payload instead of hardcoding transfer destinations in handlers.

**Files:**
- Modify: `internal/interactionstore/store.go`
- Modify: `internal/interactionstore/file_store.go`
- Modify: `internal/interactionstore/file_store_test.go`
- Modify: `README.md`
- Modify: `spec/protocol/npc-service-interactions-bootstrap.md`
- Modify: `spec/protocol/static-actor-interaction-authoring.md`

**Steps:**
1. Add RED tests for a new `warp` definition kind.
2. Extend the authored definition model so `warp` can carry at least:
   - `map_index`
   - `x`
   - `y`
   - optional `text` for the self-facing notice delivered before or with the action
3. Keep existing `info` and `talk` definitions backward-compatible and deterministic.
4. Reject malformed warp definitions fail-closed in store validation and file-load validation.
5. Keep normalization/deterministic save behavior stable.

**Verification:**
```bash
go test ./internal/interactionstore -run 'Test.*(Warp|Snapshot|RoundTrip|Invalid)' -count=1
```

---

## Task 5: Extend authoring surfaces and content bundles for warp definitions

**Objective:** make authored warp NPCs manageable through the same loopback-only content workflow already used for static actors and current interaction definitions.

**Files:**
- Modify: `internal/contentbundle/bundle.go`
- Modify: `internal/contentbundle/bundle_test.go`
- Modify: `internal/ops/pprofmux.go`
- Modify: `internal/ops/interaction_definitions_test.go`
- Modify: `internal/minimal/interaction_definitions_runtime_test.go`
- Modify: `README.md`
- Modify: `spec/protocol/static-actor-interaction-authoring.md`

**Steps:**
1. Add RED bundle tests proving warp definitions round-trip and validate correctly.
2. Extend `/local/interactions` request/response parsing to accept the warp payload fields without breaking existing `info` / `talk` authoring.
3. Ensure bundle import rejects dangling or malformed warp definitions before mutating runtime state.
4. Ensure static actors that declare `interaction_kind = "warp"` still fail closed at boot and on runtime create/update when their referenced definition is missing or invalid.
5. Update the authoring spec and README ops examples.

**Verification:**
```bash
go test ./internal/contentbundle ./internal/ops ./internal/minimal -run 'Test.*(Interaction|Bundle|Warp)' -count=1
```

---

## Task 6: Execute warp NPC interactions through the existing transfer/rebootstrap pipeline

**Objective:** land the first real NPC gameplay loop by letting a visible, in-range static actor transfer the interacting player using already-owned transfer behavior.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/minimal/interaction_definitions_runtime_test.go`
- Modify: `spec/protocol/npc-service-interactions-bootstrap.md`
- Modify: `spec/protocol/transfer-rebootstrap-burst.md`
- Modify: `README.md`

**Steps:**
1. Add RED tests for `interaction_kind = "warp"` resolving from a visible static actor.
2. Reuse the existing interaction attempt seam, authored-definition resolution, and transfer/rebootstrap path instead of inventing new relocation code.
3. Define and test the first owned success contract:
   - the player interacts with the NPC
   - optional authored text is delivered self-facing if present
   - the existing transfer/rebootstrap path applies
   - dialog/session state does not survive across the transfer
4. Keep peer visibility and static-actor rebootstrap behavior aligned with the current transfer contract.
5. Update docs to make this the first real NPC action owned by the repo.

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*(WarpInteraction|TransferTrigger|Interact).*' -count=1
```

---

## Task 7: Harden warp NPC safety and deny paths with deterministic reasons

**Objective:** make warp NPCs fail safely when authored destinations are invalid or cannot be applied, rather than partially mutating runtime state or failing opaquely.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `internal/interactionstore/file_store_test.go`
- Modify: `spec/protocol/npc-service-interactions-bootstrap.md`
- Modify: `README.md`

**Steps:**
1. Add RED tests for invalid or rejected warp destinations:
   - zero / invalid map index
   - coordinates outside the currently owned bootstrap transfer contract
   - runtime transfer rejection / not applied
2. Keep validation split honest:
   - shape validation in `interactionstore`
   - runtime applicability validation at execution time
3. Return deterministic self-facing deny messages when execution cannot proceed.
4. Confirm no partial live-state mutation survives failed warp execution.
5. Update the spec with a small failure table so operators know what authoring/runtime failures look like.

**Verification:**
```bash
go test ./internal/minimal ./internal/interactionstore -run 'Test.*(Warp|Transfer).*(Reject|Invalid|NotApplied)' -count=1
```

---

## Task 8: Add a minimal per-player per-actor interaction cooldown

**Objective:** stop spammy repeated `INTERACT` traffic from producing unbounded chat spam or repeated warp attempts against the same NPC.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `README.md`
- Modify: `spec/protocol/npc-service-interactions-bootstrap.md`

**Steps:**
1. Add RED tests for a small runtime cooldown keyed by at least:
   - subject/session identity
   - target actor identity
2. Apply the cooldown to all owned interaction kinds, including `info`, `talk`, `warp`, and later `shop_preview`.
3. Keep the first implementation runtime-only and intentionally simple.
4. Return a deterministic self-visible deny message or no-op contract; whichever is chosen, freeze it in docs and keep it consistent.
5. Ensure cooldown state is cleared appropriately across disconnect/reconnect and stale-session reclaim paths.

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*Interaction.*(Cooldown|Throttle|Repeat)' -count=1
```

---

## Task 9: Add a read-only `shop_preview` interaction kind

**Objective:** deepen NPC gameplay with a real merchant-like interaction that is still honest about current blockers by showing a catalog preview without mutating inventory or currency.

**Files:**
- Modify: `internal/interactionstore/store.go`
- Modify: `internal/interactionstore/file_store.go`
- Modify: `internal/interactionstore/file_store_test.go`
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/interaction_definitions_runtime_test.go`
- Modify: `internal/ops/interaction_definitions_test.go`
- Create: `spec/protocol/npc-shop-preview-bootstrap.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Steps:**
1. Add RED tests for a new `shop_preview` interaction kind.
2. Keep the first authored shape extremely small:
   - reuse authored preview text or a compact catalog body
   - return self-only chat-backed output only
   - no price mutation, no inventory checks, no purchase path
3. Route the new kind through the same runtime validation and range gate as the other interaction kinds.
4. Update authoring endpoints and docs so operators can seed merchant preview content without implying a real shop system exists.
5. Keep the new spec explicit that this is a browse-only merchant bootstrap, not a buy/sell implementation.

**Verification:**
```bash
go test ./internal/interactionstore ./internal/minimal ./internal/ops -run 'Test.*(ShopPreview|InteractionDefinition|Merchant)' -count=1
```

---

## Task 10: Refresh visibility previews, content examples, and manual QA for the new NPC gameplay slice

**Objective:** make the new NPC vertical operable and testable through existing local endpoints and the real client checklist, instead of leaving the behavior only in code and tests.

**Files:**
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/interaction_visibility_test.go`
- Modify: `internal/ops/interaction_visibility_test.go`
- Modify: `internal/contentbundle/bundle_test.go`
- Modify: `docs/qa/manual-client-checklist.md`
- Modify: `README.md`
- Optionally create: `docs/examples/bootstrap-npc-service-bundle.json`

**Steps:**
1. Extend interaction-visibility previews so the new kinds are inspectable through loopback tooling.
2. Add focused tests proving preview output stays deterministic for `warp` and `shop_preview` actors.
3. Update the manual QA checklist with the first NPC gameplay expectations that now should pass:
   - talk/info still work
   - warp NPC works
   - shop preview works as read-only preview only
4. Keep out-of-scope QA items explicit:
   - real buy/sell
   - inventory mutation
   - quest acceptance/progression
5. If useful, add one deterministic example content bundle showing at least:
   - one teleporter NPC
   - one merchant-preview NPC

**Verification:**
```bash
go test ./internal/minimal ./internal/ops ./internal/contentbundle -run 'Test.*(InteractionVisibility|Bundle|Warp|ShopPreview)' -count=1
```

---

## Recommended execution grouping
- **Tasks 1-3**: freeze the NPC gameplay contract and make interaction behavior safe and legible
- **Tasks 4-7**: land the first real NPC gameplay vertical with authored warp definitions and runtime-safe execution
- **Tasks 8-10**: harden spam behavior, add merchant-style preview depth, and make the whole vertical operable for QA

---

## Global validation rule for every slice in this plan
Before marking any slice complete:
- write the focused failing test first and observe RED
- run `gofmt -w` on every touched Go file
- rerun the focused tests until green
- run `go test ./...`
- run `go vet ./...`
- keep docs aligned with the actual runtime behavior, especially non-goals
- keep the tree clean before starting the next slice
- push after each completed slice

## Anti-goals for this 10-slice window
Do **not** do these here:
- real buy/sell shop flows
- inventory or currency mutation
- equipment or item-use work
- quest flags, mission progression, or script VM work
- branching dialog-window UI protocol design without captures
- combat, target selection, damage, death, or respawn
- mob AI, spawn groups, or pathing
- DB persistence redesign
- inter-channel or inter-process NPC ownership

## Ready-to-start next slice
Begin with **Task 1: Freeze the bootstrap NPC gameplay contract around service-style interactions**.
It has the best immediate return because the repo already owns the static-actor interaction ingress and transfer runtime, but the current docs still treat NPC gameplay as effectively absent beyond `info` / `talk` chat responses.
