package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
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
