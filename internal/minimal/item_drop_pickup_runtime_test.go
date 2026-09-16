package minimal

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

func TestGameRuntimeGoldDropProjectsCountOnSelfGroundAdd(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GoldCountOwner", 0x010301c1, 0x020401c1, 1300, 2300, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 2100, Vnum: 27030, Count: 2, Slot: 5}}
	issuePeerTicket(t, ticketStore, "gold-count-owner", 0xc1c1c1c1, owner)
	if err := accounts.Save(accountstore.Account{Login: "gold-count-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed gold-count owner account: %v", err)
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected gold-count runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "gold-count-owner", 0xc1c1c1c1)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5), Elk: 1200})))
	if err != nil {
		t.Fatalf("unexpected gold-count drop error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected gold-count drop to emit POINT_CHANGE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	point, err := worldproto.DecodePlayerPointChange(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode gold-count point change: %v", err)
	}
	if point != (worldproto.PlayerPointChangePacket{VID: owner.VID, Type: bootstrapGoldPointType, Amount: -1200, Value: 3800}) {
		t.Fatalf("unexpected gold-count point change: %+v", point)
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode gold-count ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 1 || ground.Count != 1200 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected gold-count ground add: %+v", ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, out[2]))
	if err != nil {
		t.Fatalf("decode gold-count ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected gold-count ownership: %+v", ownership)
	}

	maps := runtime.MapOccupancy()
	if len(maps) != 1 || maps[0].GroundItemCount != 1 || len(maps[0].GroundItems) != 1 {
		t.Fatalf("unexpected gold-count occupancy summary: %+v", maps)
	}
	got := maps[0].GroundItems[0]
	if got.VID != ground.VID || got.Vnum != 1 || got.Count != 0 || got.GoldAmount != 1200 || got.OwnerName != owner.Name {
		t.Fatalf("unexpected gold-count occupancy item: %+v", got)
	}
	if !runtime.sharedWorld.GroundItemExists(ground.VID) {
		t.Fatal("expected gold-count marker to stay registered after drop")
	}

	account, err := accounts.Load("gold-count-owner")
	if err != nil {
		t.Fatalf("load gold-count owner account: %v", err)
	}
	if account.Characters[0].Gold != 3800 {
		t.Fatalf("expected persisted gold 3800 after gold-count drop, got %d", account.Characters[0].Gold)
	}
	if !reflect.DeepEqual(account.Characters[0].Inventory, owner.Inventory) {
		t.Fatalf("expected gold-count drop to leave inventory unchanged, got %#v want %#v", account.Characters[0].Inventory, owner.Inventory)
	}

	pickupOut := pickupGroundItem(t, flow, ground.VID)
	if len(pickupOut) != 3 {
		t.Fatalf("expected gold-count pickup to emit GROUND_DEL, POINT_CHANGE, and ITEM_GET, got %d frames", len(pickupOut))
	}
	pickupGet, err := itemproto.DecodeGet(decodeSingleFrame(t, pickupOut[2]))
	if err != nil {
		t.Fatalf("decode gold-count pickup get: %v", err)
	}
	if pickupGet != (itemproto.GetPacket{Vnum: 1, Count: 1, Arg: itemproto.GetArgNormal}) {
		t.Fatalf("unexpected gold-count pickup get: %+v", pickupGet)
	}
	account, err = accounts.Load("gold-count-owner")
	if err != nil {
		t.Fatalf("reload gold-count owner account after pickup: %v", err)
	}
	if account.Characters[0].Gold != 5000 {
		t.Fatalf("expected persisted gold restored to 5000 after gold-count pickup, got %d", account.Characters[0].Gold)
	}

	drop2Out, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop2(itemproto.ClientDrop2Packet{Position: itemproto.InventoryPosition(5), Gold: 2500, Count: 4})))
	if err != nil {
		t.Fatalf("unexpected gold-count drop2 error: %v", err)
	}
	if len(drop2Out) != 3 {
		t.Fatalf("expected gold-count drop2 to emit POINT_CHANGE, GROUND_ADD, and OWNERSHIP, got %d frames", len(drop2Out))
	}
	ground2, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, drop2Out[1]))
	if err != nil {
		t.Fatalf("decode gold-count drop2 ground add: %v", err)
	}
	if ground2.VID == 0 || ground2.VID == ground.VID || ground2.Vnum != 1 || ground2.Count != 2500 {
		t.Fatalf("unexpected gold-count drop2 ground add: %+v", ground2)
	}
	if runtime.sharedWorld.GroundItemExists(ground.VID) || !runtime.sharedWorld.GroundItemExists(ground2.VID) {
		t.Fatal("expected only the drop2 gold-count marker to remain after pickup")
	}
}

