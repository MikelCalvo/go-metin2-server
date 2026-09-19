package minimal

import (
	"path/filepath"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
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

func TestGameRuntimeSafeboxCheckinAntiSaveTemplateFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AntiSaveCheckinOwner", 0x01030830, 0x02040830, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 930, Vnum: 71130, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 1, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "anti-save-checkin-owner"
	issuePeerTicket(t, ticketStore, login, 0x70708030, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed anti-save check-in owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 71130, Name: "Unsaved Storage Charm", Stackable: false, MaxCount: 1, AntiSave: true}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected anti-save check-in runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70708030)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox before anti-save check-in error: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_safebox before anti-save check-in to emit SAFEBOX_SIZE plus money change, got %d", len(openOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected anti-save check-in packet error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected anti-save check-in to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after anti-save check-in rejection, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "anti-save check-in")
	assertExchangeLiveStateUnchanged(t, runtime, owner, "anti-save check-in live")
}

func TestGameRuntimeSafeboxCheckinAntiSaveWithoutOpenFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AntiSaveClosedCheckin", 0x01030831, 0x02040831, 1100, 2100, 0, 101, 201)
	owner.Gold = 1111
	owner.Inventory = []inventory.ItemInstance{{ID: 931, Vnum: 71130, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "anti-save-closed-checkin"
	issuePeerTicket(t, ticketStore, login, 0x70708031, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed closed anti-save check-in owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 71130, Name: "Unsaved Storage Charm", Stackable: false, MaxCount: 1, AntiSave: true}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected closed anti-save check-in runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x70708031)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected closed anti-save check-in error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected closed anti-save check-in to emit no frames, got %d", len(out))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "closed anti-save check-in")
	assertExchangeLiveStateUnchanged(t, runtime, owner, "closed anti-save check-in live")
}

