# Packet matrix

This is the working packet inventory for the first protocol milestone.
It is intentionally narrow and only covers the boot path plus the first in-world movement step.

Status values:
- `documented` — the packet is part of the current written contract
- `planned` — known to be needed soon, but not yet locked by tests
- `candidate` — observed or expected, but still awaiting final confirmation in our own suite

Planned rows may temporarily use `Header = TBD` when the project freezes the family name and scope before the exact wire header/layout reaches packet tests.

## Control plane

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `PONG` | client -> server | `0x0006` | handshake/game control | documented | accepted as a header-only reply to server `PING`; phase-stable in `HANDSHAKE` and `GAME` |
| `PING` | server -> client | `0x0007` | handshake/game control | documented | carries `server_time`; current slice freezes the codec and the matching `PONG` reply path |
| `PHASE` | server -> client | `0x0008` | control | documented | phase transition control packet; on the main game socket the observed client bootstrap expects `PHASE(HANDSHAKE)` before the first `KEY_CHALLENGE` |
| `KEY_RESPONSE` | client -> server | `0x000A` | handshake | documented | cryptographic response path |
| `KEY_CHALLENGE` | server -> client | `0x000B` | handshake | documented | challenge + server key material; authd may send it immediately after connect, while the main game socket should follow the observed `PHASE(HANDSHAKE)` prelude |
| `KEY_COMPLETE` | server -> client | `0x000C` | handshake | documented | completes key exchange |
| `CLIENT_VERSION` | client -> server | `0x000D` | loading | documented | accepted as metadata in `LOADING`; no server response and no phase transition |
| `STATE_CHECKER` | client -> server | `0x000F` | control probe | documented | separate channel-status probe socket; header-only request outside the normal login/game choreography |
| `RESPOND_CHANNELSTATUS` | server -> client | `0x0010` | control probe | documented | channel-status response consumed by the dedicated state-checker client path |

## Authentication and selection surface

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `LOGIN2` | client -> server | `0x0101` | login | documented | minimal login-by-key request frozen by tests |
| `LOGIN3` | client -> server | `0x0102` | auth | documented | minimal auth-server credential request frozen by tests |
| `LOGIN_SECURE` | client -> server | `0x0103` | login | candidate | not part of the first implementation unless required |
| `LOGIN_SUCCESS3` | server -> client | `0x0104` | login/select | candidate | possible older character-list success shape |
| `LOGIN_SUCCESS4` | server -> client | `0x0105` | login/select | documented | minimal selection-surface success path frozen by tests |
| `LOGIN_FAILURE` | server -> client | `0x0106` | auth/login | documented | negative auth/login path frozen by tests |
| `LOGIN_KEY` | server -> client | `0x0107` | login | candidate | not part of the current minimal login-by-key happy path |
| `AUTH_SUCCESS` | server -> client | `0x0108` | auth | documented | minimal authd success path with issued login key frozen by tests |
| `EMPIRE` | server -> client | `0x0109` | login/select | documented | selection surface empire state frozen by tests |
| `EMPIRE` | client -> server | `0x010A` | select | documented | minimal empire selection request for empty-account bootstrap flow |

