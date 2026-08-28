package minimal

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/player"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestGameRuntimeItemExchangeStartOpensVisiblePeerWindowsWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeStarter", 0x01030763, 0x02040763, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 704, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeTarget", 0x01030764, 0x02040764, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 705, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-exchange-starter", 0x70707063, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-target", 0x70707064, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-starter", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange starter account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-target", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange target account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected exchange-start runtime error: %v", err)
	}
	starterFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-starter", 0x70707063)
	defer closeSessionFlow(t, starterFlow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-target", 0x70707064)
	defer closeSessionFlow(t, targetFlow)
	_ = flushServerFrames(t, starterFlow)
	_ = flushServerFrames(t, targetFlow)

	out, err := starterFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected exchange-start packet error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected exchange starter to receive one start frame, got %d", len(out))
	}
	assertExchangeStartFrame(t, out[0], peer.VID, "starter response")
	queued := flushServerFrames(t, targetFlow)
	if len(queued) != 1 {
		t.Fatalf("expected exchange target to receive one queued start frame, got %d", len(queued))
	}
	assertExchangeStartFrame(t, queued[0], owner.VID, "target queued response")

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-starter", owner, "starter exchange start")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-target", peer, "target exchange start")
}

func TestGameRuntimeItemExchangeStartRejectsOutOfRangeVisiblePeerWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeRangeStarter", 0x010307e0, 0x020407e0, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 780, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeRangeTarget", 0x010307e1, 0x020407e1, 1100+1041, 2100, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 781, Vnum: 27002, Count: 2, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-exchange-range-starter", 0x707070e0, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-range-target", 0x707070e1, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-range-starter", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed range exchange starter account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-range-target", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed range exchange target account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected range exchange-start runtime error: %v", err)
	}
	starterFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-range-starter", 0x707070e0)
	defer closeSessionFlow(t, starterFlow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-range-target", 0x707070e1)
	defer closeSessionFlow(t, targetFlow)
	_ = flushServerFrames(t, starterFlow)
	_ = flushServerFrames(t, targetFlow)

	out, err := starterFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected range exchange-start packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected out-of-range exchange start to emit no starter frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, targetFlow); len(queued) != 0 {
		t.Fatalf("expected out-of-range exchange start to queue no target frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-range-starter", owner, "range starter exchange start")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-range-target", peer, "range target exchange start")
}

func TestGameRuntimeItemExchangeStartRejectsActiveMerchantWindowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeMerchantStartOwner", 0x010307e4, 0x020407e4, 1100, 2100, 0, 101, 201)
	owner.Gold = 125
	owner.Inventory = []inventory.ItemInstance{{ID: 784, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeMerchantStartPeer", 0x010307e5, 0x020407e5, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 785, Vnum: 27002, Count: 2, Slot: 6}}
	ownerLogin := "exch-merch-start-owner"
	peerLogin := "exch-merch-start-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070e4, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070e5, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed merchant-open exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed merchant-open exchange peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27050, Name: "Merchant Start Guard Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	merchantDefinition := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:exchange_start_merchant_guard",
		Title: "Exchange Start Merchant Guard",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27050, Price: 50, Count: 1},
		},
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{merchantDefinition})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected merchant-open exchange runtime error: %v", err)
	}
	merchant, ok := runtime.RegisterStaticActorWithInteraction("ExchangeStartMerchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, merchantDefinition.Ref)
	if !ok {
		t.Fatal("expected merchant-open exchange static actor registration to succeed")
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070e4)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070e5)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	merchantOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(merchant.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected merchant-open interaction error: %v", err)
	}
	if len(merchantOut) != 1 {
		t.Fatalf("expected merchant-open interaction to emit one shop start frame, got %d", len(merchantOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, merchantOut[0])); err != nil {
		t.Fatalf("decode merchant-open shop start: %v", err)
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected merchant-open exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange start with open merchant window to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode merchant-open exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected merchant-open exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected exchange start with open merchant window to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "merchant-open exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "merchant-open exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsPartnerActiveMerchantWindowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangePartnerMerchantOwner", 0x010307e6, 0x020407e6, 1100, 2100, 0, 101, 201)
	owner.Gold = 125
	owner.Inventory = []inventory.ItemInstance{{ID: 786, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangePartnerMerchantPeer", 0x010307e7, 0x020407e7, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 787, Vnum: 27002, Count: 2, Slot: 6}}
	ownerLogin := "exch-partner-merch-owner"
	peerLogin := "exch-partner-merch-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070e6, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070e7, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partner-merchant exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partner-merchant exchange peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27051, Name: "Partner Merchant Guard Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	merchantDefinition := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:exchange_partner_merchant_guard",
		Title: "Exchange Partner Merchant Guard",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27051, Price: 50, Count: 1},
		},
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{merchantDefinition})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected partner-merchant exchange runtime error: %v", err)
	}
	merchant, ok := runtime.RegisterStaticActorWithInteraction("ExchangePartnerMerchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, merchantDefinition.Ref)
	if !ok {
		t.Fatal("expected partner-merchant exchange static actor registration to succeed")
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070e6)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070e7)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	merchantOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(merchant.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected partner merchant-open interaction error: %v", err)
	}
	if len(merchantOut) != 1 {
		t.Fatalf("expected partner merchant-open interaction to emit one shop start frame, got %d", len(merchantOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, merchantOut[0])); err != nil {
		t.Fatalf("decode partner merchant-open shop start: %v", err)
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected partner-merchant exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-merchant exchange start to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode partner-merchant exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner-merchant exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected partner-merchant exchange start to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "partner-merchant exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "partner-merchant exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsActiveSafeboxWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchSafeboxStartOwner", 0x010307ea, 0x020407ea, 1100, 2100, 0, 101, 201)
	owner.Gold = 125
	owner.Inventory = []inventory.ItemInstance{{ID: 788, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchSafeboxStartPeer", 0x010307eb, 0x020407eb, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 789, Vnum: 27002, Count: 2, Slot: 6}}
	ownerLogin := "exch-safe-start-owner"
	peerLogin := "exch-safe-start-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070ea, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070eb, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox-open exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed safebox-open exchange peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected safebox-open exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070ea)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070eb)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected requester /open_safebox error: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected requester /open_safebox to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}
	if _, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0])); err != nil {
		t.Fatalf("decode requester /open_safebox SAFEBOX_SIZE: %v", err)
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected safebox-open exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange start with open safebox to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode safebox-open exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected safebox-open exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected exchange start with open safebox to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "safebox-open exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "safebox-open exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsPartnerActiveSafeboxWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchPartnerSafeOwner", 0x010307ec, 0x020407ec, 1100, 2100, 0, 101, 201)
	owner.Gold = 125
	owner.Inventory = []inventory.ItemInstance{{ID: 790, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchPartnerSafePeer", 0x010307ed, 0x020407ed, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 791, Vnum: 27002, Count: 2, Slot: 6}}
	ownerLogin := "exch-partner-safe-owner"
	peerLogin := "exch-partner-safe-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070ec, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070ed, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partner-safebox exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partner-safebox exchange peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected partner-safebox exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070ec)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070ed)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	openOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected partner /open_safebox error: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected partner /open_safebox to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}
	if _, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0])); err != nil {
		t.Fatalf("decode partner /open_safebox SAFEBOX_SIZE: %v", err)
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected partner-safebox exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-safebox exchange start to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode partner-safebox exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner-safebox exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected partner-safebox exchange start to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "partner-safebox exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "partner-safebox exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsActiveRefineDialogWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchRefineStartOwner", 0x010307ee, 0x020407ee, 1100, 2100, 0, 101, 201)
	owner.Gold = 125
	owner.Inventory = []inventory.ItemInstance{{ID: 792, Vnum: 11260, Count: 1, Slot: 5}}
	peer := peerVisibilityCharacter("ExchRefineStartPeer", 0x010307ef, 0x020407ef, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 793, Vnum: 27002, Count: 2, Slot: 6}}
	ownerLogin := "exch-refine-start-owner"
	peerLogin := "exch-refine-start-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070ee, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070ef, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed refine-open exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed refine-open exchange peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:       11260,
		Name:       "Exchange Busy Refine Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11261,
			Cost:        1000,
			Probability: 100,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 1}},
		},
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected refine-open exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070ee)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070ef)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	previewOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 5, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected requester refine preview error: %v", err)
	}
	if len(previewOut) != 1 {
		t.Fatalf("expected requester refine preview to emit one frame, got %d", len(previewOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode requester refine preview: %v", err)
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected refine-open exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange start with open refine dialog to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode refine-open exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected refine-open exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected exchange start with open refine dialog to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "refine-open exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "refine-open exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsPartnerActiveRefineDialogWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchPartnerRefineOwner", 0x010307f0, 0x020407f0, 1100, 2100, 0, 101, 201)
	owner.Gold = 125
	owner.Inventory = []inventory.ItemInstance{{ID: 794, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchPartnerRefinePeer", 0x010307f1, 0x020407f1, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 795, Vnum: 11262, Count: 1, Slot: 6}}
	ownerLogin := "exch-partner-refine-owner"
	peerLogin := "exch-partner-refine-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070f0, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070f1, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partner-refine exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partner-refine exchange peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:       11262,
		Name:       "Partner Busy Refine Blade",
		Stackable:  false,
		MaxCount:   1,
		Refineable: true,
		RefineInfo: &itemcatalog.RefineInfo{
			ResultVnum:  11263,
			Cost:        1000,
			Probability: 100,
			Materials:   []itemcatalog.RefineMaterial{{Vnum: 27001, Count: 1}},
		},
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected partner-refine exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070f0)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070f1)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	previewOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientRefine(itemproto.ClientRefinePacket{Position: 6, Type: 3})))
	if err != nil {
		t.Fatalf("unexpected partner refine preview error: %v", err)
	}
	if len(previewOut) != 1 {
		t.Fatalf("expected partner refine preview to emit one frame, got %d", len(previewOut))
	}
	if _, err := itemproto.DecodeRefineInformationNew(decodeSingleFrame(t, previewOut[0])); err != nil {
		t.Fatalf("decode partner refine preview: %v", err)
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected partner-refine exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-refine exchange start to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode partner-refine exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner-refine exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected partner-refine exchange start to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "partner-refine exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "partner-refine exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsRequesterGoldCarrierWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchGoldCapOwner", 0x01030201, 0x02040201, 1100, 2100, 0, 101, 201)
	owner.Gold = exchangeGoldPointChangeCarrierMax
	owner.Inventory = []inventory.ItemInstance{{ID: 8201, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchGoldCapPeer", 0x01030202, 0x02040202, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 8202, Vnum: 27002, Count: 2, Slot: 6}}
	ownerLogin := "exch-gold-carrier-start-owner"
	peerLogin := "exch-gold-carrier-start-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707201, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707202, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed requester gold-carrier exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed requester gold-carrier exchange peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected requester gold-carrier exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707201)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707202)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected requester gold-carrier exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected requester gold-carrier exchange start to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode requester gold-carrier exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterGoldCarrierCapInfoMessage {
		t.Fatalf("unexpected requester gold-carrier exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected requester gold-carrier exchange start to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "requester gold-carrier exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "requester gold-carrier exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsPartnerGoldCarrierWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchGoldCapPartnerOwner", 0x01030203, 0x02040203, 1100, 2100, 0, 101, 201)
	owner.Gold = 125
	owner.Inventory = []inventory.ItemInstance{{ID: 8203, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchGoldCapPartnerPeer", 0x01030204, 0x02040204, 1120, 2120, 0, 101, 201)
	peer.Gold = exchangeGoldPointChangeCarrierMax
	peer.Inventory = []inventory.ItemInstance{{ID: 8204, Vnum: 27002, Count: 2, Slot: 6}}
	ownerLogin := "exch-gold-cap-partner-owner"
	peerLogin := "exch-gold-cap-partner-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707203, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707204, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partner gold-carrier exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partner gold-carrier exchange peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected partner gold-carrier exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707203)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707204)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected partner gold-carrier exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner gold-carrier exchange start to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode partner gold-carrier exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerGoldCarrierCapInfoMessage {
		t.Fatalf("unexpected partner gold-carrier exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected partner gold-carrier exchange start to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "partner gold-carrier exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "partner gold-carrier exchange start peer")
}

func TestGameRuntimeItemExchangeStartRejectsRequesterGoldCarrierWhenBothOverCapWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchGoldCapBothOwner", 0x01030205, 0x02040205, 1100, 2100, 0, 101, 201)
	owner.Gold = exchangeGoldPointChangeCarrierMax
	owner.Inventory = []inventory.ItemInstance{{ID: 8205, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchGoldCapBothPeer", 0x01030206, 0x02040206, 1120, 2120, 0, 101, 201)
	peer.Gold = exchangeGoldPointChangeCarrierMax
	peer.Inventory = []inventory.ItemInstance{{ID: 8206, Vnum: 27002, Count: 2, Slot: 6}}
	ownerLogin := "exch-gold-carrier-both-owner"
	peerLogin := "exch-gold-carrier-both-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707205, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707206, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed both gold-carrier exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed both gold-carrier exchange peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected both gold-carrier exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707205)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707206)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected both gold-carrier exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected both gold-carrier exchange start to emit one info chat frame, got %d", len(startOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, startOut[0]))
	if err != nil {
		t.Fatalf("decode both gold-carrier exchange start info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterGoldCarrierCapInfoMessage {
		t.Fatalf("unexpected both gold-carrier exchange start info chat: %+v", infoChat)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected both gold-carrier exchange start to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "both gold-carrier exchange start owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "both gold-carrier exchange start peer")
}

func TestGameRuntimeItemExchangeWalkAwayClosesShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeWalkOwner", 0x010307e2, 0x020407e2, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 782, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeWalkPeer", 0x010307e3, 0x020407e3, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 783, Vnum: 27002, Count: 2, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-exchange-walk-owner", 0x707070e2, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-walk-peer", 0x707070e3, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-walk-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed walk-away exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-walk-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed walk-away exchange peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected walk-away exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-walk-owner", 0x707070e2)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-walk-peer", 0x707070e3)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected walk-away exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected walk-away exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "walk-away owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected walk-away exchange peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "walk-away peer start")

	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    peer.X + 1041,
		Y:    peer.Y,
		Time: 0x61626364,
	})))
	if err != nil {
		t.Fatalf("unexpected walk-away move error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected walk-away move to emit one move ack, got %d frames", len(moveOut))
	}
	foundOwnerEnd := false
	for _, frame := range flushServerFrames(t, ownerFlow) {
		if exchangeFrameIsEnd(t, frame) {
			foundOwnerEnd = true
			break
		}
	}
	if !foundOwnerEnd {
		t.Fatal("expected walk-away move to deliver self GC::EXCHANGE END")
	}
	foundPeerEnd := false
	for _, frame := range flushServerFrames(t, peerFlow) {
		if exchangeFrameIsEnd(t, frame) {
			foundPeerEnd = true
			break
		}
	}
	if !foundPeerEnd {
		t.Fatal("expected walk-away move to queue peer GC::EXCHANGE END")
	}

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-walk-owner", owner, "walk-away owner")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-walk-peer", peer, "walk-away peer")
}

func TestGameRuntimeItemExchangeTransferTriggerClosesShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeXferOwner", 0x010308a1, 0x020408a1, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 841, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeXferPeer", 0x010308a2, 0x020408a2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 842, Vnum: 27002, Count: 2, Slot: 6}}
	ownerLogin := "item-exchange-xfer-owner"
	peerLogin := "item-exchange-xfer-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070a1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070a2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed transfer exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed transfer exchange peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}})
	if err != nil {
		t.Fatalf("unexpected transfer exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070a1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070a2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected transfer exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected transfer exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "transfer owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected transfer exchange peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "transfer peer start")

	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1500,
		Y:    2600,
		Time: 0x61626370,
	})))
	if err != nil {
		t.Fatalf("unexpected transfer-trigger move error with open exchange shell: %v", err)
	}
	if len(moveOut) < 2 {
		t.Fatalf("expected transfer-triggered exchange close to prepend END before transfer frames, got %d", len(moveOut))
	}
	assertExchangeEndFrame(t, moveOut[0], "transfer owner self END")
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode self transfer add after exchange close: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 1700 || selfAdd.Y != 2800 {
		t.Fatalf("unexpected self transfer add after exchange close: %+v", selfAdd)
	}
	queuedPeer := flushServerFrames(t, peerFlow)
	foundPeerEnd := false
	for _, frame := range queuedPeer {
		if exchangeFrameIsEnd(t, frame) {
			foundPeerEnd = true
			break
		}
	}
	if !foundPeerEnd {
		t.Fatalf("expected transfer-triggered exchange close to queue peer END, got %d frames", len(queuedPeer))
	}

	acceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderAccept,
	})))
	if err != nil {
		t.Fatalf("unexpected post-transfer accept error: %v", err)
	}
	if len(acceptOut) != 0 {
		t.Fatalf("expected post-transfer accept to fail closed with no frames, got %d", len(acceptOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected post-transfer accept to queue no peer frames, got %d", len(queued))
	}

	account, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted transfer exchange owner account: %v", err)
	}
	if account.Characters[0].MapIndex != 42 || account.Characters[0].X != 1700 || account.Characters[0].Y != 2800 {
		t.Fatalf("expected persisted transfer destination, got %#v", account.Characters[0])
	}
	if account.Characters[0].Gold != owner.Gold || !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected transfer teardown to leave inventory/gold unchanged, got %#v", account.Characters[0])
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "transfer peer")
}

func TestGameRuntimeItemExchangeSameMapInRangeTransferClosesShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeSameMapOwner", 0x010308a3, 0x020408a3, 1100, 2100, 0, 101, 201)
	owner.Gold = 11111
	owner.Inventory = []inventory.ItemInstance{{ID: 843, Vnum: 27001, Count: 1, Slot: 4}}
	peer := peerVisibilityCharacter("ExchangeSameMapPeer", 0x010308a4, 0x020408a4, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 844, Vnum: 27002, Count: 1, Slot: 5}}
	ownerLogin := "item-exchange-samemap-owner"
	peerLogin := "item-exchange-samemap-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070a3, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070a4, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed same-map transfer exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed same-map transfer exchange peer account: %v", err)
	}
	// Destination stays well inside the exchange-distance gate relative to the peer.
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: bootstrapMapIndex,
		TargetX:        1130,
		TargetY:        2130,
	}})
	if err != nil {
		t.Fatalf("unexpected same-map transfer exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070a3)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070a4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected same-map transfer exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected same-map transfer exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "same-map owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected same-map transfer exchange peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "same-map peer start")

	moveOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1500,
		Y:    2600,
		Time: 0x61626371,
	})))
	if err != nil {
		t.Fatalf("unexpected same-map transfer-trigger move error: %v", err)
	}
	if len(moveOut) < 2 {
		t.Fatalf("expected same-map transfer to prepend END before transfer frames, got %d", len(moveOut))
	}
	assertExchangeEndFrame(t, moveOut[0], "same-map owner self END")
	selfAdd, err := worldproto.DecodeCharacterAdd(decodeSingleFrame(t, moveOut[1]))
	if err != nil {
		t.Fatalf("decode same-map self transfer add after exchange close: %v", err)
	}
	if selfAdd.VID != owner.VID || selfAdd.X != 1130 || selfAdd.Y != 2130 {
		t.Fatalf("unexpected same-map self transfer add after exchange close: %+v", selfAdd)
	}
	queuedPeer := flushServerFrames(t, peerFlow)
	foundPeerEnd := false
	for _, frame := range queuedPeer {
		if exchangeFrameIsEnd(t, frame) {
			foundPeerEnd = true
			break
		}
	}
	if !foundPeerEnd {
		t.Fatalf("expected same-map transfer to queue peer END even while still in-range, got %d frames", len(queuedPeer))
	}

	acceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderAccept,
	})))
	if err != nil {
		t.Fatalf("unexpected post-same-map-transfer accept error: %v", err)
	}
	if len(acceptOut) != 0 {
		t.Fatalf("expected post-same-map-transfer accept to fail closed with no frames, got %d", len(acceptOut))
	}

	account, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted same-map transfer exchange owner account: %v", err)
	}
	if account.Characters[0].MapIndex != bootstrapMapIndex || account.Characters[0].X != 1130 || account.Characters[0].Y != 2130 {
		t.Fatalf("expected persisted same-map transfer destination, got %#v", account.Characters[0])
	}
	if account.Characters[0].Gold != owner.Gold || !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected same-map transfer teardown to leave inventory/gold unchanged, got %#v", account.Characters[0])
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "same-map transfer peer")
}

func TestGameRuntimeItemExchangeTransferClosesAcceptedShellBeforeSecondAccept(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExXferAccOwner", 0x010308a5, 0x020408a5, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 845, Vnum: 27045, Count: 1, Slot: 3}}
	peer := peerVisibilityCharacter("ExXferAccPeer", 0x010308a6, 0x020408a6, 1120, 2120, 0, 101, 201)
	peer.Gold = 6000
	peer.Inventory = []inventory.ItemInstance{{ID: 846, Vnum: 27002, Count: 1, Slot: 7}}
	ownerLogin := "ex-xfer-acc-owner"
	peerLogin := "ex-xfer-acc-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070a5, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070a6, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed accepted-shell transfer owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed accepted-shell transfer peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27045, Name: "Transfer Accept Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}})
	if err != nil {
		t.Fatalf("unexpected accepted-shell transfer runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070a5)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070a6)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted-shell transfer start error: %v", err)
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "accepted-shell owner start")
	assertExchangeStartFrame(t, flushServerFrames(t, peerFlow)[0], owner.VID, "accepted-shell peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderItemAdd,
		Arg2:      2,
		Position:  itemproto.InventoryPosition(3),
	})))
	if err != nil {
		t.Fatalf("unexpected accepted-shell item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected accepted-shell item-add to emit one frame, got %d", len(itemAddOut))
	}
	_ = flushServerFrames(t, peerFlow)

	acceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderAccept,
	})))
	if err != nil {
		t.Fatalf("unexpected first accept before transfer error: %v", err)
	}
	if len(acceptOut) != 1 {
		t.Fatalf("expected first accept before transfer to emit one frame, got %d", len(acceptOut))
	}
	assertExchangeAcceptFrame(t, acceptOut[0], 1, "first accept before transfer")
	_ = flushServerFrames(t, peerFlow)

	moveOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{
		Func: 1,
		Arg:  0,
		Rot:  12,
		X:    1500,
		Y:    2600,
		Time: 0x61626372,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted-shell peer transfer-trigger move error: %v", err)
	}
	if len(moveOut) < 2 {
		t.Fatalf("expected accepted-shell peer transfer to prepend END before transfer frames, got %d", len(moveOut))
	}
	assertExchangeEndFrame(t, moveOut[0], "accepted-shell peer self END")
	queuedOwner := flushServerFrames(t, ownerFlow)
	foundOwnerEnd := false
	for _, frame := range queuedOwner {
		if exchangeFrameIsEnd(t, frame) {
			foundOwnerEnd = true
			break
		}
	}
	if !foundOwnerEnd {
		t.Fatalf("expected accepted-shell peer transfer to queue owner END, got %d frames", len(queuedOwner))
	}

	secondAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderAccept,
	})))
	if err != nil {
		t.Fatalf("unexpected second accept after transfer teardown error: %v", err)
	}
	if len(secondAcceptOut) != 0 {
		t.Fatalf("expected second accept after transfer teardown to fail closed with no frames, got %d", len(secondAcceptOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected second accept after transfer teardown to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "accepted-shell transfer owner")
	peerAccount, err := accounts.Load(peerLogin)
	if err != nil {
		t.Fatalf("load persisted accepted-shell transfer peer account: %v", err)
	}
	if peerAccount.Characters[0].MapIndex != 42 || peerAccount.Characters[0].X != 1700 || peerAccount.Characters[0].Y != 2800 {
		t.Fatalf("expected peer transfer destination persistence, got %#v", peerAccount.Characters[0])
	}
	if peerAccount.Characters[0].Gold != peer.Gold || !reflect.DeepEqual(peerAccount.Characters[0].Inventory, peer.Inventory) {
		t.Fatalf("expected accepted-shell transfer teardown to leave peer inventory/gold unchanged, got %#v", peerAccount.Characters[0])
	}
}

func TestGameRuntimeItemExchangeItemAddShowsTemplateBackedDisplayWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeDisplayOwner", 0x0103076e, 0x0204076e, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 715, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeDisplayPeer", 0x0103076f, 0x0204076f, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 716, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-exchange-display-owner", 0x7070706e, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-display-peer", 0x7070706f, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-display-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange display owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-display-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange display peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:       27045,
		Name:       "Displayed Exchange Potion",
		Stackable:  true,
		MaxCount:   200,
		Sockets:    itemcatalog.SocketValues{11, 22, 33},
		Attributes: itemcatalog.AttributeValues{{Type: 3, Value: 30}, {Type: 4, Value: -5}},
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange display runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-display-owner", 0x7070706e)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-display-peer", 0x7070706f)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange-display start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange-display start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "display owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange-display peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "display peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderItemAdd,
		Arg2:      7,
		Position:  itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected exchange item-add display error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected exchange item-add to emit one self display frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], template, "owner item-add display")
	queuedDisplay := flushServerFrames(t, peerFlow)
	if len(queuedDisplay) != 1 {
		t.Fatalf("expected exchange peer to receive one queued item-add display frame, got %d", len(queuedDisplay))
	}
	assertExchangeItemAddFrame(t, queuedDisplay[0], 0, 7, owner.Inventory[0], template, "peer item-add display")

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-display-owner", owner, "owner exchange item-add display")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-display-peer", peer, "peer exchange item-add display")
}

func TestGameRuntimeItemExchangeItemAddRequiresActiveExchangeShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeNoShellOwner", 0x01030770, 0x02040770, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 717, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-exchange-no-shell-owner", 0x70707070, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-no-shell-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange no-shell owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27045, Name: "Displayed Exchange Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange no-shell runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-no-shell-owner", 0x70707070)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderItemAdd,
		Arg2:      7,
		Position:  itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected exchange item-add no-shell error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected exchange item-add without active shell to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after exchange item-add without active shell, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-no-shell-owner", owner, "exchange item-add without active shell")
}