func TestGameRuntimeSafeboxCheckinAntiSaveFailsClosedOnDeathFloor(t *testing.T) {
	login := "anti-save-death-floor"
	loginKey := uint32(0x19191a90)
	owner := peerVisibilityCharacter("DeadAntiSaveOwner", 0x01030832, 0x02040832, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 932, Vnum: 71130, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}
	template := itemcatalog.Template{Vnum: 71130, Name: "Unsaved Storage Charm", Stackable: false, MaxCount: 1, AntiSave: true}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, []itemcatalog.Template{template})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected death-floor anti-save SAFEBOX_CHECKIN dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected death-floor anti-save SAFEBOX_CHECKIN to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected death-floor anti-save SAFEBOX_CHECKIN to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "death-floor anti-save SAFEBOX_CHECKIN")
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
	if len(out) != 2 {
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
	if len(openOut) != 2 {
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
	if len(reopenOut) != 2 {
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
	if len(validOut) != 2 {
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

func TestGameRuntimeOpenSafeboxRejectsActiveCubeWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenSafeboxCubeBusy", 0x010307cd, 0x020407cd, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 773, Vnum: 27001, Count: 2, Slot: 5}}
	login := "open-safebox-cube-busy"
	issuePeerTicket(t, ticketStore, login, 0x707070cd, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-busy open-safebox owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected cube-busy open-safebox runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707070cd)
	defer closeSessionFlow(t, flow)

	openCubeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before safebox open: %v", err)
	}
	if len(openCubeOut) != 1 {
		t.Fatalf("expected /open_cube before safebox open to emit one command chat frame, got %d", len(openCubeOut))
	}
	assertCubeCommandChatFrame(t, openCubeOut[0], "cube open 20022", "cube before safebox open")

	busyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected cube-busy /open_safebox error: %v", err)
	}
	if len(busyOut) != 1 {
		t.Fatalf("expected cube-busy /open_safebox to emit one info-chat frame, got %d", len(busyOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, busyOut[0]))
	if err != nil {
		t.Fatalf("decode cube-busy /open_safebox info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected cube-busy /open_safebox chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected cube-busy /open_safebox to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-busy open-safebox owner")
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
	if len(openOut) != 2 {
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
	if len(reopenOut) != 3 {
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

func TestGameRuntimeSafeboxCheckinPreservesInstanceSocketsAndAttributes(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name       string
		sockets    *inventory.SocketValues
		attributes *inventory.AttributeValues
		wantWireS  [itemproto.ItemSocketCount]int32
		wantWireA0 itemproto.Attribute
		wantWireA1 itemproto.Attribute
	}{
		{
			name:       "active sockets and attributes",
			sockets:    &activeSockets,
			attributes: &activeAttributes,
			wantWireS:  [itemproto.ItemSocketCount]int32{11, 0, -3},
			wantWireA0: itemproto.Attribute{Type: 4, Value: 55},
			wantWireA1: itemproto.Attribute{Type: 9, Value: -7},
		},
		{
			name:       "explicit zero sockets and attributes",
			sockets:    &zeroSockets,
			attributes: &zeroAttributes,
			wantWireS:  [itemproto.ItemSocketCount]int32{},
		},
		{
			name:       "omitted sockets and attributes use template fallback",
			wantWireS:  [itemproto.ItemSocketCount]int32{21, 22, 23},
			wantWireA0: itemproto.Attribute{Type: 2, Value: 8},
		},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("SafeboxCheckinPreserve", 0x010307f1+uint32(i), 0x020407f1+uint32(i), 1100, 2100, 0, 101, 201)
			owner.Gold = 5353
			owner.Inventory = []inventory.ItemInstance{
				{ID: 880, Vnum: 27001, Count: 2, Slot: 5, Sockets: tc.sockets, Attributes: tc.attributes},
			}
			login := "safebox-checkin-preserve-" + string(rune('a'+i))
			loginKey := uint32(0x707070f1 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed safebox check-in preserve owner account: %v", err)
			}
			template := itemcatalog.Template{
				Vnum:       27001,
				Name:       "Small Red Potion",
				Stackable:  true,
				MaxCount:   200,
				Sockets:    itemcatalog.SocketValues{21, 22, 23},
				Attributes: itemcatalog.AttributeValues{{Type: 2, Value: 8}},
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected safebox check-in preserve runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)
			_ = flushServerFrames(t, flow)

			if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: "/open_safebox",
			}))); err != nil {
				t.Fatalf("unexpected /open_safebox before preserve check-in error: %v", err)
			}

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
				SafeSlot: 0,
				Position: itemproto.InventoryPosition(5),
			})))
			if err != nil {
				t.Fatalf("unexpected preserve safebox check-in error: %v", err)
			}
			if len(out) != 2 {
				t.Fatalf("expected preserve safebox check-in to emit ITEM_DEL and SAFEBOX_SET, got %d", len(out))
			}
			set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode preserve safebox check-in SAFEBOX_SET: %v", err)
			}
			if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) || set.Vnum != 27001 || set.Count != 2 {
				t.Fatalf("unexpected preserve safebox check-in SAFEBOX_SET: %+v", set)
			}
			if set.Sockets != tc.wantWireS {
				t.Fatalf("unexpected preserve check-in SAFEBOX_SET sockets %+v want %+v", set.Sockets, tc.wantWireS)
			}
			if set.Attributes[0] != tc.wantWireA0 || set.Attributes[1] != tc.wantWireA1 {
				t.Fatalf("unexpected preserve check-in SAFEBOX_SET attributes %+v want [%+v %+v]", set.Attributes, tc.wantWireA0, tc.wantWireA1)
			}

			assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "close-safebox after preserve check-in")

			reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: "/open_safebox",
			})))
			if err != nil {
				t.Fatalf("unexpected /open_safebox reopen after preserve check-in error: %v", err)
			}
			if len(reopenOut) < 2 {
				t.Fatalf("expected reopen SAFEBOX_SIZE + SAFEBOX_SET after preserve check-in, got %d", len(reopenOut))
			}
			reopenSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
			if err != nil {
				t.Fatalf("decode reopen SAFEBOX_SET after preserve check-in: %v", err)
			}
			if reopenSet.Position != set.Position || reopenSet.Vnum != set.Vnum || reopenSet.Count != set.Count {
				t.Fatalf("unexpected reopen SAFEBOX_SET after preserve check-in: %+v want %+v", reopenSet, set)
			}
			if reopenSet.Sockets != tc.wantWireS {
				t.Fatalf("unexpected reopen SAFEBOX_SET sockets %+v want %+v", reopenSet.Sockets, tc.wantWireS)
			}
			if reopenSet.Attributes[0] != tc.wantWireA0 || reopenSet.Attributes[1] != tc.wantWireA1 {
				t.Fatalf("unexpected reopen SAFEBOX_SET attributes %+v want [%+v %+v]", reopenSet.Attributes, tc.wantWireA0, tc.wantWireA1)
			}
		})
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
	if len(openOut) != 2 {
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
	if len(openOut) != 2 {
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
	if len(openOut) != 2 {
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
	if len(reopenOut) != 2 {
		t.Fatalf("expected /open_safebox reopen after check-out to emit only SAFEBOX_SIZE, got %d", len(reopenOut))
	}
}

func TestGameRuntimeSafeboxCheckoutFreeCellPreservesInstanceSocketsAndAttributes(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name       string
		sockets    *inventory.SocketValues
		attributes *inventory.AttributeValues
		wantWireS  [itemproto.ItemSocketCount]int32
		wantWireA0 itemproto.Attribute
		wantWireA1 itemproto.Attribute
	}{
		{
			name:       "active sockets and attributes",
			sockets:    &activeSockets,
			attributes: &activeAttributes,
			wantWireS:  [itemproto.ItemSocketCount]int32{11, 0, -3},
			wantWireA0: itemproto.Attribute{Type: 4, Value: 55},
			wantWireA1: itemproto.Attribute{Type: 9, Value: -7},
		},
		{
			name:       "explicit zero sockets and attributes",
			sockets:    &zeroSockets,
			attributes: &zeroAttributes,
			wantWireS:  [itemproto.ItemSocketCount]int32{},
		},
		{
			name:       "omitted sockets and attributes use template fallback",
			wantWireS:  [itemproto.ItemSocketCount]int32{21, 22, 23},
			wantWireA0: itemproto.Attribute{Type: 2, Value: 8},
		},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("SafeboxCheckoutPreserve", 0x010307e0+uint32(i), 0x020407e0+uint32(i), 1100, 2100, 0, 101, 201)
			owner.Gold = 5353
			owner.Inventory = []inventory.ItemInstance{
				{ID: 790, Vnum: 27001, Count: 2, Slot: 5, Sockets: tc.sockets, Attributes: tc.attributes},
			}
			login := "safebox-checkout-preserve-" + string(rune('a'+i))
			loginKey := uint32(0x707070e0 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed safebox check-out preserve owner account: %v", err)
			}
			template := itemcatalog.Template{
				Vnum:       27001,
				Name:       "Small Red Potion",
				Stackable:  true,
				MaxCount:   200,
				Sockets:    itemcatalog.SocketValues{21, 22, 23},
				Attributes: itemcatalog.AttributeValues{{Type: 2, Value: 8}},
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected safebox check-out preserve runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)
			_ = flushServerFrames(t, flow)

			if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: "/open_safebox",
			}))); err != nil {
				t.Fatalf("unexpected /open_safebox before preserve check-out error: %v", err)
			}
			if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
				SafeSlot: 0,
				Position: itemproto.InventoryPosition(5),
			}))); err != nil {
				t.Fatalf("unexpected safebox check-in before preserve check-out error: %v", err)
			}

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
				SafeSlot: 0,
				Position: itemproto.InventoryPosition(7),
			})))
			if err != nil {
				t.Fatalf("unexpected preserve safebox check-out error: %v", err)
			}
			if len(out) != 2 {
				t.Fatalf("expected preserve safebox check-out to emit SAFEBOX_DEL and ITEM_SET, got %d", len(out))
			}
			set, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode preserve safebox check-out ITEM_SET: %v", err)
			}
			if set.Position != itemproto.InventoryPosition(7) || set.Vnum != 27001 || set.Count != 2 {
				t.Fatalf("unexpected preserve safebox check-out ITEM_SET: %+v", set)
			}
			if set.Sockets != tc.wantWireS {
				t.Fatalf("unexpected preserve checkout ITEM_SET sockets %+v want %+v", set.Sockets, tc.wantWireS)
			}
			if set.Attributes[0] != tc.wantWireA0 || set.Attributes[1] != tc.wantWireA1 {
				t.Fatalf("unexpected preserve checkout ITEM_SET attributes %+v want [%+v %+v]", set.Attributes, tc.wantWireA0, tc.wantWireA1)
			}

			persisted, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load preserve checkout account: %v", err)
			}
			got := persisted.Characters[0].Inventory[0]
			if got.ID != 790 || got.Vnum != 27001 || got.Count != 2 || got.Slot != 7 {
				t.Fatalf("unexpected persisted free-cell checkout cell: %#v", got)
			}
			if (tc.sockets != nil) != got.HasSockets() {
				t.Fatalf("persisted HasSockets=%v want %v", got.HasSockets(), tc.sockets != nil)
			}
			if (tc.attributes != nil) != got.HasAttributes() {
				t.Fatalf("persisted HasAttributes=%v want %v", got.HasAttributes(), tc.attributes != nil)
			}
			if tc.sockets != nil {
				if got.Sockets == nil || *got.Sockets != *tc.sockets {
					t.Fatalf("expected persisted sockets %+v, got %#v", *tc.sockets, got.Sockets)
				}
			} else if got.Sockets != nil {
				t.Fatalf("expected omitted persisted sockets, got %#v", got.Sockets)
			}
			if tc.attributes != nil {
				if got.Attributes == nil || *got.Attributes != *tc.attributes {
					t.Fatalf("expected persisted attributes %+v, got %#v", *tc.attributes, got.Attributes)
				}
			} else if got.Attributes != nil {
				t.Fatalf("expected omitted persisted attributes, got %#v", got.Attributes)
			}
		})
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
	if len(reopenOut) != 3 {
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
	if len(reopenOut) != 3 {
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

func TestGameRuntimeSafeboxItemMoveWholeStackEmptyDestinationPreservesInstanceSocketsAndAttributes(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}

	cases := []struct {
		name       string
		sockets    *inventory.SocketValues
		attributes *inventory.AttributeValues
		wantWireS  [itemproto.ItemSocketCount]int32
		wantWireA0 itemproto.Attribute
		wantWireA1 itemproto.Attribute
	}{
		{
			name:       "active sockets and attributes",
			sockets:    &activeSockets,
			attributes: &activeAttributes,
			wantWireS:  [itemproto.ItemSocketCount]int32{11, 0, -3},
			wantWireA0: itemproto.Attribute{Type: 4, Value: 55},
			wantWireA1: itemproto.Attribute{Type: 9, Value: -7},
		},
		{
			name:       "explicit zero sockets and attributes",
			sockets:    &zeroSockets,
			attributes: &zeroAttributes,
			wantWireS:  [itemproto.ItemSocketCount]int32{},
		},
		{
			name:       "omitted sockets and attributes use template fallback",
			wantWireS:  [itemproto.ItemSocketCount]int32{21, 22, 23},
			wantWireA0: itemproto.Attribute{Type: 2, Value: 8},
		},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("SafeboxMovePreserve", 0x010307f2+uint32(i), 0x020407f2+uint32(i), 1100, 2100, 0, 101, 201)
			owner.Gold = 8282
			owner.Inventory = []inventory.ItemInstance{
				{ID: 890, Vnum: 27001, Count: 2, Slot: 5, Sockets: tc.sockets, Attributes: tc.attributes},
			}
			login := "safebox-move-preserve-" + string(rune('a'+i))
			loginKey := uint32(0x707070f2 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed safebox item-move preserve owner account: %v", err)
			}
			template := itemcatalog.Template{
				Vnum:       27001,
				Name:       "Small Red Potion",
				Stackable:  true,
				MaxCount:   200,
				Sockets:    itemcatalog.SocketValues{21, 22, 23},
				Attributes: itemcatalog.AttributeValues{{Type: 2, Value: 8}},
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected safebox item-move preserve runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)
			_ = flushServerFrames(t, flow)

			if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: "/open_safebox",
			}))); err != nil {
				t.Fatalf("unexpected /open_safebox before preserve item-move error: %v", err)
			}
			if _, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
				SafeSlot: 0,
				Position: itemproto.InventoryPosition(5),
			}))); err != nil {
				t.Fatalf("unexpected safebox check-in before preserve item-move error: %v", err)
			}

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxItemMove(itemproto.ClientSafeboxItemMovePacket{
				Source:      itemproto.InventoryPosition(0),
				Destination: itemproto.InventoryPosition(3),
				Count:       0,
			})))
			if err != nil {
				t.Fatalf("unexpected preserve safebox item-move error: %v", err)
			}
			if len(out) != 2 {
				t.Fatalf("expected preserve safebox item-move to emit SAFEBOX_DEL and SAFEBOX_SET, got %d", len(out))
			}
			del, err := itemproto.DecodeSafeboxDel(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode preserve safebox item-move SAFEBOX_DEL: %v", err)
			}
			if del.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 0}) {
				t.Fatalf("unexpected preserve safebox item-move SAFEBOX_DEL: %+v", del.Position)
			}
			set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode preserve safebox item-move SAFEBOX_SET: %v", err)
			}
			if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 3}) || set.Vnum != 27001 || set.Count != 2 {
				t.Fatalf("unexpected preserve safebox item-move SAFEBOX_SET: %+v", set)
			}
			if set.Sockets != tc.wantWireS {
				t.Fatalf("unexpected preserve item-move SAFEBOX_SET sockets %+v want %+v", set.Sockets, tc.wantWireS)
			}
			if set.Attributes[0] != tc.wantWireA0 || set.Attributes[1] != tc.wantWireA1 {
				t.Fatalf("unexpected preserve item-move SAFEBOX_SET attributes %+v want [%+v %+v]", set.Attributes, tc.wantWireA0, tc.wantWireA1)
			}

			assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "close-safebox after preserve item-move")

			reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
				Type:    chatproto.ChatTypeTalking,
				Message: "/open_safebox",
			})))
			if err != nil {
				t.Fatalf("unexpected /open_safebox reopen after preserve item-move error: %v", err)
			}
			if len(reopenOut) < 2 {
				t.Fatalf("expected reopen SAFEBOX_SIZE + SAFEBOX_SET after preserve item-move, got %d", len(reopenOut))
			}
			reopenSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
			if err != nil {
				t.Fatalf("decode reopen SAFEBOX_SET after preserve item-move: %v", err)
			}
			if reopenSet.Position != set.Position || reopenSet.Vnum != set.Vnum || reopenSet.Count != set.Count {
				t.Fatalf("unexpected reopen SAFEBOX_SET after preserve item-move: %+v want %+v", reopenSet, set)
			}
			if reopenSet.Sockets != tc.wantWireS {
				t.Fatalf("unexpected reopen SAFEBOX_SET sockets %+v want %+v", reopenSet.Sockets, tc.wantWireS)
			}
			if reopenSet.Attributes[0] != tc.wantWireA0 || reopenSet.Attributes[1] != tc.wantWireA1 {
				t.Fatalf("unexpected reopen SAFEBOX_SET attributes %+v want [%+v %+v]", reopenSet.Attributes, tc.wantWireA0, tc.wantWireA1)
			}
		})
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
	if len(reopenOut) != 3 {
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
	if len(reopenOut) != 4 {
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
	if len(reopenOut) != 4 {
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
	if len(reopenOut) != 4 {
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
	if len(reopenOut) != 3 {
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
	} else if len(openOut) != 2 {
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

func TestGameRuntimeSafeboxCheckinOfDisplayedExchangeItemFailsClosedWithoutClosingShell(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxDispCheckinOwner", 0x01030801, 0x02040801, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 951, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	peer := peerVisibilityCharacter("SafeboxDispCheckinPeer", 0x01030802, 0x02040802, 1120, 2120, 0, 101, 201)
	peer.Gold = 5151
	ownerLogin := "safebox-disp-checkin-owner"
	peerLogin := "safebox-disp-checkin-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707101, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707102, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed displayed-exchange safebox check-in owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed displayed-exchange safebox check-in peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected displayed-exchange safebox check-in runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707101)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707102)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange safebox check-in start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected displayed-exchange safebox check-in start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "displayed-exchange safebox check-in owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected displayed-exchange safebox check-in peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "displayed-exchange safebox check-in peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 7, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange safebox check-in item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected displayed-exchange safebox check-in item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 7, owner.Inventory[0], template, "displayed-exchange safebox check-in owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected displayed-exchange safebox check-in peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 7, owner.Inventory[0], template, "displayed-exchange safebox check-in peer item-add")

	openOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox during displayed-exchange check-in error: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_safebox during displayed-exchange check-in to emit one SAFEBOX_SIZE frame, got %d", len(openOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected /open_safebox during displayed-exchange check-in to queue no peer frames, got %d", len(queued))
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange locked safebox check-in error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected displayed-exchange locked safebox check-in to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected displayed-exchange locked safebox check-in to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, owner, "displayed-exchange locked safebox check-in owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "displayed-exchange locked safebox check-in peer")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange cancel after locked safebox check-in: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected displayed-exchange shell to remain cancellable after locked safebox check-in, got %d frames", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "displayed-exchange locked safebox check-in owner cancel")
	queuedCancel := flushServerFrames(t, peerFlow)
	if len(queuedCancel) != 1 {
		t.Fatalf("expected displayed-exchange locked safebox check-in peer cancel END, got %d", len(queuedCancel))
	}
	assertExchangeEndFrame(t, queuedCancel[0], "displayed-exchange locked safebox check-in peer cancel")
}

func TestGameRuntimeSafeboxCheckoutIntoDisplayedExchangeCellFailsClosedWithoutClosingShell(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("SafeboxDispCheckoutOwner", 0x01030803, 0x02040803, 1100, 2100, 0, 101, 201)
	owner.Gold = 6262
	owner.Inventory = []inventory.ItemInstance{
		{ID: 961, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 962, Vnum: 27001, Count: 1, Slot: 6},
	}
	peer := peerVisibilityCharacter("SafeboxDispCheckoutPeer", 0x01030804, 0x02040804, 1120, 2120, 0, 101, 201)
	peer.Gold = 7373
	ownerLogin := "safebox-disp-checkout-owner"
	peerLogin := "safebox-disp-checkout-peer"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x70707103, owner)
	issuePeerTicket(t, ticketStore, peerLogin, 0x70707104, peer)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed displayed-exchange safebox check-out owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: peerLogin, Empire: peer.Empire, Characters: cloneCharacters([]loginticket.Character{peer})}); err != nil {
		t.Fatalf("seed displayed-exchange safebox check-out peer account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected displayed-exchange safebox check-out runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x70707103)
	defer closeSessionFlow(t, ownerFlow)
	peerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), peerLogin, 0x70707104)
	defer closeSessionFlow(t, peerFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, peerFlow)

	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	}))); err != nil {
		t.Fatalf("unexpected /open_safebox before displayed-exchange check-out error: %v", err)
	}
	checkinOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected safebox check-in before displayed-exchange check-out error: %v", err)
	}
	if len(checkinOut) < 2 {
		t.Fatalf("expected safebox check-in before displayed-exchange check-out to emit inventory/safebox frames, got %d", len(checkinOut))
	}

	afterCheckin := owner
	afterCheckin.Inventory = []inventory.ItemInstance{{ID: 961, Vnum: 27001, Count: 1, Slot: 5}}
	assertExchangeAccountUnchanged(t, accounts, ownerLogin, afterCheckin, "safebox check-in before displayed-exchange check-out")
	assertCloseSafeboxCommandChat(t, ownerFlow, "/close_safebox", "close-safebox before displayed-exchange check-out start")

	startOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderStart, Arg1: peer.VID})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange safebox check-out start error: %v", err)
	}
	if len(startOut) != 1 {
		t.Fatalf("expected displayed-exchange safebox check-out start to emit one owner frame, got %d", len(startOut))
	}
	assertExchangeStartFrame(t, startOut[0], peer.VID, "displayed-exchange safebox check-out owner start")
	queuedStart := flushServerFrames(t, peerFlow)
	if len(queuedStart) != 1 {
		t.Fatalf("expected displayed-exchange safebox check-out peer start frame, got %d", len(queuedStart))
	}
	assertExchangeStartFrame(t, queuedStart[0], owner.VID, "displayed-exchange safebox check-out peer start")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemAdd, Arg2: 3, Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange safebox check-out item-add error: %v", err)
	}
	if len(itemAddOut) != 1 {
		t.Fatalf("expected displayed-exchange safebox check-out item-add to emit one owner frame, got %d", len(itemAddOut))
	}
	assertExchangeItemAddFrame(t, itemAddOut[0], 1, 3, afterCheckin.Inventory[0], template, "displayed-exchange safebox check-out owner item-add")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 1 {
		t.Fatalf("expected displayed-exchange safebox check-out peer item-add frame, got %d", len(queuedItemAdd))
	}
	assertExchangeItemAddFrame(t, queuedItemAdd[0], 0, 3, afterCheckin.Inventory[0], template, "displayed-exchange safebox check-out peer item-add")

	reopenOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox reopen during displayed-exchange before check-out error: %v", err)
	}
	if len(reopenOut) != 3 {
		t.Fatalf("expected /open_safebox reopen during displayed-exchange to emit SAFEBOX_SIZE plus remembered SAFEBOX_SET, got %d", len(reopenOut))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected /open_safebox reopen during displayed-exchange check-out to queue no peer frames, got %d", len(queued))
	}

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckout(itemproto.ClientSafeboxCheckoutPacket{
		SafeSlot: 0,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange locked safebox check-out error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected displayed-exchange locked safebox check-out to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, peerFlow); len(queued) != 0 {
		t.Fatalf("expected displayed-exchange locked safebox check-out to queue no peer frames, got %d", len(queued))
	}

	assertExchangeAccountUnchanged(t, accounts, ownerLogin, afterCheckin, "displayed-exchange locked safebox check-out owner")
	assertExchangeAccountUnchanged(t, accounts, peerLogin, peer, "displayed-exchange locked safebox check-out peer")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected displayed-exchange cancel after locked safebox check-out: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected displayed-exchange shell to remain cancellable after locked safebox check-out, got %d frames", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "displayed-exchange locked safebox check-out owner cancel")
	queuedCancel := flushServerFrames(t, peerFlow)
	if len(queuedCancel) != 1 {
		t.Fatalf("expected displayed-exchange locked safebox check-out peer cancel END, got %d", len(queuedCancel))
	}
	assertExchangeEndFrame(t, queuedCancel[0], "displayed-exchange locked safebox check-out peer cancel")
}

