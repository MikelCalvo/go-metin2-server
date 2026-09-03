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
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
)

func TestExchangePlaceIncomingDisplayedItemPreferringSlotsCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	sourceActiveSockets := inventory.SocketValues{1, 2, 3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	destZeroSockets := inventory.SocketValues{}
	destZeroAttributes := inventory.AttributeValues{}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	cases := []struct {
		name              string
		destination       inventory.ItemInstance
		source            inventory.ItemInstance
		wantHasSockets    bool
		wantHasAttributes bool
		wantSockets       inventory.SocketValues
		wantAttributes    inventory.AttributeValues
	}{
		{
			name: "active destination wins over different source",
			destination: inventory.ItemInstance{
				ID: 501, Vnum: 27001, Count: 4, Slot: 5,
				Sockets: &destActiveSockets, Attributes: &destActiveAttributes,
			},
			source: inventory.ItemInstance{
				ID: 502, Vnum: 27001, Count: 3, Slot: 7,
				Sockets: &sourceActiveSockets, Attributes: &sourceActiveAttributes,
			},
			wantHasSockets: true, wantHasAttributes: true,
			wantSockets: destActiveSockets, wantAttributes: destActiveAttributes,
		},
		{
			name: "explicit-zero destination wins over active source",
			destination: inventory.ItemInstance{
				ID: 511, Vnum: 27001, Count: 4, Slot: 5,
				Sockets: &destZeroSockets, Attributes: &destZeroAttributes,
			},
			source: inventory.ItemInstance{
				ID: 512, Vnum: 27001, Count: 3, Slot: 7,
				Sockets: &sourceActiveSockets, Attributes: &sourceActiveAttributes,
			},
			wantHasSockets: true, wantHasAttributes: true,
			wantSockets: destZeroSockets, wantAttributes: destZeroAttributes,
		},
		{
			name: "omitted destination stays omitted",
			destination: inventory.ItemInstance{
				ID: 521, Vnum: 27001, Count: 4, Slot: 5,
			},
			source: inventory.ItemInstance{
				ID: 522, Vnum: 27001, Count: 3, Slot: 7,
				Sockets: &sourceActiveSockets, Attributes: &sourceActiveAttributes,
			},
			wantHasSockets: false, wantHasAttributes: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := []inventory.ItemInstance{tc.destination}
			display := exchangeDisplayedItem{ItemID: tc.source.ID, Vnum: tc.source.Vnum, Count: tc.source.Count, Slot: tc.source.Slot}
			if !exchangePlaceIncomingDisplayedItemPreferringSlots(&items, display, template, nil, tc.source) {
				t.Fatal("expected exchange compatible merge placement to succeed")
			}
			if len(items) != 1 {
				t.Fatalf("expected count-only merge without fresh cell, got %#v", items)
			}
			got := items[0]
			if got.ID != tc.destination.ID || got.Slot != tc.destination.Slot || got.Count != tc.destination.Count+tc.source.Count {
				t.Fatalf("unexpected merged destination identity/count: %+v", got)
			}
			if got.HasSockets() != tc.wantHasSockets {
				t.Fatalf("HasSockets=%v want %v", got.HasSockets(), tc.wantHasSockets)
			}
			if got.HasAttributes() != tc.wantHasAttributes {
				t.Fatalf("HasAttributes=%v want %v", got.HasAttributes(), tc.wantHasAttributes)
			}
			if tc.wantHasSockets {
				if got.Sockets == nil || *got.Sockets != tc.wantSockets {
					t.Fatalf("expected destination sockets %+v, got %#v", tc.wantSockets, got.Sockets)
				}
				if got.Sockets == tc.source.Sockets {
					t.Fatal("merged destination sockets aliased discarded source")
				}
			} else if got.Sockets != nil {
				t.Fatalf("expected omitted destination sockets, got %#v", got.Sockets)
			}
			if tc.wantHasAttributes {
				if got.Attributes == nil || *got.Attributes != tc.wantAttributes {
					t.Fatalf("expected destination attributes %+v, got %#v", tc.wantAttributes, got.Attributes)
				}
				if got.Attributes == tc.source.Attributes {
					t.Fatal("merged destination attributes aliased discarded source")
				}
			} else if got.Attributes != nil {
				t.Fatalf("expected omitted destination attributes, got %#v", got.Attributes)
			}
		})
	}

	t.Run("free-cell preserve stays source-preserving", func(t *testing.T) {
		items := []inventory.ItemInstance{}
		sourceSockets := inventory.SocketValues{11, 0, -3}
		sourceAttributes := inventory.AttributeValues{{Type: 4, Value: 55}}
		source := inventory.ItemInstance{
			ID: 531, Vnum: 27001, Count: 3, Slot: 7,
			Sockets: &sourceSockets, Attributes: &sourceAttributes,
		}
		display := exchangeDisplayedItem{ItemID: source.ID, Vnum: source.Vnum, Count: source.Count, Slot: source.Slot}
		if !exchangePlaceIncomingDisplayedItemPreferringSlots(&items, display, template, []inventory.SlotIndex{8}, source) {
			t.Fatal("expected exchange free-cell placement to succeed")
		}
		if len(items) != 1 || items[0].ID != 531 || items[0].Slot != 8 || items[0].Count != 3 {
			t.Fatalf("unexpected free-cell placement: %#v", items)
		}
		if !items[0].HasSockets() || *items[0].Sockets != sourceSockets {
			t.Fatalf("expected source-preserving free-cell sockets, got %#v", items[0].Sockets)
		}
		if !items[0].HasAttributes() || *items[0].Attributes != sourceAttributes {
			t.Fatalf("expected source-preserving free-cell attributes, got %#v", items[0].Attributes)
		}
		if items[0].Sockets == source.Sockets || items[0].Attributes == source.Attributes {
			t.Fatal("expected free-cell placement to clone source presence independently")
		}
	})

	t.Run("locked compatible skipped", func(t *testing.T) {
		destSockets := inventory.SocketValues{7, 0, 9}
		sourceSockets := inventory.SocketValues{1, 2, 3}
		items := []inventory.ItemInstance{{
			ID: 541, Vnum: 27001, Count: 4, Slot: 5, Locked: true,
			Sockets: &destSockets,
		}}
		source := inventory.ItemInstance{
			ID: 542, Vnum: 27001, Count: 3, Slot: 7,
			Sockets: &sourceSockets,
		}
		display := exchangeDisplayedItem{ItemID: source.ID, Vnum: source.Vnum, Count: source.Count, Slot: source.Slot}
		if !exchangePlaceIncomingDisplayedItemPreferringSlots(&items, display, template, nil, source) {
			t.Fatal("expected locked destination to be skipped for free-cell placement")
		}
		if len(items) != 2 {
			t.Fatalf("expected locked cell preserved beside free-cell place, got %#v", items)
		}
		var locked, placed *inventory.ItemInstance
		for i := range items {
			switch items[i].ID {
			case 541:
				locked = &items[i]
			case 542:
				placed = &items[i]
			}
		}
		if locked == nil || !locked.Locked || locked.Count != 4 || !locked.HasSockets() || *locked.Sockets != destSockets {
			t.Fatalf("locked destination mutated: %#v", items)
		}
		if placed == nil || !placed.HasSockets() || *placed.Sockets != sourceSockets {
			t.Fatalf("expected source-preserving free-cell after locked skip: %#v", items)
		}
	})

	t.Run("already-full compatible rejects without mutation when no free cell", func(t *testing.T) {
		destSockets := inventory.SocketValues{7, 0, 9}
		sourceSockets := inventory.SocketValues{1, 2, 3}
		items := []inventory.ItemInstance{{
			ID: 551, Vnum: 27001, Count: 200, Slot: 5,
			Sockets: &destSockets,
		}}
		for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
			if slot == 5 {
				continue
			}
			items = append(items, inventory.ItemInstance{ID: uint64(2000 + slot), Vnum: 28000 + uint32(slot), Count: 1, Slot: slot})
		}
		before := append([]inventory.ItemInstance(nil), items...)
		source := inventory.ItemInstance{
			ID: 552, Vnum: 27001, Count: 2, Slot: 7,
			Sockets: &sourceSockets,
		}
		display := exchangeDisplayedItem{ItemID: source.ID, Vnum: source.Vnum, Count: source.Count, Slot: source.Slot}
		if exchangePlaceIncomingDisplayedItemPreferringSlots(&items, display, template, nil, source) {
			t.Fatal("expected already-full compatible exchange merge without free capacity to fail closed")
		}
		if !reflect.DeepEqual(items, before) {
			t.Fatalf("already-full exchange merge mutated working inventory:\ngot:  %#v\nwant: %#v", items, before)
		}
	})
}

func TestGameRuntimeItemMoveCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	destSockets := inventory.SocketValues{7, 0, 9}
	destAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	sourceSockets := inventory.SocketValues{1, 2, 3}
	sourceAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	templateSockets := itemcatalog.SocketValues{11, -22, 33}
	templateAttributes := itemcatalog.AttributeValues{{Type: 3, Value: 30}, {Type: 4, Value: -5}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:       27001,
		Name:       "Small Red Potion",
		Stackable:  true,
		MaxCount:   200,
		Sockets:    templateSockets,
		Attributes: templateAttributes,
	}})
	owner := peerVisibilityCharacter("ItemMoveDestWins", 0x01030941, 0x02040941, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1941, Vnum: 27001, Count: 5, Slot: 5, Sockets: &sourceSockets, Attributes: &sourceAttributes},
		{ID: 1942, Vnum: 27001, Count: 7, Slot: 8, Sockets: &destSockets, Attributes: &destAttributes},
	}
	login := "item-move-dest-wins"
	issuePeerTicket(t, ticketStore, login, 0x69696941, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-move destination-wins owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-move destination-wins runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x69696941)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected item-move destination-wins merge error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected item-move destination-wins merge to emit source+destination ITEM_UPDATE, got %d", len(out))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode item-move destination-wins source update: %v", err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 3 {
		t.Fatalf("unexpected item-move destination-wins source refresh: %+v", sourceUpdate)
	}
	if sourceUpdate.Sockets != ([itemproto.ItemSocketCount]int32{1, 2, 3}) {
		t.Fatalf("expected source ITEM_UPDATE to keep source instance sockets, got %+v", sourceUpdate.Sockets)
	}
	destinationUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode item-move destination-wins destination update: %v", err)
	}
	if destinationUpdate.Position != itemproto.InventoryPosition(8) || destinationUpdate.Count != 9 {
		t.Fatalf("unexpected item-move destination-wins destination refresh: %+v", destinationUpdate)
	}
	if destinationUpdate.Sockets != ([itemproto.ItemSocketCount]int32{7, 0, 9}) {
		t.Fatalf("expected destination ITEM_UPDATE sockets to stay destination presence, got %+v", destinationUpdate.Sockets)
	}
	if destinationUpdate.Attributes[0] != (itemproto.Attribute{Type: 1, Value: 25}) || destinationUpdate.Attributes[1] != (itemproto.Attribute{Type: 7, Value: -3}) {
		t.Fatalf("expected destination ITEM_UPDATE attributes to stay destination presence, got %+v", destinationUpdate.Attributes)
	}

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load item-move destination-wins account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 1941, Vnum: 27001, Count: 3, Slot: 5, Sockets: &sourceSockets, Attributes: &sourceAttributes},
		{ID: 1942, Vnum: 27001, Count: 9, Slot: 8, Sockets: &destSockets, Attributes: &destAttributes},
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("item-move destination-wins persisted inventory mismatch:\ngot:  %#v\nwant: %#v", account.Characters[0].Inventory, wantInventory)
	}
}

func TestGameRuntimeSafeboxItemMoveCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	destSockets := inventory.SocketValues{7, 0, 9}
	destAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	sourceSockets := inventory.SocketValues{1, 2, 3}
	sourceAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	owner := peerVisibilityCharacter("SafeboxMoveDestWins", 0x01030951, 0x02040951, 1100, 2100, 0, 101, 201)
	owner.Gold = 1414
	owner.Inventory = []inventory.ItemInstance{
		{ID: 851, Vnum: 27001, Count: 4, Slot: 5, Sockets: &sourceSockets, Attributes: &sourceAttributes},
		{ID: 852, Vnum: 27001, Count: 3, Slot: 6, Sockets: &destSockets, Attributes: &destAttributes},
	}
	login := "safebox-move-dest-wins"
	issuePeerTicket(t, ticketStore, login, 0x70707051, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox destination-wins owner account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:       27001,
		Name:       "Small Red Potion",
		Stackable:  true,
		MaxCount:   200,
		Sockets:    itemcatalog.SocketValues{11, -22, 33},
		Attributes: itemcatalog.AttributeValues{{Type: 3, Value: 30}},
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox destination-wins runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70707051)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before destination-wins merge: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected first safebox check-in before destination-wins merge: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 1,
		Position: itemproto.InventoryPosition(6),
	}))); err != nil {
		t.Fatalf("unexpected second safebox check-in before destination-wins merge: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
		Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1},
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected safebox destination-wins merge error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected safebox destination-wins merge to emit two SAFEBOX_SET frames, got %d", len(out))
	}
	sourceSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode safebox destination-wins source SAFEBOX_SET: %v", err)
	}
	if sourceSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || sourceSet.Vnum != 27001 || sourceSet.Count != 2 {
		t.Fatalf("unexpected safebox destination-wins source SAFEBOX_SET: %+v", sourceSet)
	}
	if sourceSet.Sockets != ([itemproto.ItemSocketCount]int32{1, 2, 3}) {
		t.Fatalf("expected source SAFEBOX_SET to keep source instance sockets, got %+v", sourceSet.Sockets)
	}
	destinationSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode safebox destination-wins destination SAFEBOX_SET: %v", err)
	}
	if destinationSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1}) || destinationSet.Vnum != 27001 || destinationSet.Count != 5 {
		t.Fatalf("unexpected safebox destination-wins destination SAFEBOX_SET: %+v", destinationSet)
	}
	if destinationSet.Sockets != ([itemproto.ItemSocketCount]int32{7, 0, 9}) {
		t.Fatalf("expected destination SAFEBOX_SET sockets to stay destination presence, got %+v", destinationSet.Sockets)
	}
	if destinationSet.Attributes[0] != (itemproto.Attribute{Type: 1, Value: 25}) || destinationSet.Attributes[1] != (itemproto.Attribute{Type: 7, Value: -3}) {
		t.Fatalf("expected destination SAFEBOX_SET attributes to stay destination presence, got %+v", destinationSet.Attributes)
	}

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen after destination-wins merge: %v", err)
	}
	if len(reopenOut) != 4 {
		t.Fatalf("expected /open_safebox reopen after destination-wins merge to emit SAFEBOX_SIZE plus two SAFEBOX_SET rows, got %d", len(reopenOut))
	}
	reopenDestination, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[2]))
	if err != nil {
		t.Fatalf("decode reopen destination SAFEBOX_SET after destination-wins merge: %v", err)
	}
	if reopenDestination != destinationSet {
		t.Fatalf("unexpected reopen destination SAFEBOX_SET after destination-wins merge: %+v want %+v", reopenDestination, destinationSet)
	}
}

