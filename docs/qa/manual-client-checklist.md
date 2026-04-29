# Manual Client QA Checklist

This checklist is the reusable manual QA gate for real-client validation against the current Go server.

Use it to:
- validate milestone progress with a real client, not only automated tests
- keep a stable regression checklist across slices
- record what the client can really do today without mixing in legacy-server expectations

This checklist is intentionally conservative.
It focuses on the current bootstrap scope and avoids treating clearly out-of-scope systems as regressions.

## How to use this document

For each manual run, create a short run note and fill the checklist against the exact build under test.

Suggested run-note template:

```md
## Manual QA Run
- Date/time:
- Tester:
- Server commit/build:
- Client build/hash:
- Target IP:
- Target auth port:
- Target game port:
- Legacy server also running: yes/no
- Result: PASS / PASS WITH ISSUES / FAIL
- Blocking issues:
- Non-blocking issues:
- Logs captured:
- Next action:
```

## Current bootstrap assumptions

Adapt these to the deployment under test:

- auth port: use the configured `authd` legacy port
- game port: use the configured `gamed` legacy port
- if the default minimal runtime is used, the current stub credentials are:
  - login: `mkmk`
  - password: `hunter2`

Important:
- if both the legacy server and the Go server are running, confirm the client is really pointing at the Go server before interpreting results
- if the channel does not appear online, stop and debug publication/firewall/target config first

---

## 0. Test run header

Fill this before starting:

- [ ] Date/time recorded
- [ ] Tester recorded
- [ ] Client build/hash recorded
- [ ] Server commit/build recorded
- [ ] Target IP recorded
- [ ] Target auth port recorded
- [ ] Target game port recorded
- [ ] It is clear whether the legacy server is also running
- [ ] A run note exists for this session

---

## 1. Preflight — safe, non-destructive

### 1.1 Service health

- [ ] `authd` is running
- [ ] `gamed` is running
- [ ] Both expected listen ports are open
- [ ] Recent logs show no fresh fatal startup failure

Expected result:
- the server is stably up before opening the client

### 1.2 Target sanity

- [ ] The client is pointing to the Go auth endpoint, not the legacy auth endpoint
- [ ] The advertised/public IP is reachable from the client machine
- [ ] There is no ambiguity about which server the client is hitting

Expected result:
- a failed client path can be interpreted as a server issue, not a targeting mistake

### 1.3 Channel visibility smoke test

- [ ] Open the client and reach the server/channel list
- [ ] Confirm the target channel appears online/normal

Expected result:
- at least one bootstrap channel is visible as online/normal

If this fails, stop the rest of the checklist and record:
- target client config
- current server publication/firewall state
- recent `authd` and `gamed` logs

---

## 2. Single-client login and selection

### 2.1 Bad credentials path

- [ ] Attempt login with a known bad password

Expected result:
- login is rejected cleanly
- the client does not hang or crash
- the server remains alive

### 2.2 Valid credentials path

- [ ] Login with the configured valid QA credentials

Expected result:
- login succeeds
- the client reaches the character selection surface
- there is no disconnect between auth and selection

### 2.3 Empty-account / empire-selection path

Run this only if the QA account is empty.

- [ ] Confirm empire selection appears when expected
- [ ] Choose an empire once
- [ ] Verify the session remains usable after empire selection

Expected result:
- empire selection is accepted
- the client returns to a valid selection/create state

### 2.4 Character list rendering

- [ ] Existing characters appear on the selection screen
- [ ] Character names render correctly
- [ ] Character slots do not show obvious corruption

Expected result:
- the selection surface is usable enough for continued testing

---

## 3. Character creation / deletion

Use dedicated QA names to avoid confusion.
A prefix like `QA_` is recommended.

### 3.1 Create character

- [ ] Create a new character in an empty slot
- [ ] Use a dedicated QA name
- [ ] Verify the new character appears in the selection screen immediately

Expected result:
- create succeeds cleanly
- the new character is visible without restarting the client

### 3.2 Invalid / duplicate create guard

- [ ] Attempt one clearly invalid or duplicate create case

Expected result:
- the client receives a clean failure path
- the session remains usable afterward
- no forced disconnect occurs

### 3.3 Delete character

Run this only on a disposable QA character.

- [ ] Delete the disposable QA character
- [ ] Confirm the slot updates correctly in the selection screen

Expected result:
- delete succeeds cleanly
- the deleted character disappears from the selection surface
- no selection-state desync occurs

---

## 4. World entry

### 4.1 Select character

- [ ] Select a valid character

Expected result:
- the client leaves selection cleanly
- the loading phase is stable

### 4.2 Enter game

- [ ] Complete the enter-game flow
- [ ] Wait until the character appears in-world

Expected result:
- the character spawns in-world
- there is no immediate disconnect
- there is no client crash
- there is no server crash

### 4.3 Stability after entry

- [ ] Stay idle for 15 seconds after spawn
- [ ] Perform only minor input such as camera rotation

Expected result:
- the session remains stable
- there is no delayed kick immediately after entry

---

## 5. Single-client movement

### 5.1 Basic movement

- [ ] Walk a short distance
- [ ] Walk again in a different direction
- [ ] Stop moving and wait 5 seconds

Expected result:
- movement works
- the client remains connected
- there is no severe rubber-band that blocks testing

### 5.2 Repeat movement after idle

- [ ] Wait 15 seconds
- [ ] Move again

Expected result:
- movement still works after idle
- there is no silent session death

### 5.3 Reconnect persistence smoke test

- [ ] Exit the client cleanly
- [ ] Reopen the client
- [ ] Login and re-enter with the same character

Expected result:
- the character still exists
- login, selection, and enter-game still work after reconnect

