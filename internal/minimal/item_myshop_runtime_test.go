package minimal

import (
	"math"
	"path/filepath"
	"reflect"
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
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
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

func TestGameRuntimeMyShopOpenDeactivatesListedAutoPotionSocket0WithBagPath(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopAutoHP", 0x01030841, 0x02040841, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	active := inventory.SocketValues{1, 9, 8}
	owner.Inventory = []inventory.ItemInstance{
		{ID: 901, Vnum: 72723, Count: 1, Slot: 5, Sockets: &active},
		{ID: 951, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "myshop-auto-hp"
	issuePeerTicket(t, ticketStore, login, 0x70707141, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop auto-hp account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 72723, Name: "Auto HP Recovery S", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{1, 0, 0}},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop auto-hp runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707141)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       72723,
			Count:      1,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP auto-hp error: %v", err)
	}
	if len(out) < 3 {
		t.Fatalf("expected bag refresh, auto-potion ITEM_UPDATE, then SHOP_SIGN, got %d", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode bag ITEM_DEL: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(4) {
		t.Fatalf("unexpected bag ITEM_DEL position: %+v", del.Position)
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode auto-potion ITEM_UPDATE: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(5) || update.Count != 1 || update.Sockets != ([itemproto.ItemSocketCount]int32{0, 9, 8}) {
		t.Fatalf("unexpected auto-potion ITEM_UPDATE: %+v", update)
	}
	assertMyShopLiveSignFrame(t, out[len(out)-1], owner.VID, "Private Shop", "auto-hp SHOP_SIGN")

	wantSockets := inventory.SocketValues{0, 9, 8}
	want := owner
	want.Inventory = []inventory.ItemInstance{{ID: 901, Vnum: 72723, Count: 1, Slot: 5, Sockets: &wantSockets}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "accepted myshop open auto-hp deactivate")
}

func TestGameRuntimeMyShopOpenDeactivatesListedAutoPotionSocket0WithSilkPath(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopAutoSP", 0x01030842, 0x02040842, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	active := inventory.SocketValues{1, 4, 5}
	owner.Inventory = []inventory.ItemInstance{
		{ID: 902, Vnum: 72727, Count: 1, Slot: 5, Sockets: &active},
		{ID: 952, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
	}
	login := "myshop-auto-sp"
	issuePeerTicket(t, ticketStore, login, 0x70707142, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop auto-sp account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 72727, Name: "Auto SP Recovery S", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{1, 0, 0}},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop auto-sp runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707142)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       72727,
			Count:      1,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP auto-sp error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected auto-potion ITEM_UPDATE then SHOP_SIGN on silk path, got %d", len(out))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode silk auto-potion ITEM_UPDATE: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(5) || update.Count != 1 || update.Sockets != ([itemproto.ItemSocketCount]int32{0, 4, 5}) {
		t.Fatalf("unexpected silk auto-potion ITEM_UPDATE: %+v", update)
	}
	assertMyShopLiveSignFrame(t, out[1], owner.VID, "Private Shop", "auto-sp SHOP_SIGN")

	wantSockets := inventory.SocketValues{0, 4, 5}
	want := owner
	want.Inventory = []inventory.ItemInstance{
		{ID: 952, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
		{ID: 902, Vnum: 72727, Count: 1, Slot: 5, Sockets: &wantSockets},
	}
	assertExchangeAccountUnchanged(t, accounts, login, want, "accepted myshop silk open auto-sp deactivate")
}

func TestGameRuntimeMyShopOpenSkipsAutoPotionDeactivateWhenSocket0NotOne(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopAutoSkip", 0x01030843, 0x02040843, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	inactive := inventory.SocketValues{0, 2, 3}
	owner.Inventory = []inventory.ItemInstance{
		{ID: 903, Vnum: 72723, Count: 1, Slot: 5, Sockets: &inactive},
		{ID: 953, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "myshop-auto-skip"
	issuePeerTicket(t, ticketStore, login, 0x70707143, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop auto-skip account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 72723, Name: "Auto HP Recovery S", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{1, 0, 0}},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop auto-skip runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707143)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       72723,
			Count:      1,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP auto-skip error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, out, owner.VID, 4, "socket0!=1 myshop open")
	for i := 0; i < len(out)-1; i++ {
		if _, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[i])); err == nil {
			t.Fatalf("expected no auto-potion ITEM_UPDATE when socket0!=1, frame %d decoded as UPDATE", i)
		}
	}
	want := owner
	want.Inventory = []inventory.ItemInstance{{ID: 903, Vnum: 72723, Count: 1, Slot: 5, Sockets: &inactive}}
	assertExchangeAccountUnchanged(t, accounts, login, want, "socket0!=1 myshop open leaves sockets")
}

func TestGameRuntimeMyShopOpenRejectDoesNotMutateAutoPotionSockets(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopAutoArmor", 0x01030844, 0x02040844, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	active := inventory.SocketValues{1, 2, 3}
	owner.Inventory = []inventory.ItemInstance{
		{ID: 904, Vnum: 72723, Count: 1, Slot: 5, Sockets: &active},
		{ID: 954, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	owner.Equipment = []inventory.ItemInstance{{ID: 955, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}}
	login := "myshop-auto-armor"
	issuePeerTicket(t, ticketStore, login, 0x70707144, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop auto-armor account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 72723, Name: "Auto HP Recovery S", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Body Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String()},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop auto-armor runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707144)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       72723,
			Count:      1,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected armor-reject MYSHOP error: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, out, myShopOpenArmorRequiredInfoMessage, "armor reject must win before auto-potion mutate")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "armor reject leaves auto-potion sockets")
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

func TestGameRuntimeMyShopOpenUsesAuthoredMyShopRejectMessageWithoutMutation(t *testing.T) {
	const authoredAntiMyShop = "This cash item cannot be listed in a private shop."
	const authoredAntiGive = "You cannot list this bound item in a private shop."

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopRejectMsg", 0x01030814, 0x02040814, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 930, Vnum: 27003, Count: 1, Slot: 7},
		{ID: 931, Vnum: 27004, Count: 1, Slot: 8},
		{ID: 932, Vnum: 27005, Count: 1, Slot: 9},
		{ID: 933, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "myshop-reject-msg"
	issuePeerTicket(t, ticketStore, login, 0x70707114, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop reject-message account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27003, Name: "Anti MyShop Potion", Stackable: true, MaxCount: 200, AntiMyShop: true, MyShopRejectText: authoredAntiMyShop},
		{Vnum: 27004, Name: "Anti Give Potion", Stackable: true, MaxCount: 200, AntiGive: true, MyShopRejectText: authoredAntiGive},
		{Vnum: 27005, Name: "Plain Cash Potion", Stackable: true, MaxCount: 200, AntiMyShop: true},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop reject-message runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707114)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	for _, tc := range []struct {
		name        string
		item        shopproto.ClientMyShopItem
		wantMessage string
	}{
		{name: "anti_myshop authored", item: shopproto.ClientMyShopItem{Vnum: 27003, Count: 1, Position: itemproto.InventoryPosition(7), Price: 1500}, wantMessage: authoredAntiMyShop},
		{name: "anti_give authored", item: shopproto.ClientMyShopItem{Vnum: 27004, Count: 1, Position: itemproto.InventoryPosition(8), Price: 1500}, wantMessage: authoredAntiGive},
		{name: "anti_myshop omitted keeps fixed English", item: shopproto.ClientMyShopItem{Vnum: 27005, Count: 1, Position: itemproto.InventoryPosition(9), Price: 1500}, wantMessage: myShopOpenCashItemInfoMessage},
	} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
			Sign:  "Private Shop",
			Items: []shopproto.ClientMyShopItem{tc.item},
		})))
		if err != nil {
			t.Fatalf("unexpected %s MYSHOP error: %v", tc.name, err)
		}
		assertMyShopOpenRejectInfoChat(t, out, tc.wantMessage, tc.name)
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop authored reject messages")
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

