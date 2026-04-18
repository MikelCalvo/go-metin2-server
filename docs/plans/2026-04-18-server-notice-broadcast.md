# Server Notice Broadcast Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: introduce the first real server-originated bootstrap path for `CHAT_TYPE_NOTICE` while keeping client-originated notice requests rejected.

Architecture: keep using the existing `GC_CHAT` system-message shape with `vid = 0`, add a tiny runtime object that owns the shared-world registry and can queue a notice broadcast programmatically, and defer the eventual operator/HTTP trigger surface to a later slice.

Tech stack: Go 1.26, existing `internal/minimal`, `internal/proto/chat`, and protocol docs.

---

## Task 1: Add failing tests
Objective: prove no programmatic notice broadcaster exists yet.

Files:
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Add a failing end-to-end test that triggers a server-originated notice broadcast for two connected `GAME` sessions.
2. Add a failing guard test that empty notice text queues nothing.
3. Keep the existing client-originated notice rejection test green as the negative side of the contract.

## Task 2: Implement minimal runtime notice broadcasting
Objective: add the smallest useful server-originated notice path without choosing the final operator surface.

Files:
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/shared_world.go`

Steps:
1. Introduce a tiny runtime object that owns the shared-world registry and session factory.
2. Preserve the existing `NewGameSessionFactory` API by wrapping the runtime object.
3. Add a shared-world helper that queues one system `GC_CHAT` notice to all connected sessions.
4. Expose a programmatic `BroadcastNotice(message string)` runtime method.

## Task 3: Update docs
Objective: separate `INFO` semantics from the new server-originated `NOTICE` path.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/session-phases.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/info-notice-bootstrap.md`
- Create: `spec/protocol/server-notice-broadcast.md`
- Create: `docs/plans/2026-04-18-server-notice-broadcast.md`

## Verification
- `go test ./internal/minimal -run 'TestGameRuntimeBroadcastNotice(QueuesSystemMessageToConnectedSessions|RejectsEmptyMessage)|TestNewGameSessionFactoryRejectsClientOriginatedNoticeChat' -count=1`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add an HTTP notice endpoint yet.
- Do not add GM command parsing yet.
- Do not re-open client-originated `CHAT_TYPE_NOTICE`.
- Do not bundle scheduling or event hooks into this same slice.
