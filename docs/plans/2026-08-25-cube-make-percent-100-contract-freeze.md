# Cube `make` success contract freeze — 2026-08-25

## Objective

Freeze the first lab cube craft-consumption seam before RED so TMP4 clients
can run one deterministic `/cube make` against already-bound craft slots and
receive the oracle's `cube success` companion without inventing `make all`,
fail rolls, or complicated OR-materials.

## Why this exists

`/cube add` / `/cube del` → `cube info` are now owned as same-socket inventory
pointers + gold hint. The oracle's next command-chat family used by the TMP4
cube UI is craft consumption:

- client → server: `/cube make` (no args for one attempt)
- server → client (success): `CHAT_TYPE_COMMAND` `cube success <vnum> <count>`
  plus inventory / gold refresh frames from material consume + reward grant
- server → client (fail roll): info chat + `cube fail` (deferred here)

Oracle evidence (`Cube_make`):

1. cube must be open and quest/NPC remembered
2. bound cube slots must exact-match one recipe for that NPC
3. character gold must cover recipe gold
4. materials are removed from the bound inventory cells
5. gold is debited
6. one roll `number(1,100)` is compared to authored `percent`
7. success emits `cube success <rewardVnum> <rewardCount>` and AutoGiveItem
8. fail emits info chat + `cube fail` after materials/gold were already consumed

Bootstrap first slice freezes only the deterministic `percent = 100` success
path, matching the refine lane's staged probability rollout. `make all`, fail
rolls (`percent` in `0..99`), `list`, and `cancel` stay deferred.

## Contract to freeze (before RED)

1. **Authored recipe percent**
   - extend `cubestore.Recipe` with optional `percent` (`uint8`, `1..100`)
   - bootstrap NPC `20022` recipe authors `percent: 100`
   - omitted / zero / out-of-range percent stay fail-closed for make until a
     later roll slice owns them (store validation reject or runtime silent)
2. **Ingress** while selected character is in `GAME`, above zero-HP floor, and
   `hasActiveCubeOpen` with remembered `activeCubeNPCVnum`:
   - talking-chat `/cube make` with no extra args
   - `/cube make all` / extra args stay recognized fail-closed-consume for now
     (no frames / no mutation) until a later `make all` slice
3. **Preconditions (fail-closed, no mutation)**
   - cube not open / remembered vnum cleared / zero-HP / no selected character
     → silent consume (bootstrap preference; do not invent closed-window info
     chat until a later localization slice)
   - bound live cells do not exact-match one authored simple recipe for the
     open NPC → self-only `CHAT_TYPE_INFO`
     `You do not have enough materials.`
   - matched recipe gold `> 0` and live gold `< gold` → self-only
     `CHAT_TYPE_INFO` `Not enough Yang or the item is not in place.`
   - reward cannot be placed into carried inventory (full / invalid reward) →
     self-only inventory-full style reject already owned by pickup/merchant
     paths (`You have too many items.`) with no material/gold mutation
4. **Success (`percent = 100` only)**
   - consume exact material counts from currently bound live inventory cells
     (same multiset matching already owned by `cube info`)
   - clear craft-slot bindings whose cells were emptied; keep remaining
     partial/unrelated bindings only when still live
   - debit recipe gold when `gold > 0`
   - grant reward `{vnum,count}` into carried inventory through the ordinary
     placement/merge path
   - persist inventory + gold
   - emit self-only frames in this order:
     1. material refresh frames (`ITEM_UPDATE` / `ITEM_DEL`) for consumed cells
     2. material-removal `QUICKSLOT_DEL` for fully emptied item quickslot cells
        (same policy as refine/use/drop)
     3. reward `ITEM_SET` / `ITEM_UPDATE`
     4. gold `PLAYER_POINT_CHANGE` when gold changed
     5. `CHAT_TYPE_COMMAND` `cube success <rewardVnum> <rewardCount>`
   - after success, emit one follow-up `cube info <gold> 0 0` from the remaining
     bindings (usually `0 0 0`) so TMP4 need-money UI stays coherent
5. **Busy shells**
   - make does not open new shells; if an exchange/merchant shell is somehow
     still active on the same socket, success prepends the already-owned
     presentation teardown order before mutation frames (SHOP before exchange)
6. Spec/QA/roadmap/packet-matrix name this `percent = 100` make seam beside
   owned add/del; `make all`, fail rolls, `list`, `cancel`, complicated
   materials stay deferred.

## Explicit non-goals

- `/cube make all` loop
- `percent` in `0..99` injected/fail rolls + `cube fail`
- `cube list` INFO dump / `cube cancel`
- complicated OR-material matching
- durable cube-slot persistence
- quest-NPC distance gate beyond lab `/open_cube`
- binary cube packet headers
- tax / empire / shop-bag / mall

## Proof shape for the later implementation slice

1. Store: round-trip `percent`; reject `0` / `>100` fail-closed; bootstrap
   fixture authors `100`.
2. Runtime/session: `/open_cube` → add matching materials → `/cube make`
   consumes materials/gold, grants `27001,1`, emits `cube success 27001 1`,
   persists, and leaves account inventory/gold matching the mutation.
3. Negatives: closed cube silent; unmatched bindings → materials info chat;
   insufficient gold → Yang info chat; inventory full → too-many-items; no
   mutation on those rejects.
4. Docs/spec/QA update only the cube bootstrap vertical.

## Status

Docs-first freeze only on `lane/items`. RED for `/cube make` (`percent = 100`)
is intentionally deferred to the next implementation run so `main` / lane stay
green.
