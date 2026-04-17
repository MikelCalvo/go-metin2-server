# Frame layout

This document defines the initial working assumptions for the client/server frame envelope used by the compatibility target.

## Working envelope

The current working assumption for the target client is a length-prefixed binary frame:

- `header`: 16-bit unsigned little-endian
- `length`: 16-bit unsigned little-endian
- `payload`: packet-specific bytes

Layout:

```text
[header:2][length:2][payload...]
```

## Rules

- `length` is the total frame size, including the 4-byte envelope
- the minimum valid frame size is `4`
- packets are carried over a TCP stream, not message-delimited datagrams
- the decoder must handle both fragmentation and coalescing

## Stream decoder requirements

The decoder must be able to:
- buffer partial frames across reads
- emit multiple complete frames from a single read
- reject frames with invalid total length
- reject frames that claim a length smaller than the envelope size
- avoid allocating unbounded memory on malformed input

## Control-plane packets observed in the current target

The following control-plane packet headers are part of the working boot-path scope:

### Client -> server
- `PONG` = `0x0006`
- `KEY_RESPONSE` = `0x000A`
- `CLIENT_VERSION` = `0x000D`

### Server -> client
- `PING` = `0x0007`
- `PHASE` = `0x0008`
- `KEY_CHALLENGE` = `0x000B`
- `KEY_COMPLETE` = `0x000C`

## Known control structures in the working model

### `PHASE`

```text
header   uint16
length   uint16
phase    uint8
```

### `PING`

```text
header       uint16
length       uint16
server_time  uint32
```

### `PONG`

```text
header   uint16
length   uint16
```

### `KEY_CHALLENGE`

```text
header            uint16
length            uint16
server_pk         [32]byte
challenge         [32]byte
server_time       uint32
```

### `KEY_RESPONSE`

```text
header              uint16
length              uint16
client_pk           [32]byte
challenge_response  [32]byte
```

### `KEY_COMPLETE`

```text
header            uint16
length            uint16
encrypted_token   [48]byte
nonce             [24]byte
```

## Decoder milestones

The frame layer should be implemented before any gameplay or auth handlers.
Its first milestone is complete when the repository has:
- golden tests for valid frames
- tests for truncated frames
- tests for multiple frames in one read
- tests for one frame split across multiple reads
- fuzz coverage for malformed lengths and payload boundaries

## Open questions

These items remain intentionally open until tests and captures lock them down:
- whether all observed packet families use exactly the same envelope in practice
- whether any compatibility edge cases require packet-family-specific parsing shortcuts
- whether the server must tolerate historical deviations in selected packet lengths
