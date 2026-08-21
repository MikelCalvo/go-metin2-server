package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
)

func TestGameRuntimeSafeboxCheckinAntiSafeboxTemplateReturnsAuthoredRejectTextWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("StorageBound", 0x010307c1, 0x020407c1, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 761, Vnum: 71124, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "storage-bound-owner", 0x707070c1, owner)
	if err := accounts.Save(accountstore.Account{Login: "storage-bound-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed storage-bound account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:              71124,
		Name:              "Protected Storage Charm",
		Stackable:         false,
		MaxCount:          1,
		AntiSafebox:       true,
		SafeboxRejectText: "This item cannot be placed in storage.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected storage-bound runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "storage-bound-owner", 0x707070c1)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{SafeSlot: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected anti-safebox checkin packet error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected anti-safebox checkin to emit one info-chat frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode anti-safebox checkin rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.SafeboxRejectText {
		t.Fatalf("unexpected anti-safebox checkin rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after anti-safebox checkin rejection, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, "storage-bound-owner", owner, "anti-safebox checkin")
}

func TestGameRuntimeSafeboxCheckinAntiSafeboxTemplateClosesActiveExchangeShellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("StorageExchangeBound", 0x010307c2, 0x020407c2, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 762, Vnum: 71125, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("StorageExchangePeer", 0x010307c3, 0x020407c3, 1120, 2120, 0, 101, 201)
	peer.Gold = 22222
	peer.Inventory = []inventory.ItemInstance{{ID: 763, Vnum: 27001, Count: 2, Slot: 6}}
	peer.Quickslots = []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "storage-exchange-bound", 0x707070c2, owner)
	issuePeerTicket(t, ticketStore, "storage-exchange-peer", 0x707070c3, peer)
	if err := accounts.Save(accountstore.Account{Login: "storage-exchange-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed storage exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "storage-exchange-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed storage exchange peer account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:              71125,
		Name:              "Exchange Protected Storage Charm",
		Stackable:         false,
		MaxCount:          1,
		AntiSafebox:       true,
		SafeboxRejectText: "This item cannot be stored while trading.",
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected storage exchange runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "storage-exchange-bound", 0x707070c2)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "storage-exchange-peer", 0x707070c3)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected storage exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected storage exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "storage exchange owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected storage exchange peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "storage exchange peer start")

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{SafeSlot: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected anti-safebox exchange checkin packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected anti-safebox checkin to emit exchange END plus info-chat frame, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "storage exchange owner close")
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode storage exchange anti-safebox rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.SafeboxRejectText {
		t.Fatalf("unexpected storage exchange anti-safebox rejection chat: %+v", delivery)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected storage exchange peer to receive one queued END, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "storage exchange peer close")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected post-storage exchange cancel error: %v", err)
	}
	if len(cancelOut) != 0 {
		t.Fatalf("expected post-storage exchange cancel to emit no frames after shell close, got %d", len(cancelOut))
	}
	assertExchangeAccountUnchanged(t, accounts, "storage-exchange-bound", owner, "owner storage exchange close")
	assertExchangeAccountUnchanged(t, accounts, "storage-exchange-peer", peer, "peer storage exchange close")
}

func TestGameRuntimeSafeboxCheckinAntiSafeboxTemplateClosesActiveMerchantWindowWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("StorageMerchantBound", 0x010307c4, 0x020407c4, 12345, []inventory.ItemInstance{{ID: 764, Vnum: 71126, Count: 1, Slot: 5}})
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "storage-merchant-bound", 0x707070c4, owner)
	if err := accounts.Save(accountstore.Account{Login: "storage-merchant-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed storage merchant account: %v", err)
	}
	template := itemcatalog.Template{
		Vnum:              71126,
		Name:              "Merchant Protected Storage Charm",
		Stackable:         false,
		MaxCount:          1,
		AntiSafebox:       true,
		SafeboxRejectText: "This item cannot be stored while shopping.",
	}
	templates := append(defaultMerchantItemTemplates(), template)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected storage merchant runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "storage-merchant-bound", 0x707070c4)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)
	interactWithMerchantForBuy(t, flow, actor.EntityID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{SafeSlot: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected anti-safebox merchant checkin packet error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected anti-safebox checkin to emit SHOP END plus info-chat frame, got %d", len(out))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, out[0])); err != nil {
		t.Fatalf("decode storage merchant SHOP END before rejection info chat: %v", err)
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode storage merchant anti-safebox rejection info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != template.SafeboxRejectText {
		t.Fatalf("unexpected storage merchant anti-safebox rejection chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after anti-safebox merchant rejection, got %d", len(queued))
	}

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected post-storage merchant SHOP END error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected post-storage merchant SHOP END to emit no frames after shell close, got %d", len(closeOut))
	}
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-storage merchant SHOP BUY error: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected post-storage merchant SHOP BUY to fail closed until reopen, got %d", len(buyOut))
	}
	assertExchangeAccountUnchanged(t, accounts, "storage-merchant-bound", owner, "storage merchant close")
}

func TestGameRuntimeOpenSafeboxEmitsSizeWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenSafeboxOwner", 0x010307c8, 0x020407c8, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 768, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 1, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "open-safebox-owner"
	issuePeerTicket(t, ticketStore, login, 0x707070c8, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed open-safebox owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected open-safebox runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070c8)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected /open_safebox to emit one SAFEBOX_SIZE frame, got %d", len(out))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode /open_safebox SAFEBOX_SIZE: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 1}) {
		t.Fatalf("unexpected /open_safebox SAFEBOX_SIZE: %+v", size)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected /open_safebox to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "open-safebox owner")
}

