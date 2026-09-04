package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestGameRuntimeShopSell2PartialPreservesInstanceSocketsAndAttributes(t *testing.T) {
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
			buyer := merchantBuyerCharacter("MerchantSellPresence", 0x01030990+uint32(i), 0x02040990+uint32(i), 125, []inventory.ItemInstance{{
				ID:         990,
				Vnum:       27001,
				Count:      3,
				Slot:       5,
				Sockets:    tc.sockets,
				Attributes: tc.attributes,
			}})
			requestedLogin := "merchant-sell-presence-" + string(rune('a'+i))
			runtime, accounts, flow, actorID, login := setupMerchantBuySession(t, requestedLogin, 0x90909090+uint32(i), buyer)
			defer closeSessionFlow(t, flow)
			runtime.itemTemplates[27001] = itemcatalog.Template{
				Vnum:          27001,
				Name:          "Sell Presence Potion",
				Stackable:     true,
				MaxCount:      200,
				ShopBuyPrice:  500,
				ShopSellPrice: 10,
				Sockets:       templateSockets,
				Attributes:    templateAttributes,
			}

			interactWithMerchantForBuy(t, flow, actorID)
			sellOut, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell2(shopproto.ClientSell2Packet{Slot: 5, Count: 2})))
			if err != nil {
				t.Fatalf("unexpected merchant sell presence packet error: %v", err)
			}
			if len(sellOut) != 2 {
				t.Fatalf("expected merchant sell update and gold point-change frames, got %d", len(sellOut))
			}
			update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, sellOut[0]))
			if err != nil {
				t.Fatalf("decode merchant sell presence update: %v", err)
			}
			if update.Position != itemproto.InventoryPosition(5) || update.Count != 1 {
				t.Fatalf("unexpected merchant sell presence update identity/count: %+v", update)
			}
			if update.Sockets != tc.wantSockets {
				t.Fatalf("expected merchant sell ITEM_UPDATE sockets %+v, got %+v", tc.wantSockets, update.Sockets)
			}
			if update.Attributes != tc.wantAttributes {
				t.Fatalf("expected merchant sell ITEM_UPDATE attributes %+v, got %+v", tc.wantAttributes, update.Attributes)
			}
			pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, sellOut[1]))
			if err != nil {
				t.Fatalf("decode merchant sell presence gold point-change: %v", err)
			}
			if pointChange.VID != buyer.VID || pointChange.Type != bootstrapGoldPointType || pointChange.Amount != 20 || pointChange.Value != 145 {
				t.Fatalf("unexpected merchant sell presence point-change: %+v", pointChange)
			}

			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load merchant sell presence account: %v", err)
			}
			persisted := findPersistedCharacter(t, account, buyer.Name)
			if persisted.Gold != 145 || len(persisted.Inventory) != 1 || persisted.Inventory[0].ID != 990 || persisted.Inventory[0].Count != 1 || persisted.Inventory[0].Slot != 5 {
				t.Fatalf("unexpected persisted merchant sell presence character: %+v", persisted)
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