func TestGameRuntimeMyShopOpenGoldOverflowGateFiresBeforeCashAndAuthoredReject(t *testing.T) {
	const authoredAntiMyShop = "This cash item cannot be listed in a private shop."

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopGoldCash", 0x01030816, 0x02040816, 1100, 2100, 0, 101, 201)
	owner.Gold = uint64(math.MaxInt32) - 100
	owner.Inventory = []inventory.ItemInstance{
		{ID: 950, Vnum: 27003, Count: 1, Slot: 7},
		{ID: 951, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "myshop-gold-cash"
	issuePeerTicket(t, ticketStore, login, 0x70707116, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop gold-before-cash account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27003, Name: "Anti MyShop Potion", Stackable: true, MaxCount: 200, AntiMyShop: true, MyShopRejectText: authoredAntiMyShop},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop gold-before-cash runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707116)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27003,
			Count:      1,
			Position:   itemproto.InventoryPosition(7),
			Price:      101,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected gold-before-cash MYSHOP error: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, out, myShopOpenGoldOverflowInfoMessage, "gold must win before cash/authored reject")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop gold-before-cash")
}

func TestGameRuntimeMyShopOpenGoldOverflowGateFiresBeforeBanword(t *testing.T) {
	cleanup := SetMyShopOpenBanwordsForTest([]string{"banme"})
	defer cleanup()

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopGoldBan", 0x01030817, 0x02040817, 1100, 2100, 0, 101, 201)
	owner.Gold = uint64(math.MaxInt32) - 100
	owner.Inventory = []inventory.ItemInstance{
		{ID: 960, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 961, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
		{ID: 962, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
	}
	login := "myshop-gold-ban"
	issuePeerTicket(t, ticketStore, login, 0x70707117, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop gold-before-banword account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop gold-before-banword runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707117)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "My banme Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      1,
			Position:   itemproto.InventoryPosition(5),
			Price:      101,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected gold-before-banword MYSHOP error: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, out, myShopOpenGoldOverflowInfoMessage, "gold must win before banword")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected gold-before-banword reject to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop gold-before-banword")
}

func TestGameRuntimeMyShopOpenGoldOverflowGateFiresAfterArmor(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopGoldArmor", 0x01030818, 0x02040818, 1100, 2100, 0, 101, 201)
	owner.Gold = uint64(math.MaxInt32) - 100
	owner.Inventory = []inventory.ItemInstance{
		{ID: 970, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 971, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	owner.Equipment = []inventory.ItemInstance{{ID: 972, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}}
	login := "myshop-gold-armor"
	issuePeerTicket(t, ticketStore, login, 0x70707118, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop armor-before-gold account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Shop Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String()},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop armor-before-gold runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707118)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      1,
			Position:   itemproto.InventoryPosition(5),
			Price:      101,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected armor-before-gold MYSHOP error: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, out, myShopOpenArmorRequiredInfoMessage, "armor must win before gold")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop armor-before-gold")
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

func TestGameRuntimeMyShopOpenRejectsBanwordSignWithInfoChatWithoutMutation(t *testing.T) {
	cleanup := SetMyShopOpenBanwordsForTest([]string{"banme", "금지어"})
	defer cleanup()

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopBanword", 0x01030812, 0x02040812, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 912, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 913, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
		{ID: 914, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
	}
	login := "myshop-banword"
	issuePeerTicket(t, ticketStore, login, 0x70707112, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop banword account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop banword runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707112)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	for _, tc := range []struct {
		name string
		sign string
	}{
		{name: "ascii substring", sign: "My banme Shop"},
		{name: "multibyte substring", sign: "상점 금지어"},
	} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
			Sign: tc.sign,
			Items: []shopproto.ClientMyShopItem{{
				Vnum:       27001,
				Count:      1,
				Position:   itemproto.InventoryPosition(5),
				Price:      1500,
				DisplayPos: 0,
			}},
		})))
		if err != nil {
			t.Fatalf("unexpected %s MYSHOP error: %v", tc.name, err)
		}
		assertMyShopOpenRejectInfoChat(t, out, myShopOpenBanwordInfoMessage, tc.name)
		if queued := flushServerFrames(t, flow); len(queued) != 0 {
			t.Fatalf("expected %s banword reject to queue no peer frames, got %d", tc.name, len(queued))
		}
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop banword rejects")
}

func TestGameRuntimeMyShopOpenBanwordGateFiresAfterArmorAndBeforeStock(t *testing.T) {
	cleanup := SetMyShopOpenBanwordsForTest([]string{"banme"})
	defer cleanup()

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopBanOrder", 0x01030813, 0x02040813, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 920, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 921, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	owner.Equipment = []inventory.ItemInstance{{ID: 922, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}}
	login := "myshop-ban-order"
	issuePeerTicket(t, ticketStore, login, 0x70707113, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop ban-order account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Shop Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String()},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop ban-order runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707113)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "My banme Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      1,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected armor-before-banword MYSHOP error: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, out, myShopOpenArmorRequiredInfoMessage, "armor must win before banword")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop armor-before-banword")
}

