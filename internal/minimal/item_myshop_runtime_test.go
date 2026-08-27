package minimal

import (
	"math"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestGameRuntimeMyShopOpenEmitsShopSignWithoutInventoryMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopHost", 0x01030801, 0x02040801, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 801, Vnum: 27001, Count: 3, Slot: 5}, {ID: 851, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "myshop-host"
	issuePeerTicket(t, ticketStore, login, 0x70707101, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop host account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop host runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707101)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, out, owner.VID, 4, "accepted myshop open")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected accepted MYSHOP to queue no peer frames, got %d", len(queued))
	}
	want := owner
	want.Inventory = []inventory.ItemInstance{{ID: 801, Vnum: 27001, Count: 3, Slot: 5}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "accepted myshop open after bag consume")
}

func TestGameRuntimeMyShopOpenRejectsEmptySignAndZeroCountWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopEmpty", 0x01030802, 0x02040802, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 802, Vnum: 27001, Count: 3, Slot: 5}, {ID: 852, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	login := "myshop-empty"
	issuePeerTicket(t, ticketStore, login, 0x70707102, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop empty account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop empty runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707102)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	for _, tc := range []struct {
		name string
		pkt  shopproto.ClientMyShopPacket
	}{
		{name: "empty sign", pkt: shopproto.ClientMyShopPacket{Sign: "", Items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 1500}}}},
		{name: "zero count", pkt: shopproto.ClientMyShopPacket{Sign: "Private Shop"}},
	} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(tc.pkt)))
		if err != nil {
			t.Fatalf("unexpected %s MYSHOP error: %v", tc.name, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected %s MYSHOP to emit no frames, got %d", tc.name, len(out))
		}
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop empty/zero rejects")
}

func TestGameRuntimeMyShopOpenRejectsInvalidStockWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopStock", 0x01030803, 0x02040803, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 803, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 804, Vnum: 27002, Count: 1, Slot: 6, Locked: true},
		{ID: 806, Vnum: 27003, Count: 1, Slot: 7},
		{ID: 807, Vnum: 27004, Count: 1, Slot: 8},
		{ID: 808, Vnum: 27001, Count: 3, Slot: 9},
		{ID: 858, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "myshop-stock"
	issuePeerTicket(t, ticketStore, login, 0x70707103, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop stock account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27002, Name: "Locked Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27003, Name: "Anti MyShop Potion", Stackable: true, MaxCount: 200, AntiMyShop: true},
		{Vnum: 27004, Name: "Anti Give Potion", Stackable: true, MaxCount: 200, AntiGive: true},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop stock runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707103)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	validItem := shopproto.ClientMyShopItem{Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 0}
	for _, tc := range []struct {
		name        string
		items       []shopproto.ClientMyShopItem
		wantMessage string
	}{
		{name: "duplicate pos", items: []shopproto.ClientMyShopItem{validItem, {Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 2000, DisplayPos: 1}}},
		{name: "missing cell", items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 1, Position: itemproto.InventoryPosition(9), Price: 1500}}},
		{name: "locked cell", items: []shopproto.ClientMyShopItem{{Vnum: 27002, Count: 1, Position: itemproto.InventoryPosition(6), Price: 1500}}, wantMessage: myShopOpenLockedItemInfoMessage},
		{name: "zero price", items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 0}}},
		{name: "count mismatch", items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 2, Position: itemproto.InventoryPosition(5), Price: 1500}}},
		{name: "vnum mismatch", items: []shopproto.ClientMyShopItem{{Vnum: 27099, Count: 3, Position: itemproto.InventoryPosition(5), Price: 1500}}},
		{name: "anti_myshop", items: []shopproto.ClientMyShopItem{{Vnum: 27003, Count: 1, Position: itemproto.InventoryPosition(7), Price: 1500}}, wantMessage: myShopOpenCashItemInfoMessage},
		{name: "anti_give", items: []shopproto.ClientMyShopItem{{Vnum: 27004, Count: 1, Position: itemproto.InventoryPosition(8), Price: 1500}}, wantMessage: myShopOpenCashItemInfoMessage},
		{name: "duplicate display_pos", items: []shopproto.ClientMyShopItem{validItem, {Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(9), Price: 2000, DisplayPos: 0}}},
		{name: "out-of-range display_pos", items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 40}}},
	} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{Sign: "Private Shop", Items: tc.items})))
		if err != nil {
			t.Fatalf("unexpected %s MYSHOP error: %v", tc.name, err)
		}
		if tc.wantMessage == "" {
			if len(out) != 0 {
				t.Fatalf("expected %s MYSHOP to emit no frames, got %d", tc.name, len(out))
			}
			continue
		}
		assertMyShopOpenRejectInfoChat(t, out, tc.wantMessage, tc.name)
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop invalid stock rejects")
}

func TestGameRuntimeMyShopOpenRejectsGoldOverflowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopGoldCap", 0x01030804, 0x02040804, 1100, 2100, 0, 101, 201)
	owner.Gold = uint64(math.MaxInt32) - 100
	owner.Inventory = []inventory.ItemInstance{{ID: 808, Vnum: 27001, Count: 1, Slot: 5}, {ID: 858, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	login := "myshop-goldcap"
	issuePeerTicket(t, ticketStore, login, 0x70707104, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop goldcap account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop goldcap runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707104)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:     27001,
			Count:    1,
			Position: itemproto.InventoryPosition(5),
			Price:    101,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected gold-overflow MYSHOP error: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, out, myShopOpenGoldOverflowInfoMessage, "gold overflow")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop gold overflow")
}

func TestGameRuntimeMyShopOpenRejectsWornBodyArmorWithInfoChatWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopArmor", 0x01030807, 0x02040807, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 820, Vnum: 27001, Count: 1, Slot: 5}, {ID: 870, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	owner.Equipment = []inventory.ItemInstance{{ID: 821, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}}
	login := "myshop-armor"
	issuePeerTicket(t, ticketStore, login, 0x70707107, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop armor account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Shop Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String()},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop armor runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707107)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:     27001,
			Count:    1,
			Position: itemproto.InventoryPosition(5),
			Price:    1500,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected armor MYSHOP error: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, out, myShopOpenArmorRequiredInfoMessage, "worn body armor")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop armor reject")
}

func TestGameRuntimeMyShopOpenRejectsEquippedStockWithInfoChatWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopEquipped", 0x0103080a, 0x0204080a, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 840, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 890, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	owner.Equipment = []inventory.ItemInstance{
		{ID: 841, Vnum: 11210, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
		{ID: 842, Vnum: 11220, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotHead},
	}
	login := "myshop-equipped"
	issuePeerTicket(t, ticketStore, login, 0x7070710a, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop equipped account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: 11210, Name: "Shop Weapon", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String()},
		{Vnum: 11220, Name: "Shop Helmet", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotHead.String()},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop equipped runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x7070710a)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	headCombined, err := itemproto.EquipmentPosition(1)
	if err != nil {
		t.Fatalf("equipment head combined position: %v", err)
	}
	weaponWindow := itemproto.Position{WindowType: itemproto.WindowEquipment, Cell: 4}
	for _, tc := range []struct {
		name  string
		items []shopproto.ClientMyShopItem
	}{
		{
			name: "combined inventory equipment namespace head",
			items: []shopproto.ClientMyShopItem{{
				Vnum:     11220,
				Count:    1,
				Position: headCombined,
				Price:    1500,
			}},
		},
		{
			name: "window-equipment weapon",
			items: []shopproto.ClientMyShopItem{{
				Vnum:     11210,
				Count:    1,
				Position: weaponWindow,
				Price:    1500,
			}},
		},
	} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
			Sign:  "Private Shop",
			Items: tc.items,
		})))
		if err != nil {
			t.Fatalf("unexpected %s MYSHOP error: %v", tc.name, err)
		}
		assertMyShopOpenRejectInfoChat(t, out, myShopOpenEquippedItemInfoMessage, tc.name)
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop equipped stock rejects")
}

