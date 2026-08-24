# Hermetic Proximity Aggro Suppress MemoryStore Tests — 2026-08-24

## Objective

Convert the six disposable content FileStores in
`internal/minimal/proximity_aggro_suppress_test.go` to hermetic MemoryStores so
leave/re-enter proximity suppress proofs no longer allocate path-isolated
static/interaction JSON stores.

This closes the remaining documented world/content hermetic follow-up from:

- [hermetic shared-world content-bundle import MemoryStore tests](2026-08-24-hermetic-shared-world-content-bundle-import-memorystore-tests.md)
- [hermetic spawn-group operator position MOVE MemoryStore tests](2026-08-24-hermetic-spawn-group-operator-position-move-memorystore-tests.md)
- [hermetic shared interaction/item template helpers](2026-08-23-hermetic-shared-interaction-item-template-helpers.md)

## Why now

- Track A bootstrap chase / homeward / proximity / anti-leak seams are otherwise
  closed through the cross-map delete/readd freeze.
- These proximity suppress proofs only assert live engagement release, death /
  respawn seed, death-floor `/restart_here`, Leave→Join park/claim, and
  content-bundle suppress remapping; they do not rematerialize content from disk
  or exercise backup/restore paths.
- Neighboring spawn-homeward / spawn-return / operator MOVE / shared-world import
  suites already inject `staticstore.NewMemoryStore()` /
  `interactionstore.NewMemoryStore()`.

## Contract owned by this slice

1. The following focused tests construct content stores with MemoryStores:
   - `TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterInRadiusRelease`
   - `TestGameRuntimeProximityAggroDeathAndRespawnSeedSuppressesNearbyReacquireUntilLeaveAndReenter`
   - `TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorRestartHere`
   - `TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorPhaseSelectRestartHere`
   - `TestGameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorReconnectRestartHere`
   - `TestGameRuntimeProximityAggroSuppressRemapsAcrossContentBundleReplacement`
2. Construction uses:
   - `staticstore.NewMemoryStore()`
   - `interactionstore.NewMemoryStore()`
3. Runtime construction continues through
   `newGameRuntimeWithAccountStoreAndContentStores(...)`.
4. Login-ticket / account FileStores remain intentional for session bootstrap and
   character HP persistence across `/restart_here` / reconnect.
5. The pure shared-world registry proof
   `TestSharedWorldRegistrySubjectReleaseSeedsProximitySuppressWhenOwnerAlreadyAtHPFloor`
   stays FileStore-free and is unchanged.
6. Explicit FileStore construction for restart/backup/path-isolated proofs stays
   on those dedicated suites.

## Explicit non-goals

- remapping proximity suppress across daemon restart (next Track A seam; needs
  contract freeze before RED)
- converting remaining FileStores in item/restart suites that still assert
  filesystem rematerialize
- production `NewGameRuntime` accepting injected content MemoryStores by default
- pack AI / synchronized respawn / pathfinding
- inventing cross-map return MOVE / `GC WARP` choreography

## Validation

```bash
gofmt -w internal/minimal/proximity_aggro_suppress_test.go
go test ./internal/minimal -run 'Test(GameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterInRadiusRelease|GameRuntimeProximityAggroDeathAndRespawnSeedSuppressesNearbyReacquireUntilLeaveAndReenter|GameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorRestartHere|GameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorPhaseSelectRestartHere|GameRuntimeProximityAggroSuppressesReacquireUntilLeaveAndReenterAfterOwnerDeathFloorReconnectRestartHere|GameRuntimeProximityAggroSuppressRemapsAcrossContentBundleReplacement|SharedWorldRegistrySubjectReleaseSeedsProximitySuppressWhenOwnerAlreadyAtHPFloor)$' -count=1
git diff --check
```

## Follow-up options

1. ~~Freeze then implement daemon-restart proximity-suppress rematerialization by
   authored `spawn_group_ref` + character VID park/claim (engagement stays
   fail-closed across restart).~~ Docs-first freeze landed: see
   [proximity suppress daemon-restart rematerialize contract freeze](2026-08-24-proximity-suppress-daemon-restart-rematerialize-contract-freeze.md).
   Next: RED then GREEN `TestGameRuntimeProximityAggroSuppressRematerializesAcrossDaemonRestart`.
2. Keep pack AI / synchronized respawn / pathfinding deferred.
3. Keep inventing cross-map return MOVE / `GC WARP` cancelled for Track A
   bootstrap scope.
