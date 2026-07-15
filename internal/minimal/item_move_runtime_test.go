package minimal

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestGameRuntimeItemMoveRetargetsSourceItemQuickslotAndDeletesStaleDestinationBinding(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MoveQuickslotRetarget", 0x0103065f, 0x0204065f, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 6001, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "move-quickslot-retarget", 0x6060605f, owner)
	if err := accounts.Save(accountstore.Account{Login: "move-quickslot-retarget", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed quickslot-retarget item-move account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Retarget Stack Potion", Stackable: true, MaxCount: 200, AntiFemale: true, AntiAssassin: true}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected quickslot-retarget item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "move-quickslot-retarget", 0x6060605f)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected quickslot-retarget item-move packet error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected item move to emit item delete, item set, quickslot delete, quickslot add; got %d frames", len(out))
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode item delete frame: %v", err)
	}
	if itemDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected item delete position: %+v", itemDel.Position)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode item set frame: %v", err)
	}
	if itemSet.Position != itemproto.InventoryPosition(6) || itemSet.Vnum != 27001 || itemSet.Count != 3 {
		t.Fatalf("unexpected item set frame: %+v", itemSet)
	}
	wantMoveAntiFlags := itemproto.AntiFlagFemale | itemproto.AntiFlagAssassin
	if itemSet.AntiFlags != wantMoveAntiFlags {
		t.Fatalf("expected moved item set anti flags %#x, got %#x", wantMoveAntiFlags, itemSet.AntiFlags)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode quickslot delete frame: %v", err)
	}
	if quickslotDel.Position != 3 {
		t.Fatalf("expected stale destination quickslot position 3 to be deleted, got %d", quickslotDel.Position)
	}
	quickslotAdd, err := quickslotproto.DecodeAdd(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode quickslot add frame: %v", err)
	}
	if quickslotAdd.Position != 2 || quickslotAdd.Slot.Type != quickslotproto.TypeItem || quickslotAdd.Slot.Position != 6 {
		t.Fatalf("expected source item quickslot position 2 to retarget to item cell 6, got %+v", quickslotAdd)
	}

	persisted, err := accounts.Load("move-quickslot-retarget")
	if err != nil {
		t.Fatalf("load quickslot-retarget item-move account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, []inventory.ItemInstance{{ID: 6001, Vnum: 27001, Count: 3, Slot: 6}}) {
		t.Fatalf("unexpected persisted inventory after quickslot-retarget item move: %+v", persisted.Characters[0].Inventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after quickslot-retarget item move: got %+v want %+v", persisted.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemBootstrapFramesCarryTemplateDisplaySocketsAndAttributes(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DisplayAttrsBootstrap", 0x0103069b, 0x0204069b, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 6603, Vnum: 71084, Count: 1, Slot: 5}}
	issuePeerTicket(t, ticketStore, "display-attrs-bootstrap", 0x6060609b, owner)
	if err := accounts.Save(accountstore.Account{Login: "display-attrs-bootstrap", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed display-attrs bootstrap account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      71084,
		Name:      "Socketed Practice Charm",
		Stackable: false,
		MaxCount:  1,
		Sockets:   itemcatalog.SocketValues{11, -2, 33},
		Attributes: itemcatalog.AttributeValues{
			{Type: 1, Value: 25},
			{Type: 7, Value: -3},
		},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected display-attrs bootstrap runtime error: %v", err)
	}

	_, frames := enterGameWithLoginTicket(t, runtime.SessionFactory(), "display-attrs-bootstrap", 0x6060609b)
	var itemSet itemproto.SetPacket
	for _, raw := range frames {
		frame := decodeSingleFrame(t, raw)
		if frame.Header != itemproto.HeaderSet {
			continue
		}
		packet, err := itemproto.DecodeSet(frame)
		if err != nil {
			t.Fatalf("decode bootstrap item set: %v", err)
		}
		if packet.Vnum == 71084 {
			itemSet = packet
			break
		}
	}
	if itemSet.Vnum != 71084 {
		t.Fatalf("expected socketed practice charm ITEM_SET in bootstrap frames")
	}
	wantSockets := [itemproto.ItemSocketCount]int32{11, -2, 33}
	if itemSet.Sockets != wantSockets {
		t.Fatalf("expected template-authored sockets %+v, got %+v", wantSockets, itemSet.Sockets)
	}
	wantAttributes := [itemproto.ItemAttributeCount]itemproto.Attribute{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	if itemSet.Attributes != wantAttributes {
		t.Fatalf("expected template-authored attributes %+v, got %+v", wantAttributes, itemSet.Attributes)
	}
}

func TestGameRuntimeItemBootstrapFramesCarryTemplateAntiFlags(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AntiFlagBootstrap", 0x0103069a, 0x0204069a, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 6601, Vnum: 27091, Count: 3, Slot: 5}}
	owner.Equipment = []inventory.ItemInstance{{ID: 6602, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	issuePeerTicket(t, ticketStore, "antiflag-bootstrap", 0x6060609a, owner)
	if err := accounts.Save(accountstore.Account{Login: "antiflag-bootstrap", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed anti-flag bootstrap account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27091, Name: "Bound Practice Potion", Stackable: true, MaxCount: 200, SellCountPerGold: true, Rare: true, Unique: true, AntiDrop: true, AntiGive: true, AntiSell: true, AntiStack: true, AntiGet: true},
		{Vnum: 11200, Name: "Warrior-Locked Sword", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String(), AntiWarrior: true, AntiMale: true, AntiEmpireC: true},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected anti-flag bootstrap runtime error: %v", err)
	}

	_, frames := enterGameWithLoginTicket(t, runtime.SessionFactory(), "antiflag-bootstrap", 0x6060609a)
	var itemSets []itemproto.SetPacket
	for _, raw := range frames {
		frame := decodeSingleFrame(t, raw)
		if frame.Header != itemproto.HeaderSet {
			continue
		}
		packet, err := itemproto.DecodeSet(frame)
		if err != nil {
			t.Fatalf("decode bootstrap item set: %v", err)
		}
		itemSets = append(itemSets, packet)
	}
	if len(itemSets) != 2 {
		t.Fatalf("expected inventory and equipment item bootstrap sets, got %d", len(itemSets))
	}
	if itemSets[0].Position != itemproto.InventoryPosition(5) || itemSets[0].Vnum != 27091 {
		t.Fatalf("unexpected carried bootstrap item set: %+v", itemSets[0])
	}
	wantCarriedAntiFlags := itemproto.AntiFlagDrop | itemproto.AntiFlagGive | itemproto.AntiFlagSell | itemproto.AntiFlagStack | itemproto.AntiFlagGet
	if itemSets[0].AntiFlags != wantCarriedAntiFlags {
		t.Fatalf("expected carried item anti flags %#x, got %#x", wantCarriedAntiFlags, itemSets[0].AntiFlags)
	}
	wantCarriedFlags := itemproto.ItemFlagStackable | itemproto.ItemFlagCountPerGold | itemproto.ItemFlagRare | itemproto.ItemFlagUnique
	if itemSets[0].Flags != wantCarriedFlags {
		t.Fatalf("expected carried item flags %#x, got %#x", wantCarriedFlags, itemSets[0].Flags)
	}
	weaponPosition, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build weapon position: %v", err)
	}
	if itemSets[1].Position != weaponPosition || itemSets[1].Vnum != 11200 {
		t.Fatalf("unexpected equipment bootstrap item set: %+v", itemSets[1])
	}
	wantEquipmentAntiFlags := itemproto.AntiFlagWarrior | itemproto.AntiFlagMale | itemproto.AntiFlagEmpireC
	if itemSets[1].AntiFlags != wantEquipmentAntiFlags {
		t.Fatalf("expected equipment item anti flags %#x, got %#x", wantEquipmentAntiFlags, itemSets[1].AntiFlags)
	}
}

func TestGameRuntimeItemMoveRejectsDuplicateSourceOrTargetOccupancyWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name: "duplicate source occupancy",
			inventory: []inventory.ItemInstance{
				{ID: 6001, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 6002, Vnum: 27001, Count: 1, Slot: 5},
				{ID: 6003, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
		{
			name: "duplicate target occupancy",
			inventory: []inventory.ItemInstance{
				{ID: 6001, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 6002, Vnum: 27001, Count: 1, Slot: 6},
				{ID: 6003, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
		{
			name: "duplicate source and target item ids",
			inventory: []inventory.ItemInstance{
				{ID: 6001, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 6001, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
	}
	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("MoveDuplicateStack", 0x01030660+uint32(index), 0x02040660+uint32(index), 1300, 2300, 0, 101, 201)
			owner.Inventory = append([]inventory.ItemInstance(nil), tc.inventory...)
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			login := "move-duplicate-stack-" + string(rune('a'+index))
			issuePeerTicket(t, ticketStore, login, 0x60606060+uint32(index), owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed duplicate-occupancy item-move account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Duplicate Stack Potion", Stackable: true, MaxCount: 200}})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected duplicate-occupancy item-move runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x60606060+uint32(index))
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
				Source:      itemproto.InventoryPosition(5),
				Destination: itemproto.InventoryPosition(6),
				Count:       0,
			})))
			if err != nil {
				t.Fatalf("unexpected duplicate-occupancy item-move packet error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s item-move stack merge to emit no frames, got %d", tc.name, len(out))
			}
			persisted, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load duplicate-occupancy item-move account: %v", err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s item-move stack merge mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s item-move stack merge mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemMoveNonStackableSameVnumFullStackSwapsWithoutMerging(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MoveNonStackSwap", 0x0103066f, 0x0204066f, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 6401, Vnum: 50001, Count: 1, Slot: 5},
		{ID: 6402, Vnum: 50001, Count: 1, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "move-non-stack-swap", 0x6060606f, owner)
	if err := accounts.Save(accountstore.Account{Login: "move-non-stack-swap", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed non-stackable same-vnum item-move account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 50001, Name: "Non-Stackable Badge", Stackable: false, MaxCount: 1}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected non-stackable same-vnum item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "move-non-stack-swap", 0x6060606f)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected non-stackable same-vnum item-move packet error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected non-stackable same-vnum full-stack move to emit source set, destination set, stale quickslot delete, and retarget quickslot add; got %d frames", len(out))
	}
	sourceSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode non-stackable same-vnum source set: %v", err)
	}
	if sourceSet.Position != itemproto.InventoryPosition(5) || sourceSet.Vnum != 50001 || sourceSet.Count != 1 {
		t.Fatalf("unexpected non-stackable same-vnum source set: %+v", sourceSet)
	}
	destinationSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode non-stackable same-vnum destination set: %v", err)
	}
	if destinationSet.Position != itemproto.InventoryPosition(6) || destinationSet.Vnum != 50001 || destinationSet.Count != 1 {
		t.Fatalf("unexpected non-stackable same-vnum destination set: %+v", destinationSet)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode non-stackable same-vnum stale quickslot delete: %v", err)
	}
	if quickslotDel.Position != 3 {
		t.Fatalf("expected stale destination item quickslot position 3 to be deleted, got %+v", quickslotDel)
	}
	quickslotAdd, err := quickslotproto.DecodeAdd(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode non-stackable same-vnum quickslot retarget: %v", err)
	}
	if quickslotAdd.Position != 2 || quickslotAdd.Slot.Type != quickslotproto.TypeItem || quickslotAdd.Slot.Position != 6 {
		t.Fatalf("expected source item quickslot position 2 to retarget to carried slot 6, got %+v", quickslotAdd)
	}

	persisted, err := accounts.Load("move-non-stack-swap")
	if err != nil {
		t.Fatalf("load non-stackable same-vnum item-move account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 6402, Vnum: 50001, Count: 1, Slot: 5},
		{ID: 6401, Vnum: 50001, Count: 1, Slot: 6},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after non-stackable same-vnum swap: got %+v want %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after non-stackable same-vnum swap: got %+v want %+v", persisted.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemMoveRejectsTransferGuardedStackTemplatesWithoutMutation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*itemcatalog.Template)
	}{
		{name: "anti get", mutate: func(template *itemcatalog.Template) { template.AntiGet = true }},
		{name: "anti drop", mutate: func(template *itemcatalog.Template) { template.AntiDrop = true }},
		{name: "anti give", mutate: func(template *itemcatalog.Template) { template.AntiGive = true }},
		{name: "anti sell", mutate: func(template *itemcatalog.Template) { template.AntiSell = true }},
		{name: "authored equip slot", mutate: func(template *itemcatalog.Template) { template.EquipSlot = inventory.EquipmentSlotBody.String() }},
	}
	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("MoveGuardedStack", 0x01030670+uint32(index), 0x02040670+uint32(index), 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{
				{ID: 6101, Vnum: 27001, Count: 3, Slot: 5},
				{ID: 6102, Vnum: 27001, Count: 4, Slot: 6},
			}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			login := "move-guarded-stack-" + string(rune('a'+index))
			issuePeerTicket(t, ticketStore, login, 0x60606070+uint32(index), owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed transfer-guarded item-move account: %v", err)
			}
			template := itemcatalog.Template{Vnum: 27001, Name: "Guarded Stack Potion", Stackable: true, MaxCount: 200}
			tc.mutate(&template)
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected transfer-guarded item-move runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x60606070+uint32(index))
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
				Source:      itemproto.InventoryPosition(5),
				Destination: itemproto.InventoryPosition(6),
				Count:       0,
			})))
			if err != nil {
				t.Fatalf("unexpected transfer-guarded item-move packet error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s item-move stack merge to emit no frames, got %d", tc.name, len(out))
			}
			persisted, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load transfer-guarded item-move account: %v", err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s item-move stack merge mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s item-move stack merge mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemMoveRejectsMissingAuthoredStackTemplateWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MoveMissingTemplate", 0x0103067f, 0x0204067f, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 6121, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 6122, Vnum: 27001, Count: 4, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "move-missing-stack-template", 0x6060607f, owner)
	if err := accounts.Save(accountstore.Account{Login: "move-missing-stack-template", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed missing-template item-move account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27002, Name: "Other Authored Stack", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected missing-template item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "move-missing-stack-template", 0x6060607f)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected missing-template item-move packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected missing-template item-move stack merge to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after missing-template item-move rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("move-missing-stack-template")
	if err != nil {
		t.Fatalf("load missing-template item-move account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("missing-template item-move stack merge mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("missing-template item-move stack merge mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemMoveRejectsOverUint8TemplateMaxWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MoveWideMaxStack", 0x0103068f, 0x0204068f, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 6151, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 6152, Vnum: 27001, Count: 4, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "move-wide-max-stack", 0x6060608f, owner)
	if err := accounts.Save(accountstore.Account{Login: "move-wide-max-stack", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed over-uint8 item-move account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected over-uint8 item-move runtime error: %v", err)
	}
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "Runtime Wide Max Stack Potion", Stackable: true, MaxCount: 256}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "move-wide-max-stack", 0x6060608f)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected over-uint8 item-move packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected over-uint8 item-move stack merge to emit no frames, got %d", len(out))
	}
	persisted, err := accounts.Load("move-wide-max-stack")
	if err != nil {
		t.Fatalf("load over-uint8 item-move account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("over-uint8 item-move stack merge mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("over-uint8 item-move stack merge mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemMoveRejectsSelectedCharacterRestrictedStackTemplatesWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		template itemcatalog.Template
		job      uint8
		raceNum  uint16
		level    uint8
	}{
		{
			name:     "anti warrior",
			template: itemcatalog.Template{Vnum: 27001, Name: "Warrior Restricted Stack Potion", Stackable: true, MaxCount: 200, AntiWarrior: true},
			job:      0, raceNum: 0, level: 1,
		},
		{
			name:     "anti male",
			template: itemcatalog.Template{Vnum: 27001, Name: "Male Restricted Stack Potion", Stackable: true, MaxCount: 200, AntiMale: true},
			job:      0, raceNum: 0, level: 1,
		},
		{
			name:     "min level",
			template: itemcatalog.Template{Vnum: 27001, Name: "Veteran Stack Potion", Stackable: true, MaxCount: 200, MinLevel: 10},
			job:      0, raceNum: 0, level: 5,
		},
	}
	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("MoveRestrictedStack", 0x01030690+uint32(index), 0x02040690+uint32(index), 1300, 2300, tc.raceNum, 101, 201)
			owner.Job = tc.job
			owner.RaceNum = tc.raceNum
			owner.Level = tc.level
			owner.Inventory = []inventory.ItemInstance{
				{ID: 6201, Vnum: 27001, Count: 3, Slot: 5},
				{ID: 6202, Vnum: 27001, Count: 4, Slot: 6},
			}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			login := "move-restricted-stack-" + string(rune('a'+index))
			issuePeerTicket(t, ticketStore, login, 0x60606090+uint32(index), owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed selected-character-restricted item-move account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{tc.template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected selected-character-restricted item-move runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x60606090+uint32(index))
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
				Source:      itemproto.InventoryPosition(5),
				Destination: itemproto.InventoryPosition(6),
				Count:       0,
			})))
			if err != nil {
				t.Fatalf("unexpected selected-character-restricted item-move packet error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s item-move stack merge to emit no frames, got %d", tc.name, len(out))
			}
			persisted, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load selected-character-restricted item-move account: %v", err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s item-move stack merge mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s item-move stack merge mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemMovePartialMergePreservesAllTargetItemQuickslots(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MovePartialMultiQS", 0x0103069f, 0x0204069f, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 6251, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 6252, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 6, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 7, Type: quickslotproto.TypeSkill, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "move-partial-multi-qs", 0x6060609f, owner)
	if err := accounts.Save(accountstore.Account{Login: "move-partial-multi-qs", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial multi-quickslot item-move account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Partial Quickslot Stack Potion", Stackable: true, MaxCount: 10}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected partial multi-quickslot item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "move-partial-multi-qs", 0x6060609f)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected partial multi-quickslot item-move packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected partial item move to emit only source and target updates, got %d", len(out))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial item-move source update: %v", err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 5 {
		t.Fatalf("unexpected partial item-move source update: %+v", sourceUpdate)
	}
	targetUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial item-move target update: %v", err)
	}
	if targetUpdate.Position != itemproto.InventoryPosition(6) || targetUpdate.Count != 10 {
		t.Fatalf("unexpected partial item-move target update: %+v", targetUpdate)
	}
	persisted, err := accounts.Load("move-partial-multi-qs")
	if err != nil {
		t.Fatalf("load partial multi-quickslot item-move account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 6251, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 6252, Vnum: 27001, Count: 10, Slot: 6},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted partial item-move inventory: got %+v want %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("unexpected persisted partial item-move quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemMoveFullStackMergeDeletesAllSourceItemQuickslots(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MoveQuickslotOwner", 0x010306A0, 0x020406A0, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 6301, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 6302, Vnum: 27001, Count: 4, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeCommand, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 5, Type: quickslotproto.TypeItem, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "move-quickslot-owner", 0x606060A0, owner)
	if err := accounts.Save(accountstore.Account{Login: "move-quickslot-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-move quickslot owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Quickslot Stack Potion", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-move quickslot runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "move-quickslot-owner", 0x606060A0)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected item-move quickslot packet error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected item-move full merge to emit target update plus all source quickslot deletes, got %d frames", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode item-move source delete: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected item-move source delete: %+v", del)
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode item-move target update: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(6) || update.Count != 7 {
		t.Fatalf("unexpected item-move target update: %+v", update)
	}
	firstQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode first item-move source quickslot delete: %v", err)
	}
	if firstQuickslotDel.Position != 2 {
		t.Fatalf("expected first source item quickslot position 2 to be deleted, got %+v", firstQuickslotDel)
	}
	secondQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode second item-move source quickslot delete: %v", err)
	}
	if secondQuickslotDel.Position != 5 {
		t.Fatalf("expected second source item quickslot position 5 to be deleted, got %+v", secondQuickslotDel)
	}
	persisted, err := accounts.Load("move-quickslot-owner")
	if err != nil {
		t.Fatalf("load item-move quickslot account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 6302, Vnum: 27001, Count: 7, Slot: 6}}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after item-move full merge: got %+v want %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeCommand, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after item-move full merge: got %+v want %+v", persisted.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemMoveEquipRejectsTemplateAntiFlagsWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		template itemcatalog.Template
	}{
		{
			name: "anti warrior",
			template: itemcatalog.Template{
				Vnum:        11500,
				Name:        "Warrior-Restricted Test Armor",
				Stackable:   false,
				MaxCount:    1,
				AntiWarrior: true,
				EquipSlot:   inventory.EquipmentSlotBody.String(),
				EquipEffect: &itemcatalog.PointEffect{PointType: 1, PointIndex: 1, PointDelta: 50},
			},
		},
		{
			name: "anti male",
			template: itemcatalog.Template{
				Vnum:        11500,
				Name:        "Male-Restricted Test Armor",
				Stackable:   false,
				MaxCount:    1,
				AntiMale:    true,
				EquipSlot:   inventory.EquipmentSlotBody.String(),
				EquipEffect: &itemcatalog.PointEffect{PointType: 1, PointIndex: 1, PointDelta: 50},
			},
		},
		{
			name: "anti stack",
			template: itemcatalog.Template{
				Vnum:        11500,
				Name:        "Anti-Stack Test Armor",
				Stackable:   false,
				MaxCount:    1,
				AntiStack:   true,
				EquipSlot:   inventory.EquipmentSlotBody.String(),
				EquipEffect: &itemcatalog.PointEffect{PointType: 1, PointIndex: 1, PointDelta: 50},
			},
		},
		{
			name: "anti drop",
			template: itemcatalog.Template{
				Vnum:        11500,
				Name:        "Anti-Drop Test Armor",
				Stackable:   false,
				MaxCount:    1,
				AntiDrop:    true,
				EquipSlot:   inventory.EquipmentSlotBody.String(),
				EquipEffect: &itemcatalog.PointEffect{PointType: 1, PointIndex: 1, PointDelta: 50},
			},
		},
		{
			name: "anti give",
			template: itemcatalog.Template{
				Vnum:        11500,
				Name:        "Anti-Give Test Armor",
				Stackable:   false,
				MaxCount:    1,
				AntiGive:    true,
				EquipSlot:   inventory.EquipmentSlotBody.String(),
				EquipEffect: &itemcatalog.PointEffect{PointType: 1, PointIndex: 1, PointDelta: 50},
			},
		},
		{
			name: "anti sell",
			template: itemcatalog.Template{
				Vnum:        11500,
				Name:        "Anti-Sell Test Armor",
				Stackable:   false,
				MaxCount:    1,
				AntiSell:    true,
				EquipSlot:   inventory.EquipmentSlotBody.String(),
				EquipEffect: &itemcatalog.PointEffect{PointType: 1, PointIndex: 1, PointDelta: 50},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
			if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{tc.template}}); err != nil {
				t.Fatalf("seed anti-flag equip template: %v", err)
			}
			owner := peerVisibilityCharacter("AntiEquipOwner", 0x01030251, 0x02040251, 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 5101, Vnum: tc.template.Vnum, Count: 1, Slot: 5}}
			owner.Equipment = nil
			owner.Points[1] = 750
			issuePeerTicket(t, ticketStore, "anti-equip-owner", 0x51515151, owner)
			if err := accounts.Save(accountstore.Account{Login: "anti-equip-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed anti-flag equip owner account: %v", err)
			}

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected anti-flag equip runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "anti-equip-owner", 0x51515151)
			defer closeSessionFlow(t, flow)
			bodyPosition, err := itemproto.EquipmentPosition(0)
			if err != nil {
				t.Fatalf("build body equipment position: %v", err)
			}

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
				Source:      itemproto.InventoryPosition(5),
				Destination: bodyPosition,
			})))
			if err != nil {
				t.Fatalf("unexpected anti-flag equip error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected anti-flag equip to emit no frames, got %d", len(out))
			}
			account, err := accounts.Load("anti-equip-owner")
			if err != nil {
				t.Fatalf("load anti-flag equip owner account: %v", err)
			}
			if len(account.Characters) != 1 {
				t.Fatalf("expected one persisted anti-flag equip owner, got %+v", account)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("anti-flag equip mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
			}
			if len(account.Characters[0].Equipment) != 0 {
				t.Fatalf("anti-flag equip mutated persisted equipment: got %#v", account.Characters[0].Equipment)
			}
			if account.Characters[0].Points[1] != owner.Points[1] {
				t.Fatalf("anti-flag equip mutated persisted point: got %d want %d", account.Characters[0].Points[1], owner.Points[1])
			}
		})
	}
}

