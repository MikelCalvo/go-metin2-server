package minimal

import (
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartHereCatchesUpStillDeadDummy(t *testing.T) {
	const profile = "practice_combined_last_hit_phase_select_still_dead_mob"
	const spawnRef = "practice.combined_last_hit_phase_select_still_dead_mob"
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
		t.Fatalf("expected %q combined last-hit still-dead /phase_select profile registration to succeed", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PhaseSelectStillDeadOwner", 0x01030181, 0x02040181, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	watcher := peerVisibilityCharacter("PhaseSelectStillDeadWatch", 0x01030182, 0x02040182, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "clh-ps-still-dead-owner", 0x81818181, owner)
	issuePeerTicket(t, store, "clh-ps-still-dead-watch", 0x82828282, watcher)
	if err := accounts.Save(accountstore.Account{Login: "clh-ps-still-dead-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combined last-hit still-dead /phase_select owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "clh-ps-still-dead-watch", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed combined last-hit still-dead /phase_select watcher account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700001501, 0)
	runtime.now = func() time.Time { return currentTime }
	targetVID := importCombinedLastHitStillDeadDummy(t, runtime, profile, spawnRef, "CombinedLastHitPhaseSelectStillDeadMob")

	factory := runtime.SessionFactory()
	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, factory, "clh-ps-still-dead-owner", 0x81818181)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, factory, "clh-ps-still-dead-watch", 0x82828282)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	defer closeSessionFlow(t, ownerFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	driveCombinedLastHitStillDeadDummyKill(t, ownerFlow, watcherFlow, runtime, spawnRef, targetVID, owner.VID, "combined last-hit still-dead /phase_select")

	persistedBeforePhaseSelect, err := accounts.Load("clh-ps-still-dead-owner")
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
	assertWatcherOwnerLeave(t, watcherFlow, owner.VID, "combined last-hit still-dead /phase_select")

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
	assertCombinedLastHitStillDeadOwnerReentrySkipsDummy(t, reenterOut, owner.VID, watcher.VID, targetVID, "combined last-hit still-dead /phase_select re-entry")
	assertWatcherStillDeadOwnerReentry(t, watcherFlow, owner.VID, "combined last-hit still-dead /phase_select re-entry")
	assertStillDeadDummyUntargetable(t, ownerFlow, runtime, spawnRef, targetVID, "combined last-hit still-dead /phase_select re-entry")

	assertCombinedLastHitStillDeadRestartHereCatchUp(t, ownerFlow, watcherFlow, accounts, "clh-ps-still-dead-owner", owner, runtime, spawnRef, targetVID, &currentTime, "CombinedLastHitPhaseSelectStillDeadMob", "combined last-hit still-dead /phase_select /restart_here")
}

func TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartHereCatchesUpStillDeadDummy(t *testing.T) {
	const profile = "practice_combined_last_hit_reconnect_still_dead_mob"
	const spawnRef = "practice.combined_last_hit_reconnect_still_dead_mob"
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
		t.Fatalf("expected %q combined last-hit still-dead reconnect profile registration to succeed", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ReconnectStillDeadOwner", 0x01030191, 0x02040191, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	watcher := peerVisibilityCharacter("ReconnectStillDeadWatch", 0x01030192, 0x02040192, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "clh-rc-still-dead-owner", 0x91919191, owner)
	issuePeerTicket(t, store, "clh-rc-still-dead-watch", 0x92929292, watcher)
	if err := accounts.Save(accountstore.Account{Login: "clh-rc-still-dead-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combined last-hit still-dead reconnect owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "clh-rc-still-dead-watch", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed combined last-hit still-dead reconnect watcher account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := interactionstore.NewFileStore(t.TempDir() + "/interaction-definitions.json")
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700001502, 0)
	runtime.now = func() time.Time { return currentTime }
	targetVID := importCombinedLastHitStillDeadDummy(t, runtime, profile, spawnRef, "CombinedLastHitReconnectStillDeadMob")

	factory := runtime.SessionFactory()
	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, factory, "clh-rc-still-dead-owner", 0x91919191)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, factory, "clh-rc-still-dead-watch", 0x92929292)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	driveCombinedLastHitStillDeadDummyKill(t, ownerFlow, watcherFlow, runtime, spawnRef, targetVID, owner.VID, "combined last-hit still-dead reconnect")

	persistedBeforeReconnect, err := accounts.Load("clh-rc-still-dead-owner")
	if err != nil {
		t.Fatalf("load persisted owner account after combined last-hit before reconnect: %v", err)
	}
	if len(persistedBeforeReconnect.Characters) != 1 || persistedBeforeReconnect.Characters[0].Points[bootstrapPlayerPointValueIndex] != 0 {
		t.Fatalf("expected combined last-hit to persist owner HP floor 0 before reconnect, got %+v", persistedBeforeReconnect.Characters)
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay / 2)

	closeSessionFlow(t, ownerFlow)
	assertWatcherOwnerLeave(t, watcherFlow, owner.VID, "combined last-hit still-dead reconnect")

	issuePeerTicket(t, store, "clh-rc-still-dead-owner", 0x93939393, persistedBeforeReconnect.Characters[0])
	reconnectFlow, reconnectEnter := enterGameWithLoginTicket(t, factory, "clh-rc-still-dead-owner", 0x93939393)
	defer closeSessionFlow(t, reconnectFlow)
	assertCombinedLastHitStillDeadOwnerReentrySkipsDummy(t, reconnectEnter, owner.VID, watcher.VID, targetVID, "combined last-hit still-dead reconnect")
	assertWatcherStillDeadOwnerReentry(t, watcherFlow, owner.VID, "combined last-hit still-dead reconnect")
	assertStillDeadDummyUntargetable(t, reconnectFlow, runtime, spawnRef, targetVID, "combined last-hit still-dead reconnect")

	assertCombinedLastHitStillDeadRestartHereCatchUp(t, reconnectFlow, watcherFlow, accounts, "clh-rc-still-dead-owner", owner, runtime, spawnRef, targetVID, &currentTime, "CombinedLastHitReconnectStillDeadMob", "combined last-hit still-dead reconnect /restart_here")
}

func importCombinedLastHitStillDeadDummy(t *testing.T, runtime *gameRuntime, profile, spawnRef, name string) uint32 {
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
			Ref:           spawnRef,
			Name:          name,
			MapIndex:      bootstrapMapIndex,
			X:             1200,
			Y:             2200,
			RaceNum:       101,
			CombatProfile: profile,
		}},
	}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import combined last-hit still-dead spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	return uint32(actors[0].EntityID)
}

