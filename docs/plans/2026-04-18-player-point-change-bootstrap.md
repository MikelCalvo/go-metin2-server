# Player-Point-Change Bootstrap Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: add the first minimal `PLAYER_POINT_CHANGE` emission after `ENTERGAME` so the selected character gets a deterministic self-only point refresh during the bootstrap transition into `GAME`.

Architecture: freeze a project-owned codec for `PLAYER_POINT_CHANGE`, extend visible-world bootstrap tests to expect the extra frame, then wire the minimal runtime to emit one deterministic self-only point refresh for the selected character.

Tech stack: Go 1.26, existing frame/session/world packages, current authd/gamed bootstrap runtime.

---

## Task 1: Freeze player-point-change packet codec
Objective: define the server-side `PLAYER_POINT_CHANGE` packet shape.

Files:
- Modify: `internal/proto/world/world.go`
- Modify: `internal/proto/world/world_test.go`

Steps:
1. Write failing tests for:
   - `PLAYER_POINT_CHANGE` encode/decode
   - invalid header rejection
2. Run: `go test ./internal/proto/world`
3. Implement the smallest codec set to pass.
4. Re-run: `go test ./internal/proto/world`
5. Commit: `feat: add player-point-change packet codec`

## Task 2: Extend visible-world bootstrap in the runtime
Objective: emit `PLAYER_POINT_CHANGE` after the selected-character visible bootstrap.

Files:
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/factory_test.go`

Steps:
1. Write failing tests for:
   - existing selected character gets the extra point-change frame
   - newly created selected character gets the same sequence
2. Run: `go test ./internal/minimal`
3. Implement the smallest selected-character packet builder.
4. Re-run: `go test ./internal/minimal`
5. Commit: `feat: add player-point-change bootstrap`

## Task 3: Cover boot composition
Objective: verify the composed boot flow and TCP harness can emit the extra point-change frame.

Files:
- Modify: `internal/boot/flow_test.go`
- Modify: `internal/boot/flow_socket_test.go`

Steps:
1. Extend visible-world bootstrap tests to expect `PLAYER_POINT_CHANGE`.
2. Run: `go test ./internal/boot`
3. Re-run after wiring the test config.
4. Commit: `test: cover player-point-change bootstrap`

## Task 4: Update docs
Objective: document the first `PLAYER_POINT_CHANGE` slice.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/visible-world-bootstrap.md`
- Modify: `spec/protocol/boot-path.md`
- Modify: `spec/protocol/packet-matrix.md`
- Create: `spec/protocol/player-point-change-bootstrap.md`

Steps:
1. Document packet layout and bootstrap order.
2. Run: `go test ./... && go vet ./...`
3. Commit: `docs: document player-point-change bootstrap slice`

## Verification
- `go test ./internal/proto/world`
- `go test ./internal/minimal`
- `go test ./internal/boot`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add repeated point-change streams yet
- Do not add other entities yet
- Do not add combat/inventory-driven point changes yet
- Keep the slice deterministic and selected-character only
