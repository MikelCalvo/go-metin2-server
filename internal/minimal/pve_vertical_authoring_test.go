package minimal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/cubestore"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	effectproto "github.com/MikelCalvo/go-metin2-server/internal/proto/effect"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

func loadBootstrapPveVerticalAuthoringBundle(t *testing.T) contentbundle.Bundle {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve minimal test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "docs", "examples", "bootstrap-pve-vertical-authoring-bundle.json"))
	if err != nil {
		t.Fatalf("read PvE vertical authoring example bundle: %v", err)
	}
	var bundle contentbundle.Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("decode PvE vertical authoring example bundle: %v", err)
	}
	return bundle
}

func TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	hero := peerVisibilityCharacter("PveVerticalHero", 0x01030160, 0x02040160, 469500, 964200, 0, 101, 201)
	hero.Gold = 40
	hero.Points[bootstrapExperiencePointType] = 40
	issuePeerTicket(t, ticketStore, "pve-vertical", 0x60606060, hero)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "pve-vertical", Empire: hero.Empire, Characters: []loginticket.Character{hero}}); err != nil {
		t.Fatalf("seed PvE vertical account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		ticketStore,
		accounts,
		staticstore.NewMemoryStore(),
		interactionstore.NewMemoryStore(),
		itemcatalog.NewMemoryStore(),
		queststate.NewMemoryStore(),
		nil,
	)
	if err != nil {
		t.Fatalf("new PvE vertical runtime: %v", err)
	}
	currentTime := time.Unix(1_700_001_000, 0)
	runtime.now = func() time.Time { return currentTime }

	authored := loadBootstrapPveVerticalAuthoringBundle(t)
	if len(authored.RegenSpawns) == 0 || len(authored.DropTables) == 0 {
		t.Fatalf("expected authored PvE vertical bundle to keep regen_spawns and drop_tables before import, got regen=%+v drop_tables=%+v", authored.RegenSpawns, authored.DropTables)
	}
	if len(authored.SpawnGroups) != 0 {
		t.Fatalf("expected authored PvE vertical bundle to expand from regen/drop tables rather than direct spawn_groups, got %+v", authored.SpawnGroups)
	}
	const pveVerticalMobMaxHP = uint8(20)
	const pveVerticalMobHitsToKill = 4 // max_hp 20 / formula damage 5
	const pveVerticalMobFormulaDamage = int32(5)
	const pveVerticalMobRespawnDelay = 2 * time.Second
	const pveVerticalKillRewardGold = uint64(60)       // loot.qa_pve_vertical_reward.reward_gold
	const pveVerticalPackRewardExperience = uint64(40) // loot.qa_pve_vertical_pack_reward.reward_experience
	const pveVerticalPackRewardGold = uint64(20)       // loot.qa_pve_vertical_pack_reward.reward_gold
	imported, err := runtime.ImportContentBundle(authored)
	if err != nil {
		t.Fatalf("import PvE vertical authoring bundle: %v", err)
	}
	if len(imported.RegenSpawns) != 0 || len(imported.DropTables) != 0 {
		t.Fatalf("expected import to canonicalize away authoring-only regen/drop collections, got regen=%+v drop_tables=%+v", imported.RegenSpawns, imported.DropTables)
	}
	if len(imported.SpawnGroups) != 3 ||
		imported.SpawnGroups[0].Ref != "practice.qa_pve_vertical_mob" ||
		imported.SpawnGroups[1].Ref != "practice.qa_pve_vertical_pack.m01" ||
		imported.SpawnGroups[2].Ref != "practice.qa_pve_vertical_pack.m02" {
		t.Fatalf("expected imported spawn groups practice.qa_pve_vertical_mob plus pack members, got %+v", imported.SpawnGroups)
	}
	if imported.SpawnGroups[0].CombatProfile != "qa_pve_vertical_practice_mob" ||
		imported.SpawnGroups[1].CombatProfile != "qa_pve_vertical_practice_mob" ||
		imported.SpawnGroups[2].CombatProfile != "qa_pve_vertical_practice_mob" {
		t.Fatalf("expected imported PvE vertical mobs to use formula combat profile, got %+v", imported.SpawnGroups)
	}
	if imported.SpawnGroups[1].RewardExperience != pveVerticalPackRewardExperience ||
		imported.SpawnGroups[1].RewardGold != pveVerticalPackRewardGold ||
		len(imported.SpawnGroups[1].RewardDropVnums) != 0 ||
		imported.SpawnGroups[1].RewardQuestRef != "" ||
		imported.SpawnGroups[1].RewardQuestFlag != "" ||
		imported.SpawnGroups[2].RewardExperience != pveVerticalPackRewardExperience ||
		imported.SpawnGroups[2].RewardGold != pveVerticalPackRewardGold ||
		len(imported.SpawnGroups[2].RewardDropVnums) != 0 ||
		imported.SpawnGroups[2].RewardQuestRef != "" ||
		imported.SpawnGroups[2].RewardQuestFlag != "" {
		t.Fatalf("expected imported pack members to carry EXP/gold-only loot.qa_pve_vertical_pack_reward, got %+v", imported.SpawnGroups[1:])
	}
	if len(imported.CombatProfiles) != 1 || imported.CombatProfiles[0].Profile != "qa_pve_vertical_practice_mob" || imported.CombatProfiles[0].MaxHP != pveVerticalMobMaxHP || imported.CombatProfiles[0].DamagePerNormalAttack != 5 || imported.CombatProfiles[0].AggroRadius != 150 || imported.CombatProfiles[0].LeashRadius != 350 || imported.CombatProfiles[0].ChaseDelayMs != 2000 || imported.CombatProfiles[0].ReturnDelayMs != 2000 || imported.CombatProfiles[0].HomewardDelayMs != 2000 || imported.CombatProfiles[0].MaxStep != 50 || imported.CombatProfiles[0].ReactionDelayMs != 2000 || imported.CombatProfiles[0].RetaliationPointDelta != -2 {
		t.Fatalf("expected imported portable formula combat profile max_hp=20 damage=5 aggro_radius=150 leash_radius=350 chase/return/homeward/reaction_delay_ms=2000 max_step=50 retaliation_point_delta=-2, got %+v", imported.CombatProfiles)
	}
	assertPveVerticalAuthoredUseAndEquipTemplates(t, imported.ItemTemplates, "imported PvE vertical authoring bundle")
	if !reflect.DeepEqual(imported.CubeRecipes, cubestore.BootstrapSnapshot().NPCs) {
		t.Fatalf("unexpected imported PvE vertical cube recipes: %#v", imported.CubeRecipes)
	}

	var guideVID, hunterVID, resetVID, merchantVID, warehouseVID, cubeVID, mobVID, talkVID, infoVID, teleporterVID, pack2VID uint32
	var foundPackMembers int
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "QuestGuide":
			guideVID = uint32(actor.EntityID)
		case "QuestHunter":
			hunterVID = uint32(actor.EntityID)
		case "QuestResetGuide":
			resetVID = uint32(actor.EntityID)
		case "Merchant":
			merchantVID = uint32(actor.EntityID)
		case "Warehouse":
			warehouseVID = uint32(actor.EntityID)
		case "CubeMaster":
			cubeVID = uint32(actor.EntityID)
		case "VillageGuide":
			talkVID = uint32(actor.EntityID)
		case "VillageSignpost":
			infoVID = uint32(actor.EntityID)
		case "Teleporter":
			teleporterVID = uint32(actor.EntityID)
		case "QAPveVerticalMob":
			mobVID = uint32(actor.EntityID)
		case "QAPveVerticalPack 1":
			foundPackMembers++
		case "QAPveVerticalPack 2":
			pack2VID = uint32(actor.EntityID)
			foundPackMembers++
		}
	}
	if guideVID == 0 || hunterVID == 0 || resetVID == 0 || merchantVID == 0 || warehouseVID == 0 || cubeVID == 0 || mobVID == 0 || talkVID == 0 || infoVID == 0 || teleporterVID == 0 || pack2VID == 0 {
		t.Fatalf("expected guide/hunter/reset/merchant/warehouse/cube/mob/talk/info/teleporter/pack-2 actors after import, got %+v", runtime.StaticActors())
	}
	if foundPackMembers != 2 {
		t.Fatalf("expected denser multi-count pack members QAPveVerticalPack 1/2 after import, found=%d actors=%+v", foundPackMembers, runtime.StaticActors())
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pve-vertical", 0x60606060)
	defer func() { closeSessionFlow(t, flow) }()

	mismatchOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected gated merchant mismatch interaction error: %v", err)
	}
	if len(mismatchOut) != 1 {
		t.Fatalf("expected 1 self-only merchant mismatch frame, got %d", len(mismatchOut))
	}
	mismatchChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, mismatchOut[0]))
	if err != nil || mismatchChat.Message != "Quest requirements are not met." {
		t.Fatalf("unexpected gated merchant mismatch chat: %+v err=%v", mismatchChat, err)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	warehouseMismatchOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: warehouseVID})))
	if err != nil {
		t.Fatalf("unexpected gated warehouse mismatch interaction error: %v", err)
	}
	if len(warehouseMismatchOut) != 1 {
		t.Fatalf("expected 1 self-only warehouse mismatch frame, got %d", len(warehouseMismatchOut))
	}
	warehouseMismatchChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, warehouseMismatchOut[0]))
	if err != nil || warehouseMismatchChat.Message != "Quest requirements are not met." {
		t.Fatalf("unexpected gated warehouse mismatch chat: %+v err=%v", warehouseMismatchChat, err)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	cubeMismatchOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: cubeVID})))
	if err != nil {
		t.Fatalf("unexpected gated cube mismatch interaction error: %v", err)
	}
	if len(cubeMismatchOut) != 1 {
		t.Fatalf("expected 1 self-only cube mismatch frame, got %d", len(cubeMismatchOut))
	}
	cubeMismatchChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, cubeMismatchOut[0]))
	if err != nil || cubeMismatchChat.Message != "Quest requirements are not met." {
		t.Fatalf("unexpected gated cube mismatch chat: %+v err=%v", cubeMismatchChat, err)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	assertPveVerticalGatedMismatch(t, flow, resetVID, "QuestResetGuide clear")
	currentTime = currentTime.Add(staticActorInteractionCooldown)
	assertPveVerticalGatedMismatch(t, flow, talkVID, "VillageGuide talk")
	currentTime = currentTime.Add(staticActorInteractionCooldown)
	assertPveVerticalGatedMismatch(t, flow, infoVID, "VillageSignpost info")
	currentTime = currentTime.Add(staticActorInteractionCooldown)
	assertPveVerticalGatedMismatch(t, flow, teleporterVID, "Teleporter warp")
	assertPveVerticalConnectedPosition(t, runtime, bootstrapMapIndex, hero.X, hero.Y, "failed gated warp")
	assertPveVerticalPersistedPosition(t, accounts, "pve-vertical", bootstrapMapIndex, hero.X, hero.Y, "failed gated warp")

	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: mobVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected pre-guide target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}
	var preGuideKillOut [][]byte
	for hit := 1; hit <= pveVerticalMobHitsToKill; hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		preGuideKillOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: mobVID})))
		if err != nil {
			t.Fatalf("unexpected pre-guide kill attack error on hit %d: %v", hit, err)
		}
		if hit == 1 {
			assertPveVerticalFormulaFirstHitFrames(t, preGuideKillOut, mobVID, hero.VID, pveVerticalMobFormulaDamage, "pre-guide")
		}
	}
	if len(preGuideKillOut) < 7 {
		t.Fatalf("expected pre-guide killing hit to include death/reward frames, got %d", len(preGuideKillOut))
	}
	for _, frame := range preGuideKillOut {
		if chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame)); err == nil {
			t.Fatalf("expected no quest chat before guide unlock, got %+v", chat)
		}
	}
	_ = decodePveVerticalAuthoredKillDrop(t, preGuideKillOut, hero, "pre-guide")
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after pre-guide kill: %v", err)
	}
	if !reflect.DeepEqual(loaded, queststate.Snapshot{Flags: []queststate.Flag{{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1}}}) {
		t.Fatalf("unexpected quest-state after pre-guide kill:\n got: %#v", loaded)
	}

	currentTime = currentTime.Add(pveVerticalMobRespawnDelay)
	_ = flushServerFrames(t, flow)

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	guideOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: guideVID})))
	if err != nil {
		t.Fatalf("unexpected QuestGuide interaction error: %v", err)
	}
	if len(guideOut) != 1 {
		t.Fatalf("expected 1 self-only QuestGuide frame, got %d", len(guideOut))
	}
	guideChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, guideOut[0]))
	if err != nil || guideChat.Message != "Quest updated: first_steps.met_guide = 1." {
		t.Fatalf("unexpected QuestGuide chat delivery: %+v err=%v", guideChat, err)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after QuestGuide: %v", err)
	}
	wantAfterGuide := queststate.Snapshot{Flags: []queststate.Flag{
		{Character: hero.Name, QuestRef: "quest:first_steps", Name: "met_guide", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}
	if !reflect.DeepEqual(loaded, wantAfterGuide) {
		t.Fatalf("unexpected quest-state after QuestGuide:\n got: %#v\nwant: %#v", loaded, wantAfterGuide)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	talkOut := interactPveVertical(t, flow, talkVID, "unlocked VillageGuide talk")
	assertPveVerticalSelfOnlyInfoChat(t, talkOut, "VillageGuide:\nWelcome to the QA square.", "unlocked VillageGuide talk")
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "unlocked VillageGuide talk")

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	infoOut := interactPveVertical(t, flow, infoVID, "unlocked VillageSignpost info")
	assertPveVerticalSelfOnlyInfoChat(t, infoOut, "The QA square contains a merchant, teleporter, guide, hunter, warehouse, cube craftsman, and reward mob.", "unlocked VillageSignpost info")
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "unlocked VillageSignpost info")

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	const pveVerticalWarpX int32 = 470200
	warpOut := interactPveVertical(t, flow, teleporterVID, "unlocked Teleporter warp")
	if len(warpOut) < 5 {
		t.Fatalf("expected chat + same-map warp rebootstrap frames after Teleporter unlock, got %d", len(warpOut))
	}
	warpChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, warpOut[0]))
	if err != nil || warpChat.Type != chatproto.ChatTypeInfo || warpChat.VID != 0 || warpChat.Empire != 0 || warpChat.Message != "Step through the gate." {
		t.Fatalf("unexpected unlocked Teleporter warp chat: %+v err=%v", warpChat, err)
	}
	warpAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, warpOut[1]))
	if err != nil {
		t.Fatalf("decode unlocked Teleporter warp self add: %v", err)
	}
	if warpAdd.VID != hero.VID || warpAdd.X != pveVerticalWarpX || warpAdd.Y != hero.Y {
		t.Fatalf("unexpected unlocked Teleporter warp self add: %+v want vid=%d x=%d y=%d", warpAdd, hero.VID, pveVerticalWarpX, hero.Y)
	}
	assertPveVerticalConnectedPosition(t, runtime, bootstrapMapIndex, pveVerticalWarpX, hero.Y, "unlocked Teleporter warp")
	assertPveVerticalPersistedPosition(t, accounts, "pve-vertical", bootstrapMapIndex, pveVerticalWarpX, hero.Y, "unlocked Teleporter warp")
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "unlocked Teleporter warp")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for self-only PvE warp, got %d", len(queued))
	}

	beforePackGold, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok {
		t.Fatal("expected currency snapshot before warp-tile pack kill")
	}
	beforePackPoints, ok := runtime.PointsSnapshot(hero.Name)
	if !ok {
		t.Fatal("expected points snapshot before warp-tile pack kill")
	}
	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: pack2VID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected warp-tile pack-2 target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}
	var packKillOut [][]byte
	for hit := 1; hit <= pveVerticalMobHitsToKill; hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		packKillOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: pack2VID})))
		if err != nil {
			t.Fatalf("unexpected warp-tile pack-2 kill attack error on hit %d: %v", hit, err)
		}
		if hit == 1 {
			assertPveVerticalFormulaFirstHitFrames(t, packKillOut, pack2VID, hero.VID, pveVerticalMobFormulaDamage, "warp-tile pack 2")
		}
	}
	wantGoldAfterPackKill := beforePackGold.Gold + pveVerticalPackRewardGold
	wantExperienceAfterPackKill := beforePackPoints.Points[bootstrapExperiencePointType] + int32(pveVerticalPackRewardExperience)
	assertPveVerticalEXPGoldOnlyKillReward(
		t,
		packKillOut,
		hero,
		pack2VID,
		pveVerticalMobFormulaDamage,
		int32(pveVerticalPackRewardExperience),
		wantExperienceAfterPackKill,
		int32(pveVerticalPackRewardGold),
		wantGoldAfterPackKill,
		"warp-tile pack 2",
	)
	assertPveVerticalCurrency(t, runtime, accounts, hero.Name, wantGoldAfterPackKill, "warp-tile pack 2 kill")
	pointsSnapshot, ok := runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapExperiencePointType] != wantExperienceAfterPackKill {
		t.Fatalf("expected live experience %d after warp-tile pack 2 kill, got ok=%v snapshot=%+v", wantExperienceAfterPackKill, ok, pointsSnapshot)
	}
	account, err := accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after warp-tile pack 2 kill: %v", err)
	}
	if account.Characters[0].Points[bootstrapExperiencePointType] != wantExperienceAfterPackKill {
		t.Fatalf("expected persisted experience %d after warp-tile pack 2 kill, got %d", wantExperienceAfterPackKill, account.Characters[0].Points[bootstrapExperiencePointType])
	}
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "warp-tile pack 2 kill")
	killedPack, ok := runtime.SpawnGroupByRef("practice.qa_pve_vertical_pack.m02")
	if !ok || !killedPack.Dead || uint32(killedPack.EntityID) != pack2VID {
		t.Fatalf("expected warp-tile pack 2 to stay dead after the accepted kill, ok=%v snapshot=%+v", ok, killedPack)
	}
	livingPack, ok := runtime.SpawnGroupByRef("practice.qa_pve_vertical_pack.m01")
	if !ok || livingPack.Dead {
		t.Fatalf("expected untargeted pack 1 to stay alive after pack 2 kill, ok=%v snapshot=%+v", ok, livingPack)
	}
	currentTime = currentTime.Add(pveVerticalMobRespawnDelay)
	packRespawnOut := flushServerFrames(t, flow)
	assertPveVerticalPackRespawn(t, packRespawnOut, pack2VID)
	respawnedPack, ok := runtime.SpawnGroupByRef("practice.qa_pve_vertical_pack.m02")
	if !ok || respawnedPack.Dead || respawnedPack.X != 470000 || respawnedPack.Y != hero.Y || respawnedPack.CombatMaxHP != pveVerticalMobMaxHP || respawnedPack.CombatHPPercent != 100 {
		t.Fatalf("expected warp-tile pack 2 respawn to restore authored live full-HP state, ok=%v snapshot=%+v", ok, respawnedPack)
	}
	livingPack, ok = runtime.SpawnGroupByRef("practice.qa_pve_vertical_pack.m01")
	if !ok || livingPack.Dead {
		t.Fatalf("expected untargeted pack 1 to stay alive after pack 2 respawn, ok=%v snapshot=%+v", ok, livingPack)
	}
	attackWithoutReselectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  pack2VID,
	})))
	if err != nil {
		t.Fatalf("unexpected warp-tile pack-2 attack without fresh target selection after respawn: %v", err)
	}
	if len(attackWithoutReselectOut) != 0 {
		t.Fatalf("expected warp-tile pack-2 attack without fresh target selection after respawn to fail closed, got %d frames", len(attackWithoutReselectOut))
	}
	reselectPackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: pack2VID})))
	if err != nil {
		t.Fatalf("unexpected warp-tile pack-2 fresh target selection after respawn: %v", err)
	}
	if len(reselectPackOut) != 1 {
		t.Fatalf("expected one warp-tile pack-2 target frame after respawn, got %d", len(reselectPackOut))
	}
	reselectedPack, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, reselectPackOut[0]))
	if err != nil {
		t.Fatalf("decode warp-tile pack-2 target frame after respawn: %v", err)
	}
	if reselectedPack.TargetVID != pack2VID || reselectedPack.HPPercent != 100 {
		t.Fatalf("unexpected warp-tile pack-2 fresh target packet after respawn: %+v", reselectedPack)
	}
	clearPackTargetOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{})))
	if err != nil {
		t.Fatalf("unexpected warp-tile pack-2 target clear after respawn: %v", err)
	}
	if len(clearPackTargetOut) != 0 {
		t.Fatalf("expected warp-tile pack-2 target clear after respawn to be consumed without frames, got %d", len(clearPackTargetOut))
	}
	_ = flushServerFrames(t, flow)

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    hero.X,
		Y:    hero.Y,
		Time: 0x71727374,
	})))
	if err != nil {
		t.Fatalf("unexpected return MOVE after PvE warp: %v", err)
	}
	if len(moveOut) == 0 {
		t.Fatalf("expected return MOVE after PvE warp to emit a self ack")
	}
	assertPveVerticalConnectedPosition(t, runtime, bootstrapMapIndex, hero.X, hero.Y, "return MOVE after PvE warp")
	assertPveVerticalPersistedPosition(t, accounts, "pve-vertical", bootstrapMapIndex, hero.X, hero.Y, "return MOVE after PvE warp")

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	shopOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected unlocked merchant interaction error: %v", err)
	}
	if len(shopOut) != 1 {
		t.Fatalf("expected 1 merchant shop-open frame after guide unlock, got %d", len(shopOut))
	}
	firstShopStart, err := shopproto.DecodeServerStart(decodeSingleFrame(t, shopOut[0]))
	if err != nil {
		t.Fatalf("decode unlocked merchant shop start: %v", err)
	}
	if firstShopStart.Items[0].Vnum != 27001 || firstShopStart.Items[0].Price != 50 || firstShopStart.Items[0].Count != 1 || firstShopStart.Items[0].DisplayPos != 0 {
		t.Fatalf("unexpected merchant catalog slot 0: %+v", firstShopStart.Items[0])
	}
	if firstShopStart.Items[1].Vnum != 11200 || firstShopStart.Items[1].Price != 500 || firstShopStart.Items[1].Count != 1 || firstShopStart.Items[1].DisplayPos != 1 {
		t.Fatalf("unexpected merchant catalog slot 1: %+v", firstShopStart.Items[1])
	}
	if firstShopStart.Items[2].Vnum != 27002 || firstShopStart.Items[2].Price != 20 || firstShopStart.Items[2].Count != 2 || firstShopStart.Items[2].DisplayPos != 2 {
		t.Fatalf("unexpected merchant catalog slot 2: %+v", firstShopStart.Items[2])
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	warehouseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: warehouseVID})))
	if err != nil {
		t.Fatalf("unexpected unlocked warehouse interaction error: %v", err)
	}
	if len(warehouseOut) != 3 {
		t.Fatalf("expected merchant close plus chat + ShowMeSafeboxPassword frames after guide unlock, got %d", len(warehouseOut))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, warehouseOut[0])); err != nil {
		t.Fatalf("decode merchant close before unlocked warehouse open: %v", err)
	}
	warehouseChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, warehouseOut[1]))
	if err != nil || warehouseChat.Message != "The warehouse keeper unlocks the vault." {
		t.Fatalf("unexpected unlocked warehouse chat: %+v err=%v", warehouseChat, err)
	}
	warehousePrompt, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, warehouseOut[2]))
	if err != nil || warehousePrompt.Type != chatproto.ChatTypeCommand || warehousePrompt.Message != safeboxShowPasswordCommandMessage {
		t.Fatalf("unexpected unlocked warehouse password prompt: %+v err=%v", warehousePrompt, err)
	}
	warehouseOpenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password 000000",
	})))
	if err != nil {
		t.Fatalf("unexpected unlocked warehouse password open: %v", err)
	}
	if len(warehouseOpenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE after unlocked warehouse password, got %d", len(warehouseOpenOut))
	}
	warehouseSize, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, warehouseOpenOut[0]))
	if err != nil {
		t.Fatalf("decode unlocked warehouse SAFEBOX_SIZE: %v", err)
	}
	if warehouseSize != (itemproto.SafeboxSizePacket{Size: 2}) {
		t.Fatalf("unexpected unlocked warehouse SAFEBOX_SIZE: %+v", warehouseSize)
	}
	assertPveVerticalSafeboxMoneyChange(t, warehouseOpenOut[1], 0, "unlocked warehouse password open")
	beforeWarehouseGold, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok || beforeWarehouseGold.Gold < pveVerticalKillRewardGold {
		t.Fatalf("expected live gold at least authored kill-reward %d before warehouse money save, got ok=%v snapshot=%+v", pveVerticalKillRewardGold, ok, beforeWarehouseGold)
	}
	wantGoldAfterWarehouseSave := beforeWarehouseGold.Gold - pveVerticalKillRewardGold
	slashPveVerticalSafeboxMoney(
		t,
		flow,
		fmt.Sprintf("/safebox_money_save %d", pveVerticalKillRewardGold),
		hero.VID,
		-int32(pveVerticalKillRewardGold),
		wantGoldAfterWarehouseSave,
		int32(pveVerticalKillRewardGold),
		"authored warehouse money save",
	)
	assertPveVerticalCurrency(t, runtime, accounts, hero.Name, wantGoldAfterWarehouseSave, "authored warehouse money save")
	assertPveVerticalDurableWarehouseGold(t, runtime, "pve-vertical", hero.ID, int64(pveVerticalKillRewardGold), "authored warehouse money save")
	_ = flushServerFrames(t, flow)
	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "pve vertical warehouse close before cube")
	assertPveVerticalDurableWarehouseGold(t, runtime, "pve-vertical", hero.ID, int64(pveVerticalKillRewardGold), "authored warehouse close after money save")

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	warehouseCooldownOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: warehouseVID})))
	if err != nil {
		t.Fatalf("unexpected warehouse interaction during reopen cooldown: %v", err)
	}
	if len(warehouseCooldownOut) != 2 {
		t.Fatalf("expected chat + ShowMeSafeboxPassword during warehouse reopen cooldown, got %d", len(warehouseCooldownOut))
	}
	warehouseCooldownChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, warehouseCooldownOut[0]))
	if err != nil || warehouseCooldownChat.Message != "The warehouse keeper unlocks the vault." {
		t.Fatalf("unexpected warehouse cooldown chat: %+v err=%v", warehouseCooldownChat, err)
	}
	warehouseCooldownPrompt, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, warehouseCooldownOut[1]))
	if err != nil || warehouseCooldownPrompt.Type != chatproto.ChatTypeCommand || warehouseCooldownPrompt.Message != safeboxShowPasswordCommandMessage {
		t.Fatalf("unexpected warehouse cooldown password prompt: %+v err=%v", warehouseCooldownPrompt, err)
	}
	warehouseCooldownPasswordOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password 000000",
	})))
	if err != nil {
		t.Fatalf("unexpected warehouse password during reopen cooldown: %v", err)
	}
	if len(warehouseCooldownPasswordOut) != 1 {
		t.Fatalf("expected warehouse reopen-cooldown info chat, got %d", len(warehouseCooldownPasswordOut))
	}
	warehouseCooldownInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, warehouseCooldownPasswordOut[0]))
	if err != nil || warehouseCooldownInfo.Type != chatproto.ChatTypeInfo || warehouseCooldownInfo.VID != 0 || warehouseCooldownInfo.Message != safeboxReopenCooldownInfoMessage {
		t.Fatalf("unexpected warehouse reopen-cooldown chat: %+v err=%v", warehouseCooldownInfo, err)
	}

	cubeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: cubeVID})))
	if err != nil {
		t.Fatalf("unexpected unlocked cube interaction error: %v", err)
	}
	if len(cubeOut) != 2 {
		t.Fatalf("expected chat + cube open frames after guide unlock, got %d", len(cubeOut))
	}
	cubeChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, cubeOut[0]))
	if err != nil || cubeChat.Message != "The craftsman lights the forge." {
		t.Fatalf("unexpected unlocked cube chat: %+v err=%v", cubeChat, err)
	}
	assertCubeCommandChatFrame(t, cubeOut[1], "cube open 20022", "pve vertical unlocked cube open")
	cubeGold, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok {
		t.Fatal("expected gold snapshot before authored cube r_info")
	}
	cubeInventory, ok := runtime.InventorySnapshot(hero.Name)
	if !ok || len(cubeInventory.Inventory) != 0 {
		t.Fatalf("expected empty inventory before authored cube r_info, got ok=%v snapshot=%+v", ok, cubeInventory)
	}
	cubeAccount, err := accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account before authored cube r_info: %v", err)
	}
	if cubeAccount.Characters[0].Gold != cubeGold.Gold {
		t.Fatalf("expected persisted gold %d to match live gold before authored cube r_info, got %d", cubeGold.Gold, cubeAccount.Characters[0].Gold)
	}
	if len(cubeAccount.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted inventory empty before authored cube r_info, got %+v", cubeAccount.Characters[0].Inventory)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "unlocked CubeMaster open")

	cubeRInfoOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info",
	})))
	if err != nil {
		t.Fatalf("unexpected authored cube r_info: %v", err)
	}
	if len(cubeRInfoOut) != 1 {
		t.Fatalf("expected 1 cube r_list frame after authored CubeMaster open, got %d", len(cubeRInfoOut))
	}
	assertCubeCommandChatFrame(t, cubeRInfoOut[0], "cube r_list 20022 1 27001,1", "pve vertical authored cube r_list")

	cubeMInfoOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info 0",
	})))
	if err != nil {
		t.Fatalf("unexpected authored cube r_info 0: %v", err)
	}
	if len(cubeMInfoOut) != 1 {
		t.Fatalf("expected 1 cube m_info frame after authored CubeMaster open, got %d", len(cubeMInfoOut))
	}
	assertCubeCommandChatFrame(t, cubeMInfoOut[0], "cube m_info 0 1 27002,2/100", "pve vertical authored cube m_info")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected authored cube r_info/m_info to queue no peer frames, got %d", len(queued))
	}
	afterCubeGold, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok || afterCubeGold.Gold != cubeGold.Gold {
		t.Fatalf("expected gold %d after authored cube r_info/m_info, got ok=%v snapshot=%+v", cubeGold.Gold, ok, afterCubeGold)
	}
	cubeInventory, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(cubeInventory.Inventory) != 0 {
		t.Fatalf("expected empty inventory after authored cube r_info/m_info, got ok=%v snapshot=%+v", ok, cubeInventory)
	}
	cubeAccount, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after authored cube r_info/m_info: %v", err)
	}
	if cubeAccount.Characters[0].Gold != cubeGold.Gold {
		t.Fatalf("expected persisted gold %d after authored cube r_info/m_info, got %d", cubeGold.Gold, cubeAccount.Characters[0].Gold)
	}
	if len(cubeAccount.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted inventory empty after authored cube r_info/m_info, got %+v", cubeAccount.Characters[0].Inventory)
	}
	assertPveVerticalDurableWarehouseGold(t, runtime, "pve-vertical", hero.ID, int64(pveVerticalKillRewardGold), "authored cube r_info/m_info")
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "authored cube r_info/m_info")

	assertCloseCubeCommandChat(t, flow, "/close_cube", "pve vertical cube close before reconnect")
	closedCubeRInfoOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info",
	})))
	if err != nil {
		t.Fatalf("unexpected closed-cube r_info after authored CubeMaster close: %v", err)
	}
	if len(closedCubeRInfoOut) != 0 {
		t.Fatalf("expected closed-cube r_info to emit no frames, got %d", len(closedCubeRInfoOut))
	}

	closeSessionFlow(t, flow)
	flow, _ = enterGameWithLoginTicket(t, runtime.SessionFactory(), "pve-vertical", 0x60606060)

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	resumeShopOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected reconnect merchant interaction error: %v", err)
	}
	if len(resumeShopOut) != 1 {
		t.Fatalf("expected 1 merchant shop-open frame after reconnect, got %d", len(resumeShopOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, resumeShopOut[0])); err != nil {
		t.Fatalf("decode reconnect merchant shop start: %v", err)
	}
	closeShopOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected reconnect merchant close error: %v", err)
	}
	if len(closeShopOut) != 1 {
		t.Fatalf("expected 1 merchant close frame after reconnect shop open, got %d", len(closeShopOut))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, closeShopOut[0])); err != nil {
		t.Fatalf("decode reconnect merchant shop end: %v", err)
	}

	if out, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: mobVID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected post-guide target selection to return 1 frame, got frames=%d err=%v", len(out), err)
	}
	var postGuideKillOut [][]byte
	for hit := 1; hit <= pveVerticalMobHitsToKill; hit++ {
		if hit > 1 {
			currentTime = currentTime.Add(bootstrapNormalAttackCadenceWindow)
		}
		postGuideKillOut, err = flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: mobVID})))
		if err != nil {
			t.Fatalf("unexpected post-guide kill attack error on hit %d: %v", hit, err)
		}
		if hit == 1 {
			assertPveVerticalFormulaFirstHitFrames(t, postGuideKillOut, mobVID, hero.VID, pveVerticalMobFormulaDamage, "post-guide")
		}
	}
	if len(postGuideKillOut) < 8 {
		t.Fatalf("expected post-guide killing hit to include death/reward frames plus quest chat, got %d", len(postGuideKillOut))
	}
	killChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, postGuideKillOut[len(postGuideKillOut)-1]))
	if err != nil || killChat.Message != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("unexpected post-guide kill quest chat: %+v err=%v", killChat, err)
	}
	postGuideGround := decodePveVerticalAuthoredKillDrop(t, postGuideKillOut, hero, "post-guide")
	pickupPveVerticalAuthoredDrop(t, flow, postGuideGround.VID, "post-guide")
	if runtime.sharedWorld.GroundItemExists(postGuideGround.VID) {
		t.Fatalf("expected credited kill-drop pickup to clear ground handle %d", postGuideGround.VID)
	}
	beforeTurnInInventory, ok := runtime.InventorySnapshot(hero.Name)
	if !ok || len(beforeTurnInInventory.Inventory) != 1 || beforeTurnInInventory.Inventory[0].Vnum != 27001 || beforeTurnInInventory.Inventory[0].Count != 1 || beforeTurnInInventory.Inventory[0].Slot != 0 {
		t.Fatalf("expected live inventory to hold the credited kill drop before QuestHunter turn-in, got ok=%v snapshot=%+v", ok, beforeTurnInInventory)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after credited kill-drop pickup: %v", err)
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 27001 || account.Characters[0].Inventory[0].Count != 1 || account.Characters[0].Inventory[0].Slot != 0 {
		t.Fatalf("expected persisted inventory to hold the credited kill drop before QuestHunter turn-in, got %+v", account.Characters[0].Inventory)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after gated kill credit: %v", err)
	}
	wantAfterKill := queststate.Snapshot{Flags: []queststate.Flag{
		{Character: hero.Name, QuestRef: "quest:first_steps", Name: "killed_qa_mob", Value: 1},
		{Character: hero.Name, QuestRef: "quest:first_steps", Name: "met_guide", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}
	if !reflect.DeepEqual(loaded, wantAfterKill) {
		t.Fatalf("unexpected quest-state after gated kill credit:\n got: %#v\nwant: %#v", loaded, wantAfterKill)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	beforeTurnInCurrency, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok {
		t.Fatal("expected currency snapshot before QuestHunter turn-in")
	}
	beforeTurnInPoints, ok := runtime.PointsSnapshot(hero.Name)
	if !ok {
		t.Fatal("expected points snapshot before QuestHunter turn-in")
	}
	turnInOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: hunterVID})))
	if err != nil {
		t.Fatalf("unexpected QuestHunter turn-in interaction error: %v", err)
	}
	if len(turnInOut) != 7 {
		t.Fatalf("expected chat + consume-gold + consume-experience + reward-gold + reward-experience + consume + reward frames for QuestHunter turn-in, got %d", len(turnInOut))
	}
	turnInChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, turnInOut[0]))
	if err != nil || turnInChat.Message != "Quest updated: first_steps.killed_qa_mob = 0." {
		t.Fatalf("unexpected QuestHunter turn-in chat: %+v err=%v", turnInChat, err)
	}
	wantGoldAfterConsume := beforeTurnInCurrency.Gold - 25
	wantGoldAfter := wantGoldAfterConsume + 100
	turnInConsumeGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, turnInOut[1]))
	if err != nil {
		t.Fatalf("decode QuestHunter turn-in consume-gold point change: %v", err)
	}
	if turnInConsumeGold.VID != hero.VID || turnInConsumeGold.Type != bootstrapGoldPointType || turnInConsumeGold.Amount != -25 || uint64(turnInConsumeGold.Value) != wantGoldAfterConsume {
		t.Fatalf("unexpected QuestHunter turn-in consume-gold point change: %+v want value=%d before=%d", turnInConsumeGold, wantGoldAfterConsume, beforeTurnInCurrency.Gold)
	}
	wantExperienceAfterConsume := beforeTurnInPoints.Points[bootstrapExperiencePointType] - 10
	turnInConsumeExperience, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, turnInOut[2]))
	if err != nil {
		t.Fatalf("decode QuestHunter turn-in consume-experience point change: %v", err)
	}
	if turnInConsumeExperience.VID != hero.VID || turnInConsumeExperience.Type != bootstrapExperiencePointType || turnInConsumeExperience.Amount != -10 || turnInConsumeExperience.Value != wantExperienceAfterConsume {
		t.Fatalf("unexpected QuestHunter turn-in consume-experience point change: %+v want value=%d before=%d", turnInConsumeExperience, wantExperienceAfterConsume, beforeTurnInPoints.Points[bootstrapExperiencePointType])
	}
	turnInGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, turnInOut[3]))
	if err != nil {
		t.Fatalf("decode QuestHunter turn-in gold point change: %v", err)
	}
	if turnInGold.VID != hero.VID || turnInGold.Type != bootstrapGoldPointType || turnInGold.Amount != 100 || uint64(turnInGold.Value) != wantGoldAfter {
		t.Fatalf("unexpected QuestHunter turn-in gold point change: %+v want value=%d before=%d", turnInGold, wantGoldAfter, beforeTurnInCurrency.Gold)
	}
	turnInExperience, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, turnInOut[4]))
	if err != nil {
		t.Fatalf("decode QuestHunter turn-in experience point change: %v", err)
	}
	wantExperienceAfter := wantExperienceAfterConsume + 50
	if turnInExperience.VID != hero.VID || turnInExperience.Type != bootstrapExperiencePointType || turnInExperience.Amount != 50 || turnInExperience.Value != wantExperienceAfter {
		t.Fatalf("unexpected QuestHunter turn-in experience point change: %+v want value=%d before=%d", turnInExperience, wantExperienceAfter, beforeTurnInPoints.Points[bootstrapExperiencePointType])
	}
	consumeDel, err := itemproto.DecodeDel(decodeSingleFrame(t, turnInOut[5]))
	if err != nil {
		t.Fatalf("decode QuestHunter turn-in consume delete: %v", err)
	}
	if consumeDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected QuestHunter turn-in consume delete: %+v", consumeDel)
	}
	itemSet0, err := itemproto.DecodeSet(decodeSingleFrame(t, turnInOut[6]))
	if err != nil {
		t.Fatalf("decode QuestHunter turn-in reward set: %v", err)
	}
	if itemSet0.Position != itemproto.InventoryPosition(0) || itemSet0.Vnum != 11200 || itemSet0.Count != 1 {
		t.Fatalf("unexpected QuestHunter turn-in reward set: %+v", itemSet0)
	}
	currencySnapshot, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfter {
		t.Fatalf("expected live gold %d after QuestHunter turn-in, got ok=%v snapshot=%+v", wantGoldAfter, ok, currencySnapshot)
	}
	pointsSnapshot, ok = runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapExperiencePointType] != wantExperienceAfter {
		t.Fatalf("expected live experience %d after QuestHunter turn-in, got ok=%v snapshot=%+v", wantExperienceAfter, ok, pointsSnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 {
		t.Fatalf("expected live inventory after QuestHunter consume+reward turn-in, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after turn-in: %v", err)
	}
	if account.Characters[0].Gold != wantGoldAfter {
		t.Fatalf("expected persisted gold %d after QuestHunter turn-in, got %d", wantGoldAfter, account.Characters[0].Gold)
	}
	if account.Characters[0].Points[bootstrapExperiencePointType] != wantExperienceAfter {
		t.Fatalf("expected persisted experience %d after QuestHunter turn-in, got %d", wantExperienceAfter, account.Characters[0].Points[bootstrapExperiencePointType])
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 11200 || account.Characters[0].Inventory[0].Count != 1 {
		t.Fatalf("expected persisted inventory after QuestHunter consume+reward turn-in, got %+v", account.Characters[0].Inventory)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after QuestHunter turn-in: %v", err)
	}
	wantAfterTurnIn := queststate.Snapshot{Flags: []queststate.Flag{
		{Character: hero.Name, QuestRef: "quest:first_steps", Name: "met_guide", Value: 1},
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}
	if !reflect.DeepEqual(loaded, wantAfterTurnIn) {
		t.Fatalf("unexpected quest-state after QuestHunter turn-in:\n got: %#v\nwant: %#v", loaded, wantAfterTurnIn)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	secondTurnInOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: hunterVID})))
	if err != nil {
		t.Fatalf("unexpected second QuestHunter interaction error: %v", err)
	}
	if len(secondTurnInOut) != 1 {
		t.Fatalf("expected 1 self-only second QuestHunter mismatch frame, got %d", len(secondTurnInOut))
	}
	secondTurnInChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, secondTurnInOut[0]))
	if err != nil || secondTurnInChat.Message != questFlagInsufficientMaterialsInfoMessage {
		t.Fatalf("unexpected second QuestHunter mismatch chat: %+v err=%v", secondTurnInChat, err)
	}
	loaded, err = runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after second QuestHunter mismatch: %v", err)
	}
	if !reflect.DeepEqual(loaded, wantAfterTurnIn) {
		t.Fatalf("second QuestHunter mismatch mutated quest-state:\n got: %#v\nwant: %#v", loaded, wantAfterTurnIn)
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfter {
		t.Fatalf("expected mismatch path to leave gold at %d, got ok=%v snapshot=%+v", wantGoldAfter, ok, currencySnapshot)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	resumeShopOut, err = flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected post-turn-in merchant interaction error: %v", err)
	}
	if len(resumeShopOut) != 1 {
		t.Fatalf("expected 1 merchant shop-open frame after QuestHunter turn-in, got %d", len(resumeShopOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, resumeShopOut[0])); err != nil {
		t.Fatalf("decode post-turn-in merchant shop start: %v", err)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterTurnIn, "post-turn-in merchant reopen")
	_ = flushServerFrames(t, flow)

	expensiveBuyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 1})))
	if err != nil {
		t.Fatalf("unexpected post-turn-in expensive merchant buy error: %v", err)
	}
	if len(expensiveBuyOut) != 1 {
		t.Fatalf("expected 1 merchant not-enough-money frame for catalog slot 1, got %d", len(expensiveBuyOut))
	}
	if err := shopproto.DecodeServerNotEnoughMoney(decodeSingleFrame(t, expensiveBuyOut[0])); err != nil {
		t.Fatalf("decode post-turn-in expensive merchant buy error frame: %v", err)
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfter {
		t.Fatalf("expected expensive merchant buy to leave gold at %d, got ok=%v snapshot=%+v", wantGoldAfter, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 {
		t.Fatalf("expected expensive merchant buy to leave turn-in inventory unchanged, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterTurnIn, "expensive merchant buy")

	const pveVerticalMerchantPotionPrice = uint64(50)
	const pveVerticalMerchantPotionVnum = uint32(27001)
	wantGoldAfterBuy := wantGoldAfter - pveVerticalMerchantPotionPrice
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-turn-in affordable merchant buy error: %v", err)
	}
	if len(buyOut) != 1 {
		t.Fatalf("expected 1 item refresh frame for catalog slot 0 merchant buy, got %d", len(buyOut))
	}
	boughtSet, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode post-turn-in affordable merchant buy item set: %v", err)
	}
	if boughtSet.Position != itemproto.InventoryPosition(1) || boughtSet.Vnum != pveVerticalMerchantPotionVnum || boughtSet.Count != 1 {
		t.Fatalf("unexpected post-turn-in affordable merchant buy item set: %+v", boughtSet)
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterBuy {
		t.Fatalf("expected live gold %d after affordable merchant buy, got ok=%v snapshot=%+v", wantGoldAfterBuy, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 2 ||
		inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 ||
		inventorySnapshot.Inventory[1].Vnum != pveVerticalMerchantPotionVnum || inventorySnapshot.Inventory[1].Count != 1 || inventorySnapshot.Inventory[1].Slot != 1 {
		t.Fatalf("expected live inventory after affordable merchant buy to keep turn-in sword and add potion in slot 1, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after affordable merchant buy: %v", err)
	}
	if account.Characters[0].Gold != wantGoldAfterBuy {
		t.Fatalf("expected persisted gold %d after affordable merchant buy, got %d", wantGoldAfterBuy, account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 2 ||
		account.Characters[0].Inventory[0].Vnum != 11200 || account.Characters[0].Inventory[0].Count != 1 || account.Characters[0].Inventory[0].Slot != 0 ||
		account.Characters[0].Inventory[1].Vnum != pveVerticalMerchantPotionVnum || account.Characters[0].Inventory[1].Count != 1 || account.Characters[0].Inventory[1].Slot != 1 {
		t.Fatalf("expected persisted inventory after affordable merchant buy, got %+v", account.Characters[0].Inventory)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterTurnIn, "affordable merchant buy")

	const pveVerticalMerchantPotionSellCredit = uint64(2)
	wantGoldAfterPotionSell := wantGoldAfterBuy + pveVerticalMerchantPotionSellCredit
	potionSellOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell(shopproto.ClientSellPacket{Slot: 1})))
	if err != nil {
		t.Fatalf("unexpected post-buy merchant sell error: %v", err)
	}
	if len(potionSellOut) != 2 {
		t.Fatalf("expected 2 frames for whole-stack merchant sell of the bought potion, got %d", len(potionSellOut))
	}
	potionSoldDel, err := itemproto.DecodeDel(decodeSingleFrame(t, potionSellOut[0]))
	if err != nil {
		t.Fatalf("decode post-buy merchant sell item delete: %v", err)
	}
	if potionSoldDel.Position != itemproto.InventoryPosition(1) {
		t.Fatalf("unexpected post-buy merchant sell item delete: %+v", potionSoldDel)
	}
	potionSellGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, potionSellOut[1]))
	if err != nil {
		t.Fatalf("decode post-buy merchant sell gold point-change: %v", err)
	}
	if potionSellGold.VID != hero.VID || potionSellGold.Type != bootstrapGoldPointType || potionSellGold.Amount != int32(pveVerticalMerchantPotionSellCredit) || uint64(potionSellGold.Value) != wantGoldAfterPotionSell {
		t.Fatalf("unexpected post-buy merchant sell gold point-change: %+v want amount=%d value=%d", potionSellGold, pveVerticalMerchantPotionSellCredit, wantGoldAfterPotionSell)
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterPotionSell {
		t.Fatalf("expected live gold %d after merchant sell-back, got ok=%v snapshot=%+v", wantGoldAfterPotionSell, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 {
		t.Fatalf("expected live inventory after merchant sell-back to keep only the turn-in sword, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after merchant sell-back: %v", err)
	}
	if account.Characters[0].Gold != wantGoldAfterPotionSell {
		t.Fatalf("expected persisted gold %d after merchant sell-back, got %d", wantGoldAfterPotionSell, account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 11200 || account.Characters[0].Inventory[0].Count != 1 || account.Characters[0].Inventory[0].Slot != 0 {
		t.Fatalf("expected persisted inventory after merchant sell-back to keep only the turn-in sword, got %+v", account.Characters[0].Inventory)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterTurnIn, "merchant sell-back")

	wantGoldAfterRebuy := wantGoldAfterPotionSell - pveVerticalMerchantPotionPrice
	rebuyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-sell merchant rebuy error: %v", err)
	}
	if len(rebuyOut) != 1 {
		t.Fatalf("expected 1 item refresh frame for catalog slot 0 merchant rebuy, got %d", len(rebuyOut))
	}
	reboughtSet, err := itemproto.DecodeSet(decodeSingleFrame(t, rebuyOut[0]))
	if err != nil {
		t.Fatalf("decode post-sell merchant rebuy item set: %v", err)
	}
	if reboughtSet.Position != itemproto.InventoryPosition(1) || reboughtSet.Vnum != pveVerticalMerchantPotionVnum || reboughtSet.Count != 1 {
		t.Fatalf("unexpected post-sell merchant rebuy item set: %+v", reboughtSet)
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterRebuy {
		t.Fatalf("expected live gold %d after merchant rebuy, got ok=%v snapshot=%+v", wantGoldAfterRebuy, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 2 ||
		inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 ||
		inventorySnapshot.Inventory[1].Vnum != pveVerticalMerchantPotionVnum || inventorySnapshot.Inventory[1].Count != 1 || inventorySnapshot.Inventory[1].Slot != 1 {
		t.Fatalf("expected live inventory after merchant rebuy to keep turn-in sword and add potion in slot 1, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after merchant rebuy: %v", err)
	}
	if account.Characters[0].Gold != wantGoldAfterRebuy {
		t.Fatalf("expected persisted gold %d after merchant rebuy, got %d", wantGoldAfterRebuy, account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 2 ||
		account.Characters[0].Inventory[0].Vnum != 11200 || account.Characters[0].Inventory[0].Count != 1 || account.Characters[0].Inventory[0].Slot != 0 ||
		account.Characters[0].Inventory[1].Vnum != pveVerticalMerchantPotionVnum || account.Characters[0].Inventory[1].Count != 1 || account.Characters[0].Inventory[1].Slot != 1 {
		t.Fatalf("expected persisted inventory after merchant rebuy, got %+v", account.Characters[0].Inventory)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterTurnIn, "merchant rebuy")

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	resetOut := interactPveVertical(t, flow, resetVID, "QuestResetGuide clear")
	if len(resetOut) != 2 {
		t.Fatalf("expected merchant close plus QuestResetGuide clear chat, got %d", len(resetOut))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, resetOut[0])); err != nil {
		t.Fatalf("decode merchant close before QuestResetGuide clear: %v", err)
	}
	assertPveVerticalSelfOnlyInfoChat(t, resetOut[1:], "Quest cleared: first_steps.met_guide = 0.", "QuestResetGuide clear")
	wantAfterReset := queststate.Snapshot{Flags: []queststate.Flag{
		{Character: "QuestHero", QuestRef: "quest:first_steps", Name: "step", Value: 1},
	}}
	assertPveVerticalQuestState(t, runtime, wantAfterReset, "QuestResetGuide clear")
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterRebuy {
		t.Fatalf("expected QuestResetGuide to leave gold at %d, got ok=%v snapshot=%+v", wantGoldAfterRebuy, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 2 ||
		inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 ||
		inventorySnapshot.Inventory[1].Vnum != pveVerticalMerchantPotionVnum || inventorySnapshot.Inventory[1].Count != 1 || inventorySnapshot.Inventory[1].Slot != 1 {
		t.Fatalf("expected QuestResetGuide to leave post-rebuy inventory unchanged, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after QuestResetGuide: %v", err)
	}
	if account.Characters[0].Gold != wantGoldAfterRebuy {
		t.Fatalf("expected persisted gold %d after QuestResetGuide, got %d", wantGoldAfterRebuy, account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 2 ||
		account.Characters[0].Inventory[0].Vnum != 11200 || account.Characters[0].Inventory[0].Count != 1 ||
		account.Characters[0].Inventory[1].Vnum != pveVerticalMerchantPotionVnum || account.Characters[0].Inventory[1].Count != 1 {
		t.Fatalf("expected persisted inventory after QuestResetGuide to stay at post-rebuy snapshot, got %+v", account.Characters[0].Inventory)
	}

	staleBuyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected packet shop buy after QuestResetGuide: %v", err)
	}
	if len(staleBuyOut) != 0 {
		t.Fatalf("expected packet shop buy to fail closed after QuestResetGuide closed the merchant, got %d frames", len(staleBuyOut))
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterRebuy {
		t.Fatalf("expected stale merchant buy to leave gold at %d, got ok=%v snapshot=%+v", wantGoldAfterRebuy, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 2 || inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[1].Vnum != pveVerticalMerchantPotionVnum {
		t.Fatalf("expected stale merchant buy to leave post-rebuy inventory unchanged, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	swordID := inventorySnapshot.Inventory[0].ID
	_ = flushServerFrames(t, flow)

	const (
		pveVerticalSkillQuickslotPosition  uint8 = 1
		pveVerticalPotionQuickslotPosition uint8 = 2
		pveVerticalSwordQuickslotPosition  uint8 = 3
		pveVerticalSkillQuickslotIndex     uint8 = 1
	)
	bindPveVerticalQuickslot(t, flow, runtime, accounts, "pve-vertical", hero.Name, pveVerticalSkillQuickslotPosition, quickslotproto.TypeSkill, pveVerticalSkillQuickslotIndex, "unrelated skill quickslot")
	bindPveVerticalQuickslot(t, flow, runtime, accounts, "pve-vertical", hero.Name, pveVerticalPotionQuickslotPosition, quickslotproto.TypeItem, 1, "authored potion item quickslot")
	bindPveVerticalQuickslot(t, flow, runtime, accounts, "pve-vertical", hero.Name, pveVerticalSwordQuickslotPosition, quickslotproto.TypeItem, 0, "authored sword item quickslot")
	assertPveVerticalQuickslots(t, runtime, accounts, "pve-vertical", hero.Name, []QuickslotSnapshot{
		{Position: pveVerticalSkillQuickslotPosition, Type: quickslotproto.TypeSkill, Slot: pveVerticalSkillQuickslotIndex},
		{Position: pveVerticalPotionQuickslotPosition, Type: quickslotproto.TypeItem, Slot: 1},
		{Position: pveVerticalSwordQuickslotPosition, Type: quickslotproto.TypeItem, Slot: 0},
	}, "post-rebuy quickslot bind")

	beforeUsePoints, ok := runtime.PointsSnapshot(hero.Name)
	if !ok {
		t.Fatal("expected points snapshot before authored potion ITEM_USE")
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account before authored potion ITEM_USE: %v", err)
	}
	wantHPAfterUse := beforeUsePoints.Points[bootstrapPlayerPointValueIndex] + 50
	wantPersistedHPAfterUse := account.Characters[0].Points[bootstrapPlayerPointValueIndex] + 50
	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(1)})))
	if err != nil {
		t.Fatalf("unexpected authored potion ITEM_USE: %v", err)
	}
	if len(useOut) != 6 {
		t.Fatalf("expected ITEM_USE echo, point-change, ITEM_DEL, potion QUICKSLOT_DEL, SPECIAL_EFFECT, and info chat for last-stack authored potion, got %d", len(useOut))
	}
	useEcho, err := itemproto.DecodeUse(decodeSingleFrame(t, useOut[0]))
	if err != nil {
		t.Fatalf("decode authored potion ITEM_USE echo: %v", err)
	}
	if useEcho.Position != itemproto.InventoryPosition(1) || useEcho.CharacterVID != hero.VID || useEcho.VictimVID != hero.VID || useEcho.Vnum != pveVerticalMerchantPotionVnum {
		t.Fatalf("unexpected authored potion ITEM_USE echo: %+v", useEcho)
	}
	usePoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, useOut[1]))
	if err != nil {
		t.Fatalf("decode authored potion point-change: %v", err)
	}
	if usePoint.VID != hero.VID || usePoint.Type != bootstrapPlayerPointType || usePoint.Amount != 50 || usePoint.Value != wantHPAfterUse {
		t.Fatalf("unexpected authored potion point-change: %+v want value=%d", usePoint, wantHPAfterUse)
	}
	useDel, err := itemproto.DecodeDel(decodeSingleFrame(t, useOut[2]))
	if err != nil {
		t.Fatalf("decode authored potion ITEM_DEL: %v", err)
	}
	if useDel.Position != itemproto.InventoryPosition(1) {
		t.Fatalf("unexpected authored potion ITEM_DEL: %+v", useDel)
	}
	potionQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, useOut[3]))
	if err != nil {
		t.Fatalf("decode authored potion QUICKSLOT_DEL: %v", err)
	}
	if potionQuickslotDel.Position != pveVerticalPotionQuickslotPosition {
		t.Fatalf("unexpected authored potion QUICKSLOT_DEL: %+v want position=%d", potionQuickslotDel, pveVerticalPotionQuickslotPosition)
	}
	useEffect, err := effectproto.DecodeSpecial(decodeSingleFrame(t, useOut[4]))
	if err != nil {
		t.Fatalf("decode authored potion SPECIAL_EFFECT: %v", err)
	}
	if useEffect.Type != effectproto.SpecialEffectHPUpRed || useEffect.VID != hero.VID {
		t.Fatalf("unexpected authored potion SPECIAL_EFFECT: %+v", useEffect)
	}
	assertPveVerticalSelfOnlyInfoChat(t, useOut[5:], "consume:27001:+50", "authored potion ITEM_USE")
	pointsSnapshot, ok = runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != wantHPAfterUse {
		t.Fatalf("expected live HP %d after authored potion ITEM_USE, got ok=%v snapshot=%+v", wantHPAfterUse, ok, pointsSnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].ID != swordID || inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[0].Slot != 0 {
		t.Fatalf("expected live inventory to keep only the turn-in sword after authored potion ITEM_USE, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after authored potion ITEM_USE: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantPersistedHPAfterUse {
		t.Fatalf("expected persisted HP %d after authored potion ITEM_USE (live=%d), got %d", wantPersistedHPAfterUse, wantHPAfterUse, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].ID != swordID || account.Characters[0].Inventory[0].Vnum != 11200 || account.Characters[0].Inventory[0].Slot != 0 {
		t.Fatalf("expected persisted inventory to keep only the turn-in sword after authored potion ITEM_USE, got %+v", account.Characters[0].Inventory)
	}
	assertPveVerticalQuickslots(t, runtime, accounts, "pve-vertical", hero.Name, []QuickslotSnapshot{
		{Position: pveVerticalSkillQuickslotPosition, Type: quickslotproto.TypeSkill, Slot: pveVerticalSkillQuickslotIndex},
		{Position: pveVerticalSwordQuickslotPosition, Type: quickslotproto.TypeItem, Slot: 0},
	}, "authored potion ITEM_USE")
	assertPveVerticalQuestState(t, runtime, wantAfterReset, "authored potion ITEM_USE")

	weaponPosition, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build weapon equipment position: %v", err)
	}
	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(0),
		Destination: weaponPosition,
	})))
	if err != nil {
		t.Fatalf("unexpected authored sword ITEM_MOVE equip: %v", err)
	}
	if len(equipOut) != 5 {
		t.Fatalf("expected ITEM_DEL, equipment ITEM_SET, PLAYER_POINT_CHANGE, CHARACTER_UPDATE, and sword QUICKSLOT_DEL for empty-weapon authored equip, got %d", len(equipOut))
	}
	equipDel, err := itemproto.DecodeDel(decodeSingleFrame(t, equipOut[0]))
	if err != nil {
		t.Fatalf("decode authored sword equip ITEM_DEL: %v", err)
	}
	if equipDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected authored sword equip ITEM_DEL: %+v", equipDel)
	}
	equipSet, err := itemproto.DecodeSet(decodeSingleFrame(t, equipOut[1]))
	if err != nil {
		t.Fatalf("decode authored sword equipment ITEM_SET: %v", err)
	}
	if equipSet.Position != weaponPosition || equipSet.Vnum != 11200 || equipSet.Count != 1 {
		t.Fatalf("unexpected authored sword equipment ITEM_SET: %+v", equipSet)
	}
	equipPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, equipOut[2]))
	if err != nil {
		t.Fatalf("decode authored sword equip PLAYER_POINT_CHANGE: %v", err)
	}
	wantHPAfterSwordEquip := wantHPAfterUse + 10
	wantPersistedHPAfterSwordEquip := wantPersistedHPAfterUse + 10
	if equipPointChange.VID != hero.VID || equipPointChange.Type != bootstrapPlayerPointType || equipPointChange.Amount != 10 || equipPointChange.Value != wantHPAfterSwordEquip {
		t.Fatalf("unexpected authored sword equip PLAYER_POINT_CHANGE: %+v want value=%d", equipPointChange, wantHPAfterSwordEquip)
	}
	appearance, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, equipOut[3]))
	if err != nil {
		t.Fatalf("decode authored sword CHARACTER_UPDATE: %v", err)
	}
	if appearance.VID != hero.VID || appearance.Parts[0] != hero.MainPart || appearance.Parts[1] != 11201 || appearance.Parts[3] != hero.HairPart {
		t.Fatalf("unexpected authored sword CHARACTER_UPDATE: %+v want vid=%d parts[0]=%d parts[1]=11201 parts[3]=%d", appearance, hero.VID, hero.MainPart, hero.HairPart)
	}
	swordQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, equipOut[4]))
	if err != nil {
		t.Fatalf("decode authored sword equip QUICKSLOT_DEL: %v", err)
	}
	if swordQuickslotDel.Position != pveVerticalSwordQuickslotPosition {
		t.Fatalf("unexpected authored sword equip QUICKSLOT_DEL: %+v want position=%d", swordQuickslotDel, pveVerticalSwordQuickslotPosition)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected live inventory empty after authored sword equip, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	equipmentSnapshot, ok := runtime.EquipmentSnapshot(hero.Name)
	if !ok || len(equipmentSnapshot.Equipment) != 1 || equipmentSnapshot.Equipment[0].ID != swordID || equipmentSnapshot.Equipment[0].Vnum != 11200 || equipmentSnapshot.Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon.String() {
		t.Fatalf("expected live weapon equipment after authored sword equip, got ok=%v snapshot=%+v", ok, equipmentSnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after authored sword equip: %v", err)
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted inventory empty after authored sword equip, got %+v", account.Characters[0].Inventory)
	}
	if len(account.Characters[0].Equipment) != 1 || account.Characters[0].Equipment[0].ID != swordID || account.Characters[0].Equipment[0].Vnum != 11200 || account.Characters[0].Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon {
		t.Fatalf("expected persisted weapon equipment after authored sword equip, got %+v", account.Characters[0].Equipment)
	}
	assertPveVerticalQuickslots(t, runtime, accounts, "pve-vertical", hero.Name, []QuickslotSnapshot{
		{Position: pveVerticalSkillQuickslotPosition, Type: quickslotproto.TypeSkill, Slot: pveVerticalSkillQuickslotIndex},
	}, "authored sword equip")
	pointsSnapshot, ok = runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != wantHPAfterSwordEquip {
		t.Fatalf("expected live HP %d after authored sword equip, got ok=%v snapshot=%+v", wantHPAfterSwordEquip, ok, pointsSnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after authored sword equip: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantPersistedHPAfterSwordEquip {
		t.Fatalf("expected persisted HP %d after authored sword equip, got %d", wantPersistedHPAfterSwordEquip, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	assertPveVerticalQuestState(t, runtime, wantAfterReset, "authored sword equip")

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	assertPveVerticalGatedMismatch(t, flow, merchantVID, "revoked Merchant")
	currentTime = currentTime.Add(staticActorInteractionCooldown)
	assertPveVerticalGatedMismatch(t, flow, talkVID, "revoked VillageGuide talk")
	currentTime = currentTime.Add(staticActorInteractionCooldown)
	assertPveVerticalGatedMismatch(t, flow, resetVID, "repeat QuestResetGuide")
	assertPveVerticalQuestState(t, runtime, wantAfterReset, "revoked services after QuestResetGuide")

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	guideOut, err = flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: guideVID})))
	if err != nil {
		t.Fatalf("unexpected QuestGuide re-unlock interaction error: %v", err)
	}
	if len(guideOut) != 1 {
		t.Fatalf("expected 1 self-only QuestGuide re-unlock frame, got %d", len(guideOut))
	}
	guideChat, err = chatproto.DecodeChatDelivery(decodeSingleFrame(t, guideOut[0]))
	if err != nil || guideChat.Message != "Quest updated: first_steps.met_guide = 1." {
		t.Fatalf("unexpected QuestGuide re-unlock chat delivery: %+v err=%v", guideChat, err)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "QuestGuide re-unlock")

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	resumeShopOut, err = flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected merchant interaction error after QuestGuide re-unlock: %v", err)
	}
	if len(resumeShopOut) != 1 {
		t.Fatalf("expected 1 merchant shop-open frame after QuestGuide re-unlock, got %d", len(resumeShopOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, resumeShopOut[0])); err != nil {
		t.Fatalf("decode merchant shop start after QuestGuide re-unlock: %v", err)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "merchant reopen after QuestGuide re-unlock")
	_ = flushServerFrames(t, flow)

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	warehouseReopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: warehouseVID})))
	if err != nil {
		t.Fatalf("unexpected warehouse interaction after QuestGuide re-unlock: %v", err)
	}
	if len(warehouseReopenOut) != 3 {
		t.Fatalf("expected merchant close plus chat + ShowMeSafeboxPassword after QuestGuide re-unlock, got %d", len(warehouseReopenOut))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, warehouseReopenOut[0])); err != nil {
		t.Fatalf("decode merchant close before authored warehouse password reopen: %v", err)
	}
	warehouseReopenChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, warehouseReopenOut[1]))
	if err != nil || warehouseReopenChat.Message != "The warehouse keeper unlocks the vault." {
		t.Fatalf("unexpected authored warehouse reopen chat: %+v err=%v", warehouseReopenChat, err)
	}
	warehouseReopenPrompt, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, warehouseReopenOut[2]))
	if err != nil || warehouseReopenPrompt.Type != chatproto.ChatTypeCommand || warehouseReopenPrompt.Message != safeboxShowPasswordCommandMessage {
		t.Fatalf("unexpected authored warehouse reopen password prompt: %+v err=%v", warehouseReopenPrompt, err)
	}
	warehouseReopenOpenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password 000000",
	})))
	if err != nil {
		t.Fatalf("unexpected authored warehouse password reopen after QuestGuide re-unlock: %v", err)
	}
	if len(warehouseReopenOpenOut) != 2 {
		t.Fatalf("expected SAFEBOX_SIZE and SAFEBOX_MONEY_CHANGE after authored warehouse password reopen, got %d", len(warehouseReopenOpenOut))
	}
	warehouseReopenSize, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, warehouseReopenOpenOut[0]))
	if err != nil {
		t.Fatalf("decode authored warehouse reopen SAFEBOX_SIZE: %v", err)
	}
	if warehouseReopenSize != (itemproto.SafeboxSizePacket{Size: 2}) {
		t.Fatalf("unexpected authored warehouse reopen SAFEBOX_SIZE: %+v", warehouseReopenSize)
	}
	warehouseReopenMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, warehouseReopenOpenOut[1]))
	if err != nil {
		t.Fatalf("decode authored warehouse reopen SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if warehouseReopenMoney != (itemproto.SafeboxMoneyChangePacket{Money: int32(pveVerticalKillRewardGold)}) {
		t.Fatalf("unexpected authored warehouse reopen SAFEBOX_MONEY_CHANGE: %+v", warehouseReopenMoney)
	}
	wantGoldAfterWarehouseWithdraw := wantGoldAfterRebuy + pveVerticalKillRewardGold
	slashPveVerticalSafeboxMoney(
		t,
		flow,
		fmt.Sprintf("/safebox_money_withdraw %d", pveVerticalKillRewardGold),
		hero.VID,
		int32(pveVerticalKillRewardGold),
		wantGoldAfterWarehouseWithdraw,
		0,
		"authored warehouse money withdraw",
	)
	assertPveVerticalCurrency(t, runtime, accounts, hero.Name, wantGoldAfterWarehouseWithdraw, "authored warehouse money withdraw")
	assertPveVerticalDurableWarehouseGold(t, runtime, "pve-vertical", hero.ID, 0, "authored warehouse money withdraw")

	equippedCheckinOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(0),
	})))
	if err != nil {
		t.Fatalf("unexpected SAFEBOX_CHECKIN while sword is still equipped: %v", err)
	}
	if len(equippedCheckinOut) != 0 {
		t.Fatalf("expected SAFEBOX_CHECKIN while sword is equipped to emit no frames, got %d", len(equippedCheckinOut))
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterWarehouseWithdraw {
		t.Fatalf("expected equipped-sword SAFEBOX_CHECKIN to leave gold at %d, got ok=%v snapshot=%+v", wantGoldAfterWarehouseWithdraw, ok, currencySnapshot)
	}
	equipmentSnapshot, ok = runtime.EquipmentSnapshot(hero.Name)
	if !ok || len(equipmentSnapshot.Equipment) != 1 || equipmentSnapshot.Equipment[0].ID != swordID || equipmentSnapshot.Equipment[0].Vnum != 11200 {
		t.Fatalf("expected equipped-sword SAFEBOX_CHECKIN to leave weapon worn, got ok=%v snapshot=%+v", ok, equipmentSnapshot)
	}

	weaponPosition, err = itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build weapon equipment position for authored unequip: %v", err)
	}
	unequipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      weaponPosition,
		Destination: itemproto.InventoryPosition(0),
	})))
	if err != nil {
		t.Fatalf("unexpected authored sword ITEM_MOVE unequip: %v", err)
	}
	if len(unequipOut) != 4 {
		t.Fatalf("expected ITEM_DEL, carried ITEM_SET, PLAYER_POINT_CHANGE, and CHARACTER_UPDATE for authored sword unequip, got %d", len(unequipOut))
	}
	unequipDel, err := itemproto.DecodeDel(decodeSingleFrame(t, unequipOut[0]))
	if err != nil {
		t.Fatalf("decode authored sword unequip ITEM_DEL: %v", err)
	}
	if unequipDel.Position != weaponPosition {
		t.Fatalf("unexpected authored sword unequip ITEM_DEL: %+v", unequipDel)
	}
	unequipSet, err := itemproto.DecodeSet(decodeSingleFrame(t, unequipOut[1]))
	if err != nil {
		t.Fatalf("decode authored sword unequip ITEM_SET: %v", err)
	}
	if unequipSet.Position != itemproto.InventoryPosition(0) || unequipSet.Vnum != 11200 || unequipSet.Count != 1 {
		t.Fatalf("unexpected authored sword unequip ITEM_SET: %+v", unequipSet)
	}
	unequipPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, unequipOut[2]))
	if err != nil {
		t.Fatalf("decode authored sword unequip PLAYER_POINT_CHANGE: %v", err)
	}
	if unequipPointChange.VID != hero.VID || unequipPointChange.Type != bootstrapPlayerPointType || unequipPointChange.Amount != -10 || unequipPointChange.Value != wantHPAfterUse {
		t.Fatalf("unexpected authored sword unequip PLAYER_POINT_CHANGE: %+v want value=%d", unequipPointChange, wantHPAfterUse)
	}
	unequipAppearance, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, unequipOut[3]))
	if err != nil {
		t.Fatalf("decode authored sword unequip CHARACTER_UPDATE: %v", err)
	}
	if unequipAppearance.VID != hero.VID || unequipAppearance.Parts[1] != 0 {
		t.Fatalf("expected authored sword unequip to clear weapon appearance, got %+v", unequipAppearance)
	}
	pointsSnapshot, ok = runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != wantHPAfterUse {
		t.Fatalf("expected live HP %d after authored sword unequip, got ok=%v snapshot=%+v", wantHPAfterUse, ok, pointsSnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after authored sword unequip: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantPersistedHPAfterUse {
		t.Fatalf("expected persisted HP %d after authored sword unequip, got %d", wantPersistedHPAfterUse, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	checkinOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(0),
	})))
	if err != nil {
		t.Fatalf("unexpected authored sword SAFEBOX_CHECKIN: %v", err)
	}
	if len(checkinOut) != 2 {
		t.Fatalf("expected ITEM_DEL and SAFEBOX_SET for authored sword SAFEBOX_CHECKIN, got %d", len(checkinOut))
	}
	checkinDel, err := itemproto.DecodeDel(decodeSingleFrame(t, checkinOut[0]))
	if err != nil {
		t.Fatalf("decode authored sword SAFEBOX_CHECKIN ITEM_DEL: %v", err)
	}
	if checkinDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected authored sword SAFEBOX_CHECKIN ITEM_DEL: %+v", checkinDel)
	}
	checkinSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, checkinOut[1]))
	if err != nil {
		t.Fatalf("decode authored sword SAFEBOX_SET: %v", err)
	}
	if checkinSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || checkinSet.Vnum != 11200 || checkinSet.Count != 1 {
		t.Fatalf("unexpected authored sword SAFEBOX_SET: %+v", checkinSet)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected live inventory empty after authored sword SAFEBOX_CHECKIN, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	equipmentSnapshot, ok = runtime.EquipmentSnapshot(hero.Name)
	if !ok || len(equipmentSnapshot.Equipment) != 0 {
		t.Fatalf("expected live equipment empty after authored sword SAFEBOX_CHECKIN, got ok=%v snapshot=%+v", ok, equipmentSnapshot)
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterWarehouseWithdraw {
		t.Fatalf("expected authored sword SAFEBOX_CHECKIN to leave gold at %d, got ok=%v snapshot=%+v", wantGoldAfterWarehouseWithdraw, ok, currencySnapshot)
	}
	assertPveVerticalDurableWarehouseCell(t, runtime, "pve-vertical", hero.ID, 0, inventory.ItemInstance{ID: swordID, Vnum: 11200, Count: 1, Slot: 0}, "authored sword SAFEBOX_CHECKIN")

	oorMoveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.InventoryPosition(0),
		Destination: itemproto.InventoryPosition(10),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected authored sword SAFEBOX_ITEM_MOVE out of size-2 range: %v", err)
	}
	if len(oorMoveOut) != 0 {
		t.Fatalf("expected authored sword SAFEBOX_ITEM_MOVE into cell 10 to emit no frames, got %d", len(oorMoveOut))
	}
	assertPveVerticalDurableWarehouseCell(t, runtime, "pve-vertical", hero.ID, 0, inventory.ItemInstance{ID: swordID, Vnum: 11200, Count: 1, Slot: 0}, "authored sword SAFEBOX_ITEM_MOVE out of size-2 range")

	safeboxMoveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.InventoryPosition(0),
		Destination: itemproto.InventoryPosition(5),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected authored sword SAFEBOX_ITEM_MOVE into size-2 cell 5: %v", err)
	}
	if len(safeboxMoveOut) != 2 {
		t.Fatalf("expected SAFEBOX_DEL and SAFEBOX_SET for authored sword SAFEBOX_ITEM_MOVE, got %d", len(safeboxMoveOut))
	}
	moveDel, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, safeboxMoveOut[0]))
	if err != nil {
		t.Fatalf("decode authored sword SAFEBOX_ITEM_MOVE SAFEBOX_DEL: %v", err)
	}
	if moveDel.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) {
		t.Fatalf("unexpected authored sword SAFEBOX_ITEM_MOVE SAFEBOX_DEL: %+v", moveDel)
	}
	moveSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, safeboxMoveOut[1]))
	if err != nil {
		t.Fatalf("decode authored sword SAFEBOX_ITEM_MOVE SAFEBOX_SET: %v", err)
	}
	if moveSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 5}) || moveSet.Vnum != 11200 || moveSet.Count != 1 {
		t.Fatalf("unexpected authored sword SAFEBOX_ITEM_MOVE SAFEBOX_SET: %+v", moveSet)
	}
	assertPveVerticalDurableWarehouseCell(t, runtime, "pve-vertical", hero.ID, 5, inventory.ItemInstance{ID: swordID, Vnum: 11200, Count: 1, Slot: 5}, "authored sword SAFEBOX_ITEM_MOVE into size-2 cell 5")
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterWarehouseWithdraw {
		t.Fatalf("expected authored sword SAFEBOX_ITEM_MOVE to leave gold at %d, got ok=%v snapshot=%+v", wantGoldAfterWarehouseWithdraw, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected live inventory empty after authored sword SAFEBOX_ITEM_MOVE, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}

	checkoutOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 5,
		Position: itemproto.InventoryPosition(0),
	})))
	if err != nil {
		t.Fatalf("unexpected authored sword SAFEBOX_CHECKOUT: %v", err)
	}
	if len(checkoutOut) != 2 {
		t.Fatalf("expected SAFEBOX_DEL and ITEM_SET for authored sword SAFEBOX_CHECKOUT, got %d", len(checkoutOut))
	}
	checkoutDel, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, checkoutOut[0]))
	if err != nil {
		t.Fatalf("decode authored sword SAFEBOX_DEL: %v", err)
	}
	if checkoutDel.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 5}) {
		t.Fatalf("unexpected authored sword SAFEBOX_DEL: %+v", checkoutDel)
	}
	checkoutSet, err := itemproto.DecodeSet(decodeSingleFrame(t, checkoutOut[1]))
	if err != nil {
		t.Fatalf("decode authored sword checkout ITEM_SET: %v", err)
	}
	if checkoutSet.Position != itemproto.InventoryPosition(0) || checkoutSet.Vnum != 11200 || checkoutSet.Count != 1 {
		t.Fatalf("unexpected authored sword checkout ITEM_SET: %+v", checkoutSet)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].ID != swordID || inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[0].Slot != 0 {
		t.Fatalf("expected live inventory to hold checked-out sword, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	assertPveVerticalDurableWarehouseEmpty(t, runtime, "pve-vertical", hero.ID, "authored sword SAFEBOX_CHECKOUT")

	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "pve vertical warehouse close before authored sword SHOP SELL")
	currentTime = currentTime.Add(staticActorInteractionCooldown)
	merchantReopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected merchant reopen after warehouse checkout: %v", err)
	}
	if len(merchantReopenOut) != 1 {
		t.Fatalf("expected 1 merchant shop-open frame after warehouse checkout, got %d", len(merchantReopenOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, merchantReopenOut[0])); err != nil {
		t.Fatalf("decode merchant shop start after warehouse checkout: %v", err)
	}

	const pveVerticalSwordEquipDelta = int32(10)
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account before merchant-phase authored sword equip: %v", err)
	}
	persistedHPBeforeMerchantSwordEquip := account.Characters[0].Points[bootstrapPlayerPointValueIndex]
	weaponPosition, err = itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build authored sword weapon equipment position: %v", err)
	}
	swordEquipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(0),
		Destination: weaponPosition,
	})))
	if err != nil {
		t.Fatalf("unexpected authored sword ITEM_MOVE equip: %v", err)
	}
	if len(swordEquipOut) != 4 {
		t.Fatalf("expected ITEM_DEL, equipment ITEM_SET, PLAYER_POINT_CHANGE, and CHARACTER_UPDATE for authored sword equip, got %d", len(swordEquipOut))
	}
	swordEquipDel, err := itemproto.DecodeDel(decodeSingleFrame(t, swordEquipOut[0]))
	if err != nil {
		t.Fatalf("decode authored sword equip ITEM_DEL: %v", err)
	}
	if swordEquipDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected authored sword equip ITEM_DEL: %+v", swordEquipDel)
	}
	swordEquipSet, err := itemproto.DecodeSet(decodeSingleFrame(t, swordEquipOut[1]))
	if err != nil {
		t.Fatalf("decode authored sword equip ITEM_SET: %v", err)
	}
	if swordEquipSet.Position != weaponPosition || swordEquipSet.Vnum != 11200 || swordEquipSet.Count != 1 {
		t.Fatalf("unexpected authored sword equip ITEM_SET: %+v", swordEquipSet)
	}
	swordEquipPoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, swordEquipOut[2]))
	if err != nil {
		t.Fatalf("decode authored sword equip PLAYER_POINT_CHANGE: %v", err)
	}
	if swordEquipPoint.VID != hero.VID || swordEquipPoint.Type != bootstrapPlayerPointType || swordEquipPoint.Amount != pveVerticalSwordEquipDelta {
		t.Fatalf("unexpected authored sword equip PLAYER_POINT_CHANGE: %+v", swordEquipPoint)
	}
	wantHPAfterMerchantSwordEquip := swordEquipPoint.Value
	swordEquipAppearance, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, swordEquipOut[3]))
	if err != nil {
		t.Fatalf("decode authored sword equip CHARACTER_UPDATE: %v", err)
	}
	if swordEquipAppearance.VID != hero.VID || swordEquipAppearance.Parts[1] != 11201 {
		t.Fatalf("unexpected authored sword equip CHARACTER_UPDATE: %+v", swordEquipAppearance)
	}
	pointsSnapshot, ok = runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != wantHPAfterMerchantSwordEquip {
		t.Fatalf("expected live HP %d after authored sword equip, got ok=%v snapshot=%+v", wantHPAfterMerchantSwordEquip, ok, pointsSnapshot)
	}
	equipmentSnapshot, ok = runtime.EquipmentSnapshot(hero.Name)
	if !ok || len(equipmentSnapshot.Equipment) != 1 || equipmentSnapshot.Equipment[0].ID != swordID || equipmentSnapshot.Equipment[0].Vnum != 11200 || equipmentSnapshot.Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon.String() {
		t.Fatalf("expected live equipment to hold authored sword after equip, got ok=%v snapshot=%+v", ok, equipmentSnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after authored sword equip: %v", err)
	}
	if len(account.Characters[0].Inventory) != 0 || len(account.Characters[0].Equipment) != 1 || account.Characters[0].Equipment[0].Vnum != 11200 || account.Characters[0].Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon || !account.Characters[0].Equipment[0].Equipped {
		t.Fatalf("expected persisted authored sword equipment placement, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != persistedHPBeforeMerchantSwordEquip+pveVerticalSwordEquipDelta {
		t.Fatalf("expected persisted authored sword equip HP %d, got %d", persistedHPBeforeMerchantSwordEquip+pveVerticalSwordEquipDelta, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}

	unequipForMerchantOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      weaponPosition,
		Destination: itemproto.InventoryPosition(0),
	})))
	if err != nil {
		t.Fatalf("unexpected authored sword ITEM_MOVE unequip before SHOP SELL: %v", err)
	}
	if len(unequipForMerchantOut) != 4 {
		t.Fatalf("expected ITEM_DEL, carried ITEM_SET, PLAYER_POINT_CHANGE, and CHARACTER_UPDATE for authored sword unequip before SHOP SELL, got %d", len(unequipForMerchantOut))
	}
	unequipForMerchantPoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, unequipForMerchantOut[2]))
	if err != nil {
		t.Fatalf("decode authored sword unequip before SHOP SELL PLAYER_POINT_CHANGE: %v", err)
	}
	if unequipForMerchantPoint.VID != hero.VID || unequipForMerchantPoint.Type != bootstrapPlayerPointType || unequipForMerchantPoint.Amount != -pveVerticalSwordEquipDelta {
		t.Fatalf("unexpected authored sword unequip before SHOP SELL PLAYER_POINT_CHANGE: %+v", unequipForMerchantPoint)
	}
	pointsSnapshot, ok = runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != unequipForMerchantPoint.Value {
		t.Fatalf("expected live HP %d after authored sword unequip before SHOP SELL, got ok=%v snapshot=%+v", unequipForMerchantPoint.Value, ok, pointsSnapshot)
	}

	const pveVerticalSwordSellPrice = int32(100)
	wantGoldAfterSwordSell := wantGoldAfterWarehouseWithdraw + uint64(pveVerticalSwordSellPrice)
	sellOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell(shopproto.ClientSellPacket{Slot: 0})))
	if err != nil {
		t.Fatalf("unexpected authored sword SHOP SELL: %v", err)
	}
	if len(sellOut) != 2 {
		t.Fatalf("expected carried ITEM_DEL and gold PLAYER_POINT_CHANGE for authored sword SHOP SELL, got %d", len(sellOut))
	}
	sellDel, err := itemproto.DecodeDel(decodeSingleFrame(t, sellOut[0]))
	if err != nil {
		t.Fatalf("decode authored sword SHOP SELL ITEM_DEL: %v", err)
	}
	if sellDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected authored sword SHOP SELL ITEM_DEL: %+v", sellDel)
	}
	sellGold, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, sellOut[1]))
	if err != nil {
		t.Fatalf("decode authored sword SHOP SELL gold point-change: %v", err)
	}
	if sellGold.VID != hero.VID || sellGold.Type != bootstrapGoldPointType || sellGold.Amount != pveVerticalSwordSellPrice || uint64(sellGold.Value) != wantGoldAfterSwordSell {
		t.Fatalf("unexpected authored sword SHOP SELL gold point-change: %+v want amount=%d value=%d", sellGold, pveVerticalSwordSellPrice, wantGoldAfterSwordSell)
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterSwordSell {
		t.Fatalf("expected live gold %d after authored sword SHOP SELL, got ok=%v snapshot=%+v", wantGoldAfterSwordSell, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected live inventory empty after authored sword SHOP SELL, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	equipmentSnapshot, ok = runtime.EquipmentSnapshot(hero.Name)
	if !ok || len(equipmentSnapshot.Equipment) != 0 {
		t.Fatalf("expected live equipment empty after authored sword SHOP SELL, got ok=%v snapshot=%+v", ok, equipmentSnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after authored sword SHOP SELL: %v", err)
	}
	if account.Characters[0].Gold != wantGoldAfterSwordSell {
		t.Fatalf("expected persisted gold %d after authored sword SHOP SELL, got %d", wantGoldAfterSwordSell, account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 0 || len(account.Characters[0].Equipment) != 0 {
		t.Fatalf("expected persisted empty inventory/equipment after authored sword SHOP SELL, got %+v", account.Characters[0])
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != persistedHPBeforeMerchantSwordEquip {
		t.Fatalf("expected persisted authored sword unequip HP %d, got %d", persistedHPBeforeMerchantSwordEquip, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "authored sword SHOP SELL")

	const pveVerticalCubeMaterialPrice = uint64(20)
	wantGoldAfterCubeMaterialBuy := wantGoldAfterSwordSell - pveVerticalCubeMaterialPrice
	cubeMaterialBuyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 2})))
	if err != nil {
		t.Fatalf("unexpected authored cube-material merchant buy error: %v", err)
	}
	if len(cubeMaterialBuyOut) != 1 {
		t.Fatalf("expected 1 item refresh frame for catalog slot 2 merchant buy, got %d", len(cubeMaterialBuyOut))
	}
	cubeMaterialSet, err := itemproto.DecodeSet(decodeSingleFrame(t, cubeMaterialBuyOut[0]))
	if err != nil {
		t.Fatalf("decode authored cube-material merchant buy item set: %v", err)
	}
	if cubeMaterialSet.Position != itemproto.InventoryPosition(0) || cubeMaterialSet.Vnum != 27002 || cubeMaterialSet.Count != 2 {
		t.Fatalf("unexpected authored cube-material merchant buy item set: %+v", cubeMaterialSet)
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterCubeMaterialBuy {
		t.Fatalf("expected live gold %d after authored cube-material merchant buy, got ok=%v snapshot=%+v", wantGoldAfterCubeMaterialBuy, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 {
		t.Fatalf("expected live inventory one 27002 stack after authored cube-material merchant buy, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	boughtMaterials := inventorySnapshot.Inventory[0]
	if boughtMaterials.Vnum != 27002 || boughtMaterials.Count != 2 || boughtMaterials.Slot != 0 || boughtMaterials.ID == 0 {
		t.Fatalf("unexpected live cube-material inventory after merchant buy: %+v", boughtMaterials)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after authored cube-material merchant buy: %v", err)
	}
	if account.Characters[0].Gold != wantGoldAfterCubeMaterialBuy {
		t.Fatalf("expected persisted gold %d after authored cube-material merchant buy, got %d", wantGoldAfterCubeMaterialBuy, account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].ID != boughtMaterials.ID || account.Characters[0].Inventory[0].Vnum != 27002 || account.Characters[0].Inventory[0].Count != 2 || account.Characters[0].Inventory[0].Slot != 0 {
		t.Fatalf("expected persisted cube-material inventory after merchant buy, got %+v", account.Characters[0].Inventory)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "authored cube-material merchant buy")

	closeShopAfterCubeMaterialOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected merchant close after authored cube-material buy: %v", err)
	}
	if len(closeShopAfterCubeMaterialOut) != 1 {
		t.Fatalf("expected 1 merchant close frame after authored cube-material buy, got %d", len(closeShopAfterCubeMaterialOut))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, closeShopAfterCubeMaterialOut[0])); err != nil {
		t.Fatalf("decode merchant shop end after authored cube-material buy: %v", err)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	craftCubeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: cubeVID})))
	if err != nil {
		t.Fatalf("unexpected CubeMaster interaction after authored cube-material buy: %v", err)
	}
	if len(craftCubeOut) != 2 {
		t.Fatalf("expected chat + cube open frames after authored cube-material buy, got %d", len(craftCubeOut))
	}
	craftCubeChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, craftCubeOut[0]))
	if err != nil || craftCubeChat.Type != chatproto.ChatTypeInfo || craftCubeChat.VID != 0 || craftCubeChat.Empire != 0 || craftCubeChat.Message != "The craftsman lights the forge." {
		t.Fatalf("unexpected authored CubeMaster craft-open chat: %+v err=%v", craftCubeChat, err)
	}
	assertCubeCommandChatFrame(t, craftCubeOut[1], "cube open 20022", "pve vertical CubeMaster craft open")

	addOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 0",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add 0 0 after authored CubeMaster craft open: %v", err)
	}
	if len(addOut) != 1 {
		t.Fatalf("expected /cube add 0 0 to emit one command chat frame, got %d", len(addOut))
	}
	assertCubeCommandChatFrame(t, addOut[0], "cube info 100 0 0", "pve vertical matched stacked cube add")
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterCubeMaterialBuy {
		t.Fatalf("expected live gold %d after stacked cube add, got ok=%v snapshot=%+v", wantGoldAfterCubeMaterialBuy, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].ID != boughtMaterials.ID || inventorySnapshot.Inventory[0].Vnum != 27002 || inventorySnapshot.Inventory[0].Count != 2 || inventorySnapshot.Inventory[0].Slot != 0 {
		t.Fatalf("expected stacked cube add to leave cube-material inventory unchanged, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "authored stacked cube add")

	makeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube make",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube make after authored CubeMaster add: %v", err)
	}
	if len(makeOut) != 5 {
		t.Fatalf("expected authored CubeMaster /cube make burst of 5 frames, got %d", len(makeOut))
	}
	materialDel, err := itemproto.DecodeDel(decodeSingleFrame(t, makeOut[0]))
	if err != nil {
		t.Fatalf("decode authored cube-make material ITEM_DEL: %v", err)
	}
	if materialDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected authored cube-make material delete: %+v", materialDel)
	}
	rewardSet, err := itemproto.DecodeSet(decodeSingleFrame(t, makeOut[1]))
	if err != nil {
		t.Fatalf("decode authored cube-make reward ITEM_SET: %v", err)
	}
	if rewardSet.Position != itemproto.InventoryPosition(0) || rewardSet.Vnum != 27001 || rewardSet.Count != 1 {
		t.Fatalf("unexpected authored cube-make reward ITEM_SET: %+v", rewardSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, makeOut[2]))
	if err != nil {
		t.Fatalf("decode authored cube-make gold PLAYER_POINT_CHANGE: %v", err)
	}
	wantGoldAfterCubeMake := wantGoldAfterCubeMaterialBuy - 100
	if goldChange.VID != hero.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -100 || uint64(goldChange.Value) != wantGoldAfterCubeMake {
		t.Fatalf("unexpected authored cube-make gold point-change: %+v want value=%d", goldChange, wantGoldAfterCubeMake)
	}
	assertCubeCommandChatFrame(t, makeOut[3], cubestore.FormatCubeSuccessCommand(27001, 1), "pve vertical cube make success")
	assertCubeCommandChatFrame(t, makeOut[4], "cube info 0 0 0", "pve vertical post-make cube info")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected authored CubeMaster /cube make to queue no peer frames, got %d", len(queued))
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterCubeMake {
		t.Fatalf("expected live gold %d after authored cube make, got ok=%v snapshot=%+v", wantGoldAfterCubeMake, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 27001 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 {
		t.Fatalf("expected live inventory one 27001 after authored cube make, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after authored cube make: %v", err)
	}
	if account.Characters[0].Gold != wantGoldAfterCubeMake {
		t.Fatalf("expected persisted gold %d after authored cube make, got %d", wantGoldAfterCubeMake, account.Characters[0].Gold)
	}
	if len(account.Characters[0].Inventory) != 1 || account.Characters[0].Inventory[0].Vnum != 27001 || account.Characters[0].Inventory[0].Count != 1 || account.Characters[0].Inventory[0].Slot != 0 {
		t.Fatalf("expected persisted inventory one 27001 after authored cube make, got %+v", account.Characters[0].Inventory)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "authored cube make")

	beforeCraftedUsePoints, ok := runtime.PointsSnapshot(hero.Name)
	if !ok {
		t.Fatal("expected points snapshot before authored cube-granted potion ITEM_USE")
	}
	wantHPAfterCraftedUse := beforeCraftedUsePoints.Points[bootstrapPlayerPointValueIndex] + 50
	wantPersistedHPAfterCraftedUse := account.Characters[0].Points[bootstrapPlayerPointValueIndex] + 50

	assertCloseCubeCommandChat(t, flow, "/close_cube", "pve vertical cube close after authored make")
	closedCraftCubeRInfoOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info",
	})))
	if err != nil {
		t.Fatalf("unexpected closed-cube r_info after authored CubeMaster make: %v", err)
	}
	if len(closedCraftCubeRInfoOut) != 0 {
		t.Fatalf("expected closed-cube r_info after authored make to emit no frames, got %d", len(closedCraftCubeRInfoOut))
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterCubeMake {
		t.Fatalf("expected live gold %d after authored cube close, got ok=%v snapshot=%+v", wantGoldAfterCubeMake, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 27001 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 {
		t.Fatalf("expected live inventory one 27001 after authored cube close, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "authored cube close after make")

	craftedUseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(0)})))
	if err != nil {
		t.Fatalf("unexpected authored cube-granted potion ITEM_USE: %v", err)
	}
	if len(craftedUseOut) != 5 {
		t.Fatalf("expected ITEM_USE echo, point-change, ITEM_DEL, SPECIAL_EFFECT, and info chat for cube-granted last-stack potion, got %d", len(craftedUseOut))
	}
	craftedUseEcho, err := itemproto.DecodeUse(decodeSingleFrame(t, craftedUseOut[0]))
	if err != nil {
		t.Fatalf("decode authored cube-granted potion ITEM_USE echo: %v", err)
	}
	if craftedUseEcho.Position != itemproto.InventoryPosition(0) || craftedUseEcho.CharacterVID != hero.VID || craftedUseEcho.VictimVID != hero.VID || craftedUseEcho.Vnum != 27001 {
		t.Fatalf("unexpected authored cube-granted potion ITEM_USE echo: %+v", craftedUseEcho)
	}
	craftedUsePoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, craftedUseOut[1]))
	if err != nil {
		t.Fatalf("decode authored cube-granted potion point-change: %v", err)
	}
	if craftedUsePoint.VID != hero.VID || craftedUsePoint.Type != bootstrapPlayerPointType || craftedUsePoint.Amount != 50 || craftedUsePoint.Value != wantHPAfterCraftedUse {
		t.Fatalf("unexpected authored cube-granted potion point-change: %+v want value=%d", craftedUsePoint, wantHPAfterCraftedUse)
	}
	craftedUseDel, err := itemproto.DecodeDel(decodeSingleFrame(t, craftedUseOut[2]))
	if err != nil {
		t.Fatalf("decode authored cube-granted potion ITEM_DEL: %v", err)
	}
	if craftedUseDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected authored cube-granted potion ITEM_DEL: %+v", craftedUseDel)
	}
	craftedUseEffect, err := effectproto.DecodeSpecial(decodeSingleFrame(t, craftedUseOut[3]))
	if err != nil {
		t.Fatalf("decode authored cube-granted potion SPECIAL_EFFECT: %v", err)
	}
	if craftedUseEffect.Type != effectproto.SpecialEffectHPUpRed || craftedUseEffect.VID != hero.VID {
		t.Fatalf("unexpected authored cube-granted potion SPECIAL_EFFECT: %+v", craftedUseEffect)
	}
	assertPveVerticalSelfOnlyInfoChat(t, craftedUseOut[4:], "consume:27001:+50", "authored cube-granted potion ITEM_USE")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected authored cube-granted potion ITEM_USE to queue no peer frames, got %d", len(queued))
	}
	pointsSnapshot, ok = runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != wantHPAfterCraftedUse {
		t.Fatalf("expected live HP %d after authored cube-granted potion ITEM_USE, got ok=%v snapshot=%+v", wantHPAfterCraftedUse, ok, pointsSnapshot)
	}
	currencySnapshot, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || currencySnapshot.Gold != wantGoldAfterCubeMake {
		t.Fatalf("expected live gold %d after authored cube-granted potion ITEM_USE, got ok=%v snapshot=%+v", wantGoldAfterCubeMake, ok, currencySnapshot)
	}
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected live inventory empty after authored cube-granted potion ITEM_USE, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after authored cube-granted potion ITEM_USE: %v", err)
	}
	if account.Characters[0].Gold != wantGoldAfterCubeMake {
		t.Fatalf("expected persisted gold %d after authored cube-granted potion ITEM_USE, got %d", wantGoldAfterCubeMake, account.Characters[0].Gold)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantPersistedHPAfterCraftedUse {
		t.Fatalf("expected persisted HP %d after authored cube-granted potion ITEM_USE (live=%d), got %d", wantPersistedHPAfterCraftedUse, wantHPAfterCraftedUse, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if len(account.Characters[0].Inventory) != 0 || len(account.Characters[0].Equipment) != 0 {
		t.Fatalf("expected persisted inventory/equipment empty after authored cube-granted potion ITEM_USE, got inventory=%+v equipment=%+v", account.Characters[0].Inventory, account.Characters[0].Equipment)
	}
	assertPveVerticalQuickslots(t, runtime, accounts, "pve-vertical", hero.Name, []QuickslotSnapshot{
		{Position: pveVerticalSkillQuickslotPosition, Type: quickslotproto.TypeSkill, Slot: pveVerticalSkillQuickslotIndex},
	}, "authored cube-granted potion ITEM_USE")
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "authored cube-granted potion ITEM_USE")

	closeSessionFlow(t, flow)
	flow, craftedReconnectEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pve-vertical", 0x60606060)
	if len(craftedReconnectEnter) < 4 {
		t.Fatalf("expected cube-grant consume reconnect bootstrap to include self state, got %d frames", len(craftedReconnectEnter))
	}
	for _, raw := range craftedReconnectEnter {
		set, err := itemproto.DecodeSet(decodeSingleFrame(t, raw))
		if err != nil {
			continue
		}
		if set.Position.WindowType == itemproto.WindowInventory && set.Position.Cell < itemproto.InventoryMaxCell {
			t.Fatalf("expected cube-grant consume reconnect bootstrap to omit carried ITEM_SET, got %+v", set)
		}
	}
	closedCubeRInfoAfterReconnectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube r_info",
	})))
	if err != nil {
		t.Fatalf("unexpected closed-cube r_info after cube-grant consume reconnect: %v", err)
	}
	if len(closedCubeRInfoAfterReconnectOut) != 0 {
		t.Fatalf("expected cube-grant consume reconnect to leave cube closed, got %d r_info frames", len(closedCubeRInfoAfterReconnectOut))
	}
	assertPveVerticalCurrency(t, runtime, accounts, hero.Name, wantGoldAfterCubeMake, "cube-grant consume reconnect")
	inventorySnapshot, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 0 {
		t.Fatalf("expected cube-grant consume reconnect to keep inventory empty, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	pointsSnapshot, ok = runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != wantPersistedHPAfterCraftedUse {
		t.Fatalf("expected rematerialized HP %d after cube-grant consume reconnect, got ok=%v snapshot=%+v", wantPersistedHPAfterCraftedUse, ok, pointsSnapshot)
	}
	account, err = accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after cube-grant consume reconnect: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantPersistedHPAfterCraftedUse {
		t.Fatalf("expected persisted HP %d after cube-grant consume reconnect, got %d", wantPersistedHPAfterCraftedUse, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if len(account.Characters[0].Inventory) != 0 || len(account.Characters[0].Equipment) != 0 {
		t.Fatalf("expected persisted inventory/equipment empty after cube-grant consume reconnect, got inventory=%+v equipment=%+v", account.Characters[0].Inventory, account.Characters[0].Equipment)
	}
	assertPveVerticalQuickslots(t, runtime, accounts, "pve-vertical", hero.Name, []QuickslotSnapshot{
		{Position: pveVerticalSkillQuickslotPosition, Type: quickslotproto.TypeSkill, Slot: pveVerticalSkillQuickslotIndex},
	}, "cube-grant consume reconnect")
	assertPveVerticalQuestState(t, runtime, wantAfterGuide, "cube-grant consume reconnect")
}

func interactPveVertical(t *testing.T, flow service.SessionFlow, targetVID uint32, context string) [][]byte {
	t.Helper()
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected %s interaction error: %v", context, err)
	}
	return out
}

func bindPveVerticalQuickslot(t *testing.T, flow service.SessionFlow, runtime *gameRuntime, accounts *accountstore.FileStore, login string, name string, position uint8, slotType uint8, slot uint8, context string) {
	t.Helper()
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, quickslotproto.EncodeClientAdd(quickslotproto.ClientAddPacket{
		Position: position,
		Slot:     quickslotproto.Slot{Type: slotType, Position: slot},
	})))
	if err != nil {
		t.Fatalf("unexpected %s QUICKSLOT_ADD: %v", context, err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 self-only QUICKSLOT_ADD for %s, got %d", context, len(out))
	}
	add, err := quickslotproto.DecodeAdd(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode %s QUICKSLOT_ADD: %v", context, err)
	}
	if add.Position != position || add.Slot.Type != slotType || add.Slot.Position != slot {
		t.Fatalf("unexpected %s QUICKSLOT_ADD: %+v want position=%d type=%d slot=%d", context, add, position, slotType, slot)
	}
	want := QuickslotSnapshot{Position: position, Type: slotType, Slot: slot}
	live, ok := runtime.QuickslotsSnapshot(name)
	if !ok {
		t.Fatalf("expected live quickslots after %s", context)
	}
	foundLive := false
	for _, quickslot := range live.Quickslots {
		if quickslot == want {
			foundLive = true
			break
		}
	}
	if !foundLive {
		t.Fatalf("expected live %s binding %+v, got %+v", context, want, live.Quickslots)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted account after %s: %v", context, err)
	}
	foundPersisted := false
	for _, quickslot := range account.Characters[0].Quickslots {
		if quickslot.Position == position && quickslot.Type == slotType && quickslot.Slot == slot {
			foundPersisted = true
			break
		}
	}
	if !foundPersisted {
		t.Fatalf("expected persisted %s binding %+v, got %+v", context, want, account.Characters[0].Quickslots)
	}
}