func TestGameRuntimeGoldDropProjectsCountOnPeerGroundAdd(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("GoldCountPeerOwner", 0x010301c2, 0x020401c2, 1300, 2300, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 2101, Vnum: 27030, Count: 2, Slot: 5}}
	watcher := peerVisibilityCharacter("GoldCountPeerWatcher", 0x010301c3, 0x020401c3, 1350, 2350, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "gold-count-peer-owner", 0xc2c2c2c2, owner)
	issuePeerTicket(t, ticketStore, "gold-count-peer-watcher", 0xc3c3c3c3, watcher)
	for _, account := range []accountstore.Account{
		{Login: "gold-count-peer-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})},
		{Login: "gold-count-peer-watcher", Empire: watcher.Empire, Characters: cloneCharacters([]loginticket.Character{watcher})},
	} {
		if err := accounts.Save(account); err != nil {
			t.Fatalf("seed %s account: %v", account.Login, err)
		}
	}

	runtime, err := newGameRuntimeWithAccountStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts)
	if err != nil {
		t.Fatalf("unexpected gold-count peer runtime error: %v", err)
	}
	ownerFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "gold-count-peer-owner", 0xc2c2c2c2)
	defer closeSessionFlow(t, ownerFlow)
	watcherFlow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "gold-count-peer-watcher", 0xc3c3c3c3)
	defer closeSessionFlow(t, watcherFlow)
	_ = flushServerFrames(t, ownerFlow)
	_ = flushServerFrames(t, watcherFlow)

	out, err := ownerFlow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientDrop(itemproto.ClientDropPacket{Position: itemproto.InventoryPosition(5), Elk: 1200})))
	if err != nil {
		t.Fatalf("unexpected gold-count peer drop error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected gold-count peer drop to emit POINT_CHANGE, GROUND_ADD, and OWNERSHIP, got %d frames", len(out))
	}
	ground, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, out[1]))
	if err != nil {
		t.Fatalf("decode gold-count owner ground add: %v", err)
	}
	if ground.VID == 0 || ground.Vnum != 1 || ground.Count != 1200 || ground.X != owner.X || ground.Y != owner.Y || ground.Z != owner.Z {
		t.Fatalf("unexpected gold-count owner ground add: %+v", ground)
	}

	queued := flushServerFrames(t, watcherFlow)
	if len(queued) != 2 {
		t.Fatalf("expected gold-count peer drop to queue GROUND_ADD and OWNERSHIP, got %d frames", len(queued))
	}
	peerGround, err := itemproto.DecodeGroundAdd(decodeSingleFrame(t, queued[0]))
	if err != nil {
		t.Fatalf("decode gold-count peer ground add: %v", err)
	}
	if peerGround != ground {
		t.Fatalf("unexpected gold-count peer ground add: got %+v want %+v", peerGround, ground)
	}
	ownership, err := itemproto.DecodeOwnership(decodeSingleFrame(t, queued[1]))
	if err != nil {
		t.Fatalf("decode gold-count peer ownership: %v", err)
	}
	if ownership != (itemproto.OwnershipPacket{VID: ground.VID, OwnerName: owner.Name}) {
		t.Fatalf("unexpected gold-count peer ownership: %+v", ownership)
	}
}
