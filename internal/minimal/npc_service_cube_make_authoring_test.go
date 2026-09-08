package minimal

import (
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/cubestore"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	effectproto "github.com/MikelCalvo/go-metin2-server/internal/proto/effect"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

func TestNpcServiceBundleCubeMasterAddMakeConsumesGrantsAndPersists(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	hero := peerVisibilityCharacter("CubeCraftHero", 0x01030170, 0x02040170, 469575, 964200, 0, 101, 201)
	hero.Gold = 4242
	login := "npc-cube-craft"
	issuePeerTicket(t, ticketStore, login, 0x70707170, hero)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: login, Empire: hero.Empire, Characters: cloneCharacters([]loginticket.Character{hero})}); err != nil {
		t.Fatalf("seed NPC-service cube-craft account: %v", err)
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
		t.Fatalf("new NPC-service cube-craft runtime: %v", err)
	}
	currentTime := time.Unix(1_700_002_000, 0)
	runtime.now = func() time.Time { return currentTime }

	authored := loadBootstrapNpcServiceKillQuestCreditBundle(t, "bootstrap-npc-service-bundle.json")
	imported, err := runtime.ImportContentBundle(authored)
	if err != nil {
		t.Fatalf("import NPC service example bundle: %v", err)
	}
	if !reflect.DeepEqual(imported.CubeRecipes, cubestore.BootstrapSnapshot().NPCs) {
		t.Fatalf("unexpected imported NPC-service cube recipes: %#v", imported.CubeRecipes)
	}

	var guideVID, merchantVID, cubeVID uint32
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "QuestGuide":
			guideVID = uint32(actor.EntityID)
		case "Merchant":
			merchantVID = uint32(actor.EntityID)
		case "CubeMaster":
			cubeVID = uint32(actor.EntityID)
		}
	}
	if guideVID == 0 || merchantVID == 0 || cubeVID == 0 {
		t.Fatalf("expected imported QuestGuide, Merchant, and CubeMaster actors, got guide=%d merchant=%d cube=%d", guideVID, merchantVID, cubeVID)
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707170)
	defer closeSessionFlow(t, flow)

	guideOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: guideVID})))
	if err != nil {
		t.Fatalf("unexpected QuestGuide interaction error: %v", err)
	}
	if len(guideOut) != 1 {
		t.Fatalf("expected 1 QuestGuide quest-flag frame, got %d", len(guideOut))
	}
	guideChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, guideOut[0]))
	if err != nil || guideChat.Type != chatproto.ChatTypeInfo || guideChat.Message != "Quest updated: first_steps.met_guide = 1." {
		t.Fatalf("unexpected QuestGuide chat: %+v err=%v", guideChat, err)
	}
	flag, ok, err := runtime.QuestStateFlag(hero.Name, "quest:first_steps", "met_guide")
	if err != nil || !ok || flag.Value != 1 {
		t.Fatalf("expected met_guide=1 after QuestGuide, got ok=%v flag=%+v err=%v", ok, flag, err)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	shopOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: merchantVID})))
	if err != nil {
		t.Fatalf("unexpected Merchant interaction error: %v", err)
	}
	if len(shopOut) != 1 {
		t.Fatalf("expected 1 merchant shop-open frame after QuestGuide unlock, got %d", len(shopOut))
	}
	start, err := shopproto.DecodeServerStart(decodeSingleFrame(t, shopOut[0]))
	if err != nil {
		t.Fatalf("decode NPC-service merchant shop start: %v", err)
	}
	if start.OwnerVID != merchantVID {
		t.Fatalf("unexpected merchant shop owner VID: got %d want %d", start.OwnerVID, merchantVID)
	}
	if start.Items[0].Vnum != 27001 || start.Items[0].Price != 50 || start.Items[0].Count != 1 || start.Items[0].DisplayPos != 0 {
		t.Fatalf("unexpected merchant catalog slot 0: %+v", start.Items[0])
	}
	if start.Items[1].Vnum != 11200 || start.Items[1].Price != 500 || start.Items[1].Count != 1 || start.Items[1].DisplayPos != 1 {
		t.Fatalf("unexpected merchant catalog slot 1: %+v", start.Items[1])
	}
	if start.Items[2].Vnum != 27002 || start.Items[2].Price != 20 || start.Items[2].Count != 2 || start.Items[2].DisplayPos != 2 {
		t.Fatalf("unexpected merchant catalog slot 2: %+v", start.Items[2])
	}
	assertNpcServiceCubeCraftUnchanged(t, runtime, accounts, login, hero, 4242, nil, "after merchant open")

	const npcServiceCubeMaterialPrice = uint64(20)
	wantGoldAfterBuy := hero.Gold - npcServiceCubeMaterialPrice
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 2})))
	if err != nil {
		t.Fatalf("unexpected NPC-service cube-material merchant buy error: %v", err)
	}
	if len(buyOut) != 1 {
		t.Fatalf("expected 1 item refresh frame for catalog slot 2 merchant buy, got %d", len(buyOut))
	}
	boughtSet, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode NPC-service cube-material merchant buy item set: %v", err)
	}
	if boughtSet.Position != itemproto.InventoryPosition(0) || boughtSet.Vnum != 27002 || boughtSet.Count != 2 {
		t.Fatalf("unexpected NPC-service cube-material merchant buy item set: %+v", boughtSet)
	}
	liveInventory, ok := runtime.InventorySnapshot(hero.Name)
	if !ok || len(liveInventory.Inventory) != 1 {
		t.Fatalf("expected live inventory one 27002 stack after merchant buy, got ok=%v snapshot=%+v", ok, liveInventory)
	}
	bought := liveInventory.Inventory[0]
	if bought.Vnum != 27002 || bought.Count != 2 || bought.Slot != 0 || bought.ID == 0 {
		t.Fatalf("unexpected live cube-material inventory after merchant buy: %+v", bought)
	}
	assertNpcServiceCubeCraftUnchanged(t, runtime, accounts, login, hero, wantGoldAfterBuy, []inventory.ItemInstance{
		{ID: bought.ID, Vnum: 27002, Count: 2, Slot: inventory.SlotIndex(0)},
	}, "after cube-material merchant buy")

	closeShopOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected NPC-service merchant close error: %v", err)
	}
	if len(closeShopOut) != 1 {
		t.Fatalf("expected 1 merchant close frame after cube-material buy, got %d", len(closeShopOut))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, closeShopOut[0])); err != nil {
		t.Fatalf("decode NPC-service merchant shop end: %v", err)
	}

	currentTime = currentTime.Add(staticActorInteractionCooldown)
	cubeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: cubeVID})))
	if err != nil {
		t.Fatalf("unexpected CubeMaster interaction error: %v", err)
	}
	if len(cubeOut) != 2 {
		t.Fatalf("expected chat + cube open frames after QuestGuide unlock, got %d", len(cubeOut))
	}
	cubeChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, cubeOut[0]))
	if err != nil || cubeChat.Type != chatproto.ChatTypeInfo || cubeChat.VID != 0 || cubeChat.Empire != 0 || cubeChat.Message != "The craftsman lights the forge." {
		t.Fatalf("unexpected CubeMaster chat: %+v err=%v", cubeChat, err)
	}
	assertCubeCommandChatFrame(t, cubeOut[1], "cube open 20022", "npc-service CubeMaster open")

	addOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/cube add 0 0",
	})))
	if err != nil {
		t.Fatalf("unexpected /cube add 0 0 after authored CubeMaster open: %v", err)
	}
	if len(addOut) != 1 {
		t.Fatalf("expected /cube add 0 0 to emit one command chat frame, got %d", len(addOut))
	}
	assertCubeCommandChatFrame(t, addOut[0], "cube info 100 0 0", "npc-service matched stacked cube add")
	assertNpcServiceCubeCraftUnchanged(t, runtime, accounts, login, hero, wantGoldAfterBuy, []inventory.ItemInstance{
		{ID: bought.ID, Vnum: 27002, Count: 2, Slot: inventory.SlotIndex(0)},
	}, "after stacked cube add")

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
	delA, err := itemproto.DecodeDel(decodeSingleFrame(t, makeOut[0]))
	if err != nil {
		t.Fatalf("decode material ITEM_DEL: %v", err)
	}
	if delA.Position.WindowType != itemproto.WindowInventory || delA.Position.Cell != 0 {
		t.Fatalf("unexpected material delete position: %+v", delA.Position)
	}
	rewardSet, err := itemproto.DecodeSet(decodeSingleFrame(t, makeOut[1]))
	if err != nil {
		t.Fatalf("decode reward ITEM_SET: %v", err)
	}
	if rewardSet.Position.WindowType != itemproto.WindowInventory || rewardSet.Vnum != 27001 || rewardSet.Count != 1 {
		t.Fatalf("unexpected reward ITEM_SET: %+v", rewardSet)
	}
	goldChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, makeOut[2]))
	if err != nil {
		t.Fatalf("decode gold PLAYER_POINT_CHANGE: %v", err)
	}
	wantGoldAfterMake := wantGoldAfterBuy - 100
	if goldChange.VID != hero.VID || goldChange.Type != bootstrapGoldPointType || goldChange.Amount != -100 || goldChange.Value != int32(wantGoldAfterMake) {
		t.Fatalf("unexpected gold point change: %+v", goldChange)
	}
	assertCubeCommandChatFrame(t, makeOut[3], cubestore.FormatCubeSuccessCommand(27001, 1), "npc-service cube make success")
	assertCubeCommandChatFrame(t, makeOut[4], "cube info 0 0 0", "npc-service post-make cube info")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected authored CubeMaster /cube make to queue no peer frames, got %d", len(queued))
	}

	liveGold, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok || liveGold.Gold != wantGoldAfterMake {
		t.Fatalf("expected live gold %d after authored cube make, got ok=%v snapshot=%+v", wantGoldAfterMake, ok, liveGold)
	}
	liveInventory, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(liveInventory.Inventory) != 1 || liveInventory.Inventory[0].Vnum != 27001 || liveInventory.Inventory[0].Count != 1 {
		t.Fatalf("expected live inventory one 27001 after authored cube make, got ok=%v snapshot=%+v", ok, liveInventory)
	}
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted NPC-service cube-craft account: %v", err)
	}
	if persisted.Characters[0].Gold != wantGoldAfterMake {
		t.Fatalf("unexpected persisted gold after authored cube make: %d", persisted.Characters[0].Gold)
	}
	if len(persisted.Characters[0].Inventory) != 1 || persisted.Characters[0].Inventory[0].Vnum != 27001 || persisted.Characters[0].Inventory[0].Count != 1 {
		t.Fatalf("unexpected persisted inventory after authored cube make: %+v", persisted.Characters[0].Inventory)
	}
	flag, ok, err = runtime.QuestStateFlag(hero.Name, "quest:first_steps", "met_guide")
	if err != nil || !ok || flag.Value != 1 {
		t.Fatalf("expected met_guide=1 after authored cube make, got ok=%v flag=%+v err=%v", ok, flag, err)
	}

	beforeCraftedUsePoints, ok := runtime.PointsSnapshot(hero.Name)
	if !ok {
		t.Fatal("expected points snapshot before NPC-service cube-granted potion ITEM_USE")
	}
	wantHPAfterCraftedUse := beforeCraftedUsePoints.Points[bootstrapPlayerPointValueIndex] + 50
	wantPersistedHPAfterCraftedUse := persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] + 50

	assertCloseCubeCommandChat(t, flow, "/close_cube", "npc-service cube close after authored make")
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
	assertNpcServiceCubeCraftUnchanged(t, runtime, accounts, login, hero, wantGoldAfterMake, []inventory.ItemInstance{
		{ID: liveInventory.Inventory[0].ID, Vnum: 27001, Count: 1, Slot: inventory.SlotIndex(0)},
	}, "after authored cube close")
	pointsSnapshot, ok := runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != beforeCraftedUsePoints.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected live HP unchanged after authored cube close, got ok=%v snapshot=%+v", ok, pointsSnapshot)
	}

	craftedUseOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(0)})))
	if err != nil {
		t.Fatalf("unexpected NPC-service cube-granted potion ITEM_USE: %v", err)
	}
	if len(craftedUseOut) != 5 {
		t.Fatalf("expected ITEM_USE echo, point-change, ITEM_DEL, SPECIAL_EFFECT, and info chat for cube-granted last-stack potion, got %d", len(craftedUseOut))
	}
	craftedUseEcho, err := itemproto.DecodeUse(decodeSingleFrame(t, craftedUseOut[0]))
	if err != nil {
		t.Fatalf("decode NPC-service cube-granted potion ITEM_USE echo: %v", err)
	}
	if craftedUseEcho.Position != itemproto.InventoryPosition(0) || craftedUseEcho.CharacterVID != hero.VID || craftedUseEcho.VictimVID != hero.VID || craftedUseEcho.Vnum != 27001 {
		t.Fatalf("unexpected NPC-service cube-granted potion ITEM_USE echo: %+v", craftedUseEcho)
	}
	craftedUsePoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, craftedUseOut[1]))
	if err != nil {
		t.Fatalf("decode NPC-service cube-granted potion point-change: %v", err)
	}
	if craftedUsePoint.VID != hero.VID || craftedUsePoint.Type != bootstrapPlayerPointType || craftedUsePoint.Amount != 50 || craftedUsePoint.Value != wantHPAfterCraftedUse {
		t.Fatalf("unexpected NPC-service cube-granted potion point-change: %+v want value=%d", craftedUsePoint, wantHPAfterCraftedUse)
	}
	craftedUseDel, err := itemproto.DecodeDel(decodeSingleFrame(t, craftedUseOut[2]))
	if err != nil {
		t.Fatalf("decode NPC-service cube-granted potion ITEM_DEL: %v", err)
	}
	if craftedUseDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected NPC-service cube-granted potion ITEM_DEL: %+v", craftedUseDel)
	}
	craftedUseEffect, err := effectproto.DecodeSpecial(decodeSingleFrame(t, craftedUseOut[3]))
	if err != nil {
		t.Fatalf("decode NPC-service cube-granted potion SPECIAL_EFFECT: %v", err)
	}
	if craftedUseEffect.Type != effectproto.SpecialEffectHPUpRed || craftedUseEffect.VID != hero.VID {
		t.Fatalf("unexpected NPC-service cube-granted potion SPECIAL_EFFECT: %+v", craftedUseEffect)
	}
	assertCubeInfoChatFrame(t, craftedUseOut[4], "consume:27001:+50", "npc-service cube-granted potion ITEM_USE")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected NPC-service cube-granted potion ITEM_USE to queue no peer frames, got %d", len(queued))
	}
	pointsSnapshot, ok = runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != wantHPAfterCraftedUse {
		t.Fatalf("expected live HP %d after NPC-service cube-granted potion ITEM_USE, got ok=%v snapshot=%+v", wantHPAfterCraftedUse, ok, pointsSnapshot)
	}
	assertNpcServiceCubeCraftUnchanged(t, runtime, accounts, login, hero, wantGoldAfterMake, nil, "after cube-granted potion ITEM_USE")
	persisted, err = accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted NPC-service cube-craft account after cube-granted potion ITEM_USE: %v", err)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantPersistedHPAfterCraftedUse {
		t.Fatalf("expected persisted HP %d after NPC-service cube-granted potion ITEM_USE (live=%d), got %d", wantPersistedHPAfterCraftedUse, wantHPAfterCraftedUse, persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	flag, ok, err = runtime.QuestStateFlag(hero.Name, "quest:first_steps", "met_guide")
	if err != nil || !ok || flag.Value != 1 {
		t.Fatalf("expected met_guide=1 after NPC-service cube-granted potion ITEM_USE, got ok=%v flag=%+v err=%v", ok, flag, err)
	}
}