func assertPveVerticalQuickslots(t *testing.T, runtime *gameRuntime, accounts *accountstore.FileStore, login string, name string, want []QuickslotSnapshot, context string) {
	t.Helper()
	live, ok := runtime.QuickslotsSnapshot(name)
	if !ok {
		t.Fatalf("expected live quickslots after %s", context)
	}
	if !reflect.DeepEqual(live.Quickslots, want) {
		t.Fatalf("unexpected live quickslots after %s: got %+v want %+v", context, live.Quickslots, want)
	}
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted account after %s: %v", context, err)
	}
	got := make([]QuickslotSnapshot, 0, len(account.Characters[0].Quickslots))
	for _, quickslot := range account.Characters[0].Quickslots {
		got = append(got, QuickslotSnapshot{Position: quickslot.Position, Type: quickslot.Type, Slot: quickslot.Slot})
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected persisted quickslots after %s: got %+v want %+v", context, got, want)
	}
}

func assertPveVerticalGatedMismatch(t *testing.T, flow service.SessionFlow, targetVID uint32, context string) {
	t.Helper()
	assertPveVerticalSelfOnlyInfoChat(t, interactPveVertical(t, flow, targetVID, "gated "+context+" mismatch"), "Quest requirements are not met.", "gated "+context+" mismatch")
}

