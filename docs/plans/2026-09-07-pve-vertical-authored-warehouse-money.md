# PvE vertical authored warehouse money — 2026-09-07

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already opens gated `Warehouse` `open_safebox`
(size `2`) with open-burst `SAFEBOX_MONEY_CHANGE` `0`, and
`/safebox_money_save` / `/safebox_money_withdraw` are already owned, but the
PvE proof never mutates warehouse gold. Carried gold at that first open
already includes authored drop-table `reward_gold = 60` from the pre-guide
`QAPveVerticalMob` kill.

## Why now

- Durable safebox `money`, open-burst `SAFEBOX_MONEY_CHANGE`, and the save /
  withdraw slashes are already owned (`docs/plans/2026-08-23-safebox-money-change-contract-freeze.md`).
- The first composed warehouse open happens after the pre-guide kill credit
  and before cube inspect / reconnect, so a deposit can rematerialize on the
  later authored password reopen without stretching the loop into cube `add`
  / `make` or a second shop visit.
- Authored `loot.qa_pve_vertical_reward.reward_gold = 60` is the deposit
  amount; do not invent a lab gold constant.
- The later shop / turn-in gold math stays derived from live snapshots; this
  slice withdraws the same `60` before check-in/out/sell so those assertions
  keep using restored carried gold.

## Contract frozen by this slice

1. After the first unlocked `Warehouse` `/safebox_password 000000`
   (`SAFEBOX_SIZE` size `2` + `SAFEBOX_MONEY_CHANGE` `0`), talking-chat
   `/safebox_money_save 60` emits self-only gold `PLAYER_POINT_CHANGE`
   (`-60`) plus `SAFEBOX_MONEY_CHANGE` `60`, deducts carried gold, and
   persists durable same-account warehouse money `60`.
2. `/close_safebox` and the same-session password cooldown reject leave that
   durable `60` in place; cube `r_info` / `m_info` still do not mutate gold.
3. After reconnect / `QuestGuide` re-unlock, authored `Warehouse` `INTERACT`
   + `/safebox_password 000000` rematerializes `SAFEBOX_MONEY_CHANGE` `60`
   (not `0`).
4. `/safebox_money_withdraw 60` restores carried gold with self-only gold
   `PLAYER_POINT_CHANGE` (`+60`) plus `SAFEBOX_MONEY_CHANGE` `0` before
   equipped `SAFEBOX_CHECKIN` / unequip / check-in / size-2 move / check-out / sell.

## What this is not yet

- cube `add` / `make` / `make all` in the same composed proof
- refine in the same composed proof
- mall / TMP4 CG `SAFEBOX_MONEY` request header / client change-password
- claiming the whole storage/economy system is now template-complete

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -count=1 \
  -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$'
git diff --check
```
