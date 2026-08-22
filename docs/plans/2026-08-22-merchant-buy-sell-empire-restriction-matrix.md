# Merchant Buy/Sell Empire Restriction Matrix — 2026-08-22

## Objective

Name empire anti-flags (`anti_empire_a` / `anti_empire_b` / `anti_empire_c`) beside the already-owned job/sex/`min_level` merchant buy/sell restriction matrix in protocol/QA, and freeze focused player-helper plus session coverage so authored merchant restrictions match runtime behavior (`CanUseTemplate` already rejects them).

## Contract owned by this slice

1. `npc-shop-transaction-bootstrap.md` buy validation / sell helper bullets that still said job/sex/level-only explicitly list empire anti-flags beside job/sex/`min_level`.
2. QA checklist merchant buy/sell restriction wording includes empire beside `min_level` / selected-character restrictions, including one authored `buy_reject_message` / `sell_reject_message` case.
3. `TestMerchantTemplateMutationsRejectSelectedCharacterAntiFlagTemplates` covers `anti_empire_a` / `anti_empire_b` / `anti_empire_c` fail-closed with no inventory/gold mutation.
4. Session coverage proves packet `SHOP BUY` / `SHOP SELL2` empire rejects emit `GC::SHOP INVALID_POS` plus authored reject chat when present, with no inventory/gold/persistence mutation.
5. No runtime behavior change beyond making the already-enforced empire path explicit in tests/docs.

## What this is not yet

- new merchant rejection packet families
- peer-facing merchant rejection text
- partner-side open player-shop / cube exchange busy rejects
- durable safebox password / money / persistence
- refine keep-grade / catalyst variants

## TDD and validation

Focused coverage:

- `go test ./internal/player -run 'MerchantTemplateMutationsRejectSelectedCharacterAntiFlagTemplates' -count=1`
- `go test ./internal/minimal -run 'ShopBuyAndSellReject.*Empire|ShopBuyAndSellRejectSelectedCharacterAntiFlag|ShopBuyAndSellRejectMinLevel' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
2. Keep durable safebox persistence / password / money deferred behind a separate store/schema contract freeze.
3. Optional later: align remaining QA smoke bullets that still say job/sex-only for consumable/equip-effect smoke when those fixtures already run through `CanUseTemplate`.

## Status

Shipped: empire anti-flags are named beside job/sex/`min_level` in merchant buy/sell protocol/QA, player merchant restriction matrix covers `anti_empire_a` / `anti_empire_b` / `anti_empire_c` fail-closed with no inventory/gold mutation, and session packet `SHOP BUY` / `SHOP SELL2` coverage freezes authored reject-chat empire rejects. Runtime behavior was already owned through `CanUseTemplate`; this slice freezes the matrix/docs/session contract.