func TestGameRuntimeItemExchangeItemAddRejectsDuplicateDisplaySlotWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeDuplicateOwner", 0x01030771, 0x02040771, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{
		{ID: 718, Vnum: 27045, Count: 3, Slot: 5},
		{ID: 719, Vnum: 27046, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeDuplicatePeer", 0x01030772, 0x02040772, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 720, Vnum: 27002, Count: 2, Slot: 8}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 8}}
	issuePeerTicket(t, ticketStore, "item-exchange-duplicate-owner", 0x70707071, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-duplicate-peer", 0x70707072, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-duplicate-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange duplicate owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-duplicate-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange duplicate peer account: %v", err)
	}
	templates := []itemcatalog.Template{
		{Vnum: 27045, Name: "Displayed Exchange Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27046, Name: "Second Displayed Exchange Potion", Stackable: true, MaxCount: 200},
	}
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange duplicate runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-duplicate-owner", 0x70707071)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-duplicate-peer", 0x70707072)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected duplicate-display exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected duplicate-display exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	firstOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected first duplicate-display item-add error: %v", err)
	}
	if len(firstOut) != 1 {
		t.Fatalf("expected first duplicate-display item-add to emit one frame, got %d", len(firstOut))
	}
	assertExchangeItemAddFrame(t, firstOut[0], 1, 7, owner.Inventory[0], templates[0], "first duplicate-display owner item-add")
	queuedFirst := flushServerFrames(t, peerFlow)
	if len(queuedFirst) != 1 {
		t.Fatalf("expected peer to receive first duplicate-display item-add frame, got %d", len(queuedFirst))
	}
	assertExchangeItemAddFrame(t, queuedFirst[0], 0, 7, owner.Inventory[0], templates[0], "first duplicate-display peer item-add")

	secondOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected second duplicate-display item-add error: %v", err)
	}
	if len(secondOut) != 0 {
		t.Fatalf("expected duplicate exchange display slot to emit no frames, got %d", len(secondOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected duplicate exchange display slot to queue no frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-duplicate-owner", owner, "owner duplicate exchange item-add display")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-duplicate-peer", peer, "peer duplicate exchange item-add display")
}

func TestGameRuntimeItemExchangeItemAddRejectsDuplicateSourceItemUntilDisplaySlotClearsWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeDuplicateSourceOwner", 0x0103077a, 0x0204077a, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 731, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeDuplicateSourcePeer", 0x0103077b, 0x0204077b, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 732, Vnum: 27002, Count: 2, Slot: 8}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 8}}
	ownerLogin := "ex-dupsrc-own"
	peerLogin := "ex-dupsrc-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x7070707a, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x7070707b, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange duplicate-source owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange duplicate-source peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27045, Name: "Displayed Exchange Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange duplicate-source runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x7070707a)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x7070707b)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected duplicate-source exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected duplicate-source exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	firstOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected first duplicate-source item-add error: %v", err)
	}
	if len(firstOut) != 1 {
		t.Fatalf("expected first duplicate-source item-add to emit one frame, got %d", len(firstOut))
	}
	assertExchangeItemAddFrame(t, firstOut[0], 1, 7, owner.Inventory[0], template, "first duplicate-source owner item-add")
	queuedFirst := flushServerFrames(t, peerFlow)
	if len(queuedFirst) != 1 {
		t.Fatalf("expected peer to receive first duplicate-source item-add frame, got %d", len(queuedFirst))
	}
	assertExchangeItemAddFrame(t, queuedFirst[0], 0, 7, owner.Inventory[0], template, "first duplicate-source peer item-add")

	duplicateOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 8, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected duplicate-source second item-add error: %v", err)
	}
	if len(duplicateOut) != 0 {
		t.Fatalf("expected duplicate-source exchange item-add to emit no frames, got %d", len(duplicateOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected duplicate-source exchange item-add to queue no frames, got %d", len(queued))
	}

	delOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemDel, Arg1: 7})))
	if err != nil {
		t.Fatalf("unexpected duplicate-source item-del error: %v", err)
	}
	if len(delOut) != 1 {
		t.Fatalf("expected duplicate-source item-del to emit one self frame, got %d", len(delOut))
	}
	assertExchangeItemDelFrame(t, delOut[0], 1, 7, "duplicate-source item-del self response")
	queuedDel := flushServerFrames(t, peerFlow)
	if len(queuedDel) != 1 {
		t.Fatalf("expected duplicate-source item-del to queue one peer frame, got %d", len(queuedDel))
	}
	assertExchangeItemDelFrame(t, queuedDel[0], 0, 7, "duplicate-source item-del peer response")

	readdOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 8, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected duplicate-source re-add item-add error: %v", err)
	}
	if len(readdOut) != 1 {
		t.Fatalf("expected source item to be displayable after item-del cleared the first slot, got %d frames", len(readdOut))
	}
	assertExchangeItemAddFrame(t, readdOut[0], 1, 8, owner.Inventory[0], template, "duplicate-source re-add owner item-add")
	queuedReadd := flushServerFrames(t, peerFlow)
	if len(queuedReadd) != 1 {
		t.Fatalf("expected duplicate-source re-add peer item-add frame, got %d", len(queuedReadd))
	}
	assertExchangeItemAddFrame(t, queuedReadd[0], 0, 8, owner.Inventory[0], template, "duplicate-source re-add peer item-add")

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "owner duplicate-source exchange item-add display")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer duplicate-source exchange item-add display")
}

func TestGameRuntimeItemExchangeItemDelClearsDisplaySlotWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeRemoveOwner", 0x01030773, 0x02040773, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{
		{ID: 721, Vnum: 27045, Count: 3, Slot: 5},
		{ID: 722, Vnum: 27046, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeRemovePeer", 0x01030774, 0x02040774, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 723, Vnum: 27002, Count: 2, Slot: 8}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 8}}
	issuePeerTicket(t, ticketStore, "item-exchange-remove-owner", 0x70707073, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-remove-peer", 0x70707074, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-remove-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange remove owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-remove-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange remove peer account: %v", err)
	}
	templates := []itemcatalog.Template{
		{Vnum: 27045, Name: "Displayed Exchange Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27046, Name: "Second Displayed Exchange Potion", Stackable: true, MaxCount: 200},
	}
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange item-del runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-remove-owner", 0x70707073)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-remove-peer", 0x70707074)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected item-del exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected item-del exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	firstOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected item-del first item-add error: %v", err)
	}
	if len(firstOut) != 1 {
		t.Fatalf("expected item-del first item-add to emit one frame, got %d", len(firstOut))
	}
	assertExchangeItemAddFrame(t, firstOut[0], 1, 7, owner.Inventory[0], templates[0], "item-del first owner add")
	queuedFirst := flushServerFrames(t, peerFlow)
	if len(queuedFirst) != 1 {
		t.Fatalf("expected item-del peer to receive first item-add frame, got %d", len(queuedFirst))
	}
	assertExchangeItemAddFrame(t, queuedFirst[0], 0, 7, owner.Inventory[0], templates[0], "item-del first peer add")

	delOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemDel, Arg1: 7})))
	if err != nil {
		t.Fatalf("unexpected exchange item-del error: %v", err)
	}
	if len(delOut) != 1 {
		t.Fatalf("expected exchange item-del to emit one self frame, got %d", len(delOut))
	}
	assertExchangeItemDelFrame(t, delOut[0], 1, 7, "item-del self response")
	queuedDel := flushServerFrames(t, peerFlow)
	if len(queuedDel) != 1 {
		t.Fatalf("expected exchange item-del to queue one peer frame, got %d", len(queuedDel))
	}
	assertExchangeItemDelFrame(t, queuedDel[0], 0, 7, "item-del peer response")

	secondOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected item-del second item-add error: %v", err)
	}
	if len(secondOut) != 1 {
		t.Fatalf("expected cleared exchange display slot to accept one new item-add frame, got %d", len(secondOut))
	}
	assertExchangeItemAddFrame(t, secondOut[0], 1, 7, owner.Inventory[1], templates[1], "item-del second owner add")
	queuedSecond := flushServerFrames(t, peerFlow)
	if len(queuedSecond) != 1 {
		t.Fatalf("expected cleared exchange display slot peer item-add frame, got %d", len(queuedSecond))
	}
	assertExchangeItemAddFrame(t, queuedSecond[0], 0, 7, owner.Inventory[1], templates[1], "item-del second peer add")

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-remove-owner", owner, "owner exchange item-del display")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-remove-peer", peer, "peer exchange item-del display")
}

func TestGameRuntimeItemExchangeGoldAddDisplaysWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeGoldOwner", 0x01030775, 0x02040775, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 724, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeGoldPeer", 0x01030776, 0x02040776, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 725, Vnum: 27002, Count: 2, Slot: 8}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 8}}
	issuePeerTicket(t, ticketStore, "item-exchange-gold-owner", 0x70707075, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-gold-peer", 0x70707076, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-gold-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange gold owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-gold-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange gold peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected exchange gold runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-gold-owner", 0x70707075)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-gold-peer", 0x70707076)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected gold exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected gold exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	goldOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 321})))
	if err != nil {
		t.Fatalf("unexpected exchange gold-add error: %v", err)
	}
	if len(goldOut) != 1 {
		t.Fatalf("expected exchange gold-add to emit one self frame, got %d", len(goldOut))
	}
	assertExchangeGoldAddFrame(t, goldOut[0], 1, 321, "gold-add self response")
	queuedGold := flushServerFrames(t, peerFlow)
	if len(queuedGold) != 1 {
		t.Fatalf("expected exchange gold-add to queue one peer frame, got %d", len(queuedGold))
	}
	assertExchangeGoldAddFrame(t, queuedGold[0], 0, 321, "gold-add peer response")

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-gold-owner", owner, "owner exchange gold display")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-gold-peer", peer, "peer exchange gold display")
}

func TestGameRuntimeItemExchangeGoldAddAboveLiveGoldReportsLessGoldWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeGoldPoor", 0x01030777, 0x02040777, 1100, 2100, 0, 101, 201)
	owner.Gold = 50
	owner.Inventory = []inventory.ItemInstance{{ID: 726, Vnum: 27045, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeGoldRichPeer", 0x01030778, 0x02040778, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	issuePeerTicket(t, ticketStore, "item-exchange-gold-poor", 0x70707077, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-gold-rich-peer", 0x70707078, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-gold-poor", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange gold poor account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-gold-rich-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange gold rich peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected exchange less-gold runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-gold-poor", 0x70707077)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-gold-rich-peer", 0x70707078)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected less-gold exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected less-gold exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	goldOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 51})))
	if err != nil {
		t.Fatalf("unexpected exchange less-gold packet error: %v", err)
	}
	if len(goldOut) != 1 {
		t.Fatalf("expected exchange less-gold to emit one self frame, got %d", len(goldOut))
	}
	assertExchangeLessGoldFrame(t, goldOut[0], "less-gold self response")
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected exchange less-gold to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-gold-poor", owner, "owner exchange less-gold")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-gold-rich-peer", peer, "peer exchange less-gold")
}

func TestExchangeDisplayedItemsStillLiveRejectsStaleDuplicateOrTemplateDriftedDisplayState(t *testing.T) {
	displayed := map[uint8]exchangeDisplayedItem{7: {ItemID: 729, Vnum: 27045, Count: 3, Slot: 5}}
	valid := loginticket.Character{Job: 0, RaceNum: 0, Empire: 1, Level: 10, Inventory: []inventory.ItemInstance{{ID: 729, Vnum: 27045, Count: 3, Slot: 5}}}
	templates := map[uint32]itemcatalog.Template{27045: {Vnum: 27045, Name: "Displayed Recheck Potion", Stackable: true, MaxCount: 200}}
	if !exchangeDisplayedItemsStillLive(displayed, valid, templates) {
		t.Fatal("expected exact live displayed item plus template to pass exchange accept revalidation")
	}

	countDrift := valid
	countDrift.Inventory = []inventory.ItemInstance{{ID: 729, Vnum: 27045, Count: 2, Slot: 5}}
	if exchangeDisplayedItemsStillLive(displayed, countDrift, templates) {
		t.Fatal("expected count-drifted displayed item to fail exchange accept revalidation")
	}

	duplicateID := valid
	duplicateID.Inventory = []inventory.ItemInstance{
		{ID: 729, Vnum: 27045, Count: 3, Slot: 5},
		{ID: 729, Vnum: 27045, Count: 3, Slot: 6},
	}
	if exchangeDisplayedItemsStillLive(displayed, duplicateID, templates) {
		t.Fatal("expected duplicate live item identity to fail exchange accept revalidation")
	}

	duplicateSlot := valid
	duplicateSlot.Inventory = []inventory.ItemInstance{
		{ID: 729, Vnum: 27045, Count: 3, Slot: 5},
		{ID: 730, Vnum: 27046, Count: 1, Slot: 5},
	}
	if exchangeDisplayedItemsStillLive(displayed, duplicateSlot, templates) {
		t.Fatal("expected duplicate carried slot occupancy to fail exchange accept revalidation")
	}

	locked := valid
	locked.Inventory = []inventory.ItemInstance{{ID: 729, Vnum: 27045, Count: 3, Slot: 5, Locked: true}}
	if exchangeDisplayedItemsStillLive(displayed, locked, templates) {
		t.Fatal("expected locked displayed item to fail exchange accept revalidation")
	}

	missingTemplate := map[uint32]itemcatalog.Template{}
	if exchangeDisplayedItemsStillLive(displayed, valid, missingTemplate) {
		t.Fatal("expected missing displayed-item template to fail exchange accept revalidation")
	}

	maxCountDrift := map[uint32]itemcatalog.Template{27045: {Vnum: 27045, Name: "Tiny Displayed Potion", Stackable: true, MaxCount: 2}}
	if exchangeDisplayedItemsStillLive(displayed, valid, maxCountDrift) {
		t.Fatal("expected displayed-item template max-count drift to fail exchange accept revalidation")
	}

	transferGuardDrift := map[uint32]itemcatalog.Template{27045: {Vnum: 27045, Name: "Now Bound Displayed Potion", Stackable: true, MaxCount: 200, AntiGive: true}}
	if exchangeDisplayedItemsStillLive(displayed, valid, transferGuardDrift) {
		t.Fatal("expected displayed-item transfer-guard template drift to fail exchange accept revalidation")
	}

	selectedCharacterGuardDrift := map[uint32]itemcatalog.Template{27045: {Vnum: 27045, Name: "Later Level Displayed Potion", Stackable: true, MaxCount: 200, MinLevel: 11}}
	if exchangeDisplayedItemsStillLive(displayed, valid, selectedCharacterGuardDrift) {
		t.Fatal("expected selected-character template drift to fail exchange accept revalidation")
	}
}

func TestExchangeRecipientCanAcceptRejectsIncomingItemIDCollisionWithRecipientEquipment(t *testing.T) {
	registry := newSharedWorldRegistry()
	registry.SetItemTemplates(map[uint32]itemcatalog.Template{
		27045: {Vnum: 27045, Name: "Displayed Equipment Collision Potion", Stackable: true, MaxCount: 200},
	})
	recipient := loginticket.Character{
		ID:      0x010307bb,
		VID:     0x020407bb,
		Name:    "ExchangeEquipmentCollisionRecipient",
		Job:     0,
		RaceNum: 0,
		Empire:  1,
		Level:   10,
		Points:  [255]int32{bootstrapPlayerPointValueIndex: 100},
		Equipment: []inventory.ItemInstance{
			{ID: 729, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
		},
	}
	incoming := map[uint8]exchangeDisplayedItem{7: {ItemID: 729, Vnum: 27045, Count: 3, Slot: 5}}
	if registry.exchangeRecipientCanAcceptLocked(recipient, incoming, 0) {
		t.Fatal("expected incoming exchange item id colliding with recipient equipment to fail finalization precondition")
	}

	incoming[7] = exchangeDisplayedItem{ItemID: 730, Vnum: 27045, Count: 3, Slot: 5}
	if !registry.exchangeRecipientCanAcceptLocked(recipient, incoming, 0) {
		t.Fatal("expected non-colliding incoming exchange item id to satisfy receiver preconditions")
	}
}

func TestExchangeRecipientCanAcceptRejectsOverTemplateMaxCompatibleStack(t *testing.T) {
	registry := newSharedWorldRegistry()
	registry.SetItemTemplates(map[uint32]itemcatalog.Template{
		27045: {Vnum: 27045, Name: "Displayed Over-Max Receiver Potion", Stackable: true, MaxCount: 200},
	})
	recipient := loginticket.Character{
		ID:      0x010307bc,
		VID:     0x020407bc,
		Name:    "ExchangeOverMaxReceiver",
		Job:     0,
		RaceNum: 0,
		Empire:  1,
		Level:   10,
		Points:  [255]int32{bootstrapPlayerPointValueIndex: 100},
		Inventory: []inventory.ItemInstance{
			{ID: 731, Vnum: 27045, Count: 201, Slot: 1},
		},
	}
	incoming := map[uint8]exchangeDisplayedItem{7: {ItemID: 732, Vnum: 27045, Count: 3, Slot: 5}}
	if registry.exchangeRecipientCanAcceptLocked(recipient, incoming, 0) {
		t.Fatal("expected receiver compatible stack already above template max_count to fail finalization precondition")
	}

	recipient.Inventory[0].Count = 200
	if !registry.exchangeRecipientCanAcceptLocked(recipient, incoming, 0) {
		t.Fatal("expected receiver compatible stack at template max_count to allow incoming placement into an empty slot")
	}
}

func TestGameRuntimeItemExchangeAcceptRevalidatesDisplayedItemAgainstCurrentSelectionWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeItemRecheckOwner", 0x0103077d, 0x0204077d, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 729, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeItemRecheckPeer", 0x0103077e, 0x0204077e, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 730, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-item-recheck-a"
	peerLogin := "ex-item-recheck-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x7070707d, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x7070707e, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange item recheck owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange item recheck peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27045, Name: "Displayed Recheck Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{5, 6, 7}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange item recheck runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x7070707d)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x7070707e)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected item recheck exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected item recheck exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "item recheck owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected item recheck exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "item recheck peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected item recheck item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected item recheck item-add to emit one self frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], template, "item recheck self item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected item recheck item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], template, "item recheck peer item-add")

	driftedOwner := owner
	driftedOwner.Inventory = []inventory.ItemInstance{{ID: 729, Vnum: 27045, Count: 2, Slot: 5}}
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{driftedOwner})}); err != nil {
		t.Fatalf("persist drifted exchange item recheck owner account: %v", err)
	}
	if !runtime.applyLiveCharacterPersistedSnapshot(owner.Name, driftedOwner) {
		t.Fatal("expected test drift to refresh the live selected character snapshot")
	}

	acceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected stale-item exchange accept error: %v", err)
	}
	if len(acceptOut) != 0 {
		t.Fatalf("expected stale displayed-item accept to emit no frames, got %d", len(acceptOut))
	}
	if queuedAccept := flushServerFrames(t, peerFlow); len(queuedAccept) != 0 {
		t.Fatalf("expected stale displayed-item accept to queue no peer frames, got %d", len(queuedAccept))
	}

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after stale-item accept error: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected exchange shell to remain cancellable after stale-item accept rejection, got %d frames", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "item recheck cancel after stale accept")
	queuedCancel := flushServerFrames(t, peerFlow)
	if len(queuedCancel) != 1 {
		t.Fatalf("expected cancel after stale-item accept to queue one peer END, got %d", len(queuedCancel))
	}
	assertExchangeEndFrame(t, queuedCancel[0], "item recheck peer cancel after stale accept")

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, driftedOwner, "stale displayed-item accept owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "stale displayed-item accept peer")
}

func TestGameRuntimeItemExchangeAcceptRevalidatesDisplayedTemplateMetadataWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		template itemcatalog.Template
	}{
		{name: "max count shrink", template: itemcatalog.Template{Vnum: 27045, Name: "Shrunk Recheck Potion", Stackable: true, MaxCount: 2}},
		{name: "transfer guard", template: itemcatalog.Template{Vnum: 27045, Name: "Bound Recheck Potion", Stackable: true, MaxCount: 200, AntiGive: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("ExchangeTemplateRecheckOwner", 0x010307ad, 0x020407ad, 1100, 2100, 0, 101, 201)
			owner.Gold = 500
			owner.Inventory = []inventory.ItemInstance{{ID: 777, Vnum: 27045, Count: 3, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			peer := peerVisibilityCharacter("ExchangeTemplateRecheckPeer", 0x010307ae, 0x020407ae, 1120, 2120, 0, 101, 201)
			peer.Gold = 22222
			peer.Inventory = []inventory.ItemInstance{{ID: 778, Vnum: 27002, Count: 2, Slot: 6}}
			peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
			ownerLogin := "ex-template-recheck-a"
			peerLogin := "ex-template-recheck-b"
			issuePeerTicket(t, ticketStore, ownerLogin, 0x707070ad, owner)
			issuePeerTicket(t, ticketStore, peerLogin, 0x707070ae, peer)
			if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed exchange template recheck owner account: %v", err)
			}
			if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
				t.Fatalf("seed exchange template recheck peer account: %v", err)
			}
			originalTemplate := itemcatalog.Template{Vnum: 27045, Name: "Displayed Template Recheck Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{5, 6, 7}}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{originalTemplate})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected exchange template recheck runtime error: %v", err)
			}
			ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070ad)
			defer closeSessionFlow(t, ownerFlow)
			peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070ae)
			defer closeSessionFlow(t, peerFlow)
			_ = flushServerFrames(t, ownerFlow)
			_ = flushServerFrames(t, peerFlow)

			startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
			if err != nil {
				t.Fatalf("unexpected template recheck exchange start error: %v", err)
			}
			if len(startOut) != 1 {
				t.Fatalf("expected template recheck exchange start to emit one owner frame, got %d", len(startOut))
			}
			assertExchangeStartFrame(t, startOut[0], peer.VID, "template recheck owner start")
			queuedStart := flushServerFrames(t, peerFlow)
			if len(queuedStart) != 1 {
				t.Fatalf("expected template recheck exchange start to queue one peer frame, got %d", len(queuedStart))
			}
			assertExchangeStartFrame(t, queuedStart[0], owner.VID, "template recheck peer start")

			itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
			if err != nil {
				t.Fatalf("unexpected template recheck item-add error: %v", err)
			}
			if len(itemAddOut) != 1 {
				t.Fatalf("expected template recheck item-add to emit one self frame, got %d", len(itemAddOut))
			}
			assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], originalTemplate, "template recheck self item-add")
			queuedItemAdd := flushServerFrames(t, peerFlow)
			if len(queuedItemAdd) != 1 {
				t.Fatalf("expected template recheck item-add to queue one peer frame, got %d", len(queuedItemAdd))
			}
			assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], originalTemplate, "template recheck peer item-add")

			runtime.itemTemplates[27045] = tc.template
			runtime.sharedWorld.SetItemTemplates(runtime.itemTemplates)

			acceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
			if err != nil {
				t.Fatalf("unexpected stale-template exchange accept error: %v", err)
			}
			if len(acceptOut) != 0 {
				t.Fatalf("expected stale-template accept to emit no frames, got %d", len(acceptOut))
			}
			if queuedAccept := flushServerFrames(t, peerFlow); len(queuedAccept) != 0 {
				t.Fatalf("expected stale-template accept to queue no peer frames, got %d", len(queuedAccept))
			}

			cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
			if err != nil {
				t.Fatalf("unexpected cancel after stale-template accept error: %v", err)
			}
			if len(cancelOut) != 1 {
				t.Fatalf("expected exchange shell to remain cancellable after stale-template accept rejection, got %d frames", len(cancelOut))
			}
			assertExchangeEndFrame(t, cancelOut[0], "template recheck cancel after stale accept")
			queuedCancel := flushServerFrames(t, peerFlow)
			if len(queuedCancel) != 1 {
				t.Fatalf("expected cancel after stale-template accept to queue one peer END, got %d", len(queuedCancel))
			}
			assertExchangeEndFrame(t, queuedCancel[0], "template recheck peer cancel after stale accept")

			assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "stale displayed-template accept owner")
			assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "stale displayed-template accept peer")
		})
	}
}

