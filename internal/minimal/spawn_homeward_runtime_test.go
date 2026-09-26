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

// One successful same-map homeward MOVE must also queue one retained-viewer
// CHANGE_SPEED for the actor VID using the already-owned CHARACTER_ADD /
// CHARACTER_UPDATE default moving_speed. This does not invent a sit/walk
// table, buff, authored chase-speed formula, return-step CHANGE_SPEED,
// operator-position CHANGE_SPEED, or cross-map MOVE/WARP. Zero-HP viewers
// stay skipped.
func TestGameRuntimeFlushServerFramesEmitsChangeSpeedOnDueSpawnGroupHomewardStep(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("HomewardSpeedOwner", 0x01030601, 0x02040601, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	deadViewer := peerVisibilityCharacter("HomewardSpeedDead", 0x01030602, 0x02040602, 1750, 2800, 0, 102, 202)
	deadViewer.MapIndex = 42
	deadViewer.Points[bootstrapPlayerPointValueIndex] = 0
	issuePeerTicket(t, store, "homeward-speed-owner", 0xe2e2e2e1, owner)
	issuePeerTicket(t, store, "homeward-speed-dead", 0xe2e2e2e2, deadViewer)
	staticActorStore := staticstore.NewMemoryStore()
	currentTime := time.Unix(1700005200, 0)

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
		t.Fatalf("new game runtime for homeward CHANGE_SPEED: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.homeward_change_speed",
		Name:          "HomewardSpeedMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import homeward CHANGE_SPEED spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.homeward_change_speed")
	if !ok {
		t.Fatal("expected homeward CHANGE_SPEED spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "homeward-speed-owner", 0xe2e2e2e1)
	defer closeSessionFlow(t, flow)
	deadFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "homeward-speed-dead", 0xe2e2e2e2)
	defer closeSessionFlow(t, deadFlow)
	flushServerFrames(t, flow)
	flushServerFrames(t, deadFlow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected owner target error before homeward CHANGE_SPEED: %v", err)
	}
	drainAcceptedTargetCreateNewIfQueued(t, flow)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	}))); err != nil {
		t.Fatalf("unexpected accepted hit before homeward CHANGE_SPEED: %v", err)
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, flow); len(queued) != 2 {
		t.Fatalf("expected the owned delayed retaliation beat to fire before homeward CHANGE_SPEED, got %d frames", len(queued))
	}
	_ = flushServerFrames(t, deadFlow)

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	chaseQueued := flushServerFrames(t, flow)
	if len(chaseQueued) == 0 {
		t.Fatal("expected due chase-step to displace actor within_radius before homeward CHANGE_SPEED")
	}
	chaseMove, err := movep.DecodeMoveAck(decodeSingleFrame(t, chaseQueued[0]))
	if err != nil || chaseMove.VID != targetVID || chaseMove.X != 1800 || chaseMove.Y != 2800 {
		t.Fatalf("expected chase displace MOVE at +100 toward owner before homeward, err=%v got %+v", err, chaseMove)
	}
	_ = flushServerFrames(t, deadFlow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    2100,
		Y:    2800,
		Time: 0x61626364,
	}))); err != nil {
		t.Fatalf("unexpected owner move outside aggro before homeward CHANGE_SPEED: %v", err)
	}
	_ = flushServerFrames(t, flow)
	clearOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: 0})))
	if err != nil {
		t.Fatalf("unexpected owner TARGET(0) clear before homeward CHANGE_SPEED: %v", err)
	}
	if len(clearOut) != 0 {
		t.Fatalf("expected TARGET(0) clear to emit no frames, got %d", len(clearOut))
	}
	drainAcceptedTargetCreateNewIfQueued(t, flow)
	if pending, ok := runtime.SpawnGroupHomewardStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected engagement release to arm homeward before CHANGE_SPEED, ok=%v snapshot=%+v", ok, pending)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no homeward CHANGE_SPEED before the 1s deadline, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupHomewardStepDelay)
	queued := flushServerFrames(t, flow)
	if len(queued) < 2 {
		t.Fatalf("expected due homeward-step to queue retained owner MOVE plus CHANGE_SPEED, got %d frames", len(queued))
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("expected retained homeward viewer to receive MOVE first, decode err=%v", err)
	}
	if moveAck.VID != targetVID || moveAck.X != 1700 || moveAck.Y != 2800 || moveAck.Duration == 0 {
		t.Fatalf("expected homeward MOVE replication at authored home, got %+v", moveAck)
	}
	speedIdx := -1
	for idx, raw := range queued[1:] {
		speed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, raw))
		if err != nil {
			continue
		}
		if speed.VID != targetVID || speed.MovingSpeed != worldproto.BootstrapCharacterMovingSpeed {
			t.Fatalf("unexpected homeward-step CHANGE_SPEED: %+v want vid=%d moving_speed=%d", speed, targetVID, worldproto.BootstrapCharacterMovingSpeed)
		}
		speedIdx = idx + 1
		break
	}
	if speedIdx < 0 {
		t.Fatalf("expected homeward-step CHANGE_SPEED companion after MOVE, got %d frames", len(queued))
	}
	for idx, raw := range queued[1:] {
		if idx+1 == speedIdx {
			continue
		}
		if extraSpeed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, raw)); err == nil && extraSpeed.VID == targetVID {
			t.Fatalf("expected exactly one homeward-step CHANGE_SPEED companion, got extra %+v among %d frames", extraSpeed, len(queued))
		}
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected homeward-step CHANGE_SPEED fanout not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
		if add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw)); err == nil && add.VID == targetVID {
			t.Fatalf("expected homeward-step CHANGE_SPEED fanout not to emit retained-viewer CHARACTER_ADD, got %+v", add)
		}
	}

	deadQueued := flushServerFrames(t, deadFlow)
	for _, raw := range deadQueued {
		if speed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, raw)); err == nil && speed.VID == targetVID {
			t.Fatalf("expected zero-HP viewer to stay skipped for homeward CHANGE_SPEED, got %+v", speed)
		}
		if move, err := movep.DecodeMoveAck(decodeSingleFrame(t, raw)); err == nil && move.VID == targetVID {
			t.Fatalf("expected zero-HP viewer to stay skipped for homeward MOVE, got %+v", move)
		}
	}

	returned, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || returned.X != 1700 || returned.Y != 2800 || returned.Dead || returned.SpawnLeash == nil || returned.SpawnLeash.Status != worldruntime.SpawnLeashStatusAtHome {
		t.Fatalf("expected homeward-step CHANGE_SPEED to restore at_home leash state, ok=%v snapshot=%+v", ok, returned)
	}
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName("HomewardSpeedOwner")
	if !ok {
		t.Fatal("expected homeward CHANGE_SPEED owner entity to remain registered")
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, ownerEntity.Entity.ID) {
		t.Fatalf("expected homeward-step CHANGE_SPEED to keep entity %d unengaged", group.EntityID)
	}
}