func TestGameRuntimeCloseSafeboxClearsOpenPresentationWithCommandChat(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CloseSafeboxOwner", 0x010307c9, 0x020407c9, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 769, Vnum: 27001, Count: 2, Slot: 5}}
	login := "close-safebox-owner"
	issuePeerTicket(t, ticketStore, login, 0x707070c9, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed close-safebox owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected close-safebox runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070c9)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox 2",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before close error: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_safebox before close to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0]))
	if err != nil {
		t.Fatalf("decode /open_safebox before close: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 2}) {
		t.Fatalf("unexpected /open_safebox size before close: %+v", size)
	}

	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "close-safebox owner")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "close-safebox owner")

	alreadyClosedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_close",
	})))
	if err != nil {
		t.Fatalf("unexpected already-closed /safebox_close error: %v", err)
	}
	if len(alreadyClosedOut) != 0 {
		t.Fatalf("expected already-closed /safebox_close to emit no frames, got %d", len(alreadyClosedOut))
	}

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox 2",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before client-slash close error: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected /open_safebox before client-slash close to emit one SAFEBOX_SIZE frame, got %d", len(reopenOut))
	}
	assertCloseSafeboxCommandChat(t, flow, "/safebox_close", "close-safebox client slash")
	assertExchangeAccountUnchanged(t, accounts, login, owner, "close-safebox client slash")
}

func assertCloseSafeboxCommandChat(t *testing.T, flow service.SessionFlow, slash string, label string) {
	t.Helper()
	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: slash,
	})))
	if err != nil {
		t.Fatalf("unexpected %s %s error: %v", label, slash, err)
	}
	if len(closeOut) != 1 {
		t.Fatalf("expected %s %s to emit one CloseSafebox command chat, got %d", label, slash, len(closeOut))
	}
	assertCloseSafeboxCommandChatFrame(t, closeOut[0], label+" "+slash)
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected %s %s to queue no peer frames, got %d", label, slash, len(queued))
	}
}

func assertCloseSafeboxCommandChatFrame(t *testing.T, frame []byte, label string) {
	t.Helper()
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, frame))
	if err != nil {
		t.Fatalf("decode %s CloseSafebox chat: %v", label, err)
	}
	if delivery.Type != chatproto.ChatTypeCommand || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != "CloseSafebox" {
		t.Fatalf("unexpected %s CloseSafebox chat: %+v", label, delivery)
	}
}

func TestGameRuntimeOpenSafeboxOutOfRangeFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenSafeboxOOR", 0x010307ca, 0x020407ca, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 770, Vnum: 27001, Count: 2, Slot: 5}}
	login := "open-safebox-oor"
	issuePeerTicket(t, ticketStore, login, 0x707070ca, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed out-of-range open-safebox owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected out-of-range open-safebox runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070ca)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox 4",
	})))
	if err != nil {
		t.Fatalf("unexpected out-of-range /open_safebox error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected out-of-range /open_safebox to emit no frames (no SAFEBOX_SIZE and no ordinary chat fallthrough), got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected out-of-range /open_safebox to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "out-of-range open-safebox owner")

	// Prove the same-socket open/busy presentation flag stayed closed: a later
	// in-range open must still emit the default size instead of refreshing a
	// phantom out-of-range presentation, and exchange busy policy must not have
	// observed an open safebox from the rejected command.
	validOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected in-range /open_safebox after out-of-range reject error: %v", err)
	}
	if len(validOut) != 1 {
		t.Fatalf("expected in-range /open_safebox after out-of-range reject to emit one SAFEBOX_SIZE frame, got %d", len(validOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, validOut[0]))
	if err != nil {
		t.Fatalf("decode in-range /open_safebox after out-of-range reject: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 1}) {
		t.Fatalf("unexpected in-range /open_safebox size after out-of-range reject: %+v", size)
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "in-range open-safebox after out-of-range reject")
}

func TestGameRuntimeSafeboxCheckinWhileOpenMovesItemToInMemorySafebox(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxCheckinOwner", 0x010307cb, 0x020407cb, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 771, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 1, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "safebox-checkin-owner"
	issuePeerTicket(t, ticketStore, login, 0x707070cb, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox check-in owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox check-in runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070cb)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before check-in error: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_safebox before check-in to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected accepted safebox check-in error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected accepted safebox check-in to emit ITEM_DEL, QUICKSLOT_DEL, and SAFEBOX_SET, got %d", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode safebox check-in ITEM_DEL: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected safebox check-in ITEM_DEL position: %+v", del.Position)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode safebox check-in QUICKSLOT_DEL: %v", err)
	}
	if quickslotDel.Position != 1 {
		t.Fatalf("unexpected safebox check-in QUICKSLOT_DEL position: %d", quickslotDel.Position)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode safebox check-in SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected safebox check-in SAFEBOX_SET: %+v", set)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected accepted safebox check-in to queue no peer frames, got %d", len(queued))
	}

	wantPersisted := owner
	wantPersisted.Inventory = nil
	wantPersisted.Quickslots = nil
	assertExchangeAccountUnchanged(t, accounts, login, wantPersisted, "accepted safebox check-in owner")
	assertExchangeLiveStateUnchanged(t, runtime, wantPersisted, "accepted safebox check-in live owner")

	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "close-safebox after check-in")

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen after check-in error: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected /open_safebox reopen to emit SAFEBOX_SIZE plus remembered SAFEBOX_SET, got %d", len(reopenOut))
	}
	reopenSize, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, reopenOut[0]))
	if err != nil {
		t.Fatalf("decode reopen SAFEBOX_SIZE: %v", err)
	}
	if reopenSize != (itemproto.SafeboxSizePacket{Size: 1}) {
		t.Fatalf("unexpected reopen SAFEBOX_SIZE: %+v", reopenSize)
	}
	reopenSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen SAFEBOX_SET: %v", err)
	}
	if reopenSet.Position != set.Position || reopenSet.Vnum != set.Vnum || reopenSet.Count != set.Count {
		t.Fatalf("unexpected reopen SAFEBOX_SET: %+v want %+v", reopenSet, set)
	}
}

