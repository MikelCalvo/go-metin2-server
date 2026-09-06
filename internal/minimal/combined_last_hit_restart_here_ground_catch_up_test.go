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

func TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartHereRematerializesKillRewardDrop(t *testing.T) {
	const profile = "practice_combined_last_hit_restart_here_ground_catch_up_mob"
	const spawnRef = "practice.combined_last_hit_restart_here_ground_catch_up_mob"
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
		t.Fatalf("expected %q combined last-hit recovery ground catch-up profile registration to succeed", profile)
	}
	t.Cleanup(func() { worldruntime.UnregisterStaticActorCombatProfileForTest(profile) })

	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GroundCatchOwner", 0x01030161, 0x02040161, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Points[bootstrapExperiencePointType] = 25
	owner.Gold = 40
	watcher := peerVisibilityCharacter("GroundCatchWatch", 0x01030162, 0x02040162, 1300, 2300, 0, 102, 202)
	issuePeerTicket(t, store, "clh-ground-owner", 0x61616161, owner)
	issuePeerTicket(t, store, "clh-ground-watch", 0x62626262, watcher)
	if err := accounts.Save(accountstore.Account{Login: "clh-ground-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combined last-hit recovery ground catch-up owner account: %v", err)
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
	currentTime := time.Unix(1700001403, 0)
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
			Name:             "CombinedLastHitRestartHereGroundCatchUpMob",
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
		t.Fatalf("import combined last-hit recovery ground catch-up spawn-group bundle: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 runtime practice-mob actor after import, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	ownerFlow, ownerEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "clh-ground-owner", 0x61616161)
	if len(ownerEnter) != 8 {
		t.Fatalf("expected 8 bootstrap frames for owner with visible content practice mob, got %d", len(ownerEnter))
	}
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "clh-ground-watch", 0x62626262)
	if len(watcherEnter) != 11 {
		t.Fatalf("expected 11 bootstrap frames for watcher with visible owner and content practice mob, got %d", len(watcherEnter))
	}
	defer closeSessionFlow(t, watcherFlow)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected 3 queued peer-visibility frames for owner after watcher joins, got %d", len(queued))
	}

	selectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target-selection error before combined last-hit recovery ground catch-up: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected 1 target-selection frame before combined last-hit recovery ground catch-up, got %d", len(selectOut))
	}

	attackOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error on combined last-hit recovery ground catch-up: %v", err)
	}
	if len(attackOut) != 11 {
		t.Fatalf("expected dummy death/clear/damage-info, exp/gold/ground/ownership, then owner-floor point-change/dead/clear/damage-info on combined last-hit recovery ground catch-up, got %d frames", len(attackOut))
	}
	remainingDeath := stripKillingHitDeathPrefix(t, attackOut, targetVID, 1, "combined last-hit recovery ground catch-up dummy death")
	if len(remainingDeath) != 8 {
		t.Fatalf("expected 4 reward frames plus 4 owner-floor frames after dummy death prefix, got %d", len(remainingDeath))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, remainingDeath[2]))
	if err != nil {
		t.Fatalf("decode combined last-hit recovery ground catch-up ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != rewardDropVnum || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected combined last-hit recovery ground catch-up ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, remainingDeath[3]))
	if err != nil {
		t.Fatalf("decode combined last-hit recovery ground catch-up ownership: %v", err)
	}
	if ownership.VID != ground.VID || ownership.OwnerName != owner.Name {
		t.Fatalf("unexpected combined last-hit recovery ground catch-up ownership: %+v", ownership)
	}
	next := assertOwnerFloorDeathSequence(t, remainingDeath[4:], 0, owner.VID, bootstrapPracticeMobRetaliationPointDelta, "combined last-hit recovery ground catch-up owner-floor")
	if next != 4 {
		t.Fatalf("expected combined last-hit recovery ground catch-up owner-floor suffix to consume 4 frames, got next=%d", next)
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected combined last-hit recovery ground catch-up drop to stay registered after owner-floor")
	}

	spawned, ok := runtime.SpawnGroupByRef(spawnRef)
	if !ok || !spawned.Dead {
		t.Fatalf("expected combined last-hit dummy to stay dead before recovery ground catch-up, ok=%v snapshot=%+v", ok, spawned)
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 6 {
		t.Fatalf("expected 6 queued visible-peer dummy death/damage-info, ground-add/ownership, then owner death/damage-info frames on combined last-hit recovery ground catch-up, got %d", len(queued))
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay / 2)

	recoveryOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/restart_here"})))
	if err != nil {
		t.Fatalf("unexpected /restart_here error after combined last-hit while dummy is still dead and drop is pending: %v", err)
	}
	if len(recoveryOut) != 11 {
		t.Fatalf("expected 4 self bootstrap frames plus 5 still-dead practice-mob catch-up frames plus 2 kill-reward ground rematerialize frames from /restart_here after combined last-hit, got %d", len(recoveryOut))
	}
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, recoveryOut[0]))
	if err != nil {
		t.Fatalf("decode self character add after combined last-hit /restart_here ground catch-up: %v", err)
	}
	if selfAdd.VID != owner.VID {
		t.Fatalf("expected combined last-hit /restart_here ground catch-up self bootstrap to rebuild owner vid %d, got %+v", owner.VID, selfAdd)
	}
	wantHP := initialStatsForRace(owner.RaceNum).MaxHP
	selfPoints, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, recoveryOut[3]))
	if err != nil {
		t.Fatalf("decode self point change after combined last-hit /restart_here ground catch-up: %v", err)
	}
	if selfPoints.Value != wantHP {
		t.Fatalf("expected combined last-hit /restart_here ground catch-up to rebuild recovered owner HP %d, got %+v", wantHP, selfPoints)
	}
	mobDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, recoveryOut[4]))
	if err != nil {
		t.Fatalf("decode still-dead practice-mob catch-up delete after combined last-hit /restart_here ground catch-up: %v", err)
	}
	if mobDelete.VID != targetVID {
		t.Fatalf("expected combined last-hit /restart_here still-dead catch-up delete for vid %d, got %+v", targetVID, mobDelete)
	}
	replayedDead, err := worldproto.DecodeDead(decodeSingleFrame(t, recoveryOut[8]))
	if err != nil {
		t.Fatalf("decode still-dead practice-mob catch-up DEAD after combined last-hit /restart_here ground catch-up: %v", err)
	}
	if replayedDead.VID != targetVID {
		t.Fatalf("expected combined last-hit /restart_here still-dead catch-up DEAD for vid %d, got %+v", targetVID, replayedDead)
	}
	recoveryGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, recoveryOut[9]))
	if err != nil {
		t.Fatalf("decode kill-reward ground rematerialize after combined last-hit /restart_here: %v", err)
	}
	if recoveryGround != ground {
		t.Fatalf("expected /restart_here to rematerialize the same kill-reward ground add, got killer=%+v recovery=%+v", ground, recoveryGround)
	}
	recoveryOwnership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, recoveryOut[10]))
	if err != nil {
		t.Fatalf("decode kill-reward ownership rematerialize after combined last-hit /restart_here: %v", err)
	}
	if recoveryOwnership != ownership {
		t.Fatalf("expected /restart_here to rematerialize the same kill-reward ownership, got killer=%+v recovery=%+v", ownership, recoveryOwnership)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no early dummy respawn rebuild after combined last-hit /restart_here ground catch-up, got %d", len(queued))
	}

	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 4 {
		t.Fatalf("expected 4 queued owner alive-again refresh frames after combined last-hit /restart_here ground catch-up, got %d", len(watcherQueued))
	}
	peerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, watcherQueued[0]))
	if err != nil {
		t.Fatalf("decode peer delete after combined last-hit /restart_here ground catch-up: %v", err)
	}
	if peerDelete.VID != owner.VID {
		t.Fatalf("expected combined last-hit /restart_here peer delete for owner vid %d, got %+v", owner.VID, peerDelete)
	}
	for idx, raw := range watcherQueued {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil && deadReplay.VID == targetVID {
			t.Fatalf("expected combined last-hit /restart_here still-dead dummy catch-up to stay self-only, got dummy DEAD at watcher frame %d: %+v", idx, deadReplay)
		}
		if groundReplay, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected combined last-hit /restart_here kill-reward ground rematerialize to stay self-only, got watcher ground add at frame %d: %+v", idx, groundReplay)
		}
	}

	staleAttack, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected stale attack error after combined last-hit /restart_here ground catch-up: %v", err)
	}
	if len(staleAttack) != 0 {
		t.Fatalf("expected stale same-target ATTACK to fail closed after combined last-hit /restart_here ground catch-up, got %d frames", len(staleAttack))
	}
	selectOut, err = ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected still-dead target error after combined last-hit /restart_here ground catch-up: %v", err)
	}
	if len(selectOut) != 0 {
		t.Fatalf("expected still-dead dummy to stay non-targetable after combined last-hit /restart_here ground catch-up, got %d frames", len(selectOut))
	}

	pickupOut := pickupGroundItem(t, ownerFlow, ground.VID)
	assertPostFloorItemPickupSuccessBurst(t, pickupOut, 0, rewardDropVnum, 1, "combined last-hit /restart_here kill-reward pickup")
	if runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected combined last-hit /restart_here kill-reward pickup to remove the ground handle")
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected no extra owner frames after combined last-hit /restart_here kill-reward pickup, got %d", len(queued))
	}
	watcherPickupQueued := flushServerFrames(t, watcherFlow)
	if len(watcherPickupQueued) != 1 {
		t.Fatalf("expected 1 queued watcher ground-delete after combined last-hit /restart_here kill-reward pickup, got %d", len(watcherPickupQueued))
	}
	watcherGroundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, watcherPickupQueued[0]))
	if err != nil || watcherGroundDel.VID != ground.VID {
		t.Fatalf("unexpected watcher ground-delete after combined last-hit /restart_here kill-reward pickup: %+v err=%v", watcherGroundDel, err)
	}

	spawned, ok = runtime.SpawnGroupByRef(spawnRef)
	if !ok || !spawned.Dead {
		t.Fatalf("expected combined last-hit dummy to stay dead after /restart_here ground catch-up pickup, ok=%v snapshot=%+v", ok, spawned)
	}
	persisted, err := accounts.Load("clh-ground-owner")
	if err != nil {
		t.Fatalf("load persisted combined last-hit recovery ground catch-up owner account: %v", err)
	}
	if len(persisted.Characters) != 1 ||
		persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantHP ||
		persisted.Characters[0].Points[bootstrapExperiencePointType] != 100 ||
		persisted.Characters[0].Gold != 100 {
		t.Fatalf("expected combined last-hit /restart_here pickup to persist recovered HP %d exp=100 gold=100, got %+v", wantHP, persisted.Characters)
	}
	wantInventory := []inventory.ItemInstance{{ID: uint64(ground.VID), Vnum: rewardDropVnum, Count: 1, Slot: 0}}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("expected combined last-hit /restart_here pickup to persist kill-reward inventory %#v, got %#v", wantInventory, persisted.Characters[0].Inventory)
	}

	currentTime = currentTime.Add(worldruntime.PracticeMobBootstrapRespawnDelay)
	respawnQueued := flushServerFrames(t, ownerFlow)
	if len(respawnQueued) != 4 {
		t.Fatalf("expected live dummy respawn rebuild after combined last-hit /restart_here ground catch-up, got %d frames", len(respawnQueued))
	}
	for idx, raw := range respawnQueued {
		if deadReplay, err := worldproto.DecodeDead(decodeSingleFrame(t, raw)); err == nil {
			t.Fatalf("expected live dummy respawn rebuild after combined last-hit /restart_here ground catch-up not to replay DEAD, got %+v at frame %d", deadReplay, idx)
		}
	}
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 4 {
		t.Fatalf("expected watcher to receive 4 dummy respawn rebuild frames after combined last-hit /restart_here ground catch-up, got %d", len(queued))
	}

	reselectOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected dummy target after combined last-hit /restart_here ground catch-up respawn: %v", err)
	}
	if len(reselectOut) != 1 {
		t.Fatalf("expected dummy to be freshly targetable after combined last-hit /restart_here ground catch-up respawn, got %d frames", len(reselectOut))
	}
	reselected, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectOut[0]))
	if err != nil {
		t.Fatalf("decode dummy target after combined last-hit /restart_here ground catch-up respawn: %v", err)
	}
	if reselected.TargetVID != targetVID || reselected.HPPercent != 100 {
		t.Fatalf("expected dummy target to be full HP after combined last-hit /restart_here ground catch-up respawn, got %+v", reselected)
	}
}