func TestGameRuntimeOpenMallEmitsOpenWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenMallOwner", 0x010309c1, 0x020409c1, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 1801, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 1, Type: quickslotproto.TypeItem, Slot: 5}}
	login := "open-mall-owner"
	issuePeerTicket(t, ticketStore, login, 0x707079c1, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed open-mall owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected open-mall runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707079c1)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected /open_mall to emit one MALL_OPEN frame, got %d", len(out))
	}
	open, err := itemproto.DecodeMallOpen(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode /open_mall MALL_OPEN: %v", err)
	}
	if open != (itemproto.MallOpenPacket{Size: 1}) {
		t.Fatalf("unexpected /open_mall MALL_OPEN: %+v", open)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected /open_mall to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "open-mall owner")
}

func TestGameRuntimeOpenMallRematerializesInRangeMallSetRows(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenMallSeed", 0x010309c2, 0x020409c2, 1100, 2100, 0, 101, 201)
	owner.Gold = 5151
	owner.Inventory = []inventory.ItemInstance{{ID: 1802, Vnum: 27001, Count: 3, Slot: 5}}
	login := "open-mall-seed"
	issuePeerTicket(t, ticketStore, login, 0x707079c2, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed open-mall rematerialize owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected open-mall rematerialize runtime error: %v", err)
	}
	zeroSockets := inventory.SocketValues{}
	runtime.SeedMallCellsForTest(login, owner.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 1901, Vnum: 27001, Count: 2, Slot: 0},
		2: {ID: 1902, Vnum: 27001, Count: 1, Slot: 2, Sockets: &zeroSockets},
		7: {ID: 1903, Vnum: 27001, Count: 4, Slot: 7},
	})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707079c2)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall rematerialize error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected /open_mall to emit MALL_OPEN plus two in-range MALL_SET frames, got %d", len(out))
	}
	open, err := itemproto.DecodeMallOpen(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode rematerialize MALL_OPEN: %v", err)
	}
	if open != (itemproto.MallOpenPacket{Size: 1}) {
		t.Fatalf("unexpected rematerialize MALL_OPEN: %+v", open)
	}
	first, err := itemproto.DecodeMallSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode first rematerialize MALL_SET: %v", err)
	}
	if first.Position != (itemproto.Position{WindowType: itemproto.WindowMall, Cell: 0}) || first.Vnum != 27001 || first.Count != 2 {
		t.Fatalf("unexpected first rematerialize MALL_SET: %+v", first)
	}
	second, err := itemproto.DecodeMallSet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode second rematerialize MALL_SET: %v", err)
	}
	if second.Position != (itemproto.Position{WindowType: itemproto.WindowMall, Cell: 2}) || second.Vnum != 27001 || second.Count != 1 {
		t.Fatalf("unexpected second rematerialize MALL_SET: %+v", second)
	}
	if second.Sockets != [itemproto.ItemSocketCount]int32{} {
		t.Fatalf("expected rematerialize MALL_SET to keep explicit-zero sockets, got %+v", second.Sockets)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected rematerialize /open_mall to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "open-mall rematerialize owner")
}

