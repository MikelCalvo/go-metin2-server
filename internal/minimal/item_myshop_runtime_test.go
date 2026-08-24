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
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestGameRuntimeMyShopOpenEmitsShopSignWithoutInventoryMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopHost", 0x01030801, 0x02040801, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 801, Vnum: 27001, Count: 3, Slot: 5}}
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
	}})
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
	if len(out) != 1 {
		t.Fatalf("expected accepted MYSHOP to emit one SHOP_SIGN frame, got %d", len(out))
	}
	sign, err := shopproto.DecodeServerShopSign(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode accepted MYSHOP SHOP_SIGN: %v", err)
	}
	if sign.VID != owner.VID || sign.Sign != "Private Shop" {
		t.Fatalf("unexpected accepted MYSHOP SHOP_SIGN: %+v", sign)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected accepted MYSHOP to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "accepted myshop open")
}

func TestGameRuntimeMyShopOpenRejectsEmptySignAndZeroCountWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopEmpty", 0x01030802, 0x02040802, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 802, Vnum: 27001, Count: 3, Slot: 5}}
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
	}})
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
		name  string
		items []shopproto.ClientMyShopItem
	}{
		{name: "duplicate pos", items: []shopproto.ClientMyShopItem{validItem, {Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 2000, DisplayPos: 1}}},
		{name: "missing cell", items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 1, Position: itemproto.InventoryPosition(9), Price: 1500}}},
		{name: "locked cell", items: []shopproto.ClientMyShopItem{{Vnum: 27002, Count: 1, Position: itemproto.InventoryPosition(6), Price: 1500}}},
		{name: "zero price", items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 3, Position: itemproto.InventoryPosition(5), Price: 0}}},
		{name: "count mismatch", items: []shopproto.ClientMyShopItem{{Vnum: 27001, Count: 2, Position: itemproto.InventoryPosition(5), Price: 1500}}},
		{name: "vnum mismatch", items: []shopproto.ClientMyShopItem{{Vnum: 27099, Count: 3, Position: itemproto.InventoryPosition(5), Price: 1500}}},
		{name: "anti_myshop", items: []shopproto.ClientMyShopItem{{Vnum: 27003, Count: 1, Position: itemproto.InventoryPosition(7), Price: 1500}}},
		{name: "anti_give", items: []shopproto.ClientMyShopItem{{Vnum: 27004, Count: 1, Position: itemproto.InventoryPosition(8), Price: 1500}}},
	} {
		out, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientMyShop(shopproto.ClientMyShopPacket{Sign: "Private Shop", Items: tc.items})))
		if err != nil {
			t.Fatalf("unexpected %s MYSHOP error: %v", tc.name, err)
		}
		if len(out) != 0 {
			t.Fatalf("expected %s MYSHOP to emit no frames, got %d", tc.name, len(out))
		}
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop invalid stock rejects")
}

func TestGameRuntimeMyShopOpenRejectsGoldOverflowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopGoldCap", 0x01030804, 0x02040804, 1100, 2100, 0, 101, 201)
	owner.Gold = uint64(math.MaxInt32) - 100
	owner.Inventory = []inventory.ItemInstance{{ID: 808, Vnum: 27001, Count: 1, Slot: 5}}
	login := "myshop-goldcap"
	issuePeerTicket(t, ticketStore, login, 0x70707104, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed myshop goldcap account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Shop Potion", Stackable: true, MaxCount: 200}})
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
	if len(out) != 0 {
		t.Fatalf("expected gold-overflow MYSHOP to emit no frames, got %d", len(out))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop gold overflow")
}

func TestGameRuntimeMyShopOpenBusyShellRejectsWithInfoChatWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("MyShopBusy", 0x01030805, 0x02040805, 12345, []inventory.ItemInstance{
		{ID: 809, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 810, Vnum: 11234, Count: 1, Slot: 6},
		{ID: 811, Vnum: 27001, Count: 2, Slot: 7},
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
	if len(acceptedOut) != 1 {
		t.Fatalf("expected accepted MYSHOP after busy clears to emit one SHOP_SIGN frame, got %d", len(acceptedOut))
	}
	if _, err := shopproto.DecodeServerShopSign(decodeSingleFrame(t, acceptedOut[0])); err != nil {
		t.Fatalf("decode accepted MYSHOP SHOP_SIGN after busy clears: %v", err)
	}
	assertMyShopBusyReject(t, flow, openPacket, "already-open myshop busy")

	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop busy rejects")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop busy peer")
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
			owner.Inventory = []inventory.ItemInstance{{ID: uint64(tc.ownerID), Vnum: 27001, Count: 3, Slot: 5}}
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
			}})
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
			if len(openOut) != 1 {
				t.Fatalf("expected %s accepted MYSHOP to emit one SHOP_SIGN frame, got %d", tc.name, len(openOut))
			}

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
			assertExchangeAccountUnchanged(t, accounts, tc.login, owner, tc.name+" myshop lifecycle close")
		})
	}
}

func TestGameRuntimeMyShopCloseSlashClearsOpenSignWithoutInventoryMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopCloseSlash", 0x01030814, 0x02040814, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 814, Vnum: 27001, Count: 3, Slot: 5}}
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
	}})
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
	if len(openOut) != 1 {
		t.Fatalf("expected accepted MYSHOP before close slash to emit one SHOP_SIGN frame, got %d", len(openOut))
	}

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
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop close slash")
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
	owner.Inventory = []inventory.ItemInstance{{ID: 821, Vnum: 27001, Count: 3, Slot: 5}}
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
	}})
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
	if len(openOut) != 1 {
		t.Fatalf("expected accepted MYSHOP before exchange start to emit one SHOP_SIGN frame, got %d", len(openOut))
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
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "myshop-open exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "myshop-open exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsPartnerActiveMyShopWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchPartnerMyShopOwner", 0x01030823, 0x02040823, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 823, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchPartnerMyShopPeer", 0x01030824, 0x02040824, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 824, Vnum: 27001, Count: 2, Slot: 6}}
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
	}})
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
	if len(openOut) != 1 {
		t.Fatalf("expected partner accepted MYSHOP before exchange start to emit one SHOP_SIGN frame, got %d", len(openOut))
	}

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
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "partner-myshop exchange start peer")
}

func TestGameRuntimeMyShopOpenLocksHostItemMutationsWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("MyShopLockHost", 0x01030831, 0x02040831, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{
		{ID: 831, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 832, Vnum: 27002, Count: 1, Slot: 6},
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
	if len(openOut) != 1 {
		t.Fatalf("expected accepted MYSHOP before mutation lock to emit one SHOP_SIGN frame, got %d", len(openOut))
	}

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
	assertExchangeAccountUnchanged(t, accounts, login, owner, "myshop open host mutation lock")

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
