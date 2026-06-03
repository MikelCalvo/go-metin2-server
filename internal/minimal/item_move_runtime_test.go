package minimal

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
)

func TestGameRuntimeItemMoveRejectsAntiStackTemplateCompatibleMergeWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	itemStore := itemcatalog.NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	if err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Bound Practice Potion",
		Stackable: true,
		MaxCount:  200,
		AntiStack: true,
	}}}); err != nil {
		t.Fatalf("seed anti-stack item template: %v", err)
	}
	owner := peerVisibilityCharacter("MoveAntiStack", 0x01030201, 0x02040201, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1101, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1102, Vnum: 27001, Count: 4, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "move-antistack-owner", 0x51515151, owner)
	if err := accounts.Save(accountstore.Account{Login: "move-antistack-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed anti-stack move owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected anti-stack item-move runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "move-antistack-owner", 0x51515151)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientMove(itemproto.ClientMovePacket{
		Source:      itemproto.InventoryPosition(5),
		Destination: itemproto.InventoryPosition(6),
		Count:       0,
	})))
	if err != nil {
		t.Fatalf("unexpected anti-stack item move error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected anti-stack compatible move to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("move-antistack-owner")
	if err != nil {
		t.Fatalf("load anti-stack move owner account: %v", err)
	}
	if len(account.Characters) != 1 {
		t.Fatalf("expected one persisted anti-stack move owner, got %+v", account)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("anti-stack compatible move mutated persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
}
