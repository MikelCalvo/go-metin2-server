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

func TestGameRuntimeItemDrop2NormalizesOversizedCountToWholeStack(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("DropOversizedOwner", 0x01030192, 0x02040192, 1250, 2250, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1020, Vnum: 27001, Count: 5, Slot: 5}}
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
	if len(out) != 3 {
		t.Fatalf("expected oversized counted drop to emit ITEM_DEL, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	del, err := itemproto.DecodeDel(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode oversized item drop2 del: %v", err)
	}
	if del.Position != itemproto.InventoryPosition(5) {
		t.Fatalf("unexpected oversized item drop2 del: %+v", del)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode oversized item drop2 ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 27001 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected oversized item drop2 ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[2]))
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
	if len(pickupOut) != 2 {
		t.Fatalf("expected gold pickup to emit GROUND_DEL and POINT_CHANGE, got %d frames", len(pickupOut))
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

	moveOut, err := moverFlow.HandleClientFrame(decodeSingleFrame(t, movep.EncodeMove(movep.MovePacket{Func: 1, Arg: 0, Rot: 12, X: 1500, Y: 2600, Time: 0x51525354})))
	if err != nil {
		t.Fatalf("unexpected transfer move error: %v", err)
	}
	if len(moveOut) != 11 {
		t.Fatalf("expected self bootstrap, peer del/add, and ground del/add/ownership transfer frames, got %d", len(moveOut))
	}
	sourceDelete, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, moveOut[8]))
	if err != nil {
		t.Fatalf("decode transfer source ground delete: %v", err)
	}
	if sourceDelete.VID != sourceGround.VID {
		t.Fatalf("unexpected source ground delete after transfer: got %+v want vid %d", sourceDelete, sourceGround.VID)
	}
	destAdd, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, moveOut[9]))
	if err != nil {
		t.Fatalf("decode transfer destination ground add: %v", err)
	}
	if destAdd != destGround {
		t.Fatalf("unexpected destination ground add after transfer: got %+v want %+v", destAdd, destGround)
	}
	destOwnership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, moveOut[10]))
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

func TestGameRuntimeItemPickupPlacesOwnedVisibleDropIntoFirstEmptySlotWhenOriginalSlotOccupied(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("PickupFirstEmptyOwner", 0x0103017b, 0x0204017b, 1400, 2400, 0, 101, 201)
	owner.Inventory = []inventory.ItemInstance{{ID: 1008, Vnum: 27007, Count: 2, Slot: 6}}
	owner.Inventory = append(owner.Inventory, inventory.ItemInstance{ID: 2008, Vnum: 27008, Count: 1, Slot: 6})
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
	if set.Position != itemproto.InventoryPosition(0) || set.Vnum != 27007 || set.Count != 2 {
		t.Fatalf("expected pickup to choose first empty slot 0, got %+v", set)
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
		{ID: 1008, Vnum: 27007, Count: 2, Slot: 0},
		{ID: 2008, Vnum: 27008, Count: 1, Slot: 6},
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
	if len(ownerOut) != 2 {
		t.Fatalf("expected failed owner-delivery handle to remain retryable for owner self pickup, got %d frames", len(ownerOut))
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
