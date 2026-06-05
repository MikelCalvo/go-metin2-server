package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestGameSessionFlowItemUseToItemRejectsLockedAndCountEdgesWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
		template  itemcatalog.Template
		source    uint16
		target    uint16
	}{
		{
			name: "locked source",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5, Locked: true},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Locked Source Potion", Stackable: true, MaxCount: 200},
		},
		{
			name: "locked target",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6, Locked: true},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Locked Target Potion", Stackable: true, MaxCount: 200},
		},
		{
			name: "non-stackable template",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 1, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 1, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Single Potion", Stackable: false, MaxCount: 1},
		},
		{
			name: "equippable template",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Equippable Stack", Stackable: true, MaxCount: 200, EquipSlot: inventory.EquipmentSlotBody.String()},
		},
		{
			name: "anti-stack template",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Anti Stack Potion", Stackable: true, MaxCount: 200, AntiStack: true},
		},
		{
			name: "anti-drop template",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Anti Drop Potion", Stackable: true, MaxCount: 200, AntiDrop: true},
		},
		{
			name: "anti-give template",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Anti Give Potion", Stackable: true, MaxCount: 200, AntiGive: true},
		},
		{
			name: "anti-sell template",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Anti Sell Potion", Stackable: true, MaxCount: 200, AntiSell: true},
		},
		{
			name:      "same source and target cell",
			inventory: []inventory.ItemInstance{{ID: 201, Vnum: 27001, Count: 2, Slot: 5}},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Same Cell Potion", Stackable: true, MaxCount: 200},
			source:    5,
			target:    5,
		},
		{
			name: "duplicate source and target item ids",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 201, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Duplicate ID Potion", Stackable: true, MaxCount: 200},
		},
		{
			name: "already full target",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 200, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Full Target Potion", Stackable: true, MaxCount: 200},
		},
		{
			name: "source count above template max",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 201, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Over Max Source Potion", Stackable: true, MaxCount: 200},
		},
		{
			name: "target count above template max",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 201, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Over Max Target Potion", Stackable: true, MaxCount: 200},
		},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemGuard", 0x0103052c, 0x0204052c, 1100, 2100, 0, 101, 201)
			owner.Inventory = append([]inventory.ItemInstance(nil), tc.inventory...)
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			login := "uitguard" + string(rune('a'+index))
			issuePeerTicket(t, ticketStore, login, 0x5050506c, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed item-use-to-item guard account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{tc.template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected item-use-to-item guard runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x5050506c)
			defer closeSessionFlow(t, flow)

			source := tc.source
			if source == 0 {
				source = 5
			}
			target := tc.target
			if target == 0 {
				target = 6
			}
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(source), Target: itemproto.InventoryPosition(target)})))
			if err != nil {
				t.Fatalf("unexpected item-use-to-item guard packet error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s ITEM_USE_TO_ITEM guard to emit no frames, got %d", tc.name, len(out))
			}
			persisted, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load persisted item-use-to-item guard account: %v", err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s ITEM_USE_TO_ITEM guard mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s ITEM_USE_TO_ITEM guard mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameSessionFlowItemUseRejectsLockedStackWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseLocked", 0x0103054c, 0x0204054c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 301, Vnum: 27001, Count: 2, Slot: 5, Locked: true}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	issuePeerTicket(t, ticketStore, "item-use-locked", 0x5050508c, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-locked", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed locked item-use account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Locked Template Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "template consume"},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected locked item-use runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-locked", 0x5050508c)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected locked item-use packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected locked item-use to emit no frames, got %d", len(out))
	}
	persisted, err := accounts.Load("item-use-locked")
	if err != nil {
		t.Fatalf("load persisted locked item-use account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("locked item-use mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("locked item-use mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("locked item-use mutated point value: got %d want %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameSessionFlowItemUseToItemFullMergeDeletesOnlySourceItemQuickslotAndSkipsUseEffect(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemFull", 0x0103055c, 0x0204055c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	issuePeerTicket(t, ticketStore, "item-use-to-item-full", 0x5050509c, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-to-item-full", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-use-to-item full account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  15,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not run"},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-use-to-item full runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-to-item-full", 0x5050509c)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected item-use-to-item full packet error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected full item-use-to-item to emit source delete, target set, and source quickslot delete only, got %d", len(out))
	}
	sourceDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode full item-use-to-item source delete: %v", err)
	}
	if sourceDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected full item-use-to-item source delete: %+v", sourceDel)
	}
	targetSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode full item-use-to-item target set: %v", err)
	}
	if targetSet.Position != itemproto.InventoryPosition(6) || targetSet.Vnum != 27001 || targetSet.Count != 15 {
		t.Fatalf("unexpected full item-use-to-item target set: %+v", targetSet)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode full item-use-to-item quickslot delete: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("expected full item-use-to-item to delete only source item quickslot position 2, got %+v", quickslotDel)
	}
	persisted, err := accounts.Load("item-use-to-item-full")
	if err != nil {
		t.Fatalf("load persisted item-use-to-item full account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 202, Vnum: 27001, Count: 15, Slot: 6}}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted full item-use-to-item inventory: got %+v want %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("expected full item-use-to-item to delete only source item quickslot, got %+v", persisted.Characters[0].Quickslots)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected drag-to-item to skip use_effect point mutation, got %d want %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameSessionFlowItemUseToItemPartialMergePreservesSourceQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemPartial", 0x0103053c, 0x0204053c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-use-to-item-partial", 0x5050507c, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-to-item-partial", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-use-to-item partial account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  10,
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-use-to-item partial runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-to-item-partial", 0x5050507c)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected item-use-to-item partial packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected partial item-use-to-item to emit source and target updates only, got %d", len(out))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial item-use-to-item source update: %v", err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 5 {
		t.Fatalf("unexpected partial item-use-to-item source update: %+v", sourceUpdate)
	}
	targetUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial item-use-to-item target update: %v", err)
	}
	if targetUpdate.Position != itemproto.InventoryPosition(6) || targetUpdate.Count != 10 {
		t.Fatalf("unexpected partial item-use-to-item target update: %+v", targetUpdate)
	}
	persisted, err := accounts.Load("item-use-to-item-partial")
	if err != nil {
		t.Fatalf("load persisted item-use-to-item partial account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 10, Slot: 6},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted partial item-use-to-item inventory: got %+v want %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected partial item-use-to-item to preserve source quickslot, got %+v", persisted.Characters[0].Quickslots)
	}
}

