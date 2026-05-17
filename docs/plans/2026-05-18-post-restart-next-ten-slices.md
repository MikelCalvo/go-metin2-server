# Post-restart Next 10 Slices Implementation Plan

> **For Hermes:** use `test-driven-development`. Keep slices tiny, keep `main` green, and land docs + focused RED tests + code + QA notes together. One commit per slice. Do not guess a non-chat restart packet without first proving the ingress from captures or owned fixtures.

**Goal:** take `go-metin2-server` from the current retaliation-owned player-death plus same-socket slash-command recovery bundle to the next honest legacy-parity decision point: either prove that `/restart_here` and `/restart_town` are already the real restart ingress we should keep owning, or own the smallest dedicated restart request packet without changing the already-frozen recovery outcomes.

**Architecture:** split the next work into three bands:
1. prove the restart-ingress seam from evidence before opening any new packet implementation,
2. if evidence proves a dedicated packet exists, own it as a tiny ingress-only adapter that reuses the existing recovery behavior,
3. independently harden the already-owned slash-command recovery seams with narrow regression coverage instead of widening gameplay claims.

Keep `spec/protocol` as the owner of restart truth, keep `internal/proto/*` tiny and evidence-backed, keep `internal/game` as the gameplay dispatch seam, keep `internal/minimal` as the same-socket recovery orchestrator, and keep the current retaliation-owned death/runtime-vs-persistence split intact until a later player-death policy slice explicitly widens it.

**Tech stack:** Go 1.26, current `internal/minimal`, `internal/game`, `internal/proto/*`, `internal/player`, `internal/worldruntime`, protocol docs under `spec/protocol/`, plans under `docs/plans/`, and live-client QA notes under `docs/qa/manual-client-checklist.md`.

---

## Current starting point
- Current `main` head when this plan is written: `a2525b3 docs: keep player restart ingress slash-command-backed`
- Most relevant recently landed slices:
  - `a2525b3 docs: keep player restart ingress slash-command-backed`
  - `0613a1a docs: freeze restart request follow-up`
  - `00dde74 player: add same-socket restart-town recovery`
  - `f3b1112 docs: freeze restart-town player recovery contract`
  - `1eef582 combat: add same-socket restart-here recovery`
  - `8e89b9a docs: freeze restart-here player recovery contract`
  - `20a6639 combat: retaliate against engaged practice-mob owner`
  - `9e3e1c0 combat: gate fresh third-party targeting on engaged practice mobs`
  - `e23d201 feat: load practice mobs from spawn groups`
- The repo now already owns:
  - secure legacy handshake/login/select/enter-game,
  - shared-world visibility/movement/chat/transfer runtime seams,
  - inventory/equipment/item-use/merchant bootstrap verticals,
  - a content-loaded stationary practice mob with target selection, normal attack cadence, runtime-owned HP, visible death/respawn, an aggro-lite ownership gate, an immediate retaliation tick, and a delayed server-origin retaliation cadence,
  - retaliation-owned player death at `0` HP with self and visible-peer `DEAD`, the current post-floor input/recipient gates, same-socket `/phase_select` recovery, same-socket `/restart_here` recovery, and same-socket `/restart_town` recovery.
- Important repo reality right now:
  - the current restart recovery **result** is already owned and documented,
  - the still-open question is only whether TMP4-era clients require any separate non-chat restart ingress at all,
  - `player-restart-request-bootstrap.md` intentionally leaves that question capture-gated instead of implementation-first,
  - the nearest low-risk code work after that decision is narrow regression coverage around the already-frozen slash-command recovery contract, not a broader revive system.

---

## Ordering principles for this window
1. **Evidence before codec.** Do not create a guessed restart header, mode byte, or packet matrix row just because broader Metin2 folklore mentions one.
2. **Reuse the already-owned recovery outcomes.** Any later dedicated restart request must funnel into the same `/restart_here` and `/restart_town` results the repo already owns today.
3. **Slash-only is an acceptable final answer.** If captures prove `/restart_here` and `/restart_town` are already the correct ingress for the target client/runtime mix, stop there and record that truth instead of inventing more protocol surface.
4. **Keep player-death persistence honest.** Restart work must not silently change the existing rule that retaliation point loss is still runtime-only while practice-mob HP remains world/runtime-owned.
5. **Prefer exact table/ordering regressions over wider behavior.** The current recovery seams already own specific coordinates, packet ordering, and fail-closed gates; lock those down before widening anything else.
6. **Do not widen into revive UI or corpse systems here.** This window is about ingress truth and narrow recovery hardening, not countdowns, revive menus, corpse interactions, or production respawn rules.

