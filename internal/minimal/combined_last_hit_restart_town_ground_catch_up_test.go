package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartTownRematerializesKillRewardDropOnSourceMapReselect(t *testing.T) {
	const profile = "practice_combined_last_hit_restart_town_ground_catch_up_mob"
	const spawnRef = "practice.combined_last_hit_restart_town_ground_catch_up_mob"
	const rewardExperience uint64 = 75
	const rewardGold uint64 = 60
	const rewardDropVnum uint32 = 27001
	if !worldruntime.RegisterStaticActorCombatProfile(profile, worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 1,
		DamagePerNormalAttack: 1,
		AttackValue:           1,
		DefenseValue:          0,
		Level:                 worldruntime.TrainingDummyBootstrapLevel,
		Rank:                  worldruntime.TrainingDummyBootstrapRank,
		RespawnDelay:          worldruntime.PracticeMobBootstrapRespawnDelay,
		RetaliationPointDelta: worldruntime.PracticeMobBootstrapRetaliationPointDelta,
	}) {
		t.Fatalf("expected %q combined last-hit town recovery ground catch-up profile registration to succeed", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("TownGroundCatchOwner", 0x01030171, 0x02040171, 1100, 2100, 0, 101, 201)
	owner.Empire = 2
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Points[bootstrapExperiencePointType] = 25
	owner.Gold = 40
	watcher := peerVisibilityCharacter("TownGroundCatchWatch", 0x01030172, 0x02040172, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "clh-town-ground-owner", 0x71717171, owner)
	issuePeerTicket(t, store, "clh-town-ground-watch", 0x72727272, watcher)
	if err := accounts.Save(accountstore.Account{Login: "clh-town-ground-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combined last-hit town recovery ground catch-up owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "clh-town-ground-watch", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed combined last-hit town recovery ground catch-up watcher account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		store,
		accounts,
		staticstore.NewFileStore(t.TempDir()+"/static-actors.json"),
		interactionstore.NewFileStore(t.TempDir()+"/interaction-definitions.json"),
		itemcatalog.NewFileStore(t.TempDir()+"/item-templates.json"),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700001411, 0)
	runtime.now = func() time.Time { return currentTime }
	targetVID := importCombinedLastHitKillRewardDummy(t, runtime, profile, spawnRef, "CombinedLastHitRestartTownGroundCatchUpMob", rewardExperience, rewardGold, rewardDropVnum)
	wantTownMap, wantTownX, wantTownY := legacyCreatePositionForEmpire(owner.Empire)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "clh-town-ground-owner", 0x71717171)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "clh-town-ground-watch", 0x72727272)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	ground, ownership := driveCombinedLastHitKillRewardDummyKill(t, ownerFlow, watcherFlow, runtime, spawnRef, targetVID, owner, rewardDropVnum, "combined last-hit town recovery ground catch-up")

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay / 2)

	assertCombinedLastHitKillRewardRestartTownCatchUp(t, ownerFlow, watcherFlow, accounts, "clh-town-ground-owner", owner, runtime, spawnRef, targetVID, watcher.VID, ground, ownership, rewardDropVnum, &currentTime, wantTownMap, wantTownX, wantTownY, "combined last-hit town recovery ground catch-up")
}
