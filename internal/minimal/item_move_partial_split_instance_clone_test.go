package minimal

import (
	"path/filepath"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
)

func TestGameRuntimeItemMoveCountedPartialSplitClonesInstanceSocketsAndAttributesIndependently(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:       27001,
		Name:       "Small Red Potion",
		Stackable:  true,
		MaxCount:   200,
		Sockets:    itemcatalog.SocketValues{91, 92, 93},
		Attributes: itemcatalog.AttributeValues{{Type: 8, Value: 44}},
	}}}); err != nil {
		t.Fatalf("seed counted split clone item-move template: %v", err)
	}

	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	owner := peerVisibilityCharacter("ItemMoveSplitClone", 0x01030982, 0x02040982, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{
		ID:         1982,
		Vnum:       27001,
		Count:      5,
		Slot:       5,
		Sockets:    &activeSockets,
		Attributes: &activeAttributes,
	}}
	login := "item-move-split-clone"
	issuePeerTicket(t, ticketStore, login, 0x82828282, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed counted split clone item-move owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected counted split clone item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x82828282)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(8),
		Count:       2,
	})))
	if err != nil {
		t.Fatalf("unexpected counted split clone item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected counted split item move to emit only source and destination ITEM_SET, got %d", len(out))
	}

	sourceSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode counted split clone source set: %v", err)
	}
	if sourceSet.Position != itemproto.InventoryPosition(5) || sourceSet.Vnum != 27001 || sourceSet.Count != 3 {
		t.Fatalf("unexpected counted split clone source refresh: %+v", sourceSet)
	}
	if sourceSet.Sockets != ([itemproto.ItemSocketCount]int32{11, 0, -3}) {
		t.Fatalf("expected source ITEM_SET sockets to keep instance presence, got %+v", sourceSet.Sockets)
	}
	if sourceSet.Attributes[0] != (itemproto.Attribute{Type: 4, Value: 55}) || sourceSet.Attributes[1] != (itemproto.Attribute{Type: 9, Value: -7}) {
		t.Fatalf("expected source ITEM_SET attributes to keep instance presence, got %+v", sourceSet.Attributes)
	}

	destinationSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode counted split clone destination set: %v", err)
	}
	if destinationSet.Position != itemproto.InventoryPosition(8) || destinationSet.Vnum != 27001 || destinationSet.Count != 2 {
		t.Fatalf("unexpected counted split clone destination refresh: %+v", destinationSet)
	}
	if destinationSet.Sockets != ([itemproto.ItemSocketCount]int32{11, 0, -3}) {
		t.Fatalf("expected destination ITEM_SET sockets to clone instance presence, got %+v", destinationSet.Sockets)
	}
	if destinationSet.Attributes[0] != (itemproto.Attribute{Type: 4, Value: 55}) || destinationSet.Attributes[1] != (itemproto.Attribute{Type: 9, Value: -7}) {
		t.Fatalf("expected destination ITEM_SET attributes to clone instance presence, got %+v", destinationSet.Attributes)
	}

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load counted split clone item-move account: %v", err)
	}
	if len(account.Characters[0].Inventory) != 2 {
		t.Fatalf("expected split item-move to persist two carried stacks, got %#v", account.Characters[0].Inventory)
	}
	var remainder, split inventory.ItemInstance
	for _, item := range account.Characters[0].Inventory {
		switch item.Slot {
		case 5:
			remainder = item
		case 8:
			split = item
		}
	}
	if remainder.ID != 1982 || remainder.Count != 3 {
		t.Fatalf("unexpected persisted remainder: %+v", remainder)
	}
	if split.ID == 0 || split.ID == 1982 || split.Count != 2 || split.Vnum != 27001 {
		t.Fatalf("expected fresh persisted split identity, got %+v", split)
	}
	if !remainder.HasSockets() || remainder.Sockets == nil || *remainder.Sockets != activeSockets {
		t.Fatalf("expected persisted remainder sockets %+v, got %#v", activeSockets, remainder.Sockets)
	}
	if !split.HasSockets() || split.Sockets == nil || *split.Sockets != activeSockets {
		t.Fatalf("expected persisted split sockets %+v, got %#v", activeSockets, split.Sockets)
	}
	if split.Sockets == remainder.Sockets {
		t.Fatal("expected persisted split sockets pointer to be independent of remainder")
	}
	if !remainder.HasAttributes() || remainder.Attributes == nil || *remainder.Attributes != activeAttributes {
		t.Fatalf("expected persisted remainder attributes %+v, got %#v", activeAttributes, remainder.Attributes)
	}
	if !split.HasAttributes() || split.Attributes == nil || *split.Attributes != activeAttributes {
		t.Fatalf("expected persisted split attributes %+v, got %#v", activeAttributes, split.Attributes)
	}
	if split.Attributes == remainder.Attributes {
		t.Fatal("expected persisted split attributes pointer to be independent of remainder")
	}

	(*split.Sockets)[0] = 99
	(*split.Attributes)[0].Value = 99
	if (*remainder.Sockets)[0] == 99 {
		t.Fatal("mutating persisted split sockets aliased the remainder")
	}
	if (*remainder.Attributes)[0].Value == 99 {
		t.Fatal("mutating persisted split attributes aliased the remainder")
	}
}

func TestGameRuntimeItemMoveCountedPartialSplitOmitsInstancePresenceIndependently(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:       27001,
		Name:       "Small Red Potion",
		Stackable:  true,
		MaxCount:   200,
		Sockets:    itemcatalog.SocketValues{91, 92, 93},
		Attributes: itemcatalog.AttributeValues{{Type: 8, Value: 44}},
	}}}); err != nil {
		t.Fatalf("seed omitted split clone item-move template: %v", err)
	}

	owner := peerVisibilityCharacter("ItemMoveSplitOmit", 0x01030983, 0x02040983, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1983, Vnum: 27001, Count: 4, Slot: 5}}
	login := "item-move-split-omit"
	issuePeerTicket(t, ticketStore, login, 0x83838383, owner)
	if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed omitted split clone item-move owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected omitted split clone item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x83838383)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(7),
		Count:       1,
	})))
	if err != nil {
		t.Fatalf("unexpected omitted split clone item-move error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected omitted split item move to emit only source and destination ITEM_SET, got %d", len(out))
	}

	sourceSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode omitted split source set: %v", err)
	}
	destinationSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode omitted split destination set: %v", err)
	}
	if sourceSet.Sockets != ([itemproto.ItemSocketCount]int32{91, 92, 93}) || destinationSet.Sockets != ([itemproto.ItemSocketCount]int32{91, 92, 93}) {
		t.Fatalf("expected omit→template socket encode on both ITEM_SET frames, source=%+v destination=%+v", sourceSet.Sockets, destinationSet.Sockets)
	}
	if sourceSet.Attributes[0] != (itemproto.Attribute{Type: 8, Value: 44}) || destinationSet.Attributes[0] != (itemproto.Attribute{Type: 8, Value: 44}) {
		t.Fatalf("expected omit→template attribute encode on both ITEM_SET frames, source=%+v destination=%+v", sourceSet.Attributes, destinationSet.Attributes)
	}

	account, err := accounts.Load(login)
	if err != nil {
		t.Fatalf("load omitted split clone item-move account: %v", err)
	}
	for _, item := range account.Characters[0].Inventory {
		if item.HasSockets() || item.HasAttributes() {
			t.Fatalf("expected omitted instance presence to stay omitted after split, got %+v", item)
		}
	}
}