func TestGameRuntimeSafeboxCheckinWithoutOpenFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxClosedCheckin", 0x010307cc, 0x020407cc, 1100, 2100, 0, 101, 201)
	owner.Gold = 1111
	owner.Inventory = []inventory.ItemInstance{{ID: 772, Vnum: 27001, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "safebox-closed-checkin"
	issuePeerTicket(t, ticketStore, login, 0x707070cc, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed closed safebox check-in owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected closed safebox check-in runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070cc)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected closed safebox check-in error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected closed safebox check-in to emit no frames, got %d", len(out))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "closed safebox check-in")
	assertExchangeLiveStateUnchanged(t, runtime, owner, "closed safebox check-in live")
}

func TestGameRuntimeSafeboxCheckinOccupiedOrOutOfRangeFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxBadSlot", 0x010307cd, 0x020407cd, 1100, 2100, 0, 101, 201)
	owner.Gold = 2222
	owner.Inventory = []inventory.ItemInstance{
		{ID: 773, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 774, Vnum: 27002, Count: 1, Slot: 6},
	}
	login := "safebox-bad-slot"
	issuePeerTicket(t, ticketStore, login, 0x707070cd, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed bad-slot safebox check-in owner account: %v", err)
	}
	templates := []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
	}
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected bad-slot safebox check-in runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070cd)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before bad-slot check-in error: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_safebox before bad-slot check-in to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}

	firstOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected first safebox check-in error: %v", err)
	}
	if len(firstOut) != 2 {
		t.Fatalf("expected first safebox check-in to emit ITEM_DEL and SAFEBOX_SET, got %d", len(firstOut))
	}

	afterFirst := owner
	afterFirst.Inventory = []inventory.ItemInstance{{ID: 774, Vnum: 27002, Count: 1, Slot: 6}}
	assertExchangeAccountUnchanged(t, accounts, login, afterFirst, "first safebox check-in before occupied reject")

	occupiedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected occupied safebox check-in error: %v", err)
	}
	if len(occupiedOut) != 0 {
		t.Fatalf("expected occupied safebox check-in to emit no frames, got %d", len(occupiedOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterFirst, "occupied safebox check-in")

	oorOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 5,
		Position: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected out-of-range safebox check-in error: %v", err)
	}
	if len(oorOut) != 0 {
		t.Fatalf("expected out-of-range safebox check-in to emit no frames, got %d", len(oorOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterFirst, "out-of-range safebox check-in")
}

func TestGameRuntimeSafeboxCheckinClosesActiveExchangeShellOnSuccess(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxExchangeCheckin", 0x010307ce, 0x020407ce, 1100, 2100, 0, 101, 201)
	owner.Gold = 3333
	owner.Inventory = []inventory.ItemInstance{{ID: 775, Vnum: 27001, Count: 1, Slot: 5}}
	peer := peerVisibilityCharacter("SafeboxExchangePeer", 0x010307cf, 0x020407cf, 1120, 2120, 0, 101, 201)
	peer.Gold = 4444
	issuePeerTicket(t, ticketStore, "safebox-exchange-checkin", 0x707070ce, owner)
	issuePeerTicket(t, ticketStore, "safebox-exchange-checkin-peer", 0x707070cf, peer)
	if err := accounts.Save(accountstore.Account{Login: "safebox-exchange-checkin", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox exchange owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "safebox-exchange-checkin-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed safebox exchange peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox exchange check-in runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-exchange-checkin", 0x707070ce)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-exchange-checkin-peer", 0x707070cf)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected safebox exchange start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected safebox exchange start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "safebox exchange owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected safebox exchange peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "safebox exchange peer start")

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox during exchange error: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_safebox during exchange to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 1,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected exchange-open safebox check-in error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected exchange-open safebox check-in to emit END, ITEM_DEL, and SAFEBOX_SET, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "safebox exchange owner close before check-in")
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode exchange-open safebox check-in ITEM_DEL: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected exchange-open safebox check-in ITEM_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode exchange-open safebox check-in SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1}) || set.Vnum != 27001 || set.Count != 1 {
		t.Fatalf("unexpected exchange-open safebox check-in SAFEBOX_SET: %+v", set)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected safebox exchange peer to receive one queued END, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "safebox exchange peer close before check-in")

	wantOwner := owner
	wantOwner.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, "safebox-exchange-checkin", wantOwner, "exchange-open safebox check-in owner")
	assertExchangeAccountUnchanged(t, accounts, "safebox-exchange-checkin-peer", peer, "exchange-open safebox check-in peer")
}

func TestGameRuntimeSafeboxCheckoutWhileOpenMovesItemToCarriedInventory(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxCheckoutOwner", 0x010307d0, 0x020407d0, 1100, 2100, 0, 101, 201)
	owner.Gold = 5252
	owner.Inventory = []inventory.ItemInstance{{ID: 781, Vnum: 27001, Count: 2, Slot: 5}}
	login := "safebox-checkout-owner"
	issuePeerTicket(t, ticketStore, login, 0x707070d0, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox check-out owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox check-out runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070d0)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before check-out error: %v", err)
	}
	if len(openOut) != 1 {
		t.Fatalf("expected /open_safebox before check-out to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}

	checkinOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected safebox check-in before check-out error: %v", err)
	}
	if len(checkinOut) != 2 {
		t.Fatalf("expected safebox check-in before check-out to emit ITEM_DEL and SAFEBOX_SET, got %d", len(checkinOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(7),
	})))
	if err != nil {
		t.Fatalf("unexpected accepted safebox check-out error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected accepted safebox check-out to emit SAFEBOX_DEL and ITEM_SET, got %d", len(out))
	}
	del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode safebox check-out SAFEBOX_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) {
		t.Fatalf("unexpected safebox check-out SAFEBOX_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode safebox check-out ITEM_SET: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(7) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected safebox check-out ITEM_SET: %+v", set)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected accepted safebox check-out to queue no peer frames, got %d", len(queued))
	}

	wantPersisted := owner
	wantPersisted.Inventory = []inventory.ItemInstance{{ID: 781, Vnum: 27001, Count: 2, Slot: 7}}
	assertExchangeAccountUnchanged(t, accounts, login, wantPersisted, "accepted safebox check-out owner")
	assertExchangeLiveStateUnchanged(t, runtime, wantPersisted, "accepted safebox check-out live owner")

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen after check-out error: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected /open_safebox reopen after check-out to emit only SAFEBOX_SIZE, got %d", len(reopenOut))
	}
}

