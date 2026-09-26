package minimal

import (
	"hash/fnv"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/player"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestKillRewardPartyOwnerSlotUsesDeterministicFNV1a(t *testing.T) {
	const vnum uint32 = 27001
	digest := fnv.New64a()
	_, _ = digest.Write([]byte("kill_reward_party_owner:27001:0"))
	if got, want := killRewardPartyOwnerSlot(vnum, 0, 2), int(digest.Sum64()%2); got != want {
		t.Fatalf("expected FNV-1a slot %d for vnum %d among 2 members, got %d", want, vnum, got)
	}
	if got := killRewardPartyOwnerName([]string{"ZuluPartyKiller", "AlphaPartyMate"}, vnum, 0); got != "AlphaPartyMate" {
		t.Fatalf("expected sorted slot 0 to pick AlphaPartyMate, got %q", got)
	}
	if got := killRewardPartyOwnerName([]string{"SoloKiller"}, vnum, 0); got != "SoloKiller" {
		t.Fatalf("expected a party of one to keep the killer, got %q", got)
	}
}

func TestGameRuntimeImplicitPartyFNV1aPicksLiveMateForSingleKillRewardDrop(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	actor := worldruntime.StaticEntity{
		Entity:        worldruntime.Entity{ID: 0x01050270, Kind: worldruntime.EntityKindStaticActor, VID: 0x01050270, Name: "PartyOwnerRewardMob"},
		Position:      worldruntime.NewPosition(bootstrapMapIndex, 1200, 2200),
		RaceNum:       20350,
		CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy,
		CombatKind:    worldruntime.StaticActorCombatKindTrainingDummy,
		SpawnGroupRef: "practice.party_owner_reward_mob",
	}
	killer := peerVisibilityCharacter("ZuluPartyKiller", 0x01030170, 0x02040170, 1100, 2100, 0, 101, 201)
	mate := peerVisibilityCharacter("AlphaPartyMate", 0x01030171, 0x02040171, 1120, 2120, 1, 102, 202)
	dead := peerVisibilityCharacter("AardvarkDeadMate", 0x01030172, 0x02040172, 1110, 2110, 2, 103, 203)
	dead.Points[bootstrapPlayerPointValueIndex] = 0
	issuePeerTicket(t, store, "party-owner-killer", 0x70707070, killer)
	issuePeerTicket(t, store, "party-owner-mate", 0x71717171, mate)
	issuePeerTicket(t, store, "party-owner-dead", 0x72727272, dead)

	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "party-owner-killer", Empire: killer.Empire, Characters: []loginticket.Character{killer}}); err != nil {
		t.Fatalf("seed party-owner killer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "party-owner-mate", Empire: mate.Empire, Characters: []loginticket.Character{mate}}); err != nil {
		t.Fatalf("seed party-owner mate account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "party-owner-dead", Empire: dead.Empire, Characters: []loginticket.Character{dead}}); err != nil {
		t.Fatalf("seed party-owner dead account: %v", err)
	}
	currentTime := time.Unix(1_700_000_770, 0)
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("new game runtime: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	if _, ok := runtime.sharedWorld.registerStaticActor(actor.Entity.ID, actor.Entity.Name, actor.Position.MapIndex, actor.Position.X, actor.Position.Y, actor.RaceNum, "", "", actor.CombatKind, actor.SpawnGroupRef, worldruntime.StaticActorDeathReward{}); !ok {
		t.Fatal("expected party-owner reward mob registration to succeed")
	}
	if !runtime.sharedWorld.overrideStaticActorDeathReward(actor.Entity.ID, worldruntime.StaticActorDeathReward{DropVnums: []uint32{27001}}) {
		t.Fatal("expected party-owner reward override to apply")
	}

	killerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "party-owner-killer", 0x70707070)
	defer closeSessionFlow(t, killerFlow)
	mateFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "party-owner-mate", 0x71717171)
	defer closeSessionFlow(t, mateFlow)
	deadFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "party-owner-dead", 0x72727272)
	defer closeSessionFlow(t, deadFlow)
	flushServerFrames(t, killerFlow)
	flushServerFrames(t, mateFlow)
	flushServerFrames(t, deadFlow)

	wantOwner := killRewardPartyOwnerName([]string{killer.Name, mate.Name}, 27001, 0)
	if wantOwner != mate.Name {
		t.Fatalf("expected live implicit-party FNV-1a to pick mate %q, got %q", mate.Name, wantOwner)
	}
	if skipped := killRewardPartyOwnerName([]string{dead.Name, killer.Name, mate.Name}, 27001, 0); skipped != dead.Name {
		t.Fatalf("expected this fixture's 0-HP mate to win a naive name-only roll so skip-0-HP is load-bearing, got %q", skipped)
	}

	targetVID := uint32(actor.Entity.ID)
	if out, err := killerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected target selection before party-owner kill to return 1 frame, got frames=%d err=%v", len(out), err)
	}

	var killOut [][]byte
	for hit := 1; hit <= int(worldruntime.TrainingDummyBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		killOut, err = killerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("unexpected party-owner attack error on hit %d: %v", hit, err)
		}
	}
	if len(killOut) != 5 {
		t.Fatalf("expected killing hit to return dead, clear target, ground-add, and ownership frames, got %d", len(killOut))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, killOut[3]))
	if err != nil {
		t.Fatalf("decode party-owner ground add: %v", err)
	}
	if ground.Vnum != 27001 || ground.X != killer.X || ground.Y != killer.Y || ground.Z != killer.Z {
		t.Fatalf("expected party-owner drop to stay at killer feet, got %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, killOut[4]))
	if err != nil {
		t.Fatalf("decode party-owner ownership: %v", err)
	}
	if ownership.VID != ground.VID || ownership.OwnerName != mate.Name {
		t.Fatalf("expected ITEM_OWNERSHIP.owner_name to be FNV-1a mate %q, got %+v", mate.Name, ownership)
	}

	snapshot, ok := runtime.GroundItem(ground.VID)
	if !ok || snapshot.OwnerName != mate.Name || snapshot.OwnerLogin != "party-owner-mate" || snapshot.OwnerVID != mate.VID || snapshot.X != killer.X || snapshot.Y != killer.Y {
		t.Fatalf("expected registered exclusive owner to be live mate at killer feet, got snapshot=%+v ok=%v", snapshot, ok)
	}
	mateEntity, mateOK := runtime.sharedWorld.playerEntityByName(mate.Name)
	registered, registeredOK := runtime.sharedWorld.groundItemsByVID[ground.VID]
	if !mateOK || !registeredOK || registered.OwnerID != mateEntity.Entity.ID {
		t.Fatalf("expected process-local OwnerID to be live mate, got ownerID=%d mateOK=%v registeredOK=%v mate=%+v", registered.OwnerID, mateOK, registeredOK, mateEntity)
	}

	if pickupOut := pickupGroundItem(t, killerFlow, ground.VID); len(pickupOut) != 0 {
		t.Fatalf("expected exclusive mate ownership to reject killer pickup, got %d frames", len(pickupOut))
	}
	if pickupOut := pickupGroundItem(t, deadFlow, ground.VID); len(pickupOut) != 0 {
		t.Fatalf("expected 0-HP implicit-party skip to reject dead-mate pickup, got %d frames", len(pickupOut))
	}

	pickupOut := pickupGroundItem(t, mateFlow, ground.VID)
	if len(pickupOut) != 3 {
		t.Fatalf("expected FNV-1a mate exclusive pickup to emit GROUND_DEL, ITEM_SET, and ITEM_GET, got %d frames", len(pickupOut))
	}
	if del, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, pickupOut[0])); err != nil || del.VID != ground.VID {
		t.Fatalf("unexpected party-owner ground delete: del=%+v err=%v", del, err)
	}
	if runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected mate exclusive pickup to remove the kill-reward handle")
	}

	mateAccount, err := accounts.Load("party-owner-mate")
	if err != nil {
		t.Fatalf("load party-owner mate account: %v", err)
	}
	if len(mateAccount.Characters) != 1 || len(mateAccount.Characters[0].Inventory) != 1 || mateAccount.Characters[0].Inventory[0].Vnum != 27001 {
		t.Fatalf("expected mate inventory to persist FNV-1a kill-reward pickup, got %#v", mateAccount.Characters)
	}
	killerAccount, err := accounts.Load("party-owner-killer")
	if err != nil {
		t.Fatalf("load party-owner killer account: %v", err)
	}
	if len(killerAccount.Characters) != 1 || len(killerAccount.Characters[0].Inventory) != 0 {
		t.Fatalf("expected killer inventory to remain empty after mate pickup, got %#v", killerAccount.Characters)
	}
}