func assertPveVerticalSelfOnlyInfoChat(t *testing.T, frames [][]byte, wantMessage string, context string) {
	t.Helper()
	if len(frames) != 1 {
		t.Fatalf("expected 1 self-only %s frame, got %d", context, len(frames))
	}
	chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frames[0]))
	if err != nil || chat.Type != chatproto.ChatTypeInfo || chat.VID != 0 || chat.Empire != 0 || chat.Message != wantMessage {
		t.Fatalf("unexpected %s chat: %+v err=%v", context, chat, err)
	}
}

func slashPveVerticalSafeboxMoney(t *testing.T, flow service.SessionFlow, command string, heroVID uint32, goldAmount int32, goldValue uint64, warehouseMoney int32, context string) {
	t.Helper()
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: command,
	})))
	if err != nil {
		t.Fatalf("unexpected %s: %v", context, err)
	}
	if len(out) != 2 {
		t.Fatalf("expected gold PLAYER_POINT_CHANGE + SAFEBOX_MONEY_CHANGE for %s, got %d", context, len(out))
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode %s gold PLAYER_POINT_CHANGE: %v", context, err)
	}
	if goldChange.VID != heroVID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != goldAmount || uint64(goldChange.Value) != goldValue {
		t.Fatalf("unexpected %s gold PLAYER_POINT_CHANGE: %+v want amount=%d value=%d", context, goldChange, goldAmount, goldValue)
	}
	assertPveVerticalSafeboxMoneyChange(t, out[1], warehouseMoney, context)
}

