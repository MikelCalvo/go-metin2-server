package minimal

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
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
	if len(out) != 4 {
		t.Fatalf("expected item drop to emit ITEM_DEL, QUICKSLOT_DEL, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
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
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode item drop ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected item drop ownership: got %+v want vid %d owner %q", ownership, ground.VID, owner.Name)
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

func TestGameRuntimeItemPickupDoesNotQueueDuplicateCollectorGroundDel(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupNoDup", 0x01030174, 0x02040174, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1004, Vnum: 27001, Count: 3, Slot: 5}}
	issuePeerTicket(t, ticketStore, "pickup-no-dup", 0x74747474, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-no-dup", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup no-duplicate account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected pickup no-duplicate runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-no-dup", 0x74747474)
	defer closeSessionFlow(t, flow)

	dropOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected item drop before pickup: %v", err)
	}
	if len(dropOut) != 3 {
		t.Fatalf("expected drop without quickslot to emit ITEM_DEL, GROUND_ADD, and OWNERSHIP, got %d frames", len(dropOut))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, dropOut[1]))
	if err != nil {
		t.Fatalf("decode dropped ground item: %v", err)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after self drop registration, got %d", len(queued))
	}

	pickupOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientPickup(itemproto.ClientPickupPacket{VID: ground.VID})))
	if err != nil {
		t.Fatalf("unexpected item pickup error: %v", err)
	}
	if len(pickupOut) != 3 {
		t.Fatalf("expected pickup to emit GROUND_DEL, ITEM_SET, and ITEM_GET directly, got %d frames", len(pickupOut))
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
	if set.Position != itemproto.InventoryPosition(5) || set.Vnum != 27001 || set.Count != 3 {
		t.Fatalf("unexpected pickup item set: %+v", set)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, pickupOut[2]))
	if err != nil {
		t.Fatalf("decode pickup item get: %v", err)
	}
	if get.Vnum != 27001 || get.Count != 3 {
		t.Fatalf("unexpected pickup item get: %+v", get)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no duplicate queued collector ground-del after direct pickup response, got %d frames", len(queued))
	}
}

func TestGameRuntimeItemDropDeletesAllSourceItemQuickslots(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropMultiQuickslot", 0x0103017e, 0x0204017e, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1008, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 6, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "drop-multi-quickslot", 0x7e7e7e7e, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-multi-quickslot", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed multi-quickslot drop owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected multi-quickslot item-drop runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-multi-quickslot", 0x7e7e7e7e)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected multi-quickslot item drop error: %v", err)
	}
	if len(out) != 5 {
		t.Fatalf("expected item drop to emit ITEM_DEL, two QUICKSLOT_DEL frames, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	if _, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0])); err != nil {
		t.Fatalf("decode multi-quickslot item drop del: %v", err)
	}
	firstQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode first multi-quickslot item drop quickslot del: %v", err)
	}
	secondQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode second multi-quickslot item drop quickslot del: %v", err)
	}
	if firstQuickslotDel.Position != 2 || secondQuickslotDel.Position != 4 {
		t.Fatalf("expected deterministic source item quickslot deletes at positions 2 and 4, got %+v and %+v", firstQuickslotDel, secondQuickslotDel)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode multi-quickslot item drop ground add: %v", err)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[4]))
	if err != nil {
		t.Fatalf("decode multi-quickslot item drop ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected multi-quickslot item drop ownership: got %+v want vid %d owner %q", ownership, ground.VID, owner.Name)
	}

	account, err := accounts.Load("drop-multi-quickslot")
	if err != nil {
		t.Fatalf("load multi-quickslot drop owner account: %v", err)
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected multi-quickslot whole-stack drop to clear persisted inventory, got %#v", account.Characters[0].Inventory)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 6, Type: quickslotproto.TypeSkill, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("expected multi-quickslot whole-stack drop to clear only source item quickslots, got %#v", account.Characters[0].Quickslots)
	}
}

func TestGameRuntimeItemDrop2PartialStackKeepsSourceItemQuickslots(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropPartialQuickslot", 0x0103019f, 0x0204019f, 1250, 2250, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1031, Vnum: 27001, Count: 5, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "drop-partial-quickslot", 0x9f9f9f9f, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-partial-quickslot", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial-count drop owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected partial-count item-drop2 runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-partial-quickslot", 0x9f9f9f9f)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 2})))
	if err != nil {
		t.Fatalf("unexpected partial-count item drop2 error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected partial-count drop to emit ITEM_UPDATE, GROUND_ADD, and OWNERSHIP without quickslot deletes, got %d frames", len(out))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial-count item drop2 update: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(5) || update.Count != 3 {
		t.Fatalf("unexpected partial-count item drop2 update: %+v", update)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial-count item drop2 ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 27001 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected partial-count item drop2 ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode partial-count item drop2 ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected partial-count item drop2 ownership: got %+v want vid %d owner %q", ownership, ground.VID, owner.Name)
	}

	account, err := accounts.Load("drop-partial-quickslot")
	if err != nil {
		t.Fatalf("load partial-count drop owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 1031, Vnum: 27001, Count: 3, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("expected partial-count drop to persist reduced source stack, got %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected partial-count drop to preserve quickslots while source remains occupied, got %#v", account.Characters[0].Quickslots)
	}
}

func TestGameRuntimeItemDropRejectsAtBootstrapHPFloorWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropDeadOwner", 0x0103017d, 0x0204017d, 1100, 2100, 0, 101, 201)
	owner.Points[bootstrapPlayerPointValueIndex] = 0
	owner.Inventory = []inventory.ItemInstance{{ID: 1007, Vnum: 27001, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "drop-dead-owner", 0x7d7d7d7d, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-dead-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed hp-floor drop owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected hp-floor item-drop runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-dead-owner", 0x7d7d7d7d)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected hp-floor item drop error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected hp-floor item drop to emit no frames, got %d", len(out))
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued frames after hp-floor item drop rejection, got %d", len(queued))
	}

	account, err := accounts.Load("drop-dead-owner")
	if err != nil {
		t.Fatalf("load hp-floor drop owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("hp-floor item drop mutated inventory: got %+v want %+v", account.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("hp-floor item drop mutated quickslots: got %+v want %+v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemDrop2NormalizesZeroCountToWholeStackAndClearsItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropZeroOwner", 0x01030193, 0x02040193, 1250, 2250, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1021, Vnum: 27001, Count: 5, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "drop-zero-owner", 0x39393939, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-zero-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed zero-count drop owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected zero-count item-drop2 runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-zero-owner", 0x39393939)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 0})))
	if err != nil {
		t.Fatalf("unexpected zero-count item drop2 error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected zero-count drop to emit ITEM_DEL, QUICKSLOT_DEL, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode zero-count item drop2 del: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected zero-count item drop2 del: %+v", del)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode zero-count item drop2 quickslot del: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("unexpected zero-count item drop2 quickslot del: %+v", quickslotDel)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode zero-count item drop2 ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 27001 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected zero-count item drop2 ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode zero-count item drop2 ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected zero-count item drop2 ownership: got %+v want vid %d owner %q", ownership, ground.VID, owner.Name)
	}

	account, err := accounts.Load("drop-zero-owner")
	if err != nil {
		t.Fatalf("load zero-count drop owner account: %v", err)
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected zero-count drop to remove the whole stack, got %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}}) {
		t.Fatalf("expected zero-count drop to clear only the item quickslot, got %#v", account.Characters[0].Quickslots)
	}
}

func TestGameRuntimeItemDrop2PartialCountPreservesItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropPartialOwner", 0x01030195, 0x02040195, 1250, 2250, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1023, Vnum: 27001, Count: 5, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "drop-partial-owner", 0x59595959, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-partial-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial-count drop owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected partial-count item-drop2 runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-partial-owner", 0x59595959)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 2})))
	if err != nil {
		t.Fatalf("unexpected partial-count item drop2 error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected partial-count drop to emit ITEM_UPDATE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial-count item drop2 update: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(5) || update.Count != 3 {
		t.Fatalf("unexpected partial-count item drop2 update: %+v", update)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial-count item drop2 ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 27001 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected partial-count item drop2 ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode partial-count item drop2 ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected partial-count item drop2 ownership: got %+v want vid %d owner %q", ownership, ground.VID, owner.Name)
	}

	account, err := accounts.Load("drop-partial-owner")
	if err != nil {
		t.Fatalf("load partial-count drop owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{{ID: 1023, Vnum: 27001, Count: 3, Slot: 5}}) {
		t.Fatalf("expected partial-count drop to persist decremented stack, got %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected partial-count drop to preserve quickslots, got %#v", account.Characters[0].Quickslots)
	}
}

func TestGameRuntimeItemDrop2NormalizesOversizedCountToWholeStackAndClearsItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropOversizedOwner", 0x01030192, 0x02040192, 1250, 2250, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1020, Vnum: 27001, Count: 5, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "drop-oversized-owner", 0x29292929, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-oversized-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed oversized drop owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected oversized item-drop2 runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-oversized-owner", 0x29292929)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 9})))
	if err != nil {
		t.Fatalf("unexpected oversized item drop2 error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected oversized counted drop to emit ITEM_DEL, QUICKSLOT_DEL, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode oversized item drop2 del: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected oversized item drop2 del: %+v", del)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode oversized item drop2 quickslot del: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("unexpected oversized item drop2 quickslot del: %+v", quickslotDel)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode oversized item drop2 ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 27001 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected oversized item drop2 ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode oversized item drop2 ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected oversized item drop2 ownership: got %+v want vid %d owner %q", ownership, ground.VID, owner.Name)
	}

	account, err := accounts.Load("drop-oversized-owner")
	if err != nil {
		t.Fatalf("load oversized drop owner account: %v", err)
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected oversized drop to remove the whole stack, got %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5}}) {
		t.Fatalf("expected oversized drop to clear only the item quickslot, got %#v", account.Characters[0].Quickslots)
	}
}

func TestGameRuntimeItemDropRejectsTransferGuardTemplatesWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		login    string
		template itemcatalog.Template
	}{
		{
			name:     "anti-drop",
			login:    "drop-anti-drop",
			template: itemcatalog.Template{Vnum: 27003, Name: "Bound Drop Potion", Stackable: true, MaxCount: 200, AntiDrop: true},
		},
		{
			name:     "anti-give",
			login:    "drop-anti-give",
			template: itemcatalog.Template{Vnum: 27003, Name: "Bound Give Potion", Stackable: true, MaxCount: 200, AntiGive: true},
		},
		{
			name:     "anti-sell",
			login:    "drop-anti-sell",
			template: itemcatalog.Template{Vnum: 27003, Name: "Bound Sell Potion", Stackable: true, MaxCount: 200, AntiSell: true},
		},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("DropTransferGuard", 0x01030194+uint32(index), 0x02040194+uint32(index), 1250, 2250, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: uint64(1022 + index), Vnum: 27003, Count: 4, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			issuePeerTicket(t, ticketStore, tc.login, 0x49494949+uint32(index), owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s drop owner account: %v", tc.name, err)
			}

			runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(
				config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
				ticketStore,
				accounts,
				nil,
				newItemTemplateStore(t, []itemcatalog.Template{tc.template}),
			)
			if err != nil {
				t.Fatalf("unexpected %s item-drop runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, 0x49494949+uint32(index))

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 1})))
			if err != nil {
				t.Fatalf("unexpected %s item drop2 error: %v", tc.name, err)
			}
			if len(out) != 1 {
				t.Fatalf("expected %s item drop to emit one rejection info frame, got %d", tc.name, len(out))
			}
			info, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
			if err != nil {
				t.Fatalf("decode %s drop info chat: %v", tc.name, err)
			}
			if info.Type != chatproto.ChatTypeInfo || info.VID != 0 || info.Message != itemDropRejectedInfoMessage {
				t.Fatalf("unexpected %s drop rejection chat: %+v", tc.name, info)
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s drop rejection, got %d", tc.name, len(queued))
			}
			account, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load %s drop account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s drop mutated inventory: got %#v want %#v", tc.name, account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s drop mutated quickslots: got %#v want %#v", tc.name, account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemDropRejectsSelectedCharacterRestrictionsWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		login    string
		job      uint8
		raceNum  uint16
		level    uint8
		template itemcatalog.Template
	}{
		{
			name:     "anti-warrior",
			login:    "drop-anti-warrior",
			job:      0,
			raceNum:  0,
			level:    1,
			template: itemcatalog.Template{Vnum: 27003, Name: "Warrior Restricted Drop Potion", Stackable: true, MaxCount: 200, AntiWarrior: true},
		},
		{
			name:     "anti-female",
			login:    "drop-anti-female",
			job:      1,
			raceNum:  1,
			level:    1,
			template: itemcatalog.Template{Vnum: 27003, Name: "Female Restricted Drop Potion", Stackable: true, MaxCount: 200, AntiFemale: true},
		},
		{
			name:     "min-level",
			login:    "drop-min-level",
			job:      0,
			raceNum:  0,
			level:    5,
			template: itemcatalog.Template{Vnum: 27003, Name: "Veteran Drop Potion", Stackable: true, MaxCount: 200, MinLevel: 10},
		},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("DropRestricted", 0x0103019a+uint32(index), 0x0204019a+uint32(index), 1250, 2250, tc.raceNum, 101, 201)
			owner.Job = tc.job
			owner.RaceNum = tc.raceNum
			owner.Level = tc.level
			owner.Inventory = []inventory.ItemInstance{{ID: uint64(1032 + index), Vnum: 27003, Count: 4, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			issuePeerTicket(t, ticketStore, tc.login, 0x4a4a4a4a+uint32(index), owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s drop owner account: %v", tc.name, err)
			}

			runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(
				config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"},
				ticketStore,
				accounts,
				nil,
				newItemTemplateStore(t, []itemcatalog.Template{tc.template}),
			)
			if err != nil {
				t.Fatalf("unexpected %s item-drop runtime error: %v", tc.name, err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, 0x4a4a4a4a+uint32(index))
			defer closeSessionFlow(t, flow)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 1})))
			if err != nil {
				t.Fatalf("unexpected %s item drop2 error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s restricted item drop to emit no frames, got %d", tc.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s restricted drop, got %d", tc.name, len(queued))
			}
			account, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load %s drop account: %v", tc.name, err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("%s restricted drop mutated inventory: got %#v want %#v", tc.name, account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("%s restricted drop mutated quickslots: got %#v want %#v", tc.name, account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemDropRejectsLockedCarriedItemWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		packet []byte
	}{
		{name: "drop", packet: itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})},
		{name: "drop2", packet: itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 1})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("DropLocked", 0x0103019b, 0x0204019b, 1250, 2250, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 1032, Vnum: 27001, Count: 5, Slot: 5, Locked: true}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			login := "drop-locked-" + tc.name
			issuePeerTicket(t, ticketStore, login, 0x6b6b6b6b, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed locked drop account: %v", err)
			}

			runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
			if err != nil {
				t.Fatalf("unexpected locked item-drop runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x6b6b6b6b)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, tc.packet))
			if err != nil {
				t.Fatalf("unexpected locked item drop error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected locked item drop to emit no frames, got %d", len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after locked drop rejection, got %d", len(queued))
			}
			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load locked drop account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("locked drop mutated inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("locked drop mutated quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemDropRejectsDuplicateSlotOccupancyWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		packet []byte
	}{
		{name: "drop", packet: itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})},
		{name: "drop2", packet: itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 1})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("DropDuplicateSlot", 0x0103019a, 0x0204019a, 1250, 2250, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{
				{ID: 1030, Vnum: 27001, Count: 5, Slot: 5},
				{ID: 1031, Vnum: 27002, Count: 1, Slot: 5},
			}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			login := "drop-duplicate-slot-" + tc.name
			issuePeerTicket(t, ticketStore, login, 0x6a6a6a6a, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed duplicate-slot drop account: %v", err)
			}

			runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
			if err != nil {
				t.Fatalf("unexpected duplicate-slot item-drop runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x6a6a6a6a)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, tc.packet))
			if err != nil {
				t.Fatalf("unexpected duplicate-slot item drop error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected duplicate-slot item drop to emit no frames, got %d", len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after duplicate-slot drop rejection, got %d", len(queued))
			}
			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load duplicate-slot drop account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("duplicate-slot drop mutated inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("duplicate-slot drop mutated quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemDropRejectsMissingTemplateWhenCatalogAuthoredWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		packet []byte
	}{
		{name: "drop", packet: itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})},
		{name: "drop2", packet: itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 1})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("DropMissingTemplate", 0x01030195, 0x02040195, 1250, 2250, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 1023, Vnum: 27001, Count: 5, Slot: 5}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
			login := "drop-missing-template-" + tc.name
			issuePeerTicket(t, ticketStore, login, 0x59595959, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed missing-template drop account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27002, Name: "Unrelated Template", Stackable: true, MaxCount: 200}})

			runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
			if err != nil {
				t.Fatalf("unexpected missing-template item-drop runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0x59595959)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, tc.packet))
			if err != nil {
				t.Fatalf("unexpected missing-template item drop error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected missing-template item drop to emit no frames, got %d", len(out))
			}
			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load missing-template drop account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("missing-template drop mutated inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("missing-template drop mutated quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemDropRejectsOverTemplateMaxCarriedStackWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropOverMax", 0x01030194, 0x02040194, 1250, 2250, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1022, Vnum: 27001, Count: 201, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "drop-over-max", 0x49494949, owner)
	if err := accounts.Save(accountstore.Account{Login: "drop-over-max", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed over-template-max drop account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}})

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, itemStore, nil)
	if err != nil {
		t.Fatalf("unexpected over-template-max item-drop runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-over-max", 0x49494949)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected over-template-max item drop error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected over-template-max item drop to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("drop-over-max")
	if err != nil {
		t.Fatalf("load over-template-max drop account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("over-template-max drop mutated inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("over-template-max drop mutated quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemDrop2DecrementsStackAndPreservesItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropCountOwner", 0x01030172, 0x02040172, 1200, 2200, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1002, Vnum: 27001, Count: 5, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
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
	if len(out) != 3 {
		t.Fatalf("expected counted item drop to emit ITEM_UPDATE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
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
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode item drop2 ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected item drop2 ownership: got %+v want vid %d owner %q", ownership, ground.VID, owner.Name)
	}

	account, err := accounts.Load("drop-count-owner")
	if err != nil {
		t.Fatalf("load counted drop owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 1002, Vnum: 27001, Count: 3, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after counted drop: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after counted drop: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemUseToItemMergesStacksAndPersistsQuickslotCleanup(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemOwner", 0x01030601, 0x02040601, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1601, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 1602, Vnum: 27001, Count: 3, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "use-to-item-owner", 0x61616161, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed use-to-item owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-owner", 0x61616161)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected use-to-item error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected use-to-item to emit ITEM_DEL, ITEM_SET, and two QUICKSLOT_DEL frames, got %d frames", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode use-to-item source del: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected use-to-item source del: %+v", del)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode use-to-item target set: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(6) || set.Vnum != 27001 || set.Count != 5 {
		t.Fatalf("unexpected use-to-item target set: %+v", set)
	}
	firstQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode use-to-item first quickslot del: %v", err)
	}
	if firstQuickslotDel.Position != 1 {
		t.Fatalf("unexpected use-to-item first quickslot del: %+v", firstQuickslotDel)
	}
	secondQuickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[3]))
	if err != nil {
		t.Fatalf("decode use-to-item second quickslot del: %v", err)
	}
	if secondQuickslotDel.Position != 2 {
		t.Fatalf("unexpected use-to-item second quickslot del: %+v", secondQuickslotDel)
	}

	account, err := accounts.Load("use-to-item-owner")
	if err != nil {
		t.Fatalf("load use-to-item owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 1602, Vnum: 27001, Count: 5, Slot: 6}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after use-to-item: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{{Position: 3, Type: quickslotproto.TypeItem, Slot: 6}, {Position: 4, Type: quickslotproto.TypeSkill, Slot: 5}}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("unexpected persisted quickslots after use-to-item: got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemUseToItemPartialMergePreservesTargetQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemTargetSlot", 0x01030612, 0x02040612, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1613, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 1614, Vnum: 27001, Count: 198, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "use-to-item-target-quickslot", 0x62626272, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-target-quickslot", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed target-quickslot use-to-item owner account: %v", err)
	}

	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected target-quickslot use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-target-quickslot", 0x62626272)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected target-quickslot partial use-to-item error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected partial use-to-item to emit source and target updates only, got %d frames", len(out))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode target-quickslot partial source update: %v", err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 3 {
		t.Fatalf("unexpected target-quickslot partial source update: %+v", sourceUpdate)
	}
	targetUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode target-quickslot partial target update: %v", err)
	}
	if targetUpdate.Position != itemproto.InventoryPosition(6) || targetUpdate.Count != 200 {
		t.Fatalf("unexpected target-quickslot partial target update: %+v", targetUpdate)
	}
	account, err := accounts.Load("use-to-item-target-quickslot")
	if err != nil {
		t.Fatalf("load target-quickslot partial use-to-item owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 1613, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1614, Vnum: 27001, Count: 200, Slot: 6},
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after target-quickslot partial use-to-item: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	wantQuickslots := []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 6},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 6},
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, wantQuickslots) {
		t.Fatalf("expected partial merge to preserve target quickslots, got %#v want %#v", account.Characters[0].Quickslots, wantQuickslots)
	}
}

func TestGameRuntimeItemUseToItemPartiallyMergesAndPreservesSourceQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemPartial", 0x01030602, 0x02040602, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1611, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 1612, Vnum: 27001, Count: 198, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeSkill, Slot: 5},
	}
	issuePeerTicket(t, ticketStore, "use-to-item-partial", 0x62626262, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-partial", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial use-to-item owner account: %v", err)
	}

	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected partial use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-partial", 0x62626262)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected partial use-to-item error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected partial use-to-item to emit source and target ITEM_UPDATE frames only, got %d frames", len(out))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial use-to-item source update: %v", err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 3 {
		t.Fatalf("unexpected partial use-to-item source update: %+v", sourceUpdate)
	}
	targetUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial use-to-item target update: %v", err)
	}
	if targetUpdate.Position != itemproto.InventoryPosition(6) || targetUpdate.Count != 200 {
		t.Fatalf("unexpected partial use-to-item target update: %+v", targetUpdate)
	}

	account, err := accounts.Load("use-to-item-partial")
	if err != nil {
		t.Fatalf("load partial use-to-item owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 1611, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 1612, Vnum: 27001, Count: 200, Slot: 6},
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after partial use-to-item: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected partial merge to preserve source item quickslot, got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemUseToItemRejectsDuplicateInstanceIDsThroughSessionPath(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemDuplicate", 0x01030603, 0x02040603, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1631, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 1631, Vnum: 27001, Count: 3, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "use-to-item-duplicate", 0x63636363, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-duplicate", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed duplicate use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected duplicate use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-duplicate", 0x63636363)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected duplicate use-to-item error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected duplicate-id use-to-item to fail closed with no frames, got %d", len(out))
	}

	account, err := accounts.Load("use-to-item-duplicate")
	if err != nil {
		t.Fatalf("load duplicate use-to-item owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected duplicate-id use-to-item inventory to stay unchanged, got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected duplicate-id use-to-item quickslots to stay unchanged, got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected duplicate-id use-to-item to avoid normal use point effect, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemUseToItemRejectsMissingSourceTemplateThroughSessionPath(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemMissingTemplate", 0x01030604, 0x02040604, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1641, Vnum: 27009, Count: 2, Slot: 5},
		{ID: 1642, Vnum: 27009, Count: 3, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 1, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "use-to-item-missing-template", 0x64646464, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-missing-template", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed missing-template use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected missing-template use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-missing-template", 0x64646464)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{
		Source: itemproto.InventoryPosition(5),
		Target: itemproto.InventoryPosition(6),
	})))
	if err != nil {
		t.Fatalf("unexpected missing-template use-to-item error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected missing-template use-to-item to fail closed with no frames, got %d", len(out))
	}

	account, err := accounts.Load("use-to-item-missing-template")
	if err != nil {
		t.Fatalf("load missing-template use-to-item owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected missing-template use-to-item inventory to stay unchanged, got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected missing-template use-to-item quickslots to stay unchanged, got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeRelocationPreviewIncludesPendingGroundItemsInBeforeAndAfterOccupancy(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PreviewGroundOwner", 0x01030501, 0x02040501, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1501, Vnum: 27001, Count: 3, Slot: 5}}
	mover := peerVisibilityCharacter("PreviewGroundMover", 0x01030502, 0x02040502, 5000, 6000, 0, 102, 202)
	mover.MapIndex = 42
	issuePeerTicket(t, ticketStore, "preview-ground-owner", 0x51515151, owner)
	issuePeerTicket(t, ticketStore, "preview-ground-mover", 0x52525252, mover)
	if err := accounts.Save(accountstore.Account{Login: "preview-ground-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed preview ground owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "preview-ground-mover", Empire: mover.Empire, Characters: cloneCharacters([]loginticket.Character{mover})}); err != nil {
		t.Fatalf("seed preview ground mover account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected ground preview runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "preview-ground-owner", 0x51515151)
	if _, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Count: 2}))); err != nil {
		t.Fatalf("unexpected preview ground drop error: %v", err)
	}
	enterGameWithLoginTicket(t, runtime.SessionFactory(), "preview-ground-mover", 0x52525252)

	preview, ok := runtime.PreviewRelocation("PreviewGroundMover", 1, 1200, 2200)
	if !ok {
		t.Fatal("expected relocation preview to resolve mover")
	}
	if preview.Applied {
		t.Fatalf("expected dry-run relocation preview, got applied result: %+v", preview)
	}
	beforeSource, ok := findMapOccupancySnapshot(preview.BeforeMapOccupancy, 1)
	if !ok || beforeSource.GroundItemCount != 1 || len(beforeSource.GroundItems) != 1 || beforeSource.GroundItems[0].Vnum != 27001 || beforeSource.GroundItems[0].Count != 2 || beforeSource.GroundItems[0].OwnerName != owner.Name {
		t.Fatalf("expected source map before occupancy to include one pending ground item, got %+v", beforeSource)
	}
	afterSource, ok := findMapOccupancySnapshot(preview.AfterMapOccupancy, 1)
	if !ok || afterSource.GroundItemCount != 1 || len(afterSource.GroundItems) != 1 || afterSource.GroundItems[0].Vnum != 27001 || afterSource.GroundItems[0].Count != 2 || afterSource.GroundItems[0].OwnerName != owner.Name || afterSource.CharacterCount != 2 {
		t.Fatalf("expected source map after occupancy to preserve pending ground item and include moved player, got %+v", afterSource)
	}
	beforeDestination, ok := findMapOccupancySnapshot(preview.BeforeMapOccupancy, 42)
	if !ok || beforeDestination.GroundItemCount != 0 || beforeDestination.CharacterCount != 1 {
		t.Fatalf("expected destination map before occupancy to contain mover only, got %+v", beforeDestination)
	}
}

