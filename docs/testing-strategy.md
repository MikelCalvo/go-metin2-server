# Testing strategy

This project uses tests to freeze compatibility behavior, not just to catch accidental regressions.

## Principles

1. Protocol behavior is part of the public contract.
2. Every code change starts with a failing test.
3. Tests should describe externally visible behavior whenever possible.
4. Captured fixtures and golden data must be owned by this project.
5. Manual testing complements the suite; it never replaces it.

## Test layers

## 1. Golden packet tests

Use golden tests when the exact bytes matter.

Typical use cases:
- frame envelope encoding and decoding
- control packets
- login and selection packets
- main-character bootstrap packets
- movement packets

Golden tests answer:
- “does this packet match the expected wire format exactly?”

Store project-owned fixtures under `testdata/packets/`.

## 2. Unit tests

Use unit tests for:
- frame parsers
- packet validators
- state transitions
- config loaders
- repositories and data mappers
- helper functions used by the protocol layer

Unit tests should be fast and deterministic.

## 3. End-to-end socket tests

Use E2E tests for behaviors that only make sense across the wire:
- connect
- handshake
- login
- character list
- create character
- select character
- enter game
- move

E2E tests answer:
- “does the server behave correctly as a networked system?”

The preferred test harness is a small Go test client that:
- opens a TCP connection
- writes raw frames or packet structs
- reads responses
- asserts bytes, fields, and phase transitions

## 4. Fuzz tests

Use fuzzing for:
- frame decoding
- packet decoding
- boundary checking
- malformed lengths
- truncated payloads

Fuzzing helps harden the server against malformed or hostile input early.

## 5. Manual validation

Manual validation is still required for milestone closures, especially when a real client is involved.

Manual validation should confirm:
- the target client connects
- the visible phase flow is correct
- the client does not hang or disconnect unexpectedly
- `pprof` and logs remain usable when the server is under interaction

The reusable manual checklist for current real-client coverage lives at:
- `docs/qa/manual-client-checklist.md`

Manual validation must be documented in commit messages, issue notes, or follow-up docs when it proves something important.

## Test-first rule

For code changes, the workflow is:

1. add or refine the spec
2. write the failing test
3. run the test and confirm it fails for the expected reason
4. implement the minimum code required
5. run the test again
6. run the broader suite

No production code should be added without a failing test first.

## Commands

Base suite:

```bash
go test ./...
```

Target a single package or pattern:

```bash
go test ./internal/ops
go test ./... -run TestName
```

Fuzzing example:

```bash
go test ./... -fuzz=Fuzz -run=^$
```

Coverage example:

```bash
go test ./... -cover
```

## Fixture policy

Fixtures in this repository must be project-owned artifacts.

Allowed fixture sources:
- packet captures produced in the lab
- byte sequences rewritten into project-owned files
- fixtures derived from current observed behavior and rewritten here in our own format

Not allowed:
- vendoring external client or server source trees into this repo
- pasting large legacy code fragments as “test helpers”
- storing proprietary assets that are not required for protocol validation

## Recommended first wave of tests

The first wave of protocol work should add tests in this order:

1. frame decoder golden tests
2. frame encoder golden tests
3. session phase transition tests
4. handshake E2E tests
5. login/auth E2E tests
6. character list E2E tests
7. create character E2E tests
8. select character E2E tests
9. enter game E2E tests
10. movement E2E tests

## Debugging tools during test work

Use:
- `dlv` for stepping through handlers and decoders
- `pprof` for memory, goroutine, blocking, and mutex analysis
- structured logs for packet-level debugging

If a test failure requires a long manual debugging session, add a regression test before changing the code.

## Documentation-only changes

Documentation-only changes do not need a failing automated test, but they still need validation:
- the document must match current repo reality
- commands must be runnable or clearly marked as future work
- links and file paths must exist after the change
