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

func TestGameRuntimeItemMoveCountedPartialSplitPreservesInstanceSocketsAndAttributes(t *testing.T) {
	sourceActiveSockets := inventory.SocketValues{11, 0, -3}
	sourceActiveAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
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
		wantSourceSockets [itemproto.ItemSocketCount]int32
		wantSourceAttrs   [itemproto.ItemAttributeCount]itemproto.Attribute
	}{
		{
			name:              "active instance presence wins over template",
			sourceSockets:     &sourceActiveSockets,
			sourceAttributes:  &sourceActiveAttributes,
			wantSourceSockets: [itemproto.ItemSocketCount]int32{11, 0, -3},
			wantSourceAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 4, Value: 55},
				{Type: 9, Value: -7},
			},
		},
		{
			name:              "explicit zero instance presence wins over template",
			sourceSockets:     &zeroSockets,
			sourceAttributes:  &zeroAttributes,
			wantSourceSockets: [itemproto.ItemSocketCount]int32{},
			wantSourceAttrs:   [itemproto.ItemAttributeCount]itemproto.Attribute{},
		},
		{
			name:              "omitted instance keeps template fallback",
			wantSourceSockets: [itemproto.ItemSocketCount]int32{-11, 202, -303},
			wantSourceAttrs: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 12, Value: 34},
				{Type: 15, Value: -9},
			},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("MovePartialSplitPresence", 0x01030990+uint32(i), 0x02040990+uint32(i), 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{
				{ID: 1990, Vnum: 27001, Count: 5, Slot: 5, Sockets: tc.sourceSockets, Attributes: tc.sourceAttributes},
			}
			login := "move-partial-split-presence-" + string(rune('a'+i))
			loginKey := uint32(0xb0b0c010 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed partial item-move split presence owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:       27001,
				Name:       "Move Partial Split Presence Potion",
				Stackable:  true,
				MaxCount:   200,
				Sockets:    templateSockets,
				Attributes: templateAttributes,
			}})

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected partial item-move split presence runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
				Source:      itemproto.InventoryPosition(5),
				Destination: itemproto.InventoryPosition(8),
				Count:       2,
			})))
			if err != nil {
				t.Fatalf("unexpected partial item-move split presence packet error: %v", err)
			}
			if len(out) != 2 {
				t.Fatalf("expected source and destination ITEM_SET, got %d frames", len(out))
			}
			sourceSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode partial item-move split presence source set: %v", err)
			}
			if sourceSet.Position != itemproto.InventoryPosition(5) || sourceSet.Vnum != 27001 || sourceSet.Count != 3 {
				t.Fatalf("unexpected partial item-move split presence source set identity/count: %+v", sourceSet)
			}
			if sourceSet.Sockets != tc.wantSourceSockets {
				t.Fatalf("expected source ITEM_SET sockets %+v, got %+v", tc.wantSourceSockets, sourceSet.Sockets)
			}
			if sourceSet.Attributes != tc.wantSourceAttrs {
				t.Fatalf("expected source ITEM_SET attributes %+v, got %+v", tc.wantSourceAttrs, sourceSet.Attributes)
			}
			destSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode partial item-move split presence destination set: %v", err)
			}
			if destSet.Position != itemproto.InventoryPosition(8) || destSet.Vnum != 27001 || destSet.Count != 2 {
				t.Fatalf("unexpected partial item-move split presence destination set identity/count: %+v", destSet)
			}
			if destSet.Sockets != tc.wantSourceSockets {
				t.Fatalf("expected destination ITEM_SET sockets %+v, got %+v", tc.wantSourceSockets, destSet.Sockets)
			}
			if destSet.Attributes != tc.wantSourceAttrs {
				t.Fatalf("expected destination ITEM_SET attributes %+v, got %+v", tc.wantSourceAttrs, destSet.Attributes)
			}

			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load partial item-move split presence account: %v", err)
			}
			persisted := findPersistedCharacter(t, account, owner.Name)
			if len(persisted.Inventory) != 2 {
				t.Fatalf("unexpected persisted inventory after partial item-move split presence: %+v", persisted.Inventory)
			}
			assertPersistedItemMovePartialMergePresence(t, persisted.Inventory, 1990, 5, 3, tc.sourceSockets, tc.sourceAttributes)
			var remainder, split inventory.ItemInstance
			for _, item := range persisted.Inventory {
				switch item.Slot {
				case 5:
					remainder = item
				case 8:
					split = item
				}
			}
			if remainder.ID != 1990 || remainder.Count != 3 {
				t.Fatalf("unexpected persisted remainder: %+v", remainder)
			}
			if split.ID == 0 || split.ID == 1990 || split.Count != 2 || split.Vnum != 27001 {
				t.Fatalf("expected fresh persisted split identity, got %+v", split)
			}
			assertPersistedItemMovePartialMergePresence(t, persisted.Inventory, split.ID, 8, 2, tc.sourceSockets, tc.sourceAttributes)
			if remainder.HasSockets() && split.HasSockets() && remainder.Sockets == split.Sockets {
				t.Fatal("expected persisted remainder sockets pointer to be independent of destination")
			}
			if remainder.HasAttributes() && split.HasAttributes() && remainder.Attributes == split.Attributes {
				t.Fatal("expected persisted remainder attributes pointer to be independent of destination")
			}
		})
	}
}
