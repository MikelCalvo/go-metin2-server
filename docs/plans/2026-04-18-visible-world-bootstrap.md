# Visible-World Bootstrap Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: add the first minimal visible-world bootstrap immediately after ENTERGAME so the selected character is inserted into the game world with deterministic self-spawn packets.

Architecture: freeze the minimal wire codecs for CHARACTER_ADD and CHAR_ADDITIONAL_INFO, then extend world-entry ENTERGAME handling so it can emit extra world bootstrap frames after PHASE(GAME). Finally wire the bootstrap runtime so the selected character is announced through those packets.

Tech stack: Go 1.26, existing frame/session/world packages, current authd/gamed bootstrap runtime.

---

## Task 1: Freeze visible-world packet codecs
Objective: define project-owned codecs for the first visible-world insert packets.

Files:
- Modify: `internal/proto/world/world.go`
- Modify: `internal/proto/world/world_test.go`

Steps:
1. Write failing tests for:
   - `CHARACTER_ADD` encode/decode
   - `CHAR_ADDITIONAL_INFO` encode/decode
   - invalid header/payload rejection
2. Run: `go test ./internal/proto/world`
3. Implement the smallest codec set to pass.
4. Re-run: `go test ./internal/proto/world`
5. Commit: `feat: add visible-world bootstrap packet codecs`

## Task 2: Extend world-entry ENTERGAME output
Objective: let ENTERGAME emit extra visible-world bootstrap frames after `PHASE(GAME)`.

Files:
- Modify: `internal/worldentry/flow.go`
- Modify: `internal/worldentry/flow_test.go`

Steps:
1. Write failing tests for:
   - ENTERGAME returns `PHASE(GAME)` plus configured visible-world frames
   - default ENTERGAME path still works without extra frames
2. Run: `go test ./internal/worldentry`
3. Implement the smallest config/result extension to pass.
4. Re-run: `go test ./internal/worldentry`
5. Commit: `feat: add world-entry visible bootstrap output`

## Task 3: Stitch visible-world bootstrap into boot tests
Objective: verify the composed boot flow can emit the first world-insert packets.

Files:
- Modify: `internal/boot/flow_test.go`
- Modify: `internal/boot/flow_socket_test.go`

Steps:
1. Write failing tests for:
   - unit boot flow receives visible-world bootstrap after ENTERGAME
   - TCP boot path receives the same sequence
2. Run targeted failing tests.
3. Implement only the required test config wiring.
4. Re-run: `go test ./internal/boot`
5. Commit: `test: cover visible-world bootstrap in boot flow`

## Task 4: Wire selected-character self-spawn in bootstrap runtime
Objective: make the real bootstrap runtime announce the selected character into the visible world immediately after ENTERGAME.

Files:
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/factory_test.go`

Steps:
1. Write failing tests for:
   - existing selected character gets `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO`
   - newly created selected character gets the same visible-world bootstrap
2. Run: `go test ./internal/minimal`
3. Implement the smallest selected-character packet builders.
4. Re-run: `go test ./internal/minimal`
5. Commit: `feat: add visible-world bootstrap to minimal runtime`

## Task 5: Update docs
Objective: document the first visible-world slice.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/select-world-entry.md`
- Optionally create: `spec/protocol/visible-world-bootstrap.md`

Steps:
1. Document current packet shapes and the ENTERGAME output order.
2. Run: `go test ./... && go vet ./...`
3. Commit: `docs: document visible-world bootstrap slice`

## Verification
- `go test ./...`
- `go vet ./...`
- Boot TCP test covers: handshake -> login -> select/create -> enter game -> visible-world bootstrap
- Minimal runtime tests cover both pre-existing and newly-created selected characters

## Scope guardrails
- Do not add other entities yet
- Do not add item or NPC bursts yet
- Do not add CHARACTER_UPDATE yet unless strictly required
- Do not redesign world state yet
- Keep the slice deterministic and self-character only