func TestGameRuntimeItemExchangeAcceptRevalidatesDisplayedGoldAgainstLiveGoldWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeGoldRecheckOwner", 0x01030779, 0x02040779, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 727, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeGoldRecheckPeer", 0x0103077c, 0x0204077c, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 728, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-gold-recheck-a"
	peerLogin := "ex-gold-recheck-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707079, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x7070707c, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange gold recheck owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange gold recheck peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected exchange gold recheck runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707079)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x7070707c)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected gold recheck exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected gold recheck exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	goldOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 400})))
	if err != nil {
		t.Fatalf("unexpected gold recheck gold-add error: %v", err)
	}
	if len(goldOut) != 1 {
		t.Fatalf("expected gold recheck gold-add to emit one self frame, got %d", len(goldOut))
	}
	assertExchangeGoldAddFrame(t, goldOut[0], 1, 400, "gold recheck self gold-add")
	queuedGold := flushServerFrames(t, peerFlow)
	if len(queuedGold) != 1 {
		t.Fatalf("expected gold recheck gold-add to queue one peer frame, got %d", len(queuedGold))
	}
	assertExchangeGoldAddFrame(t, queuedGold[0], 0, 400, "gold recheck peer gold-add")

	dropOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Elk: 200})))
	if err != nil {
		t.Fatalf("unexpected gold recheck currency drop error: %v", err)
	}
	if len(dropOut) != 3 {
		t.Fatalf("expected gold recheck currency drop to emit point, ground, ownership frames, got %d", len(dropOut))
	}
	point, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, dropOut[0]))
	if err != nil {
		t.Fatalf("decode gold recheck currency-drop point change: %v", err)
	}
	if point != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: -200, Value: 300}) {
		t.Fatalf("unexpected gold recheck currency-drop point change: %+v", point)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, dropOut[1]))
	if err != nil {
		t.Fatalf("decode gold recheck currency-drop ground add: %v", err)
	}
	queuedDrop := flushServerFrames(t, peerFlow)
	if len(queuedDrop) != 2 {
		t.Fatalf("expected gold recheck currency drop to queue visible ground frames, got %d", len(queuedDrop))
	}
	peerGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, queuedDrop[0]))
	if err != nil {
		t.Fatalf("decode gold recheck peer ground add: %v", err)
	}
	if peerGround != ground {
		t.Fatalf("unexpected gold recheck peer ground add: got %+v want %+v", peerGround, ground)
	}

	acceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected stale-gold exchange accept error: %v", err)
	}
	if len(acceptOut) != 1 {
		t.Fatalf("expected stale-gold exchange accept to emit one self status frame, got %d", len(acceptOut))
	}
	assertExchangeLessGoldFrame(t, acceptOut[0], "stale displayed-gold accept self response")
	if queuedAccept := flushServerFrames(t, peerFlow); len(queuedAccept) != 0 {
		t.Fatalf("expected stale-gold exchange accept to queue no peer accept frames, got %d", len(queuedAccept))
	}

	wantOwner := owner
	wantOwner.Gold = 300
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, wantOwner, "stale displayed-gold accept owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "stale displayed-gold accept peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsStaleAcceptedPartnerGoldWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangePartnerGoldOwner", 0x010307a1, 0x020407a1, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 747, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangePartnerGoldPeer", 0x010307a2, 0x020407a2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 748, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-partner-gold-a"
	peerLogin := "ex-partner-gold-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070a1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070a2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange partner-gold owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange partner-gold peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected exchange partner-gold runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070a1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070a2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected partner-gold exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-gold exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "partner-gold owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected partner-gold exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "partner-gold peer start")

	goldOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 400})))
	if err != nil {
		t.Fatalf("unexpected partner-gold gold-add error: %v", err)
	}
	if len(goldOut) != 1 {
		t.Fatalf("expected partner-gold gold-add to emit one self frame, got %d", len(goldOut))
	}
	assertExchangeGoldAddFrame(t, goldOut[0], 1, 400, "partner-gold self gold-add")
	queuedGold := flushServerFrames(t, peerFlow)
	if len(queuedGold) != 1 {
		t.Fatalf("expected partner-gold gold-add to queue one peer frame, got %d", len(queuedGold))
	}
	assertExchangeGoldAddFrame(t, queuedGold[0], 0, 400, "partner-gold peer gold-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected partner-gold owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected partner-gold owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "partner-gold owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected partner-gold owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "partner-gold owner accept peer")

	dropOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Elk: 200})))
	if err != nil {
		t.Fatalf("unexpected partner-gold currency drop error: %v", err)
	}
	if len(dropOut) != 3 {
		t.Fatalf("expected partner-gold currency drop to emit point, ground, ownership frames, got %d", len(dropOut))
	}
	point, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, dropOut[0]))
	if err != nil {
		t.Fatalf("decode partner-gold currency-drop point change: %v", err)
	}
	if point != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: -200, Value: 300}) {
		t.Fatalf("unexpected partner-gold currency-drop point change: %+v", point)
	}
	queuedDrop := flushServerFrames(t, peerFlow)
	if len(queuedDrop) != 2 {
		t.Fatalf("expected partner-gold currency drop to queue visible ground frames, got %d", len(queuedDrop))
	}

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected partner-gold peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept stale partner-gold reject to emit one CheckOther info chat, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode partner-gold peer CheckOther info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeFinalizeCheckOtherInfoMessage {
		t.Fatalf("unexpected partner-gold peer CheckOther info chat: %+v", infoChat)
	}
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 2 {
		t.Fatalf("expected stale accepted partner-gold rejection to queue one owner CheckSelf info chat, got %d", len(queuedAccept))
	}
	queuedInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedAccept[0]))
	if err != nil {
		t.Fatalf("decode partner-gold owner CheckSelf info chat: %v", err)
	}
	if queuedInfo.Type != chatproto.ChatTypeInfo || queuedInfo.VID != 0 || queuedInfo.Message != exchangeFinalizeCheckSelfInfoMessage {
		t.Fatalf("unexpected partner-gold owner CheckSelf info chat: %+v", queuedInfo)
	}

	assertExchangeEndFrame(t, peerAcceptOut[1], "second-accept finalize reject auto-cancel self END")
	assertExchangeEndFrame(t, queuedAccept[1], "second-accept finalize reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled finalize reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected finalize-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after finalize-reject auto-cancel, got %d", len(queuedCancel))
	}

	wantOwner := owner
	wantOwner.Gold = 300
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, wantOwner, "stale accepted partner-gold owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "stale accepted partner-gold peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsStaleAcceptedPartnerItemWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangePartnerItemOwner", 0x010307a3, 0x020407a3, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 749, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangePartnerItemPeer", 0x010307a4, 0x020407a4, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 750, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-partner-item-a"
	peerLogin := "ex-partner-item-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070a3, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070a4, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange partner-item owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange partner-item peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27045, Name: "Displayed Partner Recheck Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{9, 8, 7}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange partner-item runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070a3)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070a4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected partner-item exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-item exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "partner-item owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected partner-item exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "partner-item peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected partner-item item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected partner-item item-add to emit one self frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], template, "partner-item self item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected partner-item item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], template, "partner-item peer item-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected partner-item owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected partner-item owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "partner-item owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected partner-item owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "partner-item owner accept peer")

	driftedOwner := owner
	driftedOwner.Inventory = []inventory.ItemInstance{{ID: 749, Vnum: 27045, Count: 2, Slot: 5}}
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{driftedOwner})}); err != nil {
		t.Fatalf("persist drifted exchange partner-item owner account: %v", err)
	}
	if !runtime.applyLiveCharacterPersistedSnapshot(owner.Name, driftedOwner) {
		t.Fatal("expected test drift to refresh the partner-item live selected character snapshot")
	}
	ownerEntity, ok := runtime.sharedWorld.playerEntityByName(owner.Name)
	if !ok {
		t.Fatal("expected test drift to find partner-item owner shared-world entity")
	}
	runtime.sharedWorld.UpdateCharacterWithVisibilityTransition(ownerEntity.Entity.ID, owner, driftedOwner, nil)
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected controlled partner-item drift to queue no owner frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected controlled partner-item drift to queue no peer frames, got %d", len(queued))
	}

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected partner-item peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept stale partner-item reject to emit one CheckOther info chat, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode partner-item peer CheckOther info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeFinalizeCheckOtherInfoMessage {
		t.Fatalf("unexpected partner-item peer CheckOther info chat: %+v", infoChat)
	}
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 2 {
		t.Fatalf("expected stale accepted partner-item rejection to queue one owner CheckSelf info chat, got %d", len(queuedAccept))
	}
	queuedInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedAccept[0]))
	if err != nil {
		t.Fatalf("decode partner-item owner CheckSelf info chat: %v", err)
	}
	if queuedInfo.Type != chatproto.ChatTypeInfo || queuedInfo.VID != 0 || queuedInfo.Message != exchangeFinalizeCheckSelfInfoMessage {
		t.Fatalf("unexpected partner-item owner CheckSelf info chat: %+v", queuedInfo)
	}

	assertExchangeEndFrame(t, peerAcceptOut[1], "second-accept finalize reject auto-cancel self END")
	assertExchangeEndFrame(t, queuedAccept[1], "second-accept finalize reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled finalize reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected finalize-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after finalize-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, driftedOwner, "stale accepted partner-item owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "stale accepted partner-item peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsReceiverInventoryCapacityBeforeFinalizationWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeCapacityOwner", 0x010307b1, 0x020407b1, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 781, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeCapacityPeer", 0x010307b2, 0x020407b2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = merchantBuyerFullInventory()
	peer.Quickslots = []loginticket.Quickslot{}
	ownerLogin := "ex-capacity-a"
	peerLogin := "ex-capacity-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070b1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070b2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange capacity owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange capacity peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27045, Name: "Displayed Capacity Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{2, 4, 6}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange capacity runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070b1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070b2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected capacity exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected capacity exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "capacity owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected capacity exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "capacity peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected capacity exchange item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected capacity item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], template, "capacity owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected capacity item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], template, "capacity peer item-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected capacity owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected capacity owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "capacity owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected capacity owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "capacity owner accept peer")

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected capacity peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept inventory-capacity reject to emit self Space info chat then END, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode capacity peer Space info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeFinalizeSpaceSelfInfoMessage {
		t.Fatalf("unexpected capacity peer Space info chat: %+v", infoChat)
	}
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 2 {
		t.Fatalf("expected receiver inventory-capacity reject to queue owner SpaceOther info chat then END, got %d", len(queuedAccept))
	}
	queuedInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedAccept[0]))
	if err != nil {
		t.Fatalf("decode capacity owner SpaceOther info chat: %v", err)
	}
	if queuedInfo.Type != chatproto.ChatTypeInfo || queuedInfo.VID != 0 || queuedInfo.Message != exchangeFinalizeSpaceOtherInfoMessage {
		t.Fatalf("unexpected capacity owner SpaceOther info chat: %+v", queuedInfo)
	}

	assertExchangeEndFrame(t, peerAcceptOut[1], "second-accept finalize reject auto-cancel self END")
	assertExchangeEndFrame(t, queuedAccept[1], "second-accept finalize reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled finalize reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected finalize-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after finalize-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "receiver inventory-capacity owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "receiver inventory-capacity peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsReceiverGoldOverflowBeforeFinalizationWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeGoldOverflowOwner", 0x010307b3, 0x020407b3, 1100, 2100, 0, 101, 201)
	owner.Gold = 100
	owner.Inventory = []inventory.ItemInstance{{ID: 782, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeGoldOverflowPeer", 0x010307b4, 0x020407b4, 1120, 2120, 0, 101, 201)
	peer.Gold = exchangeGoldPointChangeCarrierMax - 5
	peer.Inventory = []inventory.ItemInstance{{ID: 783, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-gold-overflow-a"
	peerLogin := "ex-gold-overflow-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070b3, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070b4, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange gold-overflow owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange gold-overflow peer account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected exchange gold-overflow runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070b3)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070b4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected gold-overflow exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected gold-overflow exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "gold-overflow owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected gold-overflow exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "gold-overflow peer start")

	goldOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 10})))
	if err != nil {
		t.Fatalf("unexpected gold-overflow gold-add error: %v", err)
	}
	if len(goldOut) != 1 {
		t.Fatalf("expected gold-overflow gold-add to emit one owner frame, got %d", len(goldOut))
	}
	assertExchangeGoldAddFrame(t, goldOut[0], 1, 10, "gold-overflow owner gold-add")
	queuedGold := flushServerFrames(t, peerFlow)
	if len(queuedGold) != 1 {
		t.Fatalf("expected gold-overflow gold-add to queue one peer frame, got %d", len(queuedGold))
	}
	assertExchangeGoldAddFrame(t, queuedGold[0], 0, 10, "gold-overflow peer gold-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected gold-overflow owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected gold-overflow owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "gold-overflow owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected gold-overflow owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "gold-overflow owner accept peer")

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected gold-overflow peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept gold-overflow reject to emit self gold-overflow info chat then END, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode gold-overflow peer self info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeFinalizeGoldOverflowSelfInfoMessage {
		t.Fatalf("unexpected gold-overflow peer self info chat: %+v", infoChat)
	}
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 2 {
		t.Fatalf("expected receiver gold-overflow reject to queue owner gold-overflow Other info chat then END, got %d", len(queuedAccept))
	}
	queuedInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedAccept[0]))
	if err != nil {
		t.Fatalf("decode gold-overflow owner Other info chat: %v", err)
	}
	if queuedInfo.Type != chatproto.ChatTypeInfo || queuedInfo.VID != 0 || queuedInfo.Message != exchangeFinalizeGoldOverflowOtherInfoMessage {
		t.Fatalf("unexpected gold-overflow owner Other info chat: %+v", queuedInfo)
	}

	assertExchangeEndFrame(t, peerAcceptOut[1], "second-accept finalize reject auto-cancel self END")
	assertExchangeEndFrame(t, queuedAccept[1], "second-accept finalize reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled finalize reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected finalize-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after finalize-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "receiver gold-overflow owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "receiver gold-overflow peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsReceiverEquipmentIDCollisionBeforeFinalizationWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeEquipmentCollisionOwner", 0x010307b5, 0x020407b5, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 784, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeEquipmentCollisionPeer", 0x010307b6, 0x020407b6, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Equipment = []inventory.ItemInstance{{ID: 784, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-equipment-collision-a"
	peerLogin := "ex-equipment-collision-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070b5, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070b6, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange equipment-collision owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange equipment-collision peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27045, Name: "Displayed Equipment Collision Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{3, 6, 9}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		displayTemplate,
		{Vnum: 11200, Name: "Equipment Collision Sword", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String()},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange equipment-collision runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070b5)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070b6)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected equipment-collision exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected equipment-collision exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "equipment-collision owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected equipment-collision exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "equipment-collision peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected equipment-collision item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected equipment-collision item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "equipment-collision owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected equipment-collision item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "equipment-collision peer item-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected equipment-collision owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected equipment-collision owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "equipment-collision owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected equipment-collision owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "equipment-collision owner accept peer")

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected equipment-collision peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept equipment-id collision Other reject to emit self Unknown error info chat then END, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode equipment-collision peer self info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeFinalizeOtherInfoMessage {
		t.Fatalf("unexpected equipment-collision peer self info chat: %+v", infoChat)
	}
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 2 {
		t.Fatalf("expected receiver equipment-id collision Other reject to queue owner Unknown error info chat then END, got %d", len(queuedAccept))
	}
	queuedInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedAccept[0]))
	if err != nil {
		t.Fatalf("decode equipment-collision owner Other info chat: %v", err)
	}
	if queuedInfo.Type != chatproto.ChatTypeInfo || queuedInfo.VID != 0 || queuedInfo.Message != exchangeFinalizeOtherInfoMessage {
		t.Fatalf("unexpected equipment-collision owner Other info chat: %+v", queuedInfo)
	}

	assertExchangeEndFrame(t, peerAcceptOut[1], "second-accept finalize reject auto-cancel self END")
	assertExchangeEndFrame(t, queuedAccept[1], "second-accept finalize reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled finalize reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected finalize-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after finalize-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "receiver equipment-id collision owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "receiver equipment-id collision peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsReceiverInventoryIDCollisionBeforeFinalizationWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeInventoryCollisionOwner", 0x010307b7, 0x020407b7, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 785, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeInventoryCollisionPeer", 0x010307b8, 0x020407b8, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 785, Vnum: 27046, Count: 1, Slot: 0}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-inv-collision-a"
	peerLogin := "ex-inv-collision-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070b7, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070b8, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange inventory-collision owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange inventory-collision peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27045, Name: "Displayed Inventory Collision Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{3, 6, 9}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{
		displayTemplate,
		{Vnum: 27046, Name: "Peer Inventory Collision Potion", Stackable: true, MaxCount: 200},
	})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange inventory-collision runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070b7)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070b8)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected inventory-collision exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected inventory-collision exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "inventory-collision owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected inventory-collision exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "inventory-collision peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected inventory-collision item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected inventory-collision item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "inventory-collision owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected inventory-collision item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "inventory-collision peer item-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected inventory-collision owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected inventory-collision owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "inventory-collision owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected inventory-collision owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "inventory-collision owner accept peer")

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected inventory-collision peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept inventory-id collision Other reject to emit self Unknown error info chat then END, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode inventory-collision peer self info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeFinalizeOtherInfoMessage {
		t.Fatalf("unexpected inventory-collision peer self info chat: %+v", infoChat)
	}
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 2 {
		t.Fatalf("expected receiver inventory-id collision Other reject to queue owner Unknown error info chat then END, got %d", len(queuedAccept))
	}
	queuedInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedAccept[0]))
	if err != nil {
		t.Fatalf("decode inventory-collision owner Other info chat: %v", err)
	}
	if queuedInfo.Type != chatproto.ChatTypeInfo || queuedInfo.VID != 0 || queuedInfo.Message != exchangeFinalizeOtherInfoMessage {
		t.Fatalf("unexpected inventory-collision owner Other info chat: %+v", queuedInfo)
	}

	assertExchangeEndFrame(t, peerAcceptOut[1], "second-accept finalize reject auto-cancel self END")
	assertExchangeEndFrame(t, queuedAccept[1], "second-accept finalize reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled finalize reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected finalize-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after finalize-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "receiver inventory-id collision owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "receiver inventory-id collision peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsReceiverLockedCompatibleStacksWithoutFreeSlotBeforeFinalizationWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeLockedStackOwner", 0x010307bf, 0x020407bf, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 787, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeLockedStackPeer", 0x010307c0, 0x020407c0, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = merchantBuyerFullInventory()
	// Fill every carried cell, then replace slot 0 with a locked compatible stack that would
	// otherwise absorb the incoming count if locked merges were allowed.
	peer.Inventory[0] = inventory.ItemInstance{ID: 788, Vnum: 27045, Count: 10, Slot: 0, Locked: true}
	peer.Quickslots = []loginticket.Quickslot{}
	ownerLogin := "ex-locked-stack-a"
	peerLogin := "ex-locked-stack-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070bf, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070c0, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange locked-stack owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange locked-stack peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27045, Name: "Displayed Locked-Stack Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{1, 3, 5}}
	fillerTemplates := make([]itemcatalog.Template, 0, int(inventory.CarriedInventorySlotCount)+1)
	fillerTemplates = append(fillerTemplates, displayTemplate)
	for slot := inventory.SlotIndex(1); slot < inventory.CarriedInventorySlotCount; slot++ {
		fillerTemplates = append(fillerTemplates, itemcatalog.Template{Vnum: 40000 + uint32(slot), Name: "Locked Capacity Filler", MaxCount: 1})
	}
	itemStore := newItemTemplateStore(t, fillerTemplates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange locked-stack runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070bf)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070c0)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected locked-stack exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected locked-stack exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "locked-stack owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected locked-stack exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "locked-stack peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected locked-stack item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected locked-stack item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "locked-stack owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected locked-stack item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "locked-stack peer item-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected locked-stack owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected locked-stack owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "locked-stack owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected locked-stack owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "locked-stack owner accept peer")

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected locked-stack peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept locked-compatible-stack Other reject to emit self Unknown error info chat then END, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode locked-stack peer self info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeFinalizeOtherInfoMessage {
		t.Fatalf("unexpected locked-stack peer self info chat: %+v", infoChat)
	}
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 2 {
		t.Fatalf("expected receiver locked-compatible-stack Other reject to queue owner Unknown error info chat then END, got %d", len(queuedAccept))
	}
	queuedInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedAccept[0]))
	if err != nil {
		t.Fatalf("decode locked-stack owner Other info chat: %v", err)
	}
	if queuedInfo.Type != chatproto.ChatTypeInfo || queuedInfo.VID != 0 || queuedInfo.Message != exchangeFinalizeOtherInfoMessage {
		t.Fatalf("unexpected locked-stack owner Other info chat: %+v", queuedInfo)
	}

	assertExchangeEndFrame(t, peerAcceptOut[1], "second-accept finalize reject auto-cancel self END")
	assertExchangeEndFrame(t, queuedAccept[1], "second-accept finalize reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled finalize reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected finalize-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after finalize-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "receiver locked-compatible-stack owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "receiver locked-compatible-stack peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsReceiverOverTemplateMaxCompatibleStackBeforeFinalizationWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeOverMaxStackOwner", 0x010307bd, 0x020407bd, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 785, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeOverMaxStackPeer", 0x010307be, 0x020407be, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 786, Vnum: 27045, Count: 201, Slot: 8}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 8}}
	ownerLogin := "ex-overmax-stack-a"
	peerLogin := "ex-overmax-stack-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070bd, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070be, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange over-max-stack owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange over-max-stack peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27045, Name: "Displayed Over-Max Stack Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{1, 3, 5}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{displayTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange over-max-stack runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070bd)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070be)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected over-max-stack exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected over-max-stack exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "over-max-stack owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected over-max-stack exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "over-max-stack peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected over-max-stack item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected over-max-stack item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "over-max-stack owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected over-max-stack item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "over-max-stack peer item-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected over-max-stack owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected over-max-stack owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "over-max-stack owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected over-max-stack owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "over-max-stack owner accept peer")

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected over-max-stack peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept over-template-max Other reject to emit self Unknown error info chat then END, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode over-max-stack peer self info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeFinalizeOtherInfoMessage {
		t.Fatalf("unexpected over-max-stack peer self info chat: %+v", infoChat)
	}
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 2 {
		t.Fatalf("expected receiver over-template-max Other reject to queue owner Unknown error info chat then END, got %d", len(queuedAccept))
	}
	queuedInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedAccept[0]))
	if err != nil {
		t.Fatalf("decode over-max-stack owner Other info chat: %v", err)
	}
	if queuedInfo.Type != chatproto.ChatTypeInfo || queuedInfo.VID != 0 || queuedInfo.Message != exchangeFinalizeOtherInfoMessage {
		t.Fatalf("unexpected over-max-stack owner Other info chat: %+v", queuedInfo)
	}

	assertExchangeEndFrame(t, peerAcceptOut[1], "second-accept finalize reject auto-cancel self END")
	assertExchangeEndFrame(t, queuedAccept[1], "second-accept finalize reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled finalize reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected finalize-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after finalize-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "receiver over-template-max owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "receiver over-template-max peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsReceiverSelectedCharacterRestrictionBeforeFinalizationWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeReceiverRestrictOwner", 0x010307bf, 0x020407bf, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Level = 20
	owner.Inventory = []inventory.ItemInstance{{ID: 787, Vnum: 27046, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeReceiverRestrictPeer", 0x010307cf, 0x020407cf, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Level = 5
	ownerLogin := "ex-receiver-restrict-a"
	peerLogin := "ex-receiver-restrict-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070bf, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070cf, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange receiver-restriction owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange receiver-restriction peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27046, Name: "High-Level Exchange Potion", Stackable: true, MaxCount: 200, MinLevel: 10, Sockets: itemcatalog.SocketValues{1, 3, 5}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{displayTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange receiver-restriction runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070bf)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070cf)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected receiver-restriction exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected receiver-restriction exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "receiver-restriction owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected receiver-restriction exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "receiver-restriction peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected receiver-restriction item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected receiver-restriction item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "receiver-restriction owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected receiver-restriction item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "receiver-restriction peer item-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected receiver-restriction owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected receiver-restriction owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "receiver-restriction owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected receiver-restriction owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "receiver-restriction owner accept peer")

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected receiver-restriction peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept selected-character restriction Other reject to emit self Unknown error info chat then END, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode receiver-restriction peer self info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeFinalizeOtherInfoMessage {
		t.Fatalf("unexpected receiver-restriction peer self info chat: %+v", infoChat)
	}
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 2 {
		t.Fatalf("expected receiver selected-character restriction Other reject to queue owner Unknown error info chat then END, got %d", len(queuedAccept))
	}
	queuedInfo, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedAccept[0]))
	if err != nil {
		t.Fatalf("decode receiver-restriction owner Other info chat: %v", err)
	}
	if queuedInfo.Type != chatproto.ChatTypeInfo || queuedInfo.VID != 0 || queuedInfo.Message != exchangeFinalizeOtherInfoMessage {
		t.Fatalf("unexpected receiver-restriction owner Other info chat: %+v", queuedInfo)
	}

	assertExchangeEndFrame(t, peerAcceptOut[1], "second-accept finalize reject auto-cancel self END")
	assertExchangeEndFrame(t, queuedAccept[1], "second-accept finalize reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled finalize reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected finalize-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after finalize-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "receiver selected-character restriction owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "receiver selected-character restriction peer")
}

func TestGameRuntimeItemExchangeAcceptRejectsRequesterOpenSafeboxWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeAcceptSafeboxOwner", 0x010307d1, 0x020407d1, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 801, Vnum: 27047, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeAcceptSafeboxPeer", 0x010307d2, 0x020407d2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "ex-accept-safebox-a"
	peerLogin := "ex-accept-safebox-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070d2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange accept-safebox owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange accept-safebox peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27047, Name: "Accept Safebox Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{displayTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange accept-safebox runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070d2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected accept-safebox exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected accept-safebox exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "accept-safebox owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected accept-safebox exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "accept-safebox peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected accept-safebox item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected accept-safebox item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "accept-safebox owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected accept-safebox item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "accept-safebox peer item-add")

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected accept-safebox /open_safebox error: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected accept-safebox /open_safebox to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}
	if _, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0])); err != nil {
		t.Fatalf("decode accept-safebox /open_safebox SAFEBOX_SIZE: %v", err)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected accept-safebox /open_safebox to queue no peer frames, got %d", len(queued))
	}

	acceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected accept-safebox owner accept error: %v", err)
	}
	if len(acceptOut) != 2 {
		t.Fatalf("expected requester open-safebox accept to emit busy info chat then END, got %d", len(acceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, acceptOut[0]))
	if err != nil {
		t.Fatalf("decode requester open-safebox accept busy info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected requester open-safebox accept busy info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, acceptOut[1], "accept-safebox busy reject auto-cancel self END")
	queuedAccept := flushServerFrames(t, peerFlow)
	if len(queuedAccept) != 1 {
		t.Fatalf("expected requester open-safebox accept to queue one peer END, got %d", len(queuedAccept))
	}
	assertExchangeEndFrame(t, queuedAccept[0], "accept-safebox busy reject auto-cancel peer END")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled busy reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, peerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after busy-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "requester open-safebox accept owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "requester open-safebox accept peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsPartnerOpenSafeboxWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangePartnerSafeboxAcceptOwner", 0x010307d3, 0x020407d3, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 802, Vnum: 27048, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangePartnerSafeboxAcceptPeer", 0x010307d4, 0x020407d4, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "ex-partner-sfbx-acc-a"
	peerLogin := "ex-partner-sfbx-acc-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d3, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070d4, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partner-safebox accept owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partner-safebox accept peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27048, Name: "Partner Safebox Accept Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{displayTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected partner-safebox accept runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d3)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070d4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected partner-safebox accept exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-safebox accept exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "partner-safebox accept owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected partner-safebox accept exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "partner-safebox accept peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected partner-safebox accept item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected partner-safebox accept item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "partner-safebox accept owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected partner-safebox accept item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "partner-safebox accept peer item-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected partner-safebox accept owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected partner-safebox accept owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "partner-safebox accept owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected partner-safebox accept owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "partner-safebox accept owner accept peer")

	openOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected partner /open_safebox during exchange error: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected partner /open_safebox during exchange to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}
	if _, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0])); err != nil {
		t.Fatalf("decode partner /open_safebox SAFEBOX_SIZE: %v", err)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected partner /open_safebox during exchange to queue no owner frames, got %d", len(queued))
	}

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected partner-safebox second accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept to reject partner open-safebox with busy info chat then END, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode partner open-safebox second accept busy info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner open-safebox second accept busy info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, peerAcceptOut[1], "partner-safebox busy reject auto-cancel self END")
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 1 {
		t.Fatalf("expected partner open-safebox second accept to queue one owner END, got %d", len(queuedAccept))
	}
	assertExchangeEndFrame(t, queuedAccept[0], "partner-safebox busy reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled busy reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after busy-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "partner open-safebox second accept owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "partner open-safebox second accept peer")
}

