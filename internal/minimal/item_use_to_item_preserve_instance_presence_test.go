package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
)

func TestGameRuntimeItemUseToItemPartialPreservesInstanceSocketsAndAttributes(t *testing.T) {
	sourceActiveSockets := inventory.SocketValues{11, 0, -3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}
	templateSockets := itemcatalog.SocketValues{-11, 202, -303}
	templateAttributes := itemcatalog.AttributeValues{
		{Type: 12, Value: 34},
		{Type: 15, Value: -9},
	}

	cases := []struct {
		name              string
		sourceSockets     *inventory.SocketValues
		sourceAttributes  *inventory.AttributeValues
		destSockets       *inventory.SocketValues
		destAttributes    *inventory.AttributeValues
		wantSourceSockets [itemproto.ItemSocketCount]int32
		wantSourceAttrs   [itemproto.ItemAttributeCount]itemproto.Attribute
		wantDestSockets   [itemproto.ItemSocketCount]int32
		wantDestAttrs     [itemproto.ItemAttributeCount]itemproto.Attribute
	}{
		{
			name:              "active instance presence wins over template",
			sourceSockets:     &sourceActiveSockets,
			sourceAttributes:  &sourceActiveAttributes,
			destSockets:       &destActiveSockets,
			destAttributes:    &destActiveAttributes,
			wantSourceSockets: [itemproto.ItemSocketCount]int32{11, 0, -3},
			wantSourceAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 4, Value: 55},
				{Type: 9, Value: -7},
			},
			wantDestSockets: [itemproto.ItemSocketCount]int32{7, 0, 9},
			wantDestAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 1, Value: 25},
				{Type: 7, Value: -3},
			},
		},
		{
			name:              "explicit zero instance presence wins over template",
			sourceSockets:     &zeroSockets,
			sourceAttributes:  &zeroAttributes,
			destSockets:       &zeroSockets,
			destAttributes:    &zeroAttributes,
			wantSourceSockets: [itemproto.ItemSocketCount]int32{},
			wantSourceAttrs:   [itemproto.ItemAttributeCount]itemproto.Attribute{},
			wantDestSockets:   [itemproto.ItemSocketCount]int32{},
			wantDestAttrs:     [itemproto.ItemAttributeCount]itemproto.Attribute{},
		},
		{
			name:              "omitted instance keeps template fallback",
			wantSourceSockets: [itemproto.ItemSocketCount]int32{-11, 202, -303},
			wantSourceAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 12, Value: 34},
				{Type: 15, Value: -9},
			},
			wantDestSockets: [itemproto.ItemSocketCount]int32{-11, 202, -303},
			wantDestAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 12, Value: 34},
				{Type: 15, Value: -9},
			},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemPartialPresence", 0x01030ae0+uint32(i), 0x02040ae0+uint32(i), 1100, 2100, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{
				{ID: 941, Vnum: 27001, Count: 7, Slot: 5, Sockets: tc.sourceSockets, Attributes: tc.sourceAttributes},
				{ID: 942, Vnum: 27001, Count: 8, Slot: 6, Sockets: tc.destSockets, Attributes: tc.destAttributes},
			}
			login := "use-to-item-partial-presence-" + string(rune('a'+i))
			loginKey := uint32(0xa0a0a0e0 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed partial use-to-item presence owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:       27001,
				Name:       "UseToItem Presence Potion",
				Stackable:  true,
				MaxCount:   10,
				Sockets:    templateSockets,
				Attributes: templateAttributes,
			}})

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected partial use-to-item presence runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
				Source: itemproto.InventoryPosition(5),
				Target: itemproto.InventoryPosition(6),
			})))
			if err != nil {
				t.Fatalf("unexpected partial use-to-item presence packet error: %v", err)
			}
			if len(out) != 2 {
				t.Fatalf("expected source and target ITEM_UPDATE, got %d frames", len(out))
			}
			sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode partial use-to-item presence source update: %v", err)
			}
			if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 5 {
				t.Fatalf("unexpected partial use-to-item presence source update identity/count: %+v", sourceUpdate)
			}
			if sourceUpdate.Sockets != tc.wantSourceSockets {
				t.Fatalf("expected source ITEM_UPDATE sockets %+v, got %+v", tc.wantSourceSockets, sourceUpdate.Sockets)
			}
			if sourceUpdate.Attributes != tc.wantSourceAttrs {
				t.Fatalf("expected source ITEM_UPDATE attributes %+v, got %+v", tc.wantSourceAttrs, sourceUpdate.Attributes)
			}
			destUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode partial use-to-item presence destination update: %v", err)
			}
			if destUpdate.Position != itemproto.InventoryPosition(6) || destUpdate.Count != 10 {
				t.Fatalf("unexpected partial use-to-item presence destination update identity/count: %+v", destUpdate)
			}
			if destUpdate.Sockets != tc.wantDestSockets {
				t.Fatalf("expected destination ITEM_UPDATE sockets %+v, got %+v", tc.wantDestSockets, destUpdate.Sockets)
			}
			if destUpdate.Attributes != tc.wantDestAttrs {
				t.Fatalf("expected destination ITEM_UPDATE attributes %+v, got %+v", tc.wantDestAttrs, destUpdate.Attributes)
			}

			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load partial use-to-item presence account: %v", err)
			}
			persisted := findPersistedCharacter(t, account, owner.Name)
			if len(persisted.Inventory) != 2 {
				t.Fatalf("unexpected persisted inventory after partial use-to-item presence merge: %+v", persisted.Inventory)
			}
			assertPersistedUseToItemPresence(t, persisted.Inventory, 941, 5, 5, tc.sourceSockets, tc.sourceAttributes)
			assertPersistedUseToItemPresence(t, persisted.Inventory, 942, 6, 10, tc.destSockets, tc.destAttributes)
		})
	}
}