func TestGameRuntimeSafeboxCheckoutMergesCompatibleDestination(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxCheckoutMerge", 0x010307d1, 0x020407d1, 1100, 2100, 0, 101, 201)
	owner.Gold = 6262
	owner.Inventory = []inventory.ItemInstance{
		{ID: 782, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 783, Vnum: 27001, Count: 3, Slot: 7},
	}
	login := "safebox-checkout-merge"
	issuePeerTicket(t, ticketStore, login, 0x707070d1, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox check-out merge owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox check-out merge runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070d1)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before merge check-out error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 1,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected safebox check-in before merge check-out error: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 1,
		Position: itemproto.InventoryPosition(7),
	})))
	if err != nil {
		t.Fatalf("unexpected merge safebox check-out error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected merge safebox check-out to emit SAFEBOX_DEL and ITEM_UPDATE, got %d", len(out))
	}
	del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode merge safebox check-out SAFEBOX_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1}) {
		t.Fatalf("unexpected merge safebox check-out SAFEBOX_DEL: %+v", del.Position)
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merge safebox check-out ITEM_UPDATE: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(7) || update.Count != 5 {
		t.Fatalf("unexpected merge safebox check-out ITEM_UPDATE: %+v", update)
	}

	wantPersisted := owner
	wantPersisted.Inventory = []inventory.ItemInstance{{ID: 783, Vnum: 27001, Count: 5, Slot: 7}}
	assertExchangeAccountUnchanged(t, accounts, login, wantPersisted, "merge safebox check-out owner")
}

func TestGameRuntimeSafeboxCheckoutWithoutOpenOrEmptyFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxClosedCheckout", 0x010307d2, 0x020407d2, 1100, 2100, 0, 101, 201)
	owner.Gold = 1111
	owner.Inventory = []inventory.ItemInstance{{ID: 784, Vnum: 27001, Count: 1, Slot: 5}}
	login := "safebox-closed-checkout"
	issuePeerTicket(t, ticketStore, login, 0x707070d2, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed closed safebox check-out owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected closed safebox check-out runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070d2)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	closedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected closed safebox check-out error: %v", err)
	}
	if len(closedOut) != 0 {
		t.Fatalf("expected closed safebox check-out to emit no frames, got %d", len(closedOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "closed safebox check-out")

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before empty check-out error: %v", err)
	}
	emptyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected empty safebox check-out error: %v", err)
	}
	if len(emptyOut) != 0 {
		t.Fatalf("expected empty safebox check-out to emit no frames, got %d", len(emptyOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "empty safebox check-out")
}

func TestGameRuntimeSafeboxCheckoutIncompatibleOrOutOfRangeFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxBadCheckout", 0x010307d3, 0x020407d3, 1100, 2100, 0, 101, 201)
	owner.Gold = 2222
	owner.Inventory = []inventory.ItemInstance{
		{ID: 785, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 786, Vnum: 27002, Count: 1, Slot: 6},
	}
	login := "safebox-bad-checkout"
	issuePeerTicket(t, ticketStore, login, 0x707070d3, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed bad-slot safebox check-out owner account: %v", err)
	}
	templates := []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
	}
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected bad-slot safebox check-out runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070d3)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before bad check-out error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected first safebox check-in before bad check-out error: %v", err)
	}

	afterCheckin := owner
	afterCheckin.Inventory = []inventory.ItemInstance{{ID: 786, Vnum: 27002, Count: 1, Slot: 6}}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "first safebox check-in before bad check-out")

	incompatibleOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected incompatible safebox check-out error: %v", err)
	}
	if len(incompatibleOut) != 0 {
		t.Fatalf("expected incompatible safebox check-out to emit no frames, got %d", len(incompatibleOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "incompatible safebox check-out")

	oorOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 5,
		Position: itemproto.InventoryPosition(7),
	})))
	if err != nil {
		t.Fatalf("unexpected out-of-range safebox check-out error: %v", err)
	}
	if len(oorOut) != 0 {
		t.Fatalf("expected out-of-range safebox check-out to emit no frames, got %d", len(oorOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "out-of-range safebox check-out")
}

func TestGameRuntimeSafeboxCheckoutClosesActiveExchangeShellOnSuccess(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxExchangeCheckout", 0x010307d4, 0x020407d4, 1100, 2100, 0, 101, 201)
	owner.Gold = 3333
	owner.Inventory = []inventory.ItemInstance{{ID: 787, Vnum: 27001, Count: 1, Slot: 5}}
	peer := peerVisibilityCharacter("SafeboxExchangeCheckoutPeer", 0x010307d5, 0x020407d5, 1120, 2120, 0, 101, 201)
	peer.Gold = 4444
	issuePeerTicket(t, ticketStore, "safebox-exchange-checkout", 0x707070d4, owner)
	issuePeerTicket(t, ticketStore, "safebox-exchange-checkout-peer", 0x707070d5, peer)
	if err := accounts.Save(accountstore.Account{Login: "safebox-exchange-checkout", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox exchange check-out owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "safebox-exchange-checkout-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed safebox exchange check-out peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox exchange check-out runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-exchange-checkout", 0x707070d4)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-exchange-checkout-peer", 0x707070d5)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before exchange check-out error: %v", err)
	}
	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 2,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected safebox check-in before exchange check-out error: %v", err)
	}
	assertCloseSafeboxCommandChat(t, ownerFlow, "/close_safebox", "close-safebox before exchange start check-out")

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected safebox exchange start before check-out error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected safebox exchange start before check-out to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "safebox exchange owner start before check-out")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected safebox exchange peer start frame before check-out, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "safebox exchange peer start before check-out")

	reopenOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen during exchange before check-out error: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected /open_safebox reopen during exchange to emit SAFEBOX_SIZE plus remembered SAFEBOX_SET, got %d", len(reopenOut))
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 2,
		Position: itemproto.InventoryPosition(8),
	})))
	if err != nil {
		t.Fatalf("unexpected exchange-open safebox check-out error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected exchange-open safebox check-out to emit END, SAFEBOX_DEL, and ITEM_SET, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "safebox exchange owner close before check-out")
	del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode exchange-open safebox check-out SAFEBOX_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 2}) {
		t.Fatalf("unexpected exchange-open safebox check-out SAFEBOX_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode exchange-open safebox check-out ITEM_SET: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(8) || set.Vnum != 27001 || set.Count != 1 {
		t.Fatalf("unexpected exchange-open safebox check-out ITEM_SET: %+v", set)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected safebox exchange peer to receive one queued END after check-out, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "safebox exchange peer close before check-out")

	wantOwner := owner
	wantOwner.Inventory = []inventory.ItemInstance{{ID: 787, Vnum: 27001, Count: 1, Slot: 8}}
	assertExchangeAccountUnchanged(t, accounts, "safebox-exchange-checkout", wantOwner, "exchange-open safebox check-out owner")
	assertExchangeAccountUnchanged(t, accounts, "safebox-exchange-checkout-peer", peer, "exchange-open safebox check-out peer")
}