func driveCombinedLastHitStillDeadDummyKill(t *testing.T, ownerFlow, watcherFlow service.SessionFlow, runtime *gameRuntime, spawnRef string, targetVID, ownerVID uint32, context string) {
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
	if len(attackOut) != 7 {
		t.Fatalf("expected dummy death/clear/damage-info then owner-floor point-change/dead/clear/damage-info on %s, got %d frames", context, len(attackOut))
	}
	remainingDeath := stripKillingHitDeathPrefix(t, attackOut, targetVID, 1, context+" dummy death")
	next := assertOwnerFloorDeathSequence(t, remainingDeath, 0, ownerVID, bootstrapPracticeMobRetaliationPointDelta, context+" owner-floor")
	if next != 4 {
		t.Fatalf("expected %s owner-floor suffix to consume 4 frames, got next=%d", context, next)
	}

	spawned, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok || !spawned.Dead {
		t.Fatalf("expected combined last-hit dummy to stay dead before %s re-entry, ok=%v snapshot=%+v", context, ok, spawned)
	}
	queued := flushServerFrames(t, watcherFlow)
	if len(queued) != 4 {
		t.Fatalf("expected 4 queued visible-peer dummy DEAD + dummy damage-info + owner DEAD + owner damage-info frames on %s, got %d", context, len(queued))
	}
}

