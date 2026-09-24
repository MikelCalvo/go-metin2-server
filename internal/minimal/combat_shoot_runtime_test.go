package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowAcceptedShootEmitsSelfOnlyCreateFly(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ShootOwner", 0x010301B1, 0x020401B1, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("ShootPeer", 0x010301B2, 0x020401B2, 1120, 2100, 0, 102, 202)
	issuePeerTicket(t, store, "shoot-owner", 0xB1B1B1B1, owner)
	issuePeerTicket(t, store, "shoot-peer", 0xB2B2B2B2, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected shoot runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "ShootPresentationDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected shoot presentation dummy registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "shoot-owner", 0xB1B1B1B1)
	defer closeSessionFlow(t, ownerFlow)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected owner bootstrap frames with visible training dummy, got %d", len(ownerEnter))
	}
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "shoot-peer", 0xB2B2B2B2)
	defer closeSessionFlow(t, peerFlow)
	if len(peerEnter) != 11 {
		t.Fatalf("expected peer bootstrap frames with owner and visible training dummy, got %d", len(peerEnter))
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)

	unselected, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientShoot(combatproto.ClientShootPacket{ShootType: bootstrapShootPresentationType})))
	if err != nil {
		t.Fatalf("unexpected unselected shoot error: %v", err)
	}
	if len(unselected) != 0 {
		t.Fatalf("expected unselected shoot to stay fail-closed, got %d frames", len(unselected))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected unselected shoot to queue no CREATE_FLY, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before shoot presentation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before shoot presentation, got %d", len(selectOut))
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, "ShootPresentationDummy", targetVID)

	unsupported, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientShoot(combatproto.ClientShootPacket{ShootType: 0x83})))
	if err != nil {
		t.Fatalf("unexpected unsupported shoot error: %v", err)
	}
	if len(unsupported) != 0 {
		t.Fatalf("expected unsupported shoot type to stay fail-closed, got %d frames", len(unsupported))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected unsupported shoot to queue no CREATE_FLY, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected unsupported shoot to queue no peer frames, got %d", len(queued))
	}

	shootOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientShoot(combatproto.ClientShootPacket{ShootType: bootstrapShootPresentationType})))
	if err != nil {
		t.Fatalf("unexpected accepted shoot error: %v", err)
	}
	if len(shootOut) != 1 {
		t.Fatalf("expected one self-only CREATE_FLY after accepted shoot, got %d", len(shootOut))
	}
	selfFly, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, shootOut[0]))
	if err != nil {
		t.Fatalf("decode self CREATE_FLY after accepted shoot: %v", err)
	}
	if selfFly.Type != bootstrapCreateFlyType || selfFly.StartVID != owner.VID || selfFly.EndVID != targetVID {
		t.Fatalf("unexpected self CREATE_FLY after accepted shoot: %+v", selfFly)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected accepted shoot not to queue extra owner frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected visible peer to receive no CREATE_FLY, got %d", len(queued))
	}

	repeatOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientShoot(combatproto.ClientShootPacket{ShootType: bootstrapShootPresentationType})))
	if err != nil {
		t.Fatalf("unexpected repeat shoot error: %v", err)
	}
	if len(repeatOut) != 1 {
		t.Fatalf("expected repeat accepted shoot to emit one CREATE_FLY, got %d", len(repeatOut))
	}
	repeatFly, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, repeatOut[0]))
	if err != nil {
		t.Fatalf("decode repeat CREATE_FLY: %v", err)
	}
	if repeatFly.Type != bootstrapCreateFlyType || repeatFly.StartVID != owner.VID || repeatFly.EndVID != targetVID {
		t.Fatalf("unexpected repeat CREATE_FLY: %+v", repeatFly)
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error after shoot presentation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected first normal attack after shoot presentation to return target refresh plus damage-info, got %d", len(attackOut))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
	if err != nil {
		t.Fatalf("decode target refresh after shoot presentation: %v", err)
	}
	if refresh.TargetVID != targetVID || refresh.HPPercent != 90 {
		t.Fatalf("expected shoot presentation not to mutate combat HP before first normal hit, got %+v", refresh)
	}
	assertDamageInfoFrame(t, attackOut[1], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "attack after shoot presentation")
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected ordinary standing dummy hit after shoot presentation to queue no CREATE_FLY or STUN, got %d", len(queued))
	}
	peerHitQueued := flushServerFrames(t, peerFlow)
	if len(peerHitQueued) != 1 {
		t.Fatalf("expected visible peer to receive only DAMAGE_INFO after dummy hit, got %d", len(peerHitQueued))
	}
	assertDamageInfoFrame(t, peerHitQueued[0], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "peer hit after shoot presentation")
}
