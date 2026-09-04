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
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// Hit-armed / still-selected chase that already displaced a practice mob
// within_radius must clear the selected combat target, release engaged_by,
// clear pending chase, and arm homeward when the owner walks outside combat-
// target range (300) while remaining inside visibility. Contrast with
// TestGameRuntimeHitArmedSpawnGroupChaseSurvivesOwnerWalkOutsideAggroRadius,
// which keeps engagement + chase when the owner leaves aggro (200) but stays
// inside combat range. The training-dummy range-loss twin only asserts
// TARGET(0, 0) clear without chase/homeward.
func TestGameRuntimeCombatRangeLossClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Owner starts inside DefaultSpawnAggroRadius (200) so the accepted hit
	// arms chase from a normal in-aggro engagement. After the due chase beat
	// lands the mob at 1800 (within_radius), walking to 2150 leaves combat-
	// target range (350 from 1800) while staying inside visibility (800).
	owner := peerVisibilityCharacter("CombatRangeHomeOwner", 0x01030601, 0x02040601, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	// Watcher stays outside default aggro of both displaced 1800 and home 1700
	// so proximity cannot re-engage after range-loss clear, while still
	// remaining a retained visibility viewer for homeward MOVE.
	watcher := peerVisibilityCharacter("CombatRangeHomeWatcher", 0x01030602, 0x02040602, 1450, 2800, 0, 101, 201)
	watcher.MapIndex = 42
	watcher.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "combat-range-home-owner", 0x70707071, owner)
	issuePeerTicket(t, store, "combat-range-home-watcher", 0x70707072, watcher)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700005300, 0)
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
		t.Fatalf("unexpected game runtime error for combat-range-loss chase/homeward: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	ref := "practice.combat_range_loss_homeward_after_chase"
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           ref,
		Name:          "CombatRangeHomeMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import combat-range-loss chase/homeward spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef(ref)
	if !ok {
		t.Fatal("expected combat-range-loss chase/homeward spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "combat-range-home-owner", 0x70707071)
	defer closeSessionFlow(t, ownerFlow)
	flushServerFrames(t, ownerFlow)

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected owner target error before combat-range-loss chase arm: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected owner to select combat-range-loss practice mob, got %d frames", len(selectOut))
	}
	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted hit before combat-range-loss chase arm: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected target refresh, immediate retaliation, and damage-info on first chase-arming hit, got %d frames", len(attackOut))
	}
	runtime.spawnChaseMu.Lock()
	dueAt, chaseScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !chaseScheduled {
		t.Fatalf("expected accepted engagement hit to arm pending chase for entity %d", group.EntityID)
	}
	expectedChaseDueAt := currentTime.Add(bootstrapSpawnGroupChaseStepDelay)
	if !dueAt.Equal(expectedChaseDueAt) {
		t.Fatalf("expected chase deadline at %s, got %s", expectedChaseDueAt, dueAt)
	}

	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "combat-range-home-watcher", 0x70707072)
	defer closeSessionFlow(t, watcherFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, watcherFlow)

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("CombatRangeHomeOwner")
	if !ok {
		t.Fatal("expected combat-range-loss homeward owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected hit-armed engagement before chase displace for entity %d", group.EntityID)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation before combat-range-loss chase displace, got %d frames", len(queued))
	}
	_ = flushServerFrames(t, watcherFlow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, ownerFlow)
	if len(chaseQueued) == 0 {
		t.Fatal("expected due chase-step to displace actor toward owner before combat-range loss")
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil {
		t.Fatalf("decode chase displace MOVE before combat-range loss: %v", err)
	}
	if chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 {
		t.Fatalf("expected chase displace to +100 toward owner before combat-range loss, got %+v", chaseMove)
	}
	_ = flushServerFrames(t, watcherFlow)

	displaced, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || displaced.X != 1800 || displaced.Y != 2800 || displaced.SpawnLeash == nil || displaced.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected chase-displaced within_radius actor before combat-range loss, ok=%v snapshot=%+v", ok, displaced)
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected hit-armed engagement to survive chase displace for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("CombatRangeHomeOwner"); !ok || snapshot.TargetVID != targetVID {
		t.Fatalf("expected selected combat target to survive chase displace, ok=%v snapshot=%+v", ok, snapshot)
	}

	// Leave combat-target range of displaced 1800 (walk to 2150 => dist 350)
	// while staying inside visibility 800 so the clear is combat-range loss,
	// not AOI delete.
	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    2150,
		Y:    2800,
		Time: 0x71727374,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while leaving combat range after chase displace: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack after combat-range loss, got %d frames", len(moveOut))
	}
	clearedFrames := flushServerFrames(t, ownerFlow)
	if len(clearedFrames) != 1 {
		t.Fatalf("expected 1 queued TARGET(0,0) clear after combat-range loss, got %d frames", len(clearedFrames))
	}
	cleared, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, clearedFrames[0]))
	if err != nil {
		t.Fatalf("decode TARGET clear after combat-range loss: %v", err)
	}
	if cleared.TargetVID != 0 || cleared.HPPercent != 0 {
		t.Fatalf("expected zero-target clear after combat-range loss, got %+v", cleared)
	}
	_ = flushServerFrames(t, watcherFlow)

	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected combat-range loss to release engaged_by for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("CombatRangeHomeOwner"); ok {
		t.Fatalf("expected combat-range loss to clear selected combat target, got %+v", snapshot)
	}
	runtime.spawnChaseMu.Lock()
	_, chaseScheduledAfterClear := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduledAfterClear {
		t.Fatalf("expected combat-range loss to clear pending chase deadline for entity %d", group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase-step inspection to omit actor after combat-range loss, ok=%v snapshot=%+v", ok, pending)
	}

	runtime.spawnHomewardMu.Lock()
	homewardDueAt, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardScheduled {
		t.Fatalf("expected combat-range loss on within_radius displace to arm homeward deadline for entity %d", group.EntityID)
	}
	expectedHomewardDueAt := currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	if !homewardDueAt.Equal(expectedHomewardDueAt) {
		t.Fatalf("expected combat-range-loss homeward deadline at %s, got %s", expectedHomewardDueAt, homewardDueAt)
	}

	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected no homeward MOVE on watcher before the 1s deadline after combat-range loss, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	homewardQueued := flushServerFrames(t, watcherFlow)
	if len(homewardQueued) == 0 {
		t.Fatal("expected due homeward-step after combat-range loss to queue retained watcher MOVE toward home")
	}
	homewardMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, homewardQueued[0]))
	if err != nil {
		t.Fatalf("expected retained watcher after combat-range loss to receive MOVE replication, first frame decode err=%v", err)
	}
	if homewardMove.VID != targetVID || homewardMove.X != 1700 || homewardMove.Y != 2800 {
		t.Fatalf("expected homeward MOVE to authored home after combat-range loss, got %+v", homewardMove)
	}
	returned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || returned.X != 1700 || returned.Y != 2800 || returned.Dead || returned.SpawnLeash == nil || returned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected homeward-step after combat-range loss to restore at_home leash state, ok=%v snapshot=%+v", ok, returned)
	}
	runtime.spawnHomewardMu.Lock()
	_, stillHomeward := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if stillHomeward {
		t.Fatalf("expected completed at-home homeward-step after combat-range loss to clear pending deadline for entity %d", group.EntityID)
	}
}
