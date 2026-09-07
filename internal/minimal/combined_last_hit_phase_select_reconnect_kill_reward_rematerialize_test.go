package minimal

import (
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
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

func TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartHereRematerializesKillRewardDrop(t *testing.T) {
	const profile = "practice_combined_last_hit_phase_select_ground_catch_up_mob"
	const spawnRef = "practice.combined_last_hit_phase_select_ground_catch_up_mob"
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
		t.Fatalf("expected %q combined last-hit /phase_select ground catch-up profile registration to succeed", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PSGroundCatchOwner", 0x010301a1, 0x020401a1, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Points[bootstrapExperiencePointType] = 25
	owner.Gold = 40
	watcher := peerVisibilityCharacter("PSGroundCatchWatch", 0x010301a2, 0x020401a2, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "clh-ps-ground-owner", 0xa1a1a1a1, owner)
	issuePeerTicket(t, store, "clh-ps-ground-watch", 0xa2a2a2a2, watcher)
	if err := accounts.Save(accountstore.Account{Login: "clh-ps-ground-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combined last-hit /phase_select ground catch-up owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "clh-ps-ground-watch", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed combined last-hit /phase_select ground catch-up watcher account: %v", err)
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
	currentTime := time.Unix(1700001503, 0)
	runtime.now = func() time.Time { return currentTime }
	targetVID := importCombinedLastHitKillRewardDummy(t, runtime, profile, spawnRef, "CombinedLastHitPhaseSelectGroundCatchUpMob", rewardExperience, rewardGold, rewardDropVnum)

	factory := runtime.SessionFactory()
	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, factory, "clh-ps-ground-owner", 0xa1a1a1a1)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, factory, "clh-ps-ground-watch", 0xa2a2a2a2)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	defer closeSessionFlow(t, ownerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	ground, ownership := driveCombinedLastHitKillRewardDummyKill(t, ownerFlow, watcherFlow, runtime, spawnRef, targetVID, owner, rewardDropVnum, "combined last-hit still-dead /phase_select ground catch-up")

	persistedBeforePhaseSelect, err := accounts.Load("clh-ps-ground-owner")
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
	assertWatcherOwnerLeave(t, watcherFlow, owner.VID, "combined last-hit still-dead /phase_select ground catch-up")
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
	assertCombinedLastHitStillDeadOwnerReentrySkipsDummy(t, reenterOut, owner.VID, watcher.VID, targetVID, "combined last-hit still-dead /phase_select ground catch-up re-entry")
	assertCombinedLastHitOwnerReentrySkipsKillRewardDrop(t, reenterOut, ground.VID, "combined last-hit still-dead /phase_select ground catch-up re-entry")
	assertWatcherStillDeadOwnerReentry(t, watcherFlow, owner.VID, "combined last-hit still-dead /phase_select ground catch-up re-entry")
	assertStillDeadDummyUntargetable(t, ownerFlow, runtime, spawnRef, targetVID, "combined last-hit still-dead /phase_select ground catch-up re-entry")
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected parked kill-reward drop to survive /phase_select re-entry")
	}

	assertCombinedLastHitKillRewardRestartHereCatchUp(t, ownerFlow, watcherFlow, accounts, "clh-ps-ground-owner", owner, runtime, spawnRef, targetVID, ground, ownership, rewardDropVnum, &currentTime, "CombinedLastHitPhaseSelectGroundCatchUpMob", "combined last-hit still-dead /phase_select /restart_here ground catch-up")
}

func TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartHereRematerializesKillRewardDrop(t *testing.T) {
	const profile = "practice_combined_last_hit_reconnect_ground_catch_up_mob"
	const spawnRef = "practice.combined_last_hit_reconnect_ground_catch_up_mob"
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
		t.Fatalf("expected %q combined last-hit reconnect ground catch-up profile registration to succeed", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RCGroundCatchOwner", 0x010301b1, 0x020401b1, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Points[bootstrapExperiencePointType] = 25
	owner.Gold = 40
	watcher := peerVisibilityCharacter("RCGroundCatchWatch", 0x010301b2, 0x020401b2, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "clh-rc-ground-owner", 0xb1b1b1b1, owner)
	issuePeerTicket(t, store, "clh-rc-ground-watch", 0xb2b2b2b2, watcher)
	if err := accounts.Save(accountstore.Account{Login: "clh-rc-ground-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combined last-hit reconnect ground catch-up owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "clh-rc-ground-watch", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed combined last-hit reconnect ground catch-up watcher account: %v", err)
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
	currentTime := time.Unix(1700001504, 0)
	runtime.now = func() time.Time { return currentTime }
	targetVID := importCombinedLastHitKillRewardDummy(t, runtime, profile, spawnRef, "CombinedLastHitReconnectGroundCatchUpMob", rewardExperience, rewardGold, rewardDropVnum)

	factory := runtime.SessionFactory()
	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, factory, "clh-rc-ground-owner", 0xb1b1b1b1)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, factory, "clh-rc-ground-watch", 0xb2b2b2b2)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	ground, ownership := driveCombinedLastHitKillRewardDummyKill(t, ownerFlow, watcherFlow, runtime, spawnRef, targetVID, owner, rewardDropVnum, "combined last-hit still-dead reconnect ground catch-up")

	persistedBeforeReconnect, err := accounts.Load("clh-rc-ground-owner")
	if err != nil {
		t.Fatalf("load persisted owner account after combined last-hit before reconnect: %v", err)
	}
	if len(persistedBeforeReconnect.Characters) != 1 || persistedBeforeReconnect.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected combined last-hit to persist owner HP floor 0 before reconnect, got %+v", persistedBeforeReconnect.Characters)
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay / 2)

	closeSessionFlow(t, ownerFlow)
	assertWatcherOwnerLeave(t, watcherFlow, owner.VID, "combined last-hit still-dead reconnect ground catch-up")
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected reconnect floor-leave to park the pending kill-reward drop instead of deleting it")
	}

	issuePeerTicket(t, store, "clh-rc-ground-owner", 0xb3b3b3b3, persistedBeforeReconnect.Characters[0])
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "clh-rc-ground-owner", 0xb3b3b3b3)
	defer closeSessionFlow(t, reconnectFlow)
	assertCombinedLastHitStillDeadOwnerReentrySkipsDummy(t, reconnectEnter, owner.VID, watcher.VID, targetVID, "combined last-hit still-dead reconnect ground catch-up")
	assertCombinedLastHitOwnerReentrySkipsKillRewardDrop(t, reconnectEnter, ground.VID, "combined last-hit still-dead reconnect ground catch-up")
	assertWatcherStillDeadOwnerReentry(t, watcherFlow, owner.VID, "combined last-hit still-dead reconnect ground catch-up")
	assertStillDeadDummyUntargetable(t, reconnectFlow, runtime, spawnRef, targetVID, "combined last-hit still-dead reconnect ground catch-up")
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected parked kill-reward drop to survive reconnect")
	}

	assertCombinedLastHitKillRewardRestartHereCatchUp(t, reconnectFlow, watcherFlow, accounts, "clh-rc-ground-owner", owner, runtime, spawnRef, targetVID, ground, ownership, rewardDropVnum, &currentTime, "CombinedLastHitReconnectGroundCatchUpMob", "combined last-hit still-dead reconnect /restart_here ground catch-up")
}

