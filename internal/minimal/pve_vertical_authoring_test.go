package minimal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
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
	hero.Inventory = []inventory.ItemInstance{{ID: 9001, Vnum: 27001, Count: 1, Slot: 0}}
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
	imported, err := runtime.ImportContentBundle(authored)
	if err != nil {
		t.Fatalf("import PvE vertical authoring bundle: %v", err)
	}
	if len(imported.RegenSpawns) != 0 || len(imported.DropTables) != 0 {
		t.Fatalf("expected import to canonicalize away authoring-only regen/drop collections, got regen=%+v drop_tables=%+v", imported.RegenSpawns, imported.DropTables)
	}
	if len(imported.SpawnGroups) != 1 || imported.SpawnGroups[0].Ref != "practice.qa_pve_vertical_mob" {
		t.Fatalf("expected imported spawn group practice.qa_pve_vertical_mob, got %+v", imported.SpawnGroups)
	}
	if imported.SpawnGroups[0].CombatProfile != "qa_pve_vertical_practice_mob" {
		t.Fatalf("expected imported PvE vertical mob to use formula combat profile, got %+v", imported.SpawnGroups[0])
	}
	if len(imported.CombatProfiles) != 1 || imported.CombatProfiles[0].Profile != "qa_pve_vertical_practice_mob" || imported.CombatProfiles[0].MaxHP != pveVerticalMobMaxHP || imported.CombatProfiles[0].DamagePerNormalAttack != 5 || imported.CombatProfiles[0].AggroRadius != 150 || imported.CombatProfiles[0].LeashRadius != 350 {
		t.Fatalf("expected imported portable formula combat profile max_hp=20 damage=5 aggro_radius=150 leash_radius=350, got %+v", imported.CombatProfiles)
	}

	var guideVID, hunterVID, merchantVID, warehouseVID, mobVID uint32
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "QuestGuide":
			guideVID = uint32(actor.EntityID)
		case "QuestHunter":
			hunterVID = uint32(actor.EntityID)
		case "Merchant":
			merchantVID = uint32(actor.EntityID)
		case "Warehouse":
			warehouseVID = uint32(actor.EntityID)
		case "QAPveVerticalMob":
			mobVID = uint32(actor.EntityID)
		}
	}
	if guideVID == 0 || hunterVID == 0 || merchantVID == 0 || warehouseVID == 0 || mobVID == 0 {
		t.Fatalf("expected guide/hunter/merchant/warehouse/mob actors after import, got %+v", runtime.StaticActors())
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pve-vertical", 0x60606060)

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
	for _, frame := range preGuideKillOut {
		if chat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame)); err == nil {
			t.Fatalf("expected no quest chat before guide unlock, got %+v", chat)
		}
	}
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
	shopOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected unlocked merchant interaction error: %v", err)
	}
	if len(shopOut) != 1 {
		t.Fatalf("expected 1 merchant shop-open frame after guide unlock, got %d", len(shopOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, shopOut[0])); err != nil {
		t.Fatalf("decode unlocked merchant shop start: %v", err)
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

	closeSessionFlow(t, flow)
	flow, _ = enterGameWithLoginTicket(t, runtime.SessionFactory(), "pve-vertical", 0x60606060)
	defer closeSessionFlow(t, flow)

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
	killChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, postGuideKillOut[len(postGuideKillOut)-1]))
	if err != nil || killChat.Message != "Quest updated: first_steps.killed_qa_mob = 1." {
		t.Fatalf("unexpected post-guide kill quest chat: %+v err=%v", killChat, err)
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
	pointsSnapshot, ok := runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapExperiencePointType] != wantExperienceAfter {
		t.Fatalf("expected live experience %d after QuestHunter turn-in, got ok=%v snapshot=%+v", wantExperienceAfter, ok, pointsSnapshot)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(hero.Name)
	if !ok || len(inventorySnapshot.Inventory) != 1 || inventorySnapshot.Inventory[0].Vnum != 11200 || inventorySnapshot.Inventory[0].Count != 1 || inventorySnapshot.Inventory[0].Slot != 0 {
		t.Fatalf("expected live inventory after QuestHunter consume+reward turn-in, got ok=%v snapshot=%+v", ok, inventorySnapshot)
	}
	account, err := accounts.Load("pve-vertical")
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
}

func assertPveVerticalFormulaFirstHitFrames(t *testing.T, frames [][]byte, mobVID uint32, ownerVID uint32, wantMobDamage int32, context string) {
	t.Helper()
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
	if retaliation.VID != ownerVID || retaliation.Type != bootstrapPlayerPointType || retaliation.Amount != bootstrapPracticeMobRetaliationPointDelta {
		t.Fatalf("unexpected %s formula first-hit retaliation point-change: %+v", context, retaliation)
	}
	assertDamageInfoFrame(t, frames[2], mobVID, wantMobDamage, context+" mob damage-info")
	assertDamageInfoFrame(t, frames[3], ownerVID, -bootstrapPracticeMobRetaliationPointDelta, context+" owner retaliation damage-info")
}
