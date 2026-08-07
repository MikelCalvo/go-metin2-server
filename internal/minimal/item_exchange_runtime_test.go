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