## Character lifecycle

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `CHARACTER_CREATE` | client -> server | `0x0201` | select | documented | create character request |
| `CHARACTER_DELETE` | client -> server | `0x0202` | select | documented | delete character request with private-code field |
| `CHARACTER_SELECT` | client -> server | `0x0203` | select | documented | choose active character |
| `ENTERGAME` | client -> server | `0x0204` | loading | documented | accepted request; current exact `LOADING -> GAME` burst is frozen by `loading-to-game-bootstrap-burst.md` |
| `CHARACTER_ADD` | server -> client | `0x0205` | game bootstrap/visibility | documented | used for the selected-character bootstrap, for peer visibility, and now also for the first bootstrap static-actor enter-game inserts |
| `CHAR_ADDITIONAL_INFO` | server -> client | `0x0207` | game bootstrap/visibility | documented | metadata companion for visible player inserts and the current bootstrap static-actor enter-game burst; player payloads now also carry the first bootstrap equipment-appearance projection for equipped `body` / `weapon` / `head` items |
| `CHARACTER_DEL` | server -> client | `0x0208` | game visibility | documented | removes a visible peer by `vid` when that actor leaves the local world |
| `CHARACTER_UPDATE` | server -> client | `0x0209` | game bootstrap/visibility | documented | first self-only state refresh after the visible-world insert, the current self-only equip/unequip appearance refresh payload, the current queued peer-visible equip/unequip appearance refresh payload for already-visible stable peers, the current peer-visibility refresh payload, and the current bootstrap static-actor enter-game refresh payload; player payloads now also reuse the first bootstrap equipment-appearance projection for equipped `body` / `weapon` / `head` items |
| `PLAYER_CREATE_SUCCESS` | server -> client | `0x020C` | select | documented | create success result |
| `PLAYER_CREATE_FAILURE` | server -> client | `0x020D` | select | documented | create failure result |
| `PLAYER_DELETE_SUCCESS` | server -> client | `0x020E` | select | documented | delete success result carrying the cleared account slot |
| `PLAYER_DELETE_FAILURE` | server -> client | `0x020F` | select | documented | minimal delete rejection placeholder using a header-only failure frame |
| `MAIN_CHARACTER` | server -> client | `0x0210` | loading | documented | main actor bootstrap |
| `PLAYER_POINTS` | server -> client | `0x0214` | loading/game bootstrap | documented | initial stat payload |
| `PLAYER_POINT_CHANGE` | server -> client | `0x0215` | loading/game bootstrap / game | documented | first self-only point refresh after the selected-character bootstrap; the first owned consumable-use vertical also reuses it for the selected character using template-authored `use_effect` metadata (`type`, `amount`, and point index), while the current seeded bootstrap consumable still resolves to `type = 1`, `amount = 50`, and `value = updated Points[1]`; the first narrow template-backed equip/unequip point slice also reuses it self-only when the matched equipped item carries `equip_effect`, with the current seeded practice blade resolving to `vnum = 12200`, `type = 1`, and `amount = +/-10` on equip/unequip; engaged content-loaded `spawn_groups` practice mobs now also reuse it for both the immediate self-only owner retaliation piggyback on accepted live hits and one delayed self-only server-origin follow-up beat after the first accepted live owner hit, with later autonomous beats continuing every owned `1s` only while the same engagement remains live and failing closed if target intent clears/replaces or the mob dies / rebuilds before the pending beat fires |
| `DEAD` | server -> client | `0x0217` | game | documented | first owned visible death signal; payload is little-endian `uint32 vid`; the current bootstrap death/respawn contract uses it when a visible `training_dummy` reaches runtime HP `0`, while the current practice-mob retaliation slice also reuses it for an engaged owner's live HP floor at `0` with self `DEAD + TARGET(0, 0)` plus one visibility-gated peer `DEAD(owner_vid)` fanout; broader player death / respawn choreography remains out of scope |

## Movement

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `MOVE` | client -> server | `0x0301` | game | documented | first in-world action; an exact-position bootstrap transfer trigger may consume the packet without the normal same-map reply path |
| `MOVE` | server -> client | `0x0302` | game | documented | deterministic self ack for the normal same-map mover path and current queued replication payload for already-visible peers on the same bootstrap `MapIndex`; the current transfer-trigger path intentionally suppresses the immediate self `MOVE_ACK` and returns the transfer rebootstrap burst documented in `transfer-rebootstrap-burst.md` instead |
| `SYNC_POSITION` | client -> server | `0x0303` | game | documented | first selected-character position reconciliation path in `GAME`; an exact-position bootstrap transfer trigger may consume the packet without the normal same-map reply path |
| `SYNC_POSITION` | server -> client | `0x0304` | game | documented | deterministic selected-character sync reply for the normal same-map sender path and current queued replication payload for already-visible peers on the same bootstrap `MapIndex`; the current transfer-trigger path intentionally suppresses the immediate self `SYNC_POSITION_ACK` and returns the transfer rebootstrap burst documented in `transfer-rebootstrap-burst.md` instead |
| `WARP` | client -> server | `0x0305` | game | planned | out of early scope; the bootstrap runtime currently reuses `MOVE` / `SYNC_POSITION` exact-position triggers instead of owning a dedicated warp request |
| `WARP` | server -> client | `0x0306` | game | planned | out of early scope; the bootstrap runtime does not yet freeze a final self-visible warp/loading packet and currently uses the self-session transfer contract documented in `transfer-rebootstrap-burst.md` |

