# Character-Update Bootstrap Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: add the first minimal `CHARACTER_UPDATE` emission after `ENTERGAME` so the selected character gets a deterministic self-only refresh immediately after the visible-world insert.

Architecture: freeze a project-owned codec for `CHARACTER_UPDATE`, extend visible-world bootstrap tests to expect the extra frame, then wire the minimal runtime to emit the packet after `CHARACTER_ADD` and `CHAR_ADDITIONAL_INFO`.

Tech stack: Go 1.26, existing frame/session/world packages, current authd/gamed bootstrap runtime.

---

## Task 1: Freeze character-update packet codec
Objective: define the server-side `CHARACTER_UPDATE` packet shape.

Files:
- Modify: `internal/proto/world/world.go`
- Modify: `internal/proto/world/world_test.go`

Steps:
1. Write failing tests for:
   - `CHARACTER_UPDATE` encode/decode
   - invalid header rejection
2. Run: `go test ./internal/proto/world`
3. Implement the smallest codec set to pass.
4. Re-run: `go test ./internal/proto/world`
5. Commit: `feat: add character-update packet codec`

## Task 2: Extend visible-world bootstrap in the runtime
Objective: emit `CHARACTER_UPDATE` after the selected-character visible insert.

Files:
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/factory_test.go`

Steps:
1. Write failing tests for:
   - existing selected character gets `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, and `CHARACTER_UPDATE`
   - newly created selected character gets the same sequence
2. Run: `go test ./internal/minimal`
3. Implement the smallest selected-character packet builder.
4. Re-run: `go test ./internal/minimal`
5. Commit: `feat: add character-update visible bootstrap`

## Task 3: Cover boot composition
Objective: verify the composed boot flow and TCP harness can emit the extra visible-world update frame.

Files:
- Modify: `internal/boot/flow_test.go`
- Modify: `internal/boot/flow_socket_test.go`

Steps:
1. Extend visible-world bootstrap tests to expect `CHARACTER_UPDATE`.
2. Run: `go test ./internal/boot`
3. Re-run after wiring the test config.
4. Commit: `test: cover character-update bootstrap`

## Task 4: Update docs
Objective: document the first `CHARACTER_UPDATE` slice.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/visible-world-bootstrap.md`
- Modify: `spec/protocol/boot-path.md`
- Modify: `spec/protocol/packet-matrix.md`
- Create: `spec/protocol/character-update-bootstrap.md`

Steps:
1. Document packet layout and bootstrap order.
2. Run: `go test ./... && go vet ./...`
3. Commit: `docs: document character-update bootstrap slice`

## Verification
- `go test ./internal/proto/world`
- `go test ./internal/minimal`
- `go test ./internal/boot`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add `CHARACTER_UPDATE2` yet
- Do not add other entities yet
- Do not add world-state fanout yet
- Keep the slice deterministic and selected-character only