---

## Band A — prove what restart ingress actually exists

### Slice 29: Freeze the post-restart decision tree and evidence checklist

**Objective:** write down the next ten slices after the current restart-here / restart-town work so the repo has an explicit roadmap for either a slash-only conclusion or a later packet-backed ingress.

**Why now:** the current implementation has already crossed the behavior boundary, and the remaining uncertainty is about ingress proof, not about inventing another recovery result.

**Files:**
- Create: `docs/plans/2026-05-18-post-restart-next-ten-slices.md`

**Verification:**
- plan reflects current `main` truth
- no new restart behavior is claimed beyond what docs/tests already prove

---

### Slice 30: Land the first owned restart-ingress evidence artifact

**Objective:** add one sanitized capture/fixture note that proves either:
- the current slash-command ingress is already the right legacy-compatible recovery path, or
- a separate dedicated restart request really exists for the target client.

**Why now:** `player-restart-request-bootstrap.md` already says the next honest step is capture/fixture work rather than implementation-first guessing.

**Files:**
- Create or modify: a narrow evidence note under `docs/` or `references/`
- Modify: `spec/protocol/player-restart-request-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md` only if evidence truly owns a new row
- Modify: `README.md` only if public status wording changes

**Verification:**
- the evidence artifact is sanitized and clean-room safe
- the protocol note becomes more certain, not more speculative
- link/filename references stay valid

---

## Band B — branch on what the evidence proves

### Slice 31: If evidence proves slash-only ingress, freeze that conclusion and stop packet work

**Objective:** turn the evidence from Slice 30 into an explicit docs conclusion that `/restart_here` and `/restart_town` are the only owned ingress for now, with no dedicated restart packet added.

**Why now:** if the captures settle the question in favor of slash-only behavior, the honest move is to stop, update docs, and resume with nearby regression hardening instead of pretending a packet still has to exist.

