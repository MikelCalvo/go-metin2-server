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

func TestGameRuntimeItemMovePartialMergePreservesInstanceSocketsAndAttributes(t *testing.T) {
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
			owner := peerVisibilityCharacter("MovePartialMergePresence", 0x01030b10+uint32(i), 0x02040b10+uint32(i), 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{
				{ID: 961, Vnum: 27001, Count: 4, Slot: 5, Sockets: tc.sourceSockets, Attributes: tc.sourceAttributes},
				{ID: 962, Vnum: 27001, Count: 8, Slot: 8, Sockets: tc.destSockets, Attributes: tc.destAttributes},
			}
			login := "move-partial-merge-presence-" + string(rune('a'+i))
			loginKey := uint32(0xb0b0b010 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed partial item-move presence owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:       27001,
				Name:       "Move Partial Merge Presence Potion",
				Stackable:  true,
				MaxCount:   10,
				Sockets:    templateSockets,
				Attributes: templateAttributes,
			}})

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected partial item-move presence runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
				Source:      itemproto.InventoryPosition(5),
				Destination: itemproto.InventoryPosition(8),
				Count:       2,
			})))
			if err != nil {
				t.Fatalf("unexpected partial item-move presence packet error: %v", err)
			}
			if len(out) != 2 {
				t.Fatalf("expected source and destination ITEM_UPDATE, got %d frames", len(out))
			}
			sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode partial item-move presence source update: %v", err)
			}
			if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 2 {
				t.Fatalf("unexpected partial item-move presence source update identity/count: %+v", sourceUpdate)
			}
			if sourceUpdate.Sockets != tc.wantSourceSockets {
				t.Fatalf("expected source ITEM_UPDATE sockets %+v, got %+v", tc.wantSourceSockets, sourceUpdate.Sockets)
			}
			if sourceUpdate.Attributes != tc.wantSourceAttrs {
				t.Fatalf("expected source ITEM_UPDATE attributes %+v, got %+v", tc.wantSourceAttrs, sourceUpdate.Attributes)
			}
			destUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode partial item-move presence destination update: %v", err)
			}
			if destUpdate.Position != itemproto.InventoryPosition(8) || destUpdate.Count != 10 {
				t.Fatalf("unexpected partial item-move presence destination update identity/count: %+v", destUpdate)
			}
			if destUpdate.Sockets != tc.wantDestSockets {
				t.Fatalf("expected destination ITEM_UPDATE sockets %+v, got %+v", tc.wantDestSockets, destUpdate.Sockets)
			}
			if destUpdate.Attributes != tc.wantDestAttrs {
				t.Fatalf("expected destination ITEM_UPDATE attributes %+v, got %+v", tc.wantDestAttrs, destUpdate.Attributes)
			}

			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load partial item-move presence account: %v", err)
			}
			persisted := findPersistedCharacter(t, account, owner.Name)
			if len(persisted.Inventory) != 2 {
				t.Fatalf("unexpected persisted inventory after partial item-move presence merge: %+v", persisted.Inventory)
			}
			assertPersistedItemMovePartialMergePresence(t, persisted.Inventory, 961, 5, 2, tc.sourceSockets, tc.sourceAttributes)
			assertPersistedItemMovePartialMergePresence(t, persisted.Inventory, 962, 8, 10, tc.destSockets, tc.destAttributes)
		})
	}
}

func assertPersistedItemMovePartialMergePresence(
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
