package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestGameRuntimeItemUsePartialPreservesInstanceSocketsAndAttributes(t *testing.T) {
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
			owner := peerVisibilityCharacter("UsePartialPresence", 0x010309e0+uint32(i), 0x020409e0+uint32(i), 1100, 2100, 0, 101, 201)
			owner.Points[bootstrapPlayerPointValueIndex] = 700
			owner.Inventory = []inventory.ItemInstance{{
				ID:         941,
				Vnum:       27001,
				Count:      3,
				Slot:       5,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}}
			login := "use-partial-presence-" + string(rune('a'+i))
			loginKey := uint32(0xe0e0e0e0 + i)
			issuePeerTicket(t, ticketStore, login, loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed partial-use remainder presence owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:       27001,
				Name:       "Use Presence Potion",
				Stackable:  true,
				MaxCount:   200,
				Sockets:    templateSockets,
				Attributes: templateAttributes,
				UseEffect: &itemcatalog.UseEffect{
					PointType:  bootstrapPlayerPointType,
					PointIndex: bootstrapPlayerPointValueIndex,
					PointDelta: 50,
					Message:    "consume:27001:+50",
				},
			}})

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected partial-use remainder presence runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
			if err != nil {
				t.Fatalf("unexpected partial-use remainder presence packet error: %v", err)
			}
			if len(out) != 4 {
				t.Fatalf("expected ITEM_USE echo, point-change, ITEM_UPDATE, and info chat, got %d frames", len(out))
			}
			useEcho, err := itemproto.DecodeUse(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode partial-use remainder presence echo: %v", err)
			}
			if useEcho.Position != itemproto.InventoryPosition(5) || useEcho.Vnum != 27001 {
				t.Fatalf("unexpected partial-use remainder presence echo: %+v", useEcho)
			}
			pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[1]))
			if err != nil {
				t.Fatalf("decode partial-use remainder presence point-change: %v", err)
			}
			if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != 50 || pointChange.Value != 750 {
				t.Fatalf("unexpected partial-use remainder presence point-change: %+v", pointChange)
			}
			update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[2]))
			if err != nil {
				t.Fatalf("decode partial-use remainder presence update: %v", err)
			}
			if update.Position != itemproto.InventoryPosition(5) || update.Count != 2 {
				t.Fatalf("unexpected partial-use remainder presence update identity/count: %+v", update)
			}
			if update.Sockets != tc.wantSockets {
				t.Fatalf("expected partial-use ITEM_UPDATE sockets %+v, got %+v", tc.wantSockets, update.Sockets)
			}
			if update.Attributes != tc.wantAttributes {
				t.Fatalf("expected partial-use ITEM_UPDATE attributes %+v, got %+v", tc.wantAttributes, update.Attributes)
			}

			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load partial-use remainder presence account: %v", err)
			}
			persisted := findPersistedCharacter(t, account, owner.Name)
			if persisted.Points[bootstrapPlayerPointValueIndex] != 750 || len(persisted.Inventory) != 1 || persisted.Inventory[0].ID != 941 || persisted.Inventory[0].Count != 2 || persisted.Inventory[0].Slot != 5 {
				t.Fatalf("unexpected persisted partial-use remainder presence character: %+v", persisted)
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
		})
	}
}