func TestGameRuntimeItemExchangeSecondAcceptRejectsPartnerOpenMerchantWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangePartnerMerchAcceptOwner", 0x010307d5, 0x020407d5, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 803, Vnum: 27049, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangePartnerMerchAcceptPeer", 0x010307d6, 0x020407d6, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "ex-partner-merch-acc-a"
	peerLogin := "ex-partner-merch-acc-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d5, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070d6, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partner-merchant accept owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partner-merchant accept peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27049, Name: "Partner Merchant Accept Potion", Stackable: true, MaxCount: 200}
	merchantTemplate := itemcatalog.Template{Vnum: 27051, Name: "Partner Merchant Guard Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{displayTemplate, merchantTemplate})
	merchantDefinition := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:exchange_accept_merchant_guard",
		Title: "Exchange Accept Merchant Guard",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27051, Price: 50, Count: 1},
		},
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{merchantDefinition})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected partner-merchant accept runtime error: %v", err)
	}
	merchant, ok := runtime.RegisterStaticActorWithInteraction("ExchangeAcceptMerchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, merchantDefinition.Ref)
	if !ok {
		t.Fatal("expected partner-merchant accept static actor registration to succeed")
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d5)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070d6)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected partner-merchant accept exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-merchant accept exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "partner-merchant accept owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected partner-merchant accept exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "partner-merchant accept peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected partner-merchant accept item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected partner-merchant accept item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "partner-merchant accept owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected partner-merchant accept item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "partner-merchant accept peer item-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected partner-merchant accept owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected partner-merchant accept owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "partner-merchant accept owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected partner-merchant accept owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "partner-merchant accept owner accept peer")

	merchantOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(merchant.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected partner merchant-open during exchange error: %v", err)
	}
	if len(merchantOut) != 1 {
		t.Fatalf("expected partner merchant-open during exchange to emit one shop start frame, got %d", len(merchantOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, merchantOut[0])); err != nil {
		t.Fatalf("decode partner merchant-open shop start: %v", err)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected partner merchant-open during exchange to queue no owner frames, got %d", len(queued))
	}

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected partner-merchant second accept error: %v", err)
	}
	if len(peerAcceptOut) != 2 {
		t.Fatalf("expected second accept to reject partner open-merchant with busy info chat then END, got %d", len(peerAcceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[0]))
	if err != nil {
		t.Fatalf("decode partner open-merchant second accept busy info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner open-merchant second accept busy info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, peerAcceptOut[1], "partner-merchant busy reject auto-cancel self END")
	queuedAccept := flushServerFrames(t, ownerFlow)
	if len(queuedAccept) != 1 {
		t.Fatalf("expected partner open-merchant second accept to queue one owner END, got %d", len(queuedAccept))
	}
	assertExchangeEndFrame(t, queuedAccept[0], "partner-merchant busy reject auto-cancel peer END")

	cancelOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled busy reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, ownerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after busy-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "partner open-merchant second accept owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "partner open-merchant second accept peer")
}

func TestGameRuntimeItemExchangeAcceptRejectsRequesterOpenMerchantWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchAcceptMerchOwner", 0x010307d7, 0x020407d7, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 804, Vnum: 27052, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchAcceptMerchPeer", 0x010307d8, 0x020407d8, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "ex-accept-merch-a"
	peerLogin := "ex-accept-merch-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d7, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070d8, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange accept-merchant owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange accept-merchant peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27052, Name: "Accept Merchant Potion", Stackable: true, MaxCount: 200}
	merchantTemplate := itemcatalog.Template{Vnum: 27053, Name: "Accept Merchant Guard Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{displayTemplate, merchantTemplate})
	merchantDefinition := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:exchange_accept_requester_merchant_guard",
		Title: "Exchange Accept Requester Merchant Guard",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27053, Price: 50, Count: 1},
		},
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{merchantDefinition})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange accept-merchant runtime error: %v", err)
	}
	merchant, ok := runtime.RegisterStaticActorWithInteraction("ExchangeAcceptRequesterMerchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, merchantDefinition.Ref)
	if !ok {
		t.Fatal("expected exchange accept-merchant static actor registration to succeed")
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d7)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070d8)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected accept-merchant exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected accept-merchant exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "accept-merchant owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected accept-merchant exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "accept-merchant peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected accept-merchant item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected accept-merchant item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "accept-merchant owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected accept-merchant item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "accept-merchant peer item-add")

	merchantOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(merchant.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected accept-merchant requester merchant-open error: %v", err)
	}
	if len(merchantOut) != 1 {
		t.Fatalf("expected accept-merchant requester merchant-open to emit one shop start frame, got %d", len(merchantOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, merchantOut[0])); err != nil {
		t.Fatalf("decode accept-merchant requester shop start: %v", err)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected accept-merchant requester merchant-open to queue no peer frames, got %d", len(queued))
	}

	acceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected accept-merchant owner accept error: %v", err)
	}
	if len(acceptOut) != 2 {
		t.Fatalf("expected requester open-merchant accept to emit busy info chat then END, got %d", len(acceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, acceptOut[0]))
	if err != nil {
		t.Fatalf("decode requester open-merchant accept busy info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected requester open-merchant accept busy info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, acceptOut[1], "accept-merchant busy reject auto-cancel self END")
	queuedAccept := flushServerFrames(t, peerFlow)
	if len(queuedAccept) != 1 {
		t.Fatalf("expected requester open-merchant accept to queue one peer END, got %d", len(queuedAccept))
	}
	assertExchangeEndFrame(t, queuedAccept[0], "accept-merchant busy reject auto-cancel peer END")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled busy reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, peerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after busy-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "requester open-merchant accept owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "requester open-merchant accept peer")
}

func TestGameRuntimeItemExchangeAcceptRejectsPartnerOpenMerchantWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchPMerch1stOwner", 0x010307d9, 0x020407d9, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 805, Vnum: 27054, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchPMerch1stPeer", 0x010307da, 0x020407da, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "ex-p-merch-1st-a"
	peerLogin := "ex-p-merch-1st-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d9, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070da, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partner-merchant first-accept owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partner-merchant first-accept peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27054, Name: "Partner Merchant First Accept Potion", Stackable: true, MaxCount: 200}
	merchantTemplate := itemcatalog.Template{Vnum: 27055, Name: "Partner Merchant First Accept Guard", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{displayTemplate, merchantTemplate})
	merchantDefinition := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:exchange_accept_partner_merchant_first",
		Title: "Exchange Accept Partner Merchant First",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27055, Price: 50, Count: 1},
		},
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{merchantDefinition})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected partner-merchant first-accept runtime error: %v", err)
	}
	merchant, ok := runtime.RegisterStaticActorWithInteraction("ExchangeAcceptPartnerMerchantFirst", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, merchantDefinition.Ref)
	if !ok {
		t.Fatal("expected partner-merchant first-accept static actor registration to succeed")
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d9)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070da)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected partner-merchant first-accept exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-merchant first-accept exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "partner-merchant first-accept owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected partner-merchant first-accept exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "partner-merchant first-accept peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected partner-merchant first-accept item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected partner-merchant first-accept item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "partner-merchant first-accept owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected partner-merchant first-accept item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "partner-merchant first-accept peer item-add")

	merchantOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(merchant.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected partner merchant-open before first accept error: %v", err)
	}
	if len(merchantOut) != 1 {
		t.Fatalf("expected partner merchant-open before first accept to emit one shop start frame, got %d", len(merchantOut))
	}
	if _, err := shopproto.DecodeServerStart(decodeSingleFrame(t, merchantOut[0])); err != nil {
		t.Fatalf("decode partner merchant-open before first accept shop start: %v", err)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected partner merchant-open before first accept to queue no owner frames, got %d", len(queued))
	}

	acceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected partner-merchant first-accept owner accept error: %v", err)
	}
	if len(acceptOut) != 2 {
		t.Fatalf("expected first accept to reject partner open-merchant with busy info chat then END, got %d", len(acceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, acceptOut[0]))
	if err != nil {
		t.Fatalf("decode partner open-merchant accept busy info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner open-merchant accept busy info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, acceptOut[1], "partner-merchant first-accept busy reject auto-cancel self END")
	queuedAccept := flushServerFrames(t, peerFlow)
	if len(queuedAccept) != 1 {
		t.Fatalf("expected partner open-merchant first accept to queue one peer END, got %d", len(queuedAccept))
	}
	assertExchangeEndFrame(t, queuedAccept[0], "partner-merchant first-accept busy reject auto-cancel peer END")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled busy reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, peerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after busy-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "partner open-merchant first accept owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "partner open-merchant first accept peer")
}

func TestGameRuntimeItemExchangeAcceptRejectsPartnerOpenSafeboxWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchPSafe1stOwner", 0x010307db, 0x020407db, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 806, Vnum: 27056, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchPSafe1stPeer", 0x010307dc, 0x020407dc, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "ex-p-safe-1st-a"
	peerLogin := "ex-p-safe-1st-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070db, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070dc, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partner-safebox first-accept owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed partner-safebox first-accept peer account: %v", err)
	}
	displayTemplate := itemcatalog.Template{Vnum: 27056, Name: "Partner Safebox First Accept Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{displayTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected partner-safebox first-accept runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070db)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070dc)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected partner-safebox first-accept exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected partner-safebox first-accept exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "partner-safebox first-accept owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected partner-safebox first-accept exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "partner-safebox first-accept peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected partner-safebox first-accept item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected partner-safebox first-accept item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], displayTemplate, "partner-safebox first-accept owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected partner-safebox first-accept item-add to queue one peer frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], displayTemplate, "partner-safebox first-accept peer item-add")

	openOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected partner /open_safebox before first accept error: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected partner /open_safebox before first accept to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}
	if _, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0])); err != nil {
		t.Fatalf("decode partner /open_safebox before first accept SAFEBOX_SIZE: %v", err)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected partner /open_safebox before first accept to queue no owner frames, got %d", len(queued))
	}

	acceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected partner-safebox first-accept owner accept error: %v", err)
	}
	if len(acceptOut) != 2 {
		t.Fatalf("expected first accept to reject partner open-safebox with busy info chat then END, got %d", len(acceptOut))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, acceptOut[0]))
	if err != nil {
		t.Fatalf("decode partner open-safebox accept busy info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner open-safebox accept busy info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, acceptOut[1], "partner-safebox first-accept busy reject auto-cancel self END")
	queuedAccept := flushServerFrames(t, peerFlow)
	if len(queuedAccept) != 1 {
		t.Fatalf("expected partner open-safebox first accept to queue one peer END, got %d", len(queuedAccept))
	}
	assertExchangeEndFrame(t, queuedAccept[0], "partner-safebox first-accept busy reject auto-cancel peer END")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after auto-cancelled busy reject: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, got %d frames", len(cancelOut))
	}
	if queuedCancel := flushServerFrames(t, peerFlow); len(queuedCancel) != 0 {
		t.Fatalf("expected no further peer frames after busy-reject auto-cancel, got %d", len(queuedCancel))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "partner open-safebox first accept owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "partner open-safebox first accept peer")
}

func TestSharedWorldAcceptExchangeRejectsOpenRefineWindowWithoutMutation(t *testing.T) {
	registry := newSharedWorldRegistry()
	owner := peerVisibilityCharacter("ExchAcceptRefineOwner", 0x010307dd, 0x020407dd, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	peer := peerVisibilityCharacter("ExchAcceptRefinePeer", 0x010307de, 0x020407de, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerPending := newPendingServerFrames()
	peerPending := newPendingServerFrames()
	ownerID, _ := registry.Join(owner, ownerPending, nil)
	peerID, _ := registry.Join(peer, peerPending, nil)
	if ownerID == 0 || peerID == 0 {
		t.Fatalf("expected shared-world join to allocate owner/peer ids, got owner=%d peer=%d", ownerID, peerID)
	}
	_ = ownerPending.flush()
	_ = peerPending.flush()

	startFrames, ok := registry.StartExchange(ownerID, peer.VID)
	if !ok || len(startFrames) != 1 {
		t.Fatalf("expected exchange start to succeed with one owner frame, ok=%v frames=%d", ok, len(startFrames))
	}
	if queued := peerPending.flush(); len(queued) != 1 {
		t.Fatalf("expected exchange start to queue one peer frame, got %d", len(queued))
	}

	if !registry.SetRefineWindowOpen(ownerID, true) {
		t.Fatal("expected SetRefineWindowOpen(owner) to succeed")
	}
	acceptFrames, finalizePlan, ok := registry.AcceptExchange(ownerID, owner.Gold, owner)
	if !ok || finalizePlan != nil || len(acceptFrames) != 2 {
		t.Fatalf("expected requester open-refine AcceptExchange to emit busy info chat then END, ok=%v plan=%v frames=%d", ok, finalizePlan != nil, len(acceptFrames))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, acceptFrames[0]))
	if err != nil {
		t.Fatalf("decode requester open-refine AcceptExchange busy info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected requester open-refine AcceptExchange busy info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, acceptFrames[1], "requester open-refine busy reject auto-cancel self END")
	queued := peerPending.flush()
	if len(queued) != 1 {
		t.Fatalf("expected requester open-refine AcceptExchange to queue one peer END, got %d", len(queued))
	}
	assertExchangeEndFrame(t, queued[0], "requester open-refine busy reject auto-cancel peer END")

	cancelFrames, ok := registry.CancelExchange(ownerID)
	if ok || len(cancelFrames) != 0 {
		t.Fatalf("expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, ok=%v frames=%d", ok, len(cancelFrames))
	}

	startFrames, ok = registry.StartExchange(ownerID, peer.VID)
	if !ok || len(startFrames) != 1 {
		t.Fatalf("expected re-open exchange after refine busy auto-cancel to succeed, ok=%v frames=%d", ok, len(startFrames))
	}
	if queued := peerPending.flush(); len(queued) != 1 {
		t.Fatalf("expected re-open exchange start to queue one peer frame, got %d", len(queued))
	}
	if !registry.SetRefineWindowOpen(ownerID, false) {
		t.Fatal("expected SetRefineWindowOpen(owner,false) to succeed")
	}
	if !registry.SetRefineWindowOpen(peerID, true) {
		t.Fatal("expected SetRefineWindowOpen(peer) to succeed")
	}
	acceptFrames, finalizePlan, ok = registry.AcceptExchange(ownerID, owner.Gold, owner)
	if !ok || finalizePlan != nil || len(acceptFrames) != 2 {
		t.Fatalf("expected partner open-refine AcceptExchange to emit busy info chat then END, ok=%v plan=%v frames=%d", ok, finalizePlan != nil, len(acceptFrames))
	}
	infoChat, err = chatproto.DecodeChatDelivery(decodeSingleFrame(t, acceptFrames[0]))
	if err != nil {
		t.Fatalf("decode partner open-refine AcceptExchange busy info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner open-refine AcceptExchange busy info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, acceptFrames[1], "partner open-refine busy reject auto-cancel self END")
	queued = peerPending.flush()
	if len(queued) != 1 {
		t.Fatalf("expected partner open-refine AcceptExchange to queue one peer END, got %d", len(queued))
	}
	assertExchangeEndFrame(t, queued[0], "partner open-refine busy reject auto-cancel peer END")

	cancelFrames, ok = registry.CancelExchange(ownerID)
	if ok || len(cancelFrames) != 0 {
		t.Fatalf("expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, ok=%v frames=%d", ok, len(cancelFrames))
	}
}

func TestSharedWorldAcceptExchangeRejectsOpenCubeWindowWithoutMutation(t *testing.T) {
	registry := newSharedWorldRegistry()
	owner := peerVisibilityCharacter("ExchAcceptCubeOwner", 0x010308b5, 0x020408b5, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	peer := peerVisibilityCharacter("ExchAcceptCubePeer", 0x010308b6, 0x020408b6, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerPending := newPendingServerFrames()
	peerPending := newPendingServerFrames()
	ownerID, _ := registry.Join(owner, ownerPending, nil)
	peerID, _ := registry.Join(peer, peerPending, nil)
	if ownerID == 0 || peerID == 0 {
		t.Fatalf("expected shared-world join to allocate owner/peer ids, got owner=%d peer=%d", ownerID, peerID)
	}
	_ = ownerPending.flush()
	_ = peerPending.flush()

	startFrames, ok := registry.StartExchange(ownerID, peer.VID)
	if !ok || len(startFrames) != 1 {
		t.Fatalf("expected exchange start to succeed with one owner frame, ok=%v frames=%d", ok, len(startFrames))
	}
	if queued := peerPending.flush(); len(queued) != 1 {
		t.Fatalf("expected exchange start to queue one peer frame, got %d", len(queued))
	}

	if !registry.SetCubeWindowOpen(ownerID, true) {
		t.Fatal("expected SetCubeWindowOpen(owner) to succeed")
	}
	acceptFrames, finalizePlan, ok := registry.AcceptExchange(ownerID, owner.Gold, owner)
	if !ok || finalizePlan != nil || len(acceptFrames) != 2 {
		t.Fatalf("expected requester open-cube AcceptExchange to emit busy info chat then END, ok=%v plan=%v frames=%d", ok, finalizePlan != nil, len(acceptFrames))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, acceptFrames[0]))
	if err != nil {
		t.Fatalf("decode requester open-cube AcceptExchange busy info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected requester open-cube AcceptExchange busy info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, acceptFrames[1], "requester open-cube busy reject auto-cancel self END")
	queued := peerPending.flush()
	if len(queued) != 1 {
		t.Fatalf("expected requester open-cube AcceptExchange to queue one peer END, got %d", len(queued))
	}
	assertExchangeEndFrame(t, queued[0], "requester open-cube busy reject auto-cancel peer END")

	cancelFrames, ok := registry.CancelExchange(ownerID)
	if ok || len(cancelFrames) != 0 {
		t.Fatalf("expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, ok=%v frames=%d", ok, len(cancelFrames))
	}

	startFrames, ok = registry.StartExchange(ownerID, peer.VID)
	if !ok || len(startFrames) != 1 {
		t.Fatalf("expected re-open exchange after cube busy auto-cancel to succeed, ok=%v frames=%d", ok, len(startFrames))
	}
	if queued := peerPending.flush(); len(queued) != 1 {
		t.Fatalf("expected re-open exchange start to queue one peer frame, got %d", len(queued))
	}
	if !registry.SetCubeWindowOpen(ownerID, false) {
		t.Fatal("expected SetCubeWindowOpen(owner,false) to succeed")
	}
	if !registry.SetCubeWindowOpen(peerID, true) {
		t.Fatal("expected SetCubeWindowOpen(peer) to succeed")
	}
	acceptFrames, finalizePlan, ok = registry.AcceptExchange(ownerID, owner.Gold, owner)
	if !ok || finalizePlan != nil || len(acceptFrames) != 2 {
		t.Fatalf("expected partner open-cube AcceptExchange to emit busy info chat then END, ok=%v plan=%v frames=%d", ok, finalizePlan != nil, len(acceptFrames))
	}
	infoChat, err = chatproto.DecodeChatDelivery(decodeSingleFrame(t, acceptFrames[0]))
	if err != nil {
		t.Fatalf("decode partner open-cube AcceptExchange busy info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerMerchantBusyInfoMessage {
		t.Fatalf("unexpected partner open-cube AcceptExchange busy info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, acceptFrames[1], "partner open-cube busy reject auto-cancel self END")
	queued = peerPending.flush()
	if len(queued) != 1 {
		t.Fatalf("expected partner open-cube AcceptExchange to queue one peer END, got %d", len(queued))
	}
	assertExchangeEndFrame(t, queued[0], "partner open-cube busy reject auto-cancel peer END")

	cancelFrames, ok = registry.CancelExchange(ownerID)
	if ok || len(cancelFrames) != 0 {
		t.Fatalf("expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, ok=%v frames=%d", ok, len(cancelFrames))
	}
}

func TestSharedWorldCommitExchangeFinalizeRejectsBusyWindowOpenedAfterAcceptPlan(t *testing.T) {
	registry := newSharedWorldRegistry()
	registry.SetItemTemplates(map[uint32]itemcatalog.Template{
		27060: {Vnum: 27060, Name: "Commit Busy Display Potion", Stackable: true, MaxCount: 200},
	})
	owner := peerVisibilityCharacter("ExchCommitBusyOwner", 0x010307df, 0x020407df, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 910, Vnum: 27060, Count: 1, Slot: 5}}
	peer := peerVisibilityCharacter("ExchCommitBusyPeer", 0x010307e0, 0x020407e0, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerPending := newPendingServerFrames()
	peerPending := newPendingServerFrames()
	ownerID, _ := registry.Join(owner, ownerPending, nil)
	peerID, _ := registry.Join(peer, peerPending, nil)
	if ownerID == 0 || peerID == 0 {
		t.Fatalf("expected shared-world join to allocate owner/peer ids, got owner=%d peer=%d", ownerID, peerID)
	}
	_ = ownerPending.flush()
	_ = peerPending.flush()

	buildPlan := func(label string) *exchangeFinalizePlan {
		t.Helper()
		startFrames, ok := registry.StartExchange(ownerID, peer.VID)
		if !ok || len(startFrames) != 1 {
			t.Fatalf("%s: expected exchange start to succeed with one owner frame, ok=%v frames=%d", label, ok, len(startFrames))
		}
		if queued := peerPending.flush(); len(queued) != 1 {
			t.Fatalf("%s: expected exchange start to queue one peer frame, got %d", label, len(queued))
		}
		display := player.ExchangeItemAddDisplay{Item: owner.Inventory[0]}
		itemAddFrames, ok := registry.AddExchangeItem(ownerID, 3, display)
		if !ok || len(itemAddFrames) != 1 {
			t.Fatalf("%s: expected exchange item-add to succeed with one owner frame, ok=%v frames=%d", label, ok, len(itemAddFrames))
		}
		if queued := peerPending.flush(); len(queued) != 1 {
			t.Fatalf("%s: expected exchange item-add to queue one peer frame, got %d", label, len(queued))
		}
		firstAcceptFrames, finalizePlan, ok := registry.AcceptExchange(ownerID, owner.Gold, owner)
		if !ok || finalizePlan != nil || len(firstAcceptFrames) != 1 {
			t.Fatalf("%s: expected first AcceptExchange to emit accept marker without finalize plan, ok=%v plan=%v frames=%d", label, ok, finalizePlan != nil, len(firstAcceptFrames))
		}
		if queued := peerPending.flush(); len(queued) != 1 {
			t.Fatalf("%s: expected first AcceptExchange to queue one peer accept frame, got %d", label, len(queued))
		}
		secondAcceptFrames, finalizePlan, ok := registry.AcceptExchange(peerID, peer.Gold, peer)
		if !ok || finalizePlan == nil || len(secondAcceptFrames) != 0 {
			t.Fatalf("%s: expected second AcceptExchange to return finalize plan with no frames, ok=%v plan=%v frames=%d", label, ok, finalizePlan != nil, len(secondAcceptFrames))
		}
		if queued := peerPending.flush(); len(queued) != 0 {
			t.Fatalf("%s: expected second AcceptExchange to queue no frames before commit, got %d", label, len(queued))
		}
		return finalizePlan
	}

	assertBusyAutoCancel := func(label string, finalizePlan *exchangeFinalizePlan, wantMessage string) {
		t.Helper()
		updatedOrigin := cloneExchangeCharacter(owner)
		updatedPartner := cloneExchangeCharacter(peer)
		busyFrames, committed := registry.CommitExchangeFinalize(finalizePlan, updatedOrigin, updatedPartner, [][]byte{encodeExchangeEndFrame()})
		if committed {
			t.Fatalf("%s: expected CommitExchangeFinalize to fail closed after post-plan busy-window open", label)
		}
		if len(busyFrames) != 2 {
			t.Fatalf("%s: expected commit-time busy reject to emit info chat then END, got %d frames", label, len(busyFrames))
		}
		infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, busyFrames[0]))
		if err != nil {
			t.Fatalf("%s: decode commit-time busy info chat: %v", label, err)
		}
		if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != wantMessage {
			t.Fatalf("%s: unexpected commit-time busy info chat: %+v", label, infoChat)
		}
		assertExchangeEndFrame(t, busyFrames[1], label+" busy reject auto-cancel self END")
		queued := ownerPending.flush()
		if len(queued) != 1 {
			t.Fatalf("%s: expected commit-time busy reject to queue one partner END, got %d", label, len(queued))
		}
		assertExchangeEndFrame(t, queued[0], label+" busy reject auto-cancel peer END")
		if queued := peerPending.flush(); len(queued) != 0 {
			t.Fatalf("%s: expected no extra peer frames after busy auto-cancel, got %d", label, len(queued))
		}
		cancelFrames, ok := registry.CancelExchange(peerID)
		if ok || len(cancelFrames) != 0 {
			t.Fatalf("%s: expected busy-reject auto-cancel to clear the shell so CANCEL fails closed, ok=%v frames=%d", label, ok, len(cancelFrames))
		}
	}

	finalizePlan := buildPlan("partner-safebox")
	if !registry.SetSafeboxWindowOpen(ownerID, true) {
		t.Fatal("expected SetSafeboxWindowOpen(owner) after accept plan to succeed")
	}
	// Second accepter is peer (plan.OriginID); owner safebox is partner busy.
	assertBusyAutoCancel("partner-safebox", finalizePlan, exchangePartnerMerchantBusyInfoMessage)
	if !registry.SetSafeboxWindowOpen(ownerID, false) {
		t.Fatal("expected SetSafeboxWindowOpen(owner,false) to succeed")
	}

	finalizePlan = buildPlan("requester-safebox")
	if !registry.SetSafeboxWindowOpen(peerID, true) {
		t.Fatal("expected SetSafeboxWindowOpen(peer) after accept plan to succeed")
	}
	assertBusyAutoCancel("requester-safebox", finalizePlan, exchangeRequesterMerchantBusyInfoMessage)
	if !registry.SetSafeboxWindowOpen(peerID, false) {
		t.Fatal("expected SetSafeboxWindowOpen(peer,false) to succeed")
	}

	finalizePlan = buildPlan("partner-cube")
	if !registry.SetCubeWindowOpen(ownerID, true) {
		t.Fatal("expected SetCubeWindowOpen(owner) after accept plan to succeed")
	}
	assertBusyAutoCancel("partner-cube", finalizePlan, exchangePartnerMerchantBusyInfoMessage)
}

func TestSharedWorldAcceptExchangeRejectsRequesterGoldCarrierWithoutMutation(t *testing.T) {
	registry := newSharedWorldRegistry()
	owner := peerVisibilityCharacter("ExchAcceptGoldCapOwner", 0x01030211, 0x02040211, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 8211, Vnum: 27001, Count: 1, Slot: 5}}
	peer := peerVisibilityCharacter("ExchAcceptGoldCapPeer", 0x01030212, 0x02040212, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerPending := newPendingServerFrames()
	peerPending := newPendingServerFrames()
	ownerID, _ := registry.Join(owner, ownerPending, nil)
	peerID, _ := registry.Join(peer, peerPending, nil)
	if ownerID == 0 || peerID == 0 {
		t.Fatalf("expected shared-world join to allocate owner/peer ids, got owner=%d peer=%d", ownerID, peerID)
	}
	_ = ownerPending.flush()
	_ = peerPending.flush()

	startFrames, ok := registry.StartExchange(ownerID, peer.VID)
	if !ok || len(startFrames) != 1 {
		t.Fatalf("expected exchange start to succeed with one owner frame, ok=%v frames=%d", ok, len(startFrames))
	}
	if queued := peerPending.flush(); len(queued) != 1 {
		t.Fatalf("expected exchange start to queue one peer frame, got %d", len(queued))
	}

	owner.Gold = exchangeGoldPointChangeCarrierMax
	registry.UpdateCharacter(ownerID, owner)

	acceptFrames, finalizePlan, ok := registry.AcceptExchange(ownerID, owner.Gold, owner)
	if !ok || finalizePlan != nil || len(acceptFrames) != 2 {
		t.Fatalf("expected requester gold-carrier AcceptExchange to emit info chat then END, ok=%v plan=%v frames=%d", ok, finalizePlan != nil, len(acceptFrames))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, acceptFrames[0]))
	if err != nil {
		t.Fatalf("decode requester gold-carrier AcceptExchange info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterGoldCarrierCapInfoMessage {
		t.Fatalf("unexpected requester gold-carrier AcceptExchange info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, acceptFrames[1], "requester gold-carrier reject auto-cancel self END")
	queued := peerPending.flush()
	if len(queued) != 1 {
		t.Fatalf("expected requester gold-carrier AcceptExchange to queue one peer END, got %d", len(queued))
	}
	assertExchangeEndFrame(t, queued[0], "requester gold-carrier reject auto-cancel peer END")

	cancelFrames, ok := registry.CancelExchange(ownerID)
	if ok || len(cancelFrames) != 0 {
		t.Fatalf("expected gold-carrier reject auto-cancel to clear the shell so CANCEL fails closed, ok=%v frames=%d", ok, len(cancelFrames))
	}
}

func TestSharedWorldAcceptExchangeRejectsPartnerGoldCarrierWithoutMutation(t *testing.T) {
	registry := newSharedWorldRegistry()
	owner := peerVisibilityCharacter("ExchAcceptGoldCapPartnerOwner", 0x01030213, 0x02040213, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 8213, Vnum: 27001, Count: 1, Slot: 5}}
	peer := peerVisibilityCharacter("ExchAcceptGoldCapPartnerPeer", 0x01030214, 0x02040214, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerPending := newPendingServerFrames()
	peerPending := newPendingServerFrames()
	ownerID, _ := registry.Join(owner, ownerPending, nil)
	peerID, _ := registry.Join(peer, peerPending, nil)
	if ownerID == 0 || peerID == 0 {
		t.Fatalf("expected shared-world join to allocate owner/peer ids, got owner=%d peer=%d", ownerID, peerID)
	}
	_ = ownerPending.flush()
	_ = peerPending.flush()

	startFrames, ok := registry.StartExchange(ownerID, peer.VID)
	if !ok || len(startFrames) != 1 {
		t.Fatalf("expected exchange start to succeed with one owner frame, ok=%v frames=%d", ok, len(startFrames))
	}
	if queued := peerPending.flush(); len(queued) != 1 {
		t.Fatalf("expected exchange start to queue one peer frame, got %d", len(queued))
	}

	peer.Gold = exchangeGoldPointChangeCarrierMax
	registry.UpdateCharacter(peerID, peer)

	acceptFrames, finalizePlan, ok := registry.AcceptExchange(ownerID, owner.Gold, owner)
	if !ok || finalizePlan != nil || len(acceptFrames) != 2 {
		t.Fatalf("expected partner gold-carrier AcceptExchange to emit info chat then END, ok=%v plan=%v frames=%d", ok, finalizePlan != nil, len(acceptFrames))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, acceptFrames[0]))
	if err != nil {
		t.Fatalf("decode partner gold-carrier AcceptExchange info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangePartnerGoldCarrierCapInfoMessage {
		t.Fatalf("unexpected partner gold-carrier AcceptExchange info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, acceptFrames[1], "partner gold-carrier reject auto-cancel self END")
	queued := peerPending.flush()
	if len(queued) != 1 {
		t.Fatalf("expected partner gold-carrier AcceptExchange to queue one peer END, got %d", len(queued))
	}
	assertExchangeEndFrame(t, queued[0], "partner gold-carrier reject auto-cancel peer END")

	cancelFrames, ok := registry.CancelExchange(ownerID)
	if ok || len(cancelFrames) != 0 {
		t.Fatalf("expected gold-carrier reject auto-cancel to clear the shell so CANCEL fails closed, ok=%v frames=%d", ok, len(cancelFrames))
	}
}

func TestSharedWorldCommitExchangeFinalizeRejectsRequesterGoldCarrierWithoutMutation(t *testing.T) {
	registry := newSharedWorldRegistry()
	registry.SetItemTemplates(map[uint32]itemcatalog.Template{
		27001: {Vnum: 27001, Name: "Commit Gold Cap Display Potion", Stackable: true, MaxCount: 200},
	})
	owner := peerVisibilityCharacter("ExchCommitGoldCapOwner", 0x01030215, 0x02040215, 1100, 2100, 0, 101, 201)
	owner.Gold = 500
	owner.Inventory = []inventory.ItemInstance{{ID: 8215, Vnum: 27001, Count: 1, Slot: 5}}
	peer := peerVisibilityCharacter("ExchCommitGoldCapPeer", 0x01030216, 0x02040216, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerPending := newPendingServerFrames()
	peerPending := newPendingServerFrames()
	ownerID, _ := registry.Join(owner, ownerPending, nil)
	peerID, _ := registry.Join(peer, peerPending, nil)
	if ownerID == 0 || peerID == 0 {
		t.Fatalf("expected shared-world join to allocate owner/peer ids, got owner=%d peer=%d", ownerID, peerID)
	}
	_ = ownerPending.flush()
	_ = peerPending.flush()

	startFrames, ok := registry.StartExchange(ownerID, peer.VID)
	if !ok || len(startFrames) != 1 {
		t.Fatalf("expected exchange start to succeed with one owner frame, ok=%v frames=%d", ok, len(startFrames))
	}
	_ = peerPending.flush()

	display := player.ExchangeItemAddDisplay{Item: owner.Inventory[0]}
	itemAddFrames, ok := registry.AddExchangeItem(ownerID, 3, display)
	if !ok || len(itemAddFrames) != 1 {
		t.Fatalf("expected exchange item-add to succeed with one owner frame, ok=%v frames=%d", ok, len(itemAddFrames))
	}
	_ = peerPending.flush()

	firstAcceptFrames, finalizePlan, ok := registry.AcceptExchange(ownerID, owner.Gold, owner)
	if !ok || finalizePlan != nil || len(firstAcceptFrames) != 1 {
		t.Fatalf("expected first AcceptExchange to emit accept marker without finalize plan, ok=%v plan=%v frames=%d", ok, finalizePlan != nil, len(firstAcceptFrames))
	}
	_ = peerPending.flush()

	secondAcceptFrames, finalizePlan, ok := registry.AcceptExchange(peerID, peer.Gold, peer)
	if !ok || finalizePlan == nil || len(secondAcceptFrames) != 0 {
		t.Fatalf("expected second AcceptExchange to return finalize plan with no frames, ok=%v plan=%v frames=%d", ok, finalizePlan != nil, len(secondAcceptFrames))
	}

	// Commit requester is the second accepter (peer / plan.OriginID). Drift that side to the carrier max.
	updatedOrigin := cloneExchangeCharacter(peer)
	updatedOrigin.Gold = exchangeGoldPointChangeCarrierMax
	updatedPartner := cloneExchangeCharacter(owner)
	registry.UpdateCharacter(peerID, updatedOrigin)

	rejectFrames, committed := registry.CommitExchangeFinalize(finalizePlan, updatedOrigin, updatedPartner, [][]byte{encodeExchangeEndFrame()})
	if committed {
		t.Fatal("expected CommitExchangeFinalize to fail closed after commit-requester gold-carrier drift")
	}
	if len(rejectFrames) != 2 {
		t.Fatalf("expected commit-time requester gold-carrier reject to emit info chat then END, got %d frames", len(rejectFrames))
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, rejectFrames[0]))
	if err != nil {
		t.Fatalf("decode commit-time requester gold-carrier info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != exchangeRequesterGoldCarrierCapInfoMessage {
		t.Fatalf("unexpected commit-time requester gold-carrier info chat: %+v", infoChat)
	}
	assertExchangeEndFrame(t, rejectFrames[1], "commit-time gold-carrier reject auto-cancel self END")
	queued := ownerPending.flush()
	if len(queued) != 1 {
		t.Fatalf("expected commit-time gold-carrier reject to queue one partner END, got %d", len(queued))
	}
	assertExchangeEndFrame(t, queued[0], "commit-time gold-carrier reject auto-cancel peer END")

	cancelFrames, ok := registry.CancelExchange(peerID)
	if ok || len(cancelFrames) != 0 {
		t.Fatalf("expected gold-carrier reject auto-cancel to clear the shell so CANCEL fails closed, ok=%v frames=%d", ok, len(cancelFrames))
	}
}

func TestGameRuntimeStoragePacketsFailClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("StorageGuardOwner", 0x010307c0, 0x020407c0, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 760, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "storage-guard-owner", 0x707070c0, owner)
	if err := accounts.Save(accountstore.Account{Login: "storage-guard-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed storage guard account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected storage guard runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "storage-guard-owner", 0x707070c0)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	requests := []struct {
		name string
		raw  []byte
	}{
		{name: "safebox checkin", raw: itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{SafeSlot: 7, Position: itemproto.InventoryPosition(5)})},
		{name: "safebox checkout", raw: itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{SafeSlot: 8, Position: itemproto.InventoryPosition(6)})},
		{name: "safebox item move", raw: itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{Source: itemproto.InventoryPosition(7), Destination: itemproto.InventoryPosition(8), Count: 3})},
		{name: "mall checkout", raw: itemproto.EncodeClientMallCheckout(itemproto.ClientMallCheckoutPacket{MallSlot: 4, Position: itemproto.InventoryPosition(9)})},
	}
	for _, request := range requests {
		t.Run(request.name, func(t *testing.T) {
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, request.raw))
			if err != nil {
				t.Fatalf("unexpected storage packet error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected storage packet to emit no frames, got %d", len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected storage packet to queue no frames, got %d", len(queued))
			}
			assertExchangeAccountUnchanged(t, accounts, "storage-guard-owner", owner, request.name)
		})
	}
}

func TestGameRuntimeItemExchangeMutualAcceptFinalizesDisplayedTradeAndClosesShell(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeFinalizeOwner", 0x010307c1, 0x020407c1, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 860, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeFinalizePeer", 0x010307c2, 0x020407c2, 1120, 2120, 0, 101, 201)
	peer.Gold = 7000
	peer.Inventory = []inventory.ItemInstance{{ID: 861, Vnum: 27046, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-finalize-owner"
	peerLogin := "ex-finalize-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070c1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070c2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange finalize owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange finalize peer account: %v", err)
	}
	ownerTemplate := itemcatalog.Template{Vnum: 27045, Name: "Finalize Owner Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{1, 2, 3}}
	peerTemplate := itemcatalog.Template{Vnum: 27046, Name: "Finalize Peer Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{4, 5, 6}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{ownerTemplate, peerTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange finalize runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070c1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070c2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected finalize exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected finalize exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "finalize owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected finalize exchange start to queue one peer frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "finalize peer start")

	ownerItemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 3, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected finalize owner item-add error: %v", err)
	}
	if len(ownerItemAddOut) != 1 {
		t.Fatalf("expected finalize owner item-add to emit one frame, got %d", len(ownerItemAddOut))
	}
	assertExchangeItemAddFrame(t, ownerItemAddOut[0], 1, 3, owner.Inventory[0], ownerTemplate, "finalize owner item-add")
	queuedOwnerItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedOwnerItemAdd) != 1 {
		t.Fatalf("expected finalize owner item-add to queue one peer frame, got %d", len(queuedOwnerItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedOwnerItemAdd[0], 0, 3, owner.Inventory[0], ownerTemplate, "finalize peer item-add")

	peerItemAddOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 4, Position: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected finalize peer item-add error: %v", err)
	}
	if len(peerItemAddOut) != 1 {
		t.Fatalf("expected finalize peer item-add to emit one frame, got %d", len(peerItemAddOut))
	}
	assertExchangeItemAddFrame(t, peerItemAddOut[0], 1, 4, peer.Inventory[0], peerTemplate, "finalize peer self item-add")
	queuedPeerItemAdd := flushServerFrames(t, ownerFlow)
	if len(queuedPeerItemAdd) != 1 {
		t.Fatalf("expected finalize peer item-add to queue one owner frame, got %d", len(queuedPeerItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedPeerItemAdd[0], 0, 4, peer.Inventory[0], peerTemplate, "finalize owner peer item-add")

	ownerGoldOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 1200})))
	if err != nil {
		t.Fatalf("unexpected finalize owner gold-add error: %v", err)
	}
	if len(ownerGoldOut) != 1 {
		t.Fatalf("expected finalize owner gold-add to emit one frame, got %d", len(ownerGoldOut))
	}
	assertExchangeGoldAddFrame(t, ownerGoldOut[0], 1, 1200, "finalize owner gold-add")
	queuedOwnerGold := flushServerFrames(t, peerFlow)
	if len(queuedOwnerGold) != 1 {
		t.Fatalf("expected finalize owner gold-add to queue one peer frame, got %d", len(queuedOwnerGold))
	}
	assertExchangeGoldAddFrame(t, queuedOwnerGold[0], 0, 1200, "finalize peer gold-add")

	peerGoldOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 800})))
	if err != nil {
		t.Fatalf("unexpected finalize peer gold-add error: %v", err)
	}
	if len(peerGoldOut) != 1 {
		t.Fatalf("expected finalize peer gold-add to emit one frame, got %d", len(peerGoldOut))
	}
	assertExchangeGoldAddFrame(t, peerGoldOut[0], 1, 800, "finalize peer self gold-add")
	queuedPeerGold := flushServerFrames(t, ownerFlow)
	if len(queuedPeerGold) != 1 {
		t.Fatalf("expected finalize peer gold-add to queue one owner frame, got %d", len(queuedPeerGold))
	}
	assertExchangeGoldAddFrame(t, queuedPeerGold[0], 0, 800, "finalize owner peer gold-add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected finalize owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected finalize owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "finalize owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected finalize owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "finalize owner accept peer")

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected finalize peer accept error: %v", err)
	}
	// peer finalize burst: ACCEPT + ITEM_DEL(source) + ITEM_SET(incoming) + QUICKSLOT_DEL(source) + POINT_CHANGE(gold) + success chat + END
	if len(peerAcceptOut) != 7 {
		t.Fatalf("expected mutual-accept finalize peer burst of 7 frames (accept, item del, item set, quickslot del, gold, success chat, end), got %d", len(peerAcceptOut))
	}
	assertExchangeAcceptFrame(t, peerAcceptOut[0], 1, "finalize peer accept")
	peerItemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, peerAcceptOut[1]))
	if err != nil {
		t.Fatalf("decode finalize peer item delete: %v", err)
	}
	if peerItemDel.Position != itemproto.InventoryPosition(6) {
		t.Fatalf("unexpected finalize peer item delete: %+v", peerItemDel)
	}
	peerItemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, peerAcceptOut[2]))
	if err != nil {
		t.Fatalf("decode finalize peer item set: %v", err)
	}
	if peerItemSet.Position != itemproto.InventoryPosition(6) || peerItemSet.Vnum != ownerTemplate.Vnum || peerItemSet.Count != 3 {
		t.Fatalf("unexpected finalize peer item set: %+v", peerItemSet)
	}
	peerQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, peerAcceptOut[3]))
	if err != nil {
		t.Fatalf("decode finalize peer quickslot delete: %v", err)
	}
	if peerQuickslotDel.Position != 3 {
		t.Fatalf("unexpected finalize peer quickslot delete: %+v", peerQuickslotDel)
	}
	peerSuccessChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[len(peerAcceptOut)-2]))
	if err != nil {
		t.Fatalf("decode finalize peer success chat: %v", err)
	}
	if peerSuccessChat.Type != chatproto.ChatTypeInfo || peerSuccessChat.VID != 0 || peerSuccessChat.Message != exchangeFinalizeSuccessInfoMessage(owner.Name) {
		t.Fatalf("unexpected finalize peer success chat: %+v", peerSuccessChat)
	}
	assertExchangeEndFrame(t, peerAcceptOut[len(peerAcceptOut)-1], "finalize peer shell end")
	queuedPeerAccept := flushServerFrames(t, ownerFlow)
	// owner queued burst: ACCEPT + ITEM_DEL(source) + ITEM_SET(incoming) + QUICKSLOT_DEL(source) + POINT_CHANGE(gold) + success chat + END
	if len(queuedPeerAccept) != 7 {
		t.Fatalf("expected mutual-accept finalize owner queued burst of 7 frames (accept, item del, item set, quickslot del, gold, success chat, end), got %d", len(queuedPeerAccept))
	}
	assertExchangeAcceptFrame(t, queuedPeerAccept[0], 0, "finalize peer accept owner")
	ownerItemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, queuedPeerAccept[1]))
	if err != nil {
		t.Fatalf("decode finalize owner item delete: %v", err)
	}
	if ownerItemDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected finalize owner item delete: %+v", ownerItemDel)
	}
	ownerItemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, queuedPeerAccept[2]))
	if err != nil {
		t.Fatalf("decode finalize owner item set: %v", err)
	}
	if ownerItemSet.Position != itemproto.InventoryPosition(5) || ownerItemSet.Vnum != peerTemplate.Vnum || ownerItemSet.Count != 2 {
		t.Fatalf("unexpected finalize owner item set: %+v", ownerItemSet)
	}
	ownerQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, queuedPeerAccept[3]))
	if err != nil {
		t.Fatalf("decode finalize owner quickslot delete: %v", err)
	}
	if ownerQuickslotDel.Position != 2 {
		t.Fatalf("unexpected finalize owner quickslot delete: %+v", ownerQuickslotDel)
	}
	ownerSuccessChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedPeerAccept[len(queuedPeerAccept)-2]))
	if err != nil {
		t.Fatalf("decode finalize owner success chat: %v", err)
	}
	if ownerSuccessChat.Type != chatproto.ChatTypeInfo || ownerSuccessChat.VID != 0 || ownerSuccessChat.Message != exchangeFinalizeSuccessInfoMessage(peer.Name) {
		t.Fatalf("unexpected finalize owner success chat: %+v", ownerSuccessChat)
	}
	assertExchangeEndFrame(t, queuedPeerAccept[len(queuedPeerAccept)-1], "finalize owner queued shell end")

	ownerAccount, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load finalize owner account: %v", err)
	}
	peerAccount, err := accounts.Load(peerLogin)
	if err != nil {
		t.Fatalf("load finalize peer account: %v", err)
	}
	if ownerAccount.Characters[0].Gold != 4600 {
		t.Fatalf("expected finalize owner gold 4600 after +800/-1200, got %d", ownerAccount.Characters[0].Gold)
	}
	if peerAccount.Characters[0].Gold != 7400 {
		t.Fatalf("expected finalize peer gold 7400 after +1200/-800, got %d", peerAccount.Characters[0].Gold)
	}
	if !reflect.DeepEqual(ownerAccount.Characters[0].Inventory, []inventory.ItemInstance{{ID: 861, Vnum: 27046, Count: 2, Slot: 5}}) {
		t.Fatalf("unexpected finalize owner inventory after mutual accept: %#v", ownerAccount.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(peerAccount.Characters[0].Inventory, []inventory.ItemInstance{{ID: 860, Vnum: 27045, Count: 3, Slot: 6}}) {
		t.Fatalf("unexpected finalize peer inventory after mutual accept: %#v", peerAccount.Characters[0].Inventory)
	}
	if len(ownerAccount.Characters[0].Quickslots) != 0 {
		t.Fatalf("expected finalize owner source quickslot to clear after whole-stack transfer, got %#v", ownerAccount.Characters[0].Quickslots)
	}
	if len(peerAccount.Characters[0].Quickslots) != 0 {
		t.Fatalf("expected finalize peer source quickslot to clear after whole-stack transfer, got %#v", peerAccount.Characters[0].Quickslots)
	}

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-finalize cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected mutual-accept finalize to close the shell so later cancel is a no-op, got %d frames", len(cancelOut))
	}
}