func TestGameRuntimeMultiDropKillRewardKeepsKillerOwnership(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	actor := worldruntime.StaticEntity{
		Entity:        worldruntime.Entity{ID: 0x01050271, Kind: worldruntime.EntityKindStaticActor, VID: 0x01050271, Name: "PartyOwnerMultiDropMob"},
		Position:      worldruntime.NewPosition(bootstrapMapIndex, 1200, 2200),
		RaceNum:       20350,
		CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy,
		CombatKind:    worldruntime.StaticActorCombatKindTrainingDummy,
		SpawnGroupRef: "practice.party_owner_multi_drop_mob",
	}
	killer := peerVisibilityCharacter("ZuluMultiKiller", 0x01030173, 0x02040173, 1100, 2100, 0, 101, 201)
	mate := peerVisibilityCharacter("AlphaMultiMate", 0x01030174, 0x02040174, 1120, 2120, 1, 102, 202)
	issuePeerTicket(t, store, "party-multi-killer", 0x73737373, killer)
	issuePeerTicket(t, store, "party-multi-mate", 0x74747474, mate)

	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "party-multi-killer", Empire: killer.Empire, Characters: []loginticket.Character{killer}}); err != nil {
		t.Fatalf("seed party-multi killer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "party-multi-mate", Empire: mate.Empire, Characters: []loginticket.Character{mate}}); err != nil {
		t.Fatalf("seed party-multi mate account: %v", err)
	}
	currentTime := time.Unix(1_700_000_771, 0)
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("new game runtime: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	if _, ok := runtime.sharedWorld.registerStaticActor(actor.Entity.ID, actor.Entity.Name, actor.Position.MapIndex, actor.Position.X, actor.Position.Y, actor.RaceNum, "", "", actor.CombatKind, actor.SpawnGroupRef, worldruntime.StaticActorDeathReward{}); !ok {
		t.Fatal("expected party-multi reward mob registration to succeed")
	}
	if !runtime.sharedWorld.overrideStaticActorDeathReward(actor.Entity.ID, worldruntime.StaticActorDeathReward{DropVnums: []uint32{27001, 27002}}) {
		t.Fatal("expected party-multi reward override to apply")
	}

	killerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "party-multi-killer", 0x73737373)
	defer closeSessionFlow(t, killerFlow)
	mateFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "party-multi-mate", 0x74747474)
	defer closeSessionFlow(t, mateFlow)
	flushServerFrames(t, killerFlow)
	flushServerFrames(t, mateFlow)

	targetVID := uint32(actor.Entity.ID)
	if out, err := killerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected target selection before party-multi kill to return 1 frame, got frames=%d err=%v", len(out), err)
	}

	var killOut [][]byte
	for hit := 1; hit <= int(worldruntime.TrainingDummyBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		killOut, err = killerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("unexpected party-multi attack error on hit %d: %v", hit, err)
		}
	}
	if len(killOut) != 7 {
		t.Fatalf("expected killing hit to return dead, clear target, and two ground-add/ownership pairs, got %d", len(killOut))
	}
	for _, idx := range []int{4, 6} {
		ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, killOut[idx]))
		if err != nil || ownership.OwnerName != killer.Name {
			t.Fatalf("expected multi-drop kill-reward ownership to stay on the killer, idx=%d got %+v err=%v", idx, ownership, err)
		}
	}
}