func TestGameRuntimeMyShopOpenRejectsMissingShopBagSilentlyWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopNoBag", 0x01030808, 0x02040808, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 830, Vnum: 27001, Count: 1, Slot: 5}}
	login := "myshop-nobag"
	issuePeerTicket(t, ticketStore, login, 0x70707108, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop nobag account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop nobag runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707108)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:     27001,
			Count:    1,
			Position: itemproto.InventoryPosition(5),
			Price:    1500,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected missing-bag MYSHOP error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected missing-bag MYSHOP to emit no frames, got %d", len(out))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop missing bag")
}

func TestGameRuntimeMyShopOpenRejectsListedOnlyShopBagSilentlyWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopListedBag", 0x01030809, 0x02040809, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 831, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 832, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "myshop-listed-bag"
	issuePeerTicket(t, ticketStore, login, 0x70707109, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop listed-bag account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop listed-bag runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707109)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{
			{Vnum: 27001, Count: 1, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 0},
			{Vnum: myShopOpenShopBagVnum, Count: 1, Position: itemproto.InventoryPosition(4), Price: 100, DisplayPos: 1},
		},
	})))
	if err != nil {
		t.Fatalf("unexpected listed-bag MYSHOP error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected listed-only bag MYSHOP to emit no frames, got %d", len(out))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop listed-only bag")
}

func TestGameRuntimeMyShopOpenBusyShellRejectsWithInfoChatWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("MyShopBusy", 0x01030805, 0x02040805, 12345, []inventory.ItemInstance{
		{ID: 809, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 810, Vnum: 11234, Count: 1, Slot: 6},
		{ID: 811, Vnum: 27001, Count: 2, Slot: 7},
		{ID: 861, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	})
	peer := peerVisibilityCharacter("MyShopBusyPeer", 0x01030806, 0x02040806, 1120, 2120, 0, 101, 201)
	login := "myshop-busy"
	peerLogin := "myshop-busy-peer"
	issuePeerTicket(t, ticketStore, login, 0x70707105, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707106, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop busy account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop busy peer account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	templates := append(defaultMerchantItemTemplates(),
		itemcatalog.Template{
			Vnum:       11234,
			Name:       "Busy Practice Blade",
			Stackable:  false,
			MaxCount:   1,
			Refineable: true,
			RefineInfo: &itemcatalog.RefineInfo{ResultVnum: 11235, Cost: 1000, Probability: 100, Materials: []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 2}}},
		},
		itemcatalog.Template{Vnum: 11235, Name: "Busy Result Blade", Stackable: false, MaxCount: 1},
		itemcatalog.Template{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	)
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected myshop busy runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707105)
	defer closeSessionFlow(t, flow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707106)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, peerFlow)

	openPacket := shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:     27001,
			Count:    3,
			Position: itemproto.InventoryPosition(5),
			Price:    1500,
		}},
	}

	interactWithMerchantForBuy(t, flow, actor.EntityID)
	assertMyShopBusyReject(t, flow, openPacket, "merchant busy")
	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected merchant close after myshop busy reject: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected merchant close after myshop busy reject to emit SHOP END, got %d", len(closeOut))
	}

	openSafeboxOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before myshop busy reject: %v", err)
	}
	if len(openSafeboxOut) == 0 {
		t.Fatal("expected /open_safebox before myshop busy reject to emit frames")
	}
	assertMyShopBusyReject(t, flow, openPacket, "safebox busy")
	closeSafeboxOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_safebox after myshop busy reject: %v", err)
	}
	if len(closeSafeboxOut) == 0 {
		t.Fatal("expected /close_safebox after myshop busy reject to emit frames")
	}

	previewOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 6, Type: 3})))
	if err != nil || len(previewOut) != 1 {
		t.Fatalf("expected refine busy preview to emit one frame, got %d err=%v", len(previewOut), err)
	}
	assertMyShopBusyReject(t, flow, openPacket, "refine busy")
	cancelRefineOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 6, Type: 255})))
	if err != nil {
		t.Fatalf("unexpected refine cancel after myshop busy reject: %v", err)
	}
	if len(cancelRefineOut) != 0 {
		t.Fatalf("expected refine cancel after myshop busy reject to emit no frames, got %d", len(cancelRefineOut))
	}

	startOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected exchange start before myshop busy reject: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange start before myshop busy reject to emit one frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)
	assertMyShopBusyReject(t, flow, openPacket, "exchange busy")
	cancelOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected exchange cancel after myshop busy reject: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected exchange cancel after myshop busy reject to emit one frame, got %d", len(cancelOut))
	}
	_ = flushServerFrames(t, peerFlow)

	acceptedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(openPacket)))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP after busy clears: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, acceptedOut, owner.VID, 4, `expected accepted MYSHOP after busy clears to emit one SHOP_SIGN frame, got %d`)
	assertMyShopBusyReject(t, flow, openPacket, "already-open myshop busy")

	assertExchangeAccountUnchanged(t, accounts, login, characterAfterMyShopBagConsume(owner), "myshop busy rejects")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop busy peer")
}

func TestGameRuntimeMyShopOpenRejectsActiveCubeWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopCubeBusy", 0x010308c1, 0x020408c1, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 1901, Vnum: 27001, Count: 3, Slot: 5}, {ID: 1951, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	login := "myshop-cube-busy"
	issuePeerTicket(t, ticketStore, login, 0x707071c1, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop-cube busy account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop-cube busy runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071c1)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openCubeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before myshop open: %v", err)
	}
	if len(openCubeOut) != 1 {
		t.Fatalf("expected /open_cube before myshop open to emit one command chat frame, got %d", len(openCubeOut))
	}
	assertCubeCommandChatFrame(t, openCubeOut[0], "cube open 20022", "cube before myshop open")

	assertMyShopBusyReject(t, flow, shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	}, "cube busy")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop-cube busy")
}

func TestGameRuntimeMyShopGuestBrowseRejectsActiveCubeWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopBrowseCubeHost", 0x010308c3, 0x020408c3, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 1903, Vnum: 27001, Count: 3, Slot: 5}, {ID: 1953, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopBrowseCubeGuest", 0x010308c4, 0x020408c4, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "myshop-browse-cube-host"
	peerLogin := "myshop-browse-cube-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707071c3, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707071c4, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop browse cube host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop browse cube guest account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop browse cube runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707071c3)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707071c4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected MYSHOP open error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `unexpected accepted MYSHOP before guest cube browse: out=%d err=%v`)
	_ = flushServerFrames(t, peerFlow)

	openCubeOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected guest /open_cube before browse: %v", err)
	}
	if len(openCubeOut) != 1 {
		t.Fatalf("expected guest /open_cube before browse to emit one command chat frame, got %d", len(openCubeOut))
	}
	assertCubeCommandChatFrame(t, openCubeOut[0], "cube open 20022", "guest cube before browse")

	busyOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil {
		t.Fatalf("unexpected cube-busy guest browse ON_CLICK: %v", err)
	}
	if len(busyOut) != 1 {
		t.Fatalf("expected cube-busy guest browse to emit one info-chat frame, got %d", len(busyOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, busyOut[0]))
	if err != nil {
		t.Fatalf("decode cube-busy guest browse info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected cube-busy guest browse chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected cube-busy guest browse to queue no host frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop guest browse cube host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop guest browse cube guest")
}

func assertMyShopBusyReject(t *testing.T, flow service.SessionFlow, packet shopproto.ClientMyShopPacket, context string) {
	t.Helper()
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(packet)))
	if err != nil {
		t.Fatalf("unexpected %s MYSHOP error: %v", context, err)
	}
	if len(out) != 1 {
		t.Fatalf("expected %s MYSHOP to emit one busy info-chat frame, got %d", context, len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode %s MYSHOP busy info chat: %v", context, err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected %s MYSHOP busy chat: %+v", context, delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected %s MYSHOP busy reject to queue no peer frames, got %d", context, len(queued))
	}
}

