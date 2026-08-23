# MYSHOP deny-no-response GAME dispatch contract freeze — 2026-08-23

## Why this exists

`CG::MYSHOP` (`0x0802`) encode/decode is now owned in `internal/proto/shop`, but the GAME flow still treats the header as `ErrUnexpectedClientPacket`. A live TMP4 client that sends private-shop open would therefore hit the unexpected-packet disconnect edge before any later accepted open path can exist.

This freeze is **dispatch-only**. It does not open private shops, emit `GC::SHOP_SIGN`, mutate inventory/gold, or invent exchange busy-window policy for open player shops.

## Oracle summary (behavior reference only)

- Client ingress waits for the full fixed + trailing `TShopItemTable` blob, then calls `OpenMyShop`
- Until a presentation seam exists, clean-room policy is the same deny-no-response pattern already owned for mall checkout / early refine / storage packets: decode in GAME, call a handler hook, and when the default/injected handler returns `Accepted: false`, emit no frames and keep the phase in `GAME`
- Malformed payloads fail closed at the codec/dispatcher boundary (`ErrInvalidPayload` / unexpected header) rather than disconnecting as an unknown family after decode ownership exists

## Contract to freeze (before RED)

Own in `internal/game`:

1. `HandleMyShopFunc` / `Config.HandleMyShop` / default handler that returns `ShopResult{Accepted: false}` (reuse `ShopResult`, same as other shop hooks).
2. `HandleClientFrame` routes `shopproto.HeaderClientMyShop` through `shopproto.DecodeClientMyShop` then `handleMyShop`.
3. Accepted handler frames are returned unchanged (for harness injection only in this slice).
4. Denied / default path returns `nil` frames, `nil` error, and leaves the session in `GAME` — no disconnect, no inventory/gold/shop mutation, no `GC::SHOP_SIGN`.
5. Malformed MYSHOP payloads still return the codec error (`ErrInvalidPayload` / `ErrUnexpectedHeader`) and do not call the handler.
6. Spec/QA/packet-matrix name this as deny-no-response GAME dispatch beside the owned codec; accepted private-shop open / `GC::SHOP_SIGN` / partner player-shop/cube exchange busy rejects stay deferred.

## Explicit non-goals

- no accepted private-shop open, close, guest browse, buy, or stock mutation
- no `GC::SHOP_SIGN` (`0x0811`) codec or emission
- no exchange START/ACCEPT partner player-shop busy rejects yet
- no cube busy rejects
- no claim that `anti_myshop` now has a live private-shop mutation path
- no minimal/session runtime wiring beyond the game-flow hook default deny

## Proof shape for the implementation slice

1. Flow unit: injected accepted handler receives the decoded packet and returns its frames
2. Flow unit: default / denied handler emits no frames, no error, phase stays `GAME`
3. Flow unit: malformed MYSHOP payload fails closed with codec error and does not invoke the handler
4. Docs already frozen here; private-shop runtime remains untouched

## Status

Implemented on `lane/items`: `internal/game` owns `HandleMyShop` deny-no-response GAME dispatch for the owned `CG::MYSHOP` codec (default `Accepted:false`, no frames / no disconnect; malformed payloads fail closed at decode). Private-shop runtime open/close/browse/buy, `GC::SHOP_SIGN`, and partner player-shop/cube exchange busy rejects stay deferred.
