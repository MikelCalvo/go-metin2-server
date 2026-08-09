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
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
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

func TestGameRuntimeItemExchangeAntiGiveItemAddReturnsAuthoredRejectTextWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeBound", 0x01030761, 0x02040761, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 702, Vnum: 27043, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-exchange-bound", 0x70707061, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-bound", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed bound item-exchange account: %v", err)
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
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-bound", 0x70707061)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
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
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after anti-give EXCHANGE rejection, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-exchange-bound")
	if err != nil {
		t.Fatalf("load persisted bound item-exchange account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("anti-give EXCHANGE mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("anti-give EXCHANGE mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Gold != owner.Gold {
		t.Fatalf("anti-give EXCHANGE mutated gold: got %d want %d", persisted.Characters[0].Gold, owner.Gold)
	}
}

func TestGameRuntimeItemExchangeAntiGiveRejectTextRequiresValidDisplaySlotWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("ExchangeSlotGuard", 0x01030762, 0x02040762, 1100, 2100, 0, 101, 201)
	owner.Gold = 12345
	owner.Inventory = []inventory.ItemInstance{{ID: 703, Vnum: 27043, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "item-exchange-slot-guard", 0x70707062, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-exchange-slot-guard", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed display-slot guarded item-exchange account: %v", err)
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
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-exchange-slot-guard", 0x70707062)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
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
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after out-of-range EXCHANGE display slot, got %d", len(queued))
	}
	persisted, err := accounts.Load("item-exchange-slot-guard")
	if err != nil {
		t.Fatalf("load display-slot guarded item-exchange account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("out-of-range EXCHANGE display slot mutated inventory: got %+v want %+v", persisted.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("out-of-range EXCHANGE display slot mutated quickslots: got %+v want %+v", persisted.Characters[0].Quickslots, owner.Quickslots)
	}
	if persisted.Characters[0].Gold != owner.Gold {
		t.Fatalf("out-of-range EXCHANGE display slot mutated gold: got %d want %d", persisted.Characters[0].Gold, owner.Gold)
	}
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