## Chat

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `CHAT` | client -> server | `0x0601` | game | documented | current slice accepts `CHAT_TYPE_TALKING`, bootstrap `CHAT_TYPE_PARTY`, bootstrap `CHAT_TYPE_GUILD`, bootstrap `CHAT_TYPE_SHOUT`, and bootstrap `CHAT_TYPE_INFO`; once the later player-death retaliation floor has already reached `0` HP for the selected owner, peer-facing `CHAT` types plus self-only `CHAT_TYPE_INFO` fail closed instead; client-originated `CHAT_TYPE_NOTICE` is currently rejected; bootstrap `CHAT_TYPE_TALKING` currently also hosts the server-owned slash seams `/restart_here`, `/restart_town`, `/inventory_move`, `/equip_item`, `/unequip_item`, `/use_item <slot>`, and the temporary local-only merchant debug harness `/shop_buy <catalog_slot>` until richer dedicated gameplay families replace those temporary harness paths |
| `WHISPER` | client -> server | `0x0602` | game | documented | current slice routes by exact target character name with variable text payload |
| `CHAT` | server -> client | `0x0603` | game | documented | deterministic sender echo for talking/party/guild/shout; current fanout scope is same-map + same-empire for talking, same empire for shout, same non-zero `GuildID` for guild, and all connected sessions for party; sender-only bootstrap system info uses `vid = 0`; server-originated notice broadcasts also use `vid = 0` raw system text |
| `WHISPER` | server -> client | `0x0604` | game | documented | direct whisper delivery to the named target on success and current `WHISPER_TYPE_NOT_EXIST` sender feedback for unknown targets |

## Interaction

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `INTERACT` | client -> server | `0x0501` | game | documented | first owned client-originated interaction request for a visible bootstrap static actor target by `vid`; current payload is a little-endian `uint32 target_vid`, current routing stops at the dedicated `GAME`-phase interaction handler, current owned responses include self-only `info` / `talk` / `shop_preview`, and the current service-style NPC families also include transfer-backed `warp` |

## Merchants / shops

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `SHOP` | client -> server | `0x0801` | game | documented | first owned merchant-window client family freezes subheaders `BUY` and `END` on top of a merchant session opened from the existing `INTERACT` ingress; `BUY` remains valid only while an active merchant context exists, the second trailing byte after the common `SHOP` envelope is the zero-based authored `catalog_slot`, `END` now also fails closed after the current practice-mob retaliation floor has already cleared that merchant context on owner death, and `SELL(slot)` / `SELL2(slot,count)` now route to the active merchant sell-back runtime path for carried-slot stack removal/decrement with invalid/equipped/anti-sell/stale cases failing closed |
| `SHOP` | server -> client | `0x0810` | game | documented | first owned merchant-window server family freezes `START` as the open response and `END` as the explicit close companion while the socket can still deliver merchant frames; the same `END` family now also tears down an already-open merchant window after the current practice-mob retaliation floor reaches the owner at `0` HP, prepends the self transfer rebootstrap burst when a successful warp or exact-position transfer closes that same still-live merchant window, prepends the select-phase transition when a same-socket `/phase_select` closes that merchant window, and auto-closes stale packet-buy windows whose live actor/context or authored catalog changed underneath them; packet `SHOP BUY` now owns self-only `ITEM_SET` refreshes as the complete success companion without an extra bare `OK`, plus bare no-payload `NOT_ENOUGH_MONEY`, `INVENTORY_FULL`, and `INVALID_POS` error companions for insufficient-gold, no-valid-placement, and unknown-slot failures; packet `SHOP SELL` / `SELL2` success now emits carried-slot `ITEM_DEL` for whole-stack removal or `ITEM_UPDATE` for partial-stack count refresh, then self-only `PLAYER_POINT_CHANGE(POINT_GOLD)` without appending an extra bare `OK`, while invalid slots plus the first template-backed `anti_sell` and runtime-locked rejections reuse `INVALID_POS`; the reusable `UPDATE_ITEM` shop-slot refresh, `UPDATE_PRICE` signed-Elk refresh, bare no-payload `SOLDOUT`, `SOLD_OUT`, and `NOT_ENOUGH_MONEY_EX` result codecs, and `START_EX` multi-tab extended-shop open codec are now documented for later stock/player-shop/extended-shop slices |