func TestGameRuntimeSafeboxItemMoveWhileOpenRelocatesWholeStack(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxMoveOwner", 0x010307e0, 0x020407e0, 1100, 2100, 0, 101, 201)
	owner.Gold = 8080
	owner.Inventory = []inventory.ItemInstance{{ID: 801, Vnum: 27001, Count: 2, Slot: 5}}
	login := "safebox-move-owner"
	issuePeerTicket(t, ticketStore, login, 0x707070e0, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox item-move owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070e0)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before item-move error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected safebox check-in before item-move error: %v", err)
	}

	afterCheckin := owner
	afterCheckin.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "safebox check-in before item-move")

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.InventoryPosition(0),
		Destination: itemproto.InventoryPosition(3),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted TMP4 inventory-window safebox item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected accepted safebox item-move to emit SAFEBOX_DEL and SAFEBOX_SET, got %d", len(out))
	}
	del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode safebox item-move SAFEBOX_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) {
		t.Fatalf("unexpected safebox item-move SAFEBOX_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode safebox item-move SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 3}) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected safebox item-move SAFEBOX_SET: %+v", set)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected accepted safebox item-move to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "accepted safebox item-move owner")
	assertExchangeLiveStateUnchanged(t, runtime, afterCheckin, "accepted safebox item-move live owner")

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen after item-move error: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected /open_safebox reopen after item-move to emit SAFEBOX_SIZE plus remembered SAFEBOX_SET, got %d", len(reopenOut))
	}
	reopenSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen SAFEBOX_SET after item-move: %v", err)
	}
	if reopenSet != set {
		t.Fatalf("unexpected reopen SAFEBOX_SET after item-move: %+v want %+v", reopenSet, set)
	}
}

func TestGameRuntimeSafeboxItemMoveAcceptsExplicitSafeboxAndMixedWindows(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxMoveWireOwner", 0x010307ef, 0x020407ef, 1100, 2100, 0, 101, 201)
	owner.Gold = 8181
	owner.Inventory = []inventory.ItemInstance{{ID: 821, Vnum: 27001, Count: 2, Slot: 5}}
	login := "safebox-move-wire-owner"
	issuePeerTicket(t, ticketStore, login, 0x707070ef, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox item-move wire owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox item-move wire runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070ef)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before wire item-move error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected safebox check-in before wire item-move error: %v", err)
	}

	afterCheckin := owner
	afterCheckin.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "safebox check-in before wire item-move")

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
		Destination: itemproto.InventoryPosition(2),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected mixed-window safebox item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected mixed-window safebox item-move to emit SAFEBOX_DEL and SAFEBOX_SET, got %d", len(out))
	}
	del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode mixed-window safebox item-move SAFEBOX_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) {
		t.Fatalf("unexpected mixed-window safebox item-move SAFEBOX_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode mixed-window safebox item-move SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 2}) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected mixed-window safebox item-move SAFEBOX_SET: %+v", set)
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "mixed-window safebox item-move owner")

	out, err = flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.InventoryPosition(2),
		Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 4},
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected reverse mixed-window safebox item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected reverse mixed-window safebox item-move to emit SAFEBOX_DEL and SAFEBOX_SET, got %d", len(out))
	}
	set, err = itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode reverse mixed-window safebox item-move SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 4}) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected reverse mixed-window safebox item-move SAFEBOX_SET: %+v", set)
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "reverse mixed-window safebox item-move owner")
}

func TestGameRuntimeSafeboxItemMoveMergesCompatibleDestination(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxMoveMerge", 0x010307e1, 0x020407e1, 1100, 2100, 0, 101, 201)
	owner.Gold = 9090
	owner.Inventory = []inventory.ItemInstance{
		{ID: 802, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 803, Vnum: 27001, Count: 3, Slot: 6},
	}
	login := "safebox-move-merge"
	issuePeerTicket(t, ticketStore, login, 0x707070e1, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox item-move merge owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox item-move merge runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070e1)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before merge item-move error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected first safebox check-in before merge item-move error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 1,
		Position: itemproto.InventoryPosition(6),
	}))); err != nil {
		t.Fatalf("unexpected second safebox check-in before merge item-move error: %v", err)
	}

	afterCheckin := owner
	afterCheckin.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "safebox check-ins before merge item-move")

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
		Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1},
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected merge safebox item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected merge safebox item-move to emit SAFEBOX_DEL and SAFEBOX_SET, got %d", len(out))
	}
	del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode merge safebox item-move SAFEBOX_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) {
		t.Fatalf("unexpected merge safebox item-move SAFEBOX_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merge safebox item-move SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1}) || set.Vnum != 27001 || set.Count != 5 {
		t.Fatalf("unexpected merge safebox item-move SAFEBOX_SET: %+v", set)
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "merge safebox item-move owner")

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen after merge item-move error: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected /open_safebox reopen after merge item-move to emit SAFEBOX_SIZE plus one SAFEBOX_SET, got %d", len(reopenOut))
	}
	reopenSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen SAFEBOX_SET after merge item-move: %v", err)
	}
	if reopenSet != set {
		t.Fatalf("unexpected reopen SAFEBOX_SET after merge item-move: %+v want %+v", reopenSet, set)
	}
}

