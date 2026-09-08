package minimal

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
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

func TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerDaemonRestartRestartTownRematerializesKillRewardDropOnSourceMapReselect(t *testing.T) {
	assertCombinedLastHitDaemonRestartTownContractFrozen(t)
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	const profile = "practice_combined_last_hit_daemon_restart_town_ground_catch_up_mob"
	const spawnRef = "practice.combined_last_hit_daemon_restart_town_ground_catch_up_mob"
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
		t.Fatalf("expected %q combined last-hit daemon-restart /restart_town ground catch-up profile registration to succeed", profile)
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
		t.Fatalf("seed combined last-hit daemon-restart /restart_town item templates: %v", err)
	}

	owner := peerVisibilityCharacter("DRTownGroundCatchOwner", 0x010301f1, 0x020401f1, 1100, 2100, 0, 101, 201)
	owner.Empire = 2
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Points[bootstrapExperiencePointType] = 25
	owner.Gold = 40
	watcher := peerVisibilityCharacter("DRTownGroundCatchWatch", 0x010301f2, 0x020401f2, 1300, 2300, 0, 102, 202)
	const (
		ownerLogin   = "clh-dr-town-ground-owner"
		watcherLogin = "clh-dr-town-ground-watch"
		ownerKey     = uint32(0xf1f1f1f1)
		watcherKey   = uint32(0xf2f2f2f2)
	)
	issuePeerTicket(t, store, ownerLogin, ownerKey, owner)
	issuePeerTicket(t, store, watcherLogin, watcherKey, watcher)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combined last-hit daemon-restart /restart_town ground catch-up owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: watcherLogin, Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed combined last-hit daemon-restart /restart_town ground catch-up watcher account: %v", err)
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
	targetVID := importCombinedLastHitKillRewardDummy(t, runtime, profile, spawnRef, "CombinedLastHitDaemonRestartTownGroundCatchUpMob", rewardExperience, rewardGold, rewardDropVnum)
	wantTownMap, wantTownX, wantTownY := legacyCreatePositionForEmpire(owner.Empire)

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

	ground, ownership := driveCombinedLastHitKillRewardDummyKill(t, ownerFlow, watcherFlow, runtime, spawnRef, targetVID, owner, rewardDropVnum, "combined last-hit still-dead daemon-restart /restart_town ground catch-up")
	killTime := currentTime
	wantOwnershipExpiresAt := killTime.Add(bootstrapGroundItemOwnershipDuration)
	wantDespawnAt := killTime.Add(bootstrapGroundItemDespawnDuration)

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
	assertDurableKillRewardAbsoluteTimers(t, persistedGround, ground.VID, rewardDropVnum, ownerLogin, owner, wantOwnershipExpiresAt, wantDespawnAt, "before daemon restart")

	assertExpiredFileStoreRematerializePublicizesKillReward(t, groundItemPath, itemTemplatePath, ground.VID, rewardDropVnum, owner)

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
		t.Fatalf("unexpected post-restart combined last-hit /restart_town rematerialize runtime error: %v", err)
	}
	reloaded.now = func() time.Time { return currentTime }
	reloaded.sharedWorld.now = reloaded.now

	afterRestart, ok := reloaded.SpawnGroupByRef(spawnRef)
	if !ok || !afterRestart.Dead || afterRestart.EntityID != uint64(targetVID) {
		t.Fatalf("expected daemon restart to preserve still-dead dummy vid %d, ok=%v snapshot=%+v", targetVID, ok, afterRestart)
	}
	snapshot := reloaded.sharedWorld.DurableGroundItemSnapshot()
	assertDurableKillRewardAbsoluteTimers(t, snapshot, ground.VID, rewardDropVnum, ownerLogin, owner, wantOwnershipExpiresAt, wantDespawnAt, "after FileStore rematerialize")
	assertProcessLocalKillRewardOwnerID(t, reloaded, ground.VID, 0, "after FileStore rematerialize before Join")

	reloadedTicketStore := loginticket.NewFileStore(ticketDir)
	reloadedAccounts := accountstore.NewFileStore(accountDir)
	issuePeerTicket(t, reloadedTicketStore, ownerLogin, 0xf3f3f3f3, persistedBeforeRestart.Characters[0])
	issuePeerTicket(t, reloadedTicketStore, watcherLogin, 0xf4f4f4f4, watcher)

	watcherRestartFlow, watcherRestartEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), watcherLogin, 0xf4f4f4f4)
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
	assertProcessLocalKillRewardOwnerID(t, reloaded, ground.VID, 0, "after non-matching watcher Join")
	assertDurableKillRewardAbsoluteTimers(t, reloaded.sharedWorld.DurableGroundItemSnapshot(), ground.VID, rewardDropVnum, ownerLogin, owner, wantOwnershipExpiresAt, wantDespawnAt, "after non-matching watcher Join")

	ownerRestartFlow, ownerRestartEnter := enterGameWithLoginTicket(t, reloaded.SessionFactory(), ownerLogin, 0xf3f3f3f3)
	defer closeSessionFlow(t, ownerRestartFlow)
	assertCombinedLastHitStillDeadOwnerReentrySkipsDummy(t, ownerRestartEnter, owner.VID, watcher.VID, targetVID, "combined last-hit still-dead daemon-restart /restart_town ground catch-up re-entry")
	assertCombinedLastHitOwnerReentrySkipsKillRewardDrop(t, ownerRestartEnter, ground.VID, "combined last-hit still-dead daemon-restart /restart_town ground catch-up re-entry")
	assertWatcherStillDeadOwnerReentry(t, watcherRestartFlow, owner.VID, "combined last-hit still-dead daemon-restart /restart_town ground catch-up re-entry")
	assertStillDeadDummyUntargetable(t, ownerRestartFlow, reloaded, spawnRef, targetVID, "combined last-hit still-dead daemon-restart /restart_town ground catch-up re-entry")
	if !reloaded.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected rematerialized kill-reward drop to survive still-dead owner EnterGame")
	}
	ownerEntity, ownerOK := reloaded.sharedWorld.playerEntityByName(owner.Name)
	watcherEntity, watcherOK := reloaded.sharedWorld.playerEntityByName(watcher.Name)
	if !ownerOK || !watcherOK || ownerEntity.Entity.ID == 0 || watcherEntity.Entity.ID == 0 {
		t.Fatalf("expected rematerialized owner/watcher entity ids, ownerOK=%v watcherOK=%v", ownerOK, watcherOK)
	}
	assertProcessLocalKillRewardOwnerID(t, reloaded, ground.VID, ownerEntity.Entity.ID, "after matching still-dead owner Join rebind")
	assertDurableKillRewardAbsoluteTimers(t, reloaded.sharedWorld.DurableGroundItemSnapshot(), ground.VID, rewardDropVnum, ownerLogin, owner, wantOwnershipExpiresAt, wantDespawnAt, "after matching owner Join rebind")
	if _, ok := reloaded.sharedWorld.GroundItemPickupFor(watcherEntity.Entity.ID, watcher, ground.VID); ok {
		t.Fatal("expected rematerialized exclusive kill-reward ownership to block living watcher mid-window")
	}
	if _, ok := reloaded.sharedWorld.GroundItemPickupFor(ownerEntity.Entity.ID, owner, ground.VID); ok {
		t.Fatal("expected still-dead owner EnterGame to keep kill-reward pickup fail-closed until /restart_town")
	}

	assertCombinedLastHitKillRewardRestartTownCatchUp(t, ownerRestartFlow, watcherRestartFlow, reloadedAccounts, ownerLogin, owner, reloaded, spawnRef, targetVID, watcher.VID, ground, ownership, rewardDropVnum, &currentTime, wantTownMap, wantTownX, wantTownY, "combined last-hit still-dead daemon-restart /restart_town ground catch-up")
}

