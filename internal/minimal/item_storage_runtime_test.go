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

func TestGameRuntimeCloseSafeboxClearsOpenPresentationWithoutFrames(t *testing.T) {
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

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_safebox error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected /close_safebox to emit no frames, got %d", len(closeOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "close-safebox owner")
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