func assertCombinedLastHitStillDeadOwnerReentrySkipsDummy(t *testing.T, frames [][]byte, ownerVID, watcherVID, dummyVID uint32, context string) {
	t.Helper()
	if len(frames) != 9 {
		t.Fatalf("expected 9 bootstrap frames including self DEAD and living peer entry for %s, got %d", context, len(frames))
	}
	reenterPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[4]))
	if err != nil {
		t.Fatalf("decode %s bootstrap point-change: %v", context, err)
	}
	if reenterPointChange.Value != 0 || reenterPointChange.Amount != 0 {
		t.Fatalf("expected %s bootstrap to rebuild persisted points[%d] at floor 0, got %+v", context, bootstrapPlayerPointValueIndex, reenterPointChange)
	}
	reenterDead, err := worldproto.DecodeDead(decodeSingleFrame(t, frames[5]))
	if err != nil {
		t.Fatalf("decode %s bootstrap dead replay: %v", context, err)
	}
	if reenterDead.VID != ownerVID {
		t.Fatalf("expected %s bootstrap dead replay for owner vid %d, got %+v", context, ownerVID, reenterDead)
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, frames[6]))
	if err != nil {
		t.Fatalf("decode living peer add during %s: %v", context, err)
	}
	if peerAdd.VID != watcherVID {
		t.Fatalf("expected %s bootstrap to include living watcher peer vid %d, got %+v", context, watcherVID, peerAdd)
	}
	for idx, raw := range frames {
		if add, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, raw)); err == nil && add.VID == dummyVID {
			t.Fatalf("expected %s to skip still-dead dummy visibility while owner is at HP floor, got dummy add at frame %d: %+v", context, idx, add)
		}
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil && deadReplay.VID == dummyVID {
			t.Fatalf("expected %s to skip still-dead dummy DEAD while owner is at HP floor, got dummy DEAD at frame %d: %+v", context, idx, deadReplay)
		}
	}
}

func assertWatcherOwnerLeave(t *testing.T, watcherFlow service.SessionFlow, ownerVID uint32, context string) {
	t.Helper()
	leaveQueued := flushServerFrames(t, watcherFlow)
	if len(leaveQueued) != 1 {
		t.Fatalf("expected watcher to receive 1 queued owner delete after %s, got %d", context, len(leaveQueued))
	}
	ownerLeaveDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, leaveQueued[0]))
	if err != nil {
		t.Fatalf("decode watcher owner-delete after %s: %v", context, err)
	}
	if ownerLeaveDelete.VID != ownerVID {
		t.Fatalf("expected watcher owner-delete for vid %d after %s, got %+v", ownerVID, context, ownerLeaveDelete)
	}
}

func assertWatcherStillDeadOwnerReentry(t *testing.T, watcherFlow service.SessionFlow, ownerVID uint32, context string) {
	t.Helper()
	reentryQueued := flushServerFrames(t, watcherFlow)
	if len(reentryQueued) != 4 {
		t.Fatalf("expected watcher to receive 3 queued still-dead owner re-entry frames plus trailing DEAD after %s, got %d", context, len(reentryQueued))
	}
	reentryDead, err := worldproto.DecodeDead(decodeSingleFrame(t, reentryQueued[3]))
	if err != nil {
		t.Fatalf("decode watcher trailing dead replay after %s: %v", context, err)
	}
	if reentryDead.VID != ownerVID {
		t.Fatalf("expected watcher trailing DEAD(owner_vid) after %s, got %+v", context, reentryDead)
	}
}

func assertStillDeadDummyUntargetable(t *testing.T, ownerFlow service.SessionFlow, runtime *gameRuntime, spawnRef string, targetVID uint32, context string) {
	t.Helper()
	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected still-dead target error after %s: %v", context, err)
	}
	if len(selectOut) != 0 {
		t.Fatalf("expected still-dead dummy to stay non-targetable after %s, got %d frames", context, len(selectOut))
	}
	staleAttack, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale attack error after %s: %v", context, err)
	}
	if len(staleAttack) != 0 {
		t.Fatalf("expected stale same-target ATTACK to fail closed after %s, got %d frames", context, len(staleAttack))
	}
	spawned, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok || !spawned.Dead {
		t.Fatalf("expected combined last-hit dummy to stay dead after %s, ok=%v snapshot=%+v", context, ok, spawned)
	}
}

