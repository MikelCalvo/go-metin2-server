package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowAcceptedCharacterPositionEmitsSelfOnlyChangeSpeed(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SpeedOwner", 0x01030171, 0x02040171, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("SpeedPeer", 0x01030172, 0x02040172, 1120, 2100, 0, 102, 202)
	issuePeerTicket(t, store, "speed-owner", 0x71717171, owner)
	issuePeerTicket(t, store, "speed-peer", 0x72727272, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected change-speed runtime error: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "SpeedPresentationDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected change-speed presentation dummy registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "speed-owner", 0x71717171)
	defer closeSessionFlow(t, ownerFlow)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected owner bootstrap frames with visible training dummy, got %d", len(ownerEnter))
	}
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "speed-peer", 0x72727272)
	defer closeSessionFlow(t, peerFlow)
	if len(peerEnter) != 11 {
		t.Fatalf("expected peer bootstrap frames with owner and visible training dummy, got %d", len(peerEnter))
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before change-speed presentation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before change-speed presentation, got %d", len(selectOut))
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, "SpeedPresentationDummy", targetVID)

	sitOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientCharacterPosition(combatproto.ClientCharacterPositionPacket{Position: bootstrapCharacterPositionSittingGround})))
	if err != nil {
		t.Fatalf("unexpected ground-sit dispatch error: %v", err)
	}
	if len(sitOut) != 1 {
		t.Fatalf("expected ground-sit to emit one self CHARACTER_POSITION frame, got %d", len(sitOut))
	}
	selfPosition, err := worldproto.DecodeCharacterPosition(decodeSingleFrame(t, sitOut[0]))
	if err != nil {
		t.Fatalf("decode self ground-sit presentation: %v", err)
	}
	if selfPosition.VID != owner.VID || selfPosition.Position != bootstrapCharacterPositionSittingGround {
		t.Fatalf("unexpected self ground-sit presentation: %+v", selfPosition)
	}

	ownerQueued := flushServerFrames(t, ownerFlow)
	if len(ownerQueued) != 1 {
		t.Fatalf("expected one self-only CHANGE_SPEED after accepted sit, got %d", len(ownerQueued))
	}
	selfSpeed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, ownerQueued[0]))
	if err != nil {
		t.Fatalf("decode self CHANGE_SPEED after sit: %v", err)
	}
	if selfSpeed.VID != owner.VID || selfSpeed.MovingSpeed != worldproto.BootstrapCharacterMovingSpeed {
		t.Fatalf("unexpected self CHANGE_SPEED after sit: %+v", selfSpeed)
	}

	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 2 {
		t.Fatalf("expected visible peer to receive CHARACTER_POSITION plus CHANGE_SPEED, got %d", len(peerQueued))
	}
	peerPosition, err := worldproto.DecodeCharacterPosition(decodeSingleFrame(t, peerQueued[0]))
	if err != nil {
		t.Fatalf("decode peer ground-sit presentation: %v", err)
	}
	if peerPosition.VID != owner.VID || peerPosition.Position != bootstrapCharacterPositionSittingGround {
		t.Fatalf("unexpected peer ground-sit presentation: %+v", peerPosition)
	}
	peerSpeed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, peerQueued[1]))
	if err != nil {
		t.Fatalf("decode peer CHANGE_SPEED after sit: %v", err)
	}
	if peerSpeed.VID != owner.VID || peerSpeed.MovingSpeed != worldproto.BootstrapCharacterMovingSpeed {
		t.Fatalf("unexpected peer CHANGE_SPEED after sit: %+v", peerSpeed)
	}

	duplicateSit, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientCharacterPosition(combatproto.ClientCharacterPositionPacket{Position: bootstrapCharacterPositionSittingGround})))
	if err != nil {
		t.Fatalf("unexpected duplicate sit dispatch error: %v", err)
	}
	if len(duplicateSit) != 0 {
		t.Fatalf("expected duplicate sit to no-op, got %d frames", len(duplicateSit))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected duplicate sit to queue no CHANGE_SPEED, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected duplicate sit to queue no peer frames, got %d", len(queued))
	}

	chairOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientCharacterPosition(combatproto.ClientCharacterPositionPacket{Position: bootstrapCharacterPositionSittingChair})))
	if err != nil {
		t.Fatalf("unexpected duplicate chair dispatch error: %v", err)
	}
	if len(chairOut) != 0 {
		t.Fatalf("expected chair request while already sitting to no-op, got %d frames", len(chairOut))
	}

	standOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientCharacterPosition(combatproto.ClientCharacterPositionPacket{Position: bootstrapCharacterPositionGeneral})))
	if err != nil {
		t.Fatalf("unexpected stand dispatch error: %v", err)
	}
	if len(standOut) != 1 {
		t.Fatalf("expected stand-after-sit to emit one self CHARACTER_POSITION frame, got %d", len(standOut))
	}
	stand, err := worldproto.DecodeCharacterPosition(decodeSingleFrame(t, standOut[0]))
	if err != nil {
		t.Fatalf("decode stand-after-sit presentation: %v", err)
	}
	if stand.VID != owner.VID || stand.Position != bootstrapCharacterPositionGeneral {
		t.Fatalf("unexpected stand-after-sit presentation: %+v", stand)
	}
	standQueued := flushServerFrames(t, ownerFlow)
	if len(standQueued) != 1 {
		t.Fatalf("expected one self-only CHANGE_SPEED after stand, got %d", len(standQueued))
	}
	standSpeed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, standQueued[0]))
	if err != nil {
		t.Fatalf("decode self CHANGE_SPEED after stand: %v", err)
	}
	if standSpeed.VID != owner.VID || standSpeed.MovingSpeed != worldproto.BootstrapCharacterMovingSpeed {
		t.Fatalf("unexpected self CHANGE_SPEED after stand: %+v", standSpeed)
	}
	peerStand := flushServerFrames(t, peerFlow)
	if len(peerStand) != 2 {
		t.Fatalf("expected visible peer to receive stand CHARACTER_POSITION plus CHANGE_SPEED, got %d", len(peerStand))
	}
	peerStandPosition, err := worldproto.DecodeCharacterPosition(decodeSingleFrame(t, peerStand[0]))
	if err != nil {
		t.Fatalf("decode peer stand presentation: %v", err)
	}
	if peerStandPosition.VID != owner.VID || peerStandPosition.Position != bootstrapCharacterPositionGeneral {
		t.Fatalf("unexpected peer stand presentation: %+v", peerStandPosition)
	}
	peerStandSpeed, err := worldproto.DecodeChangeSpeed(decodeSingleFrame(t, peerStand[1]))
	if err != nil {
		t.Fatalf("decode peer CHANGE_SPEED after stand: %v", err)
	}
	if peerStandSpeed.VID != owner.VID || peerStandSpeed.MovingSpeed != worldproto.BootstrapCharacterMovingSpeed {
		t.Fatalf("unexpected peer CHANGE_SPEED after stand: %+v", peerStandSpeed)
	}

	unsupported, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientCharacterPosition(combatproto.ClientCharacterPositionPacket{Position: 1})))
	if err != nil {
		t.Fatalf("unexpected unsupported battle-position dispatch error: %v", err)
	}
	if len(unsupported) != 0 {
		t.Fatalf("expected unsupported battle-position to fail closed, got %d frames", len(unsupported))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected unsupported battle-position to queue no CHANGE_SPEED, got %d", len(queued))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected attack error after change-speed presentation: %v", err)
	}
	if len(attackOut) != 2 {
		t.Fatalf("expected first normal attack after change-speed presentation to return target refresh plus damage-info, got %d", len(attackOut))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, attackOut[0]))
	if err != nil {
		t.Fatalf("decode target refresh after change-speed presentation: %v", err)
	}
	if refresh.TargetVID != targetVID || refresh.HPPercent != 90 {
		t.Fatalf("expected change-speed presentation not to mutate combat HP before first normal hit, got %+v", refresh)
	}
}
