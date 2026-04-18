# First Chat Scope Hardening Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: replace the most unrealistic global actor-chat fanout behavior with the first real scope boundaries supported by the bootstrap runtime's current data model.

Architecture: keep the existing actor/system chat split, but route queued peer fanout through dedicated scope helpers: same empire for talking/shout, same non-zero `GuildID` for guild, and unchanged bootstrap-global behavior for party.

Tech stack: Go 1.26, existing `internal/minimal` shared-world registry and chat handler.

---

## Task 1: Add failing tests
Objective: prove actor chat is still leaking across global bootstrap scope.

Files:
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Add a failing negative test for talking chat across different empires.
2. Add a failing negative test for shout chat across different empires.
3. Update the positive guild test to use the same non-zero `GuildID`.
4. Add a failing negative test for guild chat across different guilds.

## Task 2: Add scope-aware enqueue helpers
Objective: express the new fanout boundaries in one place.

Files:
- Modify: `internal/minimal/shared_world.go`

Steps:
1. Add one helper for fanout to other sessions in the same empire.
2. Add one helper for fanout to other sessions with the same non-zero `GuildID`.
3. Leave the existing all-peers helper in place for party and non-chat flows.

## Task 3: Route actor chat by scope
Objective: make the minimal runtime use the new helpers.

Files:
- Modify: `internal/minimal/factory.go`

Steps:
1. Route talking fanout through the same-empire helper.
2. Keep party fanout on the existing all-peers helper.
3. Route guild fanout through the same-guild helper and reject `GuildID = 0`.
4. Route shout fanout through the same-empire helper.

## Task 4: Update docs
Objective: document the first non-global scope boundaries clearly.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/local-chat-peer-fanout.md`
- Modify: `spec/protocol/guild-chat-bootstrap.md`
- Modify: `spec/protocol/shout-chat-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/session-phases.md`
- Create: `spec/protocol/chat-scope-first-hardening.md`
- Create: `docs/plans/2026-04-18-chat-scope-hardening.md`

## Verification
- `go test ./internal/minimal -run 'TestNewGameSessionFactory(DoesNotQueueLocalChatAcrossEmpires|DoesNotQueueGuildChatAcrossDifferentGuilds|DoesNotQueueShoutAcrossEmpires|QueuesGuildChatForConnectedPeers|QueuesPeerChatForVisiblePlayers|QueuesShoutChatForConnectedPeers)' -count=1`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add map-index persistence yet.
- Do not add channel topology yet.
- Do not add sector/range visibility logic yet.
- Do not touch party membership in this slice.