func TestGameRuntimeSafeboxItemMovePartialSplitIntoEmptyCell(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxPartialSplit", 0x010307f0, 0x020407f0, 1100, 2100, 0, 101, 201)
	owner.Gold = 1212
	owner.Inventory = []inventory.ItemInstance{{ID: 820, Vnum: 27001, Count: 5, Slot: 5}}
	login := "safebox-partial-split"
	issuePeerTicket(t, ticketStore, login, 0x707070f0, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox partial-split owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox partial-split runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070f0)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before partial-split error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected safebox check-in before partial-split error: %v", err)
	}

	afterCheckin := owner
	afterCheckin.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "safebox check-in before partial-split")

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
		Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 2},
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected partial-split safebox item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected partial-split safebox item-move to emit two SAFEBOX_SET frames, got %d", len(out))
	}
	sourceSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial-split source SAFEBOX_SET: %v", err)
	}
	if sourceSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || sourceSet.Vnum != 27001 || sourceSet.Count != 3 {
		t.Fatalf("unexpected partial-split source SAFEBOX_SET: %+v", sourceSet)
	}
	destinationSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial-split destination SAFEBOX_SET: %v", err)
	}
	if destinationSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 2}) || destinationSet.Vnum != 27001 || destinationSet.Count != 2 {
		t.Fatalf("unexpected partial-split destination SAFEBOX_SET: %+v", destinationSet)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected partial-split safebox item-move to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "partial-split safebox item-move owner")
	assertExchangeLiveStateUnchanged(t, runtime, afterCheckin, "partial-split safebox item-move live owner")

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen after partial-split error: %v", err)
	}
	if len(reopenOut) != 3 {
		t.Fatalf("expected /open_safebox reopen after partial-split to emit SAFEBOX_SIZE plus two SAFEBOX_SET rows, got %d", len(reopenOut))
	}
	reopenSource, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen source SAFEBOX_SET after partial-split: %v", err)
	}
	reopenDestination, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[2]))
	if err != nil {
		t.Fatalf("decode reopen destination SAFEBOX_SET after partial-split: %v", err)
	}
	if reopenSource != sourceSet {
		t.Fatalf("unexpected reopen source SAFEBOX_SET after partial-split: %+v want %+v", reopenSource, sourceSet)
	}
	if reopenDestination != destinationSet {
		t.Fatalf("unexpected reopen destination SAFEBOX_SET after partial-split: %+v want %+v", reopenDestination, destinationSet)
	}
}

func TestGameRuntimeSafeboxItemMovePartialMergesCompatibleDestination(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxPartialMerge", 0x010307f1, 0x020407f1, 1100, 2100, 0, 101, 201)
	owner.Gold = 1313
	owner.Inventory = []inventory.ItemInstance{
		{ID: 821, Vnum: 27001, Count: 4, Slot: 5},
		{ID: 822, Vnum: 27001, Count: 3, Slot: 6},
	}
	login := "safebox-partial-merge"
	issuePeerTicket(t, ticketStore, login, 0x707070f1, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox partial-merge owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox partial-merge runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070f1)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before partial-merge error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected first safebox check-in before partial-merge error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 1,
		Position: itemproto.InventoryPosition(6),
	}))); err != nil {
		t.Fatalf("unexpected second safebox check-in before partial-merge error: %v", err)
	}

	afterCheckin := owner
	afterCheckin.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "safebox check-ins before partial-merge")

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
		Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1},
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected partial-merge safebox item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected partial-merge safebox item-move to emit two SAFEBOX_SET frames, got %d", len(out))
	}
	sourceSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial-merge source SAFEBOX_SET: %v", err)
	}
	if sourceSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || sourceSet.Vnum != 27001 || sourceSet.Count != 2 {
		t.Fatalf("unexpected partial-merge source SAFEBOX_SET: %+v", sourceSet)
	}
	destinationSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial-merge destination SAFEBOX_SET: %v", err)
	}
	if destinationSet.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1}) || destinationSet.Vnum != 27001 || destinationSet.Count != 5 {
		t.Fatalf("unexpected partial-merge destination SAFEBOX_SET: %+v", destinationSet)
	}
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "partial-merge safebox item-move owner")

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen after partial-merge error: %v", err)
	}
	if len(reopenOut) != 3 {
		t.Fatalf("expected /open_safebox reopen after partial-merge to emit SAFEBOX_SIZE plus two SAFEBOX_SET rows, got %d", len(reopenOut))
	}
	reopenSource, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen source SAFEBOX_SET after partial-merge: %v", err)
	}
	reopenDestination, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[2]))
	if err != nil {
		t.Fatalf("decode reopen destination SAFEBOX_SET after partial-merge: %v", err)
	}
	if reopenSource != sourceSet {
		t.Fatalf("unexpected reopen source SAFEBOX_SET after partial-merge: %+v want %+v", reopenSource, sourceSet)
	}
	if reopenDestination != destinationSet {
		t.Fatalf("unexpected reopen destination SAFEBOX_SET after partial-merge: %+v want %+v", reopenDestination, destinationSet)
	}
}

