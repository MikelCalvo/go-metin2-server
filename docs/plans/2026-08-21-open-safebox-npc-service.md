# Open-Safebox NPC Service Kind — 2026-08-21

## Objective

Add the first authored `open_safebox` static-actor interaction kind so a visible warehouse NPC can open the already-owned same-session safebox presentation through ordinary `INTERACT`, instead of requiring the local `/open_safebox` slash harness.

## Why now

- Track D deferred richer NPC service kinds until client-visible safebox behavior existed.
- Main now owns in-memory `SAFEBOX_CHECKIN` / `SAFEBOX_CHECKOUT` while the bootstrap safebox presentation is open.
- Manual QA still depends on slash `/open_safebox` to reach that mutation path; there is no content/NPC surface that opens storage.
- This keeps the PvE vertical content-authored: quest unlock → warehouse NPC → check-in/out.

## Contract frozen by this slice

1. New `interactionstore` kind `open_safebox` beside `info` / `talk` / `warp` / `shop_preview` / `quest_flag`.
2. Definition shape:
   - required `kind` + `ref`
   - optional `text` (self-only `CHAT_TYPE_INFO` before open; UTF-8 / NUL-free, same rule as warp)
   - optional `size` in `1..3`; omitted / `0` defaults to `1` (same bootstrap page count as `/open_safebox`)
   - optional non-mutating quest gate (`quest_ref` + `quest_flag` + optional `quest_from`; `quest_to` must stay `0`)
   - reject `title` / `catalog` / warp coords / quest turn-in reward-consume fields
3. Runtime on `INTERACT`:
   - reuse visibility, range, cooldown, and zero-HP fail-closed rules
   - gate mismatch → existing `Quest requirements are not met.` with no open
   - original GREEN: success → optional authored text, then the same presentation seam as `/open_safebox`: set busy flag, emit `SAFEBOX_SIZE`, re-emit remembered in-memory `SAFEBOX_SET` rows
   - **later owned supersession** (`docs/plans/2026-08-23-safebox-password-challenge.md` + docs sync `docs/plans/2026-08-23-open-safebox-npc-password-challenge-docs-sync.md`): success now remembers a pending password challenge and emits `ShowMeSafeboxPassword` instead of opening immediately; matching `/safebox_password` opens with durable rematerialize while slash `/open_safebox` remains the no-password lab harness
   - already-open refresh stays idempotent like slash reopen once the presentation is open
4. Content-bundle / loopback `/local/interactions` round-trip the full definition and reject invalid shapes through `ValidDefinition`.
5. Gated `open_safebox` definitions require an in-bundle writer for the gate flag, matching other gated services.
6. QA fixture adds one gated warehouse NPC to `docs/examples/bootstrap-npc-service-bundle.json`.

## What this is not yet

- ~~password / DB load (`ReqSafeboxLoad`) and `SAFEBOX_WRONG_PASSWORD`~~ Done later: warehouse password challenge + durable optional password.
- ~~durable safebox persistence / account-store / DB schema~~ Done later for FileStore rematerialize; SQL import/backfill remains deferred.
- mall open/checkout, client `SAFEBOX_CHANGE_PASSWORD` packets, TMP4 CG `SAFEBOX_MONEY` request header
- broad new content-bundle summary / import-preview endpoint sprawl beyond ordinary interaction previews
- branching quest scripts / dialog trees

## TDD and validation

Focused coverage:

```bash
go test ./internal/interactionstore -run 'OpenSafebox|QuestGated' -count=1
go test ./internal/contentbundle -run 'NpcServiceBundle|OpenSafebox|QuestGate' -count=1
go test ./internal/minimal -run 'OpenSafebox|InteractionVisibility.*OpenSafebox|SafeboxCheckin|SafeboxCheckout' -count=1
go test ./internal/ops -run 'Interaction.*OpenSafebox|OpenSafebox' -count=1
gofmt -w <touched Go files>
git diff --check
```

## Follow-up options

1. ~~Keep durable safebox persistence / password load deferred.~~ Done later; see `docs/plans/2026-08-23-safebox-password-challenge.md` and `docs/plans/2026-08-23-open-safebox-npc-password-challenge-docs-sync.md`.
2. ~~Optional light content-bundle `open_safebox` route summary later if QA needs map-local warehouse audit without full interactable previews.~~ Done: see [open-safebox content-bundle route summary](2026-08-21-open-safebox-content-bundle-route-summary.md).
3. Keep branching quest scripts deferred.
