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
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
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
	owner.Inventory = []inventory.ItemInstance{{ID: 1009, Vnum: 27009, Count: 2, Slot: 6}}
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
	if queued := flushServerFrames(t, ownerFlow); len(queued) != 2 {
		t.Fatalf("expected owner to receive ground delete and from-party ITEM_GET despite collector full inventory, got %d frames", len(queued))
	}
	collectorAccount, err := accounts.Load("pickup-full-collector")
	if err != nil {
		t.Fatalf("load pickup full collector account: %v", err)
	}
	if !reflect.DeepEqual(collectorAccount.Characters[0].Inventory, collector.Inventory) {
		t.Fatalf("expected full collector inventory to stay unchanged after owner-delivery pickup, got %#v want %#v", collectorAccount.Characters[0].Inventory, collector.Inventory)
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
	if len(ownerQueued) != 2 {
		t.Fatalf("expected owner to receive ground delete and from-party ITEM_GET, got %d frames", len(ownerQueued))
	}
	ownerDel, err := itemproto.DecodeGroundDel(decodeSingleFrame(t, ownerQueued[0]))
	if err != nil {
		t.Fatalf("decode party owner ground del: %v", err)
	}
	if ownerDel.VID != ground.VID {
		t.Fatalf("unexpected party owner ground del: got %+v want vid %d", ownerDel, ground.VID)
	}
	ownerGet, err := itemproto.DecodeGet(decodeSingleFrame(t, ownerQueued[1]))
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