func TestGameRuntimeMyShopOpenEmptyBanwordListAllowsCleanAndWouldBeBannedSigns(t *testing.T) {
	cleanup := SetMyShopOpenBanwordsForTest([]string{})
	defer cleanup()

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopBanEmpty", 0x01030814, 0x02040814, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 930, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 931, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "myshop-ban-empty"
	issuePeerTicket(t, ticketStore, login, 0x70707114, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop ban-empty account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop ban-empty runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707114)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "My banme Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      1,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected empty-banword-list MYSHOP error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSignWithText(t, out, owner.VID, "My banme Shop", 4, "empty banword list must allow open")
	want := characterAfterMyShopBagConsume(owner)
	assertExchangeAccountUnchanged(t, accounts, login, want, "empty banword list accepted open")
}

func TestGameRuntimeMyShopOpenCleanSignStillOpensAfterBanwordGate(t *testing.T) {
	cleanup := SetMyShopOpenBanwordsForTest([]string{"banme", "금지어"})
	defer cleanup()

	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopBanClean", 0x01030815, 0x02040815, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 940, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 941, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
	}
	login := "myshop-ban-clean"
	issuePeerTicket(t, ticketStore, login, 0x70707115, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop ban-clean account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop ban-clean runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707115)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       27001,
			Count:      1,
			Position:   itemproto.InventoryPosition(5),
			Price:      1500,
			DisplayPos: 0,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected clean-sign MYSHOP error: %v", err)
	}
	assertMyShopOpenSuccessSignOnly(t, out, owner.VID, "clean sign after banword gate")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "clean sign silk open after banword gate")
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

func TestGameRuntimeMyShopOpenWithSilkBagSkipsShopBagConsume(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopSilk", 0x0103080a, 0x0204080a, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 841, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 842, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "myshop-silk"
	issuePeerTicket(t, ticketStore, login, 0x7070710a, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop silk account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop silk runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x7070710a)
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
		t.Fatalf("unexpected silk-bag MYSHOP error: %v", err)
	}
	assertMyShopOpenSuccessSignOnly(t, out, owner.VID, "silk-bag myshop open")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected silk-bag MYSHOP to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "silk-bag myshop open leaves inventory unchanged")
}

func TestGameRuntimeMyShopOpenWithSilkBagPrefersSilkOverShopBagConsume(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopSilkPref", 0x0103080b, 0x0204080b, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 851, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 852, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
		{ID: 853, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "myshop-silk-pref"
	issuePeerTicket(t, ticketStore, login, 0x7070710b, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop silk-pref account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop silk-pref runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x7070710b)
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
		t.Fatalf("unexpected silk-pref MYSHOP error: %v", err)
	}
	assertMyShopOpenSuccessSignOnly(t, out, owner.VID, "silk-pref myshop open")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "silk path must not debit ordinary shop bag")
}

func TestGameRuntimeMyShopOpenRejectsListedOnlyOrLockedOnlySilkBagWithoutUnlocking(t *testing.T) {
	for _, tc := range []struct {
		name      string
		login     string
		handle    uint32
		vidBase   uint32
		inventory []inventory.ItemInstance
		items     []shopproto.ClientMyShopItem
		wantOpen  bool
	}{
		{
			name:    "listed-only silk falls through to shop bag consume",
			login:   "myshop-listed-silk",
			handle:  0x7070710c,
			vidBase: 0x0103080c,
			inventory: []inventory.ItemInstance{
				{ID: 861, Vnum: 27001, Count: 1, Slot: 5},
				{ID: 862, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
				{ID: 863, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
			},
			items: []shopproto.ClientMyShopItem{
				{Vnum: 27001, Count: 1, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 0},
				{Vnum: myShopOpenSilkBagVnum, Count: 1, Position: itemproto.InventoryPosition(3), Price: 100, DisplayPos: 1},
			},
			wantOpen: true,
		},
		{
			name:    "locked-only silk falls through to shop bag consume",
			login:   "myshop-locked-silk",
			handle:  0x7070710d,
			vidBase: 0x0103080d,
			inventory: []inventory.ItemInstance{
				{ID: 871, Vnum: 27001, Count: 1, Slot: 5},
				{ID: 872, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3, Locked: true},
				{ID: 873, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
			},
			items: []shopproto.ClientMyShopItem{
				{Vnum: 27001, Count: 1, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 0},
			},
			wantOpen: true,
		},
		{
			name:    "listed-only silk without shop bag stays silent",
			login:   "myshop-listed-silk-miss",
			handle:  0x7070710e,
			vidBase: 0x0103080e,
			inventory: []inventory.ItemInstance{
				{ID: 881, Vnum: 27001, Count: 1, Slot: 5},
				{ID: 882, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
			},
			items: []shopproto.ClientMyShopItem{
				{Vnum: 27001, Count: 1, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 0},
				{Vnum: myShopOpenSilkBagVnum, Count: 1, Position: itemproto.InventoryPosition(3), Price: 100, DisplayPos: 1},
			},
			wantOpen: false,
		},
		{
			name:    "locked-only silk without shop bag stays silent",
			login:   "myshop-locked-silk-miss",
			handle:  0x7070710f,
			vidBase: 0x0103080f,
			inventory: []inventory.ItemInstance{
				{ID: 891, Vnum: 27001, Count: 1, Slot: 5},
				{ID: 892, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3, Locked: true},
			},
			items: []shopproto.ClientMyShopItem{
				{Vnum: 27001, Count: 1, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 0},
			},
			wantOpen: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("MyShopSilkNeg", tc.vidBase, tc.vidBase+0x01010000, 1100, 2100, 0, 101, 201)
			owner.Gold = 5000
			owner.Inventory = tc.inventory
			issuePeerTicket(t, ticketStore, tc.login, tc.handle, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s account: %v", tc.name, err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{
				{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
				{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
				{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
			})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.handle)
			defer closeSessionFlow(t, flow)
			_ = flushServerFrames(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
				Sign:  "Private Shop",
				Items: tc.items,
			})))
			if err != nil {
				t.Fatalf("unexpected %s MYSHOP error: %v", tc.name, err)
			}
			if !tc.wantOpen {
				if len(out) != 0 {
					t.Fatalf("expected %s MYSHOP to emit no frames, got %d", tc.name, len(out))
				}
				assertExchangeAccountUnchanged(t, accounts, tc.login, owner, tc.name)
				return
			}
			assertMyShopOpenSuccessBagAndSign(t, out, owner.VID, 4, tc.name)
			assertExchangeAccountUnchanged(t, accounts, tc.login, characterAfterMyShopBagConsume(owner), tc.name)
		})
	}
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
		t.Fatalf("unexpected exchange start before myshop open cancel: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange start before myshop open cancel to emit one frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	acceptedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(openPacket)))
	if err != nil {
		t.Fatalf("unexpected accepted MYSHOP while exchange open: %v", err)
	}
	assertMyShopOpenSuccessBagExchangeEndAndSign(t, acceptedOut, owner.VID, 4, `expected accepted MYSHOP while exchange open to emit bag refresh, EXCHANGE END, then SHOP_SIGN, got %d`)
	peerQueuedAfterOpen := flushServerFrames(t, peerFlow)
	if len(peerQueuedAfterOpen) != 2 {
		t.Fatalf("expected peer to receive EXCHANGE END then live SHOP_SIGN after myshop open cancel, got %d", len(peerQueuedAfterOpen))
	}
	assertExchangeEndFrame(t, peerQueuedAfterOpen[0], "peer exchange END after myshop open cancel")
	assertMyShopLiveSignFrame(t, peerQueuedAfterOpen[1], owner.VID, "Private Shop", "peer live SHOP_SIGN after myshop open cancel")

	secondOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(openPacket)))
	if err != nil {
		t.Fatalf("unexpected already-open second MYSHOP after busy clears: %v", err)
	}
	if len(secondOut) != 1 {
		t.Fatalf("expected already-open second MYSHOP to emit one empty SHOP_SIGN, got %d", len(secondOut))
	}
	assertMyShopEmptySignFrame(t, secondOut[0], owner.VID, "already-open second MYSHOP after busy clears")
	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 1 {
		t.Fatalf("expected already-open second MYSHOP to around-broadcast one empty SHOP_SIGN to peer, got %d", len(peerQueued))
	}
	assertMyShopEmptySignFrame(t, peerQueued[0], owner.VID, "already-open second MYSHOP peer around-broadcast")

	assertExchangeAccountUnchanged(t, accounts, login, characterAfterMyShopBagConsume(owner), "myshop busy rejects")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop busy peer")
}