func TestGameRuntimeItemMoveEquipRejectsPointOverflowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	template := itemcatalog.Template{
		Vnum:        11500,
		Name:        "Overflowing Test Armor",
		Stackable:   false,
		MaxCount:    1,
		EquipSlot:   inventory.EquipmentSlotBody.String(),
		EquipEffect: &itemcatalog.PointEffect{PointType: 1, PointIndex: 1, PointDelta: 50},
	}
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{template}}); err != nil {
		t.Fatalf("seed overflowing equip template: %v", err)
	}
	owner := peerVisibilityCharacter("OverflowEquipOwner", 0x01030254, 0x02040254, 1300, 2300, 2, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 5401, Vnum: template.Vnum, Count: 1, Slot: 5}}
	owner.Equipment = nil
	owner.Points[1] = 1<<31 - 5
	issuePeerTicket(t, ticketStore, "overflow-equip-owner", 0x54545454, owner)
	if err := accounts.Save(accountstore.Account{Login: "overflow-equip-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed overflowing equip owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected overflowing equip runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "overflow-equip-owner", 0x54545454)
	defer closeSessionFlow(t, flow)
	bodyPosition, err := itemproto.EquipmentPosition(0)
	if err != nil {
		t.Fatalf("build body equipment position: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: bodyPosition,
	})))
	if err != nil {
		t.Fatalf("unexpected overflowing equip error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected overflowing equip to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("overflow-equip-owner")
	if err != nil {
		t.Fatalf("load overflowing equip owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("overflowing equip mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if len(account.Characters[0].Equipment) != 0 {
		t.Fatalf("overflowing equip mutated persisted equipment: got %#v", account.Characters[0].Equipment)
	}
	if account.Characters[0].Points[1] != owner.Points[1] {
		t.Fatalf("overflowing equip mutated persisted point: got %d want %d", account.Characters[0].Points[1], owner.Points[1])
	}
}

func TestGameRuntimeItemMoveEquipRejectsMissingTemplateWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      11501,
		Name:      "Unrelated Test Armor",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: inventory.EquipmentSlotBody.String(),
	}}}); err != nil {
		t.Fatalf("seed unrelated equip template: %v", err)
	}
	owner := peerVisibilityCharacter("MissingEquipOwner", 0x01030253, 0x02040253, 1300, 2300, 2, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 5301, Vnum: 11500, Count: 1, Slot: 5}}
	owner.Equipment = nil
	owner.Points[1] = 750
	issuePeerTicket(t, ticketStore, "missing-equip-owner", 0x53535353, owner)
	if err := accounts.Save(accountstore.Account{Login: "missing-equip-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed missing-template equip owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected missing-template equip runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "missing-equip-owner", 0x53535353)
	defer closeSessionFlow(t, flow)
	bodyPosition, err := itemproto.EquipmentPosition(0)
	if err != nil {
		t.Fatalf("build body equipment position: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: bodyPosition,
	})))
	if err != nil {
		t.Fatalf("unexpected missing-template equip error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected missing-template equip to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("missing-equip-owner")
	if err != nil {
		t.Fatalf("load missing-template equip owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("missing-template equip mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if len(account.Characters[0].Equipment) != 0 {
		t.Fatalf("missing-template equip mutated persisted equipment: got %#v", account.Characters[0].Equipment)
	}
	if account.Characters[0].Points[1] != owner.Points[1] {
		t.Fatalf("missing-template equip mutated persisted point: got %d want %d", account.Characters[0].Points[1], owner.Points[1])
	}
}

func TestGameRuntimeItemMoveEquipRejectsMismatchedTemplateVnumWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      11501,
		Name:      "Mismatched Test Armor",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: inventory.EquipmentSlotBody.String(),
	}}}); err != nil {
		t.Fatalf("seed mismatched equip template: %v", err)
	}
	owner := peerVisibilityCharacter("MismatchedEquipOwner", 0x01030254, 0x02040254, 1300, 2300, 2, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 5401, Vnum: 11500, Count: 1, Slot: 5}}
	owner.Equipment = nil
	owner.Points[1] = 750
	issuePeerTicket(t, ticketStore, "mismatched-equip-owner", 0x54535353, owner)
	if err := accounts.Save(accountstore.Account{Login: "mismatched-equip-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed mismatched-template equip owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected mismatched-template equip runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "mismatched-equip-owner", 0x54535353)
	defer closeSessionFlow(t, flow)
	bodyPosition, err := itemproto.EquipmentPosition(0)
	if err != nil {
		t.Fatalf("build body equipment position: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: bodyPosition,
	})))
	if err != nil {
		t.Fatalf("unexpected mismatched-template equip error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected mismatched-template equip to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("mismatched-equip-owner")
	if err != nil {
		t.Fatalf("load mismatched-template equip owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("mismatched-template equip mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if len(account.Characters[0].Equipment) != 0 {
		t.Fatalf("mismatched-template equip mutated persisted equipment: got %#v", account.Characters[0].Equipment)
	}
	if account.Characters[0].Points[1] != owner.Points[1] {
		t.Fatalf("mismatched-template equip mutated persisted point: got %d want %d", account.Characters[0].Points[1], owner.Points[1])
	}
}

func TestGameRuntimeItemMoveEquipAppliesTemplateEffectWhenAllowed(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	template := itemcatalog.Template{
		Vnum:        11500,
		Name:        "Allowed Test Armor",
		Stackable:   false,
		MaxCount:    1,
		EquipSlot:   inventory.EquipmentSlotBody.String(),
		EquipEffect: &itemcatalog.PointEffect{PointType: 1, PointIndex: 1, PointDelta: 50},
	}
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{template}}); err != nil {
		t.Fatalf("seed allowed equip template: %v", err)
	}
	owner := peerVisibilityCharacter("AllowedEquipOwner", 0x01030252, 0x02040252, 1300, 2300, 2, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 5201, Vnum: template.Vnum, Count: 1, Slot: 5}}
	owner.Equipment = nil
	owner.Points[1] = 750
	issuePeerTicket(t, ticketStore, "allowed-equip-owner", 0x52525252, owner)
	if err := accounts.Save(accountstore.Account{Login: "allowed-equip-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed allowed equip owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected allowed equip runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "allowed-equip-owner", 0x52525252)
	defer closeSessionFlow(t, flow)
	bodyPosition, err := itemproto.EquipmentPosition(0)
	if err != nil {
		t.Fatalf("build body equipment position: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: bodyPosition,
	})))
	if err != nil {
		t.Fatalf("unexpected allowed equip error: %v", err)
	}
	if len(out) < 4 {
		t.Fatalf("expected allowed equip refresh frames, got %d", len(out))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode allowed equip point change: %v", err)
	}
	if pointChange.Type != 1 || pointChange.Amount != 50 || pointChange.Value != 800 {
		t.Fatalf("unexpected allowed equip point change: %+v", pointChange)
	}
	account, err := accounts.Load("allowed-equip-owner")
	if err != nil {
		t.Fatalf("load allowed equip owner account: %v", err)
	}
	if len(account.Characters) != 1 {
		t.Fatalf("expected one persisted allowed equip owner, got %+v", account)
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected allowed equip to remove carried item, got %#v", account.Characters[0].Inventory)
	}
	if len(account.Characters[0].Equipment) != 1 || account.Characters[0].Equipment[0].Vnum != template.Vnum || account.Characters[0].Equipment[0].EquipSlot != inventory.EquipmentSlotBody {
		t.Fatalf("expected allowed equip to persist worn item, got %#v", account.Characters[0].Equipment)
	}
	if account.Characters[0].Points[1] != 800 {
		t.Fatalf("expected allowed equip to persist point effect, got %d", account.Characters[0].Points[1])
	}
}

