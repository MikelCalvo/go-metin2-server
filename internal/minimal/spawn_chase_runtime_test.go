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

// One successful same-map chase MOVE must also queue one retained-viewer
// CHANGE_SPEED for the actor VID using the already-owned CHARACTER_ADD /
// CHARACTER_UPDATE default moving_speed. This does not invent a chase-speed
// formula, homeward CHANGE_SPEED, pathfinding, pack AI, or cross-map MOVE/WARP.
func TestGameRuntimeFlushServerFramesEmitsChangeSpeedOnDueSpawnGroupChaseStep(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ChaseSpeedOwner", 0x01030401, 0x02040401, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "chase-speed-owner", 0xe1e1e1e1, owner)
	staticActorStore := staticstore.NewMemoryStore()
	currentTime := time.Unix(1700005100, 0)

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
		interactionstore.NewMemoryStore(),
	)
	if err != nil {
		t.Fatalf("new game runtime for chase CHANGE_SPEED: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.chase_change_speed",
		Name:          "ChaseSpeedMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import chase CHANGE_SPEED spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.chase_change_speed")
	if !ok {
		t.Fatal("expected chase CHANGE_SPEED spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "chase-speed-owner", 0xe1e1e1e1)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected owner target error before chase CHANGE_SPEED arm: %v", err)
	}
	drainAcceptedTargetCreateNewIfQueued(t, flow)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	}))); err != nil {
		t.Fatalf("unexpected accepted hit before chase CHANGE_SPEED arm: %v", err)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engaged hit to arm a pending chase-step row, ok=%v snapshot=%+v", ok, pending)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, flow); len(queued) != 2 {
		t.Fatalf("expected the owned delayed retaliation beat to fire before the chase CHANGE_SPEED step, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	queued := flushServerFrames(t, flow)
	if len(queued) < 2 {
		t.Fatalf("expected due chase-step to queue retained owner MOVE plus CHANGE_SPEED, got %d frames", len(queued))
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("expected retained chase-step viewer to receive MOVE first, decode err=%v", err)
	}
	if moveAck.VID != targetVID || moveAck.X != 1800 || moveAck.Y != 2800 || moveAck.Duration == 0 {
		t.Fatalf("expected chase-step MOVE replication at planned +100 toward owner, got %+v", moveAck)
	}
	speedIdx := -1
	for idx, raw := range queued[1:] {
		speed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, raw))
		if err != nil {
			continue
		}
		if speed.VID != targetVID || speed.MovingSpeed != worldproto.BootstrapCharacterMovingSpeed {
			t.Fatalf("unexpected chase-step CHANGE_SPEED: %+v want vid=%d moving_speed=%d", speed, targetVID, worldproto.BootstrapCharacterMovingSpeed)
		}
		speedIdx = idx + 1
		break
	}
	if speedIdx < 0 {
		t.Fatalf("expected chase-step CHANGE_SPEED companion after MOVE, got %d frames", len(queued))
	}
	for idx, raw := range queued[1:] {
		if idx+1 == speedIdx {
			continue
		}
		if extraSpeed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, raw)); err == nil && extraSpeed.VID == targetVID {
			t.Fatalf("expected exactly one chase-step CHANGE_SPEED companion, got extra %+v among %d frames", extraSpeed, len(queued))
		}
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected chase-step CHANGE_SPEED fanout not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
		if add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw)); err == nil && add.VID == targetVID {
			t.Fatalf("expected chase-step CHANGE_SPEED fanout not to emit retained-viewer CHARACTER_ADD, got %+v", add)
		}
		if target, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, raw)); err == nil && target.TargetVID == 0 {
			t.Fatalf("expected chase-step CHANGE_SPEED not to clear selected combat target, got %+v", target)
		}
	}

	stepped, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || stepped.X != 1800 || stepped.Y != 2800 || stepped.Dead || stepped.SpawnLeash == nil || stepped.SpawnLeash.ReturnRequired {
		t.Fatalf("expected runtime actor to move one chase step while remaining in leash, ok=%v snapshot=%+v", ok, stepped)
	}
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("ChaseSpeedOwner")
	if !ok {
		t.Fatal("expected chase CHANGE_SPEED owner entity to remain registered")
	}
	if !runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected chase-step CHANGE_SPEED not to drop engagement ownership for entity %d", group.EntityID)
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("ChaseSpeedOwner"); !ok || snapshot.TargetVID != targetVID {
		t.Fatalf("expected chase-step CHANGE_SPEED to preserve selected combat target, ok=%v snapshot=%+v", ok, snapshot)
	}
}