func TestGameRuntimeOpenMallOutOfRangeFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenMallOOR", 0x010309c3, 0x020409c3, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 1803, Vnum: 27001, Count: 2, Slot: 5}}
	login := "open-mall-oor"
	issuePeerTicket(t, ticketStore, login, 0x707079c3, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed out-of-range open-mall owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected out-of-range open-mall runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707079c3)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall 4",
	})))
	if err != nil {
		t.Fatalf("unexpected out-of-range /open_mall error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected out-of-range /open_mall to emit no frames (no MALL_OPEN and no ordinary chat fallthrough), got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected out-of-range /open_mall to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "out-of-range open-mall owner")

	validOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected in-range /open_mall after out-of-range reject error: %v", err)
	}
	if len(validOut) != 1 {
		t.Fatalf("expected in-range /open_mall after out-of-range reject to emit one MALL_OPEN frame, got %d", len(validOut))
	}
	open, err := itemproto.DecodeMallOpen(decodeSingleFrame(t, validOut[0]))
	if err != nil {
		t.Fatalf("decode in-range /open_mall after out-of-range reject: %v", err)
	}
	if open != (itemproto.MallOpenPacket{Size: 1}) {
		t.Fatalf("unexpected in-range /open_mall size after out-of-range reject: %+v", open)
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "in-range open-mall after out-of-range reject")
}