func TestGameRuntimeItemUseToItemRejectsLockedStacksWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name: "locked source",
			inventory: []inventory.ItemInstance{
				{ID: 1201, Vnum: 27001, Count: 2, Slot: 5, Locked: true},
				{ID: 1202, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
		{
			name: "locked target",
			inventory: []inventory.ItemInstance{
				{ID: 1201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1202, Vnum: 27001, Count: 3, Slot: 6, Locked: true},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
			if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
				Vnum:      27001,
				Name:      "Small Red Potion",
				Stackable: true,
				MaxCount:  200,
			}}}); err != nil {
				t.Fatalf("seed item template: %v", err)
			}
			owner := peerVisibilityCharacter("UseToItemLocked", 0x01030211, 0x02040211, 1300, 2300, 0, 101, 201)
			owner.Inventory = tc.inventory
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
			issuePeerTicket(t, ticketStore, "use-to-item-locked", 0x61616161, owner)
			if err := accounts.Save(accountstore.Account{Login: "use-to-item-locked", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed locked use-to-item owner account: %v", err)
			}

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected locked use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-locked", 0x61616161)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
				Source: itemproto.InventoryPosition(5),
				Target: itemproto.InventoryPosition(6),
			})))
			if err != nil {
				t.Fatalf("unexpected locked use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected locked use-to-item to emit no frames, got %d", len(out))
			}
			account, err := accounts.Load("use-to-item-locked")
			if err != nil {
				t.Fatalf("load locked use-to-item owner account: %v", err)
			}
			if len(account.Characters) != 1 {
				t.Fatalf("expected one persisted locked use-to-item owner, got %+v", account)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("locked use-to-item mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("locked use-to-item mutated persisted quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemUseToItemRejectsTemplateGuardEdgesWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		template  itemcatalog.Template
		inventory []inventory.ItemInstance
	}{
		{
			name:     "non-stackable template",
			template: itemcatalog.Template{Vnum: 27001, Name: "Practice Relic", Stackable: false, MaxCount: 1},
			inventory: []inventory.ItemInstance{
				{ID: 1301, Vnum: 27001, Count: 1, Slot: 5},
				{ID: 1302, Vnum: 27001, Count: 1, Slot: 6},
			},
		},
		{
			name:     "anti-stack template",
			template: itemcatalog.Template{Vnum: 27001, Name: "Bound Practice Potion", Stackable: true, MaxCount: 200, AntiStack: true},
			inventory: []inventory.ItemInstance{
				{ID: 1301, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1302, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
		{
			name:     "anti-drop template",
			template: itemcatalog.Template{Vnum: 27001, Name: "Undroppable Practice Potion", Stackable: true, MaxCount: 200, AntiDrop: true},
			inventory: []inventory.ItemInstance{
				{ID: 1301, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1302, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
		{
			name:     "anti-give template",
			template: itemcatalog.Template{Vnum: 27001, Name: "Soulbound Practice Potion", Stackable: true, MaxCount: 200, AntiGive: true},
			inventory: []inventory.ItemInstance{
				{ID: 1301, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1302, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
		{
			name:     "anti-sell template",
			template: itemcatalog.Template{Vnum: 27001, Name: "Unsellable Practice Potion", Stackable: true, MaxCount: 200, AntiSell: true},
			inventory: []inventory.ItemInstance{
				{ID: 1301, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1302, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
			if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{tc.template}}); err != nil {
				t.Fatalf("seed guarded item template: %v", err)
			}
			owner := peerVisibilityCharacter("UseToItemGuard", 0x01030212, 0x02040212, 1300, 2300, 0, 101, 201)
			owner.Inventory = tc.inventory
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
			issuePeerTicket(t, ticketStore, "use-to-item-guard", 0x62626262, owner)
			if err := accounts.Save(accountstore.Account{Login: "use-to-item-guard", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed guarded use-to-item owner account: %v", err)
			}

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected guarded use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-guard", 0x62626262)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
				Source: itemproto.InventoryPosition(5),
				Target: itemproto.InventoryPosition(6),
			})))
			if err != nil {
				t.Fatalf("unexpected guarded use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected guarded use-to-item to emit no frames, got %d", len(out))
			}
			account, err := accounts.Load("use-to-item-guard")
			if err != nil {
				t.Fatalf("load guarded use-to-item owner account: %v", err)
			}
			if len(account.Characters) != 1 {
				t.Fatalf("expected one persisted guarded use-to-item owner, got %+v", account)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("guarded use-to-item mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("guarded use-to-item mutated persisted quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemUseToItemPartialMergePreservesSourceQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  10,
	}}}); err != nil {
		t.Fatalf("seed partial use-to-item template: %v", err)
	}
	owner := peerVisibilityCharacter("UseToItemPartial", 0x01030213, 0x02040213, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1401, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 1402, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 6, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 7, Type: quickslotproto.TypeCommand, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "use-to-item-partial", 0x63636363, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-partial", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial use-to-item owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected partial use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-partial", 0x63636363)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected partial use-to-item error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected partial use-to-item to emit source and target item updates only, got %d", len(out))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial use-to-item source update: %v", err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 5 {
		t.Fatalf("unexpected partial use-to-item source update: %+v", sourceUpdate)
	}
	targetUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial use-to-item target update: %v", err)
	}
	if targetUpdate.Position != itemproto.InventoryPosition(6) || targetUpdate.Count != 10 {
		t.Fatalf("unexpected partial use-to-item target update: %+v", targetUpdate)
	}
	account, err := accounts.Load("use-to-item-partial")
	if err != nil {
		t.Fatalf("load persisted partial use-to-item owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 1401, Vnum: 27001, Count: 5, Slot: 5}, {ID: 1402, Vnum: 27001, Count: 10, Slot: 6}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("partial use-to-item persisted inventory mismatch: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("partial use-to-item should preserve source item quickslots, got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemMoveCompatibleMergeRejectsMissingAuthoredTemplateWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27002,
		Name:      "Unrelated Small Blue Potion",
		Stackable: true,
		MaxCount:  200,
	}}}); err != nil {
		t.Fatalf("seed unrelated item-move template: %v", err)
	}
	owner := peerVisibilityCharacter("ItemMoveMissingTemplate", 0x0103021a, 0x0204021a, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1801, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1802, Vnum: 27001, Count: 2, Slot: 8},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-move-missing-template", 0x68686868, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-move-missing-template", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed missing-template item-move owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected missing-template item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-move-missing-template", 0x68686868)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
	})))
	if err != nil {
		t.Fatalf("unexpected missing-template item-move error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected missing-template compatible item move to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("item-move-missing-template")
	if err != nil {
		t.Fatalf("load missing-template item-move account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("missing-template item-move mutated inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("missing-template item-move mutated quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemMoveCountedPartialSplitPreservesSourceItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
	}}}); err != nil {
		t.Fatalf("seed counted split item-move template: %v", err)
	}
	owner := peerVisibilityCharacter("ItemMoveSplitQS", 0x01030229, 0x02040229, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1901, Vnum: 27001, Count: 5, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "item-move-split-qs", 0x69696969, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-move-split-qs", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed counted split item-move owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected counted split item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-move-split-qs", 0x69696969)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected counted split item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected counted split item move to emit only source and destination item refreshes, got %d", len(out))
	}
	sourceSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode counted split source set: %v", err)
	}
	if sourceSet.Position != itemproto.InventoryPosition(5) || sourceSet.Count != 3 {
		t.Fatalf("unexpected counted split source refresh: %+v", sourceSet)
	}
	destinationSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode counted split destination set: %v", err)
	}
	if destinationSet.Position != itemproto.InventoryPosition(8) || destinationSet.Count != 2 {
		t.Fatalf("unexpected counted split destination refresh: %+v", destinationSet)
	}

	account, err := accounts.Load("item-move-split-qs")
	if err != nil {
		t.Fatalf("load counted split item-move account: %v", err)
	}
	if len(account.Characters[0].Inventory) != 2 {
		t.Fatalf("expected split item-move to persist two carried stacks, got %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("counted split item-move should preserve source quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemMoveCountedPartialMergePreservesSourceAndTargetItemQuickslots(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  10,
	}}}); err != nil {
		t.Fatalf("seed counted partial merge item-move template: %v", err)
	}
	owner := peerVisibilityCharacter("ItemMoveMergeQS", 0x01030239, 0x02040239, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1951, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 1952, Vnum: 27001, Count: 7, Slot: 8},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 8},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 8},
	}
	issuePeerTicket(t, ticketStore, "item-move-partial-merge-qs", 0x69696979, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-move-partial-merge-qs", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed counted partial merge item-move owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected counted partial merge item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-move-partial-merge-qs", 0x69696979)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected counted partial merge item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected counted partial merge item move to emit only source and destination item refreshes, got %d", len(out))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode counted partial merge source update: %v", err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 3 {
		t.Fatalf("unexpected counted partial merge source refresh: %+v", sourceUpdate)
	}
	destinationUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode counted partial merge destination update: %v", err)
	}
	if destinationUpdate.Position != itemproto.InventoryPosition(8) || destinationUpdate.Count != 9 {
		t.Fatalf("unexpected counted partial merge destination refresh: %+v", destinationUpdate)
	}

	account, err := accounts.Load("item-move-partial-merge-qs")
	if err != nil {
		t.Fatalf("load counted partial merge item-move account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 1951, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1952, Vnum: 27001, Count: 9, Slot: 8},
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("counted partial merge persisted inventory mismatch: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("counted partial merge item-move should preserve source and target quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemMoveCountedFullStackMergeDeletesSourceItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
	}}}); err != nil {
		t.Fatalf("seed counted item-move template: %v", err)
	}
	owner := peerVisibilityCharacter("ItemMoveCountQS", 0x01030219, 0x02040219, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1701, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1702, Vnum: 27001, Count: 2, Slot: 8},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 8},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "item-move-count-qs", 0x67676767, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-move-count-qs", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed counted item-move owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected counted item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-move-count-qs", 0x67676767)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       3,
	})))
	if err != nil {
		t.Fatalf("unexpected counted item-move error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected counted full-stack item move to emit item del/update and source quickslot delete, got %d", len(out))
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode counted item-move source delete: %v", err)
	}
	if itemDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected counted item-move source delete: %+v", itemDel)
	}
	targetUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode counted item-move target update: %v", err)
	}
	if targetUpdate.Position != itemproto.InventoryPosition(8) || targetUpdate.Count != 5 {
		t.Fatalf("unexpected counted item-move target update: %+v", targetUpdate)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode counted item-move source quickslot delete: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("expected source item quickslot position 2 to be deleted, got %+v", quickslotDel)
	}

	account, err := accounts.Load("item-move-count-qs")
	if err != nil {
		t.Fatalf("load counted item-move account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 1702, Vnum: 27001, Count: 5, Slot: 8}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("counted item-move persisted inventory mismatch: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 8},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("counted item-move persisted quickslots mismatch: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemUseToItemRejectsSelectedCharacterAntiFlagsWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		race     uint16
		template itemcatalog.Template
	}{
		{
			name:     "anti warrior",
			race:     0,
			template: itemcatalog.Template{Vnum: 27001, Name: "Warrior Restricted Practice Potion", Stackable: true, MaxCount: 200, AntiWarrior: true},
		},
		{
			name:     "anti male",
			race:     0,
			template: itemcatalog.Template{Vnum: 27001, Name: "Male Restricted Practice Potion", Stackable: true, MaxCount: 200, AntiMale: true},
		},
		{
			name:     "anti assassin",
			race:     1,
			template: itemcatalog.Template{Vnum: 27001, Name: "Assassin Restricted Practice Potion", Stackable: true, MaxCount: 200, AntiAssassin: true},
		},
		{
			name:     "anti female",
			race:     1,
			template: itemcatalog.Template{Vnum: 27001, Name: "Female Restricted Practice Potion", Stackable: true, MaxCount: 200, AntiFemale: true},
		},
		{
			name:     "anti sura",
			race:     2,
			template: itemcatalog.Template{Vnum: 27001, Name: "Sura Restricted Practice Potion", Stackable: true, MaxCount: 200, AntiSura: true},
		},
		{
			name:     "anti shaman",
			race:     3,
			template: itemcatalog.Template{Vnum: 27001, Name: "Shaman Restricted Practice Potion", Stackable: true, MaxCount: 200, AntiShaman: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
			if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{tc.template}}); err != nil {
				t.Fatalf("seed selected-character anti-flag template: %v", err)
			}
			owner := peerVisibilityCharacter("UseToItemAntiFlag", 0x01030216, 0x02040216, 1300, 2300, tc.race, 101, 201)
			owner.Inventory = []inventory.ItemInstance{
				{ID: 1601, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1602, Vnum: 27001, Count: 3, Slot: 6},
			}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			issuePeerTicket(t, ticketStore, "use-to-item-selected-anti-flag", 0x66666666, owner)
			if err := accounts.Save(accountstore.Account{Login: "use-to-item-selected-anti-flag", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed selected-character anti-flag owner account: %v", err)
			}

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected selected-character anti-flag runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-selected-anti-flag", 0x66666666)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
				Source: itemproto.InventoryPosition(5),
				Target: itemproto.InventoryPosition(6),
			})))
			if err != nil {
				t.Fatalf("unexpected selected-character anti-flag use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s use-to-item to emit no frames, got %d", tc.name, len(out))
			}
			account, err := accounts.Load("use-to-item-selected-anti-flag")
			if err != nil {
				t.Fatalf("load selected-character anti-flag owner account: %v", err)
			}
			if len(account.Characters) != 1 {
				t.Fatalf("expected one persisted selected-character anti-flag owner, got %+v", account)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s use-to-item mutated persisted inventory: got %#v want %#v", tc.name, account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s use-to-item mutated persisted quickslots: got %#v want %#v", tc.name, account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemUseToItemRejectsMinLevelTemplateWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	template := itemcatalog.Template{
		Vnum:      27001,
		Name:      "Level Restricted Practice Potion",
		Stackable: true,
		MaxCount:  200,
		MinLevel:  2,
	}
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{template}}); err != nil {
		t.Fatalf("seed min-level use-to-item template: %v", err)
	}
	owner := peerVisibilityCharacter("UseToItemMinLevel", 0x01030218, 0x02040218, 1300, 2300, 0, 101, 201)
	owner.Level = 1
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1801, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 1802, Vnum: 27001, Count: 3, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "use-to-item-min-level", 0x68686868, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-min-level", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed min-level use-to-item owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected min-level use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-min-level", 0x68686868)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected min-level use-to-item error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected min-level use-to-item to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("use-to-item-min-level")
	if err != nil {
		t.Fatalf("load min-level use-to-item owner account: %v", err)
	}
	if len(account.Characters) != 1 {
		t.Fatalf("expected one persisted min-level use-to-item owner, got %+v", account)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("min-level use-to-item mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("min-level use-to-item mutated persisted quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemUseToItemRejectsTargetGuardEdgesWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		template  itemcatalog.Template
		inventory []inventory.ItemInstance
	}{
		{
			name:     "already full target",
			template: itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			inventory: []inventory.ItemInstance{
				{ID: 1501, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1502, Vnum: 27001, Count: 200, Slot: 6},
			},
		},
		{
			name:     "over-template-max target",
			template: itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			inventory: []inventory.ItemInstance{
				{ID: 1501, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1502, Vnum: 27001, Count: 201, Slot: 6},
			},
		},
		{
			name:     "duplicate source and target item id",
			template: itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			inventory: []inventory.ItemInstance{
				{ID: 1501, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1501, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
			if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{tc.template}}); err != nil {
				t.Fatalf("seed target guard item template: %v", err)
			}
			owner := peerVisibilityCharacter("UseToItemTargetGuard", 0x01030215, 0x02040215, 1300, 2300, 0, 101, 201)
			owner.Inventory = tc.inventory
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
			issuePeerTicket(t, ticketStore, "use-to-item-target-guard", 0x65656565, owner)
			if err := accounts.Save(accountstore.Account{Login: "use-to-item-target-guard", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed target guard use-to-item owner account: %v", err)
			}

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected target guard use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-target-guard", 0x65656565)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
				Source: itemproto.InventoryPosition(5),
				Target: itemproto.InventoryPosition(6),
			})))
			if err != nil {
				t.Fatalf("unexpected target guard use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s use-to-item to emit no frames, got %d", tc.name, len(out))
			}
			account, err := accounts.Load("use-to-item-target-guard")
			if err != nil {
				t.Fatalf("load target guard use-to-item owner account: %v", err)
			}
			if len(account.Characters) != 1 {
				t.Fatalf("expected one persisted target guard use-to-item owner, got %+v", account)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s use-to-item mutated persisted inventory: got %#v want %#v", tc.name, account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s use-to-item mutated persisted quickslots: got %#v want %#v", tc.name, account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemUseToItemRejectsSourceGuardEdgesWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		template  itemcatalog.Template
		inventory []inventory.ItemInstance
	}{
		{
			name:     "over-template-max source",
			template: itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			inventory: []inventory.ItemInstance{
				{ID: 1701, Vnum: 27001, Count: 201, Slot: 5},
				{ID: 1702, Vnum: 27001, Count: 3, Slot: 6},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
			if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{tc.template}}); err != nil {
				t.Fatalf("seed source guard item template: %v", err)
			}
			owner := peerVisibilityCharacter("UseToItemSourceGuard", 0x01030217, 0x02040217, 1300, 2300, 0, 101, 201)
			owner.Inventory = tc.inventory
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
			issuePeerTicket(t, ticketStore, "use-to-item-source-guard", 0x67676767, owner)
			if err := accounts.Save(accountstore.Account{Login: "use-to-item-source-guard", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed source guard use-to-item owner account: %v", err)
			}

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected source guard use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-source-guard", 0x67676767)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
				Source: itemproto.InventoryPosition(5),
				Target: itemproto.InventoryPosition(6),
			})))
			if err != nil {
				t.Fatalf("unexpected source guard use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s use-to-item to emit no frames, got %d", tc.name, len(out))
			}
			account, err := accounts.Load("use-to-item-source-guard")
			if err != nil {
				t.Fatalf("load source guard use-to-item owner account: %v", err)
			}
			if len(account.Characters) != 1 {
				t.Fatalf("expected one persisted source guard use-to-item owner, got %+v", account)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s use-to-item mutated persisted inventory: got %#v want %#v", tc.name, account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s use-to-item mutated persisted quickslots: got %#v want %#v", tc.name, account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemUseToItemFullMergeDeletesOnlySourceItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  10,
	}}}); err != nil {
		t.Fatalf("seed full use-to-item template: %v", err)
	}
	owner := peerVisibilityCharacter("UseToItemFull", 0x01030214, 0x02040214, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1501, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 1502, Vnum: 27001, Count: 7, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "use-to-item-full", 0x64646464, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-full", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed full use-to-item owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected full use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-full", 0x64646464)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected full use-to-item error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected full use-to-item to emit source delete, target set, and source quickslot delete, got %d", len(out))
	}
	sourceDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode full use-to-item source delete: %v", err)
	}
	if sourceDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected full use-to-item source delete: %+v", sourceDel)
	}
	targetSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode full use-to-item target set: %v", err)
	}
	if targetSet.Position != itemproto.InventoryPosition(6) || targetSet.Vnum != 27001 || targetSet.Count != 9 {
		t.Fatalf("unexpected full use-to-item target set: %+v", targetSet)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode full use-to-item quickslot delete: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("expected full use-to-item to delete only source item quickslot position 2, got %+v", quickslotDel)
	}
	account, err := accounts.Load("use-to-item-full")
	if err != nil {
		t.Fatalf("load persisted full use-to-item owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 1502, Vnum: 27001, Count: 9, Slot: 6}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("full use-to-item persisted inventory mismatch: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}, {Position: 4, Type: quickslotproto.TypeItem, Slot: 6}}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("full use-to-item persisted quickslots mismatch: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemMoveAntiStackTemplateSwapsSameVnumStacksAndRetargetsQuickslots(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Bound Practice Potion",
		Stackable: true,
		MaxCount:  200,
		AntiStack: true,
	}}}); err != nil {
		t.Fatalf("seed anti-stack item template: %v", err)
	}
	owner := peerVisibilityCharacter("MoveAntiStack", 0x01030201, 0x02040201, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1101, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1102, Vnum: 27001, Count: 4, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "move-antistack-owner", 0x51515151, owner)
	if err := accounts.Save(accountstore.Account{Login: "move-antistack-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed anti-stack move owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected anti-stack item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "move-antistack-owner", 0x51515151)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected anti-stack item move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected anti-stack same-vnum move to swap with two item set frames, got %d", len(out))
	}
	sourceSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode anti-stack source set frame: %v", err)
	}
	if sourceSet.Position != itemproto.InventoryPosition(5) || sourceSet.Vnum != 27001 || sourceSet.Count != 4 {
		t.Fatalf("unexpected anti-stack source set frame: %+v", sourceSet)
	}
	targetSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode anti-stack target set frame: %v", err)
	}
	if targetSet.Position != itemproto.InventoryPosition(6) || targetSet.Vnum != 27001 || targetSet.Count != 3 {
		t.Fatalf("unexpected anti-stack target set frame: %+v", targetSet)
	}
	account, err := accounts.Load("move-antistack-owner")
	if err != nil {
		t.Fatalf("load anti-stack move owner account: %v", err)
	}
	if len(account.Characters) != 1 {
		t.Fatalf("expected one persisted anti-stack move owner, got %+v", account)
	}
	wantInventory := map[uint64]inventory.ItemInstance{
		1101: {ID: 1101, Vnum: 27001, Count: 3, Slot: 6},
		1102: {ID: 1102, Vnum: 27001, Count: 4, Slot: 5},
	}
	gotInventory := make(map[uint64]inventory.ItemInstance, len(account.Characters[0].Inventory))
	for _, item := range account.Characters[0].Inventory {
		gotInventory[item.ID] = item
	}
	if !reflect.DeepEqual(gotInventory, wantInventory) {
		t.Fatalf("anti-stack same-vnum move did not persist swap: got %#v want %#v", gotInventory, wantInventory)
	}
}