## Items, inventory, and equipment

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `ITEM_USE` | client -> server | `0x0502` | game | documented | first owned client-originated item-use request; payload is packed `TItemPos(window:uint8, cell:uint16)`; current live runtime accepts only carried inventory positions (`window = INVENTORY`, `cell < 90`), fails closed for locked carried-slot items, and routes accepted requests through the same bootstrap template-backed consumable path already exercised by `/use_item <slot>` |
| `ITEM_MOVE` | client -> server | `0x0504` | game | documented | first owned client-originated carried-slot move/split/merge request; payload is source packed `TItemPos`, destination packed `TItemPos`, and `count uint8`; current live runtime accepts only carried inventory positions (`window = INVENTORY`, `cell < 90`), treats `count = 0` as a full-stack move that succeeds into an empty destination, swaps with an incompatible occupied destination, or merges into a compatible occupied destination up to the source template `max_count`, accepts empty-destination partial splits and template-bounded compatible occupied-destination merges for non-zero counts, treats a non-zero count that covers the whole source stack as the same full-stack carried move/swap path for incompatible occupied destinations, fails closed for partial incompatible occupied-destination counted moves, oversized non-zero counts, template-`max_count` overflow, storage-overflowing destination stack counts, and locked source/destination slots until richer stack and item-lock semantics are owned, reuses the selected-character inventory persistence boundary, keeps identity-changing moves/splits/swaps on self-only `ITEM_DEL` / `ITEM_SET`, uses self-only `ITEM_UPDATE` count refreshes for compatible occupied-destination partial merges, and emits `ITEM_DEL(source)` then `ITEM_UPDATE(destination)` for exact counted or zero-count full-stack compatible merges |
| `ITEM_DROP` | client -> server | `0x0502` | game | documented | drop request with packed `TItemPos` plus `elk uint32`; `internal/game` routes the shared `ITEM_USE` / `ITEM_DROP` header by payload length, and the shipped runtime now accepts carried-slot drops as self-only inventory removal plus `ITEM_GROUND_ADD` |
| `ITEM_DROP2` | client -> server | `0x0503` | game | documented | counted drop request with packed `TItemPos` plus `gold uint32` and `count uint8`; the shipped runtime now accepts non-zero carried-slot counts that fit the source stack as self-only inventory decrement/removal plus `ITEM_GROUND_ADD` |
| `ITEM_PICKUP` | client -> server | `0x0505` | game | documented | pickup request with `vid uint32`; `internal/game` dispatch is frozen, and the shipped runtime now accepts visible-world pickup of temporary ground handles created by accepted carried-slot drops |
| `ITEM_SET` | server -> client | `0x0511` | game bootstrap / game | documented | first owned self-only occupied-slot bootstrap/update family for carried inventory and equipped items; total frame length `54`; position is packed `TItemPos(window:uint8, cell:uint16)` and equipped items currently ride the legacy combined inventory/equipment cell namespace (`window = INVENTORY`, `cell = 90 + wear_index`); the first consumable-use vertical also reuses it when consuming one stacked item leaves a non-zero count in the same carried slot |
| `ITEM_DEL` | server -> client | `0x0510` | game | documented | self-only slot-clear/remove companion for carried/equipped item mutations; total frame length `7` and payload is only packed `TItemPos`; the first consumable-use vertical also reuses it when consuming the last item removes that carried-slot stack entirely |
| `ITEM_UPDATE` | server -> client | `0x0514` | game | documented | count/socket/attribute refresh for an already-known item cell; total frame length `41`; payload is packed `TItemPos` + `count` + three sockets + seven attributes, with no `vnum`, flags, anti-flags, or highlight; the current merchant `SHOP SELL2` partial-stack success path and counted `ITEM_DROP2` success path emit it for the reduced carried cell |
| `ITEM_GROUND_ADD` | server -> client | `0x0515` | game | documented | first frozen ground-item spawn codec; payload is `vid uint32`, `vnum uint32`, and signed `x/y/z int32` coordinates; current runtime use is after accepted carried-slot drops, visible-peer drop fanout, and radius-AOI move/sync re-entry rebuilds for pending bootstrap handles |
| `ITEM_GROUND_DEL` | server -> client | `0x0516` | game | documented | first frozen ground-item remove codec; payload is `vid uint32`; current runtime use is accepted pickup fanout plus radius-AOI move/sync exit teardown for pending bootstrap handles |
| `QUICKSLOT_ADD` | client -> server | `0x0509` | game | documented | first owned client quickslot add codec and `GAME` dispatch seam; payload is `pos uint8` plus two-byte quickslot tuple `{type uint8, pos uint8}`; minimal runtime accepts valid selected-character edits, persists the quickslot snapshot, and returns self-only `GC::QUICKSLOT_ADD` |
| `QUICKSLOT_DEL` | client -> server | `0x050A` | game | documented | first owned client quickslot clear codec and `GAME` dispatch seam; payload is `pos uint8`; minimal runtime accepts valid selected-character deletes, persists the quickslot snapshot, and returns self-only `GC::QUICKSLOT_DEL` |
| `QUICKSLOT_SWAP` | client -> server | `0x050B` | game | documented | first owned client quickslot swap codec and `GAME` dispatch seam; payload is `pos uint8` + `pos_to uint8`; minimal runtime accepts valid selected-character swaps, persists the quickslot snapshot, and returns self-only `GC::QUICKSLOT_SWAP` |
| `QUICKSLOT_ADD` | server -> client | `0x0519` | game/bootstrap | documented | first owned server quickslot refresh codec; payload is `pos uint8` plus two-byte quickslot tuple `{type uint8, pos uint8}`; selected-character `ENTERGAME` bootstrap replays persisted quickslots as sorted self-only `QUICKSLOT_ADD` frames after the selected-character presence/state burst, and runtime `CG::QUICKSLOT_ADD` echoes the accepted self-only refresh |
| `QUICKSLOT_DEL` | server -> client | `0x051A` | game/bootstrap | documented | first owned server quickslot clear codec; payload is `pos uint8`; runtime `CG::QUICKSLOT_DEL` echoes the accepted self-only clear refresh; future item-mutation slices can reuse it when a tracked item quickslot is removed |
| `QUICKSLOT_SWAP` | server -> client | `0x051B` | game/bootstrap | documented | first owned server quickslot swap codec; payload is `pos uint8` + `pos_to uint8`; runtime `CG::QUICKSLOT_SWAP` echoes the accepted self-only swap refresh |

