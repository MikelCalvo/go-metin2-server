# MYSHOP client codec contract freeze — 2026-08-23

## Why this exists

Track C already projects template-authored `anti_myshop` into `ITEM_SET.anti_flags`, but the repository still does not own the client `CG::MYSHOP` (`0x0802`) wire shape. Partner player-shop busy-window exchange rejects and any accepted private-shop open path stay blocked until that codec seam exists.

This freeze is **codec-only**. It does not open private shops, emit `GC::SHOP_SIGN`, mutate inventory/gold, or invent exchange busy-window policy for open player shops.

## Oracle summary (behavior reference only)

- Client header: `CG::MYSHOP = 0x0802`
- Fixed header fields after the common frame envelope: `szSign[SHOP_SIGN_MAX_LEN + 1]` with `SHOP_SIGN_MAX_LEN = 32`, then `bCount uint8`
- Trailing blob: `bCount * sizeof(TShopItemTable)` under `#pragma pack(1)`
- Packed `TShopItemTable`: `vnum uint32`, `count uint8`, packed `TItemPos` (`window_type uint8`, `cell uint16 LE`), `price uint32`, `display_pos uint8` → **13 bytes** per entry
- Client send path builds `length = sizeof(TPacketCGMyShop) + sizeof(TShopItemTable) * stock.size()` then streams the trailing tables
- Server ingress waits for the full fixed + trailing blob before calling `OpenMyShop`
- Runtime open/close, armor/quest/banword/`anti_give|anti_myshop` rejects, gold-cap gates, and `GC::SHOP_SIGN` remain later slices

## Frozen Go codec contract

Own in `internal/proto/shop`:

- `HeaderClientMyShop uint16 = 0x0802`
- `ShopSignMax = 32` (wire field is `ShopSignMax + 1` bytes, NUL-padded)
- `MyShopItemTableSize = 13`
- `ClientMyShopPacket{ Sign string; Items []ClientMyShopItem }`
- `ClientMyShopItem{ Vnum uint32; Count uint8; Position itemproto.Position; Price uint32; DisplayPos uint8 }`
- `EncodeClientMyShop` / `DecodeClientMyShop`

Rules:

1. Payload layout is exactly `sign[33]` + `count uint8` + `count` packed item tables.
2. `count` must be `<= ShopHostItemMax` (`40`); oversized counts fail closed as invalid payload.
3. Payload length must equal `34 + count*13`; short / long / truncated trailing blobs fail closed.
4. Unexpected header fails closed as unexpected header.
5. Sign encode zero-pads / truncates into the 33-byte field; decode stops at the first embedded NUL (same fixed-string style as shop tab names).
6. Item `Position` reuses the owned packed `TItemPos` layout (`window_type uint8`, `cell uint16 LE`).

## Explicit non-goals

- no `internal/game` dispatch / session handler
- no accepted private-shop open, close, guest browse, buy, or stock mutation
- no `GC::SHOP_SIGN` (`0x0811`) codec in this freeze
- no exchange START/ACCEPT partner player-shop busy rejects yet
- no cube busy rejects
- no claim that `anti_myshop` now has a live private-shop mutation path

## Proof shape for the implementation slice

1. Codec round-trip + exact wire bytes for empty trailing (`count = 0`) and one/multi-item cases
2. Fail-closed decode for wrong header, truncated trailing, length/count mismatch, and `count > ShopHostItemMax`
3. Docs already frozen here; runtime stays untouched until a later presentation slice

## Status

Implemented on `lane/items`: `internal/proto/shop` owns `HeaderClientMyShop` / `EncodeClientMyShop` / `DecodeClientMyShop` with the frozen sign/count + packed item-table layout and fail-closed decode guards. Private-shop runtime open/close/browse/buy, `GC::SHOP_SIGN`, and partner player-shop/cube exchange busy rejects stay deferred.
