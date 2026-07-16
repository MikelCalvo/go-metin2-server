package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/player"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
)

func TestMerchantBuyCountOnlyUpdateCarriesTemplateDisplayMetadata(t *testing.T) {
	template := itemcatalog.Template{
		Vnum:      27001,
		Name:      "Socketed Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		Sockets:   itemcatalog.SocketValues{11, -2, 33},
		Attributes: itemcatalog.AttributeValues{
			{Type: 1, Value: 25},
			{Type: 7, Value: -3},
		},
	}
	result := player.MerchantBuyResult{ItemChanges: []player.MerchantBuyItemChange{{
		Item:    inventory.ItemInstance{ID: 42, Vnum: 27001, Count: 199, Slot: 5},
		Created: false,
	}}}

	frames, err := merchantBuyResultFrames(result, true, map[uint32]itemcatalog.Template{27001: template})
	if err != nil {
		t.Fatalf("build merchant buy update frames: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected one merged-stack ITEM_UPDATE frame, got %d", len(frames))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode merged-stack merchant buy update: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(5) || update.Count != 199 {
		t.Fatalf("unexpected merged-stack update identity/count: %+v", update)
	}
	wantSockets := [itemproto.ItemSocketCount]int32{11, -2, 33}
	if update.Sockets != wantSockets {
		t.Fatalf("expected template-authored sockets %+v, got %+v", wantSockets, update.Sockets)
	}
	wantAttributes := [itemproto.ItemAttributeCount]itemproto.Attribute{{Type: 1, Value: 25}, {Type: 7, Value: -3}}
	if update.Attributes != wantAttributes {
		t.Fatalf("expected template-authored attributes %+v, got %+v", wantAttributes, update.Attributes)
	}
}
