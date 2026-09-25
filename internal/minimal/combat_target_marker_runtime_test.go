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
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowAcceptedNonZeroTargetQueuesSelfOnlyTargetCreateNew(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MarkerOwner", 0x010301A1, 0x020401A1, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("MarkerPeer", 0x010301A2, 0x020401A2, 1120, 2100, 0, 102, 202)
	issuePeerTicket(t, store, "marker-owner", 0xA1A1A1A1, owner)
	issuePeerTicket(t, store, "marker-peer", 0xA2A2A2A2, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected target-marker runtime error: %v", err)
	}
	const dummyName = "MarkerPresentationDummy"
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, dummyName, bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected target-marker presentation dummy registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "marker-owner", 0xA1A1A1A1)
	defer closeSessionFlow(t, ownerFlow)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected owner bootstrap frames with visible training dummy, got %d", len(ownerEnter))
	}
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "marker-peer", 0xA2A2A2A2)
	defer closeSessionFlow(t, peerFlow)
	if len(peerEnter) != 11 {
		t.Fatalf("expected peer bootstrap frames with owner and visible training dummy, got %d", len(peerEnter))
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)

	rejected, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID + 1})))
	if err != nil {
		t.Fatalf("unexpected rejected target-selection error before marker presentation: %v", err)
	}
	if len(rejected) != 0 {
		t.Fatalf("expected rejected TARGET to keep no HP-ack frames, got %d", len(rejected))
	}
	drainAcceptedTargetCreateNewIfQueued(t, ownerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected rejected TARGET to queue no TARGET_CREATE_NEW, got %d", len(queued))
	}

	clearOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: 0})))
	if err != nil {
		t.Fatalf("unexpected client clear-target before marker presentation: %v", err)
	}
	if len(clearOut) != 0 {
		t.Fatalf("expected silent TARGET(0) to keep no echo frames, got %d", len(clearOut))
	}
	drainAcceptedTargetCreateNewIfQueued(t, ownerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected silent TARGET(0) to queue no TARGET_CREATE_NEW, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before marker presentation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 TARGET HP-ack frame on accepted selection, got %d", len(selectOut))
	}
	ack, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, selectOut[0]))
	if err != nil {
		t.Fatalf("decode TARGET HP ack before marker presentation: %v", err)
	}
	if ack.TargetVID != targetVID || ack.HPPercent != 100 {
		t.Fatalf("expected TARGET HP ack to keep current dummy HP, got %+v", ack)
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, dummyName, targetVID)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected visible peer to receive no TARGET_CREATE_NEW, got %d", len(queued))
	}

	repeatSelect, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected repeat target selection error: %v", err)
	}
	if len(repeatSelect) != 1 {
		t.Fatalf("expected repeat accepted TARGET to keep one HP-ack frame, got %d", len(repeatSelect))
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, dummyName, targetVID)

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error after marker presentation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected first normal attack after marker presentation to return target refresh plus damage-info, got %d", len(attackOut))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
	if err != nil {
		t.Fatalf("decode target refresh after marker presentation: %v", err)
	}
	if refresh.TargetVID != targetVID || refresh.HPPercent != 90 {
		t.Fatalf("expected marker presentation not to mutate combat HP before first normal hit, got %+v", refresh)
	}
	assertDamageInfoFrame(t, attackOut[1], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "attack after marker presentation")
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected ordinary standing dummy hit after marker presentation to queue no TARGET_CREATE_NEW, got %d", len(queued))
	}
	peerHitQueued := flushServerFrames(t, peerFlow)
	if len(peerHitQueued) != 1 {
		t.Fatalf("expected visible peer to receive only DAMAGE_INFO after dummy hit, got %d", len(peerHitQueued))
	}
	assertDamageInfoFrame(t, peerHitQueued[0], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "peer hit after marker presentation")
}

