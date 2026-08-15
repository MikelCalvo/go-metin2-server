package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
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
