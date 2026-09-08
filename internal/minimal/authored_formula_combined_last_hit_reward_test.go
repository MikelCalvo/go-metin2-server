package minimal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowAuthoredFormulaProfileKillingHitAlsoFloorsOwnerEmitsProfileDefaultRewardsBeforeOwnerFloor(t *testing.T) {
	const profile = "qa_formula_practice_mob"
	const spawnRef = "practice.qa_formula_mob"
	const rewardExperience uint64 = 40
	const rewardGold uint64 = 25
	const rewardDropVnum uint32 = 27001
	const formulaDamage int32 = 5
	const authoredRetaliationDelta int32 = -2
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate authored formula combined last-hit test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", "examples", "bootstrap-combat-profile-formula-bundle.json"))
	if err != nil {
		t.Fatalf("read formula combat-profile bundle: %v", err)
	}
	var bundle contentbundle.Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode formula combat-profile bundle: %v", err)
	}
	if len(bundle.SpawnGroups) != 1 {
		t.Fatalf("expected one formula spawn group, got %#v", bundle.SpawnGroups)
	}

	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("FormulaRewardFloorOwner", 0x010301fd, 0x020401fd, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 8
	owner.Points[bootstrapExperiencePointType] = 25
	owner.Gold = 40
	watcher := peerVisibilityCharacter("FormulaRewardFloorWatch", 0x010301fe, 0x020401fe, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "formula-reward-floor-owner", 0xfdfdfdfd, owner)
	issuePeerTicket(t, store, "formula-reward-floor-watch", 0xfefefefe, watcher)
	if err := accounts.Save(accountstore.Account{Login: "formula-reward-floor-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed formula combined last-hit owner account: %v", err)
	}

	gameRuntime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		store,
		accounts,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected formula combined last-hit runtime error: %v", err)
	}
	currentTime := time.Unix(1700000804, 0)
	gameRuntime.now = func() time.Time { return currentTime }
	bundle.SpawnGroups[0].MapIndex = bootstrapMapIndex
	bundle.SpawnGroups[0].X = 1200
	bundle.SpawnGroups[0].Y = 2200
	if _, err := gameRuntime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import formula combat-profile bundle: %v", err)
	}
	actors := gameRuntime.StaticActors()
	if len(actors) != 1 || actors[0].SpawnGroupRef != spawnRef || actors[0].CombatProfile != profile || actors[0].CombatMaxHP != 20 || actors[0].CombatNormalDamage != 5 || actors[0].RewardExperience != rewardExperience || actors[0].RewardGold != rewardGold || len(actors[0].RewardDropVnums) != 1 || actors[0].RewardDropVnums[0] != rewardDropVnum {
		t.Fatalf("expected imported formula spawn actor with profile-default reward, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, gameRuntime.SessionFactory(), "formula-reward-floor-owner", 0xfdfdfdfd)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible formula practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, gameRuntime.SessionFactory(), "formula-reward-floor-watch", 0xfefefefe)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and formula practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected formula combined last-hit target error: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 formula combined last-hit target acknowledgement, got %d", len(selectOut))
	}

	wantPercents := []uint8{75, 50, 25}
	wantOwnerHP := []int32{6, 4, 2}
	for i, wantPercent := range wantPercents {
		if i > 0 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		hitOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
			AttackType: combatproto.ClientAttackTypeNormal,
			TargetVID:  targetVID,
		})))
		if err != nil {
			t.Fatalf("unexpected formula combined last-hit live hit %d error: %v", i+1, err)
		}
		if len(hitOut) != 4 {
			t.Fatalf("expected target refresh, retaliation, and damage-info on formula combined last-hit live hit %d, got %d frames", i+1, len(hitOut))
		}
		refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, hitOut[0]))
		if err != nil {
			t.Fatalf("decode formula combined last-hit live hit %d target refresh: %v", i+1, err)
		}
		if refresh.TargetVID != targetVID || refresh.HPPercent != wantPercent {
			t.Fatalf("expected formula combined last-hit live hit %d to reach %d%% HP, got %+v", i+1, wantPercent, refresh)
		}
		retaliation, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, hitOut[1]))
		if err != nil {
			t.Fatalf("decode formula combined last-hit live hit %d retaliation: %v", i+1, err)
		}
		if retaliation.VID != owner.VID || retaliation.Type != bootstrapPlayerPointType || retaliation.Amount != authoredRetaliationDelta || retaliation.Value != wantOwnerHP[i] {
			t.Fatalf("expected formula combined last-hit live hit %d authored -2 retaliation to leave %d HP, got %+v", i+1, wantOwnerHP[i], retaliation)
		}
		assertDamageInfoFrame(t, hitOut[2], targetVID, formulaDamage, "formula combined last-hit live-hit mob damage")
		assertDamageInfoFrame(t, hitOut[3], owner.VID, 2, "formula combined last-hit live-hit owner retaliation")
		queued := flushServerFrames(t, watcherFlow)
		if len(queued) != 2 {
			t.Fatalf("expected 2 queued watcher live-hit damage-info frames on formula combined last-hit live hit %d, got %d", i+1, len(queued))
		}
	}

	currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected formula combined last-hit killing attack error: %v", err)
	}
	if len(attackOut) != 11 {
		t.Fatalf("expected dummy death/clear/damage-info, exp/gold/ground/ownership, then owner-floor point-change/dead/clear/damage-info on formula combined last-hit, got %d frames", len(attackOut))
	}
	remainingDeath := stripKillingHitDeathPrefix(t, attackOut, targetVID, formulaDamage, "formula combined last-hit dummy death")
	if len(remainingDeath) != 8 {
		t.Fatalf("expected 4 reward frames plus 4 owner-floor frames after formula dummy death prefix, got %d", len(remainingDeath))
	}
	experienceChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, remainingDeath[0]))
	if err != nil {
		t.Fatalf("decode formula combined last-hit experience point-change: %v", err)
	}
	if experienceChange.VID != owner.VID || experienceChange.Type != bootstrapExperiencePointType || experienceChange.Amount != int32(rewardExperience) || experienceChange.Value != 65 {
		t.Fatalf("unexpected formula combined last-hit experience point-change: %+v", experienceChange)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, remainingDeath[1]))
	if err != nil {
		t.Fatalf("decode formula combined last-hit gold point-change: %v", err)
	}
	if goldChange.VID != owner.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != int32(rewardGold) || goldChange.Value != 65 {
		t.Fatalf("unexpected formula combined last-hit gold point-change: %+v", goldChange)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, remainingDeath[2]))
	if err != nil {
		t.Fatalf("decode formula combined last-hit ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != rewardDropVnum || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected formula combined last-hit ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, remainingDeath[3]))
	if err != nil {
		t.Fatalf("decode formula combined last-hit ownership: %v", err)
	}
	if ownership.VID != ground.VID || ownership.OwnerName != owner.Name {
		t.Fatalf("unexpected formula combined last-hit ownership: %+v", ownership)
	}
	for _, raw := range remainingDeath[:4] {
		if point, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, raw)); err == nil && point.Amount == authoredRetaliationDelta && point.Type == bootstrapPlayerPointType {
			t.Fatalf("expected formula combined last-hit killing prefix not to mix owner retaliation into reward frames, got %+v", point)
		}
	}
	next := assertOwnerFloorDeathSequence(t, remainingDeath[4:], 0, owner.VID, authoredRetaliationDelta, "formula combined last-hit owner-floor")
	if next != 4 {
		t.Fatalf("expected formula combined last-hit owner-floor suffix to consume 4 frames, got next=%d", next)
	}
	if !gameRuntime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected formula combined last-hit reward drop to stay registered after owner-floor")
	}

	spawned, ok := gameRuntime.SpawnGroupByRef(spawnRef)
	if !ok || !spawned.Dead || spawned.CombatHPPercent != 0 {
		t.Fatalf("expected formula combined last-hit dummy to stay dead after the accepted kill, ok=%v snapshot=%+v", ok, spawned)
	}
	persisted, err := accounts.Load("formula-reward-floor-owner")
	if err != nil {
		t.Fatalf("load persisted formula combined last-hit owner account: %v", err)
	}
	if len(persisted.Characters) != 1 ||
		persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 ||
		persisted.Characters[0].Points[bootstrapExperiencePointType] != 65 ||
		persisted.Characters[0].Gold != 65 {
		t.Fatalf("expected formula combined last-hit to persist owner hp=0 exp=65 gold=65, got %+v", persisted.Characters)
	}

	staleAttack, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale post-floor attack error after formula combined last-hit: %v", err)
	}
	if len(staleAttack) != 0 {
		t.Fatalf("expected stale same-target ATTACK to fail closed after formula combined last-hit, got %d frames", len(staleAttack))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected formula combined last-hit killing hit to cancel pending delayed retaliation, got %d owner frames", len(queued))
	}

	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 6 {
		t.Fatalf("expected 6 queued visible-peer dummy death/damage-info, ground-add/ownership, then owner death/damage-info frames on formula combined last-hit, got %d", len(watcherQueued))
	}
	remainingPeer := stripKillingHitPeerDeathPrefix(t, watcherQueued, targetVID, false, formulaDamage, "formula combined last-hit peer dummy death")
	if len(remainingPeer) != 4 {
		t.Fatalf("expected ground-add/ownership plus owner-floor peer frames after formula dummy death prefix, got %d", len(remainingPeer))
	}
	peerGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, remainingPeer[0]))
	if err != nil {
		t.Fatalf("decode formula combined last-hit watcher ground add: %v", err)
	}
	if peerGround.VID != ground.VID || peerGround.Vnum != ground.Vnum || peerGround.X != ground.X || peerGround.Y != ground.Y {
		t.Fatalf("expected watcher ground add to mirror killer drop, got killer=%+v watcher=%+v", ground, peerGround)
	}
	peerOwnership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, remainingPeer[1]))
	if err != nil {
		t.Fatalf("decode formula combined last-hit watcher ownership: %v", err)
	}
	if peerOwnership.VID != ownership.VID || peerOwnership.OwnerName != ownership.OwnerName {
		t.Fatalf("expected watcher ownership to mirror killer drop, got killer=%+v watcher=%+v", ownership, peerOwnership)
	}
	remainingPeer = assertOwnerFloorPeerDeadFanout(t, remainingPeer[2:], owner.VID, int32(-authoredRetaliationDelta), "formula combined last-hit peer owner-floor")
	if len(remainingPeer) != 0 {
		t.Fatalf("expected no extra peer frames after formula combined last-hit dummy death, drop, and owner death bursts, got %d", len(remainingPeer))
	}
}