func findMapOccupancySnapshot(snapshots []MapOccupancySnapshot, mapIndex uint32) (MapOccupancySnapshot, bool) {
	for _, snapshot := range snapshots {
		if snapshot.MapIndex == mapIndex {
			return snapshot, true
		}
	}
	return MapOccupancySnapshot{}, false
}

func TestGameRuntimeItemUseToItemMergesCompatibleInventoryStacks(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemOwner", 0x01030192, 0x02040192, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1011, Vnum: 27001, Count: 3, Slot: 5}, {ID: 1012, Vnum: 27001, Count: 4, Slot: 6}}
	issuePeerTicket(t, ticketStore, "use-to-item-owner", 0x92929292, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-owner", 0x92929292)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected use-to-item error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected use-to-item stack merge to emit ITEM_DEL and ITEM_SET, got %d frames", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode use-to-item source del: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected use-to-item source del: %+v", del)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode use-to-item target set: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(6) || set.Vnum != 27001 || set.Count != 7 {
		t.Fatalf("unexpected use-to-item target set: %+v", set)
	}

	account, err := accounts.Load("use-to-item-owner")
	if err != nil {
		t.Fatalf("load use-to-item owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{{ID: 1012, Vnum: 27001, Count: 7, Slot: 6}}) {
		t.Fatalf("unexpected persisted inventory after use-to-item merge: %#v", account.Characters[0].Inventory)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected use-to-item merge to avoid normal use point effect, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemUseToItemDeletesSourceItemQuickslotOnFullMerge(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemQuick", 0x01030197, 0x02040197, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1051, Vnum: 27001, Count: 3, Slot: 5}, {ID: 1052, Vnum: 27001, Count: 4, Slot: 6}}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "use-to-item-quick", 0x97979797, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-quick", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed use-to-item quickslot owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected use-to-item quickslot runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-quick", 0x97979797)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected use-to-item quickslot error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected full use-to-item merge to emit ITEM_DEL, ITEM_SET, and QUICKSLOT_DEL, got %d frames", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode use-to-item quickslot source del: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected use-to-item quickslot source del: %+v", del)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode use-to-item quickslot target set: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(6) || set.Vnum != 27001 || set.Count != 7 {
		t.Fatalf("unexpected use-to-item quickslot target set: %+v", set)
	}
	quickslotDel, err := quickslotproto.DecodeDel(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode use-to-item source quickslot del: %v", err)
	}
	if quickslotDel.Position != 2 {
		t.Fatalf("expected only source item quickslot position 2 to be deleted, got %+v", quickslotDel)
	}

	account, err := accounts.Load("use-to-item-quick")
	if err != nil {
		t.Fatalf("load use-to-item quickslot owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{{ID: 1052, Vnum: 27001, Count: 7, Slot: 6}}) {
		t.Fatalf("unexpected persisted inventory after use-to-item quickslot merge: %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, []loginticket.Quickslot{
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 5},
		{Position: 4, Type: quickslotproto.TypeItem, Slot: 6},
	}) {
		t.Fatalf("unexpected persisted quickslots after use-to-item quickslot merge: %#v", account.Characters[0].Quickslots)
	}
}

func TestGameRuntimeItemUseToItemMergesPartialStackWithUpdateFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemPartial", 0x01030194, 0x02040194, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1031, Vnum: 27001, Count: 7, Slot: 5}, {ID: 1032, Vnum: 27001, Count: 8, Slot: 6}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "use-to-item-partial", 0x94949494, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-partial", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed partial use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  10,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected partial use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-partial", 0x94949494)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected partial use-to-item error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected partial use-to-item merge to emit two ITEM_UPDATE frames, got %d", len(out))
	}
	sourceUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode partial use-to-item source update: %v", err)
	}
	if sourceUpdate.Position != itemproto.InventoryPosition(5) || sourceUpdate.Count != 5 {
		t.Fatalf("unexpected partial use-to-item source update: %+v", sourceUpdate)
	}
	targetUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode partial use-to-item target update: %v", err)
	}
	if targetUpdate.Position != itemproto.InventoryPosition(6) || targetUpdate.Count != 10 {
		t.Fatalf("unexpected partial use-to-item target update: %+v", targetUpdate)
	}

	account, err := accounts.Load("use-to-item-partial")
	if err != nil {
		t.Fatalf("load partial use-to-item owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 1031, Vnum: 27001, Count: 5, Slot: 5}, {ID: 1032, Vnum: 27001, Count: 10, Slot: 6}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after partial use-to-item merge: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected partial use-to-item to keep source quickslot, got %#v", account.Characters[0].Quickslots)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected partial use-to-item merge to avoid normal use point effect, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemUseToItemRejectsEmptyOrSameSlotWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		login     string
		loginKey  uint32
		source    itemproto.Position
		target    itemproto.Position
		inventory []inventory.ItemInstance
	}{
		{
			name:      "empty source",
			login:     "use-to-item-empty-source",
			loginKey:  0x95959591,
			source:    itemproto.InventoryPosition(5),
			target:    itemproto.InventoryPosition(6),
			inventory: []inventory.ItemInstance{{ID: 1033, Vnum: 27001, Count: 4, Slot: 6}},
		},
		{
			name:      "empty target",
			login:     "use-to-item-empty-target",
			loginKey:  0x95959592,
			source:    itemproto.InventoryPosition(5),
			target:    itemproto.InventoryPosition(6),
			inventory: []inventory.ItemInstance{{ID: 1034, Vnum: 27001, Count: 3, Slot: 5}},
		},
		{
			name:      "same source and target",
			login:     "use-to-item-same-slot",
			loginKey:  0x95959593,
			source:    itemproto.InventoryPosition(5),
			target:    itemproto.InventoryPosition(5),
			inventory: []inventory.ItemInstance{{ID: 1035, Vnum: 27001, Count: 3, Slot: 5}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemEmpty", 0x01030195, 0x02040195, 1300, 2300, 0, 101, 201)
			owner.Inventory = append([]inventory.ItemInstance(nil), tc.inventory...)
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
			issuePeerTicket(t, ticketStore, tc.login, tc.loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed empty use-to-item owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:      27001,
				Name:      "Small Red Potion",
				Stackable: true,
				MaxCount:  200,
				UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
			}})

			runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
			if err != nil {
				t.Fatalf("unexpected empty use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.loginKey)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: tc.source, Target: tc.target})))
			if err != nil {
				t.Fatalf("unexpected empty use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s use-to-item to emit no frames, got %d", tc.name, len(out))
			}
			account, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load empty use-to-item owner account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("expected %s use-to-item inventory to stay unchanged, got %#v", tc.name, account.Characters[0].Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("expected %s use-to-item quickslots to stay unchanged, got %#v", tc.name, account.Characters[0].Quickslots)
			}
			if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
				t.Fatalf("expected %s use-to-item to avoid normal use point effect, got %d", tc.name, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
			}
		})
	}
}

