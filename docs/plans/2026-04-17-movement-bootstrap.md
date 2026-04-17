# Movement Bootstrap Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: accept a minimal MOVE packet in GAME, keep the session stable, and expose a project-owned minimal replication/ack path that is easy to test.

Architecture: add a dedicated movement protocol package, then a tiny GAME-phase flow package, then stitch it into boot and the bootstrap runtime. Start with one-player deterministic behavior only; no visible-world fanout, no sync-position, no pathfinding.

Tech stack: Go 1.26, existing frame/session packages, net/http/tcp runtime already in repo.

---

## Task 1: Freeze minimal MOVE packet codecs
Objective: define the wire layouts for client MOVE and server MOVE replication.

Files:
- Create: `internal/proto/move/move.go`
- Create: `internal/proto/move/move_test.go`
- Update later docs only after code is green

Steps:
1. Write failing tests for:
   - client MOVE encode/decode
   - server MOVE encode/decode
   - invalid header/payload rejection
2. Run: `go test ./internal/proto/move`
3. Implement the smallest codec set to pass.
4. Re-run: `go test ./internal/proto/move`
5. Commit: `feat: add move packet codecs`

## Task 2: Add a minimal GAME flow
Objective: handle MOVE only when the session is already in GAME.

Files:
- Create: `internal/game/flow.go`
- Create: `internal/game/flow_test.go`

Steps:
1. Write failing tests for:
   - accepting MOVE in GAME
   - rejecting MOVE outside GAME
   - returning one deterministic replication packet
2. Run: `go test ./internal/game`
3. Implement a tiny config-driven flow with no extra behavior.
4. Re-run: `go test ./internal/game`
5. Commit: `feat: add minimal game movement flow`

## Task 3: Stitch movement into boot
Objective: route GAME-phase packets through the new game flow.

Files:
- Modify: `internal/boot/flow.go`
- Modify: `internal/boot/flow_test.go`
- Modify: `internal/boot/flow_socket_test.go`

Steps:
1. Write failing tests for:
   - boot flow accepts MOVE after ENTERGAME
   - TCP path reaches GAME and survives one MOVE round-trip
2. Run targeted failing tests.
3. Implement the smallest boot wiring to pass.
4. Re-run targeted tests, then `go test ./internal/boot`.
5. Commit: `feat: route move packets through boot flow`

## Task 4: Wire movement into bootstrap runtime
Objective: make `gamed` emit a deterministic MOVE replication packet using the selected character VID.

Files:
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/factory_test.go`

Steps:
1. Write failing tests for:
   - selected character can move in GAME
   - created character can move in GAME
2. Run: `go test ./internal/minimal`
3. Implement the smallest selected-character tracking needed.
4. Re-run: `go test ./internal/minimal`
5. Commit: `feat: add bootstrap movement handling`

## Task 5: Update docs
Objective: record the new GAME movement slice.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/boot-path.md`
- Optionally create: `spec/protocol/movement.md`

Steps:
1. Document the frozen packet shapes and current limits.
2. Run: `go test ./... && go vet ./...`
3. Commit: `docs: document movement bootstrap slice`

## Verification
- `go test ./...`
- `go vet ./...`
- Boot TCP test covers: handshake -> login -> select/create -> enter game -> move
- Runtime tests cover both pre-existing and newly-created characters

## Scope guardrails
- Do not add pathfinding
- Do not add sync-position yet
- Do not add multi-client fanout yet
- Do not redesign world state yet
- Keep all movement behavior deterministic and single-character