func TestGameRuntimeMyShopOpenWithSilkBagCancelsActiveExchangeBeforeShopSign(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopSilkExch", 0x010308e1, 0x020408e1, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 2001, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 2002, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
	}
	peer := peerVisibilityCharacter("MyShopSilkExchPeer", 0x010308e2, 0x020408e2, 1120, 2120, 0, 101, 201)
	login := "myshop-silk-exch"
	peerLogin := "myshop-silk-exch-peer"
	issuePeerTicket(t, ticketStore, login, 0x707071e1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707071e2, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop silk-exchange account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop silk-exchange peer account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop silk-exchange runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071e1)
	defer closeSessionFlow(t, flow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707071e2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil || len(startOut) != 1 {
		t.Fatalf("expected exchange start before silk myshop open, got %d err=%v", len(startOut), err)
	}
	_ = flushServerFrames(t, peerFlow)

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
		t.Fatalf("unexpected silk myshop open while exchange active: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected silk myshop open to emit EXCHANGE END then SHOP_SIGN, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "silk myshop open self exchange END")
	assertMyShopLiveSignFrame(t, out[1], owner.VID, "Private Shop", "silk myshop open SHOP_SIGN")
	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 2 {
		t.Fatalf("expected peer EXCHANGE END then live SHOP_SIGN after silk myshop open, got %d", len(peerQueued))
	}
	assertExchangeEndFrame(t, peerQueued[0], "silk myshop open peer exchange END")
	assertMyShopLiveSignFrame(t, peerQueued[1], owner.VID, "Private Shop", "silk myshop open peer SHOP_SIGN")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "silk myshop open cancel leaves inventory unchanged")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "silk myshop open cancel peer")
}

func TestGameRuntimeMyShopOpenRejectWhileExchangeOpenLeavesShellCancellable(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopRejectExch", 0x010308e3, 0x020408e3, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 2101, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 2102, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	owner.Equipment = []inventory.ItemInstance{{ID: 2103, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}}
	peer := peerVisibilityCharacter("MyShopRejectExchPeer", 0x010308e4, 0x020408e4, 1120, 2120, 0, 101, 201)
	login := "myshop-reject-exch"
	peerLogin := "myshop-reject-exch-peer"
	issuePeerTicket(t, ticketStore, login, 0x707071e3, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707071e4, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop reject-exchange account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop reject-exchange peer account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Body Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String()},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop reject-exchange runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071e3)
	defer closeSessionFlow(t, flow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707071e4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, flow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil || len(startOut) != 1 {
		t.Fatalf("expected exchange start before armored myshop reject, got %d err=%v", len(startOut), err)
	}
	_ = flushServerFrames(t, peerFlow)

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
		t.Fatalf("unexpected armored MYSHOP while exchange open: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, out, myShopOpenArmorRequiredInfoMessage, "armored MYSHOP while exchange open")
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected armored MYSHOP reject to leave peer exchange untouched, got %d queued frames", len(queued))
	}

	cancelOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil || len(cancelOut) != 1 {
		t.Fatalf("expected exchange cancel after armored myshop reject to emit one END, got %d err=%v", len(cancelOut), err)
	}
	assertExchangeEndFrame(t, cancelOut[0], "owner cancel after armored myshop reject")
	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 1 {
		t.Fatalf("expected peer END after cancel following armored myshop reject, got %d", len(peerQueued))
	}
	assertExchangeEndFrame(t, peerQueued[0], "peer cancel after armored myshop reject")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "armored myshop reject leaves account unchanged")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "armored myshop reject peer")
}