func importCombinedLastHitKillRewardDummy(t *testing.T, runtime *gameRuntime, profile, spawnRef, name string, rewardExperience, rewardGold uint64, rewardDropVnum uint32) uint32 {
	t.Helper()
	bundle := contentbundle.Bundle{
		CombatProfiles: []worldruntime.StaticActorCombatProfileSnapshot{{
			Profile:               profile,
			MaxHP:                 1,
			DamagePerNormalAttack: 1,
			AttackValue:           1,
			DefenseValue:          0,
			Level:                 worldruntime.TrainingDummyBootstrapLevel,
			Rank:                  worldruntime.TrainingDummyBootstrapRank,
			RespawnDelayMs:        worldruntime.PracticeMobBootstrapRespawnDelay.Milliseconds(),
			RetaliationPointDelta: worldruntime.PracticeMobBootstrapRetaliationPointDelta,
		}},
		SpawnGroups: []contentbundle.SpawnGroup{{
			Ref:              spawnRef,
			Name:             name,
			MapIndex:         bootstrapMapIndex,
			X:                1200,
			Y:                2200,
			RaceNum:          101,
			CombatProfile:    profile,
			RewardExperience: rewardExperience,
			RewardGold:       rewardGold,
			RewardDropVnums:  []uint32{rewardDropVnum},
		}},
		ItemTemplates: rewardDropItemTemplates(rewardDropVnum),
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import combined last-hit kill-reward spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	return uint32(actors[0].EntityID)
}

func driveCombinedLastHitKillRewardDummyKill(
	t *testing.T,
	ownerFlow, watcherFlow service.SessionFlow,
	runtime *gameRuntime,
	spawnRef string,
	targetVID uint32,
	owner loginticket.Character,
	rewardDropVnum uint32,
	context string,
) (itemproto.GroundAddPacket, itemproto.OwnershipPacket) {
	t.Helper()
	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before %s: %v", context, err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before %s, got %d", context, len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error on %s: %v", context, err)
	}
	if len(attackOut) != 11 {
		t.Fatalf("expected dummy death/clear/damage-info, exp/gold/ground/ownership, then owner-floor point-change/dead/clear/damage-info on %s, got %d frames", context, len(attackOut))
	}
	remainingDeath := stripKillingHitDeathPrefix(t, attackOut, targetVID, 1, context+" dummy death")
	if len(remainingDeath) != 8 {
		t.Fatalf("expected 4 reward frames plus 4 owner-floor frames after dummy death prefix on %s, got %d", context, len(remainingDeath))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, remainingDeath[2]))
	if err != nil {
		t.Fatalf("decode %s ground add: %v", context, err)
	}
	if ground.VID == 0 || ground.Vnum != rewardDropVnum || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected %s ground add: %+v", context, ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, remainingDeath[3]))
	if err != nil {
		t.Fatalf("decode %s ownership: %v", context, err)
	}
	if ownership.VID != ground.VID || ownership.OwnerName != owner.Name {
		t.Fatalf("unexpected %s ownership: %+v", context, ownership)
	}
	next := assertOwnerFloorDeathSequence(t, remainingDeath[4:], 0, owner.VID, bootstrapPracticeMobRetaliationPointDelta, context+" owner-floor")
	if next != 4 {
		t.Fatalf("expected %s owner-floor suffix to consume 4 frames, got next=%d", context, next)
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatalf("expected %s drop to stay registered after owner-floor", context)
	}

	spawned, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok || !spawned.Dead {
		t.Fatalf("expected combined last-hit dummy to stay dead before %s re-entry, ok=%v snapshot=%+v", context, ok, spawned)
	}
	queued := flushServerFrames(t, watcherFlow)
	if len(queued) != 6 {
		t.Fatalf("expected 6 queued visible-peer dummy death/damage-info, ground-add/ownership, then owner death/damage-info frames on %s, got %d", context, len(queued))
	}
	return ground, ownership
}

