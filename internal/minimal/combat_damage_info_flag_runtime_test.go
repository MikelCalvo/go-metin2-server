package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowStandalonePracticeMobHitEmitsNormalDamageInfoFlag(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("FlagOwner", 0x010301B1, 0x020401B1, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("FlagPeer", 0x010301B2, 0x020401B2, 1120, 2100, 0, 102, 202)
	issuePeerTicket(t, store, "flag-owner", 0xB1B1B1B1, owner)
	issuePeerTicket(t, store, "flag-peer", 0xB2B2B2B2, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected damage-info flag runtime error: %v", err)
	}
	currentTime := time.Unix(1700000700, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "FlagPresentationPracticeMob", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatProfilePracticeMob)
	if !ok {
		t.Fatal("expected standalone practice-mob registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "flag-owner", 0xB1B1B1B1)
	defer closeSessionFlow(t, ownerFlow)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected owner bootstrap frames with visible practice mob, got %d", len(ownerEnter))
	}
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "flag-peer", 0xB2B2B2B2)
	defer closeSessionFlow(t, peerFlow)
	if len(peerEnter) != 11 {
		t.Fatalf("expected peer bootstrap frames with owner and visible practice mob, got %d", len(peerEnter))
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before normal-flag presentation: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before normal-flag presentation, got %d", len(selectOut))
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, "FlagPresentationPracticeMob", targetVID)

	hitOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected standalone practice-mob attack error: %v", err)
	}
	if len(hitOut) != 2 {
		t.Fatalf("expected target refresh plus damage-info for standalone practice-mob hit, got %d", len(hitOut))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, hitOut[0]))
	if err != nil {
		t.Fatalf("decode standalone practice-mob target refresh: %v", err)
	}
	if refresh.TargetVID != targetVID || refresh.HPPercent != 90 {
		t.Fatalf("expected normal-flag presentation not to change practice-mob HP mutation, got %+v", refresh)
	}
	assertDamageInfoPresentationFlag(t, hitOut[1], targetVID, combatproto.ServerDamageInfoFlagCritical|combatproto.ServerDamageInfoFlagNormal, int32(worldruntime.PracticeMobBootstrapDamagePerNormalAttack), "standalone practice-mob self hit")
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected standalone practice-mob hit not to queue extra owner frames, got %d", len(queued))
	}

	peerHitQueued := flushServerFrames(t, peerFlow)
	if len(peerHitQueued) != 1 {
		t.Fatalf("expected visible peer to receive only DAMAGE_INFO after standalone practice-mob hit, got %d", len(peerHitQueued))
	}
	assertDamageInfoPresentationFlag(t, peerHitQueued[0], targetVID, combatproto.ServerDamageInfoFlagCritical|combatproto.ServerDamageInfoFlagNormal, int32(worldruntime.PracticeMobBootstrapDamagePerNormalAttack), "standalone practice-mob peer hit")
}

func TestGameSessionFlowStandaloneDummyHitKeepsPlainDamageInfoFlag(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("FlagDummyOwner", 0x010301B3, 0x020401B3, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("FlagDummyPeer", 0x010301B4, 0x020401B4, 1120, 2100, 0, 102, 202)
	issuePeerTicket(t, store, "flag-dummy-owner", 0xB3B3B3B3, owner)
	issuePeerTicket(t, store, "flag-dummy-peer", 0xB4B4B4B4, peer)

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("unexpected dummy flag runtime error: %v", err)
	}
	currentTime := time.Unix(1700000701, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "FlagPresentationDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected standalone dummy registration to succeed")
	}
	targetVID := uint32(actor.EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "flag-dummy-owner", 0xB3B3B3B3)
	defer closeSessionFlow(t, ownerFlow)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected owner bootstrap frames with visible training dummy, got %d", len(ownerEnter))
	}
	peerFlow, peerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "flag-dummy-peer", 0xB4B4B4B4)
	defer closeSessionFlow(t, peerFlow)
	if len(peerEnter) != 11 {
		t.Fatalf("expected peer bootstrap frames with owner and visible training dummy, got %d", len(peerEnter))
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before dummy flag contrast: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before dummy flag contrast, got %d", len(selectOut))
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, "FlagPresentationDummy", targetVID)

	hitOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected standalone dummy attack error: %v", err)
	}
	if len(hitOut) != 2 {
		t.Fatalf("expected target refresh plus damage-info for standalone dummy hit, got %d", len(hitOut))
	}
	assertDamageInfoPresentationFlag(t, hitOut[1], targetVID, combatproto.ServerDamageInfoFlagNone, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "standalone dummy self hit")

	peerHitQueued := flushServerFrames(t, peerFlow)
	if len(peerHitQueued) != 1 {
		t.Fatalf("expected visible peer to receive only DAMAGE_INFO after dummy hit, got %d", len(peerHitQueued))
	}
	assertDamageInfoPresentationFlag(t, peerHitQueued[0], targetVID, combatproto.ServerDamageInfoFlagNone, int32(worldruntime.TrainingDummyBootstrapDamagePerNormalAttack), "standalone dummy peer hit")
}

func assertDamageInfoPresentationFlag(t *testing.T, raw []byte, targetVID uint32, wantFlag uint8, wantDamage int32, context string) {
	t.Helper()
	damage, err := combatproto.DecodeServerDamageInfo(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode %s damage-info: %v", context, err)
	}
	if damage.VID != targetVID || damage.Flag != wantFlag || damage.Damage != wantDamage {
		t.Fatalf("unexpected %s damage-info: %+v want vid=%#08x flag=%#02x damage=%d", context, damage, targetVID, wantFlag, wantDamage)
	}
}