func assertPveVerticalSafeboxMoneyChange(t *testing.T, frame []byte, want int32, context string) {
	t.Helper()
	money, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, frame))
	if err != nil {
		t.Fatalf("decode %s SAFEBOX_MONEY_CHANGE: %v", context, err)
	}
	if money != (itemproto.SafeboxMoneyChangePacket{Money: want}) {
		t.Fatalf("unexpected %s SAFEBOX_MONEY_CHANGE: %+v want %d", context, money, want)
	}
}

func assertPveVerticalCurrency(t *testing.T, runtime *gameRuntime, accounts *accountstore.FileStore, name string, wantGold uint64, context string) {
	t.Helper()
	currency, ok := runtime.CurrencySnapshot(name)
	if !ok || currency.Gold != wantGold {
		t.Fatalf("expected live gold %d after %s, got ok=%v snapshot=%+v", wantGold, context, ok, currency)
	}
	account, err := accounts.Load("pve-vertical")
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after %s: %v", context, err)
	}
	if account.Characters[0].Gold != wantGold {
		t.Fatalf("expected persisted gold %d after %s, got %d", wantGold, context, account.Characters[0].Gold)
	}
}

func assertPveVerticalDurableWarehouseGold(t *testing.T, runtime *gameRuntime, login string, characterID uint32, want int64, context string) {
	t.Helper()
	snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
	if err != nil {
		t.Fatalf("load durable safebox after %s: %v", context, err)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, characterID); got != want {
		t.Fatalf("durable warehouse gold after %s=%d want %d", context, got, want)
	}
}