func TestGameRuntimeItemUseToItemRejectsIncompatibleTargetWithoutNormalUseFallback(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemReject", 0x01030193, 0x02040193, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1021, Vnum: 27001, Count: 3, Slot: 5}, {ID: 1022, Vnum: 27002, Count: 4, Slot: 6}}
	issuePeerTicket(t, ticketStore, "use-to-item-reject", 0x93939393, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-reject", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed rejected use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}, {
		Vnum:      27002,
		Name:      "Practice Potion",
		Stackable: true,
		MaxCount:  200,
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected rejected use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-reject", 0x93939393)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected rejected use-to-item error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected incompatible use-to-item to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("use-to-item-reject")
	if err != nil {
		t.Fatalf("load rejected use-to-item owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected rejected use-to-item inventory to stay unchanged, got %#v", account.Characters[0].Inventory)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected rejected use-to-item to avoid normal use point effect, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemUseToItemRejectsOverMaxSourceStackWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemOverMax", 0x01030198, 0x02040198, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1061, Vnum: 27001, Count: 201, Slot: 5}, {ID: 1062, Vnum: 27001, Count: 4, Slot: 6}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "use-to-item-over-max", 0x98989898, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-over-max", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed over-max use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected over-max use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-over-max", 0x98989898)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected over-max use-to-item error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected over-max use-to-item to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("use-to-item-over-max")
	if err != nil {
		t.Fatalf("load over-max use-to-item owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected over-max use-to-item inventory to stay unchanged, got %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected over-max use-to-item quickslots to stay unchanged, got %#v", account.Characters[0].Quickslots)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected over-max use-to-item to avoid normal use point effect, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemUseToItemRejectsSameCellWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemSameCell", 0x0103019e, 0x0204019e, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1121, Vnum: 27001, Count: 2, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "use-to-item-same-cell", 0x9e9e9e9e, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-same-cell", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed same-cell use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected same-cell use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-same-cell", 0x9e9e9e9e)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected same-cell use-to-item error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected same-cell use-to-item to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("use-to-item-same-cell")
	if err != nil {
		t.Fatalf("load same-cell use-to-item owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected same-cell use-to-item inventory to stay unchanged, got %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected same-cell use-to-item quickslots to stay unchanged, got %#v", account.Characters[0].Quickslots)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected same-cell use-to-item to avoid normal use point effect, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemUseToItemRejectsNonStackableSourceTemplateWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemNonStack", 0x0103019d, 0x0204019d, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1151, Vnum: 27001, Count: 1, Slot: 5}, {ID: 1152, Vnum: 27001, Count: 1, Slot: 6}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "use-to-item-non-stack", 0x9d9d9d9d, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-non-stack", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed non-stackable use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Non-Stackable Practice Item",
		Stackable: false,
		MaxCount:  1,
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected non-stackable use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-non-stack", 0x9d9d9d9d)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected non-stackable use-to-item error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected non-stackable use-to-item to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("use-to-item-non-stack")
	if err != nil {
		t.Fatalf("load non-stackable use-to-item owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected non-stackable use-to-item inventory to stay unchanged, got %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected non-stackable use-to-item quickslots to stay unchanged, got %#v", account.Characters[0].Quickslots)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected non-stackable use-to-item to avoid normal use point effect, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemUseToItemRejectsFullTargetStackWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemFullTarget", 0x0103019f, 0x0204019f, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1131, Vnum: 27001, Count: 2, Slot: 5}, {ID: 1132, Vnum: 27001, Count: 200, Slot: 6}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "use-to-item-full-target", 0x9f9f9f9f, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-full-target", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed full target use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected full target use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-full-target", 0x9f9f9f9f)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected full target use-to-item error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected full target use-to-item to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("use-to-item-full-target")
	if err != nil {
		t.Fatalf("load full target use-to-item owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected full target use-to-item inventory to stay unchanged, got %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected full target use-to-item quickslots to stay unchanged, got %#v", account.Characters[0].Quickslots)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected full target use-to-item to avoid normal use point effect, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemUseToItemRejectsOverUint8TemplateMaxAtStoreBoundary(t *testing.T) {
	itemStore := itemcatalog.NewFileStore(t.TempDir() + "/item-templates.json")
	err := itemStore.Save(itemcatalog.Snapshot{Templates: []itemcatalog.Template{{
		Vnum:      27043,
		Name:      "Wide Stack Potion",
		Stackable: true,
		MaxCount:  300,
	}}})
	if !errors.Is(err, itemcatalog.ErrInvalidSnapshot) {
		t.Fatalf("expected oversized max_count use-to-item template to fail closed at store boundary, got %v", err)
	}
}

func TestGameRuntimeItemUseToItemRejectsOverMaxTargetStackWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemOverMaxTarget", 0x01030199, 0x02040199, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1071, Vnum: 27001, Count: 2, Slot: 5}, {ID: 1072, Vnum: 27001, Count: 201, Slot: 6}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
	issuePeerTicket(t, ticketStore, "use-to-item-over-max-target", 0x99999999, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-over-max-target", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed over-max target use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected over-max target use-to-item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-over-max-target", 0x99999999)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected over-max target use-to-item error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected over-max target use-to-item to emit no frames, got %d", len(out))
	}
	account, err := accounts.Load("use-to-item-over-max-target")
	if err != nil {
		t.Fatalf("load over-max target use-to-item owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected over-max target use-to-item inventory to stay unchanged, got %#v", account.Characters[0].Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected over-max target use-to-item quickslots to stay unchanged, got %#v", account.Characters[0].Quickslots)
	}
	if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("expected over-max target use-to-item to avoid normal use point effect, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
	}
}

func TestGameRuntimeItemUseToItemRejectsLockedSourceOrTargetWithoutMutation(t *testing.T) {
	cases := []struct {
		name       string
		login      string
		loginKey   uint32
		inventory  []inventory.ItemInstance
		quickslots []loginticket.Quickslot
	}{
		{
			name:     "locked source",
			login:    "use-to-item-locked-source",
			loginKey: 0x9b9b9b9b,
			inventory: []inventory.ItemInstance{
				{ID: 1081, Vnum: 27001, Count: 2, Slot: 5, Locked: true},
				{ID: 1082, Vnum: 27001, Count: 3, Slot: 6},
			},
			quickslots: []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}},
		},
		{
			name:     "locked target",
			login:    "use-to-item-locked-target",
			loginKey: 0x9c9c9c9c,
			inventory: []inventory.ItemInstance{
				{ID: 1091, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1092, Vnum: 27001, Count: 3, Slot: 6, Locked: true},
			},
			quickslots: []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemLocked", 0x0103019b, 0x0204019b, 1300, 2300, 0, 101, 201)
			owner.Inventory = append([]inventory.ItemInstance(nil), tc.inventory...)
			owner.Quickslots = append([]loginticket.Quickslot(nil), tc.quickslots...)
			issuePeerTicket(t, ticketStore, tc.login, tc.loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed locked use-to-item owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:      27001,
				Name:      "Small Red Potion",
				Stackable: true,
				MaxCount:  200,
				UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
			}})

			runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
			if err != nil {
				t.Fatalf("unexpected locked use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.loginKey)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
			if err != nil {
				t.Fatalf("unexpected locked use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected locked use-to-item to emit no frames, got %d", len(out))
			}
			account, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load locked use-to-item owner account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("expected locked use-to-item inventory to stay unchanged, got %#v", account.Characters[0].Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("expected locked use-to-item quickslots to stay unchanged, got %#v", account.Characters[0].Quickslots)
			}
			if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
				t.Fatalf("expected locked use-to-item to avoid normal use point effect, got %d", account.Characters[0].Points[bootstrapPlayerPointValueIndex])
			}
		})
	}
}

func TestGameRuntimeItemUseToItemRejectsAntiTransferTemplatesWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		login     string
		loginKey  uint32
		configure func(*itemcatalog.Template)
	}{
		{
			name:     "anti-stack template",
			login:    "use-to-item-anti-stack",
			loginKey: 0xa9a9a9a9,
			configure: func(template *itemcatalog.Template) {
				template.AntiStack = true
			},
		},
		{
			name:     "anti-drop template",
			login:    "use-to-item-anti-drop",
			loginKey: 0xaaaaaaaa,
			configure: func(template *itemcatalog.Template) {
				template.AntiDrop = true
			},
		},
		{
			name:     "anti-give template",
			login:    "use-to-item-anti-give",
			loginKey: 0xabababab,
			configure: func(template *itemcatalog.Template) {
				template.AntiGive = true
			},
		},
		{
			name:     "anti-sell template",
			login:    "use-to-item-anti-sell",
			loginKey: 0xacacacac,
			configure: func(template *itemcatalog.Template) {
				template.AntiSell = true
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemAnti", 0x010301aa, 0x020401aa, 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 1101, Vnum: 27001, Count: 2, Slot: 5}, {ID: 1102, Vnum: 27001, Count: 3, Slot: 6}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
			issuePeerTicket(t, ticketStore, tc.login, tc.loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed anti-template use-to-item owner account: %v", err)
			}
			template := itemcatalog.Template{
				Vnum:      27001,
				Name:      "Small Red Potion",
				Stackable: true,
				MaxCount:  200,
				UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
			}
			tc.configure(&template)
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{template})

			runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
			if err != nil {
				t.Fatalf("unexpected anti-template use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.loginKey)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
			if err != nil {
				t.Fatalf("unexpected anti-template use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s use-to-item to emit no frames, got %d", tc.name, len(out))
			}
			account, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load anti-template use-to-item owner account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("expected %s use-to-item inventory to stay unchanged, got %#v", tc.name, account.Characters[0].Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("expected %s use-to-item quickslots to stay unchanged, got %#v", tc.name, account.Characters[0].Quickslots)
			}
			if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
				t.Fatalf("expected %s use-to-item to avoid normal use point effect, got %d", tc.name, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
			}
		})
	}
}

func TestGameRuntimeItemUseToItemRejectsMissingOrInvalidTemplateWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		login    string
		loginKey uint32
		template itemcatalog.Template
		install  bool
	}{
		{
			name:     "missing source template",
			login:    "use-to-item-missing-template",
			loginKey: 0xa1a1a1a1,
		},
		{
			name:     "invalid source template",
			login:    "use-to-item-invalid-template",
			loginKey: 0xa2a2a2a2,
			install:  true,
			template: itemcatalog.Template{
				Vnum:      27044,
				Name:      "",
				Stackable: true,
				MaxCount:  200,
				UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27044:+50"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemTemplateGuard", 0x010301a1, 0x020401a1, 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 1141, Vnum: 27044, Count: 3, Slot: 5}, {ID: 1142, Vnum: 27044, Count: 4, Slot: 6}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
			issuePeerTicket(t, ticketStore, tc.login, tc.loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed template-guard use-to-item owner account: %v", err)
			}

			runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, newItemTemplateStore(t, nil))
			if err != nil {
				t.Fatalf("unexpected template-guard use-to-item runtime error: %v", err)
			}
			if tc.install {
				if runtime.itemTemplates == nil {
					runtime.itemTemplates = make(map[uint32]itemcatalog.Template)
				}
				runtime.itemTemplates[tc.template.Vnum] = tc.template
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.loginKey)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
			if err != nil {
				t.Fatalf("unexpected template-guard use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s use-to-item to fail closed with no frames, got %d", tc.name, len(out))
			}
			account, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load template-guard use-to-item owner account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("expected %s use-to-item inventory to stay unchanged, got %#v", tc.name, account.Characters[0].Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("expected %s use-to-item quickslots to stay unchanged, got %#v", tc.name, account.Characters[0].Quickslots)
			}
			if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
				t.Fatalf("expected %s use-to-item to avoid normal use point effect, got %d", tc.name, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
			}
		})
	}
}

func TestGameRuntimeItemUseToItemRejectsNonInventoryWindowsBeforeTemplateLookup(t *testing.T) {
	cases := []struct {
		name     string
		login    string
		loginKey uint32
		source   itemproto.Position
		target   itemproto.Position
	}{
		{
			name:     "equipment source",
			login:    "use-to-item-equipment-source",
			loginKey: 0xa5a5a5a5,
			source:   itemproto.Position{WindowType: itemproto.WindowEquipment, Cell: 5},
			target:   itemproto.InventoryPosition(6),
		},
		{
			name:     "equipment target",
			login:    "use-to-item-equipment-target",
			loginKey: 0xa6a6a6a6,
			source:   itemproto.InventoryPosition(5),
			target:   itemproto.Position{WindowType: itemproto.WindowEquipment, Cell: 6},
		},
		{
			name:     "safebox source",
			login:    "use-to-item-safebox-source",
			loginKey: 0xa7a7a7a7,
			source:   itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 5},
			target:   itemproto.InventoryPosition(6),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemWindowGuard", 0x010301a5, 0x020401a5, 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 1171, Vnum: 27047, Count: 2, Slot: 5}, {ID: 1172, Vnum: 27047, Count: 3, Slot: 6}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
			issuePeerTicket(t, ticketStore, tc.login, tc.loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed window-guard use-to-item owner account: %v", err)
			}
			runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, newItemTemplateStore(t, []itemcatalog.Template{{
				Vnum:      27047,
				Name:      "Window Guard Potion",
				Stackable: true,
				MaxCount:  200,
			}}))
			if err != nil {
				t.Fatalf("unexpected window-guard use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.loginKey)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: tc.source, Target: tc.target})))
			if err != nil {
				t.Fatalf("unexpected window-guard use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s use-to-item to fail closed with no frames, got %d", tc.name, len(out))
			}
			account, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load window-guard use-to-item owner account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("expected %s use-to-item inventory to stay unchanged, got %#v", tc.name, account.Characters[0].Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("expected %s use-to-item quickslots to stay unchanged, got %#v", tc.name, account.Characters[0].Quickslots)
			}
		})
	}
}

func TestGameRuntimeItemUseToItemRejectsNonStackableOrAntiTransferTemplatesWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		login     string
		loginKey  uint32
		template  itemcatalog.Template
		inventory []inventory.ItemInstance
	}{
		{
			name:     "non-stackable template",
			login:    "use-to-item-non-stackable",
			loginKey: 0x9d9d9d9d,
			template: itemcatalog.Template{
				Vnum:      11200,
				Name:      "Wooden Sword",
				Stackable: false,
				MaxCount:  1,
				EquipSlot: inventory.EquipmentSlotWeapon.String(),
			},
			inventory: []inventory.ItemInstance{{ID: 1111, Vnum: 11200, Count: 1, Slot: 5}, {ID: 1112, Vnum: 11200, Count: 1, Slot: 6}},
		},
		{
			name:     "stackable equippable template",
			login:    "use-to-item-equippable-stack",
			loginKey: 0xa8a8a8a8,
			template: itemcatalog.Template{
				Vnum:      11201,
				Name:      "Stackable Weapon Token",
				Stackable: true,
				MaxCount:  200,
				EquipSlot: inventory.EquipmentSlotWeapon.String(),
			},
			inventory: []inventory.ItemInstance{{ID: 1181, Vnum: 11201, Count: 2, Slot: 5}, {ID: 1182, Vnum: 11201, Count: 3, Slot: 6}},
		},
		{
			name:     "anti-stack template",
			login:    "use-to-item-anti-stack",
			loginKey: 0x9e9e9e9e,
			template: itemcatalog.Template{
				Vnum:      27003,
				Name:      "Anti-stack Potion",
				Stackable: true,
				MaxCount:  200,
				AntiStack: true,
				UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27003:+50"},
			},
			inventory: []inventory.ItemInstance{{ID: 1121, Vnum: 27003, Count: 2, Slot: 5}, {ID: 1122, Vnum: 27003, Count: 3, Slot: 6}},
		},
		{
			name:     "anti-drop template",
			login:    "use-to-item-anti-drop",
			loginKey: 0xa3a3a3a3,
			template: itemcatalog.Template{
				Vnum:      27045,
				Name:      "Anti-drop Potion",
				Stackable: true,
				MaxCount:  200,
				AntiDrop:  true,
			},
			inventory: []inventory.ItemInstance{{ID: 1151, Vnum: 27045, Count: 2, Slot: 5}, {ID: 1152, Vnum: 27045, Count: 3, Slot: 6}},
		},
		{
			name:     "anti-give template",
			login:    "use-to-item-anti-give",
			loginKey: 0xa4a4a4a4,
			template: itemcatalog.Template{
				Vnum:      27046,
				Name:      "Anti-give Potion",
				Stackable: true,
				MaxCount:  200,
				AntiGive:  true,
			},
			inventory: []inventory.ItemInstance{{ID: 1161, Vnum: 27046, Count: 2, Slot: 5}, {ID: 1162, Vnum: 27046, Count: 3, Slot: 6}},
		},
		{
			name:     "anti-sell template",
			login:    "use-to-item-anti-sell",
			loginKey: 0xa5a5a5a5,
			template: itemcatalog.Template{
				Vnum:      27047,
				Name:      "Anti-sell Potion",
				Stackable: true,
				MaxCount:  200,
				AntiSell:  true,
			},
			inventory: []inventory.ItemInstance{{ID: 1171, Vnum: 27047, Count: 2, Slot: 5}, {ID: 1172, Vnum: 27047, Count: 3, Slot: 6}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemGuard", 0x0103019d, 0x0204019d, 1300, 2300, 0, 101, 201)
			owner.Inventory = append([]inventory.ItemInstance(nil), tc.inventory...)
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
			issuePeerTicket(t, ticketStore, tc.login, tc.loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed guarded use-to-item owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{tc.template})

			runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
			if err != nil {
				t.Fatalf("unexpected guarded use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.loginKey)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
			if err != nil {
				t.Fatalf("unexpected guarded use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s use-to-item to fail closed with no frames, got %d", tc.name, len(out))
			}
			account, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load guarded use-to-item owner account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("expected %s use-to-item inventory to stay unchanged, got %#v", tc.name, account.Characters[0].Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("expected %s use-to-item quickslots to stay unchanged, got %#v", tc.name, account.Characters[0].Quickslots)
			}
			if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
				t.Fatalf("expected %s use-to-item to avoid normal use point effect, got %d", tc.name, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
			}
		})
	}
}

func TestGameRuntimeItemUseToItemRejectsSelectedCharacterAntiFlagTemplatesWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		login     string
		loginKey  uint32
		character func(loginticket.Character) loginticket.Character
		template  itemcatalog.Template
	}{
		{
			name:     "anti warrior job",
			login:    "use-to-item-anti-warrior",
			loginKey: 0xb2b2b2b2,
			character: func(owner loginticket.Character) loginticket.Character {
				owner.Job = 0
				return owner
			},
			template: itemcatalog.Template{Vnum: 27048, Name: "Warrior Restricted Potion", Stackable: true, MaxCount: 200, AntiWarrior: true},
		},
		{
			name:     "anti female race",
			login:    "use-to-item-anti-female",
			loginKey: 0xb3b3b3b3,
			character: func(owner loginticket.Character) loginticket.Character {
				owner.RaceNum = 1
				return owner
			},
			template: itemcatalog.Template{Vnum: 27049, Name: "Female Restricted Potion", Stackable: true, MaxCount: 200, AntiFemale: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("UseToItemAntiFlag", 0x010301b2, 0x020401b2, 1300, 2300, 0, 101, 201)
			owner = tc.character(owner)
			owner.Inventory = []inventory.ItemInstance{{ID: 1191, Vnum: tc.template.Vnum, Count: 2, Slot: 5}, {ID: 1192, Vnum: tc.template.Vnum, Count: 3, Slot: 6}}
			owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}, {Position: 3, Type: quickslotproto.TypeItem, Slot: 6}}
			issuePeerTicket(t, ticketStore, tc.login, tc.loginKey, owner)
			if err := accounts.Save(accountstore.Account{Login: tc.login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed anti-flag use-to-item owner account: %v", err)
			}
			itemStore := newItemTemplateStore(t, []itemcatalog.Template{tc.template})

			runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
			if err != nil {
				t.Fatalf("unexpected anti-flag use-to-item runtime error: %v", err)
			}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), tc.login, tc.loginKey)

			out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
			if err != nil {
				t.Fatalf("unexpected anti-flag use-to-item error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s use-to-item to fail closed with no frames, got %d", tc.name, len(out))
			}
			account, err := accounts.Load(tc.login)
			if err != nil {
				t.Fatalf("load anti-flag use-to-item owner account: %v", err)
			}
			if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
				t.Fatalf("expected %s use-to-item inventory to stay unchanged, got %#v", tc.name, account.Characters[0].Inventory)
			}
			if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
				t.Fatalf("expected %s use-to-item quickslots to stay unchanged, got %#v", tc.name, account.Characters[0].Quickslots)
			}
			if account.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
				t.Fatalf("expected %s use-to-item to avoid normal use point effect, got %d", tc.name, account.Characters[0].Points[bootstrapPlayerPointValueIndex])
			}
		})
	}
}

func TestGameRuntimeItemUseToItemStaleSessionEmitsLocalFramesButDoesNotPersist(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("UseToItemStale", 0x010301b1, 0x020401b1, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1181, Vnum: 27001, Count: 3, Slot: 5}, {ID: 1182, Vnum: 27001, Count: 4, Slot: 6}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "use-to-item-stale", 0xb1b1b1b1, owner)
	if err := accounts.Save(accountstore.Account{Login: "use-to-item-stale", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed stale use-to-item owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27001,
		Name:      "Small Red Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{PointType: bootstrapPlayerPointType, PointIndex: bootstrapPlayerPointValueIndex, PointDelta: 50, Message: "consume:27001:+50"},
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected stale use-to-item runtime error: %v", err)
	}
	staleFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-stale", 0xb1b1b1b1)
	closeSessionFlow(t, staleFlow)
	issuePeerTicket(t, ticketStore, "use-to-item-stale", 0xb1b1b1b2, owner)
	freshFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "use-to-item-stale", 0xb1b1b1b2)

	out, err := staleFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientUseToItem(itemproto.ClientUseToItemPacket{Source: itemproto.InventoryPosition(5), Target: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected stale use-to-item error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected stale use-to-item to emit local ITEM_DEL, ITEM_SET, and QUICKSLOT_DEL frames, got %d", len(out))
	}
	var sawSourceDel, sawTargetSet, sawQuickslotDel bool
	for _, raw := range out {
		fr := decodeSingleFrame(t, raw)
		if del, err := itemproto.DecodeDel(fr); err == nil && del.Position == itemproto.InventoryPosition(5) {
			sawSourceDel = true
			continue
		}
		if set, err := itemproto.DecodeSet(fr); err == nil && set.Position == itemproto.InventoryPosition(6) && set.Vnum == 27001 && set.Count == 7 {
			sawTargetSet = true
			continue
		}
		if del, err := quickslotproto.DecodeDel(fr); err == nil && del.Position == 2 {
			sawQuickslotDel = true
		}
	}
	if !sawSourceDel || !sawTargetSet || !sawQuickslotDel {
		t.Fatalf("expected stale use-to-item local frames to include source del, target set, and source quickslot del, got %d frames", len(out))
	}

	account, err := accounts.Load("use-to-item-stale")
	if err != nil {
		t.Fatalf("load stale use-to-item account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("stale use-to-item persisted inventory: got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("stale use-to-item persisted quickslots: got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
	players := runtime.ConnectedCharacters()
	if len(players) != 1 || players[0].Name != owner.Name {
		t.Fatalf("expected one fresh live owner after stale use-to-item, got %#v", players)
	}
	live, ok := runtime.InventorySnapshot(owner.Name)
	if !ok {
		t.Fatalf("expected fresh live owner inventory snapshot")
	}
	if !reflect.DeepEqual(live.Inventory, []InventoryItemSnapshot{{ID: 1181, Vnum: 27001, Count: 3, Slot: 5}, {ID: 1182, Vnum: 27001, Count: 4, Slot: 6}}) {
		t.Fatalf("stale use-to-item replaced fresh live inventory: %#v", live.Inventory)
	}
	if queued := flushServerFrames(t, freshFlow); len(queued) != 0 {
		t.Fatalf("stale use-to-item should not queue peer frames, got %d", len(queued))
	}
}

func TestGameRuntimeItemDropRejectsAntiDropAndAntiGiveTemplatesWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("BoundDropOwner", 0x01030191, 0x02040191, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1019, Vnum: 27019, Count: 3, Slot: 5}}
	owner.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 5}}
	issuePeerTicket(t, ticketStore, "bound-drop-owner", 0x19191919, owner)
	if err := accounts.Save(accountstore.Account{Login: "bound-drop-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed bound drop owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27019,
		Name:      "Bound Practice Potion",
		Stackable: true,
		MaxCount:  200,
		AntiDrop:  true,
		AntiGive:  true,
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected anti-drop item runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "bound-drop-owner", 0x19191919)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5)})))
	if err != nil {
		t.Fatalf("unexpected anti-drop item drop error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected anti-drop/give item drop to emit one info chat frame, got %d", len(out))
	}
	info, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode anti-drop/give info chat: %v", err)
	}
	if info.Type != chatproto.ChatTypeInfo || info.VID != 0 || info.Message != itemDropRejectedInfoMessage {
		t.Fatalf("unexpected anti-drop/give info chat: %+v", info)
	}
	account, err := accounts.Load("bound-drop-owner")
	if err != nil {
		t.Fatalf("load bound drop owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected anti-drop/give item inventory to stay unchanged, got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected anti-drop/give item quickslots to stay unchanged, got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemDropWithGoldDropsCurrencyInsteadOfInventoryItem(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GoldDropOwner", 0x0103019a, 0x0204019a, 1300, 2300, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 1100, Vnum: 27030, Count: 2, Slot: 5}}
	issuePeerTicket(t, ticketStore, "gold-drop-owner", 0x9a9a9a9a, owner)
	if err := accounts.Save(accountstore.Account{Login: "gold-drop-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed gold drop owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected gold-drop runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "gold-drop-owner", 0x9a9a9a9a)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5), Elk: 1200})))
	if err != nil {
		t.Fatalf("unexpected gold drop error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected gold drop to emit POINT_CHANGE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	point, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode gold drop point change: %v", err)
	}
	if point != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: -1200, Value: 3800}) {
		t.Fatalf("unexpected gold drop point change: %+v", point)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode gold drop ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 1 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected gold drop ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode gold drop ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected gold drop ownership: %+v", ownership)
	}

	account, err := accounts.Load("gold-drop-owner")
	if err != nil {
		t.Fatalf("load gold drop owner account: %v", err)
	}
	if account.Characters[0].Gold != 3800 {
		t.Fatalf("expected persisted gold 3800 after drop, got %d", account.Characters[0].Gold)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected gold drop to leave inventory unchanged, got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 3 {
		t.Fatalf("expected gold pickup to emit GROUND_DEL, POINT_CHANGE, and ITEM_GET, got %d frames", len(pickupOut))
	}
	groundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, pickupOut[0]))
	if err != nil {
		t.Fatalf("decode gold pickup ground del: %v", err)
	}
	if groundDel.VID != ground.VID {
		t.Fatalf("unexpected gold pickup ground del: %+v", groundDel)
	}
	pickupPoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, pickupOut[1]))
	if err != nil {
		t.Fatalf("decode gold pickup point change: %v", err)
	}
	if pickupPoint != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: 1200, Value: 5000}) {
		t.Fatalf("unexpected gold pickup point change: %+v", pickupPoint)
	}
	pickupGet, err := itemproto.DecodeGet(decodeSingleFrame(t, pickupOut[2]))
	if err != nil {
		t.Fatalf("decode gold pickup item get: %v", err)
	}
	if pickupGet != (itemproto.GetPacket{Vnum: 1, Count: 1, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("unexpected gold pickup item get: %+v", pickupGet)
	}
	account, err = accounts.Load("gold-drop-owner")
	if err != nil {
		t.Fatalf("reload gold drop owner account after pickup: %v", err)
	}
	if account.Characters[0].Gold != 5000 {
		t.Fatalf("expected persisted gold restored to 5000 after pickup, got %d", account.Characters[0].Gold)
	}
	if replay := pickupGroundItem(t, flow, ground.VID); len(replay) != 0 {
		t.Fatalf("expected replayed gold pickup to fail closed, got %d frames", len(replay))
	}
}

