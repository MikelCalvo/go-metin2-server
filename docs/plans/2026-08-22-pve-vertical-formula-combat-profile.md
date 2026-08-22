# PvE Vertical Formula Combat Profile — 2026-08-22

## Objective

Compose the already-owned formula-first portable `combat_profiles` seam into
`docs/examples/bootstrap-pve-vertical-authoring-bundle.json` so the one-bundle
playable PvE QA path exercises authored damage/HP (`max(1, attack - defense)`)
beside regen/drop expansion, quest unlock, warehouse, merchant, and turn-in.

Also correct the stale `content-spawn-groups-bootstrap.md` success-definition
wording that still claimed retaliation point-loss "does not yet persist" across
`/phase_select` / reconnect even though the bootstrap `0`-HP floor already
persists with the owned death/clear frames.

## Contract frozen by this slice

1. `bootstrap-pve-vertical-authoring-bundle.json` authors:
   - portable profile `qa_pve_vertical_practice_mob` with `max_hp = 20`,
     `attack_value = 9`, `defense_value = 4`, `respawn_delay_ms = 2000`
   - regen spawn `practice.qa_pve_vertical_mob` referencing that profile
2. Canonicalize / validate derive `damage_per_normal_attack = 5` and default
   `level = 1`, keep the portable `combat_profiles[]` row, and preserve the
   authored drop-table EXP/gold/drop + kill-quest credit on the spawn group
   (table descriptor wins over any profile-default reward).
3. Focused gameplay proof kills the vertical mob in four formula hits and still
   closes guide unlock → kill credit → hunter turn-in.
4. Spawn-groups success text distinguishes partial runtime-only retaliation
   loss from the persisted `0`-HP floor.

## What this is not yet

- full legacy combat math beyond `max(1, attack_value - defense_value)`
- weighted/random loot tables
- broader player corpse / revive menus
- skill / ranged / PvP runtime policy

## TDD and validation

```bash
go test ./internal/contentbundle -run 'CanonicalizePveVerticalAuthoringExample' -count=1
go test ./internal/ops -run 'ValidateEndpointExpandsPveVerticalAuthoringExample' -count=1
go test ./internal/minimal -run 'PveVerticalAuthoringBundle' -count=1
gofmt -w <touched Go files>
git diff --check
```

## Follow-up options

1. Keep weighted/random loot and full legacy formulas deferred.
2. ~~Optional: assert first-hit formula `DAMAGE_INFO` amount inside the vertical
   gameplay proof if QA wants packet-level coverage in the same fixture.~~ Done:
   `TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn` now asserts
   the first pre-guide and post-guide live hits emit `TARGET(75%)` plus spawn-backed
   retaliation/`DAMAGE_INFO` companions with formula `damage = 5`.
3. Next combat-lane candidates remain engagement cleanup only where an owned
   invariant is still missing, or deferred skill/ranged/PvP codecs.
