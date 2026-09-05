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

func TestGameRuntimeItemPickupFreeCellPreservesInstanceSocketsAndAttributes(t *testing.T) {
	activeSockets := inventory.SocketValues{11, 0, -3}
	activeAttributes := inventory.AttributeValues{{Type: 4, Value: 55}, {Type: 9, Value: -7}}
	zeroSockets := inventory.SocketValues{}
	zeroAttributes := inventory.AttributeValues{}
	templateSockets := itemcatalog.SocketValues{-11, 202, -303}
	templateAttributes := itemcatalog.AttributeValues{
		{Type: 12, Value: 34},
		{Type: 15, Value: -9},
	}

	cases := []struct {
		name           string
		sockets        *inventory.SocketValues
		attributes     *inventory.AttributeValues
		wantSockets    [itemproto.ItemSocketCount]int32
		wantAttributes [itemproto.ItemAttributeCount]itemproto.Attribute
	}{
		{
			name:        "active instance presence wins over template",
			sockets:     &activeSockets,
			attributes:  &activeAttributes,
			wantSockets: [itemproto.ItemSocketCount]int32{11, 0, -3},
			wantAttributes: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 4, Value: 55},
				{Type: 9, Value: -7},
			},
		},
		{
			name:           "explicit zero instance presence wins over template",
			sockets:        &zeroSockets,
			attributes:     &zeroAttributes,
			wantSockets:    [itemproto.ItemSocketCount]int32{},
			wantAttributes: [itemproto.ItemAttributeCount]itemproto.Attribute{},
		},
		{
			name:        "omitted instance keeps template fallback",
			wantSockets: [itemproto.ItemSocketCount]int32{-11, 202, -303},
			wantAttributes: [itemproto.ItemAttributeCount]itemproto.Attribute{
				{Type: 12, Value: 34},
				{Type: 15, Value: -9},
			},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("PickupFreeCellPresence", 0x010309f0+uint32(i), 0x020409f0+uint32(i), 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{
				ID:         951,
				Vnum:       27002,
				Count:      4,
				Slot:       5,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}}
			login := "pickup-free-cell-presence-" + string(rune('a'+i))
			loginKey := uint32(0xf0f0f0f0 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed free-cell pickup presence owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:       27002,
				Name:       "Pickup Presence Potion",
				Stackable:  true,
				MaxCount:   200,
				Sockets:    templateSockets,
				Attributes: templateAttributes,
			}})

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected free-cell pickup presence runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)

			ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(5))
			pickupOut := pickupGroundItem(t, flow, ground.VID)
			if len(pickupOut) != 3 {
				t.Fatalf("expected pickup to emit GROUND_DEL, ITEM_SET, and ITEM_GET, got %d frames", len(pickupOut))
			}
			set, err := itemproto.DecodeSet(decodeSingleFrame(t, pickupOut[1]))
			if err != nil {
				t.Fatalf("decode free-cell pickup presence item set: %v", err)
			}
			if set.Position != itemproto.InventoryPosition(5) || set.Vnum != 27002 || set.Count != 4 {
				t.Fatalf("unexpected free-cell pickup presence item set identity/count: %+v", set)
			}
			if set.Sockets != tc.wantSockets {
				t.Fatalf("expected free-cell pickup ITEM_SET sockets %+v, got %+v", tc.wantSockets, set.Sockets)
			}
			if set.Attributes != tc.wantAttributes {
				t.Fatalf("expected free-cell pickup ITEM_SET attributes %+v, got %+v", tc.wantAttributes, set.Attributes)
			}

			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load free-cell pickup presence account: %v", err)
			}
			persisted := findPersistedCharacter(t, account, owner.Name)
			if len(persisted.Inventory) != 1 || persisted.Inventory[0].ID != 951 || persisted.Inventory[0].Count != 4 || persisted.Inventory[0].Slot != 5 {
				t.Fatalf("unexpected persisted free-cell pickup presence character: %+v", persisted)
			}
			if (tc.sockets != nil) != persisted.Inventory[0].HasSockets() {
				t.Fatalf("persisted HasSockets=%v want %v", persisted.Inventory[0].HasSockets(), tc.sockets != nil)
			}
			if tc.sockets != nil {
				if persisted.Inventory[0].Sockets == nil || *persisted.Inventory[0].Sockets != *tc.sockets {
					t.Fatalf("expected persisted placement sockets %+v, got %#v", *tc.sockets, persisted.Inventory[0].Sockets)
				}
			} else if persisted.Inventory[0].Sockets != nil {
				t.Fatalf("expected omitted persisted placement sockets, got %#v", persisted.Inventory[0].Sockets)
			}
			if (tc.attributes != nil) != persisted.Inventory[0].HasAttributes() {
				t.Fatalf("persisted HasAttributes=%v want %v", persisted.Inventory[0].HasAttributes(), tc.attributes != nil)
			}
			if tc.attributes != nil {
				if persisted.Inventory[0].Attributes == nil || *persisted.Inventory[0].Attributes != *tc.attributes {
					t.Fatalf("expected persisted placement attributes %+v, got %#v", *tc.attributes, persisted.Inventory[0].Attributes)
				}
			} else if persisted.Inventory[0].Attributes != nil {
				t.Fatalf("expected omitted persisted placement attributes, got %#v", persisted.Inventory[0].Attributes)
			}
		})
	}
}