func assertMyShopOpenRejectInfoChat(t *testing.T, out [][]byte, wantMessage, context string) {
	t.Helper()
	if len(out) != 1 {
		t.Fatalf("expected %s MYSHOP reject to emit one info-chat frame, got %d", context, len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode %s MYSHOP reject info chat: %v", context, err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != wantMessage {
		t.Fatalf("unexpected %s MYSHOP reject chat: %+v", context, delivery)
	}
}

func assertMyShopOpenSuccessBagAndSign(t *testing.T, out [][]byte, wantVID uint32, bagSlot uint16, context string) {
	assertMyShopOpenSuccessBagAndSignWithText(t, out, wantVID, "Private Shop", bagSlot, context)
}

func assertMyShopOpenSuccessBagAndSignWithText(t *testing.T, out [][]byte, wantVID uint32, wantSign string, bagSlot uint16, context string) {
	t.Helper()
	if len(out) < 2 {
		t.Fatalf("expected %s MYSHOP to emit bag refresh then SHOP_SIGN, got %d", context, len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode %s MYSHOP bag ITEM_DEL: %v", context, err)
	}
	if del.Position != itemproto.InventoryPosition(bagSlot) {
		t.Fatalf("unexpected %s MYSHOP bag ITEM_DEL position: %+v", context, del.Position)
	}
	sign, err := shopproto.DecodeServerShopSign(decodeSingleFrame(t, out[len(out)-1]))
	if err != nil {
		t.Fatalf("decode %s MYSHOP SHOP_SIGN: %v", context, err)
	}
	if sign.VID != wantVID || sign.Sign != wantSign {
		t.Fatalf("unexpected %s MYSHOP SHOP_SIGN: %+v", context, sign)
	}
}

func characterAfterMyShopBagConsume(character loginticket.Character) loginticket.Character {
	updated := character
	updated.Inventory = cloneItemInstancesWithoutVnum(character.Inventory, myShopOpenShopBagVnum)
	updated.Quickslots = cloneQuickslotsWithoutItemSlot(character.Quickslots, character.Inventory, myShopOpenShopBagVnum)
	return updated
}

func cloneItemInstancesWithoutVnum(items []inventory.ItemInstance, vnum uint32) []inventory.ItemInstance {
	out := make([]inventory.ItemInstance, 0, len(items))
	removed := false
	for _, item := range items {
		if !removed && !item.Equipped && item.Vnum == vnum && item.Count > 0 {
			removed = true
			if item.Count > 1 {
				item.Count--
				out = append(out, item)
			}
			continue
		}
		out = append(out, item)
	}
	return out
}

func cloneQuickslotsWithoutItemSlot(slots []loginticket.Quickslot, items []inventory.ItemInstance, vnum uint32) []loginticket.Quickslot {
	bagSlots := make(map[uint8]struct{})
	for _, item := range items {
		if !item.Equipped && item.Vnum == vnum {
			bagSlots[uint8(item.Slot)] = struct{}{}
		}
	}
	if len(bagSlots) == 0 {
		return append([]loginticket.Quickslot(nil), slots...)
	}
	out := make([]loginticket.Quickslot, 0, len(slots))
	for _, slot := range slots {
		if slot.Type == quickslotproto.TypeItem {
			if _, ok := bagSlots[slot.Slot]; ok {
				continue
			}
		}
		out = append(out, slot)
	}
	return out
}

func TestGameRuntimeMyShopLifecycleCloseEmitsEmptyShopSignWithoutInventoryMutation(t *testing.T) {
	cases := []struct {
		name        string
		command     string
		wantCommand string
		wantPhase   session.Phase
		login       string
		ownerName   string
		ownerID     uint32
		ownerVID    uint32
		loginKey    uint32
	}{
		{
			name:        "quit",
			command:     "/quit",
			wantCommand: "quit",
			login:       "myshop-close-quit",
			ownerName:   "MyShopCloseQuit",
			ownerID:     0x01030811,
			ownerVID:    0x02040811,
			loginKey:    0x70707111,
		},
		{
			name:      "logout",
			command:   "/logout",
			wantPhase: session.PhaseClose,
			login:     "myshop-close-logout",
			ownerName: "MyShopCloseLogout",
			ownerID:   0x01030812,
			ownerVID:  0x02040812,
			loginKey:  0x70707112,
		},
		{
			name:      "phase_select",
			command:   "/phase_select",
			wantPhase: session.PhaseSelect,
			login:     "myshop-close-select",
			ownerName: "MyShopCloseSelect",
			ownerID:   0x01030813,
			ownerVID:  0x02040813,
			loginKey:  0x70707113,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter(tc.ownerName, tc.ownerID, tc.ownerVID, 1100, 2100, 0, 101, 201)
			owner.Gold = 5000
			owner.Inventory = []inventory.ItemInstance{{ID: uint64(tc.ownerID), Vnum: 27001, Count: 3, Slot: 5}, {ID: 9000, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			issuePeerTicket(t, ticketStore, tc.login, tc.loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s myshop close account: %v", tc.name, err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:      27001,
				Name:      "Shop Potion",
				Stackable: true,
				MaxCount:  200,
			}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s myshop close runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.loginKey)
			defer closeSessionFlow(t, flow)
			_ = flushServerFrames(t, flow)

			openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
				Sign: "Private Shop",
				Items: []shopproto.ClientMyShopItem{{
					Vnum:       27001,
					Count:      3,
					Position:   itemproto.InventoryPosition(5),
					Price:      1500,
					DisplayPos: 0,
				}},
			})))
			if err != nil {
				t.Fatalf("unexpected %s accepted MYSHOP error: %v", tc.name, err)
			}
			assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, tc.name+" accepted myshop open")

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: tc.command,
			})))
			if err != nil {
				t.Fatalf("unexpected %s myshop lifecycle close error: %v", tc.name, err)
			}
			if len(out) != 2 {
				t.Fatalf("expected %s myshop lifecycle close to emit empty SHOP_SIGN plus lifecycle frame, got %d", tc.name, len(out))
			}
			assertMyShopEmptySignFrame(t, out[0], owner.VID, tc.name+" lifecycle empty sign")
			if tc.wantCommand != "" {
				delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
				if err != nil {
					t.Fatalf("decode %s lifecycle command after myshop close: %v", tc.name, err)
				}
				if delivery.Type != chatproto.ChatTypeCommand || delivery.Message != tc.wantCommand {
					t.Fatalf("unexpected %s lifecycle command after myshop close: %+v", tc.name, delivery)
				}
			} else {
				phase, err := control.DecodePhase(decodeSingleFrame(t, out[1]))
				if err != nil {
					t.Fatalf("decode %s phase after myshop close: %v", tc.name, err)
				}
				if phase.Phase != tc.wantPhase {
					t.Fatalf("expected %s to transition to phase %q after myshop close, got %q", tc.name, tc.wantPhase, phase.Phase)
				}
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected %s myshop lifecycle close to queue no peer frames, got %d", tc.name, len(queued))
			}
			assertExchangeAccountUnchanged(t, accounts, tc.login, characterAfterMyShopBagConsume(owner), tc.name+" myshop lifecycle close")
		})
	}
}

