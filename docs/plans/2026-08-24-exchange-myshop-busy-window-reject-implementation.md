# Exchange MyShop Busy-Window Reject Implementation — 2026-08-24

## Objective

Land the frozen open-private-shop exchange busy rejects so MYSHOP open blocks trade the same way merchant/safebox/refine already do.

## Owned

1. `SetMyShopWindowOpen` peer-visible busy bit published on accepted open and cleared on empty-sign close / Leave.
2. Requester open MYSHOP rejects `EXCHANGE START` / `ACCEPT` with `You cannot trade while another trade window is open.`
3. Partner open MYSHOP rejects those seams (and commit-time busy drift) with `That player cannot trade right now.`
4. No pairing / accept / finalize mutation; private shop stays open on reject.

## Proof

- `go test ./internal/minimal -run 'TestGameRuntimeItemExchangeStartRejects(ActiveMyShop|PartnerActiveMyShop)|TestGameRuntimeMyShop' -count=1`

## Status

Shipped on `lane/items`.
