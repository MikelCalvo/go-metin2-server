# Safebox Money Process-Restart Rematerialize Proof — 2026-08-23

## Objective

Prove that durable warehouse gold already owned by `safeboxstore` FileStore +
`/safebox_money_save` / `/safebox_money_withdraw` rematerializes across a full
`gamed` process restart the same way durable cells already do, so Track E
crash/restart coverage for the PvE warehouse-money loop is no longer an
unproven claim beside same-session reopen.

## Why now

- Items-lane already landed optional durable `money`, open-burst
  `SAFEBOX_MONEY_CHANGE`, and slash deposit/withdraw with FileStore writes.
- `TestGameSessionFlowSafeboxMoneySaveWithdrawAndReopenRematerialize` only
  proves same-process close/reopen; `TestGameRuntimeSafeboxCheckinSurvivesProcessRestartRematerializeOnOpen`
  already owns the cell restart pattern.
- The money contract freeze listed "deposit → reconnect/restart reopen
  rematerializes" as required TDD shape, but the restart half was never
  closed with a focused daemon-reload test.
- `0015_character_safebox_money` already tips export/quarantine through money;
  operators still lack an explicit crash-recovery proof that warehouse gold
  survives the same FileStore rematerialize path cells use.
- `internal/safeboxstore/repository.go` still comments the exporter seam as
  tip `0014`, contradicting the shipped `0015` tip.

## Contract frozen by this slice

1. Focused `internal/minimal` coverage deposits warehouse gold via
   `/safebox_money_save`, closes the session, reloads a fresh runtime against
   the same `SafeboxStorePath` + account/ticket dirs, and asserts the next
   `/open_safebox` open-burst emits `SAFEBOX_MONEY_CHANGE` for the deposited
   total while carried gold remains the post-deposit account value.
2. `CharacterSafeboxStateExporter` docs tip the projection surface at
   `0015_character_safebox_money` (password + money + cells).
3. Track E crash/restart wording names durable safebox money rematerialize
   beside cells; mall / TMP4 CG `SAFEBOX_MONEY` / SQL import remain deferred.

## What this is not yet

- mall money / mall open-checkout
- TMP4 CG `SAFEBOX_MONEY` request packet
- SQL import/backfill from quarantined `0015` exports
- automatic artifact GC deletion
- print-only systemd/unit samples
- remote admin authentication
- README churn beyond operator docs already required

## TDD and validation

- `go test ./internal/minimal -run 'SafeboxMoneySurvivesProcessRestart' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep SQL import/backfill deferred until a driver-backed harness exists.
2. Keep mall / TMP4 CG `SAFEBOX_MONEY` deferred on the items lane.
3. Keep automatic / scheduled artifact GC deletion deferred.
4. ~~Optional later: print-only systemd/unit samples for retention / GC printers.~~ Done: see [print-only retention / GC unit samples](2026-08-23-print-only-retention-gc-unit-samples.md) and [lab retention / GC print-only unit samples](../workflow/lab-retention-gc-unit-samples.md).