func TestGameRuntimeOpenMallRejectsActiveCubeWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenMallCubeBusy", 0x010309c4, 0x020409c4, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 1804, Vnum: 27001, Count: 2, Slot: 5}}
	login := "open-mall-cube-busy"
	issuePeerTicket(t, ticketStore, login, 0x707079c4, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed cube-busy open-mall owner account: %v", err)
	}
	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected cube-busy open-mall runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707079c4)
	defer closeSessionFlow(t, flow)

	openCubeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_cube",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_cube before mall open: %v", err)
	}
	if len(openCubeOut) != 1 {
		t.Fatalf("expected /open_cube before mall open to emit one command chat frame, got %d", len(openCubeOut))
	}
	assertCubeCommandChatFrame(t, openCubeOut[0], "cube open 20022", "cube before mall open")

	busyOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected cube-busy /open_mall error: %v", err)
	}
	if len(busyOut) != 1 {
		t.Fatalf("expected cube-busy /open_mall to emit one info-chat frame, got %d", len(busyOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, busyOut[0]))
	if err != nil {
		t.Fatalf("decode cube-busy /open_mall info chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Message != exchangeRequesterMerchantBusyInfoMessage {
		t.Fatalf("unexpected cube-busy /open_mall chat: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected cube-busy /open_mall to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "cube-busy open-mall owner")
}

func TestGameRuntimeMallCheckoutWhileOpenMovesItemToCarriedInventory(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenMallCheckout", 0x010309c5, 0x020409c5, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 1805, Vnum: 27001, Count: 2, Slot: 5}}
	login := "open-mall-checkout"
	issuePeerTicket(t, ticketStore, login, 0x707079c5, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed open-mall checkout owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected open-mall checkout runtime error: %v", err)
	}
	runtime.SeedMallCellsForTest(login, owner.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 1904, Vnum: 27001, Count: 2, Slot: 0},
	})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707079c5)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall before mall checkout: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_mall before mall checkout to emit MALL_OPEN plus one MALL_SET, got %d", len(openOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMallCheckout(itemproto.ClientMallCheckoutPacket{
		MallSlot: 0,
		Position: itemproto.InventoryPosition(9),
	})))
	if err != nil {
		t.Fatalf("unexpected accepted mall checkout error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected accepted mall checkout to emit MALL_DEL and ITEM_SET, got %d", len(out))
	}
	del, err := itemproto.DecodeMallDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode mall checkout MALL_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowMall, Cell: 0}) {
		t.Fatalf("unexpected mall checkout MALL_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode mall checkout ITEM_SET: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(9) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected mall checkout ITEM_SET: %+v", set)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected accepted mall checkout to queue no peer frames, got %d", len(queued))
	}

	wantPersisted := owner
	wantPersisted.Inventory = []inventory.ItemInstance{
		{ID: 1805, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 1904, Vnum: 27001, Count: 2, Slot: 9},
	}
	assertExchangeAccountUnchanged(t, accounts, login, wantPersisted, "accepted mall checkout owner")
	assertExchangeLiveStateUnchanged(t, runtime, wantPersisted, "accepted mall checkout live owner")

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall reopen after mall checkout error: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected /open_mall reopen after mall checkout to emit only MALL_OPEN, got %d", len(reopenOut))
	}
	open, err := itemproto.DecodeMallOpen(decodeSingleFrame(t, reopenOut[0]))
	if err != nil {
		t.Fatalf("decode /open_mall reopen after mall checkout: %v", err)
	}
	if open != (itemproto.MallOpenPacket{Size: 1}) {
		t.Fatalf("unexpected /open_mall reopen after mall checkout: %+v", open)
	}
}

func TestGameRuntimeMallCheckoutMergesCompatibleDestination(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenMallCheckoutMerge", 0x010309c7, 0x020409c7, 1100, 2100, 0, 101, 201)
	owner.Gold = 4343
	owner.Inventory = []inventory.ItemInstance{{ID: 1807, Vnum: 27001, Count: 3, Slot: 7}}
	login := "open-mall-checkout-merge"
	issuePeerTicket(t, ticketStore, login, 0x707079c7, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed mall checkout merge owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected mall checkout merge runtime error: %v", err)
	}
	runtime.SeedMallCellsForTest(login, owner.ID, map[uint8]inventory.ItemInstance{
		1: {ID: 1906, Vnum: 27001, Count: 2, Slot: 1},
	})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707079c7)
	defer closeSessionFlow(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	}))); err != nil {
		t.Fatalf("unexpected /open_mall before merge mall checkout: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMallCheckout(itemproto.ClientMallCheckoutPacket{
		MallSlot: 1,
		Position: itemproto.InventoryPosition(7),
	})))
	if err != nil {
		t.Fatalf("unexpected merge mall checkout error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected merge mall checkout to emit MALL_DEL and ITEM_UPDATE, got %d", len(out))
	}
	del, err := itemproto.DecodeMallDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode merge mall checkout MALL_DEL: %v", err)
	}
	if del.Position != (itemproto.Position{WindowType: itemproto.WindowMall, Cell: 1}) {
		t.Fatalf("unexpected merge mall checkout MALL_DEL: %+v", del.Position)
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode merge mall checkout ITEM_UPDATE: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(7) || update.Count != 5 {
		t.Fatalf("unexpected merge mall checkout ITEM_UPDATE: %+v", update)
	}

	wantPersisted := owner
	wantPersisted.Inventory = []inventory.ItemInstance{{ID: 1807, Vnum: 27001, Count: 5, Slot: 7}}
	assertExchangeAccountUnchanged(t, accounts, login, wantPersisted, "merge mall checkout owner")
}

func TestGameRuntimeCloseMallClearsOpenPresentationWithoutFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CloseMallOwner", 0x010309c6, 0x020409c6, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 1806, Vnum: 27001, Count: 2, Slot: 5}}
	login := "close-mall-owner"
	issuePeerTicket(t, ticketStore, login, 0x707079c6, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed close-mall owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected close-mall runtime error: %v", err)
	}
	runtime.SeedMallCellsForTest(login, owner.ID, map[uint8]inventory.ItemInstance{
		1: {ID: 1905, Vnum: 27001, Count: 3, Slot: 1},
	})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707079c6)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall 2",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall before close error: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_mall before close to emit MALL_OPEN plus one MALL_SET, got %d", len(openOut))
	}
	open, err := itemproto.DecodeMallOpen(decodeSingleFrame(t, openOut[0]))
	if err != nil {
		t.Fatalf("decode /open_mall before close: %v", err)
	}
	if open != (itemproto.MallOpenPacket{Size: 2}) {
		t.Fatalf("unexpected /open_mall size before close: %+v", open)
	}

	closeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/close_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /close_mall error: %v", err)
	}
	if len(closeOut) != 0 {
		t.Fatalf("expected /close_mall to emit no frames, got %d", len(closeOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "close-mall owner")

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall 2",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall after close error: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected /open_mall after close to rematerialize MALL_OPEN plus MALL_SET, got %d", len(reopenOut))
	}
	reopenSet, err := itemproto.DecodeMallSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode rematerialize MALL_SET after close: %v", err)
	}
	if reopenSet.Position != (itemproto.Position{WindowType: itemproto.WindowMall, Cell: 1}) || reopenSet.Vnum != 27001 || reopenSet.Count != 3 {
		t.Fatalf("unexpected rematerialize MALL_SET after close: %+v", reopenSet)
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "open-mall after close")
}