func TestGameRuntimeItemDrop2WithGoldDropsCurrencyInsteadOfCountedItem(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GoldDrop2Owner", 0x0103019b, 0x0204019b, 1350, 2350, 0, 101, 201)
	owner.Gold = 9000
	owner.Inventory = []inventory.ItemInstance{{ID: 1101, Vnum: 27031, Count: 5, Slot: 6}}
	issuePeerTicket(t, ticketStore, "gold-drop2-owner", 0x9b9b9b9b, owner)
	if err := accounts.Save(accountstore.Account{Login: "gold-drop2-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed gold drop2 owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected gold-drop2 runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "gold-drop2-owner", 0x9b9b9b9b)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(6), Gold: 2500, Count: 4})))
	if err != nil {
		t.Fatalf("unexpected gold drop2 error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected gold drop2 to emit POINT_CHANGE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	point, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode gold drop2 point change: %v", err)
	}
	if point != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: -2500, Value: 6500}) {
		t.Fatalf("unexpected gold drop2 point change: %+v", point)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode gold drop2 ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 1 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected gold drop2 ground add: %+v", ground)
	}

	account, err := accounts.Load("gold-drop2-owner")
	if err != nil {
		t.Fatalf("load gold drop2 owner account: %v", err)
	}
	if account.Characters[0].Gold != 6500 {
		t.Fatalf("expected persisted gold 6500 after drop2, got %d", account.Characters[0].Gold)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected gold drop2 to leave inventory unchanged, got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
}

func TestGameRuntimeGoldDropIgnoresItemPositionWhenGoldIsNonZero(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GoldPositionOwner", 0x0103019c, 0x0204019c, 1400, 2400, 0, 101, 201)
	owner.Gold = 7000
	owner.Inventory = []inventory.ItemInstance{{ID: 1102, Vnum: 27032, Count: 4, Slot: 5}}
	issuePeerTicket(t, ticketStore, "gold-position-owner", 0x9c9c9c9c, owner)
	if err := accounts.Save(accountstore.Account{Login: "gold-position-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed gold position owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected gold-position runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "gold-position-owner", 0x9c9c9c9c)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.Position{WindowType: itemproto.WindowEquipment, Cell: itemproto.InventoryMaxCell}, Elk: 1500})))
	if err != nil {
		t.Fatalf("unexpected gold-position drop error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected gold-position drop to emit POINT_CHANGE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	point, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode gold-position point change: %v", err)
	}
	if point != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: -1500, Value: 5500}) {
		t.Fatalf("unexpected gold-position point change: %+v", point)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode gold-position ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 1 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected gold-position ground add: %+v", ground)
	}

	account, err := accounts.Load("gold-position-owner")
	if err != nil {
		t.Fatalf("load gold position owner account: %v", err)
	}
	if account.Characters[0].Gold != 5500 {
		t.Fatalf("expected persisted gold 5500 after position-independent drop, got %d", account.Characters[0].Gold)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected position-independent gold drop to leave inventory unchanged, got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}

	out, err = flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.Position{WindowType: itemproto.WindowEquipment, Cell: itemproto.InventoryMaxCell}, Gold: 500, Count: 7})))
	if err != nil {
		t.Fatalf("unexpected gold-position drop2 error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected gold-position drop2 to emit POINT_CHANGE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	point, err = worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode gold-position drop2 point change: %v", err)
	}
	if point != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: -500, Value: 5000}) {
		t.Fatalf("unexpected gold-position drop2 point change: %+v", point)
	}

	account, err = accounts.Load("gold-position-owner")
	if err != nil {
		t.Fatalf("reload gold position owner account: %v", err)
	}
	if account.Characters[0].Gold != 5000 {
		t.Fatalf("expected persisted gold 5000 after drop2, got %d", account.Characters[0].Gold)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected position-independent gold drop2 to leave inventory unchanged, got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
}

func TestGameRuntimeItemPickupRejectsRestrictedSelfPickupWithoutMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RestrictedPickupOwner", 0x0103019a, 0x0204019a, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1101, Vnum: 27001, Count: 2, Slot: 5}}
	owner.RaceNum = 0
	owner.Job = 0
	issuePeerTicket(t, ticketStore, "restricted-pickup-owner", 0x9a9a9a9a, owner)
	if err := accounts.Save(accountstore.Account{Login: "restricted-pickup-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed restricted pickup owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected restricted pickup runtime error: %v", err)
	}
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "restricted potion", Stackable: true, MaxCount: 200}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "restricted-pickup-owner", 0x9a9a9a9a)
	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(5))
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "restricted potion", Stackable: true, MaxCount: 200, AntiWarrior: true}

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 1 {
		t.Fatalf("expected restricted pickup to emit one info rejection, got %d frames", len(pickupOut))
	}
	rejection, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, pickupOut[0]))
	if err != nil {
		t.Fatalf("decode restricted pickup rejection: %v", err)
	}
	if rejection.Type != chatproto.ChatTypeInfo || rejection.VID != 0 || rejection.Message != itemPickupInventoryFullInfoMessage {
		t.Fatalf("unexpected restricted pickup rejection: %+v", rejection)
	}
	account, err := accounts.Load("restricted-pickup-owner")
	if err != nil {
		t.Fatalf("load restricted pickup owner account: %v", err)
	}
	if len(account.Characters[0].Inventory) != 0 {
		t.Fatalf("expected restricted item to remain out of owner inventory after drop, got %#v", account.Characters[0].Inventory)
	}
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "restricted potion", Stackable: true, MaxCount: 200}
	ownerRetry := pickupGroundItem(t, flow, ground.VID)
	if len(ownerRetry) != 3 {
		t.Fatalf("expected unrestricted owner retry to pick pending ground item back up after template relax, got %d frames", len(ownerRetry))
	}
}

func TestGameRuntimeItemPickupRejectsRestrictedOwnerDeliveryWithoutCollectorMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("RestrictedOwnerDeliveryOwner", 0x0103019e, 0x0204019e, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1103, Vnum: 27001, Count: 199, Slot: 5}}
	owner.RaceNum = 0
	owner.Job = 0
	collector := peerVisibilityCharacter("RestrictedOwnerDeliveryCollector", 0x0103019f, 0x0204019f, 1320, 2320, 0, 101, 201)
	collector.Inventory = []inventory.ItemInstance{{ID: 2103, Vnum: 27001, Count: 1, Slot: 0}}
	ownerLogin := "rod-owner"
	collectorLogin := "rod-collector"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x9e9e9e9e, owner)
	issuePeerTicket(t, ticketStore, collectorLogin, 0x9f9f9f9f, collector)
	for _, account := range []accountstore.Account{
		{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})},
		{Login: collectorLogin, Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("seed %s account: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected restricted owner-delivery pickup runtime error: %v", err)
	}
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "owner restricted potion", Stackable: true, MaxCount: 200}
	factory := runtime.SessionFactory()
	ownerFlow, _ := enterGameWithLoginTicket(t, factory, ownerLogin, 0x9e9e9e9e)
	collectorFlow, _ := enterGameWithLoginTicket(t, factory, collectorLogin, 0x9f9f9f9f)
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, collectorFlow)

	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(5))
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "owner restricted potion", Stackable: true, MaxCount: 200, AntiWarrior: true}
	flushServerFrames(t, collectorFlow)
	pickupOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(pickupOut) != 1 {
		t.Fatalf("expected restricted owner-delivery pickup to emit one info rejection, got %d frames", len(pickupOut))
	}
	rejection, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, pickupOut[0]))
	if err != nil {
		t.Fatalf("decode restricted owner-delivery pickup rejection: %v", err)
	}
	if rejection.Type != chatproto.ChatTypeInfo || rejection.VID != 0 || rejection.Message != itemPickupInventoryFullInfoMessage {
		t.Fatalf("unexpected restricted owner-delivery pickup rejection: %+v", rejection)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected rejected restricted owner-delivery pickup to avoid owner frames, got %d", len(queued))
	}

	ownerAccount, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load restricted owner-delivery owner account: %v", err)
	}
	if len(ownerAccount.Characters[0].Inventory) != 0 {
		t.Fatalf("expected restricted owner-delivery item to remain out of owner inventory, got %#v", ownerAccount.Characters[0].Inventory)
	}
	collectorAccount, err := accounts.Load(collectorLogin)
	if err != nil {
		t.Fatalf("load restricted owner-delivery collector account: %v", err)
	}
	if !reflect.DeepEqual(collectorAccount.Characters[0].Inventory, collector.Inventory) {
		t.Fatalf("expected rejected restricted owner-delivery pickup to leave collector inventory unchanged, got %#v want %#v", collectorAccount.Characters[0].Inventory, collector.Inventory)
	}
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "owner restricted potion", Stackable: true, MaxCount: 200}
	ownerRetry := pickupGroundItem(t, ownerFlow, ground.VID)
	if len(ownerRetry) != 3 {
		t.Fatalf("expected owner retry after relaxing restriction to pick pending ground item back up, got %d frames", len(ownerRetry))
	}
}

func TestGameRuntimeItemPickupRejectsTransferGuardSelfPickupWithoutMutation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*itemcatalog.Template)
	}{
		{name: "anti-give", mutate: func(template *itemcatalog.Template) { template.AntiGive = true }},
		{name: "anti-drop", mutate: func(template *itemcatalog.Template) { template.AntiDrop = true }},
		{name: "anti-sell", mutate: func(template *itemcatalog.Template) { template.AntiSell = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticketStore := loginticket.NewFileStore(t.TempDir())
			accounts := accountstore.NewFileStore(t.TempDir())
			owner := peerVisibilityCharacter("TransferGuardSelfPickup", 0x010301aa, 0x020401aa, 1300, 2300, 0, 101, 201)
			owner.Inventory = []inventory.ItemInstance{{ID: 1104, Vnum: 27001, Count: 2, Slot: 5}}
			login := "transfer-guard-self-pickup"
			issuePeerTicket(t, ticketStore, login, 0xaaaaaaaa, owner)
			if err := accounts.Save(accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
				t.Fatalf("seed %s self-pickup account: %v", tc.name, err)
			}

			runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
			if err != nil {
				t.Fatalf("unexpected %s self-pickup runtime error: %v", tc.name, err)
			}
			runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "transfer-guard self potion", Stackable: true, MaxCount: 200}
			flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, 0xaaaaaaaa)
			ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(5))
			template := itemcatalog.Template{Vnum: 27001, Name: "transfer-guard self potion", Stackable: true, MaxCount: 200}
			tc.mutate(&template)
			runtime.itemTemplates[27001] = template

			pickupOut := pickupGroundItem(t, flow, ground.VID)
			if len(pickupOut) != 1 {
				t.Fatalf("expected %s self-pickup to emit one info rejection, got %d frames", tc.name, len(pickupOut))
			}
			rejection, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, pickupOut[0]))
			if err != nil {
				t.Fatalf("decode %s self-pickup rejection: %v", tc.name, err)
			}
			if rejection.Type != chatproto.ChatTypeInfo || rejection.VID != 0 || rejection.Message != itemPickupInventoryFullInfoMessage {
				t.Fatalf("unexpected %s self-pickup rejection: %+v", tc.name, rejection)
			}
			account, err := accounts.Load(login)
			if err != nil {
				t.Fatalf("load %s self-pickup account: %v", tc.name, err)
			}
			if len(account.Characters[0].Inventory) != 0 {
				t.Fatalf("expected %s item to remain out of owner inventory after drop, got %#v", tc.name, account.Characters[0].Inventory)
			}
			runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "transfer-guard self potion", Stackable: true, MaxCount: 200}
			ownerRetry := pickupGroundItem(t, flow, ground.VID)
			if len(ownerRetry) != 3 {
				t.Fatalf("expected unrestricted self retry to pick pending ground item back up after %s relax, got %d frames", tc.name, len(ownerRetry))
			}
		})
	}
}

func TestGameRuntimeItemPickupRejectsAntiGiveOwnerDeliveryWithoutCollectorMutation(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("AntiGivePickupOwner", 0x0103019c, 0x0204019c, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1102, Vnum: 27001, Count: 199, Slot: 5}}
	collector := peerVisibilityCharacter("AntiGivePickupCollector", 0x0103019d, 0x0204019d, 1320, 2320, 0, 101, 201)
	collector.Inventory = []inventory.ItemInstance{{ID: 2102, Vnum: 27001, Count: 1, Slot: 0}}
	issuePeerTicket(t, ticketStore, "anti-give-pickup-owner", 0x9c9c9c9c, owner)
	issuePeerTicket(t, ticketStore, "anti-give-pickup-collector", 0x9d9d9d9d, collector)
	for _, account := range []accountstore.Account{
		{Login: "anti-give-pickup-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})},
		{Login: "anti-give-pickup-collector", Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("seed %s account: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected anti-give pickup runtime error: %v", err)
	}
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "anti-give potion", Stackable: true, MaxCount: 200}
	factory := runtime.SessionFactory()
	ownerFlow, _ := enterGameWithLoginTicket(t, factory, "anti-give-pickup-owner", 0x9c9c9c9c)
	collectorFlow, _ := enterGameWithLoginTicket(t, factory, "anti-give-pickup-collector", 0x9d9d9d9d)
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, collectorFlow)

	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(5))
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "anti-give potion", Stackable: true, MaxCount: 200, AntiGive: true}
	flushServerFrames(t, collectorFlow)
	pickupOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(pickupOut) != 1 {
		t.Fatalf("expected anti-give pickup to emit one info rejection, got %d frames", len(pickupOut))
	}
	rejection, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, pickupOut[0]))
	if err != nil {
		t.Fatalf("decode anti-give pickup rejection: %v", err)
	}
	if rejection.Type != chatproto.ChatTypeInfo || rejection.VID != 0 || rejection.Message != itemPickupInventoryFullInfoMessage {
		t.Fatalf("unexpected anti-give pickup rejection: %+v", rejection)
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected rejected anti-give pickup to avoid owner frames, got %d", len(queued))
	}

	ownerAccount, err := accounts.Load("anti-give-pickup-owner")
	if err != nil {
		t.Fatalf("load anti-give pickup owner account: %v", err)
	}
	if len(ownerAccount.Characters[0].Inventory) != 0 {
		t.Fatalf("expected dropped anti-give item to remain out of owner inventory, got %#v", ownerAccount.Characters[0].Inventory)
	}
	collectorAccount, err := accounts.Load("anti-give-pickup-collector")
	if err != nil {
		t.Fatalf("load anti-give pickup collector account: %v", err)
	}
	if !reflect.DeepEqual(collectorAccount.Characters[0].Inventory, collector.Inventory) {
		t.Fatalf("expected rejected anti-give pickup to leave collector inventory unchanged, got %#v want %#v", collectorAccount.Characters[0].Inventory, collector.Inventory)
	}
	runtime.itemTemplates[27001] = itemcatalog.Template{Vnum: 27001, Name: "anti-give potion", Stackable: true, MaxCount: 200}
	ownerRetry := pickupGroundItem(t, ownerFlow, ground.VID)
	if len(ownerRetry) != 3 {
		t.Fatalf("expected owner retry after relaxing anti-give to pick pending ground item back up, got %d frames", len(ownerRetry))
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
	if len(pickupOut) != 3 {
		t.Fatalf("expected pickup to emit GROUND_DEL, ITEM_SET, and ITEM_GET, got %d frames", len(pickupOut))
	}
	groundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, pickupOut[0]))
	if err != nil {
		t.Fatalf("decode pickup ground del: %v", err)
	}
	if groundDel.VID != ground.VID {
		t.Fatalf("unexpected pickup ground del vid: got %d want %d", groundDel.VID, ground.VID)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, pickupOut[1]))
	if err != nil {
		t.Fatalf("decode pickup item set: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(5) || set.Vnum != 27002 || set.Count != 4 {
		t.Fatalf("unexpected pickup item set: %+v", set)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, pickupOut[2]))
	if err != nil {
		t.Fatalf("decode pickup item get: %v", err)
	}
	if get != (itemproto.GetPacket{Vnum: 27002, Count: 4, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("unexpected pickup item get: %+v", get)
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

func TestGameRuntimeItemPickupRejectsDeadCollectorWithoutRemovingGroundHandle(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadPickupOwner", 0x010301bc, 0x020401bc, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1076, Vnum: 27032, Count: 1, Slot: 5}}
	collector := peerVisibilityCharacter("DeadPickupCollector", 0x010301bd, 0x020401bd, owner.X, owner.Y, 1, 101, 201)
	collector.Points[bootstrapPlayerPointValueIndex] = 0
	issuePeerTicket(t, ticketStore, "dead-pickup-owner", 0xbcbcbcbc, owner)
	issuePeerTicket(t, ticketStore, "dead-pickup-collector", 0xbdbdbdbd, collector)
	if err := accounts.Save(accountstore.Account{Login: "dead-pickup-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed dead-pickup owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "dead-pickup-collector", Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed dead-pickup collector account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected dead-pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "dead-pickup-owner", 0xbcbcbcbc)
	defer closeSessionFlow(t, ownerFlow)
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "dead-pickup-collector", 0xbdbdbdbd)
	defer closeSessionFlow(t, collectorFlow)
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, collectorFlow)

	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(5))
	if queued := flushServerFrames(t, collectorFlow); len(queued) != 0 {
		t.Fatalf("expected dead collector not to receive queued ground visibility frames, got %d", len(queued))
	}
	if deadOut := pickupGroundItem(t, collectorFlow, ground.VID); len(deadOut) != 0 {
		t.Fatalf("expected dead collector pickup to fail closed, got %d frames", len(deadOut))
	}

	ownerPickup := pickupGroundItem(t, ownerFlow, ground.VID)
	if len(ownerPickup) != 3 {
		t.Fatalf("expected rejected dead pickup to leave ground item available for owner, got %d owner pickup frames", len(ownerPickup))
	}
	groundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, ownerPickup[0]))
	if err != nil {
		t.Fatalf("decode owner pickup ground del after dead collector rejection: %v", err)
	}
	if groundDel.VID != ground.VID {
		t.Fatalf("unexpected owner pickup ground del after dead collector rejection: got %d want %d", groundDel.VID, ground.VID)
	}
}