func TestGameRuntimeSafeboxItemMoveWithoutOpenOrBadCellsFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxClosedMove", 0x010307e2, 0x020407e2, 1100, 2100, 0, 101, 201)
	owner.Gold = 1010
	owner.Inventory = []inventory.ItemInstance{
		{ID: 804, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 805, Vnum: 27002, Count: 1, Slot: 6},
	}
	login := "safebox-closed-move"
	issuePeerTicket(t, ticketStore, login, 0x707070e2, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed closed safebox item-move owner account: %v", err)
	}
	templates := []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
	}
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected closed safebox item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070e2)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	closedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
		Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1},
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected closed safebox item-move error: %v", err)
	}
	if len(closedOut) != 0 {
		t.Fatalf("expected closed safebox item-move to emit no frames, got %d", len(closedOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "closed safebox item-move")

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before bad item-move error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected first safebox check-in before bad item-move error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 1,
		Position: itemproto.InventoryPosition(6),
	}))); err != nil {
		t.Fatalf("unexpected second safebox check-in before bad item-move error: %v", err)
	}

	afterCheckin := owner
	afterCheckin.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, "safebox check-ins before bad item-move")

	cases := []struct {
		name   string
		packet itemproto.ClientSafeboxItemMovePacket
	}{
		{
			name: "mall windows",
			packet: itemproto.ClientSafeboxItemMovePacket{
				Source:      itemproto.Position{WindowType: itemproto.WindowMall, Cell: 0},
				Destination: itemproto.Position{WindowType: itemproto.WindowMall, Cell: 1},
				Count:       0,
			},
		},
		{
			name: "equipment windows",
			packet: itemproto.ClientSafeboxItemMovePacket{
				Source:      itemproto.Position{WindowType: itemproto.WindowEquipment, Cell: 0},
				Destination: itemproto.Position{WindowType: itemproto.WindowEquipment, Cell: 1},
				Count:       0,
			},
		},
		{
			name: "mixed inventory and mall windows",
			packet: itemproto.ClientSafeboxItemMovePacket{
				Source:      itemproto.InventoryPosition(0),
				Destination: itemproto.Position{WindowType: itemproto.WindowMall, Cell: 1},
				Count:       0,
			},
		},
		{
			name: "same cell",
			packet: itemproto.ClientSafeboxItemMovePacket{
				Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
				Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
				Count:       0,
			},
		},
		{
			name: "out of range",
			packet: itemproto.ClientSafeboxItemMovePacket{
				Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
				Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 5},
				Count:       0,
			},
		},
		{
			name: "oversize count",
			packet: itemproto.ClientSafeboxItemMovePacket{
				Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
				Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 2},
				Count:       3,
			},
		},
		{
			name: "incompatible destination",
			packet: itemproto.ClientSafeboxItemMovePacket{
				Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0},
				Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1},
				Count:       0,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(tc.packet)))
			if err != nil {
				t.Fatalf("unexpected %s safebox item-move error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s safebox item-move to emit no frames, got %d", tc.name, len(out))
			}
			assertExchangeAccountUnchanged(t, accounts, login, afterCheckin, tc.name+" safebox item-move")
		})
	}

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen after bad item-move error: %v", err)
	}
	if len(reopenOut) != 3 {
		t.Fatalf("expected /open_safebox reopen after bad item-move to emit SAFEBOX_SIZE plus two SAFEBOX_SET rows, got %d", len(reopenOut))
	}
}

func TestGameRuntimeSafeboxItemMoveClosesActiveExchangeShellOnSuccess(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxExchangeMove", 0x010307e3, 0x020407e3, 1100, 2100, 0, 101, 201)
	owner.Gold = 1112
	owner.Inventory = []inventory.ItemInstance{{ID: 806, Vnum: 27001, Count: 1, Slot: 5}}
	peer := peerVisibilityCharacter("SafeboxExchangeMovePeer", 0x010307e4, 0x020407e4, 1120, 2120, 0, 101, 201)
	peer.Gold = 1113
	issuePeerTicket(t, ticketStore, "safebox-exchange-move", 0x707070e3, owner)
	issuePeerTicket(t, ticketStore, "safebox-exchange-move-peer", 0x707070e4, peer)
	if err := accounts.Save(accountstore.Account{Login: "safebox-exchange-move", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox exchange item-move owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "safebox-exchange-move-peer", Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed safebox exchange item-move peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected safebox exchange item-move runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-exchange-move", 0x707070e3)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "safebox-exchange-move-peer", 0x707070e4)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before exchange item-move error: %v", err)
	}
	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 2,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected safebox check-in before exchange item-move error: %v", err)
	}
	assertCloseSafeboxCommandChat(t, ownerFlow, "/close_safebox", "close-safebox before exchange start item-move")

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected safebox exchange start before item-move error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected safebox exchange start before item-move to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "safebox exchange owner start before item-move")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected safebox exchange peer start frame before item-move, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "safebox exchange peer start before item-move")

	reopenOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen during exchange before item-move error: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected /open_safebox reopen during exchange to emit SAFEBOX_SIZE plus remembered SAFEBOX_SET, got %d", len(reopenOut))
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 2},
		Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 4},
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected exchange-open safebox item-move error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected exchange-open safebox item-move to emit END, SAFEBOX_DEL, and SAFEBOX_SET, got %d", len(out))
	}
	assertExchangeEndFrame(t, out[0], "safebox exchange owner close before item-move")
	del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode exchange-open safebox item-move SAFEBOX_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 2}) {
		t.Fatalf("unexpected exchange-open safebox item-move SAFEBOX_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode exchange-open safebox item-move SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 4}) || set.Vnum != 27001 || set.Count != 1 {
		t.Fatalf("unexpected exchange-open safebox item-move SAFEBOX_SET: %+v", set)
	}
	queuedClose := flushServerFrames(t, peerFlow)
	if len(queuedClose) != 1 {
		t.Fatalf("expected safebox exchange peer to receive one queued END after item-move, got %d", len(queuedClose))
	}
	assertExchangeEndFrame(t, queuedClose[0], "safebox exchange peer close before item-move")

	wantOwner := owner
	wantOwner.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, "safebox-exchange-move", wantOwner, "exchange-open safebox item-move owner")
	assertExchangeAccountUnchanged(t, accounts, "safebox-exchange-move-peer", peer, "exchange-open safebox item-move peer")
}