func TestGameRuntimeMallItemMoveWhileOpenRelocatesWholeStack(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenMallMove", 0x010309c8, 0x020409c8, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 1808, Vnum: 27001, Count: 2, Slot: 5}}
	login := "open-mall-move"
	issuePeerTicket(t, ticketStore, login, 0x707079c8, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed open-mall item-move owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected open-mall item-move runtime error: %v", err)
	}
	runtime.SeedMallCellsForTest(login, owner.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 1907, Vnum: 27001, Count: 2, Slot: 0},
	})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707079c8)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall before mall item-move: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_mall before mall item-move to emit MALL_OPEN plus one MALL_SET, got %d", len(openOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.MallPosition(0),
		Destination: itemproto.MallPosition(3),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected accepted mall item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected accepted mall item-move to emit MALL_DEL and MALL_SET, got %d", len(out))
	}
	del, err := itemproto.DecodeMallDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode mall item-move MALL_DEL: %v", err)
	}
	if del.Position != itemproto.MallPosition(0) {
		t.Fatalf("unexpected mall item-move MALL_DEL: %+v", del.Position)
	}
	set, err := itemproto.DecodeMallSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode mall item-move MALL_SET: %v", err)
	}
	if set.Position != itemproto.MallPosition(3) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected mall item-move MALL_SET: %+v", set)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected accepted mall item-move to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "accepted mall item-move owner")
	assertExchangeLiveStateUnchanged(t, runtime, owner, "accepted mall item-move live owner")

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall reopen after mall item-move error: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected /open_mall reopen after mall item-move to emit MALL_OPEN plus remembered MALL_SET, got %d", len(reopenOut))
	}
	reopenSet, err := itemproto.DecodeMallSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen MALL_SET after mall item-move: %v", err)
	}
	if reopenSet != set {
		t.Fatalf("unexpected reopen MALL_SET after mall item-move: %+v want %+v", reopenSet, set)
	}
}

func TestGameRuntimeMallItemMoveWithoutOpenOrBadCellsFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ClosedMallMove", 0x010309c9, 0x020409c9, 1100, 2100, 0, 101, 201)
	owner.Gold = 4343
	owner.Inventory = []inventory.ItemInstance{{ID: 1809, Vnum: 27001, Count: 2, Slot: 5}}
	login := "closed-mall-move"
	issuePeerTicket(t, ticketStore, login, 0x707079c9, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed closed mall item-move owner account: %v", err)
	}
	templates := []itemcatalog.Template{
		{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
		{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
	}
	itemStore := newItemTemplateStore(t, templates)
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected closed mall item-move runtime error: %v", err)
	}
	runtime.SeedMallCellsForTest(login, owner.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 1908, Vnum: 27001, Count: 2, Slot: 0},
		1: {ID: 1909, Vnum: 27002, Count: 1, Slot: 1},
	})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707079c9)
	defer closeSessionFlow(t, flow)

	closedOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.MallPosition(0),
		Destination: itemproto.MallPosition(3),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected closed mall item-move error: %v", err)
	}
	if len(closedOut) != 0 {
		t.Fatalf("expected closed mall item-move to emit no frames, got %d", len(closedOut))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "closed mall item-move")

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall before bad mall item-move error: %v", err)
	}
	if len(openOut) != 3 {
		t.Fatalf("expected /open_mall before bad mall item-move to emit MALL_OPEN plus two MALL_SET rows, got %d", len(openOut))
	}

	cases := []struct {
		name   string
		packet itemproto.ClientMovePacket
	}{
		{
			name: "inventory windows",
			packet: itemproto.ClientMovePacket{
				Source:      itemproto.InventoryPosition(5),
				Destination: itemproto.MallPosition(3),
				Count:       0,
			},
		},
		{
			name: "mixed mall and inventory windows",
			packet: itemproto.ClientMovePacket{
				Source:      itemproto.MallPosition(0),
				Destination: itemproto.InventoryPosition(9),
				Count:       0,
			},
		},
		{
			name: "same cell",
			packet: itemproto.ClientMovePacket{
				Source:      itemproto.MallPosition(0),
				Destination: itemproto.MallPosition(0),
				Count:       0,
			},
		},
		{
			name: "out of range",
			packet: itemproto.ClientMovePacket{
				Source:      itemproto.MallPosition(0),
				Destination: itemproto.MallPosition(5),
				Count:       0,
			},
		},
		{
			name: "oversize count",
			packet: itemproto.ClientMovePacket{
				Source:      itemproto.MallPosition(0),
				Destination: itemproto.MallPosition(3),
				Count:       3,
			},
		},
		{
			name: "occupied destination",
			packet: itemproto.ClientMovePacket{
				Source:      itemproto.MallPosition(0),
				Destination: itemproto.MallPosition(1),
				Count:       0,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(tc.packet)))
			if err != nil {
				t.Fatalf("unexpected %s mall item-move error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s mall item-move to emit no frames, got %d", tc.name, len(out))
			}
			assertExchangeAccountUnchanged(t, accounts, login, owner, tc.name+" mall item-move")
		})
	}

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall reopen after bad mall item-move error: %v", err)
	}
	if len(reopenOut) != 3 {
		t.Fatalf("expected /open_mall reopen after bad mall item-move to emit MALL_OPEN plus two MALL_SET rows, got %d", len(reopenOut))
	}
}

func TestGameRuntimeMallItemMoveFailsClosedAtDeathFloorWithoutMutation(t *testing.T) {
	login := "post-floor-mall-move"
	loginKey := uint32(0x19191c80)
	owner := peerVisibilityCharacter("DeadMallMoveOwner", 0x01030c80, 0x02040c80, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 1
	owner.Gold = 5151
	owner.Inventory = []inventory.ItemInstance{{ID: 1810, Vnum: 27001, Count: 2, Slot: 5}}
	templates := []itemcatalog.Template{{Vnum: 27001, Name: "Post Floor Mall Potion", Stackable: true, MaxCount: 200}}
	runtime, accounts, targetVID := newPostFloorItemGuardRuntime(t, login, loginKey, owner, templates)
	runtime.SeedMallCellsForTest(login, owner.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 1910, Vnum: 27001, Count: 2, Slot: 0},
	})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall before post-floor mall item-move: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_mall before post-floor mall item-move to emit MALL_OPEN plus one MALL_SET, got %d", len(openOut))
	}

	drivePracticeMobOwnerToBootstrapHPFloor(t, flow, owner, targetVID)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.MallPosition(0),
		Destination: itemproto.MallPosition(3),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected post-floor mall ITEM_MOVE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected post-floor mall ITEM_MOVE to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected post-floor mall ITEM_MOVE to queue no frames, got %d", len(queued))
	}
	assertPostFloorItemGuardAccountUnchanged(t, accounts, login, owner, "post-floor mall ITEM_MOVE")
	cells := runtime.mallCellsForCharacter(login, owner.ID)
	if item, ok := cells[0]; !ok || item.ID != 1910 || item.Count != 2 {
		t.Fatalf("expected post-floor mall ITEM_MOVE to leave seeded cell 0 unchanged, got %+v", cells)
	}
	if _, occupied := cells[3]; occupied {
		t.Fatalf("expected post-floor mall ITEM_MOVE not to occupy destination cell 3, got %+v", cells)
	}
}