func TestGameRuntimeItemExchangeMutualAcceptFailsClosedWhenPartnerPersistenceFails(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangePersistFailOwner", 0x010307d1, 0x020407d1, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 870, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangePersistFailPeer", 0x010307d2, 0x020407d2, 1120, 2120, 0, 101, 201)
	peer.Gold = 7000
	peer.Inventory = []inventory.ItemInstance{{ID: 871, Vnum: 27046, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-persist-fail-owner"
	peerLogin := "ex-persist-fail-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070d2, peer)
	accounts := newExchangeFinalizeFailingAccountStore(
		peerLogin,
		accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})},
		accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})},
	)
	ownerTemplate := itemcatalog.Template{Vnum: 27045, Name: "Persist Fail Owner Potion", Stackable: true, MaxCount: 200}
	peerTemplate := itemcatalog.Template{Vnum: 27046, Name: "Persist Fail Peer Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{ownerTemplate, peerTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange persist-fail runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070d2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected persist-fail exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected persist-fail exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	if out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 3, Position: itemproto.InventoryPosition(5)}))); err != nil || len(out) != 1 {
		t.Fatalf("expected persist-fail owner item-add to emit one frame, got frames=%d err=%v", len(out), err)
	}
	_ = flushServerFrames(t, peerFlow)
	if out, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 4, Position: itemproto.InventoryPosition(6)}))); err != nil || len(out) != 1 {
		t.Fatalf("expected persist-fail peer item-add to emit one frame, got frames=%d err=%v", len(out), err)
	}
	_ = flushServerFrames(t, ownerFlow)
	if out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 1200}))); err != nil || len(out) != 1 {
		t.Fatalf("expected persist-fail owner gold-add to emit one frame, got frames=%d err=%v", len(out), err)
	}
	_ = flushServerFrames(t, peerFlow)
	if out, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 800}))); err != nil || len(out) != 1 {
		t.Fatalf("expected persist-fail peer gold-add to emit one frame, got frames=%d err=%v", len(out), err)
	}
	_ = flushServerFrames(t, ownerFlow)

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected persist-fail owner accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected persist-fail owner accept to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "persist-fail owner accept")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected persist-fail owner accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "persist-fail owner accept peer")

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected persist-fail peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 0 {
		t.Fatalf("expected partner persistence failure to fail closed with no finalize frames, got %d", len(peerAcceptOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected partner persistence failure to queue no owner frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "persist-fail owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "persist-fail peer")
	assertExchangeLiveStateUnchanged(t, runtime, owner, "persist-fail owner live")
	assertExchangeLiveStateUnchanged(t, runtime, peer, "persist-fail peer live")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected persist-fail cancel after failed finalize error: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected failed mutual-accept finalize to leave the shell cancellable with one self END, got %d", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "persist-fail owner cancel")
	queuedCancel := flushServerFrames(t, peerFlow)
	if len(queuedCancel) != 1 {
		t.Fatalf("expected failed mutual-accept finalize cancel to queue one peer END, got %d", len(queuedCancel))
	}
	assertExchangeEndFrame(t, queuedCancel[0], "persist-fail peer cancel")
}

func TestGameRuntimeItemExchangeMutualAcceptFailsClosedWhenOriginPersistenceFails(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeOriginPersistFailOwner", 0x010307d3, 0x020407d3, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 872, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeOriginPersistFailPeer", 0x010307d4, 0x020407d4, 1120, 2120, 0, 101, 201)
	peer.Gold = 7000
	peer.Inventory = []inventory.ItemInstance{{ID: 873, Vnum: 27046, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-origin-persist-fail-owner"
	peerLogin := "ex-origin-persist-fail-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d3, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070d4, peer)
	accounts := newExchangeFinalizeFailingAccountStore(
		ownerLogin,
		accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})},
		accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})},
	)
	ownerTemplate := itemcatalog.Template{Vnum: 27045, Name: "Origin Persist Fail Owner Potion", Stackable: true, MaxCount: 200}
	peerTemplate := itemcatalog.Template{Vnum: 27046, Name: "Origin Persist Fail Peer Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{ownerTemplate, peerTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected origin persist-fail runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d3)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070d4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	if out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID}))); err != nil || len(out) != 1 {
		t.Fatalf("expected origin persist-fail exchange start to emit one frame, got frames=%d err=%v", len(out), err)
	}
	_ = flushServerFrames(t, peerFlow)
	if out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 3, Position: itemproto.InventoryPosition(5)}))); err != nil || len(out) != 1 {
		t.Fatalf("expected origin persist-fail owner item-add to emit one frame, got frames=%d err=%v", len(out), err)
	}
	_ = flushServerFrames(t, peerFlow)
	if out, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 4, Position: itemproto.InventoryPosition(6)}))); err != nil || len(out) != 1 {
		t.Fatalf("expected origin persist-fail peer item-add to emit one frame, got frames=%d err=%v", len(out), err)
	}
	_ = flushServerFrames(t, ownerFlow)
	if out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 1200}))); err != nil || len(out) != 1 {
		t.Fatalf("expected origin persist-fail owner gold-add to emit one frame, got frames=%d err=%v", len(out), err)
	}
	_ = flushServerFrames(t, peerFlow)
	if out, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 800}))); err != nil || len(out) != 1 {
		t.Fatalf("expected origin persist-fail peer gold-add to emit one frame, got frames=%d err=%v", len(out), err)
	}
	_ = flushServerFrames(t, ownerFlow)

	if out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept}))); err != nil || len(out) != 1 {
		t.Fatalf("expected origin persist-fail owner accept to emit one frame, got frames=%d err=%v", len(out), err)
	}
	_ = flushServerFrames(t, peerFlow)

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected origin persist-fail peer accept error: %v", err)
	}
	if len(peerAcceptOut) != 0 {
		t.Fatalf("expected origin persistence failure to fail closed with no finalize frames, got %d", len(peerAcceptOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected origin persistence failure to queue no owner frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "origin persist-fail owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "origin persist-fail peer")
	assertExchangeLiveStateUnchanged(t, runtime, owner, "origin persist-fail owner live")
	assertExchangeLiveStateUnchanged(t, runtime, peer, "origin persist-fail peer live")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected origin persist-fail cancel after failed finalize error: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected failed origin mutual-accept finalize to leave the shell cancellable with one self END, got %d", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "origin persist-fail owner cancel")
	queuedCancel := flushServerFrames(t, peerFlow)
	if len(queuedCancel) != 1 {
		t.Fatalf("expected failed origin mutual-accept finalize cancel to queue one peer END, got %d", len(queuedCancel))
	}
	assertExchangeEndFrame(t, queuedCancel[0], "origin persist-fail peer cancel")
}

func TestGameRuntimeItemExchangeAcceptDisplaysWithoutFinalizingTrade(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeAcceptOwner", 0x01030782, 0x02040782, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 740, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeAcceptPeer", 0x01030783, 0x02040783, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 741, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-exchange-accept-owner", 0x70707082, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-accept-peer", 0x70707083, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-accept-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange accept owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-accept-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange accept peer account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27045, Name: "Exchange Accept Potion", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange accept runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-accept-owner", 0x70707082)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-accept-peer", 0x70707083)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected accept exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected accept exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected owner exchange accept error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected owner exchange accept to emit one self frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "owner accept self response")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected owner exchange accept to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "owner accept peer response")

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected peer exchange accept error: %v", err)
	}
	if len(peerAcceptOut) != 3 {
		t.Fatalf("expected empty mutual-accept finalize to emit peer accept marker, success chat, and shell END, got %d", len(peerAcceptOut))
	}
	assertExchangeAcceptFrame(t, peerAcceptOut[0], 1, "peer accept self response")
	peerSuccessChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, peerAcceptOut[1]))
	if err != nil {
		t.Fatalf("decode empty mutual-accept peer success chat: %v", err)
	}
	if peerSuccessChat.Type != chatproto.ChatTypeInfo || peerSuccessChat.VID != 0 || peerSuccessChat.Message != exchangeFinalizeSuccessInfoMessage(owner.Name) {
		t.Fatalf("unexpected empty mutual-accept peer success chat: %+v", peerSuccessChat)
	}
	assertExchangeEndFrame(t, peerAcceptOut[2], "peer accept shell end")
	queuedPeerAccept := flushServerFrames(t, ownerFlow)
	if len(queuedPeerAccept) != 3 {
		t.Fatalf("expected empty mutual-accept finalize to queue owner accept marker, success chat, and shell END, got %d", len(queuedPeerAccept))
	}
	assertExchangeAcceptFrame(t, queuedPeerAccept[0], 0, "peer accept owner response")
	ownerSuccessChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, queuedPeerAccept[1]))
	if err != nil {
		t.Fatalf("decode empty mutual-accept owner success chat: %v", err)
	}
	if ownerSuccessChat.Type != chatproto.ChatTypeInfo || ownerSuccessChat.VID != 0 || ownerSuccessChat.Message != exchangeFinalizeSuccessInfoMessage(peer.Name) {
		t.Fatalf("unexpected empty mutual-accept owner success chat: %+v", ownerSuccessChat)
	}
	assertExchangeEndFrame(t, queuedPeerAccept[2], "owner queued shell end")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected exchange cancel after empty mutual accept error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected empty mutual-accept finalize to close the shell so later cancel is a no-op, got %d frames", len(cancelOut))
	}

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-accept-owner", owner, "owner exchange accept")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-accept-peer", peer, "peer exchange accept")
}

func TestGameRuntimeItemExchangeDisplayChangeResetsAcceptedMarkersWithoutFinalizingTrade(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeAcceptResetOwner", 0x01030785, 0x02040785, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 743, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeAcceptResetPeer", 0x01030786, 0x02040786, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 744, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-acc-reset-a"
	peerLogin := "ex-acc-reset-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707085, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707086, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange accept-reset owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange accept-reset peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27045, Name: "Accepted Reset Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{7, 8, 9}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange accept-reset runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707085)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707086)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected accept-reset exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected accept-reset exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected owner accept before reset error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected owner accept before reset to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "owner accept before reset")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected owner accept before reset to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "owner accept before reset peer")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderItemAdd,
		Arg2:      7,
		Position:  itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected accepted exchange item-add reset error: %v", err)
	}
	if len(itemAddOut) != 2 {
		t.Fatalf("expected one-side accepted exchange item-add to emit item display plus one accept reset, got %d frames", len(itemAddOut))
	}
	assertExchangeAcceptFrameWithValue(t, itemAddOut[0], 1, 0, "accepted-reset owner-side self marker")
	assertExchangeItemAddFrame(t, itemAddOut[1], 1, 7, owner.Inventory[0], template, "accepted-reset item-add self response")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 2 {
		t.Fatalf("expected one-side accepted exchange item-add to queue display plus one accept reset, got %d frames", len(queuedItemAdd))
	}
	assertExchangeAcceptFrameWithValue(t, queuedItemAdd[0], 0, 0, "accepted-reset owner-side peer marker")
	assertExchangeItemAddFrame(t, queuedItemAdd[1], 0, 7, owner.Inventory[0], template, "accepted-reset item-add peer response")

	reAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected owner accept after reset error: %v", err)
	}
	if len(reAcceptOut) != 1 {
		t.Fatalf("expected owner accept after reset to emit one frame, got %d", len(reAcceptOut))
	}
	assertExchangeAcceptFrame(t, reAcceptOut[0], 1, "owner accept after reset")
	queuedReAccept := flushServerFrames(t, peerFlow)
	if len(queuedReAccept) != 1 {
		t.Fatalf("expected owner accept after reset to queue one peer frame, got %d", len(queuedReAccept))
	}
	assertExchangeAcceptFrame(t, queuedReAccept[0], 0, "owner accept after reset peer")

	reacceptedGoldOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderGoldAdd, Arg1: 321})))
	if err != nil {
		t.Fatalf("unexpected reaccepted exchange gold-add reset error: %v", err)
	}
	if len(reacceptedGoldOut) != 2 {
		t.Fatalf("expected reaccepted exchange gold-add to emit display plus one owner accept reset, got %d frames", len(reacceptedGoldOut))
	}
	assertExchangeAcceptFrameWithValue(t, reacceptedGoldOut[0], 1, 0, "reaccepted gold-add self accept reset")
	assertExchangeGoldAddFrame(t, reacceptedGoldOut[1], 1, 321, "reaccepted gold-add self response")
	queuedReacceptedGold := flushServerFrames(t, peerFlow)
	if len(queuedReacceptedGold) != 2 {
		t.Fatalf("expected reaccepted exchange gold-add to queue display plus one owner accept reset, got %d frames", len(queuedReacceptedGold))
	}
	assertExchangeAcceptFrameWithValue(t, queuedReacceptedGold[0], 0, 0, "reaccepted gold-add peer accept reset")
	assertExchangeGoldAddFrame(t, queuedReacceptedGold[1], 0, 321, "reaccepted gold-add peer response")

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "owner exchange accept reset")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer exchange accept reset")
}

