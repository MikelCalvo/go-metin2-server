# Open-Safebox NPC Password-Challenge Docs Sync — 2026-08-23

## Objective

Align content-lane NPC `open_safebox` protocol / plan docs with the already-landed
warehouse password-challenge contract so authored QA content and protocol
language stop claiming immediate `SAFEBOX_SIZE` open or deferred password /
durable rematerialize.

## Why now

- Runtime + focused tests already own:
  - warehouse `INTERACT` → optional authored info chat + `CHAT_TYPE_COMMAND`
    `ShowMeSafeboxPassword` (no busy / no `SAFEBOX_SIZE` yet)
  - `/safebox_password` match → open presentation + durable rematerialize
  - slash `/open_safebox` remains the no-password lab harness
- Manual QA (`docs/qa/manual-client-checklist.md`) already describes that path.
- `spec/protocol/npc-service-interactions-bootstrap.md` and older open-safebox
  plans still say INTERACT reuses `/open_safebox` immediately and that durable
  storage / password load stay deferred.

This is a content-lane honesty fix for the owned NPC service surface, not a new
runtime behavior.

## Contract frozen / restated by this slice

1. Authored `open_safebox` `INTERACT` success remembers a same-socket pending
   password challenge with the authored effective size, optionally emits
   authored info chat, then emits self-only `ShowMeSafeboxPassword`.
2. Pending challenge does **not** set exchange busy or emit `SAFEBOX_SIZE` /
   `SAFEBOX_SET` / `SAFEBOX_MONEY_CHANGE`.
3. Matching `/safebox_password` opens the presentation and rematerializes durable
   same-account cells / money through the already-owned open seam.
4. Slash `/open_safebox [1..3]` remains the exempt no-password lab/debug opener.
5. Merchant auto-close on warehouse `INTERACT` still prepends `GC::SHOP END`
   before the warehouse chat / password-prompt frames.
6. Mall / client `SAFEBOX_CHANGE_PASSWORD` packets / TMP4 CG `SAFEBOX_MONEY`
   request header remain deferred.

## Files

- `spec/protocol/npc-service-interactions-bootstrap.md`
- `docs/plans/2026-08-21-open-safebox-npc-service.md`
- `docs/plans/2026-08-21-open-safebox-interact-merchant-auto-close.md`
- `docs/plans/2026-08-21-pve-vertical-authoring-warehouse.md`
- related hermetic/content plan follow-ups that still defer password/durable
  warehouse load

## Explicit non-goals

- no runtime / packet / store code changes
- no new NPC service kinds
- no branching quest scripts
- no README churn beyond this plan pointer if needed

## Validation

```bash
git diff --check
# docs/spec only; no gofmt / go test required unless a later RED needs them
```

## Follow-up options

1. Keep branching quest scripts deferred.
2. Keep pack AI / synchronized respawn deferred.
3. Keep mall / client `SAFEBOX_CHANGE_PASSWORD` packets / TMP4 CG `SAFEBOX_MONEY`
   deferred on the items lane.
