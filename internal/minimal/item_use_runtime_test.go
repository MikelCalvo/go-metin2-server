package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestGameSessionFlowItemUseLastStackDeletesOnlyItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseLastStackQS", 0x0103051c, 0x0204051c, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 105, Vnum: 27001, Count: 1, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	owner.Points[bootstrapPlayerPointValueIndex] = 25
	issuePeerTicket(t, ticketStore, "item-use-last-stack-qs", 0x5050505c, owner)
	if err := accounts.Save(accountstore.Account{Login: "item-use-last-stack-qs", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed item-use last-stack owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "template consume"},
	}})
	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected item-use last-stack runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "item-use-last-stack-qs", 0x5050505c)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUse(itemproto.ClientUsePacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected item-use last-stack packet error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected item-use last stack to emit point change, item delete, quickslot delete, and info chat, got %d", len(out))
	}
	pointChange, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode item-use point change: %v", err)
	}
	if pointChange.VID != owner.VID || pointChange.Type != bootstrapPlayerPointType || pointChange.Amount != 50 || pointChange.Value != 75 {
		t.Fatalf("unexpected item-use point change: %+v", pointChange)
	}
	itemDel, err := itemproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode item-use item del: %v", err)
	}
	if itemDel.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected item-use item del: %+v", itemDel)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode item-use quickslot del: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("expected only item quickslot position 2 to be deleted, got %+v", quickslotDel)
	}
	infoChat, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode item-use info chat: %v", err)
	}
	if infoChat.Type != chatproto.ChatTypeInfo || infoChat.VID != 0 || infoChat.Message != "template consume" {
		t.Fatalf("unexpected item-use info chat: %+v", infoChat)
	}

	persisted, err := accounts.Load("item-use-last-stack-qs")
	if err != nil {
		t.Fatalf("load persisted item-use last-stack account: %v", err)
	}
	if len(persisted.Characters[0].Inventory) != 0 {
		t.Fatalf("expected consumed last stack to clear persisted inventory, got %+v", persisted.Characters[0].Inventory)
	}
	if persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != 75 {
		t.Fatalf("expected persisted template-authored point value 75, got %d", persisted.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
	if !reflect.DeepEqual(persisted.Characters[0].Quickslots, []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}}) {
		t.Fatalf("expected consumed last stack to delete only item quickslot, got %+v", persisted.Characters[0].Quickslots)
	}
}