func assertCombinedLastHitOwnerReentrySkipsKillRewardDrop(t *testing.T, frames [][]byte, groundVID uint32, context string) {
	t.Helper()
	for idx, raw := range frames {
		if add, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, raw)); err == nil && add.VID == groundVID {
			t.Fatalf("expected %s to skip kill-reward rematerialize while owner is at HP floor, got ground add at frame %d: %+v", context, idx, add)
		}
		if ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, raw)); err == nil && ownership.VID == groundVID {
			t.Fatalf("expected %s to skip kill-reward ownership rematerialize while owner is at HP floor, got ownership at frame %d: %+v", context, idx, ownership)
		}
	}
}

func assertCombinedLastHitKillRewardRestartHereCatchUp(
	t *testing.T,
	ownerFlow, watcherFlow service.SessionFlow,
	accounts accountstore.Store,
	ownerLogin string,
	owner loginticket.Character,
	runtime *gameRuntime,
	spawnRef string,
	targetVID uint32,
	ground itemproto.GroundAddPacket,
	ownership itemproto.OwnershipPacket,
	rewardDropVnum uint32,
	currentTime *time.Time,
	mobName string,
	context string,
) {
	t.Helper()
	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/restart_here"})))
	if err != nil {
		t.Fatalf("unexpected /restart_here error after %s: %v", context, err)
	}
	if len(restartOut) != 11 {
		t.Fatalf("expected 4 self bootstrap frames plus 5 still-dead practice-mob catch-up frames plus 2 kill-reward ground rematerialize frames from /restart_here after %s, got %d", context, len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after %s: %v", context, err)
	}
	if selfAdd.VID != owner.VID {
		t.Fatalf("expected %s self bootstrap to rebuild owner vid %d, got %+v", context, owner.VID, selfAdd)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	selfPoints, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, restartOut[3]))
	if err != nil {
		t.Fatalf("decode self point change after %s: %v", context, err)
	}
	if selfPoints.Value != wantHP {
		t.Fatalf("expected %s to rebuild recovered owner HP %d, got %+v", context, wantHP, selfPoints)
	}
	mobDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, restartOut[4]))
	if err != nil {
		t.Fatalf("decode still-dead practice-mob catch-up delete after %s: %v", context, err)
	}
	if mobDelete.VID != targetVID {
		t.Fatalf("expected %s still-dead catch-up delete for vid %d, got %+v", context, targetVID, mobDelete)
	}
	replayedDead, err := worldproto.DecodeDead(decodeSingleFrame(t, restartOut[8]))
	if err != nil {
		t.Fatalf("decode still-dead practice-mob catch-up DEAD after %s: %v", context, err)
	}
	if replayedDead.VID != targetVID {
		t.Fatalf("expected %s still-dead catch-up DEAD for vid %d, got %+v", context, targetVID, replayedDead)
	}
	recoveryGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, restartOut[9]))
	if err != nil {
		t.Fatalf("decode kill-reward ground rematerialize after %s: %v", context, err)
	}
	if recoveryGround != ground {
		t.Fatalf("expected %s to rematerialize the same kill-reward ground add, got killer=%+v recovery=%+v", context, ground, recoveryGround)
	}
	recoveryOwnership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, restartOut[10]))
	if err != nil {
		t.Fatalf("decode kill-reward ownership rematerialize after %s: %v", context, err)
	}
	if recoveryOwnership != ownership {
		t.Fatalf("expected %s to rematerialize the same kill-reward ownership, got killer=%+v recovery=%+v", context, ownership, recoveryOwnership)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no early dummy respawn rebuild after %s still-dead catch-up, got %d", context, len(queued))
	}

	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 4 {
		t.Fatalf("expected 4 queued owner alive-again refresh frames after %s, got %d", context, len(watcherQueued))
	}
	peerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil {
		t.Fatalf("decode peer delete after %s: %v", context, err)
	}
	if peerDelete.VID != owner.VID {
		t.Fatalf("expected %s peer delete for owner vid %d, got %+v", context, owner.VID, peerDelete)
	}
	for idx, raw := range watcherQueued {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil && deadReplay.VID == targetVID {
			t.Fatalf("expected %s still-dead dummy catch-up to stay self-only, got dummy DEAD at watcher frame %d: %+v", context, idx, deadReplay)
		}
		if groundReplay, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected %s kill-reward ground rematerialize to stay self-only, got watcher ground add at frame %d: %+v", context, idx, groundReplay)
		}
	}

	assertStillDeadDummyUntargetable(t, ownerFlow, runtime, spawnRef, targetVID, context+" still-dead catch-up")

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
		t.Fatalf("load persisted %s owner account: %v", context, err)
	}
	if len(persisted.Characters) != 1 ||
		persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP ||
		persisted.Characters[0].Points[bootstrapExperiencePointType] != 100 ||
		persisted.Characters[0].Gold != 100 {
		t.Fatalf("expected %s pickup to persist recovered HP %d exp=100 gold=100, got %+v", context, wantHP, persisted.Characters)
	}
	wantInventory := []inventory.ItemInstance{{ID: uint64(ground.VID), Vnum: rewardDropVnum, Count: 1, Slot: 0}}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("expected %s pickup to persist kill-reward inventory %#v, got %#v", context, wantInventory, persisted.Characters[0].Inventory)
	}

	*currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay)
	respawnQueued := flushServerFrames(t, ownerFlow)
	if len(respawnQueued) != 4 {
		t.Fatalf("expected live dummy respawn rebuild after %s still-dead catch-up, got %d frames", context, len(respawnQueued))
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
