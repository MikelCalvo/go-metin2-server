# Whisper Name Routing Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: accept one minimal `WHISPER` request in `GAME`, route it by exact target character name to another currently connected bootstrap session, and return a minimal not-found error to the sender when the target does not exist.

Architecture: extend `internal/proto/chat` with whisper packet support, extend `internal/game` with a whisper handler callback, and reuse the current shared-world registry to locate a connected target session by character name and enqueue one `GC_WHISPER` delivery frame.

Tech stack: Go 1.26, existing `internal/proto/chat`, `internal/game`, `internal/minimal`, and shared-world pending-frame queue.

---

## Task 1: Add failing whisper codec tests
Objective: freeze the minimal whisper packet shapes before implementation.

Files:
- Modify: `internal/proto/chat/chat_test.go`

Steps:
1. Add a failing client whisper round-trip test.
2. Add a failing server whisper round-trip test.
3. Run: `go test ./internal/proto/chat`
4. Confirm the failures are due to missing whisper codec support.

## Task 2: Implement the minimal whisper codec
Objective: add clean-room support for `CG::WHISPER` / `GC::WHISPER` without broader chat behavior.

Files:
- Modify: `internal/proto/chat/chat.go`

Steps:
1. Freeze `CG::WHISPER = 0x0602` and `GC::WHISPER = 0x0604`.
2. Support the target-name fixed field plus variable whisper payload.
3. Support the first whisper response types needed by this slice.
4. Re-run `go test ./internal/proto/chat` until it passes.

## Task 3: Add failing flow/runtime tests
Objective: prove the repo still lacks whisper dispatch and routing.

Files:
- Modify: `internal/game/flow_test.go`
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Add a failing `internal/game` test for `CG::WHISPER` dispatch in `GAME`.
2. Add a failing end-to-end test for successful whisper routing to another connected peer by exact target name.
3. Add a failing end-to-end test for the not-found sender error path.

## Task 4: Implement minimal whisper routing
Objective: direct delivery to target on success, sender feedback only on failure.

Files:
- Modify: `internal/game/flow.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/factory.go`

Steps:
1. Extend `internal/game` with a whisper handler callback and `CG::WHISPER` dispatch.
2. Add a shared-world helper that locates a session by exact character name and enqueues raw frames to it.
3. In the minimal runtime, accept non-empty whisper target/message only.
4. On success, queue one `GC_WHISPER` packet only to the named target and return no sender frame.
5. On target miss, return one sender `WHISPER_TYPE_NOT_EXIST` frame.

## Task 5: Update docs
Objective: document the first whisper slice without pretending the full whisper system exists.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/session-phases.md`
- Create: `spec/protocol/whisper-name-routing.md`
- Create: `docs/plans/2026-04-18-whisper-name-routing.md`

## Verification
- `go test ./internal/proto/chat`
- `go test ./internal/game -run TestHandleClientFrameAcceptsWhisperInGameAndReturnsDelivery`
- `go test ./internal/minimal -run 'TestNewGameSessionFactoryRoutesWhisperToNamedPeer|TestNewGameSessionFactoryReturnsWhisperNotExistForUnknownTarget'`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add party/guild chat here.
- Do not add block/mute logic here.
- Do not add cross-channel relay here.
- Do not add offline whisper storage here.
- Keep target lookup exact-name and local to currently connected bootstrap sessions.
