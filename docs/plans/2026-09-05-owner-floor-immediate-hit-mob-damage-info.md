# Owner-Floor Immediate Hit Mob Damage Info — 2026-09-05

## Objective

Close the remaining bootstrap DAMAGE_INFO presentation gap where an accepted
practice-mob normal hit that also floors the owner still mutated runtime mob HP
and refreshed `TARGET(target_vid, hp_percent)`, but omitted the mob hit-effect
companion that live non-floor hits and killing hits already emit.

## Why now

- Live spawn-backed hits already emit self plus visible-peer
  `DAMAGE_INFO(target_vid)` after the immediate retaliation point-change.
- Killing hits already append one mob `DAMAGE_INFO` after `DEAD(vid)` plus
  selected-session `TARGET(0, 0)`.
- Owner-floor immediate, delayed, and proximity-armed beats already append owner
  `DAMAGE_INFO` after death/clear.
- Specs still named owner-floor immediate hits as the one accepted-hit path that
  omitted the mob companion, so a client watching a last-hit death never saw the
  dummy's hit number even though the attack was accepted.

## Contract owned by this slice

1. An accepted spawn-backed practice-mob normal hit that also drives the owner to
   the bootstrap `0`-HP floor keeps the existing death/clear prefix:
   `TARGET(target_vid, hp_percent)` → `PLAYER_POINT_CHANGE(value=0)` →
   `DEAD(owner_vid)` → `TARGET(0, 0)`.
2. That same accepted hit then appends one self plain mob
   `DAMAGE_INFO(target_vid, flag=0, damage=applied_bootstrap_damage)` before the
   already-owned owner `DAMAGE_INFO(owner_vid, abs(final_clamped_delta))`.
3. Currently visible live peers receive that same mob companion after
   `DEAD(owner_vid)` and before the owner companion. They still do not receive
   the owner's `PLAYER_POINT_CHANGE` or `TARGET(0, 0)` clear.
4. Delayed and proximity-armed owner-floor beats stay unchanged: they never
   accepted an owner `ATTACK`, so they still emit only the owner companion after
   death/clear.

## Focused coverage

- `TestGameSessionFlowPracticeMobImmediateOwnerFloorHitEmitsMobAndOwnerDamageInfo`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobImmediateOwnerFloorHitEmitsMobAndOwnerDamageInfo$' -count=1
```

Neighbor stay-green:

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobImmediate(OwnerFloorHitEmitsMobAndOwnerDamageInfo|RetaliationSendsSelfDeadBeforeTargetClearAtOwnerHPFloor|RetaliationQueuesVisiblePeerDeadAtOwnerHPFloor)|TestGameRuntimeProximityAggroDelayedRetaliationReachesOwnerDeathFloorWithoutHitOrTarget' -count=1
```

## What this is not yet

- delayed / proximity-armed floors inventing a synthetic mob hit-effect
- skill / ranged / PvP `DAMAGE_INFO` policy
- richer flag meanings (crit/miss/block)

The combined last-hit (dummy dies and owner floors in one accepted `ATTACK`) is now owned by `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsCombinedDeathBurst`.