func assertCombinedLastHitDaemonRestartTownContractFrozen(t *testing.T) {
	t.Helper()
	_, thisFile, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("expected this test file path for daemon-restart /restart_town contract freeze")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	planPath := filepath.Join(root, "docs/plans/2026-09-08-combined-last-hit-daemon-restart-restart-town-kill-reward-rematerialize.md")
	townSpecPath := filepath.Join(root, "spec/protocol/player-restart-town-bootstrap.md")
	rewardSpecPath := filepath.Join(root, "spec/protocol/non-player-reward-bootstrap.md")
	plan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("expected this slice's daemon-restart /restart_town plan to exist: %v", err)
	}
	townSpec, err := os.ReadFile(townSpecPath)
	if err != nil {
		t.Fatalf("expected player-restart-town spec for daemon-restart town rematerialize: %v", err)
	}
	rewardSpec, err := os.ReadFile(rewardSpecPath)
	if err != nil {
		t.Fatalf("expected non-player-reward spec for daemon-restart town rematerialize: %v", err)
	}
	const testName = "TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerDaemonRestartRestartTownRematerializesKillRewardDropOnSourceMapReselect"
	for _, body := range []struct {
		path    string
		content string
	}{
		{planPath, string(plan)},
		{townSpecPath, string(townSpec)},
		{rewardSpecPath, string(rewardSpec)},
	} {
		if !strings.Contains(body.content, testName) {
			t.Fatalf("expected %s to freeze %s", body.path, testName)
		}
		if !strings.Contains(body.content, "ownership_expires_at") || !strings.Contains(body.content, "OwnerID = 0") {
			t.Fatalf("expected %s to freeze absolute ownership_expires_at and OwnerID = 0 rematerialize", body.path)
		}
	}
}