func TestGameRuntimeItemUseToItemFullMergeKeepsDestinationInstancePresence(t *testing.T) {
	sourceActiveSockets := inventory.SocketValues{11, 0, -3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	destActiveSockets := inventory.SocketValues{7, 0, 9}
	destActiveAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}
	templateSockets := itemcatalog.SocketValues{-11, 202, -303}
	templateAttributes := itemcatalog.AttributeValues{
		{Type: 12, Value: 34},
		{Type: 15, Value: -9},
	}

	cases := []struct {
		name             string
		sourceSockets    *inventory.SocketValues
		sourceAttributes *inventory.AttributeValues
		destSockets      *inventory.SocketValues
		destAttributes   *inventory.AttributeValues
		wantDestSockets  [itemproto.ItemSocketCount]int32
		wantDestAttrs    [itemproto.ItemAttributeCount]itemproto.Attribute
	}{
		{
			name:             "active destination wins over different source",
			sourceSockets:    &sourceActiveSockets,
			sourceAttributes: &sourceActiveAttributes,
			destSockets:      &destActiveSockets,
			destAttributes:   &destActiveAttributes,
			wantDestSockets:  [itemproto.ItemSocketCount]int32{7, 0, 9},
			wantDestAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 1, Value: 25},
				{Type: 7, Value: -3},
			},
		},
		{
			name:             "explicit zero destination wins over active source",
			sourceSockets:    &sourceActiveSockets,
			sourceAttributes: &sourceActiveAttributes,
			destSockets:      &zeroSockets,
			destAttributes:   &zeroAttributes,
			wantDestSockets:  [itemproto.ItemSocketCount]int32{},
			wantDestAttrs:    [itemproto.ItemAttributeCount]itemproto.Attribute{},
		},
		{
			name:             "omitted destination keeps template fallback",
			sourceSockets:    &sourceActiveSockets,
			sourceAttributes: &sourceActiveAttributes,
			wantDestSockets:  [itemproto.ItemSocketCount]int32{-11, 202, -303},
			wantDestAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 12, Value: 34},
				{Type: 15, Value: -9},
			},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemFullPresence", 0x01030af0+uint32(i), 0x02040af0+uint32(i), 1100, 2100, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{
				{ID: 951, Vnum: 27001, Count: 4, Slot: 5, Sockets: tc.sourceSockets, Attributes: tc.sourceAttributes},
				{ID: 952, Vnum: 27001, Count: 6, Slot: 6, Sockets: tc.destSockets, Attributes: tc.destAttributes},
			}
			login := "use-to-item-full-presence-" + string(rune('a'+i))
			loginKey := uint32(0xa0a0a0f0 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed full use-to-item presence owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:       27001,
				Name:       "UseToItem Presence Potion",
				Stackable:  true,
				MaxCount:   10,
				Sockets:    templateSockets,
				Attributes: templateAttributes,
			}})

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected full use-to-item presence runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
				Source: itemproto.InventoryPosition(5),
				Target: itemproto.InventoryPosition(6),
			})))
			if err != nil {
				t.Fatalf("unexpected full use-to-item presence packet error: %v", err)
			}
			if len(out) != 2 {
				t.Fatalf("expected source ITEM_DEL and destination ITEM_UPDATE, got %d frames", len(out))
			}
			sourceDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode full use-to-item presence source delete: %v", err)
			}
			if sourceDel.Position != itemproto.InventoryPosition(5) {
				t.Fatalf("unexpected full use-to-item presence source delete: %+v", sourceDel)
			}
			destUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode full use-to-item presence destination update: %v", err)
			}
			if destUpdate.Position != itemproto.InventoryPosition(6) || destUpdate.Count != 10 {
				t.Fatalf("unexpected full use-to-item presence destination update identity/count: %+v", destUpdate)
			}
			if destUpdate.Sockets != tc.wantDestSockets {
				t.Fatalf("expected destination ITEM_UPDATE sockets %+v, got %+v", tc.wantDestSockets, destUpdate.Sockets)
			}
			if destUpdate.Attributes != tc.wantDestAttrs {
				t.Fatalf("expected destination ITEM_UPDATE attributes %+v, got %+v", tc.wantDestAttrs, destUpdate.Attributes)
			}

			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load full use-to-item presence account: %v", err)
			}
			persisted := findPersistedCharacter(t, account, owner.Name)
			if len(persisted.Inventory) != 1 {
				t.Fatalf("unexpected persisted inventory after full use-to-item presence merge: %+v", persisted.Inventory)
			}
			assertPersistedUseToItemPresence(t, persisted.Inventory, 952, 6, 10, tc.destSockets, tc.destAttributes)
		})
	}
}