func TestGameRuntimeItemPickupRemovesHandleButSkipsDeadOwnerDeliveryFrames(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DeadOwnerFallback", 0x010301be, 0x020401be, 1300, 2300, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1077, Vnum: 27033, Count: 1, Slot: 5}}
	collector := peerVisibilityCharacter("DeadOwnerCollector", 0x010301bf, 0x020401bf, owner.X, owner.Y, 1, 101, 201)
	issuePeerTicket(t, ticketStore, "dead-owner-fallback", 0xbebebebe, owner)
	issuePeerTicket(t, ticketStore, "dead-owner-collector", 0xbfbfbfbf, collector)
	if err := accounts.Save(accountstore.Account{Login: "dead-owner-fallback", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed dead-owner fallback account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "dead-owner-collector", Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed dead-owner collector account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected dead-owner fallback runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "dead-owner-fallback", 0xbebebebe)
	defer closeSessionFlow(t, ownerFlow)
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "dead-owner-collector", 0xbfbfbfbf)
	defer closeSessionFlow(t, collectorFlow)
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, collectorFlow)

	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(5))
	if queued := flushServerFrames(t, collectorFlow); len(queued) != 2 {
		t.Fatalf("expected collector to see owner ground add and ownership before owner death, got %d queued frames", len(queued))
	}
	deadOwner := owner
	deadOwner.Points[bootstrapPlayerPointValueIndex] = 0
	if !runtime.applyLiveCharacterPersistedSnapshot(owner.Name, deadOwner) {
		t.Fatal("expected owner live snapshot update to mark owner dead")
	}
	runtime.sharedWorld.UpdateCharacterWithVisibilityTransition(1, owner, deadOwner, nil)
	_ = flushServerFrames(t, ownerFlow)

	collectorOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(collectorOut) != 3 {
		t.Fatalf("expected collector fallback pickup to emit GROUND_DEL, ITEM_SET, and ITEM_GET, got %d frames", len(collectorOut))
	}
	if _, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, collectorOut[0])); err != nil {
		t.Fatalf("decode collector fallback ground del: %v", err)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, collectorOut[1]))
	if err != nil {
		t.Fatalf("decode collector fallback item set: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(5) || set.Vnum != 27033 || set.Count != 1 {
		t.Fatalf("unexpected collector fallback item set: %+v", set)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, collectorOut[2]))
	if err != nil {
		t.Fatalf("decode collector fallback item get: %v", err)
	}
	if get != (itemproto.GetPacket{Vnum: 27033, Count: 1, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("expected normal collector pickup notice after dead-owner fallback, got %+v", get)
	}
	if ownerQueued := flushServerFrames(t, ownerFlow); len(ownerQueued) != 0 {
		t.Fatalf("expected dead owner not to receive owner-delivery frames after collector fallback, got %d", len(ownerQueued))
	}
	ownerAccount, err := accounts.Load("dead-owner-fallback")
	if err != nil {
		t.Fatalf("load owner after dead-owner fallback: %v", err)
	}
	if len(ownerAccount.Characters[0].Inventory) != 0 {
		t.Fatalf("expected dead owner inventory to remain dropped/empty, got %#v", ownerAccount.Characters[0].Inventory)
	}
	collectorAccount, err := accounts.Load("dead-owner-collector")
	if err != nil {
		t.Fatalf("load collector after dead-owner fallback: %v", err)
	}
	if !reflect.DeepEqual(collectorAccount.Characters[0].Inventory, []inventory.ItemInstance{{ID: 1077, Vnum: 27033, Count: 1, Slot: 5}}) {
		t.Fatalf("unexpected collector inventory after dead-owner fallback: %#v", collectorAccount.Characters[0].Inventory)
	}
}

func TestGameRuntimeMapOccupancyIncludesPendingGroundItems(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GroundMapOwner", 0x010301b4, 0x020401b4, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1074, Vnum: 27031, Count: 1, Slot: 6}}
	issuePeerTicket(t, ticketStore, "ground-map-owner", 0xb4b4b4b4, owner)
	if err := accounts.Save(accountstore.Account{Login: "ground-map-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed ground map owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected ground map runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ground-map-owner", 0xb4b4b4b4)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))

	maps := runtime.MapOccupancy()
	if len(maps) != 1 {
		t.Fatalf("expected one occupied map after ground drop, got %+v", maps)
	}
	if maps[0].MapIndex != bootstrapMapIndex || maps[0].GroundItemCount != 1 || len(maps[0].GroundItems) != 1 {
		t.Fatalf("unexpected ground occupancy summary: %+v", maps[0])
	}
	got := maps[0].GroundItems[0]
	if got.VID != ground.VID || got.Vnum != ground.Vnum || got.Count != 1 || got.OwnerName != owner.Name || got.X != owner.X || got.Y != owner.Y || got.Z != owner.Z {
		t.Fatalf("unexpected ground occupancy item: got %+v want vid=%d vnum=%d count=1 owner=%q pos=(%d,%d,%d)", got, ground.VID, ground.Vnum, owner.Name, owner.X, owner.Y, owner.Z)
	}

	_ = pickupGroundItem(t, ownerFlow, ground.VID)
	maps = runtime.MapOccupancy()
	if len(maps) != 1 || maps[0].GroundItemCount != 0 || len(maps[0].GroundItems) != 0 {
		t.Fatalf("expected ground occupancy to clear after pickup while character remains connected, got %+v", maps)
	}
}

func TestGameRuntimeMapOccupancyIncludesPendingGroundGold(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GroundGoldMapOwner", 0x010301ba, 0x020401ba, 1450, 2450, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 1075, Vnum: 27031, Count: 1, Slot: 6}}
	issuePeerTicket(t, ticketStore, "ground-gold-map-owner", 0xbabababa, owner)
	if err := accounts.Save(accountstore.Account{Login: "ground-gold-map-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed ground gold map owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected ground gold map runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ground-gold-map-owner", 0xbabababa)
	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(6), Elk: 1200})))
	if err != nil {
		t.Fatalf("unexpected gold drop error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected gold drop to emit POINT_CHANGE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode ground gold add: %v", err)
	}

	maps := runtime.MapOccupancy()
	if len(maps) != 1 {
		t.Fatalf("expected one occupied map after ground gold drop, got %+v", maps)
	}
	if maps[0].MapIndex != bootstrapMapIndex || maps[0].GroundItemCount != 1 || len(maps[0].GroundItems) != 1 {
		t.Fatalf("unexpected ground gold occupancy summary: %+v", maps[0])
	}
	got := maps[0].GroundItems[0]
	if got.VID != ground.VID || got.Vnum != 1 || got.Count != 0 || got.GoldAmount != 1200 || got.OwnerName != owner.Name || got.X != owner.X || got.Y != owner.Y || got.Z != owner.Z {
		t.Fatalf("unexpected ground gold occupancy item: got %+v want vid=%d vnum=1 gold=1200 owner=%q pos=(%d,%d,%d)", got, ground.VID, owner.Name, owner.X, owner.Y, owner.Z)
	}

	_ = pickupGroundItem(t, ownerFlow, ground.VID)
	maps = runtime.MapOccupancy()
	if len(maps) != 1 || maps[0].GroundItemCount != 0 || len(maps[0].GroundItems) != 0 {
		t.Fatalf("expected ground gold occupancy to clear after pickup while character remains connected, got %+v", maps)
	}
}

func TestGameRuntimeItemDropBootstrapsPendingGroundItemsForLateVisibleEntrant(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GroundLateOwner", 0x01030194, 0x02040194, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1034, Vnum: 27031, Count: 1, Slot: 6}}
	watcher := peerVisibilityCharacter("GroundLateWatcher", 0x01030195, 0x02040195, 1450, 2450, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "ground-late-owner", 0x94949494, owner)
	issuePeerTicket(t, ticketStore, "ground-late-watcher", 0x95959595, watcher)
	for _, account := range []accountstore.Account{
		{Login: "ground-late-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})},
		{Login: "ground-late-watcher", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("seed %s account: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected late-ground runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ground-late-owner", 0x94949494)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))

	_, watcherEnter := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ground-late-watcher", 0x95959595)
	if len(watcherEnter) != 10 {
		t.Fatalf("expected self bootstrap, owner peer burst, and pending ground add/ownership for late entrant, got %d frames", len(watcherEnter))
	}
	lateGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, watcherEnter[8]))
	if err != nil {
		t.Fatalf("decode late entrant ground add: %v", err)
	}
	if lateGround != ground {
		t.Fatalf("unexpected late entrant ground add: got %+v want %+v", lateGround, ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, watcherEnter[9]))
	if err != nil {
		t.Fatalf("decode late entrant ground ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected late entrant ground ownership: got %+v want vid %d owner %q", ownership, ground.VID, owner.Name)
	}
}

func TestGameRuntimeRadiusAOIItemDropPickupRebuildsGroundVisibilityOnMove(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropAOIOwner", 0x01030176, 0x02040176, 1100, 2100, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1005, Vnum: 27004, Count: 2, Slot: 7}}
	watcher := peerVisibilityCharacter("DropAOIWatcher", 0x01030177, 0x02040177, 1900, 2900, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "drop-aoi-owner", 0x67676767, owner)
	issuePeerTicket(t, ticketStore, "drop-aoi-watcher", 0x77777777, watcher)
	if err := accounts.Save(accountstore.Account{Login: "drop-aoi-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed drop aoi owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "drop-aoi-watcher", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed drop aoi watcher account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     400,
		VisibilitySectorSize: 200,
	}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected radius item-drop runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-aoi-owner", 0x67676767)
	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "drop-aoi-watcher", 0x77777777)
	flushServerFrames(t, ownerFlow)
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected initially out-of-range watcher to see no owner frames, got %d", len(queued))
	}

	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(7))
	if queued := flushServerFrames(t, watcherFlow); len(queued) != 0 {
		t.Fatalf("expected out-of-range watcher to miss ground add, got %d frames", len(queued))
	}

	moveIn, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1200, Y: 2200, Time: 0x11121314})))
	if err != nil {
		t.Fatalf("unexpected watcher move-in error: %v", err)
	}
	if len(moveIn) != 1 {
		t.Fatalf("expected one self move ack for watcher move-in, got %d", len(moveIn))
	}
	queuedIn := flushServerFrames(t, watcherFlow)
	if len(queuedIn) != 5 {
		t.Fatalf("expected peer bootstrap plus ground add/ownership after moving into range, got %d frames", len(queuedIn))
	}
	peerGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, queuedIn[3]))
	if err != nil {
		t.Fatalf("decode moved-in watcher ground add: %v", err)
	}
	if peerGround != ground {
		t.Fatalf("unexpected moved-in watcher ground add: got %+v want %+v", peerGround, ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, queuedIn[4]))
	if err != nil {
		t.Fatalf("decode moved-in watcher ground ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected moved-in watcher ground ownership: got %+v want vid %d owner %q", ownership, ground.VID, owner.Name)
	}

	moveOut, err := watcherFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 8, X: 1900, Y: 2900, Time: 0x11121315})))
	if err != nil {
		t.Fatalf("unexpected watcher move-out error: %v", err)
	}
	if len(moveOut) != 1 {
		t.Fatalf("expected one self move ack for watcher move-out, got %d", len(moveOut))
	}
	queuedOut := flushServerFrames(t, watcherFlow)
	if len(queuedOut) != 2 {
		t.Fatalf("expected peer delete plus ground delete after moving out of range, got %d frames", len(queuedOut))
	}
	groundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, queuedOut[1]))
	if err != nil {
		t.Fatalf("decode moved-out watcher ground delete: %v", err)
	}
	if groundDel.VID != ground.VID {
		t.Fatalf("unexpected moved-out watcher ground delete: got %+v want vid %d", groundDel, ground.VID)
	}
}

func TestGameRuntimeExactPositionTransferRebuildsGroundItemVisibility(t *testing.T) {
	assertExactPositionTransferRebuildsGroundItemVisibility(t, func(flow service.SessionFlow, mover loginticket.Character) ([][]byte, error) {
		return flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x51525354})))
	})
}

func TestGameRuntimeExactPositionSyncTransferRebuildsGroundItemVisibility(t *testing.T) {
	assertExactPositionTransferRebuildsGroundItemVisibility(t, func(flow service.SessionFlow, mover loginticket.Character) ([][]byte, error) {
		return flow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeSyncPosition(movep.SyncPositionPacket{Elements: []movep.SyncPositionElement{{VID: mover.VID, X: 1500, Y: 2600}}})))
	})
}

func assertExactPositionTransferRebuildsGroundItemVisibility(t *testing.T, trigger func(service.SessionFlow, loginticket.Character) ([][]byte, error)) {
	t.Helper()
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	sourceDropper := peerVisibilityCharacter("TransferGroundSource", 0x01030178, 0x02040178, 1100, 2100, 0, 101, 201)
	sourceDropper.Inventory = []inventory.ItemInstance{{ID: 1006, Vnum: 27005, Count: 1, Slot: 8}}
	mover := peerVisibilityCharacter("TransferGroundMover", 0x01030179, 0x02040179, 1300, 2300, 0, 101, 201)
	destDropper := peerVisibilityCharacter("TransferGroundDest", 0x0103017a, 0x0204017a, 1700, 2800, 0, 101, 201)
	destDropper.MapIndex = 42
	destDropper.Inventory = []inventory.ItemInstance{{ID: 1007, Vnum: 27006, Count: 1, Slot: 9}}
	issuePeerTicket(t, ticketStore, "transfer-ground-source", 0x78787878, sourceDropper)
	issuePeerTicket(t, ticketStore, "transfer-ground-mover", 0x79797979, mover)
	issuePeerTicket(t, ticketStore, "transfer-ground-dest", 0x7a7a7a7a, destDropper)
	for _, account := range []accountstore.Account{
		{Login: "transfer-ground-source", Empire: sourceDropper.Empire, Characters: cloneCharacters([]loginticket.Character{sourceDropper})},
		{Login: "transfer-ground-mover", Empire: mover.Empire, Characters: cloneCharacters([]loginticket.Character{mover})},
		{Login: "transfer-ground-dest", Empire: destDropper.Empire, Characters: cloneCharacters([]loginticket.Character{destDropper})},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("seed %s account: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStoreAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, []bootstrapTransferTrigger{{
		SourceMapIndex: bootstrapMapIndex,
		SourceX:        1500,
		SourceY:        2600,
		TargetMapIndex: 42,
		TargetX:        1700,
		TargetY:        2800,
	}})
	if err != nil {
		t.Fatalf("unexpected transfer ground runtime error: %v", err)
	}
	factory := runtime.SessionFactory()
	sourceFlow, _ := enterGameWithLoginTicket(t, factory, "transfer-ground-source", 0x78787878)
	moverFlow, _ := enterGameWithLoginTicket(t, factory, "transfer-ground-mover", 0x79797979)
	destFlow, _ := enterGameWithLoginTicket(t, factory, "transfer-ground-dest", 0x7a7a7a7a)
	flushServerFrames(t, sourceFlow)
	flushServerFrames(t, moverFlow)
	flushServerFrames(t, destFlow)

	sourceGround := dropAndDecodeGroundAdd(t, sourceFlow, itemproto.InventoryPosition(8))
	if queued := flushServerFrames(t, moverFlow); len(queued) != 2 {
		t.Fatalf("expected mover to see source-map ground add plus ownership before transfer, got %d frames", len(queued))
	}
	destGround := dropAndDecodeGroundAdd(t, destFlow, itemproto.InventoryPosition(9))
	if queued := flushServerFrames(t, moverFlow); len(queued) != 0 {
		t.Fatalf("expected mover to miss destination-map ground add before transfer, got %d frames", len(queued))
	}

	transferOut, err := trigger(moverFlow, mover)
	if err != nil {
		t.Fatalf("unexpected transfer trigger error: %v", err)
	}
	if len(transferOut) != 11 {
		t.Fatalf("expected self bootstrap, peer del/add, and ground del/add/ownership transfer frames, got %d", len(transferOut))
	}
	sourceDelete, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, transferOut[8]))
	if err != nil {
		t.Fatalf("decode transfer source ground delete: %v", err)
	}
	if sourceDelete.VID != sourceGround.VID {
		t.Fatalf("unexpected source ground delete after transfer: got %+v want vid %d", sourceDelete, sourceGround.VID)
	}
	destAdd, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, transferOut[9]))
	if err != nil {
		t.Fatalf("decode transfer destination ground add: %v", err)
	}
	if destAdd != destGround {
		t.Fatalf("unexpected destination ground add after transfer: got %+v want %+v", destAdd, destGround)
	}
	destOwnership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, transferOut[10]))
	if err != nil {
		t.Fatalf("decode transfer destination ground ownership: %v", err)
	}
	if destOwnership != (itemproto.OwnershipPacket{VID: destGround.VID, OwnerName: destDropper.Name}) {
		t.Fatalf("unexpected destination ground ownership after transfer: got %+v want vid %d owner %q", destOwnership, destGround.VID, destDropper.Name)
	}
	if queued := flushServerFrames(t, moverFlow); len(queued) != 0 {
		t.Fatalf("expected no queued mover frames after immediate transfer ground rebuild, got %d", len(queued))
	}
}

