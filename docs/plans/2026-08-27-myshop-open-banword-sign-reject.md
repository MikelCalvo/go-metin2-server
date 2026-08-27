# MYSHOP open banword sign reject — 2026-08-27

## Objective

Close the remaining host-only `CG::MYSHOP` open feedback gap after silk-bag
`71049` consume-skip: reject non-empty shop signs that match a bootstrap
banword list with self-only INFO chat, matching the external `OpenMyShop`
`CBanwordManager::CheckString` gate without inventing Canada-locale skip,
DB-backed banword reload, quest-running open blocks, or `MYSHOP_PRICELIST`.

## Why this exists

Accepted open still accepts any non-empty sign after armor / stock / bag
gates. The oracle rejects banword hits with INFO chat and leaves the shop
closed. Manual QA can therefore open a private shop under a vulgar / slang
sign that TMP4 clients would reject. Track C prefers this client-visible
open reject over inventing DB pricelist rematerialize, quest-running,
OR-materials, or binary cube headers.

## Contract to freeze (before RED)

1. **Bootstrap banword source**: a deterministic in-process bootstrap list
   owned by the minimal runtime (or a tiny dedicated helper), seeded with a
   small fixed set sufficient for focused tests (at least one ASCII token and
   one multi-byte-friendly token). Empty list means the gate never fires
   (oracle `CheckString` returns false when the map is empty). Do **not**
   invent DB `BANWORDS` packet reload or file-store auth in this slice.
2. **Match rule**: case-sensitive contiguous substring match over the
   accepted non-empty sign bytes (oracle walks the sign and `strncmp`s each
   banword). Embedded NUL / over-length signs stay on the already-owned
   silent empty/malformed paths.
3. **Reject feedback**: one self-only `CHAT_TYPE_INFO`
   `You can't give your shop an invalid name.`
   (`vid = 0`, empire `0`). No `SHOP_SIGN`, no open/busy flag, no bag debit,
   no inventory/quickslot/gold/persistence mutation.
4. **Gate ordering** on the accepted decode path (unchanged earlier gates):
   death-floor / no-selected / no-shared → silent;
   empty sign / zero count → silent;
   busy shells → owned busy info-chat;
   body-armor → armor chat;
   **then banword** → banword chat;
   then stock structural / cash / equipped / locked / gold;
   then silk `71049` / ordinary `50200` bag branch;
   success → owned open.
5. Spec/QA/packet-matrix/roadmap name this beside owned MYSHOP open once
   GREEN; until then this freeze is the source of truth for the next RED.
6. Do **not** invent Canada-locale skip (`LC_IsCanada`), DB banword reload,
   quest-running open block, authored `myshop_reject_message`,
   `MYSHOP_PRICELIST`, polymorph/horse/mount teardown, or bag-missing INFO.

## Locale / wording note

Oracle Korean `LC_TEXT` maps to English locale
`You can't give your shop an invalid name.` in
`share/locale/english/locale_string.txt`. This freeze uses that project-owned
English string (same pattern as armor / cash / locked / gold chats). Do not
copy oracle source comments or Korean keys into runtime code.

## Explicit non-goals

- Canada-locale banword bypass
- DB / packet banword table rematerialize
- quest-running open block (`PC::IsRunning`)
- `MYSHOP_PRICELIST` / GD price-list packets
- authored `myshop_reject_message`
- bag-missing INFO chat
- polymorph / horse / mount teardown on open
- mall / TMP4 CG `SAFEBOX_MONEY` / OR-materials / binary cube headers

## Proof shape

1. Runtime/session: non-empty sign containing a bootstrap banword → one
   banword INFO chat; no `SHOP_SIGN`; inventory / gold unchanged; no open flag.
2. Runtime/session: clean sign still opens through silk / ordinary bag paths.
3. Negative: empty / zero-count stay silent; busy / armor / cash / equipped /
   locked / gold chats still win in their owned positions; banword fires
   after armor and before stock walk.
4. Negative: empty bootstrap banword list never rejects on this gate.

## Status

Implemented on `lane/items`: host-only `CG::MYSHOP` open rejects bootstrap
banword sign hits with self-only INFO
`You can't give your shop an invalid name.` after armor and before stock walk,
with no bag debit / no `SHOP_SIGN`. Canada skip / DB banword reload /
quest-running / `MYSHOP_PRICELIST` stay deferred.
