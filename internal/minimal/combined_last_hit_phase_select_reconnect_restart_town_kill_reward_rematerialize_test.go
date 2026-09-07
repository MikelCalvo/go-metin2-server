package minimal

import (
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartTownRematerializesKillRewardDropOnSourceMapReselect(t *testing.T) {
	const profile = "practice_combined_last_hit_phase_select_restart_town_ground_catch_up_mob"
	const spawnRef = "practice.combined_last_hit_phase_select_restart_town_ground_catch_up_mob"
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
		t.Fatalf("expected %q combined last-hit /phase_select /restart_town ground catch-up profile registration to succeed", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PSTownGroundOwner", 0x010301c1, 0x020401c1, 1100, 2100, 0, 101, 201)
	owner.Empire = 2
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Points[bootstrapExperiencePointType] = 25
	owner.Gold = 40
	watcher := peerVisibilityCharacter("PSTownGroundWatch", 0x010301c2, 0x020401c2, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "clh-ps-town-ground-owner", 0xc1c1c1c1, owner)
	issuePeerTicket(t, store, "clh-ps-town-ground-watch", 0xc2c2c2c2, watcher)
	if err := accounts.Save(accountstore.Account{Login: "clh-ps-town-ground-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combined last-hit /phase_select /restart_town ground catch-up owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "clh-ps-town-ground-watch", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed combined last-hit /phase_select /restart_town ground catch-up watcher account: %v", err)
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
	currentTime := time.Unix(1700001505, 0)
	runtime.now = func() time.Time { return currentTime }
	targetVID := importCombinedLastHitKillRewardDummy(t, runtime, profile, spawnRef, "CombinedLastHitPhaseSelectRestartTownGroundCatchUpMob", rewardExperience, rewardGold, rewardDropVnum)
	wantTownMap, wantTownX, wantTownY := legacyCreatePositionForEmpire(owner.Empire)

	factory := runtime.SessionFactory()
	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, factory, "clh-ps-town-ground-owner", 0xc1c1c1c1)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, factory, "clh-ps-town-ground-watch", 0xc2c2c2c2)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	defer closeSessionFlow(t, ownerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	ground, ownership := driveCombinedLastHitKillRewardDummyKill(t, ownerFlow, watcherFlow, runtime, spawnRef, targetVID, owner, rewardDropVnum, "combined last-hit still-dead /phase_select /restart_town ground catch-up")

	persistedBeforePhaseSelect, err := accounts.Load("clh-ps-town-ground-owner")
	if err != nil {
		t.Fatalf("load persisted owner account after combined last-hit before /phase_select: %v", err)
	}
	if len(persistedBeforePhaseSelect.Characters) != 1 || persistedBeforePhaseSelect.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected combined last-hit to persist owner HP floor 0 before /phase_select, got %+v", persistedBeforePhaseSelect.Characters)
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay / 2)

	phaseSelectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/phase_select"})))
	if err != nil {
		t.Fatalf("unexpected /phase_select error after combined last-hit while dummy is still dead: %v", err)
	}
	if len(phaseSelectOut) == 0 {
		t.Fatal("expected /phase_select frames after combined last-hit while dummy is still dead")
	}
	assertWatcherOwnerLeave(t, watcherFlow, owner.VID, "combined last-hit still-dead /phase_select /restart_town ground catch-up")
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected /phase_select floor-leave to park the pending kill-reward drop instead of deleting it")
	}

	selectPhaseOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0})))
	if err != nil {
		t.Fatalf("unexpected character select after combined last-hit /phase_select: %v", err)
	}
	if len(selectPhaseOut) != 3 {
		t.Fatalf("expected 3 character-select frames after combined last-hit /phase_select, got %d", len(selectPhaseOut))
	}

	reenterOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame()))
	if err != nil {
		t.Fatalf("unexpected enter-game after combined last-hit /phase_select: %v", err)
	}
	assertCombinedLastHitStillDeadOwnerReentrySkipsDummy(t, reenterOut, owner.VID, watcher.VID, targetVID, "combined last-hit still-dead /phase_select /restart_town ground catch-up re-entry")
	assertCombinedLastHitOwnerReentrySkipsKillRewardDrop(t, reenterOut, ground.VID, "combined last-hit still-dead /phase_select /restart_town ground catch-up re-entry")
	assertWatcherStillDeadOwnerReentry(t, watcherFlow, owner.VID, "combined last-hit still-dead /phase_select /restart_town ground catch-up re-entry")
	assertStillDeadDummyUntargetable(t, ownerFlow, runtime, spawnRef, targetVID, "combined last-hit still-dead /phase_select /restart_town ground catch-up re-entry")
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected parked kill-reward drop to survive /phase_select re-entry")
	}

	assertCombinedLastHitKillRewardRestartTownCatchUp(t, ownerFlow, watcherFlow, accounts, "clh-ps-town-ground-owner", owner, runtime, spawnRef, targetVID, watcher.VID, ground, ownership, rewardDropVnum, &currentTime, wantTownMap, wantTownX, wantTownY, "combined last-hit still-dead /phase_select /restart_town ground catch-up")
}

func TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartTownRematerializesKillRewardDropOnSourceMapReselect(t *testing.T) {
	const profile = "practice_combined_last_hit_reconnect_restart_town_ground_catch_up_mob"
	const spawnRef = "practice.combined_last_hit_reconnect_restart_town_ground_catch_up_mob"
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
		t.Fatalf("expected %q combined last-hit reconnect /restart_town ground catch-up profile registration to succeed", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RCTownGroundOwner", 0x010301d1, 0x020401d1, 1100, 2100, 0, 101, 201)
	owner.Empire = 2
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Points[bootstrapExperiencePointType] = 25
	owner.Gold = 40
	watcher := peerVisibilityCharacter("RCTownGroundWatch", 0x010301d2, 0x020401d2, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "clh-rc-town-ground-owner", 0xd1d1d1d1, owner)
	issuePeerTicket(t, store, "clh-rc-town-ground-watch", 0xd2d2d2d2, watcher)
	if err := accounts.Save(accountstore.Account{Login: "clh-rc-town-ground-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combined last-hit reconnect /restart_town ground catch-up owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "clh-rc-town-ground-watch", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed combined last-hit reconnect /restart_town ground catch-up watcher account: %v", err)
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
	currentTime := time.Unix(1700001506, 0)
	runtime.now = func() time.Time { return currentTime }
	targetVID := importCombinedLastHitKillRewardDummy(t, runtime, profile, spawnRef, "CombinedLastHitReconnectRestartTownGroundCatchUpMob", rewardExperience, rewardGold, rewardDropVnum)
	wantTownMap, wantTownX, wantTownY := legacyCreatePositionForEmpire(owner.Empire)

	factory := runtime.SessionFactory()
	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, factory, "clh-rc-town-ground-owner", 0xd1d1d1d1)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, factory, "clh-rc-town-ground-watch", 0xd2d2d2d2)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	ground, ownership := driveCombinedLastHitKillRewardDummyKill(t, ownerFlow, watcherFlow, runtime, spawnRef, targetVID, owner, rewardDropVnum, "combined last-hit still-dead reconnect /restart_town ground catch-up")

	persistedBeforeReconnect, err := accounts.Load("clh-rc-town-ground-owner")
	if err != nil {
		t.Fatalf("load persisted owner account after combined last-hit before reconnect: %v", err)
	}
	if len(persistedBeforeReconnect.Characters) != 1 || persistedBeforeReconnect.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected combined last-hit to persist owner HP floor 0 before reconnect, got %+v", persistedBeforeReconnect.Characters)
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay / 2)

	closeSessionFlow(t, ownerFlow)
	assertWatcherOwnerLeave(t, watcherFlow, owner.VID, "combined last-hit still-dead reconnect /restart_town ground catch-up")
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected reconnect floor-leave to park the pending kill-reward drop instead of deleting it")
	}

	issuePeerTicket(t, store, "clh-rc-town-ground-owner", 0xd3d3d3d3, persistedBeforeReconnect.Characters[0])
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "clh-rc-town-ground-owner", 0xd3d3d3d3)
	defer closeSessionFlow(t, reconnectFlow)
	assertCombinedLastHitStillDeadOwnerReentrySkipsDummy(t, reconnectEnter, owner.VID, watcher.VID, targetVID, "combined last-hit still-dead reconnect /restart_town ground catch-up")
	assertCombinedLastHitOwnerReentrySkipsKillRewardDrop(t, reconnectEnter, ground.VID, "combined last-hit still-dead reconnect /restart_town ground catch-up")
	assertWatcherStillDeadOwnerReentry(t, watcherFlow, owner.VID, "combined last-hit still-dead reconnect /restart_town ground catch-up")
	assertStillDeadDummyUntargetable(t, reconnectFlow, runtime, spawnRef, targetVID, "combined last-hit still-dead reconnect /restart_town ground catch-up")
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected parked kill-reward drop to survive reconnect")
	}

	assertCombinedLastHitKillRewardRestartTownCatchUp(t, reconnectFlow, watcherFlow, accounts, "clh-rc-town-ground-owner", owner, runtime, spawnRef, targetVID, watcher.VID, ground, ownership, rewardDropVnum, &currentTime, wantTownMap, wantTownX, wantTownY, "combined last-hit still-dead reconnect /restart_town ground catch-up")
}