func TestGameRuntimeItemExchangeItemDelResetsAcceptedMarkersBeforeDisplayClear(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeDelResetOwner", 0x01030787, 0x02040787, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 745, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeDelResetPeer", 0x01030788, 0x02040788, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 746, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	ownerLogin := "ex-del-reset-a"
	peerLogin := "ex-del-reset-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707087, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707088, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange item-del reset owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange item-del reset peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27045, Name: "Accepted Del Reset Potion", Stackable: true, MaxCount: 200, Sockets: itemcatalog.SocketValues{4, 5, 6}}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange item-del reset runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707087)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707088)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected item-del reset exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected item-del reset exchange start to emit one owner frame, got %d", len(startOut))
	}
	_ = flushServerFrames(t, peerFlow)

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderItemAdd,
		Arg2:      7,
		Position:  itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected item-del reset item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected item-del reset item-add to emit one frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], template, "item-del reset owner add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected item-del reset peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], template, "item-del reset peer add")

	ownerAcceptOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected owner accept before item-del reset error: %v", err)
	}
	if len(ownerAcceptOut) != 1 {
		t.Fatalf("expected owner accept before item-del reset to emit one frame, got %d", len(ownerAcceptOut))
	}
	assertExchangeAcceptFrame(t, ownerAcceptOut[0], 1, "owner accept before item-del reset")
	queuedOwnerAccept := flushServerFrames(t, peerFlow)
	if len(queuedOwnerAccept) != 1 {
		t.Fatalf("expected owner accept before item-del reset to queue one peer frame, got %d", len(queuedOwnerAccept))
	}
	assertExchangeAcceptFrame(t, queuedOwnerAccept[0], 0, "owner accept before item-del reset peer")

	itemDelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemDel, Arg1: 7})))
	if err != nil {
		t.Fatalf("unexpected accepted exchange item-del reset error: %v", err)
	}
	if len(itemDelOut) != 2 {
		t.Fatalf("expected one-side accepted exchange item-del to emit item-del plus one accept reset, got %d frames", len(itemDelOut))
	}
	assertExchangeAcceptFrameWithValue(t, itemDelOut[0], 1, 0, "item-del reset owner-side self marker")
	assertExchangeItemDelFrame(t, itemDelOut[1], 1, 7, "item-del reset self response")
	queuedItemDel := flushServerFrames(t, peerFlow)
	if len(queuedItemDel) != 2 {
		t.Fatalf("expected one-side accepted exchange item-del to queue item-del plus one accept reset, got %d frames", len(queuedItemDel))
	}
	assertExchangeAcceptFrameWithValue(t, queuedItemDel[0], 0, 0, "item-del reset owner-side peer marker")
	assertExchangeItemDelFrame(t, queuedItemDel[1], 0, 7, "item-del reset peer response")

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "owner exchange item-del reset")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer exchange item-del reset")
}

func TestGameRuntimeItemExchangeAcceptRequiresActiveShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeAcceptNoShell", 0x01030784, 0x02040784, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 742, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-exchange-accept-no-shell", 0x70707084, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-accept-no-shell", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange accept no-shell account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected exchange accept no-shell runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-accept-no-shell", 0x70707084)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected exchange accept no-shell error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected exchange accept without active shell to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after exchange accept without active shell, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-accept-no-shell", owner, "exchange accept without active shell")
}

func TestGameRuntimeItemExchangeStartRequiresVisiblePeerWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeFarStarter", 0x01030765, 0x02040765, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 706, Vnum: 27001, Count: 3, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeFarTarget", 0x01030766, 0x02040766, 1120, 2120, 0, 101, 201)
	peer.MapIndex = bootstrapMapIndex + 1
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 707, Vnum: 27002, Count: 2, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-exchange-far-starter", 0x70707065, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-far-target", 0x70707066, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-far-starter", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed far exchange starter account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-far-target", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed far exchange target account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected far exchange-start runtime error: %v", err)
	}
	starterFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-far-starter", 0x70707065)
	defer closeSessionFlow(t, starterFlow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-far-target", 0x70707066)
	defer closeSessionFlow(t, targetFlow)
	_ = flushServerFrames(t, starterFlow)
	_ = flushServerFrames(t, targetFlow)

	out, err := starterFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected far exchange-start packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected out-of-map exchange start to emit no starter frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, targetFlow); len(queued) != 0 {
		t.Fatalf("expected out-of-map exchange start to queue no target frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-far-starter", owner, "far starter exchange start")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-far-target", peer, "far target exchange start")
}

func TestGameRuntimeItemExchangeStartReportsAlreadyBusyPeerWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeBusyOwner", 0x0103076b, 0x0204076b, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 712, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	busyPeer := peerVisibilityCharacter("ExchangeBusyPeer", 0x0103076c, 0x0204076c, 1120, 2120, 0, 101, 201)
	busyPeer.Gold = 22222
	busyPeer.Inventory = []inventory.ItemInstance{{ID: 713, Vnum: 27002, Count: 2, Slot: 6}}
	busyPeer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	challenger := peerVisibilityCharacter("ExchangeBusyChallenger", 0x0103076d, 0x0204076d, 1140, 2140, 0, 101, 201)
	challenger.Gold = 33333
	challenger.Inventory = []inventory.ItemInstance{{ID: 714, Vnum: 27003, Count: 4, Slot: 7}}
	challenger.Quickslots = []loginticket.Quickslot{{Position: 4, Type: quickslotproto.TypeItem, Slot: 7}}
	issuePeerTicket(t, ticketStore, "item-exchange-busy-owner", 0x7070706b, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-busy-peer", 0x7070706c, busyPeer)
	issuePeerTicket(t, ticketStore, "item-exchange-busy-challenger", 0x7070706d, challenger)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-busy-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed busy exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-busy-peer", Empire: busyPeer.Empire, Characters: cloneCharacters([]loginticket.Character{busyPeer})}); err != nil {
		t.Fatalf("seed busy exchange peer account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-busy-challenger", Empire: challenger.Empire, Characters: cloneCharacters([]loginticket.Character{challenger})}); err != nil {
		t.Fatalf("seed busy exchange challenger account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected busy exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-busy-owner", 0x7070706b)
	defer closeSessionFlow(t, ownerFlow)
	busyPeerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-busy-peer", 0x7070706c)
	defer closeSessionFlow(t, busyPeerFlow)
	challengerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-busy-challenger", 0x7070706d)
	defer closeSessionFlow(t, challengerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, busyPeerFlow)
	_ = flushServerFrames(t, challengerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      busyPeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected initial exchange-start packet error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected initial exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], busyPeer.VID, "owner initial response")
	queuedStart := flushServerFrames(t, busyPeerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected busy peer to receive one initial queued start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "busy peer initial queued response")

	busyOut, err := challengerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      busyPeer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected busy exchange-start packet error: %v", err)
	}
	if len(busyOut) != 1 {
		t.Fatalf("expected busy exchange start to emit one already frame, got %d", len(busyOut))
	}
	assertExchangeAlreadyFrame(t, busyOut[0], "challenger busy response")
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected busy exchange start to queue no owner frames, got %d", len(queued))
	}
	if queued := flushServerFrames(t, busyPeerFlow); len(queued) != 0 {
		t.Fatalf("expected busy exchange start to queue no busy-peer frames, got %d", len(queued))
	}

	cancelOut, err := busyPeerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected cancel after busy exchange-start packet error: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected busy peer cancel after already response to emit one end frame, got %d", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "busy peer cancel after already response")
	queuedEnd := flushServerFrames(t, ownerFlow)
	if len(queuedEnd) != 1 {
		t.Fatalf("expected owner to receive queued end after busy peer cancel, got %d", len(queuedEnd))
	}
	assertExchangeEndFrame(t, queuedEnd[0], "owner queued cancel after already response")

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-busy-owner", owner, "busy owner exchange start")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-busy-peer", busyPeer, "busy peer exchange start")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-busy-challenger", challenger, "busy challenger exchange start")
}

func TestGameRuntimeItemExchangeCancelClosesBothPeerWindowsWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeCancelStarter", 0x01030767, 0x02040767, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 708, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeCancelTarget", 0x01030768, 0x02040768, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 709, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-exchange-cancel-starter", 0x70707067, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-cancel-target", 0x70707068, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-cancel-starter", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange cancel starter account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-cancel-target", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange cancel target account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected exchange-cancel runtime error: %v", err)
	}
	starterFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-cancel-starter", 0x70707067)
	defer closeSessionFlow(t, starterFlow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-cancel-target", 0x70707068)
	defer closeSessionFlow(t, targetFlow)
	_ = flushServerFrames(t, starterFlow)
	_ = flushServerFrames(t, targetFlow)

	startOut, err := starterFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected exchange-start before cancel error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange starter to receive one start frame before cancel, got %d", len(startOut))
	}
	if queuedStart := flushServerFrames(t, targetFlow); len(queuedStart) != 1 {
		t.Fatalf("expected exchange target to receive one queued start frame before cancel, got %d", len(queuedStart))
	}

	cancelOut, err := targetFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected exchange-cancel packet error: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected cancelling target to receive one end frame, got %d", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "target cancel response")
	queuedEnd := flushServerFrames(t, starterFlow)
	if len(queuedEnd) != 1 {
		t.Fatalf("expected starter to receive one queued end frame, got %d", len(queuedEnd))
	}
	assertExchangeEndFrame(t, queuedEnd[0], "starter queued cancel response")

	replayOut, err := starterFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected exchange-cancel replay packet error: %v", err)
	}
	if len(replayOut) != 0 {
		t.Fatalf("expected replayed exchange cancel to emit no frames, got %d", len(replayOut))
	}

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-cancel-starter", owner, "starter exchange cancel")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-cancel-target", peer, "target exchange cancel")
}

func TestGameRuntimeItemExchangeCloseQueuesPeerEndWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeCloseStarter", 0x01030769, 0x02040769, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 710, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeCloseTarget", 0x0103076a, 0x0204076a, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 711, Vnum: 27002, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "item-exchange-close-starter", 0x70707069, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-close-target", 0x7070706a, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-close-starter", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange close starter account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-close-target", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange close target account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected exchange-close runtime error: %v", err)
	}
	starterFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-close-starter", 0x70707069)
	defer closeSessionFlow(t, starterFlow)
	targetFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-close-target", 0x7070706a)
	_ = flushServerFrames(t, starterFlow)
	_ = flushServerFrames(t, targetFlow)

	startOut, err := starterFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderStart,
		Arg1:      peer.VID,
	})))
	if err != nil {
		t.Fatalf("unexpected exchange-start before close error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange starter to receive one start frame before close, got %d", len(startOut))
	}
	if queuedStart := flushServerFrames(t, targetFlow); len(queuedStart) != 1 {
		t.Fatalf("expected exchange target to receive one queued start frame before close, got %d", len(queuedStart))
	}

	closeSessionFlow(t, targetFlow)
	queuedEnd := flushServerFrames(t, starterFlow)
	if len(queuedEnd) < 1 {
		t.Fatalf("expected starter to receive queued frames after target close, got %d", len(queuedEnd))
	}
	assertExchangeEndFrame(t, queuedEnd[0], "starter queued close response")

	assertExchangeAccountUnchanged(t, accounts, "item-exchange-close-starter", owner, "starter exchange close")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-close-target", peer, "target exchange close")
}

func TestGameRuntimeItemExchangeSlashLifecycleCommandsCloseSelfAndPeerWindowsWithoutMutation(t *testing.T) {
	cases := []struct {
		name        string
		command     string
		wantCommand string
		wantPhase   session.Phase
		ownerLogin  string
		peerLogin   string
		ownerName   string
		peerName    string
		ownerID     uint32
		ownerVID    uint32
		peerID      uint32
		peerVID     uint32
	}{
		{
			name:        "quit",
			command:     "/quit",
			wantCommand: "quit",
			ownerLogin:  "ex-slash-quit-owner",
			peerLogin:   "ex-slash-quit-peer",
			ownerName:   "ExchangeQuitOwner",
			peerName:    "ExchangeQuitPeer",
			ownerID:     0x010307b1,
			ownerVID:    0x020407b1,
			peerID:      0x010307b2,
			peerVID:     0x020407b2,
		},
		{
			name:       "logout",
			command:    "/logout",
			wantPhase:  session.PhaseClose,
			ownerLogin: "ex-slash-logout-owner",
			peerLogin:  "ex-slash-logout-peer",
			ownerName:  "ExchangeLogoutOwner",
			peerName:   "ExchangeLogoutPeer",
			ownerID:    0x010307b3,
			ownerVID:   0x020407b3,
			peerID:     0x010307b4,
			peerVID:    0x020407b4,
		},
		{
			name:       "phase_select",
			command:    "/phase_select",
			wantPhase:  session.PhaseSelect,
			ownerLogin: "ex-slash-select-owner",
			peerLogin:  "ex-slash-select-peer",
			ownerName:  "ExchangeSelectOwner",
			peerName:   "ExchangeSelectPeer",
			ownerID:    0x010307b5,
			ownerVID:   0x020407b5,
			peerID:     0x010307b6,
			peerVID:    0x020407b6,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter(tc.ownerName, tc.ownerID, tc.ownerVID, 1100, 2100, 0, 101, 201)
			owner.Gold = 12345
			owner.Inventory = []inventory.ItemInstance{{ID: uint64(tc.ownerID), Vnum: 27001, Count: 3, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			peer := peerVisibilityCharacter(tc.peerName, tc.peerID, tc.peerVID, 1120, 2120, 0, 101, 201)
			peer.Gold = 22222
			peer.Inventory = []inventory.ItemInstance{{ID: uint64(tc.peerID), Vnum: 27002, Count: 2, Slot: 6}}
			peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
			issuePeerTicket(t, ticketStore, tc.ownerLogin, 0x707070b1, owner)
			issuePeerTicket(t, ticketStore, tc.peerLogin, 0x707070b2, peer)
			if err := accounts.Save(accountstore.Account{Login: tc.ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s exchange slash owner account: %v", tc.name, err)
			}
			if err := accounts.Save(accountstore.Account{Login: tc.peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
				t.Fatalf("seed %s exchange slash peer account: %v", tc.name, err)
			}
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("unexpected %s exchange slash runtime error: %v", tc.name, err)
			}
			ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.ownerLogin, 0x707070b1)
			defer closeSessionFlow(t, ownerFlow)
			peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.peerLogin, 0x707070b2)
			defer closeSessionFlow(t, peerFlow)
			_ = flushServerFrames(t, ownerFlow)
			_ = flushServerFrames(t, peerFlow)

			startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
			if err != nil {
				t.Fatalf("unexpected %s exchange start error: %v", tc.name, err)
			}
			if len(startOut) != 1 {
				t.Fatalf("expected %s exchange start to emit one owner frame, got %d", tc.name, len(startOut))
			}
			assertExchangeStartFrame(t, startOut[0], peer.VID, tc.name+" owner start")
			queuedStart := flushServerFrames(t, peerFlow)
			if len(queuedStart) != 1 {
				t.Fatalf("expected %s exchange peer start frame, got %d", tc.name, len(queuedStart))
			}
			assertExchangeStartFrame(t, queuedStart[0], owner.VID, tc.name+" peer start")

			out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: tc.command})))
			if err != nil {
				t.Fatalf("unexpected %s exchange slash command error: %v", tc.name, err)
			}
			if len(out) != 2 {
				t.Fatalf("expected %s exchange slash command to emit exchange END plus lifecycle frame, got %d", tc.name, len(out))
			}
			assertExchangeEndFrame(t, out[0], tc.name+" self slash close")
			if tc.wantCommand != "" {
				delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
				if err != nil {
					t.Fatalf("decode %s slash command delivery after exchange close: %v", tc.name, err)
				}
				if delivery.Type != chatproto.ChatTypeCommand || delivery.Message != tc.wantCommand {
					t.Fatalf("unexpected %s slash command delivery after exchange close: %+v", tc.name, delivery)
				}
			} else {
				phase, err := control.DecodePhase(decodeSingleFrame(t, out[1]))
				if err != nil {
					t.Fatalf("decode %s phase after exchange close: %v", tc.name, err)
				}
				if phase.Phase != tc.wantPhase {
					t.Fatalf("expected %s to transition to phase %q after exchange close, got %q", tc.name, tc.wantPhase, phase.Phase)
				}
			}

			queuedEnd := flushServerFrames(t, peerFlow)
			if len(queuedEnd) < 1 {
				t.Fatalf("expected %s exchange peer to receive queued close frames, got %d", tc.name, len(queuedEnd))
			}
			assertExchangeEndFrame(t, queuedEnd[0], tc.name+" peer queued slash close")

			assertExchangeAccountUnchanged(t, accounts, tc.ownerLogin, owner, tc.name+" exchange slash close owner")
			assertExchangeAccountUnchanged(t, accounts, tc.peerLogin, peer, tc.name+" exchange slash close peer")
		})
	}
}

func TestGameRuntimeItemUseClosesActiveExchangeShellBeforeUseFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeUseOwner", 0x01030794, 0x02040794, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	owner.Inventory = []inventory.ItemInstance{
		{ID: 761, Vnum: 27044, Count: 3, Slot: 5},
		{ID: 762, Vnum: 27054, Count: 1, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeUsePeer", 0x01030795, 0x02040795, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "item-exchange-use-owner"
	peerLogin := "item-exchange-use-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707094, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707095, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange item-use owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange item-use peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:      27044,
		Name:      "Exchange Use Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "exchange use consumed"},
	}
	displayTemplate := itemcatalog.Template{Vnum: 27054, Name: "Exchange Use Display Decoy", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template, displayTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange item-use runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707094)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707095)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange item-use start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange item-use start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange item-use owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange item-use peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange item-use peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected exchange item-use item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected exchange item-use item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[1], displayTemplate, "exchange item-use owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected exchange item-use peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[1], displayTemplate, "exchange item-use peer item-add")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected active-exchange item-use packet error: %v", err)
	}
	if len(out) != 5 {
		t.Fatalf("expected active-exchange ITEM_USE to emit exchange end plus use echo, point, item update, and info chat, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "active-exchange item-use self close")
	useEcho, err := itemproto.DecodeUse(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode active-exchange item-use echo: %v", err)
	}
	if useEcho.Position != itemproto.InventoryPosition(5) || useEcho.CharacterVID != owner.VID || useEcho.VictimVID != owner.VID || useEcho.Vnum != template.Vnum {
		t.Fatalf("unexpected active-exchange item-use echo: %+v", useEcho)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode active-exchange item-use point-change: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != 50 || pointChange.Value != 75 {
		t.Fatalf("unexpected active-exchange item-use point-change: %+v", pointChange)
	}
	itemUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode active-exchange item-use item update: %v", err)
	}
	if itemUpdate.Position != itemproto.InventoryPosition(5) || itemUpdate.Count != 2 {
		t.Fatalf("unexpected active-exchange item-use update: %+v", itemUpdate)
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[4]))
	if err != nil {
		t.Fatalf("decode active-exchange item-use info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != template.UseEffect.Message {
		t.Fatalf("unexpected active-exchange item-use info chat: %+v", infoChat)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected active-exchange ITEM_USE to queue one peer close frame, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange item-use peer close")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-item-use exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-item-use exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after post-item-use exchange cancel, got %d", len(queued))
	}

	slashStartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange slash-use start error: %v", err)
	}
	if len(slashStartOut) != 1 {
		t.Fatalf("expected exchange slash-use start to emit one owner frame, got %d", len(slashStartOut))
	}
	assertExchangeStartFrame(t, slashStartOut[0], peer.VID, "exchange slash-use owner start")
	slashQueuedStart := flushServerFrames(t, peerFlow)
	if len(slashQueuedStart) != 1 {
		t.Fatalf("expected exchange slash-use peer start frame, got %d", len(slashQueuedStart))
	}
	assertExchangeStartFrame(t, slashQueuedStart[0], owner.VID, "exchange slash-use peer start")

	currentDisplay := inventory.ItemInstance{ID: 762, Vnum: 27054, Count: 1, Slot: 6}
	slashItemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 8, Position: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected exchange slash-use item-add error: %v", err)
	}
	if len(slashItemAddOut) != 1 {
		t.Fatalf("expected exchange slash-use item-add to emit one owner frame, got %d", len(slashItemAddOut))
	}
	assertExchangeItemAddFrame(t, slashItemAddOut[0], 1, 8, currentDisplay, displayTemplate, "exchange slash-use owner item-add")
	slashQueuedItemAdd := flushServerFrames(t, peerFlow)
	if len(slashQueuedItemAdd) != 1 {
		t.Fatalf("expected exchange slash-use peer item-add frame, got %d", len(slashQueuedItemAdd))
	}
	assertExchangeItemAddFrame(t, slashQueuedItemAdd[0], 0, 8, currentDisplay, displayTemplate, "exchange slash-use peer item-add")

	slashOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/use_item 5"})))
	if err != nil {
		t.Fatalf("unexpected active-exchange slash item-use error: %v", err)
	}
	if len(slashOut) != 4 {
		t.Fatalf("expected active-exchange slash /use_item to emit exchange end plus point, item update, and info chat, got %d", len(slashOut))
	}
	assertExchangeEndFrame(t, slashOut[0], "active-exchange slash item-use self close")
	slashPointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, slashOut[1]))
	if err != nil {
		t.Fatalf("decode active-exchange slash item-use point-change: %v", err)
	}
	if slashPointChange.VID != owner.VID || slashPointChange.Type != bootstrapPlayerPointType || slashPointChange.Amount != 50 || slashPointChange.Value != 125 {
		t.Fatalf("unexpected active-exchange slash item-use point-change: %+v", slashPointChange)
	}
	slashItemUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, slashOut[2]))
	if err != nil {
		t.Fatalf("decode active-exchange slash item-use item update: %v", err)
	}
	if slashItemUpdate.Position != itemproto.InventoryPosition(5) || slashItemUpdate.Count != 1 {
		t.Fatalf("unexpected active-exchange slash item-use update: %+v", slashItemUpdate)
	}
	slashInfoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, slashOut[3]))
	if err != nil {
		t.Fatalf("decode active-exchange slash item-use info chat: %v", err)
	}
	if slashInfoChat.Type != chatproto.ChatTypeInfo || slashInfoChat.VID != 0 || slashInfoChat.Message != template.UseEffect.Message {
		t.Fatalf("unexpected active-exchange slash item-use info chat: %+v", slashInfoChat)
	}
	slashQueuedClose := flushServerFrames(t, peerFlow)
	if len(slashQueuedClose) != 1 {
		t.Fatalf("expected active-exchange slash /use_item to queue one peer close frame, got %d", len(slashQueuedClose))
	}
	assertExchangeEndFrame(t, slashQueuedClose[0], "active-exchange slash item-use peer close")

	persistedOwner, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange item-use owner account: %v", err)
	}
	wantOwner := owner
	wantOwner.Points[bootstrapPlayerPointValueIndex] = 125
	wantOwner.Inventory = []inventory.ItemInstance{
		{ID: 761, Vnum: 27044, Count: 1, Slot: 5},
		{ID: 762, Vnum: 27054, Count: 1, Slot: 6},
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Inventory, wantOwner.Inventory) {
		t.Fatalf("active-exchange item-use persisted inventory got %+v want %+v", persistedOwner.Characters[0].Inventory, wantOwner.Inventory)
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Quickslots, wantOwner.Quickslots) {
		t.Fatalf("active-exchange item-use persisted quickslots got %+v want %+v", persistedOwner.Characters[0].Quickslots, wantOwner.Quickslots)
	}
	if persistedOwner.Characters[0].Points[bootstrapPlayerPointValueIndex] != wantOwner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("active-exchange item-use persisted point got %d want %d", persistedOwner.Characters[0].Points[bootstrapPlayerPointValueIndex], wantOwner.Points[bootstrapPlayerPointValueIndex])
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange item-use")
}

func TestGameRuntimeItemUseOfDisplayedExchangeItemFailsClosedWithoutClosingShell(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeDispUseOwner", 0x010307d1, 0x020407d1, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	owner.Inventory = []inventory.ItemInstance{{ID: 790, Vnum: 27047, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeDispUsePeer", 0x010307d2, 0x020407d2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "item-exchange-disp-use-a"
	peerLogin := "item-exchange-disp-use-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070d2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed displayed-exchange item-use owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed displayed-exchange item-use peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:      27047,
		Name:      "Displayed Exchange Use Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "displayed exchange use consumed"},
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected displayed-exchange item-use runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070d2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange item-use start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected displayed-exchange item-use start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "displayed-exchange item-use owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected displayed-exchange item-use peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "displayed-exchange item-use peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange item-use item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected displayed-exchange item-use item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], template, "displayed-exchange item-use owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected displayed-exchange item-use peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], template, "displayed-exchange item-use peer item-add")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange item-use packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected displayed exchange item use to fail closed with no frames while the shell stays open, got %d", len(out))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected displayed exchange item use to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "displayed-exchange item-use owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "displayed-exchange item-use peer")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange cancel after locked use: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected displayed-exchange shell to remain cancellable after locked use, got %d frames", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "displayed-exchange locked-use owner cancel")
	queuedCancel := flushServerFrames(t, peerFlow)
	if len(queuedCancel) != 1 {
		t.Fatalf("expected displayed-exchange locked-use peer cancel END, got %d", len(queuedCancel))
	}
	assertExchangeEndFrame(t, queuedCancel[0], "displayed-exchange locked-use peer cancel")
}

func TestGameRuntimeDisplayedExchangeItemMutationsFailClosedWithoutClosingShell(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeDispMutOwner", 0x010307f1, 0x020407f1, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{
		{ID: 901, Vnum: 27061, Count: 3, Slot: 5},
		{ID: 902, Vnum: 27061, Count: 1, Slot: 8},
		{ID: 903, Vnum: 11201, Count: 1, Slot: 9},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeDispMutPeer", 0x010307f2, 0x020407f2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "item-exchange-disp-mut-a"
	peerLogin := "item-exchange-disp-mut-b"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070f1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070f2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed displayed-exchange mutation owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed displayed-exchange mutation peer account: %v", err)
	}
	stackTemplate := itemcatalog.Template{Vnum: 27061, Name: "Displayed Exchange Stack Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 50}
	equipTemplate := itemcatalog.Template{Vnum: 11201, Name: "Displayed Exchange Sword", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String()}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{stackTemplate, equipTemplate})
	merchantDefinition := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:exchange_disp_mut_merchant",
		Title: "Exchange Display Mutation Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27061, Price: 50, Count: 1},
		},
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{merchantDefinition})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected displayed-exchange mutation runtime error: %v", err)
	}
	merchant, ok := runtime.RegisterStaticActorWithInteraction("ExchangeDispMutMerchant", bootstrapMapIndex, 1200, 2200, 20301, interactionstore.KindShopPreview, merchantDefinition.Ref)
	if !ok {
		t.Fatal("expected displayed-exchange merchant registration to succeed")
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070f1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070f2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange mutation start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected displayed-exchange mutation start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "displayed-exchange mutation owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected displayed-exchange mutation peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "displayed-exchange mutation peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange mutation item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected displayed-exchange mutation item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], stackTemplate, "displayed-exchange mutation owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected displayed-exchange mutation peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], stackTemplate, "displayed-exchange mutation peer item-add")

	assertNoFrames := func(label string, out [][]byte) {
		t.Helper()
		if len(out) != 0 {
			t.Fatalf("expected %s to fail closed with no frames while the shell stays open, got %d", label, len(out))
		}
		if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
			t.Fatalf("expected %s to queue no peer frames, got %d", label, len(queued))
		}
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: itemproto.InventoryPosition(5), Destination: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange locked move error: %v", err)
	}
	assertNoFrames("displayed exchange item move", out)

	out, err = ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(8)})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange locked use-to-item error: %v", err)
	}
	assertNoFrames("displayed exchange item use-to-item", out)

	out, err = ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange locked drop error: %v", err)
	}
	assertNoFrames("displayed exchange item drop", out)

	merchantOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(merchant.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange merchant open error: %v", err)
	}
	if len(merchantOut) != 1 {
		t.Fatalf("expected displayed-exchange merchant open to emit one shop start frame, got %d", len(merchantOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected displayed-exchange merchant open to queue no peer frames, got %d", len(queued))
	}

	out, err = ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell2(shopproto.ClientSell2Packet{Slot: 5, Count: 1})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange locked sell error: %v", err)
	}
	assertNoFrames("displayed exchange merchant sell", out)

	weaponPosition, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build displayed-exchange weapon position: %v", err)
	}
	equipAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 8, Position: itemproto.InventoryPosition(9)})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange equip-item add error: %v", err)
	}
	if len(equipAddOut) != 1 {
		t.Fatalf("expected displayed-exchange equip-item add to emit one owner frame, got %d", len(equipAddOut))
	}
	assertExchangeItemAddFrame(t, equipAddOut[0], 1, 8, owner.Inventory[2], equipTemplate, "displayed-exchange equip owner item-add")
	queuedEquipAdd := flushServerFrames(t, peerFlow)
	if len(queuedEquipAdd) != 1 {
		t.Fatalf("expected displayed-exchange equip peer item-add frame, got %d", len(queuedEquipAdd))
	}
	assertExchangeItemAddFrame(t, queuedEquipAdd[0], 0, 8, owner.Inventory[2], equipTemplate, "displayed-exchange equip peer item-add")

	out, err = ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: itemproto.InventoryPosition(9), Destination: weaponPosition})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange locked equip error: %v", err)
	}
	assertNoFrames("displayed exchange item equip", out)

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "displayed-exchange mutation owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "displayed-exchange mutation peer")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange cancel after locked mutations: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected displayed-exchange shell to remain cancellable after locked mutations, got %d frames", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "displayed-exchange locked-mutation owner cancel")
	queuedCancel := flushServerFrames(t, peerFlow)
	if len(queuedCancel) != 1 {
		t.Fatalf("expected displayed-exchange locked-mutation peer cancel END, got %d", len(queuedCancel))
	}
	assertExchangeEndFrame(t, queuedCancel[0], "displayed-exchange locked-mutation peer cancel")
}