func assertDurableKillRewardAbsoluteTimers(
	t *testing.T,
	snapshot worldruntime.DurableGroundItemSnapshot,
	groundVID uint32,
	rewardDropVnum uint32,
	ownerLogin string,
	owner loginticket.Character,
	wantOwnershipExpiresAt time.Time,
	wantDespawnAt time.Time,
	context string,
) {
	t.Helper()
	if len(snapshot.GroundItems) != 1 {
		t.Fatalf("expected 1 durable kill-reward row %s, got %#v", context, snapshot.GroundItems)
	}
	row := snapshot.GroundItems[0]
	if row.VID != groundVID || row.Vnum != rewardDropVnum || row.ItemCount == nil || *row.ItemCount != 1 {
		t.Fatalf("unexpected durable kill-reward identity %s: %#v", context, row)
	}
	if row.OwnerLogin != ownerLogin || row.OwnerCharacterID != owner.ID || row.OwnerVID != owner.VID || row.OwnerName != owner.Name {
		t.Fatalf("expected durable owner identity login=%s id=%d vid=%d name=%q %s, got %#v", ownerLogin, owner.ID, owner.VID, owner.Name, context, row)
	}
	if !row.OwnershipExclusive || row.OwnershipExpiresAt == nil {
		t.Fatalf("expected exclusive durable kill-reward with absolute ownership_expires_at %s, got %#v", context, row)
	}
	if !row.OwnershipExpiresAt.Equal(wantOwnershipExpiresAt.UTC()) {
		t.Fatalf("expected absolute ownership_expires_at %s %s, got %s", wantOwnershipExpiresAt.UTC().Format(time.RFC3339Nano), context, row.OwnershipExpiresAt.UTC().Format(time.RFC3339Nano))
	}
	if !row.DespawnAt.Equal(wantDespawnAt.UTC()) {
		t.Fatalf("expected absolute despawn_at %s %s, got %s", wantDespawnAt.UTC().Format(time.RFC3339Nano), context, row.DespawnAt.UTC().Format(time.RFC3339Nano))
	}
}

func assertProcessLocalKillRewardOwnerID(t *testing.T, runtime *gameRuntime, groundVID uint32, wantOwnerID uint64, context string) {
	t.Helper()
	if runtime == nil || runtime.sharedWorld == nil {
		t.Fatalf("expected rematerialized shared world %s", context)
	}
	ground, ok := runtime.sharedWorld.groundItemsByVID[groundVID]
	if !ok {
		t.Fatalf("expected rematerialized kill-reward vid %d %s", groundVID, context)
	}
	if ground.OwnerID != wantOwnerID {
		t.Fatalf("expected process-local OwnerID=%d %s, got %d", wantOwnerID, context, ground.OwnerID)
	}
}