func TestGameRuntimeMallItemUseWhileOpenFailsClosedWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("OpenMallUse", 0x010309ca, 0x020409ca, 1100, 2100, 0, 101, 201)
	owner.Gold = 4444
	owner.Inventory = []inventory.ItemInstance{{ID: 1811, Vnum: 27001, Count: 2, Slot: 5}}
	login := "open-mall-use"
	issuePeerTicket(t, ticketStore, login, 0x707079ca, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed open-mall item-use owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected open-mall item-use runtime error: %v", err)
	}
	runtime.SeedMallCellsForTest(login, owner.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 1911, Vnum: 27001, Count: 2, Slot: 0},
	})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x707079ca)
	defer closeSessionFlow(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall before mall item-use: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected /open_mall before mall item-use to emit MALL_OPEN plus one MALL_SET, got %d", len(openOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{
		Position: itemproto.MallPosition(0),
	})))
	if err != nil {
		t.Fatalf("unexpected mall ITEM_USE dispatch error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected mall ITEM_USE to fail closed with no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected mall ITEM_USE to queue no peer frames, got %d", len(queued))
	}
	assertExchangeAccountUnchanged(t, accounts, login, owner, "mall ITEM_USE owner")
	assertExchangeLiveStateUnchanged(t, runtime, owner, "mall ITEM_USE live owner")
	cells := runtime.mallCellsForCharacter(login, owner.ID)
	if item, ok := cells[0]; !ok || item.ID != 1911 || item.Count != 2 {
		t.Fatalf("expected mall ITEM_USE to leave seeded cell 0 unchanged, got %+v", cells)
	}

	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall reopen after mall item-use error: %v", err)
	}
	if len(reopenOut) != 2 {
		t.Fatalf("expected /open_mall reopen after mall item-use to emit MALL_OPEN plus remembered MALL_SET, got %d", len(reopenOut))
	}
	reopenSet, err := itemproto.DecodeMallSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode reopen MALL_SET after mall item-use: %v", err)
	}
	if reopenSet.Position != itemproto.MallPosition(0) || reopenSet.Vnum != 27001 || reopenSet.Count != 2 {
		t.Fatalf("unexpected reopen MALL_SET after mall item-use: %+v", reopenSet)
	}
}

func TestGameRuntimeMallCellsSurviveProcessRestartRematerializeOnOpen(t *testing.T) {
	defer safeboxstore.DisableDurableSyncForTest()()

	root := t.TempDir()
	ticketDir := filepath.Join(root, "tickets")
	accountDir := filepath.Join(root, "accounts")
	safeboxPath := filepath.Join(root, "safebox", "safebox.json")
	mallPath := safeboxstore.MallStorePathBesideSafebox(safeboxPath)
	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)
	owner := peerVisibilityCharacter("MallDurableRestart", 0x010309d1, 0x020409d1, 1100, 2100, 0, 101, 201)
	owner.Gold = 6161
	owner.Inventory = []inventory.ItemInstance{{ID: 1812, Vnum: 27001, Count: 2, Slot: 5}}
	login := "mall-durable-restart"
	const loginKey uint32 = 0x80808101
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed durable mall restart owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	cfg := config.Service{
		LegacyAddr:       ":13000",
		PublicAddr:       "127.0.0.1",
		SafeboxStorePath: safeboxPath,
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected durable mall restart runtime error: %v", err)
	}
	zeroSockets := inventory.SocketValues{}
	runtime.SeedMallCellsForTest(login, owner.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 1912, Vnum: 27001, Count: 2, Slot: 0},
		2: {ID: 1913, Vnum: 27001, Count: 1, Slot: 2, Sockets: &zeroSockets},
	})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall before process restart: %v", err)
	}
	if len(openOut) != 3 {
		t.Fatalf("expected /open_mall before process restart to emit MALL_OPEN plus two MALL_SET frames, got %d", len(openOut))
	}
	first, err := itemproto.DecodeMallSet(decodeSingleFrame(t, openOut[1]))
	if err != nil {
		t.Fatalf("decode first MALL_SET before process restart: %v", err)
	}
	if first.Position != itemproto.MallPosition(0) || first.Vnum != 27001 || first.Count != 2 {
		t.Fatalf("unexpected first MALL_SET before process restart: %+v", first)
	}
	second, err := itemproto.DecodeMallSet(decodeSingleFrame(t, openOut[2]))
	if err != nil {
		t.Fatalf("decode second MALL_SET before process restart: %v", err)
	}
	if second.Position != itemproto.MallPosition(2) || second.Vnum != 27001 || second.Count != 1 {
		t.Fatalf("unexpected second MALL_SET before process restart: %+v", second)
	}
	closeSessionFlow(t, flow)

	persisted, err := safeboxstore.NewMallFileStore(mallPath).Load()
	if err != nil {
		t.Fatalf("load durable mall snapshot after seed: %v", err)
	}
	cells := safeboxstore.MallCharacterCells(persisted, login, owner.ID)
	if cells[0].ID != 1912 || cells[2].ID != 1913 || len(cells) != 2 {
		t.Fatalf("unexpected durable mall cells after seed: %#v", cells)
	}
	if filepath.Dir(mallPath) == filepath.Dir(safeboxPath) {
		t.Fatal("mall FileStore must sit beside the safebox store directory")
	}

	const postRestartLoginKey uint32 = 0x80808111
	reloadedTickets := loginticket.NewFileStore(ticketDir)
	issuePeerTicket(t, reloadedTickets, login, postRestartLoginKey, owner)
	reloadedAccounts := accountstore.NewFileStore(accountDir)
	reloadedItems := newItemTemplateStore(t, []itemcatalog.Template{template})
	reloaded, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, reloadedTickets, reloadedAccounts, nil, nil, reloadedItems, nil)
	if err != nil {
		t.Fatalf("reload runtime after durable mall process restart: %v", err)
	}
	restartFlow, _ := enterGameWithLoginTicket(t, reloaded.SessionFactory(), login, postRestartLoginKey)
	defer closeSessionFlow(t, restartFlow)
	_ = flushServerFrames(t, restartFlow)

	reopenOut, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall after process restart: %v", err)
	}
	if len(reopenOut) != 3 {
		t.Fatalf("expected MALL_OPEN plus two remembered MALL_SET after process restart, got %d", len(reopenOut))
	}
	reopenFirst, err := itemproto.DecodeMallSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode first MALL_SET after process restart: %v", err)
	}
	if reopenFirst.Position != first.Position || reopenFirst.Vnum != first.Vnum || reopenFirst.Count != first.Count {
		t.Fatalf("unexpected first MALL_SET after process restart: %+v want %+v", reopenFirst, first)
	}
	reopenSecond, err := itemproto.DecodeMallSet(decodeSingleFrame(t, reopenOut[2]))
	if err != nil {
		t.Fatalf("decode second MALL_SET after process restart: %v", err)
	}
	if reopenSecond.Position != second.Position || reopenSecond.Vnum != second.Vnum || reopenSecond.Count != second.Count {
		t.Fatalf("unexpected second MALL_SET after process restart: %+v want %+v", reopenSecond, second)
	}
	if reopenSecond.Sockets != [itemproto.ItemSocketCount]int32{} {
		t.Fatalf("expected rematerialize MALL_SET to keep explicit-zero sockets, got %+v", reopenSecond.Sockets)
	}
}

