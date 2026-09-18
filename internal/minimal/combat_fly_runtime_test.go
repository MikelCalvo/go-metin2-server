package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowAcceptedFlyTargetingEmitsSelfOnlyCreateFly(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("FlyOwner", 0x01030191, 0x02040191, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("FlyPeer", 0x01030192, 0x02040192, 1120, 2100, 0, 102, 202)
	issuePeerTicket(t, store, "fly-owner", 0x91919191, owner)
	issuePeerTicket(t, store, "fly-peer", 0x92929292, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected fly runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "FlyPresentationDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected fly presentation dummy registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "fly-owner", 0x91919191)
	defer closeSessionFlow(t, ownerFlow)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected owner bootstrap frames with visible training dummy, got %d", len(ownerEnter))
	}
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "fly-peer", 0x92929292)
	defer closeSessionFlow(t, peerFlow)
	if len(peerEnter) != 11 {
		t.Fatalf("expected peer bootstrap frames with owner and visible training dummy, got %d", len(peerEnter))
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)

	unselected, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID, X: 123456, Y: -234567})))
	if err != nil {
		t.Fatalf("unexpected unselected fly-targeting error: %v", err)
	}
	if len(unselected) != 0 {
		t.Fatalf("expected unselected fly-targeting to stay fail-closed, got %d frames", len(unselected))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected unselected fly-targeting to queue no CREATE_FLY, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before fly presentation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before fly presentation, got %d", len(selectOut))
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, "FlyPresentationDummy", targetVID)

	mismatch, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID + 1, X: 123456, Y: -234567})))
	if err != nil {
		t.Fatalf("unexpected mismatched fly-targeting error: %v", err)
	}
	if len(mismatch) != 0 {
		t.Fatalf("expected mismatched fly-targeting to stay fail-closed, got %d frames", len(mismatch))
	}

	addFly, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAddFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID, X: 1700, Y: -2800})))
	if err != nil {
		t.Fatalf("unexpected add-fly-targeting guard error: %v", err)
	}
	if len(addFly) != 0 {
		t.Fatalf("expected selected add-fly-targeting to stay fail-closed, got %d frames", len(addFly))
	}

	shootOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientShoot(combatproto.ClientShootPacket{ShootType: 0x83})))
	if err != nil {
		t.Fatalf("unexpected shoot guard error before fly presentation: %v", err)
	}
	if len(shootOut) != 0 {
		t.Fatalf("expected selected shoot to stay fail-closed, got %d frames", len(shootOut))
	}

	skillOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientUseSkill(combatproto.ClientUseSkillPacket{SkillVnum: 0x23, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected use-skill guard error before fly presentation: %v", err)
	}
	if len(skillOut) != 0 {
		t.Fatalf("expected selected use-skill to stay fail-closed, got %d frames", len(skillOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected fail-closed projectile/skill guards to queue no CREATE_FLY, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected fail-closed projectile/skill guards to queue no peer frames, got %d", len(queued))
	}

	flyOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID, X: 123456, Y: -234567})))
	if err != nil {
		t.Fatalf("unexpected accepted fly-targeting error: %v", err)
	}
	if len(flyOut) != 1 {
		t.Fatalf("expected one self-only CREATE_FLY after accepted fly-targeting, got %d", len(flyOut))
	}
	selfFly, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, flyOut[0]))
	if err != nil {
		t.Fatalf("decode self CREATE_FLY after accepted fly-targeting: %v", err)
	}
	if selfFly.Type != bootstrapCreateFlyType || selfFly.StartVID != owner.VID || selfFly.EndVID != targetVID {
		t.Fatalf("unexpected self CREATE_FLY after accepted fly-targeting: %+v", selfFly)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected accepted fly-targeting not to queue extra owner frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected visible peer to receive no CREATE_FLY, got %d", len(queued))
	}

	repeatFly, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID, X: 0, Y: 0})))
	if err != nil {
		t.Fatalf("unexpected repeat fly-targeting error: %v", err)
	}
	if len(repeatFly) != 1 {
		t.Fatalf("expected repeat accepted fly-targeting to emit one CREATE_FLY, got %d", len(repeatFly))
	}
	repeatDecoded, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, repeatFly[0]))
	if err != nil {
		t.Fatalf("decode repeat CREATE_FLY: %v", err)
	}
	if repeatDecoded.Type != bootstrapCreateFlyType || repeatDecoded.StartVID != owner.VID || repeatDecoded.EndVID != targetVID {
		t.Fatalf("unexpected repeat CREATE_FLY: %+v", repeatDecoded)
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error after fly presentation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected first normal attack after fly presentation to return target refresh plus damage-info, got %d", len(attackOut))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
	if err != nil {
		t.Fatalf("decode target refresh after fly presentation: %v", err)
	}
	if refresh.TargetVID != targetVID || refresh.HPPercent != 90 {
		t.Fatalf("expected fly presentation not to mutate combat HP before first normal hit, got %+v", refresh)
	}
	assertDamageInfoFrame(t, attackOut[1], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "attack after fly presentation")
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected ordinary standing dummy hit after fly presentation to queue no CREATE_FLY, got %d", len(queued))
	}
	peerHitQueued := flushServerFrames(t, peerFlow)
	if len(peerHitQueued) != 1 {
		t.Fatalf("expected visible peer to receive only DAMAGE_INFO after dummy hit, got %d", len(peerHitQueued))
	}
	assertDamageInfoFrame(t, peerHitQueued[0], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "peer hit after fly presentation")
}
