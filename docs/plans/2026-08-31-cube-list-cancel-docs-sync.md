# Cube list/cancel docs sync — 2026-08-31

## Objective

Close the remaining Track C docs drift after GREEN `/cube list` /
`/cube cancel` / `/cube close` (`docs/plans/2026-08-26-cube-list-cancel.md`)
so exchange-bootstrap, QA expected-results, and older plan status tails stop
claiming those seams are still deferred.

## Why this exists

Runtime + focused tests already own:

- `/cube list` ordered self-only INFO dump of live bound craft slots
- `/cube cancel` / `/cube close` as aliases of `/close_cube`
- fail-closed closed/malformed list paths

Canonical protocol ownership lives in `spec/protocol/item-cube-bootstrap.md`
and QA section 4.5.16 already exercises list/cancel. The exchange busy-window
surface and a few Aug-24/25 plan status tails still said
`list / cancel stay deferred` / `recipe make/add/list stay deferred`, which
contradicts the owned cube contract operators read beside exchange busy
rejects.

## Contract owned by this slice

1. `spec/protocol/item-exchange-bootstrap.md` names craft-slot
   add/del / make / make all / list / cancel as owned and keeps only
   complicated OR-materials / binary cube headers deferred.
2. `docs/qa/manual-client-checklist.md` guest-buy expected-result bullet no
   longer defers recipe make/add/list; it points at section 4.5.16 ownership.
3. Stale Aug-24/25 plan status tails are retipped to the owned list/cancel
   plan without reopening those historical freezes.
4. Track C roadmap tip records this docs sync; refine catalysts / mall /
   party ownership notices / GD/DB `MYSHOP_PRICELIST` stay deferred.

## What this is not yet

- no runtime / packet / test change
- no OR-materials / binary cube headers
- no anti_save / anti_pk_drop mutation policy beyond `ITEM_SET` projection
- no refine catalysts / mall / TMP4 CG `SAFEBOX_MONEY`

## Validation

- docs/spec only; optional smoke:
  `go test ./internal/minimal -run 'TestGameRuntimeCubeList' -count=1`
- `git diff --check`

## Status

Docs-only sync on `lane/items`.
