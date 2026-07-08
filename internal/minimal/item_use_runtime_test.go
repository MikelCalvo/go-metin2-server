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

func TestGameSessionFlowItemUseToItemRejectsAtBootstrapHPFloorWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemDead", 0x0103052d, 0x0204052d, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 0
	owner.Inventory = []inventory.ItemInstance{
		{ID: 203, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 204, Vnum: 27001, Count: 3, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "uit-hp-floor", 0x5050502d, owner)
	if err := accounts.Save(accountstore.Account{Login: "uit-hp-floor", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed hp-floor item-use-to-item account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Floor Potion", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected hp-floor item-use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "uit-hp-floor", 0x5050502d)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected hp-floor item-use-to-item packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected hp-floor ITEM_USE_TO_ITEM to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after hp-floor ITEM_USE_TO_ITEM rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("uit-hp-floor")
	if err != nil {
		t.Fatalf("load persisted hp-floor item-use-to-item account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("hp-floor ITEM_USE_TO_ITEM mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("hp-floor ITEM_USE_TO_ITEM mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameSessionFlowItemUseToItemRejectsRuntimeWideMaxWithoutMutation(t *testing.T) {
	cases := []struct {
		name            string
		login           string
		inventory       []inventory.ItemInstance
		template        itemcatalog.Template
		runtimeTemplate *itemcatalog.Template
	}{
		{
			name:  "runtime max-count exceeds item refresh byte",
			login: "uit-wide-max",
			inventory: []inventory.ItemInstance{
				{ID: 195, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 196, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Wide Max Store Potion", Stackable: true, MaxCount: 255},
			runtimeTemplate: &itemcatalog.Template{
				Vnum:      27001,
				Name:      "Wide Max Runtime Potion",
				Stackable: true,
				MaxCount:  256,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemEdge", 0x0103051b, 0x0204051b, 1100, 2100, 0, 101, 201)
			owner.Inventory = append([]inventory.ItemInstance(nil), tc.inventory...)
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			issuePeerTicket(t, ticketStore, tc.login, 0x5050501b, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s item-use-to-item account: %v", tc.name, err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{tc.template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s item-use-to-item runtime error: %v", tc.name, err)
			}
			if tc.runtimeTemplate != nil {
				runtime.itemTemplates[tc.runtimeTemplate.Vnum] = *tc.runtimeTemplate
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, 0x5050501b)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
			if err != nil {
				t.Fatalf("unexpected %s item-use-to-item packet error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s ITEM_USE_TO_ITEM to emit no frames, got %d", tc.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s ITEM_USE_TO_ITEM rejection, got %d", tc.name, len(queued))
			}
			persisted, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load persisted %s item-use-to-item account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s ITEM_USE_TO_ITEM mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s ITEM_USE_TO_ITEM mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameSessionFlowItemUseToItemRejectsMissingAuthoredTemplateWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemMissing", 0x0103052b, 0x0204052b, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "uit-missing-template", 0x5050506b, owner)
	if err := accounts.Save(accountstore.Account{Login: "uit-missing-template", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed missing-template item-use-to-item account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27002, Name: "Other Potion", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected missing-template item-use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "uit-missing-template", 0x5050506b)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected missing-template item-use-to-item packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected missing-template ITEM_USE_TO_ITEM to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after missing-template ITEM_USE_TO_ITEM rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("uit-missing-template")
	if err != nil {
		t.Fatalf("load persisted missing-template item-use-to-item account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("missing-template ITEM_USE_TO_ITEM mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("missing-template ITEM_USE_TO_ITEM mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameSessionFlowItemUseToItemRejectsLockedAndCountEdgesWithoutMutation(t *testing.T) {
	cases := []struct {
		name         string
		inventory    []inventory.ItemInstance
		template     itemcatalog.Template
		source       uint16
		target       uint16
		sourceWindow uint8
		targetWindow uint8
		level        uint8
		job          uint8
		raceNum      uint16
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
			name: "duplicate source slot occupancy",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 1, Slot: 5},
				{ID: 203, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Duplicate Source Slot Potion", Stackable: true, MaxCount: 200},
		},
		{
			name: "duplicate target slot occupancy",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 1, Slot: 6},
				{ID: 203, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Duplicate Target Slot Potion", Stackable: true, MaxCount: 200},
		},
		{
			name: "different target vnum",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27002, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Different Target Potion", Stackable: true, MaxCount: 200},
		},
		{
			name: "empty source slot",
			inventory: []inventory.ItemInstance{
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Empty Source Potion", Stackable: true, MaxCount: 200},
		},
		{
			name: "empty target slot",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Empty Target Potion", Stackable: true, MaxCount: 200},
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
		{
			name: "source position outside inventory window",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template:     itemcatalog.Template{Vnum: 27001, Name: "Source Window Potion", Stackable: true, MaxCount: 200},
			sourceWindow: itemproto.WindowEquipment,
		},
		{
			name: "target position outside inventory window",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template:     itemcatalog.Template{Vnum: 27001, Name: "Target Window Potion", Stackable: true, MaxCount: 200},
			targetWindow: itemproto.WindowEquipment,
		},
		{
			name: "source outside carried inventory range",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Out Of Range Source Potion", Stackable: true, MaxCount: 200},
			source:   uint16(inventory.CarriedInventorySlotCount),
			target:   6,
		},
		{
			name: "target outside carried inventory range",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Out Of Range Target Potion", Stackable: true, MaxCount: 200},
			source:   5,
			target:   uint16(inventory.CarriedInventorySlotCount),
		},
		{
			name: "anti warrior template",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Warrior Restricted Stack Potion", Stackable: true, MaxCount: 200, AntiWarrior: true},
			job:      0,
			raceNum:  0,
		},
		{
			name: "anti male template",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Male Restricted Stack Potion", Stackable: true, MaxCount: 200, AntiMale: true},
			job:      0,
			raceNum:  0,
		},
		{
			name: "min-level template",
			inventory: []inventory.ItemInstance{
				{ID: 201, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 202, Vnum: 27001, Count: 3, Slot: 6},
			},
			template: itemcatalog.Template{Vnum: 27001, Name: "Veteran Stack Potion", Stackable: true, MaxCount: 200, MinLevel: 10},
			level:    5,
		},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemGuard", 0x0103052c, 0x0204052c, 1100, 2100, tc.raceNum, 101, 201)
			owner.Job = tc.job
			owner.RaceNum = tc.raceNum
			if tc.level != 0 {
				owner.Level = tc.level
			}
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
			sourceWindow := tc.sourceWindow
			if sourceWindow == 0 {
				sourceWindow = itemproto.WindowInventory
			}
			targetWindow := tc.targetWindow
			if targetWindow == 0 {
				targetWindow = itemproto.WindowInventory
			}
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.Position{WindowType: sourceWindow, Cell: source}, Target: itemproto.Position{WindowType: targetWindow, Cell: target}})))
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

func TestGameSessionFlowItemUseRejectsDuplicateSlotOccupancyWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseDuplicateSlot", 0x0103057e, 0x0204057e, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 351, Vnum: 27004, Count: 2, Slot: 5},
		{ID: 352, Vnum: 27004, Count: 3, Slot: 5},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	issuePeerTicket(t, ticketStore, "item-use-duplicate-slot", 0x5050507e, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-duplicate-slot", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed duplicate-slot item-use account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27004,
		Name:      "Duplicate Slot Template Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected duplicate-slot item-use runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-duplicate-slot", 0x5050507e)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected duplicate-slot item-use packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected duplicate-slot item-use to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after duplicate-slot item-use rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-use-duplicate-slot")
	if err != nil {
		t.Fatalf("load persisted duplicate-slot item-use account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("duplicate-slot item-use mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("duplicate-slot item-use mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("duplicate-slot item-use mutated point value: got %d want %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameSessionFlowItemUseRejectsTransferGuardTemplatesWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		login    string
		template itemcatalog.Template
	}{
		{
			name:     "anti-stack",
			login:    "item-use-anti-stack",
			template: itemcatalog.Template{Vnum: 27004, Name: "Anti Stack Template Potion", Stackable: true, MaxCount: 200, AntiStack: true, UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"}},
		},
		{
			name:     "anti-drop",
			login:    "item-use-anti-drop",
			template: itemcatalog.Template{Vnum: 27004, Name: "Anti Drop Template Potion", Stackable: true, MaxCount: 200, AntiDrop: true, UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"}},
		},
		{
			name:     "anti-give",
			login:    "item-use-anti-give",
			template: itemcatalog.Template{Vnum: 27004, Name: "Anti Give Template Potion", Stackable: true, MaxCount: 200, AntiGive: true, UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"}},
		},
		{
			name:     "anti-sell",
			login:    "item-use-anti-sell",
			template: itemcatalog.Template{Vnum: 27004, Name: "Anti Sell Template Potion", Stackable: true, MaxCount: 200, AntiSell: true, UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"}},
		},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseTransferGuard", 0x0103059e+uint32(index), 0x0204059e+uint32(index), 1100, 2100, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: uint64(451 + index), Vnum: 27004, Count: 2, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			owner.Points[bootstrapPlayerPointValueIndex] = 25
			issuePeerTicket(t, ticketStore, tc.login, 0x5050509e+uint32(index), owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s item-use account: %v", tc.name, err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{tc.template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s item-use runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, 0x5050509e+uint32(index))
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
			if err != nil {
				t.Fatalf("unexpected %s item-use packet error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s item-use to emit no frames, got %d", tc.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s item-use rejection, got %d", tc.name, len(queued))
			}
			persisted, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load persisted %s item-use account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s item-use mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s item-use mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
			if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
				t.Fatalf("%s item-use mutated point value: got %d want %d", tc.name, persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
			}
		})
	}
}

func TestGameSessionFlowItemUseRejectsMinLevelTemplateWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseLowLevel", 0x0103056c, 0x0204056c, 1100, 2100, 0, 101, 201)
	owner.Level = 5
	owner.Inventory = []inventory.ItemInstance{{ID: 351, Vnum: 27004, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	issuePeerTicket(t, ticketStore, "item-use-low-level", 0x5050507c, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-low-level", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed min-level item-use account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27004,
		Name:      "Veteran Template Potion",
		Stackable: true,
		MaxCount:  200,
		MinLevel:  10,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected min-level item-use runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-low-level", 0x5050507c)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected min-level item-use packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected min-level item-use to emit no frames, got %d", len(out))
	}
	persisted, err := accounts.Load("item-use-low-level")
	if err != nil {
		t.Fatalf("load persisted min-level item-use account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("min-level item-use mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("min-level item-use mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("min-level item-use mutated point value: got %d want %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameSessionFlowItemUseRejectsPointOverflowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseOverflow", 0x0103057c, 0x0204057c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 451, Vnum: 27004, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	owner.Points[bootstrapPlayerPointValueIndex] = 1<<31 - 25
	issuePeerTicket(t, ticketStore, "item-use-overflow", 0x5050507d, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-overflow", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed overflow item-use account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27004,
		Name:      "Overflow Template Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected overflow item-use runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-overflow", 0x5050507d)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected overflow item-use packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected overflowing item-use to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after overflowing item-use rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-use-overflow")
	if err != nil {
		t.Fatalf("load persisted overflow item-use account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("overflow item-use mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("overflow item-use mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("overflow item-use mutated point value: got %d want %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameSessionFlowItemUseRejectsOverUint8TemplateMaxWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseWideMax", 0x0103058f, 0x0204058f, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 452, Vnum: 27004, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	issuePeerTicket(t, ticketStore, "item-use-wide-max", 0x5050508f, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-wide-max", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed over-uint8 item-use account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27004,
		Name:      "Wide Max Template Potion",
		Stackable: true,
		MaxCount:  255,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected over-uint8 item-use runtime error: %v", err)
	}
	runtime.itemTemplates[27004] = itemcatalog.Template{
		Vnum:      27004,
		Name:      "Runtime Wide Max Template Potion",
		Stackable: true,
		MaxCount:  256,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"},
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-wide-max", 0x5050508f)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected over-uint8 item-use packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected over-uint8 item-use to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after over-uint8 item-use rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-use-wide-max")
	if err != nil {
		t.Fatalf("load persisted over-uint8 item-use account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("over-uint8 item-use mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("over-uint8 item-use mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("over-uint8 item-use mutated point value: got %d want %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
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

func TestGameSessionFlowItemUseRejectsOverTemplateMaxStackWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseOverMax", 0x0103059d, 0x0204059d, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 501, Vnum: 27001, Count: 201, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	issuePeerTicket(t, ticketStore, "item-use-over-max", 0x505050bd, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-over-max", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed over-template-max item-use account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Over Max Template Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected over-template-max item-use runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-over-max", 0x505050bd)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected over-template-max item-use packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected over-template-max item-use to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after over-template-max item-use rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-use-over-max")
	if err != nil {
		t.Fatalf("load persisted over-template-max item-use account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("over-template-max item-use mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("over-template-max item-use mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("over-template-max item-use mutated point value: got %d want %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameSessionFlowItemUseRejectsJobAndSexAntiFlagTemplatesWithoutMutation(t *testing.T) {
	cases := []struct {
		name    string
		login   string
		job     uint8
		raceNum uint16
		mutate  func(*itemcatalog.Template)
	}{
		{name: "anti warrior", login: "item-use-anti-warrior", job: 0, raceNum: 0, mutate: func(template *itemcatalog.Template) { template.AntiWarrior = true }},
		{name: "anti assassin", login: "item-use-anti-assassin", job: 1, raceNum: 1, mutate: func(template *itemcatalog.Template) { template.AntiAssassin = true }},
		{name: "anti sura", login: "item-use-anti-sura", job: 2, raceNum: 2, mutate: func(template *itemcatalog.Template) { template.AntiSura = true }},
		{name: "anti shaman", login: "item-use-anti-shaman", job: 3, raceNum: 3, mutate: func(template *itemcatalog.Template) { template.AntiShaman = true }},
		{name: "anti male", login: "item-use-anti-male", job: 0, raceNum: 0, mutate: func(template *itemcatalog.Template) { template.AntiMale = true }},
		{name: "anti female", login: "item-use-anti-female", job: 1, raceNum: 1, mutate: func(template *itemcatalog.Template) { template.AntiFemale = true }},
		{name: "anti stack", login: "item-use-anti-stack", job: 0, raceNum: 0, mutate: func(template *itemcatalog.Template) { template.AntiStack = true }},
		{name: "anti drop", login: "item-use-anti-drop", job: 0, raceNum: 0, mutate: func(template *itemcatalog.Template) { template.AntiDrop = true }},
		{name: "anti give", login: "item-use-anti-give", job: 0, raceNum: 0, mutate: func(template *itemcatalog.Template) { template.AntiGive = true }},
		{name: "anti sell", login: "item-use-anti-sell", job: 0, raceNum: 0, mutate: func(template *itemcatalog.Template) { template.AntiSell = true }},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseRestricted", 0x0103058c+uint32(index), 0x0204058c+uint32(index), 1100, 2100, tc.raceNum, 101, 201)
			owner.Job = tc.job
			owner.RaceNum = tc.raceNum
			owner.Inventory = []inventory.ItemInstance{{ID: 401, Vnum: 27001, Count: 2, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			owner.Points[bootstrapPlayerPointValueIndex] = 25
			issuePeerTicket(t, ticketStore, tc.login, 0x505050ac+uint32(index), owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s item-use anti-flag account: %v", tc.name, err)
			}
			template := itemcatalog.Template{
				Vnum:      27001,
				Name:      "Restricted Template Potion",
				Stackable: true,
				MaxCount:  200,
				UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "must not consume"},
			}
			tc.mutate(&template)
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s item-use anti-flag runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, 0x505050ac+uint32(index))
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
			if err != nil {
				t.Fatalf("unexpected %s item-use anti-flag packet error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s item-use anti-flag request to fail closed without frames, got %d", tc.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s item-use anti-flag rejection, got %d", tc.name, len(queued))
			}
			persisted, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load persisted %s item-use anti-flag account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s item-use anti-flag rejection mutated inventory: got %+v want %+v", tc.name, persisted.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s item-use anti-flag rejection mutated quickslots: got %+v want %+v", tc.name, persisted.Characters[0].Quickslots, owner.Quickslots)
			}
			if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
				t.Fatalf("%s item-use anti-flag rejection mutated point value: got %d want %d", tc.name, persisted.Characters[0].Points[bootstrapPlayerPointValueIndex], owner.Points[bootstrapPlayerPointValueIndex])
			}
		})
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

func TestGameSessionFlowItemUseToItemPartialMergePreservesSourceAndTargetQuickslots(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemPartial", 0x0103053c, 0x0204053c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 5, Type: quickslotproto.TypeSkill, Slot: 6},
	}
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
	wantQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 5, Type: quickslotproto.TypeSkill, Slot: 6},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("expected partial item-use-to-item to preserve source and target quickslots, got %+v", persisted.Characters[0].Quickslots)
	}
}

func TestGameSessionFlowItemUseToItemRejectsOverUint8TemplateMaxWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemWideMax", 0x0103058e, 0x0204058e, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 211, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 212, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "uit-wide-max", 0x5050508e, owner)
	if err := accounts.Save(accountstore.Account{Login: "uit-wide-max", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-use-to-item over-uint8 template max account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Wide Max Template Potion",
		Stackable: true,
		MaxCount:  255,
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-use-to-item over-uint8 template max runtime error: %v", err)
	}
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "Runtime Wide Max Template Potion", Stackable: true, MaxCount: 256}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "uit-wide-max", 0x5050508e)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected item-use-to-item over-uint8 template max packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected over-uint8 source template ITEM_USE_TO_ITEM to emit no frames, got %d", len(out))
	}
	persisted, err := accounts.Load("uit-wide-max")
	if err != nil {
		t.Fatalf("load persisted item-use-to-item over-uint8 template max account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("over-uint8 source template ITEM_USE_TO_ITEM mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("over-uint8 source template ITEM_USE_TO_ITEM mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameSessionFlowItemUseToItemRejectsMismatchedSourceTemplateWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemMismatch", 0x0103057d, 0x0204057d, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "uit-mismatched-template", 0x5050503d, owner)
	if err := accounts.Save(accountstore.Account{Login: "uit-mismatched-template", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-use-to-item mismatched-template account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  10,
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-use-to-item mismatched-template runtime error: %v", err)
	}
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27002, Name: "Mismatched Runtime Template Potion", Stackable: true, MaxCount: 10}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "uit-mismatched-template", 0x5050503d)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected item-use-to-item mismatched-template packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected mismatched source template ITEM_USE_TO_ITEM to emit no frames, got %d", len(out))
	}
	persisted, err := accounts.Load("uit-mismatched-template")
	if err != nil {
		t.Fatalf("load persisted item-use-to-item mismatched-template account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("mismatched source template ITEM_USE_TO_ITEM mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("mismatched source template ITEM_USE_TO_ITEM mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameSessionFlowItemUseToItemRejectsMissingSourceTemplateWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemMissingTemplate", 0x0103057c, 0x0204057c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "uit-missing-template", 0x5050503c, owner)
	if err := accounts.Save(accountstore.Account{Login: "uit-missing-template", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-use-to-item missing-template account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27002,
		Name:      "Unrelated Template Potion",
		Stackable: true,
		MaxCount:  10,
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-use-to-item missing-template runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "uit-missing-template", 0x5050503c)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected item-use-to-item missing-template packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected missing source template ITEM_USE_TO_ITEM to emit no frames, got %d", len(out))
	}
	persisted, err := accounts.Load("uit-missing-template")
	if err != nil {
		t.Fatalf("load persisted item-use-to-item missing-template account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("missing source template ITEM_USE_TO_ITEM mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("missing source template ITEM_USE_TO_ITEM mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameSessionFlowItemUseToItemPartialMergePreservesAllTargetItemQuickslots(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemPartialMultiQS", 0x010305ac, 0x020405ac, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 7, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 8, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 6, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 7, Type: quickslotproto.TypeSkill, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "uit-partial-multi", 0x505050ac, owner)
	if err := accounts.Save(accountstore.Account{Login: "uit-partial-multi", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial multi-quickslot item-use-to-item account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  10,
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected partial multi-quickslot item-use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "uit-partial-multi", 0x505050ac)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected partial multi-quickslot item-use-to-item packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected partial merge to emit source and target updates only, got %d", len(out))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial multi-quickslot source update: %v", err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 5 {
		t.Fatalf("unexpected partial multi-quickslot source update: %+v", sourceUpdate)
	}
	targetUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial multi-quickslot target update: %v", err)
	}
	if targetUpdate.Position != itemproto.InventoryPosition(6) || targetUpdate.Count != 10 {
		t.Fatalf("unexpected partial multi-quickslot target update: %+v", targetUpdate)
	}
	persisted, err := accounts.Load("uit-partial-multi")
	if err != nil {
		t.Fatalf("load partial multi-quickslot item-use-to-item account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 10, Slot: 6},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted partial multi-quickslot inventory: got %+v want %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 6, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 7, Type: quickslotproto.TypeSkill, Slot: 6},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted partial multi-quickslot quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameSessionFlowItemUseToItemFullMergeDeletesAllSourceItemQuickslots(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemMultiQS", 0x0103059c, 0x0204059c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 10, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 5, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 6, Type: quickslotproto.TypeItem, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "item-use-to-item-multi-qs", 0x5050509c, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-to-item-multi-qs", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed multi-quickslot item-use-to-item account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  15,
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected multi-quickslot item-use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-to-item-multi-qs", 0x5050509c)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected multi-quickslot item-use-to-item packet error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected full item-use-to-item merge to emit source delete, target set, and two source quickslot deletes, got %d", len(out))
	}
	sourceDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode multi-quickslot item-use-to-item source delete: %v", err)
	}
	if sourceDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected multi-quickslot item-use-to-item source delete: %+v", sourceDel)
	}
	targetSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode multi-quickslot item-use-to-item target set: %v", err)
	}
	if targetSet.Position != itemproto.InventoryPosition(6) || targetSet.Vnum != 27001 || targetSet.Count != 15 {
		t.Fatalf("unexpected multi-quickslot item-use-to-item target set: %+v", targetSet)
	}
	firstQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode first multi-quickslot item-use-to-item quickslot delete: %v", err)
	}
	secondQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode second multi-quickslot item-use-to-item quickslot delete: %v", err)
	}
	if firstQuickslotDel.Position != 2 || secondQuickslotDel.Position != 4 {
		t.Fatalf("expected source item quickslot positions 2 and 4 to be deleted in order, got %+v and %+v", firstQuickslotDel, secondQuickslotDel)
	}

	persisted, err := accounts.Load("item-use-to-item-multi-qs")
	if err != nil {
		t.Fatalf("load multi-quickslot item-use-to-item account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 202, Vnum: 27001, Count: 15, Slot: 6}}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted multi-quickslot item-use-to-item inventory: got %+v want %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 5, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 6, Type: quickslotproto.TypeItem, Slot: 6},
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted multi-quickslot item-use-to-item quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameSessionFlowItemUseToItemStaleAfterReclaimIsSelfLocalOnly(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemStale", 0x0103054c, 0x0204054c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 201, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 202, Vnum: 27001, Count: 10, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-use-to-item-stale", 0x5050508c, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-to-item-stale", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed stale item-use-to-item account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  15,
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected stale item-use-to-item runtime error: %v", err)
	}
	staleFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-to-item-stale", 0x5050508c)
	closeSessionFlow(t, staleFlow)

	issuePeerTicket(t, ticketStore, "item-use-to-item-stale", 0x5050508d, owner)
	replacementFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-to-item-stale", 0x5050508d)
	defer closeSessionFlow(t, replacementFlow)

	staleOut, err := staleFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected stale item-use-to-item packet error: %v", err)
	}
	if len(staleOut) != 3 {
		t.Fatalf("expected stale item-use-to-item to emit self-local source delete, target set, and source quickslot delete only, got %d", len(staleOut))
	}
	staleSourceDel, err := itemproto.DecodeDel(decodeSingleFrame(t, staleOut[0]))
	if err != nil {
		t.Fatalf("decode stale item-use-to-item source delete: %v", err)
	}
	if staleSourceDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected stale item-use-to-item source delete: %+v", staleSourceDel)
	}
	staleTargetSet, err := itemproto.DecodeSet(decodeSingleFrame(t, staleOut[1]))
	if err != nil {
		t.Fatalf("decode stale item-use-to-item target set: %v", err)
	}
	if staleTargetSet.Position != itemproto.InventoryPosition(6) || staleTargetSet.Vnum != 27001 || staleTargetSet.Count != 15 {
		t.Fatalf("unexpected stale item-use-to-item target set: %+v", staleTargetSet)
	}
	staleQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, staleOut[2]))
	if err != nil {
		t.Fatalf("decode stale item-use-to-item quickslot delete: %v", err)
	}
	if staleQuickslotDel.Position != 2 {
		t.Fatalf("expected stale item-use-to-item to delete only source item quickslot position 2 locally, got %+v", staleQuickslotDel)
	}
	persisted, err := accounts.Load("item-use-to-item-stale")
	if err != nil {
		t.Fatalf("load stale item-use-to-item account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected stale item-use-to-item to leave authoritative inventory unchanged, got %+v", persisted.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected stale item-use-to-item to leave authoritative quickslots unchanged, got %+v", persisted.Characters[0].Quickslots)
	}

	replacementOut, err := replacementFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected replacement item-use-to-item packet error: %v", err)
	}
	if len(replacementOut) != 3 {
		t.Fatalf("expected replacement owner to still see the original full merge after stale local use, got %d frames", len(replacementOut))
	}
	replacementTargetSet, err := itemproto.DecodeSet(decodeSingleFrame(t, replacementOut[1]))
	if err != nil {
		t.Fatalf("decode replacement item-use-to-item target set: %v", err)
	}
	if replacementTargetSet.Position != itemproto.InventoryPosition(6) || replacementTargetSet.Count != 15 {
		t.Fatalf("expected replacement owner to merge from unchanged authoritative state, got %+v", replacementTargetSet)
	}
}

func TestGameSessionFlowItemUsePartialStackRefreshesItemAndPreservesQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UsePartialStackQS", 0x0103051d, 0x0204051d, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 106, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	issuePeerTicket(t, ticketStore, "item-use-partial-stack-qs", 0x5050505d, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-partial-stack-qs", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-use partial-stack owner account: %v", err)
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
		t.Fatalf("unexpected item-use partial-stack runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-partial-stack-qs", 0x5050505d)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected item-use partial-stack packet error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected item-use partial stack to emit point change, item set, and info chat only, got %d", len(out))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial-stack item-use point change: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != 50 || pointChange.Value != 75 {
		t.Fatalf("unexpected partial-stack item-use point change: %+v", pointChange)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial-stack item-use item set: %v", err)
	}
	if itemSet.Position != itemproto.InventoryPosition(5) || itemSet.Vnum != 27001 || itemSet.Count != 2 {
		t.Fatalf("unexpected partial-stack item-use item set: %+v", itemSet)
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode partial-stack item-use info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != "template consume" {
		t.Fatalf("unexpected partial-stack item-use info chat: %+v", infoChat)
	}

	persisted, err := accounts.Load("item-use-partial-stack-qs")
	if err != nil {
		t.Fatalf("load persisted item-use partial-stack account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 106, Vnum: 27001, Count: 2, Slot: 5}}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, wantInventory) {
		t.Fatalf("expected consumed partial stack to persist decremented inventory, got %+v want %+v", persisted.Characters[0].Inventory, wantInventory)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != 75 {
		t.Fatalf("expected persisted template-authored point value 75, got %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected partial stack consume to preserve quickslots, got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
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
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 5},
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
	if len(out) != 5 {
		t.Fatalf("expected item-use last stack to emit point change, item delete, two quickslot deletes, and info chat, got %d", len(out))
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
		t.Fatalf("decode first item-use quickslot del: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("expected first item quickslot position 2 to be deleted, got %+v", quickslotDel)
	}
	quickslotDel, err = quickslotproto.DecodeDel(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode second item-use quickslot del: %v", err)
	}
	if quickslotDel.Position != 4 {
		t.Fatalf("expected second item quickslot position 4 to be deleted, got %+v", quickslotDel)
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[4]))
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
		t.Fatalf("expected consumed last stack to delete only item quickslots, got %+v", persisted.Characters[0].Quickslots)
	}
}
