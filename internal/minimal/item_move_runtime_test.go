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
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

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

func TestGameRuntimeItemMoveRejectsAntiStackTemplateCompatibleMergeWithoutMutation(t *testing.T) {
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
	if len(out) != 0 {
		t.Fatalf("expected anti-stack compatible move to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("move-antistack-owner")
	if err != nil {
		t.Fatalf("load anti-stack move owner account: %v", err)
	}
	if len(account.Characters) != 1 {
		t.Fatalf("expected one persisted anti-stack move owner, got %+v", account)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("anti-stack compatible move mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
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