func TestGameRuntimeItemPickupRejectsVisibleButOutOfReachGroundItem(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupFarOwner", 0x01030198, 0x02040198, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1030, Vnum: 27001, Count: 2, Slot: 6}}
	collector := peerVisibilityCharacter("PickupFarCollector", 0x01030199, 0x02040199, 1900, 2400, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "pickup-far-owner", 0x98989898, owner)
	issuePeerTicket(t, ticketStore, "pickup-far-collector", 0x99999999, collector)
	for _, account := range []accountstore.Account{
		{Login: "pickup-far-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})},
		{Login: "pickup-far-collector", Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("seed %s account: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{
		LegacyAddr:           ":13000",
		PublicAddr:           "127.0.0.1",
		VisibilityMode:       "radius",
		VisibilityRadius:     900,
		VisibilitySectorSize: 300,
	}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected out-of-reach pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-far-owner", 0x98989898)
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-far-collector", 0x99999999)
	flushServerFrames(t, ownerFlow)
	flushServerFrames(t, collectorFlow)

	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))
	if queued := flushServerFrames(t, collectorFlow); len(queued) != 2 {
		t.Fatalf("expected collector to see visible-but-far ground item, got %d queued frames", len(queued))
	}
	out := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(out) != 0 {
		t.Fatalf("expected visible-but-out-of-reach pickup to fail closed, got %d frames", len(out))
	}

	ownerAccount, err := accounts.Load("pickup-far-owner")
	if err != nil {
		t.Fatalf("load out-of-reach pickup owner account: %v", err)
	}
	if !reflect.DeepEqual(ownerAccount.Characters[0].Inventory, []inventory.ItemInstance{}) {
		t.Fatalf("expected owner inventory to remain dropped after rejected far pickup, got %#v", ownerAccount.Characters[0].Inventory)
	}
	collectorAccount, err := accounts.Load("pickup-far-collector")
	if err != nil {
		t.Fatalf("load out-of-reach pickup collector account: %v", err)
	}
	if len(collectorAccount.Characters[0].Inventory) != 0 {
		t.Fatalf("expected rejected far pickup to leave collector inventory unchanged, got %#v", collectorAccount.Characters[0].Inventory)
	}
	if retry := pickupGroundItem(t, ownerFlow, ground.VID); len(retry) != 3 {
		t.Fatalf("expected ground handle to remain pending for owner retry, got %d frames", len(retry))
	}
}

func TestGameRuntimeItemPickupMergesOwnedVisibleDropIntoCompatibleStack(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupMergeOwner", 0x01030182, 0x02040182, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1011, Vnum: 27001, Count: 2, Slot: 6},
		{ID: 2011, Vnum: 27001, Count: 3, Slot: 0},
	}
	issuePeerTicket(t, ticketStore, "pickup-merge-owner", 0x82828282, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-merge-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup merge owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected item-pickup merge runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-merge-owner", 0x82828282)
	flushServerFrames(t, flow)
	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(6))

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 3 {
		t.Fatalf("expected merged pickup to emit GROUND_DEL, ITEM_UPDATE, and ITEM_GET, got %d frames", len(pickupOut))
	}
	groundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, pickupOut[0]))
	if err != nil {
		t.Fatalf("decode merged pickup ground del: %v", err)
	}
	if groundDel.VID != ground.VID {
		t.Fatalf("unexpected merged pickup ground del: got %+v want vid %d", groundDel, ground.VID)
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, pickupOut[1]))
	if err != nil {
		t.Fatalf("decode merged pickup item update: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(0) || update.Count != 5 {
		t.Fatalf("expected pickup to merge into lowest compatible slot 0 with count 5, got %+v", update)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, pickupOut[2]))
	if err != nil {
		t.Fatalf("decode merged pickup item get: %v", err)
	}
	if get != (itemproto.GetPacket{Vnum: 27001, Count: 2, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("unexpected merged pickup item get: %+v", get)
	}
	account, err := accounts.Load("pickup-merge-owner")
	if err != nil {
		t.Fatalf("load pickup merge owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 2011, Vnum: 27001, Count: 5, Slot: 0}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after merged pickup: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	if replayOut := pickupGroundItem(t, flow, ground.VID); len(replayOut) != 0 {
		t.Fatalf("expected replay after merged pickup to fail closed, got %d frames", len(replayOut))
	}
}

func TestGameRuntimeItemPickupRejectsMissingTemplateWithoutRemovingGroundItem(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupTemplateMissingOwner", 0x010301a3, 0x020401a3, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1041, Vnum: 27006, Count: 2, Slot: 6}}
	issuePeerTicket(t, ticketStore, "pickup-template-missing-owner", 0xa3a3a3a3, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-template-missing-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup template-missing owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27006, Name: "Ground Potion", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected item-pickup template-missing runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-template-missing-owner", 0xa3a3a3a3)
	flushServerFrames(t, flow)
	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(6))
	delete(runtime.itemTemplates, 27006)

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 0 {
		t.Fatalf("expected missing pickup template to reject without frames, got %d", len(pickupOut))
	}
	account, err := accounts.Load("pickup-template-missing-owner")
	if err != nil {
		t.Fatalf("load pickup template-missing owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{}) {
		t.Fatalf("expected rejected missing-template pickup to leave owner inventory dropped, got %#v", account.Characters[0].Inventory)
	}

	runtime.itemTemplates[27006] = itemcatalog.Template{Vnum: 27006, Name: "Recovered Ground Potion", Stackable: true, MaxCount: 200}
	retryOut := pickupGroundItem(t, flow, ground.VID)
	if len(retryOut) != 3 {
		t.Fatalf("expected ground handle to remain pending after missing-template rejection, got %d frames", len(retryOut))
	}
}

func TestGameRuntimeItemPickupRejectsOverTemplateMaxGroundStackWithoutRemovingGroundItem(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupOverMaxOwner", 0x01030195, 0x02040195, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1023, Vnum: 27007, Count: 2, Slot: 6}}
	issuePeerTicket(t, ticketStore, "pickup-over-max-owner", 0x95959595, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-over-max-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup over-template-max owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27007,
		Name:      "Tiny Stack Potion",
		Stackable: true,
		MaxCount:  200,
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected item-pickup over-template-max runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-over-max-owner", 0x95959595)
	flushServerFrames(t, flow)
	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(6))
	runtime.itemTemplates[27007] = itemcatalog.Template{Vnum: 27007, Name: "Tiny Stack Potion", Stackable: true, MaxCount: 1}

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 0 {
		t.Fatalf("expected over-template-max pickup to reject without frames, got %d", len(pickupOut))
	}
	account, err := accounts.Load("pickup-over-max-owner")
	if err != nil {
		t.Fatalf("load pickup over-template-max owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{}) {
		t.Fatalf("expected rejected pickup to leave owner inventory dropped, got %#v", account.Characters[0].Inventory)
	}

	runtime.itemTemplates[27007] = itemcatalog.Template{Vnum: 27007, Name: "Tiny Stack Potion", Stackable: true, MaxCount: 200}
	retryOut := pickupGroundItem(t, flow, ground.VID)
	if len(retryOut) != 3 {
		t.Fatalf("expected ground handle to remain pending after over-template-max rejection, got %d frames", len(retryOut))
	}
}

func TestGameRuntimeItemPickupRejectsAuthoredEquipSlotTemplateWithoutRemovingGroundItem(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupEquipTemplateOwner", 0x010301a4, 0x020401a4, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1042, Vnum: 27008, Count: 1, Slot: 6}}
	issuePeerTicket(t, ticketStore, "pickup-equip-template-owner", 0xa4a4a4a4, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-equip-template-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup equip-template owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27008, Name: "Ground Test Item", Stackable: true, MaxCount: 200}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected item-pickup equip-template runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-equip-template-owner", 0xa4a4a4a4)
	flushServerFrames(t, flow)
	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(6))
	runtime.itemTemplates[27008] = itemcatalog.Template{Vnum: 27008, Name: "Authored Ground Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String()}

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 0 {
		t.Fatalf("expected authored equip-slot pickup template to reject without frames, got %d", len(pickupOut))
	}
	account, err := accounts.Load("pickup-equip-template-owner")
	if err != nil {
		t.Fatalf("load pickup equip-template owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{}) {
		t.Fatalf("expected rejected equip-template pickup to leave owner inventory dropped, got %#v", account.Characters[0].Inventory)
	}

	runtime.itemTemplates[27008] = itemcatalog.Template{Vnum: 27008, Name: "Ground Test Item", Stackable: true, MaxCount: 200}
	retryOut := pickupGroundItem(t, flow, ground.VID)
	if len(retryOut) != 3 {
		t.Fatalf("expected ground handle to remain pending after equip-template rejection, got %d frames", len(retryOut))
	}
}

func TestGameRuntimeItemPickupRejectsMismatchedLoadedTemplateWithoutRemovingGroundItem(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupTemplateMismatchOwner", 0x01030191, 0x02040191, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1021, Vnum: 27004, Count: 2, Slot: 6}}
	issuePeerTicket(t, ticketStore, "pickup-template-mismatch-owner", 0x91919191, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-template-mismatch-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup template-mismatch owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27004,
		Name:      "Wrong Ground Potion",
		Stackable: true,
		MaxCount:  200,
	}})
	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected item-pickup template-mismatch runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-template-mismatch-owner", 0x91919191)
	flushServerFrames(t, flow)
	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(6))
	// Mutate the loaded runtime template after drop so the ground item's vnum still
	// has an entry, but the authored template no longer matches that vnum at pickup.
	runtime.itemTemplates[27004] = itemcatalog.Template{Vnum: 27005, Name: "Mismatched Ground Potion", Stackable: true, MaxCount: 200}

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 0 {
		t.Fatalf("expected mismatched pickup template to reject without frames, got %d", len(pickupOut))
	}
	account, err := accounts.Load("pickup-template-mismatch-owner")
	if err != nil {
		t.Fatalf("load pickup template-mismatch owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, []inventory.ItemInstance{}) {
		t.Fatalf("expected rejected pickup to leave owner inventory dropped, got %#v", account.Characters[0].Inventory)
	}

	// Restore a valid matching template before retrying so the pending handle can
	// prove it was not removed by the failed metadata guard.
	runtime.itemTemplates[27004] = itemcatalog.Template{Vnum: 27004, Name: "Wrong Ground Potion", Stackable: true, MaxCount: 200}
	retryOut := pickupGroundItem(t, flow, ground.VID)
	if len(retryOut) != 3 {
		t.Fatalf("expected ground handle to remain pending after metadata rejection, got %d frames", len(retryOut))
	}
}

func TestGameRuntimeItemPickupAntiStackTemplateRestoresFreshSlotWithoutMerging(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupAntiStackOwner", 0x0103018f, 0x0204018f, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 2019, Vnum: 27003, Count: 3, Slot: 0},
		{ID: 1019, Vnum: 27003, Count: 2, Slot: 6},
	}
	issuePeerTicket(t, ticketStore, "pickup-anti-stack-owner", 0x8f8f8f8f, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-anti-stack-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup anti-stack owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{
		Vnum:      27003,
		Name:      "Anti-stack Potion",
		Stackable: true,
		MaxCount:  200,
		AntiStack: true,
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected item-pickup anti-stack runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-anti-stack-owner", 0x8f8f8f8f)
	flushServerFrames(t, flow)
	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(6))

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 3 {
		t.Fatalf("expected anti-stack pickup to emit GROUND_DEL, ITEM_SET, and ITEM_GET, got %d frames", len(pickupOut))
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, pickupOut[1]))
	if err != nil {
		t.Fatalf("decode anti-stack pickup item set: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(6) || set.Vnum != 27003 || set.Count != 2 {
		t.Fatalf("expected anti-stack pickup to restore dropped stack into fresh slot 6 without merging, got %+v", set)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, pickupOut[2]))
	if err != nil {
		t.Fatalf("decode anti-stack pickup get: %v", err)
	}
	if get != (itemproto.GetPacket{Vnum: 27003, Count: 2, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("unexpected anti-stack pickup get: %+v", get)
	}
	account, err := accounts.Load("pickup-anti-stack-owner")
	if err != nil {
		t.Fatalf("load pickup anti-stack owner account: %v", err)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected anti-stack pickup to preserve separate stacks, got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}
}

func TestGameRuntimeItemPickupMergePreservesTargetItemQuickslot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupQuickslotOwner", 0x01030191, 0x02040191, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 2021, Vnum: 27003, Count: 3, Slot: 0},
		{ID: 1021, Vnum: 27003, Count: 2, Slot: 6},
	}
	owner.Quickslots = []loginticket.Quickslot{
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 0},
		{Position: 3, Type: quickslotproto.TypeSkill, Slot: 0},
	}
	issuePeerTicket(t, ticketStore, "pickup-quickslot-owner", 0x91919191, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-quickslot-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup quickslot owner account: %v", err)
	}
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27003, Name: "Quickslot Merge Potion", Stackable: true, MaxCount: 200}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, itemStore)
	if err != nil {
		t.Fatalf("unexpected item-pickup quickslot runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-quickslot-owner", 0x91919191)
	flushServerFrames(t, flow)
	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(6))

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 3 {
		t.Fatalf("expected merge pickup to emit GROUND_DEL, ITEM_UPDATE, and ITEM_GET without quickslot frames, got %d frames", len(pickupOut))
	}
	update, err := itemproto.DecodeUpdate(decodeSingleFrame(t, pickupOut[1]))
	if err != nil {
		t.Fatalf("decode quickslot-preserving pickup update: %v", err)
	}
	if update.Position != itemproto.InventoryPosition(0) || update.Count != 5 {
		t.Fatalf("expected pickup to merge into quickslotted target slot 0, got %+v", update)
	}
	account, err := accounts.Load("pickup-quickslot-owner")
	if err != nil {
		t.Fatalf("load pickup quickslot owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{{ID: 2021, Vnum: 27003, Count: 5, Slot: 0}}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted pickup inventory: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
	if !reflect.DeepEqual(account.Characters[0].Quickslots, owner.Quickslots) {
		t.Fatalf("expected pickup merge to preserve target item and non-item quickslots, got %#v want %#v", account.Characters[0].Quickslots, owner.Quickslots)
	}
}

func TestGameRuntimeItemPickupSplitsStackableDropAcrossPartialStacksAndFreshSlot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupSplitOwner", 0x01030184, 0x02040184, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1013, Vnum: 27001, Count: 5, Slot: 6},
		{ID: 2013, Vnum: 27001, Count: 198, Slot: 0},
		{ID: 3013, Vnum: 27001, Count: 199, Slot: 2},
	}
	issuePeerTicket(t, ticketStore, "pickup-split-owner", 0x84848484, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-split-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup split owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected item-pickup split runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-split-owner", 0x84848484)
	flushServerFrames(t, flow)
	ground := dropAndDecodeGroundAdd(t, flow, itemproto.InventoryPosition(6))

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 5 {
		t.Fatalf("expected split pickup to emit GROUND_DEL, two ITEM_UPDATEs, ITEM_SET, and ITEM_GET, got %d frames", len(pickupOut))
	}
	firstUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, pickupOut[1]))
	if err != nil {
		t.Fatalf("decode split pickup first update: %v", err)
	}
	if firstUpdate.Position != itemproto.InventoryPosition(0) || firstUpdate.Count != 200 {
		t.Fatalf("unexpected first split pickup update: %+v", firstUpdate)
	}
	secondUpdate, err := itemproto.DecodeUpdate(decodeSingleFrame(t, pickupOut[2]))
	if err != nil {
		t.Fatalf("decode split pickup second update: %v", err)
	}
	if secondUpdate.Position != itemproto.InventoryPosition(2) || secondUpdate.Count != 200 {
		t.Fatalf("unexpected second split pickup update: %+v", secondUpdate)
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, pickupOut[3]))
	if err != nil {
		t.Fatalf("decode split pickup remainder set: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(6) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected split pickup remainder set: %+v", set)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, pickupOut[4]))
	if err != nil {
		t.Fatalf("decode split pickup item get: %v", err)
	}
	if get != (itemproto.GetPacket{Vnum: 27001, Count: 5, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("unexpected split pickup get: %+v", get)
	}
	account, err := accounts.Load("pickup-split-owner")
	if err != nil {
		t.Fatalf("load pickup split owner account: %v", err)
	}
	wantInventory := []inventory.ItemInstance{
		{ID: 2013, Vnum: 27001, Count: 200, Slot: 0},
		{ID: 3013, Vnum: 27001, Count: 200, Slot: 2},
		{ID: 1013, Vnum: 27001, Count: 2, Slot: 6},
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, wantInventory) {
		t.Fatalf("unexpected persisted inventory after split pickup: got %#v want %#v", account.Characters[0].Inventory, wantInventory)
	}
}

func TestGameRuntimeItemPickupRestoresOwnedVisibleDropToOriginalSlotWhenOriginalSlotIsFree(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupFirstEmptyOwner", 0x0103017b, 0x0204017b, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1008, Vnum: 27007, Count: 2, Slot: 6}}
	owner.Inventory = append(owner.Inventory, inventory.ItemInstance{ID: 2008, Vnum: 27008, Count: 1, Slot: 0})
	issuePeerTicket(t, ticketStore, "pickup-first-empty-owner", 0x7b7b7b7b, owner)
	if err := accounts.Save(accountstore.Account{Login: "pickup-first-empty-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup first-empty owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected item-pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-first-empty-owner", 0x7b7b7b7b)
	collectorFlow := ownerFlow
	flushServerFrames(t, ownerFlow)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))

	pickupOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(pickupOut) != 3 {
		t.Fatalf("expected occupied-original pickup to emit GROUND_DEL, ITEM_SET, and ITEM_GET, got %d frames", len(pickupOut))
	}
	set, err := itemproto.DecodeSet(decodeSingleFrame(t, pickupOut[1]))
	if err != nil {
		t.Fatalf("decode occupied-original pickup item set: %v", err)
	}
	if set.Position != itemproto.InventoryPosition(6) || set.Vnum != 27007 || set.Count != 2 {
		t.Fatalf("expected pickup to restore original slot 6, got %+v", set)
	}
	get, err := itemproto.DecodeGet(decodeSingleFrame(t, pickupOut[2]))
	if err != nil {
		t.Fatalf("decode occupied-original pickup item get: %v", err)
	}
	if get != (itemproto.GetPacket{Vnum: 27007, Count: 2, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("unexpected occupied-original pickup item get: %+v", get)
	}
	ownerAccount, err := accounts.Load("pickup-first-empty-owner")
	if err != nil {
		t.Fatalf("load pickup first-empty owner account: %v", err)
	}
	wantOwnerInventory := []inventory.ItemInstance{
		{ID: 2008, Vnum: 27008, Count: 1, Slot: 0},
		{ID: 1008, Vnum: 27007, Count: 2, Slot: 6},
	}
	if !reflect.DeepEqual(ownerAccount.Characters[0].Inventory, wantOwnerInventory) {
		t.Fatalf("unexpected persisted owner inventory after occupied-original pickup: got %#v want %#v", ownerAccount.Characters[0].Inventory, wantOwnerInventory)
	}
	if replayOut := pickupGroundItem(t, ownerFlow, ground.VID); len(replayOut) != 0 {
		t.Fatalf("expected owner replay after occupied-original pickup to fail closed, got %d frames", len(replayOut))
	}
}