func TestGameRuntimeMallCheckoutSurvivesProcessRestartWithoutRematerialize(t *testing.T) {
	defer safeboxstore.DisableDurableSyncForTest()()

	root := t.TempDir()
	ticketDir := filepath.Join(root, "tickets")
	accountDir := filepath.Join(root, "accounts")
	safeboxPath := filepath.Join(root, "safebox", "safebox.json")
	ticketStore := loginticket.NewFileStore(ticketDir)
	accounts := accountstore.NewFileStore(accountDir)
	owner := peerVisibilityCharacter("MallDurableCheckout", 0x010309d2, 0x020409d2, 1100, 2100, 0, 101, 201)
	owner.Gold = 4242
	owner.Inventory = []inventory.ItemInstance{{ID: 1813, Vnum: 27001, Count: 2, Slot: 5}}
	login := "mall-durable-checkout"
	const loginKey uint32 = 0x80808102
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed durable mall checkout owner account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	cfg := config.Service{
		LegacyAddr:       ":13000",
		PublicAddr:       "127.0.0.1",
		SafeboxStorePath: safeboxPath,
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected durable mall checkout runtime error: %v", err)
	}
	runtime.SeedMallCellsForTest(login, owner.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 1914, Vnum: 27001, Count: 2, Slot: 0},
	})
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	_ = flushServerFrames(t, flow)

	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	}))); err != nil {
		t.Fatalf("unexpected /open_mall before durable mall checkout: %v", err)
	}
	checkoutOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMallCheckout(itemproto.ClientMallCheckoutPacket{
		MallSlot: 0,
		Position: itemproto.InventoryPosition(9),
	})))
	if err != nil {
		t.Fatalf("unexpected durable mall checkout error: %v", err)
	}
	if len(checkoutOut) != 2 {
		t.Fatalf("expected durable mall checkout to emit MALL_DEL and ITEM_SET, got %d", len(checkoutOut))
	}
	closeSessionFlow(t, flow)

	const postRestartLoginKey uint32 = 0x80808112
	reloadedTickets := loginticket.NewFileStore(ticketDir)
	issuePeerTicket(t, reloadedTickets, login, postRestartLoginKey, owner)
	reloadedAccounts := accountstore.NewFileStore(accountDir)
	reloadedItems := newItemTemplateStore(t, []itemcatalog.Template{template})
	reloaded, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, reloadedTickets, reloadedAccounts, nil, nil, reloadedItems, nil)
	if err != nil {
		t.Fatalf("reload runtime after durable mall checkout restart: %v", err)
	}
	restartFlow, _ := enterGameWithLoginTicket(t, reloaded.SessionFactory(), login, postRestartLoginKey)
	defer closeSessionFlow(t, restartFlow)
	_ = flushServerFrames(t, restartFlow)

	reopenOut, err := restartFlow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_mall after durable mall checkout restart: %v", err)
	}
	if len(reopenOut) != 1 {
		t.Fatalf("expected /open_mall after durable mall checkout restart to emit only MALL_OPEN, got %d", len(reopenOut))
	}
	open, err := itemproto.DecodeMallOpen(decodeSingleFrame(t, reopenOut[0]))
	if err != nil {
		t.Fatalf("decode /open_mall after durable mall checkout restart: %v", err)
	}
	if open != (itemproto.MallOpenPacket{Size: 1}) {
		t.Fatalf("unexpected /open_mall after durable mall checkout restart: %+v", open)
	}
}

func TestGameRuntimeMallDoesNotLeakForeignCharacterRowsOnSameAccount(t *testing.T) {
	defer safeboxstore.DisableDurableSyncForTest()()

	root := t.TempDir()
	safeboxPath := filepath.Join(root, "safebox", "safebox.json")
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	charA := peerVisibilityCharacter("MallLeakA", 0x010309d3, 0x020409d3, 1100, 2100, 0, 101, 201)
	charA.Gold = 100
	charA.Inventory = []inventory.ItemInstance{{ID: 1814, Vnum: 27001, Count: 1, Slot: 5}}
	charB := peerVisibilityCharacter("MallLeakB", 0x010309d4, 0x020409d4, 1200, 2200, 0, 102, 202)
	charB.Gold = 200
	login := "mall-durable-leak"
	const loginKey uint32 = 0x80808103
	if err := ticketStore.Issue(loginticket.Ticket{
		Login:      login,
		LoginKey:   loginKey,
		Empire:     charA.Empire,
		Characters: cloneCharacters([]loginticket.Character{charA, charB}),
	}); err != nil {
		t.Fatalf("issue multi-character mall ticket: %v", err)
	}
	if err := accounts.Save(accountstore.Account{
		Login:      login,
		Empire:     charA.Empire,
		Characters: cloneCharacters([]loginticket.Character{charA, charB}),
	}); err != nil {
		t.Fatalf("seed multi-character mall account: %v", err)
	}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})
	cfg := config.Service{
		LegacyAddr:       ":13000",
		PublicAddr:       "127.0.0.1",
		SafeboxStorePath: safeboxPath,
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(cfg, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected durable mall leak runtime error: %v", err)
	}
	runtime.SeedMallCellsForTest(login, charA.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 1915, Vnum: 27001, Count: 2, Slot: 0},
	})

	flowA := runtime.SessionFactory()()
	_ = mustCompleteSecureHandshake(t, flowA)
	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: login, LoginKey: loginKey})
	if err != nil {
		t.Fatalf("encode login2 for mall char A: %v", err)
	}
	if _, err := flowA.HandleClientFrame(decodeSingleFrame(t, login2Raw)); err != nil {
		t.Fatalf("login mall char A: %v", err)
	}
	if _, err := flowA.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}))); err != nil {
		t.Fatalf("select mall char A: %v", err)
	}
	if _, err := flowA.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("enter mall char A: %v", err)
	}
	_ = flushServerFrames(t, flowA)
	openA, err := flowA.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("open mall on char A: %v", err)
	}
	if len(openA) != 2 {
		t.Fatalf("expected MALL_OPEN plus one MALL_SET on char A, got %d", len(openA))
	}
	closeSessionFlow(t, flowA)

	const loginKeyB uint32 = 0x80808104
	if err := ticketStore.Issue(loginticket.Ticket{
		Login:      login,
		LoginKey:   loginKeyB,
		Empire:     charA.Empire,
		Characters: cloneCharacters([]loginticket.Character{charA, charB}),
	}); err != nil {
		t.Fatalf("reissue ticket for mall char B: %v", err)
	}

	flowB := runtime.SessionFactory()()
	defer closeSessionFlow(t, flowB)
	_ = mustCompleteSecureHandshake(t, flowB)
	login2RawB, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: login, LoginKey: loginKeyB})
	if err != nil {
		t.Fatalf("encode login2 for mall char B: %v", err)
	}
	if _, err := flowB.HandleClientFrame(decodeSingleFrame(t, login2RawB)); err != nil {
		t.Fatalf("login mall char B: %v", err)
	}
	if _, err := flowB.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 1}))); err != nil {
		t.Fatalf("select mall char B: %v", err)
	}
	if _, err := flowB.HandleClientFrame(decodeSingleFrame(t, worldproto.EncodeEnterGame())); err != nil {
		t.Fatalf("enter mall char B: %v", err)
	}
	_ = flushServerFrames(t, flowB)

	openB, err := flowB.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_mall",
	})))
	if err != nil {
		t.Fatalf("open mall on char B: %v", err)
	}
	if len(openB) != 1 {
		t.Fatalf("expected only MALL_OPEN on char B (no leaked MALL_SET from char A), got %d frames", len(openB))
	}
	open, err := itemproto.DecodeMallOpen(decodeSingleFrame(t, openB[0]))
	if err != nil {
		t.Fatalf("decode char B MALL_OPEN: %v", err)
	}
	if open != (itemproto.MallOpenPacket{Size: 1}) {
		t.Fatalf("unexpected char B MALL_OPEN: %+v", open)
	}
}