func assertPveVerticalDurableWarehouseCell(t *testing.T, runtime *gameRuntime, login string, characterID uint32, cell uint8, want inventory.ItemInstance, context string) {
	t.Helper()
	snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
	if err != nil {
		t.Fatalf("load durable safebox after %s: %v", context, err)
	}
	cells := safeboxstore.CharacterCells(snapshot, login, characterID)
	got, ok := cells[cell]
	if !ok {
		t.Fatalf("expected durable warehouse cell %d after %s, got %+v", cell, context, cells)
	}
	if got.ID != want.ID || got.Vnum != want.Vnum || got.Count != want.Count || got.Slot != want.Slot {
		t.Fatalf("durable warehouse cell %d after %s=%+v want %+v", cell, context, got, want)
	}
	if len(cells) != 1 {
		t.Fatalf("expected exactly one durable warehouse cell after %s, got %+v", context, cells)
	}
}

func assertPveVerticalDurableWarehouseEmpty(t *testing.T, runtime *gameRuntime, login string, characterID uint32, context string) {
	t.Helper()
	snapshot, err := safeboxstore.LoadOrEmpty(runtime.safeboxStore)
	if err != nil {
		t.Fatalf("load durable safebox after %s: %v", context, err)
	}
	if cells := safeboxstore.CharacterCells(snapshot, login, characterID); len(cells) != 0 {
		t.Fatalf("expected durable warehouse empty after %s, got %+v", context, cells)
	}
}

