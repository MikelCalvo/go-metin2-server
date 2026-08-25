package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Live chase executor must honor the already-frozen leash-clamp clear rule:
// a complete step that stops on the effective leash boundary clears the pending
// chase deadline even when the owner was not reached; engagement stays owned;
// a later same-engagement accepted hit re-arms chase; after the owner walks
// inward so the actor is again safely inside leash, the re-armed due chase
// applies another retained-viewer MOVE.
func TestGameRuntimeFlushServerFramesClearsLeashClampedSpawnGroupChaseStepAndRearmsOnHit(t *testing.T) {
	const profile = "practice_chase_leash_clamp_rearm_wolf"
	const leashRadius int32 = 150

	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ChaseClampRearmOwner", 0x01030311, 0x02040311, 1940, 2800, 0, 140, 240)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "chase-clamp-rearm-owner", 0xc1c1c1c1, owner)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700004200, 0)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     800,
			VisibilitySectorSize: 200,
		},
		store,
		nil,
		staticActorStore,
		interactionStore,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:           "practice.chase_leash_clamp_rearm",
			Name:          "ChaseClampRearmMob",
			MapIndex:      42,
			X:             1700,
			Y:             2800,
			RaceNum:       20350,
			CombatProfile: profile,
		}},
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:        profile,
			MaxHP:          24,
			AttackValue:    8,
			DefenseValue:   2,
			RespawnDelayMs: 1500,
			AggroRadius:    120,
			LeashRadius:    leashRadius,
		}},
	}); err != nil {
		t.Fatalf("import chase leash-clamp re-arm spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.chase_leash_clamp_rearm")
	if !ok {
		t.Fatal("expected chase leash-clamp re-arm spawn group to resolve by ref")
	}
	if got := worldruntime.EffectiveStaticActorSpawnLeashRadius(profile); got != leashRadius {
		t.Fatalf("expected imported profile effective leash radius %d, got %d", leashRadius, got)
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "chase-clamp-rearm-owner", 0xc1c1c1c1)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected owner target error before chase leash-clamp arm: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected owner to select chase leash-clamp practice mob, got %d frames", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted hit before chase leash-clamp arm: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, immediate retaliation, and damage-info on first chase-arming hit, got %d frames", len(attackOut))
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engaged hit to arm a pending chase-step row, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, flow); len(queued) != 2 {
		t.Fatalf("expected the owned delayed retaliation beat to fire before the first chase step, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	firstQueued := flushServerFrames(t, flow)
	if len(firstQueued) == 0 {
		t.Fatal("expected first due chase-step to queue retained owner MOVE replication")
	}
	firstMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, firstQueued[0]))
	if err != nil {
		t.Fatalf("decode first chase-step MOVE: %v", err)
	}
	if firstMove.VID != targetVID || firstMove.X != 1800 || firstMove.Y != 2800 || firstMove.Duration == 0 {
		t.Fatalf("expected first chase-step MOVE at +100 toward owner, got %+v", firstMove)
	}
	firstStepped, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || firstStepped.X != 1800 || firstStepped.Y != 2800 || firstStepped.SpawnLeash == nil || firstStepped.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected first chase step to leave actor within_radius, ok=%v snapshot=%+v", ok, firstStepped)
	}
	runtime.spawnChaseMu.Lock()
	_, stillScheduledAfterFirst := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !stillScheduledAfterFirst {
		t.Fatal("expected still-engaged in-leash chase actor to re-arm after non-clamped first step")
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, flow); len(queued) != 2 {
		t.Fatalf("expected re-armed delayed retaliation before leash-clamped chase step, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	clampQueued := flushServerFrames(t, flow)
	if len(clampQueued) == 0 {
		t.Fatal("expected leash-clamped chase-step to queue retained owner MOVE replication")
	}
	clampMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, clampQueued[0]))
	if err != nil {
		t.Fatalf("decode leash-clamped chase-step MOVE: %v", err)
	}
	if clampMove.VID != targetVID || clampMove.X != 1850 || clampMove.Y != 2800 || clampMove.Duration == 0 {
		t.Fatalf("expected leash-clamped chase MOVE at farthest on-segment leash boundary, got %+v", clampMove)
	}
	for _, raw := range clampQueued[1:] {
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected leash-clamped chase MOVE not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
		if add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw)); err == nil && add.VID == targetVID {
			t.Fatalf("expected leash-clamped chase MOVE not to emit retained-viewer CHARACTER_ADD, got %+v", add)
		}
		if target, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, raw)); err == nil && target.TargetVID == 0 {
			t.Fatalf("expected leash-clamped chase step to preserve selected combat target, got clear frame %+v", target)
		}
	}

	clamped, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || clamped.X != 1850 || clamped.Y != 2800 || clamped.Dead || clamped.SpawnLeash == nil || clamped.SpawnLeash.ReturnRequired || clamped.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected runtime actor to stop on leash boundary while remaining within_radius, ok=%v snapshot=%+v", ok, clamped)
	}
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("ChaseClampRearmOwner")
	if !ok {
		t.Fatal("expected chase leash-clamp owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected leash-clamped chase step to preserve engagement ownership for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("ChaseClampRearmOwner"); !ok || snapshot.TargetVID != targetVID {
		t.Fatalf("expected leash-clamped chase step to preserve selected combat target, ok=%v snapshot=%+v", ok, snapshot)
	}

	runtime.spawnChaseMu.Lock()
	_, chaseScheduledAfterClamp := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduledAfterClamp {
		t.Fatalf("expected leash-clamped complete chase step to clear pending chase deadline for entity %d", group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); ok {
		t.Fatalf("expected read-only chase inspection to omit leash-clamped actor, got %+v", pending)
	}

	// Owner remains beyond leash; a due chase must not auto-fire while unscheduled.
	// Delayed retaliation may still beat while engagement is preserved — ignore those
	// frames, but reject any chase MOVE / position change / schedule re-arm.
	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay)
	idleQueued := flushServerFrames(t, flow)
	for _, raw := range idleQueued {
		if moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, raw)); err == nil && moveAck.VID == targetVID {
			t.Fatalf("expected no automatic chase MOVE while deadline stays cleared after clamp, got %+v among %d frames", moveAck, len(idleQueued))
		}
	}
	stillClamped, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || stillClamped.X != 1850 || stillClamped.Y != 2800 {
		t.Fatalf("expected actor to remain on leash boundary while chase stays cleared, ok=%v snapshot=%+v", ok, stillClamped)
	}
	runtime.spawnChaseMu.Lock()
	_, chaseScheduledDuringIdle := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduledDuringIdle {
		t.Fatalf("expected chase deadline to stay cleared during idle wait after leash clamp for entity %d", group.EntityID)
	}

	// Same-engagement accepted hit must re-arm chase without inventing a new owner.
	currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
	rearmOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected same-engagement hit while still on leash boundary: %v", err)
	}
	if len(rearmOut) == 0 {
		t.Fatal("expected same-engagement hit on leash-boundary actor to be accepted")
	}
	runtime.spawnChaseMu.Lock()
	rearmDueAt, rearmed := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !rearmed {
		t.Fatalf("expected same-engagement hit to re-arm chase deadline after leash clamp clear for entity %d", group.EntityID)
	}
	expectedRearmDueAt := currentTime.Add(bootstrapSpawnGroupChaseStepDelay)
	if !rearmDueAt.Equal(expectedRearmDueAt) {
		t.Fatalf("expected re-armed chase deadline at %s, got %s", expectedRearmDueAt, rearmDueAt)
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected re-arming hit to keep engagement ownership for entity %d", group.EntityID)
	}

	// Walk owner inward inside the effective leash so the next planned chase can
	// actually change position instead of clamping again on the same boundary.
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1830,
		Y:    2800,
		Time: 0xc2c3c4c5,
	}))); err != nil {
		t.Fatalf("unexpected owner move inward after chase re-arm: %v", err)
	}
	_ = flushServerFrames(t, flow)
	runtime.spawnChaseMu.Lock()
	_, stillArmedAfterOwnerMove := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !stillArmedAfterOwnerMove {
		t.Fatal("expected owner walk inward not to clear the already re-armed chase deadline")
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	_ = flushServerFrames(t, flow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	followQueued := flushServerFrames(t, flow)
	if len(followQueued) == 0 {
		t.Fatal("expected re-armed due chase-step to queue retained owner MOVE after owner walked inward")
	}
	var followMove movep.MoveAckPacket
	foundFollowMove := false
	for _, raw := range followQueued {
		moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, raw))
		if err != nil || moveAck.VID != targetVID {
			continue
		}
		followMove = moveAck
		foundFollowMove = true
		break
	}
	if !foundFollowMove {
		t.Fatalf("expected re-armed due chase-step to queue retained owner MOVE after owner walked inward, got %d frames", len(followQueued))
	}
	if followMove.X != 1830 || followMove.Y != 2800 || followMove.Duration == 0 {
		t.Fatalf("expected follow-up chase MOVE onto inward owner position, got %+v", followMove)
	}
	followed, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || followed.X != 1830 || followed.Y != 2800 || followed.Dead || followed.SpawnLeash == nil || followed.SpawnLeash.ReturnRequired {
		t.Fatalf("expected follow-up chase step to land on inward owner while remaining in leash, ok=%v snapshot=%+v", ok, followed)
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected follow-up chase step to keep engagement ownership for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("ChaseClampRearmOwner"); !ok || snapshot.TargetVID != targetVID {
		t.Fatalf("expected follow-up chase step to keep selected combat target, ok=%v snapshot=%+v", ok, snapshot)
	}
}
