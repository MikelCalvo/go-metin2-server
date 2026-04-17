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
| `PONG` | client -> server | `0x0006` | handshake/control | documented | response to server ping |
| `PING` | server -> client | `0x0007` | handshake/control | documented | includes `server_time` |
| `PHASE` | server -> client | `0x0008` | control | documented | phase transition control packet |
| `KEY_RESPONSE` | client -> server | `0x000A` | handshake | documented | cryptographic response path |
| `KEY_CHALLENGE` | server -> client | `0x000B` | handshake | documented | challenge + server key material |
| `KEY_COMPLETE` | server -> client | `0x000C` | handshake | documented | completes key exchange |
| `CLIENT_VERSION` | client -> server | `0x000D` | late handshake/loading | planned | exact timing to be locked by tests |

## Authentication and selection surface

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `LOGIN2` | client -> server | `0x0101` | login | documented | current working login request variant |
| `LOGIN3` | client -> server | `0x0102` | login | candidate | keep under review until captures lock the exact path |
| `LOGIN_SECURE` | client -> server | `0x0103` | login | candidate | not part of the first implementation unless required |
| `LOGIN_SUCCESS3` | server -> client | `0x0104` | login/select | candidate | possible character-list success shape |
| `LOGIN_SUCCESS4` | server -> client | `0x0105` | login/select | candidate | possible character-list success shape |
| `LOGIN_FAILURE` | server -> client | `0x0106` | login | documented | negative auth path |
| `LOGIN_KEY` | server -> client | `0x0107` | login | documented | login-key path used before/with auth |
| `EMPIRE` | server -> client | `0x0109` | select | documented | selection surface empire state |
| `EMPIRE` | client -> server | `0x010A` | select | documented | empire selection request |

## Character lifecycle

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `CHARACTER_CREATE` | client -> server | `0x0201` | select | documented | create character request |
| `CHARACTER_DELETE` | client -> server | `0x0202` | select | planned | not required for the first milestone |
| `CHARACTER_SELECT` | client -> server | `0x0203` | select | documented | choose active character |
| `ENTERGAME` | client -> server | `0x0204` | loading | documented | enter-world request |
| `PLAYER_CREATE_SUCCESS` | server -> client | `0x020C` | select | documented | create success result |
| `PLAYER_CREATE_FAILURE` | server -> client | `0x020D` | select | documented | create failure result |
| `PLAYER_DELETE_SUCCESS` | server -> client | `0x020E` | select | planned | later milestone |
| `MAIN_CHARACTER` | server -> client | `0x0210` | loading | documented | main actor bootstrap |
| `PLAYER_POINTS` | server -> client | `0x0214` | loading/game bootstrap | documented | initial stat payload |
| `PLAYER_POINT_CHANGE` | server -> client | `0x0215` | game | planned | likely needed shortly after bootstrap |

## Movement

| Name | Direction | Header | Phase | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| `MOVE` | client -> server | `0x0301` | game | documented | first in-world action |
| `MOVE` | server -> client | `0x0302` | game | planned | replication/ack path |
| `SYNC_POSITION` | client -> server | `0x0303` | game | planned | not part of the first movement milestone |
| `SYNC_POSITION` | server -> client | `0x0304` | game | planned | not part of the first movement milestone |
| `WARP` | client -> server | `0x0305` | game | planned | out of early scope |
| `WARP` | server -> client | `0x0306` | game | planned | out of early scope |

## Notes

- This matrix is a working contract, not a dump of every observed packet family.
- Packets remain `candidate` until tests and captures freeze the final path.
- The first implementation should only touch the rows needed for the boot path milestone.