func TestGameRuntimeMyShopAlreadyOpenSecondOpenClearsWithEmptySignWithoutBagRefund(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopSecondOpen", 0x010308d1, 0x020408d1, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1901, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1951, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	peer := peerVisibilityCharacter("MyShopSecondOpenPeer", 0x010308d2, 0x020408d2, 1120, 2120, 0, 101, 201)
	login := "myshop-second-open"
	peerLogin := "myshop-second-open-peer"
	issuePeerTicket(t, ticketStore, login, 0x707071d1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707071d2, peer)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop second-open account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop second-open peer account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop second-open runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071d1)
	defer closeSessionFlow(t, flow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707071d2)
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
	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(openPacket)))
	if err != nil {
		t.Fatalf("unexpected first MYSHOP open: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected first MYSHOP to emit bag refresh then SHOP_SIGN, got %d`)
	_ = flushServerFrames(t, peerFlow)

	afterOpen := characterAfterMyShopBagConsume(owner)
	assertExchangeAccountUnchanged(t, accounts, login, afterOpen, "myshop second-open after first open")

	secondOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(openPacket)))
	if err != nil {
		t.Fatalf("unexpected second MYSHOP open: %v", err)
	}
	if len(secondOut) != 1 {
		t.Fatalf("expected second MYSHOP to emit one empty SHOP_SIGN, got %d", len(secondOut))
	}
	assertMyShopEmptySignFrame(t, secondOut[0], owner.VID, "second MYSHOP empty sign")
	peerQueued := flushServerFrames(t, peerFlow)
	if len(peerQueued) != 1 {
		t.Fatalf("expected second MYSHOP to around-broadcast one empty SHOP_SIGN, got %d", len(peerQueued))
	}
	assertMyShopEmptySignFrame(t, peerQueued[0], owner.VID, "second MYSHOP peer empty sign")
	assertExchangeAccountUnchanged(t, accounts, login, afterOpen, "myshop second-open no bag refund")

	alreadyClosedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_myshop after second-open clear: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected /close_myshop after second-open clear to stay silent, got %d", len(alreadyClosedOut))
	}

	reopenSeed := afterOpen
	reopenSeed.Inventory = append(cloneInventoryItems(afterOpen.Inventory), inventory.ItemInstance{
		ID: 1952, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4,
	})
	sortInventoryItemsBySlot(reopenSeed.Inventory)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{reopenSeed})}); err != nil {
		t.Fatalf("persist myshop second-open reopen bag seed: %v", err)
	}
	if !runtime.applyLiveCharacterPersistedSnapshot(owner.Name, reopenSeed) {
		t.Fatal("expected live reopen bag seed after second-open clear")
	}

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(openPacket)))
	if err != nil {
		t.Fatalf("unexpected reopen MYSHOP after second-open clear: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, reopenOut, owner.VID, 4, `expected reopen MYSHOP after second-open clear to emit bag refresh then SHOP_SIGN, got %d`)
	assertExchangeAccountUnchanged(t, accounts, login, characterAfterMyShopBagConsume(reopenSeed), "myshop second-open reopen")
}

func TestGameRuntimeMyShopAlreadyOpenSecondOpenKeepsArmorRejectBeforeClose(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopSecondArmor", 0x010308d3, 0x020408d3, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1911, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1961, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "myshop-second-armor"
	issuePeerTicket(t, ticketStore, login, 0x707071d3, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop second-armor account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Shop Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String()},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop second-armor runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071d3)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openPacket := shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:     27001,
			Count:    3,
			Position: itemproto.InventoryPosition(5),
			Price:    1500,
		}},
	}
	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(openPacket)))
	if err != nil {
		t.Fatalf("unexpected first MYSHOP open before armor second-open: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected first MYSHOP before armor second-open to emit bag refresh then SHOP_SIGN, got %d`)
	afterOpen := characterAfterMyShopBagConsume(owner)

	// Inject worn body armor into the still-open live session (host item
	// mutation stays locked while MYSHOP is open, so this uses the live
	// snapshotter hook already owned by exchange/drop drift proofs).
	armored := afterOpen
	armored.Equipment = []inventory.ItemInstance{{
		ID: 1921, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody,
	}}
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{armored})}); err != nil {
		t.Fatalf("persist armored myshop second-open account: %v", err)
	}
	if !runtime.applyLiveCharacterPersistedSnapshot(owner.Name, armored) {
		t.Fatal("expected live armored snapshot inject while MYSHOP stays open")
	}

	secondOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(openPacket)))
	if err != nil {
		t.Fatalf("unexpected armored already-open second MYSHOP: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, secondOut, myShopOpenArmorRequiredInfoMessage, "armor must win before already-open empty-sign close")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected armored already-open second MYSHOP to queue no peer frames, got %d", len(queued))
	}

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_myshop after armored second-open reject: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected /close_myshop after armored second-open reject to emit one empty SHOP_SIGN, got %d", len(closeOut))
	}
	assertMyShopEmptySignFrame(t, closeOut[0], owner.VID, "close after armored second-open reject")
	assertExchangeAccountUnchanged(t, accounts, login, armored, "myshop second-armor armored second-open")
}

func TestGameRuntimeMyShopAlreadyOpenEmptySignOrZeroCountSecondPacketStaysSilent(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopSecondSilent", 0x010308d4, 0x020408d4, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1931, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1981, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "myshop-second-silent"
	issuePeerTicket(t, ticketStore, login, 0x707071d4, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop second-silent account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop second-silent runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707071d4)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openPacket := shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:     27001,
			Count:    3,
			Position: itemproto.InventoryPosition(5),
			Price:    1500,
		}},
	}
	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(openPacket)))
	if err != nil {
		t.Fatalf("unexpected first MYSHOP open before silent second packets: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, `expected first MYSHOP before silent second packets to emit bag refresh then SHOP_SIGN, got %d`)
	afterOpen := characterAfterMyShopBagConsume(owner)

	for _, tc := range []struct {
		name string
		pkt  shopproto.ClientMyShopPacket
	}{
		{name: "empty sign", pkt: shopproto.ClientMyShopPacket{Sign: "", Items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 1500}}}},
		{name: "zero count", pkt: shopproto.ClientMyShopPacket{Sign: "Private Shop"}},
	} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(tc.pkt)))
		if err != nil {
			t.Fatalf("unexpected %s second MYSHOP error: %v", tc.name, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected %s second MYSHOP to stay silent while open, got %d", tc.name, len(out))
		}
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterOpen, "myshop second-silent while open")

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_myshop after silent second packets: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected /close_myshop after silent second packets to emit one empty SHOP_SIGN, got %d", len(closeOut))
	}
	assertMyShopEmptySignFrame(t, closeOut[0], owner.VID, "close after silent second packets")
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

func assertMyShopOpenSuccessSignOnly(t *testing.T, out [][]byte, wantVID uint32, context string) {
	assertMyShopOpenSuccessSignOnlyWithText(t, out, wantVID, "Private Shop", context)
}

