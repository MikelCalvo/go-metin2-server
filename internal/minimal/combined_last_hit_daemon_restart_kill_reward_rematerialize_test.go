package minimal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerDaemonRestartRestartHereRematerializesKillRewardDrop(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	const profile = "practice_combined_last_hit_daemon_restart_ground_catch_up_mob"
	const spawnRef = "practice.combined_last_hit_daemon_restart_ground_catch_up_mob"
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
		t.Fatalf("expected %q combined last-hit daemon-restart ground catch-up profile registration to succeed", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	ticketDir := t.TempDir()
	accountDir := t.TempDir()
	groundItemPath := filepath.Join(t.TempDir(), "ground-items.json")
	itemTemplatePath := filepath.Join(t.TempDir(), "item-templates.json")
	staticActorPath := filepath.Join(t.TempDir(), "static-actors.json")
	interactionPath := filepath.Join(t.TempDir(), "interaction-definitions.json")

	store := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)
	itemStore := itemcatalog.NewFileStore(itemTemplatePath)
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: rewardDropItemTemplates(rewardDropVnum)}); err != nil {
		t.Fatalf("seed combined last-hit daemon-restart item templates: %v", err)
	}

	owner := peerVisibilityCharacter("DRGroundCatchOwner", 0x010301e1, 0x020401e1, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Points[bootstrapExperiencePointType] = 25
	owner.Gold = 40
	watcher := peerVisibilityCharacter("DRGroundCatchWatch", 0x010301e2, 0x020401e2, 1300, 2300, 0, 102, 202)
	const (
		ownerLogin   = "clh-dr-ground-owner"
		watcherLogin = "clh-dr-ground-watch"
		ownerKey     = uint32(0xe1e1e1e1)
		watcherKey   = uint32(0xe2e2e2e2)
	)
	issuePeerTicket(t, store, ownerLogin, ownerKey, owner)
	issuePeerTicket(t, store, watcherLogin, watcherKey, watcher)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combined last-hit daemon-restart ground catch-up owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: watcherLogin, Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed combined last-hit daemon-restart ground catch-up watcher account: %v", err)
	}

	cfg := config.Service{
		LegacyAddr:          ":13000",
		PublicAddr:          "127.0.0.1",
		LoginTicketStoreDir: ticketDir,
		AccountStoreDir:     accountDir,
		GroundItemStorePath: groundItemPath,
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		cfg,
		store,
		accounts,
		staticstore.NewFileStore(staticActorPath),
		interactionstore.NewFileStore(interactionPath),
		itemStore,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	// Wall-clock time keeps construction-time ground rematerialize filtering
	// inside the exclusive/despawn windows after process restart.
	currentTime := time.Now().UTC().Truncate(time.Second)
	runtime.now = func() time.Time { return currentTime }
	runtime.sharedWorld.now = runtime.now
	targetVID := importCombinedLastHitKillRewardDummy(t, runtime, profile, spawnRef, "CombinedLastHitDaemonRestartGroundCatchUpMob", rewardExperience, rewardGold, rewardDropVnum)

	factory := runtime.SessionFactory()
	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, factory, ownerLogin, ownerKey)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, factory, watcherLogin, watcherKey)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	ground, ownership := driveCombinedLastHitKillRewardDummyKill(t, ownerFlow, watcherFlow, runtime, spawnRef, targetVID, owner, rewardDropVnum, "combined last-hit still-dead daemon-restart ground catch-up")

	persistedBeforeRestart, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted owner account after combined last-hit before daemon restart: %v", err)
	}
	if len(persistedBeforeRestart.Characters) != 1 || persistedBeforeRestart.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected combined last-hit to persist owner HP floor 0 before daemon restart, got %+v", persistedBeforeRestart.Characters)
	}
	persistedGround, err := worldruntime.NewGroundItemFileStore(groundItemPath).Load()
	if err != nil {
		t.Fatalf("load persisted kill-reward ground items before daemon restart: %v", err)
	}
	if len(persistedGround.GroundItems) != 1 || persistedGround.GroundItems[0].VID != ground.VID || !persistedGround.GroundItems[0].OwnershipExclusive {
		t.Fatalf("expected 1 exclusive persisted kill-reward handle before daemon restart, got %#v", persistedGround.GroundItems)
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay / 2)

	// Simulate process crash: abandon live sessions without Leave-owned deletion.
	runtime.sharedWorld.SetGroundItemsChangedHook(nil)
	closeSessionFlow(t, ownerFlow)
	closeSessionFlow(t, watcherFlow)
	worldruntime.UnregisterStaticActorCombatProfileForTest(profile)

	reloaded, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		cfg,
		loginticket.NewFileStore(ticketDir),
		accountstore.NewFileStore(accountDir),
		staticstore.NewFileStore(staticActorPath),
		interactionstore.NewFileStore(interactionPath),
		itemcatalog.NewFileStore(itemTemplatePath),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected post-restart combined last-hit rematerialize runtime error: %v", err)
	}
	reloaded.now = func() time.Time { return currentTime }
	reloaded.sharedWorld.now = reloaded.now

	afterRestart, ok := reloaded.SpawnGroupByRef(spawnRef)
	if !ok || !afterRestart.Dead || afterRestart.EntityID != uint64(targetVID) {
		t.Fatalf("expected daemon restart to preserve still-dead dummy vid %d, ok=%v snapshot=%+v", targetVID, ok, afterRestart)
	}
	snapshot := reloaded.sharedWorld.DurableGroundItemSnapshot()
	if len(snapshot.GroundItems) != 1 || snapshot.GroundItems[0].VID != ground.VID || snapshot.GroundItems[0].Vnum != rewardDropVnum || !snapshot.GroundItems[0].OwnershipExclusive {
		t.Fatalf("expected durable rematerialized kill-reward snapshot with 1 exclusive row, got %#v", snapshot.GroundItems)
	}

	reloadedTicketStore := loginticket.NewFileStore(ticketDir)
	reloadedAccounts := accountstore.NewFileStore(accountDir)
	issuePeerTicket(t, reloadedTicketStore, ownerLogin, 0xe3e3e3e3, persistedBeforeRestart.Characters[0])
	issuePeerTicket(t, reloadedTicketStore, watcherLogin, 0xe4e4e4e4, watcher)

	watcherRestartFlow, watcherRestartEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), watcherLogin, 0xe4e4e4e4)
	defer closeSessionFlow(t, watcherRestartFlow)
	if len(watcherRestartEnter) != 11 {
		t.Fatalf("expected 9 still-dead dummy bootstrap frames plus exclusive kill-reward add/ownership for living watcher after daemon restart, got %d", len(watcherRestartEnter))
	}
	staticAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, watcherRestartEnter[5]))
	if err != nil {
		t.Fatalf("decode still-dead daemon-restart watcher dummy add: %v", err)
	}
	if staticAdd.VID != targetVID {
		t.Fatalf("unexpected still-dead daemon-restart watcher dummy add: %+v", staticAdd)
	}
	deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, watcherRestartEnter[8]))
	if err != nil || deadReplay.VID != targetVID {
		t.Fatalf("expected watcher still-dead dummy DEAD replay after daemon restart, got %+v err=%v", deadReplay, err)
	}
	watcherGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, watcherRestartEnter[9]))
	if err != nil || watcherGround != ground {
		t.Fatalf("expected living watcher EnterGame to rematerialize exclusive kill-reward ground add, got %+v err=%v want %+v", watcherGround, err, ground)
	}
	watcherOwnership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, watcherRestartEnter[10]))
	if err != nil || watcherOwnership != ownership {
		t.Fatalf("expected living watcher EnterGame to rematerialize exclusive kill-reward ownership, got %+v err=%v want %+v", watcherOwnership, err, ownership)
	}

	ownerRestartFlow, ownerRestartEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), ownerLogin, 0xe3e3e3e3)
	defer closeSessionFlow(t, ownerRestartFlow)
	assertCombinedLastHitStillDeadOwnerReentrySkipsDummy(t, ownerRestartEnter, owner.VID, watcher.VID, targetVID, "combined last-hit still-dead daemon-restart ground catch-up re-entry")
	assertCombinedLastHitOwnerReentrySkipsKillRewardDrop(t, ownerRestartEnter, ground.VID, "combined last-hit still-dead daemon-restart ground catch-up re-entry")
	assertWatcherStillDeadOwnerReentry(t, watcherRestartFlow, owner.VID, "combined last-hit still-dead daemon-restart ground catch-up re-entry")
	assertStillDeadDummyUntargetable(t, ownerRestartFlow, reloaded, spawnRef, targetVID, "combined last-hit still-dead daemon-restart ground catch-up re-entry")
	if !reloaded.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected rematerialized kill-reward drop to survive still-dead owner EnterGame")
	}
	ownerEntity, ownerOK := reloaded.sharedWorld.playerEntityByName(owner.Name)
	watcherEntity, watcherOK := reloaded.sharedWorld.playerEntityByName(watcher.Name)
	if !ownerOK || !watcherOK || ownerEntity.Entity.ID == 0 || watcherEntity.Entity.ID == 0 {
		t.Fatalf("expected rematerialized owner/watcher entity ids, ownerOK=%v watcherOK=%v", ownerOK, watcherOK)
	}
	if _, ok := reloaded.sharedWorld.GroundItemPickupFor(watcherEntity.Entity.ID, watcher, ground.VID); ok {
		t.Fatal("expected rematerialized exclusive kill-reward ownership to block living watcher mid-window")
	}
	if _, ok := reloaded.sharedWorld.GroundItemPickupFor(ownerEntity.Entity.ID, owner, ground.VID); ok {
		t.Fatal("expected still-dead owner EnterGame to keep kill-reward pickup fail-closed until /restart_here")
	}

	assertCombinedLastHitKillRewardRestartHereCatchUp(t, ownerRestartFlow, watcherRestartFlow, reloadedAccounts, ownerLogin, owner, reloaded, spawnRef, targetVID, ground, ownership, rewardDropVnum, &currentTime, "CombinedLastHitDaemonRestartGroundCatchUpMob", "combined last-hit still-dead daemon-restart /restart_here ground catch-up")
}
