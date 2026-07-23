package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/player"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
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

func TestGroundPickupCountOnlyUpdateCarriesTemplateDisplayMetadata(t *testing.T) {
	template := itemcatalog.Template{
		Vnum:      27006,
		Name:      "Socketed Pickup Potion",
		Stackable: true,
		MaxCount:  200,
		Sockets:   itemcatalog.SocketValues{101, -202, 303},
		Attributes: itemcatalog.AttributeValues{
			{Type: 5, Value: 66},
			{Type: 10, Value: -8},
		},
	}
	result := player.GroundItemPickupResult{
		Item:   inventory.ItemInstance{ID: 31, Vnum: 27006, Count: 2, Slot: 5},
		Merged: true,
		Updated: inventory.ItemInstance{
			ID:    42,
			Vnum:  27006,
			Count: 200,
			Slot:  2,
		},
	}

	frames, ok := encodeBootstrapGroundPickupInventoryFrames(result, map[uint32]itemcatalog.Template{27006: template})
	if !ok {
		t.Fatalf("expected template-backed pickup update frame to be built")
	}
	if len(frames) != 1 {
		t.Fatalf("expected one pickup ITEM_UPDATE frame, got %d", len(frames))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode pickup merged-stack update: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(2) || update.Count != 200 {
		t.Fatalf("unexpected pickup merged-stack update identity/count: %+v", update)
	}
	wantSockets := [itemproto.ItemSocketCount]int32{101, -202, 303}
	if update.Sockets != wantSockets {
		t.Fatalf("expected template-authored pickup sockets %+v, got %+v", wantSockets, update.Sockets)
	}
	wantAttributes := [itemproto.ItemAttributeCount]itemproto.Attribute{{Type: 5, Value: 66}, {Type: 10, Value: -8}}
	if update.Attributes != wantAttributes {
		t.Fatalf("expected template-authored pickup attributes %+v, got %+v", wantAttributes, update.Attributes)
	}
}

func TestMerchantSellCountOnlyUpdateCarriesTemplateDisplayMetadata(t *testing.T) {
	template := itemcatalog.Template{
		Vnum:         27001,
		Name:         "Socketed Sell Potion",
		Stackable:    true,
		MaxCount:     200,
		ShopBuyPrice: 5,
		Sockets:      itemcatalog.SocketValues{-11, 202, -303},
		Attributes: itemcatalog.AttributeValues{
			{Type: 12, Value: 34},
			{Type: 15, Value: -9},
		},
	}
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("MerchantSellerTemplateUpdate", 0x01040128, 0x02050128, 125, []inventory.ItemInstance{{ID: 77, Vnum: 27001, Count: 3, Slot: 5}})
	issuePeerTicket(t, ticketStore, "merchant-sell-template-update", 0x28282828, buyer)
	if err := accounts.Save(accountstore.Account{Login: "merchant-sell-template-update", Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed merchant sell template-update account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{defaultMerchantCatalogDefinition()})
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1}, template})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected merchant sell template-update runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "merchant-sell-template-update", 0x28282828)
	defer closeSessionFlow(t, flow)
	interactWithMerchantForBuy(t, flow, actor.EntityID)

	frames, err := flow.HandleClientFrame(decodeSingleFrame(t, shopproto.EncodeClientSell2(shopproto.ClientSell2Packet{Slot: 5, Count: 2})))
	if err != nil {
		t.Fatalf("unexpected merchant sell template-update packet error: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("expected merchant sell update and gold point-change frames, got %d", len(frames))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, frames[0]))
	if err != nil {
		t.Fatalf("decode merchant sell partial-stack update: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(5) || update.Count != 1 {
		t.Fatalf("unexpected merchant sell update identity/count: %+v", update)
	}
	wantSockets := [itemproto.ItemSocketCount]int32{-11, 202, -303}
	if update.Sockets != wantSockets {
		t.Fatalf("expected template-authored sell sockets %+v, got %+v", wantSockets, update.Sockets)
	}
	wantAttributes := [itemproto.ItemAttributeCount]itemproto.Attribute{{Type: 12, Value: 34}, {Type: 15, Value: -9}}
	if update.Attributes != wantAttributes {
		t.Fatalf("expected template-authored sell attributes %+v, got %+v", wantAttributes, update.Attributes)
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, frames[1]))
	if err != nil {
		t.Fatalf("decode merchant sell gold point-change: %v", err)
	}
	if pointChange.VID != buyer.VID || pointChange.Value != 127 {
		t.Fatalf("unexpected merchant sell point-change companion: %+v", pointChange)
	}
	persisted, err := accounts.Load("merchant-sell-template-update")
	if err != nil {
		t.Fatalf("load persisted merchant sell template-update account: %v", err)
	}
	if !reflect.DeepEqual(persisted.Characters[0].Inventory, []inventory.ItemInstance{{ID: 77, Vnum: 27001, Count: 1, Slot: 5}}) {
		t.Fatalf("unexpected persisted merchant sell template-update inventory: %+v", persisted.Characters[0].Inventory)
	}
}