func assertExpiredFileStoreRematerializePublicizesKillReward(
	t *testing.T,
	groundItemPath string,
	itemTemplatePath string,
	groundVID uint32,
	rewardDropVnum uint32,
	owner loginticket.Character,
) {
	t.Helper()

	expiredGroundPath := filepath.Join(t.TempDir(), "expired-ground-items.json")
	raw, err := os.ReadFile(groundItemPath)
	if err != nil {
		t.Fatalf("read exclusive kill-reward FileStore before expiry copy: %v", err)
	}
	if err := os.WriteFile(expiredGroundPath, raw, 0o644); err != nil {
		t.Fatalf("copy exclusive kill-reward FileStore for expiry rematerialize: %v", err)
	}
	expiredStore := worldruntime.NewGroundItemFileStore(expiredGroundPath)
	expiredSnap, err := expiredStore.Load()
	if err != nil {
		t.Fatalf("load copied exclusive kill-reward FileStore: %v", err)
	}
	if len(expiredSnap.GroundItems) != 1 || !expiredSnap.GroundItems[0].OwnershipExclusive || expiredSnap.GroundItems[0].OwnershipExpiresAt == nil {
		t.Fatalf("expected copied exclusive kill-reward before expiry mutation, got %#v", expiredSnap.GroundItems)
	}
	pastExpiry := time.Now().UTC().Add(-time.Second)
	expiredSnap.GroundItems[0].OwnershipExpiresAt = &pastExpiry
	if !expiredSnap.GroundItems[0].DespawnAt.After(time.Now().UTC()) {
		t.Fatalf("expected copied kill-reward despawn_at to stay in the future before expiry rematerialize, got %s", expiredSnap.GroundItems[0].DespawnAt)
	}
	if err := expiredStore.Save(expiredSnap); err != nil {
		t.Fatalf("persist due ownership_expires_at for expiry rematerialize: %v", err)
	}

	ticketDir := t.TempDir()
	accountDir := t.TempDir()
	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)
	collector := peerVisibilityCharacter("DRTownExpireWatch", 0x010301f3, 0x020401f3, owner.X, owner.Y, 0, 103, 203)
	const (
		collectorLogin = "clh-dr-town-expire-watch"
		collectorKey   = uint32(0xf5f5f5f5)
	)
	issuePeerTicket(t, ticketStore, collectorLogin, collectorKey, collector)
	if err := accounts.Save(accountstore.Account{Login: collectorLogin, Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed expiry rematerialize collector account: %v", err)
	}

	expiredRuntime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(
		config.Service{
			LegacyAddr:          ":13000",
			PublicAddr:          "127.0.0.1",
			LoginTicketStoreDir: ticketDir,
			AccountStoreDir:     accountDir,
			GroundItemStorePath: expiredGroundPath,
		},
		ticketStore,
		accounts,
		staticstore.NewFileStore(filepath.Join(t.TempDir(), "expired-static-actors.json")),
		interactionstore.NewFileStore(filepath.Join(t.TempDir(), "expired-interaction-definitions.json")),
		itemcatalog.NewFileStore(itemTemplatePath),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected expired-ownership rematerialize runtime error: %v", err)
	}
	publicSnap := expiredRuntime.sharedWorld.DurableGroundItemSnapshot()
	if len(publicSnap.GroundItems) != 1 || publicSnap.GroundItems[0].VID != groundVID || publicSnap.GroundItems[0].Vnum != rewardDropVnum {
		t.Fatalf("expected expired FileStore rematerialize to keep the kill-reward handle, got %#v", publicSnap.GroundItems)
	}
	if publicSnap.GroundItems[0].OwnershipExclusive || publicSnap.GroundItems[0].OwnershipExpiresAt != nil {
		t.Fatalf("expected construction-time restore to publicize due ownership_expires_at, got %#v", publicSnap.GroundItems[0])
	}
	assertProcessLocalKillRewardOwnerID(t, expiredRuntime, groundVID, 0, "after expired exclusive restore")

	collectorFlow, _ := enterGameWithLoginTicket(t, expiredRuntime.SessionFactory(), collectorLogin, collectorKey)
	defer closeSessionFlow(t, collectorFlow)
	collectorEntity, ok := expiredRuntime.sharedWorld.playerEntityByName(collector.Name)
	if !ok || collectorEntity.Entity.ID == 0 {
		t.Fatal("expected expiry rematerialize collector entity id")
	}
	pickup, ok := expiredRuntime.sharedWorld.GroundItemPickupFor(collectorEntity.Entity.ID, collector, groundVID)
	if !ok || pickup.Item.Vnum != rewardDropVnum || pickup.Item.Count != 1 {
		t.Fatalf("expected publicized rematerialized kill-reward to allow living collector pickup, ok=%v pickup=%+v", ok, pickup)
	}
	pickupOut := pickupGroundItem(t, collectorFlow, groundVID)
	assertPostFloorItemPickupSuccessBurst(t, pickupOut, 0, rewardDropVnum, 1, "expired FileStore rematerialize public collector pickup")
	if expiredRuntime.sharedWorld.GroundItemExists(groundVID) {
		t.Fatal("expected publicized rematerialized kill-reward pickup to remove the ground handle")
	}
}
