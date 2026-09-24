package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowAcceptedUseSkillEmitsSelfOnlyCreateFly(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SkillOwner", 0x010301A1, 0x020401A1, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("SkillPeer", 0x010301A2, 0x020401A2, 1120, 2100, 0, 102, 202)
	issuePeerTicket(t, store, "skill-owner", 0xA1A1A1A1, owner)
	issuePeerTicket(t, store, "skill-peer", 0xA2A2A2A2, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected skill runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "SkillPresentationDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected skill presentation dummy registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "skill-owner", 0xA1A1A1A1)
	defer closeSessionFlow(t, ownerFlow)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected owner bootstrap frames with visible training dummy, got %d", len(ownerEnter))
	}
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "skill-peer", 0xA2A2A2A2)
	defer closeSessionFlow(t, peerFlow)
	if len(peerEnter) != 11 {
		t.Fatalf("expected peer bootstrap frames with owner and visible training dummy, got %d", len(peerEnter))
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)

	unselected, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientUseSkill(combatproto.ClientUseSkillPacket{SkillVnum: bootstrapUseSkillPresentationVnum, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected unselected use-skill error: %v", err)
	}
	if len(unselected) != 0 {
		t.Fatalf("expected unselected use-skill to stay fail-closed, got %d frames", len(unselected))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected unselected use-skill to queue no CREATE_FLY, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before skill presentation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before skill presentation, got %d", len(selectOut))
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, "SkillPresentationDummy", targetVID)

	unsupported, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientUseSkill(combatproto.ClientUseSkillPacket{SkillVnum: 0x23, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected unsupported use-skill error: %v", err)
	}
	if len(unsupported) != 0 {
		t.Fatalf("expected unsupported skill vnum to stay fail-closed, got %d frames", len(unsupported))
	}

	mismatch, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientUseSkill(combatproto.ClientUseSkillPacket{SkillVnum: bootstrapUseSkillPresentationVnum, TargetVID: targetVID + 1})))
	if err != nil {
		t.Fatalf("unexpected mismatched use-skill error: %v", err)
	}
	if len(mismatch) != 0 {
		t.Fatalf("expected mismatched use-skill to stay fail-closed, got %d frames", len(mismatch))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected unsupported use-skill to queue no CREATE_FLY, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected unsupported use-skill to queue no peer frames, got %d", len(queued))
	}

	skillOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientUseSkill(combatproto.ClientUseSkillPacket{SkillVnum: bootstrapUseSkillPresentationVnum, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected accepted use-skill error: %v", err)
	}
	if len(skillOut) != 1 {
		t.Fatalf("expected one self-only CREATE_FLY after accepted use-skill, got %d", len(skillOut))
	}
	selfFly, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, skillOut[0]))
	if err != nil {
		t.Fatalf("decode self CREATE_FLY after accepted use-skill: %v", err)
	}
	if selfFly.Type != bootstrapCreateFlyType || selfFly.StartVID != owner.VID || selfFly.EndVID != targetVID {
		t.Fatalf("unexpected self CREATE_FLY after accepted use-skill: %+v", selfFly)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected accepted use-skill not to queue extra owner frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected visible peer to receive no CREATE_FLY, got %d", len(queued))
	}

	repeatOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientUseSkill(combatproto.ClientUseSkillPacket{SkillVnum: bootstrapUseSkillPresentationVnum, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected repeat use-skill error: %v", err)
	}
	if len(repeatOut) != 1 {
		t.Fatalf("expected repeat accepted use-skill to emit one CREATE_FLY, got %d", len(repeatOut))
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
		t.Fatalf("unexpected attack error after skill presentation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected first normal attack after skill presentation to return target refresh plus damage-info, got %d", len(attackOut))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
	if err != nil {
		t.Fatalf("decode target refresh after skill presentation: %v", err)
	}
	if refresh.TargetVID != targetVID || refresh.HPPercent != 90 {
		t.Fatalf("expected skill presentation not to mutate combat HP before first normal hit, got %+v", refresh)
	}
	assertDamageInfoFrame(t, attackOut[1], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "attack after skill presentation")
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected ordinary standing dummy hit after skill presentation to queue no CREATE_FLY or STUN, got %d", len(queued))
	}
	peerHitQueued := flushServerFrames(t, peerFlow)
	if len(peerHitQueued) != 1 {
		t.Fatalf("expected visible peer to receive only DAMAGE_INFO after dummy hit, got %d", len(peerHitQueued))
	}
	assertDamageInfoFrame(t, peerHitQueued[0], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "peer hit after skill presentation")
}
