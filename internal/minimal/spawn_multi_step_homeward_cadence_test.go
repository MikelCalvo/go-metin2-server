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

// Live homeward executor must re-arm while the actor remains within_radius after
// a chase displace larger than max_step=100. First due homeward steps +100 and
// re-arms; second due lands on authored home and clears the deadline.
func TestGameRuntimeFlushServerFramesAppliesMultiStepSpawnGroupHomewardCadenceAfterChaseDisplaceBeyondMaxStep(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Owner at +200 from authored home so two chase beats leave the actor at
	// 1900 (displace > max_step=100) before engagement release arms homeward.
	owner := peerVisibilityCharacter("MultiHomewardOwner", 0x01030331, 0x02040331, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "multi-homeward-owner", 0xe1e1e1e1, owner)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700004400, 0)
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

	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.multi_step_homeward_after_chase",
		Name:          "MultiHomewardMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import multi-step homeward spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.multi_step_homeward_after_chase")
	if !ok {
		t.Fatal("expected multi-step homeward spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "multi-homeward-owner", 0xe1e1e1e1)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected owner target error before multi-step homeward chase displace: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	}))); err != nil {
		t.Fatalf("unexpected accepted hit before multi-step homeward chase displace: %v", err)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engaged hit to arm chase before multi-step homeward displace, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, flow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation before first chase displace, got %d frames", len(queued))
	}
	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	firstChaseQueued := flushServerFrames(t, flow)
	if len(firstChaseQueued) == 0 {
		t.Fatal("expected first due chase-step to displace actor toward owner")
	}
	firstChaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, firstChaseQueued[0]))
	if err != nil {
		t.Fatalf("decode first chase displace MOVE: %v", err)
	}
	if firstChaseMove.VID != targetVID || firstChaseMove.X != 1800 || firstChaseMove.Y != 2800 {
		t.Fatalf("expected first chase displace to +100 toward owner, got %+v", firstChaseMove)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, flow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation before second chase displace, got %d frames", len(queued))
	}
	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	secondChaseQueued := flushServerFrames(t, flow)
	if len(secondChaseQueued) == 0 {
		t.Fatal("expected second due chase-step to land on owner beyond one max_step from home")
	}
	secondChaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, secondChaseQueued[0]))
	if err != nil {
		t.Fatalf("decode second chase displace MOVE: %v", err)
	}
	if secondChaseMove.VID != targetVID || secondChaseMove.X != 1900 || secondChaseMove.Y != 2800 {
		t.Fatalf("expected second chase displace onto owner at +200 from home, got %+v", secondChaseMove)
	}
	displaced, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || displaced.X != 1900 || displaced.Y != 2800 || displaced.SpawnLeash == nil || displaced.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected chase-displaced within_radius actor beyond max_step before homeward arm, ok=%v snapshot=%+v", ok, displaced)
	}

	// Leave aggro of the displaced / mid / home coordinates before TARGET(0) so
	// proximity cannot re-lock and clear homeward mid-cadence. Wider visibility
	// (800) keeps the owner a retained viewer for both homeward beats.
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    2400,
		Y:    2800,
		Time: 0xe2e3e4e5,
	}))); err != nil {
		t.Fatalf("unexpected owner move outside aggro before multi-step homeward release: %v", err)
	}
	_ = flushServerFrames(t, flow)

	clearOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: 0})))
	if err != nil {
		t.Fatalf("unexpected owner TARGET(0) clear before multi-step homeward arm: %v", err)
	}
	if len(clearOut) != 0 {
		t.Fatalf("expected TARGET(0) clear to emit no frames, got %d", len(clearOut))
	}

	runtime.spawnChaseMu.Lock()
	_, chaseScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduled {
		t.Fatalf("expected TARGET(0) to clear chase before multi-step homeward arm for entity %d", group.EntityID)
	}
	runtime.spawnHomewardMu.Lock()
	homewardDueAt, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardScheduled {
		t.Fatalf("expected engagement release on beyond-max_step within_radius displace to arm homeward for entity %d", group.EntityID)
	}
	expectedHomewardDueAt := currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	if !homewardDueAt.Equal(expectedHomewardDueAt) {
		t.Fatalf("expected homeward deadline at %s, got %s", expectedHomewardDueAt, homewardDueAt)
	}
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("MultiHomewardOwner")
	if !ok {
		t.Fatal("expected multi-step homeward owner entity to remain registered")
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected homeward arming path to leave entity %d unengaged", group.EntityID)
	}

	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no homeward MOVE before the 1s deadline, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	firstHomewardQueued := flushServerFrames(t, flow)
	if len(firstHomewardQueued) == 0 {
		t.Fatal("expected first due homeward-step to queue retained owner MOVE toward home")
	}
	firstHomewardMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, firstHomewardQueued[0]))
	if err != nil {
		t.Fatalf("decode first homeward MOVE: %v", err)
	}
	if firstHomewardMove.VID != targetVID || firstHomewardMove.X != 1800 || firstHomewardMove.Y != 2800 || firstHomewardMove.Duration == 0 {
		t.Fatalf("expected first homeward MOVE +100 toward home, got %+v", firstHomewardMove)
	}
	for _, raw := range firstHomewardQueued[1:] {
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected first homeward MOVE not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
		if add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw)); err == nil && add.VID == targetVID {
			t.Fatalf("expected first homeward MOVE not to emit retained-viewer CHARACTER_ADD, got %+v", add)
		}
	}
	mid, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || mid.X != 1800 || mid.Y != 2800 || mid.SpawnLeash == nil || mid.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected first homeward step to leave actor still within_radius, ok=%v snapshot=%+v", ok, mid)
	}
	runtime.spawnHomewardMu.Lock()
	rearmDueAt, stillHomeward := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !stillHomeward {
		t.Fatalf("expected incomplete first homeward step to re-arm pending deadline for entity %d", group.EntityID)
	}
	expectedRearmDueAt := currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	if !rearmDueAt.Equal(expectedRearmDueAt) {
		t.Fatalf("expected re-armed homeward deadline at %s, got %s", expectedRearmDueAt, rearmDueAt)
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	secondHomewardQueued := flushServerFrames(t, flow)
	if len(secondHomewardQueued) == 0 {
		t.Fatal("expected second due homeward-step to queue retained owner MOVE onto authored home")
	}
	var secondHomewardMove movep.MoveAckPacket
	foundSecondHomewardMove := false
	for _, raw := range secondHomewardQueued {
		moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, raw))
		if err != nil || moveAck.VID != targetVID {
			continue
		}
		secondHomewardMove = moveAck
		foundSecondHomewardMove = true
		break
	}
	if !foundSecondHomewardMove {
		t.Fatalf("expected second due homeward-step to queue retained owner MOVE onto authored home, got %d frames", len(secondHomewardQueued))
	}
	if secondHomewardMove.X != 1700 || secondHomewardMove.Y != 2800 || secondHomewardMove.Duration == 0 {
		t.Fatalf("expected second homeward MOVE onto authored home, got %+v", secondHomewardMove)
	}
	returned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || returned.X != 1700 || returned.Y != 2800 || returned.Dead || returned.SpawnLeash == nil || returned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected second homeward step to restore at_home, ok=%v snapshot=%+v", ok, returned)
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected multi-step homeward to keep entity %d unengaged", group.EntityID)
	}
	runtime.spawnHomewardMu.Lock()
	_, stillHomewardAfterComplete := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if stillHomewardAfterComplete {
		t.Fatalf("expected completed at-home homeward cadence to clear pending deadline for entity %d", group.EntityID)
	}
}
