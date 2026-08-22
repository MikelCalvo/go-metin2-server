# Exchange Accept Gold-Carrier Cap Reject Chat — 2026-08-22

## Objective

Mirror the owned `EXCHANGE START` gold-carrier-cap reject chat onto first-side and second-side `ACCEPT` when either paired side's live gold has drifted to or above the owned signed `PLAYER_POINT_CHANGE` / bootstrap gold carrier max (`1<<31-1`) after the shell opened, instead of leaving that mid-shell drift silent until a later gold-overflow finalize attempt.

## Contract to own

1. On `ACCEPT` (including first-side accept and second-accept / mutual-accept finalize planning), after the already-owned busy-window gates and before displayed-item / displayed-gold Check revalidation, when the accept requester's live gold is already `>= exchangeGoldPointChangeCarrierMax` (`1<<31-1`), return one self-only `CHAT_TYPE_INFO` with `You have more than 2 Billion Yang. You cannot trade.`, emit no accept/`END`/finalize frames, leave the shell cancellable, and leave inventory/equipment/quickslots/gold/persistence unchanged.
2. When the requester is under the carrier cap but the paired partner's live gold is already `>= exchangeGoldPointChangeCarrierMax`, return one self-only `CHAT_TYPE_INFO` with `The player has more than 2 Billion Yang. You cannot trade with him.`, with the same no-accept / still-cancellable contract.
3. Evaluation order on `ACCEPT` keeps busy-window rejects ahead of this gold-carrier-cap gate, then the already-owned Check / Space / gold-overflow / silent Other finalization preconditions. When both sides are over the cap, the requester-side string wins (local-first, matching START / busy ordering).
4. `CommitExchangeFinalize` non-busy revalidation uses the same self-only gold-carrier-cap chat when post-plan live gold drift hits the carrier max on either side, rolls back any already-written account/live snapshots from that finalize attempt (already owned), emits no finalize/accept/`END` frames, and leaves the shell cancellable. Busy-window chat stays as already owned and is evaluated before this gate.
5. Spec/QA name these strings beside the owned START gold-carrier-cap and busy / finalize reject chats; do not invent a new `GC::EXCHANGE` result subheader.

## What this is not yet

- partner-side open player-shop / cube busy-window rejection text
- dual-sided chat for item-id collision / over-template-max / locked-compatible-stack / selected-character / transfer-guard finalization rejects
- changing the already-owned second-accept / commit-time gold-overflow dual-sided chat (`You cannot carry any more gold.` / partner wording)
- restart-restored ground ownership / despawn timers

## TDD and validation (implementation follow-up)

Focused coverage:

- `go test ./internal/minimal -run 'ItemExchangeAcceptRejects.*GoldCarrier|SharedWorldCommitExchangeFinalizeRejects.*GoldCarrier' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep id-collision / restriction finalize reject chat deferred until QA wants those distinguishable from silent fail-closed.
2. Keep partner-side open player-shop / cube busy rejects deferred until those presentation seams exist.
3. Optional later: name pickup selected-character empire anti-flags beside the already-owned job/sex/`min_level` pickup restriction matrix if QA still reads that list as incomplete.

## Status

Contract frozen; implementation / RED not opened in this docs-only commit.
