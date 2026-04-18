# Packet matrix

This is the working packet inventory for the first protocol milestone.
It is intentionally narrow and only covers the boot path plus the first in-world movement step.

Status values:
- `documented` — the packet is part of the current written contract
- `planned` — known to be needed soon, but not yet locked by tests
- `candidate` — observed or expected, but still awaiting final confirmation in our own suite

## Control plane

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `PONG` | client -> server | `0x0006` | handshake/game control | documented | accepted as a header-only reply to server `PING`; phase-stable in `HANDSHAKE` and `GAME` |
| `PING` | server -> client | `0x0007` | handshake/game control | documented | carries `server_time`; current slice freezes the codec and the matching `PONG` reply path |
| `PHASE` | server -> client | `0x0008` | control | documented | phase transition control packet |
| `KEY_RESPONSE` | client -> server | `0x000A` | handshake | documented | cryptographic response path |
| `KEY_CHALLENGE` | server -> client | `0x000B` | handshake | documented | challenge + server key material |
| `KEY_COMPLETE` | server -> client | `0x000C` | handshake | documented | completes key exchange |
| `CLIENT_VERSION` | client -> server | `0x000D` | loading | documented | accepted as metadata in `LOADING`; no server response and no phase transition |

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
| `ENTERGAME` | client -> server | `0x0204` | loading | documented | enter-world request |
| `CHARACTER_ADD` | server -> client | `0x0205` | game bootstrap/visibility | documented | used for the selected-character bootstrap and for peer visibility |
| `CHAR_ADDITIONAL_INFO` | server -> client | `0x0207` | game bootstrap/visibility | documented | metadata companion for visible player inserts |
| `CHARACTER_DEL` | server -> client | `0x0208` | game visibility | documented | removes a visible peer by `vid` when that actor leaves the local world |
| `CHARACTER_UPDATE` | server -> client | `0x0209` | game bootstrap/visibility | documented | first self-only state refresh after the visible-world insert and the current peer-visibility refresh payload |
| `PLAYER_CREATE_SUCCESS` | server -> client | `0x020C` | select | documented | create success result |
| `PLAYER_CREATE_FAILURE` | server -> client | `0x020D` | select | documented | create failure result |
| `PLAYER_DELETE_SUCCESS` | server -> client | `0x020E` | select | documented | delete success result carrying the cleared account slot |
| `PLAYER_DELETE_FAILURE` | server -> client | `0x020F` | select | documented | minimal delete rejection placeholder using a header-only failure frame |
| `MAIN_CHARACTER` | server -> client | `0x0210` | loading | documented | main actor bootstrap |
| `PLAYER_POINTS` | server -> client | `0x0214` | loading/game bootstrap | documented | initial stat payload |
| `PLAYER_POINT_CHANGE` | server -> client | `0x0215` | game bootstrap | documented | first self-only point refresh after the selected-character bootstrap |

## Movement

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `MOVE` | client -> server | `0x0301` | game | documented | first in-world action |
| `MOVE` | server -> client | `0x0302` | game | documented | deterministic self ack for the mover and current queued replication payload for already-visible peers |
| `SYNC_POSITION` | client -> server | `0x0303` | game | documented | first selected-character position reconciliation path in `GAME` |
| `SYNC_POSITION` | server -> client | `0x0304` | game | documented | deterministic selected-character sync reply for the sender and current queued replication payload for already-visible peers |
| `WARP` | client -> server | `0x0305` | game | planned | out of early scope |
| `WARP` | server -> client | `0x0306` | game | planned | out of early scope |

## Chat

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `CHAT` | client -> server | `0x0601` | game | documented | current slice accepts `CHAT_TYPE_TALKING`, bootstrap `CHAT_TYPE_PARTY`, bootstrap `CHAT_TYPE_GUILD`, bootstrap `CHAT_TYPE_SHOUT`, and bootstrap `CHAT_TYPE_INFO`; client-originated `CHAT_TYPE_NOTICE` is currently rejected |
| `WHISPER` | client -> server | `0x0602` | game | documented | current slice routes by exact target character name with variable text payload |
| `CHAT` | server -> client | `0x0603` | game | documented | deterministic sender echo for talking/party/guild/shout and sender-only bootstrap system info with `vid = 0`; notice remains reserved for a future server-originated path |
| `WHISPER` | server -> client | `0x0604` | game | documented | direct whisper delivery to the named target on success and current `WHISPER_TYPE_NOT_EXIST` sender feedback for unknown targets |

## Notes

- This matrix is a working contract, not a dump of every observed packet family.
- Packets remain `candidate` until tests and captures freeze the final path.
- The first implementation should only touch the rows needed for the boot path milestone.
