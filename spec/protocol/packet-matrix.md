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
| `CHAR_ADDITIONAL_INFO` | server -> client | `0x0207` | game bootstrap/visibility | documented | metadata companion for visible player inserts and the current bootstrap static-actor enter-game burst |
| `CHARACTER_DEL` | server -> client | `0x0208` | game visibility | documented | removes a visible peer by `vid` when that actor leaves the local world |
| `CHARACTER_UPDATE` | server -> client | `0x0209` | game bootstrap/visibility | documented | first self-only state refresh after the visible-world insert, the current peer-visibility refresh payload, and the current bootstrap static-actor enter-game refresh payload |
| `PLAYER_CREATE_SUCCESS` | server -> client | `0x020C` | select | documented | create success result |
| `PLAYER_CREATE_FAILURE` | server -> client | `0x020D` | select | documented | create failure result |
| `PLAYER_DELETE_SUCCESS` | server -> client | `0x020E` | select | documented | delete success result carrying the cleared account slot |
| `PLAYER_DELETE_FAILURE` | server -> client | `0x020F` | select | documented | minimal delete rejection placeholder using a header-only failure frame |
| `MAIN_CHARACTER` | server -> client | `0x0210` | loading | documented | main actor bootstrap |
| `PLAYER_POINTS` | server -> client | `0x0214` | loading/game bootstrap | documented | initial stat payload |
| `PLAYER_POINT_CHANGE` | server -> client | `0x0215` | game bootstrap / game | documented | first self-only point refresh after the selected-character bootstrap; the first owned consumable-use vertical also reuses it for the selected character with `type = 1`, fixed `amount = 50`, and `value = updated Points[1]` |

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
| `CHAT` | client -> server | `0x0601` | game | documented | current slice accepts `CHAT_TYPE_TALKING`, bootstrap `CHAT_TYPE_PARTY`, bootstrap `CHAT_TYPE_GUILD`, bootstrap `CHAT_TYPE_SHOUT`, and bootstrap `CHAT_TYPE_INFO`; client-originated `CHAT_TYPE_NOTICE` is currently rejected; bootstrap `CHAT_TYPE_TALKING` currently also hosts the server-owned slash seams `/inventory_move`, `/equip_item`, `/unequip_item`, and the first item-use bootstrap seam `/use_item <slot>` |
| `WHISPER` | client -> server | `0x0602` | game | documented | current slice routes by exact target character name with variable text payload |
| `CHAT` | server -> client | `0x0603` | game | documented | deterministic sender echo for talking/party/guild/shout; current fanout scope is same-map + same-empire for talking, same empire for shout, same non-zero `GuildID` for guild, and all connected sessions for party; sender-only bootstrap system info uses `vid = 0`; server-originated notice broadcasts also use `vid = 0` raw system text |
| `WHISPER` | server -> client | `0x0604` | game | documented | direct whisper delivery to the named target on success and current `WHISPER_TYPE_NOT_EXIST` sender feedback for unknown targets |

## Interaction

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `INTERACT` | client -> server | `0x0501` | game | documented | first owned client-originated interaction request for a visible bootstrap static actor target by `vid`; current payload is a little-endian `uint32 target_vid`, current routing stops at the dedicated `GAME`-phase interaction handler, current owned responses include self-only `info` / `talk` / `shop_preview`, and the current service-style NPC families also include transfer-backed `warp` |

## Items, inventory, and equipment

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `ITEM_USE` | client -> server | `TBD` | game | planned | final legacy compatibility ingress is not frozen yet; the current project-owned bootstrap path uses the slash seam `/use_item <slot>` for exactly one consumable prototype instead |
| `ITEM_SET` | server -> client | `0x0511` | game bootstrap / game | documented | first owned self-only occupied-slot bootstrap/update family for carried inventory and equipped items; total frame length `54`; position is packed `TItemPos(window:uint8, cell:uint16)` and equipped items currently ride the legacy combined inventory/equipment cell namespace (`window = INVENTORY`, `cell = 90 + wear_index`); the first consumable-use vertical also reuses it when consuming one stacked item leaves a non-zero count in the same carried slot |
| `ITEM_DEL` | server -> client | `0x0510` | game | documented | self-only slot-clear/remove companion for carried/equipped item mutations; total frame length `7` and payload is only packed `TItemPos`; the first consumable-use vertical also reuses it when consuming the last item removes that carried-slot stack entirely |

## Notes

- This matrix is a working contract, not a dump of every observed packet family.
- Packets remain `candidate` until tests and captures freeze the final path.
- The first implementation should only touch the rows needed for the boot path milestone.