func TestGameRuntimeMyShopCloseSlashClearsOpenSignWithoutInventoryMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopCloseSlash", 0x01030814, 0x02040814, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 814, Vnum: 27001, Count: 3, Slot: 5}, {ID: 864, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "myshop-close-slash"
	issuePeerTicket(t, ticketStore, login, 0x70707114, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop close slash account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop close slash runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707114)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before close slash: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before close slash to emit one SHOP_SIGN frame, got %d`)

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_myshop error: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected /close_myshop to emit one empty SHOP_SIGN frame, got %d", len(closeOut))
	}
	assertMyShopEmptySignFrame(t, closeOut[0], owner.VID, "close_myshop")

	alreadyClosedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /close_myshop error: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /close_myshop to emit no frames, got %d", len(alreadyClosedOut))
	}

	lifecycleOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/phase_select",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed phase_select error: %v", err)
	}
	if len(lifecycleOut) != 1 {
		t.Fatalf("expected already-closed phase_select to emit only the phase frame, got %d", len(lifecycleOut))
	}
	phase, err := control.DecodePhase(decodeSingleFrame(t, lifecycleOut[0]))
	if err != nil {
		t.Fatalf("decode already-closed phase_select: %v", err)
	}
	if phase.Phase != session.PhaseSelect {
		t.Fatalf("expected already-closed phase_select to transition to select, got %q", phase.Phase)
	}
	assertExchangeAccountUnchanged(t, accounts, login, characterAfterMyShopBagConsume(owner), "myshop close slash")
}

func assertMyShopEmptySignFrame(t *testing.T, raw []byte, wantVID uint32, context string) {
	t.Helper()
	sign, err := shopproto.DecodeServerShopSign(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode %s empty SHOP_SIGN: %v", context, err)
	}
	if sign.VID != wantVID || sign.Sign != "" {
		t.Fatalf("unexpected %s empty SHOP_SIGN: %+v", context, sign)
	}
}

func TestGameRuntimeItemExchangeStartRejectsActiveMyShopWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchMyShopStartOwner", 0x01030821, 0x02040821, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 821, Vnum: 27001, Count: 3, Slot: 5}, {ID: 871, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("ExchMyShopStartPeer", 0x01030822, 0x02040822, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "exch-myshop-start-owner"
	peerLogin := "exch-myshop-start-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707121, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707122, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop-open exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop-open exchange peer account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop-open exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707121)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707122)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before exchange start: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before exchange start to emit one SHOP_SIGN frame, got %d`)
	if queued := flushServerFrames(t, peerFlow); len(queued) != 1 {
		t.Fatalf("expected accepted MYSHOP before exchange start to around-broadcast one SHOP_SIGN to peer, got %d", len(queued))
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected myshop-open exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange start with open MYSHOP to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode myshop-open exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected myshop-open exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected exchange start with open MYSHOP to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop-open exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop-open exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsPartnerActiveMyShopWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchPartnerMyShopOwner", 0x01030823, 0x02040823, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 823, Vnum: 27001, Count: 3, Slot: 5}, {ID: 873, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("ExchPartnerMyShopPeer", 0x01030824, 0x02040824, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 824, Vnum: 27001, Count: 2, Slot: 6}, {ID: 874, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	ownerLogin := "exch-partner-myshop-owner"
	peerLogin := "exch-partner-myshop-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707123, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707124, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partner-myshop exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partner-myshop exchange peer account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected partner-myshop exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707123)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707124)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Partner Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      2,
			Position:   itemproto.InventoryPosition(6),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected partner accepted MYSHOP before exchange start: %v", err)
	}
	assertMyShopOpenSuccessBagAndSignWithText(t, openOut, peer.VID, "Partner Shop", 4, "partner accepted MYSHOP before exchange start")

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected partner-myshop exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-myshop exchange start to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode partner-myshop exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner-myshop exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected partner-myshop exchange start to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "partner-myshop exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, characterAfterMyShopBagConsume(peer), "partner-myshop exchange start peer")
}

func TestGameRuntimeMyShopOpenLocksHostItemMutationsWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopLockHost", 0x01030831, 0x02040831, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 831, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 832, Vnum: 27002, Count: 1, Slot: 6},
		{ID: 882, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "myshop-lock-host"
	issuePeerTicket(t, ticketStore, login, 0x70707131, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop lock host account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200, UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 5, Message: "consume:27001:+5"}},
		{Vnum: 27002, Name: "Spare Potion", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop lock runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707131)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before mutation lock: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before mutation lock to emit one SHOP_SIGN frame, got %d`)

	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{name: "ITEM_USE", raw: itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})},
		{name: "ITEM_MOVE", raw: itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: itemproto.InventoryPosition(5), Destination: itemproto.InventoryPosition(7)})},
		{name: "ITEM_DROP", raw: itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})},
		{name: "ITEM_DROP2", raw: itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 1})},
		{name: "ITEM_USE_TO_ITEM", raw: itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})},
		{name: "/use_item", raw: chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 5"})},
		{name: "/inventory_move", raw: chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/inventory_move 5 7"})},
	} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, tc.raw))
		if err != nil {
			t.Fatalf("unexpected %s while MYSHOP open error: %v", tc.name, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected %s while MYSHOP open to emit no frames, got %d", tc.name, len(out))
		}
	}
	assertExchangeAccountUnchanged(t, accounts, login, characterAfterMyShopBagConsume(owner), "myshop open host mutation lock")

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_myshop after mutation lock: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected /close_myshop after mutation lock to emit one empty SHOP_SIGN frame, got %d", len(closeOut))
	}

	useOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected ITEM_USE after MYSHOP close: %v", err)
	}
	if len(useOut) == 0 {
		t.Fatal("expected ITEM_USE after MYSHOP close to emit frames")
	}
}

func TestGameRuntimeMyShopOpenBroadcastsShopSignToVisiblePeer(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopAroundHost", 0x01030841, 0x02040841, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 841, Vnum: 27001, Count: 3, Slot: 5}, {ID: 891, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopAroundPeer", 0x01030842, 0x02040842, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "myshop-around-host"
	peerLogin := "myshop-around-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707141, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707142, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop around host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop around peer account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop around runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707141)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707142)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before peer around-broadcast: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP to emit one host SHOP_SIGN frame, got %d`)

	queued := flushServerFrames(t, peerFlow)
	if len(queued) != 1 {
		t.Fatalf("expected visible peer to receive one live SHOP_SIGN around-broadcast, got %d", len(queued))
	}
	peerSign, err := shopproto.DecodeServerShopSign(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode peer MYSHOP SHOP_SIGN around-broadcast: %v", err)
	}
	if peerSign.VID != owner.VID || peerSign.Sign != "Private Shop" {
		t.Fatalf("unexpected peer MYSHOP SHOP_SIGN around-broadcast: %+v", peerSign)
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop peer around-broadcast open host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop peer around-broadcast open peer")

	closeOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_myshop after peer around-broadcast: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected /close_myshop to emit one empty host SHOP_SIGN frame, got %d", len(closeOut))
	}
	assertMyShopEmptySignFrame(t, closeOut[0], owner.VID, "host close after around-broadcast")

	closeQueued := flushServerFrames(t, peerFlow)
	if len(closeQueued) != 1 {
		t.Fatalf("expected visible peer to receive one empty SHOP_SIGN around-broadcast on close, got %d", len(closeQueued))
	}
	assertMyShopEmptySignFrame(t, closeQueued[0], owner.VID, "peer close around-broadcast")
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop peer around-broadcast close host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop peer around-broadcast close peer")
}