func TestGameRuntimeItemDropRejectTextClosesActiveExchangeShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeDropRejectOwner", 0x010307a1, 0x020407a1, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 771, Vnum: 27045, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeDropRejectPeer", 0x010307a2, 0x020407a2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "exchange-drop-reject-owner"
	peerLogin := "exchange-drop-reject-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070a1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070a2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange item-drop reject owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange item-drop reject peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:           27045,
		Name:           "Exchange Drop-Sealed Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiDrop:       true,
		DropRejectText: "The seal prevents dropping this item while trading.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange item-drop reject runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070a1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070a2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange item-drop reject start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange item-drop reject start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange item-drop reject owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange item-drop reject peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange item-drop reject peer start")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected active-exchange item-drop reject packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected active-exchange item-drop reject to emit exchange END plus info chat, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "active-exchange item-drop reject self close")
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode active-exchange item-drop reject info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != template.DropRejectText {
		t.Fatalf("unexpected active-exchange item-drop reject info chat: %+v", infoChat)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected active-exchange item-drop reject to queue one peer close frame, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange item-drop reject peer close")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-drop-reject exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-drop-reject exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after post-drop-reject exchange cancel, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "owner active-exchange item-drop reject")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange item-drop reject")
}

func TestGameRuntimeItemUseToItemClosesActiveExchangeShellBeforeStackMergeFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeUseToItemOwner", 0x010307b1, 0x020407b1, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{
		{ID: 781, Vnum: 27046, Count: 2, Slot: 5},
		{ID: 782, Vnum: 27046, Count: 198, Slot: 8},
		{ID: 783, Vnum: 27056, Count: 1, Slot: 9},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 8},
	}
	peer := peerVisibilityCharacter("ExchangeUseToItemPeer", 0x010307b2, 0x020407b2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "exchange-use2item-owner"
	peerLogin := "exchange-use2item-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070b1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070b2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange use-to-item owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange use-to-item peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27046, Name: "Exchange Stack Potion", Stackable: true, MaxCount: 200}
	displayTemplate := itemcatalog.Template{Vnum: 27056, Name: "Exchange Stack Display Decoy", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template, displayTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange use-to-item runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070b1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070b2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange use-to-item start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange use-to-item start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange use-to-item owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange use-to-item peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange use-to-item peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(9)})))
	if err != nil {
		t.Fatalf("unexpected exchange use-to-item item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected exchange use-to-item item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[2], displayTemplate, "exchange use-to-item owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected exchange use-to-item peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[2], displayTemplate, "exchange use-to-item peer item-add")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(8),
	})))
	if err != nil {
		t.Fatalf("unexpected active-exchange ITEM_USE_TO_ITEM error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected active-exchange ITEM_USE_TO_ITEM to emit exchange end plus item delete, target update, and source quickslot delete, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "active-exchange use-to-item self close")
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode active-exchange use-to-item item delete: %v", err)
	}
	if itemDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected active-exchange use-to-item item delete: %+v", itemDel)
	}
	itemUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode active-exchange use-to-item item update: %v", err)
	}
	if itemUpdate.Position != itemproto.InventoryPosition(8) || itemUpdate.Count != 200 {
		t.Fatalf("unexpected active-exchange use-to-item item update: %+v", itemUpdate)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode active-exchange use-to-item quickslot delete: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("unexpected active-exchange use-to-item quickslot delete: %+v", quickslotDel)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected active-exchange ITEM_USE_TO_ITEM to queue one peer close frame, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange use-to-item peer close")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-use-to-item exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-use-to-item exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after post-use-to-item exchange cancel, got %d", len(queued))
	}

	persistedOwner, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange use-to-item owner account: %v", err)
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Inventory, []inventory.ItemInstance{
		{ID: 782, Vnum: 27046, Count: 200, Slot: 8},
		{ID: 783, Vnum: 27056, Count: 1, Slot: 9},
	}) {
		t.Fatalf("active-exchange use-to-item persisted inventory got %+v", persistedOwner.Characters[0].Inventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 8},
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("active-exchange use-to-item persisted quickslots got %+v want %+v", persistedOwner.Characters[0].Quickslots, wantQuickslots)
	}
	if persistedOwner.Characters[0].Gold != owner.Gold {
		t.Fatalf("active-exchange use-to-item mutated persisted gold got %d want %d", persistedOwner.Characters[0].Gold, owner.Gold)
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange use-to-item")
}

func TestGameRuntimeItemMoveClosesActiveExchangeShellBeforeMoveFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeMoveOwner", 0x010307c3, 0x020407c3, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{
		{ID: 791, Vnum: 27047, Count: 1, Slot: 5},
		{ID: 792, Vnum: 27057, Count: 1, Slot: 9},
	}
	peer := peerVisibilityCharacter("ExchangeMovePeer", 0x010307c4, 0x020407c4, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "exchange-move-owner"
	peerLogin := "exchange-move-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070c3, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070c4, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange item-move owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange item-move peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27047, Name: "Exchange Move Potion", Stackable: true, MaxCount: 200}
	displayTemplate := itemcatalog.Template{Vnum: 27057, Name: "Exchange Move Display Decoy", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template, displayTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange item-move runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070c3)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070c4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange item-move start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange item-move start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange item-move owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange item-move peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange item-move peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(9)})))
	if err != nil {
		t.Fatalf("unexpected exchange item-move item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected exchange item-move item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[1], displayTemplate, "exchange item-move owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected exchange item-move peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[1], displayTemplate, "exchange item-move peer item-add")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: itemproto.InventoryPosition(5), Destination: itemproto.InventoryPosition(8)})))
	if err != nil {
		t.Fatalf("unexpected active-exchange ITEM_MOVE error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected active-exchange ITEM_MOVE to emit exchange end plus item delete and set frames, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "active-exchange item-move self close")
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode active-exchange item-move item delete: %v", err)
	}
	if itemDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected active-exchange item-move item delete: %+v", itemDel)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode active-exchange item-move item set: %v", err)
	}
	if itemSet.Position != itemproto.InventoryPosition(8) || itemSet.Vnum != template.Vnum || itemSet.Count != 1 {
		t.Fatalf("unexpected active-exchange item-move item set: %+v", itemSet)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected active-exchange ITEM_MOVE to queue one peer close frame, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange item-move peer close")
	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-item-move exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-item-move exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after post-item-move exchange cancel, got %d", len(queued))
	}
	persistedOwner, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange item-move owner account: %v", err)
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Inventory, []inventory.ItemInstance{
		{ID: 791, Vnum: 27047, Count: 1, Slot: 8},
		{ID: 792, Vnum: 27057, Count: 1, Slot: 9},
	}) {
		t.Fatalf("active-exchange item-move persisted inventory got %+v", persistedOwner.Characters[0].Inventory)
	}

	slashStartOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange slash item-move start error: %v", err)
	}
	if len(slashStartOut) != 1 {
		t.Fatalf("expected exchange slash item-move start to emit one owner frame, got %d", len(slashStartOut))
	}
	assertExchangeStartFrame(t, slashStartOut[0], peer.VID, "exchange slash item-move owner start")
	slashQueuedStart := flushServerFrames(t, peerFlow)
	if len(slashQueuedStart) != 1 {
		t.Fatalf("expected exchange slash item-move peer start frame, got %d", len(slashQueuedStart))
	}
	assertExchangeStartFrame(t, slashQueuedStart[0], owner.VID, "exchange slash item-move peer start")

	currentDisplay := inventory.ItemInstance{ID: 792, Vnum: 27057, Count: 1, Slot: 9}
	slashItemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 8, Position: itemproto.InventoryPosition(9)})))
	if err != nil {
		t.Fatalf("unexpected exchange slash item-move item-add error: %v", err)
	}
	if len(slashItemAddOut) != 1 {
		t.Fatalf("expected exchange slash item-move item-add to emit one owner frame, got %d", len(slashItemAddOut))
	}
	assertExchangeItemAddFrame(t, slashItemAddOut[0], 1, 8, currentDisplay, displayTemplate, "exchange slash item-move owner item-add")
	slashQueuedItemAdd := flushServerFrames(t, peerFlow)
	if len(slashQueuedItemAdd) != 1 {
		t.Fatalf("expected exchange slash item-move peer item-add frame, got %d", len(slashQueuedItemAdd))
	}
	assertExchangeItemAddFrame(t, slashQueuedItemAdd[0], 0, 8, currentDisplay, displayTemplate, "exchange slash item-move peer item-add")

	slashOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/inventory_move 8 6"})))
	if err != nil {
		t.Fatalf("unexpected active-exchange slash inventory move error: %v", err)
	}
	if len(slashOut) != 3 {
		t.Fatalf("expected active-exchange slash /inventory_move to emit exchange end plus item delete and set frames, got %d", len(slashOut))
	}
	assertExchangeEndFrame(t, slashOut[0], "active-exchange slash item-move self close")
	slashItemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, slashOut[1]))
	if err != nil {
		t.Fatalf("decode active-exchange slash item-move item delete: %v", err)
	}
	if slashItemDel.Position != itemproto.InventoryPosition(8) {
		t.Fatalf("unexpected active-exchange slash item-move item delete: %+v", slashItemDel)
	}
	slashItemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, slashOut[2]))
	if err != nil {
		t.Fatalf("decode active-exchange slash item-move item set: %v", err)
	}
	if slashItemSet.Position != itemproto.InventoryPosition(6) || slashItemSet.Vnum != template.Vnum || slashItemSet.Count != 1 {
		t.Fatalf("unexpected active-exchange slash item-move item set: %+v", slashItemSet)
	}
	slashQueuedClose := flushServerFrames(t, peerFlow)
	if len(slashQueuedClose) != 1 {
		t.Fatalf("expected active-exchange slash /inventory_move to queue one peer close frame, got %d", len(slashQueuedClose))
	}
	assertExchangeEndFrame(t, slashQueuedClose[0], "active-exchange slash item-move peer close")

	persistedOwner, err = accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange slash item-move owner account: %v", err)
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Inventory, []inventory.ItemInstance{
		{ID: 791, Vnum: 27047, Count: 1, Slot: 6},
		{ID: 792, Vnum: 27057, Count: 1, Slot: 9},
	}) {
		t.Fatalf("active-exchange slash item-move persisted inventory got %+v", persistedOwner.Characters[0].Inventory)
	}
	if persistedOwner.Characters[0].Gold != owner.Gold {
		t.Fatalf("active-exchange item-move mutated persisted gold got %d want %d", persistedOwner.Characters[0].Gold, owner.Gold)
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange item-move")
}

func TestGameRuntimeMerchantBuyClosesActiveExchangeShellBeforeBuyFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeBuyOwner", 0x010307cf, 0x020407cf, 1100, 2100, 0, 101, 201)
	owner.Gold = 125
	owner.Inventory = nil
	peer := peerVisibilityCharacter("ExchangeBuyPeer", 0x010307d0, 0x020407d0, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "item-exchange-buy-owner"
	peerLogin := "item-exchange-buy-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070cf, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070d0, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange merchant-buy owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange merchant-buy peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27049, Name: "Exchange Buy Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 5}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	merchantDefinition := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:exchange_buy_merchant",
		Title: "Exchange Buy Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27049, Price: 50, Count: 1},
		},
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{merchantDefinition})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange merchant-buy runtime error: %v", err)
	}
	merchant, ok := runtime.RegisterStaticActorWithInteraction("ExchangeBuyMerchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, merchantDefinition.Ref)
	if !ok {
		t.Fatal("expected exchange merchant static actor registration to succeed")
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070cf)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070d0)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange merchant-buy start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange merchant-buy start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange merchant-buy owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange merchant-buy peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange merchant-buy peer start")

	merchantOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(merchant.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected active-exchange merchant-buy interaction error: %v", err)
	}
	if len(merchantOut) != 1 {
		t.Fatalf("expected active-exchange merchant-buy interaction to emit one shop start frame, got %d", len(merchantOut))
	}
	shopStart, err := shopproto.DecodeServerStart(decodeSingleFrame(t, merchantOut[0]))
	if err != nil {
		t.Fatalf("decode active-exchange merchant-buy shop start: %v", err)
	}
	if shopStart.OwnerVID != uint32(merchant.EntityID) || shopStart.Items[0].Vnum != template.Vnum || shopStart.Items[0].DisplayPos != 0 {
		t.Fatalf("unexpected active-exchange merchant-buy shop start: %+v", shopStart)
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected active-exchange merchant buy packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected active-exchange SHOP BUY to emit exchange end plus item set, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "active-exchange merchant buy self close")
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode active-exchange merchant buy item set: %v", err)
	}
	if itemSet.Position != itemproto.InventoryPosition(0) || itemSet.Vnum != template.Vnum || itemSet.Count != 1 {
		t.Fatalf("unexpected active-exchange merchant buy item set: %+v", itemSet)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected active-exchange SHOP BUY to queue one peer close frame, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange merchant buy peer close")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-merchant-buy exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-merchant-buy exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after post-merchant-buy exchange cancel, got %d", len(queued))
	}

	persistedOwner, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange merchant-buy owner account: %v", err)
	}
	if len(persistedOwner.Characters[0].Inventory) != 1 || persistedOwner.Characters[0].Inventory[0].Vnum != template.Vnum || persistedOwner.Characters[0].Inventory[0].Count != 1 || persistedOwner.Characters[0].Inventory[0].Slot != 0 {
		t.Fatalf("active-exchange merchant-buy persisted inventory got %+v", persistedOwner.Characters[0].Inventory)
	}
	if persistedOwner.Characters[0].Gold != 75 {
		t.Fatalf("active-exchange merchant-buy persisted gold got %d want %d", persistedOwner.Characters[0].Gold, uint64(75))
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange merchant-buy")
}

func TestGameRuntimeMerchantSellClosesActiveExchangeShellBeforeSellFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeSellOwner", 0x010307d1, 0x020407d1, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{
		{ID: 801, Vnum: 27048, Count: 3, Slot: 5},
		{ID: 802, Vnum: 27058, Count: 1, Slot: 9},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeSellPeer", 0x010307d2, 0x020407d2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "item-exchange-sell-owner"
	peerLogin := "item-exchange-sell-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070d1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070d2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange merchant-sell owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange merchant-sell peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27048, Name: "Exchange Sell Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 50}
	displayTemplate := itemcatalog.Template{Vnum: 27058, Name: "Exchange Sell Display Decoy", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template, displayTemplate})
	merchantDefinition := interactionstore.Definition{
		Kind:  interactionstore.KindShopPreview,
		Ref:   "npc:exchange_sell_merchant",
		Title: "Exchange Sell Merchant",
		Catalog: []interactionstore.MerchantCatalogEntry{
			{Slot: 0, ItemVnum: 27048, Price: 50, Count: 1},
		},
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{merchantDefinition})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, interactionStore, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange merchant-sell runtime error: %v", err)
	}
	merchant, ok := runtime.RegisterStaticActorWithInteraction("ExchangeSellMerchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, merchantDefinition.Ref)
	if !ok {
		t.Fatal("expected exchange merchant static actor registration to succeed")
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070d1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070d2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange merchant-sell start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange merchant-sell start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange merchant-sell owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange merchant-sell peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange merchant-sell peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(9)})))
	if err != nil {
		t.Fatalf("unexpected exchange merchant-sell item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected exchange merchant-sell item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[1], displayTemplate, "exchange merchant-sell owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected exchange merchant-sell peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[1], displayTemplate, "exchange merchant-sell peer item-add")

	merchantOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(merchant.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected active-exchange merchant interaction error: %v", err)
	}
	if len(merchantOut) != 1 {
		t.Fatalf("expected active-exchange merchant interaction to emit one shop start frame, got %d", len(merchantOut))
	}
	shopStart, err := shopproto.DecodeServerStart(decodeSingleFrame(t, merchantOut[0]))
	if err != nil {
		t.Fatalf("decode active-exchange merchant shop start: %v", err)
	}
	if shopStart.OwnerVID != uint32(merchant.EntityID) || shopStart.Items[0].Vnum != template.Vnum || shopStart.Items[0].DisplayPos != 0 {
		t.Fatalf("unexpected active-exchange merchant shop start: %+v", shopStart)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected merchant open during exchange not to queue peer frames, got %d", len(queued))
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell2(shopproto.ClientSell2Packet{Slot: 5, Count: 2})))
	if err != nil {
		t.Fatalf("unexpected active-exchange merchant sell2 packet error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected active-exchange SHOP SELL2 to emit exchange end plus item update and gold point-change, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "active-exchange merchant sell self close")
	itemUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode active-exchange merchant sell item update: %v", err)
	}
	if itemUpdate.Position != itemproto.InventoryPosition(5) || itemUpdate.Count != 1 {
		t.Fatalf("unexpected active-exchange merchant sell item update: %+v", itemUpdate)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode active-exchange merchant sell point-change: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapGoldPointType || pointChange.Amount != 20 || pointChange.Value != 12365 {
		t.Fatalf("unexpected active-exchange merchant sell point-change: %+v", pointChange)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected active-exchange SHOP SELL2 to queue one peer close frame, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange merchant sell peer close")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-merchant-sell exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-merchant-sell exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after post-merchant-sell exchange cancel, got %d", len(queued))
	}

	persistedOwner, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange merchant-sell owner account: %v", err)
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Inventory, []inventory.ItemInstance{
		{ID: 801, Vnum: 27048, Count: 1, Slot: 5},
		{ID: 802, Vnum: 27058, Count: 1, Slot: 9},
	}) {
		t.Fatalf("active-exchange merchant-sell persisted inventory got %+v", persistedOwner.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("active-exchange merchant-sell persisted quickslots got %+v want %+v", persistedOwner.Characters[0].Quickslots, owner.Quickslots)
	}
	if persistedOwner.Characters[0].Gold != 12365 {
		t.Fatalf("active-exchange merchant-sell persisted gold got %d want %d", persistedOwner.Characters[0].Gold, uint64(12365))
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange merchant-sell")
}

func TestGameRuntimeItemDropClosesActiveExchangeShellBeforeDropFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeDropOwner", 0x01030796, 0x02040796, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{
		{ID: 762, Vnum: 27045, Count: 3, Slot: 5},
		{ID: 763, Vnum: 27059, Count: 1, Slot: 9},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeDropPeer", 0x01030797, 0x02040797, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "item-exchange-drop-owner"
	peerLogin := "item-exchange-drop-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707096, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707097, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange item-drop owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange item-drop peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27045, Name: "Exchange Drop Potion", Stackable: true, MaxCount: 200}
	displayTemplate := itemcatalog.Template{Vnum: 27059, Name: "Exchange Drop Display Decoy", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template, displayTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange item-drop runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707096)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707097)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange item-drop start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange item-drop start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange item-drop owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange item-drop peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange item-drop peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(9)})))
	if err != nil {
		t.Fatalf("unexpected exchange item-drop item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected exchange item-drop item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[1], displayTemplate, "exchange item-drop owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected exchange item-drop peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[1], displayTemplate, "exchange item-drop peer item-add")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected active-exchange item-drop packet error: %v", err)
	}
	if len(out) != 5 {
		t.Fatalf("expected active-exchange ITEM_DROP to emit exchange end plus item, quickslot, ground, and ownership frames, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "active-exchange item-drop self close")
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode active-exchange item-drop item del: %v", err)
	}
	if itemDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected active-exchange item-drop item del: %+v", itemDel)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode active-exchange item-drop quickslot del: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("unexpected active-exchange item-drop quickslot del: %+v", quickslotDel)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode active-exchange item-drop ground add: %v", err)
	}
	if ground.Vnum != owner.Inventory[0].Vnum || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected active-exchange item-drop ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[4]))
	if err != nil {
		t.Fatalf("decode active-exchange item-drop ownership: %v", err)
	}
	if ownership.VID != ground.VID || ownership.OwnerName != owner.Name {
		t.Fatalf("unexpected active-exchange item-drop ownership: %+v", ownership)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 3 {
		t.Fatalf("expected active-exchange ITEM_DROP to queue exchange close plus visible ground frames, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange item-drop peer close")
	queuedGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, queuedClose[1]))
	if err != nil {
		t.Fatalf("decode active-exchange item-drop peer ground add: %v", err)
	}
	if queuedGround.VID != ground.VID || queuedGround.Vnum != ground.Vnum {
		t.Fatalf("unexpected active-exchange item-drop peer ground add: %+v", queuedGround)
	}
	queuedOwnership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, queuedClose[2]))
	if err != nil {
		t.Fatalf("decode active-exchange item-drop peer ownership: %v", err)
	}
	if queuedOwnership.VID != ground.VID || queuedOwnership.OwnerName != owner.Name {
		t.Fatalf("unexpected active-exchange item-drop peer ownership: %+v", queuedOwnership)
	}

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-item-drop exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-item-drop exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after post-item-drop exchange cancel, got %d", len(queued))
	}

	persistedOwner, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange item-drop owner account: %v", err)
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Inventory, []inventory.ItemInstance{{ID: 763, Vnum: 27059, Count: 1, Slot: 9}}) {
		t.Fatalf("active-exchange item-drop persisted inventory got %+v", persistedOwner.Characters[0].Inventory)
	}
	if len(persistedOwner.Characters[0].Quickslots) != 0 {
		t.Fatalf("active-exchange item-drop persisted quickslots got %+v want empty", persistedOwner.Characters[0].Quickslots)
	}
	if persistedOwner.Characters[0].Gold != owner.Gold {
		t.Fatalf("active-exchange item-drop mutated persisted gold got %d want %d", persistedOwner.Characters[0].Gold, owner.Gold)
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange item-drop")
}

func TestGameRuntimeItemPickupClosesActiveExchangeShellBeforePickupFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangePickupOwner", 0x010307e5, 0x020407e5, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 813, Vnum: 27049, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangePickupPeer", 0x010307e6, 0x020407e6, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "item-exchange-pickup-owner"
	peerLogin := "item-exchange-pickup-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070e5, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070e6, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange pickup owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange pickup peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27049, Name: "Exchange Pickup Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070e5)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070e6)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	dropOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected exchange pickup setup drop error: %v", err)
	}
	if len(dropOut) != 4 {
		t.Fatalf("expected exchange pickup setup drop to emit item, quickslot, ground, and ownership frames, got %d", len(dropOut))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, dropOut[2]))
	if err != nil {
		t.Fatalf("decode exchange pickup setup ground add: %v", err)
	}
	if ground.Vnum != template.Vnum || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected exchange pickup setup ground add: %+v", ground)
	}
	if queuedDrop := flushServerFrames(t, peerFlow); len(queuedDrop) != 2 {
		t.Fatalf("expected peer to receive setup ground add/ownership, got %d", len(queuedDrop))
	}

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange pickup start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange pickup start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange pickup owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange pickup peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange pickup peer start")

	pickupOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: ground.VID})))
	if err != nil {
		t.Fatalf("unexpected active-exchange item pickup packet error: %v", err)
	}
	if len(pickupOut) != 4 {
		t.Fatalf("expected active-exchange item pickup to emit exchange end plus ground delete, item set, and item get, got %d", len(pickupOut))
	}
	assertExchangeEndFrame(t, pickupOut[0], "active-exchange item pickup self close")
	groundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, pickupOut[1]))
	if err != nil {
		t.Fatalf("decode active-exchange pickup ground del: %v", err)
	}
	if groundDel.VID != ground.VID {
		t.Fatalf("unexpected active-exchange pickup ground del: %+v", groundDel)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, pickupOut[2]))
	if err != nil {
		t.Fatalf("decode active-exchange pickup item set: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(5) || set.Vnum != template.Vnum || set.Count != 3 {
		t.Fatalf("unexpected active-exchange pickup item set: %+v", set)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, pickupOut[3]))
	if err != nil {
		t.Fatalf("decode active-exchange pickup item get: %v", err)
	}
	if get.Vnum != template.Vnum || get.Count != 3 || get.Arg != itemproto.GetArgNormal {
		t.Fatalf("unexpected active-exchange pickup item get: %+v", get)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 2 {
		t.Fatalf("expected active-exchange pickup to queue exchange close before peer ground delete, got %d frames", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange item pickup peer close")
	queuedGroundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, queuedClose[1]))
	if err != nil {
		t.Fatalf("decode active-exchange pickup peer ground del: %v", err)
	}
	if queuedGroundDel.VID != ground.VID {
		t.Fatalf("unexpected active-exchange pickup peer ground del: %+v", queuedGroundDel)
	}
	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-pickup exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-pickup exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}
	if runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatalf("expected active-exchange pickup to remove ground item %d", ground.VID)
	}
	persistedOwner, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange pickup owner account: %v", err)
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Inventory, []inventory.ItemInstance{{ID: 813, Vnum: 27049, Count: 3, Slot: 5}}) {
		t.Fatalf("active-exchange pickup persisted inventory got %+v", persistedOwner.Characters[0].Inventory)
	}
	if len(persistedOwner.Characters[0].Quickslots) != 0 {
		t.Fatalf("active-exchange pickup persisted quickslots got %+v want empty after drop", persistedOwner.Characters[0].Quickslots)
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange pickup")
}

