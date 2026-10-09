package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// A live position-only operator update reuses the owned retained-viewer MOVE
// and its actor bootstrap speed; AOI entry/exit and zero-HP recipients do not
// receive a speed companion. No separate homeward/return timer is exercised.
func TestGameRuntimeOperatorPositionMoveQueuesRetainedViewerChangeSpeed(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	old := peerVisibilityCharacter("SpeedOld", 0x010309a1, 0x020409a1, 1200, 2200, 0, 101, 201)
	retained := peerVisibilityCharacter("SpeedRetained", 0x010309a2, 0x020409a2, 1650, 2200, 0, 102, 202)
	newViewer := peerVisibilityCharacter("SpeedNew", 0x010309a3, 0x020409a3, 2100, 2200, 0, 103, 203)
	dead := peerVisibilityCharacter("SpeedFloored", 0x010309a4, 0x020409a4, 1650, 2200, 0, 104, 204)
	dead.Points[bootstrapPlayerPointValueIndex] = 0
	issuePeerTicket(t, store, "speed-old", 0xd2d2d2a1, old)
	issuePeerTicket(t, store, "speed-retained", 0xd2d2d2a2, retained)
	issuePeerTicket(t, store, "speed-new", 0xd2d2d2a3, newViewer)
	issuePeerTicket(t, store, "speed-floored", 0xd2d2d2a4, dead)

	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", VisibilityMode: "radius", VisibilityRadius: 800, VisibilitySectorSize: 200},
		store, nil, staticstore.NewMemoryStore(), interactionstore.NewMemoryStore(),
	)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref: "practice.operator_speed", Name: "OperatorSpeedMob", MapIndex: bootstrapMapIndex,
		X: 1200, Y: 2200, RaceNum: 101, CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}); err != nil {
		t.Fatalf("import spawn: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.operator_speed")
	if !ok {
		t.Fatal("spawn missing")
	}
	vid := uint32(group.EntityID)
	oldFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "speed-old", 0xd2d2d2a1)
	defer closeSessionFlow(t, oldFlow)
	retainedFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "speed-retained", 0xd2d2d2a2)
	defer closeSessionFlow(t, retainedFlow)
	newFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "speed-new", 0xd2d2d2a3)
	defer closeSessionFlow(t, newFlow)
	deadFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "speed-floored", 0xd2d2d2a4)
	defer closeSessionFlow(t, deadFlow)
	flushServerFrames(t, oldFlow)
	flushServerFrames(t, retainedFlow)
	flushServerFrames(t, newFlow)
	flushServerFrames(t, deadFlow)

	if _, ok := runtime.UpdateStaticActor(group.EntityID, "OperatorSpeedMob", bootstrapMapIndex, 2100, 2200, 101); !ok {
		t.Fatal("position-only update rejected")
	}
	retainedFrames := flushServerFrames(t, retainedFlow)
	if len(retainedFrames) != 2 {
		t.Fatalf("expected exactly MOVE then CHANGE_SPEED for retained viewer, got %d frames", len(retainedFrames))
	}
	move, err := movep.DecodeMoveAck(decodeSingleFrame(t, retainedFrames[0]))
	if err != nil || move.VID != vid || move.X != 2100 || move.Y != 2200 || move.Duration == 0 {
		t.Fatalf("wrong retained MOVE: %+v err=%v", move, err)
	}
	speed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, retainedFrames[1]))
	if err != nil || speed.VID != vid || speed.MovingSpeed != worldproto.BootstrapCharacterMovingSpeed {
		t.Fatalf("wrong retained CHANGE_SPEED: %+v err=%v", speed, err)
	}
	oldFrames := flushServerFrames(t, oldFlow)
	if len(oldFrames) != 1 || !queuedFramesContainCharacterDeleteForVID(t, oldFrames, vid) {
		t.Fatalf("old-only viewer must receive delete only, got %d frames", len(oldFrames))
	}
	newFrames := flushServerFrames(t, newFlow)
	if len(newFrames) != 3 || !queuedFramesContainCharacterAddForVID(t, newFrames, vid) {
		t.Fatalf("new-only viewer must receive add/info/update only, got %d frames", len(newFrames))
	}
	if frames := flushServerFrames(t, deadFlow); len(frames) != 0 {
		t.Fatalf("zero-HP retained viewer must receive no frames, got %d", len(frames))
	}

	// Accepted no-coordinate-change refresh and rejected writes must not look
	// like a new position MOVE or emit the speed companion.
	if _, ok := runtime.UpdateStaticActor(group.EntityID, "OperatorSpeedMob", bootstrapMapIndex, 2100, 2200, 101); !ok {
		t.Fatal("same-position refresh rejected")
	}
	for _, raw := range flushServerFrames(t, retainedFlow) {
		if _, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, raw)); err == nil {
			t.Fatal("same-position refresh queued CHANGE_SPEED")
		}
	}
	if _, ok := runtime.UpdateStaticActor(group.EntityID, "OperatorSpeedMob", 0, 2200, 2200, 101); ok {
		t.Fatal("invalid map accepted")
	}
	if frames := flushServerFrames(t, retainedFlow); len(frames) != 0 {
		t.Fatalf("rejected update queued frames: %d", len(frames))
	}
}