func TestShareKillRewardExperienceSplitsEqualPartsAcrossImplicitParty(t *testing.T) {
	before := []int32{10, 20, 30}
	shares := ShareKillRewardExperience(5, 3, before)
	if len(shares) != 3 {
		t.Fatalf("expected 3 shares, got %#v", shares)
	}
	start := killRewardPartyExpRemainderSlot(5, 3)
	var sum uint64
	for i, share := range shares {
		sum += share
		want := uint64(1)
		if (i+3-start)%3 < 2 {
			want = 2
		}
		if share != want {
			t.Fatalf("share[%d]=%d, want %d (start slot %d)", i, share, want, start)
		}
	}
	if sum != 5 {
		t.Fatalf("expected shares to sum to 5, got %d start=%d shares=%#v", sum, start, shares)
	}
	solo := ShareKillRewardExperience(40, 1, []int32{7})
	if len(solo) != 1 || solo[0] != 40 {
		t.Fatalf("expected a party of one to keep the full total, got %#v", solo)
	}
	overflow := ShareKillRewardExperience(4, 2, []int32{1<<31 - 1, 3})
	if len(overflow) != 2 || overflow[0] != 0 || overflow[1] != 4 {
		t.Fatalf("expected a skipped overflow share to move to the member who can still accept it, got %#v", overflow)
	}
	bothFull := ShareKillRewardExperience(4, 2, []int32{1<<31 - 1, 1<<31 - 1})
	if len(bothFull) != 2 || bothFull[0] != 0 || bothFull[1] != 0 {
		t.Fatalf("expected EXP nobody can accept to be dropped, got %#v", bothFull)
	}
}

