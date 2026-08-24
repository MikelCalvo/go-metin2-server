# Proximity Suppress Daemon-Restart Rematerialize — 2026-08-24

## Objective

Close the frozen Track A seam so leave/re-enter proximity suppress survives a
clean `gamed` process restart for content-loaded spawn groups, without restoring
engagement or inventing a second permanent suppress store.

## Contract owned by this slice

1. Successful suppress seed on a spawn-backed actor persists optional
   `proximity_suppress_vids` (sorted unique character VIDs) on the static-actor
   snapshot.
2. `loadPersistedStaticActors` restores those VIDs into
   `pendingProximityAggroSuppressByVID` keyed by the rematerialized actor entity
   ID, filtering to still-known account character VIDs when an account store is
   available.
3. Post-restart `Join` / EnterGame reuses the already-owned VID park/claim
   handoff so a still-inside owner stays suppressed until leave/re-enter of the
   effective aggro radius.
4. Engagement / selected-target / chase / return / delayed-retaliation ownership
   stay fail-closed across restart.
5. Suppress clear on leave-radius and engagement acquisition also rewrite the
   durable overlay so restart cannot revive stale suppress.

## Focused coverage

- `TestGameRuntimeProximityAggroSuppressRematerializesAcrossDaemonRestart`
- `TestFileStoreRoundTripsProximitySuppressVIDs`
- `TestFileStoreRejectsMalformedProximitySuppressVIDs`

```bash
go test ./internal/minimal ./internal/staticstore -run 'ProximityAggroSuppressRematerializesAcrossDaemonRestart|ProximitySuppressVIDs' -count=1
```

## What this is not yet

- remapping engagement / selected-target / chase / return schedules across restart
- inventing a second permanent suppress store keyed by name
- pack AI / synchronized respawn / pathfinding
- non-spawn standalone `training_dummy` suppress durability