func TestGameSessionFlowSelectedChaseStepQueuesSelfOnlyTargetUpdate(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MarkerChaseOwner", 0x010301B1, 0x020401B1, 1900, 2800, 0, 101, 201)
	owner.MapIndex = 42
	owner.Points[bootstrapPlayerPointValueIndex] = 50
	peer := peerVisibilityCharacter("MarkerChasePeer", 0x010301B2, 0x020401B2, 1880, 2800, 0, 102, 202)
	peer.MapIndex = 42
	peer.Points[bootstrapPlayerPointValueIndex] = 50
	issuePeerTicket(t, store, "marker-chase-owner", 0xB1B1B1B1, owner)
	issuePeerTicket(t, store, "marker-chase-peer", 0xB2B2B2B2, peer)
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
		t.Fatalf("new game runtime for selected chase target update: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	_, err = runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.marker_chase_update",
		Name:          "MarkerChaseMob",
		MapIndex:      42,
		X:             1700,
		Y:             2800,
		RaceNum:       20350,
		CombatProfile: string(worldruntime.StaticActorCombatProfilePracticeMob),
	}}})
	if err != nil {
		t.Fatalf("import selected chase target-update spawn-group bundle: %v", err)
	}
	group, ok := runtime.SpawnGroupByRef("practice.marker_chase_update")
	if !ok {
		t.Fatal("expected selected chase target-update spawn group to resolve by ref")
	}
	targetVID := uint32(group.EntityID)

	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "marker-chase-owner", 0xB1B1B1B1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "marker-chase-peer", 0xB2B2B2B2)
	defer closeSessionFlow(t, peerFlow)
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil {
		t.Fatalf("unexpected owner target error before selected chase target update: %v", err)
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, "MarkerChaseMob", targetVID)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected visible peer to receive no TARGET_CREATE_NEW before chase, got %d", len(queued))
	}
	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	}))); err != nil {
		t.Fatalf("unexpected accepted hit before selected chase target update: %v", err)
	}
	if queued := flushServerFrames(t, ownerFlow); containsTargetUpdate(t, queued, int32(targetVID)) {
		t.Fatalf("expected the arming hit to omit TARGET_UPDATE before the chase step")
	}

	currentTime = currentTime.Add(bootstrapPracticeMobServerOriginRetaliationDelay)
	if queued := flushServerFrames(t, ownerFlow); containsTargetUpdate(t, queued, int32(targetVID)) {
		t.Fatalf("expected delayed retaliation to omit TARGET_UPDATE")
	}

	currentTime = currentTime.Add(bootstrapSpawnGroupChaseStepDelay - bootstrapPracticeMobServerOriginRetaliationDelay)
	queued := flushServerFrames(t, ownerFlow)
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("expected retained chase-step viewer to receive MOVE first, decode err=%v", err)
	}
	if moveAck.VID != targetVID || moveAck.X != 1800 || moveAck.Y != 2800 {
		t.Fatalf("expected chase-step MOVE at planned +100 toward owner, got %+v", moveAck)
	}
	assertSelfOnlyTargetUpdate(t, queued, int32(targetVID), 1800, 2800)
	if refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, queued[0])); err == nil && refresh.TargetVID == targetVID {
		t.Fatalf("expected chase TARGET_UPDATE not to replace the TARGET HP carrier, got %+v", refresh)
	}

	peerQueued := flushServerFrames(t, peerFlow)
	if containsTargetUpdate(t, peerQueued, int32(targetVID)) {
		t.Fatalf("expected visible unselected peer to receive no TARGET_UPDATE")
	}
	if snapshot, ok := runtime.CombatTargetSnapshot("MarkerChaseOwner"); !ok || snapshot.TargetVID != targetVID {
		t.Fatalf("expected chase TARGET_UPDATE to preserve selected combat target, ok=%v snapshot=%+v", ok, snapshot)
	}
}

func containsTargetUpdate(t *testing.T, frames [][]byte, markerID int32) bool {
	t.Helper()
	for _, raw := range frames {
		update, err := combatproto.DecodeServerTargetUpdate(decodeSingleFrame(t, raw))
		if err == nil && update.ID == markerID {
			return true
		}
	}
	return false
}

func assertSelfOnlyTargetUpdate(t *testing.T, frames [][]byte, markerID int32, x int32, y int32) {
	t.Helper()
	found := 0
	for _, raw := range frames {
		update, err := combatproto.DecodeServerTargetUpdate(decodeSingleFrame(t, raw))
		if err != nil {
			continue
		}
		if update.ID != markerID || update.X != x || update.Y != y {
			t.Fatalf("unexpected TARGET_UPDATE: %+v want id=%d x=%d y=%d", update, markerID, x, y)
		}
		found++
	}
	if found != 1 {
		t.Fatalf("expected exactly one self-only TARGET_UPDATE, got %d among %d frames", found, len(frames))
	}
}

func flushSelfOnlyTargetCreateNew(t *testing.T, flow service.SessionFlow, wantName string, targetVID uint32) {
	t.Helper()
	queued := drainPendingTargetCreateNew(flow)
	if len(queued) != 1 {
		t.Fatalf("expected one self-only TARGET_CREATE_NEW after accepted non-zero TARGET, got %d", len(queued))
	}
	marker, err := combatproto.DecodeServerTargetCreateNew(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode self TARGET_CREATE_NEW after accepted TARGET: %v", err)
	}
	if marker.ID != int32(targetVID) || marker.TargetName != wantName || marker.VID != targetVID || marker.Type != combatproto.ServerTargetMarkerTypeCharacter {
		t.Fatalf("unexpected self TARGET_CREATE_NEW after accepted TARGET: %+v", marker)
	}
}

func drainAcceptedTargetCreateNewIfQueued(t *testing.T, flow service.SessionFlow) {
	t.Helper()
	queued := drainPendingTargetCreateNew(flow)
	if len(queued) == 0 {
		return
	}
	if len(queued) != 1 {
		t.Fatalf("expected at most one pending TARGET_CREATE_NEW after TARGET, got %d", len(queued))
	}
	if _, err := combatproto.DecodeServerTargetCreateNew(decodeSingleFrame(t, queued[0])); err != nil {
		t.Fatalf("decode pending TARGET_CREATE_NEW after TARGET: %v", err)
	}
}