func TestGameRuntimeMyShopOpenRematerializesShopSignOnPeerViewEntry(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopViewEntryHost", 0x01030843, 0x02040843, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 843, Vnum: 27001, Count: 3, Slot: 5}, {ID: 893, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopViewEntryPeer", 0x01030844, 0x02040844, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "myshop-view-entry-host"
	peerLogin := "myshop-view-entry-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707143, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707144, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop view-entry host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop view-entry peer account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop view-entry runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707143)
	defer closeSessionFlow(t, ownerFlow)
	_ = flushServerFrames(t, ownerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before peer view-entry: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before peer view-entry to emit one host SHOP_SIGN frame, got %d`)

	peerFlow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707144)
	defer closeSessionFlow(t, peerFlow)

	var rematerialized *shopproto.ServerShopSignPacket
	for _, raw := range enterOut {
		payload := decodeSingleFrame(t, raw)
		sign, err := shopproto.DecodeServerShopSign(payload)
		if err != nil {
			continue
		}
		if rematerialized != nil {
			t.Fatalf("expected exactly one rematerialized live SHOP_SIGN on peer view-entry, got extras")
		}
		copied := sign
		rematerialized = &copied
	}
	if rematerialized == nil {
		queued := flushServerFrames(t, peerFlow)
		for _, raw := range queued {
			payload := decodeSingleFrame(t, raw)
			sign, err := shopproto.DecodeServerShopSign(payload)
			if err != nil {
				continue
			}
			if rematerialized != nil {
				t.Fatalf("expected exactly one rematerialized live SHOP_SIGN on peer view-entry queue, got extras")
			}
			copied := sign
			rematerialized = &copied
		}
	}
	if rematerialized == nil {
		t.Fatal("expected newly visible peer to receive one rematerialized live SHOP_SIGN for already-open host")
	}
	if rematerialized.VID != owner.VID || rematerialized.Sign != "Private Shop" {
		t.Fatalf("unexpected rematerialized MYSHOP SHOP_SIGN: %+v", rematerialized)
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop peer view-entry rematerialization host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop peer view-entry rematerialization peer")
}

func TestGameRuntimeMyShopClosedBeforePeerViewEntryOmitsLiveShopSign(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopClosedViewHost", 0x01030845, 0x02040845, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 845, Vnum: 27001, Count: 3, Slot: 5}, {ID: 895, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopClosedViewPeer", 0x01030846, 0x02040846, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "myshop-closed-view-host"
	peerLogin := "myshop-closed-view-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707145, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707146, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop closed view-entry host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop closed view-entry peer account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Shop Potion",
		Stackable: true,
		MaxCount:  200,
	}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop closed view-entry runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707145)
	defer closeSessionFlow(t, ownerFlow)
	_ = flushServerFrames(t, ownerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before closed view-entry: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before closed view-entry to emit one host SHOP_SIGN frame, got %d`)
	closeOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_myshop before peer view-entry: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected /close_myshop before peer view-entry to emit one empty SHOP_SIGN frame, got %d", len(closeOut))
	}

	peerFlow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707146)
	defer closeSessionFlow(t, peerFlow)
	for _, raw := range enterOut {
		payload := decodeSingleFrame(t, raw)
		if sign, err := shopproto.DecodeServerShopSign(payload); err == nil {
			t.Fatalf("expected closed host not to rematerialize live SHOP_SIGN on peer view-entry, got %+v", sign)
		}
	}
	for _, raw := range flushServerFrames(t, peerFlow) {
		payload := decodeSingleFrame(t, raw)
		if sign, err := shopproto.DecodeServerShopSign(payload); err == nil {
			t.Fatalf("expected closed host not to queue rematerialized SHOP_SIGN on peer view-entry, got %+v", sign)
		}
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop closed view-entry host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop closed view-entry peer")
}