func TestGameRuntimeMyShopGuestBuyCompatibleMergeKeepsDestinationInstancePresence(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())

	hostSockets := inventory.SocketValues{1, 2, 3}
	hostAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	guestDestSockets := inventory.SocketValues{7, 0, 9}
	guestDestAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}

	owner := peerVisibilityCharacter("MyShopMergeHost", 0x01030961, 0x02040961, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 961, Vnum: 27001, Count: 3, Slot: 5, Sockets: &hostSockets, Attributes: &hostAttributes},
		{ID: 971, Vnum: myShopOpenShopBagVnum, Count: 1, Slot: 4},
	}
	peer := peerVisibilityCharacter("MyShopMergeGuest", 0x01030962, 0x02040962, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{
		{ID: 962, Vnum: 27001, Count: 4, Slot: 6, Sockets: &guestDestSockets, Attributes: &guestDestAttributes},
	}
	ownerLogin := "myshop-merge-host"
	peerLogin := "myshop-merge-guest"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707161, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707162, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop merge host account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed myshop merge guest account: %v", err)
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
		t.Fatalf("unexpected myshop merge runtime error: %v", err)
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
		Sign: "Merge Shop",
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
	assertMyShopOpenSuccessBagAndSignWithText(t, openOut, owner.VID, "Merge Shop", 4, `unexpected accepted MYSHOP before merge guest buy: out=%d err=%v`)
	_ = flushServerFrames(t, peerFlow)

	browseOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: owner.VID})))
	if err != nil || len(browseOut) != 1 {
		t.Fatalf("unexpected guest browse before merge buy: out=%d err=%v", len(browseOut), err)
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, browseOut[0])); err != nil {
		t.Fatalf("decode guest browse SHOP START before merge buy: %v", err)
	}

	buyOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{
		RawLeadingByte: 1,
		CatalogSlot:    displayPos,
	})))
	if err != nil {
		t.Fatalf("unexpected guest private-shop SHOP BUY merge: %v", err)
	}
	if len(buyOut) == 0 {
		t.Fatalf("expected guest private-shop SHOP BUY merge to mutate and emit frames, got none")
	}

	var guestItemUpdate *itemproto.UpdatePacket
	for _, raw := range buyOut {
		f := decodeSingleFrame(t, raw)
		if update, err := itemproto.DecodeUpdate(f); err == nil {
			if update.Position == itemproto.InventoryPosition(6) && update.Count == 7 {
				copied := update
				guestItemUpdate = &copied
			}
		}
	}
	if guestItemUpdate == nil {
		t.Fatalf("expected guest ITEM_UPDATE for destination-wins merge count 7, frames=%d", len(buyOut))
	}
	if guestItemUpdate.Sockets != ([itemproto.ItemSocketCount]int32{7, 0, 9}) {
		t.Fatalf("expected guest merge ITEM_UPDATE sockets to stay destination presence, got %+v", guestItemUpdate.Sockets)
	}
	if guestItemUpdate.Attributes[0] != (itemproto.Attribute{Type: 1, Value: 25}) || guestItemUpdate.Attributes[1] != (itemproto.Attribute{Type: 7, Value: -3}) {
		t.Fatalf("expected guest merge ITEM_UPDATE attributes to stay destination presence, got %+v", guestItemUpdate.Attributes)
	}

	peerAccount, err := accounts.Load(peerLogin)
	if err != nil {
		t.Fatalf("load merge guest account: %v", err)
	}
	persistedPeer := findPersistedCharacter(t, peerAccount, peer.Name)
	if len(persistedPeer.Inventory) != 1 || persistedPeer.Inventory[0].ID != 962 || persistedPeer.Inventory[0].Vnum != 27001 || persistedPeer.Inventory[0].Count != 7 || persistedPeer.Inventory[0].Slot != 6 {
		t.Fatalf("unexpected merge guest inventory after buy: %+v", persistedPeer.Inventory)
	}
	if !persistedPeer.Inventory[0].HasSockets() || *persistedPeer.Inventory[0].Sockets != guestDestSockets {
		t.Fatalf("expected merge guest inventory sockets to stay destination presence %+v, got %+v", guestDestSockets, persistedPeer.Inventory[0].Sockets)
	}
	if !persistedPeer.Inventory[0].HasAttributes() || *persistedPeer.Inventory[0].Attributes != guestDestAttributes {
		t.Fatalf("expected merge guest inventory attributes to stay destination presence %+v, got %+v", guestDestAttributes, persistedPeer.Inventory[0].Attributes)
	}
}