func assertCombinedLastHitStillDeadRestartHereCatchUp(
	t *testing.T,
	ownerFlow, watcherFlow service.SessionFlow,
	accounts accountstore.Store,
	ownerLogin string,
	owner loginticket.Character,
	runtime *gameRuntime,
	spawnRef string,
	targetVID uint32,
	currentTime *time.Time,
	mobName string,
	context string,
) {
	t.Helper()
	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/restart_here"})))
	if err != nil {
		t.Fatalf("unexpected /restart_here error after %s: %v", context, err)
	}
	if len(restartOut) != 9 {
		t.Fatalf("expected 4 self bootstrap frames plus 5 still-dead practice-mob catch-up frames from /restart_here after %s, got %d", context, len(restartOut))
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
	mobAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[5]))
	if err != nil {
		t.Fatalf("decode still-dead practice-mob catch-up add after %s: %v", context, err)
	}
	if mobAdd.VID != targetVID || mobAdd.X != 1200 || mobAdd.Y != 2200 || mobAdd.RaceNum != 101 {
		t.Fatalf("expected %s still-dead catch-up add for vid %d at authored home, got %+v", context, targetVID, mobAdd)
	}
	mobInfo, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, restartOut[6]))
	if err != nil {
		t.Fatalf("decode still-dead practice-mob catch-up info after %s: %v", context, err)
	}
	if mobInfo.VID != targetVID || mobInfo.Name != mobName {
		t.Fatalf("expected %s still-dead catch-up info for vid %d name %q, got %+v", context, targetVID, mobName, mobInfo)
	}
	mobUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, restartOut[7]))
	if err != nil {
		t.Fatalf("decode still-dead practice-mob catch-up update after %s: %v", context, err)
	}
	if mobUpdate.VID != targetVID {
		t.Fatalf("expected %s still-dead catch-up update for vid %d, got %+v", context, targetVID, mobUpdate)
	}
	replayedDead, err := worldproto.DecodeDead(decodeSingleFrame(t, restartOut[8]))
	if err != nil {
		t.Fatalf("decode still-dead practice-mob catch-up DEAD after %s: %v", context, err)
	}
	if replayedDead.VID != targetVID {
		t.Fatalf("expected %s still-dead catch-up DEAD for vid %d, got %+v", context, targetVID, replayedDead)
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
	}

	assertStillDeadDummyUntargetable(t, ownerFlow, runtime, spawnRef, targetVID, context+" still-dead catch-up")

	persisted, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted %s owner account: %v", context, err)
	}
	if len(persisted.Characters) != 1 || persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP {
		t.Fatalf("expected %s to persist recovered owner HP %d, got %+v", context, wantHP, persisted.Characters)
	}

	*currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay)
	respawnQueued := flushServerFrames(t, ownerFlow)
	if len(respawnQueued) != 4 {
		t.Fatalf("expected live dummy respawn rebuild after %s still-dead catch-up, got %d frames", context, len(respawnQueued))
	}
	respawnDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, respawnQueued[0]))
	if err != nil {
		t.Fatalf("decode dummy respawn delete after %s: %v", context, err)
	}
	if respawnDelete.VID != targetVID {
		t.Fatalf("expected dummy respawn delete for vid %d after %s, got %+v", targetVID, context, respawnDelete)
	}
	respawnAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, respawnQueued[1]))
	if err != nil {
		t.Fatalf("decode dummy respawn add after %s: %v", context, err)
	}
	if respawnAdd.VID != targetVID || respawnAdd.X != 1200 || respawnAdd.Y != 2200 {
		t.Fatalf("expected dummy respawn add at authored home after %s, got %+v", context, respawnAdd)
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