func TestGameRuntimeItemPickupRejectTextClosesActiveExchangeShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangePickupRejectOwner", 0x010307e7, 0x020407e7, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 814, Vnum: 27050, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangePickupRejectPeer", 0x010307e8, 0x020407e8, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "ex-pick-reject-owner"
	peerLogin := "ex-pick-reject-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070e7, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070e8, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange pickup-reject owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange pickup-reject peer account: %v", err)
	}
	allowedTemplate := itemcatalog.Template{Vnum: 27050, Name: "Exchange Pickup Guard Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{allowedTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange pickup-reject runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070e7)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070e8)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	dropOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected exchange pickup-reject setup drop error: %v", err)
	}
	if len(dropOut) != 4 {
		t.Fatalf("expected exchange pickup-reject setup drop to emit item, quickslot, ground, and ownership frames, got %d", len(dropOut))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, dropOut[2]))
	if err != nil {
		t.Fatalf("decode exchange pickup-reject setup ground add: %v", err)
	}
	if queuedDrop := flushServerFrames(t, peerFlow); len(queuedDrop) != 2 {
		t.Fatalf("expected peer to receive pickup-reject setup ground add/ownership, got %d", len(queuedDrop))
	}

	guardedTemplate := itemcatalog.Template{
		Vnum:             allowedTemplate.Vnum,
		Name:             allowedTemplate.Name,
		Stackable:        true,
		MaxCount:         200,
		AntiGet:          true,
		PickupRejectText: "The seal prevents reclaiming this item while trading.",
	}
	runtime.itemTemplates[guardedTemplate.Vnum] = guardedTemplate
	runtime.sharedWorld.SetItemTemplates(runtime.itemTemplates)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange pickup-reject start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange pickup-reject start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange pickup-reject owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange pickup-reject peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange pickup-reject peer start")

	pickupOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: ground.VID})))
	if err != nil {
		t.Fatalf("unexpected active-exchange pickup rejection packet error: %v", err)
	}
	if len(pickupOut) != 2 {
		t.Fatalf("expected active-exchange pickup rejection to emit exchange end plus info chat, got %d", len(pickupOut))
	}
	assertExchangeEndFrame(t, pickupOut[0], "active-exchange pickup rejection self close")
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, pickupOut[1]))
	if err != nil {
		t.Fatalf("decode active-exchange pickup rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != guardedTemplate.PickupRejectText {
		t.Fatalf("unexpected active-exchange pickup rejection chat: %+v", delivery)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected active-exchange pickup rejection to queue only peer exchange close, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange pickup rejection peer close")
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatalf("expected active-exchange pickup rejection to leave ground item %d pending", ground.VID)
	}
	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-pickup-reject exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-pickup-reject exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}
	persistedOwner, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange pickup-reject owner account: %v", err)
	}
	if len(persistedOwner.Characters[0].Inventory) != 0 {
		t.Fatalf("active-exchange pickup rejection persisted inventory got %+v want empty after setup drop", persistedOwner.Characters[0].Inventory)
	}
	if len(persistedOwner.Characters[0].Quickslots) != 0 {
		t.Fatalf("active-exchange pickup rejection persisted quickslots got %+v want empty after setup drop", persistedOwner.Characters[0].Quickslots)
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange pickup rejection")
}

func TestGameRuntimeItemMoveEquipClosesActiveExchangeShellBeforeEquipFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeEquipOwner", 0x010307e1, 0x020407e1, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{
		{ID: 811, Vnum: 11200, Count: 1, Slot: 5},
		{ID: 812, Vnum: 27060, Count: 1, Slot: 9},
	}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeEquipPeer", 0x010307e2, 0x020407e2, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "item-exchange-equip-owner"
	peerLogin := "item-exchange-equip-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070e1, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070e2, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange equip owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange equip peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 11200, Name: "Exchange Equip Sword", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String()}
	displayTemplate := itemcatalog.Template{Vnum: 27060, Name: "Exchange Equip Display Decoy", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template, displayTemplate})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange equip runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070e1)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070e2)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange equip start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange equip start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange equip owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange equip peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange equip peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(9)})))
	if err != nil {
		t.Fatalf("unexpected exchange equip item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected exchange equip item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[1], displayTemplate, "exchange equip owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected exchange equip peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[1], displayTemplate, "exchange equip peer item-add")

	weaponPosition, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build exchange equip weapon position: %v", err)
	}
	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{Source: itemproto.InventoryPosition(5), Destination: weaponPosition})))
	if err != nil {
		t.Fatalf("unexpected active-exchange item equip packet error: %v", err)
	}
	if len(out) != 5 {
		t.Fatalf("expected active-exchange item equip to emit exchange end plus item delete, equipment set, update, and quickslot delete, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "active-exchange item equip self close")
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode active-exchange item equip inventory delete: %v", err)
	}
	if itemDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected active-exchange item equip inventory delete: %+v", itemDel)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode active-exchange item equip equipment set: %v", err)
	}
	if itemSet.Position != weaponPosition || itemSet.Vnum != template.Vnum || itemSet.Count != 1 {
		t.Fatalf("unexpected active-exchange item equip equipment set: %+v", itemSet)
	}
	selfUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode active-exchange item equip self update: %v", err)
	}
	if selfUpdate.VID != owner.VID {
		t.Fatalf("unexpected active-exchange item equip self update: %+v", selfUpdate)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[4]))
	if err != nil {
		t.Fatalf("decode active-exchange item equip quickslot delete: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("unexpected active-exchange item equip quickslot delete: %+v", quickslotDel)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 2 {
		t.Fatalf("expected active-exchange item equip to queue exchange close before peer appearance update, got %d frames", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange item equip peer close")
	peerUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, queuedClose[1]))
	if err != nil {
		t.Fatalf("decode active-exchange item equip peer update: %v", err)
	}
	if peerUpdate.VID != owner.VID {
		t.Fatalf("unexpected active-exchange item equip peer update: %+v", peerUpdate)
	}
	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-equip exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-equip exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}

	persistedOwner, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange equip owner account: %v", err)
	}
	if !reflect.DeepEqual(persistedOwner.Characters[0].Inventory, []inventory.ItemInstance{{ID: 812, Vnum: 27060, Count: 1, Slot: 9}}) {
		t.Fatalf("active-exchange equip persisted inventory got %+v", persistedOwner.Characters[0].Inventory)
	}
	if len(persistedOwner.Characters[0].Equipment) != 1 || persistedOwner.Characters[0].Equipment[0].Vnum != template.Vnum || persistedOwner.Characters[0].Equipment[0].EquipSlot != inventory.EquipmentSlotWeapon {
		t.Fatalf("active-exchange equip persisted equipment got %+v", persistedOwner.Characters[0].Equipment)
	}
	if len(persistedOwner.Characters[0].Quickslots) != 0 {
		t.Fatalf("active-exchange equip persisted quickslots got %+v want empty", persistedOwner.Characters[0].Quickslots)
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange equip")
}

func TestGameRuntimeSlashUnequipClosesActiveExchangeShellBeforeUnequipFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeUnequipOwner", 0x010307e3, 0x020407e3, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Equipment = []inventory.ItemInstance{{ID: 812, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	peer := peerVisibilityCharacter("ExchangeUnequipPeer", 0x010307e4, 0x020407e4, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	ownerLogin := "item-exchange-unequip-owner"
	peerLogin := "item-exchange-unequip-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x707070e3, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x707070e4, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed exchange unequip owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed exchange unequip peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 11200, Name: "Exchange Unequip Sword", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String()}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected exchange unequip runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x707070e3)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x707070e4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected exchange unequip start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected exchange unequip start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "exchange unequip owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected exchange unequip peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "exchange unequip peer start")

	weaponPosition, err := itemproto.EquipmentPosition(4)
	if err != nil {
		t.Fatalf("build exchange unequip weapon position: %v", err)
	}
	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{Type: chatproto.ChatTypeTalking, Message: "/unequip_item weapon 5"})))
	if err != nil {
		t.Fatalf("unexpected active-exchange slash unequip error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected active-exchange slash unequip to emit exchange end plus equipment delete, inventory set, and update, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "active-exchange slash unequip self close")
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode active-exchange slash unequip equipment delete: %v", err)
	}
	if itemDel.Position != weaponPosition {
		t.Fatalf("unexpected active-exchange slash unequip equipment delete: %+v", itemDel)
	}
	itemSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode active-exchange slash unequip inventory set: %v", err)
	}
	if itemSet.Position != itemproto.InventoryPosition(5) || itemSet.Vnum != template.Vnum || itemSet.Count != 1 {
		t.Fatalf("unexpected active-exchange slash unequip inventory set: %+v", itemSet)
	}
	selfUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode active-exchange slash unequip self update: %v", err)
	}
	if selfUpdate.VID != owner.VID {
		t.Fatalf("unexpected active-exchange slash unequip self update: %+v", selfUpdate)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 2 {
		t.Fatalf("expected active-exchange slash unequip to queue exchange close before peer appearance update, got %d frames", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "active-exchange slash unequip peer close")
	peerUpdate, err := worldproto.DecodeCharacterUpdate(decodeSingleFrame(t, queuedClose[1]))
	if err != nil {
		t.Fatalf("decode active-exchange slash unequip peer update: %v", err)
	}
	if peerUpdate.VID != owner.VID {
		t.Fatalf("unexpected active-exchange slash unequip peer update: %+v", peerUpdate)
	}
	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-unequip exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-unequip exchange cancel to emit no frames after the shell was closed, got %d", len(cancelOut))
	}

	persistedOwner, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load persisted active-exchange unequip owner account: %v", err)
	}
	if len(persistedOwner.Characters[0].Equipment) != 0 {
		t.Fatalf("active-exchange unequip persisted equipment got %+v want empty", persistedOwner.Characters[0].Equipment)
	}
	if len(persistedOwner.Characters[0].Inventory) != 1 || persistedOwner.Characters[0].Inventory[0].Vnum != template.Vnum || persistedOwner.Characters[0].Inventory[0].Slot != 5 {
		t.Fatalf("active-exchange unequip persisted inventory got %+v", persistedOwner.Characters[0].Inventory)
	}
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "peer active-exchange unequip")
}

func TestGameRuntimeItemExchangeTransferGuardItemAddReturnsAuthoredRejectTextInsideActiveShellWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		login     string
		peerLogin string
		mutate    func(*itemcatalog.Template)
		job       uint8
		raceNum   uint16
		level     uint8
		empire    uint8
	}{
		{name: "anti-stack", login: "item-exchange-anti-stack", peerLogin: "item-exchange-anti-stack-peer", mutate: func(template *itemcatalog.Template) { template.AntiStack = true }},
		{name: "anti-get", login: "item-exchange-anti-get", peerLogin: "item-exchange-anti-get-peer", mutate: func(template *itemcatalog.Template) { template.AntiGet = true }},
		{name: "anti-drop", login: "item-exchange-anti-drop", peerLogin: "item-exchange-anti-drop-peer", mutate: func(template *itemcatalog.Template) { template.AntiDrop = true }},
		{name: "anti-sell", login: "item-exchange-anti-sell", peerLogin: "item-exchange-anti-sell-peer", mutate: func(template *itemcatalog.Template) { template.AntiSell = true }},
		{name: "anti-warrior", login: "item-exchange-anti-war", peerLogin: "item-exchange-anti-war-peer", job: 0, raceNum: 0, mutate: func(template *itemcatalog.Template) { template.AntiWarrior = true }},
		{name: "anti-male", login: "item-exchange-anti-male", peerLogin: "item-exchange-anti-male-peer", job: 0, raceNum: 0, mutate: func(template *itemcatalog.Template) { template.AntiMale = true }},
		{name: "anti-empire-b", login: "item-exchange-anti-emp", peerLogin: "item-exchange-anti-emp-peer", empire: 2, mutate: func(template *itemcatalog.Template) { template.AntiEmpireB = true }},
		{name: "min-level", login: "item-exchange-min-level", peerLogin: "item-exchange-min-level-peer", level: 5, mutate: func(template *itemcatalog.Template) { template.MinLevel = 10 }},
	}
	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("ExchangeGuard"+tc.name, 0x01030861+uint32(index), 0x02040861+uint32(index), 1100, 2100, tc.raceNum, 101, 201)
			owner.Gold = 12345
			owner.Job = tc.job
			owner.RaceNum = tc.raceNum
			if tc.level != 0 {
				owner.Level = tc.level
			}
			if tc.empire != 0 {
				owner.Empire = tc.empire
			}
			owner.Inventory = []inventory.ItemInstance{{ID: 802 + uint64(index), Vnum: 27044, Count: 3, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			peer := peerVisibilityCharacter("ExchangeGuardPeer"+tc.name, 0x01030891+uint32(index), 0x02040891+uint32(index), 1120, 2120, 0, 101, 201)
			peer.Gold = 22222
			issuePeerTicket(t, ticketStore, tc.login, 0x70707161+uint32(index), owner)
			issuePeerTicket(t, ticketStore, tc.peerLogin, 0x70707191+uint32(index), peer)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s item-exchange account: %v", tc.name, err)
			}
			if err := accounts.Save(accountstore.Account{Login: tc.peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
				t.Fatalf("seed %s item-exchange peer account: %v", tc.name, err)
			}
			template := itemcatalog.Template{
				Vnum:           27044,
				Name:           "Guarded Exchange Potion",
				Stackable:      true,
				MaxCount:       200,
				GiveRejectText: "You cannot trade this item.",
			}
			tc.mutate(&template)
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected %s item-exchange runtime error: %v", tc.name, err)
			}
			ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, 0x70707161+uint32(index))
			defer closeSessionFlow(t, ownerFlow)
			peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.peerLogin, 0x70707191+uint32(index))
			defer closeSessionFlow(t, peerFlow)
			_ = flushServerFrames(t, ownerFlow)
			_ = flushServerFrames(t, peerFlow)

			startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
			if err != nil {
				t.Fatalf("unexpected %s exchange-start error: %v", tc.name, err)
			}
			if len(startOut) != 1 {
				t.Fatalf("expected %s exchange start to emit one owner frame, got %d", tc.name, len(startOut))
			}
			assertExchangeStartFrame(t, startOut[0], peer.VID, tc.name+" owner start")
			queuedStart := flushServerFrames(t, peerFlow)
			if len(queuedStart) != 1 {
				t.Fatalf("expected %s peer start frame, got %d", tc.name, len(queuedStart))
			}
			assertExchangeStartFrame(t, queuedStart[0], owner.VID, tc.name+" peer start")

			out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
				Subheader: itemproto.ExchangeSubheaderItemAdd,
				Arg2:      7,
				Position:  itemproto.InventoryPosition(5),
			})))
			if err != nil {
				t.Fatalf("unexpected %s item-exchange packet error: %v", tc.name, err)
			}
			if len(out) != 1 {
				t.Fatalf("expected %s EXCHANGE item-add to emit one info-chat frame, got %d", tc.name, len(out))
			}
			delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode %s item-exchange rejection info chat: %v", tc.name, err)
			}
			if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.GiveRejectText {
				t.Fatalf("unexpected %s item-exchange rejection chat: %+v", tc.name, delivery)
			}
			if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
				t.Fatalf("expected no queued peer frames after %s EXCHANGE rejection, got %d", tc.name, len(queued))
			}
			assertExchangeAccountUnchanged(t, accounts, tc.login, owner, tc.name+" EXCHANGE owner")
			assertExchangeAccountUnchanged(t, accounts, tc.peerLogin, peer, tc.name+" EXCHANGE peer")
		})
	}
}

func TestGameRuntimeItemExchangeAntiGiveItemAddReturnsAuthoredRejectTextInsideActiveShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeBound", 0x01030761, 0x02040761, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 702, Vnum: 27043, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeBoundPeer", 0x01030791, 0x02040791, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	issuePeerTicket(t, ticketStore, "item-exchange-bound", 0x70707061, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-bound-peer", 0x70707091, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed bound item-exchange account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-bound-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed bound item-exchange peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:           27043,
		Name:           "Bound Exchange Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot trade this item.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected bound item-exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-bound", 0x70707061)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-bound-peer", 0x70707091)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected anti-give exchange-start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected anti-give exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "anti-give owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected anti-give peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "anti-give peer start")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderItemAdd,
		Arg2:      7,
		Position:  itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected anti-give item-exchange packet error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected anti-give EXCHANGE item-add to emit one info-chat frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode anti-give item-exchange rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.GiveRejectText {
		t.Fatalf("unexpected anti-give item-exchange rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames after anti-give EXCHANGE rejection, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-bound", owner, "anti-give EXCHANGE owner")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-bound-peer", peer, "anti-give EXCHANGE peer")
}

func TestGameRuntimeItemExchangeAntiGiveRejectTextRequiresActiveShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeNoShellBound", 0x01030792, 0x02040792, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 750, Vnum: 27043, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-exchange-noshell-bound", 0x70707092, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-noshell-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed no-shell bound item-exchange account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:           27043,
		Name:           "Bound Exchange Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot trade this item.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected no-shell bound item-exchange runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-noshell-bound", 0x70707092)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderItemAdd,
		Arg2:      7,
		Position:  itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected no-shell anti-give item-exchange packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no-shell anti-give EXCHANGE item-add to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after no-shell anti-give EXCHANGE rejection, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-noshell-bound", owner, "no-shell anti-give EXCHANGE")
}

func TestGameRuntimeItemExchangeAntiGiveRejectTextRequiresValidDisplaySlotWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeSlotGuard", 0x01030762, 0x02040762, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 703, Vnum: 27043, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("ExchangeSlotGuardPeer", 0x01030793, 0x02040793, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	issuePeerTicket(t, ticketStore, "item-exchange-slot-guard", 0x70707062, owner)
	issuePeerTicket(t, ticketStore, "item-exchange-slot-guard-peer", 0x70707093, peer)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-slot-guard", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed display-slot guarded item-exchange account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-slot-guard-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed display-slot guarded item-exchange peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:           27043,
		Name:           "Bound Exchange Potion",
		Stackable:      true,
		MaxCount:       200,
		AntiGive:       true,
		GiveRejectText: "You cannot trade this item.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected display-slot guarded item-exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-slot-guard", 0x70707062)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-slot-guard-peer", 0x70707093)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected display-slot guarded exchange-start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected display-slot guarded exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "display-slot guarded owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected display-slot guarded peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "display-slot guarded peer start")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderItemAdd,
		Arg2:      12,
		Position:  itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected display-slot guarded item-exchange packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected out-of-range EXCHANGE display slot to suppress anti-give feedback, got %d frames", len(out))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames after out-of-range EXCHANGE display slot, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-slot-guard", owner, "out-of-range anti-give EXCHANGE owner")
	assertExchangeAccountUnchanged(t, accounts, "item-exchange-slot-guard-peer", peer, "out-of-range anti-give EXCHANGE peer")
}

func TestGameRuntimeItemExchangeFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeOwner", 0x01030760, 0x02040760, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 701, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-exchange-owner", 0x70707060, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-exchange account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected item-exchange runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-owner", 0x70707060)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderItemAdd,
		Arg2:      7,
		Position:  itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected item-exchange packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected unsupported EXCHANGE to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after unsupported EXCHANGE, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-exchange-owner")
	if err != nil {
		t.Fatalf("load persisted item-exchange account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("unsupported EXCHANGE mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("unsupported EXCHANGE mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Gold != owner.Gold {
		t.Fatalf("unsupported EXCHANGE mutated gold: got %d want %d", persisted.Characters[0].Gold, owner.Gold)
	}
}

func assertExchangeStartFrame(t *testing.T, raw []byte, peerVID uint32, context string) {
	t.Helper()
	packet, err := itemproto.DecodeServerExchange(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode exchange start frame %s: %v", context, err)
	}
	if packet.Subheader != itemproto.ExchangeServerSubheaderStart || packet.IsMe != 0 || packet.Arg1 != peerVID || packet.Position != (itemproto.Position{}) || packet.Arg3 != 0 {
		t.Fatalf("unexpected exchange start frame %s: %+v", context, packet)
	}
	if packet.Sockets != ([itemproto.ItemSocketCount]int32{}) || packet.Attributes != ([itemproto.ItemAttributeCount]itemproto.Attribute{}) {
		t.Fatalf("expected exchange start frame %s to carry empty item display fields, got sockets=%+v attrs=%+v", context, packet.Sockets, packet.Attributes)
	}
}

func assertExchangeEndFrame(t *testing.T, raw []byte, context string) {
	t.Helper()
	packet, err := itemproto.DecodeServerExchange(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode exchange end frame %s: %v", context, err)
	}
	if packet != (itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderEnd}) {
		t.Fatalf("unexpected exchange end frame %s: %+v", context, packet)
	}
}

func exchangeFrameIsEnd(t *testing.T, raw []byte) bool {
	t.Helper()
	packet, err := itemproto.DecodeServerExchange(decodeSingleFrame(t, raw))
	if err != nil {
		return false
	}
	return packet == (itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderEnd})
}

func assertExchangeAlreadyFrame(t *testing.T, raw []byte, context string) {
	t.Helper()
	packet, err := itemproto.DecodeServerExchange(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode exchange already frame %s: %v", context, err)
	}
	if packet != (itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderAlready}) {
		t.Fatalf("unexpected exchange already frame %s: %+v", context, packet)
	}
}

func assertExchangeItemDelFrame(t *testing.T, raw []byte, isMe uint8, displaySlot uint8, context string) {
	t.Helper()
	packet, err := itemproto.DecodeServerExchange(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode exchange item-del frame %s: %v", context, err)
	}
	if packet != (itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderItemDel, IsMe: isMe, Arg1: uint32(displaySlot)}) {
		t.Fatalf("unexpected exchange item-del frame %s: %+v", context, packet)
	}
}

func assertExchangeGoldAddFrame(t *testing.T, raw []byte, isMe uint8, gold uint32, context string) {
	t.Helper()
	packet, err := itemproto.DecodeServerExchange(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode exchange gold-add frame %s: %v", context, err)
	}
	if packet != (itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderGoldAdd, IsMe: isMe, Arg1: gold}) {
		t.Fatalf("unexpected exchange gold-add frame %s: %+v", context, packet)
	}
}

func assertExchangeAcceptFrame(t *testing.T, raw []byte, isMe uint8, context string) {
	t.Helper()
	assertExchangeAcceptFrameWithValue(t, raw, isMe, 1, context)
}

func assertExchangeAcceptFrameWithValue(t *testing.T, raw []byte, isMe uint8, value uint32, context string) {
	t.Helper()
	packet, err := itemproto.DecodeServerExchange(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode exchange accept frame %s: %v", context, err)
	}
	if packet != (itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderAccept, IsMe: isMe, Arg1: value}) {
		t.Fatalf("unexpected exchange accept frame %s: %+v", context, packet)
	}
}

func assertExchangeLessGoldFrame(t *testing.T, raw []byte, context string) {
	t.Helper()
	packet, err := itemproto.DecodeServerExchange(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode exchange less-gold frame %s: %v", context, err)
	}
	if packet != (itemproto.ServerExchangePacket{Subheader: itemproto.ExchangeServerSubheaderLessGold}) {
		t.Fatalf("unexpected exchange less-gold frame %s: %+v", context, packet)
	}
}

func assertExchangeItemAddFrame(t *testing.T, raw []byte, isMe uint8, displaySlot uint8, item inventory.ItemInstance, template itemcatalog.Template, context string) {
	t.Helper()
	packet, err := itemproto.DecodeServerExchange(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode exchange item-add frame %s: %v", context, err)
	}
	if packet.Subheader != itemproto.ExchangeServerSubheaderItemAdd || packet.IsMe != isMe || packet.Arg1 != item.Vnum || packet.Position != (itemproto.Position{WindowType: itemproto.WindowReserved, Cell: uint16(displaySlot)}) || packet.Arg3 != uint32(item.Count) {
		t.Fatalf("unexpected exchange item-add frame %s: %+v", context, packet)
	}
	if packet.Sockets != ([itemproto.ItemSocketCount]int32(template.Sockets)) {
		t.Fatalf("expected exchange item-add frame %s sockets %+v, got %+v", context, template.Sockets, packet.Sockets)
	}
	if packet.Attributes != bootstrapItemAttributes(template) {
		t.Fatalf("expected exchange item-add frame %s attributes %+v, got %+v", context, bootstrapItemAttributes(template), packet.Attributes)
	}
}

func assertExchangeAccountUnchanged(t *testing.T, accounts accountstore.Store, login string, want loginticket.Character, context string) {
	t.Helper()
	persisted, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load persisted %s account: %v", context, err)
	}
	if len(persisted.Characters) != 1 {
		t.Fatalf("expected one persisted %s character, got %d", context, len(persisted.Characters))
	}
	got := persisted.Characters[0]
	if !sameExchangeInventory(got.Inventory, want.Inventory) {
		t.Fatalf("%s mutated inventory: got %+v want %+v", context, got.Inventory, want.Inventory)
	}
	if !sameExchangeQuickslots(got.Quickslots, want.Quickslots) {
		t.Fatalf("%s mutated quickslots: got %+v want %+v", context, got.Quickslots, want.Quickslots)
	}
	if !sameExchangeInventory(got.Equipment, want.Equipment) {
		t.Fatalf("%s mutated equipment: got %+v want %+v", context, got.Equipment, want.Equipment)
	}
	if got.Gold != want.Gold {
		t.Fatalf("%s mutated gold: got %d want %d", context, got.Gold, want.Gold)
	}
}

func assertExchangeLiveStateUnchanged(t *testing.T, runtime *gameRuntime, want loginticket.Character, context string) {
	t.Helper()
	currency, ok := runtime.CurrencySnapshot(want.Name)
	if !ok {
		t.Fatalf("expected currency snapshot for %s", context)
	}
	if currency.Gold != want.Gold {
		t.Fatalf("%s mutated live gold: got %d want %d", context, currency.Gold, want.Gold)
	}
	inventorySnapshot, ok := runtime.InventorySnapshot(want.Name)
	if !ok {
		t.Fatalf("expected inventory snapshot for %s", context)
	}
	gotInventory := make([]inventory.ItemInstance, 0, len(inventorySnapshot.Inventory))
	for _, item := range inventorySnapshot.Inventory {
		gotInventory = append(gotInventory, inventory.ItemInstance{ID: item.ID, Vnum: item.Vnum, Count: item.Count, Slot: inventory.SlotIndex(item.Slot), Locked: item.Locked})
	}
	if !sameExchangeInventory(gotInventory, want.Inventory) {
		t.Fatalf("%s mutated live inventory: got %+v want %+v", context, gotInventory, want.Inventory)
	}
	quickslots, ok := runtime.QuickslotsSnapshot(want.Name)
	if !ok {
		t.Fatalf("expected quickslot snapshot for %s", context)
	}
	gotQuickslots := make([]loginticket.Quickslot, 0, len(quickslots.Quickslots))
	for _, slot := range quickslots.Quickslots {
		gotQuickslots = append(gotQuickslots, loginticket.Quickslot{Position: slot.Position, Type: slot.Type, Slot: slot.Slot})
	}
	if !sameExchangeQuickslots(gotQuickslots, want.Quickslots) {
		t.Fatalf("%s mutated live quickslots: got %+v want %+v", context, gotQuickslots, want.Quickslots)
	}
}

type exchangeFinalizeFailingAccountStore struct {
	accounts  map[string]accountstore.Account
	originals map[string]accountstore.Account
	failLogin string
}

func newExchangeFinalizeFailingAccountStore(failLogin string, accounts ...accountstore.Account) *exchangeFinalizeFailingAccountStore {
	cloned := make(map[string]accountstore.Account, len(accounts))
	originals := make(map[string]accountstore.Account, len(accounts))
	for _, account := range accounts {
		copyAccount := account
		copyAccount.Characters = cloneCharacters(account.Characters)
		cloned[account.Login] = copyAccount
		original := account
		original.Characters = cloneCharacters(account.Characters)
		originals[account.Login] = original
	}
	return &exchangeFinalizeFailingAccountStore{accounts: cloned, originals: originals, failLogin: failLogin}
}

func (s *exchangeFinalizeFailingAccountStore) Load(login string) (accountstore.Account, error) {
	account, ok := s.accounts[login]
	if !ok {
		return accountstore.Account{}, accountstore.ErrAccountNotFound
	}
	return accountstore.Account{Login: account.Login, Empire: account.Empire, Characters: cloneCharacters(account.Characters)}, nil
}

func (s *exchangeFinalizeFailingAccountStore) Save(account accountstore.Account) error {
	if account.Login == s.failLogin {
		original, ok := s.originals[account.Login]
		if ok && exchangeAccountTradeStateMutated(original, account) {
			return errors.New("forced exchange finalize persistence failure")
		}
	}
	s.accounts[account.Login] = accountstore.Account{Login: account.Login, Empire: account.Empire, Characters: cloneCharacters(account.Characters)}
	return nil
}

func exchangeAccountTradeStateMutated(original accountstore.Account, updated accountstore.Account) bool {
	if len(original.Characters) != len(updated.Characters) {
		return true
	}
	for i := range original.Characters {
		left := original.Characters[i]
		right := updated.Characters[i]
		if left.Gold != right.Gold {
			return true
		}
		if !sameExchangeInventory(left.Inventory, right.Inventory) {
			return true
		}
		if !sameExchangeQuickslots(left.Quickslots, right.Quickslots) {
			return true
		}
		if !sameExchangeInventory(left.Equipment, right.Equipment) {
			return true
		}
	}
	return false
}

func sameExchangeInventory(got []inventory.ItemInstance, want []inventory.ItemInstance) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}

func sameExchangeQuickslots(got []loginticket.Quickslot, want []loginticket.Quickslot) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}
