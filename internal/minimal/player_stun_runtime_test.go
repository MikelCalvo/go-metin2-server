package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowAcceptedSittingDummyHitEmitsSelfOnlyStun(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("StunOwner", 0x01030181, 0x02040181, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("StunPeer", 0x01030182, 0x02040182, 1120, 2100, 0, 102, 202)
	issuePeerTicket(t, store, "stun-owner", 0x81818181, owner)
	issuePeerTicket(t, store, "stun-peer", 0x82828282, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected stun runtime error: %v", err)
	}
	currentTime := time.Unix(1700000600, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "StunPresentationDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected stun presentation dummy registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "stun-owner", 0x81818181)
	defer closeSessionFlow(t, ownerFlow)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected owner bootstrap frames with visible training dummy, got %d", len(ownerEnter))
	}
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "stun-peer", 0x82828282)
	defer closeSessionFlow(t, peerFlow)
	if len(peerEnter) != 11 {
		t.Fatalf("expected peer bootstrap frames with owner and visible training dummy, got %d", len(peerEnter))
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before stun presentation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before stun presentation, got %d", len(selectOut))
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, "StunPresentationDummy", targetVID)

	sitOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientCharacterPosition(combatproto.ClientCharacterPositionPacket{Position: bootstrapCharacterPositionSittingGround})))
	if err != nil {
		t.Fatalf("unexpected ground-sit dispatch error: %v", err)
	}
	if len(sitOut) != 1 {
		t.Fatalf("expected ground-sit to emit one self CHARACTER_POSITION frame, got %d", len(sitOut))
	}
	ownerSitQueued := flushServerFrames(t, ownerFlow)
	if len(ownerSitQueued) != 1 {
		t.Fatalf("expected one self-only CHANGE_SPEED after sit and no STUN before the hit, got %d", len(ownerSitQueued))
	}
	if _, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, ownerSitQueued[0])); err != nil {
		t.Fatalf("decode self CHANGE_SPEED after sit: %v", err)
	}
	peerSitQueued := flushServerFrames(t, peerFlow)
	if len(peerSitQueued) != 2 {
		t.Fatalf("expected visible peer to receive CHARACTER_POSITION plus CHANGE_SPEED after sit, got %d", len(peerSitQueued))
	}
	if _, err := worldproto.DecodeCharacterPosition(decodeSingleFrame(t, peerSitQueued[0])); err != nil {
		t.Fatalf("decode peer CHARACTER_POSITION after sit: %v", err)
	}
	if _, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, peerSitQueued[1])); err != nil {
		t.Fatalf("decode peer CHANGE_SPEED after sit: %v", err)
	}

	unsupported, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientUseSkill(combatproto.ClientUseSkillPacket{SkillVnum: 0x23, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected unsupported use-skill error while sitting: %v", err)
	}
	if len(unsupported) != 0 {
		t.Fatalf("expected unsupported sitting use-skill to stay fail-closed, got %d frames", len(unsupported))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected unsupported sitting use-skill to queue no STUN, got %d", len(queued))
	}

	skillOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientUseSkill(combatproto.ClientUseSkillPacket{SkillVnum: bootstrapUseSkillPresentationVnum, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected sitting use-skill error: %v", err)
	}
	if len(skillOut) != 1 {
		t.Fatalf("expected sitting use-skill to emit one self-only CREATE_FLY, got %d", len(skillOut))
	}
	sittingFly, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, skillOut[0]))
	if err != nil {
		t.Fatalf("decode sitting use-skill CREATE_FLY: %v", err)
	}
	if sittingFly.Type != bootstrapCreateFlyType || sittingFly.StartVID != owner.VID || sittingFly.EndVID != targetVID {
		t.Fatalf("unexpected sitting use-skill CREATE_FLY: %+v", sittingFly)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected sitting use-skill to queue no STUN, got %d", len(queued))
	}

	sittingHit, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected sitting dummy attack error: %v", err)
	}
	if len(sittingHit) != 2 {
		t.Fatalf("expected sitting dummy hit to keep target refresh plus damage-info, got %d", len(sittingHit))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, sittingHit[0]))
	if err != nil {
		t.Fatalf("decode sitting dummy target refresh: %v", err)
	}
	if refresh.TargetVID != targetVID || refresh.HPPercent != 90 {
		t.Fatalf("expected sitting stun presentation not to change dummy HP mutation, got %+v", refresh)
	}
	assertDamageInfoFrame(t, sittingHit[1], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "sitting dummy hit")

	ownerStunQueued := flushServerFrames(t, ownerFlow)
	if len(ownerStunQueued) != 1 {
		t.Fatalf("expected one self-only STUN after sitting dummy hit, got %d", len(ownerStunQueued))
	}
	selfStun, err := worldproto.DecodeStun(decodeSingleFrame(t, ownerStunQueued[0]))
	if err != nil {
		t.Fatalf("decode self STUN after sitting dummy hit: %v", err)
	}
	if selfStun.VID != targetVID {
		t.Fatalf("expected self STUN to name the visible dummy %#08x, got %#08x", targetVID, selfStun.VID)
	}

	peerHitQueued := flushServerFrames(t, peerFlow)
	if len(peerHitQueued) != 2 {
		t.Fatalf("expected visible peer to receive DAMAGE_INFO plus STUN after sitting dummy hit, got %d", len(peerHitQueued))
	}
	assertDamageInfoFrame(t, peerHitQueued[0], targetVID, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "sitting dummy peer hit")
	peerStun, err := worldproto.DecodeStun(decodeSingleFrame(t, peerHitQueued[1]))
	if err != nil {
		t.Fatalf("decode peer STUN after sitting dummy hit: %v", err)
	}
	if peerStun.VID != targetVID {
		t.Fatalf("expected peer STUN to name the visible dummy %#08x, got %#08x", targetVID, peerStun.VID)
	}

	floorPeer := peerVisibilityCharacter("StunFloorPeer", 0x01030183, 0x02040183, 1140, 2100, 0, 103, 203)
	floorPeer.Points[bootstrapPlayerPointValueIndex] = 0
	issuePeerTicket(t, store, "stun-floor-peer", 0x83838383, floorPeer)
	floorFlow, floorEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "stun-floor-peer", 0x83838383)
	defer closeSessionFlow(t, floorFlow)
	if len(floorEnter) == 0 {
		t.Fatal("expected floor peer to enter")
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)
	flushServerFrames(t, floorFlow)

	currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
	repeatSittingHit, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected second sitting dummy attack error: %v", err)
	}
	if len(repeatSittingHit) != 2 {
		t.Fatalf("expected second sitting dummy hit to keep target refresh plus damage-info, got %d", len(repeatSittingHit))
	}
	ownerRepeatQueued := flushServerFrames(t, ownerFlow)
	if len(ownerRepeatQueued) != 1 {
		t.Fatalf("expected one self STUN after second sitting dummy hit, got %d", len(ownerRepeatQueued))
	}
	if _, err := worldproto.DecodeStun(decodeSingleFrame(t, ownerRepeatQueued[0])); err != nil {
		t.Fatalf("decode self STUN after second sitting dummy hit: %v", err)
	}
	livePeerRepeat := flushServerFrames(t, peerFlow)
	if len(livePeerRepeat) != 2 {
		t.Fatalf("expected live peer DAMAGE_INFO plus STUN after second sitting dummy hit, got %d", len(livePeerRepeat))
	}
	if _, err := worldproto.DecodeStun(decodeSingleFrame(t, livePeerRepeat[1])); err != nil {
		t.Fatalf("decode live peer STUN after second sitting dummy hit: %v", err)
	}
	if floorQueued := flushServerFrames(t, floorFlow); len(floorQueued) != 0 {
		t.Fatalf("expected zero-HP peer to receive no sitting dummy STUN, got %d", len(floorQueued))
	}

	deniedRepeat, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected cadence-denied sitting attack error: %v", err)
	}
	if len(deniedRepeat) != 0 {
		t.Fatalf("expected cadence-denied sitting attack to fail closed, got %d frames", len(deniedRepeat))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected cadence-denied sitting attack to queue no STUN, got %d", len(queued))
	}

	currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
	standOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientCharacterPosition(combatproto.ClientCharacterPositionPacket{Position: bootstrapCharacterPositionGeneral})))
	if err != nil {
		t.Fatalf("unexpected stand dispatch error: %v", err)
	}
	if len(standOut) != 1 {
		t.Fatalf("expected stand-after-sit to emit one self CHARACTER_POSITION frame, got %d", len(standOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 1 {
		t.Fatalf("expected one self-only CHANGE_SPEED after stand and no STUN, got %d", len(queued))
	}
	peerStandQueued := flushServerFrames(t, peerFlow)
	if len(peerStandQueued) != 2 {
		t.Fatalf("expected visible peer to receive stand CHARACTER_POSITION plus CHANGE_SPEED, got %d", len(peerStandQueued))
	}
	if _, err := worldproto.DecodeCharacterPosition(decodeSingleFrame(t, peerStandQueued[0])); err != nil {
		t.Fatalf("decode peer stand CHARACTER_POSITION: %v", err)
	}
	if _, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, peerStandQueued[1])); err != nil {
		t.Fatalf("decode peer CHANGE_SPEED after stand: %v", err)
	}

	standingHit, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected standing dummy attack error: %v", err)
	}
	if len(standingHit) != 2 {
		t.Fatalf("expected standing dummy hit to keep target refresh plus damage-info, got %d", len(standingHit))
	}
	standingRefresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, standingHit[0]))
	if err != nil {
		t.Fatalf("decode standing dummy target refresh: %v", err)
	}
	if standingRefresh.TargetVID != targetVID || standingRefresh.HPPercent != 70 {
		t.Fatalf("expected standing hit after sitting stun to continue ordinary dummy HP mutation, got %+v", standingRefresh)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected standing dummy hit to queue no STUN, got %d", len(queued))
	}
	peerStandingQueued := flushServerFrames(t, peerFlow)
	if len(peerStandingQueued) != 1 {
		t.Fatalf("expected visible peer to receive only DAMAGE_INFO after standing dummy hit, got %d", len(peerStandingQueued))
	}
}
