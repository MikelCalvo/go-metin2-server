package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
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

	unsupported, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientUseSkill(combatproto.ClientUseSkillPacket{SkillVnum: 0x23, TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected unsupported use-skill error before fly presentation: %v", err)
	}
	if len(unsupported) != 0 {
		t.Fatalf("expected unsupported use-skill to stay fail-closed, got %d frames", len(unsupported))
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
	if len(flyOut) != 2 {
		t.Fatalf("expected self FLY_TARGETING echo plus CREATE_FLY after accepted fly-targeting, got %d", len(flyOut))
	}
	selfEcho, err := combatproto.DecodeServerFlyTargeting(decodeSingleFrame(t, flyOut[0]))
	if err != nil {
		t.Fatalf("decode self FLY_TARGETING echo after accepted fly-targeting: %v", err)
	}
	if selfEcho.ShooterVID != owner.VID || selfEcho.TargetVID != targetVID || selfEcho.X != 123456 || selfEcho.Y != -234567 {
		t.Fatalf("unexpected self FLY_TARGETING echo after accepted fly-targeting: %+v", selfEcho)
	}
	selfFly, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, flyOut[1]))
	if err != nil {
		t.Fatalf("decode self CREATE_FLY after accepted fly-targeting: %v", err)
	}
	if selfFly.Type != bootstrapCreateFlyType || selfFly.StartVID != owner.VID || selfFly.EndVID != targetVID {
		t.Fatalf("unexpected self CREATE_FLY after accepted fly-targeting: %+v", selfFly)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected accepted fly-targeting not to queue extra owner frames, got %d", len(queued))
	}
	peerFlyQueued := flushServerFrames(t, peerFlow)
	if len(peerFlyQueued) != 1 {
		t.Fatalf("expected visible live peer to receive one CREATE_FLY, got %d", len(peerFlyQueued))
	}
	peerFly, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, peerFlyQueued[0]))
	if err != nil {
		t.Fatalf("decode peer CREATE_FLY after accepted fly-targeting: %v", err)
	}
	if peerFly != selfFly {
		t.Fatalf("expected peer CREATE_FLY to match self %+v, got %+v", selfFly, peerFly)
	}

	floorPeer := peerVisibilityCharacter("FlyFloorPeer", 0x01030193, 0x02040193, 1140, 2100, 0, 103, 203)
	floorPeer.Points[bootstrapPlayerPointValueIndex] = 0
	issuePeerTicket(t, store, "fly-floor-peer", 0x93939393, floorPeer)
	floorFlow, floorEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "fly-floor-peer", 0x93939393)
	defer closeSessionFlow(t, floorFlow)
	if len(floorEnter) == 0 {
		t.Fatal("expected floor peer to enter")
	}
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)
	flushServerFrames(t, floorFlow)

	repeatFly, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID, X: 0, Y: 0})))
	if err != nil {
		t.Fatalf("unexpected repeat fly-targeting error: %v", err)
	}
	if len(repeatFly) != 2 {
		t.Fatalf("expected repeat accepted fly-targeting to emit one FLY_TARGETING echo plus one CREATE_FLY, got %d", len(repeatFly))
	}
	repeatEcho, err := combatproto.DecodeServerFlyTargeting(decodeSingleFrame(t, repeatFly[0]))
	if err != nil {
		t.Fatalf("decode repeat FLY_TARGETING echo: %v", err)
	}
	if repeatEcho.ShooterVID != owner.VID || repeatEcho.TargetVID != targetVID || repeatEcho.X != 0 || repeatEcho.Y != 0 {
		t.Fatalf("unexpected repeat FLY_TARGETING echo: %+v", repeatEcho)
	}
	repeatDecoded, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, repeatFly[1]))
	if err != nil {
		t.Fatalf("decode repeat CREATE_FLY: %v", err)
	}
	if repeatDecoded.Type != bootstrapCreateFlyType || repeatDecoded.StartVID != owner.VID || repeatDecoded.EndVID != targetVID {
		t.Fatalf("unexpected repeat CREATE_FLY: %+v", repeatDecoded)
	}
	livePeerRepeat := flushServerFrames(t, peerFlow)
	if len(livePeerRepeat) != 1 {
		t.Fatalf("expected live peer to receive one repeat CREATE_FLY, got %d", len(livePeerRepeat))
	}
	if _, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, livePeerRepeat[0])); err != nil {
		t.Fatalf("decode live peer repeat CREATE_FLY: %v", err)
	}
	if floorQueued := flushServerFrames(t, floorFlow); len(floorQueued) != 0 {
		t.Fatalf("expected zero-HP peer to receive no CREATE_FLY, got %d", len(floorQueued))
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

func TestGameSessionFlowSelectedKillingHitEmitsSelfOnlyCreateFlyBeforeReward(t *testing.T) {
	const profile = "fly_killing_hit_profile"
	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP: 2, DamagePerNormalAttack: 1,
		RespawnDelay: worldruntime.PracticeMobBootstrapRespawnDelay,
		DeathReward:  worldruntime.StaticActorDeathReward{Experience: 7},
	}) {
		t.Fatal("register killing-hit fly test profile")
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("FlyKiller", 0x01030194, 0x02040194, 1100, 2100, 0, 101, 201)
	peer := peerVisibilityCharacter("FlyWitness", 0x01030195, 0x02040195, 1120, 2100, 0, 102, 202)
	issuePeerTicket(t, store, "fly-killer", 0x94949494, owner)
	issuePeerTicket(t, store, "fly-witness", 0x95959595, peer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "fly-killer", Empire: owner.Empire, Characters: []loginticket.Character{owner}}); err != nil {
		t.Fatalf("seed reward account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("new fly runtime: %v", err)
	}
	now := time.Unix(1_700_000_800, 0)
	runtime.now = func() time.Time { return now }
	actor, ok := runtime.sharedWorld.registerStaticActor(0, "FlyKillMob", bootstrapMapIndex, 1200, 2200, 20350, "", "", profile, "fly.kill.mob", worldruntime.StaticActorDeathReward{})
	if !ok {
		t.Fatal("register fly killing-hit actor")
	}
	targetVID := uint32(actor.EntityID)
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "fly-killer", 0x94949494)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "fly-witness", 0x95959595)
	defer closeSessionFlow(t, peerFlow)
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, peerFlow)

	attack := func(attackType uint8) [][]byte {
		t.Helper()
		out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: attackType, TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("attack fly mob: %v", err)
		}
		return out
	}
	if out := attack(combatproto.ClientAttackTypeNormal); len(out) != 0 {
		t.Fatalf("unselected attack must fail closed, got %d frames", len(out))
	}
	if selected, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil || len(selected) != 1 {
		t.Fatalf("select fly mob: %d frames, %v", len(selected), err)
	}
	flushSelfOnlyTargetCreateNew(t, ownerFlow, "FlyKillMob", targetVID)
	if out := attack(99); len(out) != 0 {
		t.Fatalf("unsupported attack must fail closed, got %d frames", len(out))
	}
	for _, raw := range [][]byte{
		combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID + 1}),
		combatproto.EncodeClientAddFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID}),
	} {
		out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, raw))
		if err != nil || len(out) != 0 {
			t.Fatalf("unsupported fly intent must fail closed: frames=%d err=%v", len(out), err)
		}
	}
	if extra := flushServerFrames(t, peerFlow); len(extra) != 0 {
		t.Fatalf("unsupported fly intents must not queue peer frames, got %d", len(extra))
	}
	if out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID, X: 1200, Y: 2200}))); err != nil || len(out) != 2 {
		t.Fatalf("arm selected-target fly intent: frames=%d err=%v", len(out), err)
	}
	flushServerFrames(t, peerFlow) // pre-hit FLY_TARGETING fanout is already owned
	first := attack(combatproto.ClientAttackTypeNormal)
	if len(first) < 1 {
		t.Fatal("non-lethal hit missing TARGET")
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, first[0]))
	if err != nil || refresh.TargetVID != targetVID || refresh.HPPercent != 50 {
		t.Fatalf("non-lethal hit refresh: %+v, %v", refresh, err)
	}
	for _, raw := range first {
		if _, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, raw)); err == nil {
			t.Fatal("non-lethal hit emitted CREATE_FLY")
		}
	}
	flushServerFrames(t, peerFlow)
	flushServerFrames(t, ownerFlow)
	if out := attack(combatproto.ClientAttackTypeNormal); len(out) != 0 {
		t.Fatalf("cadence-denied attack must fail closed, got %d frames", len(out))
	}
	if out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID, X: 1200, Y: 2200}))); err != nil || len(out) != 2 {
		t.Fatalf("arm killing-hit selected-target fly: frames=%d err=%v", len(out), err)
	}
	flushServerFrames(t, peerFlow)
	now = now.Add(bootstrapNormalAttackCadenceWindow)
	kill := attack(combatproto.ClientAttackTypeNormal)
	if len(kill) != 5 {
		t.Fatalf("expected DEAD, TARGET clear, DAMAGE_INFO, CREATE_FLY, EXP reward; got %d", len(kill))
	}
	dead, err := worldproto.DecodeDead(decodeSingleFrame(t, kill[0]))
	if err != nil || dead.VID != targetVID {
		t.Fatalf("killing-hit DEAD: %+v, %v", dead, err)
	}
	clear, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, kill[1]))
	if err != nil || clear.TargetVID != 0 || clear.HPPercent != 0 {
		t.Fatalf("killing-hit TARGET clear: %+v, %v", clear, err)
	}
	assertDamageInfoFrame(t, kill[2], targetVID, 1, "fly killing hit")
	fly, err := combatproto.DecodeServerCreateFly(decodeSingleFrame(t, kill[3]))
	if err != nil || fly.Type != 0 || fly.StartVID != owner.VID || fly.EndVID != targetVID {
		t.Fatalf("killing-hit fly: %+v, %v", fly, err)
	}
	reward, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, kill[4]))
	if err != nil || reward.VID != owner.VID || reward.Type != bootstrapExperiencePointType || reward.Amount != 7 {
		t.Fatalf("reward after fly: %+v, %v", reward, err)
	}
	peerKill := flushServerFrames(t, peerFlow)
	if len(peerKill) != 2 {
		t.Fatalf("peer kill must keep DEAD + DAMAGE_INFO without fly, got %d frames", len(peerKill))
	}
	if peerDead, err := worldproto.DecodeDead(decodeSingleFrame(t, peerKill[0])); err != nil || peerDead.VID != targetVID {
		t.Fatalf("peer DEAD: %+v, %v", peerDead, err)
	}
	assertDamageInfoFrame(t, peerKill[1], targetVID, 1, "fly killing peer")
	if extra := flushServerFrames(t, ownerFlow); len(extra) != 0 {
		t.Fatalf("owner must have no queued duplicate fly, got %d", len(extra))
	}
	now = now.Add(bootstrapNormalAttackCadenceWindow)
	if out := attack(combatproto.ClientAttackTypeNormal); len(out) != 0 {
		t.Fatalf("dead target must fail closed, got %d frames", len(out))
	}
	if out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID}))); err != nil || len(out) != 0 {
		t.Fatalf("fly targeting after death must fail closed: frames=%d err=%v", len(out), err)
	}
	if extra := flushServerFrames(t, peerFlow); len(extra) != 0 {
		t.Fatalf("dead target must not emit peer frames, got %d", len(extra))
	}
}

