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

// Hit-armed / still-selected chase that already displaced a practice mob
// within_radius must clear the selected combat target with CHARACTER_DEL plus
// TARGET(0, 0), release engaged_by, clear pending chase, and arm homeward when
// the owner walks outside visibility of the displaced actor. Contrast with
// TestGameRuntimeCombatRangeLossClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace,
// which clears by combat-range loss while remaining inside visibility. The
// training-dummy visibility twin only asserts CHARACTER_DEL + TARGET(0, 0)
// without chase/homeward.
func TestGameRuntimeVisibilityLossClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Owner starts inside DefaultSpawnAggroRadius (200) so the accepted hit
	// arms chase from a normal in-aggro engagement. After the due chase beat
	// lands the mob at 1800 (within_radius), walking to 2650 leaves visibility
	// of the displaced actor (850 from 1800 with VisibilityRadius 800).
	owner := peerVisibilityCharacter("VisLossHomeOwner", 0x01030701, 0x02040701, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	// Watcher sits east of the displaced actor so it stays outside default aggro
	// of both displaced 1800 and home 1700 (no proximity re-engage), remains a
	// retained visibility viewer for homeward MOVE, and also stays inside the
	// owner's visibility after the owner walks to 2650 (so the clear burst is
	// only mob CHARACTER_DEL + TARGET(0,0), not a peer leave).
	watcher := peerVisibilityCharacter("VisLossHomeWatcher", 0x01030702, 0x02040702, 2100, 2800, 0, 101, 201)
	watcher.MapIndex = 42
	watcher.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "visibility-loss-home-owner", 0x80808081, owner)
	issuePeerTicket(t, store, "visibility-loss-home-watcher", 0x80808082, watcher)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700005400, 0)
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
		t.Fatalf("unexpected game runtime error for visibility-loss chase/homeward: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }

	ref := "practice.visibility_loss_homeward_after_chase"
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           ref,
		Name:          "VisLossHomeMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import visibility-loss chase/homeward spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef(ref)
	if !ok {
		t.Fatal("expected visibility-loss chase/homeward spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "visibility-loss-home-owner", 0x80808081)
	defer closeSessionFlow(t, ownerFlow)
	flushServerFrames(t, ownerFlow)

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected owner target error before visibility-loss chase arm: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected owner to select visibility-loss practice mob, got %d frames", len(selectOut))
	}
	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted hit before visibility-loss chase arm: %v", err)
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

	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "visibility-loss-home-watcher", 0x80808082)
	defer closeSessionFlow(t, watcherFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, watcherFlow)

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("VisLossHomeOwner")
	if !ok {
		t.Fatal("expected visibility-loss homeward owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected hit-armed engagement before chase displace for entity %d", group.EntityID)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation before visibility-loss chase displace, got %d frames", len(queued))
	}
	_ = flushServerFrames(t, watcherFlow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, ownerFlow)
	if len(chaseQueued) == 0 {
		t.Fatal("expected due chase-step to displace actor toward owner before visibility loss")
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil {
		t.Fatalf("decode chase displace MOVE before visibility loss: %v", err)
	}
	if chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 {
		t.Fatalf("expected chase displace to +100 toward owner before visibility loss, got %+v", chaseMove)
	}
	_ = flushServerFrames(t, watcherFlow)

	displaced, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || displaced.X != 1800 || displaced.Y != 2800 || displaced.SpawnLeash == nil || displaced.SpawnLeash.Status != worldruntime.SpawnLeashStatusWithinRadius {
		t.Fatalf("expected chase-displaced within_radius actor before visibility loss, ok=%v snapshot=%+v", ok, displaced)
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected hit-armed engagement to survive chase displace for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("VisLossHomeOwner"); !ok || snapshot.TargetVID != targetVID {
		t.Fatalf("expected selected combat target to survive chase displace, ok=%v snapshot=%+v", ok, snapshot)
	}

	// Leave visibility of displaced 1800 (walk to 2650 => dist 850) so the
	// clear is AOI / CHARACTER_DEL + TARGET(0, 0), not combat-range-only loss.
	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    2650,
		Y:    2800,
		Time: 0x81828384,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while leaving visibility after chase displace: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack after visibility loss, got %d frames", len(moveOut))
	}
	clearedFrames := flushServerFrames(t, ownerFlow)
	if len(clearedFrames) != 2 {
		t.Fatalf("expected CHARACTER_DEL plus TARGET(0,0) clear after visibility loss, got %d frames", len(clearedFrames))
	}
	deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, clearedFrames[0]))
	if err != nil {
		t.Fatalf("decode CHARACTER_DEL after visibility loss: %v", err)
	}
	if deleted.VID != targetVID {
		t.Fatalf("expected CHARACTER_DEL for chase-displaced practice mob after visibility loss, got %+v", deleted)
	}
	cleared, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, clearedFrames[1]))
	if err != nil {
		t.Fatalf("decode TARGET clear after visibility loss: %v", err)
	}
	if cleared.TargetVID != 0 || cleared.HPPercent != 0 {
		t.Fatalf("expected zero-target clear after visibility loss, got %+v", cleared)
	}
	_ = flushServerFrames(t, watcherFlow)

	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected visibility loss to release engaged_by for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("VisLossHomeOwner"); ok {
		t.Fatalf("expected visibility loss to clear selected combat target, got %+v", snapshot)
	}
	runtime.spawnChaseMu.Lock()
	_, chaseScheduledAfterClear := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if chaseScheduledAfterClear {
		t.Fatalf("expected visibility loss to clear pending chase deadline for entity %d", group.EntityID)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); ok || pending.EntityID != 0 {
		t.Fatalf("expected chase-step inspection to omit actor after visibility loss, ok=%v snapshot=%+v", ok, pending)
	}

	runtime.spawnHomewardMu.Lock()
	homewardDueAt, homewardScheduled := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if !homewardScheduled {
		t.Fatalf("expected visibility loss on within_radius displace to arm homeward deadline for entity %d", group.EntityID)
	}
	expectedHomewardDueAt := currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	if !homewardDueAt.Equal(expectedHomewardDueAt) {
		t.Fatalf("expected visibility-loss homeward deadline at %s, got %s", expectedHomewardDueAt, homewardDueAt)
	}

	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected no homeward MOVE on watcher before the 1s deadline after visibility loss, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	homewardQueued := flushServerFrames(t, watcherFlow)
	if len(homewardQueued) == 0 {
		t.Fatal("expected due homeward-step after visibility loss to queue retained watcher MOVE toward home")
	}
	homewardMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, homewardQueued[0]))
	if err != nil {
		t.Fatalf("expected retained watcher after visibility loss to receive MOVE replication, first frame decode err=%v", err)
	}
	if homewardMove.VID != targetVID || homewardMove.X != 1700 || homewardMove.Y != 2800 {
		t.Fatalf("expected homeward MOVE to authored home after visibility loss, got %+v", homewardMove)
	}
	returned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || returned.X != 1700 || returned.Y != 2800 || returned.Dead || returned.SpawnLeash == nil || returned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected homeward-step after visibility loss to restore at_home leash state, ok=%v snapshot=%+v", ok, returned)
	}
	runtime.spawnHomewardMu.Lock()
	_, stillHomeward := runtime.spawnHomewardStepDueAt[group.EntityID]
	runtime.spawnHomewardMu.Unlock()
	if stillHomeward {
		t.Fatalf("expected completed at-home homeward-step after visibility loss to clear pending deadline for entity %d", group.EntityID)
	}
}