func TestGameRuntimeImplicitPartySharesKillRewardExperience(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	actor := worldruntime.StaticEntity{
		Entity:        worldruntime.Entity{ID: 0x01050280, Kind: worldruntime.EntityKindStaticActor, VID: 0x01050280, Name: "PartyExpRewardMob"},
		Position:      worldruntime.NewPosition(bootstrapMapIndex, 1200, 2200),
		RaceNum:       20350,
		CombatProfile: worldruntime.StaticActorCombatProfileTrainingDummy,
		CombatKind:    worldruntime.StaticActorCombatKindTrainingDummy,
		SpawnGroupRef: "practice.party_exp_reward_mob",
	}
	killer := peerVisibilityCharacter("ZuluExpKiller", 0x01030180, 0x02040180, 1100, 2100, 0, 101, 201)
	mate := peerVisibilityCharacter("AlphaExpMate", 0x01030181, 0x02040181, 1120, 2120, 1, 102, 202)
	dead := peerVisibilityCharacter("AardvarkExpDead", 0x01030182, 0x02040182, 1110, 2110, 2, 103, 203)
	killer.Points[bootstrapExperiencePointType] = 10
	mate.Points[bootstrapExperiencePointType] = 20
	dead.Points[bootstrapExperiencePointType] = 30
	dead.Points[bootstrapPlayerPointValueIndex] = 0
	const rewardExperience uint64 = 5
	issuePeerTicket(t, store, "party-exp-killer", 0x80808080, killer)
	issuePeerTicket(t, store, "party-exp-mate", 0x81818181, mate)
	issuePeerTicket(t, store, "party-exp-dead", 0x82828282, dead)

	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "party-exp-killer", Empire: killer.Empire, Characters: []loginticket.Character{killer}}); err != nil {
		t.Fatalf("seed party-exp killer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "party-exp-mate", Empire: mate.Empire, Characters: []loginticket.Character{mate}}); err != nil {
		t.Fatalf("seed party-exp mate account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "party-exp-dead", Empire: dead.Empire, Characters: []loginticket.Character{dead}}); err != nil {
		t.Fatalf("seed party-exp dead account: %v", err)
	}
	currentTime := time.Unix(1_700_000_780, 0)
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts)
	if err != nil {
		t.Fatalf("new game runtime: %v", err)
	}
	runtime.now = func() time.Time { return currentTime }
	if _, ok := runtime.sharedWorld.registerStaticActor(actor.Entity.ID, actor.Entity.Name, actor.Position.MapIndex, actor.Position.X, actor.Position.Y, actor.RaceNum, "", "", actor.CombatKind, actor.SpawnGroupRef, worldruntime.StaticActorDeathReward{}); !ok {
		t.Fatal("expected party-exp reward mob registration to succeed")
	}
	if !runtime.sharedWorld.overrideStaticActorDeathReward(actor.Entity.ID, worldruntime.StaticActorDeathReward{Experience: rewardExperience, Gold: 25}) {
		t.Fatal("expected party-exp reward override to apply")
	}

	killerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "party-exp-killer", 0x80808080)
	defer closeSessionFlow(t, killerFlow)
	mateFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "party-exp-mate", 0x81818181)
	defer closeSessionFlow(t, mateFlow)
	deadFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "party-exp-dead", 0x82828282)
	defer closeSessionFlow(t, deadFlow)
	flushServerFrames(t, killerFlow)
	flushServerFrames(t, mateFlow)
	flushServerFrames(t, deadFlow)

	shares := ShareKillRewardExperience(rewardExperience, 2, []int32{mate.Points[bootstrapExperiencePointType], killer.Points[bootstrapExperiencePointType]})
	if len(shares) != 2 || shares[0]+shares[1] != rewardExperience {
		t.Fatalf("expected equal implicit-party shares of %d, got %#v", rewardExperience, shares)
	}
	mateShare, killerShare := shares[0], shares[1]

	targetVID := uint32(actor.Entity.ID)
	if out, err := killerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected target selection before party-exp kill to return 1 frame, got frames=%d err=%v", len(out), err)
	}

	var killOut [][]byte
	for hit := 1; hit <= int(worldruntime.TrainingDummyBootstrapMaxHP); hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		killOut, err = killerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: targetVID})))
		if err != nil {
			t.Fatalf("unexpected party-exp attack error on hit %d: %v", hit, err)
		}
	}
	var killerExp *worldproto.PlayerPointChangePacket
	var killerGold *worldproto.PlayerPointChangePacket
	for _, raw := range killOut {
		frame := decodeSingleFrame(t, raw)
		if frame.Header != worldproto.HeaderPlayerPointChange {
			continue
		}
		change, err := worldproto.DecodePlayerPointChange(frame)
		if err != nil {
			t.Fatalf("decode killer point change: %v", err)
		}
		switch change.Type {
		case player.ExperiencePointIndex:
			killerExp = &change
		case bootstrapGoldPointType:
			killerGold = &change
		}
	}
	if killerExp == nil || killerExp.VID != killer.VID || uint64(killerExp.Amount) != killerShare || killerExp.Value != 10+int32(killerShare) {
		t.Fatalf("expected killer EXP share %d from 10, got %+v", killerShare, killerExp)
	}
	if killerGold == nil || killerGold.VID != killer.VID || killerGold.Amount != 25 {
		t.Fatalf("expected killer to keep the full gold reward, got %+v", killerGold)
	}

	mateFrames := flushServerFrames(t, mateFlow)
	var mateExp *worldproto.PlayerPointChangePacket
	for _, raw := range mateFrames {
		frame := decodeSingleFrame(t, raw)
		if frame.Header != worldproto.HeaderPlayerPointChange {
			continue
		}
		change, err := worldproto.DecodePlayerPointChange(frame)
		if err != nil {
			t.Fatalf("decode mate point change: %v", err)
		}
		if change.Type == player.ExperiencePointIndex {
			mateExp = &change
		}
		if change.Type == bootstrapGoldPointType {
			t.Fatalf("expected gold to stay with the killer, mate received %+v", change)
		}
	}
	if mateExp == nil || mateExp.VID != mate.VID || uint64(mateExp.Amount) != mateShare || mateExp.Value != 20+int32(mateShare) {
		t.Fatalf("expected mate EXP share %d from 20, got %+v frames=%d", mateShare, mateExp, len(mateFrames))
	}
	for _, raw := range flushServerFrames(t, deadFlow) {
		frame := decodeSingleFrame(t, raw)
		if frame.Header != worldproto.HeaderPlayerPointChange {
			continue
		}
		change, err := worldproto.DecodePlayerPointChange(frame)
		if err == nil && change.Type == player.ExperiencePointIndex {
			t.Fatalf("expected 0-HP member to be skipped for EXP share, got %+v", change)
		}
	}

	killerAccount, err := accounts.Load("party-exp-killer")
	if err != nil {
		t.Fatalf("load party-exp killer account: %v", err)
	}
	mateAccount, err := accounts.Load("party-exp-mate")
	if err != nil {
		t.Fatalf("load party-exp mate account: %v", err)
	}
	deadAccount, err := accounts.Load("party-exp-dead")
	if err != nil {
		t.Fatalf("load party-exp dead account: %v", err)
	}
	if killerAccount.Characters[0].Points[bootstrapExperiencePointType] != 10+int32(killerShare) || killerAccount.Characters[0].Gold != 25 {
		t.Fatalf("expected persisted killer share and full gold, got exp=%d gold=%d", killerAccount.Characters[0].Points[bootstrapExperiencePointType], killerAccount.Characters[0].Gold)
	}
	if mateAccount.Characters[0].Points[bootstrapExperiencePointType] != 20+int32(mateShare) || mateAccount.Characters[0].Gold != 0 {
		t.Fatalf("expected persisted mate EXP share without gold, got exp=%d gold=%d", mateAccount.Characters[0].Points[bootstrapExperiencePointType], mateAccount.Characters[0].Gold)
	}
	if deadAccount.Characters[0].Points[bootstrapExperiencePointType] != 30 {
		t.Fatalf("expected dead member EXP to stay unchanged, got %d", deadAccount.Characters[0].Points[bootstrapExperiencePointType])
	}
}