func TestGameRuntimeSafeboxCheckinClosesActiveMerchantWindowOnSuccess(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("SafeboxMerchantCheckin", 0x010307f0, 0x020407f0, 15151, []inventory.ItemInstance{{ID: 901, Vnum: 27001, Count: 1, Slot: 5}})
	login := "safebox-merchant-checkin"
	issuePeerTicket(t, ticketStore, login, 0x707070f0, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox merchant check-in owner account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected safebox merchant check-in runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070f0)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)
	interactWithMerchantForBuy(t, flow, actor.EntityID)

	if openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox during merchant check-in error: %v", err)
	} else if len(openOut) != 1 {
		t.Fatalf("expected /open_safebox during merchant check-in to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected merchant-open safebox check-in error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected merchant-open safebox check-in to emit SHOP END, ITEM_DEL, and SAFEBOX_SET, got %d", len(out))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, out[0])); err != nil {
		t.Fatalf("decode merchant SHOP END before accepted safebox check-in: %v", err)
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merchant-open safebox check-in ITEM_DEL: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected merchant-open safebox check-in ITEM_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode merchant-open safebox check-in SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || set.Vnum != 27001 || set.Count != 1 {
		t.Fatalf("unexpected merchant-open safebox check-in SAFEBOX_SET: %+v", set)
	}

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected post-check-in merchant SHOP END error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected post-check-in merchant SHOP END to emit no frames after shell close, got %d", len(closeOut))
	}
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-check-in merchant SHOP BUY error: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected post-check-in merchant SHOP BUY to fail closed until reopen, got %d", len(buyOut))
	}

	wantOwner := owner
	wantOwner.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, login, wantOwner, "merchant-open safebox check-in owner")
}

func TestGameRuntimeSafeboxCheckoutClosesActiveMerchantWindowOnSuccess(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("SafeboxMerchantCheckout", 0x010307f1, 0x020407f1, 16161, []inventory.ItemInstance{{ID: 902, Vnum: 27001, Count: 1, Slot: 5}})
	login := "safebox-merchant-checkout"
	issuePeerTicket(t, ticketStore, login, 0x707070f1, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox merchant check-out owner account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected safebox merchant check-out runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070f1)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before merchant check-out error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected safebox check-in before merchant check-out error: %v", err)
	}
	interactWithMerchantForBuy(t, flow, actor.EntityID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(7),
	})))
	if err != nil {
		t.Fatalf("unexpected merchant-open safebox check-out error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected merchant-open safebox check-out to emit SHOP END, SAFEBOX_DEL, and ITEM_SET, got %d", len(out))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, out[0])); err != nil {
		t.Fatalf("decode merchant SHOP END before accepted safebox check-out: %v", err)
	}
	del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merchant-open safebox check-out SAFEBOX_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) {
		t.Fatalf("unexpected merchant-open safebox check-out SAFEBOX_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode merchant-open safebox check-out ITEM_SET: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(7) || set.Vnum != 27001 || set.Count != 1 {
		t.Fatalf("unexpected merchant-open safebox check-out ITEM_SET: %+v", set)
	}

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected post-check-out merchant SHOP END error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected post-check-out merchant SHOP END to emit no frames after shell close, got %d", len(closeOut))
	}
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-check-out merchant SHOP BUY error: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected post-check-out merchant SHOP BUY to fail closed until reopen, got %d", len(buyOut))
	}

	wantOwner := owner
	wantOwner.Inventory = []inventory.ItemInstance{{ID: 902, Vnum: 27001, Count: 1, Slot: 7}}
	assertExchangeAccountUnchanged(t, accounts, login, wantOwner, "merchant-open safebox check-out owner")
}

func TestGameRuntimeSafeboxItemMoveClosesActiveMerchantWindowOnSuccess(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := merchantBuyerCharacter("SafeboxMerchantMove", 0x010307f2, 0x020407f2, 17171, []inventory.ItemInstance{{ID: 903, Vnum: 27001, Count: 1, Slot: 5}})
	login := "safebox-merchant-move"
	issuePeerTicket(t, ticketStore, login, 0x707070f2, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed safebox merchant item-move owner account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected safebox merchant item-move runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070f2)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before merchant item-move error: %v", err)
	}
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 1,
		Position: itemproto.InventoryPosition(5),
	}))); err != nil {
		t.Fatalf("unexpected safebox check-in before merchant item-move error: %v", err)
	}
	interactWithMerchantForBuy(t, flow, actor.EntityID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
		Source:      itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1},
		Destination: itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 3},
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected merchant-open safebox item-move error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected merchant-open safebox item-move to emit SHOP END, SAFEBOX_DEL, and SAFEBOX_SET, got %d", len(out))
	}
	if err := shopproto.DecodeServerEnd(decodeSingleFrame(t, out[0])); err != nil {
		t.Fatalf("decode merchant SHOP END before accepted safebox item-move: %v", err)
	}
	del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merchant-open safebox item-move SAFEBOX_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1}) {
		t.Fatalf("unexpected merchant-open safebox item-move SAFEBOX_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode merchant-open safebox item-move SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 3}) || set.Vnum != 27001 || set.Count != 1 {
		t.Fatalf("unexpected merchant-open safebox item-move SAFEBOX_SET: %+v", set)
	}

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientEnd()))
	if err != nil {
		t.Fatalf("unexpected post-item-move merchant SHOP END error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected post-item-move merchant SHOP END to emit no frames after shell close, got %d", len(closeOut))
	}
	buyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientBuy(shopproto.ClientBuyPacket{RawLeadingByte: 1, CatalogSlot: 0})))
	if err != nil {
		t.Fatalf("unexpected post-item-move merchant SHOP BUY error: %v", err)
	}
	if len(buyOut) != 0 {
		t.Fatalf("expected post-item-move merchant SHOP BUY to fail closed until reopen, got %d", len(buyOut))
	}

	wantOwner := owner
	wantOwner.Inventory = nil
	assertExchangeAccountUnchanged(t, accounts, login, wantOwner, "merchant-open safebox item-move owner")
}