func assertMyShopOpenSuccessSignOnlyWithText(t *testing.T, out [][]byte, wantVID uint32, wantSign string, context string) {
	t.Helper()
	if len(out) != 1 {
		t.Fatalf("expected %s MYSHOP to emit exactly one SHOP_SIGN with no bag refresh, got %d", context, len(out))
	}
	sign, err := shopproto.DecodeServerShopSign(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode %s MYSHOP SHOP_SIGN: %v", context, err)
	}
	if sign.VID != wantVID || sign.Sign != wantSign {
		t.Fatalf("unexpected %s MYSHOP SHOP_SIGN: %+v", context, sign)
	}
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

func assertMyShopOpenSuccessBagExchangeEndAndSign(t *testing.T, out [][]byte, wantVID uint32, bagSlot uint16, context string) {
	t.Helper()
	if len(out) < 3 {
		t.Fatalf(context, len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode %s MYSHOP bag ITEM_DEL: %v", context, err)
	}
	if del.Position != itemproto.InventoryPosition(bagSlot) {
		t.Fatalf("unexpected %s MYSHOP bag ITEM_DEL position: %+v", context, del.Position)
	}
	assertExchangeEndFrame(t, out[len(out)-2], context+" exchange END")
	assertMyShopLiveSignFrame(t, out[len(out)-1], wantVID, "Private Shop", context+" SHOP_SIGN")
}

func assertMyShopLiveSignFrame(t *testing.T, raw []byte, wantVID uint32, wantSign, context string) {
	t.Helper()
	sign, err := shopproto.DecodeServerShopSign(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode %s live SHOP_SIGN: %v", context, err)
	}
	if sign.VID != wantVID || sign.Sign != wantSign {
		t.Fatalf("unexpected %s live SHOP_SIGN: %+v", context, sign)
	}
}

func characterAfterMyShopBagConsume(character loginticket.Character) loginticket.Character {
	updated := character
	updated.Inventory = cloneItemInstancesWithoutVnum(character.Inventory, myShopOpenShopBagVnum)
	sortInventoryItemsBySlot(updated.Inventory)
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

func TestGameRuntimeMyShopOpenDeniesHostMoveAndSyncPositionWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopMoveDeny", 0x01030851, 0x02040851, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 851, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 901, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "myshop-move-deny"
	issuePeerTicket(t, ticketStore, login, 0x70707151, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop move-deny account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop move-deny runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707151)
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
		t.Fatalf("unexpected accepted MYSHOP before move deny: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, "accepted MYSHOP before move deny")

	assertMyShopHostPosition := func(t *testing.T, wantX, wantY int32, context string) {
		t.Helper()
		live, ok := runtime.ConnectedCharacterSnapshot(owner.Name)
		if !ok {
			t.Fatalf("expected connected snapshot for %s", context)
		}
		if live.X != wantX || live.Y != wantY {
			t.Fatalf("%s live position: got x=%d y=%d want x=%d y=%d", context, live.X, live.Y, wantX, wantY)
		}
		persisted, err := accounts.Load(login)
		if err != nil {
			t.Fatalf("load persisted %s account: %v", context, err)
		}
		if len(persisted.Characters) != 1 {
			t.Fatalf("expected one persisted %s character, got %d", context, len(persisted.Characters))
		}
		got := persisted.Characters[0]
		if got.X != wantX || got.Y != wantY {
			t.Fatalf("%s persisted position: got x=%d y=%d want x=%d y=%d", context, got.X, got.Y, wantX, wantY)
		}
	}

	moveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x11121314,
	})))
	if err != nil {
		t.Fatalf("unexpected MOVE while MYSHOP open error: %v", err)
	}
	if len(moveOut) != 0 {
		t.Fatalf("expected MOVE while MYSHOP open to emit no frames, got %d", len(moveOut))
	}
	assertMyShopHostPosition(t, owner.X, owner.Y, "MOVE while MYSHOP open")

	syncOut, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{
		Elements: []movep.SyncPositionElement{{VID: owner.VID, X: 1600, Y: 2700}},
	})))
	if err != nil {
		t.Fatalf("unexpected SyncPosition while MYSHOP open error: %v", err)
	}
	if len(syncOut) != 0 {
		t.Fatalf("expected SyncPosition while MYSHOP open to emit no frames, got %d", len(syncOut))
	}
	assertMyShopHostPosition(t, owner.X, owner.Y, "SyncPosition while MYSHOP open")
	assertExchangeAccountUnchanged(t, accounts, login, characterAfterMyShopBagConsume(owner), "myshop open host move deny")

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_myshop",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_myshop after move deny: %v", err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected /close_myshop after move deny to emit one empty SHOP_SIGN frame, got %d", len(closeOut))
	}
	assertMyShopEmptySignFrame(t, closeOut[0], owner.VID, "close_myshop after move deny")

	moveAfterClose, err := flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x21222324,
	})))
	if err != nil {
		t.Fatalf("unexpected MOVE after MYSHOP close error: %v", err)
	}
	if len(moveAfterClose) != 1 {
		t.Fatalf("expected MOVE after MYSHOP close to emit one MOVE ack, got %d", len(moveAfterClose))
	}
	moveAck, err := movep.DecodeMoveAck(decodeSingleFrame(t, moveAfterClose[0]))
	if err != nil {
		t.Fatalf("decode MOVE ack after MYSHOP close: %v", err)
	}
	if moveAck.VID != owner.VID || moveAck.X != 1500 || moveAck.Y != 2600 {
		t.Fatalf("unexpected MOVE ack after MYSHOP close: %+v", moveAck)
	}
	assertMyShopHostPosition(t, 1500, 2600, "MOVE after MYSHOP close")
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

func TestGameRuntimeMyShopGuestBuyPreservesInstanceSocketsAndAttributes(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())

	hostSockets := inventory.SocketValues{7, 0, 9}
	hostAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}

	owner := peerVisibilityCharacter("MyShopPreserveHost", 0x01030881, 0x02040881, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 881, Vnum: 11280, Count: 1, Slot: 5, Sockets: &hostSockets, Attributes: &hostAttributes},
		{ID: 931, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	peer := peerVisibilityCharacter("MyShopPreserveGuest", 0x01030882, 0x02040882, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "myshop-preserve-host"
	peerLogin := "myshop-preserve-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707181, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707182, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop preserve host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop preserve guest account: %v", err)
	}

	activeTemplate := itemcatalog.Template{
		Vnum: 11280, Name: "Preserve Host Blade", Stackable: false, MaxCount: 1,
		Sockets:    itemcatalog.SocketValues{1, 2, 3},
		Attributes: itemcatalog.AttributeValues{{Type: 3, Value: 30}},
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{activeTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected myshop preserve runtime error: %v", err)
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
		Sign: "Preserve Shop",
		Items: []shopproto.ClientMyShopItem{{
			Vnum:       activeTemplate.Vnum,
			Count:      1,
			Position:   itemproto.InventoryPosition(5),
			Price:      listedPrice,
			DisplayPos: displayPos,
		}},
	})))
	if err != nil {
		t.Fatalf("unexpected MYSHOP open error: %v", err)
	}
	assertMyShopOpenSuccessBagAndSignWithText(t, openOut, owner.VID, "Preserve Shop", 4, `unexpected accepted MYSHOP before preserve guest buy: out=%d err=%v`)
	_ = flushServerFrames(t, peerFlow)

	browseOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil || len(browseOut) != 1 {
		t.Fatalf("unexpected guest browse before preserve buy: out=%d err=%v", len(browseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP START before preserve buy: %v", err)
	}

	buyOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{
		RawLeadingByte: 1,
		CatalogSlot:    displayPos,
	})))
	if err != nil {
		t.Fatalf("unexpected guest private-shop SHOP BUY preserve: %v", err)
	}
	if len(buyOut) == 0 {
		t.Fatalf("expected guest private-shop SHOP BUY preserve to mutate and emit frames, got none")
	}

	var guestItemSet *itemproto.SetPacket
	for _, raw := range buyOut {
		f := decodeSingleFrame(t, raw)
		if set, err := itemproto.DecodeSet(f); err == nil {
			if set.Vnum == activeTemplate.Vnum && set.Count == 1 {
				copied := set
				guestItemSet = &copied
			}
		}
	}
	if guestItemSet == nil {
		t.Fatalf("expected guest ITEM_SET for preserved host blade in buy burst, frames=%d", len(buyOut))
	}
	wantSockets := [itemproto.ItemSocketCount]int32{7, 0, 9}
	if guestItemSet.Sockets != wantSockets {
		t.Fatalf("expected preserve guest ITEM_SET sockets %+v from host instance, got %+v", wantSockets, guestItemSet.Sockets)
	}
	if guestItemSet.Attributes[0] != (itemproto.Attribute{Type: 1, Value: 25}) || guestItemSet.Attributes[1] != (itemproto.Attribute{Type: 7, Value: -3}) {
		t.Fatalf("expected preserve guest ITEM_SET attributes from host instance, got %+v", guestItemSet.Attributes)
	}

	peerAccount, err := accounts.Load(peerLogin)
	if err != nil {
		t.Fatalf("load preserve guest account: %v", err)
	}
	persistedPeer := findPersistedCharacter(t, peerAccount, peer.Name)
	if len(persistedPeer.Inventory) != 1 || persistedPeer.Inventory[0].ID != 881 || persistedPeer.Inventory[0].Vnum != 11280 || persistedPeer.Inventory[0].Count != 1 {
		t.Fatalf("unexpected preserve guest inventory identity after buy: %+v", persistedPeer.Inventory)
	}
	if !persistedPeer.Inventory[0].HasSockets() || *persistedPeer.Inventory[0].Sockets != hostSockets {
		t.Fatalf("expected preserve guest inventory sockets %+v from host instance, got %+v", hostSockets, persistedPeer.Inventory[0].Sockets)
	}
	if !persistedPeer.Inventory[0].HasAttributes() || *persistedPeer.Inventory[0].Attributes != hostAttributes {
		t.Fatalf("expected preserve guest inventory attributes %+v from host instance, got %+v", hostAttributes, persistedPeer.Inventory[0].Attributes)
	}
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

