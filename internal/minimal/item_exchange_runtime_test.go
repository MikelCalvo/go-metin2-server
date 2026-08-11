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
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil, nil)
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
	if len(peerAcceptOut) != 1 {
		t.Fatalf("expected peer exchange accept to emit one self frame, got %d", len(peerAcceptOut))
	}
	assertExchangeAcceptFrame(t, peerAcceptOut[0], 1, "peer accept self response")
	queuedPeerAccept := flushServerFrames(t, ownerFlow)
	if len(queuedPeerAccept) != 1 {
		t.Fatalf("expected peer exchange accept to queue one owner frame, got %d", len(queuedPeerAccept))
	}
	assertExchangeAcceptFrame(t, queuedPeerAccept[0], 0, "peer accept owner response")

	cancelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderCancel})))
	if err != nil {
		t.Fatalf("unexpected exchange cancel after accept error: %v", err)
	}
	if len(cancelOut) != 1 {
		t.Fatalf("expected accepted display shell to remain cancellable, got %d cancel frames", len(cancelOut))
	}
	assertExchangeEndFrame(t, cancelOut[0], "owner cancel after accept")
	queuedCancel := flushServerFrames(t, peerFlow)
	if len(queuedCancel) != 1 {
		t.Fatalf("expected owner cancel after accept to queue one peer end frame, got %d", len(queuedCancel))
	}
	assertExchangeEndFrame(t, queuedCancel[0], "peer queued cancel after accept")

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

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected peer accept before reset error: %v", err)
	}
	if len(peerAcceptOut) != 1 {
		t.Fatalf("expected peer accept before reset to emit one frame, got %d", len(peerAcceptOut))
	}
	assertExchangeAcceptFrame(t, peerAcceptOut[0], 1, "peer accept before reset")
	queuedPeerAccept := flushServerFrames(t, ownerFlow)
	if len(queuedPeerAccept) != 1 {
		t.Fatalf("expected peer accept before reset to queue one owner frame, got %d", len(queuedPeerAccept))
	}
	assertExchangeAcceptFrame(t, queuedPeerAccept[0], 0, "peer accept before reset owner")

	itemAddOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{
		Subheader: itemproto.ExchangeSubheaderItemAdd,
		Arg2:      7,
		Position:  itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected accepted exchange item-add reset error: %v", err)
	}
	if len(itemAddOut) != 3 {
		t.Fatalf("expected accepted exchange item-add to emit item display plus two accept resets, got %d frames", len(itemAddOut))
	}
	assertExchangeAcceptFrameWithValue(t, itemAddOut[0], 1, 0, "accepted-reset owner-side self marker")
	assertExchangeAcceptFrameWithValue(t, itemAddOut[1], 0, 0, "accepted-reset peer-side self marker")
	assertExchangeItemAddFrame(t, itemAddOut[2], 1, 7, owner.Inventory[0], template, "accepted-reset item-add self response")
	queuedItemAdd := flushServerFrames(t, peerFlow)
	if len(queuedItemAdd) != 3 {
		t.Fatalf("expected accepted exchange item-add to queue display plus two accept resets, got %d frames", len(queuedItemAdd))
	}
	assertExchangeAcceptFrameWithValue(t, queuedItemAdd[0], 0, 0, "accepted-reset owner-side peer marker")
	assertExchangeAcceptFrameWithValue(t, queuedItemAdd[1], 1, 0, "accepted-reset peer-side peer marker")
	assertExchangeItemAddFrame(t, queuedItemAdd[2], 0, 7, owner.Inventory[0], template, "accepted-reset item-add peer response")

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

	peerAcceptOut, err := peerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderAccept})))
	if err != nil {
		t.Fatalf("unexpected peer accept before item-del reset error: %v", err)
	}
	if len(peerAcceptOut) != 1 {
		t.Fatalf("expected peer accept before item-del reset to emit one frame, got %d", len(peerAcceptOut))
	}
	assertExchangeAcceptFrame(t, peerAcceptOut[0], 1, "peer accept before item-del reset")
	queuedPeerAccept := flushServerFrames(t, ownerFlow)
	if len(queuedPeerAccept) != 1 {
		t.Fatalf("expected peer accept before item-del reset to queue one owner frame, got %d", len(queuedPeerAccept))
	}
	assertExchangeAcceptFrame(t, queuedPeerAccept[0], 0, "peer accept before item-del reset owner")

	itemDelOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientExchange(itemproto.ClientExchangePacket{Subheader: itemproto.ExchangeSubheaderItemDel, Arg1: 7})))
	if err != nil {
		t.Fatalf("unexpected accepted exchange item-del reset error: %v", err)
	}
	if len(itemDelOut) != 3 {
		t.Fatalf("expected accepted exchange item-del to emit item-del plus two accept resets, got %d frames", len(itemDelOut))
	}
	assertExchangeAcceptFrameWithValue(t, itemDelOut[0], 1, 0, "item-del reset owner-side self marker")
	assertExchangeAcceptFrameWithValue(t, itemDelOut[1], 0, 0, "item-del reset peer-side self marker")
	assertExchangeItemDelFrame(t, itemDelOut[2], 1, 7, "item-del reset self response")
	queuedItemDel := flushServerFrames(t, peerFlow)
	if len(queuedItemDel) != 3 {
		t.Fatalf("expected accepted exchange item-del to queue item-del plus two accept resets, got %d frames", len(queuedItemDel))
	}
	assertExchangeAcceptFrameWithValue(t, queuedItemDel[0], 0, 0, "item-del reset owner-side peer marker")
	assertExchangeAcceptFrameWithValue(t, queuedItemDel[1], 1, 0, "item-del reset peer-side peer marker")
	assertExchangeItemDelFrame(t, queuedItemDel[2], 0, 7, "item-del reset peer response")

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
