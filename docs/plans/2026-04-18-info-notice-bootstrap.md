# Info and Notice Bootstrap Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: freeze minimal system-chat handling for `CHAT_TYPE_INFO` and `CHAT_TYPE_NOTICE` in `GAME`, with `INFO` as a bootstrap self/system delivery and `NOTICE` as a bootstrap system broadcast.

Architecture: keep using the existing `CHAT` / `GC_CHAT` packet family, but split actor chat from system chat inside the minimal runtime so talking/party/guild/shout retain actor-formatted payloads while info/notice use raw message payloads with `vid = 0`.

Tech stack: Go 1.26, existing `internal/proto/chat`, `internal/game`, and `internal/minimal`.

---

## Task 1: Add failing tests
Objective: prove `CHAT_TYPE_INFO` and `CHAT_TYPE_NOTICE` are still rejected in the runtime.

Files:
- Modify: `internal/game/flow_test.go`
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Add `internal/game` coverage for `CHAT_TYPE_INFO` and `CHAT_TYPE_NOTICE` dispatch.
2. Add a failing `internal/minimal` test for sender-only system `INFO` delivery.
3. Add a failing `internal/minimal` test for system `NOTICE` broadcast to connected peers.
4. Confirm the real failure is the runtime acceptance in `internal/minimal`.

## Task 2: Implement minimal system-chat handling
Objective: support bootstrap `INFO` and `NOTICE` without introducing real event systems yet.

Files:
- Modify: `internal/minimal/factory.go`

Steps:
1. Split actor-chat handling from system-chat handling in the minimal chat path.
2. Keep talking/party/guild/shout on the existing `Name : message` actor payload shape.
3. Add a system-chat helper that emits raw message payloads with `vid = 0`.
4. Make `CHAT_TYPE_INFO` self-only.
5. Make `CHAT_TYPE_NOTICE` sender + queued peer broadcast.

## Task 3: Update docs
Objective: document the bootstrap system-chat policy clearly.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/session-phases.md`
- Create: `spec/protocol/info-notice-bootstrap.md`
- Create: `docs/plans/2026-04-18-info-notice-bootstrap.md`

## Verification
- `go test ./internal/minimal -run 'TestNewGameSessionFactory(ReturnsInfoChatOnlyToSenderAsSystemMessage|QueuesNoticeChatAsSystemBroadcast)' -count=1`
- `go test ./internal/game -run 'TestHandleClientFrameAccepts(Info|Notice)ChatInGameAndReturnsDelivery' -count=1`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add a separate GM/operator command path yet.
- Do not add timed notice scheduling yet.
- Do not add permission or auth rules around notice emission yet.
- Keep the bootstrap nature explicit in public docs.