func assertPveVerticalQuestState(t *testing.T, runtime *gameRuntime, want queststate.Snapshot, context string) {
	t.Helper()
	loaded, err := runtime.questStateStore.Load()
	if err != nil {
		t.Fatalf("load quest-state after %s: %v", context, err)
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected quest-state after %s:\n got: %#v\nwant: %#v", context, loaded, want)
	}
}

func assertPveVerticalConnectedPosition(t *testing.T, runtime *gameRuntime, mapIndex uint32, x int32, y int32, context string) {
	t.Helper()
	connected := runtime.ConnectedCharacters()
	if len(connected) != 1 || connected[0].MapIndex != mapIndex || connected[0].X != x || connected[0].Y != y {
		t.Fatalf("expected %s connected position map=%d x=%d y=%d, got %+v", context, mapIndex, x, y, connected)
	}
}

func assertPveVerticalPersistedPosition(t *testing.T, accounts *accountstore.FileStore, login string, mapIndex uint32, x int32, y int32, context string) {
	t.Helper()
	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted PvE vertical account after %s: %v", context, err)
	}
	if len(account.Characters) != 1 || account.Characters[0].MapIndex != mapIndex || account.Characters[0].X != x || account.Characters[0].Y != y {
		t.Fatalf("expected persisted %s position map=%d x=%d y=%d, got %+v", context, mapIndex, x, y, account.Characters)
	}
}