func TestGameRuntimeSilkBagUseFirstSessionEmitsDummyMyShopPriceListThenOpen(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SilkUseDummy", 0x01030851, 0x02040851, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 951, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3}}
	login := "silk-use-dummy"
	issuePeerTicket(t, ticketStore, login, 0x70707151, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed silk use dummy account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected silk use dummy runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707151)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(3)})))
	if err != nil {
		t.Fatalf("unexpected silk bag ITEM_USE error: %v", err)
	}
	assertMyShopBagUseCommandBurst(t, out, []string{"MyShopPriceList 1 0", myShopOpenPrivateShopCommandMessage}, "first silk use dummy")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "silk bag use must not consume")
}

func TestGameRuntimeShopBagUseEmitsOpenPrivateShopOnly(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ShopBagUse", 0x01030852, 0x02040852, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 952, Vnum: myShopOpenShopBagVnum, Count: 2, Slot: 4}}
	login := "shop-bag-use"
	issuePeerTicket(t, ticketStore, login, 0x70707152, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed shop bag use account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected shop bag use runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707152)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 4"})))
	if err != nil {
		t.Fatalf("unexpected shop bag /use_item error: %v", err)
	}
	assertMyShopBagUseCommandBurst(t, out, []string{myShopOpenPrivateShopCommandMessage}, "shop bag use")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "shop bag use must not consume")
}

func TestGameRuntimeSilkBagUseAfterSilkMyShopOpenRematerializesRememberedUnitPrices(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SilkUseRemember", 0x01030853, 0x02040853, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 961, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 962, Vnum: 27002, Count: 2, Slot: 6},
		{ID: 963, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
	}
	login := "silk-use-remember"
	issuePeerTicket(t, ticketStore, login, 0x70707153, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed silk use remember account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion A", Stackable: true, MaxCount: 200},
		{Vnum: 27002, Name: "Shop Potion B", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected silk use remember runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707153)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{
			{Vnum: 27002, Count: 2, Position: itemproto.InventoryPosition(6), Price: 400, DisplayPos: 0},
			{Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 1},
		},
	})))
	if err != nil {
		t.Fatalf("unexpected silk MYSHOP open before bag use: %v", err)
	}
	assertMyShopOpenSuccessSignOnly(t, openOut, owner.VID, "silk open before bag use")
	_ = flushServerFrames(t, flow)

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/close_myshop"})))
	if err != nil || len(closeOut) == 0 {
		t.Fatalf("unexpected /close_myshop after silk open: out=%d err=%v", len(closeOut), err)
	}
	_ = flushServerFrames(t, flow)

	firstUse, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(3)})))
	if err != nil {
		t.Fatalf("unexpected first silk bag ITEM_USE after open: %v", err)
	}
	assertMyShopBagUseCommandBurst(t, firstUse, []string{
		"MyShopPriceList 27001 500",
		"MyShopPriceList 27002 200",
		myShopOpenPrivateShopCommandMessage,
	}, "silk use rematerialize")

	secondUse, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(3)})))
	if err != nil {
		t.Fatalf("unexpected second silk bag ITEM_USE: %v", err)
	}
	assertMyShopBagUseCommandBurst(t, secondUse, []string{myShopOpenPrivateShopCommandMessage}, "later silk use open only")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load silk remember account: %v", err)
	}
	got := findPersistedCharacter(t, persisted, owner.Name)
	if !sameExchangeInventory(got.Inventory, owner.Inventory) {
		t.Fatalf("silk remember path mutated inventory: got %+v want %+v", got.Inventory, owner.Inventory)
	}
	if !sameExchangeQuickslots(got.Quickslots, owner.Quickslots) {
		t.Fatalf("silk remember path mutated quickslots: got %+v want %+v", got.Quickslots, owner.Quickslots)
	}
	if got.Gold != owner.Gold {
		t.Fatalf("silk remember path mutated gold: got %d want %d", got.Gold, owner.Gold)
	}
	wantPrices := []loginticket.MyShopUnitPrice{{Vnum: 27001, UnitPrice: 500}, {Vnum: 27002, UnitPrice: 200}}
	if !reflect.DeepEqual(got.MyShopUnitPrices, wantPrices) {
		t.Fatalf("unexpected durable myshop unit prices after silk open: got %#v want %#v", got.MyShopUnitPrices, wantPrices)
	}
}

