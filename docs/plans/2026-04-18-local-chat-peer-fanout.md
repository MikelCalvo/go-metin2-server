# Local Chat Peer Fanout Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: once two players are already visible to each other in the bootstrap runtime, make a minimal talking chat from one peer appear both for the sender and for the other visible peer.

Architecture: add a small `internal/proto/chat` package for `CG::CHAT` / `GC::CHAT`, extend `internal/game` to dispatch that header in `GAME`, and reuse the existing shared-world queued-frame hook to fan the server chat delivery out to already-visible peers.

Tech stack: Go 1.26, existing `internal/game`, `internal/minimal`, and shared-world pending-frame queue.

---

## Task 1: Add failing proto tests
Objective: freeze the minimal chat packet shapes before implementation.

Files:
- Create: `internal/proto/chat/chat_test.go`

Steps:
1. Write failing tests for one client chat packet round-trip and one server chat delivery round-trip.
2. Run: `go test ./internal/proto/chat`
3. Confirm the package fails because the codec does not exist yet.

## Task 2: Implement the minimal chat codec
Objective: add the smallest possible clean-room packet support for the first local chat slice.

Files:
- Create: `internal/proto/chat/chat.go`

Steps:
1. Freeze `CG::CHAT = 0x0601` and `GC::CHAT = 0x0603`.
2. Support `CHAT_TYPE_TALKING` plus basic chat type constants for future reuse.
3. Match client request framing with `type + NUL-terminated text`.
4. Match server delivery framing with `type + vid + empire + text`.

## Task 3: Add failing flow/runtime tests
Objective: prove the repo still does not handle chat in `GAME`.

Files:
- Modify: `internal/game/flow_test.go`
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Add a failing `internal/game` test for `CG::CHAT` dispatch in `GAME`.
2. Add a failing end-to-end `internal/minimal` test where two peers enter `GAME`, one sends chat, and the other expects one queued `GC_CHAT` frame.
3. Confirm the failures are due to missing chat support, not malformed tests.

## Task 4: Implement minimal local talking chat
Objective: sender echo plus peer fanout, no broader chat system yet.

Files:
- Modify: `internal/game/flow.go`
- Modify: `internal/minimal/factory.go`

Steps:
1. Extend `internal/game` with a chat handler callback and `CG::CHAT` dispatch.
2. In the minimal runtime, accept only `CHAT_TYPE_TALKING` with a non-empty message.
3. Build the wire text as `Name : message`.
4. Return one direct `GC_CHAT` delivery to the sender.
5. Queue the same encoded `GC_CHAT` delivery to other visible shared-world sessions.

## Task 5: Update docs
Objective: document the first local chat slice without pretending broader chat is done.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/session-phases.md`
- Create: `spec/protocol/local-chat-peer-fanout.md`
- Create: `docs/plans/2026-04-18-local-chat-peer-fanout.md`

## Verification
- `go test ./internal/proto/chat`
- `go test ./internal/game -run TestHandleClientFrameAcceptsChatInGameAndReturnsDelivery`
- `go test ./internal/minimal -run TestNewGameSessionFactoryQueuesPeerChatForVisiblePlayers`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add party chat.
- Do not add guild chat.
- Do not add whisper.
- Do not add shout.
- Do not add chat moderation or slash-command execution yet.
- Keep chat fanout limited to already-visible sessions.
