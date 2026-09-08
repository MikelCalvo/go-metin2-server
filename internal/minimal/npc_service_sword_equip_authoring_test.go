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
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

func TestNpcServiceBundleMerchantSwordBuyEquipsAppearanceAndEquipEffect(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	hero := peerVisibilityCharacter("SwordEquipHero", 0x01030171, 0x02040171, 469575, 964200, 0, 101, 201)
	hero.Gold = 4242
	login := "npc-sword-equip"
	issuePeerTicket(t, ticketStore, login, 0x70707171, hero)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: login, Empire: hero.Empire, Characters: cloneCharacters([]loginticket.Character{hero})}); err != nil {
		t.Fatalf("seed NPC-service sword-equip account: %v", err)
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
		t.Fatalf("new NPC-service sword-equip runtime: %v", err)
	}
	currentTime := time.Unix(1_700_002_100, 0)
	runtime.now = func() time.Time { return currentTime }

	authored := loadBootstrapNpcServiceKillQuestCreditBundle(t, "bootstrap-npc-service-bundle.json")
	imported, err := runtime.ImportContentBundle(authored)
	if err != nil {
		t.Fatalf("import NPC service example bundle: %v", err)
	}
	byVnum := make(map[uint32]itemcatalog.Template, len(imported.ItemTemplates))
	for _, template := range imported.ItemTemplates {
		byVnum[template.Vnum] = template
	}
	wantSwordEffect := &itemcatalog.PointEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 10}
	if byVnum[11200].EquipSlot != inventory.EquipmentSlotWeapon.String() || byVnum[11200].AppearanceVnum != 11201 || byVnum[11200].UseEffect != nil || !reflect.DeepEqual(byVnum[11200].EquipEffect, wantSwordEffect) {
		t.Fatalf("expected imported NPC service 11200 to author weapon equip_slot + appearance_vnum + equip_effect without use_effect, got %+v", byVnum[11200])
	}

	var guideVID, merchantVID uint32
	for _, actor := range runtime.StaticActors() {
		switch actor.Name {
		case "QuestGuide":
			guideVID = uint32(actor.EntityID)
		case "Merchant":
			merchantVID = uint32(actor.EntityID)
		}
	}
	if guideVID == 0 || merchantVID == 0 {
		t.Fatalf("expected imported QuestGuide and Merchant actors, got guide=%d merchant=%d", guideVID, merchantVID)
	}

	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707171)
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
	if start.Items[1].Vnum != 11200 || start.Items[1].Price != 500 || start.Items[1].Count != 1 || start.Items[1].DisplayPos != 1 {
		t.Fatalf("unexpected merchant catalog slot 1: %+v", start.Items[1])
	}

	const npcServiceSwordPrice = uint64(500)
	wantGoldAfterBuy := hero.Gold - npcServiceSwordPrice
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 1})))
	if err != nil {
		t.Fatalf("unexpected NPC-service sword merchant buy error: %v", err)
	}
	if len(buyOut) != 1 {
		t.Fatalf("expected 1 item refresh frame for catalog slot 1 merchant buy, got %d", len(buyOut))
	}
	boughtSet, err := itemproto.DecodeSet(decodeSingleFrame(t, buyOut[0]))
	if err != nil {
		t.Fatalf("decode NPC-service sword merchant buy item set: %v", err)
	}
	if boughtSet.Position != itemproto.InventoryPosition(0) || boughtSet.Vnum != 11200 || boughtSet.Count != 1 {
		t.Fatalf("unexpected NPC-service sword merchant buy item set: %+v", boughtSet)
	}
	liveInventory, ok := runtime.InventorySnapshot(hero.Name)
	if !ok || len(liveInventory.Inventory) != 1 {
		t.Fatalf("expected live inventory one 11200 after merchant buy, got ok=%v snapshot=%+v", ok, liveInventory)
	}
	bought := liveInventory.Inventory[0]
	if bought.Vnum != 11200 || bought.Count != 1 || bought.Slot != 0 || bought.ID == 0 {
		t.Fatalf("unexpected live sword inventory after merchant buy: %+v", bought)
	}
	liveGold, ok := runtime.CurrencySnapshot(hero.Name)
	if !ok || liveGold.Gold != wantGoldAfterBuy {
		t.Fatalf("expected live gold %d after sword merchant buy, got ok=%v snapshot=%+v", wantGoldAfterBuy, ok, liveGold)
	}
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted NPC-service sword-equip account after merchant buy: %v", err)
	}
	if persisted.Characters[0].Gold != wantGoldAfterBuy {
		t.Fatalf("unexpected persisted gold after sword merchant buy: %d", persisted.Characters[0].Gold)
	}
	if len(persisted.Characters[0].Inventory) != 1 || persisted.Characters[0].Inventory[0].Vnum != 11200 || persisted.Characters[0].Inventory[0].Count != 1 {
		t.Fatalf("unexpected persisted inventory after sword merchant buy: %+v", persisted.Characters[0].Inventory)
	}

	closeShopOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected NPC-service merchant close error: %v", err)
	}
	if len(closeShopOut) != 1 {
		t.Fatalf("expected 1 merchant close frame after sword buy, got %d", len(closeShopOut))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, closeShopOut[0])); err != nil {
		t.Fatalf("decode NPC-service merchant shop end: %v", err)
	}

	beforeEquipPoints, ok := runtime.PointsSnapshot(hero.Name)
	if !ok {
		t.Fatal("expected points snapshot before NPC-service sword equip")
	}
	beforeEquipPersistedHP := persisted.Characters[0].Points[bootstrapPlayerPointValueIndex]
	wantHPAfterEquip := beforeEquipPoints.Points[bootstrapPlayerPointValueIndex] + 10
	wantPersistedHPAfterEquip := beforeEquipPersistedHP + 10

	weaponPosition, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build weapon equipment position: %v", err)
	}
	equipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(0),
		Destination: weaponPosition,
	})))
	if err != nil {
		t.Fatalf("unexpected NPC-service sword ITEM_MOVE equip: %v", err)
	}
	if len(equipOut) != 4 {
		t.Fatalf("expected ITEM_DEL, equipment ITEM_SET, PLAYER_POINT_CHANGE, and CHARACTER_UPDATE for empty-weapon authored equip, got %d", len(equipOut))
	}
	equipDel, err := itemproto.DecodeDel(decodeSingleFrame(t, equipOut[0]))
	if err != nil {
		t.Fatalf("decode NPC-service sword equip ITEM_DEL: %v", err)
	}
	if equipDel.Position != itemproto.InventoryPosition(0) {
		t.Fatalf("unexpected NPC-service sword equip ITEM_DEL: %+v", equipDel)
	}
	equipSet, err := itemproto.DecodeSet(decodeSingleFrame(t, equipOut[1]))
	if err != nil {
		t.Fatalf("decode NPC-service sword equipment ITEM_SET: %v", err)
	}
	if equipSet.Position != weaponPosition || equipSet.Vnum != 11200 || equipSet.Count != 1 {
		t.Fatalf("unexpected NPC-service sword equipment ITEM_SET: %+v", equipSet)
	}
	equipPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, equipOut[2]))
	if err != nil {
		t.Fatalf("decode NPC-service sword equip PLAYER_POINT_CHANGE: %v", err)
	}
	if equipPointChange.VID != hero.VID || equipPointChange.Type != bootstrapPlayerPointType || equipPointChange.Amount != 10 || equipPointChange.Value != wantHPAfterEquip {
		t.Fatalf("unexpected NPC-service sword equip PLAYER_POINT_CHANGE: %+v want value=%d", equipPointChange, wantHPAfterEquip)
	}
	appearance, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, equipOut[3]))
	if err != nil {
		t.Fatalf("decode NPC-service sword CHARACTER_UPDATE: %v", err)
	}
	if appearance.VID != hero.VID || appearance.Parts[0] != hero.MainPart || appearance.Parts[1] != 11201 || appearance.Parts[3] != hero.HairPart {
		t.Fatalf("unexpected NPC-service sword CHARACTER_UPDATE: %+v want vid=%d parts[0]=%d parts[1]=11201 parts[3]=%d", appearance, hero.VID, hero.MainPart, hero.HairPart)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected NPC-service sword equip to queue no peer frames, got %d", len(queued))
	}

	liveInventory, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(liveInventory.Inventory) != 0 {
		t.Fatalf("expected live inventory empty after NPC-service sword equip, got ok=%v snapshot=%+v", ok, liveInventory)
	}
	equipmentSnapshot, ok := runtime.EquipmentSnapshot(hero.Name)
	if !ok || len(equipmentSnapshot.Equipment) != 1 || equipmentSnapshot.Equipment[0].ID != bought.ID || equipmentSnapshot.Equipment[0].Vnum != 11200 || equipmentSnapshot.Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon.String() {
		t.Fatalf("expected live weapon equipment after NPC-service sword equip, got ok=%v snapshot=%+v", ok, equipmentSnapshot)
	}
	pointsSnapshot, ok := runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != wantHPAfterEquip {
		t.Fatalf("expected live HP %d after NPC-service sword equip, got ok=%v snapshot=%+v", wantHPAfterEquip, ok, pointsSnapshot)
	}
	liveGold, ok = runtime.CurrencySnapshot(hero.Name)
	if !ok || liveGold.Gold != wantGoldAfterBuy {
		t.Fatalf("expected live gold %d after NPC-service sword equip, got ok=%v snapshot=%+v", wantGoldAfterBuy, ok, liveGold)
	}
	persisted, err = accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted NPC-service sword-equip account after equip: %v", err)
	}
	if persisted.Characters[0].Gold != wantGoldAfterBuy {
		t.Fatalf("unexpected persisted gold after NPC-service sword equip: %d", persisted.Characters[0].Gold)
	}
	if len(persisted.Characters[0].Inventory) != 0 {
		t.Fatalf("expected persisted inventory empty after NPC-service sword equip, got %+v", persisted.Characters[0].Inventory)
	}
	if len(persisted.Characters[0].Equipment) != 1 || persisted.Characters[0].Equipment[0].ID != bought.ID || persisted.Characters[0].Equipment[0].Vnum != 11200 || persisted.Characters[0].Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon || !persisted.Characters[0].Equipment[0].Equipped {
		t.Fatalf("expected persisted weapon equipment after NPC-service sword equip, got %+v", persisted.Characters[0].Equipment)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantPersistedHPAfterEquip {
		t.Fatalf("expected persisted HP %d after NPC-service sword equip (live=%d), got %d", wantPersistedHPAfterEquip, wantHPAfterEquip, persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	flag, ok, err = runtime.QuestStateFlag(hero.Name, "quest:first_steps", "met_guide")
	if err != nil || !ok || flag.Value != 1 {
		t.Fatalf("expected met_guide=1 after NPC-service sword equip, got ok=%v flag=%+v err=%v", ok, flag, err)
	}

	unequipOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      weaponPosition,
		Destination: itemproto.InventoryPosition(0),
	})))
	if err != nil {
		t.Fatalf("unexpected NPC-service sword ITEM_MOVE unequip: %v", err)
	}
	if len(unequipOut) != 4 {
		t.Fatalf("expected ITEM_DEL, carried ITEM_SET, PLAYER_POINT_CHANGE, and CHARACTER_UPDATE for authored sword unequip, got %d", len(unequipOut))
	}
	unequipDel, err := itemproto.DecodeDel(decodeSingleFrame(t, unequipOut[0]))
	if err != nil {
		t.Fatalf("decode NPC-service sword unequip ITEM_DEL: %v", err)
	}
	if unequipDel.Position != weaponPosition {
		t.Fatalf("unexpected NPC-service sword unequip ITEM_DEL: %+v", unequipDel)
	}
	unequipSet, err := itemproto.DecodeSet(decodeSingleFrame(t, unequipOut[1]))
	if err != nil {
		t.Fatalf("decode NPC-service sword unequip ITEM_SET: %v", err)
	}
	if unequipSet.Position != itemproto.InventoryPosition(0) || unequipSet.Vnum != 11200 || unequipSet.Count != 1 {
		t.Fatalf("unexpected NPC-service sword unequip ITEM_SET: %+v", unequipSet)
	}
	unequipPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, unequipOut[2]))
	if err != nil {
		t.Fatalf("decode NPC-service sword unequip PLAYER_POINT_CHANGE: %v", err)
	}
	if unequipPointChange.VID != hero.VID || unequipPointChange.Type != bootstrapPlayerPointType || unequipPointChange.Amount != -10 || unequipPointChange.Value != beforeEquipPoints.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("unexpected NPC-service sword unequip PLAYER_POINT_CHANGE: %+v want value=%d", unequipPointChange, beforeEquipPoints.Points[bootstrapPlayerPointValueIndex])
	}
	unequipAppearance, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, unequipOut[3]))
	if err != nil {
		t.Fatalf("decode NPC-service sword unequip CHARACTER_UPDATE: %v", err)
	}
	if unequipAppearance.VID != hero.VID || unequipAppearance.Parts[1] != 0 {
		t.Fatalf("unexpected NPC-service sword unequip CHARACTER_UPDATE: %+v", unequipAppearance)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected NPC-service sword unequip to queue no peer frames, got %d", len(queued))
	}

	liveInventory, ok = runtime.InventorySnapshot(hero.Name)
	if !ok || len(liveInventory.Inventory) != 1 || liveInventory.Inventory[0].ID != bought.ID || liveInventory.Inventory[0].Vnum != 11200 || liveInventory.Inventory[0].Slot != 0 {
		t.Fatalf("expected live inventory to hold unequipped sword, got ok=%v snapshot=%+v", ok, liveInventory)
	}
	equipmentSnapshot, ok = runtime.EquipmentSnapshot(hero.Name)
	if !ok || len(equipmentSnapshot.Equipment) != 0 {
		t.Fatalf("expected live equipment empty after NPC-service sword unequip, got ok=%v snapshot=%+v", ok, equipmentSnapshot)
	}
	pointsSnapshot, ok = runtime.PointsSnapshot(hero.Name)
	if !ok || pointsSnapshot.Points[bootstrapPlayerPointValueIndex] != beforeEquipPoints.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected live HP restored after NPC-service sword unequip, got ok=%v snapshot=%+v", ok, pointsSnapshot)
	}
	persisted, err = accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted NPC-service sword-equip account after unequip: %v", err)
	}
	if len(persisted.Characters[0].Equipment) != 0 {
		t.Fatalf("expected persisted equipment empty after NPC-service sword unequip, got %+v", persisted.Characters[0].Equipment)
	}
	if len(persisted.Characters[0].Inventory) != 1 || persisted.Characters[0].Inventory[0].ID != bought.ID || persisted.Characters[0].Inventory[0].Vnum != 11200 {
		t.Fatalf("expected persisted inventory to hold unequipped sword, got %+v", persisted.Characters[0].Inventory)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != beforeEquipPersistedHP {
		t.Fatalf("expected persisted HP restored after NPC-service sword unequip, got %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}
