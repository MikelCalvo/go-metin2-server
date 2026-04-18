# Sync-position Bootstrap Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: accept the first minimal `SYNC_POSITION` traffic after `ENTERGAME` so the bootstrap runtime can reconcile the selected character position without dropping the session.

Architecture: freeze project-owned codecs for client/server `SYNC_POSITION`, extend the in-game flow to route the packet family, then wire the minimal runtime to reconcile the selected character and return a deterministic self-only sync reply.

Tech stack: Go 1.26, existing frame/session/move packages, current authd/gamed bootstrap runtime.

---

## Task 1: Freeze sync-position packet codecs
Objective: define project-owned codecs for client/server `SYNC_POSITION` frames.

Files:
- Modify: `internal/proto/move/move.go`
- Modify: `internal/proto/move/move_test.go`

Steps:
1. Write failing tests for:
   - `SYNC_POSITION` encode/decode
   - server `SYNC_POSITION` encode/decode
   - invalid partial-element payload rejection
2. Run: `go test ./internal/proto/move`
3. Implement the smallest codec set to pass.
4. Re-run: `go test ./internal/proto/move`
5. Commit: `feat: add sync-position packet codecs`

## Task 2: Extend in-game flow
Objective: let `GAME` sessions accept `SYNC_POSITION` alongside `MOVE`.

Files:
- Modify: `internal/game/flow.go`
- Modify: `internal/game/flow_test.go`

Steps:
1. Write failing tests for:
   - `SYNC_POSITION` in `GAME` routes to a handler
   - accepted sync packets emit a server sync reply
2. Run: `go test ./internal/game`
3. Implement the smallest dispatch/result extension to pass.
4. Re-run: `go test ./internal/game`
5. Commit: `feat: route sync-position packets in game flow`

## Task 3: Cover boot composition
Objective: verify the composed boot flow and TCP harness stay stable when `SYNC_POSITION` is sent after `ENTERGAME`.

Files:
- Modify: `internal/boot/flow_test.go`
- Modify: `internal/boot/flow_socket_test.go`

Steps:
1. Add tests for:
   - unit boot flow sync-position routing in `GAME`
   - TCP boot flow sync-position round-trip after `ENTERGAME`
2. Run: `go test ./internal/boot`
3. Implement only any missing config wiring.
4. Re-run: `go test ./internal/boot`
5. Commit: `test: cover sync-position in boot flow`

## Task 4: Wire selected-character reconciliation in bootstrap runtime
Objective: make the real bootstrap runtime reconcile the selected character position from self-only sync traffic.

Files:
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/factory_test.go`

Steps:
1. Write failing tests for:
   - existing selected character gets a deterministic sync reply
   - the selected character coordinates update through the sync path
2. Run: `go test ./internal/minimal`
3. Implement the smallest selected-character sync handler.
4. Re-run: `go test ./internal/minimal`
5. Commit: `feat: add sync-position bootstrap handling`

## Task 5: Update docs
Objective: document the first sync-position slice.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/boot-path.md`
- Modify: `spec/protocol/packet-matrix.md`
- Create: `spec/protocol/sync-position-bootstrap.md`

Steps:
1. Document packet layout and current bootstrap behavior.
2. Run: `go test ./... && go vet ./...`
3. Commit: `docs: document sync-position bootstrap slice`

## Verification
- `go test ./internal/proto/move`
- `go test ./internal/game`
- `go test ./internal/boot`
- `go test ./internal/minimal`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add multi-entity sync rules yet
- Do not add fanout to other clients yet
- Do not add anti-cheat or distance validation yet
- Keep the slice deterministic and selected-character only
