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

// Hit-armed / still-selected owners that walk outside aggro while remaining
// inside combat-target range / leash / visibility must keep engagement and the
// pending chase deadline. Contrast with proximity-only leave-radius walk-away,
// which releases engaged_by and clears chase without inventing TARGET(0, 0).
func TestGameRuntimeHitArmedSpawnGroupChaseSurvivesOwnerWalkOutsideAggroRadius(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	// Start inside DefaultSpawnAggroRadius (200) so the accepted hit arms chase
	// from a normal in-aggro engagement.
	owner := peerVisibilityCharacter("HitArmedWalkAwayOwner", 0x01030321, 0x02040321, 1850, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "hit-armed-walkaway-owner", 0xd1d1d1d1, owner)
	watcher := peerVisibilityCharacter("HitArmedWalkAwayWatcher", 0x01030322, 0x02040322, 1920, 2800, 0, 101, 201)
	watcher.MapIndex = 42
	watcher.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "hit-armed-walkaway-watcher", 0xd2d2d2d2, watcher)

	staticActorStore := staticstore.NewMemoryStore()
	interactionStore := interactionstore.NewMemoryStore()
	currentTime := time.Unix(1700004300, 0)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{
			LegacyAddr:           ":13000",
			PublicAddr:           "127.0.0.1",
			VisibilityMode:       "radius",
			VisibilityRadius:     400,
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
		Ref:           "practice.hit_armed_chase_walkaway",
		Name:          "HitArmedWalkAwayMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}}); err != nil {
		t.Fatalf("import hit-armed chase walk-away spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.hit_armed_chase_walkaway")
	if !ok {
		t.Fatal("expected hit-armed chase walk-away spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "hit-armed-walkaway-owner", 0xd1d1d1d1)
	defer closeSessionFlow(t, ownerFlow)
	flushServerFrames(t, ownerFlow)

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected owner target error before hit-armed chase arm: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected owner to select hit-armed chase practice mob, got %d frames", len(selectOut))
	}
	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted hit before hit-armed chase arm: %v", err)
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
	expectedDueAt := currentTime.Add(bootstrapSpawnGroupChaseStepDelay)
	if !dueAt.Equal(expectedDueAt) {
		t.Fatalf("expected chase deadline at %s, got %s", expectedDueAt, dueAt)
	}

	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "hit-armed-walkaway-watcher", 0xd2d2d2d2)
	defer closeSessionFlow(t, watcherFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, watcherFlow)

	watcherBlocked, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target error while hit-armed engagement remains live: %v", err)
	}
	if len(watcherBlocked) != 0 {
		t.Fatalf("expected third-party TARGET to fail closed while hit-armed engagement remains live, got %d frames", len(watcherBlocked))
	}

	// Stay inside visibility/leash (400) and combat-target range (300) but leave
	// DefaultSpawnAggroRadius (200): distance from authored home 1700 -> 1950 is 250.
	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1950,
		Y:    2800,
		Time: 0xd3d4d5d6,
	})))
	if err != nil {
		t.Fatalf("unexpected owner move error while walking out of aggro radius: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected 1 immediate self move ack after walking out of aggro radius, got %d frames", len(moveOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		for _, raw := range queued {
			if target, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, raw)); err == nil && target.TargetVID == 0 {
				t.Fatalf("expected hit-armed walk outside aggro not to invent TARGET(0,0), got %+v among %d queued frames", target, len(queued))
			}
		}
		t.Fatalf("expected hit-armed walk outside aggro to stay silent before chase/retaliation due times, got %d queued frames", len(queued))
	}

	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("HitArmedWalkAwayOwner")
	if !ok {
		t.Fatal("expected hit-armed walk-away owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected hit-armed walk outside aggro to keep engaged_by for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("HitArmedWalkAwayOwner"); !ok || snapshot.TargetVID != targetVID {
		t.Fatalf("expected hit-armed walk outside aggro to keep selected combat target, ok=%v snapshot=%+v", ok, snapshot)
	}
	runtime.spawnChaseMu.Lock()
	dueAtAfterWalk, chaseScheduledAfterWalk := runtime.spawnChaseStepDueAt[group.EntityID]
	runtime.spawnChaseMu.Unlock()
	if !chaseScheduledAfterWalk {
		t.Fatalf("expected hit-armed walk outside aggro to keep pending chase for entity %d", group.EntityID)
	}
	if !dueAtAfterWalk.Equal(expectedDueAt) {
		t.Fatalf("expected chase deadline to stay at %s after walk outside aggro, got %s", expectedDueAt, dueAtAfterWalk)
	}

	watcherStillBlocked, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected watcher target error after hit-armed walk outside aggro: %v", err)
	}
	if len(watcherStillBlocked) != 0 {
		t.Fatalf("expected third-party TARGET to stay fail-closed after hit-armed walk outside aggro, got %d frames", len(watcherStillBlocked))
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 2 {
		t.Fatalf("expected delayed retaliation to continue under hit-armed engagement after walk outside aggro, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, ownerFlow)
	if len(chaseQueued) == 0 {
		t.Fatal("expected due chase-step to queue retained owner MOVE after hit-armed walk outside aggro")
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil {
		t.Fatalf("decode chase-step MOVE after hit-armed walk outside aggro: %v", err)
	}
	if chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 || chaseMove.Duration == 0 {
		t.Fatalf("expected chase MOVE +100 toward post-walk owner, got %+v", chaseMove)
	}
	for _, raw := range chaseQueued[1:] {
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected chase MOVE not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
		if add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw)); err == nil && add.VID == targetVID {
			t.Fatalf("expected chase MOVE not to emit retained-viewer CHARACTER_ADD, got %+v", add)
		}
		if target, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, raw)); err == nil && target.TargetVID == 0 {
			t.Fatalf("expected chase step to preserve selected combat target, got clear frame %+v", target)
		}
	}
	stepped, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || stepped.X != 1800 || stepped.Y != 2800 || stepped.Dead || stepped.SpawnLeash == nil || stepped.SpawnLeash.ReturnRequired {
		t.Fatalf("expected chase step to land within leash after hit-armed walk outside aggro, ok=%v snapshot=%+v", ok, stepped)
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected chase step to keep engagement ownership for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("HitArmedWalkAwayOwner"); !ok || snapshot.TargetVID != targetVID {
		t.Fatalf("expected chase step to keep selected combat target, ok=%v snapshot=%+v", ok, snapshot)
	}
}
