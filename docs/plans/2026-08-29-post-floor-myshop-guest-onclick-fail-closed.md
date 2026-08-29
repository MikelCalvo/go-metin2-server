# Post-floor guest MYSHOP ON_CLICK fail-closed + restart recovery — 2026-08-29

## Objective

Close the remaining private-shop busy-shell **open** twin after host-only
`CG::MYSHOP` and shop/silk-bag USE already own quiet fail-closed denial: prove a
still-connected zero-HP owner cannot open a fresh guest private-shop browse via
`CG::ON_CLICK` against a living peer's already-open `MYSHOP`, then recover via
`/restart_here` / `/restart_town` so the same browse succeeds with ordinary
guest `SHOP START` frames and no inventory/gold mutation.

## Contract frozen by this slice

1. Drive the guest to the retaliation HP floor with a content-loaded practice mob
   while a living same-map host already has an accepted open `MYSHOP`.
2. Later guest `CG::ON_CLICK` against that host fails closed with no
   `GC::SHOP START` and without setting the guest browse flag.
3. After `/restart_here`, the same living host can be freshly browsed with
   ordinary guest `SHOP START`.
4. After `/restart_town`, a living destination-map empire-town host with an open
   `MYSHOP` can likewise be freshly browsed.
5. Inventory/gold stay unchanged across the deny + restart + fresh-browse path
   for both guest and host.
6. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorMyShopGuestOnClickFailsClosed`
   - `TestGameSessionFlowPostFloorMyShopGuestOnClickFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific guest-browse packet family
- changing busy-shell rejects for live guests
- reopening already-owned pre-open guest-browse floor-close companions
- inventing guest `SHOP BUY` while dead as a separate surface in this slice
  (browse remains cleared on floor; fresh browse is the open surface)
- inventing revive menus or broader full action-lock policy

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorMyShopGuestOnClick' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check -- internal/minimal/player_death_busy_shell_open_guard_test.go \
  spec/protocol/player-death-bootstrap.md docs/qa/manual-client-checklist.md \
  docs/plans/2026-08-29-post-floor-myshop-guest-onclick-fail-closed.md
```
