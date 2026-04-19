# Client subsystems and control-plane behavior

This document freezes the high-level network behavior of the local TMP4-era client as seen from the project-owned clean-room analysis.

It focuses on one source of previous confusion: the client does not behave like one single session machine.
It contains three distinct network actors with materially different expectations.

This document is written in project-owned language and is intended to guide future Go compatibility slices.

## The three client-side network actors

### 1. `CPythonNetworkStream`

This is the main game/login stream used for:
- login-by-key on the game socket
- character selection and deletion/creation
- loading, enter-game, and in-world traffic

Important characteristic:
- after TCP connect it starts from an offline handler and expects a `PHASE` packet before it begins normal handshake processing

Practical server rule:
- on the main game socket, send `PHASE(HANDSHAKE)` before `KEY_CHALLENGE`

### 2. `AccountConnector`

This is the auth-server connector used for:
- secure handshake on the auth socket
- `LOGIN3` credential submission
- `AUTH_SUCCESS` / `LOGIN_FAILURE`
- delivery of the login key back into the main stream

Important characteristic:
- it enters an internal handshake state immediately on connect
- it does not need a preceding `PHASE(HANDSHAKE)` packet to accept `KEY_CHALLENGE`

Practical server rule:
- authd may begin with `KEY_CHALLENGE` directly after connect
- after handshake completion it must drive the client with `PHASE(AUTH)` before expecting `LOGIN3`

### 3. `ServerStateChecker`

This is a separate lightweight probe socket used for:
- channel-status checks
- server-list/channel UI refresh

Important characteristic:
- it does not follow the normal login/game flow
- after connect it immediately sends `STATE_CHECKER`
- on receive, it skips unrelated framed packets until it finds `RESPOND_CHANNELSTATUS`

Practical server rule:
- treat it as a separate compatibility surface
- do not assume it will perform the secure handshake before requesting status

## Control headers

The locally targeted client uses `uint16` headers with the shared project envelope.
Relevant control headers are:

- `0x0006` — `PONG`
- `0x0007` — `PING`
- `0x0008` — `PHASE`
- `0x000A` — `KEY_RESPONSE`
- `0x000B` — `KEY_CHALLENGE`
- `0x000C` — `KEY_COMPLETE`
- `0x000D` — `CLIENT_VERSION`
- `0x000F` — `STATE_CHECKER`
- `0x0010` — `RESPOND_CHANNELSTATUS`

## Phase values

Observed phase byte values:

- `0x00` — `CLOSE`
- `0x01` — `HANDSHAKE`
- `0x02` — `LOGIN`
- `0x03` — `SELECT`
- `0x04` — `LOADING`
- `0x05` — `GAME`
- `0x06` — `DEAD`
- `0x07` — `CLIENT_CONNECTING`
- `0x08` — `DBCLIENT`
- `0x09` — `P2P`
- `0x0A` — `AUTH`

## First packet expectations after TCP connect

### Main game socket (`CPythonNetworkStream`)

Expected server-owned bootstrap:

1. `PHASE(HANDSHAKE)`
2. `KEY_CHALLENGE`
3. client `KEY_RESPONSE`
4. `KEY_COMPLETE`
5. `PHASE(LOGIN)`

Why this matters:
- before receiving `PHASE(HANDSHAKE)`, the main stream still uses its offline handler
- in that state it only accepts `PHASE`
- sending `KEY_CHALLENGE` as the very first packet can leave the client stuck without replying

### Auth socket (`AccountConnector`)

Expected server-owned bootstrap:

1. `KEY_CHALLENGE`
2. client `KEY_RESPONSE`
3. `KEY_COMPLETE`
4. `PHASE(AUTH)`
5. client `LOGIN3`

### State-checker socket (`ServerStateChecker`)

Expected client-owned bootstrap:

1. client connects
2. client sends `STATE_CHECKER`
3. server replies with `RESPOND_CHANNELSTATUS`

The checker can tolerate unrelated framed packets before the status reply, but it is still best treated as its own surface.

## Accepted control packets by subsystem

### `CPythonNetworkStream`

Observed control tolerance:
- `KEY_CHALLENGE` and `KEY_COMPLETE` are accepted in more than the literal handshake phase
- the client can still consume them during `LOGIN`, `SELECT`, `LOADING`, and `GAME`
- `PING` is also accepted in multiple runtime phases

Practical meaning:
- the client is more permissive than a naive one-phase-only model
- our server policy may remain stricter, but docs and tests must not assume the client only accepts `KEY_*` inside a tiny handshake window

### `AccountConnector`

Observed control tolerance:
- `KEY_CHALLENGE`, `KEY_COMPLETE`, `PING`, and `PHASE` are accepted in both its handshake state and auth state

Practical meaning:
- authd slices should not assume `KEY_*` become impossible immediately after `PHASE(AUTH)`

### `ServerStateChecker`

Observed control tolerance:
- it does not try to interpret most packets semantically
- it peeks framed packets and skips them by length until it finds `RESPOND_CHANNELSTATUS`

Practical meaning:
- this client path is resilient to prelude packets, but only if they remain parseable in plaintext framing

## Unknown-header behavior differences

The three subsystems do not fail the same way.

### Main stream
- unknown packet in current phase is treated as a hard compatibility problem
- recent packet history is dumped
- recv buffer is cleared
- processing returns false

### Auth connector
- unexpected packet is effectively ignored by its analyzer loop
- this can leave the session waiting rather than loudly failing

### State checker
- unknown framed packets are skipped until the target response appears

Practical meaning:
- do not infer server correctness from one subsystem behaving "quietly"
- the main game stream is the strictest compatibility oracle

## Rejected assumptions

The following assumptions should be treated as wrong for this project:

1. `KEY_CHALLENGE` can safely be the first packet on every socket.
2. `KEY_*` packets are only relevant during a tiny handshake-only acceptance window.
3. the state checker follows the same choreography as the auth/game sockets.
4. all client sockets react similarly to unknown headers.

## Implementation guidance for future Go slices

When adding or reviewing compatibility slices, freeze behavior per subsystem instead of per packet family only.

Minimum rule set:
- auth socket transcript and tests
- main game socket transcript and tests
- state-checker transcript and tests

A packet being correctly encoded is not enough.
The exact subsystem, phase, and packet order must be part of the contract.

## Suggested follow-up tests

- main game socket requires `PHASE(HANDSHAKE)` before `KEY_CHALLENGE`
- auth socket accepts `KEY_CHALLENGE` immediately after connect
- state checker works on a separate socket without normal login/game traffic
- state checker skips prelude packets until `RESPOND_CHANNELSTATUS`
- main stream still tolerates `KEY_*` control packets after entering later phases, if that compatibility window becomes relevant