func TestGameRuntimeMyShopGuestBrowseOpenEmitsShopStartWithoutInventoryMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopBrowseHost", 0x01030851, 0x02040851, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 851, Vnum: 27001, Count: 3, Slot: 5}, {ID: 901, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopBrowseGuest", 0x01030852, 0x02040852, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "myshop-browse-host"
	peerLogin := "myshop-browse-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707151, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707152, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop browse host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop browse guest account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:       27001,
		Name:       "Shop Potion",
		Stackable:  true,
		MaxCount:   200,
		Sockets:    itemcatalog.SocketValues{11, -22, 33},
		Attributes: itemcatalog.AttributeValues{{Type: 3, Value: 30}, {Type: 4, Value: -5}},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop browse runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707151)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707152)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 7,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP before guest browse: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected accepted MYSHOP before guest browse to emit one SHOP_SIGN frame, got %d`)
	_ = flushServerFrames(t, peerFlow)

	browseOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil {
		t.Fatalf("unexpected guest browse ON_CLICK: %v", err)
	}
	if len(browseOut) != 1 {
		t.Fatalf("expected guest browse ON_CLICK to emit one SHOP START frame, got %d", len(browseOut))
	}
	start, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0]))
	if err != nil {
		t.Fatalf("decode guest browse SHOP START: %v", err)
	}
	if start.OwnerVID != owner.VID {
		t.Fatalf("unexpected guest browse OwnerVID: got %#08x want %#08x", start.OwnerVID, owner.VID)
	}
	want := shopproto.ItemEntry{
		Vnum:       27001,
		Price:      1500,
		Count:      3,
		DisplayPos: 7,
		Sockets:    [itemproto.ItemSocketCount]int32{11, -22, 33},
		Attributes: [itemproto.ItemAttributeCount]itemproto.Attribute{{Type: 3, Value: 30}, {Type: 4, Value: -5}},
	}
	if start.Items[7] != want {
		t.Fatalf("unexpected guest browse display slot 7: %+v want %+v", start.Items[7], want)
	}
	for i, item := range start.Items {
		if i == 7 {
			continue
		}
		if item != (shopproto.ItemEntry{}) {
			t.Fatalf("expected empty guest browse display slot %d, got %+v", i, item)
		}
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected guest browse to queue no host frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop guest browse host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop guest browse guest")
}

func TestGameRuntimeMyShopGuestBrowseRejectsBusyGuestAndSilentMisses(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopBrowseBusyHost", 0x01030853, 0x02040853, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 853, Vnum: 27001, Count: 3, Slot: 5}, {ID: 903, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopBrowseBusyGuest", 0x01030854, 0x02040854, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "myshop-browse-busy-host"
	peerLogin := "myshop-browse-busy-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707153, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707154, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop browse busy host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop browse busy guest account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop browse busy runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707153)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707154)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected MYSHOP open error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `unexpected accepted MYSHOP before busy guest browse: out=%d err=%v`)
	_ = flushServerFrames(t, peerFlow)

	openSafeboxOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil || len(openSafeboxOut) == 0 {
		t.Fatalf("expected /open_safebox before guest browse busy reject: out=%d err=%v", len(openSafeboxOut), err)
	}
	busyOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil {
		t.Fatalf("unexpected busy guest browse ON_CLICK: %v", err)
	}
	if len(busyOut) != 1 {
		t.Fatalf("expected busy guest browse to emit one info-chat frame, got %d", len(busyOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, busyOut[0]))
	if err != nil {
		t.Fatalf("decode busy guest browse info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected busy guest browse chat: %+v", delivery)
	}
	closeSafeboxOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_safebox",
	})))
	if err != nil || len(closeSafeboxOut) == 0 {
		t.Fatalf("expected /close_safebox after guest browse busy reject: out=%d err=%v", len(closeSafeboxOut), err)
	}

	missOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: 0x02999999})))
	if err != nil {
		t.Fatalf("unexpected silent miss ON_CLICK: %v", err)
	}
	if len(missOut) != 0 {
		t.Fatalf("expected unknown VID ON_CLICK to emit no frames, got %d", len(missOut))
	}

	closeOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil || len(closeOut) != 1 {
		t.Fatalf("unexpected /close_myshop before closed browse miss: out=%d err=%v", len(closeOut), err)
	}
	_ = flushServerFrames(t, peerFlow)
	closedOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil {
		t.Fatalf("unexpected closed-host browse ON_CLICK: %v", err)
	}
	if len(closedOut) != 0 {
		t.Fatalf("expected closed-host browse to emit no frames, got %d", len(closedOut))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop guest browse busy host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop guest browse busy guest")
}

func TestGameRuntimeMyShopGuestBrowseRejectsGuestOwnOpenMyShopSilently(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopBrowseOwnHost", 0x01030855, 0x02040855, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 855, Vnum: 27001, Count: 3, Slot: 5}, {ID: 905, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopBrowseOwnGuest", 0x01030856, 0x02040856, 1120, 2120, 0, 101, 201)
	peer.Gold = 5000
	peer.Inventory = []inventory.ItemInstance{{ID: 856, Vnum: 27001, Count: 1, Slot: 5}, {ID: 906, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	ownerLogin := "myshop-browse-own-host"
	peerLogin := "myshop-browse-own-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707155, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707156, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop browse own host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop browse own guest account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop browse own runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707155)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707156)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	for _, tc := range []struct {
		flow  service.SessionFlow
		login string
		pkt   shopproto.ClientMyShopPacket
	}{
		{flow: ownerFlow, login: ownerLogin, pkt: shopproto.ClientMyShopPacket{Sign: "Host Shop", Items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 0}}}},
		{flow: peerFlow, login: peerLogin, pkt: shopproto.ClientMyShopPacket{Sign: "Guest Shop", Items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 1, Position: itemproto.InventoryPosition(5), Price: 900, DisplayPos: 1}}}},
	} {
		out, err := tc.flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(tc.pkt)))
		if err != nil {
			t.Fatalf("unexpected accepted MYSHOP for %s before own-open browse: %v", tc.login, err)
		}
		wantVID := owner.VID
		wantSign := tc.pkt.Sign
		if tc.login == peerLogin {
			wantVID = peer.VID
		}
		assertMyShopOpenSuccessBagAndSignWithText(t, out, wantVID, wantSign, 4, "accepted MYSHOP for "+tc.login+" before own-open browse")
	}
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	ownOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil {
		t.Fatalf("unexpected own-open guest browse ON_CLICK: %v", err)
	}
	if len(ownOut) != 0 {
		t.Fatalf("expected own-open guest browse to emit no frames, got %d", len(ownOut))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop guest browse own-open host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, characterAfterMyShopBagConsume(peer), "myshop guest browse own-open guest")
}

func TestGameRuntimeMyShopGuestBrowseShopEndClearsPresentationWithoutInventoryMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopLeaveHost", 0x01030861, 0x02040861, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 861, Vnum: 27001, Count: 3, Slot: 5}, {ID: 911, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopLeaveGuest", 0x01030862, 0x02040862, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "myshop-leave-host"
	peerLogin := "myshop-leave-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707161, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707162, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop leave host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop leave guest account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop leave runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707161)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707162)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected MYSHOP open error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `unexpected accepted MYSHOP before guest leave: out=%d err=%v`)
	_ = flushServerFrames(t, peerFlow)

	browseOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil || len(browseOut) != 1 {
		t.Fatalf("unexpected guest browse before leave: out=%d err=%v", len(browseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP START before leave: %v", err)
	}

	endOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected guest browse SHOP END: %v", err)
	}
	if len(endOut) != 1 {
		t.Fatalf("expected guest browse SHOP END to emit one GC::SHOP END frame, got %d", len(endOut))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, endOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP END: %v", err)
	}
	secondEndOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected second guest browse SHOP END: %v", err)
	}
	if len(secondEndOut) != 0 {
		t.Fatalf("expected already-closed guest browse SHOP END to emit no frames, got %d", len(secondEndOut))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop guest leave host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop guest leave guest")
}

func TestGameRuntimeMyShopHostCloseQueuesGuestBrowseShopEndWithoutInventoryMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopHostCloseHost", 0x01030871, 0x02040871, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 871, Vnum: 27001, Count: 3, Slot: 5}, {ID: 921, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopHostCloseGuest", 0x01030872, 0x02040872, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "myshop-host-close-host"
	peerLogin := "myshop-host-close-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707171, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707172, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop host-close host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop host-close guest account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200}, {Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop host-close runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707171)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707172)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected MYSHOP open error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `unexpected accepted MYSHOP before host close: out=%d err=%v`)
	_ = flushServerFrames(t, peerFlow)

	browseOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil || len(browseOut) != 1 {
		t.Fatalf("unexpected guest browse before host close: out=%d err=%v", len(browseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP START before host close: %v", err)
	}

	closeOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/close_myshop"})))
	if err != nil {
		t.Fatalf("unexpected host /close_myshop: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected host /close_myshop to emit one empty SHOP_SIGN, got %d", len(closeOut))
	}
	closeSign, err := shopproto.DecodeServerShopSign(decodeSingleFrame(t, closeOut[0]))
	if err != nil {
		t.Fatalf("decode host empty SHOP_SIGN: %v", err)
	}
	if closeSign.VID != owner.VID || closeSign.Sign != "" {
		t.Fatalf("unexpected host empty SHOP_SIGN: vid=%#x sign=%q", closeSign.VID, closeSign.Sign)
	}

	guestQueued := flushServerFrames(t, peerFlow)
	if len(guestQueued) != 2 {
		t.Fatalf("expected guest queued SHOP END plus empty SHOP_SIGN, got %d", len(guestQueued))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, guestQueued[0])); err != nil {
		t.Fatalf("decode guest queued SHOP END after host close: %v", err)
	}
	peerSign, err := shopproto.DecodeServerShopSign(decodeSingleFrame(t, guestQueued[1]))
	if err != nil {
		t.Fatalf("decode guest queued empty SHOP_SIGN: %v", err)
	}
	if peerSign.VID != owner.VID || peerSign.Sign != "" {
		t.Fatalf("unexpected guest queued empty SHOP_SIGN: vid=%#x sign=%q", peerSign.VID, peerSign.Sign)
	}

	secondEndOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected guest SHOP END after host close: %v", err)
	}
	if len(secondEndOut) != 0 {
		t.Fatalf("expected guest SHOP END after host-forced leave to emit no frames, got %d", len(secondEndOut))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop host-close host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop host-close guest")
}

func TestGameRuntimeMyShopGuestBuyTransfersStockGoldAndClearsDisplaySlot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopBuyHost", 0x01030861, 0x02040861, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 861, Vnum: 27001, Count: 3, Slot: 5}, {ID: 911, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopBuyGuest", 0x01030862, 0x02040862, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "myshop-buy-host"
	peerLogin := "myshop-buy-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707161, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707162, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop buy host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop buy guest account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:       27001,
		Name:       "Shop Potion",
		Stackable:  true,
		MaxCount:   200,
		Sockets:    itemcatalog.SocketValues{11, -22, 33},
		Attributes: itemcatalog.AttributeValues{{Type: 3, Value: 30}, {Type: 4, Value: -5}},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop buy runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707161)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707162)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	const listedPrice uint32 = 1500
	const displayPos uint8 = 7
	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      listedPrice,
			DisplayPos: displayPos,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected MYSHOP open error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `unexpected accepted MYSHOP before guest buy: out=%d err=%v`)
	_ = flushServerFrames(t, peerFlow)

	browseOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil || len(browseOut) != 1 {
		t.Fatalf("unexpected guest browse before buy: out=%d err=%v", len(browseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP START before buy: %v", err)
	}

	buyOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{
		RawLeadingByte: 1,
		CatalogSlot:    displayPos,
	})))
	if err != nil {
		t.Fatalf("unexpected guest private-shop SHOP BUY: %v", err)
	}
	if len(buyOut) == 0 {
		t.Fatalf("expected guest private-shop SHOP BUY to mutate and emit frames, got none")
	}

	var sawGuestGold, sawGuestItem, sawUpdateItem bool
	for _, raw := range buyOut {
		f := decodeSingleFrame(t, raw)
		if update, err := shopproto.DecodeServerUpdateItem(f); err == nil {
			if update.Position != displayPos || update.Item.Vnum != 0 {
				t.Fatalf("unexpected guest UPDATE_ITEM after buy: %+v", update)
			}
			sawUpdateItem = true
			continue
		}
		if point, err := worldproto.DecodePlayerPointChange(f); err == nil {
			if point.Type == bootstrapGoldPointType && point.Amount == -int32(listedPrice) && uint64(point.Value) == peer.Gold-uint64(listedPrice) {
				sawGuestGold = true
			}
			continue
		}
		if set, err := itemproto.DecodeSet(f); err == nil {
			if set.Vnum == 27001 && set.Count == 3 {
				sawGuestItem = true
			}
			continue
		}
		if update, err := itemproto.DecodeUpdate(f); err == nil {
			if update.Count == 3 {
				sawGuestItem = true
			}
			continue
		}
	}
	if !sawGuestGold {
		t.Fatalf("expected guest gold PLAYER_POINT_CHANGE debit of %d in buy burst, frames=%d", listedPrice, len(buyOut))
	}
	if !sawGuestItem {
		t.Fatalf("expected guest inventory refresh granting vnum 27001 x3 in buy burst, frames=%d", len(buyOut))
	}
	if !sawUpdateItem {
		t.Fatalf("expected guest UPDATE_ITEM(vnum=0) for sold display slot %d in buy burst, frames=%d", displayPos, len(buyOut))
	}

	hostQueued := flushServerFrames(t, ownerFlow)
	if len(hostQueued) == 0 {
		t.Fatalf("expected host queued inventory/gold refresh after guest buy, got none")
	}
	var sawHostGold, sawHostItemClear bool
	for _, raw := range hostQueued {
		f := decodeSingleFrame(t, raw)
		if point, err := worldproto.DecodePlayerPointChange(f); err == nil {
			if point.Type == bootstrapGoldPointType && point.Amount == int32(listedPrice) && uint64(point.Value) == owner.Gold+uint64(listedPrice) {
				sawHostGold = true
			}
			continue
		}
		if _, err := itemproto.DecodeDel(f); err == nil {
			sawHostItemClear = true
			continue
		}
		if update, err := itemproto.DecodeUpdate(f); err == nil && update.Count == 0 {
			sawHostItemClear = true
			continue
		}
	}
	if !sawHostGold {
		t.Fatalf("expected host gold PLAYER_POINT_CHANGE credit of %d after guest buy, frames=%d", listedPrice, len(hostQueued))
	}
	if !sawHostItemClear {
		t.Fatalf("expected host inventory clear/refresh after guest buy, frames=%d", len(hostQueued))
	}

	ownerAccount, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load host account after guest buy: %v", err)
	}
	peerAccount, err := accounts.Load(peerLogin)
	if err != nil {
		t.Fatalf("load guest account after guest buy: %v", err)
	}
	persistedOwner := findPersistedCharacter(t, ownerAccount, owner.Name)
	persistedPeer := findPersistedCharacter(t, peerAccount, peer.Name)
	if persistedOwner.Gold != owner.Gold+uint64(listedPrice) {
		t.Fatalf("unexpected persisted host gold: got %d want %d", persistedOwner.Gold, owner.Gold+uint64(listedPrice))
	}
	if persistedPeer.Gold != peer.Gold-uint64(listedPrice) {
		t.Fatalf("unexpected persisted guest gold: got %d want %d", persistedPeer.Gold, peer.Gold-uint64(listedPrice))
	}
	if len(persistedOwner.Inventory) != 0 {
		t.Fatalf("expected persisted host inventory empty after guest buy, got %+v", persistedOwner.Inventory)
	}
	if len(persistedPeer.Inventory) != 1 || persistedPeer.Inventory[0].Vnum != 27001 || persistedPeer.Inventory[0].Count != 3 {
		t.Fatalf("unexpected persisted guest inventory after buy: %+v", persistedPeer.Inventory)
	}

	secondBuyOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{
		RawLeadingByte: 1,
		CatalogSlot:    displayPos,
	})))
	if err != nil {
		t.Fatalf("unexpected second guest private-shop SHOP BUY: %v", err)
	}
	if len(secondBuyOut) != 1 {
		t.Fatalf("expected second guest buy of sold slot to emit one sold-out/invalid frame, got %d", len(secondBuyOut))
	}
	secondFrame := decodeSingleFrame(t, secondBuyOut[0])
	if err := shopproto.DecodeServerSoldOut(secondFrame); err != nil {
		if err := shopproto.DecodeServerSoldout(secondFrame); err != nil {
			if err := shopproto.DecodeServerInvalidPos(secondFrame); err != nil {
				t.Fatalf("expected second buy sold-out/invalid companion, decode errors: %v", err)
			}
		}
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, persistedOwner, "myshop guest buy second host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, persistedPeer, "myshop guest buy second guest")
}

func TestGameRuntimeMyShopGuestBuyFansUpdateItemToOtherBrowsingGuest(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopFanHost", 0x01030871, 0x02040871, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 871, Vnum: 27001, Count: 3, Slot: 5}, {ID: 921, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	buyer := peerVisibilityCharacter("MyShopFanBuyer", 0x01030872, 0x02040872, 1120, 2120, 0, 101, 201)
	buyer.Gold = 22222
	watcher := peerVisibilityCharacter("MyShopFanWatch", 0x01030873, 0x02040873, 1140, 2140, 0, 101, 201)
	watcher.Gold = 33333
	ownerLogin := "myshop-fan-host"
	buyerLogin := "myshop-fan-buyer"
	watcherLogin := "myshop-fan-watch"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707171, owner)
	issuePeerTicket(t, ticketStore, buyerLogin, 0x70707172, buyer)
	issuePeerTicket(t, ticketStore, watcherLogin, 0x70707173, watcher)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop fan host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: buyerLogin, Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed myshop fan buyer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: watcherLogin, Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed myshop fan watcher account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:       27001,
		Name:       "Shop Potion",
		Stackable:  true,
		MaxCount:   200,
		Sockets:    itemcatalog.SocketValues{11, -22, 33},
		Attributes: itemcatalog.AttributeValues{{Type: 3, Value: 30}, {Type: 4, Value: -5}},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop fan runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707171)
	defer closeSessionFlow(t, ownerFlow)
	buyerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), buyerLogin, 0x70707172)
	defer closeSessionFlow(t, buyerFlow)
	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), watcherLogin, 0x70707173)
	defer closeSessionFlow(t, watcherFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, buyerFlow)
	_ = flushServerFrames(t, watcherFlow)

	const listedPrice uint32 = 1500
	const displayPos uint8 = 7
	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      listedPrice,
			DisplayPos: displayPos,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected MYSHOP open error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `unexpected accepted MYSHOP before multi-guest buy: out=%d err=%v`)
	_ = flushServerFrames(t, buyerFlow)
	_ = flushServerFrames(t, watcherFlow)

	buyerBrowseOut, err := buyerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil || len(buyerBrowseOut) != 1 {
		t.Fatalf("unexpected buyer browse before multi-guest buy: out=%d err=%v", len(buyerBrowseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, buyerBrowseOut[0])); err != nil {
		t.Fatalf("decode buyer browse SHOP START before multi-guest buy: %v", err)
	}
	watcherBrowseOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil || len(watcherBrowseOut) != 1 {
		t.Fatalf("unexpected watcher browse before multi-guest buy: out=%d err=%v", len(watcherBrowseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, watcherBrowseOut[0])); err != nil {
		t.Fatalf("decode watcher browse SHOP START before multi-guest buy: %v", err)
	}
	_ = flushServerFrames(t, watcherFlow)

	buyOut, err := buyerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{
		RawLeadingByte: 1,
		CatalogSlot:    displayPos,
	})))
	if err != nil {
		t.Fatalf("unexpected multi-guest private-shop SHOP BUY: %v", err)
	}
	var sawBuyerUpdateItem bool
	for _, raw := range buyOut {
		if update, err := shopproto.DecodeServerUpdateItem(decodeSingleFrame(t, raw)); err == nil {
			if update.Position != displayPos || update.Item.Vnum != 0 {
				t.Fatalf("unexpected buyer UPDATE_ITEM after multi-guest buy: %+v", update)
			}
			sawBuyerUpdateItem = true
		}
	}
	if !sawBuyerUpdateItem {
		t.Fatalf("expected buyer UPDATE_ITEM(vnum=0) for sold display slot %d in buy burst, frames=%d", displayPos, len(buyOut))
	}

	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) == 0 {
		t.Fatalf("expected watcher queued UPDATE_ITEM after other guest buy, got none")
	}
	var sawWatcherUpdateItem bool
	for _, raw := range watcherQueued {
		f := decodeSingleFrame(t, raw)
		if update, err := shopproto.DecodeServerUpdateItem(f); err == nil {
			if update.Position != displayPos || update.Item.Vnum != 0 {
				t.Fatalf("unexpected watcher UPDATE_ITEM after other guest buy: %+v", update)
			}
			sawWatcherUpdateItem = true
			continue
		}
		if _, err := itemproto.DecodeSet(f); err == nil {
			t.Fatalf("watcher must not receive inventory ITEM_SET after other guest buy")
		}
		if _, err := itemproto.DecodeUpdate(f); err == nil {
			t.Fatalf("watcher must not receive inventory ITEM_UPDATE after other guest buy")
		}
		if _, err := itemproto.DecodeDel(f); err == nil {
			t.Fatalf("watcher must not receive inventory ITEM_DEL after other guest buy")
		}
		if point, err := worldproto.DecodePlayerPointChange(f); err == nil && point.Type == bootstrapGoldPointType {
			t.Fatalf("watcher must not receive gold PLAYER_POINT_CHANGE after other guest buy: %+v", point)
		}
	}
	if !sawWatcherUpdateItem {
		t.Fatalf("expected watcher UPDATE_ITEM(vnum=0) for sold display slot %d after other guest buy, frames=%d", displayPos, len(watcherQueued))
	}

	assertExchangeAccountUnchanged(t, accounts, watcherLogin, watcher, "myshop multi-guest watcher")

	watcherBuyOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{
		RawLeadingByte: 1,
		CatalogSlot:    displayPos,
	})))
	if err != nil {
		t.Fatalf("unexpected watcher private-shop SHOP BUY of sold slot: %v", err)
	}
	if len(watcherBuyOut) != 1 {
		t.Fatalf("expected watcher buy of sold slot to emit one sold-out/invalid frame, got %d", len(watcherBuyOut))
	}
	watcherBuyFrame := decodeSingleFrame(t, watcherBuyOut[0])
	if err := shopproto.DecodeServerSoldOut(watcherBuyFrame); err != nil {
		if err := shopproto.DecodeServerSoldout(watcherBuyFrame); err != nil {
			if err := shopproto.DecodeServerInvalidPos(watcherBuyFrame); err != nil {
				t.Fatalf("expected watcher sold-out/invalid companion, decode errors: %v", err)
			}
		}
	}
	assertExchangeAccountUnchanged(t, accounts, watcherLogin, watcher, "myshop multi-guest watcher sold-out retry")
}

func TestGameRuntimeMyShopGuestSellWhileBrowsingFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopSellHost", 0x01030881, 0x02040881, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 881, Vnum: 27001, Count: 3, Slot: 5}, {ID: 931, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("MyShopSellGuest", 0x01030882, 0x02040882, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 882, Vnum: 27002, Count: 4, Slot: 6}, {ID: 932, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4}}
	ownerLogin := "myshop-sell-host"
	peerLogin := "myshop-sell-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707181, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707182, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop sell host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop sell guest account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Host Stock Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27002, Name: "Guest Sell Potion", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop sell runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707181)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707182)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	const listedPrice uint32 = 1500
	const displayPos uint8 = 7
	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      3,
			Position:   itemproto.InventoryPosition(5),
			Price:      listedPrice,
			DisplayPos: displayPos,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected MYSHOP open error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `unexpected accepted MYSHOP before guest sell reject: out=%d err=%v`)
	_ = flushServerFrames(t, peerFlow)

	browseOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil || len(browseOut) != 1 {
		t.Fatalf("unexpected guest browse before sell reject: out=%d err=%v", len(browseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP START before sell reject: %v", err)
	}

	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{name: "SELL", raw: shopproto.EncodeClientSell(shopproto.ClientSellPacket{Slot: 6})},
		{name: "SELL2", raw: shopproto.EncodeClientSell2(shopproto.ClientSell2Packet{Slot: 6, Count: 2})},
	} {
		out, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, tc.raw))
		if err != nil {
			t.Fatalf("unexpected guest private-shop SHOP %s while browsing: %v", tc.name, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected guest private-shop SHOP %s while browsing to emit no frames, got %d", tc.name, len(out))
		}
		if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
			t.Fatalf("expected guest private-shop SHOP %s while browsing to queue no host frames, got %d", tc.name, len(queued))
		}
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, characterAfterMyShopBagConsume(owner), "myshop guest sell reject host")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop guest sell reject guest")
}

func findPersistedCharacter(t *testing.T, account accountstore.Account, name string) loginticket.Character {
	t.Helper()
	for _, character := range account.Characters {
		if character.Name == name {
			return character
		}
	}
	t.Fatalf("character %q missing from account %q", name, account.Login)
	return loginticket.Character{}
}
