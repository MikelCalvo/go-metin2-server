# MYSHOP open authored myshop_reject_message — 2026-08-27

## Objective

Close the remaining host-only `CG::MYSHOP` open cash-item feedback gap after
banword sign reject: let item templates author `myshop_reject_message` so
`anti_give|anti_myshop` open-stock rejects can emit template-owned INFO text
instead of only the fixed English
`Cash items cannot be sold in a private shop.`, matching the already-owned
`give_reject_message` / `sell_reject_message` / `safebox_reject_message`
pattern without inventing quest-running open blocks, `MYSHOP_PRICELIST`,
Canada banword bypass, or bag-missing INFO.

## Why this exists

Cash-item open stock already emits fixed English INFO. Manual QA and content
authors cannot override that string per vnum the way merchant sell/buy and
exchange give already can. Track C prefers this template-backed vertical over
inventing DB pricelist rematerialize, quest-running PC seams, OR-materials, or
binary cube headers. The store/runtime boundary is already explicit enough for
a later RED once this freeze lands.

## Contract to freeze (before RED)

1. **Store field**: optional `myshop_reject_message` string on item templates
   (`MyShopRejectText` in Go). Round-trip through the file-backed itemstore with
   deterministic JSON. Empty/omitted keeps the already-owned fixed English
   cash-item chat.
2. **Store validation (fail-closed)**:
   - reject embedded NUL / invalid UTF-8 the same way other reject messages do
   - non-empty `myshop_reject_message` requires at least one owned MYSHOP open
     cash-item guard: `anti_myshop` and/or `anti_give`
   - do **not** accept the field on templates that lack those guards
3. **Runtime open feedback**: on the already-owned host-only `CG::MYSHOP` open
   stock walk, when a listed cell resolves `AntiGive || AntiMyShop`:
   - if the template authors non-empty `myshop_reject_message`, emit one
     self-only `CHAT_TYPE_INFO` with that trimmed text (`vid = 0`, empire `0`)
   - otherwise keep the fixed English
     `Cash items cannot be sold in a private shop.`
   - first offending row still wins; no `SHOP_SIGN`; no open/busy flag; no bag
     debit; no inventory/quickslot/gold/persistence mutation
4. **Gate ordering** stays unchanged: death-floor / empty / busy / armor /
   banword → then stock walk (structural silent → cash-item chat with authored
   override when present → equipped → locked → gold) → silk/ordinary bag →
   success.
5. Spec/QA/packet-matrix/roadmap name this beside owned MYSHOP open once GREEN;
   until then this freeze is the source of truth for the next RED.
6. Do **not** invent quest-running open block, `MYSHOP_PRICELIST`, Canada
   banword bypass, bag-missing INFO, polymorph/horse/mount teardown, or
   authored overrides for armor / banword / equipped / locked / gold chats.

## Explicit non-goals

- quest-running open block (`PC::IsRunning`)
- `MYSHOP_PRICELIST` / GD price-list packets
- Canada-locale banword bypass / DB banword reload
- bag-missing INFO chat
- authored overrides for armor / banword / equipped / locked / gold-overflow
- removing listed carried stock into shop ownership on open
- mall / TMP4 CG `SAFEBOX_MONEY` / OR-materials / binary cube headers

## Proof shape

1. Catalog/store: round-trip `myshop_reject_message` with `anti_myshop` and/or
   `anti_give`; reject NUL / missing-guard / contradictory metadata fail-closed;
   keep persisted JSON deterministic.
2. Runtime/session: `anti_myshop`/`anti_give` stock with authored text → one
   INFO chat with that text; no `SHOP_SIGN`; inventory/gold unchanged.
3. Runtime/session: same guards without authored text → fixed English cash-item
   chat (regression).
4. Negative: armor / banword / equipped / locked / gold chats stay on their
   fixed strings; omitted field does not invent a new silent path.

## Status

Implemented on `lane/items`: optional template-authored `myshop_reject_message`
round-trips through the file-backed itemstore (requires `anti_myshop` and/or
`anti_give`), and host-only `CG::MYSHOP` open cash-item stock emits that text
when present, otherwise the fixed English cash-item chat. Quest-running /
`MYSHOP_PRICELIST` / OR-materials stay deferred.