func TestGameRuntimeOrdinaryShopBagOpenDoesNotPersistMyShopUnitPrices(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OrdinaryNoPrice", 0x01030857, 0x02040857, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 994, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 995, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	login := "ordinary-no-price"
	const loginKey uint32 = 0x70707157
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed ordinary no-price account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenShopBagVnum, Name: "Shop Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected ordinary no-price runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{
			{Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 0},
		},
	})))
	if err != nil {
		t.Fatalf("unexpected ordinary MYSHOP open: %v", err)
	}
	assertMyShopOpenSuccessBagAndSign(t, openOut, owner.VID, 4, "ordinary open must consume bag")
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load ordinary no-price account: %v", err)
	}
	got := findPersistedCharacter(t, persisted, owner.Name)
	if len(got.MyShopUnitPrices) != 0 {
		t.Fatalf("ordinary bag open must not persist myshop_unit_prices, got %#v", got.MyShopUnitPrices)
	}
}

func TestGameRuntimeSilkBagUseRematerializesDurableMyShopUnitPricesAcrossProcessRestart(t *testing.T) {
	root := t.TempDir()
	ticketDir := filepath.Join(root, "tickets")
	accountDir := filepath.Join(root, "accounts")
	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)
	owner := peerVisibilityCharacter("SilkDurablePrice", 0x01030856, 0x02040856, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 991, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 992, Vnum: 27002, Count: 2, Slot: 6},
		{ID: 993, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3},
	}
	login := "silk-durable-price"
	const loginKey uint32 = 0x70707156
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed durable silk price account: %v", err)
	}
	templates := []itemcatalog.Template{
		{Vnum: 27001, Name: "Shop Potion A", Stackable: true, MaxCount: 200},
		{Vnum: 27002, Name: "Shop Potion B", Stackable: true, MaxCount: 200},
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	}
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected durable silk price runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{
		Sign: "Private Shop",
		Items: []shopproto.ClientMyShopItem{
			{Vnum: 27002, Count: 2, Position: itemproto.InventoryPosition(6), Price: 400, DisplayPos: 0},
			{Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 1500, DisplayPos: 1},
		},
	})))
	if err != nil {
		t.Fatalf("unexpected silk MYSHOP open before restart: %v", err)
	}
	assertMyShopOpenSuccessSignOnly(t, openOut, owner.VID, "silk open before restart")
	_ = flushServerFrames(t, flow)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/close_myshop"}))); err != nil {
		t.Fatalf("unexpected /close_myshop before restart: %v", err)
	}
	closeSessionFlow(t, flow)

	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load durable silk prices before restart: %v", err)
	}
	got := findPersistedCharacter(t, persisted, owner.Name)
	wantPrices := []loginticket.MyShopUnitPrice{{Vnum: 27001, UnitPrice: 500}, {Vnum: 27002, UnitPrice: 200}}
	if !reflect.DeepEqual(got.MyShopUnitPrices, wantPrices) {
		t.Fatalf("unexpected durable myshop unit prices before restart: got %#v want %#v", got.MyShopUnitPrices, wantPrices)
	}

	const postRestartLoginKey uint32 = 0x70707166
	reloadedTickets := loginticket.NewFileStore(ticketDir)
	issuePeerTicket(t, reloadedTickets, login, postRestartLoginKey, got)
	reloadedAccounts := accountstore.NewFileStore(accountDir)
	reloadedItems := newItemTemplateStore(t, templates)
	reloaded, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, reloadedTickets, reloadedAccounts, nil, nil, reloadedItems, nil)
	if err != nil {
		t.Fatalf("reload runtime after durable silk price process restart: %v", err)
	}
	restartFlow, _ := enterGameWithLoginTicket(t, reloaded.SessionFactory(), login, postRestartLoginKey)
	defer closeSessionFlow(t, restartFlow)
	_ = flushServerFrames(t, restartFlow)

	firstUse, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(3)})))
	if err != nil {
		t.Fatalf("unexpected silk bag ITEM_USE after process restart: %v", err)
	}
	assertMyShopBagUseCommandBurst(t, firstUse, []string{
		"MyShopPriceList 27001 500",
		"MyShopPriceList 27002 200",
		myShopOpenPrivateShopCommandMessage,
	}, "post-restart silk use rematerialize")

	secondUse, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(3)})))
	if err != nil {
		t.Fatalf("unexpected second silk bag ITEM_USE after process restart: %v", err)
	}
	assertMyShopBagUseCommandBurst(t, secondUse, []string{myShopOpenPrivateShopCommandMessage}, "post-restart later silk use open only")
}

func TestGameRuntimeSilkBagUseArmorRejectsWithUnequipInfo(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SilkUseArmor", 0x01030854, 0x02040854, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 971, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3}}
	owner.Equipment = []inventory.ItemInstance{{ID: 972, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}}
	login := "silk-use-armor"
	issuePeerTicket(t, ticketStore, login, 0x70707154, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed silk use armor account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
		{Vnum: 11200, Name: "Body Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String()},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected silk use armor runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707154)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(3)})))
	if err != nil {
		t.Fatalf("unexpected armored silk bag ITEM_USE error: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, out, myShopOpenArmorRequiredInfoMessage, "silk bag use armor")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "armored silk bag use")
}

func TestGameRuntimeSilkBagUseBusyShellRejectsWithBagBusyInfo(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SilkUseBusy", 0x01030855, 0x02040855, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 981, Vnum: myShopOpenSilkBagVnum, Count: 1, Slot: 3}}
	login := "silk-use-busy"
	issuePeerTicket(t, ticketStore, login, 0x70707155, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed silk use busy account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		{Vnum: myShopOpenSilkBagVnum, Name: "Silk Bag", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected silk use busy runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707155)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openCubeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/open_cube"})))
	if err != nil || len(openCubeOut) == 0 {
		t.Fatalf("unexpected /open_cube before silk bag use: out=%d err=%v", len(openCubeOut), err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(3)})))
	if err != nil {
		t.Fatalf("unexpected busy silk bag ITEM_USE error: %v", err)
	}
	assertMyShopOpenRejectInfoChat(t, out, myShopBagUseBusyInfoMessage, "silk bag use cube busy")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "busy silk bag use")
}

func assertMyShopBagUseCommandBurst(t *testing.T, out [][]byte, wantMessages []string, context string) {
	t.Helper()
	if len(out) != len(wantMessages) {
		t.Fatalf("expected %s bag use to emit %d command frames, got %d", context, len(wantMessages), len(out))
	}
	for i, want := range wantMessages {
		delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[i]))
		if err != nil {
			t.Fatalf("decode %s bag use command[%d]: %v", context, i, err)
		}
		if delivery.Type != chatproto.ChatTypeCommand || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != want {
			t.Fatalf("unexpected %s bag use command[%d]: %+v want %q", context, i, delivery, want)
		}
	}
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
