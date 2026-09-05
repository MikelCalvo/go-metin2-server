# Open-Cube Interaction-Visibility Owned-Kind Alignment — 2026-09-05

## Objective

Close the remaining content-lane honesty gap after authored `open_cube`
`INTERACT` already landed: prove loopback interaction-visibility previews for
visible craftsman actors, and align the owned-kind / previewable-kind contract
so operators no longer treat `open_cube` as an unfrozen future family.

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Runtime already resolves `KindOpenCube` compact previews
  (`open_cube` or `<text> [open_cube]`) and gated mismatch text
  (`Quest requirements are not met.`) through the same helper used by live
  `INTERACT`.
- Focused visibility coverage already owns the warehouse twin
  (`TestGameRuntimeInteractionVisibilityReturnsServicePreviewsForVisibleWarpAndShopPreviewActors`
  plus `TestGameRuntimeInteractionVisibilityReturnsQuestGatedOpenSafeboxMismatchPreviewWithoutMutatingQuestState`)
  but never registers `CubeMaster`.
- Authoring / bootstrap / request specs still list owned kinds as
  `info` / `talk` / `quest_flag` / `warp` / `shop_preview` (sometimes plus
  `open_safebox`) even though store validation, content-bundle import, QA
  fixtures, and live `INTERACT` already accept `open_cube`.
- Manual QA still has to invent whether `/local/interaction-visibility` should
  show craftsman rows, which drifts from the owned NPC service examples.

## Contract owned by this slice

1. Visible ungated `open_cube` actors preview
   `"<text> [open_cube]"` (or `"open_cube"` when text is blank) with no
   `resolution_failure`.
2. Gated `open_cube` mismatch previews
   `Quest requirements are not met.` without mutating quest-state.
3. Spec / QA / ops docs name `open_cube` beside `open_safebox` in the owned
   definition-kind, previewable-kind, and service-family lists.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned live `INTERACT` cube open / busy-shell contract
- further checked-in foreign-field negatives

## Validation

```bash
gofmt -w internal/minimal/interaction_visibility_test.go internal/ops/interaction_visibility_test.go
go test ./internal/minimal ./internal/ops -run 'Test(GameRuntimeInteractionVisibilityReturnsServicePreviewsForVisibleWarpAndShopPreviewActors|GameRuntimeInteractionVisibilityReturnsQuestGatedOpenCubeMismatchPreviewWithoutMutatingQuestState|LocalInteractionVisibilityEndpointReturnsPreviewJSONForLoopbackGet)$' -count=1
git diff --check
```
