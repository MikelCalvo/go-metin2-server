# Real-client PvE smoke run

Use this short template to confirm one supported end-to-end PvE journey against the exact Go build under test. It complements—not replaces—the [full manual client QA checklist](manual-client-checklist.md); run the full checklist for releases, regressions outside this path, or unexplained failures.

## Run record

- Date/time and tester:
- Server commit/build and client build/hash:
- Target auth/game endpoints:
- QA account alias and disposable character (never credentials):
- Result: PASS / FAIL / BLOCKED
- Logs, screenshots, or packet captures:
- Issue links and next action:

## Steps

1. Record the run data above and confirm the client targets the Go server, not a legacy server.
2. Confirm `authd` and `gamed` are healthy and capture fresh logs.
3. Open the client; verify the intended channel is visible and online.
4. Attempt one known-bad login; verify a clean rejection without a client crash or server failure.
5. Sign in with the designated QA account and reach character selection.
6. Select the disposable QA character (or create one using the full checklist's safe procedure).
7. Enter the supported test map and verify the character is controllable.
8. Confirm a supported mob is visible; engage it and observe a valid combat exchange.
9. Defeat the mob and verify death/respawn behaviour plus the expected reward or loot.
10. Pick up or otherwise receive the reward; verify the expected inventory/character-state change.
11. Use or equip a supported reward where applicable, then interact with the supported NPC service.
12. Disconnect and reconnect (or use the approved restart scenario); verify the intended progress remains.
13. Record pass/fail evidence, logs, and any expected-versus-actual difference; stop on a blocker and follow the full checklist for diagnosis.

This smoke run makes no claim of full legacy parity. Do not classify deferred quests, skills, social systems, broad content, or production-scale behaviour as failures unless the ticket explicitly covers them.