func TestGameSessionFlowPracticeMobItemMoveFailsClosedAfterImmediateRetaliationReachesOwnerHPFloor(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ItemMoveDeadOwner", 0x01030211, 0x02040211, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Inventory = []inventory.ItemInstance{{ID: 1101, Vnum: 27001, Count: 3, Slot: 5}}
	issuePeerTicket(t, store, "item-move-dead-owner", 0x52525252, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-move-dead-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed immediate zero-HP item-move owner account: %v", err)
	}

	staticActorStore := staticstore.NewFileStore(t.TempDir() + "/static-actors.json")
	interactionStore := newInteractionDefinitionStore(t, nil)
	runtime, err := newGameRuntimeWithAccountStoreAndContentStores(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, staticActorStore, interactionStore)
	if err != nil {
		t.Fatalf("unexpected item-move/practice-mob runtime error: %v", err)
	}
	currentTime := time.Unix(1700000820, 0)
	runtime.now = func() time.Time { return currentTime }
	bundle := contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref:           "practice.mob_alpha",
		Name:          "PracticeMobAlpha",
		MapIndex:      bootstrapMapIndex,
		X:             1200,
		Y:             2200,
		RaceNum:       101,
		CombatProfile: string(worldruntime.StaticActorCombatProfileTrainingDummy),
	}}}
	if _, err := runtime.ImportContentBundle(bundle); err != nil {
		t.Fatalf("import content practice-mob bundle for immediate zero-HP item-move denial: %v", err)
	}
	actors := runtime.StaticActors()
	if len(actors) != 1 {
		t.Fatalf("expected one content-loaded practice mob before immediate zero-HP item-move denial, got %#v", actors)
	}
	targetVID := uint32(actors[0].EntityID)

	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-move-dead-owner", 0x52525252)
	defer closeSessionFlow(t, flow)
	if len(enterOut) < 8 {
		t.Fatalf("expected item-move/practice-mob owner bootstrap to emit at least 8 frames, got %d", len(enterOut))
	}
	selectOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: targetVID})))
	if err != nil {
		t.Fatalf("unexpected target selection error before immediate zero-HP item-move denial: %v", err)
	}
	if len(selectOut) != 1 {
		t.Fatalf("expected target selection to emit 1 frame before immediate zero-HP item-move denial, got %d", len(selectOut))
	}
	attackOut, err := flow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{
		AttackType: combatproto.ClientAttackTypeNormal,
		TargetVID:  targetVID,
	})))
	if err != nil {
		t.Fatalf("unexpected attack error before immediate zero-HP item-move denial: %v", err)
	}
	if len(attackOut) != 4 {
		t.Fatalf("expected immediate retaliation floor attack to emit 4 frames before immediate zero-HP item-move denial, got %d", len(attackOut))
	}
	currentTime = currentTime.Add(time.Second)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no delayed retaliation frames after immediate zero-HP item-move denial setup, got %d", len(queued))
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected item move error after immediate retaliation reached owner HP floor: %v", err)
	}
	if len(moveOut) != 0 {
		t.Fatalf("expected item move to fail closed once immediate retaliation reached owner HP floor, got %d frames", len(moveOut))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected immediate zero-HP item-move denial not to queue frames, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-move-dead-owner")
	if err != nil {
		t.Fatalf("load persisted immediate zero-HP item-move owner account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected immediate zero-HP item-move denial to keep persisted inventory unchanged, got %#v want %#v", persisted.Characters[0].Inventory, owner.Inventory)
	}
}