func TestGameSessionFlowItemUseLastStackDeletesOnlyItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseLastStackQS", 0x0103051c, 0x0204051c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 105, Vnum: 27001, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	issuePeerTicket(t, ticketStore, "item-use-last-stack-qs", 0x5050505c, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-last-stack-qs", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-use last-stack owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "template consume"},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-use last-stack runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-last-stack-qs", 0x5050505c)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected item-use last-stack packet error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected item-use last stack to emit point change, item delete, quickslot delete, and info chat, got %d", len(out))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode item-use point change: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != 50 || pointChange.Value != 75 {
		t.Fatalf("unexpected item-use point change: %+v", pointChange)
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode item-use item del: %v", err)
	}
	if itemDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected item-use item del: %+v", itemDel)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode item-use quickslot del: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("expected only item quickslot position 2 to be deleted, got %+v", quickslotDel)
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode item-use info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != "template consume" {
		t.Fatalf("unexpected item-use info chat: %+v", infoChat)
	}

	persisted, err := accounts.Load("item-use-last-stack-qs")
	if err != nil {
		t.Fatalf("load persisted item-use last-stack account: %v", err)
	}
	if len(persisted.Characters[0].Inventory) != 0 {
		t.Fatalf("expected consumed last stack to clear persisted inventory, got %+v", persisted.Characters[0].Inventory)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != 75 {
		t.Fatalf("expected persisted template-authored point value 75, got %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}}) {
		t.Fatalf("expected consumed last stack to delete only item quickslot, got %+v", persisted.Characters[0].Quickslots)
	}
}