**Files:**
- Modify: `spec/protocol/player-restart-request-bootstrap.md`
- Modify: `spec/protocol/game-slash-command-bootstrap.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Verification:**
- docs say the ingress question is resolved in favor of slash commands
- packet matrix stays free of guessed restart rows
- README remains truthful about current scope

**Branch note:** if Slice 31 lands, skip Slices 32–36 and continue at Slice 37.

---

### Slice 32: If evidence proves a dedicated restart packet, freeze the exact contract first

**Objective:** document the exact restart request family, its header/payload shape, and how its modes map onto the already-owned `/restart_here` and `/restart_town` outcomes.

**Why now:** if a packet truly exists, the docs must own it before any codec or session flow depends on it.

**Files:**
- Modify: `spec/protocol/player-restart-request-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/README.md`
- Modify: `README.md`

**Verification:**
- docs define the exact packet bytes/modes from evidence
- docs say explicitly that the packet reuses the current recovery outcomes instead of creating a second behavior stack

---

### Slice 33: Add failing codec tests for the dedicated restart request

**Objective:** open RED in the smallest protocol package that can own the proved restart request codec.

**Why now:** the gameplay/runtime layers should not decode a new request family until its exact wire contract is owned in tests first.

**Files:**
- Create or modify: `internal/proto/*` tests for the proved restart request family
- Modify: `spec/protocol/player-restart-request-bootstrap.md`

**Verification:**
```bash
go test ./internal/proto/... -run 'Test.*Restart' -count=1
```
Expected: RED for missing codec helpers, not for uncertain packet shape.

---

### Slice 34: Implement the dedicated restart request codec helpers

**Objective:** add the minimal encode/decode helpers for the proved restart request family and nothing more.

**Why now:** this is the smallest green step after the codec RED exists.

**Files:**
- Modify or create: `internal/proto/*`
- Modify: matching codec tests
- Modify: `README.md` only if protocol status wording changes materially

**Verification:**
```bash
go test ./internal/proto/... -run 'Test.*Restart' -count=1
```

---

### Slice 35: Add failing session-flow tests proving the dedicated request reuses current recovery outcomes

**Objective:** open RED in the flow/runtime seam proving that a dedicated restart request, if it exists, still produces the same accepted/denied behavior already frozen for slash `/restart_here` and `/restart_town`.

**Why now:** this prevents a packet-backed ingress from silently widening the owned gameplay contract.

**Files:**
- Modify: `internal/game/flow_test.go`
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/player-restart-request-bootstrap.md`

**Verification:**
```bash
go test ./internal/game ./internal/minimal -run 'Test.*Restart' -count=1
```
Expected: RED for missing dispatch/runtime wiring, not for unresolved output semantics.

---

### Slice 36: Implement dedicated restart-request dispatch as a thin adapter to the current restart helpers

**Objective:** wire the proved request into `GAME`-phase dispatch while reusing the existing same-socket recovery helpers and fail-closed gates.

**Why now:** this owns the ingress without duplicating behavior or widening the restart result surface.

**Files:**
- Modify: `internal/game/flow.go`
- Modify: `internal/minimal/factory.go`
- Modify: matching tests
- Modify: `README.md`

**Verification:**
```bash
go test ./internal/game ./internal/minimal -run 'Test.*Restart' -count=1
```
```bash
go test ./... -count=1
```

---

## Band C — harden the already-owned slash-command recovery seams

### Slice 37: Freeze restart-town empire coverage across the full owned mapping table

**Objective:** add narrow regression coverage for the currently owned empire town-return coordinates instead of relying only on the already-tested empire-2 path.

**Why now:** the contract already freezes exact coordinates for empires 1, 2, and 3, plus the current fallback behavior; the safest next code-facing hardening is to prove the full fixed table before touching any broader restart behavior.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: `spec/protocol/player-restart-town-bootstrap.md` only if the fallback wording needs to become more explicit

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*RestartTown' -count=1
```
Expected first RED, if any: a missing or wrong coordinate/fallback assertion, not a vague restart failure.

---

### Slice 38: Add one more continuity regression around same-socket recovery without widening gameplay scope

**Objective:** pick the smallest remaining unproven continuity rule around the already-owned restart seams and freeze it with a focused RED/GREEN test.

**Preferred candidates, in order:**
1. same-socket `/restart_here` or `/restart_town` still requires a fresh `TARGET` while the same still-live practice mob keeps its current runtime-owned HP,
2. a late-visible peer after restart still sees the correct live/dead state instead of a stale presentation,
3. the existing fail-closed alive-owner guard still holds with no peer-visible side effects across the nearest recovery variant not yet directly covered.

**Why now:** once ingress truth is settled or deferred honestly, the highest-return code work is tiny recovery regression coverage rather than a wider player-death or revive system.

**Files:**
- Modify: `internal/minimal/shared_world_test.go`
- Modify: the nearest protocol note only if the current text is not explicit enough
- Modify: `docs/qa/manual-client-checklist.md` only if a new manual smoke step becomes truly owned

**Verification:**
```bash
go test ./internal/minimal -run 'Test.*(Restart|PhaseSelect|Death)' -count=1
```
```bash
go test ./... -count=1
```

---

## Good stopping points for this window
- after Slice 31 if the evidence proves slash-only ingress and docs are now explicit,
- after Slice 36 if a dedicated restart request was proved and implemented without changing recovery outcomes,
- after Slice 38 if the next remaining work would start to widen into broader revive/death systems rather than keeping the current restart contract tiny and truthful.

## What should still remain out of scope afterward
- revive menus, countdowns, corpse interaction, or broader player-death lifecycle claims,
- guessed restart packet work without evidence,
- EXP/loot/reward systems,
- broader persistence of retaliation-owned player HP loss,
- any rewrite of the already-owned same-socket recovery choreography unless a later capture proves it wrong.