func assertNpcServiceCubeCraftUnchanged(t *testing.T, runtime *gameRuntime, accounts accountstore.Store, login string, hero loginticket.Character, wantGold uint64, wantInventory []inventory.ItemInstance, label string) {
	t.Helper()
	liveGold, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok || liveGold.Gold != wantGold {
		t.Fatalf("expected live gold %d %s, got ok=%v snapshot=%+v", wantGold, label, ok, liveGold)
	}
	liveInventory, ok := runtime.InventorySnapshot(hero.Name)
	if !ok || len(liveInventory.Inventory) != len(wantInventory) {
		t.Fatalf("expected live inventory %d items %s, got ok=%v snapshot=%+v", len(wantInventory), label, ok, liveInventory)
	}
	for i, want := range wantInventory {
		got := liveInventory.Inventory[i]
		if got.ID != want.ID || got.Vnum != want.Vnum || got.Count != want.Count || got.Slot != uint16(want.Slot) {
			t.Fatalf("unexpected live inventory[%d] %s: %+v want %+v", i, label, got, want)
		}
	}
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted NPC-service cube-craft account %s: %v", label, err)
	}
	if persisted.Characters[0].Gold != wantGold {
		t.Fatalf("unexpected persisted gold %s: %d want %d", label, persisted.Characters[0].Gold, wantGold)
	}
	if len(wantInventory) == 0 {
		if len(persisted.Characters[0].Inventory) != 0 {
			t.Fatalf("unexpected persisted inventory %s: %+v want empty", label, persisted.Characters[0].Inventory)
		}
		return
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory %s:\n got: %+v\nwant: %+v", label, persisted.Characters[0].Inventory, wantInventory)
	}
}
