# GC::SHOP_SIGN codec contract freeze — 2026-08-23

## Why this exists

`CG::MYSHOP` codec + deny-no-response GAME dispatch are owned, but the repository still does not own the companion `GC::SHOP_SIGN` (`0x0811`) wire shape. Any later accepted private-shop open/close presentation needs that server packet before sign text can appear around a host VID.

This freeze is **codec-only**. It does not open private shops, wire GAME emission, mutate inventory/gold, or invent exchange busy-window policy for open player shops.

## Oracle summary (behavior reference only)

- Server header: `GC::SHOP_SIGN = 0x0811`
- Packed shape after the common frame envelope: `dwVID uint32 LE`, then `szSign[SHOP_SIGN_MAX_LEN + 1]` with `SHOP_SIGN_MAX_LEN = 32`
- Open path copies the shop sign into the fixed field and `PacketAround`s the frame
- Close / clear path reuses the same header with an empty NUL-terminated sign (`szSign[0] = '\0'`) for the same VID
- Runtime open/close, guest browse, buy, and partner player-shop busy rejects remain later slices

## Frozen Go codec contract

Own in `internal/proto/shop`:

- `HeaderServerShopSign uint16 = 0x0811`
- Reuse already-owned `ShopSignMax = 32` (wire field is `ShopSignMax + 1` bytes, NUL-padded)
- `ServerShopSignPacket{ VID uint32; Sign string }`
- `EncodeServerShopSign` / `DecodeServerShopSign`

Rules:

1. Payload layout is exactly `vid uint32 LE` + `sign[33]`.
2. Payload length must equal `4 + ShopSignMax + 1` (`37`); short / long payloads fail closed.
3. Unexpected header fails closed as unexpected header.
4. Sign encode zero-pads / truncates into the 33-byte field; decode stops at the first embedded NUL (same fixed-string style as MYSHOP / shop tab names).
5. Empty sign (`""`) is a valid clear/close companion payload.

## Explicit non-goals

- no `internal/game` emission / session handler wiring
- no accepted private-shop open, close, guest browse, buy, or stock mutation
- no exchange START/ACCEPT partner player-shop busy rejects yet
- no cube busy rejects
- no claim that deny-no-response `MYSHOP` dispatch now opens a shop

## Proof shape for the implementation slice

1. Codec round-trip + exact wire bytes for non-empty sign and empty clear/close sign
2. Fail-closed decode for wrong header and truncated / oversized payload
3. Docs already frozen here; runtime stays untouched until a later presentation slice

## Status

Docs/spec contract freeze on `lane/items`. Implementation RED/GREEN for encode/decode follows as the next cohesive codec slice; private-shop runtime remains deferred.
