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
			Name:             "CombinedLastHitRestartTownGroundCatchUpMob",
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
		t.Fatalf("import combined last-hit town recovery ground catch-up spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)
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

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before combined last-hit town recovery ground catch-up: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before combined last-hit town recovery ground catch-up, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error on combined last-hit town recovery ground catch-up: %v", err)
	}
	if len(attackOut) != 11 {
		t.Fatalf("expected dummy death/clear/damage-info, exp/gold/ground/ownership, then owner-floor point-change/dead/clear/damage-info on combined last-hit town recovery ground catch-up, got %d frames", len(attackOut))
	}
	remainingDeath := stripKillingHitDeathPrefix(t, attackOut, targetVID, 1, "combined last-hit town recovery ground catch-up dummy death")
	if len(remainingDeath) != 8 {
		t.Fatalf("expected 4 reward frames plus 4 owner-floor frames after dummy death prefix, got %d", len(remainingDeath))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, remainingDeath[2]))
	if err != nil {
		t.Fatalf("decode combined last-hit town recovery ground catch-up ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != rewardDropVnum || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected combined last-hit town recovery ground catch-up ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, remainingDeath[3]))
	if err != nil {
		t.Fatalf("decode combined last-hit town recovery ground catch-up ownership: %v", err)
	}
	if ownership.VID != ground.VID || ownership.OwnerName != owner.Name {
		t.Fatalf("unexpected combined last-hit town recovery ground catch-up ownership: %+v", ownership)
	}
	next := assertOwnerFloorDeathSequence(t, remainingDeath[4:], 0, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "combined last-hit town recovery ground catch-up owner-floor")
	if next != 4 {
		t.Fatalf("expected combined last-hit town recovery ground catch-up owner-floor suffix to consume 4 frames, got next=%d", next)
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected combined last-hit town recovery ground catch-up drop to stay registered after owner-floor")
	}

	spawned, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok || !spawned.Dead {
		t.Fatalf("expected combined last-hit dummy to stay dead before town recovery ground catch-up, ok=%v snapshot=%+v", ok, spawned)
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 6 {
		t.Fatalf("expected 6 queued visible-peer dummy death/damage-info, ground-add/ownership, then owner death/damage-info frames on combined last-hit town recovery ground catch-up, got %d", len(queued))
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay / 2)

	restartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/restart_town"})))
	if err != nil {
		t.Fatalf("unexpected /restart_town error after combined last-hit while dummy is still dead and drop is pending: %v", err)
	}
	if len(restartOut) != 7 {
		t.Fatalf("expected 4 self bootstrap frames plus source peer delete, still-dead dummy delete, and source kill-reward ground delete from /restart_town after combined last-hit, got %d", len(restartOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, restartOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after combined last-hit /restart_town ground catch-up: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != wantTownX || selfAdd.Y != wantTownY {
		t.Fatalf("expected combined last-hit /restart_town ground catch-up self bootstrap at empire town position, got %+v", selfAdd)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	selfPoints, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, restartOut[3]))
	if err != nil {
		t.Fatalf("decode self point change after combined last-hit /restart_town ground catch-up: %v", err)
	}
	if selfPoints.Value != wantHP {
		t.Fatalf("expected combined last-hit /restart_town ground catch-up to rebuild recovered owner HP %d, got %+v", wantHP, selfPoints)
	}
	peerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, restartOut[4]))
	if err != nil {
		t.Fatalf("decode source watcher delete after combined last-hit /restart_town ground catch-up: %v", err)
	}
	if peerDelete.VID != watcher.VID {
		t.Fatalf("expected combined last-hit /restart_town source watcher delete for vid %d, got %+v", watcher.VID, peerDelete)
	}
	mobDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, restartOut[5]))
	if err != nil {
		t.Fatalf("decode still-dead practice-mob source teardown after combined last-hit /restart_town ground catch-up: %v", err)
	}
	if mobDelete.VID != targetVID {
		t.Fatalf("expected combined last-hit /restart_town still-dead dummy source teardown for vid %d, got %+v", targetVID, mobDelete)
	}
	townGroundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, restartOut[6]))
	if err != nil {
		t.Fatalf("decode source kill-reward ground teardown after combined last-hit /restart_town: %v", err)
	}
	if townGroundDel.VID != ground.VID {
		t.Fatalf("expected combined last-hit /restart_town source ground teardown for vid %d, got %+v", ground.VID, townGroundDel)
	}
	for idx, raw := range restartOut {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil && deadReplay.VID == targetVID {
			t.Fatalf("expected combined last-hit /restart_town source dummy teardown not to replay DEAD, got %+v at frame %d", deadReplay, idx)
		}
		if groundReplay, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected combined last-hit /restart_town not to rematerialize the source kill-reward drop on the town map, got %+v at frame %d", groundReplay, idx)
		}
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected combined last-hit /restart_town transfer not to delete the still-pending source-map kill-reward handle")
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no extra owner queued frames after combined last-hit /restart_town before source-map reselect, got %d", len(queued))
	}
	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 1 {
		t.Fatalf("expected source watcher to receive 1 queued owner delete after combined last-hit /restart_town, got %d", len(watcherQueued))
	}
	watcherOwnerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil || watcherOwnerDelete.VID != owner.VID {
		t.Fatalf("unexpected watcher owner delete after combined last-hit /restart_town: %+v err=%v", watcherOwnerDelete, err)
	}
	for idx, raw := range watcherQueued {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil && deadReplay.VID == targetVID {
			t.Fatalf("expected combined last-hit /restart_town dummy teardown to stay self-only, got dummy DEAD at watcher frame %d: %+v", idx, deadReplay)
		}
		if groundReplay, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected combined last-hit /restart_town not to rematerialize kill-reward ground for the watcher, got watcher ground add at frame %d: %+v", idx, groundReplay)
		}
		if groundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected combined last-hit /restart_town source ground teardown to stay self-only, got watcher ground delete at frame %d: %+v", idx, groundDel)
		}
	}

	staleAttack, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale attack error after combined last-hit /restart_town before source-map reselect: %v", err)
	}
	if len(staleAttack) != 0 {
		t.Fatalf("expected stale same-target ATTACK to fail closed after combined last-hit /restart_town before source-map reselect, got %d frames", len(staleAttack))
	}
	townRetargetOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected town-side still-dead target error after combined last-hit /restart_town: %v", err)
	}
	if len(townRetargetOut) != 0 {
		t.Fatalf("expected town-side dummy TARGET to fail closed after combined last-hit /restart_town, got %d frames", len(townRetargetOut))
	}
	townPickupOut := pickupGroundItem(t, ownerFlow, ground.VID)
	if len(townPickupOut) != 0 {
		t.Fatalf("expected town-side kill-reward pickup to fail closed after combined last-hit /restart_town, got %d frames", len(townPickupOut))
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected town-side kill-reward pickup miss to leave the source-map handle registered")
	}

	persistedTown, err := accounts.Load("clh-town-ground-owner")
	if err != nil {
		t.Fatalf("load persisted combined last-hit /restart_town owner account: %v", err)
	}
	if len(persistedTown.Characters) != 1 ||
		persistedTown.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP ||
		persistedTown.Characters[0].Points[bootstrapExperiencePointType] != 100 ||
		persistedTown.Characters[0].Gold != 100 ||
		persistedTown.Characters[0].MapIndex != wantTownMap ||
		persistedTown.Characters[0].X != wantTownX ||
		persistedTown.Characters[0].Y != wantTownY {
		t.Fatalf("expected combined last-hit /restart_town to persist recovered HP %d exp=100 gold=100 at town map=%d x=%d y=%d, got %+v", wantHP, wantTownMap, wantTownX, wantTownY, persistedTown.Characters)
	}

	if !runtime.RelocateCharacter(owner.Name, bootstrapMapIndex, owner.X, owner.Y) {
		t.Fatal("expected relocate back to source map to succeed after combined last-hit /restart_town")
	}
	ownerRelocateFrames := flushServerFrames(t, ownerFlow)
	if len(ownerRelocateFrames) != 9 {
		t.Fatalf("expected 3 source peer add frames, 4 still-dead dummy rematerialize frames, and 2 kill-reward ground rematerialize frames after combined last-hit /restart_town relocate-back, got %d", len(ownerRelocateFrames))
	}
	peerAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, ownerRelocateFrames[0]))
	if err != nil {
		t.Fatalf("decode source watcher add after combined last-hit /restart_town relocate-back: %v", err)
	}
	if peerAdd.VID != watcher.VID {
		t.Fatalf("expected combined last-hit /restart_town relocate-back source watcher add for vid %d, got %+v", watcher.VID, peerAdd)
	}
	dummyAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, ownerRelocateFrames[3]))
	if err != nil {
		t.Fatalf("decode still-dead dummy add after combined last-hit /restart_town relocate-back: %v", err)
	}
	if dummyAdd.VID != targetVID || dummyAdd.X != 1200 || dummyAdd.Y != 2200 || dummyAdd.RaceNum != 101 {
		t.Fatalf("expected combined last-hit /restart_town relocate-back still-dead dummy add at authored home, got %+v", dummyAdd)
	}
	replayedDead, err := worldproto.DecodeDead(decodeSingleFrame(t, ownerRelocateFrames[6]))
	if err != nil {
		t.Fatalf("decode still-dead dummy DEAD after combined last-hit /restart_town relocate-back: %v", err)
	}
	if replayedDead.VID != targetVID {
		t.Fatalf("expected combined last-hit /restart_town relocate-back still-dead dummy DEAD for vid %d, got %+v", targetVID, replayedDead)
	}
	recoveryGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, ownerRelocateFrames[7]))
	if err != nil {
		t.Fatalf("decode kill-reward ground rematerialize after combined last-hit /restart_town relocate-back: %v", err)
	}
	if recoveryGround != ground {
		t.Fatalf("expected /restart_town relocate-back to rematerialize the same kill-reward ground add, got killer=%+v recovery=%+v", ground, recoveryGround)
	}
	recoveryOwnership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, ownerRelocateFrames[8]))
	if err != nil {
		t.Fatalf("decode kill-reward ownership rematerialize after combined last-hit /restart_town relocate-back: %v", err)
	}
	if recoveryOwnership != ownership {
		t.Fatalf("expected /restart_town relocate-back to rematerialize the same kill-reward ownership, got killer=%+v recovery=%+v", ownership, recoveryOwnership)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no early dummy respawn rebuild after combined last-hit /restart_town relocate-back, got %d", len(queued))
	}

	sourceQueued := flushServerFrames(t, watcherFlow)
	if len(sourceQueued) != 3 {
		t.Fatalf("expected source watcher to receive 3 queued owner re-entry frames after combined last-hit /restart_town relocate-back, got %d", len(sourceQueued))
	}
	for idx, raw := range sourceQueued {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil && deadReplay.VID == targetVID {
			t.Fatalf("expected combined last-hit /restart_town relocate-back still-dead dummy catch-up to stay self-only, got dummy DEAD at watcher frame %d: %+v", idx, deadReplay)
		}
		if groundReplay, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected combined last-hit /restart_town relocate-back kill-reward ground rematerialize to stay self-only, got watcher ground add at frame %d: %+v", idx, groundReplay)
		}
	}

	staleAttack, err = ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale attack error after combined last-hit /restart_town relocate-back: %v", err)
	}
	if len(staleAttack) != 0 {
		t.Fatalf("expected stale same-target ATTACK to fail closed after combined last-hit /restart_town relocate-back, got %d frames", len(staleAttack))
	}
	selectOut, err = ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected still-dead target error after combined last-hit /restart_town relocate-back: %v", err)
	}
	if len(selectOut) != 0 {
		t.Fatalf("expected still-dead dummy to stay non-targetable after combined last-hit /restart_town relocate-back, got %d frames", len(selectOut))
	}

	pickupOut := pickupGroundItem(t, ownerFlow, ground.VID)
	assertPostFloorItemPickupSuccessBurst(t, pickupOut, 0, rewardDropVnum, 1, "combined last-hit /restart_town kill-reward pickup")
	if runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected combined last-hit /restart_town kill-reward pickup to remove the ground handle")
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no extra owner frames after combined last-hit /restart_town kill-reward pickup, got %d", len(queued))
	}
	watcherPickupQueued := flushServerFrames(t, watcherFlow)
	if len(watcherPickupQueued) != 1 {
		t.Fatalf("expected 1 queued watcher ground-delete after combined last-hit /restart_town kill-reward pickup, got %d", len(watcherPickupQueued))
	}
	watcherGroundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, watcherPickupQueued[0]))
	if err != nil || watcherGroundDel.VID != ground.VID {
		t.Fatalf("unexpected watcher ground-delete after combined last-hit /restart_town kill-reward pickup: %+v err=%v", watcherGroundDel, err)
	}

	spawned, ok = runtime.SpawnGroupByRef(spawnRef)
	if !ok || !spawned.Dead {
		t.Fatalf("expected combined last-hit dummy to stay dead after /restart_town ground catch-up pickup, ok=%v snapshot=%+v", ok, spawned)
	}
	persisted, err := accounts.Load("clh-town-ground-owner")
	if err != nil {
		t.Fatalf("load persisted combined last-hit town recovery ground catch-up owner account: %v", err)
	}
	if len(persisted.Characters) != 1 ||
		persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP ||
		persisted.Characters[0].Points[bootstrapExperiencePointType] != 100 ||
		persisted.Characters[0].Gold != 100 ||
		persisted.Characters[0].MapIndex != bootstrapMapIndex ||
		persisted.Characters[0].X != owner.X ||
		persisted.Characters[0].Y != owner.Y {
		t.Fatalf("expected combined last-hit /restart_town pickup to persist recovered HP %d exp=100 gold=100 at source map x=%d y=%d, got %+v", wantHP, owner.X, owner.Y, persisted.Characters)
	}
	wantInventory := []inventory.ItemInstance{{ID: uint64(ground.VID), Vnum: rewardDropVnum, Count: 1, Slot: 0}}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("expected combined last-hit /restart_town pickup to persist kill-reward inventory %#v, got %#v", wantInventory, persisted.Characters[0].Inventory)
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay)
	respawnQueued := flushServerFrames(t, ownerFlow)
	if len(respawnQueued) != 4 {
		t.Fatalf("expected live dummy respawn rebuild after combined last-hit /restart_town ground catch-up, got %d frames", len(respawnQueued))
	}
	for idx, raw := range respawnQueued {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected live dummy respawn rebuild after combined last-hit /restart_town ground catch-up not to replay DEAD, got %+v at frame %d", deadReplay, idx)
		}
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 4 {
		t.Fatalf("expected watcher to receive 4 dummy respawn rebuild frames after combined last-hit /restart_town ground catch-up, got %d", len(queued))
	}

	reselectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected dummy target after combined last-hit /restart_town ground catch-up respawn: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected dummy to be freshly targetable after combined last-hit /restart_town ground catch-up respawn, got %d frames", len(reselectOut))
	}
	reselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode dummy target after combined last-hit /restart_town ground catch-up respawn: %v", err)
	}
	if reselected.TargetVID != targetVID || reselected.HPPercent != 100 {
		t.Fatalf("expected dummy target to be full HP after combined last-hit /restart_town ground catch-up respawn, got %+v", reselected)
	}
}
