# Map-Transfer Bootstrap Self-Session Contract

> Historical note: this document has been superseded by `transfer-rebootstrap-burst.md`.

The project originally froze a deliberately temporary self-session transfer behavior here.
That older slice described a contract based on queued self visibility-delta frames only.

The current owned contract now lives in:
- `transfer-rebootstrap-burst.md`

Use that newer document for the live self-session behavior after a successful bootstrap transfer.
It freezes:
- no immediate self `MOVE_ACK`
- no immediate self `SYNC_POSITION_ACK`
- immediate relocated self bootstrap on the same game socket
- trailing peer visibility deltas after that self burst

This file remains only as a compatibility pointer for older roadmap and commit references.