### 5.4 Bootstrap NPC interaction smoke

Run this only when the target build has authored QA NPC content loaded nearby.

If the lab currently has no such content, either:
- import/adapt `docs/examples/bootstrap-npc-service-bundle.json` through `/local/content-bundle`, or
- record this subsection as **N/A** instead of treating the absence of authored NPCs as a gameplay regression.

#### 5.4.1 Talk / info / shop-preview interactions

- [ ] Approach a visible authored QA NPC with `info`, `talk`, or `shop_preview`
- [ ] Interact once and wait for the self-only response
- [ ] Re-interact immediately once to confirm repeated spam is suppressed or remains stable within the current cooldown contract

Expected result:
- `info` and `talk` still return deterministic self-only text
- `shop_preview` returns deterministic structured browse-only self-only preview text
- no inventory mutation, buy/sell flow, or quest state appears
- repeated interaction does not disconnect the client

Important note:
- this smoke step does **not** require a real merchant window or successful purchase path yet
- the buy-only merchant transaction contract is now documented, but final packet choreography and implementation are still ahead

#### 5.4.2 Warp interaction

- [ ] Approach a visible authored QA warp NPC
- [ ] Interact once
- [ ] Confirm any authored informational text appears first if configured
- [ ] Confirm the client re-enters the world at the authored destination and remains connected

Expected result:
- the warp actor relocates the character through the current transfer/rebootstrap flow
- the client remains stable after the warp
- no merchant window, quest window, or inventory mutation appears

---

## 6. Two-client shared-world checks

Run this only when two real clients are available.
Prefer two disposable QA characters.

### 6.1 Dual login

- [ ] Connect client A
- [ ] Connect client B
- [ ] Enter the world on both

Expected result:
- both sessions stay connected
- one client does not kick the other during entry

### 6.2 Peer visibility

- [ ] Put both characters in the same bootstrap map
- [ ] Confirm A can see B
- [ ] Confirm B can see A

Expected result:
- mutual visibility works
- appearance/disappearance is sane enough for the current bootstrap scope

### 6.3 Peer movement replication

- [ ] Move character A while watching from B
- [ ] Move character B while watching from A

Expected result:
- movement replicates between visible peers
- there is no obvious one-way visibility bug

### 6.4 Local talking chat

- [ ] Send a normal local chat message from A
- [ ] Confirm B receives it
- [ ] Send one from B
- [ ] Confirm A receives it

Expected result:
- local chat works between visible peers in the same bootstrap scope

### 6.5 Whisper by exact name

- [ ] Whisper from A to B by exact character name
- [ ] Confirm B receives it
- [ ] Whisper to a non-existing name

Expected result:
- exact-name whisper delivery works
- an unknown target returns a clean not-exist behavior to the sender

### 6.6 Disconnect cleanup

- [ ] Close client B cleanly while A stays in-world

Expected result:
- A does not crash
- B disappears from A cleanly within the current bootstrap behavior

---

## 7. Optional bootstrap chat-scope checks

These checks are useful but secondary.
Do not block a general smoke pass on them unless the milestone specifically targets chat behavior.

### 7.1 Party chat bootstrap

- [ ] Send a party-type message if the current client path allows it

Expected result:
- the current bootstrap fanout behaves consistently with implementation and does not destabilize the session

### 7.2 Shout bootstrap

- [ ] Send a shout if the current client path allows it

Expected result:
- the current shout bootstrap behavior works without disconnecting the client

Note:
- current party/guild/shout behavior is still bootstrap-only and not backed by full gameplay systems

---

## 8. Regression watchlist

Record any of these immediately if seen:

- [ ] Channel list missing or wrong online state
- [ ] Login succeeds but selection screen fails
- [ ] Character create/delete desyncs the selection screen
- [ ] Enter-game disconnect
- [ ] Spawn succeeds but the first movement disconnects the session
- [ ] Two clients cannot coexist in-world
- [ ] Players do not see each other on the same bootstrap map
- [ ] Peer movement does not replicate
- [ ] Local chat crashes or disconnects a client
- [ ] Whisper exact-name routing is broken
- [ ] Reconnect loses the QA character unexpectedly
- [ ] Server logs show panic, fatal errors, or a restart loop

When a regression appears, record:
- exact checklist step number
- character names used
- whether the legacy server was also running
- recent `authd` log lines
- recent `gamed` log lines

---

## 9. Do NOT treat these as failures yet

These are currently out of scope for the present server state unless the milestone explicitly says otherwise:

- [ ] inventory UX completeness
- [ ] equipment logic
- [ ] item use
- [ ] full merchant UI semantics or real shop buy/sell flow
- [ ] inventory or currency mutation from NPC interactions
- [ ] mobs, combat, or skills
- [ ] quest acceptance, progression, or rewards
- [ ] death / respawn loop
- [ ] multi-channel real behavior
- [ ] polished client-facing warp/loading choreography

Important note:
- the project has operator-side transfer primitives and ongoing runtime transfer work
- for current QA, validate only the existing bootstrap warp/rebootstrap path; polished final warp/loading choreography is still not a general pass/fail gate

---

## 10. Exit criteria for a healthy current build

A current build is a good candidate when all of these pass:

- [ ] channel visible online
- [ ] valid login works
- [ ] selection screen is usable
- [ ] create/select/enter-game work
- [ ] single-client movement works
- [ ] reconnect works
- [ ] when authored QA NPC content is loaded, supported NPC smoke checks (`info` / `talk`, `shop_preview`, `warp`) pass without disconnecting the client
- [ ] with two clients: peer visibility works
- [ ] with two clients: peer movement works
- [ ] with two clients: local chat and whisper work
- [ ] no crash or forced disconnect occurs during the run