func decodePveVerticalAuthoredKillDrop(t *testing.T, frames [][]byte, killer loginticket.Character, context string) itemproto.GroundAddPacket {
	t.Helper()
	for i := 0; i+1 < len(frames); i++ {
		ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, frames[i]))
		if err != nil {
			continue
		}
		ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, frames[i+1]))
		if err != nil {
			t.Fatalf("expected %s GROUND_ADD to be followed by OWNERSHIP: %+v err=%v", context, ground, err)
		}
		if ground.VID == 0 || ground.Vnum != 27001 || ground.X != killer.X || ground.Y != killer.Y || ground.Z != killer.Z {
			t.Fatalf("unexpected %s kill ground add: %+v", context, ground)
		}
		if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: killer.Name}) {
			t.Fatalf("unexpected %s kill ownership: %+v", context, ownership)
		}
		return ground
	}
	t.Fatalf("expected %s killing hit to include GROUND_ADD + OWNERSHIP for vnum 27001, got %d frames", context, len(frames))
	return itemproto.GroundAddPacket{}
}

func pickupPveVerticalAuthoredDrop(t *testing.T, flow service.SessionFlow, groundVID uint32, context string) {
	t.Helper()
	pickupOut := pickupGroundItem(t, flow, groundVID)
	if len(pickupOut) != 3 {
		t.Fatalf("expected %s drop pickup to emit GROUND_DEL, ITEM_SET, and ITEM_GET, got %d", context, len(pickupOut))
	}
	if del, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, pickupOut[0])); err != nil || del.VID != groundVID {
		t.Fatalf("unexpected %s pickup ground delete: del=%+v err=%v", context, del, err)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, pickupOut[1]))
	if err != nil || set.Position != itemproto.InventoryPosition(0) || set.Vnum != 27001 || set.Count != 1 {
		t.Fatalf("unexpected %s pickup item set: %+v err=%v", context, set, err)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, pickupOut[2]))
	if err != nil || get.Vnum != 27001 || get.Count != 1 {
		t.Fatalf("unexpected %s pickup item get: %+v err=%v", context, get, err)
	}
}

