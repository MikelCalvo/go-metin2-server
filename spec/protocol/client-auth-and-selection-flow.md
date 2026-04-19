# Client auth and selection choreography

This document freezes the client-driven choreography from auth handshake through the character-selection surface.

It complements the packet-layout documents by recording the order, prerequisites, and compatibility-sensitive assumptions that matter for the Go implementation.

## Scope

Covered path:
- auth socket connect
- secure handshake on authd
- `PHASE(AUTH)`
- `LOGIN3`
- `AUTH_SUCCESS`
- login-key handoff into the main stream
- game socket connect
- secure handshake on gamed
- `PHASE(LOGIN)`
- `LOGIN2`
- transition to the select surface
- empire/create/delete/select requests

## High-level model

The client uses two distinct login surfaces:

1. `AccountConnector`
   - password-based credential submission to authd using `LOGIN3`
   - receives `AUTH_SUCCESS` with a `login_key`

2. `CPythonNetworkStream`
   - game/login socket that performs login-by-key using `LOGIN2`
   - drives the selection and world-entry flow

The password does not travel on the main game socket.
The login key is the bridge.

## Auth socket flow

Expected compatibility path:

1. TCP connect to authd
2. secure control handshake completes
3. server emits `PHASE(AUTH)`
4. client sends `LOGIN3`
5. server replies with either:
   - `AUTH_SUCCESS`, or
   - `LOGIN_FAILURE`

On successful auth:
- client extracts `login_key`
- client stores it into the main network stream
- client connects the main stream to the configured game address/port
- auth connector disconnects afterwards

## `LOGIN3` contract

`LOGIN3` is the auth-socket credential request.

Practical implementation note:
- this project already discovered that the real local client uses the shorter legacy password field layout, not a naive wider one
- auth slices should keep that layout frozen by tests and fixtures

Behavioral note:
- `LOGIN3` is only sent after the auth connector receives `PHASE(AUTH)`
- a correct secure handshake alone is not enough to trigger the request

## Main game/login socket flow

Expected compatibility path:

1. TCP connect to gamed
2. server emits `PHASE(HANDSHAKE)`
3. secure control handshake completes
4. server emits `PHASE(LOGIN)`
5. client sends `LOGIN2`
6. server emits the selection-surface packets and transitions toward `SELECT`

## `LOGIN2` contract

`LOGIN2` is the main-socket login-by-key request.

Operationally:
- it uses account name plus `login_key`
- it does not require the account password on this socket
- if the client does not have a non-zero `login_key`, it will not perform this step correctly

Practical server rule:
- gamed must treat `LOGIN2` as a ticket/key validation step, not a password-auth step

## Selection-surface packet ordering

The client is stricter than it first appears, but it also has some tolerated ordering overlap.

Observed safe model:
- `LOGIN_SUCCESS4`
- `EMPIRE`
- `PHASE(SELECT)`

Important nuance:
- the client selection path can still tolerate some of these packets around the phase boundary
- this means the server does not need to assume a single fragile ordering if compatibility tests freeze the chosen sequence

Even so, future slices should prefer one deterministic order and lock it with end-to-end tests.

## Empty-account path

If the account has no finalized empire/characters path yet, the client can use:
- `CG::EMPIRE`
- `CG::CHARACTER_CREATE`

This is why the MVP selection surface must include more than just character listing.
A realistic first bootstrap also needs the empty-account recovery path.

## Character request contracts

### `EMPIRE`
- client -> server
- single-byte empire choice on the wire
- client updates its local empire selection state immediately when sending

### `CHARACTER_CREATE`
- client -> server
- includes:
  - target slot
  - name
  - job / race choice
  - shape
  - starting stat bytes

Practical note:
- the client wire layout uses the legacy packed field widths and a 16-bit job field
- keep this frozen in Go fixtures/tests

### `CHARACTER_DELETE`
- client -> server
- includes slot plus fixed-width private-code field

Practical note:
- the client expects exact field width here
- deletion validation must be server-side strict even if the client-side wrapper validation is loose or buggy

### `CHARACTER_SELECT`
- client -> server
- small selection request carrying the target slot/index

On success, the server should move to `LOADING` and start the loading/bootstrap path.

## Response expectations

Character lifecycle responses expected by the client include:
- `PLAYER_CREATE_SUCCESS`
- `PLAYER_CREATE_FAILURE`
- `PLAYER_DELETE_SUCCESS`
- delete-failure placeholder packet

The delete-failure path is especially easy to get wrong because the client expects the legacy placeholder semantics rather than an arbitrary rich error payload.

## Direct-enter and reconnect notes

The client also supports reconnect-style entry paths driven by per-character address/port fields.
This means the implementation should keep the distinction between:
- auth success
- game connect
- selection surface
- direct game reconnect

Even if the first MVP keeps one address/port only, the docs should preserve the fact that the client model already allows reconnect-based world entry.

## Rejected assumptions

1. The password-auth socket and the game/login socket are basically the same flow.
2. `LOGIN2` is another password login request.
3. `LOGIN3` is sent immediately after auth TCP connect.
4. character selection can be reduced to one happy-path packet with no create/delete implications.

## Implementation guidance for future Go slices

### Freeze these contracts separately
- authd secure connect -> `PHASE(AUTH)` -> `LOGIN3` -> `AUTH_SUCCESS`
- gamed secure connect -> `PHASE(LOGIN)` -> `LOGIN2`
- select-surface ordering
- empty-account path (`EMPIRE` + `CHARACTER_CREATE`)
- delete path (`CHARACTER_DELETE` + legacy failure/success behavior)

### Prefer end-to-end tests for these points
- auth connector transcript test
- auth success hands login key to game socket test
- game login-by-key transcript test
- create/select/delete transcript tests

### Keep server-side validation conservative
The client should not be treated as trustworthy for:
- slot indices
- delete private-code semantics
- implicit phase correctness

The server must validate these explicitly.

## Suggested next slices

1. Freeze `AUTH_SUCCESS` / `LOGIN3` transcript with real-client-compatible packet sequencing.
2. Freeze `LOGIN2` ticket validation transcript on gamed.
3. Freeze a deterministic select-surface ordering in one E2E test.
4. Freeze create/delete/select for both non-empty and empty-account cases.