func assertCombinedLastHitKillRewardRestartTownCatchUp(
	t *testing.T,
	ownerFlow, watcherFlow service.SessionFlow,
	accounts accountstore.Store,
	ownerLogin string,
	owner loginticket.Character,
	runtime *gameRuntime,
	spawnRef string,
	targetVID uint32,
	watcherVID uint32,
	ground itemproto.GroundAddPacket,
	ownership itemproto.OwnershipPacket,
	rewardDropVnum uint32,
	currentTime *time.Time,
	wantTownMap uint32,
	wantTownX, wantTownY int32,
	context string,
) {
	t.Helper()
	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/restart_town"})))
	if err != nil {
		t.Fatalf("unexpected /restart_town error after %s: %v", context, err)
	}
	if len(restartOut) != 7 {
		t.Fatalf("expected 4 self bootstrap frames plus source peer delete, still-dead dummy delete, and source kill-reward ground delete from /restart_town after %s, got %d", context, len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after %s: %v", context, err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != wantTownX || selfAdd.Y != wantTownY {
		t.Fatalf("expected %s self bootstrap at empire town position, got %+v", context, selfAdd)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	selfPoints, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, restartOut[3]))
	if err != nil {
		t.Fatalf("decode self point change after %s: %v", context, err)
	}
	if selfPoints.Value != wantHP {
		t.Fatalf("expected %s to rebuild recovered owner HP %d, got %+v", context, wantHP, selfPoints)
	}
	peerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, restartOut[4]))
	if err != nil {
		t.Fatalf("decode source watcher delete after %s: %v", context, err)
	}
	if peerDelete.VID != watcherVID {
		t.Fatalf("expected %s source watcher delete for vid %d, got %+v", context, watcherVID, peerDelete)
	}
	mobDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, restartOut[5]))
	if err != nil {
		t.Fatalf("decode still-dead practice-mob source teardown after %s: %v", context, err)
	}
	if mobDelete.VID != targetVID {
		t.Fatalf("expected %s still-dead dummy source teardown for vid %d, got %+v", context, targetVID, mobDelete)
	}
	townGroundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, restartOut[6]))
	if err != nil {
		t.Fatalf("decode source kill-reward ground teardown after %s: %v", context, err)
	}
	if townGroundDel.VID != ground.VID {
		t.Fatalf("expected %s source ground teardown for vid %d, got %+v", context, ground.VID, townGroundDel)
	}
	for idx, raw := range restartOut {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil && deadReplay.VID == targetVID {
			t.Fatalf("expected %s source dummy teardown not to replay DEAD, got %+v at frame %d", context, deadReplay, idx)
		}
		if groundReplay, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected %s not to rematerialize the source kill-reward drop on the town map, got %+v at frame %d", context, groundReplay, idx)
		}
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatalf("expected %s transfer not to delete the still-pending source-map kill-reward handle", context)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no extra owner queued frames after %s before source-map reselect, got %d", context, len(queued))
	}
	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected source watcher to receive 1 queued owner delete after %s, got %d", context, len(watcherQueued))
	}
	watcherOwnerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil || watcherOwnerDelete.VID != owner.VID {
		t.Fatalf("unexpected watcher owner delete after %s: %+v err=%v", context, watcherOwnerDelete, err)
	}
	for idx, raw := range watcherQueued {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil && deadReplay.VID == targetVID {
			t.Fatalf("expected %s dummy teardown to stay self-only, got dummy DEAD at watcher frame %d: %+v", context, idx, deadReplay)
		}
		if groundReplay, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected %s not to rematerialize kill-reward ground for the watcher, got watcher ground add at frame %d: %+v", context, idx, groundReplay)
		}
		if groundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected %s source ground teardown to stay self-only, got watcher ground delete at frame %d: %+v", context, idx, groundDel)
		}
	}

	staleAttack, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale attack error after %s before source-map reselect: %v", context, err)
	}
	if len(staleAttack) != 0 {
		t.Fatalf("expected stale same-target ATTACK to fail closed after %s before source-map reselect, got %d frames", context, len(staleAttack))
	}
	townRetargetOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected town-side still-dead target error after %s: %v", context, err)
	}
	if len(townRetargetOut) != 0 {
		t.Fatalf("expected town-side dummy TARGET to fail closed after %s, got %d frames", context, len(townRetargetOut))
	}
	townPickupOut := pickupGroundItem(t, ownerFlow, ground.VID)
	if len(townPickupOut) != 0 {
		t.Fatalf("expected town-side kill-reward pickup to fail closed after %s, got %d frames", context, len(townPickupOut))
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatalf("expected town-side kill-reward pickup miss after %s to leave the source-map handle registered", context)
	}

	persistedTown, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted %s owner account: %v", context, err)
	}
	if len(persistedTown.Characters) != 1 ||
		persistedTown.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP ||
		persistedTown.Characters[0].Points[bootstrapExperiencePointType] != 100 ||
		persistedTown.Characters[0].Gold != 100 ||
		persistedTown.Characters[0].MapIndex != wantTownMap ||
		persistedTown.Characters[0].X != wantTownX ||
		persistedTown.Characters[0].Y != wantTownY {
		t.Fatalf("expected %s to persist recovered HP %d exp=100 gold=100 at town map=%d x=%d y=%d, got %+v", context, wantHP, wantTownMap, wantTownX, wantTownY, persistedTown.Characters)
	}

	if !runtime.RelocateCharacter(owner.Name, bootstrapMapIndex, owner.X, owner.Y) {
		t.Fatalf("expected relocate back to source map to succeed after %s", context)
	}
	ownerRelocateFrames := flushServerFrames(t, ownerFlow)
	if len(ownerRelocateFrames) != 9 {
		t.Fatalf("expected 3 source peer add frames, 4 still-dead dummy rematerialize frames, and 2 kill-reward ground rematerialize frames after %s relocate-back, got %d", context, len(ownerRelocateFrames))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, ownerRelocateFrames[0]))
	if err != nil {
		t.Fatalf("decode source watcher add after %s relocate-back: %v", context, err)
	}
	if peerAdd.VID != watcherVID {
		t.Fatalf("expected %s relocate-back source watcher add for vid %d, got %+v", context, watcherVID, peerAdd)
	}
	dummyAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, ownerRelocateFrames[3]))
	if err != nil {
		t.Fatalf("decode still-dead dummy add after %s relocate-back: %v", context, err)
	}
	if dummyAdd.VID != targetVID || dummyAdd.X != 1200 || dummyAdd.Y != 2200 || dummyAdd.RaceNum != 101 {
		t.Fatalf("expected %s relocate-back still-dead dummy add at authored home, got %+v", context, dummyAdd)
	}
	replayedDead, err := worldproto.DecodeDead(decodeSingleFrame(t, ownerRelocateFrames[6]))
	if err != nil {
		t.Fatalf("decode still-dead dummy DEAD after %s relocate-back: %v", context, err)
	}
	if replayedDead.VID != targetVID {
		t.Fatalf("expected %s relocate-back still-dead dummy DEAD for vid %d, got %+v", context, targetVID, replayedDead)
	}
	recoveryGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, ownerRelocateFrames[7]))
	if err != nil {
		t.Fatalf("decode kill-reward ground rematerialize after %s relocate-back: %v", context, err)
	}
	if recoveryGround != ground {
		t.Fatalf("expected %s relocate-back to rematerialize the same kill-reward ground add, got killer=%+v recovery=%+v", context, ground, recoveryGround)
	}
	recoveryOwnership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, ownerRelocateFrames[8]))
	if err != nil {
		t.Fatalf("decode kill-reward ownership rematerialize after %s relocate-back: %v", context, err)
	}
	if recoveryOwnership != ownership {
		t.Fatalf("expected %s relocate-back to rematerialize the same kill-reward ownership, got killer=%+v recovery=%+v", context, ownership, recoveryOwnership)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no early dummy respawn rebuild after %s relocate-back, got %d", context, len(queued))
	}

	sourceQueued := flushServerFrames(t, watcherFlow)
	if len(sourceQueued) != 3 {
		t.Fatalf("expected source watcher to receive 3 queued owner re-entry frames after %s relocate-back, got %d", context, len(sourceQueued))
	}
	for idx, raw := range sourceQueued {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil && deadReplay.VID == targetVID {
			t.Fatalf("expected %s relocate-back still-dead dummy catch-up to stay self-only, got dummy DEAD at watcher frame %d: %+v", context, idx, deadReplay)
		}
		if groundReplay, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected %s relocate-back kill-reward ground rematerialize to stay self-only, got watcher ground add at frame %d: %+v", context, idx, groundReplay)
		}
	}

	staleAttack, err = ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale attack error after %s relocate-back: %v", context, err)
	}
	if len(staleAttack) != 0 {
		t.Fatalf("expected stale same-target ATTACK to fail closed after %s relocate-back, got %d frames", context, len(staleAttack))
	}
	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected still-dead target error after %s relocate-back: %v", context, err)
	}
	if len(selectOut) != 0 {
		t.Fatalf("expected still-dead dummy to stay non-targetable after %s relocate-back, got %d frames", context, len(selectOut))
	}

	pickupOut := pickupGroundItem(t, ownerFlow, ground.VID)
	assertPostFloorItemPickupSuccessBurst(t, pickupOut, 0, rewardDropVnum, 1, context+" kill-reward pickup")
	if runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatalf("expected %s kill-reward pickup to remove the ground handle", context)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no extra owner frames after %s kill-reward pickup, got %d", context, len(queued))
	}
	watcherPickupQueued := flushServerFrames(t, watcherFlow)
	if len(watcherPickupQueued) != 1 {
		t.Fatalf("expected 1 queued watcher ground-delete after %s kill-reward pickup, got %d", context, len(watcherPickupQueued))
	}
	watcherGroundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, watcherPickupQueued[0]))
	if err != nil || watcherGroundDel.VID != ground.VID {
		t.Fatalf("unexpected watcher ground-delete after %s kill-reward pickup: %+v err=%v", context, watcherGroundDel, err)
	}

	spawned, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok || !spawned.Dead {
		t.Fatalf("expected combined last-hit dummy to stay dead after %s pickup, ok=%v snapshot=%+v", context, ok, spawned)
	}
	persisted, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted %s owner account after pickup: %v", context, err)
	}
	if len(persisted.Characters) != 1 ||
		persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP ||
		persisted.Characters[0].Points[bootstrapExperiencePointType] != 100 ||
		persisted.Characters[0].Gold != 100 ||
		persisted.Characters[0].MapIndex != bootstrapMapIndex ||
		persisted.Characters[0].X != owner.X ||
		persisted.Characters[0].Y != owner.Y {
		t.Fatalf("expected %s pickup to persist recovered HP %d exp=100 gold=100 at source map x=%d y=%d, got %+v", context, wantHP, owner.X, owner.Y, persisted.Characters)
	}
	wantInventory := []inventory.ItemInstance{{ID: uint64(ground.VID), Vnum: rewardDropVnum, Count: 1, Slot: 0}}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("expected %s pickup to persist kill-reward inventory %#v, got %#v", context, wantInventory, persisted.Characters[0].Inventory)
	}

	*currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay)
	respawnQueued := flushServerFrames(t, ownerFlow)
	if len(respawnQueued) != 4 {
		t.Fatalf("expected live dummy respawn rebuild after %s, got %d frames", context, len(respawnQueued))
	}
	for idx, raw := range respawnQueued {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected live dummy respawn rebuild after %s not to replay DEAD, got %+v at frame %d", context, deadReplay, idx)
		}
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 4 {
		t.Fatalf("expected watcher to receive 4 dummy respawn rebuild frames after %s, got %d", context, len(queued))
	}

	reselectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected dummy target after %s respawn: %v", context, err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected dummy to be freshly targetable after %s respawn, got %d frames", context, len(reselectOut))
	}
	reselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode dummy target after %s respawn: %v", context, err)
	}
	if reselected.TargetVID != targetVID || reselected.HPPercent != 100 {
		t.Fatalf("expected dummy target to be full HP after %s respawn, got %+v", context, reselected)
	}
}