func assertPveVerticalFormulaFirstHitFrames(t *testing.T, frames [][]byte, mobVID uint32, ownerVID uint32, wantMobDamage int32, context string) {
	t.Helper()
	const pveVerticalMobRetaliationDelta = int32(-2)
	if len(frames) != 4 {
		t.Fatalf("expected target refresh, retaliation, and damage-info on %s formula first hit, got %d frames", context, len(frames))
	}
	refresh, err := combatproto.DecodeServerTarget(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode %s formula first-hit target refresh: %v", context, err)
	}
	if refresh.TargetVID != mobVID || refresh.HPPercent != 75 {
		t.Fatalf("expected %s formula first hit to reach 75%% HP (20->15), got %+v", context, refresh)
	}
	retaliation, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode %s formula first-hit retaliation point-change: %v", context, err)
	}
	if retaliation.VID != ownerVID || retaliation.Type != bootstrapPlayerPointType || retaliation.Amount != pveVerticalMobRetaliationDelta {
		t.Fatalf("unexpected %s formula first-hit retaliation point-change: %+v", context, retaliation)
	}
	assertDamageInfoFrame(t, frames[2], mobVID, wantMobDamage, context+" mob damage-info")
	assertDamageInfoFrame(t, frames[3], ownerVID, -pveVerticalMobRetaliationDelta, context+" owner retaliation damage-info")
}

func assertPveVerticalEXPGoldOnlyKillReward(
	t *testing.T,
	frames [][]byte,
	killer loginticket.Character,
	targetVID uint32,
	wantDamage int32,
	wantExperienceAmount int32,
	wantExperienceValue int32,
	wantGoldAmount int32,
	wantGoldValue uint64,
	context string,
) {
	t.Helper()
	remaining := stripKillingHitDeathPrefix(t, frames, targetVID, wantDamage, context)
	if len(remaining) != 2 {
		t.Fatalf("expected %s killing hit to emit EXP then gold after death/clear, got %d remaining frames", context, len(remaining))
	}
	experienceChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, remaining[0]))
	if err != nil {
		t.Fatalf("decode %s experience point-change: %v", context, err)
	}
	if experienceChange.VID != killer.VID || experienceChange.Type != bootstrapExperiencePointType || experienceChange.Amount != wantExperienceAmount || experienceChange.Value != wantExperienceValue {
		t.Fatalf("unexpected %s experience point-change: %+v want amount=%d value=%d", context, experienceChange, wantExperienceAmount, wantExperienceValue)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, remaining[1]))
	if err != nil {
		t.Fatalf("decode %s gold point-change: %v", context, err)
	}
	if goldChange.VID != killer.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != wantGoldAmount || uint64(goldChange.Value) != wantGoldValue {
		t.Fatalf("unexpected %s gold point-change: %+v want amount=%d value=%d", context, goldChange, wantGoldAmount, wantGoldValue)
	}
	for _, frame := range frames {
		if chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame)); err == nil {
			t.Fatalf("expected no quest chat on %s EXP/gold-only pack kill, got %+v", context, chat)
		}
		if _, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, frame)); err == nil {
			t.Fatalf("expected no GROUND_ADD on %s EXP/gold-only pack kill", context)
		}
	}
}

func assertPveVerticalPackRespawn(t *testing.T, frames [][]byte, packVID uint32) {
	t.Helper()
	if len(frames) != 4 {
		t.Fatalf("expected warp-tile pack 2 respawn to emit delete + add/info/update, got %d frames", len(frames))
	}
	deleted, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode warp-tile pack 2 respawn delete: %v", err)
	}
	if deleted.VID != packVID {
		t.Fatalf("unexpected warp-tile pack 2 respawn delete: %+v", deleted)
	}
	added, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode warp-tile pack 2 respawn add: %v", err)
	}
	if added.VID != packVID || added.X != 470000 || added.Y != 964200 || added.RaceNum != 20350 {
		t.Fatalf("unexpected warp-tile pack 2 respawn add: %+v", added)
	}
	info, err := worldproto.DecodeCharacterAdditionalInfo(decodeSingleFrame(t, frames[2]))
	if err != nil {
		t.Fatalf("decode warp-tile pack 2 respawn additional info: %v", err)
	}
	if info.VID != packVID || info.Name != "QAPveVerticalPack 2" {
		t.Fatalf("unexpected warp-tile pack 2 respawn additional info: %+v", info)
	}
	updated, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, frames[3]))
	if err != nil {
		t.Fatalf("decode warp-tile pack 2 respawn update: %v", err)
	}
	if updated.VID != packVID {
		t.Fatalf("unexpected warp-tile pack 2 respawn update: %+v", updated)
	}
}

func assertPveVerticalAuthoredUseAndEquipTemplates(t *testing.T, templates []itemcatalog.Template, context string) {
	t.Helper()
	byVnum := make(map[uint32]itemcatalog.Template, len(templates))
	for _, template := range templates {
		byVnum[template.Vnum] = template
	}
	sword, ok := byVnum[11200]
	wantSwordEffect := &itemcatalog.PointEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 10}
	if !ok || sword.Name != "Wooden Sword" || sword.Stackable || sword.MaxCount != 1 || sword.ShopSellPrice != 100 || sword.EquipSlot != inventory.EquipmentSlotWeapon.String() || sword.AppearanceVnum != 11201 || sword.UseEffect != nil || !reflect.DeepEqual(sword.EquipEffect, wantSwordEffect) {
		t.Fatalf("expected %s 11200 to author weapon equip_slot + appearance_vnum + equip_effect without use_effect, got %+v", context, sword)
	}
	potion, ok := byVnum[27001]
	if !ok || potion.Name != "Small Red Potion" || !potion.Stackable || potion.MaxCount != 200 || potion.ShopBuyPrice != 5 || potion.ShopSellPrice != 2 || potion.EquipSlot != "" || potion.UseEffect == nil {
		t.Fatalf("expected %s 27001 to author use_effect without equip_slot, got %+v", context, potion)
	}
	wantEffect := &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50", SpecialEffectType: effectproto.SpecialEffectHPUpRed}
	if !reflect.DeepEqual(potion.UseEffect, wantEffect) {
		t.Fatalf("unexpected %s 27001 use_effect: got %+v want %+v", context, potion.UseEffect, wantEffect)
	}
	material, ok := byVnum[27002]
	if !ok || material.Name != "Small Blue Potion" || !material.Stackable || material.MaxCount != 200 || material.ShopBuyPrice != 10 {
		t.Fatalf("expected %s 27002 to author cube material template with shop_buy_price 10, got %+v", context, material)
	}
}

func TestPveVerticalTemplateBackedUseAndEquipFailClosedWithoutAuthoredMetadata(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	hero := peerVisibilityCharacter("PveVerticalHero", 0x01030161, 0x02040161, 469500, 964200, 0, 101, 201)
	hero.Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 11200, Count: 1, Slot: 0},
		{ID: 12, Vnum: 27001, Count: 1, Slot: 1},
	}
	issuePeerTicket(t, ticketStore, "pve-vertical-fail-closed", 0x61616161, hero)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "pve-vertical-fail-closed", Empire: hero.Empire, Characters: []loginticket.Character{hero}}); err != nil {
		t.Fatalf("seed PvE vertical fail-closed account: %v", err)
	}
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{
		{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, ShopSellPrice: 100},
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5, ShopSellPrice: 2},
	}}); err != nil {
		t.Fatalf("seed shop-only PvE templates: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(
		config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
		ticketStore,
		accounts,
		staticstore.NewMemoryStore(),
		interactionstore.NewMemoryStore(),
		itemStore,
		queststate.NewMemoryStore(),
		nil,
	)
	if err != nil {
		t.Fatalf("new PvE vertical fail-closed runtime: %v", err)
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pve-vertical-fail-closed", 0x61616161)
	defer closeSessionFlow(t, flow)

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(1)})))
	if err != nil {
		t.Fatalf("unexpected missing-use_effect ITEM_USE: %v", err)
	}
	if len(useOut) != 0 {
		t.Fatalf("expected missing-use_effect ITEM_USE to emit no frames, got %d", len(useOut))
	}
	weaponPosition, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build weapon equipment position: %v", err)
	}
	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(0),
		Destination: weaponPosition,
	})))
	if err != nil {
		t.Fatalf("unexpected missing-equip_slot ITEM_MOVE: %v", err)
	}
	if len(equipOut) != 0 {
		t.Fatalf("expected missing-equip_slot ITEM_MOVE to emit no frames, got %d", len(equipOut))
	}

	pointsSnapshot, ok := runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != hero.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected missing-use_effect ITEM_USE to leave HP unchanged, got ok=%v snapshot=%+v", ok, pointsSnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 2 || inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[1].Vnum != 27001 {
		t.Fatalf("expected fail-closed use/equip to leave live inventory unchanged, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	equipmentSnapshot, ok := runtime.EquipmentSnapshot(hero.Name)
	if !ok || len(equipmentSnapshot.Equipment) != 0 {
		t.Fatalf("expected fail-closed weapon equip to leave equipment empty, got ok=%v snapshot=%+v", ok, equipmentSnapshot)
	}
	account, err := accounts.Load("pve-vertical-fail-closed")
	if err != nil {
		t.Fatalf("load persisted PvE vertical fail-closed account: %v", err)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != hero.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected persisted HP unchanged after fail-closed use/equip, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, hero.Inventory) {
		t.Fatalf("expected persisted inventory unchanged after fail-closed use/equip, got %+v want %+v", account.Characters[0].Inventory, hero.Inventory)
	}
	if len(account.Characters[0].Equipment) != 0 {
		t.Fatalf("expected persisted equipment empty after fail-closed weapon equip, got %+v", account.Characters[0].Equipment)
	}
}
