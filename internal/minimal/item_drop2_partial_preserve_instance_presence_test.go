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

func TestGameRuntimeItemDrop2PartialPreservesInstanceSocketsAndAttributes(t *testing.T) {
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
			owner := peerVisibilityCharacter("DropPartialPresence", 0x010309d0+uint32(i), 0x020409d0+uint32(i), 1250, 2250, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{
				ID:         931,
				Vnum:       27001,
				Count:      5,
				Slot:       5,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}}
			login := "drop-partial-presence-" + string(rune('a'+i))
			loginKey := uint32(0xd0d0d0d0 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed partial-drop remainder presence owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:       27001,
				Name:       "Drop Presence Potion",
				Stackable:  true,
				MaxCount:   200,
				Sockets:    templateSockets,
				Attributes: templateAttributes,
			}})

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected partial-drop remainder presence runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 2})))
			if err != nil {
				t.Fatalf("unexpected partial-drop remainder presence drop2 error: %v", err)
			}
			if len(out) != 3 {
				t.Fatalf("expected partial drop to emit ITEM_UPDATE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
			}
			update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode partial-drop remainder presence update: %v", err)
			}
			if update.Position != itemproto.InventoryPosition(5) || update.Count != 3 {
				t.Fatalf("unexpected partial-drop remainder presence update identity/count: %+v", update)
			}
			if update.Sockets != tc.wantSockets {
				t.Fatalf("expected partial-drop ITEM_UPDATE sockets %+v, got %+v", tc.wantSockets, update.Sockets)
			}
			if update.Attributes != tc.wantAttributes {
				t.Fatalf("expected partial-drop ITEM_UPDATE attributes %+v, got %+v", tc.wantAttributes, update.Attributes)
			}
			ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode partial-drop remainder presence ground add: %v", err)
			}

			ownerEntity, ok := runtime.sharedWorld.playerEntityByName(owner.Name)
			if !ok {
				t.Fatal("expected live owner entity after partial drop")
			}
			pickup, ok := runtime.sharedWorld.GroundItemPickupFor(ownerEntity.Entity.ID, owner, ground.VID)
			if !ok {
				t.Fatal("expected pending ground handle after partial drop")
			}
			if pickup.Item.ID == 0 || pickup.Item.ID == 931 || pickup.Item.Count != 2 || pickup.Item.Vnum != 27001 {
				t.Fatalf("expected fresh ground identity distinct from remainder 931, got %+v", pickup.Item)
			}

			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load partial-drop remainder presence account: %v", err)
			}
			persisted := findPersistedCharacter(t, account, owner.Name)
			if len(persisted.Inventory) != 1 || persisted.Inventory[0].ID != 931 || persisted.Inventory[0].Count != 3 || persisted.Inventory[0].Slot != 5 {
				t.Fatalf("unexpected persisted partial-drop remainder presence character: %+v", persisted)
			}
			if (tc.sockets != nil) != persisted.Inventory[0].HasSockets() {
				t.Fatalf("persisted HasSockets=%v want %v", persisted.Inventory[0].HasSockets(), tc.sockets != nil)
			}
			if tc.sockets != nil {
				if persisted.Inventory[0].Sockets == nil || *persisted.Inventory[0].Sockets != *tc.sockets {
					t.Fatalf("expected persisted remainder sockets %+v, got %#v", *tc.sockets, persisted.Inventory[0].Sockets)
				}
			} else if persisted.Inventory[0].Sockets != nil {
				t.Fatalf("expected omitted persisted remainder sockets, got %#v", persisted.Inventory[0].Sockets)
			}
			if (tc.attributes != nil) != persisted.Inventory[0].HasAttributes() {
				t.Fatalf("persisted HasAttributes=%v want %v", persisted.Inventory[0].HasAttributes(), tc.attributes != nil)
			}
			if tc.attributes != nil {
				if persisted.Inventory[0].Attributes == nil || *persisted.Inventory[0].Attributes != *tc.attributes {
					t.Fatalf("expected persisted remainder attributes %+v, got %#v", *tc.attributes, persisted.Inventory[0].Attributes)
				}
			} else if persisted.Inventory[0].Attributes != nil {
				t.Fatalf("expected omitted persisted remainder attributes, got %#v", persisted.Inventory[0].Attributes)
			}

			if pickup.Item.HasSockets() != (tc.sockets != nil) {
				t.Fatalf("ground HasSockets=%v want %v", pickup.Item.HasSockets(), tc.sockets != nil)
			}
			if tc.sockets != nil {
				if pickup.Item.Sockets == nil || *pickup.Item.Sockets != *tc.sockets {
					t.Fatalf("expected cloned ground sockets %+v, got %#v", *tc.sockets, pickup.Item.Sockets)
				}
				if pickup.Item.Sockets == persisted.Inventory[0].Sockets {
					t.Fatal("expected ground sockets to stay independent of the persisted remainder")
				}
			} else if pickup.Item.Sockets != nil {
				t.Fatalf("expected omitted ground sockets, got %#v", pickup.Item.Sockets)
			}
			if pickup.Item.HasAttributes() != (tc.attributes != nil) {
				t.Fatalf("ground HasAttributes=%v want %v", pickup.Item.HasAttributes(), tc.attributes != nil)
			}
			if tc.attributes != nil {
				if pickup.Item.Attributes == nil || *pickup.Item.Attributes != *tc.attributes {
					t.Fatalf("expected cloned ground attributes %+v, got %#v", *tc.attributes, pickup.Item.Attributes)
				}
				if pickup.Item.Attributes == persisted.Inventory[0].Attributes {
					t.Fatal("expected ground attributes to stay independent of the persisted remainder")
				}
			} else if pickup.Item.Attributes != nil {
				t.Fatalf("expected omitted ground attributes, got %#v", pickup.Item.Attributes)
			}
		})
	}
}