## Combat (bootstrap)

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `TARGET` | client -> server | `0x0A01` | game | documented | first owned combat-preparation request for selecting one currently visible `training_dummy` actor by visible `vid`; payload is little-endian `uint32 target_vid`; the current live slice routes it through the shared-world `AttemptStaticActorCombatTarget(...)` seam, accepts only visible in-range `training_dummy` targets, and fails closed for malformed payloads, out-of-range targets, invisible actors, or visible non-targetable actors |
| `TARGET` | server -> client | `0x0A10` | game | documented | minimal self-only target-state acknowledgement; payload is little-endian `uint32 target_vid` + `uint8 hp_percent`; the current bootstrap dummy path returns `hp_percent = 100` on fresh accepted selection, reuses the same family for accepted attack-driven HP refreshes in `10`-point steps (`100`, `90`, `80`, ...), and freezes `target_vid = 0`, `hp_percent = 0` as the first clear-target companion for visibility/range invalidation, reconnect cleanup, dummy death, and the first retaliation-owned player death floor |
| `ATTACK` | client -> server | `0x0401` | game | documented | first owned attack-intent family after accepted target selection; exact payload is `uint8 attack_type`, little-endian `uint32 target_vid`, `uint8 crc_proc_piece`, `uint8 crc_file_piece`; the current live bootstrap path accepts only `attack_type = 0`, requires the request `target_vid` to match the session's active selected target exactly, applies one fixed same-target `250ms` normal-attack cadence window between accepted hits on the same live selected target snapshot, decrements runtime-owned `training_dummy` HP deterministically from `10` in `1`-HP steps, reuses self-only `TARGET(target_vid, hp_percent)` for non-lethal hit refreshes, and transitions the zero-HP edge to `DEAD(vid)` plus target clear instead of a final `TARGET(..., 0)` refresh |
| `DEAD` | server -> client | `0x0217` | game | documented | first visible zero-HP death signal family reused by both the bootstrap `training_dummy` and the first practice-mob retaliation-owned player death floor; payload is little-endian `uint32 vid`; dummy death fanout is limited to sessions that can currently see that actor, and sessions that still had it selected pair that death edge with the existing self-only `TARGET(0, 0)` clear companion, while the player-death floor currently uses self `DEAD(owner_vid)` + self `TARGET(0, 0)` plus one visibility-gated queued peer `DEAD(owner_vid)` without broader corpse/respawn choreography |

## Notes

- This matrix is a working contract, not a dump of every observed packet family.
- Packets remain `candidate` until tests and captures freeze the final path.
- The first implementation should only touch the rows needed for the boot path milestone.