func assertPersistedUseToItemPresence(
	t *testing.T,
	items []inventory.ItemInstance,
	id uint64,
	slot inventory.SlotIndex,
	count uint16,
	wantSockets *inventory.SocketValues,
	wantAttributes *inventory.AttributeValues,
) {
	t.Helper()
	var got *inventory.ItemInstance
	for i := range items {
		if items[i].ID == id {
			got = &items[i]
			break
		}
	}
	if got == nil || got.Vnum != 27001 || got.Slot != slot || got.Count != count {
		t.Fatalf("unexpected persisted item id=%d slot=%d count=%d: %+v", id, slot, count, items)
	}
	if (wantSockets != nil) != got.HasSockets() {
		t.Fatalf("persisted id=%d HasSockets=%v want %v", id, got.HasSockets(), wantSockets != nil)
	}
	if wantSockets != nil {
		if got.Sockets == nil || *got.Sockets != *wantSockets {
			t.Fatalf("expected persisted id=%d sockets %+v, got %#v", id, *wantSockets, got.Sockets)
		}
	} else if got.Sockets != nil {
		t.Fatalf("expected omitted persisted id=%d sockets, got %#v", id, got.Sockets)
	}
	if (wantAttributes != nil) != got.HasAttributes() {
		t.Fatalf("persisted id=%d HasAttributes=%v want %v", id, got.HasAttributes(), wantAttributes != nil)
	}
	if wantAttributes != nil {
		if got.Attributes == nil || *got.Attributes != *wantAttributes {
			t.Fatalf("expected persisted id=%d attributes %+v, got %#v", id, *wantAttributes, got.Attributes)
		}
	} else if got.Attributes != nil {
		t.Fatalf("expected omitted persisted id=%d attributes, got %#v", id, got.Attributes)
	}
}