func TestGameRuntimeItemPickupIgnoresCollectorCapacityWhenDeliveringToOwner(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupFullOwner", 0x0103017d, 0x0204017d, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1009, Vnum: 27001, Count: 2, Slot: 6}}
	collector := peerVisibilityCharacter("PickupFullCollector", 0x0103017e, 0x0204017e, 1450, 2450, 0, 101, 201)
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		collector.Inventory = append(collector.Inventory, inventory.ItemInstance{ID: uint64(3000 + slot), Vnum: 28000 + uint32(slot), Count: 1, Slot: slot})
	}
	issuePeerTicket(t, ticketStore, "pickup-full-owner", 0x7d7d7d7d, owner)
	issuePeerTicket(t, ticketStore, "pickup-full-collector", 0x7e7e7e7e, collector)
	if err := accounts.Save(accountstore.Account{Login: "pickup-full-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup full owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "pickup-full-collector", Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed pickup full collector account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected full-inventory item-pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-full-owner", 0x7d7d7d7d)
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "pickup-full-collector", 0x7e7e7e7e)
	flushServerFrames(t, ownerFlow)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))
	flushServerFrames(t, collectorFlow)

	collectorOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(collectorOut) != 2 {
		t.Fatalf("expected party owner-delivery pickup to emit GROUND_DEL and delivered ITEM_GET, got %d frames", len(collectorOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 3 {
		t.Fatalf("expected owner to receive ground delete, inventory refresh, and from-party ITEM_GET despite collector full inventory, got %d frames", len(queued))
	}
	collectorAccount, err := accounts.Load("pickup-full-collector")
	if err != nil {
		t.Fatalf("load pickup full collector account: %v", err)
	}
	if !reflect.DeepEqual(collectorAccount.Characters[0].Inventory, collector.Inventory) {
		t.Fatalf("expected full collector inventory to stay unchanged after owner-delivery pickup, got %#v want %#v", collectorAccount.Characters[0].Inventory, collector.Inventory)
	}
}

func TestGameRuntimeItemPickupOwnedByPartyMemberUsesOwnerCompatibleStackBeforeFreshSlot(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PartyPickupStackOwner", 0x01030185, 0x02040185, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{
		{ID: 1015, Vnum: 27001, Count: 3, Slot: 6},
		{ID: 2015, Vnum: 27001, Count: 4, Slot: 0},
	}
	collector := peerVisibilityCharacter("PartyPickupStackCollector", 0x01030186, 0x02040186, 1450, 2450, 0, 101, 201)
	collector.Inventory = []inventory.ItemInstance{{ID: 3015, Vnum: 27013, Count: 1, Slot: 6}}
	ownerLogin := "party-pickup-stack-owner-login"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x85858585, owner)
	issuePeerTicket(t, ticketStore, "party-pickup-stack-collector", 0x86868686, collector)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed party pickup stack owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "party-pickup-stack-collector", Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed party pickup stack collector account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected party stack item-pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x85858585)
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "party-pickup-stack-collector", 0x86868686)
	flushServerFrames(t, ownerFlow)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))
	flushServerFrames(t, collectorFlow)

	collectorOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(collectorOut) != 2 {
		t.Fatalf("expected party collector pickup to emit GROUND_DEL and delivered ITEM_GET, got %d frames", len(collectorOut))
	}
	ownerQueued := flushServerFrames(t, ownerFlow)
	if len(ownerQueued) != 3 {
		t.Fatalf("expected owner to receive peer ground delete, merged ITEM_UPDATE, and from-party ITEM_GET, got %d frames", len(ownerQueued))
	}
	ownerDelete, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, ownerQueued[0]))
	if err != nil {
		t.Fatalf("decode party owner peer ground delete: %v", err)
	}
	if ownerDelete.VID != ground.VID {
		t.Fatalf("unexpected party owner peer ground delete: %+v", ownerDelete)
	}
	ownerFrame := decodeSingleFrame(t, ownerQueued[1])
	if ownerFrame.Header != itemproto.HeaderUpdate {
		t.Fatalf("expected party owner merged refresh to use ITEM_UPDATE, got header %#x", ownerFrame.Header)
	}
	ownerUpdate, err := itemproto.DecodeUpdate(ownerFrame)
	if err != nil {
		t.Fatalf("decode party owner merged update: %v", err)
	}
	if ownerUpdate.Position != itemproto.InventoryPosition(0) || ownerUpdate.Count != 7 {
		t.Fatalf("expected owner-delivery pickup to merge into owner slot 0 with count 7, got %+v", ownerUpdate)
	}
	ownerGet, err := itemproto.DecodeGet(decodeSingleFrame(t, ownerQueued[2]))
	if err != nil {
		t.Fatalf("decode party owner get notice after merge: %v", err)
	}
	if ownerGet != (itemproto.GetPacket{Vnum: 27001, Count: 3, Arg: itemproto.GetArgFromPartyMember, FromName: collector.Name}) {
		t.Fatalf("unexpected party owner get notice after merge: %+v", ownerGet)
	}
	ownerAccount, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load party pickup stack owner: %v", err)
	}
	wantOwnerInventory := []inventory.ItemInstance{{ID: 2015, Vnum: 27001, Count: 7, Slot: 0}}
	if !reflect.DeepEqual(ownerAccount.Characters[0].Inventory, wantOwnerInventory) {
		t.Fatalf("unexpected owner inventory after party stack merge pickup: got %#v want %#v", ownerAccount.Characters[0].Inventory, wantOwnerInventory)
	}
}

func TestGameRuntimePartyItemPickupFallsBackToCollectorWhenOwnerInventoryIsFull(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PartyOwnerFullFallback", 0x01030198, 0x02040198, 1500, 2500, 0, 101, 201)
	owner.Inventory = fullBootstrapInventoryExcept(28042, 1, 5200)
	owner.Inventory[6] = inventory.ItemInstance{ID: 1024, Vnum: 27024, Count: 2, Slot: 6}
	collector := peerVisibilityCharacter("PartyCollectorFallback", 0x01030199, 0x02040199, owner.X, owner.Y, 0, 101, 201)
	ownerLogin := "party-owner-fallback"
	collectorLogin := "party-collector-fb"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x98989898, owner)
	issuePeerTicket(t, ticketStore, collectorLogin, 0x99999999, collector)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed party pickup fallback owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: collectorLogin, Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed party pickup fallback collector account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected party pickup fallback item runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x98989898)
	flushServerFrames(t, ownerFlow)
	dropOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(6), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected party pickup fallback owner drop error: %v", err)
	}
	if len(dropOut) != 3 {
		t.Fatalf("expected counted owner drop to emit ITEM_UPDATE, GROUND_ADD, and OWNERSHIP, got %d frames", len(dropOut))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, dropOut[1]))
	if err != nil {
		t.Fatalf("decode party pickup fallback ground add: %v", err)
	}
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), collectorLogin, 0x99999999)
	flushServerFrames(t, collectorFlow)
	flushServerFrames(t, ownerFlow)

	out := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(out) != 3 {
		t.Fatalf("expected party pickup fallback to collector to emit GROUND_DEL, ITEM_SET, and normal ITEM_GET, got %d", len(out))
	}
	collectorDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode party fallback collector ground del: %v", err)
	}
	if collectorDel.VID != ground.VID {
		t.Fatalf("unexpected party fallback collector ground del: got %+v want vid %d", collectorDel, ground.VID)
	}
	collectorSet, err := itemproto.DecodeSet(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode party fallback collector item set: %v", err)
	}
	if collectorSet.Position != itemproto.InventoryPosition(6) || collectorSet.Vnum != 27024 || collectorSet.Count != 1 {
		t.Fatalf("unexpected party fallback collector item set: %+v", collectorSet)
	}
	collectorGet, err := itemproto.DecodeGet(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode party fallback collector get notice: %v", err)
	}
	if collectorGet != (itemproto.GetPacket{Vnum: 27024, Count: 1, Arg: itemproto.GetArgNormal, FromName: ""}) {
		t.Fatalf("unexpected party fallback collector get notice: %+v", collectorGet)
	}

	ownerQueued := flushServerFrames(t, ownerFlow)
	if len(ownerQueued) != 1 {
		t.Fatalf("expected owner to receive only peer ground delete after collector fallback pickup, got %d frames", len(ownerQueued))
	}
	ownerDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, ownerQueued[0]))
	if err != nil {
		t.Fatalf("decode party fallback owner queued ground del: %v", err)
	}
	if ownerDel.VID != ground.VID {
		t.Fatalf("unexpected party fallback owner queued ground del: %+v", ownerDel)
	}

	ownerAccount, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load party pickup fallback owner account: %v", err)
	}
	wantOwnerInventory := append([]inventory.ItemInstance(nil), owner.Inventory...)
	wantOwnerInventory[6].Count = 1
	if !reflect.DeepEqual(ownerAccount.Characters[0].Inventory, wantOwnerInventory) {
		t.Fatalf("expected party fallback pickup to leave owner inventory unchanged after counted drop, got %#v want %#v", ownerAccount.Characters[0].Inventory, wantOwnerInventory)
	}
	collectorAccount, err := accounts.Load(collectorLogin)
	if err != nil {
		t.Fatalf("load party pickup fallback collector account: %v", err)
	}
	wantCollectorInventory := []inventory.ItemInstance{{ID: 1024, Vnum: 27024, Count: 1, Slot: 6}}
	if !reflect.DeepEqual(collectorAccount.Characters[0].Inventory, wantCollectorInventory) {
		t.Fatalf("expected party fallback pickup to place item in collector inventory, got %#v want %#v", collectorAccount.Characters[0].Inventory, wantCollectorInventory)
	}
	if replay := pickupGroundItem(t, collectorFlow, ground.VID); len(replay) != 0 {
		t.Fatalf("expected party fallback pickup to remove ground handle, replay got %d frames", len(replay))
	}
}

func TestGameRuntimePartyItemPickupNoFreeOwnerSlotEmitsInventoryFullInfoWithoutRemovingGroundHandle(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PartyPickupFullOwner", 0x01030196, 0x02040196, 1500, 2500, 0, 101, 201)
	owner.Inventory = fullBootstrapInventoryExcept(28042, 1, 5000)
	owner.Inventory[6] = inventory.ItemInstance{ID: 1022, Vnum: 27022, Count: 2, Slot: 6}
	collector := peerVisibilityCharacter("PartyPickupFullCollector", 0x01030197, 0x02040197, owner.X, owner.Y, 0, 101, 201)
	collector.Inventory = fullBootstrapInventoryExcept(29042, 1, 6000)
	ownerLogin := "party-pickup-full-owner"
	collectorLogin := "party-pickup-full-collector"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x96969696, owner)
	issuePeerTicket(t, ticketStore, collectorLogin, 0x97979797, collector)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed party pickup-full owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: collectorLogin, Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed party pickup-full collector account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected party pickup-full item runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x96969696)
	flushServerFrames(t, ownerFlow)
	dropOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(6), Count: 1})))
	if err != nil {
		t.Fatalf("unexpected party pickup-full owner drop error: %v", err)
	}
	if len(dropOut) != 3 {
		t.Fatalf("expected counted owner drop to emit ITEM_UPDATE, GROUND_ADD, and OWNERSHIP, got %d frames", len(dropOut))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, dropOut[1]))
	if err != nil {
		t.Fatalf("decode party pickup-full ground add: %v", err)
	}
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), collectorLogin, 0x97979797)
	flushServerFrames(t, collectorFlow)

	out := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(out) != 1 {
		t.Fatalf("expected party owner full-inventory pickup to emit one info chat frame, got %d", len(out))
	}
	info, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode party owner full-inventory info chat: %v", err)
	}
	if info.Type != chatproto.ChatTypeInfo || info.VID != 0 || info.Message != itemPickupInventoryFullInfoMessage {
		t.Fatalf("unexpected party owner full-inventory info chat: %+v", info)
	}

	ownerQueued := flushServerFrames(t, ownerFlow)
	if len(ownerQueued) != 3 {
		t.Fatalf("expected failed party pickup to leave only collector join frames queued for owner, got %d frames", len(ownerQueued))
	}
	ownerAccount, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load party pickup-full owner account: %v", err)
	}
	wantOwnerInventory := append([]inventory.ItemInstance(nil), owner.Inventory...)
	wantOwnerInventory[6].Count = 1
	if !reflect.DeepEqual(ownerAccount.Characters[0].Inventory, wantOwnerInventory) {
		t.Fatalf("expected party owner full-inventory pickup to leave owner inventory unchanged after counted drop, got %#v want %#v", ownerAccount.Characters[0].Inventory, wantOwnerInventory)
	}

	retry := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(retry) != 1 {
		t.Fatalf("expected failed party pickup to keep ground handle pending for retry, got %d frames", len(retry))
	}
}

func TestGameRuntimeItemPickupNoFreeSlotEmitsInventoryFullInfoWithoutRemovingGroundHandle(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupFullOwner", 0x01030194, 0x02040194, 1500, 2500, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1021, Vnum: 27021, Count: 1, Slot: 6}}
	collector := peerVisibilityCharacter("PickupFullCollector", 0x01030195, 0x02040195, owner.X, owner.Y, 0, 101, 201)
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		collector.Inventory = append(collector.Inventory, inventory.ItemInstance{ID: uint64(4096 + slot), Vnum: 28000 + uint32(slot), Count: 1, Slot: slot})
	}
	ownerLogin := "pickup-full-owner"
	collectorLogin := "pickup-full-collector"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x94949494, owner)
	issuePeerTicket(t, ticketStore, collectorLogin, 0x95959595, collector)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed pickup-full owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: collectorLogin, Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed pickup-full collector account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected pickup-full item runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x94949494)
	flushServerFrames(t, ownerFlow)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), collectorLogin, 0x95959595)
	flushServerFrames(t, collectorFlow)

	out := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(out) != 2 {
		t.Fatalf("expected full-inventory pickup to leave ground visible without committing inventory, got %d", len(out))
	}
	if groundDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, out[0])); err != nil || groundDel.VID != ground.VID {
		t.Fatalf("expected full-inventory pickup to keep ground handle available, got %+v err %v", groundDel, err)
	}

	collectorAccount, err := accounts.Load(collectorLogin)
	if err != nil {
		t.Fatalf("load pickup-full collector account: %v", err)
	}
	if !reflect.DeepEqual(collectorAccount.Characters[0].Inventory, collector.Inventory) {
		t.Fatalf("expected full-inventory collector inventory to stay unchanged, got %#v want %#v", collectorAccount.Characters[0].Inventory, collector.Inventory)
	}
}

func TestGameRuntimePartyItemPickupUpdatesLiveOwnerRuntimeForLaterItemActions(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PartyPickupLiveOwner", 0x01030196, 0x02040196, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1022, Vnum: 27022, Count: 1, Slot: 6}}
	collector := peerVisibilityCharacter("PartyPickupLiveCollector", 0x01030197, 0x02040197, owner.X, owner.Y, 0, 101, 201)
	collector.Inventory = []inventory.ItemInstance{{ID: 2022, Vnum: 27023, Count: 1, Slot: 6}}
	ownerLogin := "party-pickup-live-owner"
	collectorLogin := "party-pickup-live-collector"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x96969696, owner)
	issuePeerTicket(t, ticketStore, collectorLogin, 0x97979797, collector)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed party-pickup-live owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: collectorLogin, Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed party-pickup-live collector account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected party-pickup-live item runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x96969696)
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), collectorLogin, 0x97979797)
	flushServerFrames(t, ownerFlow)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))
	flushServerFrames(t, collectorFlow)

	collectorOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(collectorOut) != 2 {
		t.Fatalf("expected party collector pickup to emit GROUND_DEL and delivered ITEM_GET, got %d frames", len(collectorOut))
	}
	ownerQueued := flushServerFrames(t, ownerFlow)
	if len(ownerQueued) != 3 {
		t.Fatalf("expected owner to receive ground delete, ITEM_SET, and from-party ITEM_GET, got %d frames", len(ownerQueued))
	}

	redropOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(6)})))
	if err != nil {
		t.Fatalf("unexpected owner redrop error: %v", err)
	}
	if len(redropOut) != 3 {
		t.Fatalf("expected party owner live runtime to allow dropping delivered slot 6 item, got %d frames", len(redropOut))
	}
	redropGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, redropOut[1]))
	if err != nil {
		t.Fatalf("decode party owner redrop ground add: %v", err)
	}
	if redropGround.Vnum != 27022 {
		t.Fatalf("expected owner to redrop delivered vnum 27022, got %+v", redropGround)
	}
}

func TestGameRuntimeItemDropOwnerCloseRemovesPendingGroundHandleForVisiblePeers(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GroundCloseOwner", 0x01030192, 0x02040192, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1020, Vnum: 27020, Count: 1, Slot: 6}}
	watcher := peerVisibilityCharacter("GroundCloseWatcher", 0x01030193, 0x02040193, 1450, 2450, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "ground-close-owner", 0x92929292, owner)
	issuePeerTicket(t, ticketStore, "ground-close-watcher", 0x93939393, watcher)
	if err := accounts.Save(accountstore.Account{Login: "ground-close-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed ground-close owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "ground-close-watcher", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})}); err != nil {
		t.Fatalf("seed ground-close watcher account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected ground-close item runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ground-close-owner", 0x92929292)
	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "ground-close-watcher", 0x93939393)
	flushServerFrames(t, ownerFlow)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))
	watcherQueued := flushServerFrames(t, watcherFlow)
	if len(watcherQueued) != 2 {
		t.Fatalf("expected watcher to receive dropped ground add and ownership before owner close, got %d frames", len(watcherQueued))
	}

	closeSessionFlow(t, ownerFlow)
	ownerCloseQueued := flushServerFrames(t, watcherFlow)
	if len(ownerCloseQueued) != 2 {
		t.Fatalf("expected owner close to queue owner character delete and ground item delete, got %d frames", len(ownerCloseQueued))
	}
	ownerDelete, err := worldproto.DecodeCharacterDeleteNotice(decodeSingleFrame(t, ownerCloseQueued[0]))
	if err != nil {
		t.Fatalf("decode owner character delete after ground owner close: %v", err)
	}
	if ownerDelete.VID != owner.VID {
		t.Fatalf("expected owner close character delete for vid %d, got %+v", owner.VID, ownerDelete)
	}
	groundDelete, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, ownerCloseQueued[1]))
	if err != nil {
		t.Fatalf("decode ground delete after owner close: %v", err)
	}
	if groundDelete.VID != ground.VID {
		t.Fatalf("expected owner close to remove pending ground vid %d, got %+v", ground.VID, groundDelete)
	}

	pickupOut := pickupGroundItem(t, watcherFlow, ground.VID)
	if len(pickupOut) != 0 {
		t.Fatalf("expected closed owner's temporary ground handle to reject pickup, got %d frames", len(pickupOut))
	}
	closeSessionFlow(t, watcherFlow)
}

