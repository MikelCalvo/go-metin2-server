# Restart ingress evidence note — slash-command path

This note records the first clean-room-safe public evidence artifact for the current player-restart ingress question.

## Question

For the TMP4-era compatibility target of this repository, do we need to own a separate dedicated non-chat restart request packet, or should we keep owning restart intent through in-game slash commands?

## Public artifacts inspected

### 1. Public client source showing slash gameplay commands ride the chat send path

The inspected public client tree `NakiuS/Metin2Client` shows:
- `source/UserInterface/PythonNetworkStream.cpp` routes several gameplay commands through `SendChatPacket(...)`, including `/quit`, `/phase_select`, and `/logout`.
- `source/UserInterface/PythonNetworkStreamPhaseGame.cpp` implements `SendChatPacket(...)` through the client chat request family.
- `source/UserInterface/Packet.h` defines the client chat request header (`HEADER_CG_CHAT = 3`).

Taken together, those files prove an important compatibility fact for this repo: at least some gameplay slash commands in the inspected TMP4-era-style client path are already sent as ordinary client-chat requests rather than as a separate dedicated command packet family.

### 2. Negative evidence from the same inspected public client tree

Searches of that same public client tree did **not** find evidence for:
- a dedicated `HEADER_CG_RESTART`-style client request header
- a dedicated restart or respawn request packet name in the inspected user-interface packet declarations
- literal `/restart_here` or `/restart_town` strings in the inspected client source tree

That absence does not prove such a packet can never exist in every legacy branch, but it does mean this repository should not guess a dedicated restart codec from folklore alone.

### 3. Secondary public UI evidence for restart buttons using chat commands

A second public artifact, `Engin622/Metin2-Pack-Agent-Skills/root/uirestart/skills.md`, documents the restart UI actions as sending:
- `/restart_here`
- `/restart_town`

through `net.SendChatPacket(...)`.

This is weaker than the inspected public client tree above, but it supports the same practical conclusion: restart intent can plausibly travel through the existing slash-chat ingress instead of a separate dedicated restart packet family.

## Repo-facing conclusion

Current clean-room-safe public evidence supports the repository's existing slash-command-backed restart ingress:
- keep `/restart_here` and `/restart_town` as the owned restart intents
- do **not** add a guessed dedicated restart request packet or packet-matrix row
- treat any future non-chat restart ingress as capture- or fixture-gated follow-up work only

## Confidence and limits

This is a strong enough conclusion for the current bootstrap scope, but it is still intentionally scoped:
- it supports the repo's current owned behavior for the target compatibility track
- it does **not** claim that every historical Metin2 client build used the exact same restart UI plumbing
- a future packet capture or owned fixture may still prove an additional dedicated ingress for some other client branch

Until that stronger contrary evidence exists, slash-command-backed restart ingress is the most honest owned contract for this repository.
