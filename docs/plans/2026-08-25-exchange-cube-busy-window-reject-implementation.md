# Exchange Cube Busy-Window Reject Implementation — 2026-08-25

## Objective

Land the frozen open-cube exchange busy rejects so lab `/open_cube` blocks
trade the same way merchant/safebox/refine/MYSHOP already do.

## Owned

1. Requester open cube rejects `EXCHANGE START` / `ACCEPT` with
   `You cannot trade while another trade window is open.`
2. Partner open cube rejects those seams (and commit-time busy drift) with
   `That player cannot trade right now.`
3. No pairing / accept / finalize mutation; cube presentation stays open on
   reject.
4. Shared-world `hasCubeWindowOpenLocked` participates in START/ACCEPT/commit
   busy gates beside merchant/safebox/refine/MYSHOP.

## Proof

- `go test ./internal/minimal -run 'TestGameRuntimeItemExchangeStartRejects(ActiveCube|PartnerActiveCube)|TestSharedWorldAcceptExchangeRejectsOpenCube|TestSharedWorldCommitExchangeFinalizeRejectsBusyWindow|TestGameRuntimeItemCube' -count=1`

## Status

Shipped on `lane/items`.