func TestGameRuntimeGoldPickupRejectsOverflowBeforeRemovingGroundHandle(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GoldOverflowOwner", 0x010301aa, 0x020401aa, 1400, 2400, 0, 101, 201)
	owner.Gold = uint64(1<<31 - 1)
	owner.Inventory = []inventory.ItemInstance{{ID: 1135, Vnum: 27032, Count: 1, Slot: 5}}
	issuePeerTicket(t, ticketStore, "gold-overflow-owner", 0xaaaaaaaa, owner)
	if err := accounts.Save(accountstore.Account{Login: "gold-overflow-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed gold overflow owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected gold-overflow runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "gold-overflow-owner", 0xaaaaaaaa)

	dropOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5), Elk: 1})))
	if err != nil {
		t.Fatalf("drop overflow test gold: %v", err)
	}
	if len(dropOut) != 3 {
		t.Fatalf("expected gold drop to emit point/add/ownership frames, got %d", len(dropOut))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, dropOut[1]))
	if err != nil {
		t.Fatalf("decode overflow gold ground add: %v", err)
	}

	reload, err := accounts.Load("gold-overflow-owner")
	if err != nil {
		t.Fatalf("load overflow account after drop: %v", err)
	}
	reload.Characters[0].Gold = uint64(1<<31 - 1)
	if err := accounts.Save(reload); err != nil {
		t.Fatalf("simulate external gold restoration before pickup: %v", err)
	}
	if !runtime.applyLiveCharacterPersistedSnapshot(owner.Name, reload.Characters[0]) {
		t.Fatal("expected live gold snapshot restoration before pickup")
	}

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 0 {
		t.Fatalf("expected overflowing gold pickup to fail closed without frames, got %d", len(pickupOut))
	}
	account, err := accounts.Load("gold-overflow-owner")
	if err != nil {
		t.Fatalf("reload overflow account after rejected pickup: %v", err)
	}
	if account.Characters[0].Gold != uint64(1<<31-1) {
		t.Fatalf("expected overflowing gold pickup to leave gold unchanged, got %d", account.Characters[0].Gold)
	}

	account.Characters[0].Gold = uint64(1<<31 - 2)
	if err := accounts.Save(account); err != nil {
		t.Fatalf("restore retryable account state: %v", err)
	}
	if !runtime.applyLiveCharacterPersistedSnapshot(owner.Name, account.Characters[0]) {
		t.Fatal("expected live gold snapshot restoration before retry")
	}
	retryOut := pickupGroundItem(t, flow, ground.VID)
	if len(retryOut) != 3 {
		t.Fatalf("expected overflow-rejected gold handle to remain retryable for self pickup, got %d frames", len(retryOut))
	}
	if _, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, retryOut[0])); err != nil {
		t.Fatalf("decode retry ground del: %v", err)
	}
	retryPoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, retryOut[1]))
	if err != nil {
		t.Fatalf("decode retry gold point change: %v", err)
	}
	if retryPoint != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: 1, Value: 1<<31 - 1}) {
		t.Fatalf("unexpected retry gold point change: %+v", retryPoint)
	}
	if _, err := itemproto.DecodeGet(decodeSingleFrame(t, retryOut[2])); err != nil {
		t.Fatalf("decode retry gold get: %v", err)
	}
}

func TestGameRuntimeGoldPickupOwnedByPartyMemberDeliversCurrencyToOwner(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PartyGoldOwner", 0x0103019c, 0x0204019c, 1400, 2400, 0, 101, 201)
	owner.Gold = 7000
	owner.Inventory = []inventory.ItemInstance{{ID: 1110, Vnum: 27032, Count: 1, Slot: 5}}
	collector := peerVisibilityCharacter("PartyGoldCollector", 0x0103019d, 0x0204019d, 1450, 2450, 0, 101, 201)
	collector.Gold = 300
	ownerLogin := "party-gold-owner-login"
	collectorLogin := "party-gold-collector-login"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x9c9c9c9c, owner)
	issuePeerTicket(t, ticketStore, collectorLogin, 0x9d9d9d9d, collector)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed party gold owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: collectorLogin, Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed party gold collector account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected party gold-pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x9c9c9c9c)
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), collectorLogin, 0x9d9d9d9d)
	flushServerFrames(t, ownerFlow)

	dropOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5), Elk: 1200})))
	if err != nil {
		t.Fatalf("drop party gold: %v", err)
	}
	if len(dropOut) != 3 {
		t.Fatalf("expected party gold drop point/add/ownership frames, got %d", len(dropOut))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, dropOut[1]))
	if err != nil {
		t.Fatalf("decode party gold ground add: %v", err)
	}
	flushServerFrames(t, collectorFlow)

	collectorOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(collectorOut) != 2 {
		t.Fatalf("expected party gold collector pickup to emit GROUND_DEL and delivered ITEM_GET, got %d frames", len(collectorOut))
	}
	collectorDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, collectorOut[0]))
	if err != nil {
		t.Fatalf("decode party gold collector ground del: %v", err)
	}
	if collectorDel.VID != ground.VID {
		t.Fatalf("unexpected party gold collector ground del: got %+v want vid %d", collectorDel, ground.VID)
	}
	collectorGet, err := itemproto.DecodeGet(decodeSingleFrame(t, collectorOut[1]))
	if err != nil {
		t.Fatalf("decode party gold collector get notice: %v", err)
	}
	if collectorGet != (itemproto.GetPacket{Vnum: 1, Count: 1, Arg: itemproto.GetArgDeliveredToPartyMember, FromName: owner.Name}) {
		t.Fatalf("unexpected party gold collector get notice: %+v", collectorGet)
	}

	ownerQueued := flushServerFrames(t, ownerFlow)
	if len(ownerQueued) != 3 {
		t.Fatalf("expected owner to receive ground delete, gold point change, and from-party ITEM_GET, got %d frames", len(ownerQueued))
	}
	ownerDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, ownerQueued[0]))
	if err != nil {
		t.Fatalf("decode party gold owner ground del: %v", err)
	}
	if ownerDel.VID != ground.VID {
		t.Fatalf("unexpected party gold owner ground del: got %+v want vid %d", ownerDel, ground.VID)
	}
	ownerPoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, ownerQueued[1]))
	if err != nil {
		t.Fatalf("decode party gold owner point change: %v", err)
	}
	if ownerPoint != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: 1200, Value: 7000}) {
		t.Fatalf("unexpected party gold owner point change: %+v", ownerPoint)
	}
	ownerGet, err := itemproto.DecodeGet(decodeSingleFrame(t, ownerQueued[2]))
	if err != nil {
		t.Fatalf("decode party gold owner get notice: %v", err)
	}
	if ownerGet != (itemproto.GetPacket{Vnum: 1, Count: 1, Arg: itemproto.GetArgFromPartyMember, FromName: collector.Name}) {
		t.Fatalf("unexpected party gold owner get notice: %+v", ownerGet)
	}

	ownerAccount, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load party gold owner account: %v", err)
	}
	if ownerAccount.Characters[0].Gold != 7000 {
		t.Fatalf("expected party gold owner to receive dropped gold back, got %d", ownerAccount.Characters[0].Gold)
	}
	collectorAccount, err := accounts.Load(collectorLogin)
	if err != nil {
		t.Fatalf("load party gold collector account: %v", err)
	}
	if collectorAccount.Characters[0].Gold != collector.Gold {
		t.Fatalf("expected party gold collector gold to stay unchanged, got %d want %d", collectorAccount.Characters[0].Gold, collector.Gold)
	}
}

func TestGameRuntimeGoldPickupOwnedByPartyMemberFailsClosedWhenOwnerPersistenceFails(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PartyGoldFailOwner", 0x0103019e, 0x0204019e, 1400, 2400, 0, 101, 201)
	owner.Gold = 7000
	owner.Inventory = []inventory.ItemInstance{{ID: 1120, Vnum: 27032, Count: 1, Slot: 5}}
	collector := peerVisibilityCharacter("PartyGoldFailCollector", 0x0103019f, 0x0204019f, 1450, 2450, 0, 101, 201)
	collector.Gold = 300
	ownerLogin := "pgfail-owner"
	collectorLogin := "pgfail-coll"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x9e9e9e9e, owner)
	issuePeerTicket(t, ticketStore, collectorLogin, 0x9f9f9f9f, collector)
	accounts := &failingSaveAccountStore{
		accounts: map[string]accountstore.Account{
			ownerLogin:     {Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})},
			collectorLogin: {Login: collectorLogin, Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})},
		},
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected party gold-pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x9e9e9e9e)
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), collectorLogin, 0x9f9f9f9f)
	flushServerFrames(t, ownerFlow)

	dropOut, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5), Elk: 1200})))
	if err != nil {
		t.Fatalf("drop party gold before failing pickup: %v", err)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, dropOut[1]))
	if err != nil {
		t.Fatalf("decode party gold ground add before failing pickup: %v", err)
	}
	flushServerFrames(t, collectorFlow)

	collectorOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(collectorOut) != 0 {
		t.Fatalf("expected owner-persistence failure to reject party gold pickup without collector frames, got %d", len(collectorOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected owner-persistence failure to avoid queued owner delivery frames, got %d", len(queued))
	}
	ownerAccount := accounts.accounts[ownerLogin]
	if ownerAccount.Characters[0].Gold != 5800 {
		t.Fatalf("expected failed owner delivery to leave owner account at dropped-gold total, got %d", ownerAccount.Characters[0].Gold)
	}
	collectorAccount := accounts.accounts[collectorLogin]
	if collectorAccount.Characters[0].Gold != collector.Gold {
		t.Fatalf("expected failed owner delivery to leave collector gold unchanged, got %d want %d", collectorAccount.Characters[0].Gold, collector.Gold)
	}

	ownerOut := pickupGroundItem(t, ownerFlow, ground.VID)
	if len(ownerOut) != 3 {
		t.Fatalf("expected failed owner-delivery handle to remain retryable for owner self pickup with visible confirmation, got %d frames", len(ownerOut))
	}
	ownerDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, ownerOut[0]))
	if err != nil {
		t.Fatalf("decode owner retry ground del: %v", err)
	}
	if ownerDel.VID != ground.VID {
		t.Fatalf("unexpected owner retry ground del: got %+v want vid %d", ownerDel, ground.VID)
	}
	ownerPoint, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, ownerOut[1]))
	if err != nil {
		t.Fatalf("decode owner retry point change: %v", err)
	}
	if ownerPoint != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: 1200, Value: 7000}) {
		t.Fatalf("unexpected owner retry point change: %+v", ownerPoint)
	}
	ownerGet, err := itemproto.DecodeGet(decodeSingleFrame(t, ownerOut[2]))
	if err != nil {
		t.Fatalf("decode owner retry gold pickup get notice: %v", err)
	}
	if ownerGet != (itemproto.GetPacket{Vnum: 1, Count: 1, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("unexpected owner retry gold pickup get notice: %+v", ownerGet)
	}
}

func TestGameRuntimeItemPickupOwnedByPartyMemberFailsClosedWhenOwnerPersistenceFails(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PartyItemFailOwner", 0x010301a4, 0x020401a4, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1130, Vnum: 27032, Count: 1, Slot: 5}}
	collector := peerVisibilityCharacter("PartyItemFailCollector", 0x010301a5, 0x020401a5, 1450, 2450, 0, 101, 201)
	ownerLogin := "pifail-owner"
	collectorLogin := "pifail-coll"
	issuePeerTicket(t, ticketStore, ownerLogin, 0xa4a4a4a4, owner)
	issuePeerTicket(t, ticketStore, collectorLogin, 0xa5a5a5a5, collector)
	accounts := &failingSaveAccountStore{
		accounts: map[string]accountstore.Account{
			ownerLogin:     {Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})},
			collectorLogin: {Login: collectorLogin, Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})},
		},
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected party item-pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0xa4a4a4a4)
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), collectorLogin, 0xa5a5a5a5)
	flushServerFrames(t, ownerFlow)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(5))
	flushServerFrames(t, collectorFlow)

	collectorOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(collectorOut) != 0 {
		t.Fatalf("expected owner-persistence failure to reject party item pickup without collector frames, got %d", len(collectorOut))
	}
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 0 {
		t.Fatalf("expected owner-persistence failure to avoid queued owner delivery frames, got %d", len(queued))
	}
	ownerAccount := accounts.accounts[ownerLogin]
	if len(ownerAccount.Characters[0].Inventory) != 0 {
		t.Fatalf("expected failed owner delivery to leave owner account at dropped empty inventory, got %#v", ownerAccount.Characters[0].Inventory)
	}
	collectorAccount := accounts.accounts[collectorLogin]
	if !reflect.DeepEqual(collectorAccount.Characters[0].Inventory, collector.Inventory) {
		t.Fatalf("expected failed owner delivery to leave collector inventory unchanged, got %#v want %#v", collectorAccount.Characters[0].Inventory, collector.Inventory)
	}

	ownerOut := pickupGroundItem(t, ownerFlow, ground.VID)
	if len(ownerOut) != 3 {
		t.Fatalf("expected failed owner-delivery handle to remain retryable for owner self pickup, got %d frames", len(ownerOut))
	}
	ownerDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, ownerOut[0]))
	if err != nil {
		t.Fatalf("decode owner retry ground del: %v", err)
	}
	if ownerDel.VID != ground.VID {
		t.Fatalf("unexpected owner retry ground del: got %+v want vid %d", ownerDel, ground.VID)
	}
	ownerSet, err := itemproto.DecodeSet(decodeSingleFrame(t, ownerOut[1]))
	if err != nil {
		t.Fatalf("decode owner retry item set: %v", err)
	}
	if ownerSet.Position != itemproto.InventoryPosition(5) || ownerSet.Vnum != 27032 || ownerSet.Count != 1 {
		t.Fatalf("unexpected owner retry item set: %+v", ownerSet)
	}
	ownerGet, err := itemproto.DecodeGet(decodeSingleFrame(t, ownerOut[2]))
	if err != nil {
		t.Fatalf("decode owner retry item get: %v", err)
	}
	if ownerGet.Vnum != 27032 || ownerGet.Count != 1 || ownerGet.Arg != itemproto.GetArgNormal || ownerGet.FromName != "" {
		t.Fatalf("unexpected owner retry item get: %+v", ownerGet)
	}
}

func TestGameRuntimeItemPickupOwnedByPartyMemberDeliversToOwner(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PartyPickupOwner", 0x01030180, 0x02040180, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1010, Vnum: 27010, Count: 2, Slot: 6}}
	collector := peerVisibilityCharacter("PartyPickupCollector", 0x01030181, 0x02040181, 1450, 2450, 0, 101, 201)
	collector.Inventory = []inventory.ItemInstance{{ID: 2010, Vnum: 27011, Count: 1, Slot: 6}}
	ownerLogin := "party-pickup-owner-login"
	issuePeerTicket(t, ticketStore, ownerLogin, 0x80808080, owner)
	issuePeerTicket(t, ticketStore, "party-pickup-collector", 0x81818181, collector)
	if err := accounts.Save(accountstore.Account{Login: ownerLogin, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed party pickup owner account: %v", err)
	}
	if err := accounts.Save(accountstore.Account{Login: "party-pickup-collector", Empire: collector.Empire, Characters: cloneCharacters([]loginticket.Character{collector})}); err != nil {
		t.Fatalf("seed party pickup collector account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected party item-pickup runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), ownerLogin, 0x80808080)
	collectorFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "party-pickup-collector", 0x81818181)
	flushServerFrames(t, ownerFlow)
	ground := dropAndDecodeGroundAdd(t, ownerFlow, itemproto.InventoryPosition(6))
	flushServerFrames(t, collectorFlow)

	collectorOut := pickupGroundItem(t, collectorFlow, ground.VID)
	if len(collectorOut) != 2 {
		t.Fatalf("expected party collector pickup to emit GROUND_DEL and delivered ITEM_GET, got %d frames", len(collectorOut))
	}
	collectorDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, collectorOut[0]))
	if err != nil {
		t.Fatalf("decode party collector ground del: %v", err)
	}
	if collectorDel.VID != ground.VID {
		t.Fatalf("unexpected party collector ground del: got %+v want vid %d", collectorDel, ground.VID)
	}
	collectorGet, err := itemproto.DecodeGet(decodeSingleFrame(t, collectorOut[1]))
	if err != nil {
		t.Fatalf("decode party collector get notice: %v", err)
	}
	if collectorGet != (itemproto.GetPacket{Vnum: 27010, Count: 2, Arg: itemproto.GetArgDeliveredToPartyMember, FromName: owner.Name}) {
		t.Fatalf("unexpected party collector get notice: %+v", collectorGet)
	}

	ownerQueued := flushServerFrames(t, ownerFlow)
	if len(ownerQueued) != 3 {
		t.Fatalf("expected owner to receive ground delete, ITEM_SET, and from-party ITEM_GET, got %d frames", len(ownerQueued))
	}
	ownerDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, ownerQueued[0]))
	if err != nil {
		t.Fatalf("decode party owner ground del: %v", err)
	}
	if ownerDel.VID != ground.VID {
		t.Fatalf("unexpected party owner ground del: got %+v want vid %d", ownerDel, ground.VID)
	}
	ownerSet, err := itemproto.DecodeSet(decodeSingleFrame(t, ownerQueued[1]))
	if err != nil {
		t.Fatalf("decode party owner set: %v", err)
	}
	if ownerSet.Position != itemproto.InventoryPosition(6) || ownerSet.Vnum != 27010 || ownerSet.Count != 2 {
		t.Fatalf("unexpected party owner set: %+v", ownerSet)
	}
	ownerGet, err := itemproto.DecodeGet(decodeSingleFrame(t, ownerQueued[2]))
	if err != nil {
		t.Fatalf("decode party owner get notice: %v", err)
	}
	if ownerGet != (itemproto.GetPacket{Vnum: 27010, Count: 2, Arg: itemproto.GetArgFromPartyMember, FromName: collector.Name}) {
		t.Fatalf("unexpected party owner get notice: %+v", ownerGet)
	}

	ownerAccount, err := accounts.Load(ownerLogin)
	if err != nil {
		t.Fatalf("load party pickup owner account: %v", err)
	}
	wantOwnerInventory := []inventory.ItemInstance{{ID: 1010, Vnum: 27010, Count: 2, Slot: 6}}
	if !reflect.DeepEqual(ownerAccount.Characters[0].Inventory, wantOwnerInventory) {
		t.Fatalf("unexpected persisted owner inventory after party pickup: got %#v want %#v", ownerAccount.Characters[0].Inventory, wantOwnerInventory)
	}
	collectorAccount, err := accounts.Load("party-pickup-collector")
	if err != nil {
		t.Fatalf("load party pickup collector account: %v", err)
	}
	if !reflect.DeepEqual(collectorAccount.Characters[0].Inventory, collector.Inventory) {
		t.Fatalf("expected party collector inventory to stay unchanged, got %#v want %#v", collectorAccount.Characters[0].Inventory, collector.Inventory)
	}
}

type failingSaveAccountStore struct {
	accounts       map[string]accountstore.Account
	failedGoldSave bool
	failedItemSave bool
}

func (s *failingSaveAccountStore) Load(login string) (accountstore.Account, error) {
	account, ok := s.accounts[login]
	if !ok {
		return accountstore.Account{}, accountstore.ErrAccountNotFound
	}
	return accountstore.Account{Login: account.Login, Empire: account.Empire, Characters: cloneCharacters(account.Characters)}, nil
}

func (s *failingSaveAccountStore) Save(account accountstore.Account) error {
	if account.Login == "pgfail-owner" && len(account.Characters) > 0 && account.Characters[0].Gold > 5800 && !s.failedGoldSave {
		s.failedGoldSave = true
		return errors.New("account save failed")
	}
	if account.Login == "pifail-owner" && len(account.Characters) > 0 && len(account.Characters[0].Inventory) > 0 && !s.failedItemSave {
		s.failedItemSave = true
		return errors.New("account save failed")
	}
	s.accounts[account.Login] = accountstore.Account{Login: account.Login, Empire: account.Empire, Characters: cloneCharacters(account.Characters)}
	return nil
}

func fullBootstrapInventoryExcept(vnum uint32, count uint16, startID uint64) []inventory.ItemInstance {
	items := make([]inventory.ItemInstance, 0, inventory.CarriedInventorySlotCount)
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		items = append(items, inventory.ItemInstance{ID: startID + uint64(slot) + 1, Vnum: vnum, Count: count, Slot: slot})
	}
	return items
}

func dropAndDecodeGroundAdd(t *testing.T, flow interface {
	HandleClientFrame(frame.Frame) ([][]byte, error)
}, position itemproto.Position) itemproto.GroundAddPacket {
	t.Helper()
	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: position})))
	if err != nil {
		t.Fatalf("unexpected item drop error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected item drop to emit ITEM_DEL, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
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
