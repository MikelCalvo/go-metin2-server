# PvE vertical authored CubeMaster r_info / m_info — 2026-09-06

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already INTERACTs gated `CubeMaster` `open_cube`
(`cube open 20022`) and closes it before reconnect, but that window never
requests the already-owned bootstrap recipe list / material-info command-chat
burst. Manual QA already expects `/cube r_info` / `/cube r_info 0` on an open
cube.

## Why now

- Authored `open_cube` INTERACT and lab `/cube r_info` → `cube r_list` /
  `/cube r_info <index>` → `cube m_info` are already owned.
- Runtime boot still falls back to the deterministic lab snapshot for NPC
  `20022` (`reward {27001,1}`, materials `{27002,2}`, gold `100`,
  `percent: 100`) when no cube-recipe FileStore is wired.
- Opening and closing CubeMaster without `r_info` hid a missing client-visible
  craft-inspect step on the same authoring-form import path.
- The composed loop still has empty inventory at that first cube open (gold
  already includes the pre-guide kill credit), so `add` / `make` stay
  fail-closed / out of scope.

## Contract frozen by this slice

1. After unlocked `CubeMaster` INTERACT emits authored info chat +
   `cube open 20022`, talking-chat `/cube r_info` emits one self-only
   `CHAT_TYPE_COMMAND` `cube r_list 20022 1 27001,1`.
2. Talking-chat `/cube r_info 0` on that same open window emits one self-only
   `CHAT_TYPE_COMMAND` `cube m_info 0 1 27002,2/100`.
3. Live and persisted gold / inventory / quest-state stay at the pre-`r_info`
   snapshot (no material/gold/slot mutation).
4. After `/close_cube`, `/cube r_info` stays silent/no-frame.

## What this is not yet

- cube `add` / `make` / `make all` in the same composed proof
- refine in the same composed proof
- claiming the whole cube/craft system is now template-complete
- mall / refine catalysts / party ownership

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -count=1 \
  -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$'
git diff --check
```
