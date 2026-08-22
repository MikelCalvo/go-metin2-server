# Durable Safebox Persistence Contract Freeze — 2026-08-22

## Objective

Freeze the first durable same-account safebox persistence contract before opening RED for password/load/money/item rematerialize, so in-memory open-presentation check-in/out/move no longer discard contents across reconnect / process restart / logout / Leave.

## Why docs-first

`item-storage-guard-bootstrap.md` already owns the open-presentation seam and same-session in-memory mutations, and explicitly defers password load, durable item persistence, money, and mall. Opening RED without a store/schema freeze would invent persistence shape. This plan freezes the narrow contract only.

## Contract to freeze (before RED)

1. Dedicated safebox FileStore (or named account-character safebox snapshot seam) with deterministic JSON, crash-temp + rename + fsync, manifested backup/restore hooks matching other bootstrap stores.
2. Durable rows are keyed by selected-character identity (account login + character id), not process-local entity id.
3. On successful open-presentation mutation (`SAFEBOX_CHECKIN` / `CHECKOUT` / `ITEM_MOVE`), persist safebox contents together with the already-owned inventory/quickslot account snapshot, fail-closed on write errors with live rollback.
4. Reconnect / process restart / EnterGame rematerializes remembered safebox cells into the same-session presentation table when `/open_safebox` (or authored `open_safebox` interact) opens again; closed presentation still emits no rows until open.
5. Password / `SAFEBOX_WRONG_PASSWORD` / money / `SAFEBOX_MONEY_CHANGE` / mall remain deferred unless this freeze explicitly includes a no-password bootstrap open path that matches current slash/`open_safebox` presentation.
6. Spec/QA name restart/reconnect rematerialize for safebox cells beside inventory/equipment/ground durability; do not claim password or money ownership.

## What this is not yet

- safebox password challenge / wrong-password status emission
- safebox money mutation
- mall checkout/open
- partner-side open player-shop / cube exchange busy rejects
- SQL import/backfill from quarantined exports

## TDD shape after the freeze lands

1. Catalog/store tests: round-trip durable safebox snapshot; reject malformed; deterministic JSON.
2. Runtime unit: mutation persists; persist failure rolls back live inventory + safebox.
3. Minimal session: check-in → reconnect/restart → reopen presentation → remembered `SAFEBOX_SET` rows; Leave/reclaim does not leak foreign-character rows.

## Status

Docs-only planning freeze for the next items-lane storage slice. RED intentionally deferred until this contract is accepted into protocol/QA (separate docs commit or the same docs commit before tests).
