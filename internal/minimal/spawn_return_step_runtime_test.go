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

// One successful same-map return-step MOVE must also queue one retained-viewer
// CHANGE_SPEED for the actor VID using the already-owned CHARACTER_ADD /
// CHARACTER_UPDATE default moving_speed. This does not invent a sit/walk
// table, buff, authored chase-speed formula, homeward CHANGE_SPEED,
// operator-position CHANGE_SPEED, or cross-map MOVE/WARP. Zero-HP viewers
// stay skipped.
func TestGameRuntimeFlushServerFramesEmitsChangeSpeedOnDueSpawnGroupReturnStep(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	viewer := peerVisibilityCharacter("ReturnSpeedViewer", 0x01030701, 0x02040701, 2301, 2800, 0, 101, 201)
	viewer.MapIndex = 42
	viewer.Points[bootstrapPlayerPointValueIndex] = 50
	deadViewer := peerVisibilityCharacter("ReturnSpeedDead", 0x01030702, 0x02040702, 2250, 2800, 0, 102, 202)
	deadViewer.MapIndex = 42
	deadViewer.Points[bootstrapPlayerPointValueIndex] = 0
	issuePeerTicket(t, store, "return-speed-viewer", 0xe3e3e3e1, viewer)
	issuePeerTicket(t, store, "return-speed-dead", 0xe3e3e3e2, deadViewer)
	staticActorStore := staticstore.NewMemoryStore()
	currentTime := time.Unix(1700005300, 0)

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
		t.Fatalf("new game runtime for return-step CHANGE_SPEED: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.return_change_speed",
		Name:          "ReturnSpeedMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import return-step CHANGE_SPEED spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.return_change_speed")
	if !ok {
		t.Fatal("expected return-step CHANGE_SPEED spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "return-speed-viewer", 0xe3e3e3e1)
	defer closeSessionFlow(t, flow)
	deadFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "return-speed-dead", 0xe3e3e3e2)
	defer closeSessionFlow(t, deadFlow)
	flushServerFrames(t, flow)
	flushServerFrames(t, deadFlow)

	if _, ok := runtime.UpdateStaticActor(group.EntityID, "ReturnSpeedMob", 42, 2301, 2800, 20350); !ok {
		t.Fatal("expected same-map return_required displace to succeed")
	}
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, deadFlow)
	if pending, ok := runtime.SpawnGroupReturnStep(group.EntityID); !ok || pending.EntityID != group.EntityID {
		t.Fatalf("expected return_required displace to arm return-step before CHANGE_SPEED, ok=%v snapshot=%+v", ok, pending)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no return-step CHANGE_SPEED before the 1s deadline, got %d frames", len(queued))
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupReturnStepDelay)
	queued := flushServerFrames(t, flow)
	if len(queued) < 2 {
		t.Fatalf("expected due return-step to queue retained viewer MOVE plus CHANGE_SPEED, got %d frames", len(queued))
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("expected retained return-step viewer to receive MOVE first, decode err=%v", err)
	}
	if moveAck.VID != targetVID || moveAck.X != 2201 || moveAck.Y != 2800 || moveAck.Duration == 0 {
		t.Fatalf("expected return-step MOVE replication at planned -100 toward home, got %+v", moveAck)
	}
	speed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("expected return-step CHANGE_SPEED companion immediately after MOVE, decode err=%v", err)
	}
	if speed.VID != targetVID || speed.MovingSpeed != worldproto.BootstrapCharacterMovingSpeed {
		t.Fatalf("unexpected return-step CHANGE_SPEED: %+v want vid=%d moving_speed=%d", speed, targetVID, worldproto.BootstrapCharacterMovingSpeed)
	}
	for _, raw := range queued[2:] {
		if extraSpeed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, raw)); err == nil && extraSpeed.VID == targetVID {
			t.Fatalf("expected exactly one return-step CHANGE_SPEED companion, got extra %+v among %d frames", extraSpeed, len(queued))
		}
		if deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, raw)); err == nil && deleted.VID == targetVID {
			t.Fatalf("expected return-step CHANGE_SPEED fanout not to emit retained-viewer CHARACTER_DEL, got %+v", deleted)
		}
		if add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw)); err == nil && add.VID == targetVID {
			t.Fatalf("expected return-step CHANGE_SPEED fanout not to emit retained-viewer CHARACTER_ADD, got %+v", add)
		}
		if target, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, raw)); err == nil && target.TargetVID == targetVID {
			t.Fatalf("expected return-step CHANGE_SPEED not to invent selected combat target, got %+v", target)
		}
	}

	deadQueued := flushServerFrames(t, deadFlow)
	for _, raw := range deadQueued {
		if speed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, raw)); err == nil && speed.VID == targetVID {
			t.Fatalf("expected zero-HP viewer to stay skipped for return-step CHANGE_SPEED, got %+v", speed)
		}
		if move, err := movep.DecodeMoveAck(decodeSingleFrame(t, raw)); err == nil && move.VID == targetVID {
			t.Fatalf("expected zero-HP viewer to stay skipped for return-step MOVE, got %+v", move)
		}
	}

	stepped, ok := runtime.SpawnGroup(group.EntityID)
	if !ok || stepped.X != 2201 || stepped.Y != 2800 || stepped.Dead || stepped.SpawnLeash == nil || stepped.SpawnLeash.Status != worldruntime.SpawnLeashStatusReturnRequired {
		t.Fatalf("expected return-step CHANGE_SPEED to leave the actor one step closer and still return_required, ok=%v snapshot=%+v", ok, stepped)
	}
	viewerEntity, ok := runtime.sharedWorld.playerEntityByName("ReturnSpeedViewer")
	if !ok {
		t.Fatal("expected return-step CHANGE_SPEED viewer entity to remain registered")
	}
	if runtime.sharedWorld.StaticActorCombatEngagedBySubject(group.EntityID, viewerEntity.Entity.ID) {
		t.Fatalf("expected return-step CHANGE_SPEED to keep entity %d unengaged", group.EntityID)
	}
}