func TestGameSessionFlowKillingFlyIntentDiscardsOnExplicitTargetClear(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("FlyClearer", 0x01030196, 0x02040196, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "fly-clearer", 0x96969696, owner)
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil)
	if err != nil {
		t.Fatalf("new fly clear runtime: %v", err)
	}
	actor, ok := runtime.sharedWorld.RegisterStaticActorWithCombatKind(0, "FlyClearDummy", bootstrapMapIndex, 1200, 2200, 20350, worldruntime.StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("register fly clear dummy")
	}
	targetVID := uint32(actor.EntityID)
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "fly-clearer", 0x96969696)
	defer closeSessionFlow(t, flow)
	flushServerFrames(t, flow)
	selectTarget := func(vid uint32) {
		t.Helper()
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: vid})))
		if err != nil || (vid != 0 && len(out) != 1) {
			t.Fatalf("select fly target %d: frames=%d err=%v", vid, len(out), err)
		}
		flushServerFrames(t, flow)
	}
	selectTarget(targetVID)
	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: targetVID}))); err != nil || len(out) != 2 {
		t.Fatalf("arm fly before clear: frames=%d err=%v", len(out), err)
	}
	selectTarget(0)
	selectTarget(targetVID)
	now := time.Unix(1_700_000_900, 0)
	runtime.now = func() time.Time { return now }
	for hit := 1; hit <= int(worldruntime.TrainingDummyBootstrapMaxHP/worldruntime.TrainingDummyBootstrapDamagePerNormalAttack); hit++ {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("attack after clear hit %d: %v", hit, err)
		}
		if hit == int(worldruntime.TrainingDummyBootstrapMaxHP/worldruntime.TrainingDummyBootstrapDamagePerNormalAttack) {
			if len(out) != 3 {
				t.Fatalf("unarmed death after explicit clear must keep three frames, got %d", len(out))
			}
			stripTrainingDummyKillingHitDeathPrefix(t, out, targetVID, "cleared fly intent")
		}
		now = now.Add(bootstrapNormalAttackCadenceWindow)
	}
}
