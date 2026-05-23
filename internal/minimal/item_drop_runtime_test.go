package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

func TestGameRuntimeItemDropRemovesWholeStackAndEmitsGroundAdd(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropOwner", 0x01030171, 0x02040171, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "drop-owner", 0x17171717, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed drop owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected item-drop runtime error: %v", err)
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-owner", 0x17171717)
	if len(enterOut) < 5 {
		t.Fatalf("expected drop owner bootstrap frames, got %d", len(enterOut))
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected item drop error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected item drop to emit ITEM_DEL, QUICKSLOT_DEL, and GROUND_ADD, got %d frames", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode item drop del: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected item drop del packet: %+v", del)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode item drop quickslot del: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("unexpected item drop quickslot del: %+v", quickslotDel)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode item drop ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 27001 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected item drop ground add: %+v", ground)
	}

	account, err := accounts.Load("drop-owner")
	if err != nil {
		t.Fatalf("load drop owner account: %v", err)
	}
	if len(account.Characters) != 1 {
		t.Fatalf("expected one persisted drop owner, got %+v", account)
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected whole-stack drop to clear persisted inventory, got %#v", account.Characters[0].Inventory)
	}
	if len(account.Characters[0].Quickslots) != 0 {
		t.Fatalf("expected whole-stack drop to clear persisted item quickslot, got %#v", account.Characters[0].Quickslots)
	}
}

func TestGameRuntimeItemDrop2DecrementsStackAndEmitsGroundAdd(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropCountOwner", 0x01030172, 0x02040172, 1200, 2200, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1002, Vnum: 27001, Count: 5, Slot: 5}}
	issuePeerTicket(t, ticketStore, "drop-count-owner", 0x27272727, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-count-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed counted drop owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected item-drop2 runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-count-owner", 0x27272727)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 2})))
	if err != nil {
		t.Fatalf("unexpected item drop2 error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected counted item drop to emit ITEM_UPDATE and GROUND_ADD, got %d frames", len(out))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode item drop2 update: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(5) || update.Count != 3 {
		t.Fatalf("unexpected item drop2 update: %+v", update)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode item drop2 ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 27001 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected item drop2 ground add: %+v", ground)
	}

	account, err := accounts.Load("drop-count-owner")
	if err != nil {
		t.Fatalf("load counted drop owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 1002, Vnum: 27001, Count: 3, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after counted drop: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
}

func TestGameRuntimeItemPickupRestoresSelfDroppedWholeStack(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupOwner", 0x01030173, 0x02040173, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1003, Vnum: 27002, Count: 4, Slot: 5}}
	issuePeerTicket(t, ticketStore, "pickup-owner", 0x37373737, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected item-pickup runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-owner", 0x37373737)
	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(5))

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 2 {
		t.Fatalf("expected pickup to emit GROUND_DEL and ITEM_SET, got %d frames", len(pickupOut))
	}
	groundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, pickupOut[0]))
	if err != nil {
		t.Fatalf("decode pickup ground del: %v", err)
	}
	if groundDel.VID != ground.VID {
		t.Fatalf("unexpected pickup ground del: got %+v want vid %d", groundDel, ground.VID)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, pickupOut[1]))
	if err != nil {
		t.Fatalf("decode pickup item set: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(5) || set.Vnum != 27002 || set.Count != 4 {
		t.Fatalf("unexpected pickup item set: %+v", set)
	}

	account, err := accounts.Load("pickup-owner")
	if err != nil {
		t.Fatalf("load pickup owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("unexpected persisted inventory after pickup: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}

	replayOut := pickupGroundItem(t, flow, ground.VID)
	if len(replayOut) != 0 {
		t.Fatalf("expected replayed pickup to fail closed, got %d frames", len(replayOut))
	}
}

func TestGameRuntimeItemPickupRejectsOtherSessionGroundHandle(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupOwner", 0x01030174, 0x02040174, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1004, Vnum: 27003, Count: 2, Slot: 6}}
	snooper := peerVisibilityCharacter("PickupSnooper", 0x01030175, 0x02040175, 1450, 2450, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "pickup-owner", 0x47474747, owner)
	issuePeerTicket(t, ticketStore, "pickup-snooper", 0x57575757, snooper)
	if err := accounts.Save(accountstore.Account{Login: "pickup-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "pickup-snooper", Empire: snooper.Empire, Characters: cloneCharacters([]loginticket.Character{snooper})}); err != nil {
		t.Fatalf("seed pickup snooper account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected item-pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-owner", 0x47474747)
	snooperFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-snooper", 0x57575757)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))

	if out := pickupGroundItem(t, snooperFlow, ground.VID); len(out) != 0 {
		t.Fatalf("expected cross-session pickup to fail closed, got %d frames", len(out))
	}
	ownerOut := pickupGroundItem(t, ownerFlow, ground.VID)
	if len(ownerOut) != 2 {
		t.Fatalf("expected owner pickup after cross-session denial to still succeed, got %d frames", len(ownerOut))
	}
	account, err := accounts.Load("pickup-owner")
	if err != nil {
		t.Fatalf("load pickup owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("unexpected persisted owner inventory after pickup: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
}

func dropAndDecodeGroundAdd(t *testing.T, flow interface {
	HandleClientFrame(frame.Frame) ([][]byte, error)
}, position itemproto.Position) itemproto.GroundAddPacket {
	t.Helper()
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: position})))
	if err != nil {
		t.Fatalf("unexpected item drop error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected item drop to emit ITEM_DEL and GROUND_ADD, got %d frames", len(out))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode item drop ground add: %v", err)
	}
	return ground
}

func pickupGroundItem(t *testing.T, flow interface {
	HandleClientFrame(frame.Frame) ([][]byte, error)
}, vid uint32) [][]byte {
	t.Helper()
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: vid})))
	if err != nil {
		t.Fatalf("unexpected item pickup error: %v", err)
	}
	return out
}
