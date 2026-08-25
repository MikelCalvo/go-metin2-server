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

// Live chase executor must plan from the engaged owner's live coordinates at
// flush time when the owner moves between arm and the first due beat.
//
// Ordinary GREEN twin (not dishonest RED): stepSpawnGroupChase already resolves
// ownerPos from the live engaged owner at flush.
func TestGameRuntimeFlushServerFramesReplansSpawnGroupChaseTowardOwnerMovedBetweenArmAndDue(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Start at +100 from authored home so the arm-time owner snapshot would only
	// plan one step to 1800; after arm the owner walks to +200 so the due flush
	// must replan onto the live post-move coords instead.
	owner := peerVisibilityCharacter("ChaseReplanOwner", 0x01030341, 0x02040341, 1800, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "chase-replan-owner", 0xf1f1f1f1, owner)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700004500, 0)
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
		Ref:           "practice.chase_replan_owner_moved",
		Name:          "ChaseReplanMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import chase-replan spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.chase_replan_owner_moved")
	if !ok {
		t.Fatal("expected chase-replan spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "chase-replan-owner", 0xf1f1f1f1)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected owner target error before chase replan arm: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	}))); err != nil {
		t.Fatalf("unexpected accepted hit before chase replan arm: %v", err)
	}
	runtime.spawnChaseMu.Lock()
	armDueAt, chaseArmed := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !chaseArmed {
		t.Fatalf("expected engaged hit to arm chase before owner move, entity=%d", group.EntityID)
	}
	expectedArmDueAt := currentTime.Add(bootstrapSpawnGroupChaseStepDelay)
	if !armDueAt.Equal(expectedArmDueAt) {
		t.Fatalf("expected chase deadline at %s, got %s", expectedArmDueAt, armDueAt)
	}

	// Move farther along the same axis while still inside combat-target range /
	// leash / visibility so engagement and the pending chase row stay armed.
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1900,
		Y:    2800,
		Time: 0xf2f3f4f5,
	}))); err != nil {
		t.Fatalf("unexpected owner move between chase arm and due: %v", err)
	}
	_ = flushServerFrames(t, flow)
	runtime.spawnChaseMu.Lock()
	_, stillArmedAfterOwnerMove := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !stillArmedAfterOwnerMove {
		t.Fatal("expected owner walk between arm and due not to clear the pending chase deadline")
	}
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("ChaseReplanOwner")
	if !ok {
		t.Fatal("expected chase-replan owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected owner move between arm and due to keep engagement for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("ChaseReplanOwner"); !ok || snapshot.TargetVID != targetVID {
		t.Fatalf("expected owner move between arm and due to keep selected combat target, ok=%v snapshot=%+v", ok, snapshot)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, flow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation before due chase replan, got %d frames", len(queued))
	}
	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	dueQueued := flushServerFrames(t, flow)
	if len(dueQueued) == 0 {
		t.Fatal("expected due chase-step to queue retained owner MOVE toward live post-move owner")
	}
	dueMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, dueQueued[0]))
	if err != nil {
		t.Fatalf("decode due chase replan MOVE: %v", err)
	}
	if dueMove.VID != targetVID || dueMove.X != 1800 || dueMove.Y != 2800 || dueMove.Duration == 0 {
		t.Fatalf("expected due chase MOVE +100 toward live owner at 1900 (not arm-time 1800), got %+v", dueMove)
	}
	for _, raw := range dueQueued[1:] {
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected chase replan MOVE not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
		if add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw)); err == nil && add.VID == targetVID {
			t.Fatalf("expected chase replan MOVE not to emit retained-viewer CHARACTER_ADD, got %+v", add)
		}
	}
	stepped, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || stepped.X != 1800 || stepped.Y != 2800 || stepped.Dead || stepped.SpawnLeash == nil || stepped.SpawnLeash.ReturnRequired {
		t.Fatalf("expected due chase replan to land one max_step toward live owner, ok=%v snapshot=%+v", ok, stepped)
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected chase replan to keep engagement ownership for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("ChaseReplanOwner"); !ok || snapshot.TargetVID != targetVID {
		t.Fatalf("expected chase replan to keep selected combat target, ok=%v snapshot=%+v", ok, snapshot)
	}
	runtime.spawnChaseMu.Lock()
	_, stillScheduled := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !stillScheduled {
		t.Fatalf("expected incomplete chase replan step to re-arm pending deadline for entity %d", group.EntityID)
	}
}
